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
	"github.com/zlc-ai/opc-platform/pkg/cost"
	"github.com/zlc-ai/opc-platform/pkg/storage"
)

// Controller manages Agent lifecycles.
type Controller struct {
	mu       sync.RWMutex
	store    storage.Store
	registry *adapter.Registry
	agents   map[string]*managedAgent
	logger   *zap.SugaredLogger
	costMgr  *cost.Tracker
	quotaEnforcer *cost.QuotaEnforcer

	// Circuit breaker: consecutive failure count per agent.
	failCounts map[string]int

	// Lifecycle management fields.
	lifecycleMu     sync.Mutex
	lifecycleStates map[string]*lifecycleState
}

// managedAgent tracks a running agent instance.
type managedAgent struct {
	spec    v1.AgentSpec
	adapter adapter.Adapter
	cancel  context.CancelFunc
}

// circuitBreakerThreshold is the number of consecutive failures before
// an agent is automatically stopped.
const circuitBreakerThreshold = 5

// New creates a new Controller.
func New(store storage.Store, registry *adapter.Registry, logger *zap.SugaredLogger) *Controller {
	return &Controller{
		store:         store,
		registry:      registry,
		agents:        make(map[string]*managedAgent),
		failCounts:    make(map[string]int),
		logger:        logger,
		quotaEnforcer: cost.NewQuotaEnforcer(logger),
	}
}

// SetCostTracker sets the cost tracker for recording task costs.
func (c *Controller) SetCostTracker(tracker *cost.Tracker) {
	c.costMgr = tracker
}

// RecoverAgents restarts all agents that were previously in Running phase.
// Call this on daemon startup to restore agent state from a prior session.
func (c *Controller) RecoverAgents(ctx context.Context) {
	start := time.Now()
	c.logger.Infow("RecoverAgents: starting agent recovery")
	agents, err := c.store.ListAgents(ctx)
	if err != nil {
		c.logger.Errorw("RecoverAgents: failed to list agents", "error", err)
		return
	}

	var recovered, failed int
	for _, record := range agents {
		if record.Phase == v1.AgentPhaseRunning || record.Phase == v1.AgentPhaseStarting {
			c.logger.Infow("RecoverAgents: recovering agent", "agentName", record.Name, "type", record.Type)
			if err := c.StartAgent(ctx, record.Name); err != nil {
				c.logger.Warnw("RecoverAgents: failed to recover agent", "agentName", record.Name, "error", err)
				failed++
			} else {
				recovered++
			}
		}
	}
	c.logger.Infow("RecoverAgents completed", "recovered", recovered, "failed", failed, "duration", time.Since(start))
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
	start := time.Now()
	c.logger.Infow("StartAgent", "agentName", name)
	record, err := c.store.GetAgent(ctx, name)
	if err != nil {
		c.logger.Errorw("StartAgent: agent not found", "agentName", name, "error", err)
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

	agentCtx, cancel := context.WithCancel(context.Background())
	startCtx, startCancel := context.WithTimeout(agentCtx, 30*time.Second)
	defer startCancel()
	if err := adp.Start(startCtx, spec); err != nil {
		cancel()
		record.Phase = v1.AgentPhaseFailed
		record.Message = err.Error()
		c.store.UpdateAgent(ctx, record)
		c.logger.Errorw("StartAgent: adapter start failed", "agentName", name, "error", err, "duration", time.Since(start))
		return fmt.Errorf("start agent %q: %w", name, err)
	}

	c.mu.Lock()
	c.agents[name] = &managedAgent{spec: spec, adapter: adp, cancel: cancel}
	c.mu.Unlock()

	record.Phase = v1.AgentPhaseRunning
	record.Message = ""
	c.store.UpdateAgent(ctx, record)

	c.logger.Infow("StartAgent completed", "agentName", name, "status", v1.AgentPhaseRunning, "duration", time.Since(start))
	return nil
}

// StopAgent stops a running agent.
func (c *Controller) StopAgent(ctx context.Context, name string) error {
	start := time.Now()
	c.logger.Infow("StopAgent", "agentName", name)
	c.mu.Lock()
	ma, ok := c.agents[name]
	if ok {
		delete(c.agents, name)
	}
	c.mu.Unlock()

	if !ok {
		c.logger.Warnw("StopAgent: agent not running", "agentName", name)
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

	c.logger.Infow("StopAgent completed", "agentName", name, "status", v1.AgentPhaseStopped, "duration", time.Since(start))
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
	execStart := time.Now()
	c.logger.Infow("ExecuteTask", "taskId", task.ID, "agentName", task.AgentName,
		"goalId", task.GoalID, "projectId", task.ProjectID, "issueId", task.IssueID)
	// Check budget before execution.
	if c.costMgr != nil {
		status := c.costMgr.GetBudgetStatus()
		if status.Exceeded {
			c.logger.Warnw("ExecuteTask: budget exceeded",
				"taskId", task.ID, "agentName", task.AgentName,
				"dailySpent", status.DailySpent, "dailyLimit", status.DailyLimit,
				"monthlySpent", status.MonthlySpent, "monthlyLimit", status.MonthlyLimit)
			now := time.Now()
			task.Status = v1.TaskStatusFailed
			task.Error = fmt.Sprintf("budget exceeded: daily %.2f/%.2f, monthly %.2f/%.2f",
				status.DailySpent, status.DailyLimit, status.MonthlySpent, status.MonthlyLimit)
			task.EndedAt = &now
			if storeErr := c.store.UpdateTask(ctx, task); storeErr != nil {
				c.logger.Errorw("failed to update task status", "task", task.ID, "error", storeErr)
			}
			return adapter.ExecuteResult{}, fmt.Errorf("budget exceeded")
		}
	}

	adp, err := c.GetAdapter(task.AgentName)
	if err != nil {
		now := time.Now()
		task.Status = v1.TaskStatusFailed
		task.Error = fmt.Sprintf("agent not available: %v", err)
		task.EndedAt = &now
		if storeErr := c.store.UpdateTask(ctx, task); storeErr != nil {
			c.logger.Errorw("failed to update task status", "task", task.ID, "error", storeErr)
		}
		return adapter.ExecuteResult{}, err
	}

	// Check per-agent quota (v0.7).
	if c.quotaEnforcer != nil {
		qc := c.buildQuotaConfig(task.AgentName)
		qr := c.quotaEnforcer.Check(task.AgentName, qc)
		if qr.AlertMsg != "" {
			c.logger.Warnw("quota alert", "agent", task.AgentName, "alert", qr.AlertMsg)
		}
		if !qr.Allowed {
			c.logger.Warnw("quota exceeded", "agent", task.AgentName, "reason", qr.Reason, "action", qr.Action)
			if qr.Action == cost.ExceedPause {
				if stopErr := c.StopAgent(ctx, task.AgentName); stopErr == nil {
					if rec, getErr := c.store.GetAgent(ctx, task.AgentName); getErr == nil {
						rec.Phase = v1.AgentPhaseStopped
						rec.Message = "paused: " + qr.Reason
						c.store.UpdateAgent(ctx, rec)
					}
				}
			}
			if qr.Action != cost.ExceedAlert {
				now := time.Now()
				task.Status = v1.TaskStatusFailed
				task.Error = "quota exceeded: " + qr.Reason
				task.EndedAt = &now
				c.store.UpdateTask(ctx, task)
				return adapter.ExecuteResult{}, fmt.Errorf("quota exceeded: %s", qr.Reason)
			}
		}
	}

	now := time.Now()
	task.Status = v1.TaskStatusRunning
	task.StartedAt = &now
	if storeErr := c.store.UpdateTask(ctx, task); storeErr != nil {
		c.logger.Errorw("failed to update task status", "task", task.ID, "error", storeErr)
	}

	result, err := adp.Execute(ctx, task)
	endTime := time.Now()
	if err != nil {
		c.logger.Errorw("ExecuteTask: execution failed",
			"taskId", task.ID, "agentName", task.AgentName, "error", err, "duration", endTime.Sub(execStart))
		task.Status = v1.TaskStatusFailed
		task.Error = err.Error()
		task.EndedAt = &endTime
		if storeErr := c.store.UpdateTask(ctx, task); storeErr != nil {
			c.logger.Errorw("failed to update task status", "task", task.ID, "error", storeErr)
		}

		// Circuit breaker: track consecutive failures per agent.
		c.mu.Lock()
		c.failCounts[task.AgentName]++
		count := c.failCounts[task.AgentName]
		c.mu.Unlock()

		if count >= circuitBreakerThreshold {
			c.logger.Warnw("circuit breaker triggered: auto-stopping agent after consecutive failures",
				"agent", task.AgentName, "consecutiveFailures", count)
			if stopErr := c.StopAgent(ctx, task.AgentName); stopErr != nil {
				c.logger.Errorw("failed to auto-stop agent via circuit breaker",
					"agent", task.AgentName, "error", stopErr)
			}
			// Mark agent phase as Failed.
			if record, getErr := c.store.GetAgent(ctx, task.AgentName); getErr == nil {
				record.Phase = v1.AgentPhaseFailed
				record.Message = fmt.Sprintf("circuit breaker: %d consecutive task failures", count)
				c.store.UpdateAgent(ctx, record)
			}
		}

		return adapter.ExecuteResult{}, err
	}

	// Reset consecutive failure count on success.
	c.mu.Lock()
	delete(c.failCounts, task.AgentName)
	c.mu.Unlock()

	task.Status = v1.TaskStatusCompleted
	task.Result = result.Output
	task.TokensIn = result.TokensIn
	task.TokensOut = result.TokensOut
	task.EndedAt = &endTime

	// Calculate and record cost.
	// Prefer agent-reported cost (e.g., Claude CLI's total_cost_usd) over our pricing table.
	if c.costMgr != nil {
		provider, model := c.getAgentModel(task.AgentName)
		var inCost, outCost float64
		if result.Cost > 0 {
			// Agent reported its own cost — use it directly.
			task.Cost = result.Cost
		} else {
			// Fall back to pricing table calculation.
			inCost, outCost = c.costMgr.CalculateCost(result.TokensIn, result.TokensOut, provider, model)
			task.Cost = inCost + outCost
		}
		_ = c.costMgr.RecordCost(cost.CostEvent{
			AgentName:     task.AgentName,
			TaskID:        task.ID,
			GoalRef:       task.GoalID,
			ProjectRef:    task.ProjectID,
			TokensIn:      result.TokensIn,
			TokensOut:     result.TokensOut,
			InputCost:     inCost,
			OutputCost:    outCost,
			Duration:      endTime.Sub(now),
			ModelProvider: provider,
			ModelName:     model,
		})
	}

	// Record usage for quota enforcement (v0.7).
	if c.quotaEnforcer != nil {
		c.quotaEnforcer.RecordUsage(task.AgentName, result.TokensIn, result.TokensOut, task.Cost)
	}

	if storeErr := c.store.UpdateTask(ctx, task); storeErr != nil {
		c.logger.Errorw("failed to update task status", "task", task.ID, "error", storeErr)
	}

	c.logger.Infow("ExecuteTask completed",
		"taskId", task.ID, "agentName", task.AgentName,
		"tokensIn", result.TokensIn, "tokensOut", result.TokensOut,
		"cost", task.Cost, "duration", endTime.Sub(execStart))

	return result, nil
}

// buildQuotaConfig extracts QuotaConfig from the agent's spec.
func (c *Controller) buildQuotaConfig(agentName string) cost.QuotaConfig {
	c.mu.RLock()
	ma, ok := c.agents[agentName]
	c.mu.RUnlock()
	if !ok {
		return cost.QuotaConfig{}
	}

	res := ma.spec.Spec.Resources
	return cost.QuotaConfig{
		TokenPerTask: res.TokenBudget.PerTask,
		TokenPerHour: res.TokenBudget.PerHour,
		TokenPerDay:  res.TokenBudget.PerDay,
		CostPerTask:  cost.ParseCostString(res.CostLimit.PerTask),
		CostPerDay:   cost.ParseCostString(res.CostLimit.PerDay),
		CostPerMonth: cost.ParseCostString(res.CostLimit.PerMonth),
		OnExceed:     cost.ExceedAction(res.OnExceed),
	}
}

// StreamTask runs a task with streaming output.
func (c *Controller) StreamTask(ctx context.Context, task v1.TaskRecord) (<-chan adapter.Chunk, error) {
	adp, err := c.GetAdapter(task.AgentName)
	if err != nil {
		now := time.Now()
		task.Status = v1.TaskStatusFailed
		task.Error = fmt.Sprintf("agent not available: %v", err)
		task.EndedAt = &now
		if storeErr := c.store.UpdateTask(ctx, task); storeErr != nil {
			c.logger.Errorw("failed to update task status", "task", task.ID, "error", storeErr)
		}
		return nil, err
	}

	now := time.Now()
	task.Status = v1.TaskStatusRunning
	task.StartedAt = &now
	if storeErr := c.store.UpdateTask(ctx, task); storeErr != nil {
		c.logger.Errorw("failed to update task status", "task", task.ID, "error", storeErr)
	}

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
		m := ma.adapter.Metrics()

		// Enrich with cost data from task records.
		tasks, err := c.store.ListTasksByAgent(context.Background(), name)
		if err == nil {
			var totalCost float64
			var totalIn, totalOut int
			var completed, failed, running int
			for _, t := range tasks {
				totalCost += t.Cost
				totalIn += t.TokensIn
				totalOut += t.TokensOut
				switch t.Status {
				case v1.TaskStatusCompleted:
					completed++
				case v1.TaskStatusFailed:
					failed++
				case v1.TaskStatusRunning:
					running++
				}
			}
			m.TotalCost = totalCost
			m.TotalTokensIn = totalIn
			m.TotalTokensOut = totalOut
			m.TasksCompleted = completed
			m.TasksFailed = failed
			m.TasksRunning = running
		}

		result[name] = m
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

// getAgentModel returns the provider and model name for a running agent.
func (c *Controller) getAgentModel(agentName string) (provider, model string) {
	c.mu.RLock()
	ma, ok := c.agents[agentName]
	c.mu.RUnlock()
	if ok {
		return ma.spec.Spec.Runtime.Model.Provider, ma.spec.Spec.Runtime.Model.Name
	}
	return "anthropic", "claude-sonnet-4" // default fallback
}
