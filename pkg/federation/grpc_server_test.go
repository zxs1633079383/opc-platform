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

func setupFederationTestServer(t *testing.T) (opcpb.FederationServiceClient, func()) {
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
		t.Fatalf("failed to create client: %v", err)
	}

	client := opcpb.NewFederationServiceClient(conn)
	cleanup := func() {
		conn.Close()
		s.Stop()
	}
	return client, cleanup
}

func TestFederationGRPCServer_Register(t *testing.T) {
	client, cleanup := setupFederationTestServer(t)
	defer cleanup()

	resp, err := client.Register(context.Background(), &opcpb.RegisterRequest{
		NodeId:          "node-1",
		Company:         "test-company",
		Endpoint:        "http://localhost:9090",
		AvailableAgents: []string{"agent-a", "agent-b"},
		ApiKey:          "test-key",
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
	if resp.GetMessage() == "" {
		t.Error("expected non-empty message")
	}
}

func TestFederationGRPCServer_GetFederationStatus(t *testing.T) {
	client, cleanup := setupFederationTestServer(t)
	defer cleanup()

	// Register a company first.
	_, err := client.Register(context.Background(), &opcpb.RegisterRequest{
		NodeId:          "node-1",
		Company:         "status-company",
		Endpoint:        "http://localhost:9091",
		AvailableAgents: []string{"agent-x"},
	})
	if err != nil {
		t.Fatalf("Register failed: %v", err)
	}

	resp, err := client.GetFederationStatus(context.Background(), &opcpb.GetFederationStatusRequest{})
	if err != nil {
		t.Fatalf("GetFederationStatus failed: %v", err)
	}

	if len(resp.GetNodes()) == 0 {
		t.Fatal("expected at least one node")
	}

	found := false
	for _, node := range resp.GetNodes() {
		if node.GetCompany() == "status-company" {
			found = true
			if len(node.GetAgents()) != 1 || node.GetAgents()[0] != "agent-x" {
				t.Errorf("expected agents [agent-x], got %v", node.GetAgents())
			}
		}
	}
	if !found {
		t.Error("status-company not found in federation status")
	}

	if resp.GetTotalAgents() < 1 {
		t.Errorf("expected total_agents >= 1, got %d", resp.GetTotalAgents())
	}
}

func TestFederationGRPCServer_DispatchProject(t *testing.T) {
	client, cleanup := setupFederationTestServer(t)
	defer cleanup()

	resp, err := client.DispatchProject(context.Background(), &opcpb.DispatchProjectRequest{
		GoalId:      "goal-1",
		ProjectName: "test-project",
		AgentName:   "agent-a",
		Company:     "test-company",
		TaskMessage: &a2apb.Message{
			Role:  "user",
			Parts: []*a2apb.Part{{Part: &a2apb.Part_TextPart{TextPart: &a2apb.TextPart{Text: "do work"}}}},
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

func TestFederationGRPCServer_ReportTaskResult(t *testing.T) {
	client, cleanup := setupFederationTestServer(t)
	defer cleanup()

	resp, err := client.ReportTaskResult(context.Background(), &opcpb.ReportTaskResultRequest{
		GoalId:      "goal-1",
		ProjectName: "test-project",
		CompletedTask: &a2apb.Task{
			Id: "task-1",
			Status: &a2apb.TaskStatus{
				State: a2apb.TaskState_TASK_STATE_COMPLETED,
			},
		},
	})
	if err != nil {
		t.Fatalf("ReportTaskResult failed: %v", err)
	}

	if !resp.GetAcknowledged() {
		t.Error("expected acknowledged=true")
	}
}
