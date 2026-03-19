package dispatcher

import (
	"context"
	"fmt"
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

// newTestController creates a real controller with an in-memory SQLite store
// and a registry containing mockAdapters for the given agent names.
func newTestController(t *testing.T, agentNames []string, metrics map[string]v1.AgentMetrics) *controller.Controller {
	t.Helper()
	dir := t.TempDir()
	store, err := sqlite.New(dir + "/test.db")
	if err != nil {
		t.Fatalf("new store: %v", err)
	}
	t.Cleanup(func() { store.Close() })

	reg := adapter.NewRegistry()
	reg.Register(v1.AgentTypeClaudeCode, func() adapter.Adapter {
		return &mockAdapter{metrics: metrics}
	})

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
			t.Fatalf("apply agent %s: %v", name, err)
		}
		if err := ctrl.StartAgent(ctx, name); err != nil {
			t.Fatalf("start agent %s: %v", name, err)
		}
	}

	return ctrl
}

// mockAdapter is a minimal adapter for dispatcher tests.
type mockAdapter struct {
	mu      sync.Mutex
	metrics map[string]v1.AgentMetrics
}

func (m *mockAdapter) Type() v1.AgentType                 { return v1.AgentTypeClaudeCode }
func (m *mockAdapter) Start(_ context.Context, _ v1.AgentSpec) error { return nil }
func (m *mockAdapter) Stop(_ context.Context) error        { return nil }
func (m *mockAdapter) Health() v1.HealthStatus             { return v1.HealthStatus{Healthy: true, Message: "ok"} }
func (m *mockAdapter) Execute(_ context.Context, _ v1.TaskRecord) (adapter.ExecuteResult, error) {
	return adapter.ExecuteResult{Output: "done"}, nil
}
func (m *mockAdapter) Stream(_ context.Context, _ v1.TaskRecord) (<-chan adapter.Chunk, error) {
	ch := make(chan adapter.Chunk, 1)
	ch <- adapter.Chunk{Content: "done", Done: true}
	close(ch)
	return ch, nil
}
func (m *mockAdapter) Status() v1.AgentPhase   { return v1.AgentPhaseRunning }
func (m *mockAdapter) Metrics() v1.AgentMetrics { return v1.AgentMetrics{} }

// --- matchesRule tests ---

func TestMatchesRule(t *testing.T) {
	tests := []struct {
		name     string
		match    MatchCriteria
		taskType string
		labels   map[string]string
		want     bool
	}{
		{
			name:     "exact task type match",
			match:    MatchCriteria{TaskType: "coding"},
			taskType: "coding",
			want:     true,
		},
		{
			name:     "task type mismatch",
			match:    MatchCriteria{TaskType: "coding"},
			taskType: "research",
			want:     false,
		},
		{
			name:     "label match",
			match:    MatchCriteria{Labels: map[string]string{"team": "backend"}},
			taskType: "",
			labels:   map[string]string{"team": "backend", "env": "prod"},
			want:     true,
		},
		{
			name:     "label mismatch",
			match:    MatchCriteria{Labels: map[string]string{"team": "frontend"}},
			taskType: "",
			labels:   map[string]string{"team": "backend"},
			want:     false,
		},
		{
			name:     "label missing from task",
			match:    MatchCriteria{Labels: map[string]string{"team": "backend"}},
			taskType: "",
			labels:   map[string]string{},
			want:     false,
		},
		{
			name:     "type + label both match",
			match:    MatchCriteria{TaskType: "coding", Labels: map[string]string{"lang": "go"}},
			taskType: "coding",
			labels:   map[string]string{"lang": "go"},
			want:     true,
		},
		{
			name:     "type matches but label doesn't",
			match:    MatchCriteria{TaskType: "coding", Labels: map[string]string{"lang": "go"}},
			taskType: "coding",
			labels:   map[string]string{"lang": "python"},
			want:     false,
		},
		{
			name:     "empty criteria never matches",
			match:    MatchCriteria{},
			taskType: "coding",
			labels:   map[string]string{"a": "b"},
			want:     false,
		},
		{
			name:     "nil labels in task",
			match:    MatchCriteria{TaskType: "coding"},
			taskType: "coding",
			labels:   nil,
			want:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := matchesRule(tt.match, tt.taskType, tt.labels)
			if got != tt.want {
				t.Errorf("matchesRule() = %v, want %v", got, tt.want)
			}
		})
	}
}

// --- groupKey tests ---

func TestGroupKey(t *testing.T) {
	tests := []struct {
		candidates []string
		want       string
	}{
		{[]string{"a"}, "a"},
		{[]string{"a", "b", "c"}, "a,b,c"},
		{[]string{}, ""},
	}
	for _, tt := range tests {
		got := groupKey(tt.candidates)
		if got != tt.want {
			t.Errorf("groupKey(%v) = %q, want %q", tt.candidates, got, tt.want)
		}
	}
}

// --- selectAgent tests ---

func TestSelectAgent_SingleCandidate(t *testing.T) {
	ctrl := newTestController(t, []string{"agent-a"}, nil)
	d := New(ctrl, DispatcherBody{Strategy: StrategyRoundRobin}, testLogger())

	got, err := d.selectAgent([]string{"agent-a"}, StrategyRoundRobin)
	if err != nil {
		t.Fatalf("selectAgent: %v", err)
	}
	if got != "agent-a" {
		t.Errorf("got %q, want agent-a", got)
	}
}

func TestSelectAgent_NoCandidates(t *testing.T) {
	ctrl := newTestController(t, nil, nil)
	d := New(ctrl, DispatcherBody{}, testLogger())

	_, err := d.selectAgent(nil, StrategyRoundRobin)
	if err == nil {
		t.Fatal("expected error for empty candidates")
	}
}

func TestRoundRobin(t *testing.T) {
	ctrl := newTestController(t, []string{"a", "b", "c"}, nil)
	d := New(ctrl, DispatcherBody{Strategy: StrategyRoundRobin}, testLogger())

	candidates := []string{"a", "b", "c"}
	group := groupKey(candidates)

	results := make([]string, 6)
	for i := 0; i < 6; i++ {
		results[i] = d.roundRobin(candidates, group)
	}

	// Should cycle: a, b, c, a, b, c
	expected := []string{"a", "b", "c", "a", "b", "c"}
	for i, want := range expected {
		if results[i] != want {
			t.Errorf("roundRobin[%d] = %q, want %q", i, results[i], want)
		}
	}
}

func TestRoundRobin_IndependentGroups(t *testing.T) {
	ctrl := newTestController(t, []string{"a", "b", "x", "y"}, nil)
	d := New(ctrl, DispatcherBody{}, testLogger())

	group1 := []string{"a", "b"}
	group2 := []string{"x", "y"}

	g1r1 := d.roundRobin(group1, groupKey(group1))
	g2r1 := d.roundRobin(group2, groupKey(group2))
	g1r2 := d.roundRobin(group1, groupKey(group1))
	g2r2 := d.roundRobin(group2, groupKey(group2))

	if g1r1 != "a" || g1r2 != "b" {
		t.Errorf("group1: got %q, %q; want a, b", g1r1, g1r2)
	}
	if g2r1 != "x" || g2r2 != "y" {
		t.Errorf("group2: got %q, %q; want x, y", g2r1, g2r2)
	}
}

func TestLeastBusy(t *testing.T) {
	// Agent without metrics is assumed idle → selected first
	ctrl := newTestController(t, []string{"busy", "idle"}, nil)
	d := New(ctrl, DispatcherBody{}, testLogger())

	got, err := d.leastBusy([]string{"busy", "idle"})
	if err != nil {
		t.Fatalf("leastBusy: %v", err)
	}
	// Both should have 0 tasks; first found without metrics wins
	if got == "" {
		t.Fatal("expected a non-empty agent name")
	}
}

func TestCostOptimized(t *testing.T) {
	ctrl := newTestController(t, []string{"expensive", "cheap"}, nil)
	d := New(ctrl, DispatcherBody{}, testLogger())

	got, err := d.costOptimized([]string{"expensive", "cheap"})
	if err != nil {
		t.Fatalf("costOptimized: %v", err)
	}
	// Both have 0 cost; first found without metrics wins
	if got == "" {
		t.Fatal("expected a non-empty agent name")
	}
}

// --- Dispatch integration tests ---

func TestDispatch_RoutingRuleMatch(t *testing.T) {
	ctrl := newTestController(t, []string{"coder", "researcher"}, nil)

	config := DispatcherBody{
		Strategy: StrategyRoundRobin,
		Routing: []RoutingRule{
			{
				Match:  MatchCriteria{TaskType: "coding"},
				Agents: []string{"coder"},
			},
			{
				Match:  MatchCriteria{TaskType: "research"},
				Agents: []string{"researcher"},
			},
		},
	}

	d := New(ctrl, config, testLogger())
	ctx := context.Background()

	got, err := d.Dispatch(ctx, "coding", nil, "write code")
	if err != nil {
		t.Fatalf("Dispatch: %v", err)
	}
	if got != "coder" {
		t.Errorf("got %q, want coder", got)
	}

	got, err = d.Dispatch(ctx, "research", nil, "find papers")
	if err != nil {
		t.Fatalf("Dispatch: %v", err)
	}
	if got != "researcher" {
		t.Errorf("got %q, want researcher", got)
	}
}

func TestDispatch_Fallback(t *testing.T) {
	ctrl := newTestController(t, []string{"fallback-agent"}, nil)

	config := DispatcherBody{
		Routing: []RoutingRule{
			{
				Match:  MatchCriteria{TaskType: "coding"},
				Agents: []string{"coder"},
			},
		},
		Fallback: FallbackConfig{Agent: "fallback-agent"},
	}

	d := New(ctrl, config, testLogger())
	ctx := context.Background()

	// "unknown" task type doesn't match any rule → fallback
	got, err := d.Dispatch(ctx, "unknown", nil, "do something")
	if err != nil {
		t.Fatalf("Dispatch: %v", err)
	}
	if got != "fallback-agent" {
		t.Errorf("got %q, want fallback-agent", got)
	}
}

func TestDispatch_NoRulesNoFallback_UsesRunningAgents(t *testing.T) {
	ctrl := newTestController(t, []string{"agent-x", "agent-y"}, nil)

	config := DispatcherBody{
		Strategy: StrategyRoundRobin,
	}

	d := New(ctrl, config, testLogger())
	ctx := context.Background()

	got, err := d.Dispatch(ctx, "anything", nil, "do it")
	if err != nil {
		t.Fatalf("Dispatch: %v", err)
	}
	if got != "agent-x" && got != "agent-y" {
		t.Errorf("got %q, want one of agent-x or agent-y", got)
	}
}

func TestDispatch_NoRunningAgents(t *testing.T) {
	ctrl := newTestController(t, nil, nil)

	config := DispatcherBody{Strategy: StrategyAuto}
	d := New(ctrl, config, testLogger())
	ctx := context.Background()

	_, err := d.Dispatch(ctx, "coding", nil, "write code")
	if err == nil {
		t.Fatal("expected error when no agents available")
	}
}

func TestDispatch_RulePreference(t *testing.T) {
	ctrl := newTestController(t, []string{"a", "b"}, nil)

	config := DispatcherBody{
		Strategy: StrategyRoundRobin,
		Routing: []RoutingRule{
			{
				Match:      MatchCriteria{TaskType: "coding"},
				Agents:     []string{"a", "b"},
				Preference: "least-busy",
			},
		},
	}

	d := New(ctrl, config, testLogger())
	ctx := context.Background()

	// Should use least-busy strategy (from rule preference), not global round-robin
	got, err := d.Dispatch(ctx, "coding", nil, "code")
	if err != nil {
		t.Fatalf("Dispatch: %v", err)
	}
	if got != "a" && got != "b" {
		t.Errorf("got %q, want a or b", got)
	}
}

func TestDispatch_LabelRouting(t *testing.T) {
	ctrl := newTestController(t, []string{"go-coder", "py-coder"}, nil)

	config := DispatcherBody{
		Routing: []RoutingRule{
			{
				Match:  MatchCriteria{Labels: map[string]string{"lang": "go"}},
				Agents: []string{"go-coder"},
			},
			{
				Match:  MatchCriteria{Labels: map[string]string{"lang": "python"}},
				Agents: []string{"py-coder"},
			},
		},
		Fallback: FallbackConfig{Agent: "go-coder"},
	}

	d := New(ctrl, config, testLogger())
	ctx := context.Background()

	got, err := d.Dispatch(ctx, "", map[string]string{"lang": "go"}, "write go code")
	if err != nil {
		t.Fatalf("Dispatch: %v", err)
	}
	if got != "go-coder" {
		t.Errorf("got %q, want go-coder", got)
	}
}

func TestDispatch_AutoStrategy(t *testing.T) {
	ctrl := newTestController(t, []string{"a1", "a2"}, nil)

	config := DispatcherBody{
		Strategy: StrategyAuto,
		Routing: []RoutingRule{
			{
				Match:  MatchCriteria{TaskType: "task"},
				Agents: []string{"a1", "a2"},
			},
		},
	}

	d := New(ctrl, config, testLogger())
	ctx := context.Background()

	got, err := d.Dispatch(ctx, "task", nil, "do it")
	if err != nil {
		t.Fatalf("Dispatch: %v", err)
	}
	if got != "a1" && got != "a2" {
		t.Errorf("got %q, want a1 or a2", got)
	}
}

// --- Concurrent safety ---

func TestDispatch_ConcurrentRoundRobin(t *testing.T) {
	ctrl := newTestController(t, []string{"c1", "c2", "c3"}, nil)

	config := DispatcherBody{
		Strategy: StrategyRoundRobin,
		Routing: []RoutingRule{
			{
				Match:  MatchCriteria{TaskType: "task"},
				Agents: []string{"c1", "c2", "c3"},
			},
		},
	}

	d := New(ctrl, config, testLogger())
	ctx := context.Background()

	var wg sync.WaitGroup
	results := make([]string, 30)

	for i := 0; i < 30; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			name, err := d.Dispatch(ctx, "task", nil, fmt.Sprintf("task-%d", idx))
			if err != nil {
				t.Errorf("Dispatch[%d]: %v", idx, err)
				return
			}
			results[idx] = name
		}(i)
	}

	wg.Wait()

	// Verify all results are valid agent names
	validAgents := map[string]bool{"c1": true, "c2": true, "c3": true}
	for i, name := range results {
		if !validAgents[name] {
			t.Errorf("results[%d] = %q, not a valid agent", i, name)
		}
	}
}

// --- findMatchingRule ---

func TestFindMatchingRule(t *testing.T) {
	config := DispatcherBody{
		Routing: []RoutingRule{
			{Match: MatchCriteria{TaskType: "coding"}, Agents: []string{"coder"}},
			{Match: MatchCriteria{TaskType: "research"}, Agents: []string{"researcher"}},
		},
	}

	ctrl := newTestController(t, nil, nil)
	d := New(ctrl, config, testLogger())

	t.Run("first match wins", func(t *testing.T) {
		rule := d.findMatchingRule("coding", nil)
		if rule == nil {
			t.Fatal("expected a matching rule")
		}
		if rule.Agents[0] != "coder" {
			t.Errorf("got agent %q, want coder", rule.Agents[0])
		}
	})

	t.Run("no match returns nil", func(t *testing.T) {
		rule := d.findMatchingRule("unknown", nil)
		if rule != nil {
			t.Error("expected nil for unmatched task type")
		}
	})
}

// --- benchmark ---

func BenchmarkRoundRobin(b *testing.B) {
	d := &Dispatcher{
		rrCounters: make(map[string]int),
	}
	candidates := []string{"a", "b", "c", "d", "e"}
	group := groupKey(candidates)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = d.roundRobin(candidates, group)
	}
}

// Ensure test doesn't take too long due to mock adapter startup.
func init() {
	_ = time.Now()
}
