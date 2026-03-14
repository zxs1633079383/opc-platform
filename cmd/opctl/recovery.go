package main

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"github.com/zlc-ai/opc-platform/pkg/controller"
)

var recoveryCmd = &cobra.Command{
	Use:   "recovery",
	Short: "Agent recovery operations",
}

var recoveryAgentCmd = &cobra.Command{
	Use:   "agent <name>",
	Short: "Recover an agent from checkpoint or memory",
	Args:  cobra.ExactArgs(1),
	RunE:  runRecoveryAgent,
}

var (
	recoveryFrom       string
	recoveryCheckpoint string
)

func init() {
	recoveryAgentCmd.Flags().StringVar(&recoveryFrom, "from", "checkpoint", "recovery source: checkpoint|memory|manual")
	recoveryAgentCmd.Flags().StringVar(&recoveryCheckpoint, "checkpoint", "", "specific checkpoint ID to recover from")

	recoveryCmd.AddCommand(recoveryAgentCmd)
	rootCmd.AddCommand(recoveryCmd)
}

func runRecoveryAgent(cmd *cobra.Command, args []string) error {
	name := args[0]

	ctrl, cleanup, err := getController()
	if err != nil {
		return err
	}
	defer cleanup()

	var result *controller.RecoveryResult

	if recoveryCheckpoint != "" {
		fmt.Printf("Recovering agent/%s from checkpoint %s...\n", name, recoveryCheckpoint)
		result, err = ctrl.RecoverFromCheckpoint(cmd.Context(), name, recoveryCheckpoint)
	} else {
		source := controller.RecoverySource(recoveryFrom)
		fmt.Printf("Recovering agent/%s from %s...\n", name, source)
		result, err = ctrl.RecoverAgent(cmd.Context(), name, source)
	}

	if err != nil {
		return fmt.Errorf("recovery failed: %w", err)
	}

	if result.Success {
		fmt.Printf("agent/%s recovered successfully\n", name)
		fmt.Printf("  Source: %s\n", result.Source)
		if result.CheckpointID != "" {
			fmt.Printf("  Checkpoint: %s\n", result.CheckpointID)
		}
		if len(result.PendingTasks) > 0 {
			fmt.Printf("  Pending tasks: %s\n", strings.Join(result.PendingTasks, ", "))
		}
	} else {
		fmt.Printf("agent/%s recovery failed: %s\n", name, result.Message)
	}

	return nil
}
