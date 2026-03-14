package adapter_test

import (
	"context"
	"errors"
	"testing"

	v1 "github.com/zlc-ai/opc-platform/api/v1"
	"github.com/zlc-ai/opc-platform/pkg/adapter"
	"github.com/zlc-ai/opc-platform/pkg/adapter/claudecode"
	"github.com/zlc-ai/opc-platform/pkg/adapter/codex"
	"github.com/zlc-ai/opc-platform/pkg/adapter/custom"
)

// ---------------------------------------------------------------------------
// Registry tests
// ---------------------------------------------------------------------------

func TestRegistryCreate(t *testing.T) {
	reg := adapter.NewRegistry()
	reg.Register(v1.AgentTypeClaudeCode, func() adapter.Adapter { return claudecode.New() })

	a, err := reg.Create(v1.AgentTypeClaudeCode)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if a == nil {
		t.Fatal("expected non-nil adapter")
	}
	if got := a.Type(); got != v1.AgentTypeClaudeCode {
		t.Errorf("expected type %s, got %s", v1.AgentTypeClaudeCode, got)
	}
}

func TestRegistryUnsupportedType(t *testing.T) {
	reg := adapter.NewRegistry()

	_, err := reg.Create(v1.AgentType("unknown"))
	if err == nil {
		t.Fatal("expected error for unsupported type")
	}

	var unsupported *adapter.UnsupportedAgentTypeError
	if !errors.As(err, &unsupported) {
		t.Fatalf("expected UnsupportedAgentTypeError, got %T: %v", err, err)
	}
	if unsupported.Type != v1.AgentType("unknown") {
		t.Errorf("expected type 'unknown', got %s", unsupported.Type)
	}
}

func TestRegistryMultipleTypes(t *testing.T) {
	reg := adapter.NewRegistry()
	reg.Register(v1.AgentTypeClaudeCode, func() adapter.Adapter { return claudecode.New() })
	reg.Register(v1.AgentTypeCodex, func() adapter.Adapter { return codex.New() })
	reg.Register(v1.AgentTypeCustom, func() adapter.Adapter { return custom.New() })

	types := []v1.AgentType{v1.AgentTypeClaudeCode, v1.AgentTypeCodex, v1.AgentTypeCustom}
	for _, agentType := range types {
		a, err := reg.Create(agentType)
		if err != nil {
			t.Errorf("unexpected error for type %s: %v", agentType, err)
			continue
		}
		if a.Type() != agentType {
			t.Errorf("expected type %s, got %s", agentType, a.Type())
		}
	}
}

// ---------------------------------------------------------------------------
// Claude Code adapter tests
// ---------------------------------------------------------------------------

func TestClaudeCodeAdapterType(t *testing.T) {
	a := claudecode.New()
	if got := a.Type(); got != v1.AgentTypeClaudeCode {
		t.Errorf("expected type %s, got %s", v1.AgentTypeClaudeCode, got)
	}
}

func TestClaudeCodeAdapterLifecycle(t *testing.T) {
	a := claudecode.New()

	// Initial status should be Created.
	if got := a.Status(); got != v1.AgentPhaseCreated {
		t.Errorf("initial status: expected %s, got %s", v1.AgentPhaseCreated, got)
	}

	// Health should report unhealthy before start.
	health := a.Health()
	if health.Healthy {
		t.Error("expected unhealthy before start")
	}

	// Start should set phase to Running.
	ctx := context.Background()
	spec := v1.AgentSpec{
		Spec: v1.AgentSpecBody{
			Runtime: v1.RuntimeConfig{
				Model: v1.ModelConfig{Name: "claude-sonnet-4-20250514"},
			},
		},
	}
	if err := a.Start(ctx, spec); err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	if got := a.Status(); got != v1.AgentPhaseRunning {
		t.Errorf("after Start: expected %s, got %s", v1.AgentPhaseRunning, got)
	}

	// Health should report healthy after start.
	health = a.Health()
	if !health.Healthy {
		t.Error("expected healthy after start")
	}

	// Metrics should have uptime > 0 after start.
	metrics := a.Metrics()
	if metrics.UptimeSeconds <= 0 {
		t.Error("expected positive uptime after start")
	}

	// Stop should set phase to Stopped.
	if err := a.Stop(ctx); err != nil {
		t.Fatalf("Stop failed: %v", err)
	}
	if got := a.Status(); got != v1.AgentPhaseStopped {
		t.Errorf("after Stop: expected %s, got %s", v1.AgentPhaseStopped, got)
	}

	// Health should report unhealthy after stop.
	health = a.Health()
	if health.Healthy {
		t.Error("expected unhealthy after stop")
	}
}

// ---------------------------------------------------------------------------
// Codex adapter tests
// ---------------------------------------------------------------------------

func TestCodexAdapterType(t *testing.T) {
	a := codex.New()
	if got := a.Type(); got != v1.AgentTypeCodex {
		t.Errorf("expected type %s, got %s", v1.AgentTypeCodex, got)
	}
}

func TestCodexAdapterLifecycle(t *testing.T) {
	a := codex.New()

	// Initial status should be Created.
	if got := a.Status(); got != v1.AgentPhaseCreated {
		t.Errorf("initial status: expected %s, got %s", v1.AgentPhaseCreated, got)
	}

	// Health should report unhealthy before start.
	health := a.Health()
	if health.Healthy {
		t.Error("expected unhealthy before start")
	}

	// Start should set phase to Running.
	ctx := context.Background()
	spec := v1.AgentSpec{
		Spec: v1.AgentSpecBody{
			Runtime: v1.RuntimeConfig{
				Model: v1.ModelConfig{Name: "codex-mini"},
			},
		},
	}
	if err := a.Start(ctx, spec); err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	if got := a.Status(); got != v1.AgentPhaseRunning {
		t.Errorf("after Start: expected %s, got %s", v1.AgentPhaseRunning, got)
	}

	// Health should report healthy after start.
	health = a.Health()
	if !health.Healthy {
		t.Error("expected healthy after start")
	}

	// Metrics should have uptime > 0 after start.
	metrics := a.Metrics()
	if metrics.UptimeSeconds <= 0 {
		t.Error("expected positive uptime after start")
	}

	// Stop should set phase to Stopped.
	if err := a.Stop(ctx); err != nil {
		t.Fatalf("Stop failed: %v", err)
	}
	if got := a.Status(); got != v1.AgentPhaseStopped {
		t.Errorf("after Stop: expected %s, got %s", v1.AgentPhaseStopped, got)
	}

	// Health should report unhealthy after stop.
	health = a.Health()
	if health.Healthy {
		t.Error("expected unhealthy after stop")
	}
}

// ---------------------------------------------------------------------------
// Custom adapter tests
// ---------------------------------------------------------------------------

func TestCustomAdapterType(t *testing.T) {
	a := custom.New()
	if got := a.Type(); got != v1.AgentTypeCustom {
		t.Errorf("expected type %s, got %s", v1.AgentTypeCustom, got)
	}
}
