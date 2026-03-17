package cost

import (
	"bufio"
	"bytes"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"
)

const costEventsFile = "cost-events.jsonl"

// CostEvent records a cost event for tracking.
type CostEvent struct {
	ID            string            `json:"id"`
	Timestamp     time.Time         `json:"timestamp"`
	AgentName     string            `json:"agentName"`
	TaskID        string            `json:"taskId"`
	GoalRef       string            `json:"goalRef,omitempty"`
	ProjectRef    string            `json:"projectRef,omitempty"`
	TokensIn      int               `json:"tokensIn"`
	TokensOut     int               `json:"tokensOut"`
	TotalTokens   int               `json:"totalTokens"`
	InputCost     float64           `json:"inputCost"`
	OutputCost    float64           `json:"outputCost"`
	TotalCost     float64           `json:"totalCost"`
	Duration      time.Duration     `json:"duration"`
	ModelProvider string            `json:"modelProvider,omitempty"`
	ModelName     string            `json:"modelName,omitempty"`
	Labels        map[string]string `json:"labels,omitempty"`
}

// ModelPricing contains per-token pricing for a model.
type ModelPricing struct {
	Provider   string  `json:"provider"`
	Model      string  `json:"model"`
	InputPer1K float64 `json:"inputPer1k"`
	OutputPer1K float64 `json:"outputPer1k"`
}

// BudgetConfig defines budget limits.
type BudgetConfig struct {
	DailyLimit   float64 `json:"dailyLimit"`
	MonthlyLimit float64 `json:"monthlyLimit"`
	AlertPct     float64 `json:"alertPct"` // e.g., 0.8 for 80%
}

// BudgetStatus reports current spending vs limits.
type BudgetStatus struct {
	DailySpent   float64 `json:"dailySpent"`
	DailyLimit   float64 `json:"dailyLimit"`
	DailyPct     float64 `json:"dailyPct"`
	MonthlySpent float64 `json:"monthlySpent"`
	MonthlyLimit float64 `json:"monthlyLimit"`
	MonthlyPct   float64 `json:"monthlyPct"`
	Exceeded     bool    `json:"exceeded"`
}

// CostReport contains aggregated cost data.
type CostReport struct {
	TotalCost   float64            `json:"totalCost"`
	TotalTokens int                `json:"totalTokens"`
	EventCount  int                `json:"eventCount"`
	ByAgent     map[string]float64 `json:"byAgent,omitempty"`
	ByGoal      map[string]float64 `json:"byGoal,omitempty"`
	ByProject   map[string]float64 `json:"byProject,omitempty"`
	Period      string             `json:"period"`
}

// Tracker manages cost events and budgets.
type Tracker struct {
	mu      sync.RWMutex
	events  []CostEvent
	budget  BudgetConfig
	pricing map[string]ModelPricing // key: "provider/model"
	dir     string
	logger  *zap.SugaredLogger
}

// pricingKey returns the map key for a provider/model combination.
func pricingKey(provider, model string) string {
	return provider + "/" + model
}

// defaultPricing returns the built-in pricing table for known models.
// Prices are per 1K tokens in USD (as of 2025-Q2).
func defaultPricing() map[string]ModelPricing {
	models := []ModelPricing{
		// --- Anthropic Claude ---
		{Provider: "anthropic", Model: "claude-opus-4", InputPer1K: 0.015, OutputPer1K: 0.075},
		{Provider: "anthropic", Model: "claude-sonnet-4", InputPer1K: 0.003, OutputPer1K: 0.015},
		{Provider: "anthropic", Model: "claude-haiku-4", InputPer1K: 0.0008, OutputPer1K: 0.004},
		// Versioned variants
		{Provider: "anthropic", Model: "claude-opus-4-20250514", InputPer1K: 0.015, OutputPer1K: 0.075},
		{Provider: "anthropic", Model: "claude-sonnet-4-20250514", InputPer1K: 0.003, OutputPer1K: 0.015},
		{Provider: "anthropic", Model: "claude-haiku-4-5-20251001", InputPer1K: 0.0008, OutputPer1K: 0.004},
		// Legacy
		{Provider: "anthropic", Model: "claude-3-5-sonnet-20241022", InputPer1K: 0.003, OutputPer1K: 0.015},
		{Provider: "anthropic", Model: "claude-3-5-haiku-20241022", InputPer1K: 0.0008, OutputPer1K: 0.004},

		// --- OpenAI ---
		{Provider: "openai", Model: "gpt-4o", InputPer1K: 0.0025, OutputPer1K: 0.01},
		{Provider: "openai", Model: "gpt-4o-mini", InputPer1K: 0.00015, OutputPer1K: 0.0006},
		{Provider: "openai", Model: "gpt-4-turbo", InputPer1K: 0.01, OutputPer1K: 0.03},
		{Provider: "openai", Model: "o4-mini", InputPer1K: 0.0011, OutputPer1K: 0.0044},
		{Provider: "openai", Model: "o3", InputPer1K: 0.01, OutputPer1K: 0.04},
		{Provider: "openai", Model: "o3-mini", InputPer1K: 0.0011, OutputPer1K: 0.0044},
		{Provider: "openai", Model: "o1", InputPer1K: 0.015, OutputPer1K: 0.06},
		{Provider: "openai", Model: "o1-mini", InputPer1K: 0.003, OutputPer1K: 0.012},

		// --- OpenClaw (uses Anthropic models underneath) ---
		{Provider: "openclaw", Model: "claude-sonnet-4", InputPer1K: 0.003, OutputPer1K: 0.015},
		{Provider: "openclaw", Model: "claude-haiku-4", InputPer1K: 0.0008, OutputPer1K: 0.004},
		{Provider: "openclaw", Model: "claude-opus-4", InputPer1K: 0.015, OutputPer1K: 0.075},

		// --- Codex (uses OpenAI models) ---
		{Provider: "codex", Model: "o4-mini", InputPer1K: 0.0011, OutputPer1K: 0.0044},
		{Provider: "codex", Model: "codex", InputPer1K: 0.0011, OutputPer1K: 0.0044},

		// --- Google Gemini ---
		{Provider: "google", Model: "gemini-2.5-pro", InputPer1K: 0.00125, OutputPer1K: 0.01},
		{Provider: "google", Model: "gemini-2.5-flash", InputPer1K: 0.00015, OutputPer1K: 0.0006},
		{Provider: "google", Model: "gemini-2.0-flash", InputPer1K: 0.0001, OutputPer1K: 0.0004},

		// --- DeepSeek ---
		{Provider: "deepseek", Model: "deepseek-v3", InputPer1K: 0.00027, OutputPer1K: 0.0011},
		{Provider: "deepseek", Model: "deepseek-r1", InputPer1K: 0.00055, OutputPer1K: 0.0022},
	}

	m := make(map[string]ModelPricing, len(models))
	for _, p := range models {
		m[pricingKey(p.Provider, p.Model)] = p
	}
	return m
}

// NewTracker creates a new cost tracker that persists events to dir.
func NewTracker(dir string, logger *zap.SugaredLogger) *Tracker {
	t := &Tracker{
		events:  make([]CostEvent, 0),
		pricing: defaultPricing(),
		dir:     dir,
		logger:  logger,
	}

	if err := os.MkdirAll(dir, 0o755); err != nil {
		logger.Warnw("failed to create cost directory", "dir", dir, "error", err)
	}

	if err := t.loadFromDisk(); err != nil {
		logger.Warnw("failed to load cost events from disk", "error", err)
	}

	return t
}

// RecordCost records a cost event and persists it to disk.
func (t *Tracker) RecordCost(event CostEvent) error {
	if event.ID == "" {
		event.ID = uuid.New().String()
	}
	if event.Timestamp.IsZero() {
		event.Timestamp = time.Now()
	}
	event.TotalTokens = event.TokensIn + event.TokensOut
	event.TotalCost = event.InputCost + event.OutputCost

	if err := t.persistEvent(event); err != nil {
		return fmt.Errorf("persist cost event: %w", err)
	}

	t.mu.Lock()
	t.events = append(t.events, event)
	t.mu.Unlock()

	t.logger.Infow("cost event recorded",
		"id", event.ID,
		"agent", event.AgentName,
		"totalCost", event.TotalCost,
		"totalTokens", event.TotalTokens,
	)

	return nil
}

// CalculateCost computes input and output cost for the given token counts
// using the pricing table. Supports fuzzy matching: if "anthropic/claude-sonnet-4-20250514"
// is not found, tries prefix matching against known models (e.g., "claude-sonnet-4").
func (t *Tracker) CalculateCost(tokensIn, tokensOut int, provider, model string) (inputCost, outputCost float64) {
	t.mu.RLock()
	p, ok := t.pricing[pricingKey(provider, model)]
	if !ok {
		// Fuzzy match: find the longest model name that is a prefix of the given model.
		var bestMatch ModelPricing
		bestLen := 0
		for _, candidate := range t.pricing {
			if candidate.Provider == provider && len(candidate.Model) > bestLen {
				if strings.HasPrefix(model, candidate.Model) {
					bestMatch = candidate
					bestLen = len(candidate.Model)
					ok = true
				}
			}
		}
		if ok {
			p = bestMatch
		}
	}
	t.mu.RUnlock()

	if !ok {
		return 0, 0
	}

	inputCost = float64(tokensIn) / 1000.0 * p.InputPer1K
	outputCost = float64(tokensOut) / 1000.0 * p.OutputPer1K
	return inputCost, outputCost
}

// SetBudget configures budget limits.
func (t *Tracker) SetBudget(config BudgetConfig) {
	t.mu.Lock()
	t.budget = config
	t.mu.Unlock()
}

// GetBudgetStatus returns current spending compared against configured limits.
func (t *Tracker) GetBudgetStatus() BudgetStatus {
	t.mu.RLock()
	defer t.mu.RUnlock()

	now := time.Now()
	startOfDay := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	startOfMonth := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())

	var dailySpent, monthlySpent float64
	for _, e := range t.events {
		if !e.Timestamp.Before(startOfMonth) {
			monthlySpent += e.TotalCost
		}
		if !e.Timestamp.Before(startOfDay) {
			dailySpent += e.TotalCost
		}
	}

	status := BudgetStatus{
		DailySpent:   dailySpent,
		DailyLimit:   t.budget.DailyLimit,
		MonthlySpent: monthlySpent,
		MonthlyLimit: t.budget.MonthlyLimit,
	}

	if t.budget.DailyLimit > 0 {
		status.DailyPct = dailySpent / t.budget.DailyLimit
	}
	if t.budget.MonthlyLimit > 0 {
		status.MonthlyPct = monthlySpent / t.budget.MonthlyLimit
	}

	status.Exceeded = (t.budget.DailyLimit > 0 && dailySpent >= t.budget.DailyLimit) ||
		(t.budget.MonthlyLimit > 0 && monthlySpent >= t.budget.MonthlyLimit)

	return status
}

// CheckBudget checks whether any budget limit has been exceeded or is
// approaching the alert threshold. Returns true if exceeded, along with
// a human-readable message.
func (t *Tracker) CheckBudget() (exceeded bool, message string) {
	status := t.GetBudgetStatus()

	if status.Exceeded {
		if status.DailyLimit > 0 && status.DailySpent >= status.DailyLimit {
			return true, fmt.Sprintf(
				"daily budget exceeded: $%.4f spent of $%.4f limit (%.1f%%)",
				status.DailySpent, status.DailyLimit, status.DailyPct*100,
			)
		}
		return true, fmt.Sprintf(
			"monthly budget exceeded: $%.4f spent of $%.4f limit (%.1f%%)",
			status.MonthlySpent, status.MonthlyLimit, status.MonthlyPct*100,
		)
	}

	t.mu.RLock()
	alertPct := t.budget.AlertPct
	t.mu.RUnlock()

	if alertPct > 0 {
		if status.DailyPct >= alertPct {
			return false, fmt.Sprintf(
				"daily budget alert: $%.4f spent of $%.4f limit (%.1f%%)",
				status.DailySpent, status.DailyLimit, status.DailyPct*100,
			)
		}
		if status.MonthlyPct >= alertPct {
			return false, fmt.Sprintf(
				"monthly budget alert: $%.4f spent of $%.4f limit (%.1f%%)",
				status.MonthlySpent, status.MonthlyLimit, status.MonthlyPct*100,
			)
		}
	}

	return false, ""
}

// GenerateReport produces an aggregated cost report. Events are filtered to
// those within the given period from now. groupBy can be "agent", "goal",
// "project", or empty for totals only.
func (t *Tracker) GenerateReport(groupBy string, period time.Duration) CostReport {
	t.mu.RLock()
	defer t.mu.RUnlock()

	cutoff := time.Now().Add(-period)
	report := CostReport{
		ByAgent:  make(map[string]float64),
		ByGoal:   make(map[string]float64),
		ByProject: make(map[string]float64),
		Period:   period.String(),
	}

	for _, e := range t.events {
		if e.Timestamp.Before(cutoff) {
			continue
		}

		report.TotalCost += e.TotalCost
		report.TotalTokens += e.TotalTokens
		report.EventCount++

		if e.AgentName != "" {
			report.ByAgent[e.AgentName] += e.TotalCost
		}
		if e.GoalRef != "" {
			report.ByGoal[e.GoalRef] += e.TotalCost
		}
		if e.ProjectRef != "" {
			report.ByProject[e.ProjectRef] += e.TotalCost
		}
	}

	// Remove empty maps based on groupBy to keep output clean.
	switch groupBy {
	case "agent":
		report.ByGoal = nil
		report.ByProject = nil
	case "goal":
		report.ByAgent = nil
		report.ByProject = nil
	case "project":
		report.ByAgent = nil
		report.ByGoal = nil
	default:
		// Include all breakdowns.
	}

	return report
}

// ExportCSV exports all recorded cost events as CSV bytes.
func (t *Tracker) ExportCSV() ([]byte, error) {
	t.mu.RLock()
	events := make([]CostEvent, len(t.events))
	copy(events, t.events)
	t.mu.RUnlock()

	var buf bytes.Buffer
	w := csv.NewWriter(&buf)

	header := []string{
		"id", "timestamp", "agentName", "taskId", "goalRef", "projectRef",
		"tokensIn", "tokensOut", "totalTokens",
		"inputCost", "outputCost", "totalCost",
		"duration", "modelProvider", "modelName",
	}
	if err := w.Write(header); err != nil {
		return nil, fmt.Errorf("write csv header: %w", err)
	}

	for _, e := range events {
		row := []string{
			e.ID,
			e.Timestamp.Format(time.RFC3339),
			e.AgentName,
			e.TaskID,
			e.GoalRef,
			e.ProjectRef,
			fmt.Sprintf("%d", e.TokensIn),
			fmt.Sprintf("%d", e.TokensOut),
			fmt.Sprintf("%d", e.TotalTokens),
			fmt.Sprintf("%.6f", e.InputCost),
			fmt.Sprintf("%.6f", e.OutputCost),
			fmt.Sprintf("%.6f", e.TotalCost),
			e.Duration.String(),
			e.ModelProvider,
			e.ModelName,
		}
		if err := w.Write(row); err != nil {
			return nil, fmt.Errorf("write csv row: %w", err)
		}
	}

	w.Flush()
	if err := w.Error(); err != nil {
		return nil, fmt.Errorf("flush csv: %w", err)
	}

	return buf.Bytes(), nil
}

// DayCost returns total cost for events between start and end times.
func (t *Tracker) DayCost(start, end time.Time) float64 {
	t.mu.RLock()
	defer t.mu.RUnlock()

	var total float64
	for _, e := range t.events {
		if !e.Timestamp.Before(start) && e.Timestamp.Before(end) {
			total += e.TotalCost
		}
	}
	return total
}

// RecentEvents returns the most recent n cost events.
func (t *Tracker) RecentEvents(n int) []CostEvent {
	t.mu.RLock()
	defer t.mu.RUnlock()

	if len(t.events) <= n {
		result := make([]CostEvent, len(t.events))
		copy(result, t.events)
		return result
	}
	result := make([]CostEvent, n)
	copy(result, t.events[len(t.events)-n:])
	return result
}

// SetPricing adds or updates pricing for a model.
func (t *Tracker) SetPricing(pricing ModelPricing) {
	t.mu.Lock()
	t.pricing[pricingKey(pricing.Provider, pricing.Model)] = pricing
	t.mu.Unlock()
}

// loadFromDisk reads persisted JSONL events into memory.
func (t *Tracker) loadFromDisk() error {
	path := filepath.Join(t.dir, costEventsFile)
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("open cost events file: %w", err)
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	// Allow lines up to 1 MB.
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	var loaded int
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}
		var event CostEvent
		if err := json.Unmarshal(line, &event); err != nil {
			t.logger.Warnw("skipping malformed cost event line", "error", err)
			continue
		}
		t.events = append(t.events, event)
		loaded++
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("scan cost events file: %w", err)
	}

	t.logger.Infow("loaded cost events from disk", "count", loaded)
	return nil
}

// persistEvent appends a single event as a JSON line to the events file.
func (t *Tracker) persistEvent(event CostEvent) error {
	path := filepath.Join(t.dir, costEventsFile)
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("open cost events file for append: %w", err)
	}
	defer f.Close()

	data, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("marshal cost event: %w", err)
	}

	data = append(data, '\n')
	if _, err := f.Write(data); err != nil {
		return fmt.Errorf("write cost event: %w", err)
	}

	return nil
}
