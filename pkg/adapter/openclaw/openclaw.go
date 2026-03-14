package openclaw

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os/exec"
	"sync"
	"time"

	v1 "github.com/zlc-ai/opc-platform/api/v1"
	"github.com/zlc-ai/opc-platform/pkg/adapter"
)

// Adapter implements adapter.Adapter for OpenClaw agents.
type Adapter struct {
	mu      sync.RWMutex
	cmd     *exec.Cmd
	stdin   io.WriteCloser
	stdout  io.ReadCloser
	phase   v1.AgentPhase
	metrics v1.AgentMetrics
	spec    v1.AgentSpec
	startAt time.Time
}

// New creates a new OpenClaw adapter.
func New() adapter.Adapter {
	return &Adapter{
		phase: v1.AgentPhaseCreated,
	}
}

func (a *Adapter) Type() v1.AgentType {
	return v1.AgentTypeOpenClaw
}

func (a *Adapter) Start(ctx context.Context, spec v1.AgentSpec) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	a.spec = spec
	a.phase = v1.AgentPhaseStarting

	// Build the openclaw command.
	args := []string{"agent", "start"}
	if spec.Spec.Context.Workdir != "" {
		args = append(args, "--workdir", spec.Spec.Context.Workdir)
	}
	if spec.Spec.Runtime.Model.Name != "" {
		args = append(args, "--model", spec.Spec.Runtime.Model.Name)
	}

	cmd := exec.CommandContext(ctx, "openclaw", args...)

	stdin, err := cmd.StdinPipe()
	if err != nil {
		a.phase = v1.AgentPhaseFailed
		return fmt.Errorf("openclaw stdin pipe: %w", err)
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		a.phase = v1.AgentPhaseFailed
		return fmt.Errorf("openclaw stdout pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		a.phase = v1.AgentPhaseFailed
		return fmt.Errorf("openclaw start: %w", err)
	}

	a.cmd = cmd
	a.stdin = stdin
	a.stdout = stdout
	a.phase = v1.AgentPhaseRunning
	a.startAt = time.Now()

	return nil
}

func (a *Adapter) Stop(ctx context.Context) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.cmd == nil || a.cmd.Process == nil {
		a.phase = v1.AgentPhaseStopped
		return nil
	}

	// Close stdin to signal the agent to stop.
	if a.stdin != nil {
		a.stdin.Close()
	}

	// Wait for graceful shutdown with context timeout.
	done := make(chan error, 1)
	go func() { done <- a.cmd.Wait() }()

	select {
	case <-ctx.Done():
		a.cmd.Process.Kill()
		a.phase = v1.AgentPhaseTerminated
		return ctx.Err()
	case err := <-done:
		a.phase = v1.AgentPhaseStopped
		if err != nil {
			return fmt.Errorf("openclaw stop: %w", err)
		}
		return nil
	}
}

func (a *Adapter) Health() v1.HealthStatus {
	a.mu.RLock()
	defer a.mu.RUnlock()

	if a.cmd == nil || a.cmd.Process == nil {
		return v1.HealthStatus{Healthy: false, Message: "not started"}
	}

	// Check if process is still running.
	if a.cmd.ProcessState != nil && a.cmd.ProcessState.Exited() {
		return v1.HealthStatus{Healthy: false, Message: "process exited"}
	}

	return v1.HealthStatus{Healthy: true, Message: "running"}
}

// openclawRequest is the JSON request sent to the openclaw agent via stdin.
type openclawRequest struct {
	Type    string `json:"type"`
	Message string `json:"message"`
	ID      string `json:"id"`
}

// openclawResponse is the JSON response from the openclaw agent via stdout.
type openclawResponse struct {
	Type      string `json:"type"`
	Content   string `json:"content"`
	Done      bool   `json:"done"`
	TokensIn  int    `json:"tokens_in"`
	TokensOut int    `json:"tokens_out"`
	Error     string `json:"error,omitempty"`
}

func (a *Adapter) Execute(ctx context.Context, task v1.TaskRecord) (adapter.ExecuteResult, error) {
	a.mu.RLock()
	if a.phase != v1.AgentPhaseRunning {
		a.mu.RUnlock()
		return adapter.ExecuteResult{}, fmt.Errorf("agent not running (phase: %s)", a.phase)
	}
	a.mu.RUnlock()

	// Send request.
	req := openclawRequest{Type: "execute", Message: task.Message, ID: task.ID}
	data, _ := json.Marshal(req)
	data = append(data, '\n')

	a.mu.Lock()
	_, err := a.stdin.Write(data)
	a.mu.Unlock()
	if err != nil {
		return adapter.ExecuteResult{}, fmt.Errorf("write to openclaw: %w", err)
	}

	// Read response lines until done.
	scanner := bufio.NewScanner(a.stdout)
	var result adapter.ExecuteResult
	var output string

	for scanner.Scan() {
		var resp openclawResponse
		if err := json.Unmarshal(scanner.Bytes(), &resp); err != nil {
			continue
		}
		if resp.Error != "" {
			return adapter.ExecuteResult{}, fmt.Errorf("openclaw error: %s", resp.Error)
		}
		output += resp.Content
		result.TokensIn = resp.TokensIn
		result.TokensOut = resp.TokensOut
		if resp.Done {
			break
		}
	}
	if err := scanner.Err(); err != nil {
		return adapter.ExecuteResult{}, fmt.Errorf("read from openclaw: %w", err)
	}

	result.Output = output

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

	// Send request.
	req := openclawRequest{Type: "stream", Message: task.Message, ID: task.ID}
	data, _ := json.Marshal(req)
	data = append(data, '\n')

	a.mu.Lock()
	_, err := a.stdin.Write(data)
	a.mu.Unlock()
	if err != nil {
		return nil, fmt.Errorf("write to openclaw: %w", err)
	}

	ch := make(chan adapter.Chunk, 64)
	go func() {
		defer close(ch)
		scanner := bufio.NewScanner(a.stdout)
		for scanner.Scan() {
			var resp openclawResponse
			if err := json.Unmarshal(scanner.Bytes(), &resp); err != nil {
				continue
			}
			if resp.Error != "" {
				ch <- adapter.Chunk{Error: fmt.Errorf("openclaw: %s", resp.Error)}
				return
			}
			ch <- adapter.Chunk{Content: resp.Content, Done: resp.Done}
			if resp.Done {
				return
			}
		}
		if err := scanner.Err(); err != nil {
			ch <- adapter.Chunk{Error: err}
		}
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
