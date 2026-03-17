package controller

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	v1 "github.com/zlc-ai/opc-platform/api/v1"
	"github.com/zlc-ai/opc-platform/pkg/adapter"
	"github.com/zlc-ai/opc-platform/pkg/storage/sqlite"
	"go.uber.org/zap"
)

// ---------------------------------------------------------------------------
// Mock Adapter
// ---------------------------------------------------------------------------

type mockAdapter struct {
	mu            sync.Mutex
	agentType     v1.AgentType
	started       bool
	stopped       bool
	startCount    int
	stopCount     int
	executeCount  int
	healthStatus  v1.HealthStatus
	phase         v1.AgentPhase
	metrics       v1.AgentMetrics
	startErr      error
	stopErr       error
	executeResult adapter.ExecuteResult
	executeErr    error
	streamChunks  []adapter.Chunk
	streamErr     error
}

func newMockAdapter(agentType v1.AgentType) *mockAdapter {
	return &mockAdapter{
		agentType: agentType,
		healthStatus: v1.HealthStatus{
			Healthy: true,
			Message: "ok",
		},
		phase: v1.AgentPhaseRunning,
	}
}

func (m *mockAdapter) Type() v1.AgentType {
	return m.agentType
}

func (m *mockAdapter) Start(_ context.Context, _ v1.AgentSpec) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.started = true
	m.startCount++
	return m.startErr
}

func (m *mockAdapter) Stop(_ context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.stopped = true
	m.stopCount++
	return m.stopErr
}

func (m *mockAdapter) Health() v1.HealthStatus {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.healthStatus
}

func (m *mockAdapter) Execute(_ context.Context, _ v1.TaskRecord) (adapter.ExecuteResult, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.executeCount++
	return m.executeResult, m.executeErr
}

func (m *mockAdapter) Stream(_ context.Context, _ v1.TaskRecord) (<-chan adapter.Chunk, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.streamErr != nil {
		return nil, m.streamErr
	}
	ch := make(chan adapter.Chunk, len(m.streamChunks)+1)
	for _, c := range m.streamChunks {
		ch <- c
	}
	ch <- adapter.Chunk{Done: true}
	close(ch)
	return ch, nil
}

func (m *mockAdapter) Status() v1.AgentPhase {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.phase
}

func (m *mockAdapter) Metrics() v1.AgentMetrics {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.metrics
}

func (m *mockAdapter) setHealth(healthy bool, msg string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.healthStatus = v1.HealthStatus{Healthy: healthy, Message: msg}
}

func (m *mockAdapter) setExecuteResult(output string, tokensIn, tokensOut int) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.executeResult = adapter.ExecuteResult{
		Output:    output,
		TokensIn:  tokensIn,
		TokensOut: tokensOut,
	}
	m.executeErr = nil
}

// ---------------------------------------------------------------------------
// Test Helpers
// ---------------------------------------------------------------------------

// setupTest creates a temp dir, SQLite store, registry with a mock adapter
// factory, and a Controller. It also overrides HOME so that config.GetConfigDir
// points to a temp location for checkpoint/crash file tests.
func setupTest(t *testing.T) (*Controller, *mockAdapter, func()) {
	t.Helper()

	tmpDir := t.TempDir()

	// Override HOME so config.GetConfigDir() returns tmpDir/.opc
	origHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)

	dbPath := filepath.Join(tmpDir, "test.db")
	store, err := sqlite.New(dbPath)
	if err != nil {
		t.Fatalf("create sqlite store: %v", err)
	}

	var latestMock *mockAdapter
	registry := adapter.NewRegistry()
	registry.Register(v1.AgentTypeClaudeCode, func() adapter.Adapter {
		m := newMockAdapter(v1.AgentTypeClaudeCode)
		latestMock = m
		return m
	})

	logger := zap.NewNop().Sugar()
	ctrl := New(store, registry, logger)

	cleanup := func() {
		store.Close()
		os.Setenv("HOME", origHome)
	}

	return ctrl, latestMock, cleanup
}

// testSpec returns a minimal AgentSpec for testing.
func testSpec(name string) v1.AgentSpec {
	return v1.AgentSpec{
		APIVersion: v1.APIVersion,
		Kind:       v1.KindAgentSpec,
		Metadata:   v1.Metadata{Name: name},
		Spec: v1.AgentSpecBody{
			Type: v1.AgentTypeClaudeCode,
		},
	}
}

// applyAndStart is a helper that applies a spec and starts the agent.
func applyAndStart(t *testing.T, ctrl *Controller, name string) {
	t.Helper()
	ctx := context.Background()
	if err := ctrl.Apply(ctx, testSpec(name)); err != nil {
		t.Fatalf("apply: %v", err)
	}
	if err := ctrl.StartAgent(ctx, name); err != nil {
		t.Fatalf("start: %v", err)
	}
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

func TestApply(t *testing.T) {
	ctrl, _, cleanup := setupTest(t)
	defer cleanup()

	ctx := context.Background()

	t.Run("create new agent", func(t *testing.T) {
		spec := testSpec("agent-1")
		if err := ctrl.Apply(ctx, spec); err != nil {
			t.Fatalf("Apply() error = %v", err)
		}

		record, err := ctrl.GetAgent(ctx, "agent-1")
		if err != nil {
			t.Fatalf("GetAgent() error = %v", err)
		}
		if record.Name != "agent-1" {
			t.Errorf("name = %q, want %q", record.Name, "agent-1")
		}
		if record.Phase != v1.AgentPhaseCreated {
			t.Errorf("phase = %q, want %q", record.Phase, v1.AgentPhaseCreated)
		}
		if record.Type != v1.AgentTypeClaudeCode {
			t.Errorf("type = %q, want %q", record.Type, v1.AgentTypeClaudeCode)
		}
	})

	t.Run("update existing agent", func(t *testing.T) {
		spec := testSpec("agent-1")
		spec.Spec.Replicas = 3
		if err := ctrl.Apply(ctx, spec); err != nil {
			t.Fatalf("Apply() update error = %v", err)
		}

		record, err := ctrl.GetAgent(ctx, "agent-1")
		if err != nil {
			t.Fatalf("GetAgent() error = %v", err)
		}
		// Verify it's still the same agent (not duplicated).
		if record.Name != "agent-1" {
			t.Errorf("name = %q, want %q", record.Name, "agent-1")
		}
	})

	t.Run("empty name returns error", func(t *testing.T) {
		spec := testSpec("")
		err := ctrl.Apply(ctx, spec)
		if err == nil {
			t.Fatal("Apply() with empty name should return error")
		}
	})
}

func TestStartStopAgent(t *testing.T) {
	ctrl, _, cleanup := setupTest(t)
	defer cleanup()

	ctx := context.Background()

	t.Run("start agent", func(t *testing.T) {
		if err := ctrl.Apply(ctx, testSpec("runner")); err != nil {
			t.Fatalf("Apply() error = %v", err)
		}

		if err := ctrl.StartAgent(ctx, "runner"); err != nil {
			t.Fatalf("StartAgent() error = %v", err)
		}

		// Verify phase is Running.
		record, err := ctrl.GetAgent(ctx, "runner")
		if err != nil {
			t.Fatalf("GetAgent() error = %v", err)
		}
		if record.Phase != v1.AgentPhaseRunning {
			t.Errorf("phase = %q, want %q", record.Phase, v1.AgentPhaseRunning)
		}

		// Verify adapter is accessible.
		adp, err := ctrl.GetAdapter("runner")
		if err != nil {
			t.Fatalf("GetAdapter() error = %v", err)
		}
		if adp == nil {
			t.Fatal("adapter should not be nil")
		}
	})

	t.Run("stop agent", func(t *testing.T) {
		if err := ctrl.StopAgent(ctx, "runner"); err != nil {
			t.Fatalf("StopAgent() error = %v", err)
		}

		// Verify phase is Stopped.
		record, err := ctrl.GetAgent(ctx, "runner")
		if err != nil {
			t.Fatalf("GetAgent() error = %v", err)
		}
		if record.Phase != v1.AgentPhaseStopped {
			t.Errorf("phase = %q, want %q", record.Phase, v1.AgentPhaseStopped)
		}

		// Verify adapter is no longer accessible.
		_, err = ctrl.GetAdapter("runner")
		if err == nil {
			t.Fatal("GetAdapter() should return error for stopped agent")
		}
	})

	t.Run("stop non-running agent returns error", func(t *testing.T) {
		err := ctrl.StopAgent(ctx, "runner")
		if err == nil {
			t.Fatal("StopAgent() on stopped agent should return error")
		}
	})

	t.Run("start non-existent agent returns error", func(t *testing.T) {
		err := ctrl.StartAgent(ctx, "nonexistent")
		if err == nil {
			t.Fatal("StartAgent() on nonexistent agent should return error")
		}
	})
}

func TestDeleteAgent(t *testing.T) {
	ctrl, _, cleanup := setupTest(t)
	defer cleanup()

	ctx := context.Background()

	t.Run("delete non-running agent", func(t *testing.T) {
		if err := ctrl.Apply(ctx, testSpec("to-delete")); err != nil {
			t.Fatalf("Apply() error = %v", err)
		}

		if err := ctrl.DeleteAgent(ctx, "to-delete"); err != nil {
			t.Fatalf("DeleteAgent() error = %v", err)
		}

		_, err := ctrl.GetAgent(ctx, "to-delete")
		if err == nil {
			t.Fatal("GetAgent() should fail after delete")
		}
	})

	t.Run("delete running agent stops it first", func(t *testing.T) {
		applyAndStart(t, ctrl, "running-delete")

		// Confirm it is running.
		_, err := ctrl.GetAdapter("running-delete")
		if err != nil {
			t.Fatalf("agent should be running: %v", err)
		}

		if err := ctrl.DeleteAgent(ctx, "running-delete"); err != nil {
			t.Fatalf("DeleteAgent() error = %v", err)
		}

		_, err = ctrl.GetAdapter("running-delete")
		if err == nil {
			t.Fatal("agent adapter should be gone after delete")
		}

		_, err = ctrl.GetAgent(ctx, "running-delete")
		if err == nil {
			t.Fatal("agent record should be gone after delete")
		}
	})

	t.Run("delete nonexistent agent returns error", func(t *testing.T) {
		err := ctrl.DeleteAgent(ctx, "ghost")
		if err == nil {
			t.Fatal("DeleteAgent() on nonexistent agent should return error")
		}
	})
}

func TestRestartAgent(t *testing.T) {
	ctrl, _, cleanup := setupTest(t)
	defer cleanup()

	ctx := context.Background()

	t.Run("restart running agent", func(t *testing.T) {
		applyAndStart(t, ctrl, "restartable")

		if err := ctrl.RestartAgent(ctx, "restartable"); err != nil {
			t.Fatalf("RestartAgent() error = %v", err)
		}

		// Verify agent is still running after restart.
		record, err := ctrl.GetAgent(ctx, "restartable")
		if err != nil {
			t.Fatalf("GetAgent() error = %v", err)
		}
		if record.Phase != v1.AgentPhaseRunning {
			t.Errorf("phase = %q, want %q", record.Phase, v1.AgentPhaseRunning)
		}

		// Verify restart count was reset.
		count := ctrl.GetRestartCount("restartable")
		if count != 0 {
			t.Errorf("restart count = %d, want 0", count)
		}
	})

	t.Run("restart non-running agent starts it", func(t *testing.T) {
		if err := ctrl.Apply(ctx, testSpec("not-started")); err != nil {
			t.Fatalf("Apply() error = %v", err)
		}

		if err := ctrl.RestartAgent(ctx, "not-started"); err != nil {
			t.Fatalf("RestartAgent() error = %v", err)
		}

		record, err := ctrl.GetAgent(ctx, "not-started")
		if err != nil {
			t.Fatalf("GetAgent() error = %v", err)
		}
		if record.Phase != v1.AgentPhaseRunning {
			t.Errorf("phase = %q, want %q", record.Phase, v1.AgentPhaseRunning)
		}
	})
}

func TestExecuteTask(t *testing.T) {
	ctrl, _, cleanup := setupTest(t)
	defer cleanup()

	ctx := context.Background()
	applyAndStart(t, ctrl, "exec-agent")

	// Configure the mock adapter with a result.
	adp, _ := ctrl.GetAdapter("exec-agent")
	mock := adp.(*mockAdapter)
	mock.setExecuteResult("hello world", 100, 50)

	// Create a task record in the store first.
	task := v1.TaskRecord{
		ID:        "task-1",
		AgentName: "exec-agent",
		Message:   "say hello",
		Status:    v1.TaskStatusPending,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	if err := ctrl.Store().CreateTask(ctx, task); err != nil {
		t.Fatalf("CreateTask() error = %v", err)
	}

	t.Run("execute task successfully", func(t *testing.T) {
		result, err := ctrl.ExecuteTask(ctx, task)
		if err != nil {
			t.Fatalf("ExecuteTask() error = %v", err)
		}
		if result.Output != "hello world" {
			t.Errorf("output = %q, want %q", result.Output, "hello world")
		}
		if result.TokensIn != 100 {
			t.Errorf("tokensIn = %d, want 100", result.TokensIn)
		}
		if result.TokensOut != 50 {
			t.Errorf("tokensOut = %d, want 50", result.TokensOut)
		}

		// Verify task status updated in store.
		stored, err := ctrl.Store().GetTask(ctx, "task-1")
		if err != nil {
			t.Fatalf("GetTask() error = %v", err)
		}
		if stored.Status != v1.TaskStatusCompleted {
			t.Errorf("stored status = %q, want %q", stored.Status, v1.TaskStatusCompleted)
		}
	})

	t.Run("execute task on non-running agent marks task Failed", func(t *testing.T) {
		badTask := v1.TaskRecord{
			ID:        "task-bad",
			AgentName: "nonexistent",
			Message:   "should fail",
			Status:    v1.TaskStatusPending,
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}
		if err := ctrl.Store().CreateTask(ctx, badTask); err != nil {
			t.Fatalf("CreateTask() error = %v", err)
		}

		_, err := ctrl.ExecuteTask(ctx, badTask)
		if err == nil {
			t.Fatal("ExecuteTask() on non-running agent should return error")
		}

		// Verify task status is Failed (not stuck in Pending).
		stored, getErr := ctrl.Store().GetTask(ctx, "task-bad")
		if getErr != nil {
			t.Fatalf("GetTask() error = %v", getErr)
		}
		if stored.Status != v1.TaskStatusFailed {
			t.Errorf("stored status = %q, want %q", stored.Status, v1.TaskStatusFailed)
		}
		if stored.Error == "" {
			t.Error("stored error should not be empty")
		}
		if stored.EndedAt == nil {
			t.Error("stored endedAt should be set")
		}
	})

	t.Run("execute task with adapter error", func(t *testing.T) {
		mock.mu.Lock()
		mock.executeErr = errors.New("adapter failure")
		mock.mu.Unlock()

		failTask := v1.TaskRecord{
			ID:        "task-fail",
			AgentName: "exec-agent",
			Message:   "will fail",
			Status:    v1.TaskStatusPending,
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}
		if err := ctrl.Store().CreateTask(ctx, failTask); err != nil {
			t.Fatalf("CreateTask() error = %v", err)
		}

		_, err := ctrl.ExecuteTask(ctx, failTask)
		if err == nil {
			t.Fatal("ExecuteTask() should return error when adapter fails")
		}

		stored, err := ctrl.Store().GetTask(ctx, "task-fail")
		if err != nil {
			t.Fatalf("GetTask() error = %v", err)
		}
		if stored.Status != v1.TaskStatusFailed {
			t.Errorf("stored status = %q, want %q", stored.Status, v1.TaskStatusFailed)
		}
	})
}

func TestStreamTask(t *testing.T) {
	ctrl, _, cleanup := setupTest(t)
	defer cleanup()

	ctx := context.Background()
	applyAndStart(t, ctrl, "stream-agent")

	adp, _ := ctrl.GetAdapter("stream-agent")
	mock := adp.(*mockAdapter)
	mock.mu.Lock()
	mock.streamChunks = []adapter.Chunk{
		{Content: "chunk1"},
		{Content: "chunk2"},
	}
	mock.mu.Unlock()

	task := v1.TaskRecord{
		ID:        "stream-task-1",
		AgentName: "stream-agent",
		Message:   "stream me",
		Status:    v1.TaskStatusPending,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	if err := ctrl.Store().CreateTask(ctx, task); err != nil {
		t.Fatalf("CreateTask() error = %v", err)
	}

	ch, err := ctrl.StreamTask(ctx, task)
	if err != nil {
		t.Fatalf("StreamTask() error = %v", err)
	}

	var chunks []adapter.Chunk
	for c := range ch {
		chunks = append(chunks, c)
	}

	if len(chunks) != 3 { // 2 content chunks + 1 done
		t.Fatalf("got %d chunks, want 3", len(chunks))
	}
	if chunks[0].Content != "chunk1" {
		t.Errorf("chunk[0] = %q, want %q", chunks[0].Content, "chunk1")
	}
	if chunks[1].Content != "chunk2" {
		t.Errorf("chunk[1] = %q, want %q", chunks[1].Content, "chunk2")
	}
	if !chunks[2].Done {
		t.Error("last chunk should have Done=true")
	}
}

func TestCalculateBackoff(t *testing.T) {
	base := 5 * time.Second

	tests := []struct {
		name     string
		restarts int
		strategy string
		want     time.Duration
	}{
		{
			name:     "exponential restart 0",
			restarts: 0,
			strategy: "exponential",
			want:     5 * time.Second, // 5 * 2^0 = 5
		},
		{
			name:     "exponential restart 1",
			restarts: 1,
			strategy: "exponential",
			want:     10 * time.Second, // 5 * 2^1 = 10
		},
		{
			name:     "exponential restart 2",
			restarts: 2,
			strategy: "exponential",
			want:     20 * time.Second, // 5 * 2^2 = 20
		},
		{
			name:     "exponential restart 3",
			restarts: 3,
			strategy: "exponential",
			want:     40 * time.Second, // 5 * 2^3 = 40
		},
		{
			name:     "linear restart 0",
			restarts: 0,
			strategy: "linear",
			want:     5 * time.Second, // 5 * (0+1) = 5
		},
		{
			name:     "linear restart 1",
			restarts: 1,
			strategy: "linear",
			want:     10 * time.Second, // 5 * (1+1) = 10
		},
		{
			name:     "linear restart 2",
			restarts: 2,
			strategy: "linear",
			want:     15 * time.Second, // 5 * (2+1) = 15
		},
		{
			name:     "fixed restart 0",
			restarts: 0,
			strategy: "fixed",
			want:     5 * time.Second,
		},
		{
			name:     "fixed restart 5",
			restarts: 5,
			strategy: "fixed",
			want:     5 * time.Second,
		},
		{
			name:     "unknown strategy defaults to exponential",
			restarts: 1,
			strategy: "unknown",
			want:     10 * time.Second,
		},
		{
			name:     "exponential capped at maxBackoffDelay",
			restarts: 20,
			strategy: "exponential",
			want:     maxBackoffDelay,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := calculateBackoff(tt.restarts, base, tt.strategy)
			if got != tt.want {
				t.Errorf("calculateBackoff(%d, %v, %q) = %v, want %v",
					tt.restarts, base, tt.strategy, got, tt.want)
			}
		})
	}
}

func TestParseOrDefault(t *testing.T) {
	def := 30 * time.Second

	tests := []struct {
		name string
		s    string
		want time.Duration
	}{
		{"empty string returns default", "", def},
		{"valid duration", "10s", 10 * time.Second},
		{"valid duration minutes", "2m", 2 * time.Minute},
		{"valid duration millis", "500ms", 500 * time.Millisecond},
		{"invalid string returns default", "notaduration", def},
		{"numeric without unit returns default", "42", def},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseOrDefault(tt.s, def)
			if got != tt.want {
				t.Errorf("parseOrDefault(%q, %v) = %v, want %v", tt.s, def, got, tt.want)
			}
		})
	}
}

func TestCreateCheckpoint(t *testing.T) {
	ctrl, _, cleanup := setupTest(t)
	defer cleanup()

	ctx := context.Background()
	applyAndStart(t, ctrl, "cp-agent")

	t.Run("create checkpoint for running agent", func(t *testing.T) {
		cp, err := ctrl.CreateCheckpoint(ctx, "cp-agent")
		if err != nil {
			t.Fatalf("CreateCheckpoint() error = %v", err)
		}
		if cp.AgentName != "cp-agent" {
			t.Errorf("agentName = %q, want %q", cp.AgentName, "cp-agent")
		}
		if cp.ID == "" {
			t.Error("checkpoint ID should not be empty")
		}
		if cp.Phase != v1.AgentPhaseRunning {
			t.Errorf("phase = %q, want %q", cp.Phase, v1.AgentPhaseRunning)
		}
		if cp.SpecYAML == "" {
			t.Error("specYAML should not be empty")
		}
		if cp.Timestamp.IsZero() {
			t.Error("timestamp should not be zero")
		}
	})

	t.Run("create checkpoint for nonexistent agent fails", func(t *testing.T) {
		_, err := ctrl.CreateCheckpoint(ctx, "nope")
		if err == nil {
			t.Fatal("CreateCheckpoint() for nonexistent agent should return error")
		}
	})
}

func TestListCheckpoints(t *testing.T) {
	ctrl, _, cleanup := setupTest(t)
	defer cleanup()

	ctx := context.Background()
	applyAndStart(t, ctrl, "list-cp-agent")

	// Create multiple checkpoints.
	for i := 0; i < 3; i++ {
		_, err := ctrl.CreateCheckpoint(ctx, "list-cp-agent")
		if err != nil {
			t.Fatalf("CreateCheckpoint() #%d error = %v", i, err)
		}
		// Small sleep to ensure distinct timestamps.
		time.Sleep(5 * time.Millisecond)
	}

	t.Run("list checkpoints returns all, sorted newest first", func(t *testing.T) {
		cps, err := ctrl.ListCheckpoints(ctx, "list-cp-agent")
		if err != nil {
			t.Fatalf("ListCheckpoints() error = %v", err)
		}
		if len(cps) != 3 {
			t.Fatalf("got %d checkpoints, want 3", len(cps))
		}
		// Verify sorted newest first.
		for i := 0; i < len(cps)-1; i++ {
			if cps[i].Timestamp.Before(cps[i+1].Timestamp) {
				t.Errorf("checkpoint %d (%v) is before checkpoint %d (%v); expected newest first",
					i, cps[i].Timestamp, i+1, cps[i+1].Timestamp)
			}
		}
	})

	t.Run("list checkpoints for agent with none returns nil", func(t *testing.T) {
		cps, err := ctrl.ListCheckpoints(ctx, "no-checkpoints")
		if err != nil {
			t.Fatalf("ListCheckpoints() error = %v", err)
		}
		if cps != nil {
			t.Errorf("expected nil, got %v", cps)
		}
	})
}

func TestGetCheckpoint(t *testing.T) {
	ctrl, _, cleanup := setupTest(t)
	defer cleanup()

	ctx := context.Background()
	applyAndStart(t, ctrl, "get-cp-agent")

	cp, err := ctrl.CreateCheckpoint(ctx, "get-cp-agent")
	if err != nil {
		t.Fatalf("CreateCheckpoint() error = %v", err)
	}

	t.Run("get existing checkpoint", func(t *testing.T) {
		found, err := ctrl.GetCheckpoint(ctx, cp.ID)
		if err != nil {
			t.Fatalf("GetCheckpoint() error = %v", err)
		}
		if found.ID != cp.ID {
			t.Errorf("id = %q, want %q", found.ID, cp.ID)
		}
		if found.AgentName != "get-cp-agent" {
			t.Errorf("agentName = %q, want %q", found.AgentName, "get-cp-agent")
		}
	})

	t.Run("get nonexistent checkpoint returns error", func(t *testing.T) {
		_, err := ctrl.GetCheckpoint(ctx, "cp-nonexistent-12345")
		if err == nil {
			t.Fatal("GetCheckpoint() for nonexistent ID should return error")
		}
	})
}

func TestCleanupOldCheckpoints(t *testing.T) {
	ctrl, _, cleanup := setupTest(t)
	defer cleanup()

	ctx := context.Background()
	applyAndStart(t, ctrl, "cleanup-agent")

	// Create 5 checkpoints.
	for i := 0; i < 5; i++ {
		_, err := ctrl.CreateCheckpoint(ctx, "cleanup-agent")
		if err != nil {
			t.Fatalf("CreateCheckpoint() #%d error = %v", i, err)
		}
		time.Sleep(5 * time.Millisecond)
	}

	// Cleanup keeping only 2.
	ctrl.cleanupOldCheckpoints("cleanup-agent", 2)

	cps, err := ctrl.ListCheckpoints(ctx, "cleanup-agent")
	if err != nil {
		t.Fatalf("ListCheckpoints() error = %v", err)
	}
	if len(cps) != 2 {
		t.Errorf("got %d checkpoints after cleanup, want 2", len(cps))
	}
}

func TestSaveCrashReport(t *testing.T) {
	ctrl, _, cleanup := setupTest(t)
	defer cleanup()

	ctx := context.Background()

	t.Run("save and list crash reports", func(t *testing.T) {
		for i := 0; i < 3; i++ {
			err := ctrl.SaveCrashReport(ctx, "crash-agent",
				fmt.Errorf("crash error %d", i))
			if err != nil {
				t.Fatalf("SaveCrashReport() #%d error = %v", i, err)
			}
			time.Sleep(5 * time.Millisecond)
		}

		reports, err := ctrl.ListCrashReports(ctx, "crash-agent")
		if err != nil {
			t.Fatalf("ListCrashReports() error = %v", err)
		}
		if len(reports) != 3 {
			t.Fatalf("got %d reports, want 3", len(reports))
		}

		// Verify sorted newest first.
		for i := 0; i < len(reports)-1; i++ {
			if reports[i].Timestamp.Before(reports[i+1].Timestamp) {
				t.Errorf("report %d is before report %d; expected newest first", i, i+1)
			}
		}

		// Verify content.
		if reports[0].AgentName != "crash-agent" {
			t.Errorf("agentName = %q, want %q", reports[0].AgentName, "crash-agent")
		}
	})

	t.Run("list crash reports for agent with none returns nil", func(t *testing.T) {
		reports, err := ctrl.ListCrashReports(ctx, "no-crashes")
		if err != nil {
			t.Fatalf("ListCrashReports() error = %v", err)
		}
		if reports != nil {
			t.Errorf("expected nil, got %v", reports)
		}
	})
}

func TestRecoverFromCheckpoint(t *testing.T) {
	ctrl, _, cleanup := setupTest(t)
	defer cleanup()

	ctx := context.Background()
	applyAndStart(t, ctrl, "recover-agent")

	// Create a checkpoint while agent is running.
	cp, err := ctrl.CreateCheckpoint(ctx, "recover-agent")
	if err != nil {
		t.Fatalf("CreateCheckpoint() error = %v", err)
	}

	// Stop the agent to simulate failure.
	if err := ctrl.StopAgent(ctx, "recover-agent"); err != nil {
		t.Fatalf("StopAgent() error = %v", err)
	}

	t.Run("recover from specific checkpoint", func(t *testing.T) {
		result, err := ctrl.RecoverFromCheckpoint(ctx, "recover-agent", cp.ID)
		if err != nil {
			t.Fatalf("RecoverFromCheckpoint() error = %v", err)
		}
		if !result.Success {
			t.Fatalf("recovery should succeed, got message: %s", result.Message)
		}
		if result.Source != RecoverySourceCheckpoint {
			t.Errorf("source = %q, want %q", result.Source, RecoverySourceCheckpoint)
		}
		if result.CheckpointID != cp.ID {
			t.Errorf("checkpointID = %q, want %q", result.CheckpointID, cp.ID)
		}
		if result.AgentName != "recover-agent" {
			t.Errorf("agentName = %q, want %q", result.AgentName, "recover-agent")
		}

		// Verify agent is running again.
		record, err := ctrl.GetAgent(ctx, "recover-agent")
		if err != nil {
			t.Fatalf("GetAgent() error = %v", err)
		}
		if record.Phase != v1.AgentPhaseRunning {
			t.Errorf("phase = %q, want %q", record.Phase, v1.AgentPhaseRunning)
		}

		// Clean up: stop the agent.
		ctrl.StopAgent(ctx, "recover-agent")
	})

	t.Run("recover from checkpoint with wrong agent name", func(t *testing.T) {
		_, err := ctrl.RecoverFromCheckpoint(ctx, "wrong-agent", cp.ID)
		if err == nil {
			t.Fatal("RecoverFromCheckpoint() with wrong agent name should error")
		}
	})

	t.Run("recover from nonexistent checkpoint", func(t *testing.T) {
		result, err := ctrl.RecoverFromCheckpoint(ctx, "recover-agent", "cp-fake-999")
		if err == nil {
			t.Fatal("RecoverFromCheckpoint() with bad ID should return error")
		}
		if result == nil {
			t.Fatal("result should not be nil even on failure")
		}
		if result.Success {
			t.Error("result.Success should be false")
		}
	})
}

func TestRecoverFromLatest(t *testing.T) {
	ctrl, _, cleanup := setupTest(t)
	defer cleanup()

	ctx := context.Background()

	t.Run("recover with no checkpoints returns error", func(t *testing.T) {
		if err := ctrl.Apply(ctx, testSpec("no-cp-agent")); err != nil {
			t.Fatalf("Apply() error = %v", err)
		}
		result, err := ctrl.RecoverFromLatest(ctx, "no-cp-agent")
		if err == nil {
			t.Fatal("RecoverFromLatest() with no checkpoints should return error")
		}
		if result == nil {
			t.Fatal("result should not be nil")
		}
		if result.Success {
			t.Error("result.Success should be false")
		}
	})

	t.Run("recover from latest checkpoint", func(t *testing.T) {
		applyAndStart(t, ctrl, "latest-agent")

		// Create two checkpoints.
		_, err := ctrl.CreateCheckpoint(ctx, "latest-agent")
		if err != nil {
			t.Fatalf("CreateCheckpoint() #1 error = %v", err)
		}
		time.Sleep(5 * time.Millisecond)
		cp2, err := ctrl.CreateCheckpoint(ctx, "latest-agent")
		if err != nil {
			t.Fatalf("CreateCheckpoint() #2 error = %v", err)
		}

		// Stop agent.
		ctrl.StopAgent(ctx, "latest-agent")

		result, err := ctrl.RecoverFromLatest(ctx, "latest-agent")
		if err != nil {
			t.Fatalf("RecoverFromLatest() error = %v", err)
		}
		if !result.Success {
			t.Fatalf("recovery should succeed, got message: %s", result.Message)
		}
		if result.CheckpointID != cp2.ID {
			t.Errorf("checkpointID = %q, want latest %q", result.CheckpointID, cp2.ID)
		}

		// Clean up.
		ctrl.StopAgent(ctx, "latest-agent")
	})
}

func TestRecoverAgent(t *testing.T) {
	ctrl, _, cleanup := setupTest(t)
	defer cleanup()

	ctx := context.Background()

	t.Run("manual recovery", func(t *testing.T) {
		result, err := ctrl.RecoverAgent(ctx, "manual-agent", RecoverySourceManual)
		if err != nil {
			t.Fatalf("RecoverAgent(manual) error = %v", err)
		}
		if !result.Success {
			t.Errorf("manual recovery should succeed")
		}
		if result.Source != RecoverySourceManual {
			t.Errorf("source = %q, want %q", result.Source, RecoverySourceManual)
		}
	})

	t.Run("memory recovery", func(t *testing.T) {
		applyAndStart(t, ctrl, "mem-agent")
		ctrl.StopAgent(ctx, "mem-agent")

		result, err := ctrl.RecoverAgent(ctx, "mem-agent", RecoverySourceMemory)
		if err != nil {
			t.Fatalf("RecoverAgent(memory) error = %v", err)
		}
		if !result.Success {
			t.Fatalf("memory recovery should succeed, got: %s", result.Message)
		}
		if result.Source != RecoverySourceMemory {
			t.Errorf("source = %q, want %q", result.Source, RecoverySourceMemory)
		}

		// Clean up.
		ctrl.StopAgent(ctx, "mem-agent")
	})

	t.Run("unknown source returns error", func(t *testing.T) {
		_, err := ctrl.RecoverAgent(ctx, "any", RecoverySource("unknown"))
		if err == nil {
			t.Fatal("RecoverAgent() with unknown source should return error")
		}
	})
}

func TestHealthAndMetrics(t *testing.T) {
	ctrl, _, cleanup := setupTest(t)
	defer cleanup()

	applyAndStart(t, ctrl, "health-agent")

	t.Run("health returns status for running agents", func(t *testing.T) {
		health := ctrl.Health()
		status, ok := health["health-agent"]
		if !ok {
			t.Fatal("health map should contain health-agent")
		}
		if !status.Healthy {
			t.Error("agent should be healthy by default")
		}
	})

	t.Run("metrics returns metrics for running agents", func(t *testing.T) {
		metrics := ctrl.AgentMetrics()
		_, ok := metrics["health-agent"]
		if !ok {
			t.Fatal("metrics map should contain health-agent")
		}
	})
}

func TestListAgents(t *testing.T) {
	ctrl, _, cleanup := setupTest(t)
	defer cleanup()

	ctx := context.Background()

	for i := 0; i < 3; i++ {
		name := fmt.Sprintf("list-agent-%d", i)
		if err := ctrl.Apply(ctx, testSpec(name)); err != nil {
			t.Fatalf("Apply() error = %v", err)
		}
	}

	agents, err := ctrl.ListAgents(ctx)
	if err != nil {
		t.Fatalf("ListAgents() error = %v", err)
	}
	if len(agents) < 3 {
		t.Errorf("got %d agents, want at least 3", len(agents))
	}
}

func TestGetRestartCount(t *testing.T) {
	ctrl, _, cleanup := setupTest(t)
	defer cleanup()
	t.Run("initial restart count is zero", func(t *testing.T) {
		count := ctrl.GetRestartCount("any-agent")
		if count != 0 {
			t.Errorf("initial restart count = %d, want 0", count)
		}
	})

	t.Run("reset restart count", func(t *testing.T) {
		// Manually increment by getting lifecycle state.
		ls := ctrl.getLifecycleState("reset-agent")
		ls.mu.Lock()
		ls.restartCount = 5
		ls.mu.Unlock()

		if ctrl.GetRestartCount("reset-agent") != 5 {
			t.Fatal("restart count should be 5")
		}

		ctrl.ResetRestartCount("reset-agent")
		if ctrl.GetRestartCount("reset-agent") != 0 {
			t.Error("restart count should be 0 after reset")
		}
	})
}
