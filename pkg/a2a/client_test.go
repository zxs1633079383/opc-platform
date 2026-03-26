package a2a

import (
	"context"
	"net"
	"testing"

	a2apb "github.com/zlc-ai/opc-platform/gen/a2a"
	opcpb "github.com/zlc-ai/opc-platform/gen/opc"
	v1 "github.com/zlc-ai/opc-platform/api/v1"
	"github.com/zlc-ai/opc-platform/pkg/adapter"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/test/bufconn"
)

func setupTestA2AClient(t *testing.T, mock *mockAdapter) (*A2AClient, func()) {
	t.Helper()

	bridge := NewBridge()
	bridge.RegisterAdapter("test-agent", mock)

	cards := map[string]*a2apb.AgentCard{
		"test-agent": {Name: "test-agent", Description: "test"},
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

	a2aClient := &A2AClient{
		conn:   conn,
		client: opcpb.NewAgentServiceClient(conn),
		agent:  "test-agent",
	}

	cleanup := func() {
		a2aClient.Close()
		s.Stop()
	}
	return a2aClient, cleanup
}

func TestA2AClientExecute(t *testing.T) {
	mock := &mockAdapter{
		executeResult: adapter.ExecuteResult{
			Output:    "hello from remote",
			TokensIn:  300,
			TokensOut: 150,
			Cost:      0.15,
		},
		healthy: true,
	}

	client, cleanup := setupTestA2AClient(t, mock)
	defer cleanup()

	task := v1.TaskRecord{
		ID:        "task-rt-1",
		AgentName: "test-agent",
		Message:   "do remote work",
	}

	result, err := client.Execute(context.Background(), task)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	if result.Output != "hello from remote" {
		t.Errorf("expected output 'hello from remote', got %s", result.Output)
	}
	if result.TokensIn != 300 {
		t.Errorf("expected tokensIn 300, got %d", result.TokensIn)
	}
	if result.TokensOut != 150 {
		t.Errorf("expected tokensOut 150, got %d", result.TokensOut)
	}
	if result.Cost != 0.15 {
		t.Errorf("expected cost 0.15, got %f", result.Cost)
	}
}

func TestA2AClientHealth(t *testing.T) {
	mock := &mockAdapter{healthy: true}
	client, cleanup := setupTestA2AClient(t, mock)
	defer cleanup()

	hs := client.Health()
	if !hs.Healthy {
		t.Error("expected healthy=true")
	}
}

func TestA2AClientType(t *testing.T) {
	client := &A2AClient{agent: "x"}
	if client.Type() != v1.AgentType("a2a") {
		t.Errorf("expected type 'a2a', got %s", client.Type())
	}
}

func TestA2AClientStatus(t *testing.T) {
	client := &A2AClient{agent: "x"}
	if client.Status() != v1.AgentPhaseRunning {
		t.Errorf("expected AgentPhaseRunning, got %s", client.Status())
	}
}
