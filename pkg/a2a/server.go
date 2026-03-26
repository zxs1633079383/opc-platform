package a2a

import (
	"context"

	a2apb "github.com/zlc-ai/opc-platform/gen/a2a"
	opcpb "github.com/zlc-ai/opc-platform/gen/opc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// AgentServiceServer implements the opc.AgentServiceServer gRPC interface
// by delegating to the Bridge and serving AgentCards from an in-memory map.
type AgentServiceServer struct {
	opcpb.UnimplementedAgentServiceServer
	bridge *Bridge
	cards  map[string]*a2apb.AgentCard
}

// NewAgentServiceServer creates a new AgentServiceServer.
func NewAgentServiceServer(bridge *Bridge, cards map[string]*a2apb.AgentCard) *AgentServiceServer {
	return &AgentServiceServer{
		bridge: bridge,
		cards:  cards,
	}
}

// SendTask delegates to bridge.SendTask.
func (s *AgentServiceServer) SendTask(ctx context.Context, req *opcpb.SendTaskRequest) (*opcpb.SendTaskResponse, error) {
	resp, err := s.bridge.SendTask(ctx, req)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "send task failed: %v", err)
	}
	return resp, nil
}

// Health delegates to bridge.Health.
func (s *AgentServiceServer) Health(ctx context.Context, req *opcpb.HealthRequest) (*opcpb.HealthResponse, error) {
	resp, err := s.bridge.Health(req.GetAgentName())
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "health check failed: %v", err)
	}
	return resp, nil
}

// GetAgentCard looks up the AgentCard from the cards map.
func (s *AgentServiceServer) GetAgentCard(_ context.Context, req *opcpb.GetAgentCardRequest) (*a2apb.AgentCard, error) {
	name := req.GetAgentName()
	card, ok := s.cards[name]
	if !ok {
		return nil, status.Errorf(codes.NotFound, "agent card not found: %s", name)
	}
	return card, nil
}

// Start returns a success response indicating delegation to the controller.
func (s *AgentServiceServer) Start(_ context.Context, _ *opcpb.StartRequest) (*opcpb.StartResponse, error) {
	return &opcpb.StartResponse{
		Success: true,
		Message: "delegated to controller",
	}, nil
}

// Stop returns a success response.
func (s *AgentServiceServer) Stop(_ context.Context, _ *opcpb.StopRequest) (*opcpb.StopResponse, error) {
	return &opcpb.StopResponse{
		Success: true,
	}, nil
}

// GetTask is not yet implemented.
func (s *AgentServiceServer) GetTask(_ context.Context, req *opcpb.GetTaskRequest) (*a2apb.Task, error) {
	return nil, status.Errorf(codes.Unimplemented, "GetTask not implemented for task %s", req.GetTaskId())
}

// CancelTask is not yet implemented.
func (s *AgentServiceServer) CancelTask(_ context.Context, req *opcpb.CancelTaskRequest) (*a2apb.Task, error) {
	return nil, status.Errorf(codes.Unimplemented, "CancelTask not implemented for task %s", req.GetTaskId())
}

// compile-time check
var _ opcpb.AgentServiceServer = (*AgentServiceServer)(nil)

