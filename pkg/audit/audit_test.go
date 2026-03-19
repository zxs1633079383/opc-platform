package audit

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"go.uber.org/zap"
)

func nopLogger() *zap.SugaredLogger {
	return zap.NewNop().Sugar()
}

// ---------------------------------------------------------------------------
// 1. NewLogger
// ---------------------------------------------------------------------------

func TestNewLogger(t *testing.T) {
	t.Run("empty dir creates logger with no events", func(t *testing.T) {
		dir := t.TempDir()
		l := NewLogger(dir, nopLogger())

		if l == nil {
			t.Fatal("expected non-nil Logger")
		}
		if len(l.events) != 0 {
			t.Fatalf("expected 0 events, got %d", len(l.events))
		}
	})

	t.Run("loads existing JSONL on startup", func(t *testing.T) {
		dir := t.TempDir()
		event := AuditEvent{
			ID:           "evt-1",
			Timestamp:    time.Now().UTC(),
			EventType:    EventCreated,
			ResourceType: ResourceAgent,
			ResourceName: "agent-a",
		}
		data, _ := json.Marshal(event)
		data = append(data, '\n')
		if err := os.WriteFile(filepath.Join(dir, auditFileName), data, 0o644); err != nil {
			t.Fatal(err)
		}

		l := NewLogger(dir, nopLogger())
		if len(l.events) != 1 {
			t.Fatalf("expected 1 event, got %d", len(l.events))
		}
		if l.events[0].ID != "evt-1" {
			t.Fatalf("expected ID evt-1, got %s", l.events[0].ID)
		}
	})

	t.Run("skips malformed lines", func(t *testing.T) {
		dir := t.TempDir()
		good := AuditEvent{
			ID:           "evt-ok",
			Timestamp:    time.Now().UTC(),
			EventType:    EventStarted,
			ResourceType: ResourceTask,
			ResourceName: "task-1",
		}
		goodData, _ := json.Marshal(good)

		content := append(goodData, '\n')
		content = append(content, []byte("NOT VALID JSON\n")...)
		content = append(content, []byte("\n")...) // empty line
		if err := os.WriteFile(filepath.Join(dir, auditFileName), content, 0o644); err != nil {
			t.Fatal(err)
		}

		l := NewLogger(dir, nopLogger())
		if len(l.events) != 1 {
			t.Fatalf("expected 1 event (malformed skipped), got %d", len(l.events))
		}
		if l.events[0].ID != "evt-ok" {
			t.Fatalf("expected ID evt-ok, got %s", l.events[0].ID)
		}
	})

	t.Run("creates nested directory if missing", func(t *testing.T) {
		dir := filepath.Join(t.TempDir(), "a", "b", "c")
		l := NewLogger(dir, nopLogger())
		if l == nil {
			t.Fatal("expected non-nil Logger")
		}
		info, err := os.Stat(dir)
		if err != nil {
			t.Fatalf("expected directory to be created: %v", err)
		}
		if !info.IsDir() {
			t.Fatal("expected a directory")
		}
	})
}

// ---------------------------------------------------------------------------
// 2. Log
// ---------------------------------------------------------------------------

func TestLog(t *testing.T) {
	t.Run("normal event is recorded", func(t *testing.T) {
		dir := t.TempDir()
		l := NewLogger(dir, nopLogger())

		err := l.Log(AuditEvent{
			EventType:    EventCreated,
			ResourceType: ResourceAgent,
			ResourceName: "agent-1",
			Details:      "created agent",
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(l.events) != 1 {
			t.Fatalf("expected 1 event, got %d", len(l.events))
		}
	})

	t.Run("auto-fills ID when empty", func(t *testing.T) {
		dir := t.TempDir()
		l := NewLogger(dir, nopLogger())

		_ = l.Log(AuditEvent{
			EventType:    EventStarted,
			ResourceType: ResourceTask,
			ResourceName: "task-1",
		})

		if l.events[0].ID == "" {
			t.Fatal("expected ID to be auto-filled")
		}
	})

	t.Run("auto-fills timestamp when zero", func(t *testing.T) {
		dir := t.TempDir()
		l := NewLogger(dir, nopLogger())

		before := time.Now().UTC().Add(-time.Second)
		_ = l.Log(AuditEvent{
			EventType:    EventCompleted,
			ResourceType: ResourceGoal,
			ResourceName: "goal-1",
		})
		after := time.Now().UTC().Add(time.Second)

		ts := l.events[0].Timestamp
		if ts.Before(before) || ts.After(after) {
			t.Fatalf("timestamp %v not in expected range [%v, %v]", ts, before, after)
		}
	})

	t.Run("preserves explicitly set ID and timestamp", func(t *testing.T) {
		dir := t.TempDir()
		l := NewLogger(dir, nopLogger())

		fixed := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
		_ = l.Log(AuditEvent{
			ID:           "my-custom-id",
			Timestamp:    fixed,
			EventType:    EventFailed,
			ResourceType: ResourceWorkflow,
			ResourceName: "wf-1",
		})

		if l.events[0].ID != "my-custom-id" {
			t.Fatalf("expected custom ID, got %s", l.events[0].ID)
		}
		if !l.events[0].Timestamp.Equal(fixed) {
			t.Fatalf("expected custom timestamp, got %v", l.events[0].Timestamp)
		}
	})

	t.Run("persists event to disk", func(t *testing.T) {
		dir := t.TempDir()
		l := NewLogger(dir, nopLogger())

		_ = l.Log(AuditEvent{
			ID:           "disk-evt",
			EventType:    EventCreated,
			ResourceType: ResourceAgent,
			ResourceName: "agent-x",
		})

		data, err := os.ReadFile(filepath.Join(dir, auditFileName))
		if err != nil {
			t.Fatalf("failed to read audit file: %v", err)
		}
		if len(data) == 0 {
			t.Fatal("expected non-empty audit file")
		}

		var persisted AuditEvent
		if err := json.Unmarshal(data[:len(data)-1], &persisted); err != nil {
			t.Fatalf("failed to unmarshal persisted event: %v", err)
		}
		if persisted.ID != "disk-evt" {
			t.Fatalf("expected persisted ID disk-evt, got %s", persisted.ID)
		}
	})
}

// ---------------------------------------------------------------------------
// 3. ListEvents
// ---------------------------------------------------------------------------

func TestListEvents(t *testing.T) {
	dir := t.TempDir()
	l := NewLogger(dir, nopLogger())

	events := []AuditEvent{
		{EventType: EventCreated, ResourceType: ResourceAgent, ResourceName: "agent-1"},
		{EventType: EventStarted, ResourceType: ResourceAgent, ResourceName: "agent-2"},
		{EventType: EventCompleted, ResourceType: ResourceTask, ResourceName: "task-1"},
		{EventType: EventFailed, ResourceType: ResourceTask, ResourceName: "task-1"},
		{EventType: EventCreated, ResourceType: ResourceGoal, ResourceName: "goal-1"},
	}
	for _, e := range events {
		if err := l.Log(e); err != nil {
			t.Fatalf("failed to log event: %v", err)
		}
	}

	t.Run("filter by resource type only", func(t *testing.T) {
		result, err := l.ListEvents(ResourceAgent, "")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(result) != 2 {
			t.Fatalf("expected 2 agent events, got %d", len(result))
		}
	})

	t.Run("filter by type and name", func(t *testing.T) {
		result, err := l.ListEvents(ResourceTask, "task-1")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(result) != 2 {
			t.Fatalf("expected 2 task-1 events, got %d", len(result))
		}
	})

	t.Run("empty name returns all of type", func(t *testing.T) {
		result, err := l.ListEvents(ResourceTask, "")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(result) != 2 {
			t.Fatalf("expected 2 task events, got %d", len(result))
		}
	})

	t.Run("no matches returns empty slice", func(t *testing.T) {
		result, err := l.ListEvents(ResourceWorkflow, "")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(result) != 0 {
			t.Fatalf("expected 0 events, got %d", len(result))
		}
	})

	t.Run("no matches for existing type wrong name", func(t *testing.T) {
		result, err := l.ListEvents(ResourceAgent, "nonexistent")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(result) != 0 {
			t.Fatalf("expected 0 events, got %d", len(result))
		}
	})
}

// ---------------------------------------------------------------------------
// 4. ListByGoal
// ---------------------------------------------------------------------------

func TestListByGoal(t *testing.T) {
	dir := t.TempDir()
	l := NewLogger(dir, nopLogger())

	_ = l.Log(AuditEvent{
		EventType:    EventCreated,
		ResourceType: ResourceGoal,
		ResourceName: "goal-alpha",
	})
	_ = l.Log(AuditEvent{
		EventType:    EventStarted,
		ResourceType: ResourceTask,
		ResourceName: "task-1",
		GoalRef:      "goal-alpha",
	})
	_ = l.Log(AuditEvent{
		EventType:    EventCompleted,
		ResourceType: ResourceAgent,
		ResourceName: "agent-1",
		GoalRef:      "goal-beta",
	})

	t.Run("direct goal events", func(t *testing.T) {
		result, err := l.ListByGoal("goal-alpha")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		// Should include the goal resource event + the task with GoalRef
		if len(result) != 2 {
			t.Fatalf("expected 2 events, got %d", len(result))
		}
	})

	t.Run("events with GoalRef only", func(t *testing.T) {
		result, err := l.ListByGoal("goal-beta")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(result) != 1 {
			t.Fatalf("expected 1 event, got %d", len(result))
		}
		if result[0].ResourceName != "agent-1" {
			t.Fatalf("expected agent-1, got %s", result[0].ResourceName)
		}
	})

	t.Run("no matches", func(t *testing.T) {
		result, err := l.ListByGoal("goal-nonexistent")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(result) != 0 {
			t.Fatalf("expected 0 events, got %d", len(result))
		}
	})
}

// ---------------------------------------------------------------------------
// 5. Trace
// ---------------------------------------------------------------------------

func TestTrace(t *testing.T) {
	t.Run("full hierarchy trace from issue", func(t *testing.T) {
		dir := t.TempDir()
		l := NewLogger(dir, nopLogger())

		// Goal event
		_ = l.Log(AuditEvent{
			ID:           "g1",
			EventType:    EventCreated,
			ResourceType: ResourceGoal,
			ResourceName: "goal-1",
		})
		// Project event
		_ = l.Log(AuditEvent{
			ID:           "p1",
			EventType:    EventCreated,
			ResourceType: ResourceProject,
			ResourceName: "proj-1",
			GoalRef:      "goal-1",
		})
		// Task event
		_ = l.Log(AuditEvent{
			ID:           "t1",
			EventType:    EventStarted,
			ResourceType: ResourceTask,
			ResourceName: "task-1",
			ProjectRef:   "proj-1",
			GoalRef:      "goal-1",
		})
		// Issue event (the one we trace)
		_ = l.Log(AuditEvent{
			ID:           "i1",
			EventType:    EventCreated,
			ResourceType: ResourceIssue,
			ResourceName: "issue-1",
			TaskRef:      "task-1",
			ProjectRef:   "proj-1",
			GoalRef:      "goal-1",
		})
		// Agent event referencing the issue
		_ = l.Log(AuditEvent{
			ID:           "a1",
			EventType:    EventAgentAssigned,
			ResourceType: ResourceAgent,
			ResourceName: "agent-1",
			IssueRef:     "issue-1",
			GoalRef:      "goal-1",
		})

		result, err := l.Trace(ResourceIssue, "issue-1")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		// Direct: i1
		// From issue's refs: goal-1, proj-1, task-1 events
		// From reverse ref scan: a1 (IssueRef == "issue-1")
		// So: i1 + g1 + p1 + t1 + a1 = 5
		if len(result) != 5 {
			ids := make([]string, len(result))
			for i, e := range result {
				ids[i] = e.ID
			}
			t.Fatalf("expected 5 events in trace, got %d: %v", len(result), ids)
		}

		// First event should be the direct one
		if result[0].ID != "i1" {
			t.Fatalf("expected first event to be i1, got %s", result[0].ID)
		}
	})

	t.Run("agent ref included in trace", func(t *testing.T) {
		dir := t.TempDir()
		l := NewLogger(dir, nopLogger())

		_ = l.Log(AuditEvent{
			ID:           "t1",
			EventType:    EventStarted,
			ResourceType: ResourceTask,
			ResourceName: "task-x",
			AgentRef:     "agent-z",
		})
		_ = l.Log(AuditEvent{
			ID:           "a1",
			EventType:    EventCreated,
			ResourceType: ResourceAgent,
			ResourceName: "agent-z",
		})

		result, err := l.Trace(ResourceTask, "task-x")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(result) != 2 {
			t.Fatalf("expected 2 events (task + agent), got %d", len(result))
		}
	})

	t.Run("no related events returns only direct", func(t *testing.T) {
		dir := t.TempDir()
		l := NewLogger(dir, nopLogger())

		_ = l.Log(AuditEvent{
			ID:           "solo",
			EventType:    EventCreated,
			ResourceType: ResourceAgent,
			ResourceName: "loner",
		})

		result, err := l.Trace(ResourceAgent, "loner")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(result) != 1 {
			t.Fatalf("expected 1 event, got %d", len(result))
		}
	})

	t.Run("trace nonexistent resource returns empty", func(t *testing.T) {
		dir := t.TempDir()
		l := NewLogger(dir, nopLogger())

		result, err := l.Trace(ResourceGoal, "no-such-goal")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(result) != 0 {
			t.Fatalf("expected 0 events, got %d", len(result))
		}
	})
}

// ---------------------------------------------------------------------------
// 6. Export
// ---------------------------------------------------------------------------

func TestExport(t *testing.T) {
	t.Run("json format works", func(t *testing.T) {
		dir := t.TempDir()
		l := NewLogger(dir, nopLogger())

		_ = l.Log(AuditEvent{
			ID:           "exp-1",
			EventType:    EventCreated,
			ResourceType: ResourceAgent,
			ResourceName: "agent-1",
		})
		_ = l.Log(AuditEvent{
			ID:           "exp-2",
			EventType:    EventCompleted,
			ResourceType: ResourceTask,
			ResourceName: "task-1",
		})

		data, err := l.Export("json")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		var exported []AuditEvent
		if err := json.Unmarshal(data, &exported); err != nil {
			t.Fatalf("failed to unmarshal exported JSON: %v", err)
		}
		if len(exported) != 2 {
			t.Fatalf("expected 2 exported events, got %d", len(exported))
		}
		if exported[0].ID != "exp-1" {
			t.Fatalf("expected first exported ID exp-1, got %s", exported[0].ID)
		}
	})

	t.Run("json format on empty logger", func(t *testing.T) {
		dir := t.TempDir()
		l := NewLogger(dir, nopLogger())

		data, err := l.Export("json")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		var exported []AuditEvent
		if err := json.Unmarshal(data, &exported); err != nil {
			t.Fatalf("failed to unmarshal: %v", err)
		}
		if len(exported) != 0 {
			t.Fatalf("expected 0 events, got %d", len(exported))
		}
	})

	t.Run("unsupported format returns error", func(t *testing.T) {
		dir := t.TempDir()
		l := NewLogger(dir, nopLogger())

		_, err := l.Export("csv")
		if err == nil {
			t.Fatal("expected error for unsupported format")
		}
	})

	t.Run("unsupported format yaml", func(t *testing.T) {
		dir := t.TempDir()
		l := NewLogger(dir, nopLogger())

		_, err := l.Export("yaml")
		if err == nil {
			t.Fatal("expected error for unsupported format yaml")
		}
	})
}

// ---------------------------------------------------------------------------
// 7. Persistence
// ---------------------------------------------------------------------------

func TestPersistence(t *testing.T) {
	t.Run("events survive across Logger instances", func(t *testing.T) {
		dir := t.TempDir()
		l1 := NewLogger(dir, nopLogger())

		_ = l1.Log(AuditEvent{
			ID:           "persist-1",
			EventType:    EventCreated,
			ResourceType: ResourceGoal,
			ResourceName: "goal-persist",
			Details:      "first logger",
		})
		_ = l1.Log(AuditEvent{
			ID:           "persist-2",
			EventType:    EventStarted,
			ResourceType: ResourceTask,
			ResourceName: "task-persist",
			GoalRef:      "goal-persist",
		})

		// Create new Logger from the same directory
		l2 := NewLogger(dir, nopLogger())

		if len(l2.events) != 2 {
			t.Fatalf("expected 2 events loaded, got %d", len(l2.events))
		}

		// Verify data integrity
		if l2.events[0].ID != "persist-1" {
			t.Fatalf("expected persist-1, got %s", l2.events[0].ID)
		}
		if l2.events[1].GoalRef != "goal-persist" {
			t.Fatalf("expected goalRef goal-persist, got %s", l2.events[1].GoalRef)
		}

		// Queries should work on the reloaded logger
		result, _ := l2.ListByGoal("goal-persist")
		if len(result) != 2 {
			t.Fatalf("expected 2 events from ListByGoal on reloaded logger, got %d", len(result))
		}
	})

	t.Run("new events appended after reload", func(t *testing.T) {
		dir := t.TempDir()
		l1 := NewLogger(dir, nopLogger())
		_ = l1.Log(AuditEvent{
			ID:           "first",
			EventType:    EventCreated,
			ResourceType: ResourceAgent,
			ResourceName: "a1",
		})

		l2 := NewLogger(dir, nopLogger())
		_ = l2.Log(AuditEvent{
			ID:           "second",
			EventType:    EventStarted,
			ResourceType: ResourceAgent,
			ResourceName: "a2",
		})

		// Third logger should see both
		l3 := NewLogger(dir, nopLogger())
		if len(l3.events) != 2 {
			t.Fatalf("expected 2 events, got %d", len(l3.events))
		}
	})
}

// ---------------------------------------------------------------------------
// 8. Concurrent safety
// ---------------------------------------------------------------------------

func TestConcurrentSafety(t *testing.T) {
	dir := t.TempDir()
	l := NewLogger(dir, nopLogger())

	const goroutines = 20
	const eventsPerGoroutine = 10
	var wg sync.WaitGroup

	// Writers
	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < eventsPerGoroutine; j++ {
				_ = l.Log(AuditEvent{
					EventType:    EventCreated,
					ResourceType: ResourceAgent,
					ResourceName: "concurrent-agent",
				})
			}
		}(i)
	}

	// Readers running concurrently with writers
	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < eventsPerGoroutine; j++ {
				_, _ = l.ListEvents(ResourceAgent, "concurrent-agent")
				_, _ = l.ListByGoal("nonexistent")
				_, _ = l.Export("json")
			}
		}()
	}

	wg.Wait()

	total := goroutines * eventsPerGoroutine
	if len(l.events) != total {
		t.Fatalf("expected %d events, got %d", total, len(l.events))
	}

	// Verify all events can be listed
	result, err := l.ListEvents(ResourceAgent, "concurrent-agent")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result) != total {
		t.Fatalf("expected %d events from ListEvents, got %d", total, len(result))
	}
}
