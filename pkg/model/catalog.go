package model

import "fmt"

// InputType represents the types of input a model can accept.
type InputType string

const (
	InputText  InputType = "text"
	InputImage InputType = "image"
)

// ModelCost contains per-token pricing in USD per 1M tokens.
type ModelCost struct {
	Input      float64 `json:"input"`      // USD per 1M input tokens
	Output     float64 `json:"output"`     // USD per 1M output tokens
	CacheRead  float64 `json:"cacheRead"`  // USD per 1M cached-read tokens
	CacheWrite float64 `json:"cacheWrite"` // USD per 1M cache-write tokens
}

// ModelEntry defines a single model in the catalog.
type ModelEntry struct {
	ID            string      `json:"id"`            // Full model identifier (e.g. "claude-sonnet-4-6")
	Name          string      `json:"name"`          // Human-readable display name
	Provider      string      `json:"provider"`      // Provider key: "anthropic", "openai"
	Reasoning     bool        `json:"reasoning"`     // Supports extended thinking / chain-of-thought
	Input         []InputType `json:"input"`         // Supported input modalities
	Cost          ModelCost   `json:"cost"`          // Token pricing
	ContextWindow int         `json:"contextWindow"` // Max context in tokens
	MaxOutput     int         `json:"maxOutput"`     // Max output tokens
	CLIModelID    string      `json:"cliModelId"`    // Exact ID for CLI --model flag
}

// FormatCostPer1K returns the per-1K token cost string for display.
func (m *ModelEntry) FormatCostPer1K() string {
	return fmt.Sprintf("$%.4f / $%.4f per 1K", m.Cost.Input/1000, m.Cost.Output/1000)
}

// FormatContextWindow returns a human-readable context window string.
func (m *ModelEntry) FormatContextWindow() string {
	if m.ContextWindow >= 1_000_000 {
		return fmt.Sprintf("%.1fM", float64(m.ContextWindow)/1_000_000)
	}
	return fmt.Sprintf("%dK", m.ContextWindow/1000)
}

// FormatMaxOutput returns a human-readable max output string.
func (m *ModelEntry) FormatMaxOutput() string {
	if m.MaxOutput >= 1_000_000 {
		return fmt.Sprintf("%.1fM", float64(m.MaxOutput)/1_000_000)
	}
	return fmt.Sprintf("%dK", m.MaxOutput/1000)
}

// -------------------------------------------------------------------
// Built-in model catalogs — referenced from OpenClaw model definitions
// Pricing as of 2025-Q2 (USD per 1M tokens)
// -------------------------------------------------------------------

// ClaudeModels returns the Anthropic Claude model catalog.
func ClaudeModels() []ModelEntry {
	return []ModelEntry{
		{
			ID:            "claude-opus-4-6",
			Name:          "Claude Opus 4.6",
			Provider:      "anthropic",
			Reasoning:     true,
			Input:         []InputType{InputText, InputImage},
			Cost:          ModelCost{Input: 15.0, Output: 75.0, CacheRead: 1.5, CacheWrite: 18.75},
			ContextWindow: 200_000,
			MaxOutput:     32_000,
			CLIModelID:    "claude-opus-4-6",
		},
		{
			ID:            "claude-sonnet-4-6",
			Name:          "Claude Sonnet 4.6",
			Provider:      "anthropic",
			Reasoning:     true,
			Input:         []InputType{InputText, InputImage},
			Cost:          ModelCost{Input: 3.0, Output: 15.0, CacheRead: 0.3, CacheWrite: 3.75},
			ContextWindow: 200_000,
			MaxOutput:     16_000,
			CLIModelID:    "claude-sonnet-4-6",
		},
		{
			ID:            "claude-haiku-4-5",
			Name:          "Claude Haiku 4.5",
			Provider:      "anthropic",
			Reasoning:     false,
			Input:         []InputType{InputText, InputImage},
			Cost:          ModelCost{Input: 0.8, Output: 4.0, CacheRead: 0.08, CacheWrite: 1.0},
			ContextWindow: 200_000,
			MaxOutput:     8_192,
			CLIModelID:    "claude-haiku-4-5-20251001",
		},
		{
			ID:            "claude-sonnet-4-5",
			Name:          "Claude Sonnet 4.5",
			Provider:      "anthropic",
			Reasoning:     true,
			Input:         []InputType{InputText, InputImage},
			Cost:          ModelCost{Input: 3.0, Output: 15.0, CacheRead: 0.3, CacheWrite: 3.75},
			ContextWindow: 200_000,
			MaxOutput:     16_000,
			CLIModelID:    "claude-sonnet-4-5-20250514",
		},
		{
			ID:            "claude-opus-4-5",
			Name:          "Claude Opus 4.5",
			Provider:      "anthropic",
			Reasoning:     true,
			Input:         []InputType{InputText, InputImage},
			Cost:          ModelCost{Input: 15.0, Output: 75.0, CacheRead: 1.5, CacheWrite: 18.75},
			ContextWindow: 200_000,
			MaxOutput:     32_000,
			CLIModelID:    "claude-opus-4-5-20250514",
		},
	}
}

// CodexModels returns the OpenAI Codex model catalog.
func CodexModels() []ModelEntry {
	return []ModelEntry{
		{
			ID:            "o4-mini",
			Name:          "O4 Mini",
			Provider:      "openai",
			Reasoning:     true,
			Input:         []InputType{InputText, InputImage},
			Cost:          ModelCost{Input: 1.1, Output: 4.4, CacheRead: 0.275, CacheWrite: 0},
			ContextWindow: 200_000,
			MaxOutput:     100_000,
			CLIModelID:    "o4-mini",
		},
		{
			ID:            "o3",
			Name:          "O3",
			Provider:      "openai",
			Reasoning:     true,
			Input:         []InputType{InputText, InputImage},
			Cost:          ModelCost{Input: 10.0, Output: 40.0, CacheRead: 2.5, CacheWrite: 0},
			ContextWindow: 200_000,
			MaxOutput:     100_000,
			CLIModelID:    "o3",
		},
		{
			ID:            "gpt-4o",
			Name:          "GPT-4o",
			Provider:      "openai",
			Reasoning:     false,
			Input:         []InputType{InputText, InputImage},
			Cost:          ModelCost{Input: 2.5, Output: 10.0, CacheRead: 1.25, CacheWrite: 0},
			ContextWindow: 128_000,
			MaxOutput:     16_384,
			CLIModelID:    "gpt-4o",
		},
		{
			ID:            "gpt-4o-mini",
			Name:          "GPT-4o Mini",
			Provider:      "openai",
			Reasoning:     false,
			Input:         []InputType{InputText, InputImage},
			Cost:          ModelCost{Input: 0.15, Output: 0.6, CacheRead: 0.075, CacheWrite: 0},
			ContextWindow: 128_000,
			MaxOutput:     16_384,
			CLIModelID:    "gpt-4o-mini",
		},
		{
			ID:            "o3-mini",
			Name:          "O3 Mini",
			Provider:      "openai",
			Reasoning:     true,
			Input:         []InputType{InputText},
			Cost:          ModelCost{Input: 1.1, Output: 4.4, CacheRead: 0.55, CacheWrite: 0},
			ContextWindow: 200_000,
			MaxOutput:     100_000,
			CLIModelID:    "o3-mini",
		},
	}
}

// AllModels returns the combined catalog for all providers.
func AllModels() []ModelEntry {
	var all []ModelEntry
	all = append(all, ClaudeModels()...)
	all = append(all, CodexModels()...)
	return all
}

// ModelsForAgent returns models appropriate for the given agent type.
func ModelsForAgent(agentType string) []ModelEntry {
	switch agentType {
	case "claude-code":
		return ClaudeModels()
	case "codex":
		return CodexModels()
	case "openclaw":
		// OpenClaw supports all models via gateway routing.
		return AllModels()
	default:
		return nil
	}
}

// FindModel looks up a model by ID across all catalogs.
func FindModel(id string) *ModelEntry {
	for _, m := range AllModels() {
		if m.ID == id || m.CLIModelID == id {
			return &m
		}
	}
	return nil
}
