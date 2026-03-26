package a2a

import (
	"context"
	"fmt"

	v1 "github.com/zlc-ai/opc-platform/api/v1"
	opcpb "github.com/zlc-ai/opc-platform/gen/opc"
	"github.com/zlc-ai/opc-platform/pkg/adapter"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// A2AClient implements adapter.Adapter over a gRPC connection to a remote
// AgentService. It converts between OPC internal types and gRPC requests.
type A2AClient struct {
	conn   *grpc.ClientConn
	client opcpb.AgentServiceClient
	agent  string
}

// NewA2AClient creates a new A2AClient connected to the given gRPC target.
func NewA2AClient(target string, agentName string) (*A2AClient, error) {
	conn, err := grpc.NewClient(target,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		return nil, fmt.Errorf("a2a client dial %s: %w", target, err)
	}

	return &A2AClient{
		conn:   conn,
		client: opcpb.NewAgentServiceClient(conn),
		agent:  agentName,
	}, nil
}

// Type returns the agent type "a2a".
func (c *A2AClient) Type() v1.AgentType {
	return v1.AgentType("a2a")
}

// Start is a no-op for remote agents.
func (c *A2AClient) Start(_ context.Context, _ v1.AgentSpec) error {
	return nil
}

// Stop closes the gRPC connection.
func (c *A2AClient) Stop(_ context.Context) error {
	return c.Close()
}

// Health calls the remote Health RPC and returns the result as a HealthStatus.
func (c *A2AClient) Health() v1.HealthStatus {
	resp, err := c.client.Health(context.Background(), &opcpb.HealthRequest{
		AgentName: c.agent,
	})
	if err != nil {
		return v1.HealthStatus{
			Healthy: false,
			Message: fmt.Sprintf("health RPC failed: %v", err),
		}
	}
	return v1.HealthStatus{
		Healthy: resp.GetHealthy(),
		Message: resp.GetMessage(),
	}
}

// Execute converts a TaskRecord to a SendTaskRequest, calls SendTask over gRPC,
// and extracts the result from the response artifact and cost report.
func (c *A2AClient) Execute(ctx context.Context, task v1.TaskRecord) (adapter.ExecuteResult, error) {
	req := TaskRecordToSendTaskRequest(task)

	resp, err := c.client.SendTask(ctx, req)
	if err != nil {
		return adapter.ExecuteResult{}, fmt.Errorf("a2a SendTask RPC: %w", err)
	}

	a2aTask := resp.GetTask()
	if a2aTask == nil {
		return adapter.ExecuteResult{}, fmt.Errorf("a2a SendTask: nil task in response")
	}

	// Extract output from first artifact's first text part.
	var output string
	if arts := a2aTask.GetArtifacts(); len(arts) > 0 {
		output = extractFirstTextFromParts(arts[0].GetParts())
	}

	// Extract cost from the cost report.
	var tokensIn, tokensOut int
	var cost float64
	if cr := resp.GetCost(); cr != nil {
		tokensIn = int(cr.GetTokensIn())
		tokensOut = int(cr.GetTokensOut())
		cost = cr.GetCostUsd()
	}

	return adapter.ExecuteResult{
		Output:    output,
		TokensIn:  tokensIn,
		TokensOut: tokensOut,
		Cost:      cost,
	}, nil
}

// Stream is not implemented for A2A clients.
func (c *A2AClient) Stream(_ context.Context, _ v1.TaskRecord) (<-chan adapter.Chunk, error) {
	return nil, fmt.Errorf("streaming not implemented for a2a client")
}

// Status returns AgentPhaseRunning for remote agents.
func (c *A2AClient) Status() v1.AgentPhase {
	return v1.AgentPhaseRunning
}

// Metrics returns empty metrics for remote agents.
func (c *A2AClient) Metrics() v1.AgentMetrics {
	return v1.AgentMetrics{}
}

// Close closes the underlying gRPC connection.
func (c *A2AClient) Close() error {
	if c.conn != nil {
		return c.conn.Close()
	}
	return nil
}

// compile-time check that A2AClient implements adapter.Adapter.
var _ adapter.Adapter = (*A2AClient)(nil)
