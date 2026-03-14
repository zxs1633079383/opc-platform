package main

import (
	"fmt"

	"github.com/spf13/cobra"
)

var restartCmd = &cobra.Command{
	Use:   "restart <resource> <name>",
	Short: "Restart a resource",
}

var restartAgentCmd = &cobra.Command{
	Use:   "agent <name>",
	Short: "Restart an agent",
	Args:  cobra.ExactArgs(1),
	RunE:  runRestartAgent,
}

func init() {
	restartCmd.AddCommand(restartAgentCmd)
	rootCmd.AddCommand(restartCmd)
}

func runRestartAgent(cmd *cobra.Command, args []string) error {
	name := args[0]

	if c := getDaemonClient(); c != nil {
		fmt.Printf("Restarting agent/%s...\n", name)
		if err := c.RestartAgent(cmd.Context(), name); err != nil {
			return err
		}
		fmt.Printf("agent/%s restarted\n", name)
		return nil
	}

	ctrl, cleanup, err := getController()
	if err != nil {
		return err
	}
	defer cleanup()

	fmt.Printf("Restarting agent/%s...\n", name)

	if err := ctrl.RestartAgent(cmd.Context(), name); err != nil {
		return err
	}

	fmt.Printf("agent/%s restarted\n", name)
	return nil
}
