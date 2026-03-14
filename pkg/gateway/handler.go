package gateway

import (
	"context"
	"fmt"
	"strings"
	"time"

	v1 "github.com/zlc-ai/opc-platform/api/v1"
	"github.com/zlc-ai/opc-platform/pkg/controller"
	"go.uber.org/zap"
)

// CommandHandler routes incoming gateway messages to OPC controller actions.
type CommandHandler struct {
	ctrl   *controller.Controller
	logger *zap.SugaredLogger
}

// NewCommandHandler creates a new command handler.
func NewCommandHandler(ctrl *controller.Controller, logger *zap.SugaredLogger) *CommandHandler {
	return &CommandHandler{ctrl: ctrl, logger: logger}
}

// Handle processes an incoming message and returns a response.
// Supported commands: /run, /status, /agents, /help
func (h *CommandHandler) Handle(ctx context.Context, msg *Message) (*Response, error) {
	text := strings.TrimSpace(msg.Text)

	// Parse command and args.
	parts := strings.SplitN(text, " ", 2)
	cmd := strings.ToLower(parts[0])
	args := ""
	if len(parts) > 1 {
		args = strings.TrimSpace(parts[1])
	}

	// Strip leading slash if present.
	cmd = strings.TrimPrefix(cmd, "/")

	switch cmd {
	case "run":
		return h.handleRun(ctx, args)
	case "status":
		return h.handleStatus(ctx)
	case "agents":
		return h.handleAgents(ctx)
	case "help":
		return h.handleHelp()
	default:
		return h.handleHelp()
	}
}

// handleRun executes a task: /run <agent> <message>
func (h *CommandHandler) handleRun(ctx context.Context, args string) (*Response, error) {
	if args == "" {
		return &Response{
			Text: "Usage: /run <agent> <message>\nExample: /run coder-main Fix the login bug",
		}, nil
	}

	parts := strings.SplitN(args, " ", 2)
	agentName := parts[0]
	message := ""
	if len(parts) > 1 {
		message = parts[1]
	}

	if message == "" {
		return &Response{
			Text: "Usage: /run <agent> <message>\nPlease provide a task message.",
		}, nil
	}

	taskID := fmt.Sprintf("task-%d", time.Now().UnixNano()/1e6)
	task := v1.TaskRecord{
		ID:        taskID,
		AgentName: agentName,
		Message:   message,
		Status:    v1.TaskStatusPending,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	if err := h.ctrl.Store().CreateTask(ctx, task); err != nil {
		return nil, fmt.Errorf("create task: %w", err)
	}

	result, err := h.ctrl.ExecuteTask(ctx, task)
	if err != nil {
		return &Response{
			Text: fmt.Sprintf("Task %s submitted to %s but failed:\n%v", taskID, agentName, err),
		}, nil
	}

	output := result.Output
	if len(output) > 1500 {
		output = output[:1500] + "\n... (truncated)"
	}

	return &Response{
		Text: fmt.Sprintf("Task %s completed\nAgent: %s\nTokens: %d in / %d out\n\n%s",
			taskID, agentName, result.TokensIn, result.TokensOut, output),
	}, nil
}

// handleStatus returns cluster status.
func (h *CommandHandler) handleStatus(ctx context.Context) (*Response, error) {
	agents, _ := h.ctrl.ListAgents(ctx)
	tasks, _ := h.ctrl.Store().ListTasks(ctx)

	var running, stopped, failed int
	for _, a := range agents {
		switch a.Phase {
		case v1.AgentPhaseRunning:
			running++
		case v1.AgentPhaseStopped:
			stopped++
		case v1.AgentPhaseFailed:
			failed++
		}
	}

	var pending, taskRunning, completed, taskFailed int
	for _, t := range tasks {
		switch t.Status {
		case v1.TaskStatusPending:
			pending++
		case v1.TaskStatusRunning:
			taskRunning++
		case v1.TaskStatusCompleted:
			completed++
		case v1.TaskStatusFailed:
			taskFailed++
		}
	}

	text := fmt.Sprintf(
		"OPC Platform Status\n\n"+
			"Agents: %d total (%d running, %d stopped, %d failed)\n"+
			"Tasks: %d total (%d pending, %d running, %d completed, %d failed)",
		len(agents), running, stopped, failed,
		len(tasks), pending, taskRunning, completed, taskFailed,
	)

	return &Response{Text: text}, nil
}

// handleAgents lists all agents and their status.
func (h *CommandHandler) handleAgents(ctx context.Context) (*Response, error) {
	agents, err := h.ctrl.ListAgents(ctx)
	if err != nil {
		return nil, fmt.Errorf("list agents: %w", err)
	}

	if len(agents) == 0 {
		return &Response{Text: "No agents configured."}, nil
	}

	var sb strings.Builder
	sb.WriteString("Agents:\n\n")
	for _, a := range agents {
		icon := "⚪"
		switch a.Phase {
		case v1.AgentPhaseRunning:
			icon = "🟢"
		case v1.AgentPhaseStopped:
			icon = "🔴"
		case v1.AgentPhaseFailed:
			icon = "❌"
		case v1.AgentPhaseStarting:
			icon = "🟡"
		}
		sb.WriteString(fmt.Sprintf("%s %s (%s) - %s\n", icon, a.Name, a.Type, a.Phase))
	}

	return &Response{Text: sb.String()}, nil
}

// handleHelp returns available commands.
func (h *CommandHandler) handleHelp() (*Response, error) {
	text := "OPC Platform Commands:\n\n" +
		"/run <agent> <message> - Execute a task\n" +
		"/status - Show cluster status\n" +
		"/agents - List all agents\n" +
		"/help - Show this help message"

	return &Response{Text: text}, nil
}
