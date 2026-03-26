package model

import "sync"

// ModelInfo describes a model available for agent configuration.
type ModelInfo struct {
	ID          string  `json:"id"`
	Provider    string  `json:"provider"`    // "anthropic" | "openai" | "custom"
	DisplayName string  `json:"displayName"`
	Tier        string  `json:"tier"`        // "economy" | "standard" | "premium"
	CostPer1K   float64 `json:"costPer1k"`
	Capability  string  `json:"capability"`  // "fast" | "balanced" | "reasoning"
	Default     bool    `json:"default,omitempty"`
}

// Registry holds available models.
type Registry struct {
	mu     sync.RWMutex
	models []ModelInfo
}

// NewRegistry initializes a Registry with built-in models.
func NewRegistry() *Registry {
	return &Registry{
		models: builtinModels(),
	}
}

// List returns all registered models.
func (r *Registry) List() []ModelInfo {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]ModelInfo, len(r.models))
	copy(out, r.models)
	return out
}

// Get looks up a model by ID. Returns the model and true if found.
func (r *Registry) Get(id string) (ModelInfo, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	for _, m := range r.models {
		if m.ID == id {
			return m, true
		}
	}
	return ModelInfo{}, false
}

// Add appends a model to the registry.
func (r *Registry) Add(m ModelInfo) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.models = append(r.models, m)
}

func builtinModels() []ModelInfo {
	return []ModelInfo{
		{ID: "claude-sonnet-4-6", Provider: "anthropic", DisplayName: "Claude Sonnet 4.6", Tier: "standard", CostPer1K: 0.003, Capability: "balanced", Default: true},
		{ID: "claude-opus-4-6", Provider: "anthropic", DisplayName: "Claude Opus 4.6", Tier: "premium", CostPer1K: 0.015, Capability: "reasoning"},
		{ID: "claude-haiku-4-5", Provider: "anthropic", DisplayName: "Claude Haiku 4.5", Tier: "economy", CostPer1K: 0.00025, Capability: "fast"},
		{ID: "gpt-4o", Provider: "openai", DisplayName: "GPT-4o", Tier: "standard", CostPer1K: 0.0025, Capability: "balanced"},
		{ID: "gpt-4o-mini", Provider: "openai", DisplayName: "GPT-4o Mini", Tier: "economy", CostPer1K: 0.00015, Capability: "fast"},
		{ID: "o3", Provider: "openai", DisplayName: "o3", Tier: "premium", CostPer1K: 0.01, Capability: "reasoning"},
	}
}
