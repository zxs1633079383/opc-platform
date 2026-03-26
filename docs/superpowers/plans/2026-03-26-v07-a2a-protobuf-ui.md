# OPC Platform v0.7 — A2A Protobuf + UI Overhaul Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace all OPC internal communication with Google A2A protocol + protobuf, add no-code Agent wizard, polish Dashboard UI, and bring test coverage to 80%+.

**Architecture:** A2A protobuf as transport protocol layer with Bridge pattern preserving existing adapters. gRPC on :9528 for Agent/Federation communication, REST on :9527 for Dashboard/CLI. OPC keeps its own concept model (Goal/Project/Task/Issue) with bidirectional A2A mapping.

**Tech Stack:** Go 1.22+, google.golang.org/protobuf, google.golang.org/grpc, Next.js 14, React, TanStack Query, recharts, @xyflow/react, framer-motion, Tailwind CSS

**Design Spec:** `docs/superpowers/specs/2026-03-26-v07-a2a-protobuf-ui-design.md`

**Important:** Execute tasks strictly serially. Do NOT use parallel agents for code changes — they will overwrite each other's files.

---

## File Map

### New Files — Proto & A2A Layer

| File | Responsibility |
|------|---------------|
| `proto/a2a/a2a.proto` | Google A2A core types: AgentCard, Task, Message, Part, Artifact, TaskState |
| `proto/opc/types.proto` | OPC extension types: CostReport, CostConstraints, ResourceUsage |
| `proto/opc/agent_service.proto` | AgentService gRPC: SendTask, GetTask, CancelTask, Start, Stop, Health, GetAgentCard |
| `proto/opc/federation_service.proto` | FederationService gRPC: DispatchProject, ReportTaskResult, HeartbeatStream, AssessResult |
| `pkg/a2a/convert.go` | Bidirectional OPC Model ↔ A2A Proto conversion functions |
| `pkg/a2a/convert_test.go` | Tests for all conversion functions |
| `pkg/a2a/agentcard.go` | AgentSpec → AgentCard conversion + storage |
| `pkg/a2a/agentcard_test.go` | AgentCard conversion tests |
| `pkg/a2a/bridge.go` | Bridge: routes gRPC SendTask to native Adapter (openclaw/claude/custom) |
| `pkg/a2a/bridge_test.go` | Bridge tests with mock adapters |
| `pkg/a2a/server.go` | AgentServiceServer gRPC implementation |
| `pkg/a2a/server_test.go` | gRPC server tests |
| `pkg/a2a/client.go` | A2AClient implements adapter.Adapter over gRPC |
| `pkg/a2a/client_test.go` | gRPC client tests |
| `pkg/a2a/interceptor.go` | HMAC gRPC unary + stream interceptors |
| `pkg/a2a/interceptor_test.go` | Interceptor tests |
| `pkg/federation/grpc_client.go` | Federation gRPC client (Master → Worker dispatch) |
| `pkg/federation/grpc_server.go` | FederationServiceServer implementation |
| `pkg/federation/grpc_client_test.go` | Federation client tests |
| `pkg/federation/grpc_server_test.go` | Federation server tests |

### New Files — Backend API Extensions

| File | Responsibility |
|------|---------------|
| `pkg/model/registry.go` | ModelInfo registry: built-in models + custom model CRUD |
| `pkg/model/registry_test.go` | Model registry tests |
| `pkg/evolve/metrics.go` | MetricsCollector: SuccessRate, AvgLatency, RetryRate, CostPerGoal |
| `pkg/evolve/rfc.go` | RFC data structure + storage interface |
| `pkg/evolve/metrics_test.go` | MetricsCollector tests |

### New Files — Dashboard Frontend

| File | Responsibility |
|------|---------------|
| `dashboard/src/app/goals/[id]/page.tsx` | Goal detail page with tree visualization |
| `dashboard/src/app/rfcs/page.tsx` | RFC list + approve/reject |
| `dashboard/src/app/metrics/page.tsx` | System metrics dashboard |
| `dashboard/src/app/workflows/[name]/runs/[id]/page.tsx` | Workflow run detail |
| `dashboard/src/components/GoalTree.tsx` | Goal → Project → Task → Issue collapsible tree |
| `dashboard/src/components/DAGVisualization.tsx` | DAG dependency graph using @xyflow/react |
| `dashboard/src/components/RFCCard.tsx` | RFC card with approve/reject buttons |
| `dashboard/src/components/MetricsChart.tsx` | Time-series metrics chart using recharts |
| `dashboard/src/components/WorkflowRunDetail.tsx` | Step-by-step workflow run view |
| `dashboard/src/components/NodeStatusBadge.tsx` | Federation node status indicator |
| `dashboard/src/components/AgentCardView.tsx` | A2A AgentCard display component |
| `dashboard/src/components/BudgetProgress.tsx` | Budget consumption progress bar |
| `dashboard/src/components/Skeleton.tsx` | Loading skeleton component |
| `dashboard/src/components/EmptyState.tsx` | Empty state placeholder |
| `dashboard/src/components/ThemeToggle.tsx` | Dark/light mode toggle |
| `dashboard/src/components/AgentWizard/AgentWizard.tsx` | Main wizard container (step state machine) |
| `dashboard/src/components/AgentWizard/StepTypeSelect.tsx` | Step 1: Agent type cards |
| `dashboard/src/components/AgentWizard/StepDescribe.tsx` | Step 2: Description + model selection |
| `dashboard/src/components/AgentWizard/StepResources.tsx` | Step 3: Budget presets + sliders |
| `dashboard/src/components/AgentWizard/StepConfirm.tsx` | Step 4: Preview + create |
| `dashboard/src/components/AgentWizard/YAMLPreview.tsx` | YAML/AgentCard collapsible preview |

### Modified Files

| File | Changes |
|------|---------|
| `go.mod` | Add google.golang.org/grpc, google.golang.org/protobuf (already indirect, promote to direct) |
| `api/v1/types.go` | Add `Transport` field to ProtocolConfig |
| `pkg/adapter/adapter.go` | No changes (interface preserved) |
| `pkg/server/server.go` | Add gRPC server startup on :9528, model/wizard/RFC/metrics API endpoints |
| `pkg/controller/controller.go` | Add gRPC-aware routing (local adapter vs A2AClient) |
| `pkg/federation/federation.go` | Add `SupportsGRPC()` check, dual-protocol dispatch |
| `dashboard/src/lib/api.ts` | Add model, wizard, RFC, metrics, goal-detail API functions |
| `dashboard/src/types/index.ts` | Add ModelInfo, RFC, SystemMetrics, WorkflowRunDetail types |
| `dashboard/src/components/Sidebar.tsx` | Add Metrics, RFCs nav items |
| `dashboard/src/app/goals/page.tsx` | Add link to goal detail page |
| `dashboard/src/app/federation/page.tsx` | Enhance with gRPC status, node badges |
| `dashboard/src/components/AddAgentModal.tsx` | Replace with AgentWizard |
| `dashboard/package.json` | Add @xyflow/react, recharts, framer-motion |

---

## Phase A: Proto Definitions + A2A Mapping Layer

### Task 1: Define A2A Core Proto

**Files:**
- Create: `proto/a2a/a2a.proto`

- [ ] **Step 1: Create proto directory and a2a.proto**

```protobuf
// proto/a2a/a2a.proto
syntax = "proto3";

package a2a;

option go_package = "github.com/zlc-ai/opc-platform/gen/a2a";

import "google/protobuf/timestamp.proto";
import "google/protobuf/struct.proto";

// TaskState mirrors Google A2A task lifecycle.
enum TaskState {
  TASK_STATE_UNSPECIFIED = 0;
  TASK_STATE_SUBMITTED = 1;
  TASK_STATE_WORKING = 2;
  TASK_STATE_INPUT_REQUIRED = 3;
  TASK_STATE_COMPLETED = 4;
  TASK_STATE_FAILED = 5;
  TASK_STATE_CANCELED = 6;
}

// Part is a content unit within a Message.
message Part {
  oneof part {
    TextPart text = 1;
    FilePart file = 2;
    DataPart data = 3;
  }
}

message TextPart {
  string text = 1;
}

message FilePart {
  string name = 1;
  string mime_type = 2;
  oneof content {
    bytes bytes = 3;
    string uri = 4;
  }
}

message DataPart {
  string mime_type = 1;
  google.protobuf.Struct data = 2;
}

// Message is a conversation turn.
message Message {
  string role = 1;  // "user" or "agent"
  repeated Part parts = 2;
  google.protobuf.Timestamp timestamp = 3;
}

// Artifact is an output produced by an agent.
message Artifact {
  string id = 1;
  string name = 2;
  repeated Part parts = 3;
  map<string, string> metadata = 4;
}

// TaskStatus holds current state + messages for a task.
message TaskStatus {
  TaskState state = 1;
  string reason = 2;
  google.protobuf.Timestamp timestamp = 3;
}

// Task is the A2A task object.
message Task {
  string id = 1;
  string session_id = 2;
  TaskStatus status = 3;
  repeated Message messages = 4;
  repeated Artifact artifacts = 5;
  map<string, string> metadata = 6;
}

// AgentSkill describes a capability of an agent.
message AgentSkill {
  string id = 1;
  string name = 2;
  string description = 3;
  repeated string tags = 4;
  repeated string examples = 5;
}

// AgentCard describes an agent's capabilities (Google A2A discovery).
message AgentCard {
  string name = 1;
  string description = 2;
  string url = 3;
  string version = 4;
  string provider = 5;
  repeated AgentSkill skills = 6;
  repeated string input_modes = 7;   // e.g., ["text"]
  repeated string output_modes = 8;  // e.g., ["text"]
  map<string, string> metadata = 9;
}
```

- [ ] **Step 2: Verify proto syntax**

Run: `protoc --proto_path=proto proto/a2a/a2a.proto --go_out=. --go_opt=paths=source_relative 2>&1 || echo "protoc not installed yet — will install in next step"`

Expected: Either clean output or "protoc not installed" (both acceptable at this stage)

### Task 2: Define OPC Service Protos

**Files:**
- Create: `proto/opc/types.proto`
- Create: `proto/opc/agent_service.proto`
- Create: `proto/opc/federation_service.proto`

- [ ] **Step 1: Create opc/types.proto**

```protobuf
// proto/opc/types.proto
syntax = "proto3";

package opc;

option go_package = "github.com/zlc-ai/opc-platform/gen/opc";

// CostReport holds token/cost statistics for a completed task.
message CostReport {
  int64 tokens_in = 1;
  int64 tokens_out = 2;
  double cost_usd = 3;
  int64 duration_ms = 4;
  bool estimated = 5;
}

// CostConstraints defines budget limits for dispatched work.
message CostConstraints {
  int64 max_tokens = 1;
  double max_cost_usd = 2;
  string on_exceed = 3;  // "pause" | "alert" | "reject"
}

// ResourceUsage reports node resource consumption.
message ResourceUsage {
  double cpu_percent = 1;
  double memory_percent = 2;
  int32 active_agents = 3;
  int32 running_tasks = 4;
}

// AssessmentResult is the quality judgment on a task result.
message AssessmentResult {
  bool satisfied = 1;
  string reason = 2;
  string follow_up = 3;
  string category = 4;  // "satisfied" | "empty_result" | "execution_error" | "quality_issue"
}

// PendingDispatch is a task the master sends to a worker via heartbeat.
message PendingDispatch {
  string goal_id = 1;
  string project_name = 2;
  string agent_name = 3;
  string message = 4;
}
```

- [ ] **Step 2: Create opc/agent_service.proto**

```protobuf
// proto/opc/agent_service.proto
syntax = "proto3";

package opc;

option go_package = "github.com/zlc-ai/opc-platform/gen/opc";

import "a2a/a2a.proto";
import "opc/types.proto";

// AgentService handles Master ↔ Agent communication.
service AgentService {
  // Lifecycle
  rpc Start(StartRequest) returns (StartResponse);
  rpc Stop(StopRequest) returns (StopResponse);
  rpc Health(HealthRequest) returns (HealthResponse);

  // Task execution (A2A semantics)
  rpc SendTask(SendTaskRequest) returns (SendTaskResponse);
  rpc SendTaskStreaming(SendTaskStreamingRequest) returns (stream TaskStatusUpdate);

  // A2A standard: task queries
  rpc GetTask(GetTaskRequest) returns (a2a.Task);
  rpc CancelTask(CancelTaskRequest) returns (a2a.Task);

  // Agent discovery
  rpc GetAgentCard(GetAgentCardRequest) returns (a2a.AgentCard);
}

message StartRequest {
  string agent_name = 1;
  string spec_yaml = 2;
}

message StartResponse {
  bool success = 1;
  string message = 2;
}

message StopRequest {
  string agent_name = 1;
}

message StopResponse {
  bool success = 1;
}

message HealthRequest {
  string agent_name = 1;
}

message HealthResponse {
  bool healthy = 1;
  string message = 2;
}

message SendTaskRequest {
  string task_id = 1;
  string agent_name = 2;
  a2a.Message message = 3;
  string session_id = 4;
  map<string, string> metadata = 5;  // goalId, projectId, issueId etc.
}

message SendTaskResponse {
  a2a.Task task = 1;
  CostReport cost = 2;
}

message SendTaskStreamingRequest {
  string task_id = 1;
  string agent_name = 2;
  a2a.Message message = 3;
  string session_id = 4;
}

message TaskStatusUpdate {
  a2a.TaskStatus status = 1;
  a2a.Artifact artifact = 2;  // partial artifact for streaming
  bool final = 3;
  CostReport cost = 4;  // only set on final=true
}

message GetTaskRequest {
  string task_id = 1;
}

message CancelTaskRequest {
  string task_id = 1;
}

message GetAgentCardRequest {
  string agent_name = 1;
}
```

- [ ] **Step 3: Create opc/federation_service.proto**

```protobuf
// proto/opc/federation_service.proto
syntax = "proto3";

package opc;

option go_package = "github.com/zlc-ai/opc-platform/gen/opc";

import "a2a/a2a.proto";
import "opc/types.proto";

// FederationService handles Master ↔ Master communication.
service FederationService {
  // Node registration
  rpc Register(RegisterRequest) returns (RegisterResponse);

  // Bidirectional heartbeat stream
  rpc HeartbeatStream(stream HeartbeatPing) returns (stream HeartbeatPong);

  // Goal dispatch (Master → Worker)
  rpc DispatchProject(DispatchProjectRequest) returns (DispatchProjectResponse);

  // Result callback (Worker → Master)
  rpc ReportTaskResult(ReportTaskResultRequest) returns (ReportTaskResultResponse);

  // Cross-node A2A assessment
  rpc AssessResult(AssessRequest) returns (AssessResponse);

  // Status query
  rpc GetFederationStatus(GetFederationStatusRequest) returns (FederationStatusResponse);
}

message RegisterRequest {
  string node_id = 1;
  string company = 2;
  string endpoint = 3;
  repeated string available_agents = 4;
  string api_key = 5;
}

message RegisterResponse {
  bool accepted = 1;
  string assigned_id = 2;
  string message = 3;
}

message HeartbeatPing {
  string node_id = 1;
  string company = 2;
  repeated string available_agents = 3;
  ResourceUsage resources = 4;
  int64 timestamp = 5;
}

message HeartbeatPong {
  bool accepted = 1;
  repeated PendingDispatch pending = 2;
}

message DispatchProjectRequest {
  string goal_id = 1;
  string project_name = 2;
  string agent_name = 3;
  string company = 4;
  a2a.Message task_message = 5;
  repeated string dependencies = 6;
  string trace_context = 7;
  CostConstraints cost_constraints = 8;
}

message DispatchProjectResponse {
  bool accepted = 1;
  string task_id = 2;
  string message = 3;
}

message ReportTaskResultRequest {
  string goal_id = 1;
  string project_name = 2;
  a2a.Task completed_task = 3;
  CostReport cost = 4;
  AssessmentResult assessment = 5;
}

message ReportTaskResultResponse {
  bool acknowledged = 1;
  string next_action = 2;  // "none" | "retry" | "escalate"
}

message AssessRequest {
  string goal_name = 1;
  string project_description = 2;
  string result = 3;
}

message AssessResponse {
  AssessmentResult assessment = 1;
}

message GetFederationStatusRequest {}

message FederationStatusResponse {
  repeated NodeStatus nodes = 1;
  int32 total_agents = 2;
  int32 active_goals = 3;
}

message NodeStatus {
  string node_id = 1;
  string company = 2;
  string status = 3;  // "online" | "offline" | "busy"
  int64 last_heartbeat = 4;
  repeated string agents = 5;
  ResourceUsage resources = 6;
}
```

- [ ] **Step 4: Commit protos**

```bash
git add proto/
git commit -m "feat(proto): define A2A core types and OPC gRPC service protos"
```

### Task 3: Install protoc and Generate Go Code

**Files:**
- Create: `Makefile` (or add targets to existing)
- Create: `gen/a2a/*.go` (generated)
- Create: `gen/opc/*.go` (generated)

- [ ] **Step 1: Install protoc tools**

Run: `go install google.golang.org/protobuf/cmd/protoc-gen-go@latest && go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest`

- [ ] **Step 2: Create Makefile with proto target**

```makefile
# Add to Makefile (or create if not exists)
.PHONY: proto
proto:
	@mkdir -p gen/a2a gen/opc
	protoc \
		--proto_path=proto \
		--go_out=gen --go_opt=paths=source_relative \
		--go-grpc_out=gen --go-grpc_opt=paths=source_relative \
		proto/a2a/a2a.proto \
		proto/opc/types.proto \
		proto/opc/agent_service.proto \
		proto/opc/federation_service.proto
```

- [ ] **Step 3: Generate code**

Run: `make proto`

Expected: `gen/a2a/a2a.pb.go`, `gen/a2a/a2a_grpc.pb.go`, `gen/opc/*.pb.go`, `gen/opc/*_grpc.pb.go` created

- [ ] **Step 4: Promote grpc/protobuf to direct dependencies**

Run: `go mod tidy`

Verify: `google.golang.org/grpc` and `google.golang.org/protobuf` move from indirect to direct in `go.mod`

- [ ] **Step 5: Commit generated code**

```bash
git add Makefile gen/ go.mod go.sum
git commit -m "build: add protoc generation and gRPC Go code"
```

### Task 4: A2A Convert Layer

**Files:**
- Create: `pkg/a2a/convert.go`
- Create: `pkg/a2a/convert_test.go`

- [ ] **Step 1: Write convert tests**

```go
// pkg/a2a/convert_test.go
package a2a

import (
	"testing"
	"time"

	v1 "github.com/zlc-ai/opc-platform/api/v1"
	a2apb "github.com/zlc-ai/opc-platform/gen/a2a"
	opcpb "github.com/zlc-ai/opc-platform/gen/opc"
	"github.com/zlc-ai/opc-platform/pkg/adapter"
)

func TestTaskRecordToA2ATask(t *testing.T) {
	now := time.Now()
	rec := v1.TaskRecord{
		ID:        "task-1",
		AgentName: "coder",
		Message:   "write tests",
		Status:    v1.TaskStatusRunning,
		Result:    "done",
		TokensIn:  100,
		TokensOut: 200,
		CreatedAt: now,
		UpdatedAt: now,
	}

	task := TaskRecordToA2ATask(rec)

	if task.Id != "task-1" {
		t.Errorf("expected id task-1, got %s", task.Id)
	}
	if task.Status.State != a2apb.TaskState_TASK_STATE_WORKING {
		t.Errorf("expected WORKING, got %v", task.Status.State)
	}
	if len(task.Messages) != 1 {
		t.Fatalf("expected 1 message, got %d", len(task.Messages))
	}
	if task.Messages[0].Role != "user" {
		t.Errorf("expected role user, got %s", task.Messages[0].Role)
	}
	if task.Messages[0].Parts[0].GetText().Text != "write tests" {
		t.Errorf("unexpected message text")
	}
}

func TestA2ATaskToTaskRecord(t *testing.T) {
	task := &a2apb.Task{
		Id: "task-2",
		Status: &a2apb.TaskStatus{
			State: a2apb.TaskState_TASK_STATE_COMPLETED,
		},
		Messages: []*a2apb.Message{
			{Role: "user", Parts: []*a2apb.Part{{Part: &a2apb.Part_Text{Text: &a2apb.TextPart{Text: "hello"}}}}},
		},
		Artifacts: []*a2apb.Artifact{
			{Parts: []*a2apb.Part{{Part: &a2apb.Part_Text{Text: &a2apb.TextPart{Text: "result text"}}}}},
		},
	}

	rec := A2ATaskToTaskRecord(task, "agent-x")

	if rec.ID != "task-2" {
		t.Errorf("expected id task-2, got %s", rec.ID)
	}
	if rec.Status != v1.TaskStatusCompleted {
		t.Errorf("expected Completed, got %s", rec.Status)
	}
	if rec.Message != "hello" {
		t.Errorf("expected message hello, got %s", rec.Message)
	}
	if rec.Result != "result text" {
		t.Errorf("expected result text, got %s", rec.Result)
	}
}

func TestTaskStateMapping(t *testing.T) {
	tests := []struct {
		opcStatus v1.TaskStatus
		a2aState  a2apb.TaskState
	}{
		{v1.TaskStatusPending, a2apb.TaskState_TASK_STATE_SUBMITTED},
		{v1.TaskStatusRunning, a2apb.TaskState_TASK_STATE_WORKING},
		{v1.TaskStatusCompleted, a2apb.TaskState_TASK_STATE_COMPLETED},
		{v1.TaskStatusFailed, a2apb.TaskState_TASK_STATE_FAILED},
		{v1.TaskStatusCancelled, a2apb.TaskState_TASK_STATE_CANCELED},
	}

	for _, tt := range tests {
		got := OPCStatusToA2AState(tt.opcStatus)
		if got != tt.a2aState {
			t.Errorf("OPCStatusToA2AState(%s) = %v, want %v", tt.opcStatus, got, tt.a2aState)
		}
		back := A2AStateToOPCStatus(tt.a2aState)
		if back != tt.opcStatus {
			t.Errorf("A2AStateToOPCStatus(%v) = %s, want %s", tt.a2aState, back, tt.opcStatus)
		}
	}
}

func TestExecuteResultToArtifact(t *testing.T) {
	res := adapter.ExecuteResult{
		Output:    "code output",
		TokensIn:  500,
		TokensOut: 300,
		Cost:      0.05,
		Estimated: true,
	}

	art, cost := ExecuteResultToArtifact(res)

	if art.Parts[0].GetText().Text != "code output" {
		t.Errorf("unexpected artifact text")
	}
	if cost.TokensIn != 500 || cost.TokensOut != 300 {
		t.Errorf("unexpected cost tokens")
	}
	if !cost.Estimated {
		t.Error("expected estimated=true")
	}
}

func TestSendTaskRequestToTaskRecord(t *testing.T) {
	req := &opcpb.SendTaskRequest{
		TaskId:    "t-1",
		AgentName: "reviewer",
		Message: &a2apb.Message{
			Role: "user",
			Parts: []*a2apb.Part{
				{Part: &a2apb.Part_Text{Text: &a2apb.TextPart{Text: "review this"}}},
			},
		},
		Metadata: map[string]string{
			"goalId":    "g-1",
			"projectId": "p-1",
		},
	}

	rec := SendTaskRequestToTaskRecord(req)

	if rec.ID != "t-1" {
		t.Errorf("expected id t-1, got %s", rec.ID)
	}
	if rec.AgentName != "reviewer" {
		t.Errorf("expected agent reviewer, got %s", rec.AgentName)
	}
	if rec.Message != "review this" {
		t.Errorf("expected message 'review this', got %s", rec.Message)
	}
	if rec.GoalID != "g-1" {
		t.Errorf("expected goalId g-1, got %s", rec.GoalID)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd /Users/mac28/workspace/java/zlc_ai/opc_platform && go test ./pkg/a2a/ -v -run TestTaskRecordToA2ATask`

Expected: FAIL — package/functions not defined

- [ ] **Step 3: Implement convert.go**

```go
// pkg/a2a/convert.go
package a2a

import (
	"strings"
	"time"

	v1 "github.com/zlc-ai/opc-platform/api/v1"
	a2apb "github.com/zlc-ai/opc-platform/gen/a2a"
	opcpb "github.com/zlc-ai/opc-platform/gen/opc"
	"github.com/zlc-ai/opc-platform/pkg/adapter"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// OPCStatusToA2AState maps OPC TaskStatus to A2A TaskState.
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

// A2AStateToOPCStatus maps A2A TaskState to OPC TaskStatus.
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

// TaskRecordToA2ATask converts an OPC TaskRecord to an A2A Task proto.
func TaskRecordToA2ATask(rec v1.TaskRecord) *a2apb.Task {
	task := &a2apb.Task{
		Id: rec.ID,
		Status: &a2apb.TaskStatus{
			State:     OPCStatusToA2AState(rec.Status),
			Timestamp: timestamppb.New(rec.UpdatedAt),
		},
		Metadata: map[string]string{
			"agentName": rec.AgentName,
		},
	}

	// Add user message.
	if rec.Message != "" {
		task.Messages = append(task.Messages, &a2apb.Message{
			Role:      "user",
			Parts:     []*a2apb.Part{{Part: &a2apb.Part_Text{Text: &a2apb.TextPart{Text: rec.Message}}}},
			Timestamp: timestamppb.New(rec.CreatedAt),
		})
	}

	// Add result as artifact.
	if rec.Result != "" {
		task.Artifacts = append(task.Artifacts, &a2apb.Artifact{
			Id:    rec.ID + "-result",
			Parts: []*a2apb.Part{{Part: &a2apb.Part_Text{Text: &a2apb.TextPart{Text: rec.Result}}}},
		})
	}

	if rec.GoalID != "" {
		task.Metadata["goalId"] = rec.GoalID
	}
	if rec.ProjectID != "" {
		task.Metadata["projectId"] = rec.ProjectID
	}

	return task
}

// A2ATaskToTaskRecord converts an A2A Task proto to an OPC TaskRecord.
func A2ATaskToTaskRecord(task *a2apb.Task, agentName string) v1.TaskRecord {
	rec := v1.TaskRecord{
		ID:        task.Id,
		AgentName: agentName,
		Status:    A2AStateToOPCStatus(task.Status.GetState()),
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	// Extract first user message.
	for _, msg := range task.Messages {
		if msg.Role == "user" && len(msg.Parts) > 0 {
			if tp := msg.Parts[0].GetText(); tp != nil {
				rec.Message = tp.Text
				break
			}
		}
	}

	// Extract result from first artifact.
	if len(task.Artifacts) > 0 {
		var parts []string
		for _, p := range task.Artifacts[0].Parts {
			if tp := p.GetText(); tp != nil {
				parts = append(parts, tp.Text)
			}
		}
		rec.Result = strings.Join(parts, "\n")
	}

	// Extract metadata.
	if task.Metadata != nil {
		rec.GoalID = task.Metadata["goalId"]
		rec.ProjectID = task.Metadata["projectId"]
		rec.IssueID = task.Metadata["issueId"]
	}

	return rec
}

// ExecuteResultToArtifact converts an adapter.ExecuteResult to an A2A Artifact + CostReport.
func ExecuteResultToArtifact(res adapter.ExecuteResult) (*a2apb.Artifact, *opcpb.CostReport) {
	art := &a2apb.Artifact{
		Parts: []*a2apb.Part{
			{Part: &a2apb.Part_Text{Text: &a2apb.TextPart{Text: res.Output}}},
		},
	}

	cost := &opcpb.CostReport{
		TokensIn:  int64(res.TokensIn),
		TokensOut: int64(res.TokensOut),
		CostUsd:   res.Cost,
		Estimated: res.Estimated,
	}

	return art, cost
}

// SendTaskRequestToTaskRecord converts a gRPC SendTaskRequest to an OPC TaskRecord.
func SendTaskRequestToTaskRecord(req *opcpb.SendTaskRequest) v1.TaskRecord {
	rec := v1.TaskRecord{
		ID:        req.TaskId,
		AgentName: req.AgentName,
		Status:    v1.TaskStatusPending,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	// Extract text from message.
	if req.Message != nil && len(req.Message.Parts) > 0 {
		if tp := req.Message.Parts[0].GetText(); tp != nil {
			rec.Message = tp.Text
		}
	}

	// Extract metadata.
	if req.Metadata != nil {
		rec.GoalID = req.Metadata["goalId"]
		rec.ProjectID = req.Metadata["projectId"]
		rec.IssueID = req.Metadata["issueId"]
	}

	return rec
}

// TaskRecordToSendTaskRequest converts an OPC TaskRecord to a gRPC SendTaskRequest.
func TaskRecordToSendTaskRequest(rec v1.TaskRecord) *opcpb.SendTaskRequest {
	return &opcpb.SendTaskRequest{
		TaskId:    rec.ID,
		AgentName: rec.AgentName,
		Message: &a2apb.Message{
			Role:      "user",
			Parts:     []*a2apb.Part{{Part: &a2apb.Part_Text{Text: &a2apb.TextPart{Text: rec.Message}}}},
			Timestamp: timestamppb.New(rec.CreatedAt),
		},
		Metadata: map[string]string{
			"goalId":    rec.GoalID,
			"projectId": rec.ProjectID,
			"issueId":   rec.IssueID,
		},
	}
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd /Users/mac28/workspace/java/zlc_ai/opc_platform && go test ./pkg/a2a/ -v`

Expected: All tests PASS

- [ ] **Step 5: Commit**

```bash
git add pkg/a2a/convert.go pkg/a2a/convert_test.go
git commit -m "feat(a2a): implement OPC ↔ A2A proto bidirectional conversion layer"
```

### Task 5: AgentCard Conversion

**Files:**
- Create: `pkg/a2a/agentcard.go`
- Create: `pkg/a2a/agentcard_test.go`

- [ ] **Step 1: Write AgentCard tests**

```go
// pkg/a2a/agentcard_test.go
package a2a

import (
	"testing"

	v1 "github.com/zlc-ai/opc-platform/api/v1"
)

func TestAgentSpecToAgentCard(t *testing.T) {
	spec := v1.AgentSpec{
		APIVersion: "opc/v1",
		Kind:       "AgentSpec",
		Metadata:   v1.Metadata{Name: "code-reviewer"},
		Spec: v1.AgentSpecBody{
			Type:        v1.AgentTypeClaudeCode,
			Description: "Reviews code for bugs and style",
			Runtime: v1.RuntimeConfig{
				Model: v1.ModelConfig{
					Name:     "claude-sonnet-4-6",
					Fallback: "claude-haiku-4-5",
				},
			},
			Context: v1.ContextConfig{
				Skills: []string{"code-review", "testing"},
			},
		},
	}

	card := AgentSpecToAgentCard(spec, "http://localhost:9528")

	if card.Name != "code-reviewer" {
		t.Errorf("expected name code-reviewer, got %s", card.Name)
	}
	if card.Description != "Reviews code for bugs and style" {
		t.Errorf("unexpected description: %s", card.Description)
	}
	if card.Url != "http://localhost:9528" {
		t.Errorf("unexpected url: %s", card.Url)
	}
	if len(card.Skills) != 2 {
		t.Fatalf("expected 2 skills, got %d", len(card.Skills))
	}
	if card.Skills[0].Name != "code-review" {
		t.Errorf("expected skill code-review, got %s", card.Skills[0].Name)
	}
	if card.Metadata["agentType"] != "claude-code" {
		t.Errorf("expected agentType claude-code, got %s", card.Metadata["agentType"])
	}
	if card.Metadata["model"] != "claude-sonnet-4-6" {
		t.Errorf("expected model claude-sonnet-4-6, got %s", card.Metadata["model"])
	}
}

func TestAgentSpecToAgentCard_NoSkills(t *testing.T) {
	spec := v1.AgentSpec{
		Metadata: v1.Metadata{Name: "basic-agent"},
		Spec: v1.AgentSpecBody{
			Type: v1.AgentTypeCustom,
		},
	}

	card := AgentSpecToAgentCard(spec, "")

	if card.Name != "basic-agent" {
		t.Errorf("expected name basic-agent, got %s", card.Name)
	}
	if len(card.Skills) != 0 {
		t.Errorf("expected 0 skills, got %d", len(card.Skills))
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./pkg/a2a/ -v -run TestAgentSpec`

Expected: FAIL

- [ ] **Step 3: Implement agentcard.go**

```go
// pkg/a2a/agentcard.go
package a2a

import (
	v1 "github.com/zlc-ai/opc-platform/api/v1"
	a2apb "github.com/zlc-ai/opc-platform/gen/a2a"
)

// AgentSpecToAgentCard converts an OPC AgentSpec to a Google A2A AgentCard.
func AgentSpecToAgentCard(spec v1.AgentSpec, serverURL string) *a2apb.AgentCard {
	card := &a2apb.AgentCard{
		Name:        spec.Metadata.Name,
		Description: spec.Spec.Description,
		Url:         serverURL,
		Version:     spec.APIVersion,
		Provider:    "opc-platform",
		InputModes:  []string{"text"},
		OutputModes: []string{"text"},
		Metadata: map[string]string{
			"agentType": string(spec.Spec.Type),
			"model":     spec.Spec.Runtime.Model.Name,
		},
	}

	if spec.Spec.Runtime.Model.Fallback != "" {
		card.Metadata["fallbackModel"] = spec.Spec.Runtime.Model.Fallback
	}

	for _, skill := range spec.Spec.Context.Skills {
		card.Skills = append(card.Skills, &a2apb.AgentSkill{
			Id:   skill,
			Name: skill,
		})
	}

	return card
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./pkg/a2a/ -v`

Expected: All tests PASS

- [ ] **Step 5: Commit**

```bash
git add pkg/a2a/agentcard.go pkg/a2a/agentcard_test.go
git commit -m "feat(a2a): AgentSpec to A2A AgentCard conversion"
```

---

## Phase B: AgentService gRPC Implementation

### Task 6: A2A Bridge

**Files:**
- Create: `pkg/a2a/bridge.go`
- Create: `pkg/a2a/bridge_test.go`

- [ ] **Step 1: Write Bridge tests**

```go
// pkg/a2a/bridge_test.go
package a2a

import (
	"context"
	"testing"

	v1 "github.com/zlc-ai/opc-platform/api/v1"
	a2apb "github.com/zlc-ai/opc-platform/gen/a2a"
	opcpb "github.com/zlc-ai/opc-platform/gen/opc"
	"github.com/zlc-ai/opc-platform/pkg/adapter"
)

// mockAdapter implements adapter.Adapter for testing.
type mockAdapter struct {
	executeResult adapter.ExecuteResult
	executeErr    error
	started       bool
	stopped       bool
	healthy       bool
}

func (m *mockAdapter) Type() v1.AgentType                   { return v1.AgentTypeCustom }
func (m *mockAdapter) Start(_ context.Context, _ v1.AgentSpec) error { m.started = true; return nil }
func (m *mockAdapter) Stop(_ context.Context) error         { m.stopped = true; return nil }
func (m *mockAdapter) Health() v1.HealthStatus              { return v1.HealthStatus{Healthy: m.healthy} }
func (m *mockAdapter) Status() v1.AgentPhase                { return v1.AgentPhaseRunning }
func (m *mockAdapter) Metrics() v1.AgentMetrics             { return v1.AgentMetrics{} }
func (m *mockAdapter) Stream(_ context.Context, _ v1.TaskRecord) (<-chan adapter.Chunk, error) {
	return nil, nil
}
func (m *mockAdapter) Execute(_ context.Context, _ v1.TaskRecord) (adapter.ExecuteResult, error) {
	return m.executeResult, m.executeErr
}

func TestBridgeSendTask(t *testing.T) {
	mock := &mockAdapter{
		executeResult: adapter.ExecuteResult{
			Output:    "reviewed code: looks good",
			TokensIn:  100,
			TokensOut: 50,
			Cost:      0.01,
		},
	}

	bridge := NewBridge()
	bridge.RegisterAdapter("reviewer", mock)

	req := &opcpb.SendTaskRequest{
		TaskId:    "t-1",
		AgentName: "reviewer",
		Message: &a2apb.Message{
			Role:  "user",
			Parts: []*a2apb.Part{{Part: &a2apb.Part_Text{Text: &a2apb.TextPart{Text: "review PR #5"}}}},
		},
	}

	resp, err := bridge.SendTask(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if resp.Task.Status.State != a2apb.TaskState_TASK_STATE_COMPLETED {
		t.Errorf("expected COMPLETED, got %v", resp.Task.Status.State)
	}
	if resp.Cost.TokensIn != 100 {
		t.Errorf("expected 100 tokens in, got %d", resp.Cost.TokensIn)
	}
}

func TestBridgeSendTask_UnknownAgent(t *testing.T) {
	bridge := NewBridge()

	req := &opcpb.SendTaskRequest{
		TaskId:    "t-1",
		AgentName: "nonexistent",
	}

	_, err := bridge.SendTask(context.Background(), req)
	if err == nil {
		t.Fatal("expected error for unknown agent")
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./pkg/a2a/ -v -run TestBridge`

Expected: FAIL

- [ ] **Step 3: Implement bridge.go**

```go
// pkg/a2a/bridge.go
package a2a

import (
	"context"
	"fmt"
	"sync"

	a2apb "github.com/zlc-ai/opc-platform/gen/a2a"
	opcpb "github.com/zlc-ai/opc-platform/gen/opc"
	"github.com/zlc-ai/opc-platform/pkg/adapter"
)

// Bridge routes A2A gRPC requests to native OPC Adapters.
type Bridge struct {
	mu       sync.RWMutex
	adapters map[string]adapter.Adapter
}

// NewBridge creates a new Bridge.
func NewBridge() *Bridge {
	return &Bridge{
		adapters: make(map[string]adapter.Adapter),
	}
}

// RegisterAdapter registers a native adapter for an agent name.
func (b *Bridge) RegisterAdapter(agentName string, a adapter.Adapter) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.adapters[agentName] = a
}

// UnregisterAdapter removes an adapter.
func (b *Bridge) UnregisterAdapter(agentName string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	delete(b.adapters, agentName)
}

// SendTask routes a gRPC SendTask request to the appropriate native adapter.
func (b *Bridge) SendTask(ctx context.Context, req *opcpb.SendTaskRequest) (*opcpb.SendTaskResponse, error) {
	b.mu.RLock()
	a, ok := b.adapters[req.AgentName]
	b.mu.RUnlock()

	if !ok {
		return nil, fmt.Errorf("agent %q not registered in bridge", req.AgentName)
	}

	// Convert A2A request → OPC TaskRecord.
	task := SendTaskRequestToTaskRecord(req)

	// Execute via native adapter.
	result, err := a.Execute(ctx, task)
	if err != nil {
		// Return A2A Task with FAILED status.
		return &opcpb.SendTaskResponse{
			Task: &a2apb.Task{
				Id: req.TaskId,
				Status: &a2apb.TaskStatus{
					State:  a2apb.TaskState_TASK_STATE_FAILED,
					Reason: err.Error(),
				},
			},
		}, nil
	}

	// Convert result → A2A Artifact + CostReport.
	artifact, cost := ExecuteResultToArtifact(result)
	artifact.Id = req.TaskId + "-result"

	return &opcpb.SendTaskResponse{
		Task: &a2apb.Task{
			Id: req.TaskId,
			Status: &a2apb.TaskStatus{
				State: a2apb.TaskState_TASK_STATE_COMPLETED,
			},
			Messages:  []*a2apb.Message{req.Message},
			Artifacts: []*a2apb.Artifact{artifact},
		},
		Cost: cost,
	}, nil
}

// Health checks the health of a registered adapter.
func (b *Bridge) Health(agentName string) (*opcpb.HealthResponse, error) {
	b.mu.RLock()
	a, ok := b.adapters[agentName]
	b.mu.RUnlock()

	if !ok {
		return nil, fmt.Errorf("agent %q not registered", agentName)
	}

	hs := a.Health()
	return &opcpb.HealthResponse{
		Healthy: hs.Healthy,
		Message: hs.Message,
	}, nil
}
```

- [ ] **Step 4: Run tests**

Run: `go test ./pkg/a2a/ -v -run TestBridge`

Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add pkg/a2a/bridge.go pkg/a2a/bridge_test.go
git commit -m "feat(a2a): Bridge routes gRPC requests to native adapters"
```

### Task 7: gRPC AgentService Server

**Files:**
- Create: `pkg/a2a/server.go`
- Create: `pkg/a2a/server_test.go`

- [ ] **Step 1: Write server tests**

```go
// pkg/a2a/server_test.go
package a2a

import (
	"context"
	"net"
	"testing"

	a2apb "github.com/zlc-ai/opc-platform/gen/a2a"
	opcpb "github.com/zlc-ai/opc-platform/gen/opc"
	"github.com/zlc-ai/opc-platform/pkg/adapter"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/test/bufconn"
)

const bufSize = 1024 * 1024

func setupTestServer(t *testing.T, mock *mockAdapter) (opcpb.AgentServiceClient, func()) {
	t.Helper()

	bridge := NewBridge()
	bridge.RegisterAdapter("test-agent", mock)

	cards := map[string]*a2apb.AgentCard{
		"test-agent": {Name: "test-agent", Description: "test"},
	}

	srv := NewAgentServiceServer(bridge, cards)

	lis := bufconn.Listen(bufSize)
	s := grpc.NewServer()
	opcpb.RegisterAgentServiceServer(s, srv)

	go func() { _ = s.Serve(lis) }()

	conn, err := grpc.NewClient("passthrough:///bufnet",
		grpc.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) {
			return lis.DialContext(ctx)
		}),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}

	client := opcpb.NewAgentServiceClient(conn)

	return client, func() {
		conn.Close()
		s.Stop()
	}
}

func TestAgentServiceServer_SendTask(t *testing.T) {
	mock := &mockAdapter{
		executeResult: adapter.ExecuteResult{
			Output:    "task done",
			TokensIn:  200,
			TokensOut: 100,
		},
	}

	client, cleanup := setupTestServer(t, mock)
	defer cleanup()

	resp, err := client.SendTask(context.Background(), &opcpb.SendTaskRequest{
		TaskId:    "t-1",
		AgentName: "test-agent",
		Message: &a2apb.Message{
			Role:  "user",
			Parts: []*a2apb.Part{{Part: &a2apb.Part_Text{Text: &a2apb.TextPart{Text: "do it"}}}},
		},
	})

	if err != nil {
		t.Fatalf("SendTask error: %v", err)
	}
	if resp.Task.Status.State != a2apb.TaskState_TASK_STATE_COMPLETED {
		t.Errorf("expected COMPLETED, got %v", resp.Task.Status.State)
	}
	if resp.Cost.TokensIn != 200 {
		t.Errorf("expected 200 tokens in, got %d", resp.Cost.TokensIn)
	}
}

func TestAgentServiceServer_GetAgentCard(t *testing.T) {
	mock := &mockAdapter{}
	client, cleanup := setupTestServer(t, mock)
	defer cleanup()

	card, err := client.GetAgentCard(context.Background(), &opcpb.GetAgentCardRequest{
		AgentName: "test-agent",
	})

	if err != nil {
		t.Fatalf("GetAgentCard error: %v", err)
	}
	if card.Name != "test-agent" {
		t.Errorf("expected name test-agent, got %s", card.Name)
	}
}

func TestAgentServiceServer_Health(t *testing.T) {
	mock := &mockAdapter{healthy: true}
	client, cleanup := setupTestServer(t, mock)
	defer cleanup()

	resp, err := client.Health(context.Background(), &opcpb.HealthRequest{
		AgentName: "test-agent",
	})

	if err != nil {
		t.Fatalf("Health error: %v", err)
	}
	if !resp.Healthy {
		t.Error("expected healthy=true")
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./pkg/a2a/ -v -run TestAgentServiceServer`

Expected: FAIL

- [ ] **Step 3: Implement server.go**

```go
// pkg/a2a/server.go
package a2a

import (
	"context"
	"fmt"

	a2apb "github.com/zlc-ai/opc-platform/gen/a2a"
	opcpb "github.com/zlc-ai/opc-platform/gen/opc"
)

// AgentServiceServer implements the AgentService gRPC service.
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

func (s *AgentServiceServer) SendTask(ctx context.Context, req *opcpb.SendTaskRequest) (*opcpb.SendTaskResponse, error) {
	return s.bridge.SendTask(ctx, req)
}

func (s *AgentServiceServer) Health(ctx context.Context, req *opcpb.HealthRequest) (*opcpb.HealthResponse, error) {
	return s.bridge.Health(req.AgentName)
}

func (s *AgentServiceServer) GetAgentCard(_ context.Context, req *opcpb.GetAgentCardRequest) (*a2apb.AgentCard, error) {
	card, ok := s.cards[req.AgentName]
	if !ok {
		return nil, fmt.Errorf("agent card not found: %s", req.AgentName)
	}
	return card, nil
}

func (s *AgentServiceServer) Start(_ context.Context, req *opcpb.StartRequest) (*opcpb.StartResponse, error) {
	return &opcpb.StartResponse{Success: true, Message: "delegated to controller"}, nil
}

func (s *AgentServiceServer) Stop(_ context.Context, req *opcpb.StopRequest) (*opcpb.StopResponse, error) {
	return &opcpb.StopResponse{Success: true}, nil
}

func (s *AgentServiceServer) GetTask(_ context.Context, req *opcpb.GetTaskRequest) (*a2apb.Task, error) {
	return nil, fmt.Errorf("not implemented: use REST API for task queries")
}

func (s *AgentServiceServer) CancelTask(_ context.Context, req *opcpb.CancelTaskRequest) (*a2apb.Task, error) {
	return nil, fmt.Errorf("not implemented: use REST API for task cancellation")
}
```

- [ ] **Step 4: Run tests**

Run: `go test ./pkg/a2a/ -v`

Expected: All tests PASS

- [ ] **Step 5: Commit**

```bash
git add pkg/a2a/server.go pkg/a2a/server_test.go
git commit -m "feat(a2a): AgentService gRPC server implementation"
```

### Task 8: A2A gRPC Client

**Files:**
- Create: `pkg/a2a/client.go`
- Create: `pkg/a2a/client_test.go`

- [ ] **Step 1: Write client tests**

```go
// pkg/a2a/client_test.go
package a2a

import (
	"context"
	"testing"

	v1 "github.com/zlc-ai/opc-platform/api/v1"
	"github.com/zlc-ai/opc-platform/pkg/adapter"
)

func TestA2AClientExecute(t *testing.T) {
	// Spin up a test gRPC server with a mock adapter.
	mock := &mockAdapter{
		executeResult: adapter.ExecuteResult{
			Output:    "client test result",
			TokensIn:  300,
			TokensOut: 150,
			Cost:      0.03,
		},
	}

	client, cleanup := setupTestA2AClient(t, mock)
	defer cleanup()

	task := v1.TaskRecord{
		ID:        "ct-1",
		AgentName: "test-agent",
		Message:   "do something",
	}

	result, err := client.Execute(context.Background(), task)
	if err != nil {
		t.Fatalf("Execute error: %v", err)
	}

	if result.Output != "client test result" {
		t.Errorf("expected 'client test result', got %s", result.Output)
	}
	if result.TokensIn != 300 {
		t.Errorf("expected 300, got %d", result.TokensIn)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./pkg/a2a/ -v -run TestA2AClient`

Expected: FAIL

- [ ] **Step 3: Implement client.go**

```go
// pkg/a2a/client.go
package a2a

import (
	"context"
	"fmt"
	"net"
	"testing"

	v1 "github.com/zlc-ai/opc-platform/api/v1"
	a2apb "github.com/zlc-ai/opc-platform/gen/a2a"
	opcpb "github.com/zlc-ai/opc-platform/gen/opc"
	"github.com/zlc-ai/opc-platform/pkg/adapter"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/test/bufconn"
)

// A2AClient implements adapter.Adapter over gRPC.
type A2AClient struct {
	conn   *grpc.ClientConn
	client opcpb.AgentServiceClient
	agent  string
}

// NewA2AClient creates a new A2A gRPC client.
func NewA2AClient(target string, agentName string) (*A2AClient, error) {
	conn, err := grpc.NewClient(target,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		return nil, fmt.Errorf("grpc dial %s: %w", target, err)
	}

	return &A2AClient{
		conn:   conn,
		client: opcpb.NewAgentServiceClient(conn),
		agent:  agentName,
	}, nil
}

func (c *A2AClient) Type() v1.AgentType      { return "a2a" }
func (c *A2AClient) Status() v1.AgentPhase    { return v1.AgentPhaseRunning }
func (c *A2AClient) Metrics() v1.AgentMetrics { return v1.AgentMetrics{} }

func (c *A2AClient) Start(_ context.Context, _ v1.AgentSpec) error { return nil }

func (c *A2AClient) Stop(_ context.Context) error {
	return c.conn.Close()
}

func (c *A2AClient) Health() v1.HealthStatus {
	resp, err := c.client.Health(context.Background(), &opcpb.HealthRequest{AgentName: c.agent})
	if err != nil {
		return v1.HealthStatus{Healthy: false, Message: err.Error()}
	}
	return v1.HealthStatus{Healthy: resp.Healthy, Message: resp.Message}
}

func (c *A2AClient) Execute(ctx context.Context, task v1.TaskRecord) (adapter.ExecuteResult, error) {
	req := TaskRecordToSendTaskRequest(task)

	resp, err := c.client.SendTask(ctx, req)
	if err != nil {
		return adapter.ExecuteResult{}, fmt.Errorf("a2a SendTask: %w", err)
	}

	if resp.Task.Status.State == a2apb.TaskState_TASK_STATE_FAILED {
		return adapter.ExecuteResult{}, fmt.Errorf("task failed: %s", resp.Task.Status.Reason)
	}

	result := adapter.ExecuteResult{}

	// Extract output from artifacts.
	if len(resp.Task.Artifacts) > 0 {
		for _, p := range resp.Task.Artifacts[0].Parts {
			if tp := p.GetText(); tp != nil {
				result.Output += tp.Text
			}
		}
	}

	// Extract cost.
	if resp.Cost != nil {
		result.TokensIn = int(resp.Cost.TokensIn)
		result.TokensOut = int(resp.Cost.TokensOut)
		result.Cost = resp.Cost.CostUsd
		result.Estimated = resp.Cost.Estimated
	}

	return result, nil
}

func (c *A2AClient) Stream(_ context.Context, _ v1.TaskRecord) (<-chan adapter.Chunk, error) {
	return nil, fmt.Errorf("streaming not implemented for A2A client yet")
}

// Close closes the gRPC connection.
func (c *A2AClient) Close() error {
	return c.conn.Close()
}

// --- Test helper (only used in tests) ---

func setupTestA2AClient(t *testing.T, mock *mockAdapter) (*A2AClient, func()) {
	t.Helper()

	bridge := NewBridge()
	bridge.RegisterAdapter("test-agent", mock)

	cards := map[string]*a2apb.AgentCard{
		"test-agent": {Name: "test-agent"},
	}

	srv := NewAgentServiceServer(bridge, cards)

	lis := bufconn.Listen(bufSize)
	s := grpc.NewServer()
	opcpb.RegisterAgentServiceServer(s, srv)
	go func() { _ = s.Serve(lis) }()

	conn, err := grpc.NewClient("passthrough:///bufnet",
		grpc.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) {
			return lis.DialContext(ctx)
		}),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}

	client := &A2AClient{
		conn:   conn,
		client: opcpb.NewAgentServiceClient(conn),
		agent:  "test-agent",
	}

	return client, func() {
		conn.Close()
		s.Stop()
	}
}
```

- [ ] **Step 4: Run tests**

Run: `go test ./pkg/a2a/ -v`

Expected: All tests PASS

- [ ] **Step 5: Commit**

```bash
git add pkg/a2a/client.go pkg/a2a/client_test.go
git commit -m "feat(a2a): A2AClient implements adapter.Adapter over gRPC"
```

### Task 9: gRPC Server Startup in OPC Server

**Files:**
- Modify: `pkg/server/server.go`

- [ ] **Step 1: Add gRPC server startup to server.go**

Add a `startGRPC` method that:
1. Creates a `net.Listener` on `:9528`
2. Creates `grpc.NewServer()`
3. Instantiates `Bridge` and `AgentServiceServer`
4. Registers the service
5. Starts serving in a goroutine

Add to `Server` struct:
```go
grpcServer *grpc.Server
bridge     *Bridge
```

Add to `Start()` method:
```go
if err := s.startGRPC(); err != nil {
    return fmt.Errorf("start gRPC: %w", err)
}
```

Add `startGRPC()`:
```go
func (s *Server) startGRPC() error {
    lis, err := net.Listen("tcp", fmt.Sprintf("%s:%d", s.config.Host, 9528))
    if err != nil {
        return fmt.Errorf("listen :9528: %w", err)
    }

    s.bridge = a2a.NewBridge()
    cards := make(map[string]*a2apb.AgentCard)
    agentSrv := a2a.NewAgentServiceServer(s.bridge, cards)

    s.grpcServer = grpc.NewServer()
    opcpb.RegisterAgentServiceServer(s.grpcServer, agentSrv)

    go func() {
        s.logger.Infow("gRPC server starting", "port", 9528)
        if err := s.grpcServer.Serve(lis); err != nil {
            s.logger.Errorw("gRPC server error", "error", err)
        }
    }()

    return nil
}
```

Add to `Stop()`:
```go
if s.grpcServer != nil {
    s.grpcServer.GracefulStop()
}
```

- [ ] **Step 2: Verify build compiles**

Run: `go build ./...`

Expected: Clean build

- [ ] **Step 3: Commit**

```bash
git add pkg/server/server.go
git commit -m "feat(server): start gRPC server on :9528 alongside REST"
```

---

## Phase C: FederationService gRPC Implementation

### Task 10: HMAC gRPC Interceptor

**Files:**
- Create: `pkg/a2a/interceptor.go`
- Create: `pkg/a2a/interceptor_test.go`

- [ ] **Step 1: Write interceptor tests**

Test that valid HMAC passes, invalid HMAC is rejected, missing key is rejected.

- [ ] **Step 2: Run tests to verify they fail**
- [ ] **Step 3: Implement interceptor.go**

Extract HMAC from `metadata.FromIncomingContext(ctx)`, validate against `federation.APIKeyStore`. Return `codes.Unauthenticated` on failure.

- [ ] **Step 4: Run tests, verify pass**
- [ ] **Step 5: Commit**

```bash
git commit -m "feat(a2a): HMAC gRPC unary + stream interceptors"
```

### Task 11: Federation gRPC Server

**Files:**
- Create: `pkg/federation/grpc_server.go`
- Create: `pkg/federation/grpc_server_test.go`

- [ ] **Step 1: Write tests** — DispatchProject, ReportTaskResult, HeartbeatStream
- [ ] **Step 2: Run tests, verify fail**
- [ ] **Step 3: Implement FederationServiceServer**

Delegates to existing `federation.FederationController` methods, converting between proto and Go types using `pkg/a2a/convert.go`.

- [ ] **Step 4: Run tests, verify pass**
- [ ] **Step 5: Commit**

```bash
git commit -m "feat(federation): FederationService gRPC server"
```

### Task 12: Federation gRPC Client

**Files:**
- Create: `pkg/federation/grpc_client.go`
- Create: `pkg/federation/grpc_client_test.go`

- [ ] **Step 1: Write tests** — DispatchProject over gRPC, ReportTaskResult over gRPC
- [ ] **Step 2: Run tests, verify fail**
- [ ] **Step 3: Implement FederationGRPCClient**

Wraps `opcpb.FederationServiceClient`, adds HMAC metadata via `grpc.WithUnaryInterceptor`.

- [ ] **Step 4: Run tests, verify pass**
- [ ] **Step 5: Commit**

```bash
git commit -m "feat(federation): gRPC client for Master → Worker dispatch"
```

### Task 13: Dual-Protocol Federation Dispatch

**Files:**
- Modify: `pkg/federation/federation.go`

- [ ] **Step 1: Add `SupportsGRPC()` to company/node tracking**
- [ ] **Step 2: In dispatch logic, check gRPC support and route accordingly**
- [ ] **Step 3: Verify build + existing tests pass**
- [ ] **Step 4: Commit**

```bash
git commit -m "feat(federation): dual-protocol dispatch (gRPC preferred, HTTP fallback)"
```

### Task 14: Register FederationService in gRPC Server

**Files:**
- Modify: `pkg/server/server.go`

- [ ] **Step 1: Register FederationServiceServer alongside AgentServiceServer in startGRPC()**
- [ ] **Step 2: Add HMAC interceptor to gRPC server options**
- [ ] **Step 3: Verify build**
- [ ] **Step 4: Commit**

```bash
git commit -m "feat(server): register FederationService on gRPC :9528 with HMAC auth"
```

---

## Phase D: Dashboard New Pages

### Task 15: Install Frontend Dependencies

**Files:**
- Modify: `dashboard/package.json`

- [ ] **Step 1: Install packages**

Run: `cd dashboard && npm install @xyflow/react recharts framer-motion`

- [ ] **Step 2: Commit**

```bash
git add dashboard/package.json dashboard/package-lock.json
git commit -m "chore(dashboard): add xyflow, recharts, framer-motion"
```

### Task 16: Frontend Types + API Extensions

**Files:**
- Modify: `dashboard/src/types/index.ts`
- Modify: `dashboard/src/lib/api.ts`

- [ ] **Step 1: Add new types to types/index.ts**

```typescript
export interface ModelInfo {
  id: string
  provider: string
  displayName: string
  tier: 'economy' | 'standard' | 'premium'
  costPer1k: number
  capability: 'fast' | 'balanced' | 'reasoning'
  default?: boolean
}

export interface RFC {
  id: string
  title: string
  problem: string
  solution: string
  expectedBenefit: string
  risk: string
  status: 'pending' | 'approved' | 'rejected'
  createdAt: string
}

export interface SystemMetrics {
  successRate: number
  avgLatency: number
  costPerGoal: number
  retryRate: number
  coverageGap: number
  errorPatterns: string[]
  timestamp: string
}

export interface WorkflowRunDetailStep {
  name: string
  agent: string
  status: string
  result?: string
  error?: string
  tokensIn?: number
  tokensOut?: number
  cost?: number
  startedAt?: string
  endedAt?: string
  dependsOn?: string[]
}

export interface WizardRequest {
  type: string
  description: string
  model: string
  fallbackModel?: string
  preset: 'light' | 'standard' | 'power' | 'custom'
  replicas: number
  onExceed: string
  customBudget?: {
    tokenPerDay: number
    costPerDay: number
  }
}
```

- [ ] **Step 2: Add API functions to api.ts**

```typescript
// Models
export async function fetchModels(): Promise<ModelInfo[]> {
  return fetchJson<ModelInfo[]>('/models')
}

// Agent Wizard
export async function createAgentWizard(data: WizardRequest): Promise<Agent> {
  const response = await fetch(`${API_BASE}/agents/wizard`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(data),
  })
  if (!response.ok) throw new Error(`Failed to create agent: ${response.statusText}`)
  return response.json()
}

// Goal detail
export async function fetchGoalDetail(id: string): Promise<Goal> {
  return fetchJson<Goal>(`/goals/${id}`)
}

export async function fetchGoalIssues(goalId: string): Promise<Issue[]> {
  return fetchJson<Issue[]>(`/goals/${goalId}/issues`)
}

export async function approveGoal(id: string): Promise<void> {
  const response = await fetch(`${API_BASE}/goals/${id}/approve`, { method: 'POST' })
  if (!response.ok) throw new Error('Failed to approve goal')
}

// RFCs
export async function fetchRFCs(): Promise<RFC[]> {
  return fetchJson<RFC[]>('/system/rfcs')
}

export async function approveRFC(id: string): Promise<void> {
  const response = await fetch(`${API_BASE}/system/rfcs/${id}/approve`, { method: 'POST' })
  if (!response.ok) throw new Error('Failed to approve RFC')
}

export async function rejectRFC(id: string): Promise<void> {
  const response = await fetch(`${API_BASE}/system/rfcs/${id}/reject`, { method: 'POST' })
  if (!response.ok) throw new Error('Failed to reject RFC')
}

// System metrics
export async function fetchSystemMetrics(): Promise<SystemMetrics[]> {
  return fetchJson<SystemMetrics[]>('/system/metrics')
}

// Workflow run detail
export async function fetchWorkflowRunDetail(name: string, runId: string): Promise<WorkflowRunDetailStep[]> {
  return fetchJson<WorkflowRunDetailStep[]>(`/workflows/${name}/runs/${runId}/steps`)
}
```

- [ ] **Step 3: Commit**

```bash
git add dashboard/src/types/index.ts dashboard/src/lib/api.ts
git commit -m "feat(dashboard): add types and API functions for models, RFCs, metrics, wizard"
```

### Task 17: Shared UI Components

**Files:**
- Create: `dashboard/src/components/Skeleton.tsx`
- Create: `dashboard/src/components/EmptyState.tsx`
- Create: `dashboard/src/components/ThemeToggle.tsx`
- Create: `dashboard/src/components/BudgetProgress.tsx`

- [ ] **Step 1: Implement Skeleton, EmptyState, ThemeToggle, BudgetProgress**

Each is a small, focused component. Skeleton uses Tailwind `animate-pulse`. EmptyState shows icon + message + optional CTA. ThemeToggle toggles `dark` class on document. BudgetProgress shows a progress bar with color thresholds.

- [ ] **Step 2: Verify build**

Run: `cd dashboard && npm run build`

- [ ] **Step 3: Commit**

```bash
git commit -m "feat(dashboard): add Skeleton, EmptyState, ThemeToggle, BudgetProgress components"
```

### Task 18: Goal Tree Visualization

**Files:**
- Create: `dashboard/src/components/GoalTree.tsx`
- Create: `dashboard/src/app/goals/[id]/page.tsx`
- Modify: `dashboard/src/app/goals/page.tsx` (add link to detail)

- [ ] **Step 1: Implement GoalTree.tsx**

Recursive collapsible tree: Goal → Project → Task → Issue. Each node shows status badge, progress bar, cost. Uses Tailwind for indentation and collapse animation.

- [ ] **Step 2: Implement goal detail page**

Fetches goal detail, projects, issues, stats. Renders GoalTree + stats summary + approve button (if phase=planned).

- [ ] **Step 3: Add link from goals list to detail**

In `goals/page.tsx`, wrap goal name in `<Link href={/goals/${goal.id}}>`.

- [ ] **Step 4: Verify build**
- [ ] **Step 5: Commit**

```bash
git commit -m "feat(dashboard): Goal tree visualization with detail page"
```

### Task 19: DAG Visualization Component

**Files:**
- Create: `dashboard/src/components/DAGVisualization.tsx`

- [ ] **Step 1: Implement DAGVisualization using @xyflow/react**

Takes `nodes` (projects/steps) and `edges` (dependencies) as props. Each node shows name + status badge + cost. Uses dagre layout algorithm for automatic positioning.

- [ ] **Step 2: Integrate into goal detail page** (optional DAG view toggle)
- [ ] **Step 3: Verify build**
- [ ] **Step 4: Commit**

```bash
git commit -m "feat(dashboard): DAG dependency visualization with @xyflow/react"
```

### Task 20: RFC Approval Page

**Files:**
- Create: `dashboard/src/components/RFCCard.tsx`
- Create: `dashboard/src/app/rfcs/page.tsx`
- Modify: `dashboard/src/components/Sidebar.tsx` (add RFCs nav item)

- [ ] **Step 1: Implement RFCCard** — shows problem/solution/benefit/risk, approve/reject buttons
- [ ] **Step 2: Implement rfcs/page.tsx** — list of RFCCards with filtering
- [ ] **Step 3: Add "RFCs" to Sidebar**
- [ ] **Step 4: Verify build**
- [ ] **Step 5: Commit**

```bash
git commit -m "feat(dashboard): RFC approval page with approve/reject actions"
```

### Task 21: System Metrics Dashboard

**Files:**
- Create: `dashboard/src/components/MetricsChart.tsx`
- Create: `dashboard/src/app/metrics/page.tsx`
- Modify: `dashboard/src/components/Sidebar.tsx` (add Metrics nav item)

- [ ] **Step 1: Implement MetricsChart** using recharts — line chart with time x-axis
- [ ] **Step 2: Implement metrics/page.tsx** — grid of MetricsCharts for each metric
- [ ] **Step 3: Add "Metrics" to Sidebar**
- [ ] **Step 4: Verify build**
- [ ] **Step 5: Commit**

```bash
git commit -m "feat(dashboard): system metrics dashboard with time-series charts"
```

### Task 22: Workflow Run Detail Page

**Files:**
- Create: `dashboard/src/components/WorkflowRunDetail.tsx`
- Create: `dashboard/src/app/workflows/[name]/runs/[id]/page.tsx`

- [ ] **Step 1: Implement WorkflowRunDetail** — step list with status/duration/output, DAG view
- [ ] **Step 2: Implement run detail page** — fetches steps, renders WorkflowRunDetail
- [ ] **Step 3: Add link from workflows page to run detail**
- [ ] **Step 4: Verify build**
- [ ] **Step 5: Commit**

```bash
git commit -m "feat(dashboard): workflow run detail page with step-by-step view"
```

---

## Phase E: Agent Wizard + Model Registry

### Task 23: Model Registry Backend

**Files:**
- Create: `pkg/model/registry.go`
- Create: `pkg/model/registry_test.go`
- Modify: `pkg/server/server.go` (add GET/POST /api/models endpoints)

- [ ] **Step 1: Write registry tests**

Test `ListModels()` returns built-in models, `AddModel()` adds custom, `GetModel()` by ID.

- [ ] **Step 2: Run tests, verify fail**
- [ ] **Step 3: Implement registry.go**

In-memory registry with built-in models (Claude Sonnet/Opus/Haiku, GPT-4o/mini/o3) + CRUD for custom models.

- [ ] **Step 4: Run tests, verify pass**
- [ ] **Step 5: Add REST endpoints to server.go**

```go
router.GET("/api/models", s.handleListModels)
router.POST("/api/models", s.handleAddModel)
```

- [ ] **Step 6: Verify build**
- [ ] **Step 7: Commit**

```bash
git commit -m "feat(model): model registry with built-in models + custom CRUD + REST API"
```

### Task 24: Agent Wizard Backend

**Files:**
- Modify: `pkg/server/server.go` (add POST /api/agents/wizard)

- [ ] **Step 1: Implement wizard endpoint**

Accepts `WizardRequest`, expands preset to full `AgentSpec`, generates `AgentCard`, calls `controller.Apply()`.

Preset expansion:
- `light`: perDay=100000 tokens, $5/day
- `standard`: perDay=1000000 tokens, $20/day
- `power`: perDay=0 (unlimited), $100/day

Skill inference: keyword matching from description → skill tags.

- [ ] **Step 2: Verify build**
- [ ] **Step 3: Commit**

```bash
git commit -m "feat(server): wizard endpoint for no-code agent creation"
```

### Task 25: Agent Wizard Frontend

**Files:**
- Create: `dashboard/src/components/AgentWizard/AgentWizard.tsx`
- Create: `dashboard/src/components/AgentWizard/StepTypeSelect.tsx`
- Create: `dashboard/src/components/AgentWizard/StepDescribe.tsx`
- Create: `dashboard/src/components/AgentWizard/StepResources.tsx`
- Create: `dashboard/src/components/AgentWizard/StepConfirm.tsx`
- Create: `dashboard/src/components/AgentWizard/YAMLPreview.tsx`
- Modify: `dashboard/src/components/AddAgentModal.tsx` (replace with wizard trigger)

- [ ] **Step 1: Implement AgentWizard** — 4-step state machine with prev/next
- [ ] **Step 2: StepTypeSelect** — card grid for agent types
- [ ] **Step 3: StepDescribe** — textarea + model selection grouped by provider + fallback dropdown
- [ ] **Step 4: StepResources** — preset radio + custom sliders + onExceed select
- [ ] **Step 5: StepConfirm** — summary + YAMLPreview + create button
- [ ] **Step 6: Replace AddAgentModal** with wizard modal trigger
- [ ] **Step 7: Verify build**
- [ ] **Step 8: Commit**

```bash
git commit -m "feat(dashboard): no-code Agent creation wizard with model selection"
```

---

## Phase F: Dashboard Polish

### Task 26: AgentCard View + Federation Enhancement

**Files:**
- Create: `dashboard/src/components/AgentCardView.tsx`
- Create: `dashboard/src/components/NodeStatusBadge.tsx`
- Modify: `dashboard/src/app/federation/page.tsx`

- [ ] **Step 1: Implement AgentCardView** — displays A2A AgentCard info (skills, model, capabilities)
- [ ] **Step 2: Implement NodeStatusBadge** — green/yellow/red dot + label
- [ ] **Step 3: Enhance federation page** — node cards with badges, gRPC status, agent list per node
- [ ] **Step 4: Verify build**
- [ ] **Step 5: Commit**

```bash
git commit -m "feat(dashboard): AgentCard view + enhanced federation status"
```

### Task 27: Cost Report Enhancement

**Files:**
- Modify: `dashboard/src/app/costs/page.tsx`

- [ ] **Step 1: Add dimension switcher** (by Goal / by Agent / by Time)
- [ ] **Step 2: Add trend chart** using recharts
- [ ] **Step 3: Add BudgetProgress** bars for daily/monthly budgets
- [ ] **Step 4: Verify build**
- [ ] **Step 5: Commit**

```bash
git commit -m "feat(dashboard): enhanced cost reports with multi-dimension analysis"
```

### Task 28: Responsive + Dark Mode + Animations

**Files:**
- Modify: `dashboard/src/components/Sidebar.tsx` (collapsible)
- Modify: `dashboard/src/app/layout.tsx` (add ThemeToggle, framer-motion AnimatePresence)
- Modify: various page files (add Skeleton loading states, EmptyState)

- [ ] **Step 1: Make Sidebar collapsible** — hamburger button, responsive breakpoint
- [ ] **Step 2: Add ThemeToggle** to header
- [ ] **Step 3: Audit all dark: classes** — ensure consistent dark mode
- [ ] **Step 4: Add Skeleton** to all async-loading pages
- [ ] **Step 5: Add EmptyState** to all list pages
- [ ] **Step 6: Add framer-motion** page transitions (AnimatePresence + motion.div)
- [ ] **Step 7: Verify build**
- [ ] **Step 8: Commit**

```bash
git commit -m "feat(dashboard): responsive layout, dark mode, loading skeletons, animations"
```

---

## Phase G: Test Coverage + Feature Completion

### Task 29: pkg/controller Test Coverage → 80%

**Files:**
- Modify: `pkg/controller/*_test.go`

- [ ] **Step 1: Run coverage baseline**

Run: `go test ./pkg/controller/ -coverprofile=coverage.out && go tool cover -func=coverage.out | tail -1`

- [ ] **Step 2: Identify uncovered paths** — read coverage.out, find functions <80%
- [ ] **Step 3: Write tests for uncovered paths** — focus on error paths, edge cases, concurrent access
- [ ] **Step 4: Verify coverage ≥ 80%**
- [ ] **Step 5: Commit**

```bash
git commit -m "test(controller): improve coverage to 80%+"
```

### Task 30: pkg/server Test Coverage → 80%

Same pattern as Task 29 but for `pkg/server/`.

### Task 31: pkg/adapter/claudecode Test Coverage → 80%

Same pattern as Task 29 but for `pkg/adapter/claudecode/`.

### Task 32: pkg/storage/sqlite Test Coverage → 80%

Same pattern as Task 29 but for `pkg/storage/sqlite/`.

### Task 33: pkg/evolve Skeleton

**Files:**
- Create: `pkg/evolve/metrics.go`
- Create: `pkg/evolve/rfc.go`
- Create: `pkg/evolve/metrics_test.go`

- [ ] **Step 1: Write MetricsCollector tests**

Test that `Collect()` returns metrics from storage, `SuccessRate()` calculation is correct.

- [ ] **Step 2: Implement MetricsCollector**

Queries storage for task success/failure counts, average latency, cost per goal, retry rate.

- [ ] **Step 3: Implement RFC data structure**

```go
type RFC struct {
    ID              string    `json:"id"`
    Title           string    `json:"title"`
    Problem         string    `json:"problem"`
    Solution        string    `json:"solution"`
    ExpectedBenefit string    `json:"expectedBenefit"`
    Risk            string    `json:"risk"`
    Status          string    `json:"status"` // "pending" | "approved" | "rejected"
    CreatedAt       time.Time `json:"createdAt"`
}
```

- [ ] **Step 4: Add REST endpoints for metrics + RFC**

```go
router.GET("/api/system/metrics", s.handleSystemMetrics)
router.GET("/api/system/rfcs", s.handleListRFCs)
router.POST("/api/system/rfcs/:id/approve", s.handleApproveRFC)
router.POST("/api/system/rfcs/:id/reject", s.handleRejectRFC)
```

- [ ] **Step 5: Run tests, verify pass**
- [ ] **Step 6: Commit**

```bash
git commit -m "feat(evolve): MetricsCollector + RFC skeleton for self-evolving loop"
```

### Task 34: Workflow Run Detail API

**Files:**
- Modify: `pkg/server/server.go`

- [ ] **Step 1: Add GET /api/workflows/:name/runs/:id/steps endpoint**

Returns parsed step details from `WorkflowRunRecord.StepsJSON`.

- [ ] **Step 2: Verify build + test**
- [ ] **Step 3: Commit**

```bash
git commit -m "feat(server): workflow run detail API with step-level data"
```

---

## Phase H: Integration + Cleanup

### Task 35: End-to-End A2A Integration Test

**Files:**
- Create: `test/integration/a2a_test.go`

- [ ] **Step 1: Write integration test**

1. Start OPC Server (REST + gRPC)
2. Register a mock agent via REST
3. Send task via gRPC AgentService.SendTask
4. Verify task completes with correct A2A Task state
5. Verify CostReport is populated

- [ ] **Step 2: Run test**
- [ ] **Step 3: Commit**

```bash
git commit -m "test: end-to-end A2A gRPC integration test"
```

### Task 36: Federation gRPC Integration Test

**Files:**
- Create: `test/integration/federation_grpc_test.go`

- [ ] **Step 1: Write integration test**

1. Start Master + Worker OPC servers
2. Worker registers via gRPC FederationService.Register
3. Master dispatches project via gRPC
4. Worker reports result via gRPC
5. Verify goal completes

- [ ] **Step 2: Run test**
- [ ] **Step 3: Commit**

```bash
git commit -m "test: federation gRPC integration test (Master ↔ Worker)"
```

### Task 37: CI Update

**Files:**
- Modify: `.github/workflows/integration-test.yml`

- [ ] **Step 1: Add protoc install step**
- [ ] **Step 2: Add `make proto` step before build**
- [ ] **Step 3: Add gRPC integration test to CI matrix**
- [ ] **Step 4: Commit**

```bash
git commit -m "ci: add protoc generation and gRPC tests to CI workflow"
```

### Task 38: Dashboard Build Verification

- [ ] **Step 1: Run full dashboard build**

Run: `cd dashboard && npm run build`

Expected: Clean build, no errors

- [ ] **Step 2: Run lint**

Run: `cd dashboard && npm run lint`

- [ ] **Step 3: Fix any issues**
- [ ] **Step 4: Commit if fixes needed**

### Task 39: Final Coverage Check

- [ ] **Step 1: Run full test suite with coverage**

Run: `go test ./... -coverprofile=coverage.out && go tool cover -func=coverage.out`

- [ ] **Step 2: Verify all target packages ≥ 80%**
- [ ] **Step 3: Tag release**

```bash
git tag v0.7.0
```

---

*Plan version*: v1.0
*Created*: 2026-03-26
*Design spec*: `docs/superpowers/specs/2026-03-26-v07-a2a-protobuf-ui-design.md`
*Total tasks*: 39
*Estimated phases*: 8 (A through H)
