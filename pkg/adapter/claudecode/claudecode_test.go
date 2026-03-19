package claudecode

import (
	"context"
	"encoding/json"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	v1 "github.com/zlc-ai/opc-platform/api/v1"
	"github.com/zlc-ai/opc-platform/pkg/adapter"
)

// --- New tests ---

func TestNew(t *testing.T) {
	a := New()
	if a == nil {
		t.Fatal("New() returned nil")
	}

	ca, ok := a.(*Adapter)
	if !ok {
		t.Fatalf("expected *Adapter, got %T", a)
	}

	if ca.Type() != v1.AgentTypeClaudeCode {
		t.Errorf("Type() = %q, want %q", ca.Type(), v1.AgentTypeClaudeCode)
	}
	if ca.phase != v1.AgentPhaseCreated {
		t.Errorf("initial phase = %q, want Created", ca.phase)
	}
}

// --- Start tests ---

func TestStart(t *testing.T) {
	t.Run("sets phase to Running", func(t *testing.T) {
		a := New().(*Adapter)
		workdir := t.TempDir()
		spec := v1.AgentSpec{
			Metadata: v1.Metadata{Name: "test-agent"},
			Spec: v1.AgentSpecBody{
				Type:    v1.AgentTypeClaudeCode,
				Context: v1.ContextConfig{Workdir: workdir},
			},
		}

		err := a.Start(context.Background(), spec)
		if err != nil {
			t.Fatalf("Start: %v", err)
		}
		if a.Status() != v1.AgentPhaseRunning {
			t.Errorf("phase = %q, want Running", a.Status())
		}
	})

	t.Run("creates default workdir", func(t *testing.T) {
		a := New().(*Adapter)
		spec := v1.AgentSpec{
			Metadata: v1.Metadata{Name: "test-agent"},
			Spec:     v1.AgentSpecBody{Type: v1.AgentTypeClaudeCode},
		}

		err := a.Start(context.Background(), spec)
		if err != nil {
			t.Fatalf("Start: %v", err)
		}

		if _, err := os.Stat("/tmp/opc"); os.IsNotExist(err) {
			t.Error("default workdir /tmp/opc not created")
		}
	})

	t.Run("stores spec", func(t *testing.T) {
		a := New().(*Adapter)
		spec := v1.AgentSpec{
			Metadata: v1.Metadata{Name: "my-agent"},
			Spec: v1.AgentSpecBody{
				Type: v1.AgentTypeClaudeCode,
				Runtime: v1.RuntimeConfig{
					Model: v1.ModelConfig{Name: "claude-sonnet-4-6"},
				},
			},
		}

		a.Start(context.Background(), spec)
		if a.spec.Metadata.Name != "my-agent" {
			t.Errorf("spec name = %q, want my-agent", a.spec.Metadata.Name)
		}
	})
}

// --- Stop tests ---

func TestStop(t *testing.T) {
	t.Run("sets phase to Stopped", func(t *testing.T) {
		a := New().(*Adapter)
		a.Start(context.Background(), v1.AgentSpec{
			Spec: v1.AgentSpecBody{Type: v1.AgentTypeClaudeCode},
		})

		err := a.Stop(context.Background())
		if err != nil {
			t.Fatalf("Stop: %v", err)
		}
		if a.Status() != v1.AgentPhaseStopped {
			t.Errorf("phase = %q, want Stopped", a.Status())
		}
	})

	t.Run("clears activeCmd", func(t *testing.T) {
		a := New().(*Adapter)
		a.Start(context.Background(), v1.AgentSpec{
			Spec: v1.AgentSpecBody{Type: v1.AgentTypeClaudeCode},
		})
		a.Stop(context.Background())

		a.mu.RLock()
		defer a.mu.RUnlock()
		if a.activeCmd != nil {
			t.Error("activeCmd should be nil after Stop")
		}
	})
}

// --- Health tests ---

func TestHealth(t *testing.T) {
	t.Run("healthy when running", func(t *testing.T) {
		a := New().(*Adapter)
		a.Start(context.Background(), v1.AgentSpec{
			Spec: v1.AgentSpecBody{Type: v1.AgentTypeClaudeCode},
		})

		h := a.Health()
		if !h.Healthy {
			t.Error("expected healthy when running")
		}
		if h.Message != "ready" {
			t.Errorf("message = %q, want ready", h.Message)
		}
	})

	t.Run("unhealthy when created", func(t *testing.T) {
		a := New().(*Adapter)
		h := a.Health()
		if h.Healthy {
			t.Error("expected unhealthy when created")
		}
	})

	t.Run("unhealthy when stopped", func(t *testing.T) {
		a := New().(*Adapter)
		a.Start(context.Background(), v1.AgentSpec{
			Spec: v1.AgentSpecBody{Type: v1.AgentTypeClaudeCode},
		})
		a.Stop(context.Background())

		h := a.Health()
		if h.Healthy {
			t.Error("expected unhealthy when stopped")
		}
	})
}

// --- buildBaseArgs tests ---

func TestBuildBaseArgs(t *testing.T) {
	tests := []struct {
		name     string
		spec     v1.AgentSpec
		contains []string
		excludes []string
	}{
		{
			name: "with model name",
			spec: v1.AgentSpec{
				Spec: v1.AgentSpecBody{
					Runtime: v1.RuntimeConfig{
						Model: v1.ModelConfig{Name: "claude-sonnet-4-6"},
					},
				},
			},
			contains: []string{"--model", "claude-sonnet-4-6"},
		},
		{
			name: "model name mapping dot to dash",
			spec: v1.AgentSpec{
				Spec: v1.AgentSpecBody{
					Runtime: v1.RuntimeConfig{
						Model: v1.ModelConfig{Name: "claude-sonnet-4.6"},
					},
				},
			},
			contains: []string{"--model", "claude-sonnet-4-6"},
		},
		{
			name: "legacy model alias",
			spec: v1.AgentSpec{
				Spec: v1.AgentSpecBody{
					Runtime: v1.RuntimeConfig{
						Model: v1.ModelConfig{Name: "claude-sonnet-4"},
					},
				},
			},
			contains: []string{"--model", "claude-sonnet-4-6"},
		},
		{
			name: "with maxTokens",
			spec: v1.AgentSpec{
				Spec: v1.AgentSpecBody{
					Runtime: v1.RuntimeConfig{
						Inference: v1.InferenceConfig{MaxTokens: 100},
					},
				},
			},
			contains: []string{"--max-turns", "100"},
		},
		{
			name:     "no model no maxTokens",
			spec:     v1.AgentSpec{},
			excludes: []string{"--model", "--max-turns"},
		},
		{
			name: "unknown model passed through",
			spec: v1.AgentSpec{
				Spec: v1.AgentSpecBody{
					Runtime: v1.RuntimeConfig{
						Model: v1.ModelConfig{Name: "custom-model-xyz"},
					},
				},
			},
			contains: []string{"--model", "custom-model-xyz"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := &Adapter{spec: tt.spec}
			args := a.buildBaseArgs()
			argsStr := strings.Join(args, " ")

			for _, want := range tt.contains {
				if !strings.Contains(argsStr, want) {
					t.Errorf("args %q should contain %q", argsStr, want)
				}
			}
			for _, exclude := range tt.excludes {
				if strings.Contains(argsStr, exclude) {
					t.Errorf("args %q should not contain %q", argsStr, exclude)
				}
			}
		})
	}
}

// --- claudeModelMap tests ---

func TestClaudeModelMap(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"claude-opus-4-6", "claude-opus-4-6"},
		{"claude-opus-4.6", "claude-opus-4-6"},
		{"claude-sonnet-4-6", "claude-sonnet-4-6"},
		{"claude-sonnet-4.6", "claude-sonnet-4-6"},
		{"claude-opus-4-5", "claude-opus-4-5-20250514"},
		{"claude-opus-4.5", "claude-opus-4-5-20250514"},
		{"claude-sonnet-4-5", "claude-sonnet-4-5-20250514"},
		{"claude-haiku-4-5", "claude-haiku-4-5-20251001"},
		{"claude-sonnet-4", "claude-sonnet-4-6"},
		{"claude-opus-4", "claude-opus-4-6"},
		{"claude-haiku-4", "claude-haiku-4-5-20251001"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got, ok := claudeModelMap[tt.input]
			if !ok {
				t.Fatalf("model %q not in map", tt.input)
			}
			if got != tt.want {
				t.Errorf("claudeModelMap[%q] = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

// --- Execute: not running error ---

func TestExecute_NotRunning(t *testing.T) {
	a := New().(*Adapter)
	task := v1.TaskRecord{ID: "t1", Message: "hello"}
	_, err := a.Execute(context.Background(), task)
	if err == nil {
		t.Fatal("expected error when not running")
	}
	if !strings.Contains(err.Error(), "not running") {
		t.Errorf("error = %q, want 'not running'", err.Error())
	}
}

// --- Stream: not running error ---

func TestStream_NotRunning(t *testing.T) {
	a := New().(*Adapter)
	task := v1.TaskRecord{ID: "t1", Message: "hello"}

	_, err := a.Stream(context.Background(), task)
	if err == nil {
		t.Fatal("expected error when not running")
	}
	if !strings.Contains(err.Error(), "not running") {
		t.Errorf("error = %q, want 'not running'", err.Error())
	}
}

// --- Metrics tests ---

func TestMetrics(t *testing.T) {
	a := New().(*Adapter)
	a.Start(context.Background(), v1.AgentSpec{
		Spec: v1.AgentSpecBody{Type: v1.AgentTypeClaudeCode},
	})
	time.Sleep(10 * time.Millisecond)

	m := a.Metrics()
	if m.UptimeSeconds <= 0 {
		t.Errorf("UptimeSeconds = %f, want > 0", m.UptimeSeconds)
	}
}

func TestMetrics_TaskCounting(t *testing.T) {
	a := New().(*Adapter)
	a.Start(context.Background(), v1.AgentSpec{
		Spec: v1.AgentSpecBody{Type: v1.AgentTypeClaudeCode},
	})

	a.mu.Lock()
	a.metrics.TasksCompleted = 5
	a.metrics.TasksFailed = 2
	a.metrics.TotalTokensIn = 1000
	a.metrics.TotalTokensOut = 500
	a.mu.Unlock()

	m := a.Metrics()
	if m.TasksCompleted != 5 {
		t.Errorf("TasksCompleted = %d, want 5", m.TasksCompleted)
	}
	if m.TasksFailed != 2 {
		t.Errorf("TasksFailed = %d, want 2", m.TasksFailed)
	}
	if m.TotalTokensIn != 1000 {
		t.Errorf("TotalTokensIn = %d, want 1000", m.TotalTokensIn)
	}
}

// --- Interface compliance ---

func TestAdapter_ImplementsInterface(t *testing.T) {
	var _ adapter.Adapter = New()
}

// --- Concurrent access safety ---

func TestConcurrent_StatusAndHealth(t *testing.T) {
	a := New().(*Adapter)
	a.Start(context.Background(), v1.AgentSpec{
		Spec: v1.AgentSpecBody{Type: v1.AgentTypeClaudeCode},
	})

	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(3)
		go func() {
			defer wg.Done()
			_ = a.Health()
		}()
		go func() {
			defer wg.Done()
			_ = a.Status()
		}()
		go func() {
			defer wg.Done()
			_ = a.Metrics()
		}()
	}
	wg.Wait()
}

// --- claudeCodeResult JSON parsing ---

func TestClaudeCodeResult_Parsing(t *testing.T) {
	t.Run("with usage field", func(t *testing.T) {
		jsonStr := `{"type":"result","result":"hello world","usage":{"input_tokens":100,"output_tokens":50,"cache_creation_input_tokens":10,"cache_read_input_tokens":5},"total_cost_usd":0.05}`

		var parsed claudeCodeResult
		if err := json.Unmarshal([]byte(jsonStr), &parsed); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}

		if parsed.Result != "hello world" {
			t.Errorf("Result = %q", parsed.Result)
		}
		if parsed.TotalCostUSD != 0.05 {
			t.Errorf("TotalCostUSD = %f", parsed.TotalCostUSD)
		}
		if parsed.Usage == nil {
			t.Fatal("Usage is nil")
		}
		if parsed.Usage.InputTokens != 100 {
			t.Errorf("InputTokens = %d", parsed.Usage.InputTokens)
		}
		if parsed.Usage.OutputTokens != 50 {
			t.Errorf("OutputTokens = %d", parsed.Usage.OutputTokens)
		}
		if parsed.Usage.CacheCreationInputTokens != 10 {
			t.Errorf("CacheCreationInputTokens = %d", parsed.Usage.CacheCreationInputTokens)
		}
		if parsed.Usage.CacheReadInputTokens != 5 {
			t.Errorf("CacheReadInputTokens = %d", parsed.Usage.CacheReadInputTokens)
		}
	})

	t.Run("with modelUsage", func(t *testing.T) {
		jsonStr := `{"type":"result","result":"output","modelUsage":{"claude-sonnet-4-6":{"inputTokens":200,"outputTokens":100,"cacheReadInputTokens":20,"costUSD":0.03}}}`

		var parsed claudeCodeResult
		if err := json.Unmarshal([]byte(jsonStr), &parsed); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}

		if len(parsed.ModelUsage) != 1 {
			t.Fatalf("ModelUsage length = %d, want 1", len(parsed.ModelUsage))
		}

		mu := parsed.ModelUsage["claude-sonnet-4-6"]
		if mu.InputTokens != 200 || mu.OutputTokens != 100 {
			t.Errorf("tokens: in=%d out=%d, want 200/100", mu.InputTokens, mu.OutputTokens)
		}
		if mu.CacheReadInputTokens != 20 {
			t.Errorf("CacheReadInputTokens = %d, want 20", mu.CacheReadInputTokens)
		}
		if mu.CostUSD != 0.03 {
			t.Errorf("CostUSD = %f, want 0.03", mu.CostUSD)
		}
	})

	t.Run("is_error true", func(t *testing.T) {
		jsonStr := `{"type":"result","is_error":true,"error":"something went wrong"}`

		var parsed claudeCodeResult
		json.Unmarshal([]byte(jsonStr), &parsed)

		if !parsed.IsError {
			t.Error("expected IsError = true")
		}
		if parsed.Error != "something went wrong" {
			t.Errorf("Error = %q", parsed.Error)
		}
	})

	t.Run("content fallback when result empty", func(t *testing.T) {
		jsonStr := `{"type":"result","content":"fallback content"}`

		var parsed claudeCodeResult
		json.Unmarshal([]byte(jsonStr), &parsed)

		if parsed.Result != "" {
			t.Errorf("Result should be empty, got %q", parsed.Result)
		}
		if parsed.Content != "fallback content" {
			t.Errorf("Content = %q", parsed.Content)
		}
	})

	t.Run("top-level token fields", func(t *testing.T) {
		jsonStr := `{"type":"result","result":"ok","input_tokens":300,"output_tokens":150}`

		var parsed claudeCodeResult
		json.Unmarshal([]byte(jsonStr), &parsed)

		if parsed.TokensIn != 300 {
			t.Errorf("TokensIn = %d, want 300", parsed.TokensIn)
		}
		if parsed.TokensOut != 150 {
			t.Errorf("TokensOut = %d, want 150", parsed.TokensOut)
		}
	})
}

// --- stream event parsing ---

func TestClaudeCodeStreamEvent_Parsing(t *testing.T) {
	t.Run("assistant event", func(t *testing.T) {
		jsonStr := `{"type":"assistant","content":"partial output"}`

		var event claudeCodeStreamEvent
		json.Unmarshal([]byte(jsonStr), &event)

		if event.Type != "assistant" {
			t.Errorf("Type = %q", event.Type)
		}
		if event.Content != "partial output" {
			t.Errorf("Content = %q", event.Content)
		}
	})

	t.Run("result event with usage", func(t *testing.T) {
		jsonStr := `{"type":"result","result":"final output","usage":{"input_tokens":500,"output_tokens":200}}`

		var event claudeCodeStreamEvent
		json.Unmarshal([]byte(jsonStr), &event)

		if event.Type != "result" {
			t.Errorf("Type = %q", event.Type)
		}
		if event.Usage == nil || event.Usage.InputTokens != 500 {
			t.Error("Usage not parsed correctly")
		}
	})

	t.Run("error event", func(t *testing.T) {
		jsonStr := `{"type":"error","error":"rate limited"}`

		var event claudeCodeStreamEvent
		json.Unmarshal([]byte(jsonStr), &event)

		if event.Error != "rate limited" {
			t.Errorf("Error = %q", event.Error)
		}
	})
}
