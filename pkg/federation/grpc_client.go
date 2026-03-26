package federation

import (
	"context"

	opcpb "github.com/zlc-ai/opc-platform/gen/opc"
	"github.com/zlc-ai/opc-platform/pkg/a2a"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// FederationGRPCClient wraps a gRPC client for Master to Worker communication.
type FederationGRPCClient struct {
	conn   *grpc.ClientConn
	client opcpb.FederationServiceClient
}

// NewFederationGRPCClient creates a new FederationGRPCClient that connects to
// the given target address and authenticates with the provided API key.
func NewFederationGRPCClient(target string, apiKey string) (*FederationGRPCClient, error) {
	conn, err := grpc.NewClient(target,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithUnaryInterceptor(a2a.WithAPIKey(apiKey)),
	)
	if err != nil {
		return nil, err
	}

	return &FederationGRPCClient{
		conn:   conn,
		client: opcpb.NewFederationServiceClient(conn),
	}, nil
}

// Register sends a registration request to the federation master.
func (c *FederationGRPCClient) Register(ctx context.Context, req *opcpb.RegisterRequest) (*opcpb.RegisterResponse, error) {
	return c.client.Register(ctx, req)
}

// DispatchProject dispatches a project to a worker node for execution.
func (c *FederationGRPCClient) DispatchProject(ctx context.Context, req *opcpb.DispatchProjectRequest) (*opcpb.DispatchProjectResponse, error) {
	return c.client.DispatchProject(ctx, req)
}

// ReportTaskResult reports a completed task result back to the master.
func (c *FederationGRPCClient) ReportTaskResult(ctx context.Context, req *opcpb.ReportTaskResultRequest) (*opcpb.ReportTaskResultResponse, error) {
	return c.client.ReportTaskResult(ctx, req)
}

// GetFederationStatus retrieves the current federation status from the master.
func (c *FederationGRPCClient) GetFederationStatus(ctx context.Context) (*opcpb.FederationStatusResponse, error) {
	return c.client.GetFederationStatus(ctx, &opcpb.GetFederationStatusRequest{})
}

// Close closes the underlying gRPC connection.
func (c *FederationGRPCClient) Close() error {
	return c.conn.Close()
}
