package main

import (
	"encoding/json"
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/spf13/cobra"
)

var cronCmd = &cobra.Command{
	Use:   "cron",
	Short: "Manage scheduled workflows",
}

var cronListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all scheduled workflows",
	RunE:  runCronList,
}

var cronEnableCmd = &cobra.Command{
	Use:   "enable <workflow-name>",
	Short: "Enable a scheduled workflow",
	Args:  cobra.ExactArgs(1),
	RunE:  runCronEnable,
}

var cronDisableCmd = &cobra.Command{
	Use:   "disable <workflow-name>",
	Short: "Disable a scheduled workflow",
	Args:  cobra.ExactArgs(1),
	RunE:  runCronDisable,
}

func init() {
	cronCmd.AddCommand(cronListCmd)
	cronCmd.AddCommand(cronEnableCmd)
	cronCmd.AddCommand(cronDisableCmd)
	rootCmd.AddCommand(cronCmd)
}

func runCronList(cmd *cobra.Command, args []string) error {
	ctrl, cleanup, err := getController()
	if err != nil {
		return err
	}
	defer cleanup()

	workflows, err := ctrl.Store().ListWorkflows(cmd.Context())
	if err != nil {
		return err
	}

	// Filter to only scheduled workflows.
	var scheduled []struct {
		Name     string `json:"name"`
		Schedule string `json:"schedule"`
		Enabled  string `json:"enabled"`
	}
	for _, wf := range workflows {
		if wf.Schedule == "" {
			continue
		}
		enabled := "yes"
		if !wf.Enabled {
			enabled = "no"
		}
		scheduled = append(scheduled, struct {
			Name     string `json:"name"`
			Schedule string `json:"schedule"`
			Enabled  string `json:"enabled"`
		}{wf.Name, wf.Schedule, enabled})
	}

	if len(scheduled) == 0 {
		fmt.Println("No scheduled workflows found.")
		return nil
	}

	if output == "json" {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(scheduled)
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
	fmt.Fprintln(w, "NAME\tSCHEDULE\tENABLED")
	for _, s := range scheduled {
		fmt.Fprintf(w, "%s\t%s\t%s\n", s.Name, s.Schedule, s.Enabled)
	}
	return w.Flush()
}

func runCronEnable(cmd *cobra.Command, args []string) error {
	name := args[0]

	ctrl, cleanup, err := getController()
	if err != nil {
		return err
	}
	defer cleanup()

	wf, err := ctrl.Store().GetWorkflow(cmd.Context(), name)
	if err != nil {
		return fmt.Errorf("workflow %q not found: %w", name, err)
	}

	wf.Enabled = true
	if err := ctrl.Store().UpdateWorkflow(cmd.Context(), wf); err != nil {
		return err
	}

	fmt.Printf("workflow/%s enabled\n", name)
	return nil
}

func runCronDisable(cmd *cobra.Command, args []string) error {
	name := args[0]

	ctrl, cleanup, err := getController()
	if err != nil {
		return err
	}
	defer cleanup()

	wf, err := ctrl.Store().GetWorkflow(cmd.Context(), name)
	if err != nil {
		return fmt.Errorf("workflow %q not found: %w", name, err)
	}

	wf.Enabled = false
	if err := ctrl.Store().UpdateWorkflow(cmd.Context(), wf); err != nil {
		return err
	}

	fmt.Printf("workflow/%s disabled\n", name)
	return nil
}
