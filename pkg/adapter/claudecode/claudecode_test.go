package claudecode

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	v1 "github.com/zlc-ai/opc-platform/api/v1"
	"github.com/zlc-ai/opc-platform/pkg/adapter"
)

// --- Helper: fake claude binary ---

// writeFakeClaude creates a shell script at dir/claude that outputs the given
// string to stdout (or writes to stderr and exits 1 if wantErr is true).
func writeFakeClaude(t *testing.T, dir, stdout string, wantErr bool) {
	t.Helper()
	var script string
	if wantErr {
		script = fmt.Sprintf("#!/bin/sh\necho '%s' >&2\nexit 1\n", stdout)
	} else {
		script = fmt.Sprintf("#!/bin/sh\necho '%s'\n", stdout)
	}
	path := filepath.Join(dir, "claude")
	if err := os.WriteFile(path, []byte(script), 0o755); err != nil {
		t.Fatalf("write fake claude: %v", err)
	}
}

// writeFakeClaudeMultiline creates a fake claude that outputs multiple lines (for streaming).
func writeFakeClaudeMultiline(t *testing.T, dir string, lines []string) {
	t.Helper()
	var sb strings.Builder
	sb.WriteString("#!/bin/sh\n")
	for _, line := range lines {
		sb.WriteString(fmt.Sprintf("echo '%s'\n", line))
	}
	path := filepath.Join(dir, "claude")
	if err := os.WriteFile(path, []byte(sb.String()), 0o755); err != nil {
		t.Fatalf("write fake claude: %v", err)
	}
}

// prependPath returns a modified PATH with dir prepended.
func prependPath(dir string) string {
	return dir + ":" + os.Getenv("PATH")
}

// newRunningAdapter creates an adapter in Running state with the given workdir
// and prepended PATH so the fake claude binary is found.
func newRunningAdapter(t *testing.T, fakeBinDir, workdir string) *Adapter {
	t.Helper()
	a := New().(*Adapter)
	spec := v1.AgentSpec{
		Metadata: v1.Metadata{Name: "test-agent"},
		Spec: v1.AgentSpecBody{
			Type:    v1.AgentTypeClaudeCode,
			Context: v1.ContextConfig{Workdir: workdir},
		},
	}
	if err := a.Start(context.Background(), spec); err != nil {
		t.Fatalf("Start: %v", err)
	}
	// Override PATH so exec finds our fake claude.
	t.Setenv("PATH", prependPath(fakeBinDir))
	return a
}

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

	t.Run("sets startAt", func(t *testing.T) {
		a := New().(*Adapter)
		before := time.Now()
		a.Start(context.Background(), v1.AgentSpec{
			Spec: v1.AgentSpecBody{Type: v1.AgentTypeClaudeCode},
		})
		if a.startAt.Before(before) {
			t.Error("startAt should be >= test start time")
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

	t.Run("stops active process that already exited", func(t *testing.T) {
		a := New().(*Adapter)
		a.Start(context.Background(), v1.AgentSpec{
			Spec: v1.AgentSpecBody{Type: v1.AgentTypeClaudeCode},
		})

		// Start a process that exits immediately.
		cmd := exec.Command("true")
		if err := cmd.Start(); err != nil {
			t.Fatalf("start true: %v", err)
		}
		// Wait for process to finish before assigning as activeCmd.
		// This tests the "process already exited" path.
		cmd.Wait()

		a.mu.Lock()
		a.activeCmd = cmd
		a.mu.Unlock()

		err := a.Stop(context.Background())
		if err != nil {
			t.Fatalf("Stop: %v", err)
		}
		if a.Status() != v1.AgentPhaseStopped {
			t.Errorf("phase = %q, want Stopped", a.Status())
		}
	})

	t.Run("stops running process via context cancel", func(t *testing.T) {
		a := New().(*Adapter)
		a.Start(context.Background(), v1.AgentSpec{
			Spec: v1.AgentSpecBody{Type: v1.AgentTypeClaudeCode},
		})

		// Start a process that will block.
		cmd := exec.Command("sleep", "60")
		if err := cmd.Start(); err != nil {
			t.Fatalf("start sleep: %v", err)
		}
		a.mu.Lock()
		a.activeCmd = cmd
		a.mu.Unlock()

		// Use a short timeout so ctx.Done fires and the process gets killed.
		ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
		defer cancel()

		err := a.Stop(ctx)
		if err == nil {
			t.Fatal("expected context error")
		}
		if a.Status() != v1.AgentPhaseTerminated {
			t.Errorf("phase = %q, want Terminated", a.Status())
		}
	})

	t.Run("handles already-cancelled context during stop", func(t *testing.T) {
		a := New().(*Adapter)
		a.Start(context.Background(), v1.AgentSpec{
			Spec: v1.AgentSpecBody{Type: v1.AgentTypeClaudeCode},
		})

		// Start a process that will block.
		cmd := exec.Command("sleep", "60")
		if err := cmd.Start(); err != nil {
			t.Fatalf("start sleep: %v", err)
		}
		a.mu.Lock()
		a.activeCmd = cmd
		a.mu.Unlock()

		// Use an already-cancelled context.
		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		err := a.Stop(ctx)
		if err == nil {
			t.Fatal("expected context error")
		}
		if a.Status() != v1.AgentPhaseTerminated {
			t.Errorf("phase = %q, want Terminated", a.Status())
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
		if !strings.Contains(h.Message, "not running") {
			t.Errorf("message = %q, want contains 'not running'", h.Message)
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
		{
			name: "model and maxTokens together",
			spec: v1.AgentSpec{
				Spec: v1.AgentSpecBody{
					Runtime: v1.RuntimeConfig{
						Model:     v1.ModelConfig{Name: "claude-opus-4-6"},
						Inference: v1.InferenceConfig{MaxTokens: 50},
					},
				},
			},
			contains: []string{"--model", "claude-opus-4-6", "--max-turns", "50"},
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
		{"claude-sonnet-4.5", "claude-sonnet-4-5-20250514"},
		{"claude-haiku-4-5", "claude-haiku-4-5-20251001"},
		{"claude-haiku-4.5", "claude-haiku-4-5-20251001"},
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

func TestClaudeModelMap_AllEntriesCovered(t *testing.T) {
	// Ensure all entries in the map are tested
	if len(claudeModelMap) != 13 {
		t.Errorf("expected 13 entries in claudeModelMap, got %d — update tests if entries were added", len(claudeModelMap))
	}
}

// --- Execute tests ---

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

func TestExecute_SuccessWithUsageField(t *testing.T) {
	fakeBinDir := t.TempDir()
	workdir := t.TempDir()

	result := claudeCodeResult{
		Type:         "result",
		Result:       "task completed",
		TotalCostUSD: 0.05,
		Usage: &claudeCodeUsage{
			InputTokens:              100,
			OutputTokens:             50,
			CacheCreationInputTokens: 10,
			CacheReadInputTokens:     5,
		},
	}
	jsonBytes, _ := json.Marshal(result)
	writeFakeClaude(t, fakeBinDir, string(jsonBytes), false)

	a := newRunningAdapter(t, fakeBinDir, workdir)
	task := v1.TaskRecord{ID: "t1", Message: "do something"}

	res, err := a.Execute(context.Background(), task)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}

	if res.Output != "task completed" {
		t.Errorf("Output = %q, want 'task completed'", res.Output)
	}
	if res.Cost != 0.05 {
		t.Errorf("Cost = %f, want 0.05", res.Cost)
	}
	if res.TokensIn != 115 { // 100 + 10 + 5
		t.Errorf("TokensIn = %d, want 115", res.TokensIn)
	}
	if res.TokensOut != 50 {
		t.Errorf("TokensOut = %d, want 50", res.TokensOut)
	}

	// Verify metrics updated.
	m := a.Metrics()
	if m.TasksCompleted != 1 {
		t.Errorf("TasksCompleted = %d, want 1", m.TasksCompleted)
	}
}

func TestExecute_SuccessWithModelUsage(t *testing.T) {
	fakeBinDir := t.TempDir()
	workdir := t.TempDir()

	result := claudeCodeResult{
		Type:   "result",
		Result: "done with model usage",
		ModelUsage: map[string]modelUsage{
			"claude-sonnet-4-6": {
				InputTokens:  200,
				OutputTokens: 100,
				CacheReadInputTokens: 20,
				CostUSD:      0.03,
			},
			"claude-haiku-4-5": {
				InputTokens:  50,
				OutputTokens: 30,
			},
		},
	}
	jsonBytes, _ := json.Marshal(result)
	writeFakeClaude(t, fakeBinDir, string(jsonBytes), false)

	a := newRunningAdapter(t, fakeBinDir, workdir)
	task := v1.TaskRecord{ID: "t2", Message: "multi model"}

	res, err := a.Execute(context.Background(), task)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}

	if res.Output != "done with model usage" {
		t.Errorf("Output = %q", res.Output)
	}
	// 200+20 + 50 = 270
	if res.TokensIn != 270 {
		t.Errorf("TokensIn = %d, want 270", res.TokensIn)
	}
	// 100 + 30 = 130
	if res.TokensOut != 130 {
		t.Errorf("TokensOut = %d, want 130", res.TokensOut)
	}
}

func TestExecute_SuccessWithTopLevelTokens(t *testing.T) {
	fakeBinDir := t.TempDir()
	workdir := t.TempDir()

	result := claudeCodeResult{
		Type:      "result",
		Result:    "basic output",
		TokensIn:  300,
		TokensOut: 150,
	}
	jsonBytes, _ := json.Marshal(result)
	writeFakeClaude(t, fakeBinDir, string(jsonBytes), false)

	a := newRunningAdapter(t, fakeBinDir, workdir)
	task := v1.TaskRecord{ID: "t3", Message: "basic"}

	res, err := a.Execute(context.Background(), task)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}

	if res.TokensIn != 300 {
		t.Errorf("TokensIn = %d, want 300", res.TokensIn)
	}
	if res.TokensOut != 150 {
		t.Errorf("TokensOut = %d, want 150", res.TokensOut)
	}
}

func TestExecute_ContentFallback(t *testing.T) {
	fakeBinDir := t.TempDir()
	workdir := t.TempDir()

	result := claudeCodeResult{
		Type:    "result",
		Content: "content fallback",
	}
	jsonBytes, _ := json.Marshal(result)
	writeFakeClaude(t, fakeBinDir, string(jsonBytes), false)

	a := newRunningAdapter(t, fakeBinDir, workdir)
	task := v1.TaskRecord{ID: "t4", Message: "test"}

	res, err := a.Execute(context.Background(), task)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}

	if res.Output != "content fallback" {
		t.Errorf("Output = %q, want 'content fallback'", res.Output)
	}
}

func TestExecute_ClaudeError(t *testing.T) {
	fakeBinDir := t.TempDir()
	workdir := t.TempDir()

	result := claudeCodeResult{
		Type:    "result",
		IsError: true,
		Error:   "something went wrong",
	}
	jsonBytes, _ := json.Marshal(result)
	writeFakeClaude(t, fakeBinDir, string(jsonBytes), false)

	a := newRunningAdapter(t, fakeBinDir, workdir)
	task := v1.TaskRecord{ID: "t5", Message: "fail"}

	_, err := a.Execute(context.Background(), task)
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "something went wrong") {
		t.Errorf("error = %q, want 'something went wrong'", err.Error())
	}

	m := a.Metrics()
	if m.TasksFailed != 1 {
		t.Errorf("TasksFailed = %d, want 1", m.TasksFailed)
	}
}

func TestExecute_ClaudeErrorResultFallback(t *testing.T) {
	fakeBinDir := t.TempDir()
	workdir := t.TempDir()

	// IsError true but Error is empty, should fall back to Result.
	result := claudeCodeResult{
		Type:    "result",
		IsError: true,
		Result:  "error in result field",
	}
	jsonBytes, _ := json.Marshal(result)
	writeFakeClaude(t, fakeBinDir, string(jsonBytes), false)

	a := newRunningAdapter(t, fakeBinDir, workdir)
	task := v1.TaskRecord{ID: "t6", Message: "fail2"}

	_, err := a.Execute(context.Background(), task)
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "error in result field") {
		t.Errorf("error = %q", err.Error())
	}
}

func TestExecute_ProcessFailure(t *testing.T) {
	fakeBinDir := t.TempDir()
	workdir := t.TempDir()

	writeFakeClaude(t, fakeBinDir, "process error message", true)

	a := newRunningAdapter(t, fakeBinDir, workdir)
	task := v1.TaskRecord{ID: "t7", Message: "fail3"}

	_, err := a.Execute(context.Background(), task)
	if err == nil {
		t.Fatal("expected error on process failure")
	}
	if !strings.Contains(err.Error(), "claude execute") {
		t.Errorf("error = %q, want contains 'claude execute'", err.Error())
	}

	m := a.Metrics()
	if m.TasksFailed != 1 {
		t.Errorf("TasksFailed = %d, want 1", m.TasksFailed)
	}
}

func TestExecute_NonJSONOutput(t *testing.T) {
	fakeBinDir := t.TempDir()
	workdir := t.TempDir()

	writeFakeClaude(t, fakeBinDir, "plain text output", false)

	a := newRunningAdapter(t, fakeBinDir, workdir)
	task := v1.TaskRecord{ID: "t8", Message: "plain"}

	res, err := a.Execute(context.Background(), task)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}

	// Should fall back to raw text.
	if !strings.Contains(res.Output, "plain text output") {
		t.Errorf("Output = %q, want contains 'plain text output'", res.Output)
	}
}

func TestExecute_DefaultWorkdir(t *testing.T) {
	fakeBinDir := t.TempDir()

	result := claudeCodeResult{Type: "result", Result: "ok"}
	jsonBytes, _ := json.Marshal(result)
	writeFakeClaude(t, fakeBinDir, string(jsonBytes), false)

	// Create adapter without workdir in spec.
	a := New().(*Adapter)
	spec := v1.AgentSpec{
		Metadata: v1.Metadata{Name: "test-agent"},
		Spec:     v1.AgentSpecBody{Type: v1.AgentTypeClaudeCode},
	}
	a.Start(context.Background(), spec)
	t.Setenv("PATH", prependPath(fakeBinDir))

	task := v1.TaskRecord{ID: "t9", Message: "default dir"}
	res, err := a.Execute(context.Background(), task)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if res.Output != "ok" {
		t.Errorf("Output = %q, want 'ok'", res.Output)
	}
}

func TestExecute_WithWorkdirSpec(t *testing.T) {
	fakeBinDir := t.TempDir()
	workdir := t.TempDir()

	// Ensure Execute uses the spec workdir.
	result := claudeCodeResult{Type: "result", Result: "workdir ok"}
	jsonBytes, _ := json.Marshal(result)
	writeFakeClaude(t, fakeBinDir, string(jsonBytes), false)

	a := New().(*Adapter)
	spec := v1.AgentSpec{
		Metadata: v1.Metadata{Name: "wd-agent"},
		Spec: v1.AgentSpecBody{
			Type:    v1.AgentTypeClaudeCode,
			Context: v1.ContextConfig{Workdir: workdir},
			Runtime: v1.RuntimeConfig{
				Model:     v1.ModelConfig{Name: "claude-opus-4.5"},
				Inference: v1.InferenceConfig{MaxTokens: 10},
			},
		},
	}
	a.Start(context.Background(), spec)
	t.Setenv("PATH", prependPath(fakeBinDir))

	task := v1.TaskRecord{ID: "t-wd", Message: "test workdir"}
	res, err := a.Execute(context.Background(), task)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if res.Output != "workdir ok" {
		t.Errorf("Output = %q", res.Output)
	}
}

// --- Stream tests ---

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

func TestStream_Success(t *testing.T) {
	fakeBinDir := t.TempDir()
	workdir := t.TempDir()

	lines := []string{
		`{"type":"assistant","content":"partial "}`,
		`{"type":"assistant","content":"output"}`,
		`{"type":"result","result":"final output","usage":{"input_tokens":500,"output_tokens":200}}`,
	}
	writeFakeClaudeMultiline(t, fakeBinDir, lines)

	a := newRunningAdapter(t, fakeBinDir, workdir)
	task := v1.TaskRecord{ID: "s1", Message: "stream me"}

	ch, err := a.Stream(context.Background(), task)
	if err != nil {
		t.Fatalf("Stream: %v", err)
	}

	var chunks []adapter.Chunk
	for chunk := range ch {
		chunks = append(chunks, chunk)
	}

	if len(chunks) < 2 {
		t.Fatalf("expected at least 2 chunks, got %d", len(chunks))
	}

	// Last chunk should be done.
	last := chunks[len(chunks)-1]
	if !last.Done {
		t.Error("last chunk should be Done")
	}
	if last.Content != "final output" {
		t.Errorf("last Content = %q, want 'final output'", last.Content)
	}

	// First chunks should not be done.
	if chunks[0].Done {
		t.Error("first chunk should not be Done")
	}
	if chunks[0].Content != "partial " {
		t.Errorf("first Content = %q, want 'partial '", chunks[0].Content)
	}

	// Check metrics.
	m := a.Metrics()
	if m.TasksCompleted != 1 {
		t.Errorf("TasksCompleted = %d, want 1", m.TasksCompleted)
	}
	if m.TotalTokensIn != 500 {
		t.Errorf("TotalTokensIn = %d, want 500", m.TotalTokensIn)
	}
	if m.TotalTokensOut != 200 {
		t.Errorf("TotalTokensOut = %d, want 200", m.TotalTokensOut)
	}
}

func TestStream_ErrorEvent(t *testing.T) {
	fakeBinDir := t.TempDir()
	workdir := t.TempDir()

	lines := []string{
		`{"type":"assistant","content":"starting"}`,
		`{"type":"error","error":"rate limited"}`,
	}
	writeFakeClaudeMultiline(t, fakeBinDir, lines)

	a := newRunningAdapter(t, fakeBinDir, workdir)
	task := v1.TaskRecord{ID: "s2", Message: "stream error"}

	ch, err := a.Stream(context.Background(), task)
	if err != nil {
		t.Fatalf("Stream: %v", err)
	}

	var chunks []adapter.Chunk
	for chunk := range ch {
		chunks = append(chunks, chunk)
	}

	// Should have at least the error chunk.
	var foundError bool
	for _, c := range chunks {
		if c.Error != nil {
			foundError = true
			if !strings.Contains(c.Error.Error(), "rate limited") {
				t.Errorf("error = %q, want 'rate limited'", c.Error.Error())
			}
		}
	}
	if !foundError {
		t.Error("expected error chunk")
	}

	m := a.Metrics()
	if m.TasksFailed != 1 {
		t.Errorf("TasksFailed = %d, want 1", m.TasksFailed)
	}
}

func TestStream_ResultWithContentFallback(t *testing.T) {
	fakeBinDir := t.TempDir()
	workdir := t.TempDir()

	// Result event with content but empty result field.
	lines := []string{
		`{"type":"result","content":"content only"}`,
	}
	writeFakeClaudeMultiline(t, fakeBinDir, lines)

	a := newRunningAdapter(t, fakeBinDir, workdir)
	task := v1.TaskRecord{ID: "s3", Message: "content fallback"}

	ch, err := a.Stream(context.Background(), task)
	if err != nil {
		t.Fatalf("Stream: %v", err)
	}

	var chunks []adapter.Chunk
	for chunk := range ch {
		chunks = append(chunks, chunk)
	}

	if len(chunks) != 1 {
		t.Fatalf("expected 1 chunk, got %d", len(chunks))
	}
	if chunks[0].Content != "content only" {
		t.Errorf("Content = %q, want 'content only'", chunks[0].Content)
	}
}

func TestStream_DefaultWorkdir(t *testing.T) {
	fakeBinDir := t.TempDir()

	lines := []string{
		`{"type":"result","result":"ok"}`,
	}
	writeFakeClaudeMultiline(t, fakeBinDir, lines)

	// Create adapter without workdir.
	a := New().(*Adapter)
	spec := v1.AgentSpec{
		Metadata: v1.Metadata{Name: "test-agent"},
		Spec:     v1.AgentSpecBody{Type: v1.AgentTypeClaudeCode},
	}
	a.Start(context.Background(), spec)
	t.Setenv("PATH", prependPath(fakeBinDir))

	task := v1.TaskRecord{ID: "s4", Message: "default"}
	ch, err := a.Stream(context.Background(), task)
	if err != nil {
		t.Fatalf("Stream: %v", err)
	}

	for range ch {
		// Drain.
	}
}

func TestStream_InvalidJSON(t *testing.T) {
	fakeBinDir := t.TempDir()
	workdir := t.TempDir()

	// Mix valid and invalid JSON lines.
	lines := []string{
		`not json at all`,
		`{"type":"result","result":"after invalid"}`,
	}
	writeFakeClaudeMultiline(t, fakeBinDir, lines)

	a := newRunningAdapter(t, fakeBinDir, workdir)
	task := v1.TaskRecord{ID: "s5", Message: "invalid json"}

	ch, err := a.Stream(context.Background(), task)
	if err != nil {
		t.Fatalf("Stream: %v", err)
	}

	var chunks []adapter.Chunk
	for chunk := range ch {
		chunks = append(chunks, chunk)
	}

	// The invalid JSON line should be skipped, result should come through.
	if len(chunks) != 1 {
		t.Fatalf("expected 1 chunk (skipping invalid), got %d", len(chunks))
	}
	if chunks[0].Content != "after invalid" {
		t.Errorf("Content = %q", chunks[0].Content)
	}
}

func TestStream_EmptyLines(t *testing.T) {
	fakeBinDir := t.TempDir()
	workdir := t.TempDir()

	// The script writes empty lines between events.
	script := "#!/bin/sh\necho ''\necho '{\"type\":\"result\",\"result\":\"ok\"}'\necho ''\n"
	if err := os.WriteFile(filepath.Join(fakeBinDir, "claude"), []byte(script), 0o755); err != nil {
		t.Fatalf("write: %v", err)
	}

	a := newRunningAdapter(t, fakeBinDir, workdir)
	task := v1.TaskRecord{ID: "s6", Message: "empty lines"}

	ch, err := a.Stream(context.Background(), task)
	if err != nil {
		t.Fatalf("Stream: %v", err)
	}

	var chunks []adapter.Chunk
	for chunk := range ch {
		chunks = append(chunks, chunk)
	}

	if len(chunks) != 1 {
		t.Fatalf("expected 1 chunk, got %d", len(chunks))
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

func TestMetrics_NoUptime(t *testing.T) {
	a := New().(*Adapter)
	m := a.Metrics()
	if m.UptimeSeconds != 0 {
		t.Errorf("UptimeSeconds = %f, want 0 before start", m.UptimeSeconds)
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
