package goal

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	v1 "github.com/zlc-ai/opc-platform/api/v1"
	"go.uber.org/zap"
)

const (
	assessorAgentName = "opc-goal-driver"
	maxRoundsDefault  = 3
)

// AssessmentResult is the LLM's judgment on whether a task result satisfies the requirement.
type AssessmentResult struct {
	Satisfied bool   `json:"satisfied"`
	Reason    string `json:"reason"`
	FollowUp  string `json:"followUp,omitempty"` // instruction to send if not satisfied
}

// GoalDriver assesses task results and generates follow-up instructions
// to autonomously push a federated goal to completion.
type GoalDriver struct {
	controller AgentController
	logger     *zap.SugaredLogger
}

// NewGoalDriver creates a new GoalDriver.
func NewGoalDriver(ctrl AgentController, logger *zap.SugaredLogger) *GoalDriver {
	return &GoalDriver{
		controller: ctrl,
		logger:     logger,
	}
}

// AssessResult checks if a task result satisfies the project requirement.
// Returns an assessment with follow-up instructions if not satisfied.
func (gd *GoalDriver) AssessResult(ctx context.Context, goalName, projectDesc, result string) (*AssessmentResult, error) {
	if result == "" {
		return &AssessmentResult{
			Satisfied: false,
			Reason:    "result is empty — agent produced no output",
			FollowUp:  fmt.Sprintf("上次执行没有产出任何结果。请直接完成以下任务，不要等待用户确认或交互：\n\n%s", projectDesc),
		}, nil
	}

	// Quick heuristic checks before calling LLM.
	if isInteractivePrompt(result) {
		return &AssessmentResult{
			Satisfied: false,
			Reason:    "result contains interactive prompt — agent is waiting for user input instead of completing the task",
			FollowUp:  fmt.Sprintf("上次执行中你在等待用户交互输入。请不要等待用户确认，直接自主完成任务。\n\n任务要求：%s\n\n上次输出（供参考）：%s", projectDesc, truncateStr(result, 500)),
		}, nil
	}

	// Use LLM to assess result quality.
	assessment, err := gd.callAssessment(ctx, goalName, projectDesc, result)
	if err != nil {
		gd.logger.Warnw("LLM assessment failed, falling back to accept",
			"error", err)
		// Fallback: accept the result if LLM assessment fails.
		return &AssessmentResult{Satisfied: true, Reason: "LLM assessment unavailable, accepted by default"}, nil
	}

	return assessment, nil
}

// callAssessment uses LLM to judge result quality.
func (gd *GoalDriver) callAssessment(ctx context.Context, goalName, projectDesc, result string) (*AssessmentResult, error) {
	start := time.Now()

	if err := gd.ensureAgent(ctx); err != nil {
		return nil, fmt.Errorf("ensure goal driver agent: %w", err)
	}

	prompt := buildAssessmentPrompt(goalName, projectDesc, result)

	taskID := fmt.Sprintf("assess-%d", time.Now().UnixMilli())
	task := v1.TaskRecord{
		ID:        taskID,
		AgentName: assessorAgentName,
		Message:   prompt,
		Status:    v1.TaskStatusPending,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	execResult, err := gd.controller.ExecuteTask(ctx, task)
	if err != nil {
		return nil, fmt.Errorf("execute assessment: %w", err)
	}

	gd.logger.Debugw("assessment LLM response",
		"responseLen", len(execResult.Output),
		"duration", time.Since(start),
	)

	// Parse JSON from response.
	output := extractJSON(execResult.Output)
	var assessment AssessmentResult
	if err := json.Unmarshal([]byte(output), &assessment); err != nil {
		// Try to be lenient — if LLM didn't return clean JSON.
		gd.logger.Warnw("failed to parse assessment JSON, accepting result",
			"error", err,
			"rawLen", len(execResult.Output),
		)
		return &AssessmentResult{Satisfied: true, Reason: "could not parse assessment, accepted by default"}, nil
	}

	return &assessment, nil
}

func (gd *GoalDriver) ensureAgent(ctx context.Context) error {
	if _, err := gd.controller.GetAgent(ctx, assessorAgentName); err == nil {
		return nil
	}

	spec := v1.AgentSpec{
		APIVersion: v1.APIVersion,
		Kind:       v1.KindAgentSpec,
		Metadata:   v1.Metadata{Name: assessorAgentName},
		Spec: v1.AgentSpecBody{
			Type: v1.AgentTypeClaudeCode,
			Runtime: v1.RuntimeConfig{
				Model:   v1.ModelConfig{Name: "claude-sonnet-4"},
				Timeout: v1.TimeoutConfig{Task: "2m"},
			},
			Context: v1.ContextConfig{Workdir: "/tmp/opc/goal-driver"},
		},
	}

	if err := gd.controller.Apply(ctx, spec); err != nil {
		return fmt.Errorf("apply goal driver agent: %w", err)
	}
	if err := gd.controller.StartAgent(ctx, assessorAgentName); err != nil {
		return fmt.Errorf("start goal driver agent: %w", err)
	}

	gd.logger.Infow("created goal driver agent", "name", assessorAgentName)
	return nil
}

func buildAssessmentPrompt(goalName, projectDesc, result string) string {
	return fmt.Sprintf(`You are a Goal Driver for the OPC Platform. Your job is to assess whether an agent's output satisfies the task requirement.

## Goal
%s

## Task Requirement
%s

## Agent Output
%s

## Instructions
Assess whether the agent output satisfies the task requirement. Consider:
1. Did the agent produce actual deliverables (not just ask questions or propose to do something)?
2. Is the output substantive content (not an interactive prompt waiting for user input)?
3. Does the output address the core requirement?

Respond with ONLY this JSON (no markdown, no explanation):
{"satisfied": true/false, "reason": "brief explanation", "followUp": "if not satisfied, the instruction to send to the agent to continue/complete the task"}

If the agent output is interactive prompts, questions, or proposals instead of actual work, set satisfied=false and provide a followUp that tells the agent to directly complete the work without waiting for confirmation.`,
		goalName,
		projectDesc,
		truncateStr(result, 2000),
	)
}

// isInteractivePrompt detects common patterns of agents waiting for input.
func isInteractivePrompt(result string) bool {
	lower := strings.ToLower(result)
	patterns := []string{
		"want to try it?",
		"would you like",
		"shall i",
		"should i",
		"do you want",
		"need your confirmation",
		"waiting for",
		"please confirm",
		"需要你的确认",
		"需要进一步",
		"是否继续",
		"你想试试",
		"requires opening a local url",
	}
	for _, p := range patterns {
		if strings.Contains(lower, p) {
			return true
		}
	}
	return false
}

func truncateStr(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
