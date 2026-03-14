package custom

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os/exec"
	"strings"
	"sync"
	"time"

	v1 "github.com/zlc-ai/opc-platform/api/v1"
	"github.com/zlc-ai/opc-platform/pkg/adapter"
)

const (
	protocolStdio = "stdio"
	protocolHTTP  = "http"

	formatJSONL = "jsonl"
	formatText  = "text"

	// textSentinel is the end-of-message marker for the text protocol.
	textSentinel = "---END---"
)

// Adapter implements adapter.Adapter for user-defined custom agents.
// Custom agents are external executables that communicate via stdin/stdout (stdio)
// or HTTP. The protocol and message format are configurable through the AgentSpec.
type Adapter struct {
	mu      sync.RWMutex
	cmd     *exec.Cmd
	stdin   io.WriteCloser
	stdout  io.ReadCloser
	phase   v1.AgentPhase
	metrics v1.AgentMetrics
	spec    v1.AgentSpec
	startAt time.Time

	// httpClient is used for the HTTP protocol.
	httpClient *http.Client
	// httpBaseURL is the base URL for the HTTP protocol (derived from command/args).
	httpBaseURL string
}

// New creates a new custom agent adapter.
func New() adapter.Adapter {
	return &Adapter{
		phase: v1.AgentPhaseCreated,
		httpClient: &http.Client{
			Timeout: 5 * time.Minute,
		},
	}
}

func (a *Adapter) Type() v1.AgentType {
	return v1.AgentTypeCustom
}

func (a *Adapter) Start(ctx context.Context, spec v1.AgentSpec) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	a.spec = spec
	a.phase = v1.AgentPhaseStarting

	protocol := resolveProtocol(spec.Spec.Protocol.Type)

	if len(spec.Spec.Command) == 0 {
		a.phase = v1.AgentPhaseFailed
		return fmt.Errorf("custom agent: command is required")
	}

	cmdName := spec.Spec.Command[0]
	cmdArgs := append(spec.Spec.Command[1:], spec.Spec.Args...)

	cmd := exec.CommandContext(ctx, cmdName, cmdArgs...)

	// Set environment variables.
	if len(spec.Spec.Env) > 0 {
		env := cmd.Environ()
		for k, v := range spec.Spec.Env {
			env = append(env, k+"="+v)
		}
		cmd.Env = env
	}

	// Set working directory if specified.
	if spec.Spec.Context.Workdir != "" {
		cmd.Dir = spec.Spec.Context.Workdir
	}

	switch protocol {
	case protocolStdio:
		if err := a.startStdio(cmd); err != nil {
			return err
		}
	case protocolHTTP:
		if err := a.startHTTP(cmd, spec); err != nil {
			return err
		}
	default:
		a.phase = v1.AgentPhaseFailed
		return fmt.Errorf("custom agent: unsupported protocol type: %s", protocol)
	}

	a.cmd = cmd
	a.phase = v1.AgentPhaseRunning
	a.startAt = time.Now()
	return nil
}

// startStdio launches the command and sets up stdin/stdout pipes.
func (a *Adapter) startStdio(cmd *exec.Cmd) error {
	stdin, err := cmd.StdinPipe()
	if err != nil {
		a.phase = v1.AgentPhaseFailed
		return fmt.Errorf("custom agent stdin pipe: %w", err)
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		a.phase = v1.AgentPhaseFailed
		return fmt.Errorf("custom agent stdout pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		a.phase = v1.AgentPhaseFailed
		return fmt.Errorf("custom agent start: %w", err)
	}

	a.stdin = stdin
	a.stdout = stdout
	return nil
}

// startHTTP launches the command (which runs an HTTP server) and waits for it
// to become ready. The base URL is derived from the OPC_HTTP_PORT env var or
// defaults to http://localhost:8080.
func (a *Adapter) startHTTP(cmd *exec.Cmd, spec v1.AgentSpec) error {
	if err := cmd.Start(); err != nil {
		a.phase = v1.AgentPhaseFailed
		return fmt.Errorf("custom agent start (http): %w", err)
	}

	// Determine the base URL from env or default.
	baseURL := "http://localhost:8080"
	if port, ok := spec.Spec.Env["OPC_HTTP_PORT"]; ok {
		baseURL = "http://localhost:" + port
	}
	if url, ok := spec.Spec.Env["OPC_HTTP_URL"]; ok {
		baseURL = url
	}
	a.httpBaseURL = strings.TrimRight(baseURL, "/")

	// Wait briefly for the server to start accepting connections.
	if err := a.waitForHTTPReady(); err != nil {
		_ = cmd.Process.Kill()
		a.phase = v1.AgentPhaseFailed
		return fmt.Errorf("custom agent http not ready: %w", err)
	}

	return nil
}

// waitForHTTPReady polls the HTTP health endpoint until it responds or times out.
func (a *Adapter) waitForHTTPReady() error {
	deadline := time.Now().Add(30 * time.Second)
	for time.Now().Before(deadline) {
		resp, err := a.httpClient.Get(a.httpBaseURL + "/health")
		if err == nil {
			resp.Body.Close()
			if resp.StatusCode < 500 {
				return nil
			}
		}
		time.Sleep(250 * time.Millisecond)
	}
	return fmt.Errorf("timeout waiting for HTTP server at %s", a.httpBaseURL)
}

func (a *Adapter) Stop(ctx context.Context) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.cmd == nil || a.cmd.Process == nil {
		a.phase = v1.AgentPhaseStopped
		return nil
	}

	// Close stdin to signal the agent to stop (stdio protocol).
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
			return fmt.Errorf("custom agent stop: %w", err)
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

	// Check if the process has exited.
	if a.cmd.ProcessState != nil && a.cmd.ProcessState.Exited() {
		return v1.HealthStatus{Healthy: false, Message: "process exited"}
	}

	protocol := resolveProtocol(a.spec.Spec.Protocol.Type)
	if protocol == protocolHTTP {
		return a.healthHTTP()
	}

	return v1.HealthStatus{Healthy: true, Message: "running"}
}

// healthHTTP checks health by pinging the HTTP health endpoint.
func (a *Adapter) healthHTTP() v1.HealthStatus {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, a.httpBaseURL+"/health", nil)
	if err != nil {
		return v1.HealthStatus{Healthy: false, Message: "health check request error: " + err.Error()}
	}

	resp, err := a.httpClient.Do(req)
	if err != nil {
		return v1.HealthStatus{Healthy: false, Message: "health check failed: " + err.Error()}
	}
	resp.Body.Close()

	if resp.StatusCode >= 500 {
		return v1.HealthStatus{Healthy: false, Message: fmt.Sprintf("health check returned %d", resp.StatusCode)}
	}
	return v1.HealthStatus{Healthy: true, Message: "running"}
}

// jsonlRequest is the JSON request sent to the custom agent via stdin (jsonl format).
type jsonlRequest struct {
	Type    string `json:"type"`
	Message string `json:"message"`
	ID      string `json:"id"`
}

// jsonlResponse is the JSON response from the custom agent (jsonl format).
type jsonlResponse struct {
	Type      string `json:"type"`
	Content   string `json:"content"`
	Done      bool   `json:"done"`
	TokensIn  int    `json:"tokens_in"`
	TokensOut int    `json:"tokens_out"`
	Error     string `json:"error,omitempty"`
}

// httpRequest is the JSON body sent to the custom agent via HTTP POST.
type httpRequest struct {
	Type    string `json:"type"`
	Message string `json:"message"`
	ID      string `json:"id"`
}

// httpResponse is the JSON body returned from the custom agent via HTTP.
type httpResponse struct {
	Content   string `json:"content"`
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
	protocol := resolveProtocol(a.spec.Spec.Protocol.Type)
	format := resolveFormat(a.spec.Spec.Protocol.Format)
	a.mu.RUnlock()

	var result adapter.ExecuteResult
	var err error

	switch protocol {
	case protocolStdio:
		result, err = a.executeStdio(task, format)
	case protocolHTTP:
		result, err = a.executeHTTP(ctx, task)
	default:
		return adapter.ExecuteResult{}, fmt.Errorf("unsupported protocol: %s", protocol)
	}

	if err != nil {
		a.mu.Lock()
		a.metrics.TasksFailed++
		a.mu.Unlock()
		return adapter.ExecuteResult{}, err
	}

	a.mu.Lock()
	a.metrics.TasksCompleted++
	a.metrics.TotalTokensIn += result.TokensIn
	a.metrics.TotalTokensOut += result.TokensOut
	a.mu.Unlock()

	return result, nil
}

// executeStdio handles task execution over the stdio protocol.
func (a *Adapter) executeStdio(task v1.TaskRecord, format string) (adapter.ExecuteResult, error) {
	switch format {
	case formatJSONL:
		return a.executeStdioJSONL(task)
	case formatText:
		return a.executeStdioText(task)
	default:
		return adapter.ExecuteResult{}, fmt.Errorf("unsupported format: %s", format)
	}
}

// executeStdioJSONL uses the JSONL protocol (same as OpenClaw).
func (a *Adapter) executeStdioJSONL(task v1.TaskRecord) (adapter.ExecuteResult, error) {
	req := jsonlRequest{Type: "execute", Message: task.Message, ID: task.ID}
	data, _ := json.Marshal(req)
	data = append(data, '\n')

	a.mu.Lock()
	_, err := a.stdin.Write(data)
	a.mu.Unlock()
	if err != nil {
		return adapter.ExecuteResult{}, fmt.Errorf("write to custom agent: %w", err)
	}

	scanner := bufio.NewScanner(a.stdout)
	var result adapter.ExecuteResult
	var output strings.Builder

	for scanner.Scan() {
		var resp jsonlResponse
		if err := json.Unmarshal(scanner.Bytes(), &resp); err != nil {
			continue
		}
		if resp.Error != "" {
			return adapter.ExecuteResult{}, fmt.Errorf("custom agent error: %s", resp.Error)
		}
		output.WriteString(resp.Content)
		result.TokensIn = resp.TokensIn
		result.TokensOut = resp.TokensOut
		if resp.Done {
			break
		}
	}
	if err := scanner.Err(); err != nil {
		return adapter.ExecuteResult{}, fmt.Errorf("read from custom agent: %w", err)
	}

	result.Output = output.String()
	return result, nil
}

// executeStdioText uses the plain text protocol: write message + newline,
// read lines until the sentinel "---END---" or EOF.
func (a *Adapter) executeStdioText(task v1.TaskRecord) (adapter.ExecuteResult, error) {
	msg := task.Message + "\n"

	a.mu.Lock()
	_, err := a.stdin.Write([]byte(msg))
	a.mu.Unlock()
	if err != nil {
		return adapter.ExecuteResult{}, fmt.Errorf("write to custom agent: %w", err)
	}

	scanner := bufio.NewScanner(a.stdout)
	var output strings.Builder

	for scanner.Scan() {
		line := scanner.Text()
		if line == textSentinel {
			break
		}
		if output.Len() > 0 {
			output.WriteString("\n")
		}
		output.WriteString(line)
	}
	if err := scanner.Err(); err != nil {
		return adapter.ExecuteResult{}, fmt.Errorf("read from custom agent: %w", err)
	}

	return adapter.ExecuteResult{Output: output.String()}, nil
}

// executeHTTP sends a POST request to the custom agent's /execute endpoint.
func (a *Adapter) executeHTTP(ctx context.Context, task v1.TaskRecord) (adapter.ExecuteResult, error) {
	reqBody := httpRequest{Type: "execute", Message: task.Message, ID: task.ID}
	data, _ := json.Marshal(reqBody)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, a.httpBaseURL+"/execute", bytes.NewReader(data))
	if err != nil {
		return adapter.ExecuteResult{}, fmt.Errorf("custom agent http request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := a.httpClient.Do(req)
	if err != nil {
		return adapter.ExecuteResult{}, fmt.Errorf("custom agent http execute: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		return adapter.ExecuteResult{}, fmt.Errorf("custom agent http error (%d): %s", resp.StatusCode, string(body))
	}

	var httpResp httpResponse
	if err := json.NewDecoder(resp.Body).Decode(&httpResp); err != nil {
		return adapter.ExecuteResult{}, fmt.Errorf("custom agent http decode: %w", err)
	}
	if httpResp.Error != "" {
		return adapter.ExecuteResult{}, fmt.Errorf("custom agent error: %s", httpResp.Error)
	}

	return adapter.ExecuteResult{
		Output:    httpResp.Content,
		TokensIn:  httpResp.TokensIn,
		TokensOut: httpResp.TokensOut,
	}, nil
}

func (a *Adapter) Stream(ctx context.Context, task v1.TaskRecord) (<-chan adapter.Chunk, error) {
	a.mu.RLock()
	if a.phase != v1.AgentPhaseRunning {
		a.mu.RUnlock()
		return nil, fmt.Errorf("agent not running (phase: %s)", a.phase)
	}
	protocol := resolveProtocol(a.spec.Spec.Protocol.Type)
	format := resolveFormat(a.spec.Spec.Protocol.Format)
	a.mu.RUnlock()

	switch protocol {
	case protocolStdio:
		return a.streamStdio(task, format)
	case protocolHTTP:
		return a.streamHTTP(ctx, task)
	default:
		return nil, fmt.Errorf("unsupported protocol: %s", protocol)
	}
}

// streamStdio handles streaming over the stdio protocol.
func (a *Adapter) streamStdio(task v1.TaskRecord, format string) (<-chan adapter.Chunk, error) {
	switch format {
	case formatJSONL:
		return a.streamStdioJSONL(task)
	case formatText:
		return a.streamStdioText(task)
	default:
		return nil, fmt.Errorf("unsupported format: %s", format)
	}
}

// streamStdioJSONL uses the JSONL protocol for streaming.
func (a *Adapter) streamStdioJSONL(task v1.TaskRecord) (<-chan adapter.Chunk, error) {
	req := jsonlRequest{Type: "stream", Message: task.Message, ID: task.ID}
	data, _ := json.Marshal(req)
	data = append(data, '\n')

	a.mu.Lock()
	_, err := a.stdin.Write(data)
	a.mu.Unlock()
	if err != nil {
		return nil, fmt.Errorf("write to custom agent: %w", err)
	}

	ch := make(chan adapter.Chunk, 64)
	go func() {
		defer close(ch)
		scanner := bufio.NewScanner(a.stdout)
		for scanner.Scan() {
			var resp jsonlResponse
			if err := json.Unmarshal(scanner.Bytes(), &resp); err != nil {
				continue
			}
			if resp.Error != "" {
				ch <- adapter.Chunk{Error: fmt.Errorf("custom agent: %s", resp.Error)}
				return
			}
			ch <- adapter.Chunk{Content: resp.Content, Done: resp.Done}
			if resp.Done {
				a.mu.Lock()
				a.metrics.TasksCompleted++
				a.metrics.TotalTokensIn += resp.TokensIn
				a.metrics.TotalTokensOut += resp.TokensOut
				a.mu.Unlock()
				return
			}
		}
		if err := scanner.Err(); err != nil {
			ch <- adapter.Chunk{Error: err}
		}
	}()

	return ch, nil
}

// streamStdioText uses the plain text protocol for streaming.
func (a *Adapter) streamStdioText(task v1.TaskRecord) (<-chan adapter.Chunk, error) {
	msg := task.Message + "\n"

	a.mu.Lock()
	_, err := a.stdin.Write([]byte(msg))
	a.mu.Unlock()
	if err != nil {
		return nil, fmt.Errorf("write to custom agent: %w", err)
	}

	ch := make(chan adapter.Chunk, 64)
	go func() {
		defer close(ch)
		scanner := bufio.NewScanner(a.stdout)
		for scanner.Scan() {
			line := scanner.Text()
			if line == textSentinel {
				ch <- adapter.Chunk{Content: "", Done: true}
				a.mu.Lock()
				a.metrics.TasksCompleted++
				a.mu.Unlock()
				return
			}
			ch <- adapter.Chunk{Content: line + "\n", Done: false}
		}
		if err := scanner.Err(); err != nil {
			ch <- adapter.Chunk{Error: err}
		}
	}()

	return ch, nil
}

// streamHTTP handles streaming over HTTP using chunked/NDJSON responses.
func (a *Adapter) streamHTTP(ctx context.Context, task v1.TaskRecord) (<-chan adapter.Chunk, error) {
	reqBody := httpRequest{Type: "stream", Message: task.Message, ID: task.ID}
	data, _ := json.Marshal(reqBody)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, a.httpBaseURL+"/stream", bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("custom agent http request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := a.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("custom agent http stream: %w", err)
	}

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		return nil, fmt.Errorf("custom agent http error (%d): %s", resp.StatusCode, string(body))
	}

	ch := make(chan adapter.Chunk, 64)
	go func() {
		defer close(ch)
		defer resp.Body.Close()

		scanner := bufio.NewScanner(resp.Body)
		for scanner.Scan() {
			line := scanner.Text()
			if line == "" {
				continue
			}
			var chunk jsonlResponse
			if err := json.Unmarshal([]byte(line), &chunk); err != nil {
				continue
			}
			if chunk.Error != "" {
				ch <- adapter.Chunk{Error: fmt.Errorf("custom agent: %s", chunk.Error)}
				return
			}
			ch <- adapter.Chunk{Content: chunk.Content, Done: chunk.Done}
			if chunk.Done {
				a.mu.Lock()
				a.metrics.TasksCompleted++
				a.metrics.TotalTokensIn += chunk.TokensIn
				a.metrics.TotalTokensOut += chunk.TokensOut
				a.mu.Unlock()
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

// resolveProtocol returns the protocol type, defaulting to stdio.
func resolveProtocol(p string) string {
	switch strings.ToLower(p) {
	case protocolHTTP:
		return protocolHTTP
	case protocolStdio, "":
		return protocolStdio
	default:
		return p
	}
}

// resolveFormat returns the message format, defaulting to jsonl.
func resolveFormat(f string) string {
	switch strings.ToLower(f) {
	case formatText:
		return formatText
	case formatJSONL, "":
		return formatJSONL
	default:
		return f
	}
}
