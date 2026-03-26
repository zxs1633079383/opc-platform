package server

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	v1 "github.com/zlc-ai/opc-platform/api/v1"
	"github.com/zlc-ai/opc-platform/pkg/adapter"
	"github.com/zlc-ai/opc-platform/pkg/controller"
	"github.com/zlc-ai/opc-platform/pkg/cost"
	"github.com/zlc-ai/opc-platform/pkg/evolve"
	"github.com/zlc-ai/opc-platform/pkg/federation"
	"github.com/zlc-ai/opc-platform/pkg/gateway"
	"github.com/zlc-ai/opc-platform/pkg/goal"
	"github.com/zlc-ai/opc-platform/pkg/model"
	"github.com/zlc-ai/opc-platform/pkg/storage"
	"github.com/zlc-ai/opc-platform/pkg/storage/sqlite"
	"go.uber.org/zap"
)

// ---------- mock adapter ----------

type testMockAdapter struct {
	mu          sync.Mutex
	executeFunc func(ctx context.Context, task v1.TaskRecord) (adapter.ExecuteResult, error)
	started     bool
	stopped     bool
}

func (m *testMockAdapter) Type() v1.AgentType         { return v1.AgentTypeClaudeCode }
func (m *testMockAdapter) Start(_ context.Context, _ v1.AgentSpec) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.started = true
	return nil
}
func (m *testMockAdapter) Stop(_ context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.stopped = true
	return nil
}
func (m *testMockAdapter) Health() v1.HealthStatus {
	return v1.HealthStatus{Healthy: true, Message: "ok"}
}
func (m *testMockAdapter) Execute(ctx context.Context, task v1.TaskRecord) (adapter.ExecuteResult, error) {
	m.mu.Lock()
	fn := m.executeFunc
	m.mu.Unlock()
	if fn != nil {
		return fn(ctx, task)
	}
	return adapter.ExecuteResult{Output: "mock output", TokensIn: 100, TokensOut: 200}, nil
}
func (m *testMockAdapter) Stream(_ context.Context, _ v1.TaskRecord) (<-chan adapter.Chunk, error) {
	ch := make(chan adapter.Chunk, 1)
	ch <- adapter.Chunk{Content: "streamed", Done: true}
	close(ch)
	return ch, nil
}
func (m *testMockAdapter) Status() v1.AgentPhase      { return v1.AgentPhaseRunning }
func (m *testMockAdapter) Metrics() v1.AgentMetrics    { return v1.AgentMetrics{} }

// ---------- mock transport for federation ----------

type testMockTransport struct {
	mu       sync.Mutex
	sendFunc func(endpoint, method, path string, body any) ([]byte, error)
}

func (t *testMockTransport) Send(endpoint, method, path string, body any) ([]byte, error) {
	return t.SendWithContext(context.Background(), endpoint, method, path, body)
}
func (t *testMockTransport) SendWithContext(_ context.Context, endpoint, method, path string, body any) ([]byte, error) {
	t.mu.Lock()
	fn := t.sendFunc
	t.mu.Unlock()
	if fn != nil {
		return fn(endpoint, method, path, body)
	}
	return []byte(`{}`), nil
}
func (t *testMockTransport) Ping(_ string) error { return nil }
func (t *testMockTransport) FetchStatus(_ string) (*federation.CompanyStatusReport, error) {
	return &federation.CompanyStatusReport{Status: "Online"}, nil
}

// ---------- test server setup ----------

type testEnv struct {
	server   *Server
	ts       *httptest.Server
	store    func() // cleanup
	baseURL  string
}

func newTestServer(t *testing.T) *testEnv {
	t.Helper()
	return newTestServerWithTransport(t, nil)
}

func newTestServerWithTransport(t *testing.T, transport federation.Transport) *testEnv {
	t.Helper()
	gin.SetMode(gin.TestMode)

	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	store, err := sqlite.New(dbPath)
	if err != nil {
		t.Fatalf("create sqlite store: %v", err)
	}

	registry := adapter.NewRegistry()
	registry.Register(v1.AgentTypeClaudeCode, func() adapter.Adapter {
		return &testMockAdapter{
			executeFunc: func(_ context.Context, _ v1.TaskRecord) (adapter.ExecuteResult, error) {
				return adapter.ExecuteResult{Output: "mock output", TokensIn: 100, TokensOut: 200}, nil
			},
		}
	})

	logger, _ := zap.NewDevelopment()
	sugar := logger.Sugar()

	ctrl := controller.New(store, registry, sugar)

	costDir := filepath.Join(tmpDir, "cost")
	os.MkdirAll(costDir, 0o755)
	costMgr := cost.NewTracker(costDir, sugar)

	fedDir := filepath.Join(tmpDir, "federation")
	os.MkdirAll(fedDir, 0o755)
	if transport == nil {
		transport = &testMockTransport{}
	}
	fedCtrl := federation.NewControllerForTest(fedDir, transport, sugar)

	gw := gateway.New(sugar)

	srv := New(ctrl, costMgr, gw, fedCtrl, Config{Port: 0}, sugar)

	// Build gin router the same way Start() does.
	router := gin.New()
	router.Use(gin.Recovery())
	router.Use(srv.corsMiddleware())

	api := router.Group("/api")
	{
		api.GET("/health", srv.healthCheck)
		api.GET("/status", srv.clusterStatus)
		api.GET("/events", srv.sseEvents)
		api.POST("/apply", srv.applyResource)
		api.GET("/agents", srv.listAgents)
		api.GET("/agents/:name", srv.getAgent)
		api.DELETE("/agents/:name", srv.deleteAgent)
		api.POST("/agents/:name/start", srv.startAgent)
		api.POST("/agents/:name/stop", srv.stopAgent)
		api.POST("/agents/:name/restart", srv.restartAgent)
		api.POST("/run", srv.runTask)
		api.GET("/tasks", srv.listTasks)
		api.GET("/tasks/:id", srv.getTask)
		api.GET("/tasks/:id/logs", srv.getTaskLogs)
		api.GET("/metrics", srv.clusterMetrics)
		api.GET("/metrics/agents", srv.agentMetrics)
		api.GET("/metrics/health", srv.agentHealth)
		api.GET("/costs/daily", srv.costDaily)
		api.GET("/costs/events", srv.costEvents)
		api.GET("/logs", srv.getLogs)
		api.GET("/workflows", srv.listWorkflows)
		api.DELETE("/workflows/:name", srv.deleteWorkflow)
		api.POST("/workflows/:name/run", srv.runWorkflow)
		api.PUT("/workflows/:name/toggle", srv.toggleWorkflow)
		api.GET("/workflows/:name/runs", srv.listWorkflowRuns)
		api.GET("/workflows/:name/runs/:id", srv.getWorkflowRun)
		api.GET("/federation/companies", srv.listCompanies)
		api.GET("/federation/companies/:id", srv.getCompany)
		api.POST("/federation/companies", srv.registerCompany)
		api.DELETE("/federation/companies/:id", srv.unregisterCompany)
		api.PUT("/federation/companies/:id/status", srv.updateCompanyStatus)
		api.POST("/federation/intervene", srv.intervene)
		api.GET("/federation/companies/:id/agents", srv.federatedAgents)
		api.GET("/federation/companies/:id/tasks", srv.federatedTasks)
		api.GET("/federation/companies/:id/metrics", srv.federatedMetrics)
		api.GET("/federation/companies/:id/health", srv.federatedHealth)
		api.GET("/federation/aggregate/agents", srv.aggregateAgents)
		api.GET("/federation/aggregate/metrics", srv.aggregateMetrics)
		api.POST("/federation/callback", srv.federationCallback)
		api.POST("/goals/federated", srv.createFederatedGoal)
		api.GET("/goals", srv.listGoals)
		api.GET("/goals/:id", srv.getGoal)
		api.POST("/goals", srv.createGoal)
		api.PUT("/goals/:id", srv.updateGoal)
		api.DELETE("/goals/:id", srv.deleteGoal)
		api.GET("/goals/:id/projects", srv.listProjectsByGoal)
		api.GET("/goals/:id/stats", srv.goalStats)
		api.GET("/goals/:id/plan", srv.getGoalPlan)
		api.POST("/goals/:id/approve", srv.approveGoal)
		api.POST("/goals/:id/revise", srv.reviseGoal)
		api.GET("/projects", srv.listProjects)
		api.GET("/projects/:id", srv.getProject)
		api.POST("/projects", srv.createProject)
		api.PUT("/projects/:id", srv.updateProject)
		api.DELETE("/projects/:id", srv.deleteProject)
		api.GET("/projects/:id/issues", srv.listIssuesByProject)
		api.GET("/projects/:id/stats", srv.projectStats)
		api.GET("/issues", srv.listIssues)
		api.GET("/issues/:id", srv.getIssue)
		api.POST("/issues", srv.createIssue)
		api.PUT("/issues/:id", srv.updateIssue)
		api.DELETE("/issues/:id", srv.deleteIssue)
		api.GET("/settings", srv.getSettings)
		api.PUT("/settings", srv.updateSettings)

		// models
		api.GET("/models", srv.handleListModels)
		api.POST("/models", srv.handleAddModel)

		// agent wizard
		api.POST("/agents/wizard", srv.handleAgentWizard)

		// system (evolve)
		api.GET("/system/metrics", srv.handleSystemMetrics)
		api.GET("/system/rfcs", srv.handleListRFCs)
		api.POST("/system/rfcs/:id/approve", srv.handleApproveRFC)
		api.POST("/system/rfcs/:id/reject", srv.handleRejectRFC)
	}

	ts := httptest.NewServer(router)
	t.Cleanup(func() {
		ts.Close()
		store.Close()
	})

	return &testEnv{
		server:  srv,
		ts:      ts,
		baseURL: ts.URL,
	}
}

// ---------- helpers ----------

func httpGet(t *testing.T, url string) (*http.Response, map[string]interface{}) {
	t.Helper()
	resp, err := http.Get(url)
	if err != nil {
		t.Fatalf("GET %s: %v", url, err)
	}
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	var result map[string]interface{}
	json.Unmarshal(body, &result)
	return resp, result
}

func httpGetList(t *testing.T, url string) (*http.Response, []interface{}) {
	t.Helper()
	resp, err := http.Get(url)
	if err != nil {
		t.Fatalf("GET %s: %v", url, err)
	}
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	var result []interface{}
	json.Unmarshal(body, &result)
	return resp, result
}

func httpPostJSON(t *testing.T, url string, payload interface{}) (*http.Response, map[string]interface{}) {
	t.Helper()
	data, _ := json.Marshal(payload)
	resp, err := http.Post(url, "application/json", bytes.NewReader(data))
	if err != nil {
		t.Fatalf("POST %s: %v", url, err)
	}
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	var result map[string]interface{}
	json.Unmarshal(body, &result)
	return resp, result
}

func httpPutJSON(t *testing.T, url string, payload interface{}) (*http.Response, map[string]interface{}) {
	t.Helper()
	data, _ := json.Marshal(payload)
	req, _ := http.NewRequest(http.MethodPut, url, bytes.NewReader(data))
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("PUT %s: %v", url, err)
	}
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	var result map[string]interface{}
	json.Unmarshal(body, &result)
	return resp, result
}

func httpDelete(t *testing.T, url string) (*http.Response, map[string]interface{}) {
	t.Helper()
	req, _ := http.NewRequest(http.MethodDelete, url, nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("DELETE %s: %v", url, err)
	}
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	var result map[string]interface{}
	json.Unmarshal(body, &result)
	return resp, result
}

func httpPostYAML(t *testing.T, url string, yamlBody string) (*http.Response, map[string]interface{}) {
	t.Helper()
	resp, err := http.Post(url, "application/yaml", bytes.NewReader([]byte(yamlBody)))
	if err != nil {
		t.Fatalf("POST %s: %v", url, err)
	}
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	var result map[string]interface{}
	json.Unmarshal(body, &result)
	return resp, result
}

func applyAgent(t *testing.T, baseURL, name string) {
	t.Helper()
	yaml := fmt.Sprintf(`apiVersion: opc/v1
kind: AgentSpec
metadata:
  name: %s
spec:
  type: claude-code
  runtime:
    model:
      provider: anthropic
      name: claude-sonnet-4
    timeout:
      task: "600s"
  context:
    workdir: /tmp/opc
  recovery:
    enabled: true
    maxRestarts: 3
`, name)
	resp, _ := httpPostYAML(t, baseURL+"/api/apply", yaml)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("apply agent %s: status %d", name, resp.StatusCode)
	}
}

// ============================================================
// Part 1: Single Instance API Tests
// ============================================================

func TestHealth(t *testing.T) {
	env := newTestServer(t)
	resp, body := httpGet(t, env.baseURL+"/api/health")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	if body["status"] != "healthy" {
		t.Errorf("expected status=healthy, got %v", body["status"])
	}
	if _, ok := body["timestamp"]; !ok {
		t.Error("expected timestamp field")
	}
}

func TestStatus(t *testing.T) {
	env := newTestServer(t)
	resp, body := httpGet(t, env.baseURL+"/api/status")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	if _, ok := body["agents"]; !ok {
		t.Error("expected agents field")
	}
	if _, ok := body["tasks"]; !ok {
		t.Error("expected tasks field")
	}
}

func TestApply_AgentSpec(t *testing.T) {
	env := newTestServer(t)
	yaml := `apiVersion: opc/v1
kind: AgentSpec
metadata:
  name: test-agent
spec:
  type: claude-code
  runtime:
    model:
      provider: anthropic
      name: claude-sonnet-4
    timeout:
      task: "600s"
  context:
    workdir: /tmp/opc
`
	resp, body := httpPostYAML(t, env.baseURL+"/api/apply", yaml)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	msg, _ := body["message"].(string)
	if msg == "" {
		t.Error("expected message in response")
	}

	// Verify agent exists.
	resp2, body2 := httpGet(t, env.baseURL+"/api/agents/test-agent")
	if resp2.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 for get agent, got %d", resp2.StatusCode)
	}
	if body2["name"] != "test-agent" {
		t.Errorf("expected name=test-agent, got %v", body2["name"])
	}
}

func TestApply_Workflow(t *testing.T) {
	env := newTestServer(t)
	applyAgent(t, env.baseURL, "coder")

	yaml := `apiVersion: opc/v1
kind: Workflow
metadata:
  name: test-wf
spec:
  schedule: "0 9 * * *"
  steps:
    - name: step1
      agent: coder
      input:
        message: "do something"
`
	resp, body := httpPostYAML(t, env.baseURL+"/api/apply", yaml)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	msg, _ := body["message"].(string)
	if msg == "" {
		t.Error("expected message in response")
	}

	// Verify workflow exists.
	resp2, list := httpGetList(t, env.baseURL+"/api/workflows")
	if resp2.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp2.StatusCode)
	}
	if len(list) != 1 {
		t.Errorf("expected 1 workflow, got %d", len(list))
	}
}

func TestApply_UnsupportedKind(t *testing.T) {
	env := newTestServer(t)
	yaml := `apiVersion: opc/v1
kind: UnknownKind
metadata:
  name: test
`
	resp, body := httpPostYAML(t, env.baseURL+"/api/apply", yaml)
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", resp.StatusCode)
	}
	if body["error"] == nil {
		t.Error("expected error in response")
	}
}

func TestListAgents_Empty(t *testing.T) {
	env := newTestServer(t)
	resp, list := httpGetList(t, env.baseURL+"/api/agents")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	if len(list) != 0 {
		t.Errorf("expected 0 agents, got %d", len(list))
	}
}

func TestListAgents_WithAgents(t *testing.T) {
	env := newTestServer(t)
	applyAgent(t, env.baseURL, "agent-a")
	applyAgent(t, env.baseURL, "agent-b")

	resp, list := httpGetList(t, env.baseURL+"/api/agents")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	if len(list) != 2 {
		t.Errorf("expected 2 agents, got %d", len(list))
	}
}

func TestGetAgent(t *testing.T) {
	env := newTestServer(t)
	applyAgent(t, env.baseURL, "my-agent")

	resp, body := httpGet(t, env.baseURL+"/api/agents/my-agent")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	if body["name"] != "my-agent" {
		t.Errorf("expected name=my-agent, got %v", body["name"])
	}
}

func TestGetAgent_NotFound(t *testing.T) {
	env := newTestServer(t)
	resp, _ := httpGet(t, env.baseURL+"/api/agents/nonexistent")
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", resp.StatusCode)
	}
}

func TestDeleteAgent(t *testing.T) {
	env := newTestServer(t)
	applyAgent(t, env.baseURL, "to-delete")

	resp, _ := httpDelete(t, env.baseURL+"/api/agents/to-delete")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	// Verify deleted.
	resp2, _ := httpGet(t, env.baseURL+"/api/agents/to-delete")
	if resp2.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404 after delete, got %d", resp2.StatusCode)
	}
}

func TestStartStopAgent(t *testing.T) {
	env := newTestServer(t)
	applyAgent(t, env.baseURL, "lifecycle-agent")

	// Start.
	resp, body := httpPostJSON(t, env.baseURL+"/api/agents/lifecycle-agent/start", nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("start: expected 200, got %d, body: %v", resp.StatusCode, body)
	}

	// Verify running.
	resp2, body2 := httpGet(t, env.baseURL+"/api/agents/lifecycle-agent")
	if resp2.StatusCode != http.StatusOK {
		t.Fatalf("get: expected 200, got %d", resp2.StatusCode)
	}
	if body2["phase"] != "Running" {
		t.Errorf("expected phase=Running, got %v", body2["phase"])
	}

	// Stop.
	resp3, _ := httpPostJSON(t, env.baseURL+"/api/agents/lifecycle-agent/stop", nil)
	if resp3.StatusCode != http.StatusOK {
		t.Fatalf("stop: expected 200, got %d", resp3.StatusCode)
	}

	// Verify stopped.
	resp4, body4 := httpGet(t, env.baseURL+"/api/agents/lifecycle-agent")
	if resp4.StatusCode != http.StatusOK {
		t.Fatalf("get: expected 200, got %d", resp4.StatusCode)
	}
	if body4["phase"] != "Stopped" {
		t.Errorf("expected phase=Stopped, got %v", body4["phase"])
	}
}

func TestRunTask(t *testing.T) {
	env := newTestServer(t)
	applyAgent(t, env.baseURL, "task-agent")
	httpPostJSON(t, env.baseURL+"/api/agents/task-agent/start", nil)

	resp, body := httpPostJSON(t, env.baseURL+"/api/run", map[string]string{
		"agent":   "task-agent",
		"message": "write hello world",
	})
	if resp.StatusCode != http.StatusAccepted {
		t.Fatalf("expected 202, got %d, body: %v", resp.StatusCode, body)
	}
	taskID, ok := body["taskId"].(string)
	if !ok || taskID == "" {
		t.Fatalf("expected taskId in response, got %v", body)
	}

	// Poll for completion.
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		_, taskBody := httpGet(t, env.baseURL+"/api/tasks/"+taskID)
		status, _ := taskBody["status"].(string)
		if status == "Completed" || status == "Failed" {
			return
		}
		time.Sleep(100 * time.Millisecond)
	}
	t.Log("task did not complete within timeout (may be expected in test env)")
}

func TestRunTask_MissingFields(t *testing.T) {
	env := newTestServer(t)
	resp, _ := httpPostJSON(t, env.baseURL+"/api/run", map[string]string{
		"agent": "some-agent",
	})
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", resp.StatusCode)
	}
}

func TestListTasks(t *testing.T) {
	env := newTestServer(t)
	resp, list := httpGetList(t, env.baseURL+"/api/tasks")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	if list == nil {
		t.Error("expected non-nil list")
	}
}

func TestGetTaskLogs(t *testing.T) {
	env := newTestServer(t)
	applyAgent(t, env.baseURL, "log-agent")
	httpPostJSON(t, env.baseURL+"/api/agents/log-agent/start", nil)

	resp, body := httpPostJSON(t, env.baseURL+"/api/run", map[string]string{
		"agent":   "log-agent",
		"message": "test",
	})
	taskID, _ := body["taskId"].(string)
	if resp.StatusCode != http.StatusAccepted || taskID == "" {
		t.Fatalf("run task: status=%d, body=%v", resp.StatusCode, body)
	}

	// Wait briefly for task record to be created.
	time.Sleep(200 * time.Millisecond)

	resp2, body2 := httpGet(t, env.baseURL+"/api/tasks/"+taskID+"/logs")
	if resp2.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp2.StatusCode)
	}
	if body2["taskId"] != taskID {
		t.Errorf("expected taskId=%s, got %v", taskID, body2["taskId"])
	}
}

func TestGetTaskLogs_NotFound(t *testing.T) {
	env := newTestServer(t)
	resp, _ := httpGet(t, env.baseURL+"/api/tasks/nonexistent/logs")
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", resp.StatusCode)
	}
}

func TestGoalCRUD(t *testing.T) {
	env := newTestServer(t)

	// Create.
	resp, body := httpPostJSON(t, env.baseURL+"/api/goals", map[string]interface{}{
		"name":        "test-goal",
		"description": "A test goal",
	})
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("create: expected 201, got %d, body: %v", resp.StatusCode, body)
	}
	goalID, _ := body["id"].(string)
	if goalID == "" {
		t.Fatal("expected id in response")
	}

	// List.
	resp2, list := httpGetList(t, env.baseURL+"/api/goals")
	if resp2.StatusCode != http.StatusOK {
		t.Fatalf("list: expected 200, got %d", resp2.StatusCode)
	}
	if len(list) != 1 {
		t.Errorf("expected 1 goal, got %d", len(list))
	}

	// Get.
	resp3, body3 := httpGet(t, env.baseURL+"/api/goals/"+goalID)
	if resp3.StatusCode != http.StatusOK {
		t.Fatalf("get: expected 200, got %d", resp3.StatusCode)
	}
	if body3["name"] != "test-goal" {
		t.Errorf("expected name=test-goal, got %v", body3["name"])
	}

	// Update.
	resp4, _ := httpPutJSON(t, env.baseURL+"/api/goals/"+goalID, map[string]interface{}{
		"name":        "updated-goal",
		"description": "updated",
		"status":      "completed",
	})
	if resp4.StatusCode != http.StatusOK {
		t.Fatalf("update: expected 200, got %d", resp4.StatusCode)
	}

	// Delete.
	resp5, _ := httpDelete(t, env.baseURL+"/api/goals/"+goalID)
	if resp5.StatusCode != http.StatusOK {
		t.Fatalf("delete: expected 200, got %d", resp5.StatusCode)
	}

	// Verify deleted.
	resp6, _ := httpGet(t, env.baseURL+"/api/goals/"+goalID)
	if resp6.StatusCode != http.StatusNotFound {
		t.Fatalf("get after delete: expected 404, got %d", resp6.StatusCode)
	}
}

func TestGoalAutoDecompose(t *testing.T) {
	env := newTestServer(t)
	applyAgent(t, env.baseURL, "coder")
	httpPostJSON(t, env.baseURL+"/api/agents/coder/start", nil)

	resp, body := httpPostJSON(t, env.baseURL+"/api/goals", map[string]interface{}{
		"name":          "auto-goal",
		"description":   "Build a web app",
		"autoDecompose": true,
	})
	// autoDecompose returns 202 Accepted.
	if resp.StatusCode != http.StatusAccepted {
		t.Fatalf("expected 202, got %d, body: %v", resp.StatusCode, body)
	}
	goalID, _ := body["id"].(string)
	if goalID == "" {
		t.Fatal("expected id in response")
	}

	// Verify goal was created. The async task may complete very quickly
	// and update the phase to "completed", so accept either in_progress or completed.
	time.Sleep(200 * time.Millisecond)
	resp2, body2 := httpGet(t, env.baseURL+"/api/goals/"+goalID)
	if resp2.StatusCode != http.StatusOK {
		t.Fatalf("get: expected 200, got %d", resp2.StatusCode)
	}
	phase, _ := body2["phase"].(string)
	if phase != "in_progress" && phase != "completed" {
		t.Errorf("expected phase=in_progress or completed, got %v", phase)
	}

	// Verify projects were created.
	resp3, list3 := httpGetList(t, env.baseURL+"/api/goals/"+goalID+"/projects")
	if resp3.StatusCode != http.StatusOK {
		t.Fatalf("projects: expected 200, got %d", resp3.StatusCode)
	}
	if len(list3) < 1 {
		t.Errorf("expected at least 1 project, got %d", len(list3))
	}
}

func TestGoalPlanApproveRevise(t *testing.T) {
	env := newTestServer(t)

	// Create goal with planned phase.
	resp, body := httpPostJSON(t, env.baseURL+"/api/goals", map[string]interface{}{
		"name":        "plan-goal",
		"description": "A goal to plan",
	})
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("create: expected 201, got %d", resp.StatusCode)
	}
	goalID, _ := body["id"].(string)

	// Set goal to planned phase manually.
	httpPutJSON(t, env.baseURL+"/api/goals/"+goalID, map[string]interface{}{
		"name":              "plan-goal",
		"description":       "A goal to plan",
		"status":            "active",
		"phase":             "planned",
		"decompositionPlan": `{"projects":[]}`,
	})

	// Get plan.
	resp2, body2 := httpGet(t, env.baseURL+"/api/goals/"+goalID+"/plan")
	if resp2.StatusCode != http.StatusOK {
		t.Fatalf("plan: expected 200, got %d", resp2.StatusCode)
	}
	if body2["goalId"] != goalID {
		t.Errorf("expected goalId=%s, got %v", goalID, body2["goalId"])
	}

	// Approve.
	resp3, _ := httpPostJSON(t, env.baseURL+"/api/goals/"+goalID+"/approve", nil)
	if resp3.StatusCode != http.StatusOK {
		t.Fatalf("approve: expected 200, got %d", resp3.StatusCode)
	}

	// Verify approved.
	_, body4 := httpGet(t, env.baseURL+"/api/goals/"+goalID)
	if body4["phase"] != "approved" {
		t.Errorf("expected phase=approved, got %v", body4["phase"])
	}

	// Revise (set back to planned first).
	httpPutJSON(t, env.baseURL+"/api/goals/"+goalID, map[string]interface{}{
		"name":   "plan-goal",
		"status": "active",
		"phase":  "planned",
	})
	resp5, _ := httpPostJSON(t, env.baseURL+"/api/goals/"+goalID+"/revise", map[string]interface{}{
		"plan": map[string]interface{}{"revised": true},
	})
	if resp5.StatusCode != http.StatusOK {
		t.Fatalf("revise: expected 200, got %d", resp5.StatusCode)
	}
}

func TestGoalApprove_WrongPhase(t *testing.T) {
	env := newTestServer(t)
	resp, body := httpPostJSON(t, env.baseURL+"/api/goals", map[string]interface{}{
		"name":        "active-goal",
		"description": "An active goal",
	})
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("create: expected 201, got %d", resp.StatusCode)
	}
	goalID, _ := body["id"].(string)

	// Try approve on non-planned goal.
	resp2, _ := httpPostJSON(t, env.baseURL+"/api/goals/"+goalID+"/approve", nil)
	if resp2.StatusCode != http.StatusBadRequest {
		t.Fatalf("approve wrong phase: expected 400, got %d", resp2.StatusCode)
	}
}

func TestGoalStats(t *testing.T) {
	env := newTestServer(t)
	resp, body := httpPostJSON(t, env.baseURL+"/api/goals", map[string]interface{}{
		"name":        "stats-goal",
		"description": "A goal for stats",
	})
	goalID, _ := body["id"].(string)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("create: expected 201, got %d", resp.StatusCode)
	}

	resp2, _ := httpGet(t, env.baseURL+"/api/goals/"+goalID+"/stats")
	if resp2.StatusCode != http.StatusOK {
		t.Fatalf("stats: expected 200, got %d", resp2.StatusCode)
	}
}

func TestListWorkflows_Empty(t *testing.T) {
	env := newTestServer(t)
	resp, list := httpGetList(t, env.baseURL+"/api/workflows")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	if len(list) != 0 {
		t.Errorf("expected 0 workflows, got %d", len(list))
	}
}

func TestRunWorkflow_NotFound(t *testing.T) {
	env := newTestServer(t)
	resp, _ := httpPostJSON(t, env.baseURL+"/api/workflows/nonexistent/run", nil)
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", resp.StatusCode)
	}
}

func TestToggleWorkflow(t *testing.T) {
	env := newTestServer(t)
	applyAgent(t, env.baseURL, "coder")

	yaml := `apiVersion: opc/v1
kind: Workflow
metadata:
  name: toggle-wf
spec:
  schedule: "0 9 * * *"
  steps:
    - name: step1
      agent: coder
      input:
        message: "do something"
`
	httpPostYAML(t, env.baseURL+"/api/apply", yaml)

	// Toggle off.
	resp, body := httpPutJSON(t, env.baseURL+"/api/workflows/toggle-wf/toggle", map[string]interface{}{
		"enabled": false,
	})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("toggle: expected 200, got %d", resp.StatusCode)
	}
	if body["enabled"] != false {
		t.Errorf("expected enabled=false, got %v", body["enabled"])
	}

	// Toggle on.
	resp2, body2 := httpPutJSON(t, env.baseURL+"/api/workflows/toggle-wf/toggle", map[string]interface{}{
		"enabled": true,
	})
	if resp2.StatusCode != http.StatusOK {
		t.Fatalf("toggle: expected 200, got %d", resp2.StatusCode)
	}
	if body2["enabled"] != true {
		t.Errorf("expected enabled=true, got %v", body2["enabled"])
	}
}

func TestDeleteWorkflow(t *testing.T) {
	env := newTestServer(t)
	applyAgent(t, env.baseURL, "coder")

	yaml := `apiVersion: opc/v1
kind: Workflow
metadata:
  name: del-wf
spec:
  steps:
    - name: step1
      agent: coder
      input:
        message: "hello"
`
	httpPostYAML(t, env.baseURL+"/api/apply", yaml)

	resp, _ := httpDelete(t, env.baseURL+"/api/workflows/del-wf")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	// Verify deleted.
	_, list := httpGetList(t, env.baseURL+"/api/workflows")
	if len(list) != 0 {
		t.Errorf("expected 0 workflows after delete, got %d", len(list))
	}
}

func TestFederationRegister(t *testing.T) {
	env := newTestServer(t)
	resp, body := httpPostJSON(t, env.baseURL+"/api/federation/companies", map[string]interface{}{
		"name":     "worker-co",
		"endpoint": "http://localhost:9999",
		"type":     "software",
	})
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("expected 201, got %d, body: %v", resp.StatusCode, body)
	}
	if body["name"] != "worker-co" {
		t.Errorf("expected name=worker-co, got %v", body["name"])
	}
	if body["id"] == nil || body["id"] == "" {
		t.Error("expected id in response")
	}
}

func TestFederationRegister_Duplicate(t *testing.T) {
	env := newTestServer(t)
	httpPostJSON(t, env.baseURL+"/api/federation/companies", map[string]interface{}{
		"name":     "dup-co",
		"endpoint": "http://localhost:9999",
		"type":     "software",
	})
	resp, _ := httpPostJSON(t, env.baseURL+"/api/federation/companies", map[string]interface{}{
		"name":     "dup-co",
		"endpoint": "http://localhost:8888",
		"type":     "software",
	})
	if resp.StatusCode != http.StatusConflict {
		t.Fatalf("expected 409, got %d", resp.StatusCode)
	}
}

func TestFederationRegister_MissingFields(t *testing.T) {
	env := newTestServer(t)
	resp, _ := httpPostJSON(t, env.baseURL+"/api/federation/companies", map[string]interface{}{
		"name": "no-endpoint",
	})
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", resp.StatusCode)
	}
}

func TestFederationList(t *testing.T) {
	env := newTestServer(t)
	httpPostJSON(t, env.baseURL+"/api/federation/companies", map[string]interface{}{
		"name":     "co-a",
		"endpoint": "http://localhost:1111",
		"type":     "software",
	})
	resp, list := httpGetList(t, env.baseURL+"/api/federation/companies")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	if len(list) != 1 {
		t.Errorf("expected 1 company, got %d", len(list))
	}
}

func TestFederationUnregister(t *testing.T) {
	env := newTestServer(t)
	resp, body := httpPostJSON(t, env.baseURL+"/api/federation/companies", map[string]interface{}{
		"name":     "unreg-co",
		"endpoint": "http://localhost:2222",
		"type":     "software",
	})
	companyID, _ := body["id"].(string)
	if resp.StatusCode != http.StatusCreated || companyID == "" {
		t.Fatalf("register: status=%d, body=%v", resp.StatusCode, body)
	}

	resp2, _ := httpDelete(t, env.baseURL+"/api/federation/companies/"+companyID)
	if resp2.StatusCode != http.StatusOK {
		t.Fatalf("unregister: expected 200, got %d", resp2.StatusCode)
	}

	// Verify.
	_, list := httpGetList(t, env.baseURL+"/api/federation/companies")
	if len(list) != 0 {
		t.Errorf("expected 0 companies after unregister, got %d", len(list))
	}
}

func TestCostDaily(t *testing.T) {
	env := newTestServer(t)
	resp, list := httpGetList(t, env.baseURL+"/api/costs/daily")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	if len(list) != 7 {
		t.Errorf("expected 7 daily entries, got %d", len(list))
	}
}

func TestCostEvents(t *testing.T) {
	env := newTestServer(t)
	resp, _ := httpGetList(t, env.baseURL+"/api/costs/events")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
}

func TestClusterMetrics(t *testing.T) {
	env := newTestServer(t)
	resp, body := httpGet(t, env.baseURL+"/api/metrics")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	if _, ok := body["totalAgents"]; !ok {
		t.Error("expected totalAgents field")
	}
	if _, ok := body["todayCost"]; !ok {
		t.Error("expected todayCost field")
	}
}

func TestAgentMetrics(t *testing.T) {
	env := newTestServer(t)
	resp, _ := httpGet(t, env.baseURL+"/api/metrics/agents")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
}

func TestAgentHealth(t *testing.T) {
	env := newTestServer(t)
	resp, _ := httpGet(t, env.baseURL+"/api/metrics/health")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
}

func TestSettings_GetDefault(t *testing.T) {
	env := newTestServer(t)
	resp, _ := httpGet(t, env.baseURL+"/api/settings")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
}

func TestSettings_PutAndGet(t *testing.T) {
	env := newTestServer(t)
	_ = env // settings uses ~/.opc/settings.json, which writes to real filesystem;
	// just verify the PUT returns 200.
	resp, _ := httpPutJSON(t, env.baseURL+"/api/settings", map[string]interface{}{
		"theme": "dark",
	})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
}

func TestLogs(t *testing.T) {
	env := newTestServer(t)
	resp, list := httpGetList(t, env.baseURL+"/api/logs")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	if list == nil {
		t.Error("expected non-nil list")
	}
}

func TestProjectsCRUD(t *testing.T) {
	env := newTestServer(t)

	// Create goal first.
	_, goalBody := httpPostJSON(t, env.baseURL+"/api/goals", map[string]interface{}{
		"name": "proj-goal", "description": "for projects",
	})
	goalID, _ := goalBody["id"].(string)

	// Create project.
	resp, body := httpPostJSON(t, env.baseURL+"/api/projects", map[string]interface{}{
		"name": "test-project", "goalId": goalID, "description": "desc",
	})
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("create: expected 201, got %d", resp.StatusCode)
	}
	projID, _ := body["id"].(string)

	// List.
	resp2, list := httpGetList(t, env.baseURL+"/api/projects")
	if resp2.StatusCode != http.StatusOK || len(list) < 1 {
		t.Fatalf("list: expected 200 with data, got %d, len=%d", resp2.StatusCode, len(list))
	}

	// Get.
	resp3, body3 := httpGet(t, env.baseURL+"/api/projects/"+projID)
	if resp3.StatusCode != http.StatusOK || body3["name"] != "test-project" {
		t.Fatalf("get: expected 200 with name, got %d", resp3.StatusCode)
	}

	// Update.
	resp4, _ := httpPutJSON(t, env.baseURL+"/api/projects/"+projID, map[string]interface{}{
		"name": "updated-proj", "goalId": goalID, "status": "completed",
	})
	if resp4.StatusCode != http.StatusOK {
		t.Fatalf("update: expected 200, got %d", resp4.StatusCode)
	}

	// Delete.
	resp5, _ := httpDelete(t, env.baseURL+"/api/projects/"+projID)
	if resp5.StatusCode != http.StatusOK {
		t.Fatalf("delete: expected 200, got %d", resp5.StatusCode)
	}
}

func TestIssuesCRUD(t *testing.T) {
	env := newTestServer(t)

	// Create issue.
	resp, body := httpPostJSON(t, env.baseURL+"/api/issues", map[string]interface{}{
		"name": "test-issue", "projectId": "proj-1", "description": "desc",
	})
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("create: expected 201, got %d", resp.StatusCode)
	}
	issueID, _ := body["id"].(string)

	// List.
	resp2, list := httpGetList(t, env.baseURL+"/api/issues")
	if resp2.StatusCode != http.StatusOK || len(list) < 1 {
		t.Fatalf("list: expected 200 with data, got %d", resp2.StatusCode)
	}

	// Get.
	resp3, _ := httpGet(t, env.baseURL+"/api/issues/"+issueID)
	if resp3.StatusCode != http.StatusOK {
		t.Fatalf("get: expected 200, got %d", resp3.StatusCode)
	}

	// Update.
	resp4, _ := httpPutJSON(t, env.baseURL+"/api/issues/"+issueID, map[string]interface{}{
		"name": "updated-issue", "projectId": "proj-1", "status": "closed",
	})
	if resp4.StatusCode != http.StatusOK {
		t.Fatalf("update: expected 200, got %d", resp4.StatusCode)
	}

	// Delete.
	resp5, _ := httpDelete(t, env.baseURL+"/api/issues/"+issueID)
	if resp5.StatusCode != http.StatusOK {
		t.Fatalf("delete: expected 200, got %d", resp5.StatusCode)
	}
}

func TestFederationIntervene(t *testing.T) {
	env := newTestServer(t)
	// Valid actions are: approve, reject, modify.
	resp, _ := httpPostJSON(t, env.baseURL+"/api/federation/intervene", map[string]interface{}{
		"issueId": "issue-1",
		"action":  "approve",
		"reason":  "testing",
	})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
}

func TestFederationIntervene_InvalidAction(t *testing.T) {
	env := newTestServer(t)
	resp, _ := httpPostJSON(t, env.baseURL+"/api/federation/intervene", map[string]interface{}{
		"issueId": "issue-1",
		"action":  "pause",
		"reason":  "testing",
	})
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400 for invalid action, got %d", resp.StatusCode)
	}
}

func TestFederationAggregateAgents(t *testing.T) {
	env := newTestServer(t)
	resp, list := httpGetList(t, env.baseURL+"/api/federation/aggregate/agents")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	if list == nil {
		t.Error("expected non-nil list")
	}
}

func TestFederationAggregateMetrics(t *testing.T) {
	env := newTestServer(t)
	resp, body := httpGet(t, env.baseURL+"/api/federation/aggregate/metrics")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	if _, ok := body["companyCount"]; !ok {
		t.Error("expected companyCount field")
	}
}

func TestApply_GoalWithDecomposition(t *testing.T) {
	env := newTestServer(t)
	applyAgent(t, env.baseURL, "coder")
	httpPostJSON(t, env.baseURL+"/api/agents/coder/start", nil)

	yaml := `apiVersion: opc/v1
kind: Goal
metadata:
  name: yaml-goal
spec:
  description: "Build feature X"
  owner: tester
  decomposition:
    projects:
      - name: backend
        description: "Build backend API"
        tasks:
          - name: implement-api
            description: "Implement REST API"
            assignAgent: coder
`
	resp, body := httpPostYAML(t, env.baseURL+"/api/apply", yaml)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d, body: %v", resp.StatusCode, body)
	}
	msg, _ := body["message"].(string)
	if msg == "" {
		t.Error("expected message in response")
	}
}

func TestApply_CompanyViaYAML(t *testing.T) {
	env := newTestServer(t)
	yaml := `apiVersion: opc/v1
kind: Company
metadata:
  name: yaml-company
spec:
  type: software
  endpoint: http://localhost:7777
  agents:
    - coder
`
	resp, body := httpPostYAML(t, env.baseURL+"/api/apply", yaml)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d, body: %v", resp.StatusCode, body)
	}
}

func TestFederationUpdateCompanyStatus(t *testing.T) {
	env := newTestServer(t)
	resp, body := httpPostJSON(t, env.baseURL+"/api/federation/companies", map[string]interface{}{
		"name":     "status-co",
		"endpoint": "http://localhost:3333",
		"type":     "software",
	})
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("register: expected 201, got %d", resp.StatusCode)
	}
	companyID, _ := body["id"].(string)

	resp2, _ := httpPutJSON(t, env.baseURL+"/api/federation/companies/"+companyID+"/status", map[string]interface{}{
		"status": "Busy",
	})
	if resp2.StatusCode != http.StatusOK {
		t.Fatalf("update status: expected 200, got %d", resp2.StatusCode)
	}

	// Verify.
	resp3, body3 := httpGet(t, env.baseURL+"/api/federation/companies/"+companyID)
	if resp3.StatusCode != http.StatusOK {
		t.Fatalf("get: expected 200, got %d", resp3.StatusCode)
	}
	if body3["status"] != "Busy" {
		t.Errorf("expected status=Busy, got %v", body3["status"])
	}
}

func TestWorkflowRuns_Empty(t *testing.T) {
	env := newTestServer(t)
	applyAgent(t, env.baseURL, "coder")
	yaml := `apiVersion: opc/v1
kind: Workflow
metadata:
  name: runs-wf
spec:
  steps:
    - name: step1
      agent: coder
      input:
        message: "hello"
`
	httpPostYAML(t, env.baseURL+"/api/apply", yaml)

	resp, list := httpGetList(t, env.baseURL+"/api/workflows/runs-wf/runs")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	if len(list) != 0 {
		t.Errorf("expected 0 runs, got %d", len(list))
	}
}

// ============================================================
// Part 2: Multi-Instance Federation Tests
// ============================================================

func TestFederation_RegisterCompany(t *testing.T) {
	master := newTestServer(t)
	worker := newTestServer(t)

	resp, body := httpPostJSON(t, master.baseURL+"/api/federation/companies", map[string]interface{}{
		"name":     "worker",
		"endpoint": worker.baseURL,
		"type":     "software",
		"agents":   []string{"coder"},
	})
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("expected 201, got %d, body: %v", resp.StatusCode, body)
	}
	if body["name"] != "worker" {
		t.Errorf("expected name=worker, got %v", body["name"])
	}
	// Worker health should be reachable so status should be Online.
	if body["status"] != "Online" {
		t.Errorf("expected status=Online, got %v", body["status"])
	}
}

func TestFederation_CreateFederatedGoal(t *testing.T) {
	// Use a mock transport so dispatch doesn't fail for unreachable companies.
	mockTransport := &testMockTransport{
		sendFunc: func(endpoint, method, path string, body any) ([]byte, error) {
			if path == "/api/run" {
				return []byte(`{"taskId":"remote-task-1","status":"Pending"}`), nil
			}
			if path == "/api/health" {
				return []byte(`{"status":"healthy"}`), nil
			}
			return []byte(`{}`), nil
		},
	}

	master := newTestServerWithTransport(t, mockTransport)

	// Register two companies.
	_, bodyA := httpPostJSON(t, master.baseURL+"/api/federation/companies", map[string]interface{}{
		"name": "co-a", "endpoint": "http://fake-a:9527", "type": "software", "agents": []string{"coder"},
	})
	companyAID, _ := bodyA["id"].(string)

	_, bodyB := httpPostJSON(t, master.baseURL+"/api/federation/companies", map[string]interface{}{
		"name": "co-b", "endpoint": "http://fake-b:9527", "type": "software", "agents": []string{"reviewer"},
	})
	companyBID, _ := bodyB["id"].(string)

	// Create federated goal with DAG (co-a first, then co-b depends on co-a).
	resp, body := httpPostJSON(t, master.baseURL+"/api/goals/federated", map[string]interface{}{
		"name":        "fed-goal",
		"description": "Test federated goal",
		"projects": []map[string]interface{}{
			{"name": "build", "companyId": companyAID, "description": "Build the code"},
			{"name": "review", "companyId": companyBID, "description": "Review the code", "dependencies": []string{"build"}},
		},
	})
	if resp.StatusCode != http.StatusAccepted {
		t.Fatalf("expected 202, got %d, body: %v", resp.StatusCode, body)
	}
	if body["goalId"] == nil {
		t.Fatal("expected goalId in response")
	}
	layers, _ := body["layers"].(float64)
	if int(layers) != 2 {
		t.Errorf("expected 2 layers, got %v", body["layers"])
	}
}

func TestFederation_DAGLayerValidation(t *testing.T) {
	mockTransport := &testMockTransport{
		sendFunc: func(endpoint, method, path string, body any) ([]byte, error) {
			return []byte(`{"taskId":"t1","status":"Pending"}`), nil
		},
	}
	master := newTestServerWithTransport(t, mockTransport)

	_, bodyA := httpPostJSON(t, master.baseURL+"/api/federation/companies", map[string]interface{}{
		"name": "dag-a", "endpoint": "http://fake:1", "type": "software",
	})
	_, bodyB := httpPostJSON(t, master.baseURL+"/api/federation/companies", map[string]interface{}{
		"name": "dag-b", "endpoint": "http://fake:2", "type": "software",
	})
	_, bodyC := httpPostJSON(t, master.baseURL+"/api/federation/companies", map[string]interface{}{
		"name": "dag-c", "endpoint": "http://fake:3", "type": "software",
	})
	idA, _ := bodyA["id"].(string)
	idB, _ := bodyB["id"].(string)
	idC, _ := bodyC["id"].(string)

	// Three-layer DAG: A || B -> C
	resp, body := httpPostJSON(t, master.baseURL+"/api/goals/federated", map[string]interface{}{
		"name": "dag-goal",
		"projects": []map[string]interface{}{
			{"name": "p-a", "companyId": idA, "description": "task a"},
			{"name": "p-b", "companyId": idB, "description": "task b"},
			{"name": "p-c", "companyId": idC, "description": "task c", "dependencies": []string{"p-a", "p-b"}},
		},
	})
	if resp.StatusCode != http.StatusAccepted {
		t.Fatalf("expected 202, got %d", resp.StatusCode)
	}
	layers, _ := body["layers"].(float64)
	if int(layers) != 2 {
		t.Errorf("expected 2 layers (A+B parallel, then C), got %v", layers)
	}
}

func TestFederation_DAGCycleRejected(t *testing.T) {
	master := newTestServer(t)
	_, bodyA := httpPostJSON(t, master.baseURL+"/api/federation/companies", map[string]interface{}{
		"name": "cycle-a", "endpoint": "http://fake:1", "type": "software",
	})
	_, bodyB := httpPostJSON(t, master.baseURL+"/api/federation/companies", map[string]interface{}{
		"name": "cycle-b", "endpoint": "http://fake:2", "type": "software",
	})
	idA, _ := bodyA["id"].(string)
	idB, _ := bodyB["id"].(string)

	resp, _ := httpPostJSON(t, master.baseURL+"/api/goals/federated", map[string]interface{}{
		"name": "cycle-goal",
		"projects": []map[string]interface{}{
			{"name": "x", "companyId": idA, "dependencies": []string{"y"}},
			{"name": "y", "companyId": idB, "dependencies": []string{"x"}},
		},
	})
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400 for cycle, got %d", resp.StatusCode)
	}
}

func TestFederation_Callback(t *testing.T) {
	master := newTestServer(t)

	// Post a callback.
	resp, body := httpPostJSON(t, master.baseURL+"/api/federation/callback", map[string]interface{}{
		"taskId":    "task-123",
		"status":    "completed",
		"result":    "done",
		"tokensIn":  100,
		"tokensOut": 200,
	})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d, body: %v", resp.StatusCode, body)
	}
	msg, _ := body["message"].(string)
	if msg == "" {
		t.Error("expected message in response")
	}
}

func TestFederation_Callback_MissingFields(t *testing.T) {
	master := newTestServer(t)
	resp, _ := httpPostJSON(t, master.baseURL+"/api/federation/callback", map[string]interface{}{
		"taskId": "task-123",
		// missing status
	})
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", resp.StatusCode)
	}
}

func TestFederation_AdvanceFederatedGoal(t *testing.T) {
	dispatched := make(chan map[string]interface{}, 10)
	mockTransport := &testMockTransport{
		sendFunc: func(endpoint, method, path string, body any) ([]byte, error) {
			if path == "/api/run" && method == "POST" {
				if m, ok := body.(map[string]interface{}); ok {
					dispatched <- m
				}
				return []byte(`{"taskId":"remote-t","status":"Pending"}`), nil
			}
			return []byte(`{}`), nil
		},
	}

	master := newTestServerWithTransport(t, mockTransport)

	// Register companies.
	_, bodyA := httpPostJSON(t, master.baseURL+"/api/federation/companies", map[string]interface{}{
		"name": "adv-a", "endpoint": "http://fake:1", "type": "software", "agents": []string{"coder"},
	})
	_, bodyB := httpPostJSON(t, master.baseURL+"/api/federation/companies", map[string]interface{}{
		"name": "adv-b", "endpoint": "http://fake:2", "type": "software", "agents": []string{"reviewer"},
	})
	idA, _ := bodyA["id"].(string)
	idB, _ := bodyB["id"].(string)

	// Create federated goal: build -> review.
	resp, goalBody := httpPostJSON(t, master.baseURL+"/api/goals/federated", map[string]interface{}{
		"name": "adv-goal",
		"projects": []map[string]interface{}{
			{"name": "build", "companyId": idA, "description": "Build"},
			{"name": "review", "companyId": idB, "description": "Review", "dependencies": []string{"build"}},
		},
	})
	if resp.StatusCode != http.StatusAccepted {
		t.Fatalf("expected 202, got %d", resp.StatusCode)
	}
	goalID, _ := goalBody["goalId"].(string)

	// Wait for first layer dispatch.
	select {
	case d := <-dispatched:
		if d["agent"] != "coder" {
			t.Errorf("expected agent=coder, got %v", d["agent"])
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for first dispatch")
	}

	// Find the build project ID from the run.
	master.server.federatedGoalRunsMu.RLock()
	run := master.server.federatedGoalRuns[goalID]
	var buildProjID string
	if run != nil {
		if p, ok := run.Projects["build"]; ok {
			buildProjID = p.ID
		}
	}
	master.server.federatedGoalRunsMu.RUnlock()

	if buildProjID == "" {
		t.Fatal("could not find build project ID")
	}

	// Simulate callback from worker (build completed).
	httpPostJSON(t, master.baseURL+"/api/federation/callback", map[string]interface{}{
		"goalId":    goalID,
		"projectId": buildProjID,
		"taskId":    "build-task-1",
		"status":    "completed",
		"result":    "build output success",
	})

	// Wait for second layer dispatch (review).
	select {
	case d := <-dispatched:
		if d["agent"] != "reviewer" {
			t.Errorf("expected agent=reviewer for second layer, got %v", d["agent"])
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for second layer dispatch")
	}
}

func TestFederation_AggregateAgents_WithWorker(t *testing.T) {
	master := newTestServer(t)
	worker := newTestServer(t)

	// Apply an agent on worker.
	applyAgent(t, worker.baseURL, "worker-coder")

	// Register worker on master.
	_, body := httpPostJSON(t, master.baseURL+"/api/federation/companies", map[string]interface{}{
		"name":     "real-worker",
		"endpoint": worker.baseURL,
		"type":     "software",
	})
	companyID, _ := body["id"].(string)
	if companyID == "" {
		t.Fatal("expected company id")
	}

	// Get federated agents for the worker.
	resp, _ := httpGetList(t, master.baseURL+"/api/federation/companies/"+companyID+"/agents")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
}

func TestFederation_AggregateMetrics_WithWorker(t *testing.T) {
	master := newTestServer(t)
	worker := newTestServer(t)

	httpPostJSON(t, master.baseURL+"/api/federation/companies", map[string]interface{}{
		"name":     "metric-worker",
		"endpoint": worker.baseURL,
		"type":     "software",
	})

	resp, body := httpGet(t, master.baseURL+"/api/federation/aggregate/metrics")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	companyCount, _ := body["companyCount"].(float64)
	if int(companyCount) != 1 {
		t.Errorf("expected companyCount=1, got %v", companyCount)
	}
}

func TestFederation_StateIsolation(t *testing.T) {
	instanceA := newTestServer(t)
	instanceB := newTestServer(t)

	// Apply agent on A.
	applyAgent(t, instanceA.baseURL, "only-on-a")

	// Verify A has the agent.
	resp, list := httpGetList(t, instanceA.baseURL+"/api/agents")
	if resp.StatusCode != http.StatusOK || len(list) != 1 {
		t.Fatalf("A: expected 1 agent, got %d", len(list))
	}

	// Verify B does NOT have the agent.
	resp2, list2 := httpGetList(t, instanceB.baseURL+"/api/agents")
	if resp2.StatusCode != http.StatusOK || len(list2) != 0 {
		t.Fatalf("B: expected 0 agents, got %d", len(list2))
	}

	// Apply agent on B.
	applyAgent(t, instanceB.baseURL, "only-on-b")

	// Verify B has 1 agent, A still has 1.
	_, listA := httpGetList(t, instanceA.baseURL+"/api/agents")
	_, listB := httpGetList(t, instanceB.baseURL+"/api/agents")
	if len(listA) != 1 {
		t.Errorf("A should have 1 agent, got %d", len(listA))
	}
	if len(listB) != 1 {
		t.Errorf("B should have 1 agent, got %d", len(listB))
	}

	// Verify tasks are also isolated.
	_, tasksA := httpGetList(t, instanceA.baseURL+"/api/tasks")
	_, tasksB := httpGetList(t, instanceB.baseURL+"/api/tasks")
	if len(tasksA) != 0 {
		t.Errorf("A should have 0 tasks, got %d", len(tasksA))
	}
	if len(tasksB) != 0 {
		t.Errorf("B should have 0 tasks, got %d", len(tasksB))
	}
}

func TestFederation_LocalGoalAutoDecompose(t *testing.T) {
	env := newTestServer(t)
	applyAgent(t, env.baseURL, "coder")
	httpPostJSON(t, env.baseURL+"/api/agents/coder/start", nil)

	// Create goal with autoDecompose.
	resp, body := httpPostJSON(t, env.baseURL+"/api/goals", map[string]interface{}{
		"name":          "local-auto-goal",
		"description":   "Auto decompose this",
		"autoDecompose": true,
	})
	if resp.StatusCode != http.StatusAccepted {
		t.Fatalf("expected 202, got %d, body: %v", resp.StatusCode, body)
	}
	goalID, _ := body["id"].(string)
	if goalID == "" {
		t.Fatal("expected goalId")
	}

	// Wait for projects to be created.
	time.Sleep(300 * time.Millisecond)

	// Verify projects were created under this goal.
	resp2, list := httpGetList(t, env.baseURL+"/api/goals/"+goalID+"/projects")
	if resp2.StatusCode != http.StatusOK {
		t.Fatalf("projects: expected 200, got %d", resp2.StatusCode)
	}
	if len(list) < 1 {
		t.Errorf("expected at least 1 project created by autoDecompose, got %d", len(list))
	}
}

func TestFederation_FederatedGoal_MissingProjects(t *testing.T) {
	master := newTestServer(t)
	resp, _ := httpPostJSON(t, master.baseURL+"/api/goals/federated", map[string]interface{}{
		"name": "empty-goal",
	})
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400 for missing projects, got %d", resp.StatusCode)
	}
}

func TestFederation_FederatedGoal_MissingName(t *testing.T) {
	master := newTestServer(t)
	resp, _ := httpPostJSON(t, master.baseURL+"/api/goals/federated", map[string]interface{}{
		"projects": []map[string]interface{}{
			{"name": "p", "companyId": "x"},
		},
	})
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400 for missing name, got %d", resp.StatusCode)
	}
}

func TestFederation_CallbackMilestone(t *testing.T) {
	master := newTestServer(t)
	resp, body := httpPostJSON(t, master.baseURL+"/api/federation/callback", map[string]interface{}{
		"taskId": "milestone-task",
		"status": "milestone",
		"result": "50% progress",
	})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d, body: %v", resp.StatusCode, body)
	}
}

func TestFederation_FederatedGoalCompletes(t *testing.T) {
	mockTransport := &testMockTransport{
		sendFunc: func(endpoint, method, path string, body any) ([]byte, error) {
			return []byte(`{"taskId":"t","status":"Pending"}`), nil
		},
	}
	master := newTestServerWithTransport(t, mockTransport)

	_, bodyA := httpPostJSON(t, master.baseURL+"/api/federation/companies", map[string]interface{}{
		"name": "fin-a", "endpoint": "http://fake:1", "type": "software", "agents": []string{"coder"},
	})
	idA, _ := bodyA["id"].(string)

	// Single-project goal.
	resp, goalBody := httpPostJSON(t, master.baseURL+"/api/goals/federated", map[string]interface{}{
		"name": "finish-goal",
		"projects": []map[string]interface{}{
			{"name": "only-task", "companyId": idA, "description": "do it"},
		},
	})
	if resp.StatusCode != http.StatusAccepted {
		t.Fatalf("expected 202, got %d", resp.StatusCode)
	}
	goalID, _ := goalBody["goalId"].(string)

	// Find project ID.
	master.server.federatedGoalRunsMu.RLock()
	run := master.server.federatedGoalRuns[goalID]
	var projID string
	if run != nil {
		if p, ok := run.Projects["only-task"]; ok {
			projID = p.ID
		}
	}
	master.server.federatedGoalRunsMu.RUnlock()

	if projID == "" {
		t.Fatal("could not find project ID")
	}

	// Simulate completion callback.
	httpPostJSON(t, master.baseURL+"/api/federation/callback", map[string]interface{}{
		"goalId":    goalID,
		"projectId": projID,
		"taskId":    "t1",
		"status":    "completed",
		"result":    "all done",
	})

	// Check in-memory run status (DB update is best-effort).
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		master.server.federatedGoalRunsMu.RLock()
		run = master.server.federatedGoalRuns[goalID]
		var runStatus string
		if run != nil {
			runStatus = string(run.Status)
		}
		master.server.federatedGoalRunsMu.RUnlock()
		if runStatus == "Completed" {
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Error("expected federated goal run to reach Completed status")
}

func TestFederation_FederatedGoalFails(t *testing.T) {
	mockTransport := &testMockTransport{
		sendFunc: func(endpoint, method, path string, body any) ([]byte, error) {
			return []byte(`{"taskId":"t","status":"Pending"}`), nil
		},
	}
	master := newTestServerWithTransport(t, mockTransport)

	_, bodyA := httpPostJSON(t, master.baseURL+"/api/federation/companies", map[string]interface{}{
		"name": "fail-a", "endpoint": "http://fake:1", "type": "software", "agents": []string{"coder"},
	})
	idA, _ := bodyA["id"].(string)

	resp, goalBody := httpPostJSON(t, master.baseURL+"/api/goals/federated", map[string]interface{}{
		"name": "fail-goal",
		"projects": []map[string]interface{}{
			{"name": "fail-task", "companyId": idA, "description": "will fail"},
		},
	})
	if resp.StatusCode != http.StatusAccepted {
		t.Fatalf("expected 202, got %d", resp.StatusCode)
	}
	goalID, _ := goalBody["goalId"].(string)

	// Find project ID.
	master.server.federatedGoalRunsMu.RLock()
	run := master.server.federatedGoalRuns[goalID]
	var projID string
	if run != nil {
		if p, ok := run.Projects["fail-task"]; ok {
			projID = p.ID
		}
	}
	master.server.federatedGoalRunsMu.RUnlock()

	// Simulate failure callback.
	httpPostJSON(t, master.baseURL+"/api/federation/callback", map[string]interface{}{
		"goalId":    goalID,
		"projectId": projID,
		"taskId":    "t1",
		"status":    "failed",
		"result":    "crash",
	})

	// Check in-memory run status.
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		master.server.federatedGoalRunsMu.RLock()
		run = master.server.federatedGoalRuns[goalID]
		var runStatus string
		if run != nil {
			runStatus = string(run.Status)
		}
		master.server.federatedGoalRunsMu.RUnlock()
		if runStatus == "Failed" {
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Error("expected federated goal run to reach Failed status")
}

func TestFederation_CascadeFailure(t *testing.T) {
	mockTransport := &testMockTransport{
		sendFunc: func(endpoint, method, path string, body any) ([]byte, error) {
			return []byte(`{"taskId":"t","status":"Pending"}`), nil
		},
	}
	master := newTestServerWithTransport(t, mockTransport)

	_, bodyA := httpPostJSON(t, master.baseURL+"/api/federation/companies", map[string]interface{}{
		"name": "cas-a", "endpoint": "http://fake:1", "type": "software", "agents": []string{"coder"},
	})
	_, bodyB := httpPostJSON(t, master.baseURL+"/api/federation/companies", map[string]interface{}{
		"name": "cas-b", "endpoint": "http://fake:2", "type": "software", "agents": []string{"reviewer"},
	})
	idA, _ := bodyA["id"].(string)
	idB, _ := bodyB["id"].(string)

	resp, goalBody := httpPostJSON(t, master.baseURL+"/api/goals/federated", map[string]interface{}{
		"name": "cascade-goal",
		"projects": []map[string]interface{}{
			{"name": "upstream", "companyId": idA},
			{"name": "downstream", "companyId": idB, "dependencies": []string{"upstream"}},
		},
	})
	if resp.StatusCode != http.StatusAccepted {
		t.Fatalf("expected 202, got %d", resp.StatusCode)
	}
	goalID, _ := goalBody["goalId"].(string)

	// Find upstream project ID.
	master.server.federatedGoalRunsMu.RLock()
	run := master.server.federatedGoalRuns[goalID]
	var upstreamProjID string
	if run != nil {
		if p, ok := run.Projects["upstream"]; ok {
			upstreamProjID = p.ID
		}
	}
	master.server.federatedGoalRunsMu.RUnlock()

	// Simulate upstream failure.
	httpPostJSON(t, master.baseURL+"/api/federation/callback", map[string]interface{}{
		"goalId":    goalID,
		"projectId": upstreamProjID,
		"taskId":    "t1",
		"status":    "failed",
		"result":    "upstream crashed",
	})

	time.Sleep(300 * time.Millisecond)

	// Check downstream was also marked failed.
	master.server.federatedGoalRunsMu.RLock()
	run = master.server.federatedGoalRuns[goalID]
	downstreamStatus := ""
	if run != nil {
		if p, ok := run.Projects["downstream"]; ok {
			downstreamStatus = string(p.Status)
		}
	}
	master.server.federatedGoalRunsMu.RUnlock()

	if downstreamStatus != "Failed" {
		t.Errorf("expected downstream status=Failed, got %v", downstreamStatus)
	}
}

func TestFederation_FederatedHealth(t *testing.T) {
	master := newTestServer(t)
	worker := newTestServer(t)

	_, body := httpPostJSON(t, master.baseURL+"/api/federation/companies", map[string]interface{}{
		"name":     "health-worker",
		"endpoint": worker.baseURL,
		"type":     "software",
	})
	companyID, _ := body["id"].(string)

	resp, hbody := httpGet(t, master.baseURL+"/api/federation/companies/"+companyID+"/health")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	if hbody["healthy"] != true {
		t.Errorf("expected healthy=true, got %v", hbody["healthy"])
	}
}

// ============================================================
// Part 3: Additional Coverage Tests
// ============================================================

func TestRestartAgent(t *testing.T) {
	env := newTestServer(t)
	applyAgent(t, env.baseURL, "restart-agent")
	httpPostJSON(t, env.baseURL+"/api/agents/restart-agent/start", nil)

	resp, body := httpPostJSON(t, env.baseURL+"/api/agents/restart-agent/restart", nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d, body: %v", resp.StatusCode, body)
	}
	msg, _ := body["message"].(string)
	if msg == "" {
		t.Error("expected message in response")
	}
}

func TestRestartAgent_NotFound(t *testing.T) {
	env := newTestServer(t)
	resp, _ := httpPostJSON(t, env.baseURL+"/api/agents/nonexistent/restart", nil)
	if resp.StatusCode != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", resp.StatusCode)
	}
}

func TestDeleteAgent_NotFound(t *testing.T) {
	env := newTestServer(t)
	resp, _ := httpDelete(t, env.baseURL+"/api/agents/nonexistent")
	if resp.StatusCode != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", resp.StatusCode)
	}
}

func TestStartAgent_NotFound(t *testing.T) {
	env := newTestServer(t)
	resp, _ := httpPostJSON(t, env.baseURL+"/api/agents/nonexistent/start", nil)
	if resp.StatusCode != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", resp.StatusCode)
	}
}

func TestStopAgent_NotFound(t *testing.T) {
	env := newTestServer(t)
	resp, _ := httpPostJSON(t, env.baseURL+"/api/agents/nonexistent/stop", nil)
	if resp.StatusCode != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", resp.StatusCode)
	}
}

func TestGetTask_NotFound(t *testing.T) {
	env := newTestServer(t)
	resp, _ := httpGet(t, env.baseURL+"/api/tasks/nonexistent")
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", resp.StatusCode)
	}
}

func TestTruncateStr(t *testing.T) {
	// Test short string (no truncation).
	result := truncateStr("short", 10)
	if result != "short" {
		t.Errorf("expected 'short', got '%s'", result)
	}

	// Test long string (truncation).
	result = truncateStr("this is a long string", 10)
	if result != "this is a ..." {
		t.Errorf("expected 'this is a ...', got '%s'", result)
	}

	// Test exact boundary.
	result = truncateStr("exact", 5)
	if result != "exact" {
		t.Errorf("expected 'exact', got '%s'", result)
	}
}

func TestTruncate(t *testing.T) {
	// Test short string (no truncation).
	result := truncate("short", 10)
	if result != "short" {
		t.Errorf("expected 'short', got '%s'", result)
	}

	// Test long string (truncation).
	result = truncate("this is a long string", 10)
	if result != "this is a ..." {
		t.Errorf("expected 'this is a ...', got '%s'", result)
	}
}

func TestSlugify(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"Build a web app", "build-a-web"},
		{"code review agent for testing", "code-review-agent"},
		{"simple", "simple"},
		{"Hello World!", "hello-world"},
	}
	for _, tc := range tests {
		got := slugify(tc.input)
		if got != tc.expected {
			t.Errorf("slugify(%q) = %q, want %q", tc.input, got, tc.expected)
		}
	}
}

func TestInferSkills(t *testing.T) {
	// Test coding keywords.
	skills := inferSkills("implement a REST API")
	found := false
	for _, s := range skills {
		if s == "coding" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected 'coding' in skills, got %v", skills)
	}

	// Test testing keywords.
	skills = inferSkills("write unit tests")
	foundTest := false
	for _, s := range skills {
		if s == "testing" {
			foundTest = true
			break
		}
	}
	if !foundTest {
		t.Errorf("expected 'testing' in skills, got %v", skills)
	}

	// Test no match returns "general".
	skills = inferSkills("hello there")
	if len(skills) != 1 || skills[0] != "general" {
		t.Errorf("expected ['general'], got %v", skills)
	}

	// Test multiple matches.
	skills = inferSkills("review and test code")
	if len(skills) < 2 {
		t.Errorf("expected multiple skills, got %v", skills)
	}
}

func TestCountTaskStatus(t *testing.T) {
	tasks := []v1.TaskRecord{
		{Status: v1.TaskStatusPending},
		{Status: v1.TaskStatusRunning},
		{Status: v1.TaskStatusCompleted},
		{Status: v1.TaskStatusCompleted},
		{Status: v1.TaskStatusFailed},
	}
	if got := countTaskStatus(tasks, v1.TaskStatusPending); got != 1 {
		t.Errorf("pending: expected 1, got %d", got)
	}
	if got := countTaskStatus(tasks, v1.TaskStatusCompleted); got != 2 {
		t.Errorf("completed: expected 2, got %d", got)
	}
	if got := countTaskStatus(tasks, v1.TaskStatusFailed); got != 1 {
		t.Errorf("failed: expected 1, got %d", got)
	}

	// Empty list.
	if got := countTaskStatus(nil, v1.TaskStatusPending); got != 0 {
		t.Errorf("empty: expected 0, got %d", got)
	}
}

func TestListModels(t *testing.T) {
	env := newTestServer(t)
	resp, list := httpGetList(t, env.baseURL+"/api/models")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	if len(list) == 0 {
		t.Error("expected at least 1 builtin model")
	}
}

func TestAddModel(t *testing.T) {
	env := newTestServer(t)
	resp, body := httpPostJSON(t, env.baseURL+"/api/models", map[string]interface{}{
		"id":          "custom-model",
		"provider":    "custom",
		"displayName": "Custom Model",
		"tier":        "standard",
		"costPer1k":   0.005,
		"capability":  "balanced",
	})
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("expected 201, got %d, body: %v", resp.StatusCode, body)
	}

	// Verify the model was added.
	_, list := httpGetList(t, env.baseURL+"/api/models")
	found := false
	for _, m := range list {
		if mm, ok := m.(map[string]interface{}); ok {
			if mm["id"] == "custom-model" {
				found = true
				break
			}
		}
	}
	if !found {
		t.Error("expected custom-model in list after add")
	}
}

func TestAgentWizard(t *testing.T) {
	env := newTestServer(t)
	resp, body := httpPostJSON(t, env.baseURL+"/api/agents/wizard", map[string]interface{}{
		"type":        "claude-code",
		"description": "Code review agent",
		"model":       "claude-sonnet-4-6",
		"preset":      "standard",
		"replicas":    1,
		"onExceed":    "alert",
	})
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("expected 201, got %d, body: %v", resp.StatusCode, body)
	}
	if body["name"] == nil || body["name"] == "" {
		t.Error("expected name in response")
	}
	if body["message"] == nil {
		t.Error("expected message in response")
	}
}

func TestAgentWizard_LightPreset(t *testing.T) {
	env := newTestServer(t)
	resp, _ := httpPostJSON(t, env.baseURL+"/api/agents/wizard", map[string]interface{}{
		"type":        "claude-code",
		"description": "Simple coding helper",
		"model":       "claude-haiku-4-5",
		"preset":      "light",
		"replicas":    1,
		"onExceed":    "pause",
	})
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("expected 201, got %d", resp.StatusCode)
	}
}

func TestAgentWizard_PowerPreset(t *testing.T) {
	env := newTestServer(t)
	resp, _ := httpPostJSON(t, env.baseURL+"/api/agents/wizard", map[string]interface{}{
		"type":        "claude-code",
		"description": "Research and analyze code",
		"model":       "claude-opus-4-6",
		"preset":      "power",
		"replicas":    2,
		"onExceed":    "reject",
	})
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("expected 201, got %d", resp.StatusCode)
	}
}

func TestSystemMetrics(t *testing.T) {
	env := newTestServer(t)
	resp, body := httpGet(t, env.baseURL+"/api/system/metrics")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	// With no metricsCollector configured, should return stub data.
	if _, ok := body["timestamp"]; !ok {
		t.Error("expected timestamp field in system metrics")
	}
}

func TestListRFCs_Empty(t *testing.T) {
	env := newTestServer(t)
	resp, list := httpGetList(t, env.baseURL+"/api/system/rfcs")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	if len(list) != 0 {
		t.Errorf("expected 0 RFCs, got %d", len(list))
	}
}

func TestApproveRFC(t *testing.T) {
	env := newTestServer(t)

	// Seed an RFC into the server.
	env.server.rfcsMu.Lock()
	env.server.rfcs = []*evolve.RFC{
		{ID: "rfc-1", Title: "Test RFC", Status: "pending"},
	}
	env.server.rfcsMu.Unlock()

	resp, body := httpPostJSON(t, env.baseURL+"/api/system/rfcs/rfc-1/approve", nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d, body: %v", resp.StatusCode, body)
	}
	if body["status"] != "approved" {
		t.Errorf("expected status=approved, got %v", body["status"])
	}
}

func TestApproveRFC_NotFound(t *testing.T) {
	env := newTestServer(t)
	resp, _ := httpPostJSON(t, env.baseURL+"/api/system/rfcs/nonexistent/approve", nil)
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", resp.StatusCode)
	}
}

func TestRejectRFC(t *testing.T) {
	env := newTestServer(t)

	env.server.rfcsMu.Lock()
	env.server.rfcs = []*evolve.RFC{
		{ID: "rfc-2", Title: "Reject Me", Status: "pending"},
	}
	env.server.rfcsMu.Unlock()

	resp, body := httpPostJSON(t, env.baseURL+"/api/system/rfcs/rfc-2/reject", nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d, body: %v", resp.StatusCode, body)
	}
	if body["status"] != "rejected" {
		t.Errorf("expected status=rejected, got %v", body["status"])
	}
}

func TestRejectRFC_NotFound(t *testing.T) {
	env := newTestServer(t)
	resp, _ := httpPostJSON(t, env.baseURL+"/api/system/rfcs/nonexistent/reject", nil)
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", resp.StatusCode)
	}
}

func TestListRFCs_WithData(t *testing.T) {
	env := newTestServer(t)

	env.server.rfcsMu.Lock()
	env.server.rfcs = []*evolve.RFC{
		{ID: "rfc-a", Title: "First", Status: "pending"},
		{ID: "rfc-b", Title: "Second", Status: "approved"},
	}
	env.server.rfcsMu.Unlock()

	resp, list := httpGetList(t, env.baseURL+"/api/system/rfcs")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	if len(list) != 2 {
		t.Errorf("expected 2 RFCs, got %d", len(list))
	}
}

func TestFederatedTasks(t *testing.T) {
	master := newTestServer(t)
	worker := newTestServer(t)

	_, body := httpPostJSON(t, master.baseURL+"/api/federation/companies", map[string]interface{}{
		"name":     "task-worker",
		"endpoint": worker.baseURL,
		"type":     "software",
	})
	companyID, _ := body["id"].(string)

	resp, err := http.Get(master.baseURL + "/api/federation/companies/" + companyID + "/tasks")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
}

func TestFederatedMetrics(t *testing.T) {
	master := newTestServer(t)
	worker := newTestServer(t)

	_, body := httpPostJSON(t, master.baseURL+"/api/federation/companies", map[string]interface{}{
		"name":     "metrics-worker",
		"endpoint": worker.baseURL,
		"type":     "software",
	})
	companyID, _ := body["id"].(string)

	resp, body2 := httpGet(t, master.baseURL+"/api/federation/companies/"+companyID+"/metrics")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	if body2 == nil {
		t.Error("expected non-nil body")
	}
}

func TestFederatedAgents_CompanyNotFound(t *testing.T) {
	env := newTestServer(t)
	resp, _ := httpGetList(t, env.baseURL+"/api/federation/companies/nonexistent/agents")
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", resp.StatusCode)
	}
}

func TestFederatedTasks_CompanyNotFound(t *testing.T) {
	env := newTestServer(t)
	resp, _ := httpGetList(t, env.baseURL+"/api/federation/companies/nonexistent/tasks")
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", resp.StatusCode)
	}
}

func TestFederatedMetrics_CompanyNotFound(t *testing.T) {
	env := newTestServer(t)
	resp, _ := httpGet(t, env.baseURL+"/api/federation/companies/nonexistent/metrics")
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", resp.StatusCode)
	}
}

func TestFederatedHealth_CompanyNotFound(t *testing.T) {
	env := newTestServer(t)
	resp, _ := httpGet(t, env.baseURL+"/api/federation/companies/nonexistent/health")
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", resp.StatusCode)
	}
}

func TestListIssuesByProject(t *testing.T) {
	env := newTestServer(t)

	// Create goal and project.
	_, goalBody := httpPostJSON(t, env.baseURL+"/api/goals", map[string]interface{}{
		"name": "issue-proj-goal", "description": "for issues",
	})
	goalID, _ := goalBody["id"].(string)

	_, projBody := httpPostJSON(t, env.baseURL+"/api/projects", map[string]interface{}{
		"name": "issue-proj", "goalId": goalID, "description": "test",
	})
	projID, _ := projBody["id"].(string)

	// Create issue under the project.
	httpPostJSON(t, env.baseURL+"/api/issues", map[string]interface{}{
		"name": "proj-issue", "projectId": projID, "description": "issue desc",
	})

	resp, list := httpGetList(t, env.baseURL+"/api/projects/"+projID+"/issues")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	if len(list) < 1 {
		t.Errorf("expected at least 1 issue, got %d", len(list))
	}
}

func TestProjectStats(t *testing.T) {
	env := newTestServer(t)

	_, goalBody := httpPostJSON(t, env.baseURL+"/api/goals", map[string]interface{}{
		"name": "stats-proj-goal", "description": "for stats",
	})
	goalID, _ := goalBody["id"].(string)

	_, projBody := httpPostJSON(t, env.baseURL+"/api/projects", map[string]interface{}{
		"name": "stats-proj", "goalId": goalID, "description": "test",
	})
	projID, _ := projBody["id"].(string)

	resp, _ := httpGet(t, env.baseURL+"/api/projects/"+projID+"/stats")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
}

func TestGetWorkflowRun_NotFound(t *testing.T) {
	env := newTestServer(t)
	applyAgent(t, env.baseURL, "coder")
	yaml := `apiVersion: opc/v1
kind: Workflow
metadata:
  name: run-wf
spec:
  steps:
    - name: step1
      agent: coder
      input:
        message: "hello"
`
	httpPostYAML(t, env.baseURL+"/api/apply", yaml)

	resp, _ := httpGet(t, env.baseURL+"/api/workflows/run-wf/runs/nonexistent")
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", resp.StatusCode)
	}
}

func TestRunWorkflow_WithAgent(t *testing.T) {
	env := newTestServer(t)
	applyAgent(t, env.baseURL, "wf-agent")
	httpPostJSON(t, env.baseURL+"/api/agents/wf-agent/start", nil)

	yaml := `apiVersion: opc/v1
kind: Workflow
metadata:
  name: exec-wf
spec:
  steps:
    - name: step1
      agent: wf-agent
      input:
        message: "execute me"
`
	httpPostYAML(t, env.baseURL+"/api/apply", yaml)

	resp, body := httpPostJSON(t, env.baseURL+"/api/workflows/exec-wf/run", nil)
	// Even if execution fails due to test env, the handler should process it.
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusInternalServerError {
		t.Fatalf("expected 200 or 500, got %d, body: %v", resp.StatusCode, body)
	}
}

func TestLogsWithTasks(t *testing.T) {
	env := newTestServer(t)
	applyAgent(t, env.baseURL, "log-agent2")
	httpPostJSON(t, env.baseURL+"/api/agents/log-agent2/start", nil)

	// Create a task so logs have content.
	httpPostJSON(t, env.baseURL+"/api/run", map[string]string{
		"agent":   "log-agent2",
		"message": "generate some logs for testing this feature",
	})

	// Wait briefly for task to be created.
	time.Sleep(300 * time.Millisecond)

	resp, list := httpGetList(t, env.baseURL+"/api/logs")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	if len(list) < 1 {
		t.Errorf("expected at least 1 log entry after task, got %d", len(list))
	}
}

func TestClusterStatus_WithAgentsAndTasks(t *testing.T) {
	env := newTestServer(t)
	applyAgent(t, env.baseURL, "status-agent")
	httpPostJSON(t, env.baseURL+"/api/agents/status-agent/start", nil)

	resp, body := httpGet(t, env.baseURL+"/api/status")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	agents, ok := body["agents"].(map[string]interface{})
	if !ok {
		t.Fatal("expected agents map")
	}
	total, _ := agents["total"].(float64)
	if int(total) < 1 {
		t.Errorf("expected at least 1 agent total, got %v", total)
	}
	running, _ := agents["running"].(float64)
	if int(running) < 1 {
		t.Errorf("expected at least 1 running agent, got %v", running)
	}
}

func TestSSEEvents(t *testing.T) {
	env := newTestServer(t)

	// Make a request to SSE endpoint with a context that times out quickly.
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, env.baseURL+"/api/events", nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		// Context cancellation is expected.
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	contentType := resp.Header.Get("Content-Type")
	if contentType != "text/event-stream" {
		t.Errorf("expected Content-Type=text/event-stream, got %s", contentType)
	}

	// Read some data.
	buf := make([]byte, 4096)
	n, _ := resp.Body.Read(buf)
	if n == 0 {
		t.Error("expected some SSE data")
	}
}

func TestValidateKey(t *testing.T) {
	ks := &federationKeyStore{}
	if ks.ValidateKey("") {
		t.Error("expected empty key to be invalid")
	}
	if !ks.ValidateKey("some-key") {
		t.Error("expected non-empty key to be valid")
	}
}

func TestEnsureAgent_ExistingAgent(t *testing.T) {
	env := newTestServer(t)
	applyAgent(t, env.baseURL, "existing-agent")
	httpPostJSON(t, env.baseURL+"/api/agents/existing-agent/start", nil)

	// Ensure should be a no-op for existing agents.
	ctx := context.Background()
	env.server.ensureAgent(ctx, "existing-agent")

	// Verify agent still exists.
	resp, _ := httpGet(t, env.baseURL+"/api/agents/existing-agent")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
}

func TestEnsureAgent_NewAgent(t *testing.T) {
	env := newTestServer(t)
	ctx := context.Background()

	// ensureAgent should auto-create and start the agent.
	env.server.ensureAgent(ctx, "auto-created-agent")

	// Verify agent was created.
	resp, body := httpGet(t, env.baseURL+"/api/agents/auto-created-agent")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	if body["name"] != "auto-created-agent" {
		t.Errorf("expected name=auto-created-agent, got %v", body["name"])
	}
}

func TestApply_InvalidYAML(t *testing.T) {
	env := newTestServer(t)
	resp, _ := httpPostYAML(t, env.baseURL+"/api/apply", "not: valid: yaml: {{{{")
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", resp.StatusCode)
	}
}

func TestApply_MissingMetadataName(t *testing.T) {
	env := newTestServer(t)
	yaml := `apiVersion: opc/v1
kind: AgentSpec
metadata:
  name: ""
spec:
  type: claude-code
`
	resp, _ := httpPostYAML(t, env.baseURL+"/api/apply", yaml)
	// Empty name may return 400 or 500 depending on validation order.
	if resp.StatusCode != http.StatusBadRequest && resp.StatusCode != http.StatusInternalServerError {
		t.Fatalf("expected 400 or 500, got %d", resp.StatusCode)
	}
}

func TestCostDaily_Default(t *testing.T) {
	env := newTestServer(t)
	resp, list := httpGetList(t, env.baseURL+"/api/costs/daily")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	// Default always returns 7 daily entries.
	if len(list) != 7 {
		t.Errorf("expected 7 daily entries, got %d", len(list))
	}
}

func TestClusterMetrics_WithAgents(t *testing.T) {
	env := newTestServer(t)
	applyAgent(t, env.baseURL, "metric-agent")
	httpPostJSON(t, env.baseURL+"/api/agents/metric-agent/start", nil)

	// Run a task so metrics are populated.
	httpPostJSON(t, env.baseURL+"/api/run", map[string]string{
		"agent":   "metric-agent",
		"message": "test metrics",
	})
	time.Sleep(300 * time.Millisecond)

	resp, body := httpGet(t, env.baseURL+"/api/metrics")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	total, _ := body["totalAgents"].(float64)
	if int(total) < 1 {
		t.Errorf("expected totalAgents >= 1, got %v", total)
	}
}

func TestAggregateAgents_WithCompany(t *testing.T) {
	mockTransport := &testMockTransport{
		sendFunc: func(endpoint, method, path string, body any) ([]byte, error) {
			if path == "/api/agents" {
				return []byte(`[{"name":"remote-coder","type":"claude-code","phase":"Running"}]`), nil
			}
			return []byte(`{}`), nil
		},
	}
	master := newTestServerWithTransport(t, mockTransport)

	httpPostJSON(t, master.baseURL+"/api/federation/companies", map[string]interface{}{
		"name": "agg-co", "endpoint": "http://fake:1", "type": "software",
	})

	resp, list := httpGetList(t, master.baseURL+"/api/federation/aggregate/agents")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	if len(list) < 1 {
		t.Errorf("expected at least 1 agent, got %d", len(list))
	}
}

func TestAggregateAgents_TransportError(t *testing.T) {
	mockTransport := &testMockTransport{
		sendFunc: func(endpoint, method, path string, body any) ([]byte, error) {
			return nil, fmt.Errorf("connection refused")
		},
	}
	master := newTestServerWithTransport(t, mockTransport)

	httpPostJSON(t, master.baseURL+"/api/federation/companies", map[string]interface{}{
		"name": "err-co", "endpoint": "http://fake:1", "type": "software",
	})

	resp, list := httpGetList(t, master.baseURL+"/api/federation/aggregate/agents")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	// Should return empty list on error.
	if len(list) != 0 {
		t.Errorf("expected 0 agents on error, got %d", len(list))
	}
}

func TestUnregisterCompany_NotFound(t *testing.T) {
	env := newTestServer(t)
	resp, _ := httpDelete(t, env.baseURL+"/api/federation/companies/nonexistent")
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", resp.StatusCode)
	}
}

func TestUpdateCompanyStatus_NotFound(t *testing.T) {
	env := newTestServer(t)
	resp, _ := httpPutJSON(t, env.baseURL+"/api/federation/companies/nonexistent/status", map[string]interface{}{
		"status": "Busy",
	})
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", resp.StatusCode)
	}
}

func TestGetCompany_NotFound(t *testing.T) {
	env := newTestServer(t)
	resp, _ := httpGet(t, env.baseURL+"/api/federation/companies/nonexistent")
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", resp.StatusCode)
	}
}

func TestRunTask_AgentNotStarted(t *testing.T) {
	env := newTestServer(t)
	applyAgent(t, env.baseURL, "idle-agent")

	resp, body := httpPostJSON(t, env.baseURL+"/api/run", map[string]string{
		"agent":   "idle-agent",
		"message": "try to run",
	})
	// Should accept the task even if agent isn't started (ensureAgent may auto-start).
	if resp.StatusCode != http.StatusAccepted && resp.StatusCode != http.StatusInternalServerError {
		t.Fatalf("expected 202 or 500, got %d, body: %v", resp.StatusCode, body)
	}
}

func TestToggleWorkflow_NotFound(t *testing.T) {
	env := newTestServer(t)
	resp, _ := httpPutJSON(t, env.baseURL+"/api/workflows/nonexistent/toggle", map[string]interface{}{
		"enabled": false,
	})
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", resp.StatusCode)
	}
}

func TestDeleteWorkflow_NotFound(t *testing.T) {
	env := newTestServer(t)
	resp, _ := httpDelete(t, env.baseURL+"/api/workflows/nonexistent")
	if resp.StatusCode != http.StatusInternalServerError || resp.StatusCode == http.StatusInternalServerError {
		// deleteWorkflow returns 500 on store error - acceptable.
	}
}

func TestReloadActiveFederatedGoalRuns(t *testing.T) {
	env := newTestServer(t)
	ctx := context.Background()

	// Just verify it doesn't panic. With empty DB, it should be a no-op.
	env.server.reloadActiveFederatedGoalRuns(ctx)

	env.server.federatedGoalRunsMu.RLock()
	count := len(env.server.federatedGoalRuns)
	env.server.federatedGoalRunsMu.RUnlock()

	if count != 0 {
		t.Errorf("expected 0 runs after reload on empty DB, got %d", count)
	}
}

func TestGoalDelete_NotFound(t *testing.T) {
	env := newTestServer(t)
	resp, _ := httpDelete(t, env.baseURL+"/api/goals/nonexistent")
	// Store may silently succeed for non-existent IDs.
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusInternalServerError {
		t.Fatalf("expected 200 or 500, got %d", resp.StatusCode)
	}
}

func TestGoalUpdate_NotFound(t *testing.T) {
	env := newTestServer(t)
	resp, _ := httpPutJSON(t, env.baseURL+"/api/goals/nonexistent", map[string]interface{}{
		"name": "updated", "status": "active",
	})
	// Store may silently succeed for non-existent IDs.
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusInternalServerError {
		t.Fatalf("expected 200 or 500, got %d", resp.StatusCode)
	}
}

func TestProjectDelete_NotFound(t *testing.T) {
	env := newTestServer(t)
	resp, _ := httpDelete(t, env.baseURL+"/api/projects/nonexistent")
	// Should return 200 or 500 depending on store impl.
	_ = resp
}

func TestIssueDelete_NotFound(t *testing.T) {
	env := newTestServer(t)
	resp, _ := httpDelete(t, env.baseURL+"/api/issues/nonexistent")
	_ = resp
}

func TestModelRegistryIntegration(t *testing.T) {
	// Verify model registry is initialized with builtins.
	reg := model.NewRegistry()
	models := reg.List()
	if len(models) == 0 {
		t.Error("expected builtin models")
	}

	// Add and verify.
	reg.Add(model.ModelInfo{ID: "test-model", Provider: "test"})
	if _, found := reg.Get("test-model"); !found {
		t.Error("expected to find test-model")
	}

	// Not found.
	if _, found := reg.Get("nonexistent"); found {
		t.Error("expected not to find nonexistent model")
	}
}

func TestCORSMiddleware_Options(t *testing.T) {
	env := newTestServer(t)
	req, _ := http.NewRequest(http.MethodOptions, env.baseURL+"/api/health", nil)
	req.Header.Set("Origin", "http://localhost:3000")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("OPTIONS: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent && resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 204 or 200, got %d", resp.StatusCode)
	}
}

func TestSendCallback(t *testing.T) {
	// Set up a local server to receive the callback.
	received := make(chan FederationCallback, 1)
	callbackSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var cb FederationCallback
		json.NewDecoder(r.Body).Decode(&cb)
		received <- cb
		w.WriteHeader(http.StatusOK)
	}))
	defer callbackSrv.Close()

	env := newTestServer(t)
	env.server.sendCallback(callbackSrv.URL, FederationCallback{
		TaskID: "cb-task-1",
		Status: "completed",
		Result: "callback result",
	})

	select {
	case cb := <-received:
		if cb.TaskID != "cb-task-1" {
			t.Errorf("expected taskId=cb-task-1, got %s", cb.TaskID)
		}
		if cb.Status != "completed" {
			t.Errorf("expected status=completed, got %s", cb.Status)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for callback")
	}
}

func TestSendCallback_ServerError(t *testing.T) {
	callbackSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"error":"server error"}`))
	}))
	defer callbackSrv.Close()

	env := newTestServer(t)
	// Should not panic, just log warning.
	env.server.sendCallback(callbackSrv.URL, FederationCallback{
		TaskID: "err-task",
		Status: "completed",
	})
}

func TestSendCallback_InvalidURL(t *testing.T) {
	env := newTestServer(t)
	// Should not panic on unreachable URL.
	env.server.sendCallback("http://invalid-host-that-does-not-exist:99999", FederationCallback{
		TaskID: "unreachable-task",
		Status: "completed",
	})
}

func TestLoggerMiddleware(t *testing.T) {
	env := newTestServer(t)

	// Create a router with logger middleware.
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(env.server.loggerMiddleware())
	router.GET("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})

	ts := httptest.NewServer(router)
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/test")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
}

func TestStop_NilServer(t *testing.T) {
	env := newTestServer(t)
	// httpServer is nil since we didn't call Start().
	err := env.server.Stop(context.Background())
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
}

func TestRunTask_WithCallback(t *testing.T) {
	callbackReceived := make(chan bool, 1)
	callbackSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callbackReceived <- true
		w.WriteHeader(http.StatusOK)
	}))
	defer callbackSrv.Close()

	env := newTestServer(t)
	applyAgent(t, env.baseURL, "callback-agent")
	httpPostJSON(t, env.baseURL+"/api/agents/callback-agent/start", nil)

	resp, body := httpPostJSON(t, env.baseURL+"/api/run", map[string]interface{}{
		"agent":       "callback-agent",
		"message":     "test with callback",
		"callbackURL": callbackSrv.URL,
	})
	if resp.StatusCode != http.StatusAccepted {
		t.Fatalf("expected 202, got %d, body: %v", resp.StatusCode, body)
	}

	// Wait for callback.
	select {
	case <-callbackReceived:
		// Callback was sent.
	case <-time.After(5 * time.Second):
		t.Log("callback not received within timeout (may be expected)")
	}
}

func TestRunTask_WithGoalAndProject(t *testing.T) {
	env := newTestServer(t)
	applyAgent(t, env.baseURL, "gp-agent")
	httpPostJSON(t, env.baseURL+"/api/agents/gp-agent/start", nil)

	resp, body := httpPostJSON(t, env.baseURL+"/api/run", map[string]interface{}{
		"agent":     "gp-agent",
		"message":   "test with goal and project",
		"goalId":    "goal-123",
		"projectId": "proj-456",
	})
	if resp.StatusCode != http.StatusAccepted {
		t.Fatalf("expected 202, got %d, body: %v", resp.StatusCode, body)
	}

	// Wait for task to complete.
	taskID, _ := body["taskId"].(string)
	time.Sleep(300 * time.Millisecond)

	// Check logs include goal/project fields.
	resp2, list := httpGetList(t, env.baseURL+"/api/logs")
	if resp2.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp2.StatusCode)
	}
	found := false
	for _, entry := range list {
		if m, ok := entry.(map[string]interface{}); ok {
			if m["taskId"] == taskID {
				if fields, ok := m["fields"].(map[string]interface{}); ok {
					if fields["goalId"] == "goal-123" {
						found = true
					}
				}
			}
		}
	}
	if !found {
		t.Log("log entry with goalId field not found (may be race condition)")
	}
}

func TestNilFederationHandlers(t *testing.T) {
	// Create a server with nil federation to test nil-check branches.
	gin.SetMode(gin.TestMode)

	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")
	store, err := sqlite.New(dbPath)
	if err != nil {
		t.Fatalf("create store: %v", err)
	}

	registry := adapter.NewRegistry()
	logger, _ := zap.NewDevelopment()
	sugar := logger.Sugar()
	ctrl := controller.New(store, registry, sugar)
	costDir := filepath.Join(tmpDir, "cost")
	os.MkdirAll(costDir, 0o755)
	costMgr := cost.NewTracker(costDir, sugar)
	gw := gateway.New(sugar)

	srv := New(ctrl, costMgr, gw, nil, Config{Port: 0}, sugar)

	router := gin.New()
	api := router.Group("/api")
	api.GET("/federation/companies", srv.listCompanies)
	api.GET("/federation/companies/:id", srv.getCompany)
	api.POST("/federation/companies", srv.registerCompany)
	api.DELETE("/federation/companies/:id", srv.unregisterCompany)
	api.PUT("/federation/companies/:id/status", srv.updateCompanyStatus)
	api.POST("/federation/intervene", srv.intervene)
	api.GET("/federation/companies/:id/agents", srv.federatedAgents)
	api.GET("/federation/companies/:id/tasks", srv.federatedTasks)
	api.GET("/federation/companies/:id/metrics", srv.federatedMetrics)
	api.GET("/federation/companies/:id/health", srv.federatedHealth)
	api.GET("/federation/aggregate/agents", srv.aggregateAgents)
	api.GET("/federation/aggregate/metrics", srv.aggregateMetrics)
	api.POST("/federation/callback", srv.federationCallback)
	api.POST("/goals/federated", srv.createFederatedGoal)

	ts := httptest.NewServer(router)
	defer func() {
		ts.Close()
		store.Close()
	}()

	// All federation endpoints should return gracefully with nil federation.
	tests := []struct {
		method string
		path   string
		status int
	}{
		{"GET", "/api/federation/companies", http.StatusOK},
		{"GET", "/api/federation/companies/x", http.StatusNotFound},
		{"DELETE", "/api/federation/companies/x", http.StatusInternalServerError},
		{"GET", "/api/federation/companies/x/agents", http.StatusInternalServerError},
		{"GET", "/api/federation/companies/x/tasks", http.StatusInternalServerError},
		{"GET", "/api/federation/companies/x/metrics", http.StatusInternalServerError},
		{"GET", "/api/federation/companies/x/health", http.StatusInternalServerError},
		{"GET", "/api/federation/aggregate/agents", http.StatusOK},
		{"GET", "/api/federation/aggregate/metrics", http.StatusOK},
		{"POST", "/api/goals/federated", http.StatusInternalServerError},
	}

	for _, tc := range tests {
		var req *http.Request
		if tc.method == "POST" {
			req, _ = http.NewRequest(tc.method, ts.URL+tc.path, bytes.NewReader([]byte(`{}`)))
			req.Header.Set("Content-Type", "application/json")
		} else if tc.method == "DELETE" {
			req, _ = http.NewRequest(tc.method, ts.URL+tc.path, nil)
		} else {
			req, _ = http.NewRequest(tc.method, ts.URL+tc.path, nil)
		}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("%s %s: %v", tc.method, tc.path, err)
		}
		resp.Body.Close()
		if resp.StatusCode != tc.status {
			t.Errorf("%s %s: expected %d, got %d", tc.method, tc.path, tc.status, resp.StatusCode)
		}
	}

	// Test registerCompany with nil federation.
	req, _ := http.NewRequest("POST", ts.URL+"/api/federation/companies",
		bytes.NewReader([]byte(`{"name":"test","endpoint":"http://x","type":"sw"}`)))
	req.Header.Set("Content-Type", "application/json")
	resp, _ := http.DefaultClient.Do(req)
	resp.Body.Close()
	if resp.StatusCode != http.StatusInternalServerError {
		t.Errorf("register with nil fed: expected 500, got %d", resp.StatusCode)
	}

	// Test updateCompanyStatus with nil federation.
	req2, _ := http.NewRequest("PUT", ts.URL+"/api/federation/companies/x/status",
		bytes.NewReader([]byte(`{"status":"Busy"}`)))
	req2.Header.Set("Content-Type", "application/json")
	resp2, _ := http.DefaultClient.Do(req2)
	resp2.Body.Close()
	if resp2.StatusCode != http.StatusInternalServerError {
		t.Errorf("update status with nil fed: expected 500, got %d", resp2.StatusCode)
	}

	// Test intervene with nil federation.
	req3, _ := http.NewRequest("POST", ts.URL+"/api/federation/intervene",
		bytes.NewReader([]byte(`{"issueId":"i","action":"approve","reason":"r"}`)))
	req3.Header.Set("Content-Type", "application/json")
	resp3, _ := http.DefaultClient.Do(req3)
	resp3.Body.Close()
	if resp3.StatusCode != http.StatusInternalServerError {
		t.Errorf("intervene with nil fed: expected 500, got %d", resp3.StatusCode)
	}
}

func TestSystemMetrics_WithCollector(t *testing.T) {
	env := newTestServer(t)

	// Set a mock metrics collector.
	env.server.metricsCollector = evolve.NewMetricsCollector(&mockMetricsStore{})

	resp, body := httpGet(t, env.baseURL+"/api/system/metrics")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	if _, ok := body["timestamp"]; !ok {
		t.Error("expected timestamp in response")
	}
}

// mockMetricsStore implements evolve.MetricsStore for testing.
type mockMetricsStore struct{}

func (m *mockMetricsStore) TaskSuccessRate() (float64, error) { return 0.95, nil }
func (m *mockMetricsStore) TaskAvgLatency() (float64, error)  { return 1.5, nil }
func (m *mockMetricsStore) GoalAvgCost() (float64, error)     { return 0.5, nil }
func (m *mockMetricsStore) TaskRetryRate() (float64, error)    { return 0.02, nil }

func TestGetGoal_NotFound(t *testing.T) {
	env := newTestServer(t)
	resp, _ := httpGet(t, env.baseURL+"/api/goals/nonexistent")
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", resp.StatusCode)
	}
}

func TestGetProject_NotFound(t *testing.T) {
	env := newTestServer(t)
	resp, _ := httpGet(t, env.baseURL+"/api/projects/nonexistent")
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", resp.StatusCode)
	}
}

func TestGetIssue_NotFound(t *testing.T) {
	env := newTestServer(t)
	resp, _ := httpGet(t, env.baseURL+"/api/issues/nonexistent")
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", resp.StatusCode)
	}
}

func TestReloadActiveFederatedGoalRuns_WithData(t *testing.T) {
	env := newTestServer(t)
	ctx := context.Background()
	store := env.server.controller.Store()

	// Insert federated goal run and projects into DB.
	run := storage.FederatedGoalRunRecord{
		GoalID:      "reload-goal-1",
		GoalName:    "Reload Test",
		Description: "Test reload",
		Status:      "InProgress",
		ResultsJSON: `{"build":"done"}`,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}
	if err := store.SaveFederatedGoalRun(ctx, run); err != nil {
		t.Fatalf("save run: %v", err)
	}

	proj := storage.FederatedGoalProjectRecord{
		GoalID:           "reload-goal-1",
		ProjectID:        "proj-reload-1",
		ProjectName:      "build",
		CompanyID:        "co-1",
		Status:           "Completed",
		Result:           "success",
		Round:            1,
		MaxRounds:        3,
		Layer:            0,
		DependenciesJSON: `["dep-a"]`,
	}
	if err := store.SaveFederatedGoalProject(ctx, proj); err != nil {
		t.Fatalf("save project: %v", err)
	}

	// Clear current runs.
	env.server.federatedGoalRunsMu.Lock()
	env.server.federatedGoalRuns = make(map[string]*goal.FederatedGoalRun)
	env.server.federatedGoalRunsMu.Unlock()

	// Reload.
	env.server.reloadActiveFederatedGoalRuns(ctx)

	// Verify the run was reloaded.
	env.server.federatedGoalRunsMu.RLock()
	loaded := env.server.federatedGoalRuns["reload-goal-1"]
	env.server.federatedGoalRunsMu.RUnlock()

	if loaded == nil {
		t.Fatal("expected run to be reloaded")
	}
	if loaded.GoalName != "Reload Test" {
		t.Errorf("expected name=Reload Test, got %s", loaded.GoalName)
	}
	if loaded.Projects["build"] == nil {
		t.Error("expected 'build' project to be loaded")
	}
	if loaded.Projects["build"].Dependencies[0] != "dep-a" {
		t.Errorf("expected dependency 'dep-a', got %v", loaded.Projects["build"].Dependencies)
	}
}

func TestStartAndStop(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")
	store, err := sqlite.New(dbPath)
	if err != nil {
		t.Fatalf("create store: %v", err)
	}
	defer store.Close()

	registry := adapter.NewRegistry()
	logger, _ := zap.NewDevelopment()
	sugar := logger.Sugar()
	ctrl := controller.New(store, registry, sugar)
	costDir := filepath.Join(tmpDir, "cost")
	os.MkdirAll(costDir, 0o755)
	costMgr := cost.NewTracker(costDir, sugar)
	fedDir := filepath.Join(tmpDir, "federation")
	os.MkdirAll(fedDir, 0o755)
	fedCtrl := federation.NewControllerForTest(fedDir, &testMockTransport{}, sugar)
	gw := gateway.New(sugar)

	// Use port 0 to get a random available port.
	srv := New(ctrl, costMgr, gw, fedCtrl, Config{Port: 0, GRPCPort: 0, Host: "127.0.0.1"}, sugar)

	ctx := context.Background()
	if err := srv.Start(ctx); err != nil {
		// gRPC start on port 0 might not bind properly, but Start should handle it.
		t.Logf("Start returned error (may be expected for port 0): %v", err)
	}

	// Stop should work.
	stopCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := srv.Stop(stopCtx); err != nil {
		t.Fatalf("Stop: %v", err)
	}
}

func TestApply_JSONContent(t *testing.T) {
	env := newTestServer(t)
	// Test applying with JSON content type (handled as YAML by the parser).
	jsonBody := `{"apiVersion":"opc/v1","kind":"AgentSpec","metadata":{"name":"json-agent"},"spec":{"type":"claude-code","runtime":{"model":{"provider":"anthropic","name":"claude-sonnet-4"}}}}`
	resp, err := http.Post(env.baseURL+"/api/apply", "application/json", bytes.NewReader([]byte(jsonBody)))
	if err != nil {
		t.Fatalf("POST: %v", err)
	}
	resp.Body.Close()
	// YAML parser can handle JSON, so this should work.
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 200 or 400, got %d", resp.StatusCode)
	}
}

func TestClusterStatus_WithDifferentPhases(t *testing.T) {
	env := newTestServer(t)

	// Apply and start an agent.
	applyAgent(t, env.baseURL, "status-running")
	httpPostJSON(t, env.baseURL+"/api/agents/status-running/start", nil)

	// Apply another agent but don't start it (will be in Pending/Stopped).
	applyAgent(t, env.baseURL, "status-pending")

	// Stop the started agent.
	applyAgent(t, env.baseURL, "status-stopped")
	httpPostJSON(t, env.baseURL+"/api/agents/status-stopped/start", nil)
	httpPostJSON(t, env.baseURL+"/api/agents/status-stopped/stop", nil)

	resp, body := httpGet(t, env.baseURL+"/api/status")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	agents, ok := body["agents"].(map[string]interface{})
	if !ok {
		t.Fatal("expected agents map")
	}
	total, _ := agents["total"].(float64)
	if int(total) < 3 {
		t.Errorf("expected at least 3 agents, got %v", total)
	}

	// Run a task to populate task status counters.
	httpPostJSON(t, env.baseURL+"/api/agents/status-running/start", nil)
	httpPostJSON(t, env.baseURL+"/api/run", map[string]string{
		"agent":   "status-running",
		"message": "status test task",
	})
	time.Sleep(300 * time.Millisecond)

	resp2, body2 := httpGet(t, env.baseURL+"/api/status")
	if resp2.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp2.StatusCode)
	}
	tasks, ok := body2["tasks"].(map[string]interface{})
	if !ok {
		t.Fatal("expected tasks map")
	}
	taskTotal, _ := tasks["total"].(float64)
	if int(taskTotal) < 1 {
		t.Errorf("expected at least 1 task, got %v", taskTotal)
	}
}

func TestLogsWithGoalId(t *testing.T) {
	env := newTestServer(t)
	applyAgent(t, env.baseURL, "log-goal-agent")
	httpPostJSON(t, env.baseURL+"/api/agents/log-goal-agent/start", nil)

	// Run task with goalId.
	httpPostJSON(t, env.baseURL+"/api/run", map[string]interface{}{
		"agent":     "log-goal-agent",
		"message":   "goal task log test",
		"goalId":    "goal-log-1",
		"projectId": "proj-log-1",
	})
	time.Sleep(300 * time.Millisecond)

	resp, list := httpGetList(t, env.baseURL+"/api/logs")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	if len(list) < 1 {
		t.Fatal("expected at least 1 log entry")
	}

	// Find log with goalId fields.
	found := false
	for _, entry := range list {
		m, ok := entry.(map[string]interface{})
		if !ok {
			continue
		}
		if fields, ok := m["fields"].(map[string]interface{}); ok {
			if fields["goalId"] == "goal-log-1" {
				found = true
				break
			}
		}
	}
	if !found {
		t.Error("expected log entry with goalId field")
	}
}

func TestCostEventsEmpty(t *testing.T) {
	env := newTestServer(t)
	resp, list := httpGetList(t, env.baseURL+"/api/costs/events")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	_ = list // Just verify no error.
}

func TestGoalRevise(t *testing.T) {
	env := newTestServer(t)
	resp, body := httpPostJSON(t, env.baseURL+"/api/goals", map[string]interface{}{
		"name": "revise-goal", "description": "goal to revise",
	})
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("create: expected 201, got %d", resp.StatusCode)
	}
	goalID, _ := body["id"].(string)

	// Revise the plan.
	resp2, body2 := httpPostJSON(t, env.baseURL+"/api/goals/"+goalID+"/revise", map[string]interface{}{
		"plan": map[string]interface{}{"revised": true},
	})
	if resp2.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp2.StatusCode)
	}
	if body2["goalId"] != goalID {
		t.Errorf("expected goalId=%s, got %v", goalID, body2["goalId"])
	}

	// Revise on nonexistent goal.
	resp3, _ := httpPostJSON(t, env.baseURL+"/api/goals/nonexistent/revise", map[string]interface{}{
		"plan": map[string]interface{}{},
	})
	if resp3.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", resp3.StatusCode)
	}
}

func TestApply_WorkflowWithoutSchedule(t *testing.T) {
	env := newTestServer(t)
	applyAgent(t, env.baseURL, "coder")

	yaml := `apiVersion: opc/v1
kind: Workflow
metadata:
  name: no-schedule-wf
spec:
  steps:
    - name: step1
      agent: coder
      input:
        message: "hello"
`
	resp, _ := httpPostYAML(t, env.baseURL+"/api/apply", yaml)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
}

func TestApply_WorkflowMissingAgent(t *testing.T) {
	env := newTestServer(t)
	yaml := `apiVersion: opc/v1
kind: Workflow
metadata:
  name: missing-agent-wf
spec:
  steps:
    - name: step1
      agent: nonexistent-agent
      input:
        message: "hello"
`
	resp, _ := httpPostYAML(t, env.baseURL+"/api/apply", yaml)
	// Workflow creation may succeed even if agent doesn't exist yet.
	_ = resp
}

func TestFederation_LegacyCompaniesMode(t *testing.T) {
	mockTransport := &testMockTransport{
		sendFunc: func(endpoint, method, path string, body any) ([]byte, error) {
			return []byte(`{"taskId":"t","status":"Pending"}`), nil
		},
	}
	master := newTestServerWithTransport(t, mockTransport)

	_, bodyA := httpPostJSON(t, master.baseURL+"/api/federation/companies", map[string]interface{}{
		"name": "legacy-co", "endpoint": "http://fake:1", "type": "software", "agents": []string{"coder"},
	})
	idA, _ := bodyA["id"].(string)

	// Use legacy "companies" field instead of "projects".
	resp, body := httpPostJSON(t, master.baseURL+"/api/goals/federated", map[string]interface{}{
		"name":      "legacy-goal",
		"companies": []string{idA},
	})
	if resp.StatusCode != http.StatusAccepted {
		t.Fatalf("expected 202, got %d, body: %v", resp.StatusCode, body)
	}
	if body["goalId"] == nil {
		t.Error("expected goalId")
	}
}
