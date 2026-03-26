package federation

import (
	"context"
	"net"
	"testing"

	a2apb "github.com/zlc-ai/opc-platform/gen/a2a"
	opcpb "github.com/zlc-ai/opc-platform/gen/opc"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/test/bufconn"
)

// setupFederationClientTest creates a test gRPC server with bufconn and returns
// a FederationGRPCClient connected to it.
func setupFederationClientTest(t *testing.T) (*FederationGRPCClient, func()) {
	t.Helper()

	fc := newTestController(t, &mockTransport{})
	srv := NewFederationGRPCServer(fc)

	lis := bufconn.Listen(1024 * 1024)
	s := grpc.NewServer()
	opcpb.RegisterFederationServiceServer(s, srv)
	go func() { _ = s.Serve(lis) }()

	conn, err := grpc.NewClient("passthrough:///bufnet",
		grpc.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) {
			return lis.DialContext(ctx)
		}),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		t.Fatalf("failed to create client conn: %v", err)
	}

	client := &FederationGRPCClient{
		conn:   conn,
		client: opcpb.NewFederationServiceClient(conn),
	}

	cleanup := func() {
		client.Close()
		s.Stop()
	}
	return client, cleanup
}

func TestFederationGRPCClient_Register(t *testing.T) {
	client, cleanup := setupFederationClientTest(t)
	defer cleanup()

	resp, err := client.Register(context.Background(), &opcpb.RegisterRequest{
		NodeId:          "node-client-1",
		Company:         "client-test-company",
		Endpoint:        "http://localhost:8080",
		AvailableAgents: []string{"agent-1"},
		ApiKey:          "key-1",
	})
	if err != nil {
		t.Fatalf("Register failed: %v", err)
	}
	if !resp.GetAccepted() {
		t.Error("expected accepted=true")
	}
	if resp.GetAssignedId() == "" {
		t.Error("expected non-empty assigned_id")
	}
}

func TestFederationGRPCClient_DispatchProject(t *testing.T) {
	client, cleanup := setupFederationClientTest(t)
	defer cleanup()

	resp, err := client.DispatchProject(context.Background(), &opcpb.DispatchProjectRequest{
		GoalId:      "goal-2",
		ProjectName: "client-project",
		AgentName:   "agent-1",
		Company:     "client-company",
		TaskMessage: &a2apb.Message{
			Role:  "user",
			Parts: []*a2apb.Part{{Part: &a2apb.Part_TextPart{TextPart: &a2apb.TextPart{Text: "build feature"}}}},
		},
	})
	if err != nil {
		t.Fatalf("DispatchProject failed: %v", err)
	}
	if !resp.GetAccepted() {
		t.Error("expected accepted=true")
	}
	if resp.GetTaskId() == "" {
		t.Error("expected non-empty task_id")
	}
}
