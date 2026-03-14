package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"
	v1 "github.com/zlc-ai/opc-platform/api/v1"
)

var getCmd = &cobra.Command{
	Use:   "get <resource>",
	Short: "List resources",
	Long:  `List agents, tasks, or other OPC resources.`,
}

var getAgentsCmd = &cobra.Command{
	Use:     "agents",
	Aliases: []string{"agent"},
	Short:   "List all agents",
	RunE:    runGetAgents,
}

var getTasksCmd = &cobra.Command{
	Use:     "tasks",
	Aliases: []string{"task"},
	Short:   "List all tasks",
	RunE:    runGetTasks,
}

var getWorkflowsCmd = &cobra.Command{
	Use:     "workflows",
	Aliases: []string{"workflow", "wf"},
	Short:   "List all workflows",
	RunE:    runGetWorkflows,
}

func init() {
	getCmd.AddCommand(getAgentsCmd)
	getCmd.AddCommand(getTasksCmd)
	getCmd.AddCommand(getWorkflowsCmd)
	rootCmd.AddCommand(getCmd)
}

func runGetAgents(cmd *cobra.Command, args []string) error {
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

	format, _ := cmd.Flags().GetString("output")
	switch format {
	case "json":
		return printJSON(agents)
	default:
		return printAgentTable(agents)
	}
}

func runGetTasks(cmd *cobra.Command, args []string) error {
	ctrl, cleanup, err := getController()
	if err != nil {
		return err
	}
	defer cleanup()

	tasks, err := ctrl.Store().ListTasks(cmd.Context())
	if err != nil {
		return err
	}

	if len(tasks) == 0 {
		fmt.Println("No tasks found.")
		return nil
	}

	format, _ := cmd.Flags().GetString("output")
	switch format {
	case "json":
		return printJSON(tasks)
	default:
		return printTaskTable(tasks)
	}
}

func printAgentTable(agents []v1.AgentRecord) error {
	w := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
	fmt.Fprintln(w, "NAME\tTYPE\tSTATUS\tRESTARTS\tAGE")
	for _, a := range agents {
		age := formatAge(time.Since(a.CreatedAt))
		fmt.Fprintf(w, "%s\t%s\t%s\t%d\t%s\n",
			a.Name, a.Type, a.Phase, a.Restarts, age)
	}
	return w.Flush()
}

func printTaskTable(tasks []v1.TaskRecord) error {
	w := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
	fmt.Fprintln(w, "ID\tAGENT\tSTATUS\tMESSAGE\tAGE")
	for _, t := range tasks {
		age := formatAge(time.Since(t.CreatedAt))
		msg := truncate(t.Message, 40)
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n",
			t.ID, t.AgentName, t.Status, msg, age)
	}
	return w.Flush()
}

func printJSON(v any) error {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}

func formatAge(d time.Duration) string {
	switch {
	case d < time.Minute:
		return fmt.Sprintf("%ds", int(d.Seconds()))
	case d < time.Hour:
		return fmt.Sprintf("%dm", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%dh", int(d.Hours()))
	default:
		return fmt.Sprintf("%dd", int(d.Hours()/24))
	}
}

func runGetWorkflows(cmd *cobra.Command, args []string) error {
	ctrl, cleanup, err := getController()
	if err != nil {
		return err
	}
	defer cleanup()

	workflows, err := ctrl.Store().ListWorkflows(cmd.Context())
	if err != nil {
		return err
	}

	if len(workflows) == 0 {
		fmt.Println("No workflows found.")
		return nil
	}

	format, _ := cmd.Flags().GetString("output")
	switch format {
	case "json":
		return printJSON(workflows)
	default:
		w := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
		fmt.Fprintln(w, "NAME\tSCHEDULE\tENABLED\tAGE")
		for _, wf := range workflows {
			schedule := wf.Schedule
			if schedule == "" {
				schedule = "-"
			}
			enabled := "yes"
			if !wf.Enabled {
				enabled = "no"
			}
			age := formatAge(time.Since(wf.CreatedAt))
			fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", wf.Name, schedule, enabled, age)
		}
		return w.Flush()
	}
}

func truncate(s string, n int) string {
	s = strings.ReplaceAll(s, "\n", " ")
	if len(s) > n {
		return s[:n-3] + "..."
	}
	return s
}
