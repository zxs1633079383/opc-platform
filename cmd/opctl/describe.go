package main

import (
	"fmt"
	"time"

	"github.com/spf13/cobra"
)

var describeCmd = &cobra.Command{
	Use:   "describe <resource> <name>",
	Short: "Show detailed information about a resource",
}

var describeAgentCmd = &cobra.Command{
	Use:   "agent <name>",
	Short: "Show detailed information about an agent",
	Args:  cobra.ExactArgs(1),
	RunE:  runDescribeAgent,
}

func init() {
	describeCmd.AddCommand(describeAgentCmd)
	rootCmd.AddCommand(describeCmd)
}

func runDescribeAgent(cmd *cobra.Command, args []string) error {
	name := args[0]

	ctrl, cleanup, err := getController()
	if err != nil {
		return err
	}
	defer cleanup()

	agent, err := ctrl.GetAgent(cmd.Context(), name)
	if err != nil {
		return fmt.Errorf("agent %q not found", name)
	}

	fmt.Printf("Name:         %s\n", agent.Name)
	fmt.Printf("Type:         %s\n", agent.Type)
	fmt.Printf("Status:       %s\n", agent.Phase)
	fmt.Printf("Restarts:     %d\n", agent.Restarts)
	if agent.Message != "" {
		fmt.Printf("Message:      %s\n", agent.Message)
	}
	fmt.Printf("Created:      %s\n", agent.CreatedAt.Format(time.RFC3339))
	fmt.Printf("Updated:      %s\n", agent.UpdatedAt.Format(time.RFC3339))

	// Show tasks for this agent.
	tasks, err := ctrl.Store().ListTasksByAgent(cmd.Context(), name)
	if err == nil && len(tasks) > 0 {
		fmt.Printf("\nRecent Tasks (%d total):\n", len(tasks))
		limit := 10
		if len(tasks) < limit {
			limit = len(tasks)
		}
		for _, t := range tasks[:limit] {
			fmt.Printf("  %s  %s  %s  %s\n",
				t.ID, t.Status, truncate(t.Message, 30), formatAge(time.Since(t.CreatedAt)))
		}
	}

	return nil
}
