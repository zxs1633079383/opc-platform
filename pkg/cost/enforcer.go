package cost

import (
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"go.uber.org/zap"
)

// ExceedAction defines what to do when a quota is exceeded.
type ExceedAction string

const (
	ExceedPause  ExceedAction = "pause"  // Pause the agent.
	ExceedAlert  ExceedAction = "alert"  // Log a warning but continue.
	ExceedReject ExceedAction = "reject" // Reject the task.
)

// QuotaConfig holds the quota configuration for an agent.
type QuotaConfig struct {
	TokenPerTask int     // Max tokens per single task.
	TokenPerHour int     // Max tokens per hour.
	TokenPerDay  int     // Max tokens per day.
	CostPerTask  float64 // Max cost ($) per task.
	CostPerDay   float64 // Max cost ($) per day.
	CostPerMonth float64 // Max cost ($) per month.
	OnExceed     ExceedAction
}

// QuotaCheckResult holds the result of a quota check.
type QuotaCheckResult struct {
	Allowed  bool
	Action   ExceedAction
	Reason   string
	AlertMsg string // Non-empty if approaching limit (>80%).
}

// agentUsage tracks cumulative usage for an agent.
type agentUsage struct {
	hourlyTokens  int
	dailyTokens   int
	dailyCost     float64
	monthlyCost   float64
	hourReset     time.Time
	dayReset      time.Time
	monthReset    time.Time
}

// QuotaEnforcer checks agent-level token and cost quotas before task execution.
type QuotaEnforcer struct {
	mu     sync.RWMutex
	usage  map[string]*agentUsage // agent name -> usage
	logger *zap.SugaredLogger
}

// NewQuotaEnforcer creates a new QuotaEnforcer.
func NewQuotaEnforcer(logger *zap.SugaredLogger) *QuotaEnforcer {
	return &QuotaEnforcer{
		usage:  make(map[string]*agentUsage),
		logger: logger,
	}
}

// Check verifies that executing a task for the given agent would not exceed quotas.
// Call this BEFORE executing a task.
func (e *QuotaEnforcer) Check(agentName string, config QuotaConfig) QuotaCheckResult {
	if isZeroConfig(config) {
		return QuotaCheckResult{Allowed: true}
	}

	e.mu.RLock()
	u := e.getOrCreateUsageLocked(agentName)
	e.mu.RUnlock()

	now := time.Now()
	action := config.OnExceed
	if action == "" {
		action = ExceedReject
	}

	// Check hourly token budget.
	if config.TokenPerHour > 0 {
		hourlyTokens := u.hourlyTokens
		if now.After(u.hourReset) {
			hourlyTokens = 0
		}
		if hourlyTokens >= config.TokenPerHour {
			return QuotaCheckResult{
				Allowed: false, Action: action,
				Reason: fmt.Sprintf("agent %q hourly token budget exceeded: %d/%d", agentName, hourlyTokens, config.TokenPerHour),
			}
		}
		if pct := float64(hourlyTokens) / float64(config.TokenPerHour); pct >= 0.8 {
			return QuotaCheckResult{
				Allowed: true,
				AlertMsg: fmt.Sprintf("agent %q hourly token budget at %.0f%%: %d/%d", agentName, pct*100, hourlyTokens, config.TokenPerHour),
			}
		}
	}

	// Check daily token budget.
	if config.TokenPerDay > 0 {
		dailyTokens := u.dailyTokens
		if now.After(u.dayReset) {
			dailyTokens = 0
		}
		if dailyTokens >= config.TokenPerDay {
			return QuotaCheckResult{
				Allowed: false, Action: action,
				Reason: fmt.Sprintf("agent %q daily token budget exceeded: %d/%d", agentName, dailyTokens, config.TokenPerDay),
			}
		}
		if pct := float64(dailyTokens) / float64(config.TokenPerDay); pct >= 0.8 {
			return QuotaCheckResult{
				Allowed: true,
				AlertMsg: fmt.Sprintf("agent %q daily token budget at %.0f%%: %d/%d", agentName, pct*100, dailyTokens, config.TokenPerDay),
			}
		}
	}

	// Check daily cost limit.
	if config.CostPerDay > 0 {
		dailyCost := u.dailyCost
		if now.After(u.dayReset) {
			dailyCost = 0
		}
		if dailyCost >= config.CostPerDay {
			return QuotaCheckResult{
				Allowed: false, Action: action,
				Reason: fmt.Sprintf("agent %q daily cost limit exceeded: $%.4f/$%.4f", agentName, dailyCost, config.CostPerDay),
			}
		}
	}

	// Check monthly cost limit.
	if config.CostPerMonth > 0 {
		monthlyCost := u.monthlyCost
		if now.After(u.monthReset) {
			monthlyCost = 0
		}
		if monthlyCost >= config.CostPerMonth {
			return QuotaCheckResult{
				Allowed: false, Action: action,
				Reason: fmt.Sprintf("agent %q monthly cost limit exceeded: $%.4f/$%.4f", agentName, monthlyCost, config.CostPerMonth),
			}
		}
	}

	return QuotaCheckResult{Allowed: true}
}

// RecordUsage updates the cumulative usage for an agent after a task completes.
func (e *QuotaEnforcer) RecordUsage(agentName string, tokensIn, tokensOut int, cost float64) {
	e.mu.Lock()
	defer e.mu.Unlock()

	u := e.getOrCreateUsageLocked(agentName)
	now := time.Now()

	// Reset windows if expired.
	if now.After(u.hourReset) {
		u.hourlyTokens = 0
		u.hourReset = now.Truncate(time.Hour).Add(time.Hour)
	}
	if now.After(u.dayReset) {
		u.dailyTokens = 0
		u.dailyCost = 0
		u.dayReset = time.Date(now.Year(), now.Month(), now.Day()+1, 0, 0, 0, 0, now.Location())
	}
	if now.After(u.monthReset) {
		u.monthlyCost = 0
		u.monthReset = time.Date(now.Year(), now.Month()+1, 1, 0, 0, 0, 0, now.Location())
	}

	totalTokens := tokensIn + tokensOut
	u.hourlyTokens += totalTokens
	u.dailyTokens += totalTokens
	u.dailyCost += cost
	u.monthlyCost += cost
}

// GetUsage returns the current usage for an agent (for API/debugging).
func (e *QuotaEnforcer) GetUsage(agentName string) (hourlyTokens, dailyTokens int, dailyCost, monthlyCost float64) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	u, ok := e.usage[agentName]
	if !ok {
		return 0, 0, 0, 0
	}

	now := time.Now()
	if now.After(u.hourReset) {
		hourlyTokens = 0
	} else {
		hourlyTokens = u.hourlyTokens
	}
	if now.After(u.dayReset) {
		dailyTokens = 0
		dailyCost = 0
	} else {
		dailyTokens = u.dailyTokens
		dailyCost = u.dailyCost
	}
	if now.After(u.monthReset) {
		monthlyCost = 0
	} else {
		monthlyCost = u.monthlyCost
	}
	return
}

func (e *QuotaEnforcer) getOrCreateUsageLocked(agentName string) *agentUsage {
	u, ok := e.usage[agentName]
	if !ok {
		now := time.Now()
		u = &agentUsage{
			hourReset:  now.Truncate(time.Hour).Add(time.Hour),
			dayReset:   time.Date(now.Year(), now.Month(), now.Day()+1, 0, 0, 0, 0, now.Location()),
			monthReset: time.Date(now.Year(), now.Month()+1, 1, 0, 0, 0, 0, now.Location()),
		}
		e.usage[agentName] = u
	}
	return u
}

func isZeroConfig(c QuotaConfig) bool {
	return c.TokenPerTask == 0 && c.TokenPerHour == 0 && c.TokenPerDay == 0 &&
		c.CostPerTask == 0 && c.CostPerDay == 0 && c.CostPerMonth == 0
}

// ParseCostString parses a cost string like "$1.00" or "1.5" into a float64.
func ParseCostString(s string) float64 {
	s = strings.TrimSpace(s)
	s = strings.TrimPrefix(s, "$")
	v, _ := strconv.ParseFloat(s, 64)
	return v
}
