package claudecode

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"sync"
	"time"

	"go.uber.org/zap"
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
	logger  *zap.SugaredLogger

	// activeCmd tracks the currently running process (for Stream or long Execute).
	activeCmd *exec.Cmd
}

// New creates a new Claude Code adapter.
func New() adapter.Adapter {
	l, _ := zap.NewProduction()
	return &Adapter{
		phase:  v1.AgentPhaseCreated,
		logger: l.Sugar().Named("claudecode"),
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

	// Ensure workdir exists.
	workdir := spec.Spec.Context.Workdir
	if workdir == "" {
		workdir = "/tmp/opc"
	}
	os.MkdirAll(workdir, 0o755)

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

// claudeModelMap maps short model names to Claude CLI-compatible model IDs.
var claudeModelMap = map[string]string{
	"claude-sonnet-4":   "claude-sonnet-4-20250514",
	"claude-opus-4":     "claude-opus-4-20250514",
	"claude-haiku-4":    "claude-haiku-4-5-20251001",
	"claude-haiku-4.5":  "claude-haiku-4-5-20251001",
}

// buildBaseArgs constructs the common CLI arguments from the agent spec.
func (a *Adapter) buildBaseArgs() []string {
	var args []string

	if a.spec.Spec.Runtime.Model.Name != "" {
		modelName := a.spec.Spec.Runtime.Model.Name
		// Map short names to full model IDs that Claude CLI accepts.
		if mapped, ok := claudeModelMap[modelName]; ok {
			modelName = mapped
		}
		args = append(args, "--model", modelName)
	}

	if a.spec.Spec.Runtime.Inference.MaxTokens > 0 {
		args = append(args, "--max-turns", fmt.Sprintf("%d", a.spec.Spec.Runtime.Inference.MaxTokens))
	}

	return args
}

// claudeCodeResult represents the JSON output from `claude --print --output-format json`.
type claudeCodeResult struct {
	Type         string  `json:"type"`
	Content      string  `json:"content,omitempty"`
	Result       string  `json:"result,omitempty"`
	TokensIn     int     `json:"input_tokens,omitempty"`
	TokensOut    int     `json:"output_tokens,omitempty"`
	Error        string  `json:"error,omitempty"`
	IsError      bool    `json:"is_error,omitempty"`
	TotalCostUSD float64 `json:"total_cost_usd,omitempty"`

	// Usage contains detailed token usage from Claude Code CLI.
	Usage      *claudeCodeUsage      `json:"usage,omitempty"`
	ModelUsage map[string]modelUsage `json:"modelUsage,omitempty"`
}

// claudeCodeUsage contains token usage info from Claude Code output.
type claudeCodeUsage struct {
	InputTokens              int `json:"input_tokens"`
	OutputTokens             int `json:"output_tokens"`
	CacheCreationInputTokens int `json:"cache_creation_input_tokens,omitempty"`
	CacheReadInputTokens     int `json:"cache_read_input_tokens,omitempty"`
}

// modelUsage contains per-model usage from Claude Code CLI.
type modelUsage struct {
	InputTokens              int     `json:"inputTokens"`
	OutputTokens             int     `json:"outputTokens"`
	CacheReadInputTokens     int     `json:"cacheReadInputTokens,omitempty"`
	CacheCreationInputTokens int     `json:"cacheCreationInputTokens,omitempty"`
	CostUSD                  float64 `json:"costUSD,omitempty"`
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
	execStart := time.Now()
	a.logger.Infow("Execute", "taskId", task.ID, "agentName", a.spec.Metadata.Name)
	a.mu.RLock()
	if a.phase != v1.AgentPhaseRunning {
		a.mu.RUnlock()
		a.logger.Warnw("Execute: agent not running", "taskId", task.ID, "phase", a.phase)
		return adapter.ExecuteResult{}, fmt.Errorf("agent not running (phase: %s)", a.phase)
	}
	a.mu.RUnlock()

	// Build command: claude --print --permission-mode acceptEdits -p "<message>"
	args := []string{"--print", "--permission-mode", "acceptEdits"}
	args = append(args, a.buildBaseArgs()...)
	args = append(args, "--output-format", "json")
	args = append(args, "-p", task.Message)

	cmd := exec.CommandContext(ctx, "claude", args...)
	if a.spec.Spec.Context.Workdir != "" {
		cmd.Dir = a.spec.Spec.Context.Workdir
	} else {
		// Default workdir — ensure it exists.
		cmd.Dir = "/tmp/opc"
		os.MkdirAll(cmd.Dir, 0o755)
	}

	var stderr bytes.Buffer
	cmd.Stderr = &stderr

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
		errMsg := stderr.String()
		if errMsg == "" {
			errMsg = err.Error()
		}
		a.logger.Errorw("Execute: claude process failed", "taskId", task.ID, "error", errMsg, "duration", time.Since(execStart))
		return adapter.ExecuteResult{}, fmt.Errorf("claude execute: %s", errMsg)
	}

	// Try to parse JSON output for token usage.
	var result adapter.ExecuteResult
	var parsed claudeCodeResult
	if jsonErr := json.Unmarshal(output, &parsed); jsonErr == nil {
		if parsed.IsError || parsed.Error != "" {
			a.mu.Lock()
			a.metrics.TasksFailed++
			a.mu.Unlock()
			errMsg := parsed.Error
			if errMsg == "" {
				errMsg = parsed.Result
			}
			return adapter.ExecuteResult{}, fmt.Errorf("claude error: %s", errMsg)
		}

		result.Output = parsed.Result
		if result.Output == "" {
			result.Output = parsed.Content
		}

		// Use total_cost_usd reported by Claude CLI (most accurate).
		result.Cost = parsed.TotalCostUSD

		// Token usage: prefer usage field, then modelUsage aggregate.
		if parsed.Usage != nil {
			result.TokensIn = parsed.Usage.InputTokens + parsed.Usage.CacheCreationInputTokens + parsed.Usage.CacheReadInputTokens
			result.TokensOut = parsed.Usage.OutputTokens
		} else if len(parsed.ModelUsage) > 0 {
			for _, mu := range parsed.ModelUsage {
				result.TokensIn += mu.InputTokens + mu.CacheReadInputTokens + mu.CacheCreationInputTokens
				result.TokensOut += mu.OutputTokens
			}
		} else {
			result.TokensIn = parsed.TokensIn
			result.TokensOut = parsed.TokensOut
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

	a.logger.Infow("Execute completed", "taskId", task.ID,
		"tokensIn", result.TokensIn, "tokensOut", result.TokensOut,
		"cost", result.Cost, "duration", time.Since(execStart))
	return result, nil
}

func (a *Adapter) Stream(ctx context.Context, task v1.TaskRecord) (<-chan adapter.Chunk, error) {
	a.mu.RLock()
	if a.phase != v1.AgentPhaseRunning {
		a.mu.RUnlock()
		return nil, fmt.Errorf("agent not running (phase: %s)", a.phase)
	}
	a.mu.RUnlock()

	// Build command: claude --permission-mode acceptEdits --output-format stream-json -p "<message>"
	args := []string{"--permission-mode", "acceptEdits", "--output-format", "stream-json"}
	args = append(args, a.buildBaseArgs()...)
	args = append(args, "-p", task.Message)

	cmd := exec.CommandContext(ctx, "claude", args...)
	if a.spec.Spec.Context.Workdir != "" {
		cmd.Dir = a.spec.Spec.Context.Workdir
	} else {
		cmd.Dir = os.TempDir()
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
