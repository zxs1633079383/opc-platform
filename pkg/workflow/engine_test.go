package workflow

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	v1 "github.com/zlc-ai/opc-platform/api/v1"
	"github.com/zlc-ai/opc-platform/pkg/adapter"
	"github.com/zlc-ai/opc-platform/pkg/controller"
	"github.com/zlc-ai/opc-platform/pkg/storage/sqlite"
	"go.uber.org/zap"
)

// --- helpers ---

func testLogger() *zap.SugaredLogger { return zap.NewNop().Sugar() }

type mockAdapterForWorkflow struct {
	mu          sync.Mutex
	executeFunc func(ctx context.Context, task v1.TaskRecord) (adapter.ExecuteResult, error)
}

func (m *mockAdapterForWorkflow) Type() v1.AgentType                 { return v1.AgentTypeClaudeCode }
func (m *mockAdapterForWorkflow) Start(_ context.Context, _ v1.AgentSpec) error { return nil }
func (m *mockAdapterForWorkflow) Stop(_ context.Context) error        { return nil }
func (m *mockAdapterForWorkflow) Health() v1.HealthStatus {
	return v1.HealthStatus{Healthy: true, Message: "ok"}
}
func (m *mockAdapterForWorkflow) Execute(ctx context.Context, task v1.TaskRecord) (adapter.ExecuteResult, error) {
	m.mu.Lock()
	fn := m.executeFunc
	m.mu.Unlock()
	if fn != nil {
		return fn(ctx, task)
	}
	return adapter.ExecuteResult{Output: "mock output for " + task.Message}, nil
}
func (m *mockAdapterForWorkflow) Stream(_ context.Context, _ v1.TaskRecord) (<-chan adapter.Chunk, error) {
	ch := make(chan adapter.Chunk, 1)
	ch <- adapter.Chunk{Content: "done", Done: true}
	close(ch)
	return ch, nil
}
func (m *mockAdapterForWorkflow) Status() v1.AgentPhase   { return v1.AgentPhaseRunning }
func (m *mockAdapterForWorkflow) Metrics() v1.AgentMetrics { return v1.AgentMetrics{} }

func newTestEngine(t *testing.T, agentNames []string, execFunc func(ctx context.Context, task v1.TaskRecord) (adapter.ExecuteResult, error)) (*Engine, *controller.Controller) {
	t.Helper()
	dir := t.TempDir()
	store, err := sqlite.New(dir + "/test.db")
	if err != nil {
		t.Fatalf("new store: %v", err)
	}
	t.Cleanup(func() { store.Close() })

	mock := &mockAdapterForWorkflow{executeFunc: execFunc}
	reg := adapter.NewRegistry()
	reg.Register(v1.AgentTypeClaudeCode, func() adapter.Adapter { return mock })

	ctrl := controller.New(store, reg, testLogger())

	ctx := context.Background()
	for _, name := range agentNames {
		spec := v1.AgentSpec{
			APIVersion: v1.APIVersion,
			Kind:       v1.KindAgentSpec,
			Metadata:   v1.Metadata{Name: name},
			Spec:       v1.AgentSpecBody{Type: v1.AgentTypeClaudeCode},
		}
		if err := ctrl.Apply(ctx, spec); err != nil {
			t.Fatalf("apply %s: %v", name, err)
		}
		if err := ctrl.StartAgent(ctx, name); err != nil {
			t.Fatalf("start %s: %v", name, err)
		}
	}

	engine := NewEngine(ctrl, store, testLogger())
	return engine, ctrl
}

// --- ParseWorkflow tests ---

func TestParseWorkflow(t *testing.T) {
	t.Run("valid workflow", func(t *testing.T) {
		yaml := `
apiVersion: opc/v1
kind: Workflow
metadata:
  name: test-wf
spec:
  schedule: "0 7 * * *"
  steps:
    - name: step1
      agent: coder
      input:
        message: "do something"
`
		spec, err := ParseWorkflow([]byte(yaml))
		if err != nil {
			t.Fatalf("ParseWorkflow: %v", err)
		}
		if spec.Metadata.Name != "test-wf" {
			t.Errorf("name = %q, want test-wf", spec.Metadata.Name)
		}
		if len(spec.Spec.Steps) != 1 {
			t.Errorf("steps = %d, want 1", len(spec.Spec.Steps))
		}
		if spec.Spec.Schedule != "0 7 * * *" {
			t.Errorf("schedule = %q, want '0 7 * * *'", spec.Spec.Schedule)
		}
	})

	t.Run("missing apiVersion", func(t *testing.T) {
		yaml := `kind: Workflow
metadata:
  name: x
spec:
  steps:
    - name: s1
      agent: a
      input:
        message: m`
		_, err := ParseWorkflow([]byte(yaml))
		if err == nil {
			t.Fatal("expected error for missing apiVersion")
		}
	})

	t.Run("wrong kind", func(t *testing.T) {
		yaml := `apiVersion: opc/v1
kind: AgentSpec
metadata:
  name: x
spec:
  steps:
    - name: s1
      agent: a
      input:
        message: m`
		_, err := ParseWorkflow([]byte(yaml))
		if err == nil || !strings.Contains(err.Error(), "expected kind") {
			t.Fatalf("expected kind error, got: %v", err)
		}
	})

	t.Run("empty steps", func(t *testing.T) {
		yaml := `apiVersion: opc/v1
kind: Workflow
metadata:
  name: x
spec:
  steps: []`
		_, err := ParseWorkflow([]byte(yaml))
		if err == nil || !strings.Contains(err.Error(), "at least one step") {
			t.Fatalf("expected step error, got: %v", err)
		}
	})

	t.Run("missing metadata name", func(t *testing.T) {
		yaml := `apiVersion: opc/v1
kind: Workflow
metadata:
  name: ""
spec:
  steps:
    - name: s
      agent: a
      input:
        message: m`
		_, err := ParseWorkflow([]byte(yaml))
		if err == nil {
			t.Fatal("expected error for empty metadata.name")
		}
	})
}

// --- validateDAG tests ---

func TestValidateDAG(t *testing.T) {
	e := &Engine{logger: testLogger()}

	t.Run("valid linear DAG", func(t *testing.T) {
		steps := []WorkflowStep{
			{Name: "a", Agent: "x"},
			{Name: "b", Agent: "x", DependsOn: []string{"a"}},
			{Name: "c", Agent: "x", DependsOn: []string{"b"}},
		}
		if err := e.validateDAG(steps); err != nil {
			t.Errorf("expected no error: %v", err)
		}
	})

	t.Run("valid parallel DAG", func(t *testing.T) {
		steps := []WorkflowStep{
			{Name: "a", Agent: "x"},
			{Name: "b", Agent: "x"},
			{Name: "c", Agent: "x", DependsOn: []string{"a", "b"}},
		}
		if err := e.validateDAG(steps); err != nil {
			t.Errorf("expected no error: %v", err)
		}
	})

	t.Run("duplicate step name", func(t *testing.T) {
		steps := []WorkflowStep{
			{Name: "a", Agent: "x"},
			{Name: "a", Agent: "x"},
		}
		err := e.validateDAG(steps)
		if err == nil || !strings.Contains(err.Error(), "duplicate") {
			t.Fatalf("expected duplicate error, got: %v", err)
		}
	})

	t.Run("self dependency", func(t *testing.T) {
		steps := []WorkflowStep{
			{Name: "a", Agent: "x", DependsOn: []string{"a"}},
		}
		err := e.validateDAG(steps)
		if err == nil || !strings.Contains(err.Error(), "depends on itself") {
			t.Fatalf("expected self-dependency error, got: %v", err)
		}
	})

	t.Run("unknown dependency", func(t *testing.T) {
		steps := []WorkflowStep{
			{Name: "a", Agent: "x", DependsOn: []string{"nonexistent"}},
		}
		err := e.validateDAG(steps)
		if err == nil || !strings.Contains(err.Error(), "unknown step") {
			t.Fatalf("expected unknown step error, got: %v", err)
		}
	})

	t.Run("cycle detection", func(t *testing.T) {
		steps := []WorkflowStep{
			{Name: "a", Agent: "x", DependsOn: []string{"c"}},
			{Name: "b", Agent: "x", DependsOn: []string{"a"}},
			{Name: "c", Agent: "x", DependsOn: []string{"b"}},
		}
		err := e.validateDAG(steps)
		if err == nil || !strings.Contains(err.Error(), "cycle") {
			t.Fatalf("expected cycle error, got: %v", err)
		}
	})

	t.Run("empty step name", func(t *testing.T) {
		steps := []WorkflowStep{
			{Name: "", Agent: "x"},
		}
		err := e.validateDAG(steps)
		if err == nil || !strings.Contains(err.Error(), "name is required") {
			t.Fatalf("expected name error, got: %v", err)
		}
	})

	t.Run("empty agent", func(t *testing.T) {
		steps := []WorkflowStep{
			{Name: "a", Agent: ""},
		}
		err := e.validateDAG(steps)
		if err == nil || !strings.Contains(err.Error(), "agent is required") {
			t.Fatalf("expected agent error, got: %v", err)
		}
	})
}

// --- buildDAG tests ---

func TestBuildDAG(t *testing.T) {
	e := &Engine{logger: testLogger()}

	t.Run("single step", func(t *testing.T) {
		steps := []WorkflowStep{{Name: "a", Agent: "x"}}
		layers, err := e.buildDAG(steps)
		if err != nil {
			t.Fatalf("buildDAG: %v", err)
		}
		if len(layers) != 1 {
			t.Fatalf("layers = %d, want 1", len(layers))
		}
		if len(layers[0]) != 1 || layers[0][0].Name != "a" {
			t.Error("unexpected layer content")
		}
	})

	t.Run("parallel steps", func(t *testing.T) {
		steps := []WorkflowStep{
			{Name: "a", Agent: "x"},
			{Name: "b", Agent: "x"},
			{Name: "c", Agent: "x"},
		}
		layers, err := e.buildDAG(steps)
		if err != nil {
			t.Fatalf("buildDAG: %v", err)
		}
		if len(layers) != 1 {
			t.Fatalf("layers = %d, want 1 (all parallel)", len(layers))
		}
		if len(layers[0]) != 3 {
			t.Errorf("layer[0] has %d steps, want 3", len(layers[0]))
		}
	})

	t.Run("linear chain", func(t *testing.T) {
		steps := []WorkflowStep{
			{Name: "a", Agent: "x"},
			{Name: "b", Agent: "x", DependsOn: []string{"a"}},
			{Name: "c", Agent: "x", DependsOn: []string{"b"}},
		}
		layers, err := e.buildDAG(steps)
		if err != nil {
			t.Fatalf("buildDAG: %v", err)
		}
		if len(layers) != 3 {
			t.Fatalf("layers = %d, want 3", len(layers))
		}
	})

	t.Run("diamond shape", func(t *testing.T) {
		// a → b, a → c, b+c → d
		steps := []WorkflowStep{
			{Name: "a", Agent: "x"},
			{Name: "b", Agent: "x", DependsOn: []string{"a"}},
			{Name: "c", Agent: "x", DependsOn: []string{"a"}},
			{Name: "d", Agent: "x", DependsOn: []string{"b", "c"}},
		}
		layers, err := e.buildDAG(steps)
		if err != nil {
			t.Fatalf("buildDAG: %v", err)
		}
		if len(layers) != 3 {
			t.Fatalf("layers = %d, want 3 (a → b,c → d)", len(layers))
		}
		// Layer 0: a, Layer 1: b+c, Layer 2: d
		if len(layers[1]) != 2 {
			t.Errorf("layer[1] has %d steps, want 2 (b and c)", len(layers[1]))
		}
	})
}

// --- substituteVars tests ---

func TestSubstituteVars(t *testing.T) {
	outputs := map[string]string{
		"research": "AI Agent findings",
		"analyze":  "Key insights report",
	}

	tests := []struct {
		name   string
		input  string
		want   string
	}{
		{"single var", "Review: ${{ steps.research.outputs.findings }}", "Review: AI Agent findings"},
		{"multiple vars", "${{ steps.research.outputs.x }} and ${{ steps.analyze.outputs.y }}", "AI Agent findings and Key insights report"},
		{"no vars", "plain message", "plain message"},
		{"unresolved var", "${{ steps.unknown.outputs.x }}", "${{ steps.unknown.outputs.x }}"},
		{"mixed resolved and unresolved", "${{ steps.research.outputs.x }} ${{ steps.nope.outputs.y }}", "AI Agent findings ${{ steps.nope.outputs.y }}"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := substituteVars(tt.input, outputs)
			if got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

// --- resolveContext tests ---

func TestResolveContext(t *testing.T) {
	e := &Engine{logger: testLogger()}
	outputs := map[string]string{"step1": "output1"}

	t.Run("message only", func(t *testing.T) {
		input := WorkflowInput{Message: "hello"}
		got := e.resolveContext(input, outputs)
		if got != "hello" {
			t.Errorf("got %q, want hello", got)
		}
	})

	t.Run("message with context", func(t *testing.T) {
		input := WorkflowInput{
			Message: "Analyze this",
			Context: []string{"${{ steps.step1.outputs.data }}"},
		}
		got := e.resolveContext(input, outputs)
		if !strings.Contains(got, "Analyze this") || !strings.Contains(got, "output1") {
			t.Errorf("got %q, expected both message and context output", got)
		}
	})

	t.Run("empty context strings filtered", func(t *testing.T) {
		input := WorkflowInput{
			Message: "msg",
			Context: []string{""},
		}
		got := e.resolveContext(input, outputs)
		if got != "msg" {
			t.Errorf("got %q, want just 'msg'", got)
		}
	})
}

// --- Execute integration tests ---

func TestExecute_SingleStep(t *testing.T) {
	engine, _ := newTestEngine(t, []string{"coder"}, nil)

	spec := WorkflowSpec{
		APIVersion: v1.APIVersion,
		Kind:       v1.KindWorkflow,
		Metadata:   v1.Metadata{Name: "single-step"},
		Spec: WorkflowBody{
			Steps: []WorkflowStep{
				{Name: "build", Agent: "coder", Input: WorkflowInput{Message: "build the app"}},
			},
		},
	}

	run, err := engine.Execute(context.Background(), spec)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if run.Status != WorkflowStatusCompleted {
		t.Errorf("status = %q, want Completed", run.Status)
	}
	if run.Steps["build"].Status != "Completed" {
		t.Errorf("step status = %q, want Completed", run.Steps["build"].Status)
	}
}

func TestExecute_MultiLayer(t *testing.T) {
	engine, _ := newTestEngine(t, []string{"agent-a"}, nil)

	spec := WorkflowSpec{
		APIVersion: v1.APIVersion,
		Kind:       v1.KindWorkflow,
		Metadata:   v1.Metadata{Name: "multi-layer"},
		Spec: WorkflowBody{
			Steps: []WorkflowStep{
				{Name: "research", Agent: "agent-a", Input: WorkflowInput{Message: "research"}},
				{Name: "analyze", Agent: "agent-a", Input: WorkflowInput{Message: "analyze"}, DependsOn: []string{"research"}},
				{Name: "report", Agent: "agent-a", Input: WorkflowInput{Message: "report"}, DependsOn: []string{"analyze"}},
			},
		},
	}

	run, err := engine.Execute(context.Background(), spec)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if run.Status != WorkflowStatusCompleted {
		t.Errorf("status = %q, want Completed", run.Status)
	}
	for _, name := range []string{"research", "analyze", "report"} {
		if run.Steps[name].Status != "Completed" {
			t.Errorf("step %q status = %q, want Completed", name, run.Steps[name].Status)
		}
	}
}

func TestExecute_StepFailure_SkipsRemaining(t *testing.T) {
	failOnAnalyze := func(ctx context.Context, task v1.TaskRecord) (adapter.ExecuteResult, error) {
		if strings.Contains(task.Message, "analyze") {
			return adapter.ExecuteResult{}, fmt.Errorf("analysis failed")
		}
		return adapter.ExecuteResult{Output: "ok"}, nil
	}

	engine, _ := newTestEngine(t, []string{"agent-a"}, failOnAnalyze)

	spec := WorkflowSpec{
		APIVersion: v1.APIVersion,
		Kind:       v1.KindWorkflow,
		Metadata:   v1.Metadata{Name: "fail-test"},
		Spec: WorkflowBody{
			Steps: []WorkflowStep{
				{Name: "research", Agent: "agent-a", Input: WorkflowInput{Message: "research"}},
				{Name: "analyze", Agent: "agent-a", Input: WorkflowInput{Message: "analyze"}, DependsOn: []string{"research"}},
				{Name: "report", Agent: "agent-a", Input: WorkflowInput{Message: "report"}, DependsOn: []string{"analyze"}},
			},
		},
	}

	run, err := engine.Execute(context.Background(), spec)
	if err == nil {
		t.Fatal("expected error from failed step")
	}
	if run.Status != WorkflowStatusFailed {
		t.Errorf("status = %q, want Failed", run.Status)
	}
	if run.Steps["research"].Status != "Completed" {
		t.Errorf("research should be Completed, got %q", run.Steps["research"].Status)
	}
	if run.Steps["analyze"].Status != "Failed" {
		t.Errorf("analyze should be Failed, got %q", run.Steps["analyze"].Status)
	}
	if run.Steps["report"].Status != "Skipped" {
		t.Errorf("report should be Skipped, got %q", run.Steps["report"].Status)
	}
}

func TestExecute_ContextPassing(t *testing.T) {
	var receivedMessage string
	captureFunc := func(ctx context.Context, task v1.TaskRecord) (adapter.ExecuteResult, error) {
		if strings.Contains(task.Message, "Analyze") {
			receivedMessage = task.Message
		}
		return adapter.ExecuteResult{Output: "research findings here"}, nil
	}

	engine, _ := newTestEngine(t, []string{"agent-a"}, captureFunc)

	spec := WorkflowSpec{
		APIVersion: v1.APIVersion,
		Kind:       v1.KindWorkflow,
		Metadata:   v1.Metadata{Name: "ctx-test"},
		Spec: WorkflowBody{
			Steps: []WorkflowStep{
				{
					Name:  "research",
					Agent: "agent-a",
					Input: WorkflowInput{Message: "do research"},
					Outputs: []WorkflowOutput{{Name: "findings"}},
				},
				{
					Name:      "analyze",
					Agent:     "agent-a",
					DependsOn: []string{"research"},
					Input: WorkflowInput{
						Message: "Analyze this",
						Context: []string{"${{ steps.research.outputs.findings }}"},
					},
				},
			},
		},
	}

	_, err := engine.Execute(context.Background(), spec)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}

	if !strings.Contains(receivedMessage, "research findings here") {
		t.Errorf("context not passed; got message %q", receivedMessage)
	}
}

func TestExecute_ParallelSteps(t *testing.T) {
	var mu sync.Mutex
	executionOrder := make([]string, 0)

	trackFunc := func(ctx context.Context, task v1.TaskRecord) (adapter.ExecuteResult, error) {
		mu.Lock()
		executionOrder = append(executionOrder, task.Message)
		mu.Unlock()
		time.Sleep(10 * time.Millisecond) // simulate work
		return adapter.ExecuteResult{Output: task.Message + " done"}, nil
	}

	engine, _ := newTestEngine(t, []string{"agent-a"}, trackFunc)

	spec := WorkflowSpec{
		APIVersion: v1.APIVersion,
		Kind:       v1.KindWorkflow,
		Metadata:   v1.Metadata{Name: "parallel-test"},
		Spec: WorkflowBody{
			Steps: []WorkflowStep{
				{Name: "a", Agent: "agent-a", Input: WorkflowInput{Message: "task-a"}},
				{Name: "b", Agent: "agent-a", Input: WorkflowInput{Message: "task-b"}},
				{Name: "c", Agent: "agent-a", Input: WorkflowInput{Message: "task-c"}, DependsOn: []string{"a", "b"}},
			},
		},
	}

	run, err := engine.Execute(context.Background(), spec)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if run.Status != WorkflowStatusCompleted {
		t.Errorf("status = %q, want Completed", run.Status)
	}

	// c should be last (depends on a and b)
	mu.Lock()
	if len(executionOrder) != 3 {
		t.Errorf("expected 3 executions, got %d", len(executionOrder))
	}
	if executionOrder[len(executionOrder)-1] != "task-c" {
		t.Errorf("task-c should execute last, got order: %v", executionOrder)
	}
	mu.Unlock()
}

// --- helper function tests ---

func TestStepNames(t *testing.T) {
	steps := []WorkflowStep{
		{Name: "a"},
		{Name: "b"},
		{Name: "c"},
	}
	got := stepNames(steps)
	if len(got) != 3 || got[0] != "a" || got[1] != "b" || got[2] != "c" {
		t.Errorf("stepNames = %v, want [a b c]", got)
	}
}

func TestGenerateRunID(t *testing.T) {
	id := generateRunID("test-wf")
	if !strings.HasPrefix(id, "wfr-test-wf-") {
		t.Errorf("generateRunID = %q, want prefix wfr-test-wf-", id)
	}
}

func TestGenerateTaskID(t *testing.T) {
	id := generateTaskID("step1")
	if !strings.HasPrefix(id, "wft-step1-") {
		t.Errorf("generateTaskID = %q, want prefix wft-step1-", id)
	}
}
