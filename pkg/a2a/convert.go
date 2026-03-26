// Package a2a provides bidirectional conversion between OPC internal types
// and A2A/OPC protobuf types.
package a2a

import (
	"time"

	v1 "github.com/zlc-ai/opc-platform/api/v1"
	a2apb "github.com/zlc-ai/opc-platform/gen/a2a"
	opcpb "github.com/zlc-ai/opc-platform/gen/opc"
	"github.com/zlc-ai/opc-platform/pkg/adapter"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// OPCStatusToA2AState maps an OPC TaskStatus to an A2A TaskState.
func OPCStatusToA2AState(s v1.TaskStatus) a2apb.TaskState {
	switch s {
	case v1.TaskStatusPending:
		return a2apb.TaskState_TASK_STATE_SUBMITTED
	case v1.TaskStatusRunning:
		return a2apb.TaskState_TASK_STATE_WORKING
	case v1.TaskStatusCompleted:
		return a2apb.TaskState_TASK_STATE_COMPLETED
	case v1.TaskStatusFailed:
		return a2apb.TaskState_TASK_STATE_FAILED
	case v1.TaskStatusCancelled:
		return a2apb.TaskState_TASK_STATE_CANCELED
	default:
		return a2apb.TaskState_TASK_STATE_UNSPECIFIED
	}
}

// A2AStateToOPCStatus maps an A2A TaskState to an OPC TaskStatus.
func A2AStateToOPCStatus(s a2apb.TaskState) v1.TaskStatus {
	switch s {
	case a2apb.TaskState_TASK_STATE_SUBMITTED:
		return v1.TaskStatusPending
	case a2apb.TaskState_TASK_STATE_WORKING:
		return v1.TaskStatusRunning
	case a2apb.TaskState_TASK_STATE_COMPLETED:
		return v1.TaskStatusCompleted
	case a2apb.TaskState_TASK_STATE_FAILED:
		return v1.TaskStatusFailed
	case a2apb.TaskState_TASK_STATE_CANCELED:
		return v1.TaskStatusCancelled
	default:
		return v1.TaskStatusPending
	}
}

// newTextPart creates an A2A Part wrapping a TextPart.
func newTextPart(text string) *a2apb.Part {
	return &a2apb.Part{
		Part: &a2apb.Part_TextPart{
			TextPart: &a2apb.TextPart{Text: text},
		},
	}
}

// extractFirstTextFromParts returns the text of the first TextPart found, or "".
func extractFirstTextFromParts(parts []*a2apb.Part) string {
	for _, p := range parts {
		if tp := p.GetTextPart(); tp != nil {
			return tp.GetText()
		}
	}
	return ""
}

// TaskRecordToA2ATask converts a full TaskRecord to an A2A Task.
func TaskRecordToA2ATask(rec v1.TaskRecord) *a2apb.Task {
	task := &a2apb.Task{
		Id: rec.ID,
		Status: &a2apb.TaskStatus{
			State:     OPCStatusToA2AState(rec.Status),
			Timestamp: timestamppb.New(rec.UpdatedAt),
		},
		Messages: []*a2apb.Message{
			{
				Role:      "user",
				Parts:     []*a2apb.Part{newTextPart(rec.Message)},
				Timestamp: timestamppb.New(rec.CreatedAt),
			},
		},
	}

	// Add artifact only if Result is non-empty.
	if rec.Result != "" {
		task.Artifacts = []*a2apb.Artifact{
			{
				Parts: []*a2apb.Part{newTextPart(rec.Result)},
			},
		}
	}

	// Populate metadata from hierarchy IDs.
	meta := make(map[string]string)
	if rec.GoalID != "" {
		meta["goalId"] = rec.GoalID
	}
	if rec.ProjectID != "" {
		meta["projectId"] = rec.ProjectID
	}
	if rec.IssueID != "" {
		meta["issueId"] = rec.IssueID
	}
	if len(meta) > 0 {
		task.Metadata = meta
	}

	return task
}

// A2ATaskToTaskRecord converts an A2A Task back to an OPC TaskRecord.
func A2ATaskToTaskRecord(task *a2apb.Task, agentName string) v1.TaskRecord {
	if task == nil {
		return v1.TaskRecord{}
	}

	rec := v1.TaskRecord{
		ID:        task.GetId(),
		AgentName: agentName,
		Status:    A2AStateToOPCStatus(task.GetStatus().GetState()),
	}

	// Extract first user message text.
	for _, msg := range task.GetMessages() {
		if msg.GetRole() == "user" {
			rec.Message = extractFirstTextFromParts(msg.GetParts())
			break
		}
	}

	// Extract first artifact text as Result.
	if arts := task.GetArtifacts(); len(arts) > 0 {
		rec.Result = extractFirstTextFromParts(arts[0].GetParts())
	}

	// Extract metadata.
	meta := task.GetMetadata()
	if meta != nil {
		rec.GoalID = meta["goalId"]
		rec.ProjectID = meta["projectId"]
		rec.IssueID = meta["issueId"]
	}

	now := time.Now()
	rec.CreatedAt = now
	rec.UpdatedAt = now

	return rec
}

// ExecuteResultToArtifact converts an adapter.ExecuteResult to an A2A Artifact
// and an OPC CostReport.
func ExecuteResultToArtifact(res adapter.ExecuteResult) (*a2apb.Artifact, *opcpb.CostReport) {
	art := &a2apb.Artifact{
		Parts: []*a2apb.Part{newTextPart(res.Output)},
	}

	cost := &opcpb.CostReport{
		TokensIn:  int64(res.TokensIn),
		TokensOut: int64(res.TokensOut),
		CostUsd:   res.Cost,
		Estimated: res.Estimated,
	}

	return art, cost
}

// SendTaskRequestToTaskRecord converts a gRPC SendTaskRequest to a TaskRecord.
func SendTaskRequestToTaskRecord(req *opcpb.SendTaskRequest) v1.TaskRecord {
	if req == nil {
		return v1.TaskRecord{}
	}

	rec := v1.TaskRecord{
		ID:        req.GetTaskId(),
		AgentName: req.GetAgentName(),
		Status:    v1.TaskStatusPending,
	}

	// Extract text from message parts.
	if msg := req.GetMessage(); msg != nil {
		rec.Message = extractFirstTextFromParts(msg.GetParts())
	}

	// Extract metadata.
	meta := req.GetMetadata()
	if meta != nil {
		rec.GoalID = meta["goalId"]
		rec.ProjectID = meta["projectId"]
		rec.IssueID = meta["issueId"]
	}

	now := time.Now()
	rec.CreatedAt = now
	rec.UpdatedAt = now

	return rec
}

// TaskRecordToSendTaskRequest converts a TaskRecord to a gRPC SendTaskRequest.
func TaskRecordToSendTaskRequest(rec v1.TaskRecord) *opcpb.SendTaskRequest {
	req := &opcpb.SendTaskRequest{
		TaskId:    rec.ID,
		AgentName: rec.AgentName,
		Message: &a2apb.Message{
			Role:  "user",
			Parts: []*a2apb.Part{newTextPart(rec.Message)},
		},
	}

	// Populate metadata from hierarchy IDs.
	meta := make(map[string]string)
	if rec.GoalID != "" {
		meta["goalId"] = rec.GoalID
	}
	if rec.ProjectID != "" {
		meta["projectId"] = rec.ProjectID
	}
	if rec.IssueID != "" {
		meta["issueId"] = rec.IssueID
	}
	if len(meta) > 0 {
		req.Metadata = meta
	}

	return req
}
