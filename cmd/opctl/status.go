package main

import (
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/spf13/cobra"
)

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show cluster status overview",
	RunE:  runStatus,
}

func init() {
	rootCmd.AddCommand(statusCmd)
}

func runStatus(cmd *cobra.Command, args []string) error {
	ctrl, cleanup, err := getController()
	if err != nil {
		return err
	}
	defer cleanup()

	agents, err := ctrl.ListAgents(cmd.Context())
	if err != nil {
		return err
	}

	tasks, err := ctrl.Store().ListTasks(cmd.Context())
	if err != nil {
		return err
	}

	// Summary.
	var running, stopped, failed int
	for _, a := range agents {
		switch a.Phase {
		case "Running":
			running++
		case "Stopped", "Completed", "Terminated":
			stopped++
		case "Failed":
			failed++
		}
	}

	var pending, taskRunning, completed, taskFailed int
	for _, t := range tasks {
		switch t.Status {
		case "Pending":
			pending++
		case "Running":
			taskRunning++
		case "Completed":
			completed++
		case "Failed":
			taskFailed++
		}
	}

	fmt.Println("OPC Platform Status")
	fmt.Println("====================")
	fmt.Printf("\nAgents: %d total (%d running, %d stopped, %d failed)\n",
		len(agents), running, stopped, failed)
	fmt.Printf("Tasks:  %d total (%d pending, %d running, %d completed, %d failed)\n",
		len(tasks), pending, taskRunning, completed, taskFailed)

	if len(agents) > 0 {
		fmt.Println("\nAgents:")
		w := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
		fmt.Fprintln(w, "  NAME\tTYPE\tSTATUS\tRESTARTS")
		for _, a := range agents {
			fmt.Fprintf(w, "  %s\t%s\t%s\t%d\n", a.Name, a.Type, a.Phase, a.Restarts)
		}
		w.Flush()
	}

	return nil
}
