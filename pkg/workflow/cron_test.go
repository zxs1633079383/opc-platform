package workflow

import (
	"context"
	"testing"
	"time"

	"go.uber.org/zap"
)

func TestMatchesCronField(t *testing.T) {
	tests := []struct {
		name  string
		field string
		value int
		want  bool
	}{
		// Wildcard
		{"wildcard matches zero", "*", 0, true},
		{"wildcard matches any value", "*", 59, true},

		// Exact number
		{"exact match", "7", 7, true},
		{"exact no match", "7", 8, false},
		{"exact zero", "0", 0, true},

		// Every N (*/N)
		{"every 5 matches 0", "*/5", 0, true},
		{"every 5 matches 15", "*/5", 15, true},
		{"every 5 no match 3", "*/5", 3, false},
		{"every 2 matches 4", "*/2", 4, true},
		{"every 2 no match 5", "*/2", 5, false},
		{"every N invalid suffix", "*/abc", 0, false},
		{"every N zero step", "*/0", 0, false},
		{"every N negative step", "*/-1", 0, false},

		// Range
		{"range match low bound", "1-5", 1, true},
		{"range match high bound", "1-5", 5, true},
		{"range match middle", "1-5", 3, true},
		{"range no match below", "1-5", 0, false},
		{"range no match above", "1-5", 6, false},
		{"range invalid low", "a-5", 3, false},
		{"range invalid high", "1-b", 3, false},

		// Comma-separated
		{"comma match first", "1,3,5", 1, true},
		{"comma match middle", "1,3,5", 3, true},
		{"comma match last", "1,3,5", 5, true},
		{"comma no match", "1,3,5", 2, false},

		// Mixed comma + range
		{"comma with range match range", "1-3,7,10-12", 2, true},
		{"comma with range match exact", "1-3,7,10-12", 7, true},
		{"comma with range match second range", "1-3,7,10-12", 11, true},
		{"comma with range no match", "1-3,7,10-12", 5, false},

		// Invalid field
		{"invalid field", "abc", 0, false},
		{"empty field", "", 0, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := matchesCronField(tt.field, tt.value)
			if got != tt.want {
				t.Errorf("matchesCronField(%q, %d) = %v, want %v", tt.field, tt.value, got, tt.want)
			}
		})
	}
}

func TestMatchesCron(t *testing.T) {
	// Reference time: Monday, January 6, 2025, 07:00
	// Weekday=1 (Monday), Month=1, Day=6, Hour=7, Minute=0
	refTime := time.Date(2025, time.January, 6, 7, 0, 0, 0, time.UTC)

	tests := []struct {
		name string
		expr string
		now  time.Time
		want bool
	}{
		// Daily at 7am: minute=0, hour=7
		{"daily 7am match", "0 7 * * *", refTime, true},
		{"daily 7am no match wrong hour",
			"0 7 * * *",
			time.Date(2025, 1, 6, 8, 0, 0, 0, time.UTC),
			false,
		},
		{"daily 7am no match wrong minute",
			"0 7 * * *",
			time.Date(2025, 1, 6, 7, 1, 0, 0, time.UTC),
			false,
		},

		// Every 5 minutes
		{"every 5 min match at 0", "*/5 * * * *", refTime, true},
		{"every 5 min match at 15",
			"*/5 * * * *",
			time.Date(2025, 1, 6, 7, 15, 0, 0, time.UTC),
			true,
		},
		{"every 5 min no match at 3",
			"*/5 * * * *",
			time.Date(2025, 1, 6, 7, 3, 0, 0, time.UTC),
			false,
		},

		// First of month at midnight
		{"first of month match",
			"0 0 1 * *",
			time.Date(2025, 3, 1, 0, 0, 0, 0, time.UTC),
			true,
		},
		{"first of month no match day 2",
			"0 0 1 * *",
			time.Date(2025, 3, 2, 0, 0, 0, 0, time.UTC),
			false,
		},

		// Complex: range + comma
		{"complex range+comma match",
			"0,30 9-17 * * 1-5",
			time.Date(2025, 1, 6, 12, 30, 0, 0, time.UTC), // Monday 12:30
			true,
		},
		{"complex range+comma no match weekend",
			"0,30 9-17 * * 1-5",
			time.Date(2025, 1, 5, 12, 30, 0, 0, time.UTC), // Sunday 12:30
			false,
		},

		// Invalid field count
		{"too few fields", "0 7 * *", refTime, false},
		{"too many fields", "0 7 * * * *", refTime, false},
		{"empty expression", "", refTime, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := matchesCron(tt.expr, tt.now)
			if got != tt.want {
				t.Errorf("matchesCron(%q, %v) = %v, want %v", tt.expr, tt.now, got, tt.want)
			}
		})
	}
}

func newTestScheduler(t *testing.T) *CronScheduler {
	t.Helper()
	logger := zap.NewNop().Sugar()
	return NewCronScheduler(nil, logger)
}

func TestCronScheduler_AddAndList(t *testing.T) {
	cs := newTestScheduler(t)

	cs.AddWorkflow("build", "0 7 * * *")
	cs.AddWorkflow("deploy", "0 0 * * 1")

	list := cs.List()
	if len(list) != 2 {
		t.Fatalf("List() returned %d entries, want 2", len(list))
	}

	found := map[string]CronStatus{}
	for _, s := range list {
		found[s.WorkflowName] = s
	}

	if s, ok := found["build"]; !ok {
		t.Error("missing workflow 'build'")
	} else {
		if s.Schedule != "0 7 * * *" {
			t.Errorf("build schedule = %q, want %q", s.Schedule, "0 7 * * *")
		}
		if !s.Enabled {
			t.Error("build should be enabled by default")
		}
	}

	if _, ok := found["deploy"]; !ok {
		t.Error("missing workflow 'deploy'")
	}
}

func TestCronScheduler_RemoveWorkflow(t *testing.T) {
	cs := newTestScheduler(t)

	cs.AddWorkflow("build", "0 7 * * *")
	cs.AddWorkflow("deploy", "0 0 * * 1")

	cs.RemoveWorkflow("build")

	list := cs.List()
	if len(list) != 1 {
		t.Fatalf("List() returned %d entries after remove, want 1", len(list))
	}
	if list[0].WorkflowName != "deploy" {
		t.Errorf("remaining workflow = %q, want %q", list[0].WorkflowName, "deploy")
	}

	// Removing a non-existent workflow should not panic
	cs.RemoveWorkflow("nonexistent")
	if len(cs.List()) != 1 {
		t.Error("list length changed after removing nonexistent workflow")
	}
}

func TestCronScheduler_EnableDisableToggle(t *testing.T) {
	cs := newTestScheduler(t)

	cs.AddWorkflow("build", "0 7 * * *")

	// Initially enabled
	list := cs.List()
	if !list[0].Enabled {
		t.Fatal("workflow should be enabled by default")
	}

	// Disable
	if err := cs.Disable("build"); err != nil {
		t.Fatalf("Disable() error: %v", err)
	}
	list = cs.List()
	if list[0].Enabled {
		t.Error("workflow should be disabled after Disable()")
	}

	// Re-enable
	if err := cs.Enable("build"); err != nil {
		t.Fatalf("Enable() error: %v", err)
	}
	list = cs.List()
	if !list[0].Enabled {
		t.Error("workflow should be enabled after Enable()")
	}
}

func TestCronScheduler_DisableUnknownReturnsError(t *testing.T) {
	cs := newTestScheduler(t)

	err := cs.Disable("nonexistent")
	if err == nil {
		t.Fatal("Disable() should return error for unknown workflow")
	}

	want := `workflow "nonexistent" not in cron schedule`
	if err.Error() != want {
		t.Errorf("error = %q, want %q", err.Error(), want)
	}
}

func TestCronScheduler_EnableUnknownReturnsError(t *testing.T) {
	cs := newTestScheduler(t)

	err := cs.Enable("nonexistent")
	if err == nil {
		t.Fatal("Enable() should return error for unknown workflow")
	}

	want := `workflow "nonexistent" not in cron schedule`
	if err.Error() != want {
		t.Errorf("error = %q, want %q", err.Error(), want)
	}
}

func TestCronScheduler_StartStop(t *testing.T) {
	cs := newTestScheduler(t)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	cs.AddWorkflow("build", "0 7 * * *")

	// Start and stop should not panic
	cs.Start(ctx)

	// Give the goroutine a moment to launch
	time.Sleep(10 * time.Millisecond)

	cs.Stop()
}

func TestCronScheduler_StopWithoutStart(t *testing.T) {
	cs := newTestScheduler(t)

	// Stop without Start should not panic (cancelFn is nil)
	cs.Stop()
}
