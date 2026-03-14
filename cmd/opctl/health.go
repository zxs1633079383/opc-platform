package main

import (
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/spf13/cobra"
)

var healthCmd = &cobra.Command{
	Use:   "health",
	Short: "Check health of all agents",
	RunE:  runHealth,
}

func init() {
	rootCmd.AddCommand(healthCmd)
}

func runHealth(cmd *cobra.Command, args []string) error {
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

	healthMap := ctrl.Health()

	w := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
	fmt.Fprintln(w, "NAME\tTYPE\tHEALTHY\tMESSAGE")
	for _, a := range agents {
		h, ok := healthMap[a.Name]
		if !ok {
			fmt.Fprintf(w, "%s\t%s\t-\tnot running\n", a.Name, a.Type)
			continue
		}
		healthy := "Yes"
		if !h.Healthy {
			healthy = "No"
		}
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", a.Name, a.Type, healthy, h.Message)
	}
	return w.Flush()
}
