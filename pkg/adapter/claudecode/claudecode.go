package claudecode

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"sync"
	"time"

	v1 "github.com/zlc-ai/opc-platform/api/v1"
	"github.com/zlc-ai/opc-platform/pkg/adapter"
)

// Adapter implements adapter.Adapter for Claude Code CLI agents.
// Unlike OpenClaw, Claude Code operates in a non-persistent mode:
// each Execute call spawns a new `claude --print` process,
// and each Stream call spawns a `claude --output-format stream-json` process.
type Adapter struct {
	mu      sync.RWMutex
	phase   v1.AgentPhase
	metrics v1.AgentMetrics
	spec    v1.AgentSpec
	startAt time.Time

	// activeCmd tracks the currently running process (for Stream or long Execute).
	activeCmd *exec.Cmd
}

// New creates a new Claude Code adapter.
func New() adapter.Adapter {
	return &Adapter{
		phase: v1.AgentPhaseCreated,
	}
}

func (a *Adapter) Type() v1.AgentType {
	return v1.AgentTypeClaudeCode
}

// Start stores the agent spec and marks the adapter as Running.
// Claude Code --print mode does not require a persistent process;
// processes are spawned per-task in Execute and Stream.
func (a *Adapter) Start(_ context.Context, spec v1.AgentSpec) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	a.spec = spec
	a.phase = v1.AgentPhaseRunning
	a.startAt = time.Now()

	return nil
}

func (a *Adapter) Stop(ctx context.Context) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	// Kill any active process.
	if a.activeCmd != nil && a.activeCmd.Process != nil {
		done := make(chan error, 1)
		go func() { done <- a.activeCmd.Wait() }()

		select {
		case <-ctx.Done():
			a.activeCmd.Process.Kill()
			a.phase = v1.AgentPhaseTerminated
			return ctx.Err()
		case <-done:
			// Process already exited.
		}
	}

	a.activeCmd = nil
	a.phase = v1.AgentPhaseStopped
	return nil
}

func (a *Adapter) Health() v1.HealthStatus {
	a.mu.RLock()
	defer a.mu.RUnlock()

	if a.phase == v1.AgentPhaseRunning {
		return v1.HealthStatus{Healthy: true, Message: "ready"}
	}

	return v1.HealthStatus{Healthy: false, Message: fmt.Sprintf("not running (phase: %s)", a.phase)}
}

// buildBaseArgs constructs the common CLI arguments from the agent spec.
func (a *Adapter) buildBaseArgs() []string {
	var args []string

	if a.spec.Spec.Runtime.Model.Name != "" {
		args = append(args, "--model", a.spec.Spec.Runtime.Model.Name)
	}

	if a.spec.Spec.Runtime.Inference.MaxTokens > 0 {
		args = append(args, "--max-turns", fmt.Sprintf("%d", a.spec.Spec.Runtime.Inference.MaxTokens))
	}

	return args
}

// claudeCodeResult represents the JSON output from `claude --print --output-format json`.
type claudeCodeResult struct {
	Type      string `json:"type"`
	Content   string `json:"content,omitempty"`
	Result    string `json:"result,omitempty"`
	TokensIn  int    `json:"input_tokens,omitempty"`
	TokensOut int    `json:"output_tokens,omitempty"`
	Error     string `json:"error,omitempty"`

	// Usage is an alternative field structure Claude Code may use.
	Usage *claudeCodeUsage `json:"usage,omitempty"`
}

// claudeCodeUsage contains token usage info from Claude Code output.
type claudeCodeUsage struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
}

// claudeCodeStreamEvent represents a single JSONL event from stream-json output.
type claudeCodeStreamEvent struct {
	Type    string `json:"type"`
	Content string `json:"content,omitempty"`
	Result  string `json:"result,omitempty"`
	Error   string `json:"error,omitempty"`

	// Usage may appear in the final "result" event.
	Usage *claudeCodeUsage `json:"usage,omitempty"`
}

func (a *Adapter) Execute(ctx context.Context, task v1.TaskRecord) (adapter.ExecuteResult, error) {
	a.mu.RLock()
	if a.phase != v1.AgentPhaseRunning {
		a.mu.RUnlock()
		return adapter.ExecuteResult{}, fmt.Errorf("agent not running (phase: %s)", a.phase)
	}
	a.mu.RUnlock()

	// Build command: claude --print -p "<message>"
	args := []string{"--print"}
	args = append(args, a.buildBaseArgs()...)
	args = append(args, "--output-format", "json")
	args = append(args, "-p", task.Message)

	cmd := exec.CommandContext(ctx, "claude", args...)
	if a.spec.Spec.Context.Workdir != "" {
		cmd.Dir = a.spec.Spec.Context.Workdir
	}

	a.mu.Lock()
	a.activeCmd = cmd
	a.mu.Unlock()

	output, err := cmd.Output()

	a.mu.Lock()
	a.activeCmd = nil
	a.mu.Unlock()

	if err != nil {
		a.mu.Lock()
		a.metrics.TasksFailed++
		a.mu.Unlock()
		return adapter.ExecuteResult{}, fmt.Errorf("claude execute: %w", err)
	}

	// Try to parse JSON output for token usage.
	var result adapter.ExecuteResult
	var parsed claudeCodeResult
	if jsonErr := json.Unmarshal(output, &parsed); jsonErr == nil {
		if parsed.Error != "" {
			a.mu.Lock()
			a.metrics.TasksFailed++
			a.mu.Unlock()
			return adapter.ExecuteResult{}, fmt.Errorf("claude error: %s", parsed.Error)
		}

		result.Output = parsed.Result
		if result.Output == "" {
			result.Output = parsed.Content
		}
		result.TokensIn = parsed.TokensIn
		result.TokensOut = parsed.TokensOut
		if parsed.Usage != nil {
			result.TokensIn = parsed.Usage.InputTokens
			result.TokensOut = parsed.Usage.OutputTokens
		}
	} else {
		// Fallback: treat raw output as plain text.
		result.Output = string(output)
	}

	a.mu.Lock()
	a.metrics.TasksCompleted++
	a.metrics.TotalTokensIn += result.TokensIn
	a.metrics.TotalTokensOut += result.TokensOut
	a.mu.Unlock()

	return result, nil
}

func (a *Adapter) Stream(ctx context.Context, task v1.TaskRecord) (<-chan adapter.Chunk, error) {
	a.mu.RLock()
	if a.phase != v1.AgentPhaseRunning {
		a.mu.RUnlock()
		return nil, fmt.Errorf("agent not running (phase: %s)", a.phase)
	}
	a.mu.RUnlock()

	// Build command: claude --output-format stream-json -p "<message>"
	args := []string{"--output-format", "stream-json"}
	args = append(args, a.buildBaseArgs()...)

	args = append(args, "-p", task.Message)

	cmd := exec.CommandContext(ctx, "claude", args...)
	if a.spec.Spec.Context.Workdir != "" {
		cmd.Dir = a.spec.Spec.Context.Workdir
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("claude stdout pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("claude stream start: %w", err)
	}

	a.mu.Lock()
	a.activeCmd = cmd
	a.mu.Unlock()

	ch := make(chan adapter.Chunk, 64)
	go func() {
		defer close(ch)
		defer func() {
			a.mu.Lock()
			a.activeCmd = nil
			a.mu.Unlock()
		}()

		scanner := bufio.NewScanner(stdout)
		var totalIn, totalOut int

		for scanner.Scan() {
			line := scanner.Bytes()
			if len(line) == 0 {
				continue
			}

			var event claudeCodeStreamEvent
			if jsonErr := json.Unmarshal(line, &event); jsonErr != nil {
				continue
			}

			if event.Error != "" {
				a.mu.Lock()
				a.metrics.TasksFailed++
				a.mu.Unlock()
				ch <- adapter.Chunk{Error: fmt.Errorf("claude: %s", event.Error)}
				return
			}

			if event.Usage != nil {
				totalIn = event.Usage.InputTokens
				totalOut = event.Usage.OutputTokens
			}

			switch event.Type {
			case "assistant":
				ch <- adapter.Chunk{Content: event.Content, Done: false}
			case "result":
				content := event.Result
				if content == "" {
					content = event.Content
				}
				ch <- adapter.Chunk{Content: content, Done: true}

				a.mu.Lock()
				a.metrics.TasksCompleted++
				a.metrics.TotalTokensIn += totalIn
				a.metrics.TotalTokensOut += totalOut
				a.mu.Unlock()
				return
			}
		}

		if scanErr := scanner.Err(); scanErr != nil {
			ch <- adapter.Chunk{Error: scanErr}
		}

		// Wait for process to exit.
		_ = cmd.Wait()
	}()

	return ch, nil
}

func (a *Adapter) Status() v1.AgentPhase {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.phase
}

func (a *Adapter) Metrics() v1.AgentMetrics {
	a.mu.RLock()
	defer a.mu.RUnlock()
	m := a.metrics
	if !a.startAt.IsZero() {
		m.UptimeSeconds = time.Since(a.startAt).Seconds()
	}
	return m
}
