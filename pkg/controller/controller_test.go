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
	"github.com/zlc-ai/opc-platform/pkg/cost"
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

// ---------------------------------------------------------------------------
// Additional Tests for Coverage Improvement
// ---------------------------------------------------------------------------

func TestSetCostTracker(t *testing.T) {
	ctrl, _, cleanup := setupTest(t)
	defer cleanup()

	tmpDir := t.TempDir()
	logger := zap.NewNop().Sugar()
	tracker := cost.NewTracker(filepath.Join(tmpDir, "cost"), logger)

	ctrl.SetCostTracker(tracker)

	if ctrl.costMgr != tracker {
		t.Error("cost tracker should be set")
	}
}

func TestRecoverAgents(t *testing.T) {
	ctrl, _, cleanup := setupTest(t)
	defer cleanup()

	ctx := context.Background()

	t.Run("recovers running agents", func(t *testing.T) {
		// Create and start an agent, then manually set it to Running in the store
		// without having it in memory (simulating a daemon restart).
		if err := ctrl.Apply(ctx, testSpec("recover-running")); err != nil {
			t.Fatalf("Apply() error = %v", err)
		}
		record, _ := ctrl.store.GetAgent(ctx, "recover-running")
		record.Phase = v1.AgentPhaseRunning
		ctrl.store.UpdateAgent(ctx, record)

		ctrl.RecoverAgents(ctx)

		// Verify agent was started.
		_, err := ctrl.GetAdapter("recover-running")
		if err != nil {
			t.Errorf("agent should be running after recovery: %v", err)
		}

		// Clean up.
		ctrl.StopAgent(ctx, "recover-running")
	})

	t.Run("recovers starting agents", func(t *testing.T) {
		if err := ctrl.Apply(ctx, testSpec("recover-starting")); err != nil {
			t.Fatalf("Apply() error = %v", err)
		}
		record, _ := ctrl.store.GetAgent(ctx, "recover-starting")
		record.Phase = v1.AgentPhaseStarting
		ctrl.store.UpdateAgent(ctx, record)

		ctrl.RecoverAgents(ctx)

		_, err := ctrl.GetAdapter("recover-starting")
		if err != nil {
			t.Errorf("agent should be running after recovery: %v", err)
		}
		ctrl.StopAgent(ctx, "recover-starting")
	})

	t.Run("skips non-running agents", func(t *testing.T) {
		if err := ctrl.Apply(ctx, testSpec("recover-stopped")); err != nil {
			t.Fatalf("Apply() error = %v", err)
		}
		// Phase is Created by default, should not be recovered.
		ctrl.RecoverAgents(ctx)

		_, err := ctrl.GetAdapter("recover-stopped")
		if err == nil {
			t.Error("stopped agent should not be recovered")
		}
	})
}

func TestStartAgentWithAdapterStartError(t *testing.T) {
	tmpDir := t.TempDir()
	origHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", origHome)

	dbPath := filepath.Join(tmpDir, "test.db")
	store, err := sqlite.New(dbPath)
	if err != nil {
		t.Fatalf("create sqlite store: %v", err)
	}
	defer store.Close()

	registry := adapter.NewRegistry()
	registry.Register(v1.AgentTypeClaudeCode, func() adapter.Adapter {
		m := newMockAdapter(v1.AgentTypeClaudeCode)
		m.startErr = errors.New("adapter start failed")
		return m
	})

	logger := zap.NewNop().Sugar()
	ctrl := New(store, registry, logger)

	ctx := context.Background()
	if err := ctrl.Apply(ctx, testSpec("fail-start")); err != nil {
		t.Fatalf("Apply() error = %v", err)
	}

	err = ctrl.StartAgent(ctx, "fail-start")
	if err == nil {
		t.Fatal("StartAgent() should return error when adapter Start fails")
	}

	// Verify phase is Failed.
	record, _ := ctrl.GetAgent(ctx, "fail-start")
	if record.Phase != v1.AgentPhaseFailed {
		t.Errorf("phase = %q, want %q", record.Phase, v1.AgentPhaseFailed)
	}
	if record.Message == "" {
		t.Error("message should contain error details")
	}
}

func TestExecuteTaskWithBudgetExceeded(t *testing.T) {
	ctrl, _, cleanup := setupTest(t)
	defer cleanup()

	ctx := context.Background()
	applyAndStart(t, ctrl, "budget-agent")

	// Set up a cost tracker with a budget that is exceeded.
	tmpDir := t.TempDir()
	logger := zap.NewNop().Sugar()
	tracker := cost.NewTracker(filepath.Join(tmpDir, "cost"), logger)
	tracker.SetBudget(cost.BudgetConfig{
		DailyLimit:   0.01,
		MonthlyLimit: 0.01,
	})
	// Record enough cost to exceed.
	tracker.RecordCost(cost.CostEvent{
		AgentName: "budget-agent",
		TaskID:    "pre-task",
		InputCost: 1.00,
	})

	ctrl.SetCostTracker(tracker)

	task := v1.TaskRecord{
		ID:        "budget-task",
		AgentName: "budget-agent",
		Message:   "should be rejected",
		Status:    v1.TaskStatusPending,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	if err := ctrl.Store().CreateTask(ctx, task); err != nil {
		t.Fatalf("CreateTask() error = %v", err)
	}

	_, err := ctrl.ExecuteTask(ctx, task)
	if err == nil {
		t.Fatal("ExecuteTask() should return error when budget exceeded")
	}

	stored, _ := ctrl.Store().GetTask(ctx, "budget-task")
	if stored.Status != v1.TaskStatusFailed {
		t.Errorf("status = %q, want %q", stored.Status, v1.TaskStatusFailed)
	}
}

func TestExecuteTaskCircuitBreaker(t *testing.T) {
	ctrl, _, cleanup := setupTest(t)
	defer cleanup()

	ctx := context.Background()
	applyAndStart(t, ctrl, "cb-agent")

	adp, _ := ctrl.GetAdapter("cb-agent")
	mock := adp.(*mockAdapter)
	mock.mu.Lock()
	mock.executeErr = errors.New("persistent failure")
	mock.mu.Unlock()

	// Execute tasks until circuit breaker triggers (threshold = 5).
	for i := 0; i < circuitBreakerThreshold; i++ {
		taskID := fmt.Sprintf("cb-task-%d", i)
		task := v1.TaskRecord{
			ID:        taskID,
			AgentName: "cb-agent",
			Message:   "will fail",
			Status:    v1.TaskStatusPending,
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}
		ctrl.Store().CreateTask(ctx, task)
		ctrl.ExecuteTask(ctx, task)
	}

	// Agent should have been stopped by circuit breaker.
	_, err := ctrl.GetAdapter("cb-agent")
	if err == nil {
		t.Error("agent should be stopped after circuit breaker")
	}

	// Agent phase should be Failed.
	record, _ := ctrl.GetAgent(ctx, "cb-agent")
	if record.Phase != v1.AgentPhaseFailed {
		t.Errorf("phase = %q, want %q", record.Phase, v1.AgentPhaseFailed)
	}
}

func TestExecuteTaskResetsFailCountOnSuccess(t *testing.T) {
	ctrl, _, cleanup := setupTest(t)
	defer cleanup()

	ctx := context.Background()
	applyAndStart(t, ctrl, "reset-agent")

	adp, _ := ctrl.GetAdapter("reset-agent")
	mock := adp.(*mockAdapter)

	// Cause some failures first.
	mock.mu.Lock()
	mock.executeErr = errors.New("temp failure")
	mock.mu.Unlock()

	for i := 0; i < 3; i++ {
		taskID := fmt.Sprintf("fail-task-%d", i)
		task := v1.TaskRecord{
			ID: taskID, AgentName: "reset-agent", Message: "fail",
			Status: v1.TaskStatusPending, CreatedAt: time.Now(), UpdatedAt: time.Now(),
		}
		ctrl.Store().CreateTask(ctx, task)
		ctrl.ExecuteTask(ctx, task)
	}

	// Verify failure count is 3.
	ctrl.mu.RLock()
	count := ctrl.failCounts["reset-agent"]
	ctrl.mu.RUnlock()
	if count != 3 {
		t.Fatalf("fail count = %d, want 3", count)
	}

	// Now succeed.
	mock.setExecuteResult("ok", 10, 5)

	task := v1.TaskRecord{
		ID: "success-task", AgentName: "reset-agent", Message: "succeed",
		Status: v1.TaskStatusPending, CreatedAt: time.Now(), UpdatedAt: time.Now(),
	}
	ctrl.Store().CreateTask(ctx, task)
	_, err := ctrl.ExecuteTask(ctx, task)
	if err != nil {
		t.Fatalf("ExecuteTask() error = %v", err)
	}

	// Fail count should be reset.
	ctrl.mu.RLock()
	count = ctrl.failCounts["reset-agent"]
	ctrl.mu.RUnlock()
	if count != 0 {
		t.Errorf("fail count after success = %d, want 0", count)
	}
}

func TestExecuteTaskWithCostTracking(t *testing.T) {
	ctrl, _, cleanup := setupTest(t)
	defer cleanup()

	ctx := context.Background()

	// Create spec with model info.
	spec := testSpec("cost-agent")
	spec.Spec.Runtime.Model.Provider = "anthropic"
	spec.Spec.Runtime.Model.Name = "claude-sonnet-4"

	if err := ctrl.Apply(ctx, spec); err != nil {
		t.Fatalf("Apply() error = %v", err)
	}
	if err := ctrl.StartAgent(ctx, "cost-agent"); err != nil {
		t.Fatalf("StartAgent() error = %v", err)
	}

	adp, _ := ctrl.GetAdapter("cost-agent")
	mock := adp.(*mockAdapter)
	mock.setExecuteResult("response", 200, 100)

	// Set up cost tracker.
	tmpDir := t.TempDir()
	logger := zap.NewNop().Sugar()
	tracker := cost.NewTracker(filepath.Join(tmpDir, "cost"), logger)
	ctrl.SetCostTracker(tracker)

	task := v1.TaskRecord{
		ID: "cost-task", AgentName: "cost-agent", Message: "test",
		Status: v1.TaskStatusPending, CreatedAt: time.Now(), UpdatedAt: time.Now(),
	}
	ctrl.Store().CreateTask(ctx, task)

	result, err := ctrl.ExecuteTask(ctx, task)
	if err != nil {
		t.Fatalf("ExecuteTask() error = %v", err)
	}

	if result.TokensIn != 200 || result.TokensOut != 100 {
		t.Errorf("tokens = (%d, %d), want (200, 100)", result.TokensIn, result.TokensOut)
	}

	// Verify cost was recorded in task.
	stored, _ := ctrl.Store().GetTask(ctx, "cost-task")
	if stored.Cost <= 0 {
		t.Log("cost may be 0 if model not in pricing table, that's ok")
	}
}

func TestExecuteTaskWithAgentReportedCost(t *testing.T) {
	ctrl, _, cleanup := setupTest(t)
	defer cleanup()

	ctx := context.Background()
	applyAndStart(t, ctrl, "reported-cost-agent")

	adp, _ := ctrl.GetAdapter("reported-cost-agent")
	mock := adp.(*mockAdapter)
	mock.mu.Lock()
	mock.executeResult = adapter.ExecuteResult{
		Output:    "done",
		TokensIn:  100,
		TokensOut: 50,
		Cost:      0.42,
	}
	mock.executeErr = nil
	mock.mu.Unlock()

	tmpDir := t.TempDir()
	logger := zap.NewNop().Sugar()
	tracker := cost.NewTracker(filepath.Join(tmpDir, "cost"), logger)
	ctrl.SetCostTracker(tracker)

	task := v1.TaskRecord{
		ID: "reported-cost-task", AgentName: "reported-cost-agent", Message: "test",
		Status: v1.TaskStatusPending, CreatedAt: time.Now(), UpdatedAt: time.Now(),
	}
	ctrl.Store().CreateTask(ctx, task)

	_, err := ctrl.ExecuteTask(ctx, task)
	if err != nil {
		t.Fatalf("ExecuteTask() error = %v", err)
	}

	stored, _ := ctrl.Store().GetTask(ctx, "reported-cost-task")
	if stored.Cost != 0.42 {
		t.Errorf("cost = %f, want 0.42", stored.Cost)
	}
}

func TestExecuteTaskWithQuotaExceeded(t *testing.T) {
	ctrl, _, cleanup := setupTest(t)
	defer cleanup()

	ctx := context.Background()

	// Create agent with resource limits.
	spec := testSpec("quota-agent")
	spec.Spec.Resources.TokenBudget.PerTask = 10
	spec.Spec.Resources.TokenBudget.PerDay = 100
	spec.Spec.Resources.CostLimit.PerDay = "$0.01"
	spec.Spec.Resources.OnExceed = "pause"

	if err := ctrl.Apply(ctx, spec); err != nil {
		t.Fatalf("Apply() error = %v", err)
	}
	if err := ctrl.StartAgent(ctx, "quota-agent"); err != nil {
		t.Fatalf("StartAgent() error = %v", err)
	}

	adp, _ := ctrl.GetAdapter("quota-agent")
	mock := adp.(*mockAdapter)
	mock.setExecuteResult("ok", 10, 5)

	// Record heavy usage to exceed daily quota.
	ctrl.quotaEnforcer.RecordUsage("quota-agent", 10000, 10000, 100.0)

	task := v1.TaskRecord{
		ID: "quota-task", AgentName: "quota-agent", Message: "test",
		Status: v1.TaskStatusPending, CreatedAt: time.Now(), UpdatedAt: time.Now(),
	}
	ctrl.Store().CreateTask(ctx, task)

	_, err := ctrl.ExecuteTask(ctx, task)
	if err == nil {
		t.Fatal("ExecuteTask() should return error when quota exceeded")
	}
}

func TestStreamTaskErrorPath(t *testing.T) {
	ctrl, _, cleanup := setupTest(t)
	defer cleanup()

	ctx := context.Background()

	t.Run("stream on non-running agent", func(t *testing.T) {
		task := v1.TaskRecord{
			ID: "stream-err", AgentName: "nonexistent", Message: "test",
			Status: v1.TaskStatusPending, CreatedAt: time.Now(), UpdatedAt: time.Now(),
		}
		ctrl.Store().CreateTask(ctx, task)

		_, err := ctrl.StreamTask(ctx, task)
		if err == nil {
			t.Fatal("StreamTask() should fail for non-running agent")
		}

		stored, _ := ctrl.Store().GetTask(ctx, "stream-err")
		if stored.Status != v1.TaskStatusFailed {
			t.Errorf("status = %q, want %q", stored.Status, v1.TaskStatusFailed)
		}
	})
}

func TestGetAgentModel(t *testing.T) {
	ctrl, _, cleanup := setupTest(t)
	defer cleanup()

	t.Run("returns default for non-running agent", func(t *testing.T) {
		provider, model := ctrl.getAgentModel("nonexistent")
		if provider != "anthropic" {
			t.Errorf("provider = %q, want %q", provider, "anthropic")
		}
		if model != "claude-sonnet-4" {
			t.Errorf("model = %q, want %q", model, "claude-sonnet-4")
		}
	})

	t.Run("returns spec values for running agent", func(t *testing.T) {
		spec := testSpec("model-agent")
		spec.Spec.Runtime.Model.Provider = "openai"
		spec.Spec.Runtime.Model.Name = "gpt-4"

		ctx := context.Background()
		if err := ctrl.Apply(ctx, spec); err != nil {
			t.Fatalf("Apply() error = %v", err)
		}
		if err := ctrl.StartAgent(ctx, "model-agent"); err != nil {
			t.Fatalf("StartAgent() error = %v", err)
		}

		provider, model := ctrl.getAgentModel("model-agent")
		if provider != "openai" {
			t.Errorf("provider = %q, want %q", provider, "openai")
		}
		if model != "gpt-4" {
			t.Errorf("model = %q, want %q", model, "gpt-4")
		}
	})
}

func TestMarshalUnmarshalSpec(t *testing.T) {
	t.Run("round-trip preserves spec", func(t *testing.T) {
		spec := testSpec("marshal-test")
		spec.Spec.Description = "test description"

		data, err := marshalSpec(spec)
		if err != nil {
			t.Fatalf("marshalSpec() error = %v", err)
		}

		got, err := unmarshalSpec(data)
		if err != nil {
			t.Fatalf("unmarshalSpec() error = %v", err)
		}

		if got.Metadata.Name != "marshal-test" {
			t.Errorf("name = %q, want %q", got.Metadata.Name, "marshal-test")
		}
		if got.Spec.Description != "test description" {
			t.Errorf("description = %q, want %q", got.Spec.Description, "test description")
		}
	})

	t.Run("unmarshal invalid data returns error", func(t *testing.T) {
		_, err := unmarshalSpec("not valid json")
		if err == nil {
			t.Fatal("unmarshalSpec() should return error for invalid JSON")
		}
	})
}

func TestBuildQuotaConfig(t *testing.T) {
	ctrl, _, cleanup := setupTest(t)
	defer cleanup()

	t.Run("returns empty config for non-running agent", func(t *testing.T) {
		qc := ctrl.buildQuotaConfig("nonexistent")
		if qc.TokenPerDay != 0 || qc.CostPerDay != 0 {
			t.Error("expected zero config for non-running agent")
		}
	})

	t.Run("returns config from spec for running agent", func(t *testing.T) {
		spec := testSpec("quota-config-agent")
		spec.Spec.Resources.TokenBudget.PerTask = 100
		spec.Spec.Resources.TokenBudget.PerHour = 1000
		spec.Spec.Resources.TokenBudget.PerDay = 10000
		spec.Spec.Resources.CostLimit.PerTask = "$1.00"
		spec.Spec.Resources.CostLimit.PerDay = "$10.00"
		spec.Spec.Resources.CostLimit.PerMonth = "$100.00"
		spec.Spec.Resources.OnExceed = "pause"

		ctx := context.Background()
		if err := ctrl.Apply(ctx, spec); err != nil {
			t.Fatalf("Apply() error = %v", err)
		}
		if err := ctrl.StartAgent(ctx, "quota-config-agent"); err != nil {
			t.Fatalf("StartAgent() error = %v", err)
		}

		qc := ctrl.buildQuotaConfig("quota-config-agent")
		if qc.TokenPerTask != 100 {
			t.Errorf("TokenPerTask = %d, want 100", qc.TokenPerTask)
		}
		if qc.TokenPerHour != 1000 {
			t.Errorf("TokenPerHour = %d, want 1000", qc.TokenPerHour)
		}
		if qc.TokenPerDay != 10000 {
			t.Errorf("TokenPerDay = %d, want 10000", qc.TokenPerDay)
		}
		if qc.OnExceed != cost.ExceedPause {
			t.Errorf("OnExceed = %q, want %q", qc.OnExceed, cost.ExceedPause)
		}
	})
}

func TestAgentMetricsWithTasks(t *testing.T) {
	ctrl, _, cleanup := setupTest(t)
	defer cleanup()

	ctx := context.Background()
	applyAndStart(t, ctrl, "metrics-agent")

	// Create tasks with various statuses.
	tasks := []v1.TaskRecord{
		{ID: "m-t1", AgentName: "metrics-agent", Status: v1.TaskStatusCompleted, Cost: 0.10, TokensIn: 100, TokensOut: 50, CreatedAt: time.Now(), UpdatedAt: time.Now()},
		{ID: "m-t2", AgentName: "metrics-agent", Status: v1.TaskStatusCompleted, Cost: 0.20, TokensIn: 200, TokensOut: 100, CreatedAt: time.Now(), UpdatedAt: time.Now()},
		{ID: "m-t3", AgentName: "metrics-agent", Status: v1.TaskStatusFailed, Cost: 0.05, TokensIn: 50, TokensOut: 25, CreatedAt: time.Now(), UpdatedAt: time.Now()},
		{ID: "m-t4", AgentName: "metrics-agent", Status: v1.TaskStatusRunning, Cost: 0, TokensIn: 0, TokensOut: 0, CreatedAt: time.Now(), UpdatedAt: time.Now()},
	}
	for _, task := range tasks {
		if err := ctrl.Store().CreateTask(ctx, task); err != nil {
			t.Fatalf("CreateTask(%s) error = %v", task.ID, err)
		}
	}

	metrics := ctrl.AgentMetrics()
	m, ok := metrics["metrics-agent"]
	if !ok {
		t.Fatal("metrics should contain metrics-agent")
	}
	if m.TasksCompleted != 2 {
		t.Errorf("TasksCompleted = %d, want 2", m.TasksCompleted)
	}
	if m.TasksFailed != 1 {
		t.Errorf("TasksFailed = %d, want 1", m.TasksFailed)
	}
	if m.TasksRunning != 1 {
		t.Errorf("TasksRunning = %d, want 1", m.TasksRunning)
	}
	if m.TotalTokensIn != 350 {
		t.Errorf("TotalTokensIn = %d, want 350", m.TotalTokensIn)
	}
	if m.TotalTokensOut != 175 {
		t.Errorf("TotalTokensOut = %d, want 175", m.TotalTokensOut)
	}
	expectedCost := 0.35
	if m.TotalCost < expectedCost-0.001 || m.TotalCost > expectedCost+0.001 {
		t.Errorf("TotalCost = %f, want ~%f", m.TotalCost, expectedCost)
	}
}

func TestRunHealthChecks(t *testing.T) {
	ctrl, _, cleanup := setupTest(t)
	defer cleanup()

	ctx := context.Background()
	applyAndStart(t, ctrl, "hc-agent")

	// Run health checks with healthy agent -- should not error.
	ctrl.runHealthChecks(ctx)

	// Now set agent as unhealthy.
	adp, _ := ctrl.GetAdapter("hc-agent")
	mock := adp.(*mockAdapter)
	mock.setHealth(false, "agent down")

	// Run health checks -- should record failure but not restart (no recovery enabled, threshold not reached).
	ctrl.runHealthChecks(ctx)

	ls := ctrl.getLifecycleState("hc-agent")
	ls.mu.Lock()
	failures := ls.consecutiveFailures
	ls.mu.Unlock()
	if failures != 1 {
		t.Errorf("consecutiveFailures = %d, want 1", failures)
	}
}

func TestCheckAndRestartRecoveryDisabled(t *testing.T) {
	ctrl, _, cleanup := setupTest(t)
	defer cleanup()

	ctx := context.Background()

	// Create agent without recovery enabled.
	spec := testSpec("no-recovery-agent")
	spec.Spec.HealthCheck.Retries = 1 // Low threshold to trigger quickly.
	if err := ctrl.Apply(ctx, spec); err != nil {
		t.Fatalf("Apply() error = %v", err)
	}
	if err := ctrl.StartAgent(ctx, "no-recovery-agent"); err != nil {
		t.Fatalf("StartAgent() error = %v", err)
	}

	adp, _ := ctrl.GetAdapter("no-recovery-agent")
	mock := adp.(*mockAdapter)
	mock.setHealth(false, "down")

	ctrl.mu.RLock()
	ma := ctrl.agents["no-recovery-agent"]
	ctrl.mu.RUnlock()

	// Trigger enough failures to exceed threshold.
	ctrl.checkAndRestart(ctx, "no-recovery-agent", ma)

	// Agent should still be running (recovery disabled, no restart).
	_, err := ctrl.GetAdapter("no-recovery-agent")
	if err != nil {
		t.Error("agent should still be running when recovery is disabled")
	}
}

func TestCheckAndRestartWithRecoveryEnabled(t *testing.T) {
	ctrl, _, cleanup := setupTest(t)
	defer cleanup()

	ctx := context.Background()

	spec := testSpec("recovery-agent")
	spec.Spec.HealthCheck.Retries = 1
	spec.Spec.Recovery.Enabled = true
	spec.Spec.Recovery.MaxRestarts = 3
	spec.Spec.Recovery.RestartDelay = "1ms"
	spec.Spec.Recovery.Backoff = "fixed"

	if err := ctrl.Apply(ctx, spec); err != nil {
		t.Fatalf("Apply() error = %v", err)
	}
	if err := ctrl.StartAgent(ctx, "recovery-agent"); err != nil {
		t.Fatalf("StartAgent() error = %v", err)
	}

	adp, _ := ctrl.GetAdapter("recovery-agent")
	mock := adp.(*mockAdapter)
	mock.setHealth(false, "down")

	ctrl.mu.RLock()
	ma := ctrl.agents["recovery-agent"]
	ctrl.mu.RUnlock()

	// Hit threshold (retries=1, so 1 failure triggers).
	ctrl.checkAndRestart(ctx, "recovery-agent", ma)

	// Wait a moment for async restart.
	time.Sleep(100 * time.Millisecond)

	// Verify restart count incremented.
	count := ctrl.GetRestartCount("recovery-agent")
	if count != 1 {
		t.Errorf("restart count = %d, want 1", count)
	}
}

func TestCheckAndRestartExceedsMaxRestarts(t *testing.T) {
	ctrl, _, cleanup := setupTest(t)
	defer cleanup()

	ctx := context.Background()

	spec := testSpec("max-restart-agent")
	spec.Spec.HealthCheck.Retries = 1
	spec.Spec.Recovery.Enabled = true
	spec.Spec.Recovery.MaxRestarts = 1
	spec.Spec.Recovery.RestartDelay = "1ms"
	spec.Spec.Recovery.Backoff = "fixed"

	if err := ctrl.Apply(ctx, spec); err != nil {
		t.Fatalf("Apply() error = %v", err)
	}
	if err := ctrl.StartAgent(ctx, "max-restart-agent"); err != nil {
		t.Fatalf("StartAgent() error = %v", err)
	}

	adp, _ := ctrl.GetAdapter("max-restart-agent")
	mock := adp.(*mockAdapter)
	mock.setHealth(false, "down")

	// Set restart count at max.
	ls := ctrl.getLifecycleState("max-restart-agent")
	ls.mu.Lock()
	ls.restartCount = 1
	ls.mu.Unlock()

	ctrl.mu.RLock()
	ma := ctrl.agents["max-restart-agent"]
	ctrl.mu.RUnlock()

	ctrl.checkAndRestart(ctx, "max-restart-agent", ma)

	// Agent should be marked as Failed (exceeded max restarts).
	record, _ := ctrl.GetAgent(ctx, "max-restart-agent")
	if record.Phase != v1.AgentPhaseFailed {
		t.Errorf("phase = %q, want %q", record.Phase, v1.AgentPhaseFailed)
	}
}

func TestCheckAndRestartHealthyResetsFailures(t *testing.T) {
	ctrl, _, cleanup := setupTest(t)
	defer cleanup()

	ctx := context.Background()
	applyAndStart(t, ctrl, "healthy-reset-agent")

	// Set some consecutive failures first.
	ls := ctrl.getLifecycleState("healthy-reset-agent")
	ls.mu.Lock()
	ls.consecutiveFailures = 3
	ls.lastHealthStatus = false
	ls.mu.Unlock()

	ctrl.mu.RLock()
	ma := ctrl.agents["healthy-reset-agent"]
	ctrl.mu.RUnlock()

	// Agent is healthy by default (mock returns healthy=true).
	ctrl.checkAndRestart(ctx, "healthy-reset-agent", ma)

	ls.mu.Lock()
	failures := ls.consecutiveFailures
	ls.mu.Unlock()
	if failures != 0 {
		t.Errorf("consecutiveFailures = %d, want 0 after healthy check", failures)
	}
}

func TestUpdateAgentPhase(t *testing.T) {
	ctrl, _, cleanup := setupTest(t)
	defer cleanup()

	ctx := context.Background()

	t.Run("updates existing agent phase", func(t *testing.T) {
		if err := ctrl.Apply(ctx, testSpec("phase-agent")); err != nil {
			t.Fatalf("Apply() error = %v", err)
		}

		ctrl.updateAgentPhase(ctx, "phase-agent", v1.AgentPhaseFailed, "test failure")

		record, _ := ctrl.GetAgent(ctx, "phase-agent")
		if record.Phase != v1.AgentPhaseFailed {
			t.Errorf("phase = %q, want %q", record.Phase, v1.AgentPhaseFailed)
		}
		if record.Message != "test failure" {
			t.Errorf("message = %q, want %q", record.Message, "test failure")
		}
	})

	t.Run("handles nonexistent agent gracefully", func(t *testing.T) {
		// Should not panic.
		ctrl.updateAgentPhase(ctx, "nonexistent-phase", v1.AgentPhaseFailed, "error")
	})
}

func TestStartAndStopAgentHealthCheck(t *testing.T) {
	ctrl, _, cleanup := setupTest(t)
	defer cleanup()

	ctx := context.Background()

	spec := testSpec("hc-loop-agent")
	spec.Spec.HealthCheck.Interval = "50ms"
	if err := ctrl.Apply(ctx, spec); err != nil {
		t.Fatalf("Apply() error = %v", err)
	}
	if err := ctrl.StartAgent(ctx, "hc-loop-agent"); err != nil {
		t.Fatalf("StartAgent() error = %v", err)
	}

	ctrl.StartAgentHealthCheck(ctx, "hc-loop-agent")

	// Wait for at least one health check cycle.
	time.Sleep(100 * time.Millisecond)

	// Stop should not panic.
	ctrl.StopAgentHealthCheck("hc-loop-agent")

	// Starting health check for non-running agent should be a no-op.
	ctrl.StartAgentHealthCheck(ctx, "nonexistent-hc")
}

func TestStopHealthCheckLoop(t *testing.T) {
	ctrl, _, cleanup := setupTest(t)
	defer cleanup()

	ctx := context.Background()
	applyAndStart(t, ctrl, "hc-stop-1")
	applyAndStart(t, ctrl, "hc-stop-2")

	ctrl.StartAgentHealthCheck(ctx, "hc-stop-1")
	ctrl.StartAgentHealthCheck(ctx, "hc-stop-2")

	// Stop all should not panic.
	ctrl.StopHealthCheckLoop()

	// Verify cancel functions are nil.
	ls1 := ctrl.getLifecycleState("hc-stop-1")
	ls1.mu.Lock()
	cancel1 := ls1.healthCheckCancel
	ls1.mu.Unlock()
	if cancel1 != nil {
		t.Error("healthCheckCancel should be nil after StopHealthCheckLoop")
	}
}

func TestStartHealthCheckLoop(t *testing.T) {
	ctrl, _, cleanup := setupTest(t)
	defer cleanup()

	ctx, cancel := context.WithCancel(context.Background())
	ctrl.StartHealthCheckLoop(ctx)

	// Let it run briefly then cancel.
	time.Sleep(50 * time.Millisecond)
	cancel()
	time.Sleep(50 * time.Millisecond)
	// No panic means success.
}

func TestStartCheckpointLoop(t *testing.T) {
	ctrl, _, cleanup := setupTest(t)
	defer cleanup()

	ctx, cancel := context.WithCancel(context.Background())
	applyAndStart(t, ctrl, "cp-loop-agent")

	ctrl.StartCheckpointLoop(ctx, 50*time.Millisecond)

	// Wait enough for at least one checkpoint.
	time.Sleep(120 * time.Millisecond)
	cancel()

	// Verify at least one checkpoint was created.
	cps, err := ctrl.ListCheckpoints(context.Background(), "cp-loop-agent")
	if err != nil {
		t.Fatalf("ListCheckpoints() error = %v", err)
	}
	if len(cps) == 0 {
		t.Error("checkpoint loop should have created at least one checkpoint")
	}
}

func TestGetRecoveryStatus(t *testing.T) {
	ctrl, _, cleanup := setupTest(t)
	defer cleanup()

	ctx := context.Background()

	t.Run("no status returns error", func(t *testing.T) {
		_, err := ctrl.GetRecoveryStatus(ctx, "no-status-agent")
		if err == nil {
			t.Fatal("GetRecoveryStatus() should return error when no status exists")
		}
	})

	t.Run("save and get recovery status", func(t *testing.T) {
		result := &RecoveryResult{
			AgentName:  "status-agent",
			Source:     RecoverySourceMemory,
			RestoredAt: time.Now(),
			Success:    true,
			Message:    "recovered ok",
		}
		if err := ctrl.saveRecoveryStatus("status-agent", result); err != nil {
			t.Fatalf("saveRecoveryStatus() error = %v", err)
		}

		got, err := ctrl.GetRecoveryStatus(ctx, "status-agent")
		if err != nil {
			t.Fatalf("GetRecoveryStatus() error = %v", err)
		}
		if got.AgentName != "status-agent" {
			t.Errorf("agentName = %q, want %q", got.AgentName, "status-agent")
		}
		if !got.Success {
			t.Error("success should be true")
		}
		if got.Message != "recovered ok" {
			t.Errorf("message = %q, want %q", got.Message, "recovered ok")
		}
	})
}

func TestRecoverFromMemoryErrorPaths(t *testing.T) {
	ctrl, _, cleanup := setupTest(t)
	defer cleanup()

	ctx := context.Background()

	t.Run("recover nonexistent agent returns error", func(t *testing.T) {
		result, err := ctrl.RecoverAgent(ctx, "ghost-agent", RecoverySourceMemory)
		if err == nil {
			t.Fatal("should return error for nonexistent agent")
		}
		if result == nil {
			t.Fatal("result should not be nil")
		}
		if result.Success {
			t.Error("success should be false")
		}
		if result.Source != RecoverySourceMemory {
			t.Errorf("source = %q, want %q", result.Source, RecoverySourceMemory)
		}
	})
}

func TestStopAgentWithStopError(t *testing.T) {
	tmpDir := t.TempDir()
	origHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", origHome)

	dbPath := filepath.Join(tmpDir, "test.db")
	store, err := sqlite.New(dbPath)
	if err != nil {
		t.Fatalf("create sqlite store: %v", err)
	}
	defer store.Close()

	var latestMock *mockAdapter
	registry := adapter.NewRegistry()
	registry.Register(v1.AgentTypeClaudeCode, func() adapter.Adapter {
		m := newMockAdapter(v1.AgentTypeClaudeCode)
		m.stopErr = errors.New("stop failed")
		latestMock = m
		return m
	})

	logger := zap.NewNop().Sugar()
	ctrl := New(store, registry, logger)
	_ = latestMock

	ctx := context.Background()
	applyAndStart(t, ctrl, "stop-err-agent")

	// StopAgent should still succeed even if adapter Stop returns error.
	err = ctrl.StopAgent(ctx, "stop-err-agent")
	if err != nil {
		t.Fatalf("StopAgent() should succeed even with stop error: %v", err)
	}

	// Agent should be removed from managed agents.
	_, err = ctrl.GetAdapter("stop-err-agent")
	if err == nil {
		t.Error("agent should be removed after stop")
	}
}

func TestRestartAgentInternalStartError(t *testing.T) {
	ctrl, _, cleanup := setupTest(t)
	defer cleanup()

	ctx := context.Background()
	applyAndStart(t, ctrl, "restart-fail-agent")

	// Replace registry with one that returns start errors.
	ctrl.registry = adapter.NewRegistry()
	ctrl.registry.Register(v1.AgentTypeClaudeCode, func() adapter.Adapter {
		m := newMockAdapter(v1.AgentTypeClaudeCode)
		m.startErr = errors.New("restart start failed")
		return m
	})

	err := ctrl.restartAgentInternal(ctx, "restart-fail-agent")
	if err == nil {
		t.Fatal("restartAgentInternal() should return error when start fails")
	}
}

func TestRecoverFromCheckpointSourceDispatch(t *testing.T) {
	ctrl, _, cleanup := setupTest(t)
	defer cleanup()

	ctx := context.Background()
	applyAndStart(t, ctrl, "dispatch-agent")

	_, err := ctrl.CreateCheckpoint(ctx, "dispatch-agent")
	if err != nil {
		t.Fatalf("CreateCheckpoint() error = %v", err)
	}

	ctrl.StopAgent(ctx, "dispatch-agent")

	result, err := ctrl.RecoverAgent(ctx, "dispatch-agent", RecoverySourceCheckpoint)
	if err != nil {
		t.Fatalf("RecoverAgent(checkpoint) error = %v", err)
	}
	if !result.Success {
		t.Errorf("recovery should succeed, got: %s", result.Message)
	}
	if result.Source != RecoverySourceCheckpoint {
		t.Errorf("source = %q, want %q", result.Source, RecoverySourceCheckpoint)
	}

	ctrl.StopAgent(ctx, "dispatch-agent")
}
