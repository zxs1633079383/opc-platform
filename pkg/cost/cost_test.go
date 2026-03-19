package cost

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"go.uber.org/zap"
)

func nopLogger() *zap.SugaredLogger {
	return zap.NewNop().Sugar()
}

// ---------------------------------------------------------------------------
// 1. NewTracker
// ---------------------------------------------------------------------------

func TestNewTracker(t *testing.T) {
	t.Run("empty dir creates tracker with no events", func(t *testing.T) {
		dir := t.TempDir()
		tr := NewTracker(dir, nopLogger())

		if got := len(tr.events); got != 0 {
			t.Fatalf("expected 0 events, got %d", got)
		}
	})

	t.Run("loads existing JSONL events from disk", func(t *testing.T) {
		dir := t.TempDir()

		events := []CostEvent{
			{ID: "e1", Timestamp: time.Now().Add(-time.Hour), AgentName: "a1", TotalCost: 0.5, TotalTokens: 100},
			{ID: "e2", Timestamp: time.Now(), AgentName: "a2", TotalCost: 1.0, TotalTokens: 200},
		}
		writeJSONL(t, dir, events)

		tr := NewTracker(dir, nopLogger())
		if got := len(tr.events); got != 2 {
			t.Fatalf("expected 2 events, got %d", got)
		}
		if tr.events[0].ID != "e1" || tr.events[1].ID != "e2" {
			t.Fatalf("unexpected event IDs: %s, %s", tr.events[0].ID, tr.events[1].ID)
		}
	})

	t.Run("skips malformed JSONL lines", func(t *testing.T) {
		dir := t.TempDir()

		good := CostEvent{ID: "good", AgentName: "a1", TotalCost: 1.0}
		goodJSON, _ := json.Marshal(good)

		content := string(goodJSON) + "\n" + "NOT VALID JSON\n" + string(goodJSON) + "\n"
		path := filepath.Join(dir, costEventsFile)
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}

		tr := NewTracker(dir, nopLogger())
		if got := len(tr.events); got != 2 {
			t.Fatalf("expected 2 events (skipping malformed), got %d", got)
		}
	})
}

// ---------------------------------------------------------------------------
// 2. RecordCost
// ---------------------------------------------------------------------------

func TestRecordCost(t *testing.T) {
	t.Run("normal recording", func(t *testing.T) {
		dir := t.TempDir()
		tr := NewTracker(dir, nopLogger())

		err := tr.RecordCost(CostEvent{
			AgentName: "coder",
			TokensIn:  500,
			TokensOut: 300,
			InputCost: 0.01,
			OutputCost: 0.02,
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if got := len(tr.events); got != 1 {
			t.Fatalf("expected 1 event, got %d", got)
		}
	})

	t.Run("auto-fills ID and timestamp", func(t *testing.T) {
		dir := t.TempDir()
		tr := NewTracker(dir, nopLogger())

		before := time.Now()
		err := tr.RecordCost(CostEvent{AgentName: "test"})
		if err != nil {
			t.Fatal(err)
		}

		ev := tr.events[0]
		if ev.ID == "" {
			t.Fatal("expected auto-generated ID")
		}
		if ev.Timestamp.Before(before) {
			t.Fatal("expected auto-filled timestamp to be >= before")
		}
	})

	t.Run("computes TotalTokens and TotalCost", func(t *testing.T) {
		dir := t.TempDir()
		tr := NewTracker(dir, nopLogger())

		err := tr.RecordCost(CostEvent{
			TokensIn:   1000,
			TokensOut:  2000,
			InputCost:  0.003,
			OutputCost: 0.030,
		})
		if err != nil {
			t.Fatal(err)
		}

		ev := tr.events[0]
		if ev.TotalTokens != 3000 {
			t.Fatalf("expected TotalTokens 3000, got %d", ev.TotalTokens)
		}
		if !floatEq(ev.TotalCost, 0.033) {
			t.Fatalf("expected TotalCost 0.033, got %f", ev.TotalCost)
		}
	})

	t.Run("persists to disk", func(t *testing.T) {
		dir := t.TempDir()
		tr := NewTracker(dir, nopLogger())

		err := tr.RecordCost(CostEvent{AgentName: "persist-test", InputCost: 0.01, OutputCost: 0.02})
		if err != nil {
			t.Fatal(err)
		}

		// Create a new tracker from same dir and verify the event is loaded.
		tr2 := NewTracker(dir, nopLogger())
		if got := len(tr2.events); got != 1 {
			t.Fatalf("expected 1 persisted event, got %d", got)
		}
		if tr2.events[0].AgentName != "persist-test" {
			t.Fatalf("unexpected agent name: %s", tr2.events[0].AgentName)
		}
	})

	t.Run("preserves provided ID and timestamp", func(t *testing.T) {
		dir := t.TempDir()
		tr := NewTracker(dir, nopLogger())

		ts := time.Date(2025, 6, 15, 12, 0, 0, 0, time.UTC)
		err := tr.RecordCost(CostEvent{ID: "custom-id", Timestamp: ts})
		if err != nil {
			t.Fatal(err)
		}

		ev := tr.events[0]
		if ev.ID != "custom-id" {
			t.Fatalf("expected custom-id, got %s", ev.ID)
		}
		if !ev.Timestamp.Equal(ts) {
			t.Fatalf("expected preserved timestamp")
		}
	})
}

// ---------------------------------------------------------------------------
// 3. CalculateCost
// ---------------------------------------------------------------------------

func TestCalculateCost(t *testing.T) {
	t.Run("exact match anthropic/claude-sonnet-4-6", func(t *testing.T) {
		dir := t.TempDir()
		tr := NewTracker(dir, nopLogger())

		inCost, outCost := tr.CalculateCost(1000, 1000, "anthropic", "claude-sonnet-4-6")
		// InputPer1K = 0.003, OutputPer1K = 0.015
		expectedIn := 1000.0 / 1000.0 * 0.003
		expectedOut := 1000.0 / 1000.0 * 0.015
		if !floatEq(inCost, expectedIn) {
			t.Fatalf("expected input cost %f, got %f", expectedIn, inCost)
		}
		if !floatEq(outCost, expectedOut) {
			t.Fatalf("expected output cost %f, got %f", expectedOut, outCost)
		}
	})

	t.Run("fuzzy prefix match claude-sonnet-4-6-20250514", func(t *testing.T) {
		dir := t.TempDir()
		tr := NewTracker(dir, nopLogger())

		inCost, outCost := tr.CalculateCost(2000, 500, "anthropic", "claude-sonnet-4-6-20250514")
		expectedIn := 2000.0 / 1000.0 * 0.003
		expectedOut := 500.0 / 1000.0 * 0.015
		if !floatEq(inCost, expectedIn) {
			t.Fatalf("expected input cost %f, got %f", expectedIn, inCost)
		}
		if !floatEq(outCost, expectedOut) {
			t.Fatalf("expected output cost %f, got %f", expectedOut, outCost)
		}
	})

	t.Run("unknown model returns 0", func(t *testing.T) {
		dir := t.TempDir()
		tr := NewTracker(dir, nopLogger())

		inCost, outCost := tr.CalculateCost(1000, 1000, "unknown", "unknown-model")
		if inCost != 0 || outCost != 0 {
			t.Fatalf("expected 0,0 for unknown model, got %f,%f", inCost, outCost)
		}
	})
}

// ---------------------------------------------------------------------------
// 4. SetBudget + GetBudgetStatus
// ---------------------------------------------------------------------------

func TestSetBudgetAndGetBudgetStatus(t *testing.T) {
	t.Run("daily limit not exceeded", func(t *testing.T) {
		dir := t.TempDir()
		tr := NewTracker(dir, nopLogger())

		tr.SetBudget(BudgetConfig{DailyLimit: 10.0, MonthlyLimit: 100.0})
		// Record a small cost event with today's timestamp.
		_ = tr.RecordCost(CostEvent{InputCost: 1.0, OutputCost: 1.0})

		status := tr.GetBudgetStatus()
		if status.Exceeded {
			t.Fatal("budget should not be exceeded")
		}
		if !floatEq(status.DailySpent, 2.0) {
			t.Fatalf("expected daily spent 2.0, got %f", status.DailySpent)
		}
	})

	t.Run("daily limit exceeded", func(t *testing.T) {
		dir := t.TempDir()
		tr := NewTracker(dir, nopLogger())

		tr.SetBudget(BudgetConfig{DailyLimit: 1.0, MonthlyLimit: 100.0})
		_ = tr.RecordCost(CostEvent{InputCost: 0.5, OutputCost: 0.6})

		status := tr.GetBudgetStatus()
		if !status.Exceeded {
			t.Fatal("daily budget should be exceeded")
		}
	})

	t.Run("monthly limit exceeded", func(t *testing.T) {
		dir := t.TempDir()
		tr := NewTracker(dir, nopLogger())

		tr.SetBudget(BudgetConfig{DailyLimit: 100.0, MonthlyLimit: 1.0})
		_ = tr.RecordCost(CostEvent{InputCost: 0.5, OutputCost: 0.6})

		status := tr.GetBudgetStatus()
		if !status.Exceeded {
			t.Fatal("monthly budget should be exceeded")
		}
	})

	t.Run("no budget set", func(t *testing.T) {
		dir := t.TempDir()
		tr := NewTracker(dir, nopLogger())

		_ = tr.RecordCost(CostEvent{InputCost: 100.0, OutputCost: 100.0})

		status := tr.GetBudgetStatus()
		if status.Exceeded {
			t.Fatal("no budget set, should not be exceeded")
		}
		if status.DailyPct != 0 || status.MonthlyPct != 0 {
			t.Fatal("percentages should be 0 with no limits")
		}
	})
}

// ---------------------------------------------------------------------------
// 5. CheckBudget
// ---------------------------------------------------------------------------

func TestCheckBudget(t *testing.T) {
	t.Run("exceeded returns true with message", func(t *testing.T) {
		dir := t.TempDir()
		tr := NewTracker(dir, nopLogger())

		tr.SetBudget(BudgetConfig{DailyLimit: 0.5, MonthlyLimit: 100.0})
		_ = tr.RecordCost(CostEvent{InputCost: 0.3, OutputCost: 0.3})

		exceeded, msg := tr.CheckBudget()
		if !exceeded {
			t.Fatal("expected exceeded")
		}
		if !strings.Contains(msg, "daily budget exceeded") {
			t.Fatalf("expected daily exceeded message, got: %s", msg)
		}
	})

	t.Run("alert threshold returns message", func(t *testing.T) {
		dir := t.TempDir()
		tr := NewTracker(dir, nopLogger())

		tr.SetBudget(BudgetConfig{DailyLimit: 10.0, MonthlyLimit: 100.0, AlertPct: 0.8})
		// Spend 85% of daily
		_ = tr.RecordCost(CostEvent{InputCost: 4.25, OutputCost: 4.25})

		exceeded, msg := tr.CheckBudget()
		if exceeded {
			t.Fatal("should not be exceeded, only alert")
		}
		if !strings.Contains(msg, "daily budget alert") {
			t.Fatalf("expected alert message, got: %s", msg)
		}
	})

	t.Run("no budget returns empty", func(t *testing.T) {
		dir := t.TempDir()
		tr := NewTracker(dir, nopLogger())

		exceeded, msg := tr.CheckBudget()
		if exceeded {
			t.Fatal("should not be exceeded")
		}
		if msg != "" {
			t.Fatalf("expected empty message, got: %s", msg)
		}
	})
}

// ---------------------------------------------------------------------------
// 6. GenerateReport
// ---------------------------------------------------------------------------

func TestGenerateReport(t *testing.T) {
	setup := func(t *testing.T) *Tracker {
		t.Helper()
		dir := t.TempDir()
		tr := NewTracker(dir, nopLogger())

		_ = tr.RecordCost(CostEvent{AgentName: "coder", GoalRef: "goal-1", ProjectRef: "proj-1", InputCost: 1.0, OutputCost: 2.0, TokensIn: 100, TokensOut: 200})
		_ = tr.RecordCost(CostEvent{AgentName: "reviewer", GoalRef: "goal-1", ProjectRef: "proj-2", InputCost: 0.5, OutputCost: 1.0, TokensIn: 50, TokensOut: 100})
		_ = tr.RecordCost(CostEvent{AgentName: "coder", GoalRef: "goal-2", ProjectRef: "proj-1", InputCost: 0.3, OutputCost: 0.7, TokensIn: 30, TokensOut: 70})

		return tr
	}

	t.Run("group by agent", func(t *testing.T) {
		tr := setup(t)
		report := tr.GenerateReport("agent", 24*time.Hour)

		if report.EventCount != 3 {
			t.Fatalf("expected 3 events, got %d", report.EventCount)
		}
		if report.ByAgent == nil {
			t.Fatal("ByAgent should not be nil")
		}
		if report.ByGoal != nil {
			t.Fatal("ByGoal should be nil for agent grouping")
		}
		// coder: 3.0 + 1.0 = 4.0
		if !floatEq(report.ByAgent["coder"], 4.0) {
			t.Fatalf("expected coder cost 4.0, got %f", report.ByAgent["coder"])
		}
		if !floatEq(report.ByAgent["reviewer"], 1.5) {
			t.Fatalf("expected reviewer cost 1.5, got %f", report.ByAgent["reviewer"])
		}
	})

	t.Run("group by goal", func(t *testing.T) {
		tr := setup(t)
		report := tr.GenerateReport("goal", 24*time.Hour)

		if report.ByGoal == nil {
			t.Fatal("ByGoal should not be nil")
		}
		if report.ByAgent != nil {
			t.Fatal("ByAgent should be nil for goal grouping")
		}
		// goal-1: 3.0 + 1.5 = 4.5
		if !floatEq(report.ByGoal["goal-1"], 4.5) {
			t.Fatalf("expected goal-1 cost 4.5, got %f", report.ByGoal["goal-1"])
		}
		if !floatEq(report.ByGoal["goal-2"], 1.0) {
			t.Fatalf("expected goal-2 cost 1.0, got %f", report.ByGoal["goal-2"])
		}
	})

	t.Run("time period filter excludes old events", func(t *testing.T) {
		dir := t.TempDir()
		tr := NewTracker(dir, nopLogger())

		// Insert an old event directly into the events slice.
		oldEvent := CostEvent{
			ID:        "old",
			Timestamp: time.Now().Add(-48 * time.Hour),
			AgentName: "old-agent",
			InputCost: 10.0,
			OutputCost: 10.0,
			TotalCost: 20.0,
			TotalTokens: 1000,
		}
		tr.mu.Lock()
		tr.events = append(tr.events, oldEvent)
		tr.mu.Unlock()

		_ = tr.RecordCost(CostEvent{AgentName: "new-agent", InputCost: 1.0, OutputCost: 1.0})

		report := tr.GenerateReport("", 24*time.Hour)
		if report.EventCount != 1 {
			t.Fatalf("expected 1 event in last 24h, got %d", report.EventCount)
		}
		if !floatEq(report.TotalCost, 2.0) {
			t.Fatalf("expected total cost 2.0, got %f", report.TotalCost)
		}
	})
}

// ---------------------------------------------------------------------------
// 7. ExportCSV
// ---------------------------------------------------------------------------

func TestExportCSV(t *testing.T) {
	t.Run("correct format with header", func(t *testing.T) {
		dir := t.TempDir()
		tr := NewTracker(dir, nopLogger())

		ts := time.Date(2025, 7, 1, 10, 0, 0, 0, time.UTC)
		_ = tr.RecordCost(CostEvent{
			ID:            "csv-1",
			Timestamp:     ts,
			AgentName:     "agent-x",
			TaskID:        "task-1",
			GoalRef:       "goal-1",
			ProjectRef:    "proj-1",
			TokensIn:      500,
			TokensOut:     300,
			InputCost:     0.001,
			OutputCost:    0.002,
			Duration:      5 * time.Second,
			ModelProvider: "anthropic",
			ModelName:     "claude-sonnet-4-6",
		})

		data, err := tr.ExportCSV()
		if err != nil {
			t.Fatal(err)
		}

		reader := csv.NewReader(strings.NewReader(string(data)))
		records, err := reader.ReadAll()
		if err != nil {
			t.Fatal(err)
		}

		if len(records) != 2 { // header + 1 row
			t.Fatalf("expected 2 records (header + 1 row), got %d", len(records))
		}

		header := records[0]
		expectedHeader := []string{
			"id", "timestamp", "agentName", "taskId", "goalRef", "projectRef",
			"tokensIn", "tokensOut", "totalTokens",
			"inputCost", "outputCost", "totalCost",
			"duration", "modelProvider", "modelName",
		}
		for i, h := range expectedHeader {
			if header[i] != h {
				t.Fatalf("header[%d] expected %q, got %q", i, h, header[i])
			}
		}

		row := records[1]
		if row[0] != "csv-1" {
			t.Fatalf("expected ID csv-1, got %s", row[0])
		}
		if row[2] != "agent-x" {
			t.Fatalf("expected agentName agent-x, got %s", row[2])
		}
	})

	t.Run("empty events produces header only", func(t *testing.T) {
		dir := t.TempDir()
		tr := NewTracker(dir, nopLogger())

		data, err := tr.ExportCSV()
		if err != nil {
			t.Fatal(err)
		}

		reader := csv.NewReader(strings.NewReader(string(data)))
		records, err := reader.ReadAll()
		if err != nil {
			t.Fatal(err)
		}

		if len(records) != 1 {
			t.Fatalf("expected 1 record (header only), got %d", len(records))
		}
	})
}

// ---------------------------------------------------------------------------
// 8. DayCost
// ---------------------------------------------------------------------------

func TestDayCost(t *testing.T) {
	t.Run("sum within range", func(t *testing.T) {
		dir := t.TempDir()
		tr := NewTracker(dir, nopLogger())

		now := time.Now()
		inRange := CostEvent{
			ID:        "in1",
			Timestamp: now.Add(-2 * time.Hour),
			InputCost: 1.0,
			OutputCost: 2.0,
			TotalCost: 3.0,
		}
		inRange2 := CostEvent{
			ID:        "in2",
			Timestamp: now.Add(-1 * time.Hour),
			InputCost: 0.5,
			OutputCost: 0.5,
			TotalCost: 1.0,
		}

		tr.mu.Lock()
		tr.events = append(tr.events, inRange, inRange2)
		tr.mu.Unlock()

		start := now.Add(-3 * time.Hour)
		end := now.Add(time.Hour)
		total := tr.DayCost(start, end)
		if !floatEq(total, 4.0) {
			t.Fatalf("expected 4.0, got %f", total)
		}
	})

	t.Run("outside range excluded", func(t *testing.T) {
		dir := t.TempDir()
		tr := NewTracker(dir, nopLogger())

		now := time.Now()
		outside := CostEvent{
			ID:        "out",
			Timestamp: now.Add(-48 * time.Hour),
			TotalCost: 10.0,
		}
		inside := CostEvent{
			ID:        "in",
			Timestamp: now.Add(-1 * time.Hour),
			TotalCost: 2.0,
		}

		tr.mu.Lock()
		tr.events = append(tr.events, outside, inside)
		tr.mu.Unlock()

		start := now.Add(-24 * time.Hour)
		end := now.Add(time.Hour)
		total := tr.DayCost(start, end)
		if !floatEq(total, 2.0) {
			t.Fatalf("expected 2.0, got %f", total)
		}
	})
}

// ---------------------------------------------------------------------------
// 9. RecentEvents
// ---------------------------------------------------------------------------

func TestRecentEvents(t *testing.T) {
	t.Run("returns last N", func(t *testing.T) {
		dir := t.TempDir()
		tr := NewTracker(dir, nopLogger())

		for i := 0; i < 10; i++ {
			_ = tr.RecordCost(CostEvent{AgentName: fmt.Sprintf("agent-%d", i)})
		}

		recent := tr.RecentEvents(3)
		if len(recent) != 3 {
			t.Fatalf("expected 3, got %d", len(recent))
		}
		if recent[0].AgentName != "agent-7" {
			t.Fatalf("expected agent-7, got %s", recent[0].AgentName)
		}
		if recent[2].AgentName != "agent-9" {
			t.Fatalf("expected agent-9, got %s", recent[2].AgentName)
		}
	})

	t.Run("N greater than total returns all", func(t *testing.T) {
		dir := t.TempDir()
		tr := NewTracker(dir, nopLogger())

		_ = tr.RecordCost(CostEvent{AgentName: "only"})

		recent := tr.RecentEvents(100)
		if len(recent) != 1 {
			t.Fatalf("expected 1, got %d", len(recent))
		}
	})

	t.Run("empty events returns empty slice", func(t *testing.T) {
		dir := t.TempDir()
		tr := NewTracker(dir, nopLogger())

		recent := tr.RecentEvents(5)
		if len(recent) != 0 {
			t.Fatalf("expected 0, got %d", len(recent))
		}
	})
}

// ---------------------------------------------------------------------------
// 10. SetPricing
// ---------------------------------------------------------------------------

func TestSetPricing(t *testing.T) {
	t.Run("custom pricing used in CalculateCost", func(t *testing.T) {
		dir := t.TempDir()
		tr := NewTracker(dir, nopLogger())

		tr.SetPricing(ModelPricing{
			Provider:   "custom",
			Model:      "my-model",
			InputPer1K: 0.01,
			OutputPer1K: 0.05,
		})

		inCost, outCost := tr.CalculateCost(2000, 1000, "custom", "my-model")
		expectedIn := 2000.0 / 1000.0 * 0.01
		expectedOut := 1000.0 / 1000.0 * 0.05
		if !floatEq(inCost, expectedIn) {
			t.Fatalf("expected input cost %f, got %f", expectedIn, inCost)
		}
		if !floatEq(outCost, expectedOut) {
			t.Fatalf("expected output cost %f, got %f", expectedOut, outCost)
		}
	})

	t.Run("overrides existing pricing", func(t *testing.T) {
		dir := t.TempDir()
		tr := NewTracker(dir, nopLogger())

		// Override anthropic/claude-sonnet-4-6 with custom pricing.
		tr.SetPricing(ModelPricing{
			Provider:   "anthropic",
			Model:      "claude-sonnet-4-6",
			InputPer1K: 0.1,
			OutputPer1K: 0.5,
		})

		inCost, outCost := tr.CalculateCost(1000, 1000, "anthropic", "claude-sonnet-4-6")
		if !floatEq(inCost, 0.1) {
			t.Fatalf("expected 0.1, got %f", inCost)
		}
		if !floatEq(outCost, 0.5) {
			t.Fatalf("expected 0.5, got %f", outCost)
		}
	})
}

// ---------------------------------------------------------------------------
// 11. Concurrent safety
// ---------------------------------------------------------------------------

func TestConcurrentRecordCost(t *testing.T) {
	t.Run("multiple goroutines RecordCost simultaneously", func(t *testing.T) {
		dir := t.TempDir()
		tr := NewTracker(dir, nopLogger())

		const goroutines = 50
		var wg sync.WaitGroup
		wg.Add(goroutines)

		errCh := make(chan error, goroutines)

		for i := 0; i < goroutines; i++ {
			go func(idx int) {
				defer wg.Done()
				err := tr.RecordCost(CostEvent{
					AgentName:  fmt.Sprintf("agent-%d", idx),
					TokensIn:   100,
					TokensOut:  200,
					InputCost:  0.001,
					OutputCost: 0.002,
				})
				if err != nil {
					errCh <- err
				}
			}(i)
		}

		wg.Wait()
		close(errCh)

		for err := range errCh {
			t.Fatalf("concurrent RecordCost error: %v", err)
		}

		if got := len(tr.events); got != goroutines {
			t.Fatalf("expected %d events, got %d", goroutines, got)
		}

		// Verify persistence: reload from disk.
		tr2 := NewTracker(dir, nopLogger())
		if got := len(tr2.events); got != goroutines {
			t.Fatalf("expected %d persisted events, got %d", goroutines, got)
		}
	})
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func writeJSONL(t *testing.T, dir string, events []CostEvent) {
	t.Helper()
	path := filepath.Join(dir, costEventsFile)
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	for _, e := range events {
		data, err := json.Marshal(e)
		if err != nil {
			t.Fatal(err)
		}
		_, _ = f.Write(data)
		_, _ = f.Write([]byte("\n"))
	}
}

func floatEq(a, b float64) bool {
	return math.Abs(a-b) < 1e-9
}
