package workflow

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"go.uber.org/zap"
)

// CronScheduler manages periodic workflow execution based on cron expressions.
type CronScheduler struct {
	mu       sync.Mutex
	engine   *Engine
	entries  map[string]*cronEntry
	logger   *zap.SugaredLogger
	cancelFn context.CancelFunc
}

// cronEntry represents a scheduled workflow.
type cronEntry struct {
	workflowName string
	schedule     string
	enabled      bool
	cancel       context.CancelFunc
}

// CronStatus reports the state of a scheduled workflow.
type CronStatus struct {
	WorkflowName string `json:"workflowName"`
	Schedule     string `json:"schedule"`
	Enabled      bool   `json:"enabled"`
}

// NewCronScheduler creates a new cron scheduler.
func NewCronScheduler(engine *Engine, logger *zap.SugaredLogger) *CronScheduler {
	return &CronScheduler{
		engine:  engine,
		entries: make(map[string]*cronEntry),
		logger:  logger,
	}
}

// Start begins the scheduler loop.
func (cs *CronScheduler) Start(ctx context.Context) {
	ctx, cancel := context.WithCancel(ctx)
	cs.cancelFn = cancel

	go cs.run(ctx)
	cs.logger.Infow("cron scheduler started")
}

// Stop halts the scheduler.
func (cs *CronScheduler) Stop() {
	if cs.cancelFn != nil {
		cs.cancelFn()
	}
	cs.mu.Lock()
	for _, entry := range cs.entries {
		if entry.cancel != nil {
			entry.cancel()
		}
	}
	cs.mu.Unlock()
	cs.logger.Infow("cron scheduler stopped")
}

// AddWorkflow registers a workflow with a cron schedule.
func (cs *CronScheduler) AddWorkflow(name, schedule string) {
	cs.mu.Lock()
	defer cs.mu.Unlock()

	cs.entries[name] = &cronEntry{
		workflowName: name,
		schedule:     schedule,
		enabled:      true,
	}
	cs.logger.Infow("workflow added to cron", "name", name, "schedule", schedule)
}

// RemoveWorkflow unregisters a workflow.
func (cs *CronScheduler) RemoveWorkflow(name string) {
	cs.mu.Lock()
	defer cs.mu.Unlock()

	if entry, ok := cs.entries[name]; ok {
		if entry.cancel != nil {
			entry.cancel()
		}
		delete(cs.entries, name)
		cs.logger.Infow("workflow removed from cron", "name", name)
	}
}

// Enable enables a scheduled workflow.
func (cs *CronScheduler) Enable(name string) error {
	cs.mu.Lock()
	defer cs.mu.Unlock()

	entry, ok := cs.entries[name]
	if !ok {
		return fmt.Errorf("workflow %q not in cron schedule", name)
	}
	entry.enabled = true
	return nil
}

// Disable disables a scheduled workflow.
func (cs *CronScheduler) Disable(name string) error {
	cs.mu.Lock()
	defer cs.mu.Unlock()

	entry, ok := cs.entries[name]
	if !ok {
		return fmt.Errorf("workflow %q not in cron schedule", name)
	}
	entry.enabled = false
	return nil
}

// List returns all scheduled workflows.
func (cs *CronScheduler) List() []CronStatus {
	cs.mu.Lock()
	defer cs.mu.Unlock()

	var statuses []CronStatus
	for _, entry := range cs.entries {
		statuses = append(statuses, CronStatus{
			WorkflowName: entry.workflowName,
			Schedule:     entry.schedule,
			Enabled:      entry.enabled,
		})
	}
	return statuses
}

// run is the main scheduler loop that checks every minute if any workflows
// should be triggered based on their cron expressions.
func (cs *CronScheduler) run(ctx context.Context) {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case now := <-ticker.C:
			cs.checkAndTrigger(ctx, now)
		}
	}
}

// checkAndTrigger checks all entries and triggers matching ones.
func (cs *CronScheduler) checkAndTrigger(ctx context.Context, now time.Time) {
	cs.mu.Lock()
	entries := make(map[string]*cronEntry, len(cs.entries))
	for k, v := range cs.entries {
		entries[k] = v
	}
	cs.mu.Unlock()

	for name, entry := range entries {
		if !entry.enabled {
			continue
		}
		if matchesCron(entry.schedule, now) {
			cs.logger.Infow("triggering scheduled workflow", "name", name)
			go cs.triggerWorkflow(ctx, name)
		}
	}
}

// triggerWorkflow loads and executes a workflow.
func (cs *CronScheduler) triggerWorkflow(ctx context.Context, name string) {
	wf, err := cs.engine.store.GetWorkflow(ctx, name)
	if err != nil {
		cs.logger.Errorw("failed to load workflow for cron execution", "name", name, "error", err)
		return
	}

	spec, err := ParseWorkflow([]byte(wf.SpecYAML))
	if err != nil {
		cs.logger.Errorw("failed to parse workflow spec", "name", name, "error", err)
		return
	}

	run, err := cs.engine.Execute(ctx, spec)
	if err != nil {
		cs.logger.Errorw("cron workflow execution failed", "name", name, "error", err)
		return
	}

	cs.logger.Infow("cron workflow completed", "name", name, "runID", run.ID, "status", run.Status)
}

// matchesCron checks if the current time matches a simplified cron expression.
// Supports standard 5-field format: minute hour day-of-month month day-of-week
// Each field supports: * (any), specific number, */N (every N)
func matchesCron(expr string, now time.Time) bool {
	fields := strings.Fields(expr)
	if len(fields) != 5 {
		return false
	}

	values := []int{
		now.Minute(),
		now.Hour(),
		now.Day(),
		int(now.Month()),
		int(now.Weekday()),
	}

	for i, field := range fields {
		if !matchesCronField(field, values[i]) {
			return false
		}
	}

	return true
}

// matchesCronField checks if a single cron field matches a value.
func matchesCronField(field string, value int) bool {
	if field == "*" {
		return true
	}

	// Handle */N (every N)
	if strings.HasPrefix(field, "*/") {
		n, err := strconv.Atoi(field[2:])
		if err != nil || n <= 0 {
			return false
		}
		return value%n == 0
	}

	// Handle comma-separated values
	for _, part := range strings.Split(field, ",") {
		// Handle range (e.g., 1-5)
		if strings.Contains(part, "-") {
			rangeParts := strings.SplitN(part, "-", 2)
			low, err1 := strconv.Atoi(rangeParts[0])
			high, err2 := strconv.Atoi(rangeParts[1])
			if err1 == nil && err2 == nil && value >= low && value <= high {
				return true
			}
			continue
		}

		// Handle exact value
		n, err := strconv.Atoi(part)
		if err == nil && n == value {
			return true
		}
	}

	return false
}
