package workflow

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"sync"
	"time"

	"go.uber.org/zap"
	"gopkg.in/yaml.v3"

	v1 "github.com/zlc-ai/opc-platform/api/v1"
	"github.com/zlc-ai/opc-platform/pkg/controller"
	"github.com/zlc-ai/opc-platform/pkg/storage"
)

// WorkflowStatus represents the overall status of a workflow run.
type WorkflowStatus string

const (
	WorkflowStatusPending   WorkflowStatus = "Pending"
	WorkflowStatusRunning   WorkflowStatus = "Running"
	WorkflowStatusCompleted WorkflowStatus = "Completed"
	WorkflowStatusFailed    WorkflowStatus = "Failed"
)

// WorkflowSpec is the YAML definition of a workflow.
type WorkflowSpec struct {
	APIVersion string       `yaml:"apiVersion" json:"apiVersion"`
	Kind       string       `yaml:"kind" json:"kind"`
	Metadata   v1.Metadata  `yaml:"metadata" json:"metadata"`
	Spec       WorkflowBody `yaml:"spec" json:"spec"`
}

// WorkflowBody contains the body of a workflow spec.
type WorkflowBody struct {
	Schedule string         `yaml:"schedule,omitempty" json:"schedule,omitempty"`
	Steps    []WorkflowStep `yaml:"steps" json:"steps"`
}

// WorkflowStep defines a single step in the workflow DAG.
type WorkflowStep struct {
	Name      string           `yaml:"name" json:"name"`
	Agent     string           `yaml:"agent" json:"agent"`
	DependsOn []string         `yaml:"dependsOn,omitempty" json:"dependsOn,omitempty"`
	Input     WorkflowInput    `yaml:"input" json:"input"`
	Outputs   []WorkflowOutput `yaml:"outputs,omitempty" json:"outputs,omitempty"`
}

// WorkflowInput contains the input configuration for a step.
type WorkflowInput struct {
	Message string   `yaml:"message" json:"message"`
	Context []string `yaml:"context,omitempty" json:"context,omitempty"`
}

// WorkflowOutput defines a named output from a step.
type WorkflowOutput struct {
	Name string `yaml:"name" json:"name"`
}

// WorkflowRun tracks a running workflow instance.
type WorkflowRun struct {
	ID           string                `json:"id"`
	WorkflowName string                `json:"workflowName"`
	Status       WorkflowStatus        `json:"status"`
	Steps        map[string]*StepRun   `json:"steps"`
	StartedAt    time.Time             `json:"startedAt"`
	EndedAt      *time.Time            `json:"endedAt,omitempty"`
}

// StepRun tracks the execution state of a single workflow step.
type StepRun struct {
	Name      string     `json:"name"`
	Status    string     `json:"status"` // Pending, Running, Completed, Failed, Skipped
	TaskID    string     `json:"taskId,omitempty"`
	Output    string     `json:"output,omitempty"`
	Error     string     `json:"error,omitempty"`
	StartedAt *time.Time `json:"startedAt,omitempty"`
	EndedAt   *time.Time `json:"endedAt,omitempty"`
}

// contextVarPattern matches ${{ steps.<step>.outputs.<name> }} expressions.
var contextVarPattern = regexp.MustCompile(`\$\{\{\s*steps\.([^.]+)\.outputs\.([^}\s]+)\s*\}\}`)

// Engine orchestrates DAG-based multi-agent workflows.
type Engine struct {
	controller *controller.Controller
	store      storage.Store
	logger     *zap.SugaredLogger
}

// NewEngine creates a new workflow Engine.
func NewEngine(ctrl *controller.Controller, store storage.Store, logger *zap.SugaredLogger) *Engine {
	return &Engine{
		controller: ctrl,
		store:      store,
		logger:     logger,
	}
}

// ParseWorkflow parses a YAML document into a WorkflowSpec.
func ParseWorkflow(data []byte) (WorkflowSpec, error) {
	var spec WorkflowSpec
	if err := yaml.Unmarshal(data, &spec); err != nil {
		return WorkflowSpec{}, fmt.Errorf("parse workflow YAML: %w", err)
	}

	if spec.APIVersion == "" {
		return WorkflowSpec{}, fmt.Errorf("apiVersion is required")
	}
	if spec.Kind != v1.KindWorkflow {
		return WorkflowSpec{}, fmt.Errorf("expected kind %q, got %q", v1.KindWorkflow, spec.Kind)
	}
	if spec.Metadata.Name == "" {
		return WorkflowSpec{}, fmt.Errorf("metadata.name is required")
	}
	if len(spec.Spec.Steps) == 0 {
		return WorkflowSpec{}, fmt.Errorf("workflow must have at least one step")
	}

	return spec, nil
}

// Execute runs a workflow to completion.
// Steps are executed in topological order; independent steps within the same
// layer run concurrently.
func (e *Engine) Execute(ctx context.Context, spec WorkflowSpec) (*WorkflowRun, error) {
	if err := e.validateDAG(spec.Spec.Steps); err != nil {
		return nil, fmt.Errorf("invalid workflow DAG: %w", err)
	}

	layers, err := e.buildDAG(spec.Spec.Steps)
	if err != nil {
		return nil, fmt.Errorf("build DAG: %w", err)
	}

	run := &WorkflowRun{
		ID:           generateRunID(spec.Metadata.Name),
		WorkflowName: spec.Metadata.Name,
		Status:       WorkflowStatusRunning,
		Steps:        make(map[string]*StepRun, len(spec.Spec.Steps)),
		StartedAt:    time.Now(),
	}

	for _, step := range spec.Spec.Steps {
		run.Steps[step.Name] = &StepRun{
			Name:   step.Name,
			Status: "Pending",
		}
	}

	e.logger.Infow("workflow started",
		"workflow", spec.Metadata.Name,
		"runID", run.ID,
		"layers", len(layers),
	)

	// outputs collects step name -> output text for context substitution.
	outputs := make(map[string]string)

	for layerIdx, layer := range layers {
		e.logger.Debugw("executing layer",
			"workflow", spec.Metadata.Name,
			"layer", layerIdx,
			"steps", stepNames(layer),
		)

		if err := e.executeLayer(ctx, layer, outputs, run); err != nil {
			run.Status = WorkflowStatusFailed
			endTime := time.Now()
			run.EndedAt = &endTime

			// Mark remaining pending steps as Skipped.
			for _, sr := range run.Steps {
				if sr.Status == "Pending" {
					sr.Status = "Skipped"
				}
			}

			e.logger.Errorw("workflow failed",
				"workflow", spec.Metadata.Name,
				"runID", run.ID,
				"layer", layerIdx,
				"error", err,
			)
			return run, err
		}
	}

	run.Status = WorkflowStatusCompleted
	endTime := time.Now()
	run.EndedAt = &endTime

	e.logger.Infow("workflow completed",
		"workflow", spec.Metadata.Name,
		"runID", run.ID,
		"duration", run.EndedAt.Sub(run.StartedAt),
	)

	return run, nil
}

// executeLayer runs all steps in a single DAG layer concurrently.
func (e *Engine) executeLayer(
	ctx context.Context,
	layer []WorkflowStep,
	outputs map[string]string,
	run *WorkflowRun,
) error {
	if len(layer) == 1 {
		// Single step: execute directly without goroutine overhead.
		sr, err := e.executeStep(ctx, layer[0], outputs)
		run.Steps[layer[0].Name] = sr
		if err != nil {
			return fmt.Errorf("step %q failed: %w", layer[0].Name, err)
		}
		outputs[layer[0].Name] = sr.Output
		return nil
	}

	var (
		mu      sync.Mutex
		wg      sync.WaitGroup
		firstErr error
	)

	for _, step := range layer {
		wg.Add(1)
		go func(s WorkflowStep) {
			defer wg.Done()

			// Take a snapshot of outputs for safe concurrent reads.
			mu.Lock()
			snapshot := make(map[string]string, len(outputs))
			for k, v := range outputs {
				snapshot[k] = v
			}
			mu.Unlock()

			sr, err := e.executeStep(ctx, s, snapshot)

			mu.Lock()
			defer mu.Unlock()

			run.Steps[s.Name] = sr
			if err != nil {
				if firstErr == nil {
					firstErr = fmt.Errorf("step %q failed: %w", s.Name, err)
				}
				return
			}
			outputs[s.Name] = sr.Output
		}(step)
	}

	wg.Wait()
	return firstErr
}

// executeStep runs a single workflow step by delegating to the controller.
func (e *Engine) executeStep(
	ctx context.Context,
	step WorkflowStep,
	outputs map[string]string,
) (*StepRun, error) {
	sr := &StepRun{
		Name:   step.Name,
		Status: "Running",
	}
	now := time.Now()
	sr.StartedAt = &now

	e.logger.Infow("step started", "step", step.Name, "agent", step.Agent)

	// Build the message with context substitution.
	message := e.resolveContext(step.Input, outputs)

	// Create a task record for this step.
	taskID := generateTaskID(step.Name)
	task := v1.TaskRecord{
		ID:        taskID,
		AgentName: step.Agent,
		Message:   message,
		Status:    v1.TaskStatusPending,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	sr.TaskID = taskID

	// Persist the task.
	if err := e.store.CreateTask(ctx, task); err != nil {
		sr.Status = "Failed"
		sr.Error = fmt.Sprintf("create task: %v", err)
		endTime := time.Now()
		sr.EndedAt = &endTime
		return sr, fmt.Errorf("create task for step %q: %w", step.Name, err)
	}

	// Execute via controller.
	result, err := e.controller.ExecuteTask(ctx, task)
	endTime := time.Now()
	sr.EndedAt = &endTime

	if err != nil {
		sr.Status = "Failed"
		sr.Error = err.Error()
		e.logger.Errorw("step failed", "step", step.Name, "error", err)
		return sr, err
	}

	sr.Status = "Completed"
	sr.Output = result.Output

	e.logger.Infow("step completed",
		"step", step.Name,
		"outputLen", len(result.Output),
	)

	return sr, nil
}

// resolveContext substitutes ${{ steps.<step>.outputs.<name> }} placeholders
// in the input message and context strings with actual output values.
func (e *Engine) resolveContext(input WorkflowInput, outputs map[string]string) string {
	var parts []string
	parts = append(parts, substituteVars(input.Message, outputs))

	for _, c := range input.Context {
		resolved := substituteVars(c, outputs)
		if resolved != "" {
			parts = append(parts, resolved)
		}
	}

	return strings.Join(parts, "\n\n")
}

// substituteVars replaces all ${{ steps.<step>.outputs.<name> }} expressions
// in s with the corresponding output value from the outputs map.
func substituteVars(s string, outputs map[string]string) string {
	return contextVarPattern.ReplaceAllStringFunc(s, func(match string) string {
		submatch := contextVarPattern.FindStringSubmatch(match)
		if len(submatch) < 3 {
			return match
		}
		stepName := submatch[1]
		// For now, each step produces a single output string regardless of
		// output name. The output name is preserved for future structured
		// output support.
		if val, ok := outputs[stepName]; ok {
			return val
		}
		return match // leave unresolved if step output not found
	})
}

// validateDAG checks that all dependencies exist and there are no cycles.
func (e *Engine) validateDAG(steps []WorkflowStep) error {
	stepSet := make(map[string]struct{}, len(steps))
	for _, s := range steps {
		if s.Name == "" {
			return fmt.Errorf("step name is required")
		}
		if s.Agent == "" {
			return fmt.Errorf("step %q: agent is required", s.Name)
		}
		if _, dup := stepSet[s.Name]; dup {
			return fmt.Errorf("duplicate step name: %q", s.Name)
		}
		stepSet[s.Name] = struct{}{}
	}

	// Check that all dependencies reference existing steps.
	for _, s := range steps {
		for _, dep := range s.DependsOn {
			if _, ok := stepSet[dep]; !ok {
				return fmt.Errorf("step %q depends on unknown step %q", s.Name, dep)
			}
			if dep == s.Name {
				return fmt.Errorf("step %q depends on itself", s.Name)
			}
		}
	}

	// Detect cycles using Kahn's algorithm (topological sort).
	inDegree := make(map[string]int, len(steps))
	adjacency := make(map[string][]string, len(steps))
	for _, s := range steps {
		if _, ok := inDegree[s.Name]; !ok {
			inDegree[s.Name] = 0
		}
		for _, dep := range s.DependsOn {
			adjacency[dep] = append(adjacency[dep], s.Name)
			inDegree[s.Name]++
		}
	}

	queue := make([]string, 0)
	for name, deg := range inDegree {
		if deg == 0 {
			queue = append(queue, name)
		}
	}

	visited := 0
	for len(queue) > 0 {
		node := queue[0]
		queue = queue[1:]
		visited++
		for _, next := range adjacency[node] {
			inDegree[next]--
			if inDegree[next] == 0 {
				queue = append(queue, next)
			}
		}
	}

	if visited != len(steps) {
		return fmt.Errorf("workflow contains a dependency cycle")
	}

	return nil
}

// buildDAG performs a topological sort on the workflow steps and groups them
// into layers. Steps within the same layer have no interdependencies and can
// be executed in parallel.
func (e *Engine) buildDAG(steps []WorkflowStep) ([][]WorkflowStep, error) {
	stepMap := make(map[string]WorkflowStep, len(steps))
	inDegree := make(map[string]int, len(steps))
	adjacency := make(map[string][]string, len(steps))

	for _, s := range steps {
		stepMap[s.Name] = s
		if _, ok := inDegree[s.Name]; !ok {
			inDegree[s.Name] = 0
		}
		for _, dep := range s.DependsOn {
			adjacency[dep] = append(adjacency[dep], s.Name)
			inDegree[s.Name]++
		}
	}

	var layers [][]WorkflowStep

	for {
		// Collect all nodes with in-degree 0 (current layer).
		var currentLayer []WorkflowStep
		for name, deg := range inDegree {
			if deg == 0 {
				currentLayer = append(currentLayer, stepMap[name])
			}
		}

		if len(currentLayer) == 0 {
			break
		}

		// Remove processed nodes and update in-degrees.
		for _, s := range currentLayer {
			delete(inDegree, s.Name)
			for _, next := range adjacency[s.Name] {
				inDegree[next]--
			}
		}

		layers = append(layers, currentLayer)
	}

	if len(inDegree) > 0 {
		return nil, fmt.Errorf("cycle detected in workflow DAG")
	}

	return layers, nil
}

// --- helpers ---

// generateRunID creates a unique workflow run ID.
func generateRunID(workflowName string) string {
	return fmt.Sprintf("wfr-%s-%d", workflowName, time.Now().UnixNano())
}

// generateTaskID creates a unique task ID for a workflow step.
func generateTaskID(stepName string) string {
	return fmt.Sprintf("wft-%s-%d", stepName, time.Now().UnixNano())
}

// stepNames extracts step names from a slice of WorkflowStep.
func stepNames(steps []WorkflowStep) []string {
	names := make([]string, len(steps))
	for i, s := range steps {
		names[i] = s.Name
	}
	return names
}
