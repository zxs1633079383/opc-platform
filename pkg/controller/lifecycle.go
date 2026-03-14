package controller

import (
	"context"
	"fmt"
	"math"
	"sync"
	"time"

	v1 "github.com/zlc-ai/opc-platform/api/v1"
)

const (
	defaultHealthCheckInterval = 30 * time.Second
	defaultMaxRestarts         = 5
	defaultRestartDelay        = 5 * time.Second
	defaultBackoffStrategy     = "exponential"
	maxBackoffDelay            = 5 * time.Minute
)

// lifecycleState tracks restart and health check state for a managed agent.
type lifecycleState struct {
	mu                sync.Mutex
	restartCount      int
	lastRestart       time.Time
	healthCheckCancel context.CancelFunc
}

// StartHealthCheckLoop starts a background goroutine that periodically checks
// the health of all running agents and triggers auto-restart when needed.
func (c *Controller) StartHealthCheckLoop(ctx context.Context) {
	go c.healthCheckLoop(ctx)
	c.logger.Infow("health check loop started")
}

// healthCheckLoop runs the main health check loop.
func (c *Controller) healthCheckLoop(ctx context.Context) {
	ticker := time.NewTicker(defaultHealthCheckInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			c.logger.Infow("health check loop stopped", "reason", ctx.Err())
			return
		case <-ticker.C:
			c.runHealthChecks(ctx)
		}
	}
}

// runHealthChecks iterates over all running agents and checks their health.
func (c *Controller) runHealthChecks(ctx context.Context) {
	c.mu.RLock()
	agents := make(map[string]*managedAgent, len(c.agents))
	for name, ma := range c.agents {
		agents[name] = ma
	}
	c.mu.RUnlock()

	for name, ma := range agents {
		c.checkAndRestart(ctx, name, ma)
	}
}

// checkAndRestart checks the health of a specific agent and triggers a restart
// if the health check fails and recovery is enabled.
func (c *Controller) checkAndRestart(ctx context.Context, name string, ma *managedAgent) {
	status := ma.adapter.Health()
	if status.Healthy {
		return
	}

	c.logger.Warnw("agent health check failed",
		"name", name,
		"message", status.Message,
	)

	recovery := ma.spec.Spec.Recovery
	if !recovery.Enabled {
		c.logger.Infow("auto-restart disabled for agent, skipping", "name", name)
		return
	}

	ls := c.getLifecycleState(name)
	ls.mu.Lock()
	defer ls.mu.Unlock()

	maxRestarts := recovery.MaxRestarts
	if maxRestarts <= 0 {
		maxRestarts = defaultMaxRestarts
	}

	if ls.restartCount >= maxRestarts {
		c.logger.Errorw("agent exceeded max restart limit",
			"name", name,
			"restarts", ls.restartCount,
			"maxRestarts", maxRestarts,
		)
		c.updateAgentPhase(ctx, name, v1.AgentPhaseFailed,
			fmt.Sprintf("exceeded max restarts (%d)", maxRestarts))
		return
	}

	baseDelay := parseOrDefault(recovery.RestartDelay, defaultRestartDelay)
	strategy := recovery.Backoff
	if strategy == "" {
		strategy = defaultBackoffStrategy
	}

	backoff := calculateBackoff(ls.restartCount, baseDelay, strategy)

	timeSinceLastRestart := time.Since(ls.lastRestart)
	if timeSinceLastRestart < backoff {
		c.logger.Debugw("backoff period active, skipping restart",
			"name", name,
			"remaining", backoff-timeSinceLastRestart,
		)
		return
	}

	c.logger.Infow("auto-restarting agent",
		"name", name,
		"attempt", ls.restartCount+1,
		"maxRestarts", maxRestarts,
		"backoff", backoff,
	)

	ls.restartCount++
	ls.lastRestart = time.Now()

	c.updateAgentPhase(ctx, name, v1.AgentPhaseRetrying,
		fmt.Sprintf("restart attempt %d/%d", ls.restartCount, maxRestarts))

	go func() {
		if err := c.restartAgentInternal(ctx, name); err != nil {
			c.logger.Errorw("auto-restart failed",
				"name", name,
				"error", err,
				"attempt", ls.restartCount,
			)
			c.updateAgentPhase(ctx, name, v1.AgentPhaseFailed, err.Error())
		}
	}()
}

// RestartAgent stops and restarts a specific agent by name. This is the public
// method used by the `opctl restart agent <name>` command. It resets the
// restart counter on successful restart.
func (c *Controller) RestartAgent(ctx context.Context, name string) error {
	c.mu.RLock()
	_, running := c.agents[name]
	c.mu.RUnlock()

	if !running {
		// Agent is not running; try to start it fresh.
		return c.StartAgent(ctx, name)
	}

	if err := c.restartAgentInternal(ctx, name); err != nil {
		return fmt.Errorf("restart agent %q: %w", name, err)
	}

	// Reset restart count on manual restart.
	ls := c.getLifecycleState(name)
	ls.mu.Lock()
	ls.restartCount = 0
	ls.lastRestart = time.Time{}
	ls.mu.Unlock()

	c.logger.Infow("agent restarted (manual)", "name", name)
	return nil
}

// restartAgentInternal performs the stop-then-start sequence for an agent.
func (c *Controller) restartAgentInternal(ctx context.Context, name string) error {
	if err := c.StopAgent(ctx, name); err != nil {
		c.logger.Warnw("error stopping agent during restart", "name", name, "error", err)
		// Continue with start attempt even if stop had issues.
	}

	if err := c.StartAgent(ctx, name); err != nil {
		return fmt.Errorf("start agent after restart: %w", err)
	}

	return nil
}

// GetRestartCount returns the current restart count for an agent.
func (c *Controller) GetRestartCount(name string) int {
	ls := c.getLifecycleState(name)
	ls.mu.Lock()
	defer ls.mu.Unlock()
	return ls.restartCount
}

// ResetRestartCount resets the restart counter for an agent.
func (c *Controller) ResetRestartCount(name string) {
	ls := c.getLifecycleState(name)
	ls.mu.Lock()
	ls.restartCount = 0
	ls.lastRestart = time.Time{}
	ls.mu.Unlock()
}

// StopHealthCheckLoop cancels all per-agent health check goroutines.
func (c *Controller) StopHealthCheckLoop() {
	c.lifecycleMu.Lock()
	defer c.lifecycleMu.Unlock()

	for name, ls := range c.lifecycleStates {
		ls.mu.Lock()
		if ls.healthCheckCancel != nil {
			ls.healthCheckCancel()
			ls.healthCheckCancel = nil
		}
		ls.mu.Unlock()
		c.logger.Debugw("health check stopped for agent", "name", name)
	}

	c.logger.Infow("all health check loops stopped")
}

// StartAgentHealthCheck starts an individual health check goroutine for a
// specific agent with the interval defined in its spec.
func (c *Controller) StartAgentHealthCheck(ctx context.Context, name string) {
	c.mu.RLock()
	ma, ok := c.agents[name]
	c.mu.RUnlock()
	if !ok {
		return
	}

	interval := parseOrDefault(ma.spec.Spec.HealthCheck.Interval, defaultHealthCheckInterval)

	ls := c.getLifecycleState(name)
	ls.mu.Lock()
	if ls.healthCheckCancel != nil {
		ls.healthCheckCancel()
	}
	hcCtx, cancel := context.WithCancel(ctx)
	ls.healthCheckCancel = cancel
	ls.mu.Unlock()

	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for {
			select {
			case <-hcCtx.Done():
				return
			case <-ticker.C:
				c.mu.RLock()
				currentMA, exists := c.agents[name]
				c.mu.RUnlock()
				if !exists {
					return
				}
				c.checkAndRestart(hcCtx, name, currentMA)
			}
		}
	}()

	c.logger.Infow("agent health check started", "name", name, "interval", interval)
}

// StopAgentHealthCheck stops the health check goroutine for a specific agent.
func (c *Controller) StopAgentHealthCheck(name string) {
	ls := c.getLifecycleState(name)
	ls.mu.Lock()
	defer ls.mu.Unlock()

	if ls.healthCheckCancel != nil {
		ls.healthCheckCancel()
		ls.healthCheckCancel = nil
	}
}

// getLifecycleState returns the lifecycleState for an agent, creating one if
// it does not exist.
func (c *Controller) getLifecycleState(name string) *lifecycleState {
	c.lifecycleMu.Lock()
	defer c.lifecycleMu.Unlock()

	if c.lifecycleStates == nil {
		c.lifecycleStates = make(map[string]*lifecycleState)
	}

	ls, ok := c.lifecycleStates[name]
	if !ok {
		ls = &lifecycleState{}
		c.lifecycleStates[name] = ls
	}
	return ls
}

// updateAgentPhase updates the phase and message for an agent record in storage.
func (c *Controller) updateAgentPhase(ctx context.Context, name string, phase v1.AgentPhase, message string) {
	record, err := c.store.GetAgent(ctx, name)
	if err != nil {
		c.logger.Warnw("failed to get agent record for phase update",
			"name", name, "error", err)
		return
	}
	record.Phase = phase
	record.Message = message
	record.UpdatedAt = time.Now()
	if err := c.store.UpdateAgent(ctx, record); err != nil {
		c.logger.Warnw("failed to update agent phase",
			"name", name, "phase", phase, "error", err)
	}
}

// calculateBackoff computes the delay before the next restart attempt based on
// the number of previous restarts, a base delay, and a backoff strategy.
// Supported strategies: "exponential" (default), "linear", "fixed".
func calculateBackoff(restarts int, baseDelay time.Duration, strategy string) time.Duration {
	var delay time.Duration

	switch strategy {
	case "linear":
		// Linear: baseDelay * (restarts + 1)
		delay = baseDelay * time.Duration(restarts+1)
	case "fixed":
		// Fixed: always use baseDelay
		delay = baseDelay
	case "exponential":
		fallthrough
	default:
		// Exponential: baseDelay * 2^restarts
		multiplier := math.Pow(2, float64(restarts))
		delay = time.Duration(float64(baseDelay) * multiplier)
	}

	if delay > maxBackoffDelay {
		delay = maxBackoffDelay
	}

	return delay
}

// parseOrDefault parses a duration string and returns a default if parsing
// fails or the string is empty.
func parseOrDefault(s string, def time.Duration) time.Duration {
	if s == "" {
		return def
	}
	d, err := time.ParseDuration(s)
	if err != nil {
		return def
	}
	return d
}
