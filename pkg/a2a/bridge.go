package a2a

import (
	"context"
	"fmt"
	"sync"
	"time"

	a2apb "github.com/zlc-ai/opc-platform/gen/a2a"
	opcpb "github.com/zlc-ai/opc-platform/gen/opc"
	"github.com/zlc-ai/opc-platform/pkg/adapter"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// Bridge routes A2A gRPC requests to native OPC Adapters.
type Bridge struct {
	mu       sync.RWMutex
	adapters map[string]adapter.Adapter
}

// NewBridge creates a new Bridge with an empty adapter registry.
func NewBridge() *Bridge {
	return &Bridge{
		adapters: make(map[string]adapter.Adapter),
	}
}

// RegisterAdapter registers an adapter under the given agent name.
func (b *Bridge) RegisterAdapter(agentName string, a adapter.Adapter) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.adapters[agentName] = a
}

// UnregisterAdapter removes the adapter for the given agent name.
func (b *Bridge) UnregisterAdapter(agentName string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	delete(b.adapters, agentName)
}

// getAdapter returns the adapter for the given agent name or an error.
func (b *Bridge) getAdapter(agentName string) (adapter.Adapter, error) {
	b.mu.RLock()
	defer b.mu.RUnlock()
	a, ok := b.adapters[agentName]
	if !ok {
		return nil, fmt.Errorf("unknown agent: %s", agentName)
	}
	return a, nil
}

// SendTask routes a gRPC SendTaskRequest to the appropriate adapter.
// On adapter error it returns a response with FAILED state (not a Go error).
// Only returns a Go error for routing failures (unknown agent).
func (b *Bridge) SendTask(ctx context.Context, req *opcpb.SendTaskRequest) (*opcpb.SendTaskResponse, error) {
	agentName := req.GetAgentName()
	a, err := b.getAdapter(agentName)
	if err != nil {
		return nil, err
	}

	taskRecord := SendTaskRequestToTaskRecord(req)
	result, execErr := a.Execute(ctx, taskRecord)

	now := timestamppb.New(time.Now())

	if execErr != nil {
		// Return FAILED state with error message — no Go-level error.
		return &opcpb.SendTaskResponse{
			Task: &a2apb.Task{
				Id: req.GetTaskId(),
				Status: &a2apb.TaskStatus{
					State:     a2apb.TaskState_TASK_STATE_FAILED,
					Reason:    execErr.Error(),
					Timestamp: now,
				},
			},
		}, nil
	}

	artifact, costReport := ExecuteResultToArtifact(result)

	return &opcpb.SendTaskResponse{
		Task: &a2apb.Task{
			Id: req.GetTaskId(),
			Status: &a2apb.TaskStatus{
				State:     a2apb.TaskState_TASK_STATE_COMPLETED,
				Timestamp: now,
			},
			Artifacts: []*a2apb.Artifact{artifact},
		},
		Cost: costReport,
	}, nil
}

// Health returns the health status of the named agent's adapter.
func (b *Bridge) Health(agentName string) (*opcpb.HealthResponse, error) {
	a, err := b.getAdapter(agentName)
	if err != nil {
		return nil, err
	}

	hs := a.Health()
	return &opcpb.HealthResponse{
		Healthy: hs.Healthy,
		Message: hs.Message,
	}, nil
}
