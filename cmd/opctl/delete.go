package main

import (
	"fmt"

	"github.com/spf13/cobra"
)

var deleteCmd = &cobra.Command{
	Use:   "delete <resource> <name>",
	Short: "Delete a resource",
}

var deleteAgentCmd = &cobra.Command{
	Use:   "agent <name>",
	Short: "Delete an agent",
	Args:  cobra.ExactArgs(1),
	RunE:  runDeleteAgent,
}

func init() {
	deleteCmd.AddCommand(deleteAgentCmd)
	rootCmd.AddCommand(deleteCmd)
}

func runDeleteAgent(cmd *cobra.Command, args []string) error {
	name := args[0]

	if c := getDaemonClient(); c != nil {
		if err := c.DeleteAgent(cmd.Context(), name); err != nil {
			return err
		}
		fmt.Printf("agent/%s deleted\n", name)
		return nil
	}

	ctrl, cleanup, err := getController()
	if err != nil {
		return err
	}
	defer cleanup()

	if err := ctrl.DeleteAgent(cmd.Context(), name); err != nil {
		return err
	}

	fmt.Printf("agent/%s deleted\n", name)
	return nil
}
