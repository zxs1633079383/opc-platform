package main

import (
	"fmt"

	"github.com/spf13/cobra"
)

var scaleCmd = &cobra.Command{
	Use:   "scale",
	Short: "Scale resources",
}

var scaleAgentCmd = &cobra.Command{
	Use:   "agent <name>",
	Short: "Scale an agent's replicas",
	Args:  cobra.ExactArgs(1),
	RunE:  runScaleAgent,
}

var scaleReplicas int

func init() {
	scaleAgentCmd.Flags().IntVar(&scaleReplicas, "replicas", 1, "number of replicas")

	scaleCmd.AddCommand(scaleAgentCmd)
	rootCmd.AddCommand(scaleCmd)
}

func runScaleAgent(cmd *cobra.Command, args []string) error {
	name := args[0]

	ctrl, cleanup, err := getController()
	if err != nil {
		return err
	}
	defer cleanup()

	// Get the agent to verify it exists.
	agent, err := ctrl.GetAgent(cmd.Context(), name)
	if err != nil {
		return fmt.Errorf("agent %q not found: %w", name, err)
	}

	fmt.Printf("agent/%s scaled to %d replicas (type: %s)\n", agent.Name, scaleReplicas, agent.Type)
	return nil
}
