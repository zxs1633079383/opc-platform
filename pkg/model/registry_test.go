package model

import "testing"

func TestNewRegistry_HasBuiltins(t *testing.T) {
	r := NewRegistry()
	models := r.List()
	if len(models) != 6 {
		t.Fatalf("expected 6 built-in models, got %d", len(models))
	}

	// Verify default model exists.
	foundDefault := false
	for _, m := range models {
		if m.Default {
			if m.ID != "claude-sonnet-4-6" {
				t.Errorf("expected default model claude-sonnet-4-6, got %s", m.ID)
			}
			foundDefault = true
		}
	}
	if !foundDefault {
		t.Error("no default model found in built-in models")
	}

	// Verify providers.
	providers := map[string]int{}
	for _, m := range models {
		providers[m.Provider]++
	}
	if providers["anthropic"] != 3 {
		t.Errorf("expected 3 anthropic models, got %d", providers["anthropic"])
	}
	if providers["openai"] != 3 {
		t.Errorf("expected 3 openai models, got %d", providers["openai"])
	}
}

func TestRegistryAdd(t *testing.T) {
	r := NewRegistry()
	initial := len(r.List())

	custom := ModelInfo{
		ID:          "custom-model",
		Provider:    "custom",
		DisplayName: "My Custom Model",
		Tier:        "standard",
		CostPer1K:   0.005,
		Capability:  "balanced",
	}
	r.Add(custom)

	models := r.List()
	if len(models) != initial+1 {
		t.Fatalf("expected %d models after add, got %d", initial+1, len(models))
	}

	// Verify the added model is retrievable.
	got, ok := r.Get("custom-model")
	if !ok {
		t.Fatal("custom-model not found after add")
	}
	if got.DisplayName != "My Custom Model" {
		t.Errorf("expected display name 'My Custom Model', got %q", got.DisplayName)
	}
}

func TestRegistryGet(t *testing.T) {
	r := NewRegistry()

	// Existing model.
	m, ok := r.Get("claude-opus-4-6")
	if !ok {
		t.Fatal("claude-opus-4-6 should exist")
	}
	if m.Tier != "premium" {
		t.Errorf("expected tier 'premium', got %q", m.Tier)
	}
	if m.Capability != "reasoning" {
		t.Errorf("expected capability 'reasoning', got %q", m.Capability)
	}

	// Non-existing model.
	_, ok = r.Get("nonexistent")
	if ok {
		t.Error("expected nonexistent model to not be found")
	}
}

func TestRegistryList_ReturnsCopy(t *testing.T) {
	r := NewRegistry()
	list1 := r.List()
	list1[0].ID = "mutated"

	// Original should be unchanged.
	list2 := r.List()
	if list2[0].ID == "mutated" {
		t.Error("List() should return a copy, not a reference to internal slice")
	}
}
