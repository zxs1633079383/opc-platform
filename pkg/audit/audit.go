package audit

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"
)

// EventType represents the type of audit event.
type EventType string

const (
	EventCreated       EventType = "created"
	EventStarted       EventType = "started"
	EventCompleted     EventType = "completed"
	EventFailed        EventType = "failed"
	EventDeleted       EventType = "deleted"
	EventRestarted     EventType = "restarted"
	EventRecovered     EventType = "recovered"
	EventCostIncurred  EventType = "cost_incurred"
	EventAgentAssigned EventType = "agent_assigned"
	EventConfigChanged EventType = "config_changed"
)

// ResourceType identifies the kind of resource an event relates to.
type ResourceType string

const (
	ResourceAgent    ResourceType = "agent"
	ResourceTask     ResourceType = "task"
	ResourceGoal     ResourceType = "goal"
	ResourceProject  ResourceType = "project"
	ResourceIssue    ResourceType = "issue"
	ResourceWorkflow ResourceType = "workflow"
)

const auditFileName = "audit.jsonl"

// AuditEvent is a single audit log entry.
type AuditEvent struct {
	ID           string            `json:"id"`
	Timestamp    time.Time         `json:"timestamp"`
	EventType    EventType         `json:"eventType"`
	ResourceType ResourceType      `json:"resourceType"`
	ResourceName string            `json:"resourceName"`
	Details      string            `json:"details,omitempty"`
	Labels       map[string]string `json:"labels,omitempty"`

	// Hierarchy refs for tracing.
	GoalRef    string `json:"goalRef,omitempty"`
	ProjectRef string `json:"projectRef,omitempty"`
	TaskRef    string `json:"taskRef,omitempty"`
	AgentRef   string `json:"agentRef,omitempty"`
}

// Logger records and queries audit events.
type Logger struct {
	mu     sync.RWMutex
	events []AuditEvent
	dir    string
	logger *zap.SugaredLogger
}

// NewLogger creates a new audit Logger backed by the given directory.
// It loads any previously persisted events from disk.
func NewLogger(dir string, logger *zap.SugaredLogger) *Logger {
	l := &Logger{
		events: make([]AuditEvent, 0),
		dir:    dir,
		logger: logger,
	}

	if err := os.MkdirAll(dir, 0o755); err != nil {
		logger.Warnw("failed to create audit directory", "dir", dir, "error", err)
	}

	if err := l.loadFromDisk(); err != nil {
		logger.Warnw("failed to load audit events from disk", "error", err)
	}

	return l
}

// Log records an audit event. It assigns an ID and timestamp if they are
// not already set, appends the event to in-memory storage, and persists it
// to the JSONL file on disk.
func (l *Logger) Log(event AuditEvent) error {
	if event.ID == "" {
		event.ID = uuid.New().String()
	}
	if event.Timestamp.IsZero() {
		event.Timestamp = time.Now().UTC()
	}

	l.mu.Lock()
	l.events = append(l.events, event)
	l.mu.Unlock()

	if err := l.persistEvent(event); err != nil {
		l.logger.Errorw("failed to persist audit event", "eventID", event.ID, "error", err)
		return fmt.Errorf("persist audit event: %w", err)
	}

	l.logger.Debugw("audit event recorded",
		"eventID", event.ID,
		"eventType", event.EventType,
		"resourceType", event.ResourceType,
		"resourceName", event.ResourceName,
	)

	return nil
}

// ListEvents returns all events matching the given resource type and name.
// If resourceName is empty, all events of that resource type are returned.
func (l *Logger) ListEvents(resourceType ResourceType, resourceName string) ([]AuditEvent, error) {
	l.mu.RLock()
	defer l.mu.RUnlock()

	result := make([]AuditEvent, 0)
	for _, e := range l.events {
		if e.ResourceType != resourceType {
			continue
		}
		if resourceName != "" && e.ResourceName != resourceName {
			continue
		}
		result = append(result, e)
	}

	return result, nil
}

// ListByGoal returns all events that reference the given goal name,
// either as the resource itself or through the GoalRef field.
func (l *Logger) ListByGoal(goalName string) ([]AuditEvent, error) {
	l.mu.RLock()
	defer l.mu.RUnlock()

	result := make([]AuditEvent, 0)
	for _, e := range l.events {
		if e.GoalRef == goalName {
			result = append(result, e)
			continue
		}
		if e.ResourceType == ResourceGoal && e.ResourceName == goalName {
			result = append(result, e)
		}
	}

	return result, nil
}

// Trace returns the full chain of events related to a resource and its
// parent hierarchy. For example, tracing an issue returns events for the
// issue itself plus events for the referenced task, project, and goal.
func (l *Logger) Trace(resourceType ResourceType, resourceName string) ([]AuditEvent, error) {
	l.mu.RLock()
	defer l.mu.RUnlock()

	// First, collect direct events for the resource.
	directEvents := make([]AuditEvent, 0)
	for _, e := range l.events {
		if e.ResourceType == resourceType && e.ResourceName == resourceName {
			directEvents = append(directEvents, e)
		}
	}

	// Build the set of related resource keys to look up by inspecting
	// hierarchy refs on the direct events.
	type resourceKey struct {
		rType ResourceType
		rName string
	}
	related := make(map[resourceKey]struct{})
	for _, e := range directEvents {
		if e.GoalRef != "" {
			related[resourceKey{ResourceGoal, e.GoalRef}] = struct{}{}
		}
		if e.ProjectRef != "" {
			related[resourceKey{ResourceProject, e.ProjectRef}] = struct{}{}
		}
		if e.TaskRef != "" {
			related[resourceKey{ResourceTask, e.TaskRef}] = struct{}{}
		}
		if e.AgentRef != "" {
			related[resourceKey{ResourceAgent, e.AgentRef}] = struct{}{}
		}
	}

	// Also scan all events that reference this resource through their ref fields.
	for _, e := range l.events {
		matched := false
		switch resourceType {
		case ResourceGoal:
			matched = e.GoalRef == resourceName
		case ResourceProject:
			matched = e.ProjectRef == resourceName
		case ResourceTask:
			matched = e.TaskRef == resourceName
		case ResourceAgent:
			matched = e.AgentRef == resourceName
		}
		if matched {
			related[resourceKey{e.ResourceType, e.ResourceName}] = struct{}{}
		}
	}

	// Collect all matching events: direct + related resources.
	seen := make(map[string]struct{})
	result := make([]AuditEvent, 0, len(directEvents))

	for _, e := range directEvents {
		seen[e.ID] = struct{}{}
		result = append(result, e)
	}

	for _, e := range l.events {
		if _, ok := seen[e.ID]; ok {
			continue
		}
		key := resourceKey{e.ResourceType, e.ResourceName}
		if _, ok := related[key]; ok {
			seen[e.ID] = struct{}{}
			result = append(result, e)
		}
	}

	return result, nil
}

// Export serialises all audit events in the requested format.
// Currently only "json" is supported.
func (l *Logger) Export(format string) ([]byte, error) {
	l.mu.RLock()
	defer l.mu.RUnlock()

	switch format {
	case "json":
		data, err := json.MarshalIndent(l.events, "", "  ")
		if err != nil {
			return nil, fmt.Errorf("marshal audit events: %w", err)
		}
		return data, nil
	default:
		return nil, fmt.Errorf("unsupported export format: %s", format)
	}
}

// loadFromDisk reads the JSONL audit file and populates the in-memory
// event slice. It is called once during Logger initialisation.
func (l *Logger) loadFromDisk() error {
	path := filepath.Join(l.dir, auditFileName)

	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // no persisted data yet
		}
		return fmt.Errorf("open audit file: %w", err)
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	lineNum := 0
	for scanner.Scan() {
		lineNum++
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}

		var event AuditEvent
		if err := json.Unmarshal(line, &event); err != nil {
			l.logger.Warnw("skipping malformed audit line",
				"line", lineNum,
				"error", err,
			)
			continue
		}
		l.events = append(l.events, event)
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("scan audit file: %w", err)
	}

	l.logger.Infow("loaded audit events from disk", "count", len(l.events))
	return nil
}

// persistEvent appends a single event as a JSON line to the audit file.
func (l *Logger) persistEvent(event AuditEvent) error {
	path := filepath.Join(l.dir, auditFileName)

	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("open audit file for append: %w", err)
	}
	defer f.Close()

	data, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("marshal audit event: %w", err)
	}

	data = append(data, '\n')
	if _, err := f.Write(data); err != nil {
		return fmt.Errorf("write audit event: %w", err)
	}

	return nil
}
