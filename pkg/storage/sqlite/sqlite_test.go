package sqlite

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	v1 "github.com/zlc-ai/opc-platform/api/v1"
	"github.com/zlc-ai/opc-platform/pkg/storage"
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

func TestFederatedGoalRunCRUD(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	run := storage.FederatedGoalRunRecord{
		GoalID:       "goal-001",
		GoalName:     "test federated goal",
		Description:  "build login feature",
		CallbackURL:  "http://localhost:9527/api/federation/callback",
		Status:       "InProgress",
		TraceContext: "00-abc-def-01",
		ResultsJSON:  "{}",
	}

	// Save.
	if err := s.SaveFederatedGoalRun(ctx, run); err != nil {
		t.Fatalf("save: %v", err)
	}

	// Get.
	got, err := s.GetFederatedGoalRun(ctx, "goal-001")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.GoalName != "test federated goal" || got.Status != "InProgress" {
		t.Errorf("got %+v", got)
	}

	// List active.
	active, err := s.ListActiveFederatedGoalRuns(ctx)
	if err != nil {
		t.Fatalf("list active: %v", err)
	}
	if len(active) != 1 {
		t.Errorf("expected 1 active, got %d", len(active))
	}

	// Update status.
	if err := s.UpdateFederatedGoalRunStatus(ctx, "goal-001", "Completed"); err != nil {
		t.Fatalf("update status: %v", err)
	}
	got2, _ := s.GetFederatedGoalRun(ctx, "goal-001")
	if got2.Status != "Completed" {
		t.Errorf("expected Completed, got %s", got2.Status)
	}

	// After completing, should not appear in active list.
	active2, _ := s.ListActiveFederatedGoalRuns(ctx)
	if len(active2) != 0 {
		t.Errorf("expected 0 active after completion, got %d", len(active2))
	}

	// Delete.
	if err := s.DeleteFederatedGoalRun(ctx, "goal-001"); err != nil {
		t.Fatalf("delete: %v", err)
	}
	_, err = s.GetFederatedGoalRun(ctx, "goal-001")
	if err == nil {
		t.Error("expected error after delete")
	}
}

func TestFederatedGoalProjectCRUD(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	// Create parent run first.
	run := storage.FederatedGoalRunRecord{
		GoalID:      "goal-002",
		GoalName:    "multi-project goal",
		Status:      "InProgress",
		ResultsJSON: "{}",
	}
	s.SaveFederatedGoalRun(ctx, run)

	// Save projects.
	proj1 := storage.FederatedGoalProjectRecord{
		GoalID:           "goal-002",
		ProjectID:        "proj-001",
		ProjectName:      "ui-design",
		CompanyID:        "design-team",
		AgentName:        "designer",
		Description:      "design the login UI",
		Status:           "Pending",
		MaxRounds:        3,
		Layer:            0,
		DependenciesJSON: "[]",
	}
	proj2 := storage.FederatedGoalProjectRecord{
		GoalID:           "goal-002",
		ProjectID:        "proj-002",
		ProjectName:      "api-impl",
		CompanyID:        "dev-team",
		AgentName:        "coder",
		Description:      "implement the login API",
		Status:           "Pending",
		MaxRounds:        3,
		Layer:            1,
		DependenciesJSON: `["ui-design"]`,
	}

	if err := s.SaveFederatedGoalProject(ctx, proj1); err != nil {
		t.Fatalf("save proj1: %v", err)
	}
	if err := s.SaveFederatedGoalProject(ctx, proj2); err != nil {
		t.Fatalf("save proj2: %v", err)
	}

	// List.
	projects, err := s.ListFederatedGoalProjects(ctx, "goal-002")
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(projects) != 2 {
		t.Fatalf("expected 2, got %d", len(projects))
	}
	// Should be ordered by layer.
	if projects[0].ProjectName != "ui-design" || projects[0].Layer != 0 {
		t.Errorf("first project should be ui-design layer 0, got %s layer %d", projects[0].ProjectName, projects[0].Layer)
	}
	if projects[1].ProjectName != "api-impl" || projects[1].Layer != 1 {
		t.Errorf("second project should be api-impl layer 1, got %s layer %d", projects[1].ProjectName, projects[1].Layer)
	}

	// Update.
	proj1.Status = "Completed"
	proj1.Result = "UI mockup delivered"
	proj1.Round = 1
	if err := s.UpdateFederatedGoalProject(ctx, proj1); err != nil {
		t.Fatalf("update: %v", err)
	}
	updated, _ := s.ListFederatedGoalProjects(ctx, "goal-002")
	for _, p := range updated {
		if p.ProjectName == "ui-design" {
			if p.Status != "Completed" || p.Result != "UI mockup delivered" || p.Round != 1 {
				t.Errorf("update failed: %+v", p)
			}
		}
	}
}

func TestFederatedGoalRunUpsert(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	run := storage.FederatedGoalRunRecord{
		GoalID:      "goal-upsert",
		GoalName:    "original",
		Status:      "InProgress",
		ResultsJSON: "{}",
	}
	s.SaveFederatedGoalRun(ctx, run)

	// Upsert with new name.
	run.GoalName = "updated"
	run.ResultsJSON = `{"proj1":"done"}`
	if err := s.SaveFederatedGoalRun(ctx, run); err != nil {
		t.Fatalf("upsert: %v", err)
	}

	got, _ := s.GetFederatedGoalRun(ctx, "goal-upsert")
	if got.GoalName != "updated" || got.ResultsJSON != `{"proj1":"done"}` {
		t.Errorf("upsert failed: %+v", got)
	}
}

func TestFederatedGoalPersistence(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "fed-persist.db")

	// Create, save, close.
	s1, _ := New(dbPath)
	ctx := context.Background()
	s1.SaveFederatedGoalRun(ctx, storage.FederatedGoalRunRecord{
		GoalID: "goal-persist", GoalName: "persist test", Status: "InProgress", ResultsJSON: "{}",
	})
	s1.SaveFederatedGoalProject(ctx, storage.FederatedGoalProjectRecord{
		GoalID: "goal-persist", ProjectID: "proj-p1", ProjectName: "p1",
		Status: "Running", DependenciesJSON: "[]",
	})
	s1.Close()

	// Reopen.
	s2, err := New(dbPath)
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	defer s2.Close()

	got, err := s2.GetFederatedGoalRun(ctx, "goal-persist")
	if err != nil {
		t.Fatalf("get after reopen: %v", err)
	}
	if got.GoalName != "persist test" {
		t.Errorf("expected 'persist test', got %q", got.GoalName)
	}

	projects, _ := s2.ListFederatedGoalProjects(ctx, "goal-persist")
	if len(projects) != 1 || projects[0].ProjectName != "p1" {
		t.Errorf("project persistence failed: %+v", projects)
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
