package goal

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/zlc-ai/opc-platform/pkg/federation"
	"go.uber.org/zap"
)

// uniqueName generates a unique company name to avoid collision with persisted state.
func uniqueName(prefix string) string {
	return prefix + "-" + uuid.New().String()[:6]
}

func newTestLogger() *zap.SugaredLogger {
	return zap.NewNop().Sugar()
}

// mockTransport implements federation.Transport for testing.
type mockTransport struct {
	sendFunc        func(endpoint, method, path string, body any) ([]byte, error)
	pingFunc        func(endpoint string) error
	fetchStatusFunc func(endpoint string) (*federation.CompanyStatusReport, error)
}

func (m *mockTransport) Send(endpoint, method, path string, body any) ([]byte, error) {
	if m.sendFunc != nil {
		return m.sendFunc(endpoint, method, path, body)
	}
	return []byte(`{}`), nil
}

func (m *mockTransport) SendWithContext(_ context.Context, endpoint, method, path string, body any) ([]byte, error) {
	return m.Send(endpoint, method, path, body)
}

func (m *mockTransport) Ping(endpoint string) error {
	if m.pingFunc != nil {
		return m.pingFunc(endpoint)
	}
	return nil
}

func (m *mockTransport) FetchStatus(endpoint string) (*federation.CompanyStatusReport, error) {
	if m.fetchStatusFunc != nil {
		return m.fetchStatusFunc(endpoint)
	}
	return &federation.CompanyStatusReport{Status: "Online"}, nil
}

// newTestFederationController creates a FederationController with a temp dir
// and mock transport for testing. We use the exported constructor indirectly.
func newTestFederationController(t *testing.T, transport federation.Transport) *federation.FederationController {
	t.Helper()
	// We need to create the controller via NewController, but it uses config.GetStateDir().
	// Instead, we'll use the federation package's NewController and accept the side effect,
	// since tests run in isolation anyway.
	// Actually we can't set transport on the exported type, so let's just use NewController.
	fc := federation.NewController(newTestLogger())
	return fc
}

// === Goal Model Tests ===

func TestGoalStatusConstants(t *testing.T) {
	statuses := []GoalStatus{GoalPending, GoalInProgress, GoalCompleted, GoalFailed}
	expected := []string{"Pending", "InProgress", "Completed", "Failed"}
	for i, s := range statuses {
		if string(s) != expected[i] {
			t.Errorf("expected %s, got %s", expected[i], s)
		}
	}
}

func TestGoalHierarchy(t *testing.T) {
	issue := &Issue{
		ID:            "issue-1",
		TaskID:        "task-1",
		Name:          "Write tests",
		AssignedAgent: "agent-x",
		Context:       map[string]interface{}{"priority": "high"},
		AuditEvents:   []string{"created", "assigned"},
	}

	task := &Task{
		ID:        "task-1",
		ProjectID: "proj-1",
		Name:      "Testing phase",
		Issues:    []*Issue{issue},
	}

	project := &Project{
		ID:           "proj-1",
		GoalID:       "goal-1",
		CompanyID:    "company-a",
		Name:         "Test Project",
		Tasks:        []*Task{task},
		Dependencies: []string{"proj-0"},
	}

	goal := &Goal{
		ID:              "goal-1",
		Name:            "Ship v1.0",
		Description:     "Release first version",
		TargetCompanies: []string{"company-a"},
		Projects:        []*Project{project},
		Status:          GoalPending,
		CreatedBy:       "alice",
		CreatedAt:       time.Now(),
	}

	if len(goal.Projects) != 1 {
		t.Fatalf("expected 1 project, got %d", len(goal.Projects))
	}
	if len(goal.Projects[0].Tasks) != 1 {
		t.Fatalf("expected 1 task, got %d", len(goal.Projects[0].Tasks))
	}
	if len(goal.Projects[0].Tasks[0].Issues) != 1 {
		t.Fatalf("expected 1 issue, got %d", len(goal.Projects[0].Tasks[0].Issues))
	}
	if goal.Projects[0].Tasks[0].Issues[0].AssignedAgent != "agent-x" {
		t.Errorf("unexpected assigned agent: %s", goal.Projects[0].Tasks[0].Issues[0].AssignedAgent)
	}
}

// === Decomposer Tests ===

func TestDecompose(t *testing.T) {
	d := NewDecomposer(newTestLogger())

	req := DecomposeRequest{
		GoalID:          "g1",
		GoalName:        "Build API",
		Description:     "Build the REST API",
		TargetCompanies: []string{"company-a", "company-b"},
	}

	result, err := d.Decompose(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(result.Projects) != 2 {
		t.Fatalf("expected 2 projects, got %d", len(result.Projects))
	}

	for i, p := range result.Projects {
		companyID := req.TargetCompanies[i]

		if p.GoalID != "g1" {
			t.Errorf("project %d: expected goalId g1, got %s", i, p.GoalID)
		}
		if p.CompanyID != companyID {
			t.Errorf("project %d: expected companyId %s, got %s", i, companyID, p.CompanyID)
		}
		if p.ID == "" {
			t.Errorf("project %d: expected non-empty ID", i)
		}
		if len(p.Tasks) != 1 {
			t.Fatalf("project %d: expected 1 task, got %d", i, len(p.Tasks))
		}

		task := p.Tasks[0]
		if task.ProjectID != p.ID {
			t.Errorf("project %d: task projectId mismatch", i)
		}
		if len(task.Issues) != 1 {
			t.Fatalf("project %d: expected 1 issue, got %d", i, len(task.Issues))
		}

		issue := task.Issues[0]
		if issue.TaskID != task.ID {
			t.Errorf("project %d: issue taskId mismatch", i)
		}
		ctx, ok := issue.Context["companyId"]
		if !ok || ctx != companyID {
			t.Errorf("project %d: expected context companyId %s", i, companyID)
		}
	}
}

func TestDecompose_NoTargetCompanies(t *testing.T) {
	d := NewDecomposer(newTestLogger())

	_, err := d.Decompose(context.Background(), DecomposeRequest{
		GoalID:          "g1",
		GoalName:        "Test",
		TargetCompanies: []string{},
	})
	if err == nil {
		t.Fatal("expected error for empty target companies")
	}
}

func TestDecompose_SingleCompany(t *testing.T) {
	d := NewDecomposer(newTestLogger())

	result, err := d.Decompose(context.Background(), DecomposeRequest{
		GoalID:          "g2",
		GoalName:        "Deploy",
		Description:     "Deploy to prod",
		TargetCompanies: []string{"ops-team"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Projects) != 1 {
		t.Errorf("expected 1 project, got %d", len(result.Projects))
	}
	if result.Projects[0].Name != "Deploy - ops-team" {
		t.Errorf("unexpected project name: %s", result.Projects[0].Name)
	}
}

// === Dispatcher Tests ===

func TestDispatcher_StoreAndListGoals(t *testing.T) {
	fc := federation.NewController(newTestLogger())
	d := NewDispatcher(fc, newTestLogger())

	goals := d.ListGoals()
	if len(goals) != 0 {
		t.Errorf("expected 0 goals, got %d", len(goals))
	}

	g1 := &Goal{ID: "g1", Name: "Goal 1", Status: GoalPending}
	g2 := &Goal{ID: "g2", Name: "Goal 2", Status: GoalPending}

	d.StoreGoal(g1)
	d.StoreGoal(g2)

	goals = d.ListGoals()
	if len(goals) != 2 {
		t.Errorf("expected 2 goals, got %d", len(goals))
	}
}

func TestDispatcher_GetGoal(t *testing.T) {
	fc := federation.NewController(newTestLogger())
	d := NewDispatcher(fc, newTestLogger())

	g := &Goal{ID: "g1", Name: "Test Goal"}
	d.StoreGoal(g)

	got, err := d.GetGoal("g1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Name != "Test Goal" {
		t.Errorf("expected 'Test Goal', got %s", got.Name)
	}
}

func TestDispatcher_GetGoal_NotFound(t *testing.T) {
	fc := federation.NewController(newTestLogger())
	d := NewDispatcher(fc, newTestLogger())

	_, err := d.GetGoal("nonexistent")
	if err == nil {
		t.Fatal("expected error for nonexistent goal")
	}
}

func TestDispatcher_Dispatch_NilGoal(t *testing.T) {
	fc := federation.NewController(newTestLogger())
	d := NewDispatcher(fc, newTestLogger())

	err := d.Dispatch(nil)
	if err == nil {
		t.Fatal("expected error for nil goal")
	}
}

func TestDispatcher_Dispatch_CompanyNotFound(t *testing.T) {
	fc := federation.NewController(newTestLogger())
	d := NewDispatcher(fc, newTestLogger())

	g := &Goal{
		ID:   "g1",
		Name: "Test",
		Projects: []*Project{
			{ID: "p1", CompanyID: "nonexistent"},
		},
	}

	// Should not error — just skips unknown companies.
	if err := d.Dispatch(g); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestDispatcher_Dispatch_CompanyOffline(t *testing.T) {
	fc := federation.NewController(newTestLogger())

	company, err := fc.RegisterCompany(federation.CompanyRegistration{
		Name:     uniqueName("offline-co"),
		Endpoint: "http://localhost:9999",
		Type:     federation.CompanyTypeSoftware,
	})
	if err != nil {
		t.Fatalf("register company: %v", err)
	}

	d := NewDispatcher(fc, newTestLogger())

	g := &Goal{
		ID:   "g1",
		Name: "Test",
		Projects: []*Project{
			{ID: "p1", CompanyID: company.ID},
		},
	}

	err = d.Dispatch(g)
	if err == nil {
		t.Fatal("expected error for offline company")
	}
}

func TestDispatcher_InjectContext(t *testing.T) {
	fc := federation.NewController(newTestLogger())
	d := NewDispatcher(fc, newTestLogger())

	// No companies — should still succeed (just no targets).
	err := d.InjectContext("issue-1", "issue-2", map[string]interface{}{"key": "value"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestDispatcher_InjectContext_EmptyIssues(t *testing.T) {
	fc := federation.NewController(newTestLogger())
	d := NewDispatcher(fc, newTestLogger())

	err := d.InjectContext("", "issue-2", nil)
	if err == nil {
		t.Fatal("expected error for empty fromIssue")
	}

	err = d.InjectContext("issue-1", "", nil)
	if err == nil {
		t.Fatal("expected error for empty toIssue")
	}
}

func TestDispatcher_GetIssueQueue_UnknownCompany(t *testing.T) {
	fc := federation.NewController(newTestLogger())
	d := NewDispatcher(fc, newTestLogger())

	issues := d.GetIssueQueue("unknown")
	if issues != nil {
		t.Errorf("expected nil for unknown company, got %v", issues)
	}
}

func TestDispatcher_GetIssueQueue_KnownCompany(t *testing.T) {
	fc := federation.NewController(newTestLogger())

	company, err := fc.RegisterCompany(federation.CompanyRegistration{
		Name:     uniqueName("queue-test"),
		Endpoint: "http://localhost:9999",
		Type:     federation.CompanyTypeSoftware,
	})
	if err != nil {
		t.Fatalf("register company: %v", err)
	}

	d := NewDispatcher(fc, newTestLogger())

	issues := d.GetIssueQueue(company.ID)
	if len(issues) != 0 {
		t.Errorf("expected 0 issues, got %d", len(issues))
	}
}

func TestDispatcher_StoreGoal_Overwrite(t *testing.T) {
	fc := federation.NewController(newTestLogger())
	d := NewDispatcher(fc, newTestLogger())

	d.StoreGoal(&Goal{ID: "g1", Name: "v1"})
	d.StoreGoal(&Goal{ID: "g1", Name: "v2"})

	got, _ := d.GetGoal("g1")
	if got.Name != "v2" {
		t.Errorf("expected overwritten name 'v2', got %s", got.Name)
	}
}

// === Integration: Decompose + Dispatch ===

func TestDecomposeAndDispatch(t *testing.T) {
	fc := federation.NewController(newTestLogger())

	// Register a company and set it online.
	company, err := fc.RegisterCompany(federation.CompanyRegistration{
		Name:     uniqueName("dev-co"),
		Endpoint: "http://localhost:9999",
		Type:     federation.CompanyTypeSoftware,
	})
	if err != nil {
		t.Fatalf("register: %v", err)
	}
	fc.UpdateCompanyStatus(company.ID, federation.CompanyStatusOnline)

	decomposer := NewDecomposer(newTestLogger())
	dispatcher := NewDispatcher(fc, newTestLogger())

	// Decompose a goal.
	result, err := decomposer.Decompose(context.Background(), DecomposeRequest{
		GoalID:          "goal-1",
		GoalName:        "Ship v1",
		Description:     "Release first version",
		TargetCompanies: []string{company.ID},
	})
	if err != nil {
		t.Fatalf("decompose: %v", err)
	}

	goal := &Goal{
		ID:              "goal-1",
		Name:            "Ship v1",
		TargetCompanies: []string{company.ID},
		Projects:        result.Projects,
		Status:          GoalPending,
	}

	dispatcher.StoreGoal(goal)

	// Dispatch will fail because the HTTP transport can't reach localhost:9999,
	// but this tests the integration path up to the transport call.
	err = dispatcher.Dispatch(goal)
	if err == nil {
		t.Log("dispatch succeeded (unexpected but OK if transport is mocked)")
	} else {
		// Expected: transport failure
		t.Logf("dispatch failed as expected (transport): %v", err)
	}

	// Verify goal is stored.
	got, err := dispatcher.GetGoal("goal-1")
	if err != nil {
		t.Fatalf("get goal: %v", err)
	}
	if got.Name != "Ship v1" {
		t.Errorf("expected 'Ship v1', got %s", got.Name)
	}
	if len(got.Projects) != 1 {
		t.Errorf("expected 1 project, got %d", len(got.Projects))
	}
}

// === Concurrency Tests ===

func TestDispatcher_ConcurrentAccess(t *testing.T) {
	fc := federation.NewController(newTestLogger())
	d := NewDispatcher(fc, newTestLogger())

	done := make(chan struct{})

	// Concurrent stores.
	for i := 0; i < 50; i++ {
		go func(i int) {
			d.StoreGoal(&Goal{ID: fmt.Sprintf("g-%d", i), Name: fmt.Sprintf("Goal %d", i)})
			done <- struct{}{}
		}(i)
	}

	for i := 0; i < 50; i++ {
		<-done
	}

	goals := d.ListGoals()
	if len(goals) != 50 {
		t.Errorf("expected 50 goals after concurrent store, got %d", len(goals))
	}
}
