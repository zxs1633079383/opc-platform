package main

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"github.com/zlc-ai/opc-platform/pkg/model"
)

var modelsCmd = &cobra.Command{
	Use:   "models [agent-type]",
	Short: "List available models with pricing and capabilities",
	Long: `Display the model catalog with pricing, context window, and capability details.

Optional argument filters by agent type: claude-code, codex, openclaw.
Without an argument, shows all models.`,
	Args: cobra.MaximumNArgs(1),
	RunE: runModels,
}

func init() {
	rootCmd.AddCommand(modelsCmd)
}

func runModels(cmd *cobra.Command, args []string) error {
	var models []model.ModelEntry

	if len(args) > 0 {
		agentType := args[0]
		models = model.ModelsForAgent(agentType)
		if models == nil {
			return fmt.Errorf("unknown agent type %q (use: claude-code, codex, openclaw)", agentType)
		}
		fmt.Printf("Models for %s:\n", agentType)
	} else {
		fmt.Println("Claude (Anthropic) Models:")
		printModelsTable(model.ClaudeModels())
		fmt.Println("Codex (OpenAI) Models:")
		printModelsTable(model.CodexModels())
		return nil
	}

	printModelsTable(models)
	return nil
}

func printModelsTable(models []model.ModelEntry) {
	fmt.Printf("  %-24s %-10s %-8s %-8s %-12s %-12s %-10s %-9s %s\n",
		"Model", "Provider", "Context", "Output",
		"In/1M", "Out/1M", "CacheR/1M", "Reasoning", "Input")
	fmt.Printf("  %s\n", strings.Repeat("-", 115))

	for _, m := range models {
		reasoning := "-"
		if m.Reasoning {
			reasoning = "yes"
		}

		inputTypes := make([]string, len(m.Input))
		for j, t := range m.Input {
			inputTypes[j] = string(t)
		}

		cacheStr := "-"
		if m.Cost.CacheRead > 0 {
			cacheStr = fmt.Sprintf("$%.2f", m.Cost.CacheRead)
		}

		fmt.Printf("  %-24s %-10s %-8s %-8s $%-11.2f $%-11.2f %-10s %-9s %s\n",
			m.Name,
			m.Provider,
			m.FormatContextWindow(),
			m.FormatMaxOutput(),
			m.Cost.Input,
			m.Cost.Output,
			cacheStr,
			reasoning,
			strings.Join(inputTypes, ","),
		)
	}
	fmt.Println()
}
