package a2a

import (
	"testing"
	"time"

	v1 "github.com/zlc-ai/opc-platform/api/v1"
	a2apb "github.com/zlc-ai/opc-platform/gen/a2a"
	opcpb "github.com/zlc-ai/opc-platform/gen/opc"
	"github.com/zlc-ai/opc-platform/pkg/adapter"
)

func TestTaskStateMapping(t *testing.T) {
	tests := []struct {
		name      string
		opcStatus v1.TaskStatus
		a2aState  a2apb.TaskState
	}{
		{"Pending→SUBMITTED", v1.TaskStatusPending, a2apb.TaskState_TASK_STATE_SUBMITTED},
		{"Running→WORKING", v1.TaskStatusRunning, a2apb.TaskState_TASK_STATE_WORKING},
		{"Completed→COMPLETED", v1.TaskStatusCompleted, a2apb.TaskState_TASK_STATE_COMPLETED},
		{"Failed→FAILED", v1.TaskStatusFailed, a2apb.TaskState_TASK_STATE_FAILED},
		{"Cancelled→CANCELED", v1.TaskStatusCancelled, a2apb.TaskState_TASK_STATE_CANCELED},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := OPCStatusToA2AState(tt.opcStatus)
			if got != tt.a2aState {
				t.Errorf("OPCStatusToA2AState(%q) = %v, want %v", tt.opcStatus, got, tt.a2aState)
			}
		})
	}

	// Test default case.
	t.Run("Unknown→UNSPECIFIED", func(t *testing.T) {
		got := OPCStatusToA2AState(v1.TaskStatus("Unknown"))
		if got != a2apb.TaskState_TASK_STATE_UNSPECIFIED {
			t.Errorf("OPCStatusToA2AState(%q) = %v, want UNSPECIFIED", "Unknown", got)
		}
	})

	// Test roundtrip for all known states.
	t.Run("Roundtrip", func(t *testing.T) {
		for _, tt := range tests {
			roundtripped := A2AStateToOPCStatus(OPCStatusToA2AState(tt.opcStatus))
			if roundtripped != tt.opcStatus {
				t.Errorf("roundtrip failed for %q: got %q", tt.opcStatus, roundtripped)
			}
		}
	})

	// Test UNSPECIFIED → Pending.
	t.Run("UNSPECIFIED→Pending", func(t *testing.T) {
		got := A2AStateToOPCStatus(a2apb.TaskState_TASK_STATE_UNSPECIFIED)
		if got != v1.TaskStatusPending {
			t.Errorf("A2AStateToOPCStatus(UNSPECIFIED) = %q, want Pending", got)
		}
	})

	// Test INPUT_REQUIRED → Pending (unmapped A2A state).
	t.Run("INPUT_REQUIRED→Pending", func(t *testing.T) {
		got := A2AStateToOPCStatus(a2apb.TaskState_TASK_STATE_INPUT_REQUIRED)
		if got != v1.TaskStatusPending {
			t.Errorf("A2AStateToOPCStatus(INPUT_REQUIRED) = %q, want Pending", got)
		}
	})
}

func TestTaskRecordToA2ATask(t *testing.T) {
	now := time.Now()
	rec := v1.TaskRecord{
		ID:        "task-123",
		AgentName: "agent-1",
		Message:   "implement feature X",
		Status:    v1.TaskStatusCompleted,
		Result:    "feature X implemented",
		GoalID:    "goal-1",
		ProjectID: "proj-2",
		IssueID:   "issue-3",
		CreatedAt: now,
		UpdatedAt: now,
	}

	task := TaskRecordToA2ATask(rec)

	// Verify ID.
	if task.GetId() != "task-123" {
		t.Errorf("ID = %q, want %q", task.GetId(), "task-123")
	}

	// Verify status.
	if task.GetStatus().GetState() != a2apb.TaskState_TASK_STATE_COMPLETED {
		t.Errorf("State = %v, want COMPLETED", task.GetStatus().GetState())
	}

	// Verify message.
	msgs := task.GetMessages()
	if len(msgs) != 1 {
		t.Fatalf("len(Messages) = %d, want 1", len(msgs))
	}
	if msgs[0].GetRole() != "user" {
		t.Errorf("Message.Role = %q, want %q", msgs[0].GetRole(), "user")
	}
	parts := msgs[0].GetParts()
	if len(parts) != 1 {
		t.Fatalf("len(Parts) = %d, want 1", len(parts))
	}
	tp := parts[0].GetTextPart()
	if tp == nil || tp.GetText() != "implement feature X" {
		t.Errorf("Message text = %q, want %q", tp.GetText(), "implement feature X")
	}

	// Verify artifact.
	artifacts := task.GetArtifacts()
	if len(artifacts) != 1 {
		t.Fatalf("len(Artifacts) = %d, want 1", len(artifacts))
	}
	artParts := artifacts[0].GetParts()
	if len(artParts) != 1 {
		t.Fatalf("len(artifact parts) = %d, want 1", len(artParts))
	}
	artTP := artParts[0].GetTextPart()
	if artTP == nil || artTP.GetText() != "feature X implemented" {
		t.Errorf("Artifact text = %q, want %q", artTP.GetText(), "feature X implemented")
	}

	// Verify metadata.
	meta := task.GetMetadata()
	if meta["goalId"] != "goal-1" {
		t.Errorf("metadata[goalId] = %q, want %q", meta["goalId"], "goal-1")
	}
	if meta["projectId"] != "proj-2" {
		t.Errorf("metadata[projectId] = %q, want %q", meta["projectId"], "proj-2")
	}
	if meta["issueId"] != "issue-3" {
		t.Errorf("metadata[issueId] = %q, want %q", meta["issueId"], "issue-3")
	}

	// Verify empty result produces no artifact.
	recNoResult := v1.TaskRecord{
		ID:      "task-456",
		Message: "do something",
		Status:  v1.TaskStatusPending,
	}
	taskNoResult := TaskRecordToA2ATask(recNoResult)
	if len(taskNoResult.GetArtifacts()) != 0 {
		t.Errorf("expected no artifacts for empty result, got %d", len(taskNoResult.GetArtifacts()))
	}
}

func TestA2ATaskToTaskRecord(t *testing.T) {
	task := &a2apb.Task{
		Id: "task-abc",
		Status: &a2apb.TaskStatus{
			State: a2apb.TaskState_TASK_STATE_WORKING,
		},
		Messages: []*a2apb.Message{
			{
				Role: "user",
				Parts: []*a2apb.Part{
					{Part: &a2apb.Part_TextPart{TextPart: &a2apb.TextPart{Text: "hello world"}}},
				},
			},
		},
		Artifacts: []*a2apb.Artifact{
			{
				Parts: []*a2apb.Part{
					{Part: &a2apb.Part_TextPart{TextPart: &a2apb.TextPart{Text: "result text"}}},
				},
			},
		},
		Metadata: map[string]string{
			"goalId":    "g-1",
			"projectId": "p-2",
			"issueId":   "i-3",
		},
	}

	rec := A2ATaskToTaskRecord(task, "my-agent")

	if rec.ID != "task-abc" {
		t.Errorf("ID = %q, want %q", rec.ID, "task-abc")
	}
	if rec.AgentName != "my-agent" {
		t.Errorf("AgentName = %q, want %q", rec.AgentName, "my-agent")
	}
	if rec.Status != v1.TaskStatusRunning {
		t.Errorf("Status = %q, want %q", rec.Status, v1.TaskStatusRunning)
	}
	if rec.Message != "hello world" {
		t.Errorf("Message = %q, want %q", rec.Message, "hello world")
	}
	if rec.Result != "result text" {
		t.Errorf("Result = %q, want %q", rec.Result, "result text")
	}
	if rec.GoalID != "g-1" {
		t.Errorf("GoalID = %q, want %q", rec.GoalID, "g-1")
	}
	if rec.ProjectID != "p-2" {
		t.Errorf("ProjectID = %q, want %q", rec.ProjectID, "p-2")
	}
	if rec.IssueID != "i-3" {
		t.Errorf("IssueID = %q, want %q", rec.IssueID, "i-3")
	}

	// Test nil task.
	recNil := A2ATaskToTaskRecord(nil, "agent")
	if recNil.ID != "" {
		t.Errorf("nil task should produce empty TaskRecord, got ID=%q", recNil.ID)
	}
}

func TestExecuteResultToArtifact(t *testing.T) {
	res := adapter.ExecuteResult{
		Output:    "generated code here",
		TokensIn:  1500,
		TokensOut: 3000,
		Cost:      0.045,
		Estimated: true,
	}

	art, cost := ExecuteResultToArtifact(res)

	// Verify artifact.
	if art == nil {
		t.Fatal("artifact should not be nil")
	}
	artParts := art.GetParts()
	if len(artParts) != 1 {
		t.Fatalf("len(artifact parts) = %d, want 1", len(artParts))
	}
	tp := artParts[0].GetTextPart()
	if tp == nil || tp.GetText() != "generated code here" {
		t.Errorf("artifact text = %q, want %q", tp.GetText(), "generated code here")
	}

	// Verify cost report.
	if cost == nil {
		t.Fatal("cost should not be nil")
	}
	if cost.GetTokensIn() != 1500 {
		t.Errorf("TokensIn = %d, want 1500", cost.GetTokensIn())
	}
	if cost.GetTokensOut() != 3000 {
		t.Errorf("TokensOut = %d, want 3000", cost.GetTokensOut())
	}
	if cost.GetCostUsd() != 0.045 {
		t.Errorf("CostUsd = %f, want 0.045", cost.GetCostUsd())
	}
	if !cost.GetEstimated() {
		t.Error("Estimated should be true")
	}
}

func TestSendTaskRequestToTaskRecord(t *testing.T) {
	req := &opcpb.SendTaskRequest{
		TaskId:    "task-req-1",
		AgentName: "agent-2",
		Message: &a2apb.Message{
			Role: "user",
			Parts: []*a2apb.Part{
				{Part: &a2apb.Part_TextPart{TextPart: &a2apb.TextPart{Text: "do the thing"}}},
			},
		},
		Metadata: map[string]string{
			"goalId":    "g-10",
			"projectId": "p-20",
			"issueId":   "i-30",
		},
	}

	rec := SendTaskRequestToTaskRecord(req)

	if rec.ID != "task-req-1" {
		t.Errorf("ID = %q, want %q", rec.ID, "task-req-1")
	}
	if rec.AgentName != "agent-2" {
		t.Errorf("AgentName = %q, want %q", rec.AgentName, "agent-2")
	}
	if rec.Message != "do the thing" {
		t.Errorf("Message = %q, want %q", rec.Message, "do the thing")
	}
	if rec.Status != v1.TaskStatusPending {
		t.Errorf("Status = %q, want %q", rec.Status, v1.TaskStatusPending)
	}
	if rec.GoalID != "g-10" {
		t.Errorf("GoalID = %q, want %q", rec.GoalID, "g-10")
	}
	if rec.ProjectID != "p-20" {
		t.Errorf("ProjectID = %q, want %q", rec.ProjectID, "p-20")
	}
	if rec.IssueID != "i-30" {
		t.Errorf("IssueID = %q, want %q", rec.IssueID, "i-30")
	}

	// Test nil request.
	recNil := SendTaskRequestToTaskRecord(nil)
	if recNil.ID != "" {
		t.Errorf("nil req should produce empty TaskRecord")
	}
}

func TestTaskRecordToSendTaskRequest(t *testing.T) {
	rec := v1.TaskRecord{
		ID:        "task-out-1",
		AgentName: "agent-3",
		Message:   "build module",
		GoalID:    "g-100",
		ProjectID: "p-200",
		IssueID:   "i-300",
	}

	req := TaskRecordToSendTaskRequest(rec)

	if req.GetTaskId() != "task-out-1" {
		t.Errorf("TaskId = %q, want %q", req.GetTaskId(), "task-out-1")
	}
	if req.GetAgentName() != "agent-3" {
		t.Errorf("AgentName = %q, want %q", req.GetAgentName(), "agent-3")
	}

	// Verify message.
	msg := req.GetMessage()
	if msg == nil {
		t.Fatal("message should not be nil")
	}
	if msg.GetRole() != "user" {
		t.Errorf("Role = %q, want %q", msg.GetRole(), "user")
	}
	msgParts := msg.GetParts()
	if len(msgParts) != 1 {
		t.Fatalf("len(parts) = %d, want 1", len(msgParts))
	}
	tp := msgParts[0].GetTextPart()
	if tp == nil || tp.GetText() != "build module" {
		t.Errorf("message text = %q, want %q", tp.GetText(), "build module")
	}

	// Verify metadata.
	meta := req.GetMetadata()
	if meta["goalId"] != "g-100" {
		t.Errorf("metadata[goalId] = %q, want %q", meta["goalId"], "g-100")
	}
	if meta["projectId"] != "p-200" {
		t.Errorf("metadata[projectId] = %q, want %q", meta["projectId"], "p-200")
	}
	if meta["issueId"] != "i-300" {
		t.Errorf("metadata[issueId] = %q, want %q", meta["issueId"], "i-300")
	}
}
