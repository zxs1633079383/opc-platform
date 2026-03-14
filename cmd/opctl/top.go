package main

import (
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/spf13/cobra"
)

var topCmd = &cobra.Command{
	Use:   "top <resource>",
	Short: "Display resource usage",
}

var topAgentsCmd = &cobra.Command{
	Use:   "agents",
	Short: "Display agent resource usage",
	RunE:  runTopAgents,
}

func init() {
	topCmd.AddCommand(topAgentsCmd)
	rootCmd.AddCommand(topCmd)
}

func runTopAgents(cmd *cobra.Command, args []string) error {
	ctrl, cleanup, err := getController()
	if err != nil {
		return err
	}
	defer cleanup()

	agents, err := ctrl.ListAgents(cmd.Context())
	if err != nil {
		return err
	}

	if len(agents) == 0 {
		fmt.Println("No agents found.")
		return nil
	}

	metrics := ctrl.AgentMetrics()

	w := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
	fmt.Fprintln(w, "NAME\tSTATUS\tTASKS(C/F/R)\tTOKENS(IN/OUT)\tCOST\tUPTIME")
	for _, a := range agents {
		m, ok := metrics[a.Name]
		if !ok {
			fmt.Fprintf(w, "%s\t%s\t-\t-\t-\t-\n", a.Name, a.Phase)
			continue
		}
		fmt.Fprintf(w, "%s\t%s\t%d/%d/%d\t%d/%d\t$%.2f\t%.0fs\n",
			a.Name, a.Phase,
			m.TasksCompleted, m.TasksFailed, m.TasksRunning,
			m.TotalTokensIn, m.TotalTokensOut,
			m.TotalCost, m.UptimeSeconds)
	}
	return w.Flush()
}
