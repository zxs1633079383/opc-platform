package main

import (
	"encoding/json"
	"fmt"
	"text/tabwriter"

	"github.com/spf13/cobra"
)

var checkpointsCmd = &cobra.Command{
	Use:   "checkpoints",
	Short: "Manage agent checkpoints",
}

var checkpointsListCmd = &cobra.Command{
	Use:   "list agent <name>",
	Short: "List checkpoints for an agent",
	Args:  cobra.ExactArgs(2),
	RunE:  runCheckpointsList,
}

func init() {
	checkpointsCmd.AddCommand(checkpointsListCmd)
	rootCmd.AddCommand(checkpointsCmd)
}

func runCheckpointsList(cmd *cobra.Command, args []string) error {
	if args[0] != "agent" {
		return fmt.Errorf("expected 'agent', got %q", args[0])
	}
	name := args[1]

	ctrl, cleanup, err := getController()
	if err != nil {
		return err
	}
	defer cleanup()

	checkpoints, err := ctrl.ListCheckpoints(cmd.Context(), name)
	if err != nil {
		return err
	}

	if len(checkpoints) == 0 {
		fmt.Printf("No checkpoints found for agent/%s\n", name)
		return nil
	}

	if output == "json" {
		enc := json.NewEncoder(cmd.OutOrStdout())
		enc.SetIndent("", "  ")
		return enc.Encode(checkpoints)
	}

	w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 4, 2, ' ', 0)
	fmt.Fprintln(w, "ID\tAGENT\tPHASE\tPENDING TASKS\tTIMESTAMP")
	for _, cp := range checkpoints {
		fmt.Fprintf(w, "%s\t%s\t%s\t%d\t%s\n",
			cp.ID, cp.AgentName, cp.Phase,
			len(cp.PendingTasks),
			cp.Timestamp.Format("2006-01-02 15:04:05"),
		)
	}
	return w.Flush()
}
