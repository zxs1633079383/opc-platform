package a2a

import (
	"context"
	"errors"
	"testing"

	a2apb "github.com/zlc-ai/opc-platform/gen/a2a"
	opcpb "github.com/zlc-ai/opc-platform/gen/opc"
	v1 "github.com/zlc-ai/opc-platform/api/v1"
	"github.com/zlc-ai/opc-platform/pkg/adapter"
)

// mockAdapter implements adapter.Adapter for testing.
type mockAdapter struct {
	executeResult adapter.ExecuteResult
	executeErr    error
	healthy       bool
}

func (m *mockAdapter) Type() v1.AgentType                                          { return v1.AgentTypeCustom }
func (m *mockAdapter) Start(_ context.Context, _ v1.AgentSpec) error               { return nil }
func (m *mockAdapter) Stop(_ context.Context) error                                { return nil }
func (m *mockAdapter) Health() v1.HealthStatus                                     { return v1.HealthStatus{Healthy: m.healthy, Message: "mock"} }
func (m *mockAdapter) Execute(_ context.Context, _ v1.TaskRecord) (adapter.ExecuteResult, error) {
	return m.executeResult, m.executeErr
}
func (m *mockAdapter) Stream(_ context.Context, _ v1.TaskRecord) (<-chan adapter.Chunk, error) {
	return nil, nil
}
func (m *mockAdapter) Status() v1.AgentPhase    { return v1.AgentPhaseRunning }
func (m *mockAdapter) Metrics() v1.AgentMetrics { return v1.AgentMetrics{} }

func TestBridgeSendTask(t *testing.T) {
	b := NewBridge()
	b.RegisterAdapter("agent-1", &mockAdapter{
		executeResult: adapter.ExecuteResult{
			Output:    "hello world",
			TokensIn:  100,
			TokensOut: 50,
			Cost:      0.05,
		},
	})

	req := &opcpb.SendTaskRequest{
		TaskId:    "task-1",
		AgentName: "agent-1",
		Message: &a2apb.Message{
			Role:  "user",
			Parts: []*a2apb.Part{{Part: &a2apb.Part_TextPart{TextPart: &a2apb.TextPart{Text: "do something"}}}},
		},
	}

	resp, err := b.SendTask(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp == nil {
		t.Fatal("response is nil")
	}

	task := resp.GetTask()
	if task == nil {
		t.Fatal("task is nil")
	}
	if task.GetStatus().GetState() != a2apb.TaskState_TASK_STATE_COMPLETED {
		t.Errorf("expected COMPLETED, got %v", task.GetStatus().GetState())
	}
	if task.GetId() != "task-1" {
		t.Errorf("expected task id task-1, got %s", task.GetId())
	}

	// Verify artifacts
	arts := task.GetArtifacts()
	if len(arts) == 0 {
		t.Fatal("expected at least one artifact")
	}
	tp := arts[0].GetParts()[0].GetTextPart()
	if tp == nil || tp.GetText() != "hello world" {
		t.Errorf("expected artifact text 'hello world', got %v", tp)
	}

	// Verify cost report
	cost := resp.GetCost()
	if cost == nil {
		t.Fatal("cost report is nil")
	}
	if cost.GetTokensIn() != 100 {
		t.Errorf("expected tokensIn 100, got %d", cost.GetTokensIn())
	}
	if cost.GetTokensOut() != 50 {
		t.Errorf("expected tokensOut 50, got %d", cost.GetTokensOut())
	}
	if cost.GetCostUsd() != 0.05 {
		t.Errorf("expected costUsd 0.05, got %f", cost.GetCostUsd())
	}
}

func TestBridgeSendTask_UnknownAgent(t *testing.T) {
	b := NewBridge()

	req := &opcpb.SendTaskRequest{
		TaskId:    "task-2",
		AgentName: "nonexistent",
		Message: &a2apb.Message{
			Role:  "user",
			Parts: []*a2apb.Part{{Part: &a2apb.Part_TextPart{TextPart: &a2apb.TextPart{Text: "test"}}}},
		},
	}

	_, err := b.SendTask(context.Background(), req)
	if err == nil {
		t.Fatal("expected error for unknown agent")
	}
}

func TestBridgeSendTask_AdapterError(t *testing.T) {
	b := NewBridge()
	b.RegisterAdapter("agent-fail", &mockAdapter{
		executeErr: errors.New("adapter exploded"),
	})

	req := &opcpb.SendTaskRequest{
		TaskId:    "task-3",
		AgentName: "agent-fail",
		Message: &a2apb.Message{
			Role:  "user",
			Parts: []*a2apb.Part{{Part: &a2apb.Part_TextPart{TextPart: &a2apb.TextPart{Text: "fail"}}}},
		},
	}

	resp, err := b.SendTask(context.Background(), req)
	if err != nil {
		t.Fatalf("should not return Go error, got: %v", err)
	}

	task := resp.GetTask()
	if task == nil {
		t.Fatal("task is nil")
	}
	if task.GetStatus().GetState() != a2apb.TaskState_TASK_STATE_FAILED {
		t.Errorf("expected FAILED, got %v", task.GetStatus().GetState())
	}
	if task.GetStatus().GetReason() != "adapter exploded" {
		t.Errorf("expected error reason 'adapter exploded', got %s", task.GetStatus().GetReason())
	}
}

func TestBridgeHealth(t *testing.T) {
	b := NewBridge()
	b.RegisterAdapter("healthy-agent", &mockAdapter{healthy: true})

	resp, err := b.Health("healthy-agent")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !resp.GetHealthy() {
		t.Error("expected healthy=true")
	}
	if resp.GetMessage() != "mock" {
		t.Errorf("expected message 'mock', got %s", resp.GetMessage())
	}

	// Unknown agent
	_, err = b.Health("unknown")
	if err == nil {
		t.Error("expected error for unknown agent")
	}
}
