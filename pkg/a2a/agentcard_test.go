package a2a

import (
	"testing"

	v1 "github.com/zlc-ai/opc-platform/api/v1"
)

func TestAgentSpecToAgentCard(t *testing.T) {
	spec := v1.AgentSpec{
		APIVersion: "opc/v1",
		Kind:       "AgentSpec",
		Metadata: v1.Metadata{
			Name: "code-agent",
		},
		Spec: v1.AgentSpecBody{
			Type:        v1.AgentTypeClaudeCode,
			Description: "A coding assistant agent",
			Runtime: v1.RuntimeConfig{
				Model: v1.ModelConfig{
					Provider: "anthropic",
					Name:     "claude-sonnet-4-20250514",
					Fallback: "claude-haiku-4-20250514",
				},
			},
			Context: v1.ContextConfig{
				Skills: []string{"golang", "testing", "refactoring"},
			},
		},
	}

	card := AgentSpecToAgentCard(spec, "http://localhost:8080")

	if card.Name != "code-agent" {
		t.Errorf("Name = %q, want %q", card.Name, "code-agent")
	}
	if card.Description != "A coding assistant agent" {
		t.Errorf("Description = %q, want %q", card.Description, "A coding assistant agent")
	}
	if card.Url != "http://localhost:8080" {
		t.Errorf("Url = %q, want %q", card.Url, "http://localhost:8080")
	}
	if card.Version != "opc/v1" {
		t.Errorf("Version = %q, want %q", card.Version, "opc/v1")
	}
	if card.Provider != "opc-platform" {
		t.Errorf("Provider = %q, want %q", card.Provider, "opc-platform")
	}

	// Skills
	if len(card.Skills) != 3 {
		t.Fatalf("Skills length = %d, want 3", len(card.Skills))
	}
	for i, name := range []string{"golang", "testing", "refactoring"} {
		if card.Skills[i].Id != name {
			t.Errorf("Skills[%d].Id = %q, want %q", i, card.Skills[i].Id, name)
		}
		if card.Skills[i].Name != name {
			t.Errorf("Skills[%d].Name = %q, want %q", i, card.Skills[i].Name, name)
		}
	}

	// Input/Output modes
	if len(card.InputModes) != 1 || card.InputModes[0] != "text" {
		t.Errorf("InputModes = %v, want [text]", card.InputModes)
	}
	if len(card.OutputModes) != 1 || card.OutputModes[0] != "text" {
		t.Errorf("OutputModes = %v, want [text]", card.OutputModes)
	}

	// Metadata
	if card.Metadata["agentType"] != "claude-code" {
		t.Errorf("Metadata[agentType] = %q, want %q", card.Metadata["agentType"], "claude-code")
	}
	if card.Metadata["model"] != "claude-sonnet-4-20250514" {
		t.Errorf("Metadata[model] = %q, want %q", card.Metadata["model"], "claude-sonnet-4-20250514")
	}
	if card.Metadata["fallbackModel"] != "claude-haiku-4-20250514" {
		t.Errorf("Metadata[fallbackModel] = %q, want %q", card.Metadata["fallbackModel"], "claude-haiku-4-20250514")
	}
}

func TestAgentSpecToAgentCard_NoSkills(t *testing.T) {
	spec := v1.AgentSpec{
		APIVersion: "opc/v1",
		Kind:       "AgentSpec",
		Metadata: v1.Metadata{
			Name: "minimal-agent",
		},
		Spec: v1.AgentSpecBody{
			Type:        v1.AgentTypeOpenClaw,
			Description: "Minimal agent",
			Runtime: v1.RuntimeConfig{
				Model: v1.ModelConfig{
					Name: "gpt-4",
				},
			},
		},
	}

	card := AgentSpecToAgentCard(spec, "http://example.com")

	if card.Name != "minimal-agent" {
		t.Errorf("Name = %q, want %q", card.Name, "minimal-agent")
	}
	if len(card.Skills) != 0 {
		t.Errorf("Skills length = %d, want 0", len(card.Skills))
	}
	if _, ok := card.Metadata["fallbackModel"]; ok {
		t.Error("Metadata should not contain fallbackModel when no fallback is set")
	}
	if card.Metadata["model"] != "gpt-4" {
		t.Errorf("Metadata[model] = %q, want %q", card.Metadata["model"], "gpt-4")
	}
}

func TestAgentSpecToAgentCard_EmptyDescription(t *testing.T) {
	spec := v1.AgentSpec{
		APIVersion: "opc/v1",
		Kind:       "AgentSpec",
		Metadata: v1.Metadata{
			Name: "no-desc-agent",
		},
		Spec: v1.AgentSpecBody{
			Type: v1.AgentTypeCodex,
		},
	}

	card := AgentSpecToAgentCard(spec, "http://localhost:9090")

	if card.Description != "" {
		t.Errorf("Description = %q, want empty", card.Description)
	}
	if card.Name != "no-desc-agent" {
		t.Errorf("Name = %q, want %q", card.Name, "no-desc-agent")
	}
	if card.Url != "http://localhost:9090" {
		t.Errorf("Url = %q, want %q", card.Url, "http://localhost:9090")
	}
}
