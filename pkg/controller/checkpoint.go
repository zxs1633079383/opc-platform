package controller

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"

	v1 "github.com/zlc-ai/opc-platform/api/v1"
	"github.com/zlc-ai/opc-platform/internal/config"
)

// Checkpoint represents a saved agent state snapshot.
type Checkpoint struct {
	ID           string          `json:"id"`
	AgentName    string          `json:"agentName"`
	Timestamp    time.Time       `json:"timestamp"`
	Phase        v1.AgentPhase   `json:"phase"`
	Metrics      v1.AgentMetrics `json:"metrics"`
	PendingTasks []string        `json:"pendingTasks,omitempty"`
	SpecYAML     string          `json:"specYaml"`
}

// checkpointDir returns the directory for storing checkpoints of a given agent.
func checkpointDir(agentName string) string {
	return filepath.Join(config.GetConfigDir(), "checkpoints", agentName)
}

// checkpointFilename returns the filename for a checkpoint ID.
func checkpointFilename(id string) string {
	return id + ".json"
}

// CreateCheckpoint saves the current agent state as a checkpoint.
func (c *Controller) CreateCheckpoint(ctx context.Context, name string) (*Checkpoint, error) {
	record, err := c.store.GetAgent(ctx, name)
	if err != nil {
		return nil, fmt.Errorf("get agent %q: %w", name, err)
	}

	// Collect metrics if the agent is running.
	var metrics v1.AgentMetrics
	c.mu.RLock()
	ma, running := c.agents[name]
	c.mu.RUnlock()
	if running {
		metrics = ma.adapter.Metrics()
	}

	// Collect pending task IDs.
	var pendingTasks []string
	tasks, err := c.store.ListTasksByAgent(ctx, name)
	if err == nil {
		for _, t := range tasks {
			if t.Status == v1.TaskStatusPending || t.Status == v1.TaskStatusRunning {
				pendingTasks = append(pendingTasks, t.ID)
			}
		}
	}

	now := time.Now()
	cp := &Checkpoint{
		ID:           fmt.Sprintf("cp-%s-%d", name, now.UnixNano()),
		AgentName:    name,
		Timestamp:    now,
		Phase:        record.Phase,
		Metrics:      metrics,
		PendingTasks: pendingTasks,
		SpecYAML:     record.SpecYAML,
	}

	dir := checkpointDir(name)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("create checkpoint dir: %w", err)
	}

	data, err := json.MarshalIndent(cp, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("marshal checkpoint: %w", err)
	}

	path := filepath.Join(dir, checkpointFilename(cp.ID))
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return nil, fmt.Errorf("write checkpoint file: %w", err)
	}

	c.logger.Infow("checkpoint created", "agent", name, "id", cp.ID)
	return cp, nil
}

// ListCheckpoints returns all checkpoints for an agent, sorted by timestamp descending.
func (c *Controller) ListCheckpoints(_ context.Context, name string) ([]Checkpoint, error) {
	dir := checkpointDir(name)
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read checkpoint dir: %w", err)
	}

	var checkpoints []Checkpoint
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}
		data, err := os.ReadFile(filepath.Join(dir, entry.Name()))
		if err != nil {
			c.logger.Warnw("skip unreadable checkpoint", "file", entry.Name(), "error", err)
			continue
		}
		var cp Checkpoint
		if err := json.Unmarshal(data, &cp); err != nil {
			c.logger.Warnw("skip unparseable checkpoint", "file", entry.Name(), "error", err)
			continue
		}
		checkpoints = append(checkpoints, cp)
	}

	sort.Slice(checkpoints, func(i, j int) bool {
		return checkpoints[i].Timestamp.After(checkpoints[j].Timestamp)
	})

	return checkpoints, nil
}

// GetCheckpoint returns a specific checkpoint by ID.
func (c *Controller) GetCheckpoint(_ context.Context, id string) (*Checkpoint, error) {
	// The checkpoint ID encodes the agent name: cp-<agentName>-<nanos>.
	// We search all agent directories to find it.
	baseDir := filepath.Join(config.GetConfigDir(), "checkpoints")
	agents, err := os.ReadDir(baseDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("checkpoint %q not found", id)
		}
		return nil, fmt.Errorf("read checkpoints base dir: %w", err)
	}

	filename := checkpointFilename(id)
	for _, agent := range agents {
		if !agent.IsDir() {
			continue
		}
		path := filepath.Join(baseDir, agent.Name(), filename)
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		var cp Checkpoint
		if err := json.Unmarshal(data, &cp); err != nil {
			return nil, fmt.Errorf("parse checkpoint: %w", err)
		}
		return &cp, nil
	}

	return nil, fmt.Errorf("checkpoint %q not found", id)
}

// StartCheckpointLoop periodically creates checkpoints for all running agents.
func (c *Controller) StartCheckpointLoop(ctx context.Context, interval time.Duration) {
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				c.logger.Infow("checkpoint loop stopped")
				return
			case <-ticker.C:
				c.mu.RLock()
				names := make([]string, 0, len(c.agents))
				for name := range c.agents {
					names = append(names, name)
				}
				c.mu.RUnlock()

				for _, name := range names {
					if _, err := c.CreateCheckpoint(ctx, name); err != nil {
						c.logger.Warnw("checkpoint failed", "agent", name, "error", err)
					}
				}
			}
		}
	}()

	c.logger.Infow("checkpoint loop started", "interval", interval)
}

// cleanupOldCheckpoints keeps only the maxKeep most recent checkpoints for an agent.
func (c *Controller) cleanupOldCheckpoints(name string, maxKeep int) {
	dir := checkpointDir(name)
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}

	// Collect JSON files with their parsed timestamps.
	type fileInfo struct {
		name string
		time time.Time
	}
	var files []fileInfo
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}
		data, err := os.ReadFile(filepath.Join(dir, entry.Name()))
		if err != nil {
			continue
		}
		var cp Checkpoint
		if err := json.Unmarshal(data, &cp); err != nil {
			continue
		}
		files = append(files, fileInfo{name: entry.Name(), time: cp.Timestamp})
	}

	if len(files) <= maxKeep {
		return
	}

	// Sort newest first.
	sort.Slice(files, func(i, j int) bool {
		return files[i].time.After(files[j].time)
	})

	// Remove files beyond maxKeep.
	for _, f := range files[maxKeep:] {
		path := filepath.Join(dir, f.name)
		if err := os.Remove(path); err != nil {
			c.logger.Warnw("failed to remove old checkpoint", "file", f.name, "error", err)
		} else {
			c.logger.Debugw("removed old checkpoint", "agent", name, "file", f.name)
		}
	}
}
