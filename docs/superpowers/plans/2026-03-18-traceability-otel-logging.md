# Traceability + OpenTelemetry + Configurable Logging Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add full issue traceability (A+B hybrid: lineage chain + traceId/spanId), integrate OpenTelemetry with Jaeger, and implement configurable log levels — making every issue traceable across federated OPC instances.

**Architecture:** Each issue carries both a `lineage` array (fast flat query) and `traceId/spanId/parentSpans` (structured OTel-compatible trace). OpenTelemetry SDK exports traces to Jaeger via OTLP. Log level is a config parameter (`--log-level debug|info|warn|error`) parsed from CLI flag, config.yaml, and env var. Docker-compose bundles Jaeger for one-command startup.

**Tech Stack:** Go 1.23, OpenTelemetry Go SDK (`go.opentelemetry.io/otel`), Jaeger (all-in-one Docker image), Zap logger, existing Gin/Cobra/Viper stack.

---

## File Structure

### New Files
| File | Responsibility |
|------|---------------|
| `pkg/trace/tracer.go` | OpenTelemetry tracer provider init, shutdown, span helpers |
| `pkg/trace/tracer_test.go` | Unit tests for tracer init and span creation |
| `pkg/goal/dag_test.go` | Already exists (created in prior session) |
| `pkg/goal/lineage.go` | LineageRef type, lineage builder helpers |
| `pkg/goal/lineage_test.go` | Tests for lineage append/query |
| `docker/docker-compose.observability.yaml` | Jaeger + OPC compose stack |
| `examples/federation-workflow/README.md` | Usage guide for multi-OPC federation demo |
| `examples/federation-workflow/start-federation.sh` | Script to start 3 OPC instances locally |
| `examples/federation-workflow/goal-login-feature.json` | Example goal with project dependencies |
| `examples/federation-workflow/stop-federation.sh` | Cleanup script |

### Modified Files
| File | Changes |
|------|---------|
| `go.mod` | Add OpenTelemetry dependencies |
| `internal/config/config.go` | Add `LogLevel` parsing with `debug\|info\|warn\|error` support |
| `cmd/opctl/root.go` | Add `--log-level` CLI flag |
| `cmd/opctl/serve.go` | Init OTel tracer provider, pass to server, shutdown on exit |
| `api/v1/types.go` | Add `TraceID, SpanID, ParentSpans, Lineage` to `IssueRecord`; add `GoalID` to `IssueRecord`; add `LineageJSON` to `TaskRecord` |
| `pkg/storage/sqlite/sqlite.go` | ALTER TABLE migrations for new columns on tasks and issues |
| `pkg/storage/sqlite/sqlite.go` | Update CRUD queries to read/write new columns |
| `pkg/audit/audit.go` | Add `IssueRef` to `AuditEvent` |
| `pkg/server/server.go` | Add OTel spans to `runTask`, `createFederatedGoal`, `federationCallback`; fix task GoalID/ProjectID assignment in `runTask`; carry lineage in federation dispatch/callback |
| `pkg/goal/goal.go` | Add `LineageRef` type definition (if not in separate file) |

---

## Review Fixes Applied

The following issues from plan review have been addressed inline:
1. **CRITICAL**: CREATE TABLE for tasks now includes `issue_id, project_id, goal_id` columns (fixes existing test failures)
2. **CRITICAL**: Task 4 uses `scanTask` helper pattern instead of inline Scan (preserves `sql.NullTime` handling)
3. **HIGH**: Removed duplicate `InitLogger` call from `serve.go` (PersistentPreRun already calls it)
4. **HIGH**: OTel semconv version set to `v1.24.0` (verified stable)
5. **HIGH**: Added `--state-dir` CLI flag for demo instance isolation

---

## Task 1: Configurable Log Level

**Files:**
- Modify: `internal/config/config.go:64-88`
- Modify: `cmd/opctl/root.go:28-34`
- Modify: `cmd/opctl/serve.go:46`

- [ ] **Step 1: Write failing test for log level parsing**

Create `internal/config/config_test.go`:

```go
package config

import (
	"testing"

	"go.uber.org/zap/zapcore"
)

func TestParseLogLevel(t *testing.T) {
	tests := []struct {
		input    string
		expected zapcore.Level
	}{
		{"debug", zapcore.DebugLevel},
		{"info", zapcore.InfoLevel},
		{"warn", zapcore.WarnLevel},
		{"error", zapcore.ErrorLevel},
		{"DEBUG", zapcore.DebugLevel},
		{"", zapcore.InfoLevel},        // default
		{"invalid", zapcore.InfoLevel}, // fallback
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := ParseLogLevel(tt.input)
			if got != tt.expected {
				t.Errorf("ParseLogLevel(%q) = %v, want %v", tt.input, got, tt.expected)
			}
		})
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/config/ -run TestParseLogLevel -v`
Expected: FAIL — `ParseLogLevel` undefined

- [ ] **Step 3: Implement ParseLogLevel in config.go**

Add to `internal/config/config.go`:

```go
// ParseLogLevel converts a string log level to zapcore.Level.
// Supported values: debug, info, warn, error. Defaults to info.
func ParseLogLevel(level string) zapcore.Level {
	switch strings.ToLower(strings.TrimSpace(level)) {
	case "debug":
		return zapcore.DebugLevel
	case "info":
		return zapcore.InfoLevel
	case "warn", "warning":
		return zapcore.WarnLevel
	case "error":
		return zapcore.ErrorLevel
	default:
		return zapcore.InfoLevel
	}
}
```

Add `"strings"` to imports.

- [ ] **Step 4: Update InitLogger to accept log level string**

Replace `InitLogger(verbose bool)` with:

```go
// InitLogger initializes the global zap logger.
// The level parameter accepts: debug, info, warn, error.
// When verbose is true, it overrides level to debug.
func InitLogger(verbose bool, level string) {
	zapLevel := ParseLogLevel(level)
	if verbose {
		zapLevel = zapcore.DebugLevel
	}

	cfg := zap.Config{
		Level:            zap.NewAtomicLevelAt(zapLevel),
		Development:      zapLevel == zapcore.DebugLevel,
		Encoding:         "console",
		EncoderConfig:    zap.NewDevelopmentEncoderConfig(),
		OutputPaths:      []string{"stderr"},
		ErrorOutputPaths: []string{"stderr"},
	}

	logger, err := cfg.Build()
	if err != nil {
		logger = zap.NewNop()
	}

	Logger = logger.Sugar()
}
```

- [ ] **Step 5: Run test to verify it passes**

Run: `go test ./internal/config/ -run TestParseLogLevel -v`
Expected: PASS

- [ ] **Step 6: Add --log-level CLI flag to root.go**

In `cmd/opctl/root.go`, add variable and flag:

```go
var logLevel string
```

In `init()`, add:
```go
rootCmd.PersistentFlags().StringVar(&logLevel, "log-level", "", "log level: debug|info|warn|error (default from config or 'info')")
```

Update `PersistentPreRun` to:
```go
PersistentPreRun: func(cmd *cobra.Command, args []string) {
    level := logLevel
    if level == "" {
        level = viper.GetString("logLevel")
    }
    config.InitLogger(verbose, level)
},
```

- [ ] **Step 7: Remove duplicate InitLogger call from serve.go**

In `cmd/opctl/serve.go:46`, **remove** the line `config.InitLogger(verbose)` entirely.
`PersistentPreRun` in `root.go` already initializes the logger before `runServe` executes, so calling it again is redundant.

- [ ] **Step 8: Build and verify**

Run: `go build ./...`
Expected: clean build

- [ ] **Step 9: Commit**

```bash
git add internal/config/config.go internal/config/config_test.go cmd/opctl/root.go cmd/opctl/serve.go
git commit -m "feat(config): configurable log level via --log-level flag and config.yaml"
```

---

## Task 2: OpenTelemetry Tracer Provider

**Files:**
- Create: `pkg/trace/tracer.go`
- Create: `pkg/trace/tracer_test.go`
- Modify: `go.mod` (add OTel deps)

- [ ] **Step 1: Add OpenTelemetry dependencies**

```bash
go get go.opentelemetry.io/otel@latest
go get go.opentelemetry.io/otel/sdk@latest
go get go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp@latest
go get go.opentelemetry.io/otel/trace@latest
```

- [ ] **Step 2: Write failing test for tracer init**

Create `pkg/trace/tracer_test.go`:

```go
package trace

import (
	"context"
	"testing"
)

func TestInitTracer_NoopWhenDisabled(t *testing.T) {
	shutdown, err := InitTracer(Config{Enabled: false})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer shutdown(context.Background())

	// Should get a valid (noop) tracer without error.
	tr := Tracer()
	if tr == nil {
		t.Fatal("tracer should not be nil even when disabled")
	}
}

func TestStartSpan(t *testing.T) {
	shutdown, err := InitTracer(Config{Enabled: false})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer shutdown(context.Background())

	ctx, span := StartSpan(context.Background(), "test-op")
	if ctx == nil {
		t.Fatal("context should not be nil")
	}
	if span == nil {
		t.Fatal("span should not be nil")
	}
	span.End()
}
```

- [ ] **Step 3: Run test to verify it fails**

Run: `go test ./pkg/trace/ -v`
Expected: FAIL — package doesn't exist

- [ ] **Step 4: Implement tracer.go**

Create `pkg/trace/tracer.go`:

```go
package trace

import (
	"context"
	"fmt"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.24.0"
	"go.opentelemetry.io/otel/trace"
	"go.opentelemetry.io/otel/trace/noop"
)

const tracerName = "opc-platform"

// Config holds OpenTelemetry configuration.
type Config struct {
	Enabled      bool   // Whether tracing is enabled.
	OTLPEndpoint string // OTLP HTTP endpoint (e.g. "localhost:4318").
	ServiceName  string // Service name for this OPC instance.
}

// InitTracer initializes the OpenTelemetry tracer provider.
// Returns a shutdown function that must be called on application exit.
func InitTracer(cfg Config) (func(context.Context) error, error) {
	if !cfg.Enabled {
		// Register a noop provider so Tracer() always returns a valid tracer.
		otel.SetTracerProvider(noop.NewTracerProvider())
		return func(ctx context.Context) error { return nil }, nil
	}

	if cfg.ServiceName == "" {
		cfg.ServiceName = "opc-platform"
	}
	if cfg.OTLPEndpoint == "" {
		cfg.OTLPEndpoint = "localhost:4318"
	}

	exporter, err := otlptracehttp.New(
		context.Background(),
		otlptracehttp.WithEndpoint(cfg.OTLPEndpoint),
		otlptracehttp.WithInsecure(),
	)
	if err != nil {
		return nil, fmt.Errorf("create OTLP exporter: %w", err)
	}

	res, err := resource.New(
		context.Background(),
		resource.WithAttributes(
			semconv.ServiceNameKey.String(cfg.ServiceName),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("create resource: %w", err)
	}

	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
		sdktrace.WithResource(res),
	)

	otel.SetTracerProvider(tp)

	return func(ctx context.Context) error {
		return tp.Shutdown(ctx)
	}, nil
}

// Tracer returns the global OPC tracer.
func Tracer() trace.Tracer {
	return otel.Tracer(tracerName)
}

// StartSpan is a convenience wrapper that starts a new span.
func StartSpan(ctx context.Context, name string, opts ...trace.SpanStartOption) (context.Context, trace.Span) {
	return Tracer().Start(ctx, name, opts...)
}

// SpanIDFromContext extracts the span ID hex string from the current span context.
func SpanIDFromContext(ctx context.Context) string {
	sc := trace.SpanFromContext(ctx).SpanContext()
	if sc.HasSpanID() {
		return sc.SpanID().String()
	}
	return ""
}

// TraceIDFromContext extracts the trace ID hex string from the current span context.
func TraceIDFromContext(ctx context.Context) string {
	sc := trace.SpanFromContext(ctx).SpanContext()
	if sc.HasTraceID() {
		return sc.TraceID().String()
	}
	return ""
}
```

- [ ] **Step 5: Run test to verify it passes**

Run: `go test ./pkg/trace/ -v`
Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add pkg/trace/ go.mod go.sum
git commit -m "feat(trace): OpenTelemetry tracer provider with OTLP/HTTP exporter"
```

---

## Task 3: Lineage Data Model

**Files:**
- Create: `pkg/goal/lineage.go`
- Create: `pkg/goal/lineage_test.go`
- Modify: `api/v1/types.go:234-248` (IssueRecord)
- Modify: `api/v1/types.go:167-185` (TaskRecord)

- [ ] **Step 1: Write failing test for lineage builder**

Create `pkg/goal/lineage_test.go`:

```go
package goal

import (
	"testing"
)

func TestAppendLineage(t *testing.T) {
	upstream := []LineageRef{
		{GoalID: "g-1", ProjectName: "ui-design", IssueID: "iss-1", OPCNode: "design-opc", Label: "UI 设计稿"},
	}
	newRef := LineageRef{
		GoalID: "g-1", ProjectName: "frontend", IssueID: "iss-2", OPCNode: "frontend-opc", Label: "前端实现",
	}

	result := AppendLineage(upstream, newRef)
	if len(result) != 2 {
		t.Fatalf("expected 2 refs, got %d", len(result))
	}
	if result[1].IssueID != "iss-2" {
		t.Errorf("expected new ref at end, got %v", result[1])
	}
	// Verify upstream was not mutated.
	if len(upstream) != 1 {
		t.Errorf("upstream was mutated")
	}
}

func TestLineageToJSON(t *testing.T) {
	refs := []LineageRef{
		{GoalID: "g-1", ProjectName: "ui", IssueID: "i-1", OPCNode: "design", Label: "design"},
	}
	data, err := LineageToJSON(refs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(data) == 0 {
		t.Fatal("expected non-empty JSON")
	}

	parsed, err := LineageFromJSON(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(parsed) != 1 || parsed[0].GoalID != "g-1" {
		t.Errorf("round-trip failed: %v", parsed)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./pkg/goal/ -run TestAppendLineage -v`
Expected: FAIL — undefined types

- [ ] **Step 3: Implement lineage.go**

Create `pkg/goal/lineage.go`:

```go
package goal

import "encoding/json"

// LineageRef tracks the provenance of an issue through the federation chain.
type LineageRef struct {
	GoalID      string `json:"goalId"`
	ProjectName string `json:"projectName"`
	IssueID     string `json:"issueId"`
	OPCNode     string `json:"opcNode"`    // which OPC instance produced this
	Label       string `json:"label"`      // human-readable description
}

// AppendLineage returns a new lineage slice with ref appended, without mutating upstream.
func AppendLineage(upstream []LineageRef, ref LineageRef) []LineageRef {
	result := make([]LineageRef, len(upstream)+1)
	copy(result, upstream)
	result[len(upstream)] = ref
	return result
}

// LineageToJSON serializes lineage refs to JSON.
func LineageToJSON(refs []LineageRef) (string, error) {
	if len(refs) == 0 {
		return "[]", nil
	}
	data, err := json.Marshal(refs)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// LineageFromJSON deserializes lineage refs from JSON.
func LineageFromJSON(data string) ([]LineageRef, error) {
	if data == "" || data == "[]" {
		return nil, nil
	}
	var refs []LineageRef
	if err := json.Unmarshal([]byte(data), &refs); err != nil {
		return nil, err
	}
	return refs, nil
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./pkg/goal/ -run TestAppendLineage -v && go test ./pkg/goal/ -run TestLineageToJSON -v`
Expected: PASS

- [ ] **Step 5: Add traceability fields to IssueRecord**

In `api/v1/types.go`, modify `IssueRecord` (lines 234-248) to:

```go
type IssueRecord struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	ProjectID   string    `json:"projectId"`
	GoalID      string    `json:"goalId,omitempty"`
	Description string    `json:"description"`
	AgentRef    string    `json:"agentRef,omitempty"`
	Status      string    `json:"status"`
	SpecYAML    string    `json:"specYaml,omitempty"`
	TokensIn    int       `json:"tokensIn"`
	TokensOut   int       `json:"tokensOut"`
	Cost        float64   `json:"cost"`
	// Traceability fields (A+B hybrid).
	TraceID     string    `json:"traceId,omitempty"`     // = root goal ID, OTel-compatible
	SpanID      string    `json:"spanId,omitempty"`      // unique per issue
	ParentSpans []string  `json:"parentSpans,omitempty"` // upstream issue spanIds
	LineageJSON string    `json:"lineage,omitempty"`     // JSON-serialized []LineageRef
	CreatedAt   time.Time `json:"createdAt"`
	UpdatedAt   time.Time `json:"updatedAt"`
}
```

- [ ] **Step 6: Add lineage field to TaskRecord**

In `api/v1/types.go`, add to `TaskRecord` (after `GoalID` field):

```go
	LineageJSON string     `json:"lineage,omitempty"` // JSON-serialized upstream lineage
```

- [ ] **Step 7: Commit**

```bash
git add pkg/goal/lineage.go pkg/goal/lineage_test.go api/v1/types.go
git commit -m "feat(traceability): lineage chain + traceId/spanId on IssueRecord and TaskRecord"
```

---

## Task 4: Database Schema Migrations

**Files:**
- Modify: `pkg/storage/sqlite/sqlite.go:101-143`
- Modify: `pkg/storage/sqlite/sqlite.go` (CRUD methods for tasks and issues)

- [ ] **Step 1: Fix CREATE TABLE for tasks to include missing columns**

**CRITICAL FIX**: The existing CREATE TABLE is missing `issue_id, project_id, goal_id` columns (causing TestTaskCRUD failures). Update the CREATE TABLE statement in `pkg/storage/sqlite/sqlite.go` (lines 46-61) to include them:

```sql
CREATE TABLE IF NOT EXISTS tasks (
    id         TEXT PRIMARY KEY,
    agent_name TEXT NOT NULL,
    message    TEXT NOT NULL,
    status     TEXT NOT NULL DEFAULT 'Pending',
    result     TEXT NOT NULL DEFAULT '',
    error      TEXT NOT NULL DEFAULT '',
    tokens_in  INTEGER NOT NULL DEFAULT 0,
    tokens_out INTEGER NOT NULL DEFAULT 0,
    cost       REAL NOT NULL DEFAULT 0,
    issue_id   TEXT NOT NULL DEFAULT '',
    project_id TEXT NOT NULL DEFAULT '',
    goal_id    TEXT NOT NULL DEFAULT '',
    lineage_json TEXT NOT NULL DEFAULT '[]',
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    started_at DATETIME,
    ended_at   DATETIME,
    FOREIGN KEY (agent_name) REFERENCES agents(name)
)
```

Also add ALTER TABLE for existing databases (after the goals alterMigrations block):

```go
	taskAlterMigrations := []string{
		"ALTER TABLE tasks ADD COLUMN issue_id TEXT NOT NULL DEFAULT ''",
		"ALTER TABLE tasks ADD COLUMN project_id TEXT NOT NULL DEFAULT ''",
		"ALTER TABLE tasks ADD COLUMN goal_id TEXT NOT NULL DEFAULT ''",
		"ALTER TABLE tasks ADD COLUMN lineage_json TEXT NOT NULL DEFAULT '[]'",
	}
	for _, m := range taskAlterMigrations {
		s.db.Exec(m)
	}
```

- [ ] **Step 2: Add ALTER TABLE migrations for issues table**

```go
	// Issue table: add traceability columns.
	issueAlterMigrations := []string{
		"ALTER TABLE issues ADD COLUMN goal_id TEXT NOT NULL DEFAULT ''",
		"ALTER TABLE issues ADD COLUMN trace_id TEXT NOT NULL DEFAULT ''",
		"ALTER TABLE issues ADD COLUMN span_id TEXT NOT NULL DEFAULT ''",
		"ALTER TABLE issues ADD COLUMN parent_spans TEXT NOT NULL DEFAULT '[]'",
		"ALTER TABLE issues ADD COLUMN lineage_json TEXT NOT NULL DEFAULT '[]'",
	}
	for _, m := range issueAlterMigrations {
		s.db.Exec(m)
	}
```

- [ ] **Step 3: Update CreateTask to include lineage_json column**

The existing `CreateTask` already handles `issue_id, project_id, goal_id`. Add `lineage_json` to the INSERT:

In the SQL string, add `lineage_json` after `goal_id`:
```sql
INSERT INTO tasks (id, agent_name, message, status, result, error, tokens_in, tokens_out, cost, issue_id, project_id, goal_id, lineage_json, created_at, updated_at, started_at, ended_at)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
```
Add `task.LineageJSON` to the values after `task.GoalID`.

- [ ] **Step 4: Update scanTask helper to include lineage_json**

**CRITICAL**: Use the existing `scanTask` helper (line 537) — do NOT replace with inline Scan (it handles `sql.NullTime` for startedAt/endedAt).

Add `lineage_json` to the SELECT in `GetTask`, `ListTasks`, `ListTasksByAgent` queries:
```sql
SELECT id, agent_name, message, status, result, error, tokens_in, tokens_out, cost, issue_id, project_id, goal_id, lineage_json, created_at, updated_at, started_at, ended_at
```

Update the `scanTask` helper to scan the new column:
```go
func scanTask(s scanner) (v1.TaskRecord, error) {
	var t v1.TaskRecord
	var status string
	var startedAt, endedAt sql.NullTime
	err := s.Scan(&t.ID, &t.AgentName, &t.Message, &status,
		&t.Result, &t.Error, &t.TokensIn, &t.TokensOut, &t.Cost,
		&t.IssueID, &t.ProjectID, &t.GoalID, &t.LineageJSON,
		&t.CreatedAt, &t.UpdatedAt, &startedAt, &endedAt)
	// ... rest unchanged
}
```

- [ ] **Step 5: Update UpdateTask to persist lineage_json**

Add `lineage_json=?` to the UPDATE SET clause, and `task.LineageJSON` to the values.

- [ ] **Step 7: Update CreateIssue/GetIssue/ListIssues/UpdateIssue for new columns**

Add `goal_id, trace_id, span_id, parent_spans, lineage_json` to all issue CRUD methods.

- [ ] **Step 8: Delete existing opc.db and run tests**

```bash
rm -f ~/.opc/state/opc.db
go test ./pkg/storage/sqlite/ -v
```

Expected: PASS (including previously failing TestTaskCRUD)

- [ ] **Step 9: Commit**

```bash
git add pkg/storage/sqlite/sqlite.go
git commit -m "fix(storage): add missing task/issue columns for traceability"
```

---

## Task 5: Add IssueRef to Audit System

**Files:**
- Modify: `pkg/audit/audit.go:47-61`

- [ ] **Step 1: Add IssueRef field to AuditEvent**

In `pkg/audit/audit.go`, add after `AgentRef` (line 60):

```go
	IssueRef   string `json:"issueRef,omitempty"`
```

- [ ] **Step 2: Update Trace method to handle IssueRef**

In the `Trace` method, add IssueRef handling alongside the other refs:

In the hierarchy refs collection (line ~193), add:
```go
		if e.IssueRef != "" {
			related[resourceKey{ResourceIssue, e.IssueRef}] = struct{}{}
		}
```

In the ref matching switch (line ~200), add case:
```go
		case ResourceIssue:
			matched = e.IssueRef == resourceName
```

- [ ] **Step 3: Build and verify**

Run: `go build ./...`
Expected: clean build

- [ ] **Step 4: Commit**

```bash
git add pkg/audit/audit.go
git commit -m "feat(audit): add IssueRef for full traceability in audit events"
```

---

## Task 6: Fix runTask Handler — Assign GoalID/ProjectID + Lineage

**Files:**
- Modify: `pkg/server/server.go:619-690`

- [ ] **Step 1: Update runTask request struct to accept lineage**

```go
	var req struct {
		Agent       string `json:"agent"`
		Message     string `json:"message"`
		CallbackURL string `json:"callbackURL,omitempty"`
		GoalID      string `json:"goalId,omitempty"`
		ProjectID   string `json:"projectId,omitempty"`
		LineageJSON string `json:"lineage,omitempty"` // upstream lineage chain
	}
```

- [ ] **Step 2: Assign GoalID, ProjectID, LineageJSON to task record**

Change the task creation (line ~640-643) to:

```go
	task := v1.TaskRecord{
		ID: taskID, AgentName: req.Agent, Message: req.Message,
		Status:      v1.TaskStatusPending,
		GoalID:      req.GoalID,
		ProjectID:   req.ProjectID,
		LineageJSON: req.LineageJSON,
		CreatedAt:   time.Now(), UpdatedAt: time.Now(),
	}
```

- [ ] **Step 3: Add OTel span to runTask**

Wrap the handler body with a span:

```go
	ctx, span := opctrace.StartSpan(ctx, "runTask",
		trace.WithAttributes(
			attribute.String("task.id", taskID),
			attribute.String("agent", req.Agent),
			attribute.String("goal.id", req.GoalID),
			attribute.String("project.id", req.ProjectID),
		))
	defer span.End()
```

Add import: `opctrace "github.com/zlc-ai/opc-platform/pkg/trace"` and `"go.opentelemetry.io/otel/attribute"` and `"go.opentelemetry.io/otel/trace"`

- [ ] **Step 4: Include lineage in federation callback**

Update the callback result in `FederationCallback` struct to include lineage. Add field to FederationCallback:

```go
type FederationCallback struct {
	GoalID      string  `json:"goalId"`
	ProjectID   string  `json:"projectId"`
	TaskID      string  `json:"taskId"`
	Status      string  `json:"status"`
	Result      string  `json:"result,omitempty"`
	TokensIn    int     `json:"tokensIn,omitempty"`
	TokensOut   int     `json:"tokensOut,omitempty"`
	Cost        float64 `json:"cost,omitempty"`
	LineageJSON string  `json:"lineage,omitempty"` // lineage chain from executed task
}
```

In the callback building code, add: `cb.LineageJSON = req.LineageJSON`

- [ ] **Step 5: Build and verify**

Run: `go build ./...`
Expected: clean build

- [ ] **Step 6: Commit**

```bash
git add pkg/server/server.go
git commit -m "fix(server): assign goalId/projectId/lineage in runTask, add OTel span"
```

---

## Task 7: OTel Spans on Federation Dispatch + Callback

**Files:**
- Modify: `pkg/server/server.go` (createFederatedGoal, dispatchProjectLayer, federationCallback, advanceFederatedGoal)

- [ ] **Step 1: Add span to createFederatedGoal**

After validating request and before building projects:

```go
	ctx := c.Request.Context()
	ctx, span := opctrace.StartSpan(ctx, "createFederatedGoal",
		trace.WithAttributes(
			attribute.String("goal.id", goalID),
			attribute.String("goal.name", req.Name),
			attribute.Int("projects.count", len(req.Projects)),
		))
	defer span.End()
```

- [ ] **Step 2: Add span to dispatchProjectLayer**

For each project dispatched, create a child span:

```go
	ctx, projSpan := opctrace.StartSpan(context.Background(), "dispatchProject",
		trace.WithAttributes(
			attribute.String("goal.id", run.GoalID),
			attribute.String("project.name", proj.Name),
			attribute.String("company.id", proj.CompanyID),
		))
	defer projSpan.End()
```

- [ ] **Step 3: Carry lineage in dispatch payload**

In `dispatchProjectLayer`, build lineage from upstream results and include in payload:

```go
		// Build lineage from completed upstream projects.
		var upstreamLineage []goal.LineageRef
		for _, depName := range proj.Dependencies {
			if depProj, ok := run.Projects[depName]; ok {
				upstreamLineage = goal.AppendLineage(upstreamLineage, goal.LineageRef{
					GoalID:      run.GoalID,
					ProjectName: depName,
					IssueID:     depProj.ID,
					OPCNode:     depProj.CompanyID,
					Label:       depProj.Name,
				})
			}
		}
		lineageStr, _ := goal.LineageToJSON(upstreamLineage)

		payload := map[string]interface{}{
			"agent":       agent,
			"message":     message,
			"callbackURL": run.CallbackURL,
			"goalId":      run.GoalID,
			"projectId":   proj.ID,
			"lineage":     lineageStr,
		}
```

- [ ] **Step 4: Add span to federationCallback**

```go
	ctx := c.Request.Context()
	ctx, span := opctrace.StartSpan(ctx, "federationCallback",
		trace.WithAttributes(
			attribute.String("goal.id", cb.GoalID),
			attribute.String("project.id", cb.ProjectID),
			attribute.String("task.id", cb.TaskID),
			attribute.String("status", cb.Status),
		))
	defer span.End()
```

- [ ] **Step 5: Build and verify**

Run: `go build ./...`
Expected: clean build

- [ ] **Step 6: Commit**

```bash
git add pkg/server/server.go
git commit -m "feat(trace): OTel spans on federation dispatch, callback, and lineage propagation"
```

---

## Task 8: Wire OTel into Server Startup

**Files:**
- Modify: `cmd/opctl/serve.go`
- Modify: `cmd/opctl/root.go` (add OTel flags)

- [ ] **Step 1: Add OTel + state-dir CLI flags**

In `cmd/opctl/serve.go`, add variables and flags:

```go
var (
	servePort     int
	serveHost     string
	stateDir      string
	otelEnabled   bool
	otelEndpoint  string
	otelService   string
)

func init() {
	serveCmd.Flags().IntVar(&servePort, "port", 9527, "HTTP listen port")
	serveCmd.Flags().StringVar(&serveHost, "host", "127.0.0.1", "HTTP listen host")
	serveCmd.Flags().StringVar(&stateDir, "state-dir", "", "state directory (default ~/.opc/state)")
	serveCmd.Flags().BoolVar(&otelEnabled, "otel", false, "enable OpenTelemetry tracing")
	serveCmd.Flags().StringVar(&otelEndpoint, "otel-endpoint", "localhost:4318", "OTLP HTTP endpoint")
	serveCmd.Flags().StringVar(&otelService, "otel-service", "", "OTel service name (default: opc-{port})")
	rootCmd.AddCommand(serveCmd)
}
```

**HIGH FIX**: Also add `--state-dir` support in `runServe()` so each OPC instance can use isolated storage:

```go
	// Override state dir if flag provided.
	if stateDir != "" {
		os.MkdirAll(stateDir, 0o755)
		viper.Set("stateDir", stateDir)
	}
```

This must be placed before `getController()` is called. Also update `getController()` in `helpers.go` to read `viper.GetString("stateDir")` for the DB path.
```

- [ ] **Step 2: Init tracer in runServe**

After logger init, before controller creation:

```go
	// Initialize OpenTelemetry tracer.
	if otelService == "" {
		otelService = fmt.Sprintf("opc-%d", servePort)
	}
	shutdownTracer, err := opctrace.InitTracer(opctrace.Config{
		Enabled:      otelEnabled,
		OTLPEndpoint: otelEndpoint,
		ServiceName:  otelService,
	})
	if err != nil {
		return fmt.Errorf("init tracer: %w", err)
	}
	defer shutdownTracer(context.Background())

	if otelEnabled {
		logger.Infow("OpenTelemetry tracing enabled",
			"endpoint", otelEndpoint,
			"service", otelService,
		)
	}
```

Add import: `opctrace "github.com/zlc-ai/opc-platform/pkg/trace"`

- [ ] **Step 3: Build and verify**

Run: `go build ./...`
Expected: clean build

- [ ] **Step 4: Commit**

```bash
git add cmd/opctl/serve.go
git commit -m "feat(serve): wire OpenTelemetry tracer init with --otel flag"
```

---

## Task 9: Docker Compose with Jaeger

**Files:**
- Create: `docker/docker-compose.observability.yaml`

- [ ] **Step 1: Create the compose file**

Create `docker/docker-compose.observability.yaml`:

```yaml
services:
  jaeger:
    image: jaegertracing/all-in-one:1.54
    container_name: opc-jaeger
    ports:
      - "16686:16686"  # Jaeger UI
      - "4318:4318"    # OTLP HTTP receiver
    environment:
      - COLLECTOR_OTLP_ENABLED=true
    restart: unless-stopped

  opc-master:
    build:
      context: ..
      dockerfile: Dockerfile
    container_name: opc-master
    command: ["opctl", "serve", "--host", "0.0.0.0", "--port", "9527", "--otel", "--otel-endpoint", "jaeger:4318", "--otel-service", "opc-master"]
    ports:
      - "9527:9527"
    volumes:
      - opc-master-data:/data/opc
    depends_on:
      - jaeger
    restart: unless-stopped

  opc-design:
    build:
      context: ..
      dockerfile: Dockerfile
    container_name: opc-design
    command: ["opctl", "serve", "--host", "0.0.0.0", "--port", "9528", "--otel", "--otel-endpoint", "jaeger:4318", "--otel-service", "opc-design"]
    ports:
      - "9528:9528"
    volumes:
      - opc-design-data:/data/opc
    depends_on:
      - jaeger
    restart: unless-stopped

  opc-frontend:
    build:
      context: ..
      dockerfile: Dockerfile
    container_name: opc-frontend
    command: ["opctl", "serve", "--host", "0.0.0.0", "--port", "9529", "--otel", "--otel-endpoint", "jaeger:4318", "--otel-service", "opc-frontend"]
    ports:
      - "9529:9529"
    volumes:
      - opc-frontend-data:/data/opc
    depends_on:
      - jaeger
    restart: unless-stopped

  opc-backend:
    build:
      context: ..
      dockerfile: Dockerfile
    container_name: opc-backend
    command: ["opctl", "serve", "--host", "0.0.0.0", "--port", "9530", "--otel", "--otel-endpoint", "jaeger:4318", "--otel-service", "opc-backend"]
    ports:
      - "9530:9530"
    volumes:
      - opc-backend-data:/data/opc
    depends_on:
      - jaeger
    restart: unless-stopped

volumes:
  opc-master-data:
  opc-design-data:
  opc-frontend-data:
  opc-backend-data:
```

- [ ] **Step 2: Verify compose syntax**

```bash
docker compose -f docker/docker-compose.observability.yaml config
```

Expected: valid YAML output

- [ ] **Step 3: Commit**

```bash
git add docker/docker-compose.observability.yaml
git commit -m "feat(docker): Jaeger + multi-OPC observability compose stack"
```

---

## Task 10: Federation Workflow Example

**Files:**
- Create: `examples/federation-workflow/README.md`
- Create: `examples/federation-workflow/start-federation.sh`
- Create: `examples/federation-workflow/stop-federation.sh`
- Create: `examples/federation-workflow/goal-login-feature.json`
- Create: `examples/federation-workflow/register-companies.sh`

- [ ] **Step 1: Create start script**

Create `examples/federation-workflow/start-federation.sh`:

```bash
#!/usr/bin/env bash
set -euo pipefail

# Start 4 OPC instances locally (master + 3 team nodes).
# Each uses a separate state directory and port.

BASE_DIR="${HOME}/.opc-federation-demo"
OPCTL="${OPCTL:-opctl}"

echo "=== OPC Federation Workflow Demo ==="
echo "Starting 4 OPC instances..."

# Cleanup previous state.
rm -rf "${BASE_DIR}"
mkdir -p "${BASE_DIR}"/{master,design,frontend,backend}/state

# Start instances in background with isolated state dirs.
$OPCTL serve --port 9527 --host 0.0.0.0 --state-dir "${BASE_DIR}/master/state" \
  --otel --otel-endpoint localhost:4318 --otel-service opc-master &
echo "  Master  → :9527 (PID $!)"

$OPCTL serve --port 9528 --host 0.0.0.0 --state-dir "${BASE_DIR}/design/state" \
  --otel --otel-endpoint localhost:4318 --otel-service opc-design &
echo "  Design  → :9528 (PID $!)"

$OPCTL serve --port 9529 --host 0.0.0.0 --state-dir "${BASE_DIR}/frontend/state" \
  --otel --otel-endpoint localhost:4318 --otel-service opc-frontend &
echo "  Frontend → :9529 (PID $!)"

$OPCTL serve --port 9530 --host 0.0.0.0 --state-dir "${BASE_DIR}/backend/state" \
  --otel --otel-endpoint localhost:4318 --otel-service opc-backend &
echo "  Backend  → :9530 (PID $!)"

echo ""
echo "All instances started. Waiting for health checks..."
sleep 3

# Health check.
for port in 9527 9528 9529 9530; do
  if curl -sf "http://localhost:${port}/api/health" > /dev/null 2>&1; then
    echo "  :${port} ✓"
  else
    echo "  :${port} ✗ (may need more time)"
  fi
done

echo ""
echo "Next steps:"
echo "  1. Run: bash register-companies.sh"
echo "  2. Run: bash dispatch-goal.sh"
echo "  3. Open Jaeger UI: http://localhost:16686"
echo ""
echo "To stop: bash stop-federation.sh"
```

- [ ] **Step 2: Create register-companies script**

Create `examples/federation-workflow/register-companies.sh`:

```bash
#!/usr/bin/env bash
set -euo pipefail

MASTER="http://localhost:9527"

echo "=== Registering federation companies ==="

# Register Design team.
curl -sf -X POST "${MASTER}/api/federation/companies" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "design-team",
    "endpoint": "http://localhost:9528",
    "type": "software",
    "agents": ["designer"]
  }' | jq .
echo "  design-team registered"

# Register Frontend team.
curl -sf -X POST "${MASTER}/api/federation/companies" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "frontend-team",
    "endpoint": "http://localhost:9529",
    "type": "software",
    "agents": ["coder"]
  }' | jq .
echo "  frontend-team registered"

# Register Backend team.
curl -sf -X POST "${MASTER}/api/federation/companies" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "backend-team",
    "endpoint": "http://localhost:9530",
    "type": "software",
    "agents": ["coder"]
  }' | jq .
echo "  backend-team registered"

echo ""
echo "Federation ready. Companies:"
curl -sf "${MASTER}/api/federation/companies" | jq '.[] | {id, name, endpoint, status}'
```

- [ ] **Step 3: Create goal dispatch example**

Create `examples/federation-workflow/goal-login-feature.json`:

```json
{
  "name": "实现用户登录功能",
  "description": "完整的用户登录功能，包含 UI 设计、前后端开发。设计团队出设计稿和交互标注，前端根据设计稿实现 UI，后端实现 REST API，前后端需要先对齐接口文档。",
  "projects": [
    {
      "name": "ui-design",
      "companyId": "<DESIGN_COMPANY_ID>",
      "agent": "designer",
      "description": "设计登录页面，输出：1) 登录页高保真设计稿 2) 交互标注文档 3) 切图资源清单"
    },
    {
      "name": "api-spec",
      "companyId": "<BACKEND_COMPANY_ID>",
      "agent": "coder",
      "description": "根据登录功能需求，定义 REST API 接口文档（OpenAPI 格式），包含：POST /api/auth/login, POST /api/auth/register, GET /api/auth/me",
      "dependencies": ["ui-design"]
    },
    {
      "name": "frontend-dev",
      "companyId": "<FRONTEND_COMPANY_ID>",
      "agent": "coder",
      "description": "根据 UI 设计稿和 API 接口文档，实现前端登录页面",
      "dependencies": ["ui-design", "api-spec"]
    },
    {
      "name": "backend-dev",
      "companyId": "<BACKEND_COMPANY_ID>",
      "agent": "coder",
      "description": "根据 API 接口文档，实现后端登录 REST API",
      "dependencies": ["api-spec"]
    }
  ]
}
```

Create `examples/federation-workflow/dispatch-goal.sh`:

```bash
#!/usr/bin/env bash
set -euo pipefail

MASTER="http://localhost:9527"

echo "=== Fetching company IDs ==="
COMPANIES=$(curl -sf "${MASTER}/api/federation/companies")

DESIGN_ID=$(echo "$COMPANIES" | jq -r '.[] | select(.name=="design-team") | .id')
FRONTEND_ID=$(echo "$COMPANIES" | jq -r '.[] | select(.name=="frontend-team") | .id')
BACKEND_ID=$(echo "$COMPANIES" | jq -r '.[] | select(.name=="backend-team") | .id')

echo "  Design:   ${DESIGN_ID}"
echo "  Frontend: ${FRONTEND_ID}"
echo "  Backend:  ${BACKEND_ID}"

echo ""
echo "=== Dispatching federated goal ==="

# Read template and substitute company IDs.
GOAL=$(cat goal-login-feature.json \
  | sed "s/<DESIGN_COMPANY_ID>/${DESIGN_ID}/g" \
  | sed "s/<FRONTEND_COMPANY_ID>/${FRONTEND_ID}/g" \
  | sed "s/<BACKEND_COMPANY_ID>/${BACKEND_ID}/g")

curl -sf -X POST "${MASTER}/api/federation/goals" \
  -H "Content-Type: application/json" \
  -d "$GOAL" | jq .

echo ""
echo "Goal dispatched! Monitor progress:"
echo "  Jaeger UI:    http://localhost:16686"
echo "  Master API:   curl ${MASTER}/api/goals | jq"
echo "  Design node:  curl http://localhost:9528/api/tasks | jq"
echo "  Frontend node: curl http://localhost:9529/api/tasks | jq"
echo "  Backend node:  curl http://localhost:9530/api/tasks | jq"
```

- [ ] **Step 4: Create stop script**

Create `examples/federation-workflow/stop-federation.sh`:

```bash
#!/usr/bin/env bash
set -euo pipefail

echo "=== Stopping OPC Federation Demo ==="

# Kill all opctl serve processes on demo ports.
for port in 9527 9528 9529 9530; do
  pid=$(lsof -ti :"${port}" 2>/dev/null || true)
  if [ -n "$pid" ]; then
    kill "$pid" 2>/dev/null && echo "  Stopped :${port} (PID ${pid})" || true
  fi
done

echo "All instances stopped."
```

- [ ] **Step 5: Create README**

Create `examples/federation-workflow/README.md`:

```markdown
# Federation Workflow Example

Multi-OPC federation demo: master dispatches a goal with project dependencies across team nodes.

## Architecture

```
Master (:9527)
  ├─ dispatch → Design (:9528)   [no deps, starts immediately]
  │                ↓ callback
  ├─ dispatch → Backend (:9530)  [depends: ui-design]
  │                ↓ callback
  ├─ dispatch → Frontend (:9529) [depends: ui-design, api-spec]
  │                ↓ callback
  └─ dispatch → Backend (:9530)  [depends: api-spec]

Jaeger (:16686) ← all instances report traces via OTLP
```

## Quick Start

### Prerequisites
- `opctl` binary built: `go build -o opctl ./cmd/opctl`
- Docker (for Jaeger): `docker run -d -p 16686:16686 -p 4318:4318 -e COLLECTOR_OTLP_ENABLED=true jaegertracing/all-in-one:1.54`
- `jq` installed

### Run

```bash
# 1. Start Jaeger (if not using docker-compose)
docker run -d --name jaeger -p 16686:16686 -p 4318:4318 \
  -e COLLECTOR_OTLP_ENABLED=true jaegertracing/all-in-one:1.54

# 2. Start OPC instances
bash start-federation.sh

# 3. Register companies in federation
bash register-companies.sh

# 4. Dispatch the goal
bash dispatch-goal.sh

# 5. Watch traces in Jaeger
open http://localhost:16686

# 6. Cleanup
bash stop-federation.sh
docker stop jaeger && docker rm jaeger
```

### Or with Docker Compose (all-in-one)

```bash
docker compose -f ../../docker/docker-compose.observability.yaml up -d
```

## Traceability

Every issue carries:
- `traceId` — root goal ID (same across all nodes)
- `spanId` — unique issue identifier
- `parentSpans` — upstream issue IDs
- `lineage` — flat array of all ancestors with human-readable labels

Query lineage for any task:
```bash
curl http://localhost:9529/api/tasks/<taskId> | jq '.lineage'
```

Trace in Jaeger: search by service `opc-master`, then follow the trace across `opc-design`, `opc-frontend`, `opc-backend`.
```

- [ ] **Step 6: Make scripts executable**

```bash
chmod +x examples/federation-workflow/*.sh
```

- [ ] **Step 7: Commit**

```bash
git add examples/federation-workflow/
git commit -m "feat(examples): federation workflow demo with multi-OPC + Jaeger tracing"
```

---

## Task 11: Integration Verification

- [ ] **Step 1: Full build**

```bash
go build ./...
```

Expected: clean build

- [ ] **Step 2: Run all tests**

```bash
go test ./... -v
```

Expected: all tests pass (including previously failing TestTaskCRUD)

- [ ] **Step 3: Manual smoke test — local federation**

```bash
# Terminal 1: Start Jaeger
docker run -d --name jaeger-test -p 16686:16686 -p 4318:4318 \
  -e COLLECTOR_OTLP_ENABLED=true jaegertracing/all-in-one:1.54

# Terminal 2: Start master with OTel
./opctl serve --port 9527 --otel --log-level debug

# Terminal 3: Start design node
./opctl serve --port 9528 --otel --otel-service opc-design --log-level info

# Terminal 4: Register and dispatch
curl -X POST http://localhost:9527/api/federation/companies \
  -H "Content-Type: application/json" \
  -d '{"name":"design","endpoint":"http://localhost:9528","type":"software","agents":["coder"]}'

# Verify trace appears in Jaeger UI at http://localhost:16686
```

- [ ] **Step 4: Verify log levels**

```bash
# Should show debug logs:
./opctl serve --port 9999 --log-level debug 2>&1 | head -5

# Should NOT show debug logs:
./opctl serve --port 9998 --log-level error 2>&1 | head -5
```

- [ ] **Step 5: Cleanup and final commit**

```bash
docker stop jaeger-test && docker rm jaeger-test
git add -A
git commit -m "feat: traceability (lineage+trace), OpenTelemetry/Jaeger, configurable logging"
```

---

## Summary

| Task | What | Files | Est. Lines Changed |
|------|------|-------|-------------------|
| 1 | Configurable log level | config.go, root.go, serve.go | ~50 |
| 2 | OTel tracer provider | pkg/trace/tracer.go | ~100 |
| 3 | Lineage data model | lineage.go, types.go | ~80 |
| 4 | DB schema migrations | sqlite.go | ~120 |
| 5 | Audit IssueRef | audit.go | ~15 |
| 6 | Fix runTask traceability | server.go | ~30 |
| 7 | OTel spans on federation | server.go | ~60 |
| 8 | Wire OTel into serve | serve.go | ~30 |
| 9 | Docker Compose + Jaeger | docker-compose.yaml | ~70 |
| 10 | Federation example | examples/ | ~200 |
| 11 | Integration verification | — | ~0 |

**Total: ~755 lines across 11 tasks**
