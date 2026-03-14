package dispatcher

import (
	"context"
	"fmt"
	"sync"

	v1 "github.com/zlc-ai/opc-platform/api/v1"
	"github.com/zlc-ai/opc-platform/pkg/controller"
	"go.uber.org/zap"
)

// Strategy defines how tasks are routed to agents.
type Strategy string

const (
	StrategyAuto          Strategy = "auto"
	StrategyRoundRobin    Strategy = "round-robin"
	StrategyLeastBusy     Strategy = "least-busy"
	StrategyCostOptimized Strategy = "cost-optimized"
)

// DispatcherSpec is the YAML configuration for the dispatcher.
type DispatcherSpec struct {
	APIVersion string         `yaml:"apiVersion" json:"apiVersion"`
	Kind       string         `yaml:"kind" json:"kind"`
	Metadata   v1.Metadata    `yaml:"metadata" json:"metadata"`
	Spec       DispatcherBody `yaml:"spec" json:"spec"`
}

// DispatcherBody contains the spec fields for a DispatcherSpec.
type DispatcherBody struct {
	Strategy Strategy       `yaml:"strategy" json:"strategy"`
	Routing  []RoutingRule  `yaml:"routing,omitempty" json:"routing,omitempty"`
	Fallback FallbackConfig `yaml:"fallback,omitempty" json:"fallback,omitempty"`
}

// RoutingRule maps matching criteria to a set of candidate agents.
type RoutingRule struct {
	Match      MatchCriteria `yaml:"match" json:"match"`
	Agents     []string      `yaml:"agents" json:"agents"`
	Preference string        `yaml:"preference,omitempty" json:"preference,omitempty"`
}

// MatchCriteria defines the conditions under which a routing rule applies.
type MatchCriteria struct {
	TaskType string            `yaml:"taskType,omitempty" json:"taskType,omitempty"`
	Labels   map[string]string `yaml:"labels,omitempty" json:"labels,omitempty"`
}

// FallbackConfig specifies a default agent when no routing rules match.
type FallbackConfig struct {
	Agent string `yaml:"agent" json:"agent"`
}

// Dispatcher routes tasks to the best available agent based on configurable strategies.
type Dispatcher struct {
	controller *controller.Controller
	config     DispatcherBody
	logger     *zap.SugaredLogger

	// Round-robin state.
	mu         sync.Mutex
	rrCounters map[string]int // per-group round-robin counter
}

// New creates a new Dispatcher.
func New(ctrl *controller.Controller, config DispatcherBody, logger *zap.SugaredLogger) *Dispatcher {
	return &Dispatcher{
		controller: ctrl,
		config:     config,
		logger:     logger,
		rrCounters: make(map[string]int),
	}
}

// Dispatch selects the best agent for the given task and returns its name.
func (d *Dispatcher) Dispatch(ctx context.Context, taskType string, labels map[string]string, message string) (string, error) {
	// Find a matching routing rule.
	rule := d.findMatchingRule(taskType, labels)

	var candidates []string
	var strategy Strategy

	if rule != nil {
		candidates = rule.Agents
		// Use rule-level preference if set, otherwise fall back to global strategy.
		if rule.Preference != "" {
			strategy = Strategy(rule.Preference)
		} else {
			strategy = d.config.Strategy
		}
		d.logger.Debugw("routing rule matched",
			"taskType", taskType,
			"candidates", candidates,
			"strategy", strategy,
		)
	} else if d.config.Fallback.Agent != "" {
		// No rule matched; use the fallback agent.
		d.logger.Debugw("no routing rule matched, using fallback",
			"fallback", d.config.Fallback.Agent,
		)
		return d.config.Fallback.Agent, nil
	} else {
		// No rules and no fallback; collect all running agents as candidates.
		agents, err := d.controller.ListAgents(ctx)
		if err != nil {
			return "", fmt.Errorf("list agents: %w", err)
		}
		for _, a := range agents {
			if a.Phase == v1.AgentPhaseRunning {
				candidates = append(candidates, a.Name)
			}
		}
		strategy = d.config.Strategy
		if strategy == "" {
			strategy = StrategyAuto
		}
	}

	if len(candidates) == 0 {
		return "", fmt.Errorf("no available agents for dispatch (taskType=%q)", taskType)
	}

	selected, err := d.selectAgent(candidates, strategy)
	if err != nil {
		return "", fmt.Errorf("select agent: %w", err)
	}

	d.logger.Infow("task dispatched",
		"taskType", taskType,
		"agent", selected,
		"strategy", strategy,
	)
	return selected, nil
}

// selectAgent picks the best agent from candidates using the given strategy.
func (d *Dispatcher) selectAgent(candidates []string, strategy Strategy) (string, error) {
	if len(candidates) == 0 {
		return "", fmt.Errorf("no candidates provided")
	}

	// Single candidate is a trivial case.
	if len(candidates) == 1 {
		return candidates[0], nil
	}

	switch strategy {
	case StrategyRoundRobin:
		return d.roundRobin(candidates, groupKey(candidates)), nil
	case StrategyLeastBusy:
		return d.leastBusy(candidates)
	case StrategyCostOptimized:
		return d.costOptimized(candidates)
	case StrategyAuto:
		// Auto strategy: try least-busy first, fall back to round-robin.
		agent, err := d.leastBusy(candidates)
		if err != nil {
			return d.roundRobin(candidates, groupKey(candidates)), nil
		}
		return agent, nil
	default:
		return d.roundRobin(candidates, groupKey(candidates)), nil
	}
}

// roundRobin selects the next agent in a round-robin fashion for the given group.
func (d *Dispatcher) roundRobin(candidates []string, group string) string {
	d.mu.Lock()
	defer d.mu.Unlock()

	idx := d.rrCounters[group] % len(candidates)
	d.rrCounters[group] = idx + 1

	return candidates[idx]
}

// leastBusy picks the agent with the fewest running tasks.
func (d *Dispatcher) leastBusy(candidates []string) (string, error) {
	allMetrics := d.controller.AgentMetrics()

	bestAgent := ""
	bestCount := -1

	for _, name := range candidates {
		m, ok := allMetrics[name]
		if !ok {
			// Agent without metrics is assumed idle (0 running tasks).
			return name, nil
		}
		if bestCount < 0 || m.TasksRunning < bestCount {
			bestCount = m.TasksRunning
			bestAgent = name
		}
	}

	if bestAgent == "" {
		return "", fmt.Errorf("no metrics available for any candidate")
	}
	return bestAgent, nil
}

// costOptimized picks the agent with the lowest total cost so far.
func (d *Dispatcher) costOptimized(candidates []string) (string, error) {
	allMetrics := d.controller.AgentMetrics()

	bestAgent := ""
	bestCost := -1.0

	for _, name := range candidates {
		m, ok := allMetrics[name]
		if !ok {
			// Agent without metrics is assumed zero cost.
			return name, nil
		}
		if bestCost < 0 || m.TotalCost < bestCost {
			bestCost = m.TotalCost
			bestAgent = name
		}
	}

	if bestAgent == "" {
		return "", fmt.Errorf("no metrics available for any candidate")
	}
	return bestAgent, nil
}

// findMatchingRule returns the first routing rule whose criteria match the given
// task type and labels, or nil if no rule matches.
func (d *Dispatcher) findMatchingRule(taskType string, labels map[string]string) *RoutingRule {
	for i := range d.config.Routing {
		rule := &d.config.Routing[i]
		if matchesRule(rule.Match, taskType, labels) {
			return rule
		}
	}
	return nil
}

// matchesRule checks whether a task's type and labels satisfy a MatchCriteria.
func matchesRule(match MatchCriteria, taskType string, labels map[string]string) bool {
	// If the rule specifies a task type, it must match.
	if match.TaskType != "" && match.TaskType != taskType {
		return false
	}

	// All labels specified in the rule must be present with matching values.
	for k, v := range match.Labels {
		if labels[k] != v {
			return false
		}
	}

	// At least one criterion must be specified for a match.
	if match.TaskType == "" && len(match.Labels) == 0 {
		return false
	}

	return true
}

// groupKey produces a stable key for a set of candidate agent names.
func groupKey(candidates []string) string {
	key := ""
	for i, c := range candidates {
		if i > 0 {
			key += ","
		}
		key += c
	}
	return key
}
