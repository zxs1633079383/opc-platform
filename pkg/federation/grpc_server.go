package federation

import (
	"context"
	"fmt"
	"io"
	"time"

	"github.com/google/uuid"
	opcpb "github.com/zlc-ai/opc-platform/gen/opc"
	"google.golang.org/grpc"
)

// FederationGRPCServer implements the FederationService gRPC service.
type FederationGRPCServer struct {
	opcpb.UnimplementedFederationServiceServer
	fc *FederationController
}

// NewFederationGRPCServer creates a new FederationGRPCServer backed by the
// given FederationController.
func NewFederationGRPCServer(fc *FederationController) *FederationGRPCServer {
	return &FederationGRPCServer{fc: fc}
}

// Register handles a node registration request by creating a new company in the
// federation controller.
func (s *FederationGRPCServer) Register(_ context.Context, req *opcpb.RegisterRequest) (*opcpb.RegisterResponse, error) {
	reg := CompanyRegistration{
		Name:     req.GetCompany(),
		Endpoint: req.GetEndpoint(),
		Type:     CompanyTypeSoftware, // default type for gRPC registrations
		Agents:   req.GetAvailableAgents(),
	}

	company, err := s.fc.RegisterCompany(reg)
	if err != nil {
		return &opcpb.RegisterResponse{
			Accepted: false,
			Message:  fmt.Sprintf("registration failed: %v", err),
		}, nil
	}

	return &opcpb.RegisterResponse{
		Accepted:   true,
		AssignedId: company.ID,
		Message:    fmt.Sprintf("company %q registered successfully", company.Name),
	}, nil
}

// DispatchProject accepts a project dispatch request. The actual agent execution
// integration is deferred to Task 13; for now it returns accepted with a
// generated task ID.
func (s *FederationGRPCServer) DispatchProject(_ context.Context, req *opcpb.DispatchProjectRequest) (*opcpb.DispatchProjectResponse, error) {
	taskID := fmt.Sprintf("task-%s", uuid.New().String()[:8])

	s.fc.logger.Infow("project dispatched via gRPC",
		"goal_id", req.GetGoalId(),
		"project", req.GetProjectName(),
		"agent", req.GetAgentName(),
		"company", req.GetCompany(),
		"task_id", taskID,
	)

	return &opcpb.DispatchProjectResponse{
		Accepted: true,
		TaskId:   taskID,
		Message:  fmt.Sprintf("project %q accepted for execution", req.GetProjectName()),
	}, nil
}

// ReportTaskResult accepts a completed task result report from a worker node.
func (s *FederationGRPCServer) ReportTaskResult(_ context.Context, req *opcpb.ReportTaskResultRequest) (*opcpb.ReportTaskResultResponse, error) {
	s.fc.logger.Infow("task result reported via gRPC",
		"goal_id", req.GetGoalId(),
		"project", req.GetProjectName(),
	)

	return &opcpb.ReportTaskResultResponse{
		Acknowledged: true,
		NextAction:   "",
	}, nil
}

// GetFederationStatus returns the current federation status including all
// registered nodes and their agents.
func (s *FederationGRPCServer) GetFederationStatus(_ context.Context, _ *opcpb.GetFederationStatusRequest) (*opcpb.FederationStatusResponse, error) {
	companies := s.fc.ListCompanies()

	nodes := make([]*opcpb.NodeStatus, 0, len(companies))
	var totalAgents int32

	for _, c := range companies {
		nodes = append(nodes, &opcpb.NodeStatus{
			NodeId:        c.ID,
			Company:       c.Name,
			Status:        string(c.Status),
			LastHeartbeat: c.JoinedAt.Unix(),
			Agents:        c.Agents,
		})
		totalAgents += int32(len(c.Agents))
	}

	return &opcpb.FederationStatusResponse{
		Nodes:       nodes,
		TotalAgents: totalAgents,
		ActiveGoals: 0, // no goal tracking at this layer yet
	}, nil
}

// AssessResult is a stub that returns a satisfied assessment. The actual
// assessment logic lives in existing Go code and will be wired in later.
func (s *FederationGRPCServer) AssessResult(_ context.Context, req *opcpb.AssessRequest) (*opcpb.AssessResponse, error) {
	return &opcpb.AssessResponse{
		Assessment: &opcpb.AssessmentResult{
			Satisfied: true,
			Reason:    fmt.Sprintf("assessment stub for goal %q", req.GetGoalName()),
		},
	}, nil
}

// HeartbeatStream handles the bidirectional heartbeat stream. It receives pings
// from worker nodes and responds with pongs. Company status is updated on each
// heartbeat.
func (s *FederationGRPCServer) HeartbeatStream(stream grpc.BidiStreamingServer[opcpb.HeartbeatPing, opcpb.HeartbeatPong]) error {
	for {
		ping, err := stream.Recv()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}

		s.fc.logger.Debugw("heartbeat received",
			"node_id", ping.GetNodeId(),
			"company", ping.GetCompany(),
			"timestamp", time.Unix(ping.GetTimestamp(), 0),
		)

		// Try to find the company and update its status.
		company, findErr := s.fc.FindCompanyByName(ping.GetCompany())
		if findErr == nil {
			_ = s.fc.UpdateCompanyStatus(company.ID, CompanyStatusOnline)
		}

		pong := &opcpb.HeartbeatPong{
			Accepted: true,
		}
		if err := stream.Send(pong); err != nil {
			return err
		}
	}
}
