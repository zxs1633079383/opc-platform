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

// --- Agent CRUD ---

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

func TestGetAgent_NotFound(t *testing.T) {
	s := newTestStore(t)
	_, err := s.GetAgent(context.Background(), "nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent agent")
	}
}

func TestAgentUpdateWithSpecYAML(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	agent := v1.AgentRecord{
		Name:     "yaml-agent",
		Type:     v1.AgentTypeClaudeCode,
		Phase:    v1.AgentPhaseCreated,
		SpecYAML: "apiVersion: v1\nkind: Agent",
		Message:  "initial message",
	}
	s.CreateAgent(ctx, agent)

	agent.SpecYAML = "apiVersion: v2\nkind: Agent"
	agent.Message = "updated"
	s.UpdateAgent(ctx, agent)

	got, _ := s.GetAgent(ctx, "yaml-agent")
	if got.SpecYAML != "apiVersion: v2\nkind: Agent" {
		t.Errorf("SpecYAML = %q", got.SpecYAML)
	}
	if got.Message != "updated" {
		t.Errorf("Message = %q", got.Message)
	}
}

// --- Task CRUD ---

func TestTaskCRUD(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

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

func TestTaskWithHierarchy(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	s.CreateAgent(ctx, v1.AgentRecord{Name: "a1", Type: v1.AgentTypeOpenClaw, Phase: v1.AgentPhaseRunning})

	task := v1.TaskRecord{
		ID:          "task-hier",
		AgentName:   "a1",
		Message:     "hierarchical task",
		Status:      v1.TaskStatusPending,
		IssueID:     "issue-1",
		ProjectID:   "proj-1",
		GoalID:      "goal-1",
		LineageJSON: `["goal-1","proj-1","issue-1"]`,
	}
	if err := s.CreateTask(ctx, task); err != nil {
		t.Fatalf("create: %v", err)
	}

	got, _ := s.GetTask(ctx, "task-hier")
	if got.IssueID != "issue-1" || got.ProjectID != "proj-1" || got.GoalID != "goal-1" {
		t.Errorf("hierarchy fields: issue=%q project=%q goal=%q", got.IssueID, got.ProjectID, got.GoalID)
	}
	if got.LineageJSON != `["goal-1","proj-1","issue-1"]` {
		t.Errorf("LineageJSON = %q", got.LineageJSON)
	}
}

func TestTaskWithCost(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	s.CreateAgent(ctx, v1.AgentRecord{Name: "a1", Type: v1.AgentTypeOpenClaw, Phase: v1.AgentPhaseRunning})

	task := v1.TaskRecord{
		ID:        "task-cost",
		AgentName: "a1",
		Message:   "cost task",
		Status:    v1.TaskStatusCompleted,
		TokensIn:  500,
		TokensOut: 200,
		Cost:      0.05,
	}
	s.CreateTask(ctx, task)
	got, _ := s.GetTask(ctx, "task-cost")
	if got.Cost != 0.05 {
		t.Errorf("Cost = %f, want 0.05", got.Cost)
	}
}

func TestGetTask_NotFound(t *testing.T) {
	s := newTestStore(t)
	_, err := s.GetTask(context.Background(), "nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent task")
	}
}

func TestListTasksByAgent_Empty(t *testing.T) {
	s := newTestStore(t)
	tasks, err := s.ListTasksByAgent(context.Background(), "noagent")
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(tasks) != 0 {
		t.Errorf("expected 0, got %d", len(tasks))
	}
}

// --- Workflow CRUD ---

func TestWorkflowCRUD(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	wf := v1.WorkflowRecord{
		Name:     "build-pipeline",
		SpecYAML: "steps:\n  - name: build\n    agent: builder",
		Schedule: "0 * * * *",
		Enabled:  true,
	}

	// Create.
	if err := s.CreateWorkflow(ctx, wf); err != nil {
		t.Fatalf("create: %v", err)
	}

	// Get.
	got, err := s.GetWorkflow(ctx, "build-pipeline")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.Name != "build-pipeline" {
		t.Errorf("Name = %q", got.Name)
	}
	if got.Schedule != "0 * * * *" {
		t.Errorf("Schedule = %q", got.Schedule)
	}
	if !got.Enabled {
		t.Error("expected enabled")
	}
	if got.SpecYAML != wf.SpecYAML {
		t.Errorf("SpecYAML = %q", got.SpecYAML)
	}

	// List.
	wfs, err := s.ListWorkflows(ctx)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(wfs) != 1 {
		t.Errorf("expected 1, got %d", len(wfs))
	}

	// Update.
	got.Schedule = "*/5 * * * *"
	got.Enabled = false
	if err := s.UpdateWorkflow(ctx, got); err != nil {
		t.Fatalf("update: %v", err)
	}
	updated, _ := s.GetWorkflow(ctx, "build-pipeline")
	if updated.Schedule != "*/5 * * * *" {
		t.Errorf("Schedule after update = %q", updated.Schedule)
	}
	if updated.Enabled {
		t.Error("expected disabled after update")
	}

	// Delete.
	if err := s.DeleteWorkflow(ctx, "build-pipeline"); err != nil {
		t.Fatalf("delete: %v", err)
	}
	_, err = s.GetWorkflow(ctx, "build-pipeline")
	if err == nil {
		t.Error("expected error after delete")
	}
}

func TestDeleteWorkflow_NotFound(t *testing.T) {
	s := newTestStore(t)
	err := s.DeleteWorkflow(context.Background(), "nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent workflow")
	}
}

func TestGetWorkflow_NotFound(t *testing.T) {
	s := newTestStore(t)
	_, err := s.GetWorkflow(context.Background(), "nonexistent")
	if err == nil {
		t.Error("expected error")
	}
}

func TestWorkflowDuplicate(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	wf := v1.WorkflowRecord{Name: "dup-wf", SpecYAML: "v1", Enabled: true}
	s.CreateWorkflow(ctx, wf)
	err := s.CreateWorkflow(ctx, wf)
	if err == nil {
		t.Error("expected error on duplicate workflow")
	}
}

// --- Workflow Run CRUD ---

func TestWorkflowRunCRUD(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	// Create parent workflow.
	s.CreateWorkflow(ctx, v1.WorkflowRecord{Name: "wf1", SpecYAML: "v1", Enabled: true})

	run := v1.WorkflowRunRecord{
		ID:           "run-001",
		WorkflowName: "wf1",
		Status:       v1.WorkflowStatusPending,
		StepsJSON:    `{"build":"pending"}`,
		StartedAt:    time.Now(),
	}

	// Create.
	if err := s.CreateWorkflowRun(ctx, run); err != nil {
		t.Fatalf("create run: %v", err)
	}

	// Get.
	got, err := s.GetWorkflowRun(ctx, "run-001")
	if err != nil {
		t.Fatalf("get run: %v", err)
	}
	if got.WorkflowName != "wf1" {
		t.Errorf("WorkflowName = %q", got.WorkflowName)
	}
	if got.Status != v1.WorkflowStatusPending {
		t.Errorf("Status = %q", got.Status)
	}
	if got.StepsJSON != `{"build":"pending"}` {
		t.Errorf("StepsJSON = %q", got.StepsJSON)
	}
	if got.EndedAt != nil {
		t.Error("EndedAt should be nil for pending run")
	}

	// Update with ended_at.
	now := time.Now()
	got.Status = v1.WorkflowStatusCompleted
	got.StepsJSON = `{"build":"done"}`
	got.EndedAt = &now
	if err := s.UpdateWorkflowRun(ctx, got); err != nil {
		t.Fatalf("update run: %v", err)
	}
	updated, _ := s.GetWorkflowRun(ctx, "run-001")
	if updated.Status != v1.WorkflowStatusCompleted {
		t.Errorf("Status = %q", updated.Status)
	}
	if updated.EndedAt == nil {
		t.Error("EndedAt should be set after update")
	}

	// List.
	runs, err := s.ListWorkflowRuns(ctx, "wf1")
	if err != nil {
		t.Fatalf("list runs: %v", err)
	}
	if len(runs) != 1 {
		t.Errorf("expected 1, got %d", len(runs))
	}
}

func TestGetWorkflowRun_NotFound(t *testing.T) {
	s := newTestStore(t)
	_, err := s.GetWorkflowRun(context.Background(), "nonexistent")
	if err == nil {
		t.Error("expected error")
	}
}

func TestListWorkflowRuns_Empty(t *testing.T) {
	s := newTestStore(t)
	runs, err := s.ListWorkflowRuns(context.Background(), "nonexistent")
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(runs) != 0 {
		t.Errorf("expected 0, got %d", len(runs))
	}
}

func TestWorkflowRunMultiple(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	s.CreateWorkflow(ctx, v1.WorkflowRecord{Name: "multi-wf", SpecYAML: "v1", Enabled: true})

	for i := 0; i < 3; i++ {
		run := v1.WorkflowRunRecord{
			ID:           "run-" + string(rune('A'+i)),
			WorkflowName: "multi-wf",
			Status:       v1.WorkflowStatusRunning,
			StepsJSON:    "{}",
			StartedAt:    time.Now(),
		}
		s.CreateWorkflowRun(ctx, run)
	}

	runs, _ := s.ListWorkflowRuns(ctx, "multi-wf")
	if len(runs) != 3 {
		t.Errorf("expected 3, got %d", len(runs))
	}
}

// --- Goal CRUD ---

func TestGoalCRUD(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	goal := v1.GoalRecord{
		ID:                "goal-001",
		Name:              "Launch MVP",
		Description:       "Build and launch the minimum viable product",
		Owner:             "alice",
		Deadline:          "2026-04-01",
		Status:            "active",
		Phase:             v1.GoalPhaseActive,
		SpecYAML:          "kind: Goal",
		DecompositionPlan: `{"projects":["auth","dashboard"]}`,
		DecomposeCost:     0.10,
		TokensIn:          1000,
		TokensOut:         500,
		Cost:              0.50,
	}

	// Create.
	if err := s.CreateGoal(ctx, goal); err != nil {
		t.Fatalf("create: %v", err)
	}

	// Get.
	got, err := s.GetGoal(ctx, "goal-001")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.Name != "Launch MVP" {
		t.Errorf("Name = %q", got.Name)
	}
	if got.Description != "Build and launch the minimum viable product" {
		t.Errorf("Description = %q", got.Description)
	}
	if got.Phase != v1.GoalPhaseActive {
		t.Errorf("Phase = %q", got.Phase)
	}
	if got.DecompositionPlan != `{"projects":["auth","dashboard"]}` {
		t.Errorf("DecompositionPlan = %q", got.DecompositionPlan)
	}
	if got.DecomposeCost != 0.10 {
		t.Errorf("DecomposeCost = %f", got.DecomposeCost)
	}
	if got.TokensIn != 1000 {
		t.Errorf("TokensIn = %d", got.TokensIn)
	}
	if got.Cost != 0.50 {
		t.Errorf("Cost = %f", got.Cost)
	}

	// List.
	goals, err := s.ListGoals(ctx)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(goals) != 1 {
		t.Errorf("expected 1, got %d", len(goals))
	}
	if goals[0].Phase != v1.GoalPhaseActive {
		t.Errorf("listed Phase = %q", goals[0].Phase)
	}

	// Update.
	got.Name = "Launch MVP v2"
	got.Phase = v1.GoalPhaseInProgress
	got.Status = "in_progress"
	got.TokensIn = 2000
	if err := s.UpdateGoal(ctx, got); err != nil {
		t.Fatalf("update: %v", err)
	}
	updated, _ := s.GetGoal(ctx, "goal-001")
	if updated.Name != "Launch MVP v2" {
		t.Errorf("Name = %q", updated.Name)
	}
	if updated.Phase != v1.GoalPhaseInProgress {
		t.Errorf("Phase = %q", updated.Phase)
	}
	if updated.TokensIn != 2000 {
		t.Errorf("TokensIn = %d", updated.TokensIn)
	}

	// Delete.
	if err := s.DeleteGoal(ctx, "goal-001"); err != nil {
		t.Fatalf("delete: %v", err)
	}
	_, err = s.GetGoal(ctx, "goal-001")
	if err == nil {
		t.Error("expected error after delete")
	}
}

func TestGoalDefaultPhaseAndStatus(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	// Create with empty phase and status to test defaults.
	goal := v1.GoalRecord{
		ID:   "goal-defaults",
		Name: "Default Goal",
	}
	if err := s.CreateGoal(ctx, goal); err != nil {
		t.Fatalf("create: %v", err)
	}

	got, _ := s.GetGoal(ctx, "goal-defaults")
	if got.Status != "active" {
		t.Errorf("Status = %q, want 'active'", got.Status)
	}
	if got.Phase != "active" {
		t.Errorf("Phase = %q, want 'active'", got.Phase)
	}
}

func TestGetGoal_NotFound(t *testing.T) {
	s := newTestStore(t)
	_, err := s.GetGoal(context.Background(), "nonexistent")
	if err == nil {
		t.Error("expected error")
	}
}

func TestListGoals_Multiple(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	for i := 0; i < 3; i++ {
		s.CreateGoal(ctx, v1.GoalRecord{
			ID:   "g-" + string(rune('A'+i)),
			Name: "Goal " + string(rune('A'+i)),
		})
	}
	goals, _ := s.ListGoals(ctx)
	if len(goals) != 3 {
		t.Errorf("expected 3, got %d", len(goals))
	}
}

// --- GoalStats ---

func TestGoalStats(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	s.CreateGoal(ctx, v1.GoalRecord{ID: "g1", Name: "Goal 1"})
	s.CreateAgent(ctx, v1.AgentRecord{Name: "a1", Type: v1.AgentTypeOpenClaw, Phase: v1.AgentPhaseRunning})

	// Create tasks for this goal.
	s.CreateTask(ctx, v1.TaskRecord{
		ID: "t1", AgentName: "a1", Message: "task1", Status: v1.TaskStatusCompleted,
		GoalID: "g1", TokensIn: 100, TokensOut: 50, Cost: 0.01,
	})
	s.CreateTask(ctx, v1.TaskRecord{
		ID: "t2", AgentName: "a1", Message: "task2", Status: v1.TaskStatusFailed,
		GoalID: "g1", TokensIn: 200, TokensOut: 80, Cost: 0.02,
	})
	s.CreateTask(ctx, v1.TaskRecord{
		ID: "t3", AgentName: "a1", Message: "task3", Status: v1.TaskStatusCompleted,
		GoalID: "g1", TokensIn: 150, TokensOut: 60, Cost: 0.015,
	})

	stats, err := s.GoalStats(ctx, "g1")
	if err != nil {
		t.Fatalf("GoalStats: %v", err)
	}
	if stats.TaskCount != 3 {
		t.Errorf("TaskCount = %d, want 3", stats.TaskCount)
	}
	if stats.CompletedTasks != 2 {
		t.Errorf("CompletedTasks = %d, want 2", stats.CompletedTasks)
	}
	if stats.FailedTasks != 1 {
		t.Errorf("FailedTasks = %d, want 1", stats.FailedTasks)
	}
	if stats.TotalTokensIn != 450 {
		t.Errorf("TotalTokensIn = %d, want 450", stats.TotalTokensIn)
	}
	if stats.TotalTokensOut != 190 {
		t.Errorf("TotalTokensOut = %d, want 190", stats.TotalTokensOut)
	}
}

func TestGoalStats_Empty(t *testing.T) {
	s := newTestStore(t)
	stats, err := s.GoalStats(context.Background(), "nonexistent")
	if err != nil {
		t.Fatalf("GoalStats: %v", err)
	}
	if stats.TaskCount != 0 {
		t.Errorf("TaskCount = %d, want 0", stats.TaskCount)
	}
}

// --- Project CRUD ---

func TestProjectCRUD(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	proj := v1.ProjectRecord{
		ID:          "proj-001",
		Name:        "auth-service",
		GoalID:      "goal-001",
		Description: "Authentication microservice",
		Status:      "active",
		SpecYAML:    "kind: Project",
	}

	// Create.
	if err := s.CreateProject(ctx, proj); err != nil {
		t.Fatalf("create: %v", err)
	}

	// Get.
	got, err := s.GetProject(ctx, "proj-001")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.Name != "auth-service" {
		t.Errorf("Name = %q", got.Name)
	}
	if got.GoalID != "goal-001" {
		t.Errorf("GoalID = %q", got.GoalID)
	}
	if got.Status != "active" {
		t.Errorf("Status = %q", got.Status)
	}

	// List.
	projs, err := s.ListProjects(ctx)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(projs) != 1 {
		t.Errorf("expected 1, got %d", len(projs))
	}

	// ListByGoal.
	byGoal, err := s.ListProjectsByGoal(ctx, "goal-001")
	if err != nil {
		t.Fatalf("list by goal: %v", err)
	}
	if len(byGoal) != 1 {
		t.Errorf("expected 1, got %d", len(byGoal))
	}

	// Update.
	got.Status = "completed"
	got.TokensIn = 500
	got.TokensOut = 200
	got.Cost = 0.10
	if err := s.UpdateProject(ctx, got); err != nil {
		t.Fatalf("update: %v", err)
	}
	updated, _ := s.GetProject(ctx, "proj-001")
	if updated.Status != "completed" {
		t.Errorf("Status = %q", updated.Status)
	}
	if updated.TokensIn != 500 {
		t.Errorf("TokensIn = %d", updated.TokensIn)
	}
	if updated.Cost != 0.10 {
		t.Errorf("Cost = %f", updated.Cost)
	}

	// Delete.
	if err := s.DeleteProject(ctx, "proj-001"); err != nil {
		t.Fatalf("delete: %v", err)
	}
	_, err = s.GetProject(ctx, "proj-001")
	if err == nil {
		t.Error("expected error after delete")
	}
}

func TestGetProject_NotFound(t *testing.T) {
	s := newTestStore(t)
	_, err := s.GetProject(context.Background(), "nonexistent")
	if err == nil {
		t.Error("expected error")
	}
}

func TestListProjectsByGoal_Empty(t *testing.T) {
	s := newTestStore(t)
	projs, err := s.ListProjectsByGoal(context.Background(), "nogoal")
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(projs) != 0 {
		t.Errorf("expected 0, got %d", len(projs))
	}
}

func TestProjectDuplicate(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	proj := v1.ProjectRecord{ID: "dup-proj", Name: "dup", GoalID: "g1", Status: "active"}
	s.CreateProject(ctx, proj)
	err := s.CreateProject(ctx, proj)
	if err == nil {
		t.Error("expected error on duplicate project")
	}
}

// --- ProjectStats ---

func TestProjectStats(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	s.CreateAgent(ctx, v1.AgentRecord{Name: "a1", Type: v1.AgentTypeOpenClaw, Phase: v1.AgentPhaseRunning})

	s.CreateTask(ctx, v1.TaskRecord{
		ID: "t1", AgentName: "a1", Message: "task1", Status: v1.TaskStatusCompleted,
		ProjectID: "proj-1", TokensIn: 100, TokensOut: 50, Cost: 0.01,
	})
	s.CreateTask(ctx, v1.TaskRecord{
		ID: "t2", AgentName: "a1", Message: "task2", Status: v1.TaskStatusFailed,
		ProjectID: "proj-1", TokensIn: 200, TokensOut: 80, Cost: 0.02,
	})

	stats, err := s.ProjectStats(ctx, "proj-1")
	if err != nil {
		t.Fatalf("ProjectStats: %v", err)
	}
	if stats.TaskCount != 2 {
		t.Errorf("TaskCount = %d, want 2", stats.TaskCount)
	}
	if stats.CompletedTasks != 1 {
		t.Errorf("CompletedTasks = %d, want 1", stats.CompletedTasks)
	}
	if stats.FailedTasks != 1 {
		t.Errorf("FailedTasks = %d, want 1", stats.FailedTasks)
	}
	if stats.TotalTokensIn != 300 {
		t.Errorf("TotalTokensIn = %d, want 300", stats.TotalTokensIn)
	}
}

func TestProjectStats_Empty(t *testing.T) {
	s := newTestStore(t)
	stats, err := s.ProjectStats(context.Background(), "nonexistent")
	if err != nil {
		t.Fatalf("ProjectStats: %v", err)
	}
	if stats.TaskCount != 0 {
		t.Errorf("TaskCount = %d", stats.TaskCount)
	}
}

// --- Issue CRUD ---

func TestIssueCRUD(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	issue := v1.IssueRecord{
		ID:          "issue-001",
		Name:        "Fix login bug",
		ProjectID:   "proj-001",
		Description: "Users cannot login with SSO",
		AgentRef:    "agent-coder",
		Status:      "open",
		SpecYAML:    "kind: Issue",
		GoalID:      "goal-001",
		TraceID:     "trace-abc",
		SpanID:      "span-def",
		ParentSpans: []string{"span-parent1", "span-parent2"},
		LineageJSON: `["goal-001","proj-001"]`,
	}

	// Create.
	if err := s.CreateIssue(ctx, issue); err != nil {
		t.Fatalf("create: %v", err)
	}

	// Get.
	got, err := s.GetIssue(ctx, "issue-001")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.Name != "Fix login bug" {
		t.Errorf("Name = %q", got.Name)
	}
	if got.ProjectID != "proj-001" {
		t.Errorf("ProjectID = %q", got.ProjectID)
	}
	if got.AgentRef != "agent-coder" {
		t.Errorf("AgentRef = %q", got.AgentRef)
	}
	if got.GoalID != "goal-001" {
		t.Errorf("GoalID = %q", got.GoalID)
	}
	if got.TraceID != "trace-abc" {
		t.Errorf("TraceID = %q", got.TraceID)
	}
	if got.SpanID != "span-def" {
		t.Errorf("SpanID = %q", got.SpanID)
	}
	if len(got.ParentSpans) != 2 || got.ParentSpans[0] != "span-parent1" {
		t.Errorf("ParentSpans = %v", got.ParentSpans)
	}
	if got.LineageJSON != `["goal-001","proj-001"]` {
		t.Errorf("LineageJSON = %q", got.LineageJSON)
	}

	// List.
	issues, err := s.ListIssues(ctx)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(issues) != 1 {
		t.Errorf("expected 1, got %d", len(issues))
	}

	// ListByProject.
	byProject, err := s.ListIssuesByProject(ctx, "proj-001")
	if err != nil {
		t.Fatalf("list by project: %v", err)
	}
	if len(byProject) != 1 {
		t.Errorf("expected 1, got %d", len(byProject))
	}

	// Update.
	got.Status = "closed"
	got.AgentRef = "agent-reviewer"
	got.TokensIn = 100
	got.TokensOut = 50
	got.Cost = 0.01
	if err := s.UpdateIssue(ctx, got); err != nil {
		t.Fatalf("update: %v", err)
	}
	updated, _ := s.GetIssue(ctx, "issue-001")
	if updated.Status != "closed" {
		t.Errorf("Status = %q", updated.Status)
	}
	if updated.AgentRef != "agent-reviewer" {
		t.Errorf("AgentRef = %q", updated.AgentRef)
	}
	if updated.TokensIn != 100 {
		t.Errorf("TokensIn = %d", updated.TokensIn)
	}

	// Delete.
	if err := s.DeleteIssue(ctx, "issue-001"); err != nil {
		t.Fatalf("delete: %v", err)
	}
	_, err = s.GetIssue(ctx, "issue-001")
	if err == nil {
		t.Error("expected error after delete")
	}
}

func TestIssueNilParentSpans(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	issue := v1.IssueRecord{
		ID:        "issue-nil-spans",
		Name:      "No parent spans",
		ProjectID: "proj-1",
		Status:    "open",
	}
	if err := s.CreateIssue(ctx, issue); err != nil {
		t.Fatalf("create: %v", err)
	}
	got, _ := s.GetIssue(ctx, "issue-nil-spans")
	// ParentSpans should be empty (not nil panic).
	if len(got.ParentSpans) != 0 {
		t.Errorf("ParentSpans = %v, want empty", got.ParentSpans)
	}
}

func TestIssueEmptyLineage(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	issue := v1.IssueRecord{
		ID:        "issue-no-lineage",
		Name:      "No lineage",
		ProjectID: "proj-1",
		Status:    "open",
	}
	s.CreateIssue(ctx, issue)
	got, _ := s.GetIssue(ctx, "issue-no-lineage")
	if got.LineageJSON != "[]" {
		t.Errorf("LineageJSON = %q, want '[]'", got.LineageJSON)
	}
}

func TestUpdateIssueNilParentSpans(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	issue := v1.IssueRecord{
		ID: "issue-update-nil", Name: "test", ProjectID: "p1", Status: "open",
		ParentSpans: []string{"s1"},
	}
	s.CreateIssue(ctx, issue)

	// Update with nil ParentSpans.
	issue.ParentSpans = nil
	issue.LineageJSON = ""
	s.UpdateIssue(ctx, issue)

	got, _ := s.GetIssue(ctx, "issue-update-nil")
	if len(got.ParentSpans) != 0 {
		t.Errorf("ParentSpans = %v", got.ParentSpans)
	}
	if got.LineageJSON != "[]" {
		t.Errorf("LineageJSON = %q", got.LineageJSON)
	}
}

func TestGetIssue_NotFound(t *testing.T) {
	s := newTestStore(t)
	_, err := s.GetIssue(context.Background(), "nonexistent")
	if err == nil {
		t.Error("expected error")
	}
}

func TestListIssuesByProject_Empty(t *testing.T) {
	s := newTestStore(t)
	issues, err := s.ListIssuesByProject(context.Background(), "noproject")
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(issues) != 0 {
		t.Errorf("expected 0, got %d", len(issues))
	}
}

func TestListIssues_Multiple(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	for i := 0; i < 3; i++ {
		s.CreateIssue(ctx, v1.IssueRecord{
			ID: "i-" + string(rune('A'+i)), Name: "Issue " + string(rune('A'+i)),
			ProjectID: "proj-1", Status: "open",
		})
	}
	issues, _ := s.ListIssues(ctx)
	if len(issues) != 3 {
		t.Errorf("expected 3, got %d", len(issues))
	}
}

// --- Persistence ---

func TestPersistence(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "persist.db")

	s1, err := New(dbPath)
	if err != nil {
		t.Fatalf("new: %v", err)
	}
	s1.CreateAgent(context.Background(), v1.AgentRecord{Name: "persist-agent", Type: v1.AgentTypeOpenClaw, Phase: v1.AgentPhaseCreated})
	s1.Close()

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

// --- Federated Goal Run CRUD ---

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

	if err := s.SaveFederatedGoalRun(ctx, run); err != nil {
		t.Fatalf("save: %v", err)
	}

	got, err := s.GetFederatedGoalRun(ctx, "goal-001")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.GoalName != "test federated goal" || got.Status != "InProgress" {
		t.Errorf("got %+v", got)
	}

	active, err := s.ListActiveFederatedGoalRuns(ctx)
	if err != nil {
		t.Fatalf("list active: %v", err)
	}
	if len(active) != 1 {
		t.Errorf("expected 1 active, got %d", len(active))
	}

	if err := s.UpdateFederatedGoalRunStatus(ctx, "goal-001", "Completed"); err != nil {
		t.Fatalf("update status: %v", err)
	}
	got2, _ := s.GetFederatedGoalRun(ctx, "goal-001")
	if got2.Status != "Completed" {
		t.Errorf("expected Completed, got %s", got2.Status)
	}

	active2, _ := s.ListActiveFederatedGoalRuns(ctx)
	if len(active2) != 0 {
		t.Errorf("expected 0 active after completion, got %d", len(active2))
	}

	if err := s.DeleteFederatedGoalRun(ctx, "goal-001"); err != nil {
		t.Fatalf("delete: %v", err)
	}
	_, err = s.GetFederatedGoalRun(ctx, "goal-001")
	if err == nil {
		t.Error("expected error after delete")
	}
}

// --- Federated Goal Project CRUD ---

func TestFederatedGoalProjectCRUD(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	run := storage.FederatedGoalRunRecord{
		GoalID:      "goal-002",
		GoalName:    "multi-project goal",
		Status:      "InProgress",
		ResultsJSON: "{}",
	}
	s.SaveFederatedGoalRun(ctx, run)

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

	projects, err := s.ListFederatedGoalProjects(ctx, "goal-002")
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(projects) != 2 {
		t.Fatalf("expected 2, got %d", len(projects))
	}
	if projects[0].ProjectName != "ui-design" || projects[0].Layer != 0 {
		t.Errorf("first project should be ui-design layer 0, got %s layer %d", projects[0].ProjectName, projects[0].Layer)
	}
	if projects[1].ProjectName != "api-impl" || projects[1].Layer != 1 {
		t.Errorf("second project should be api-impl layer 1, got %s layer %d", projects[1].ProjectName, projects[1].Layer)
	}

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

	s1, _ := New(dbPath)
	s1.Close()
	s2, err := New(dbPath)
	if err != nil {
		t.Fatalf("second open should succeed: %v", err)
	}
	s2.Close()

	if _, err := os.Stat(dbPath); err != nil {
		t.Fatalf("db file should exist: %v", err)
	}
}

// --- Store interface compliance ---

func TestStore_ImplementsInterface(t *testing.T) {
	s := newTestStore(t)
	var _ storage.Store = s
}

// --- New with invalid path ---

func TestNew_InvalidPath(t *testing.T) {
	// Attempt to create a DB in a nonexistent directory should fail at migrate.
	_, err := New("/nonexistent/dir/test.db")
	if err == nil {
		t.Error("expected error for invalid path")
	}
}

func TestGetFederatedGoalRun_NotFound(t *testing.T) {
	s := newTestStore(t)
	_, err := s.GetFederatedGoalRun(context.Background(), "nonexistent")
	if err == nil {
		t.Error("expected error")
	}
}

func TestListFederatedGoalProjects_Empty(t *testing.T) {
	s := newTestStore(t)
	projs, err := s.ListFederatedGoalProjects(context.Background(), "nonexistent")
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(projs) != 0 {
		t.Errorf("expected 0, got %d", len(projs))
	}
}

func TestMultipleAgentListOrder(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	for _, name := range []string{"alpha", "beta", "gamma"} {
		s.CreateAgent(ctx, v1.AgentRecord{Name: name, Type: v1.AgentTypeOpenClaw, Phase: v1.AgentPhaseCreated})
	}
	agents, _ := s.ListAgents(ctx)
	if len(agents) != 3 {
		t.Errorf("expected 3, got %d", len(agents))
	}
}

func TestTaskStartedAt(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	s.CreateAgent(ctx, v1.AgentRecord{Name: "a1", Type: v1.AgentTypeOpenClaw, Phase: v1.AgentPhaseRunning})

	now := time.Now()
	task := v1.TaskRecord{
		ID: "task-started", AgentName: "a1", Message: "with start time",
		Status: v1.TaskStatusRunning, StartedAt: &now,
	}
	s.CreateTask(ctx, task)

	got, _ := s.GetTask(ctx, "task-started")
	if got.StartedAt == nil {
		t.Error("StartedAt should not be nil")
	}
}
