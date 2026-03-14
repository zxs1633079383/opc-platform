package openai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"sync"
	"time"

	v1 "github.com/zlc-ai/opc-platform/api/v1"
	"github.com/zlc-ai/opc-platform/pkg/adapter"
)

const defaultBaseURL = "https://api.openai.com/v1"
const defaultModel = "gpt-4o"

// Adapter implements adapter.Adapter for OpenAI API-based agents.
type Adapter struct {
	mu      sync.RWMutex
	phase   v1.AgentPhase
	metrics v1.AgentMetrics
	spec    v1.AgentSpec
	startAt time.Time
	client  *http.Client
	apiKey  string
	baseURL string
}

// New creates a new OpenAI adapter.
func New() adapter.Adapter {
	return &Adapter{
		phase:  v1.AgentPhaseCreated,
		client: &http.Client{Timeout: 5 * time.Minute},
	}
}

func (a *Adapter) Type() v1.AgentType {
	return v1.AgentTypeOpenAI
}

func (a *Adapter) Start(_ context.Context, spec v1.AgentSpec) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	a.spec = spec
	a.phase = v1.AgentPhaseRunning
	a.startAt = time.Now()

	a.apiKey = spec.Spec.Env["OPENAI_API_KEY"]
	if a.apiKey == "" {
		a.apiKey = os.Getenv("OPENAI_API_KEY")
	}
	if a.apiKey == "" {
		a.phase = v1.AgentPhaseFailed
		return fmt.Errorf("OPENAI_API_KEY is required")
	}

	a.baseURL = spec.Spec.Env["OPENAI_BASE_URL"]
	if a.baseURL == "" {
		a.baseURL = os.Getenv("OPENAI_BASE_URL")
	}
	if a.baseURL == "" {
		a.baseURL = defaultBaseURL
	}

	return nil
}

func (a *Adapter) Stop(_ context.Context) error {
	a.mu.Lock()
	defer a.mu.Unlock()
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

// chatRequest is the OpenAI chat completions request body.
type chatRequest struct {
	Model    string        `json:"model"`
	Messages []chatMessage `json:"messages"`
	Tools    []tool        `json:"tools,omitempty"`
	Stream   bool          `json:"stream,omitempty"`
}

type chatMessage struct {
	Role       string     `json:"role"`
	Content    string     `json:"content,omitempty"`
	ToolCalls  []toolCall `json:"tool_calls,omitempty"`
	ToolCallID string     `json:"tool_call_id,omitempty"`
}

type tool struct {
	Type     string       `json:"type"`
	Function toolFunction `json:"function"`
}

type toolFunction struct {
	Name        string          `json:"name"`
	Description string          `json:"description,omitempty"`
	Parameters  json.RawMessage `json:"parameters,omitempty"`
}

type toolCall struct {
	ID       string           `json:"id"`
	Type     string           `json:"type"`
	Function toolCallFunction `json:"function"`
}

type toolCallFunction struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

// chatResponse is the OpenAI chat completions response body.
type chatResponse struct {
	ID      string         `json:"id"`
	Choices []chatChoice   `json:"choices"`
	Usage   chatUsage      `json:"usage"`
}

type chatChoice struct {
	Index        int         `json:"index"`
	Message      chatMessage `json:"message"`
	FinishReason string      `json:"finish_reason"`
}

type chatUsage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

// streamChunk is a chunk from the streaming response.
type streamChunk struct {
	ID      string              `json:"id"`
	Choices []streamChunkChoice `json:"choices"`
}

type streamChunkChoice struct {
	Delta        chatMessage `json:"delta"`
	FinishReason *string     `json:"finish_reason"`
}

func (a *Adapter) modelName() string {
	a.mu.RLock()
	defer a.mu.RUnlock()
	if a.spec.Spec.Runtime.Model.Name != "" {
		return a.spec.Spec.Runtime.Model.Name
	}
	return defaultModel
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

	reqBody := chatRequest{
		Model: a.modelName(),
		Messages: []chatMessage{
			{Role: "user", Content: task.Message},
		},
	}

	respBody, err := a.doChat(ctx, reqBody)
	if err != nil {
		a.mu.Lock()
		a.metrics.TasksFailed++
		a.mu.Unlock()
		return adapter.ExecuteResult{}, fmt.Errorf("openai execute: %w", err)
	}

	var output string
	if len(respBody.Choices) > 0 {
		output = respBody.Choices[0].Message.Content
	}

	result := adapter.ExecuteResult{
		Output:    output,
		TokensIn:  respBody.Usage.PromptTokens,
		TokensOut: respBody.Usage.CompletionTokens,
	}

	a.mu.Lock()
	a.metrics.TasksCompleted++
	a.metrics.TotalTokensIn += result.TokensIn
	a.metrics.TotalTokensOut += result.TokensOut
	a.mu.Unlock()

	return result, nil
}

func (a *Adapter) doChat(ctx context.Context, reqBody chatRequest) (chatResponse, error) {
	body, err := json.Marshal(reqBody)
	if err != nil {
		return chatResponse{}, fmt.Errorf("marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		a.baseURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return chatResponse{}, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+a.apiKey)

	resp, err := a.client.Do(req)
	if err != nil {
		return chatResponse{}, fmt.Errorf("http request: %w", err)
	}
	defer resp.Body.Close()

	respData, err := io.ReadAll(resp.Body)
	if err != nil {
		return chatResponse{}, fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return chatResponse{}, fmt.Errorf("API error (status %d): %s", resp.StatusCode, string(respData))
	}

	var result chatResponse
	if err := json.Unmarshal(respData, &result); err != nil {
		return chatResponse{}, fmt.Errorf("unmarshal response: %w", err)
	}

	return result, nil
}

func (a *Adapter) Stream(ctx context.Context, task v1.TaskRecord) (<-chan adapter.Chunk, error) {
	a.mu.RLock()
	if a.phase != v1.AgentPhaseRunning {
		a.mu.RUnlock()
		return nil, fmt.Errorf("agent not running (phase: %s)", a.phase)
	}
	a.mu.RUnlock()

	a.mu.Lock()
	a.metrics.TasksRunning++
	a.mu.Unlock()

	reqBody := chatRequest{
		Model: a.modelName(),
		Messages: []chatMessage{
			{Role: "user", Content: task.Message},
		},
		Stream: true,
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		a.mu.Lock()
		a.metrics.TasksRunning--
		a.mu.Unlock()
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		a.baseURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		a.mu.Lock()
		a.metrics.TasksRunning--
		a.mu.Unlock()
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+a.apiKey)

	resp, err := a.client.Do(req)
	if err != nil {
		a.mu.Lock()
		a.metrics.TasksRunning--
		a.metrics.TasksFailed++
		a.mu.Unlock()
		return nil, fmt.Errorf("http request: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		respData, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		a.mu.Lock()
		a.metrics.TasksRunning--
		a.metrics.TasksFailed++
		a.mu.Unlock()
		return nil, fmt.Errorf("API error (status %d): %s", resp.StatusCode, string(respData))
	}

	ch := make(chan adapter.Chunk, 64)
	go func() {
		defer close(ch)
		defer resp.Body.Close()
		defer func() {
			a.mu.Lock()
			a.metrics.TasksRunning--
			a.mu.Unlock()
		}()

		decoder := json.NewDecoder(resp.Body)
		for {
			// Read SSE lines: "data: {...}" or "data: [DONE]"
			var buf [1]byte
			var line []byte
			for {
				_, err := resp.Body.Read(buf[:])
				if err != nil {
					if err == io.EOF {
						ch <- adapter.Chunk{Done: true}
						a.mu.Lock()
						a.metrics.TasksCompleted++
						a.mu.Unlock()
						return
					}
					ch <- adapter.Chunk{Error: fmt.Errorf("read stream: %w", err)}
					a.mu.Lock()
					a.metrics.TasksFailed++
					a.mu.Unlock()
					return
				}
				if buf[0] == '\n' {
					break
				}
				line = append(line, buf[0])
			}

			lineStr := string(line)
			if lineStr == "" || lineStr == "\r" {
				continue
			}

			// Strip "data: " prefix
			const prefix = "data: "
			if len(lineStr) < len(prefix) {
				continue
			}
			data := lineStr[len(prefix):]
			if data == "[DONE]" {
				ch <- adapter.Chunk{Done: true}
				a.mu.Lock()
				a.metrics.TasksCompleted++
				a.mu.Unlock()
				return
			}

			var chunk streamChunk
			if err := json.Unmarshal([]byte(data), &chunk); err != nil {
				continue
			}

			for _, choice := range chunk.Choices {
				if choice.Delta.Content != "" {
					select {
					case ch <- adapter.Chunk{Content: choice.Delta.Content}:
					case <-ctx.Done():
						ch <- adapter.Chunk{Error: ctx.Err()}
						return
					}
				}
			}
		}
		_ = decoder // suppress unused warning
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
