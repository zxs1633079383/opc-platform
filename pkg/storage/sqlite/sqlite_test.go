package sqlite

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	v1 "github.com/zlc-ai/opc-platform/api/v1"
)

func newTestStore(t *testing.T) *sqliteStore {
	t.Helper()
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")
	s, err := New(dbPath)
	if err != nil {
		t.Fatalf("new store: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	return s.(*sqliteStore)
}

func TestAgentCRUD(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	// Create.
	agent := v1.AgentRecord{
		Name:  "test-agent",
		Type:  v1.AgentTypeOpenClaw,
		Phase: v1.AgentPhaseCreated,
	}
	if err := s.CreateAgent(ctx, agent); err != nil {
		t.Fatalf("create: %v", err)
	}

	// Get.
	got, err := s.GetAgent(ctx, "test-agent")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.Name != "test-agent" || got.Type != v1.AgentTypeOpenClaw {
		t.Errorf("got %+v", got)
	}

	// List.
	agents, err := s.ListAgents(ctx)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(agents) != 1 {
		t.Errorf("expected 1, got %d", len(agents))
	}

	// Update.
	got.Phase = v1.AgentPhaseRunning
	got.Restarts = 2
	if err := s.UpdateAgent(ctx, got); err != nil {
		t.Fatalf("update: %v", err)
	}
	updated, _ := s.GetAgent(ctx, "test-agent")
	if updated.Phase != v1.AgentPhaseRunning || updated.Restarts != 2 {
		t.Errorf("update failed: %+v", updated)
	}

	// Delete.
	if err := s.DeleteAgent(ctx, "test-agent"); err != nil {
		t.Fatalf("delete: %v", err)
	}
	_, err = s.GetAgent(ctx, "test-agent")
	if err == nil {
		t.Error("expected error after delete")
	}
}

func TestAgentDuplicate(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	agent := v1.AgentRecord{Name: "dup", Type: v1.AgentTypeOpenClaw, Phase: v1.AgentPhaseCreated}
	if err := s.CreateAgent(ctx, agent); err != nil {
		t.Fatalf("first create: %v", err)
	}
	if err := s.CreateAgent(ctx, agent); err == nil {
		t.Error("expected error on duplicate")
	}
}

func TestDeleteNonExistent(t *testing.T) {
	s := newTestStore(t)
	if err := s.DeleteAgent(context.Background(), "nope"); err == nil {
		t.Error("expected error")
	}
}

func TestTaskCRUD(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	// Create agent first.
	s.CreateAgent(ctx, v1.AgentRecord{Name: "agent1", Type: v1.AgentTypeOpenClaw, Phase: v1.AgentPhaseRunning})

	task := v1.TaskRecord{
		ID:        "task-001",
		AgentName: "agent1",
		Message:   "hello world",
		Status:    v1.TaskStatusPending,
	}
	if err := s.CreateTask(ctx, task); err != nil {
		t.Fatalf("create task: %v", err)
	}

	// Get.
	got, err := s.GetTask(ctx, "task-001")
	if err != nil {
		t.Fatalf("get task: %v", err)
	}
	if got.Message != "hello world" || got.Status != v1.TaskStatusPending {
		t.Errorf("got %+v", got)
	}

	// Update.
	now := time.Now()
	got.Status = v1.TaskStatusCompleted
	got.Result = "done"
	got.TokensIn = 100
	got.TokensOut = 200
	got.EndedAt = &now
	if err := s.UpdateTask(ctx, got); err != nil {
		t.Fatalf("update task: %v", err)
	}
	updated, _ := s.GetTask(ctx, "task-001")
	if updated.Status != v1.TaskStatusCompleted || updated.TokensIn != 100 {
		t.Errorf("update failed: %+v", updated)
	}

	// List.
	tasks, err := s.ListTasks(ctx)
	if err != nil {
		t.Fatalf("list tasks: %v", err)
	}
	if len(tasks) != 1 {
		t.Errorf("expected 1, got %d", len(tasks))
	}

	// List by agent.
	byAgent, err := s.ListTasksByAgent(ctx, "agent1")
	if err != nil {
		t.Fatalf("list by agent: %v", err)
	}
	if len(byAgent) != 1 {
		t.Errorf("expected 1, got %d", len(byAgent))
	}
}

func TestPersistence(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "persist.db")

	// Create store, add data, close.
	s1, err := New(dbPath)
	if err != nil {
		t.Fatalf("new: %v", err)
	}
	s1.CreateAgent(context.Background(), v1.AgentRecord{Name: "persist-agent", Type: v1.AgentTypeOpenClaw, Phase: v1.AgentPhaseCreated})
	s1.Close()

	// Reopen and verify data survives.
	s2, err := New(dbPath)
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	defer s2.Close()

	got, err := s2.GetAgent(context.Background(), "persist-agent")
	if err != nil {
		t.Fatalf("get after reopen: %v", err)
	}
	if got.Name != "persist-agent" {
		t.Errorf("expected persist-agent, got %s", got.Name)
	}
}

func TestMigrateIdempotent(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "migrate.db")

	// Run migrations twice.
	s1, _ := New(dbPath)
	s1.Close()
	s2, err := New(dbPath)
	if err != nil {
		t.Fatalf("second open should succeed: %v", err)
	}
	s2.Close()

	// Verify file exists.
	if _, err := os.Stat(dbPath); err != nil {
		t.Fatalf("db file should exist: %v", err)
	}
}
