package goal

import (
	"context"
	"fmt"
	"strings"
	"testing"

	v1 "github.com/zlc-ai/opc-platform/api/v1"
	"go.uber.org/zap"
)

// mockAgentController implements AgentController for testing.
type mockAgentController struct {
	executeFunc  func(ctx context.Context, task v1.TaskRecord) (ExecuteResult, error)
	applyFunc    func(ctx context.Context, spec v1.AgentSpec) error
	startFunc    func(ctx context.Context, name string) error
	getAgentFunc func(ctx context.Context, name string) (v1.AgentRecord, error)
}

func (m *mockAgentController) ExecuteTask(ctx context.Context, task v1.TaskRecord) (ExecuteResult, error) {
	if m.executeFunc != nil {
		return m.executeFunc(ctx, task)
	}
	return ExecuteResult{}, nil
}

func (m *mockAgentController) Apply(ctx context.Context, spec v1.AgentSpec) error {
	if m.applyFunc != nil {
		return m.applyFunc(ctx, spec)
	}
	return nil
}

func (m *mockAgentController) StartAgent(ctx context.Context, name string) error {
	if m.startFunc != nil {
		return m.startFunc(ctx, name)
	}
	return nil
}

func (m *mockAgentController) GetAgent(ctx context.Context, name string) (v1.AgentRecord, error) {
	if m.getAgentFunc != nil {
		return m.getAgentFunc(ctx, name)
	}
	return v1.AgentRecord{}, nil
}

func testLogger() *zap.SugaredLogger {
	return zap.NewNop().Sugar()
}

// validAIJSON returns a valid AI decomposition JSON string.
func validAIJSON() string {
	return `{
		"projects": [
			{
				"name": "backend-api",
				"description": "Build REST API",
				"tasks": [
					{
						"name": "setup-server",
						"description": "Initialize HTTP server with routing",
						"assignAgent": "backend-coder",
						"complexity": "medium",
						"issues": [
							{
								"name": "create-main-go",
								"description": "Create main.go with server setup"
							}
						]
					}
				]
			}
		]
	}`
}

// --- TestAIDecomposer_Decompose_Success ---

func TestAIDecomposer_Decompose_Success(t *testing.T) {
	ctrl := &mockAgentController{
		getAgentFunc: func(ctx context.Context, name string) (v1.AgentRecord, error) {
			// Agent already exists.
			return v1.AgentRecord{Name: name}, nil
		},
		executeFunc: func(ctx context.Context, task v1.TaskRecord) (ExecuteResult, error) {
			return ExecuteResult{Output: validAIJSON()}, nil
		},
	}

	d := NewAIDecomposer(ctrl, testLogger())

	result, err := d.Decompose(context.Background(), DecomposeRequest{
		GoalID:      "goal-1",
		GoalName:    "Build Platform",
		Description: "Build the OPC platform",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(result.Projects) != 1 {
		t.Fatalf("expected 1 project, got %d", len(result.Projects))
	}
	if result.Projects[0].Name != "backend-api" {
		t.Errorf("expected project name 'backend-api', got %s", result.Projects[0].Name)
	}
	if result.Projects[0].GoalID != "goal-1" {
		t.Errorf("expected goalID 'goal-1', got %s", result.Projects[0].GoalID)
	}
	if len(result.Projects[0].Tasks) != 1 {
		t.Fatalf("expected 1 task, got %d", len(result.Projects[0].Tasks))
	}
	if len(result.Projects[0].Tasks[0].Issues) != 1 {
		t.Fatalf("expected 1 issue, got %d", len(result.Projects[0].Tasks[0].Issues))
	}
}

// --- TestAIDecomposer_Decompose_RetryOnInvalidJSON ---

func TestAIDecomposer_Decompose_RetryOnInvalidJSON(t *testing.T) {
	callCount := 0
	ctrl := &mockAgentController{
		getAgentFunc: func(ctx context.Context, name string) (v1.AgentRecord, error) {
			return v1.AgentRecord{Name: name}, nil
		},
		executeFunc: func(ctx context.Context, task v1.TaskRecord) (ExecuteResult, error) {
			callCount++
			if callCount == 1 {
				return ExecuteResult{Output: "not valid json {{"}, nil
			}
			return ExecuteResult{Output: validAIJSON()}, nil
		},
	}

	d := NewAIDecomposer(ctrl, testLogger())

	result, err := d.Decompose(context.Background(), DecomposeRequest{
		GoalID:      "goal-retry",
		GoalName:    "Retry Test",
		Description: "Test retry",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if callCount < 2 {
		t.Errorf("expected at least 2 calls, got %d", callCount)
	}
	if len(result.Projects) != 1 {
		t.Fatalf("expected 1 project, got %d", len(result.Projects))
	}
}

// --- TestAIDecomposer_Decompose_AllRetriesFail ---

func TestAIDecomposer_Decompose_AllRetriesFail(t *testing.T) {
	ctrl := &mockAgentController{
		getAgentFunc: func(ctx context.Context, name string) (v1.AgentRecord, error) {
			return v1.AgentRecord{Name: name}, nil
		},
		executeFunc: func(ctx context.Context, task v1.TaskRecord) (ExecuteResult, error) {
			return ExecuteResult{Output: "garbage {{"}, nil
		},
	}

	d := NewAIDecomposer(ctrl, testLogger())

	_, err := d.Decompose(context.Background(), DecomposeRequest{
		GoalID:      "goal-fail",
		GoalName:    "Fail Test",
		Description: "All retries fail",
	})
	if err == nil {
		t.Fatal("expected error after all retries fail")
	}
	if !strings.Contains(err.Error(), "failed after") {
		t.Errorf("expected 'failed after' in error, got: %v", err)
	}
}

// --- TestAIDecomposer_ValidateResult_EmptyProjects ---

func TestAIDecomposer_ValidateResult_EmptyProjects(t *testing.T) {
	d := NewAIDecomposer(nil, testLogger())
	err := d.validateResult(&AIDecomposeResult{Projects: []AIProject{}})
	if err == nil {
		t.Fatal("expected error for empty projects")
	}
	if !strings.Contains(err.Error(), "no projects") {
		t.Errorf("expected 'no projects' error, got: %v", err)
	}
}

// --- TestAIDecomposer_ValidateResult_EmptyTaskName ---

func TestAIDecomposer_ValidateResult_EmptyTaskName(t *testing.T) {
	d := NewAIDecomposer(nil, testLogger())
	result := &AIDecomposeResult{
		Projects: []AIProject{
			{
				Name: "proj-1",
				Tasks: []AITask{
					{
						Name:        "",
						Description: "something",
					},
				},
			},
		},
	}
	err := d.validateResult(result)
	if err == nil {
		t.Fatal("expected error for empty task name")
	}
	if !strings.Contains(err.Error(), "empty name") {
		t.Errorf("expected 'empty name' error, got: %v", err)
	}
}

// --- TestAIDecomposer_ValidateResult_InvalidAgentName ---

func TestAIDecomposer_ValidateResult_InvalidAgentName(t *testing.T) {
	d := NewAIDecomposer(nil, testLogger())
	result := &AIDecomposeResult{
		Projects: []AIProject{
			{
				Name: "proj-1",
				Tasks: []AITask{
					{
						Name:        "task-1",
						Description: "do something",
						AssignAgent: "InvalidAgent",
						Complexity:  "low",
					},
				},
			},
		},
	}
	err := d.validateResult(result)
	if err == nil {
		t.Fatal("expected error for invalid agent name with uppercase")
	}
	if !strings.Contains(err.Error(), "invalid agent name") {
		t.Errorf("expected 'invalid agent name' error, got: %v", err)
	}
}

// --- TestAIDecomposer_ValidateResult_InvalidComplexity ---

func TestAIDecomposer_ValidateResult_InvalidComplexity(t *testing.T) {
	d := NewAIDecomposer(nil, testLogger())
	result := &AIDecomposeResult{
		Projects: []AIProject{
			{
				Name: "proj-1",
				Tasks: []AITask{
					{
						Name:        "task-1",
						Description: "do something",
						AssignAgent: "backend-coder",
						Complexity:  "extreme",
					},
				},
			},
		},
	}
	err := d.validateResult(result)
	if err == nil {
		t.Fatal("expected error for invalid complexity")
	}
	if !strings.Contains(err.Error(), "invalid complexity") {
		t.Errorf("expected 'invalid complexity' error, got: %v", err)
	}
}

// --- TestValidateConstraints_MaxProjects ---

func TestValidateConstraints_MaxProjects(t *testing.T) {
	result := &AIDecomposeResult{
		Projects: []AIProject{
			{Name: "p1", Tasks: []AITask{{Name: "t1", Description: "d1"}}},
			{Name: "p2", Tasks: []AITask{{Name: "t2", Description: "d2"}}},
			{Name: "p3", Tasks: []AITask{{Name: "t3", Description: "d3"}}},
		},
	}
	constraints := &v1.DecomposeConstraints{MaxProjects: 2}
	err := validateConstraints(result, constraints)
	if err == nil {
		t.Fatal("expected error for exceeding maxProjects")
	}
	if !strings.Contains(err.Error(), "3 projects") {
		t.Errorf("expected '3 projects' in error, got: %v", err)
	}
}

// --- TestValidateConstraints_MaxTasksPerProject ---

func TestValidateConstraints_MaxTasksPerProject(t *testing.T) {
	result := &AIDecomposeResult{
		Projects: []AIProject{
			{
				Name: "p1",
				Tasks: []AITask{
					{Name: "t1", Description: "d1"},
					{Name: "t2", Description: "d2"},
					{Name: "t3", Description: "d3"},
				},
			},
		},
	}
	constraints := &v1.DecomposeConstraints{MaxTasksPerProject: 2}
	err := validateConstraints(result, constraints)
	if err == nil {
		t.Fatal("expected error for exceeding maxTasksPerProject")
	}
	if !strings.Contains(err.Error(), "3 tasks") {
		t.Errorf("expected '3 tasks' in error, got: %v", err)
	}
}

// --- TestValidateConstraints_MaxAgents ---

func TestValidateConstraints_MaxAgents(t *testing.T) {
	result := &AIDecomposeResult{
		Projects: []AIProject{
			{
				Name: "p1",
				Tasks: []AITask{
					{Name: "t1", Description: "d1", AssignAgent: "agent-a"},
					{Name: "t2", Description: "d2", AssignAgent: "agent-b"},
					{Name: "t3", Description: "d3", AssignAgent: "agent-c"},
				},
			},
		},
	}
	constraints := &v1.DecomposeConstraints{MaxAgents: 2}
	err := validateConstraints(result, constraints)
	if err == nil {
		t.Fatal("expected error for exceeding maxAgents")
	}
	if !strings.Contains(err.Error(), "3 unique agents") {
		t.Errorf("expected '3 unique agents' in error, got: %v", err)
	}
}

// --- TestExtractJSON_CodeFence ---

func TestExtractJSON_CodeFence(t *testing.T) {
	input := "Here is the result:\n```json\n{\"projects\": []}\n```\nDone."
	got := extractJSON(input)
	if got != `{"projects": []}` {
		t.Errorf("expected extracted JSON, got: %s", got)
	}
}

// --- TestExtractJSON_BareJSON ---

func TestExtractJSON_BareJSON(t *testing.T) {
	input := "Some text before {\"key\": \"value\"} and after"
	got := extractJSON(input)
	if got != `{"key": "value"}` {
		t.Errorf("expected extracted bare JSON, got: %s", got)
	}
}

// --- TestExtractJSON_NoJSON ---

func TestExtractJSON_NoJSON(t *testing.T) {
	input := "no json here at all"
	got := extractJSON(input)
	if got != input {
		t.Errorf("expected original text returned, got: %s", got)
	}
}

// --- TestBuildDecompositionPrompt_WithConstraints ---

func TestBuildDecompositionPrompt_WithConstraints(t *testing.T) {
	constraints := &v1.DecomposeConstraints{
		MaxProjects:        5,
		MaxTasksPerProject: 10,
		MaxAgents:          3,
		MaxBudget:          "$100",
	}
	prompt := BuildDecompositionPrompt("My Goal", "Build something great", constraints)

	if !strings.Contains(prompt, "My Goal") {
		t.Error("prompt should contain goal name")
	}
	if !strings.Contains(prompt, "Build something great") {
		t.Error("prompt should contain description")
	}
	if !strings.Contains(prompt, "Maximum number of projects: 5") {
		t.Error("prompt should contain maxProjects constraint")
	}
	if !strings.Contains(prompt, "Maximum tasks per project: 10") {
		t.Error("prompt should contain maxTasksPerProject constraint")
	}
	if !strings.Contains(prompt, "Maximum number of unique agents: 3") {
		t.Error("prompt should contain maxAgents constraint")
	}
	if !strings.Contains(prompt, "Maximum cost budget: $100") {
		t.Error("prompt should contain maxBudget constraint")
	}
}

// --- TestBuildDecompositionPrompt_NoConstraints ---

func TestBuildDecompositionPrompt_NoConstraints(t *testing.T) {
	prompt := BuildDecompositionPrompt("Simple Goal", "Do a thing", nil)

	if !strings.Contains(prompt, "Simple Goal") {
		t.Error("prompt should contain goal name")
	}
	if !strings.Contains(prompt, "Do a thing") {
		t.Error("prompt should contain description")
	}
	if strings.Contains(prompt, "Constraints:") {
		t.Error("prompt should not contain constraints section when nil")
	}
}

// --- TestStaticDecomposer_Interface ---

func TestStaticDecomposer_Interface(t *testing.T) {
	var _ Decomposer = NewStaticDecomposer(testLogger())
}

// --- TestAIDecomposer_Interface ---

func TestAIDecomposer_Interface(t *testing.T) {
	var _ Decomposer = NewAIDecomposer(nil, testLogger())
}

// --- TestAIDecomposer_EnsureDecomposerAgent_CreatesNew ---

func TestAIDecomposer_EnsureDecomposerAgent_CreatesNew(t *testing.T) {
	applyCalled := false
	startCalled := false

	ctrl := &mockAgentController{
		getAgentFunc: func(ctx context.Context, name string) (v1.AgentRecord, error) {
			return v1.AgentRecord{}, fmt.Errorf("not found")
		},
		applyFunc: func(ctx context.Context, spec v1.AgentSpec) error {
			applyCalled = true
			if spec.Metadata.Name != decomposerAgentName {
				t.Errorf("expected agent name %s, got %s", decomposerAgentName, spec.Metadata.Name)
			}
			return nil
		},
		startFunc: func(ctx context.Context, name string) error {
			startCalled = true
			return nil
		},
		executeFunc: func(ctx context.Context, task v1.TaskRecord) (ExecuteResult, error) {
			return ExecuteResult{Output: validAIJSON()}, nil
		},
	}

	d := NewAIDecomposer(ctrl, testLogger())
	_, err := d.Decompose(context.Background(), DecomposeRequest{
		GoalID:      "g1",
		GoalName:    "Test",
		Description: "Test",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !applyCalled {
		t.Error("expected Apply to be called for new agent")
	}
	if !startCalled {
		t.Error("expected StartAgent to be called for new agent")
	}
}

// --- TestAIDecomposer_ValidateResult_EmptyProjectName ---

func TestAIDecomposer_ValidateResult_EmptyProjectName(t *testing.T) {
	d := NewAIDecomposer(nil, testLogger())
	result := &AIDecomposeResult{
		Projects: []AIProject{
			{
				Name:  "",
				Tasks: []AITask{{Name: "t1", Description: "d1"}},
			},
		},
	}
	err := d.validateResult(result)
	if err == nil {
		t.Fatal("expected error for empty project name")
	}
	if !strings.Contains(err.Error(), "empty name") {
		t.Errorf("expected 'empty name' error, got: %v", err)
	}
}

// --- TestAIDecomposer_ValidateResult_EmptyTaskDescription ---

func TestAIDecomposer_ValidateResult_EmptyTaskDescription(t *testing.T) {
	d := NewAIDecomposer(nil, testLogger())
	result := &AIDecomposeResult{
		Projects: []AIProject{
			{
				Name: "proj-1",
				Tasks: []AITask{
					{
						Name:        "task-1",
						Description: "",
					},
				},
			},
		},
	}
	err := d.validateResult(result)
	if err == nil {
		t.Fatal("expected error for empty task description")
	}
	if !strings.Contains(err.Error(), "empty description") {
		t.Errorf("expected 'empty description' error, got: %v", err)
	}
}

// --- TestAIDecomposer_ValidateResult_ProjectNoTasks ---

func TestAIDecomposer_ValidateResult_ProjectNoTasks(t *testing.T) {
	d := NewAIDecomposer(nil, testLogger())
	result := &AIDecomposeResult{
		Projects: []AIProject{
			{
				Name:  "proj-1",
				Tasks: []AITask{},
			},
		},
	}
	err := d.validateResult(result)
	if err == nil {
		t.Fatal("expected error for project with no tasks")
	}
	if !strings.Contains(err.Error(), "no tasks") {
		t.Errorf("expected 'no tasks' error, got: %v", err)
	}
}

// --- TestAIDecomposer_ValidateResult_ValidComplexities ---

func TestAIDecomposer_ValidateResult_ValidComplexities(t *testing.T) {
	d := NewAIDecomposer(nil, testLogger())

	for _, complexity := range []string{"low", "medium", "high", ""} {
		result := &AIDecomposeResult{
			Projects: []AIProject{
				{
					Name: "proj-1",
					Tasks: []AITask{
						{
							Name:        "task-1",
							Description: "do something",
							Complexity:  complexity,
						},
					},
				},
			},
		}
		err := d.validateResult(result)
		if err != nil {
			t.Errorf("unexpected error for valid complexity %q: %v", complexity, err)
		}
	}
}

// --- TestAIDecomposer_SetConstraints ---

func TestAIDecomposer_SetConstraints(t *testing.T) {
	ctrl := &mockAgentController{
		getAgentFunc: func(ctx context.Context, name string) (v1.AgentRecord, error) {
			return v1.AgentRecord{Name: name}, nil
		},
		executeFunc: func(ctx context.Context, task v1.TaskRecord) (ExecuteResult, error) {
			// Return 3 projects, which exceeds MaxProjects=2.
			return ExecuteResult{Output: `{
				"projects": [
					{"name": "p1", "tasks": [{"name": "t1", "description": "d1"}]},
					{"name": "p2", "tasks": [{"name": "t2", "description": "d2"}]},
					{"name": "p3", "tasks": [{"name": "t3", "description": "d3"}]}
				]
			}`}, nil
		},
	}

	d := NewAIDecomposer(ctrl, testLogger())
	d.SetConstraints(&v1.DecomposeConstraints{MaxProjects: 2})

	_, err := d.Decompose(context.Background(), DecomposeRequest{
		GoalID:      "g1",
		GoalName:    "Test",
		Description: "Test constraints",
	})
	if err == nil {
		t.Fatal("expected error when constraints are violated on all retries")
	}
}
