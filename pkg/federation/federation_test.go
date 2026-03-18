package federation

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"go.uber.org/zap"
)

// newTestLogger returns a no-op sugared logger for tests.
func newTestLogger() *zap.SugaredLogger {
	return zap.NewNop().Sugar()
}

// newTestController creates a FederationController backed by a temp directory
// and a mock transport.
func newTestController(t *testing.T, transport Transport) *FederationController {
	t.Helper()
	stateDir := t.TempDir()
	fc := &FederationController{
		companies: make(map[string]*Company),
		transport: transport,
		logger:    newTestLogger(),
		stateDir:  stateDir,
	}
	return fc
}

// --- Mock Transport ---

type mockTransport struct {
	sendFunc        func(endpoint, method, path string, body any) ([]byte, error)
	pingFunc        func(endpoint string) error
	fetchStatusFunc func(endpoint string) (*CompanyStatusReport, error)
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

func (m *mockTransport) FetchStatus(endpoint string) (*CompanyStatusReport, error) {
	if m.fetchStatusFunc != nil {
		return m.fetchStatusFunc(endpoint)
	}
	return &CompanyStatusReport{Status: "Online"}, nil
}

// === Company Tests ===

func TestCompanyStatusConstants(t *testing.T) {
	if CompanyStatusOnline != "Online" {
		t.Errorf("expected Online, got %s", CompanyStatusOnline)
	}
	if CompanyStatusOffline != "Offline" {
		t.Errorf("expected Offline, got %s", CompanyStatusOffline)
	}
	if CompanyStatusBusy != "Busy" {
		t.Errorf("expected Busy, got %s", CompanyStatusBusy)
	}
}

func TestCompanyTypeConstants(t *testing.T) {
	types := []CompanyType{CompanyTypeSoftware, CompanyTypeOperations, CompanyTypeSales, CompanyTypeCustom}
	expected := []string{"software", "operations", "sales", "custom"}
	for i, ct := range types {
		if string(ct) != expected[i] {
			t.Errorf("expected %s, got %s", expected[i], ct)
		}
	}
}

// === FederationController Tests ===

func TestRegisterCompany(t *testing.T) {
	fc := newTestController(t, &mockTransport{})

	reg := CompanyRegistration{
		Name:     "dev-team",
		Endpoint: "http://localhost:8080",
		Type:     CompanyTypeSoftware,
		Agents:   []string{"agent-1"},
	}

	company, err := fc.RegisterCompany(reg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if company.Name != "dev-team" {
		t.Errorf("expected name dev-team, got %s", company.Name)
	}
	// Mock transport Ping returns nil (success), so initial status should be Online.
	if company.Status != CompanyStatusOnline {
		t.Errorf("expected status Online, got %s", company.Status)
	}
	if company.Type != CompanyTypeSoftware {
		t.Errorf("expected type software, got %s", company.Type)
	}
	if len(company.Agents) != 1 || company.Agents[0] != "agent-1" {
		t.Errorf("unexpected agents: %v", company.Agents)
	}
	if company.ID == "" {
		t.Error("expected non-empty ID")
	}
}

func TestRegisterCompany_DuplicateName(t *testing.T) {
	fc := newTestController(t, &mockTransport{})

	reg := CompanyRegistration{Name: "dup", Endpoint: "http://a", Type: CompanyTypeSoftware}
	if _, err := fc.RegisterCompany(reg); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	_, err := fc.RegisterCompany(reg)
	if err == nil {
		t.Fatal("expected error for duplicate name")
	}
}

func TestUnregisterCompany(t *testing.T) {
	fc := newTestController(t, &mockTransport{})

	company, _ := fc.RegisterCompany(CompanyRegistration{Name: "to-remove", Endpoint: "http://a", Type: CompanyTypeSoftware})

	if err := fc.UnregisterCompany(company.ID); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	_, err := fc.GetCompany(company.ID)
	if err == nil {
		t.Fatal("expected error after unregister")
	}
}

func TestUnregisterCompany_NotFound(t *testing.T) {
	fc := newTestController(t, &mockTransport{})

	err := fc.UnregisterCompany("nonexistent")
	if err == nil {
		t.Fatal("expected error for nonexistent company")
	}
}

func TestGetCompany(t *testing.T) {
	fc := newTestController(t, &mockTransport{})

	company, _ := fc.RegisterCompany(CompanyRegistration{Name: "get-test", Endpoint: "http://a", Type: CompanyTypeSoftware})

	got, err := fc.GetCompany(company.ID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Name != "get-test" {
		t.Errorf("expected get-test, got %s", got.Name)
	}
}

func TestGetCompany_NotFound(t *testing.T) {
	fc := newTestController(t, &mockTransport{})

	_, err := fc.GetCompany("missing")
	if err == nil {
		t.Fatal("expected error for missing company")
	}
}

func TestFindCompanyByName(t *testing.T) {
	fc := newTestController(t, &mockTransport{})

	fc.RegisterCompany(CompanyRegistration{Name: "find-me", Endpoint: "http://a", Type: CompanyTypeSales})

	company, err := fc.FindCompanyByName("find-me")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if company.Name != "find-me" {
		t.Errorf("expected find-me, got %s", company.Name)
	}
}

func TestFindCompanyByName_NotFound(t *testing.T) {
	fc := newTestController(t, &mockTransport{})

	_, err := fc.FindCompanyByName("ghost")
	if err == nil {
		t.Fatal("expected error for missing company")
	}
}

func TestListCompanies(t *testing.T) {
	fc := newTestController(t, &mockTransport{})

	fc.RegisterCompany(CompanyRegistration{Name: "a", Endpoint: "http://a", Type: CompanyTypeSoftware})
	fc.RegisterCompany(CompanyRegistration{Name: "b", Endpoint: "http://b", Type: CompanyTypeOperations})

	companies := fc.ListCompanies()
	if len(companies) != 2 {
		t.Errorf("expected 2 companies, got %d", len(companies))
	}
}

func TestListCompanies_Empty(t *testing.T) {
	fc := newTestController(t, &mockTransport{})

	companies := fc.ListCompanies()
	if len(companies) != 0 {
		t.Errorf("expected 0 companies, got %d", len(companies))
	}
}

func TestUpdateCompanyStatus(t *testing.T) {
	fc := newTestController(t, &mockTransport{})

	company, _ := fc.RegisterCompany(CompanyRegistration{Name: "status-test", Endpoint: "http://a", Type: CompanyTypeSoftware})

	if err := fc.UpdateCompanyStatus(company.ID, CompanyStatusOnline); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got, _ := fc.GetCompany(company.ID)
	if got.Status != CompanyStatusOnline {
		t.Errorf("expected Online, got %s", got.Status)
	}
}

func TestUpdateCompanyStatus_NotFound(t *testing.T) {
	fc := newTestController(t, &mockTransport{})

	err := fc.UpdateCompanyStatus("missing", CompanyStatusOnline)
	if err == nil {
		t.Fatal("expected error for missing company")
	}
}

func TestTransport(t *testing.T) {
	mt := &mockTransport{}
	fc := newTestController(t, mt)

	if fc.Transport() != mt {
		t.Error("expected transport to match")
	}
}

// === State Persistence Tests ===

func TestSaveAndLoadState(t *testing.T) {
	stateDir := t.TempDir()
	fc := &FederationController{
		companies: make(map[string]*Company),
		transport: &mockTransport{},
		logger:    newTestLogger(),
		stateDir:  stateDir,
	}

	fc.RegisterCompany(CompanyRegistration{Name: "persist-test", Endpoint: "http://a", Type: CompanyTypeSoftware})

	// Create a new controller pointing at the same state dir.
	fc2 := &FederationController{
		companies: make(map[string]*Company),
		transport: &mockTransport{},
		logger:    newTestLogger(),
		stateDir:  stateDir,
	}

	if err := fc2.loadState(); err != nil {
		t.Fatalf("loadState failed: %v", err)
	}

	if len(fc2.companies) != 1 {
		t.Errorf("expected 1 company after load, got %d", len(fc2.companies))
	}
}

func TestLoadState_NoFile(t *testing.T) {
	fc := newTestController(t, &mockTransport{})

	// Should not error when file doesn't exist.
	if err := fc.loadState(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestLoadState_InvalidJSON(t *testing.T) {
	stateDir := t.TempDir()
	os.WriteFile(filepath.Join(stateDir, federationFileName), []byte("not json"), 0o644)

	fc := &FederationController{
		companies: make(map[string]*Company),
		transport: &mockTransport{},
		logger:    newTestLogger(),
		stateDir:  stateDir,
	}

	if err := fc.loadState(); err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

// === Heartbeat Tests ===

func TestHeartbeatMonitor_StartStop(t *testing.T) {
	fc := newTestController(t, &mockTransport{})
	hm := NewHeartbeatMonitor(fc, newTestLogger())

	hm.Start()
	// Starting again should be a no-op.
	hm.Start()

	hm.Stop()
	// Stopping again should be a no-op.
	hm.Stop()
}

func TestHeartbeatMonitor_PendingIssues(t *testing.T) {
	fc := newTestController(t, &mockTransport{})
	hm := NewHeartbeatMonitor(fc, newTestLogger())

	issues := hm.ListPendingIssues()
	if len(issues) != 0 {
		t.Errorf("expected 0 pending issues, got %d", len(issues))
	}

	hm.AddPendingIssue(PendingIssue{
		IssueID:   "issue-1",
		CompanyID: "company-1",
		Reason:    "blocked",
	})

	hm.AddPendingIssue(PendingIssue{
		IssueID:   "issue-2",
		CompanyID: "company-2",
		Reason:    "waiting",
		Since:     time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
	})

	issues = hm.ListPendingIssues()
	if len(issues) != 2 {
		t.Fatalf("expected 2 pending issues, got %d", len(issues))
	}

	// Verify Since is auto-filled when zero.
	if issues[0].Since.IsZero() {
		t.Error("expected Since to be auto-filled")
	}
	// Verify explicit Since is preserved.
	if issues[1].Since.Year() != 2025 {
		t.Errorf("expected preserved Since year 2025, got %d", issues[1].Since.Year())
	}

	hm.RemovePendingIssue("issue-1")
	issues = hm.ListPendingIssues()
	if len(issues) != 1 {
		t.Errorf("expected 1 pending issue after remove, got %d", len(issues))
	}
	if issues[0].IssueID != "issue-2" {
		t.Errorf("expected issue-2, got %s", issues[0].IssueID)
	}

	// Removing non-existent issue is a no-op.
	hm.RemovePendingIssue("nonexistent")
	if len(hm.ListPendingIssues()) != 1 {
		t.Error("removing nonexistent issue should be a no-op")
	}
}

// === Intervention Tests ===

func TestIntervene(t *testing.T) {
	fc := newTestController(t, &mockTransport{})

	// Reset package-level registry for test isolation.
	registry = newInterventionRegistry()

	if err := fc.Intervene("issue-1", "approve", "looks good", "alice"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	registry.mu.RLock()
	count := len(registry.interventions)
	registry.mu.RUnlock()

	if count != 1 {
		t.Errorf("expected 1 intervention, got %d", count)
	}
}

func TestIntervene_InvalidAction(t *testing.T) {
	fc := newTestController(t, &mockTransport{})

	err := fc.Intervene("issue-1", "invalid", "", "alice")
	if err == nil {
		t.Fatal("expected error for invalid action")
	}
}

func TestIntervene_EmptyIssueID(t *testing.T) {
	fc := newTestController(t, &mockTransport{})

	err := fc.Intervene("", "approve", "", "alice")
	if err == nil {
		t.Fatal("expected error for empty issue ID")
	}
}

func TestApprovalGate_SetAndApprove(t *testing.T) {
	fc := newTestController(t, &mockTransport{})
	registry = newInterventionRegistry()

	if err := fc.SetApprovalGate("task-1"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if err := fc.ApproveTask("task-1", "bob"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	registry.mu.RLock()
	approval, ok := registry.taskApprovals["task-1"]
	registry.mu.RUnlock()

	if !ok {
		t.Fatal("expected task approval to be recorded")
	}
	if approval.Action != "approve" {
		t.Errorf("expected approve, got %s", approval.Action)
	}
}

func TestApprovalGate_SetAndReject(t *testing.T) {
	fc := newTestController(t, &mockTransport{})
	registry = newInterventionRegistry()

	fc.SetApprovalGate("task-2")

	if err := fc.RejectTask("task-2", "charlie", "not ready"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	registry.mu.RLock()
	approval := registry.taskApprovals["task-2"]
	registry.mu.RUnlock()

	if approval.Action != "reject" {
		t.Errorf("expected reject, got %s", approval.Action)
	}
	if approval.Reason != "not ready" {
		t.Errorf("expected reason 'not ready', got %s", approval.Reason)
	}
}

func TestApproveTask_NoGate(t *testing.T) {
	fc := newTestController(t, &mockTransport{})
	registry = newInterventionRegistry()

	err := fc.ApproveTask("no-gate", "alice")
	if err == nil {
		t.Fatal("expected error when no gate set")
	}
}

func TestRejectTask_NoGate(t *testing.T) {
	fc := newTestController(t, &mockTransport{})
	registry = newInterventionRegistry()

	err := fc.RejectTask("no-gate", "alice", "reason")
	if err == nil {
		t.Fatal("expected error when no gate set")
	}
}

func TestSetApprovalGate_EmptyTaskID(t *testing.T) {
	fc := newTestController(t, &mockTransport{})

	err := fc.SetApprovalGate("")
	if err == nil {
		t.Fatal("expected error for empty task ID")
	}
}

func TestApproveTask_EmptyTaskID(t *testing.T) {
	fc := newTestController(t, &mockTransport{})

	err := fc.ApproveTask("", "alice")
	if err == nil {
		t.Fatal("expected error for empty task ID")
	}
}

func TestRejectTask_EmptyTaskID(t *testing.T) {
	fc := newTestController(t, &mockTransport{})

	err := fc.RejectTask("", "alice", "reason")
	if err == nil {
		t.Fatal("expected error for empty task ID")
	}
}

// === InterventionHandler Tests ===

func TestInterventionHandler_Handle(t *testing.T) {
	fc := newTestController(t, &mockTransport{})
	registry = newInterventionRegistry()

	handler := NewInterventionHandler(newTestLogger(), fc)

	result, err := handler.Handle(InterventionRequest{
		IssueID: "issue-x",
		Action:  "approve",
		Reason:  "good to go",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Status != "applied" {
		t.Errorf("expected applied, got %s", result.Status)
	}
	if result.IssueID != "issue-x" {
		t.Errorf("expected issue-x, got %s", result.IssueID)
	}
}

func TestInterventionHandler_HandleInvalidAction(t *testing.T) {
	fc := newTestController(t, &mockTransport{})
	registry = newInterventionRegistry()

	handler := NewInterventionHandler(newTestLogger(), fc)

	_, err := handler.Handle(InterventionRequest{
		IssueID: "issue-y",
		Action:  "destroy",
	})
	if err == nil {
		t.Fatal("expected error for invalid action")
	}
}

// === Transport Tests (via httptest) ===

func TestCompanyStatusReport_JSON(t *testing.T) {
	report := CompanyStatusReport{
		CompanyID:   "c1",
		CompanyName: "dev",
		Status:      "Online",
		AgentCount:  3,
		Agents:      []string{"a1", "a2", "a3"},
	}

	data, err := json.Marshal(report)
	if err != nil {
		t.Fatalf("marshal error: %v", err)
	}

	var got CompanyStatusReport
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}

	if got.CompanyID != "c1" || got.AgentCount != 3 {
		t.Errorf("unexpected report: %+v", got)
	}
}
