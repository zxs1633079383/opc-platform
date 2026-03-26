package a2a

import (
	v1 "github.com/zlc-ai/opc-platform/api/v1"
	a2apb "github.com/zlc-ai/opc-platform/gen/a2a"
)

// AgentSpecToAgentCard converts an OPC AgentSpec to an A2A AgentCard.
func AgentSpecToAgentCard(spec v1.AgentSpec, serverURL string) *a2apb.AgentCard {
	skills := make([]*a2apb.AgentSkill, 0, len(spec.Spec.Context.Skills))
	for _, s := range spec.Spec.Context.Skills {
		skills = append(skills, &a2apb.AgentSkill{
			Id:   s,
			Name: s,
		})
	}

	meta := map[string]string{
		"agentType": string(spec.Spec.Type),
		"model":     spec.Spec.Runtime.Model.Name,
	}
	if spec.Spec.Runtime.Model.Fallback != "" {
		meta["fallbackModel"] = spec.Spec.Runtime.Model.Fallback
	}

	return &a2apb.AgentCard{
		Name:        spec.Metadata.Name,
		Description: spec.Spec.Description,
		Url:         serverURL,
		Version:     spec.APIVersion,
		Provider:    "opc-platform",
		Skills:      skills,
		InputModes:  []string{"text"},
		OutputModes: []string{"text"},
		Metadata:    meta,
	}
}
