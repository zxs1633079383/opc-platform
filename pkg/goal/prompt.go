package goal

import (
	"fmt"
	"strings"

	v1 "github.com/zlc-ai/opc-platform/api/v1"
)

// BuildDecompositionPrompt constructs the prompt sent to the AI agent
// for automatic goal decomposition.
func BuildDecompositionPrompt(goalName, description string, constraints *v1.DecomposeConstraints) string {
	var sb strings.Builder

	sb.WriteString("You are an expert project manager and software architect. ")
	sb.WriteString("Your task is to decompose the following goal into a structured execution plan.\n\n")

	sb.WriteString(fmt.Sprintf("## Goal: %s\n\n", goalName))
	sb.WriteString(fmt.Sprintf("## Description:\n%s\n\n", description))

	if constraints != nil {
		sb.WriteString("## Constraints:\n")
		if constraints.MaxProjects > 0 {
			sb.WriteString(fmt.Sprintf("- Maximum number of projects: %d\n", constraints.MaxProjects))
		}
		if constraints.MaxTasksPerProject > 0 {
			sb.WriteString(fmt.Sprintf("- Maximum tasks per project: %d\n", constraints.MaxTasksPerProject))
		}
		if constraints.MaxAgents > 0 {
			sb.WriteString(fmt.Sprintf("- Maximum number of unique agents: %d\n", constraints.MaxAgents))
		}
		if constraints.MaxBudget != "" {
			sb.WriteString(fmt.Sprintf("- Maximum cost budget: %s\n", constraints.MaxBudget))
		}
		sb.WriteString("\n")
	}

	sb.WriteString("## Output Requirements:\n")
	sb.WriteString("Return ONLY a valid JSON object (no markdown, no explanation) with this exact structure:\n\n")
	sb.WriteString(`{
  "projects": [
    {
      "name": "string - concise project name",
      "description": "string - what this project delivers",
      "tasks": [
        {
          "name": "string - concise task name",
          "description": "string - detailed prompt for agent execution, include all context needed",
          "assignAgent": "string - suggested agent name (lowercase, kebab-case, e.g. backend-coder)",
          "complexity": "low|medium|high",
          "dependsOn": ["other-task-name"],
          "issues": [
            {
              "name": "string - specific action item",
              "description": "string - detailed description of what to do"
            }
          ]
        }
      ]
    }
  ]
}`)

	sb.WriteString("\n\n## Guidelines:\n")
	sb.WriteString("- Each project should represent a logical deliverable or module\n")
	sb.WriteString("- Tasks within a project should be ordered by dependency\n")
	sb.WriteString("- Use meaningful agent names that reflect their role (e.g. backend-coder, frontend-dev, qa-tester)\n")
	sb.WriteString("- Task descriptions must be self-contained prompts that an AI agent can execute independently\n")
	sb.WriteString("- Issues are the smallest executable units within a task\n")
	sb.WriteString("- Complexity should reflect estimated effort: low (<1h), medium (1-4h), high (4h+)\n")

	return sb.String()
}
