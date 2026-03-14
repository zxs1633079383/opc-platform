package main

import (
	"encoding/json"
	"fmt"
	"text/tabwriter"

	"github.com/spf13/cobra"
)

var crashesCmd = &cobra.Command{
	Use:   "crashes",
	Short: "View agent crash history",
}

var crashesAgentCmd = &cobra.Command{
	Use:   "agent <name>",
	Short: "View crash history for an agent",
	Args:  cobra.ExactArgs(1),
	RunE:  runCrashesAgent,
}

func init() {
	crashesCmd.AddCommand(crashesAgentCmd)
	rootCmd.AddCommand(crashesCmd)
}

func runCrashesAgent(cmd *cobra.Command, args []string) error {
	name := args[0]

	ctrl, cleanup, err := getController()
	if err != nil {
		return err
	}
	defer cleanup()

	reports, err := ctrl.ListCrashReports(cmd.Context(), name)
	if err != nil {
		return err
	}

	if len(reports) == 0 {
		fmt.Printf("No crash reports found for agent/%s\n", name)
		return nil
	}

	if output == "json" {
		enc := json.NewEncoder(cmd.OutOrStdout())
		enc.SetIndent("", "  ")
		return enc.Encode(reports)
	}

	w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 4, 2, ' ', 0)
	fmt.Fprintln(w, "AGENT\tERROR\tTIMESTAMP")
	for _, r := range reports {
		errMsg := r.Error
		if len(errMsg) > 60 {
			errMsg = errMsg[:57] + "..."
		}
		fmt.Fprintf(w, "%s\t%s\t%s\n",
			r.AgentName, errMsg,
			r.Timestamp.Format("2006-01-02 15:04:05"),
		)
	}
	return w.Flush()
}
