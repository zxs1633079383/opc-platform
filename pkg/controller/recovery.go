package controller

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/zlc-ai/opc-platform/internal/config"
)

// RecoverySource represents where recovery data comes from.
type RecoverySource string

const (
	RecoverySourceCheckpoint RecoverySource = "checkpoint"
	RecoverySourceMemory     RecoverySource = "memory"
	RecoverySourceManual     RecoverySource = "manual"
)

// RecoveryResult contains the result of a recovery attempt.
type RecoveryResult struct {
	AgentName    string         `json:"agentName"`
	Source       RecoverySource `json:"source"`
	CheckpointID string        `json:"checkpointId,omitempty"`
	RestoredAt   time.Time     `json:"restoredAt"`
	PendingTasks []string      `json:"pendingTasks,omitempty"`
	Success      bool          `json:"success"`
	Message      string        `json:"message"`
}

// CrashReport contains information about an agent crash.
type CrashReport struct {
	AgentName string    `json:"agentName"`
	Error     string    `json:"error"`
	Timestamp time.Time `json:"timestamp"`
}

// crashDir returns the directory for storing crash reports of a given agent.
func crashDir(agentName string) string {
	return filepath.Join(config.GetConfigDir(), "crashes", agentName)
}

// RecoverAgent recovers an agent from the specified source.
func (c *Controller) RecoverAgent(ctx context.Context, name string, source RecoverySource) (*RecoveryResult, error) {
	switch source {
	case RecoverySourceCheckpoint:
		return c.RecoverFromLatest(ctx, name)
	case RecoverySourceMemory:
		return c.recoverFromMemory(ctx, name)
	case RecoverySourceManual:
		return c.recoverManual(ctx, name)
	default:
		return nil, fmt.Errorf("unknown recovery source: %q", source)
	}
}

// RecoverFromCheckpoint recovers an agent from a specific checkpoint.
func (c *Controller) RecoverFromCheckpoint(ctx context.Context, name string, checkpointID string) (*RecoveryResult, error) {
	cp, err := c.GetCheckpoint(ctx, checkpointID)
	if err != nil {
		return &RecoveryResult{
			AgentName: name,
			Source:    RecoverySourceCheckpoint,
			RestoredAt: time.Now(),
			Success:   false,
			Message:   fmt.Sprintf("failed to load checkpoint: %v", err),
		}, err
	}

	if cp.AgentName != name {
		return nil, fmt.Errorf("checkpoint %q belongs to agent %q, not %q", checkpointID, cp.AgentName, name)
	}

	return c.restoreFromCheckpoint(ctx, cp)
}

// RecoverFromLatest auto-recovers an agent from the latest available checkpoint.
func (c *Controller) RecoverFromLatest(ctx context.Context, name string) (*RecoveryResult, error) {
	checkpoints, err := c.ListCheckpoints(ctx, name)
	if err != nil {
		return &RecoveryResult{
			AgentName:  name,
			Source:     RecoverySourceCheckpoint,
			RestoredAt: time.Now(),
			Success:    false,
			Message:    fmt.Sprintf("failed to list checkpoints: %v", err),
		}, err
	}

	if len(checkpoints) == 0 {
		return &RecoveryResult{
			AgentName:  name,
			Source:     RecoverySourceCheckpoint,
			RestoredAt: time.Now(),
			Success:    false,
			Message:    "no checkpoints available",
		}, fmt.Errorf("no checkpoints found for agent %q", name)
	}

	// checkpoints are sorted newest first by ListCheckpoints.
	latest := checkpoints[0]
	return c.restoreFromCheckpoint(ctx, &latest)
}

// restoreFromCheckpoint restores an agent from a checkpoint snapshot.
func (c *Controller) restoreFromCheckpoint(ctx context.Context, cp *Checkpoint) (*RecoveryResult, error) {
	// Parse the spec from the checkpoint.
	spec, err := unmarshalSpec(cp.SpecYAML)
	if err != nil {
		return &RecoveryResult{
			AgentName:    cp.AgentName,
			Source:       RecoverySourceCheckpoint,
			CheckpointID: cp.ID,
			RestoredAt:   time.Now(),
			Success:      false,
			Message:      fmt.Sprintf("failed to parse spec: %v", err),
		}, err
	}

	// Re-apply the spec to restore agent record.
	if err := c.Apply(ctx, spec); err != nil {
		return &RecoveryResult{
			AgentName:    cp.AgentName,
			Source:       RecoverySourceCheckpoint,
			CheckpointID: cp.ID,
			RestoredAt:   time.Now(),
			Success:      false,
			Message:      fmt.Sprintf("failed to apply spec: %v", err),
		}, err
	}

	// Restart the agent.
	if err := c.StartAgent(ctx, cp.AgentName); err != nil {
		return &RecoveryResult{
			AgentName:    cp.AgentName,
			Source:       RecoverySourceCheckpoint,
			CheckpointID: cp.ID,
			RestoredAt:   time.Now(),
			PendingTasks: cp.PendingTasks,
			Success:      false,
			Message:      fmt.Sprintf("spec restored but agent failed to start: %v", err),
		}, err
	}

	c.logger.Infow("agent recovered from checkpoint", "agent", cp.AgentName, "checkpoint", cp.ID)

	return &RecoveryResult{
		AgentName:    cp.AgentName,
		Source:       RecoverySourceCheckpoint,
		CheckpointID: cp.ID,
		RestoredAt:   time.Now(),
		PendingTasks: cp.PendingTasks,
		Success:      true,
		Message:      "agent recovered successfully from checkpoint",
	}, nil
}

// recoverFromMemory recovers an agent using in-memory state (store record).
func (c *Controller) recoverFromMemory(ctx context.Context, name string) (*RecoveryResult, error) {
	record, err := c.store.GetAgent(ctx, name)
	if err != nil {
		return &RecoveryResult{
			AgentName:  name,
			Source:     RecoverySourceMemory,
			RestoredAt: time.Now(),
			Success:    false,
			Message:    fmt.Sprintf("agent record not found: %v", err),
		}, err
	}

	spec, err := unmarshalSpec(record.SpecYAML)
	if err != nil {
		return &RecoveryResult{
			AgentName:  name,
			Source:     RecoverySourceMemory,
			RestoredAt: time.Now(),
			Success:    false,
			Message:    fmt.Sprintf("failed to parse stored spec: %v", err),
		}, err
	}

	if err := c.Apply(ctx, spec); err != nil {
		return &RecoveryResult{
			AgentName:  name,
			Source:     RecoverySourceMemory,
			RestoredAt: time.Now(),
			Success:    false,
			Message:    fmt.Sprintf("failed to apply spec: %v", err),
		}, err
	}

	if err := c.StartAgent(ctx, name); err != nil {
		return &RecoveryResult{
			AgentName:  name,
			Source:     RecoverySourceMemory,
			RestoredAt: time.Now(),
			Success:    false,
			Message:    fmt.Sprintf("failed to start agent: %v", err),
		}, err
	}

	c.logger.Infow("agent recovered from memory", "agent", name)

	return &RecoveryResult{
		AgentName:  name,
		Source:     RecoverySourceMemory,
		RestoredAt: time.Now(),
		Success:    true,
		Message:    "agent recovered successfully from stored state",
	}, nil
}

// recoverManual creates a recovery result for manual intervention.
func (c *Controller) recoverManual(_ context.Context, name string) (*RecoveryResult, error) {
	return &RecoveryResult{
		AgentName:  name,
		Source:     RecoverySourceManual,
		RestoredAt: time.Now(),
		Success:    true,
		Message:    "manual recovery initiated; re-apply agent spec to complete",
	}, nil
}

// GetRecoveryStatus returns the latest recovery result for an agent.
// It checks for a saved recovery status file in the crashes directory.
func (c *Controller) GetRecoveryStatus(_ context.Context, name string) (*RecoveryResult, error) {
	dir := crashDir(name)
	statusPath := filepath.Join(dir, "recovery-status.json")

	data, err := os.ReadFile(statusPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("no recovery status for agent %q", name)
		}
		return nil, fmt.Errorf("read recovery status: %w", err)
	}

	var result RecoveryResult
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("parse recovery status: %w", err)
	}
	return &result, nil
}

// saveRecoveryStatus persists the recovery result for later retrieval.
func (c *Controller) saveRecoveryStatus(name string, result *RecoveryResult) error {
	dir := crashDir(name)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create crash dir: %w", err)
	}

	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal recovery status: %w", err)
	}

	path := filepath.Join(dir, "recovery-status.json")
	return os.WriteFile(path, data, 0o644)
}

// SaveCrashReport saves crash information for an agent.
func (c *Controller) SaveCrashReport(_ context.Context, name string, crashErr error) error {
	dir := crashDir(name)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create crash dir: %w", err)
	}

	now := time.Now()
	report := CrashReport{
		AgentName: name,
		Error:     crashErr.Error(),
		Timestamp: now,
	}

	data, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal crash report: %w", err)
	}

	filename := fmt.Sprintf("crash-%d.json", now.UnixNano())
	path := filepath.Join(dir, filename)
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("write crash report: %w", err)
	}

	c.logger.Infow("crash report saved", "agent", name, "file", filename)
	return nil
}

// ListCrashReports returns all crash reports for an agent, sorted newest first.
func (c *Controller) ListCrashReports(_ context.Context, name string) ([]CrashReport, error) {
	dir := crashDir(name)
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read crash dir: %w", err)
	}

	var reports []CrashReport
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}
		if entry.Name() == "recovery-status.json" {
			continue
		}
		data, err := os.ReadFile(filepath.Join(dir, entry.Name()))
		if err != nil {
			continue
		}
		var report CrashReport
		if err := json.Unmarshal(data, &report); err != nil {
			continue
		}
		reports = append(reports, report)
	}

	sort.Slice(reports, func(i, j int) bool {
		return reports[i].Timestamp.After(reports[j].Timestamp)
	})

	return reports, nil
}
