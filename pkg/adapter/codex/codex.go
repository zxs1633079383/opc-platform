package codex

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"sync"
	"time"

	v1 "github.com/zlc-ai/opc-platform/api/v1"
	"github.com/zlc-ai/opc-platform/pkg/adapter"
)

// Adapter implements adapter.Adapter for OpenAI Codex CLI agents.
// Unlike the long-running OpenClaw adapter, each Execute/Stream call
// spawns a new `codex` process using quiet mode (`-q`).
type Adapter struct {
	mu       sync.RWMutex
	phase    v1.AgentPhase
	metrics  v1.AgentMetrics
	spec     v1.AgentSpec
	startAt  time.Time
	cancelFn context.CancelFunc // cancels active process
}

// New creates a new Codex CLI adapter.
func New() adapter.Adapter {
	return &Adapter{
		phase: v1.AgentPhaseCreated,
	}
}

func (a *Adapter) Type() v1.AgentType {
	return v1.AgentTypeCodex
}

func (a *Adapter) Start(_ context.Context, spec v1.AgentSpec) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	a.spec = spec
	a.phase = v1.AgentPhaseRunning
	a.startAt = time.Now()

	return nil
}

func (a *Adapter) Stop(_ context.Context) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	// Cancel any running process.
	if a.cancelFn != nil {
		a.cancelFn()
		a.cancelFn = nil
	}

	a.phase = v1.AgentPhaseStopped
	return nil
}

func (a *Adapter) Health() v1.HealthStatus {
	a.mu.RLock()
	defer a.mu.RUnlock()

	if a.phase == v1.AgentPhaseRunning {
		return v1.HealthStatus{Healthy: true, Message: "running"}
	}
	return v1.HealthStatus{Healthy: false, Message: fmt.Sprintf("not running (phase: %s)", a.phase)}
}

// buildArgs constructs the codex CLI arguments from the stored spec and task message.
func (a *Adapter) buildArgs(message string) []string {
	args := []string{"-q", "--approval-mode", "full-auto"}

	if a.spec.Spec.Runtime.Model.Name != "" {
		args = append(args, "--model", a.spec.Spec.Runtime.Model.Name)
	}

	args = append(args, message)
	return args
}

// buildCmd creates an exec.Cmd for a codex invocation.
func (a *Adapter) buildCmd(ctx context.Context, message string) *exec.Cmd {
	args := a.buildArgs(message)
	cmd := exec.CommandContext(ctx, "codex", args...)

	if a.spec.Spec.Context.Workdir != "" {
		cmd.Dir = a.spec.Spec.Context.Workdir
	}

	// Forward environment variables from the spec.
	for k, v := range a.spec.Spec.Env {
		cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", k, v))
	}

	return cmd
}

func (a *Adapter) Execute(ctx context.Context, task v1.TaskRecord) (adapter.ExecuteResult, error) {
	a.mu.RLock()
	if a.phase != v1.AgentPhaseRunning {
		a.mu.RUnlock()
		return adapter.ExecuteResult{}, fmt.Errorf("agent not running (phase: %s)", a.phase)
	}
	a.mu.RUnlock()

	a.mu.Lock()
	a.metrics.TasksRunning++
	a.mu.Unlock()

	defer func() {
		a.mu.Lock()
		a.metrics.TasksRunning--
		a.mu.Unlock()
	}()

	procCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	a.mu.Lock()
	a.cancelFn = cancel
	a.mu.Unlock()

	cmd := a.buildCmd(procCtx, task.Message)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		a.mu.Lock()
		a.metrics.TasksFailed++
		a.mu.Unlock()

		errMsg := stderr.String()
		if errMsg == "" {
			errMsg = err.Error()
		}
		return adapter.ExecuteResult{}, fmt.Errorf("codex execute: %s", errMsg)
	}

	result := adapter.ExecuteResult{
		Output: stdout.String(),
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

	procCtx, cancel := context.WithCancel(ctx)

	a.mu.Lock()
	a.cancelFn = cancel
	a.metrics.TasksRunning++
	a.mu.Unlock()

	cmd := a.buildCmd(procCtx, task.Message)

	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		cancel()
		a.mu.Lock()
		a.metrics.TasksRunning--
		a.mu.Unlock()
		return nil, fmt.Errorf("codex stdout pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		cancel()
		a.mu.Lock()
		a.metrics.TasksRunning--
		a.metrics.TasksFailed++
		a.mu.Unlock()
		return nil, fmt.Errorf("codex start: %w", err)
	}

	ch := make(chan adapter.Chunk, 64)
	go func() {
		defer close(ch)
		defer cancel()
		defer func() {
			a.mu.Lock()
			a.metrics.TasksRunning--
			a.mu.Unlock()
		}()

		scanner := bufio.NewScanner(stdoutPipe)
		for scanner.Scan() {
			line := scanner.Text()
			if line == "" {
				continue
			}
			select {
			case ch <- adapter.Chunk{Content: line + "\n"}:
			case <-procCtx.Done():
				ch <- adapter.Chunk{Error: procCtx.Err()}
				return
			}
		}
		if err := scanner.Err(); err != nil {
			ch <- adapter.Chunk{Error: fmt.Errorf("codex read: %w", err)}
			a.mu.Lock()
			a.metrics.TasksFailed++
			a.mu.Unlock()
			return
		}

		// Wait for the process to finish.
		if err := cmd.Wait(); err != nil {
			ch <- adapter.Chunk{Error: fmt.Errorf("codex process: %w", err)}
			a.mu.Lock()
			a.metrics.TasksFailed++
			a.mu.Unlock()
			return
		}

		ch <- adapter.Chunk{Done: true}

		a.mu.Lock()
		a.metrics.TasksCompleted++
		a.mu.Unlock()
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
