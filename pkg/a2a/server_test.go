package a2a

import (
	"context"
	"net"
	"testing"

	a2apb "github.com/zlc-ai/opc-platform/gen/a2a"
	opcpb "github.com/zlc-ai/opc-platform/gen/opc"
	"github.com/zlc-ai/opc-platform/pkg/adapter"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/test/bufconn"
)

func setupTestServer(t *testing.T, mock *mockAdapter) (opcpb.AgentServiceClient, func()) {
	t.Helper()

	bridge := NewBridge()
	bridge.RegisterAdapter("test-agent", mock)

	cards := map[string]*a2apb.AgentCard{
		"test-agent": {Name: "test-agent", Description: "test agent"},
	}

	srv := NewAgentServiceServer(bridge, cards)

	lis := bufconn.Listen(1024 * 1024)
	s := grpc.NewServer()
	opcpb.RegisterAgentServiceServer(s, srv)
	go func() { _ = s.Serve(lis) }()

	conn, err := grpc.NewClient("passthrough:///bufnet",
		grpc.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) {
			return lis.DialContext(ctx)
		}),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	client := opcpb.NewAgentServiceClient(conn)
	cleanup := func() {
		conn.Close()
		s.Stop()
	}
	return client, cleanup
}

func TestAgentServiceServer_SendTask(t *testing.T) {
	mock := &mockAdapter{
		executeResult: adapter.ExecuteResult{
			Output:    "result from agent",
			TokensIn:  200,
			TokensOut: 100,
			Cost:      0.10,
		},
		healthy: true,
	}

	client, cleanup := setupTestServer(t, mock)
	defer cleanup()

	resp, err := client.SendTask(context.Background(), &opcpb.SendTaskRequest{
		TaskId:    "task-1",
		AgentName: "test-agent",
		Message: &a2apb.Message{
			Role:  "user",
			Parts: []*a2apb.Part{{Part: &a2apb.Part_TextPart{TextPart: &a2apb.TextPart{Text: "do work"}}}},
		},
	})
	if err != nil {
		t.Fatalf("SendTask failed: %v", err)
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

	arts := task.GetArtifacts()
	if len(arts) == 0 {
		t.Fatal("expected at least one artifact")
	}
	tp := arts[0].GetParts()[0].GetTextPart()
	if tp == nil || tp.GetText() != "result from agent" {
		t.Errorf("expected artifact text 'result from agent', got %v", tp)
	}

	cost := resp.GetCost()
	if cost == nil {
		t.Fatal("cost report is nil")
	}
	if cost.GetTokensIn() != 200 {
		t.Errorf("expected tokensIn 200, got %d", cost.GetTokensIn())
	}
}

func TestAgentServiceServer_GetAgentCard(t *testing.T) {
	mock := &mockAdapter{healthy: true}
	client, cleanup := setupTestServer(t, mock)
	defer cleanup()

	card, err := client.GetAgentCard(context.Background(), &opcpb.GetAgentCardRequest{
		AgentName: "test-agent",
	})
	if err != nil {
		t.Fatalf("GetAgentCard failed: %v", err)
	}
	if card.GetName() != "test-agent" {
		t.Errorf("expected name 'test-agent', got %s", card.GetName())
	}
	if card.GetDescription() != "test agent" {
		t.Errorf("expected description 'test agent', got %s", card.GetDescription())
	}
}

func TestAgentServiceServer_GetAgentCard_NotFound(t *testing.T) {
	mock := &mockAdapter{healthy: true}
	client, cleanup := setupTestServer(t, mock)
	defer cleanup()

	_, err := client.GetAgentCard(context.Background(), &opcpb.GetAgentCardRequest{
		AgentName: "nonexistent",
	})
	if err == nil {
		t.Fatal("expected error for unknown agent card")
	}
}

func TestAgentServiceServer_Health(t *testing.T) {
	mock := &mockAdapter{healthy: true}
	client, cleanup := setupTestServer(t, mock)
	defer cleanup()

	resp, err := client.Health(context.Background(), &opcpb.HealthRequest{
		AgentName: "test-agent",
	})
	if err != nil {
		t.Fatalf("Health failed: %v", err)
	}
	if !resp.GetHealthy() {
		t.Error("expected healthy=true")
	}
	if resp.GetMessage() != "mock" {
		t.Errorf("expected message 'mock', got %s", resp.GetMessage())
	}
}
