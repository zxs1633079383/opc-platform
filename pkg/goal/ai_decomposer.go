package goal

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"time"

	v1 "github.com/zlc-ai/opc-platform/api/v1"
	"go.uber.org/zap"
)

// AgentController defines the minimal controller interface needed by AIDecomposer.
type AgentController interface {
	ExecuteTask(ctx context.Context, task v1.TaskRecord) (ExecuteResult, error)
	Apply(ctx context.Context, spec v1.AgentSpec) error
	StartAgent(ctx context.Context, name string) error
	GetAgent(ctx context.Context, name string) (v1.AgentRecord, error)
}

// ExecuteResult mirrors adapter.ExecuteResult to avoid circular imports.
type ExecuteResult struct {
	Output    string
	TokensIn  int
	TokensOut int
}

// controllerShim wraps a real controller to satisfy the AgentController interface.
type controllerShim struct {
	ctrl interface {
		ExecuteTask(ctx context.Context, task v1.TaskRecord) (interface{ GetOutput() string }, error)
	}
}

const (
	decomposerAgentName = "opc-decomposer"
	maxRetries          = 2
)

// AIDecomposer uses an AI agent (via the controller) to intelligently
// decompose goals into projects, tasks, and issues.
type AIDecomposer struct {
	controller  AgentController
	constraints *v1.DecomposeConstraints
	logger      *zap.SugaredLogger
}

// NewAIDecomposer creates a new AI-powered decomposer.
func NewAIDecomposer(ctrl AgentController, logger *zap.SugaredLogger) *AIDecomposer {
	return &AIDecomposer{
		controller: ctrl,
		logger:     logger,
	}
}

// Decompose uses an AI agent to break down the goal into a structured plan.
func (d *AIDecomposer) Decompose(ctx context.Context, req DecomposeRequest) (*DecomposeResult, error) {
	start := time.Now()
	d.logger.Infow("Decompose", "goalId", req.GoalID, "goalName", req.GoalName)

	// Ensure the system decomposer agent exists.
	if err := d.ensureDecomposerAgent(ctx); err != nil {
		d.logger.Errorw("Decompose: failed to ensure decomposer agent", "goalId", req.GoalID, "error", err)
		return nil, fmt.Errorf("ensure decomposer agent: %w", err)
	}

	prompt := BuildDecompositionPrompt(req.GoalName, req.Description, d.constraints)
	d.logger.Debugw("Decompose: prompt built", "goalId", req.GoalID, "promptLen", len(prompt))

	var lastErr error
	for attempt := 0; attempt <= maxRetries; attempt++ {
		if attempt > 0 {
			d.logger.Infow("retrying AI decomposition",
				"goalId", req.GoalID,
				"attempt", attempt+1,
			)
		}

		aiResult, err := d.callAI(ctx, req.GoalID, prompt)
		if err != nil {
			lastErr = err
			continue
		}

		if err := d.validateResult(aiResult); err != nil {
			lastErr = fmt.Errorf("validation failed: %w", err)
			d.logger.Warnw("AI decomposition validation failed",
				"goalId", req.GoalID,
				"attempt", attempt+1,
				"error", err,
			)
			continue
		}

		// Convert AIDecomposeResult to DecomposeResult.
		result := d.convertResult(req.GoalID, aiResult)

		d.logger.Infow("Decompose completed",
			"goalId", req.GoalID,
			"projects", len(result.Projects),
			"duration", time.Since(start),
		)

		return result, nil
	}

	d.logger.Errorw("Decompose: all retries exhausted",
		"goalId", req.GoalID, "totalAttempts", maxRetries+1, "error", lastErr, "duration", time.Since(start))
	return nil, fmt.Errorf("AI decomposition failed after %d attempts: %w", maxRetries+1, lastErr)
}

// SetConstraints configures decomposition constraints for validation.
func (d *AIDecomposer) SetConstraints(c *v1.DecomposeConstraints) {
	d.constraints = c
}

// ensureDecomposerAgent creates the system decomposer agent if it doesn't exist.
func (d *AIDecomposer) ensureDecomposerAgent(ctx context.Context) error {
	_, err := d.controller.GetAgent(ctx, decomposerAgentName)
	if err == nil {
		// Agent already exists.
		return nil
	}

	spec := v1.AgentSpec{
		APIVersion: v1.APIVersion,
		Kind:       v1.KindAgentSpec,
		Metadata: v1.Metadata{
			Name: decomposerAgentName,
		},
		Spec: v1.AgentSpecBody{
			Type: v1.AgentTypeClaudeCode,
			Runtime: v1.RuntimeConfig{
				Model: v1.ModelConfig{
					Name: "sonnet-4",
				},
				Timeout: v1.TimeoutConfig{
					Task: "5m",
				},
			},
			Context: v1.ContextConfig{
				Workdir: "/tmp/opc/decomposer",
			},
		},
	}

	if err := d.controller.Apply(ctx, spec); err != nil {
		return fmt.Errorf("apply decomposer agent: %w", err)
	}

	if err := d.controller.StartAgent(ctx, decomposerAgentName); err != nil {
		return fmt.Errorf("start decomposer agent: %w", err)
	}

	d.logger.Infow("created system decomposer agent", "name", decomposerAgentName)
	return nil
}

// callAI sends the decomposition prompt to the AI agent and parses the response.
func (d *AIDecomposer) callAI(ctx context.Context, goalID, prompt string) (*AIDecomposeResult, error) {
	start := time.Now()
	taskID := fmt.Sprintf("decompose-%s-%d", goalID, time.Now().UnixMilli())
	d.logger.Infow("callAI", "goalId", goalID, "taskId", taskID, "promptLen", len(prompt))

	task := v1.TaskRecord{
		ID:        taskID,
		AgentName: decomposerAgentName,
		Message:   prompt,
		Status:    v1.TaskStatusPending,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	result, err := d.controller.ExecuteTask(ctx, task)
	if err != nil {
		d.logger.Errorw("callAI: execution failed", "goalId", goalID, "taskId", taskID, "error", err, "duration", time.Since(start))
		return nil, fmt.Errorf("execute decomposition task: %w", err)
	}

	output := result.Output
	d.logger.Debugw("callAI: AI response received", "goalId", goalID, "responseLen", len(output), "duration", time.Since(start))

	// Try to extract JSON from the response (in case it's wrapped in markdown).
	output = extractJSON(output)

	var aiResult AIDecomposeResult
	if err := json.Unmarshal([]byte(output), &aiResult); err != nil {
		d.logger.Warnw("callAI: failed to parse AI response", "goalId", goalID, "error", err, "rawLen", len(output))
		return nil, fmt.Errorf("parse AI response as JSON: %w (raw: %.200s)", err, output)
	}

	d.logger.Infow("callAI completed", "goalId", goalID, "taskId", taskID, "projects", len(aiResult.Projects), "duration", time.Since(start))
	return &aiResult, nil
}

// validateResult checks the structural integrity of the AI decomposition output.
func (d *AIDecomposer) validateResult(result *AIDecomposeResult) error {
	d.logger.Debugw("validateResult", "projects", len(result.Projects))
	if len(result.Projects) == 0 {
		return fmt.Errorf("no projects in decomposition result")
	}

	agentNameRegex := regexp.MustCompile(`^[a-z][a-z0-9-]*$`)

	for _, project := range result.Projects {
		if project.Name == "" {
			return fmt.Errorf("project has empty name")
		}
		if len(project.Tasks) == 0 {
			return fmt.Errorf("project %q has no tasks", project.Name)
		}
		for _, task := range project.Tasks {
			if task.Name == "" {
				return fmt.Errorf("task has empty name in project %q", project.Name)
			}
			if task.Description == "" {
				return fmt.Errorf("task %q has empty description", task.Name)
			}
			if task.AssignAgent != "" && !agentNameRegex.MatchString(task.AssignAgent) {
				return fmt.Errorf("task %q has invalid agent name %q (must be lowercase kebab-case)", task.Name, task.AssignAgent)
			}
			switch task.Complexity {
			case "low", "medium", "high", "":
				// valid
			default:
				return fmt.Errorf("task %q has invalid complexity %q", task.Name, task.Complexity)
			}
		}
	}

	// Validate constraints if set.
	if d.constraints != nil {
		if err := validateConstraints(result, d.constraints); err != nil {
			return err
		}
	}

	return nil
}

// validateConstraints checks the decomposition result against the given constraints.
func validateConstraints(result *AIDecomposeResult, constraints *v1.DecomposeConstraints) error {
	if constraints.MaxProjects > 0 && len(result.Projects) > constraints.MaxProjects {
		return fmt.Errorf("decomposition has %d projects, max allowed is %d",
			len(result.Projects), constraints.MaxProjects)
	}

	uniqueAgents := make(map[string]struct{})

	for _, project := range result.Projects {
		if constraints.MaxTasksPerProject > 0 && len(project.Tasks) > constraints.MaxTasksPerProject {
			return fmt.Errorf("project %q has %d tasks, max allowed is %d",
				project.Name, len(project.Tasks), constraints.MaxTasksPerProject)
		}
		for _, task := range project.Tasks {
			if task.AssignAgent != "" {
				uniqueAgents[task.AssignAgent] = struct{}{}
			}
		}
	}

	if constraints.MaxAgents > 0 && len(uniqueAgents) > constraints.MaxAgents {
		return fmt.Errorf("decomposition uses %d unique agents, max allowed is %d",
			len(uniqueAgents), constraints.MaxAgents)
	}

	return nil
}

// convertResult transforms an AIDecomposeResult into the internal DecomposeResult format.
func (d *AIDecomposer) convertResult(goalID string, aiResult *AIDecomposeResult) *DecomposeResult {
	projects := make([]*Project, 0, len(aiResult.Projects))

	for _, ap := range aiResult.Projects {
		projectID := fmt.Sprintf("proj-%s-%d", goalID, len(projects)+1)

		tasks := make([]*Task, 0, len(ap.Tasks))
		for ti, at := range ap.Tasks {
			taskID := fmt.Sprintf("task-%s-%d-%d", goalID, len(projects)+1, ti+1)

			issues := make([]*Issue, 0, len(at.Issues))
			for ii, ai := range at.Issues {
				issueID := fmt.Sprintf("issue-%s-%d-%d-%d", goalID, len(projects)+1, ti+1, ii+1)
				issues = append(issues, &Issue{
					ID:            issueID,
					TaskID:        taskID,
					Name:          ai.Name,
					AssignedAgent: at.AssignAgent,
					Context: map[string]interface{}{
						"description": ai.Description,
						"complexity":  at.Complexity,
					},
				})
			}

			tasks = append(tasks, &Task{
				ID:        taskID,
				ProjectID: projectID,
				Name:      at.Name,
				Issues:    issues,
			})
		}

		projects = append(projects, &Project{
			ID:     projectID,
			GoalID: goalID,
			Name:   ap.Name,
			Tasks:  tasks,
		})
	}

	return &DecomposeResult{Projects: projects}
}

// extractJSON attempts to extract a JSON object from text that may contain
// markdown code fences or other wrapping.
func extractJSON(s string) string {
	// Try to find JSON between code fences.
	re := regexp.MustCompile("(?s)```(?:json)?\\s*\n?(\\{.*?})\\s*\n?```")
	if matches := re.FindStringSubmatch(s); len(matches) > 1 {
		return matches[1]
	}

	// Try to find a bare JSON object.
	start := -1
	for i, c := range s {
		if c == '{' {
			start = i
			break
		}
	}
	if start >= 0 {
		depth := 0
		for i := start; i < len(s); i++ {
			switch s[i] {
			case '{':
				depth++
			case '}':
				depth--
				if depth == 0 {
					return s[start : i+1]
				}
			}
		}
	}

	return s
}
