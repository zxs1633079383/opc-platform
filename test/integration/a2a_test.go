//go:build integration

package integration

import (
	"context"
	"net"
	"testing"

	a2apb "github.com/zlc-ai/opc-platform/gen/a2a"
	opcpb "github.com/zlc-ai/opc-platform/gen/opc"
	v1 "github.com/zlc-ai/opc-platform/api/v1"
	"github.com/zlc-ai/opc-platform/pkg/a2a"
	"github.com/zlc-ai/opc-platform/pkg/adapter"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/test/bufconn"
)

const bufSize = 1024 * 1024

// testAdapter is a mock adapter that returns canned results.
type testAdapter struct {
	output string
	tokens int
}

func (a *testAdapter) Type() v1.AgentType              { return v1.AgentTypeCustom }
func (a *testAdapter) Start(_ context.Context, _ v1.AgentSpec) error { return nil }
func (a *testAdapter) Stop(_ context.Context) error     { return nil }
func (a *testAdapter) Health() v1.HealthStatus          { return v1.HealthStatus{Healthy: true, Message: "ok"} }
func (a *testAdapter) Execute(_ context.Context, _ v1.TaskRecord) (adapter.ExecuteResult, error) {
	return adapter.ExecuteResult{Output: a.output, TokensIn: a.tokens, TokensOut: a.tokens}, nil
}
func (a *testAdapter) Stream(_ context.Context, _ v1.TaskRecord) (<-chan adapter.Chunk, error) {
	return nil, nil
}
func (a *testAdapter) Status() v1.AgentPhase  { return v1.AgentPhaseRunning }
func (a *testAdapter) Metrics() v1.AgentMetrics { return v1.AgentMetrics{} }

// compile-time check
var _ adapter.Adapter = (*testAdapter)(nil)

// startA2AServer creates a bufconn-backed gRPC server with the given bridge and
// agent cards, returning a client connection and cleanup function.
func startA2AServer(t *testing.T, bridge *a2a.Bridge, cards map[string]*a2apb.AgentCard) (*grpc.ClientConn, func()) {
	t.Helper()

	lis := bufconn.Listen(bufSize)
	srv := grpc.NewServer()
	agentSvc := a2a.NewAgentServiceServer(bridge, cards)
	opcpb.RegisterAgentServiceServer(srv, agentSvc)

	go func() {
		if err := srv.Serve(lis); err != nil {
			// Server stopped — expected during cleanup.
		}
	}()

	conn, err := grpc.NewClient(
		"passthrough:///bufconn",
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithContextDialer(func(_ context.Context, _ string) (net.Conn, error) {
			return lis.Dial()
		}),
	)
	if err != nil {
		t.Fatalf("failed to dial bufconn: %v", err)
	}

	cleanup := func() {
		conn.Close()
		srv.GracefulStop()
		lis.Close()
	}

	return conn, cleanup
}

// TestA2A_SendTask_EndToEnd verifies the full chain: gRPC client -> server ->
// bridge -> adapter -> response with completed status and artifact.
func TestA2A_SendTask_EndToEnd(t *testing.T) {
	bridge := a2a.NewBridge()
	bridge.RegisterAdapter("test-agent", &testAdapter{
		output: "hello from test adapter",
		tokens: 42,
	})

	conn, cleanup := startA2AServer(t, bridge, nil)
	defer cleanup()

	client := opcpb.NewAgentServiceClient(conn)

	resp, err := client.SendTask(context.Background(), &opcpb.SendTaskRequest{
		TaskId:    "task-001",
		AgentName: "test-agent",
		Message: &a2apb.Message{
			Role: "user",
			Parts: []*a2apb.Part{
				{Part: &a2apb.Part_TextPart{TextPart: &a2apb.TextPart{Text: "do something"}}},
			},
		},
	})
	if err != nil {
		t.Fatalf("SendTask RPC failed: %v", err)
	}

	task := resp.GetTask()
	if task == nil {
		t.Fatal("expected non-nil task in response")
	}
	if task.GetId() != "task-001" {
		t.Errorf("task ID = %q, want %q", task.GetId(), "task-001")
	}
	if task.GetStatus().GetState() != a2apb.TaskState_TASK_STATE_COMPLETED {
		t.Errorf("task state = %v, want COMPLETED", task.GetStatus().GetState())
	}

	arts := task.GetArtifacts()
	if len(arts) == 0 {
		t.Fatal("expected at least one artifact")
	}
	firstPart := arts[0].GetParts()
	if len(firstPart) == 0 {
		t.Fatal("expected at least one part in artifact")
	}
	text := firstPart[0].GetTextPart().GetText()
	if text != "hello from test adapter" {
		t.Errorf("artifact text = %q, want %q", text, "hello from test adapter")
	}

	// Verify cost report.
	cost := resp.GetCost()
	if cost == nil {
		t.Fatal("expected non-nil cost report")
	}
	if cost.GetTokensIn() != 42 {
		t.Errorf("tokens_in = %d, want 42", cost.GetTokensIn())
	}
	if cost.GetTokensOut() != 42 {
		t.Errorf("tokens_out = %d, want 42", cost.GetTokensOut())
	}
}

// TestA2A_SendTask_UnknownAgent verifies that sending a task to an unregistered
// agent returns a gRPC error.
func TestA2A_SendTask_UnknownAgent(t *testing.T) {
	bridge := a2a.NewBridge()

	conn, cleanup := startA2AServer(t, bridge, nil)
	defer cleanup()

	client := opcpb.NewAgentServiceClient(conn)

	_, err := client.SendTask(context.Background(), &opcpb.SendTaskRequest{
		TaskId:    "task-002",
		AgentName: "nonexistent",
		Message: &a2apb.Message{
			Role:  "user",
			Parts: []*a2apb.Part{{Part: &a2apb.Part_TextPart{TextPart: &a2apb.TextPart{Text: "hello"}}}},
		},
	})
	if err == nil {
		t.Fatal("expected error for unknown agent, got nil")
	}
}

// TestA2A_AgentCard_EndToEnd registers an AgentCard and queries it via gRPC.
func TestA2A_AgentCard_EndToEnd(t *testing.T) {
	bridge := a2a.NewBridge()
	cards := map[string]*a2apb.AgentCard{
		"card-agent": {
			Name:        "card-agent",
			Description: "Test agent for card lookup",
			Url:         "http://localhost:9528",
			Version:     "opc/v1",
			Provider:    "opc-platform",
			Skills: []*a2apb.AgentSkill{
				{Id: "coding", Name: "coding"},
			},
			InputModes:  []string{"text"},
			OutputModes: []string{"text"},
		},
	}

	conn, cleanup := startA2AServer(t, bridge, cards)
	defer cleanup()

	client := opcpb.NewAgentServiceClient(conn)

	card, err := client.GetAgentCard(context.Background(), &opcpb.GetAgentCardRequest{
		AgentName: "card-agent",
	})
	if err != nil {
		t.Fatalf("GetAgentCard RPC failed: %v", err)
	}
	if card.GetName() != "card-agent" {
		t.Errorf("card name = %q, want %q", card.GetName(), "card-agent")
	}
	if card.GetDescription() != "Test agent for card lookup" {
		t.Errorf("card description = %q, want %q", card.GetDescription(), "Test agent for card lookup")
	}
	if card.GetProvider() != "opc-platform" {
		t.Errorf("card provider = %q, want %q", card.GetProvider(), "opc-platform")
	}
	if len(card.GetSkills()) != 1 || card.GetSkills()[0].GetId() != "coding" {
		t.Errorf("card skills unexpected: %v", card.GetSkills())
	}
}

// TestA2A_AgentCard_NotFound verifies that querying a missing card returns an error.
func TestA2A_AgentCard_NotFound(t *testing.T) {
	bridge := a2a.NewBridge()
	cards := map[string]*a2apb.AgentCard{}

	conn, cleanup := startA2AServer(t, bridge, cards)
	defer cleanup()

	client := opcpb.NewAgentServiceClient(conn)

	_, err := client.GetAgentCard(context.Background(), &opcpb.GetAgentCardRequest{
		AgentName: "missing",
	})
	if err == nil {
		t.Fatal("expected error for missing card, got nil")
	}
}

// TestA2A_Health_EndToEnd registers a healthy adapter and verifies the health
// response through the full gRPC chain.
func TestA2A_Health_EndToEnd(t *testing.T) {
	bridge := a2a.NewBridge()
	bridge.RegisterAdapter("healthy-agent", &testAdapter{
		output: "unused",
		tokens: 0,
	})

	conn, cleanup := startA2AServer(t, bridge, nil)
	defer cleanup()

	client := opcpb.NewAgentServiceClient(conn)

	resp, err := client.Health(context.Background(), &opcpb.HealthRequest{
		AgentName: "healthy-agent",
	})
	if err != nil {
		t.Fatalf("Health RPC failed: %v", err)
	}
	if !resp.GetHealthy() {
		t.Error("expected healthy=true, got false")
	}
}

// TestA2A_Health_UnknownAgent verifies that health check for an unknown agent
// returns an error.
func TestA2A_Health_UnknownAgent(t *testing.T) {
	bridge := a2a.NewBridge()

	conn, cleanup := startA2AServer(t, bridge, nil)
	defer cleanup()

	client := opcpb.NewAgentServiceClient(conn)

	_, err := client.Health(context.Background(), &opcpb.HealthRequest{
		AgentName: "ghost",
	})
	if err == nil {
		t.Fatal("expected error for unknown agent health, got nil")
	}
}
