//go:build integration

package integration

import (
	"context"
	"net"
	"testing"

	a2apb "github.com/zlc-ai/opc-platform/gen/a2a"
	opcpb "github.com/zlc-ai/opc-platform/gen/opc"
	"github.com/zlc-ai/opc-platform/pkg/federation"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/test/bufconn"
)

// mockTransport implements federation.Transport for integration tests.
type mockTransport struct{}

func (m *mockTransport) Send(_, _, _ string, _ any) ([]byte, error)                         { return []byte(`{}`), nil }
func (m *mockTransport) SendWithContext(_ context.Context, _, _, _ string, _ any) ([]byte, error) { return []byte(`{}`), nil }
func (m *mockTransport) Ping(_ string) error                                                  { return nil }
func (m *mockTransport) FetchStatus(_ string) (*federation.CompanyStatusReport, error) {
	return &federation.CompanyStatusReport{Status: "Online"}, nil
}

// startFederationServer creates a bufconn-backed gRPC server with a
// FederationGRPCServer, returning a client connection and cleanup function.
func startFederationServer(t *testing.T) (opcpb.FederationServiceClient, func()) {
	t.Helper()

	logger := zap.NewNop().Sugar()
	stateDir := t.TempDir()
	fc := federation.NewControllerForTest(stateDir, &mockTransport{}, logger)

	lis := bufconn.Listen(bufSize)
	srv := grpc.NewServer()
	fedSvc := federation.NewFederationGRPCServer(fc)
	opcpb.RegisterFederationServiceServer(srv, fedSvc)

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

	client := opcpb.NewFederationServiceClient(conn)
	cleanup := func() {
		conn.Close()
		srv.GracefulStop()
		lis.Close()
	}

	return client, cleanup
}

// TestFederation_Register_EndToEnd starts a federation gRPC server, registers a
// company, and verifies the response.
func TestFederation_Register_EndToEnd(t *testing.T) {
	client, cleanup := startFederationServer(t)
	defer cleanup()

	resp, err := client.Register(context.Background(), &opcpb.RegisterRequest{
		Company:         "test-company",
		Endpoint:        "http://localhost:9999",
		AvailableAgents: []string{"agent-a", "agent-b"},
	})
	if err != nil {
		t.Fatalf("Register RPC failed: %v", err)
	}
	if !resp.GetAccepted() {
		t.Errorf("expected accepted=true, got false: %s", resp.GetMessage())
	}
	if resp.GetAssignedId() == "" {
		t.Error("expected non-empty assigned_id")
	}
}

// TestFederation_Register_Duplicate verifies that registering the same company
// twice is rejected.
func TestFederation_Register_Duplicate(t *testing.T) {
	client, cleanup := startFederationServer(t)
	defer cleanup()

	req := &opcpb.RegisterRequest{
		Company:  "dup-company",
		Endpoint: "http://localhost:8888",
	}

	resp1, err := client.Register(context.Background(), req)
	if err != nil {
		t.Fatalf("first Register failed: %v", err)
	}
	if !resp1.GetAccepted() {
		t.Fatalf("first register should be accepted")
	}

	resp2, err := client.Register(context.Background(), req)
	if err != nil {
		t.Fatalf("second Register RPC error: %v", err)
	}
	if resp2.GetAccepted() {
		t.Error("expected duplicate registration to be rejected")
	}
}

// TestFederation_DispatchProject_EndToEnd registers a company and then
// dispatches a project, verifying the project is accepted.
func TestFederation_DispatchProject_EndToEnd(t *testing.T) {
	client, cleanup := startFederationServer(t)
	defer cleanup()

	// Register first.
	regResp, err := client.Register(context.Background(), &opcpb.RegisterRequest{
		Company:         "dispatch-co",
		Endpoint:        "http://localhost:7777",
		AvailableAgents: []string{"worker-1"},
	})
	if err != nil {
		t.Fatalf("Register failed: %v", err)
	}
	if !regResp.GetAccepted() {
		t.Fatalf("registration not accepted: %s", regResp.GetMessage())
	}

	// Dispatch a project.
	dispResp, err := client.DispatchProject(context.Background(), &opcpb.DispatchProjectRequest{
		GoalId:      "goal-001",
		ProjectName: "test-project",
		AgentName:   "worker-1",
		Company:     "dispatch-co",
	})
	if err != nil {
		t.Fatalf("DispatchProject RPC failed: %v", err)
	}
	if !dispResp.GetAccepted() {
		t.Errorf("expected accepted=true, got false: %s", dispResp.GetMessage())
	}
	if dispResp.GetTaskId() == "" {
		t.Error("expected non-empty task_id")
	}
}

// TestFederation_ReportResult_EndToEnd runs a full cycle: register -> dispatch
// -> report result.
func TestFederation_ReportResult_EndToEnd(t *testing.T) {
	client, cleanup := startFederationServer(t)
	defer cleanup()

	// Step 1: Register.
	_, err := client.Register(context.Background(), &opcpb.RegisterRequest{
		Company:         "result-co",
		Endpoint:        "http://localhost:6666",
		AvailableAgents: []string{"agent-x"},
	})
	if err != nil {
		t.Fatalf("Register failed: %v", err)
	}

	// Step 2: Dispatch.
	dispResp, err := client.DispatchProject(context.Background(), &opcpb.DispatchProjectRequest{
		GoalId:      "goal-002",
		ProjectName: "result-project",
		AgentName:   "agent-x",
		Company:     "result-co",
	})
	if err != nil {
		t.Fatalf("DispatchProject failed: %v", err)
	}

	// Step 3: Report result.
	reportResp, err := client.ReportTaskResult(context.Background(), &opcpb.ReportTaskResultRequest{
		GoalId:      "goal-002",
		ProjectName: "result-project",
		CompletedTask: &a2apb.Task{
			Id: dispResp.GetTaskId(),
			Status: &a2apb.TaskStatus{
				State: a2apb.TaskState_TASK_STATE_COMPLETED,
			},
			Artifacts: []*a2apb.Artifact{
				{Parts: []*a2apb.Part{{Part: &a2apb.Part_TextPart{TextPart: &a2apb.TextPart{Text: "task completed successfully"}}}}},
			},
		},
	})
	if err != nil {
		t.Fatalf("ReportTaskResult RPC failed: %v", err)
	}
	if !reportResp.GetAcknowledged() {
		t.Error("expected acknowledged=true")
	}
}

// TestFederation_GetStatus_EndToEnd registers companies and verifies the
// federation status response.
func TestFederation_GetStatus_EndToEnd(t *testing.T) {
	client, cleanup := startFederationServer(t)
	defer cleanup()

	// Register two companies.
	for _, name := range []string{"co-alpha", "co-beta"} {
		resp, err := client.Register(context.Background(), &opcpb.RegisterRequest{
			Company:         name,
			Endpoint:        "http://localhost:5555",
			AvailableAgents: []string{"agent-1"},
		})
		if err != nil {
			t.Fatalf("Register %s failed: %v", name, err)
		}
		if !resp.GetAccepted() {
			t.Fatalf("Register %s not accepted: %s", name, resp.GetMessage())
		}
	}

	// Get status.
	statusResp, err := client.GetFederationStatus(context.Background(), &opcpb.GetFederationStatusRequest{})
	if err != nil {
		t.Fatalf("GetFederationStatus RPC failed: %v", err)
	}
	if len(statusResp.GetNodes()) != 2 {
		t.Errorf("expected 2 nodes, got %d", len(statusResp.GetNodes()))
	}
	if statusResp.GetTotalAgents() != 2 {
		t.Errorf("expected 2 total agents, got %d", statusResp.GetTotalAgents())
	}
}
