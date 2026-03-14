package controller

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"go.uber.org/zap"
	v1 "github.com/zlc-ai/opc-platform/api/v1"
	"github.com/zlc-ai/opc-platform/pkg/adapter"
	"github.com/zlc-ai/opc-platform/pkg/storage"
)

// Controller manages Agent lifecycles.
type Controller struct {
	mu       sync.RWMutex
	store    storage.Store
	registry *adapter.Registry
	agents   map[string]*managedAgent
	logger   *zap.SugaredLogger
}

// managedAgent tracks a running agent instance.
type managedAgent struct {
	spec    v1.AgentSpec
	adapter adapter.Adapter
	cancel  context.CancelFunc
}

// New creates a new Controller.
func New(store storage.Store, registry *adapter.Registry, logger *zap.SugaredLogger) *Controller {
	return &Controller{
		store:    store,
		registry: registry,
		agents:   make(map[string]*managedAgent),
		logger:   logger,
	}
}

// Apply creates or updates an Agent from an AgentSpec.
func (c *Controller) Apply(ctx context.Context, spec v1.AgentSpec) error {
	name := spec.Metadata.Name
	if name == "" {
		return fmt.Errorf("agent name is required")
	}

	// Check if agent exists.
	existing, err := c.store.GetAgent(ctx, name)
	if err == nil {
		// Update existing agent.
		specYAML, _ := marshalSpec(spec)
		existing.Type = spec.Spec.Type
		existing.SpecYAML = specYAML
		existing.UpdatedAt = time.Now()
		if err := c.store.UpdateAgent(ctx, existing); err != nil {
			return fmt.Errorf("update agent: %w", err)
		}
		c.logger.Infow("agent updated", "name", name)
		return nil
	}

	// Create new agent.
	specYAML, _ := marshalSpec(spec)
	record := v1.AgentRecord{
		Name:      name,
		Type:      spec.Spec.Type,
		Phase:     v1.AgentPhaseCreated,
		SpecYAML:  specYAML,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	if err := c.store.CreateAgent(ctx, record); err != nil {
		return fmt.Errorf("create agent: %w", err)
	}

	c.logger.Infow("agent created", "name", name, "type", spec.Spec.Type)
	return nil
}

// StartAgent starts an agent by name.
func (c *Controller) StartAgent(ctx context.Context, name string) error {
	record, err := c.store.GetAgent(ctx, name)
	if err != nil {
		return fmt.Errorf("get agent %q: %w", name, err)
	}

	spec, err := unmarshalSpec(record.SpecYAML)
	if err != nil {
		return fmt.Errorf("parse agent spec: %w", err)
	}

	adp, err := c.registry.Create(record.Type)
	if err != nil {
		return fmt.Errorf("create adapter for %q: %w", record.Type, err)
	}

	agentCtx, cancel := context.WithCancel(ctx)
	if err := adp.Start(agentCtx, spec); err != nil {
		cancel()
		record.Phase = v1.AgentPhaseFailed
		record.Message = err.Error()
		c.store.UpdateAgent(ctx, record)
		return fmt.Errorf("start agent %q: %w", name, err)
	}

	c.mu.Lock()
	c.agents[name] = &managedAgent{spec: spec, adapter: adp, cancel: cancel}
	c.mu.Unlock()

	record.Phase = v1.AgentPhaseRunning
	record.Message = ""
	c.store.UpdateAgent(ctx, record)

	c.logger.Infow("agent started", "name", name)
	return nil
}

// StopAgent stops a running agent.
func (c *Controller) StopAgent(ctx context.Context, name string) error {
	c.mu.Lock()
	ma, ok := c.agents[name]
	if ok {
		delete(c.agents, name)
	}
	c.mu.Unlock()

	if !ok {
		return fmt.Errorf("agent %q is not running", name)
	}

	if err := ma.adapter.Stop(ctx); err != nil {
		c.logger.Warnw("error stopping agent", "name", name, "error", err)
	}
	ma.cancel()

	record, err := c.store.GetAgent(ctx, name)
	if err == nil {
		record.Phase = v1.AgentPhaseStopped
		c.store.UpdateAgent(ctx, record)
	}

	c.logger.Infow("agent stopped", "name", name)
	return nil
}

// DeleteAgent removes an agent.
func (c *Controller) DeleteAgent(ctx context.Context, name string) error {
	// Stop if running.
	c.mu.RLock()
	_, running := c.agents[name]
	c.mu.RUnlock()
	if running {
		c.StopAgent(ctx, name)
	}

	if err := c.store.DeleteAgent(ctx, name); err != nil {
		return err
	}

	c.logger.Infow("agent deleted", "name", name)
	return nil
}

// GetAgent returns an agent record.
func (c *Controller) GetAgent(ctx context.Context, name string) (v1.AgentRecord, error) {
	return c.store.GetAgent(ctx, name)
}

// ListAgents returns all agent records.
func (c *Controller) ListAgents(ctx context.Context) ([]v1.AgentRecord, error) {
	return c.store.ListAgents(ctx)
}

// Store returns the underlying store.
func (c *Controller) Store() storage.Store {
	return c.store
}

// GetAdapter returns the adapter for a running agent.
func (c *Controller) GetAdapter(name string) (adapter.Adapter, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	ma, ok := c.agents[name]
	if !ok {
		return nil, fmt.Errorf("agent %q is not running", name)
	}
	return ma.adapter, nil
}

// ExecuteTask runs a task against a named agent.
func (c *Controller) ExecuteTask(ctx context.Context, task v1.TaskRecord) (adapter.ExecuteResult, error) {
	adp, err := c.GetAdapter(task.AgentName)
	if err != nil {
		return adapter.ExecuteResult{}, err
	}

	now := time.Now()
	task.Status = v1.TaskStatusRunning
	task.StartedAt = &now
	c.store.UpdateTask(ctx, task)

	result, err := adp.Execute(ctx, task)
	endTime := time.Now()
	if err != nil {
		task.Status = v1.TaskStatusFailed
		task.Error = err.Error()
		task.EndedAt = &endTime
		c.store.UpdateTask(ctx, task)
		return adapter.ExecuteResult{}, err
	}

	task.Status = v1.TaskStatusCompleted
	task.Result = result.Output
	task.TokensIn = result.TokensIn
	task.TokensOut = result.TokensOut
	task.EndedAt = &endTime
	c.store.UpdateTask(ctx, task)

	return result, nil
}

// StreamTask runs a task with streaming output.
func (c *Controller) StreamTask(ctx context.Context, task v1.TaskRecord) (<-chan adapter.Chunk, error) {
	adp, err := c.GetAdapter(task.AgentName)
	if err != nil {
		return nil, err
	}

	now := time.Now()
	task.Status = v1.TaskStatusRunning
	task.StartedAt = &now
	c.store.UpdateTask(ctx, task)

	return adp.Stream(ctx, task)
}

// Health returns health status for all running agents.
func (c *Controller) Health() map[string]v1.HealthStatus {
	c.mu.RLock()
	defer c.mu.RUnlock()

	result := make(map[string]v1.HealthStatus, len(c.agents))
	for name, ma := range c.agents {
		result[name] = ma.adapter.Health()
	}
	return result
}

// Metrics returns metrics for all running agents.
func (c *Controller) AgentMetrics() map[string]v1.AgentMetrics {
	c.mu.RLock()
	defer c.mu.RUnlock()

	result := make(map[string]v1.AgentMetrics, len(c.agents))
	for name, ma := range c.agents {
		result[name] = ma.adapter.Metrics()
	}
	return result
}

// --- helpers ---

func marshalSpec(spec v1.AgentSpec) (string, error) {
	// Use JSON for internal storage (simpler than YAML dependency).
	data, err := json.Marshal(spec)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func unmarshalSpec(data string) (v1.AgentSpec, error) {
	var spec v1.AgentSpec
	if err := json.Unmarshal([]byte(data), &spec); err != nil {
		return spec, err
	}
	return spec, nil
}
