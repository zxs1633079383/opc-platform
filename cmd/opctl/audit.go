package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"text/tabwriter"

	"github.com/spf13/cobra"
	"github.com/zlc-ai/opc-platform/internal/config"
	"github.com/zlc-ai/opc-platform/pkg/audit"
)

var auditCmd = &cobra.Command{
	Use:   "audit <resource> [name]",
	Short: "View audit trail for resources",
}

var auditGoalCmd = &cobra.Command{
	Use:   "goal <name>",
	Short: "View audit trail for a goal",
	Args:  cobra.ExactArgs(1),
	RunE:  runAuditGoal,
}

var auditTraceCmd = &cobra.Command{
	Use:   "trace <resourceType> <name>",
	Short: "Trace full audit chain for a resource",
	Args:  cobra.ExactArgs(2),
	RunE:  runAuditTrace,
}

var auditExportCmd = &cobra.Command{
	Use:   "export",
	Short: "Export audit logs",
	RunE:  runAuditExport,
}

var auditExportFormat string

func init() {
	auditExportCmd.Flags().StringVar(&auditExportFormat, "format", "json", "export format: json")

	auditCmd.AddCommand(auditGoalCmd)
	auditCmd.AddCommand(auditTraceCmd)
	auditCmd.AddCommand(auditExportCmd)
	rootCmd.AddCommand(auditCmd)
}

func getAuditLogger() (*audit.Logger, error) {
	config.InitLogger(false, "")
	dir := filepath.Join(config.GetConfigDir(), "audit")
	return audit.NewLogger(dir, config.Logger), nil
}

func runAuditGoal(cmd *cobra.Command, args []string) error {
	name := args[0]

	logger, err := getAuditLogger()
	if err != nil {
		return err
	}

	events, err := logger.ListByGoal(name)
	if err != nil {
		return err
	}

	if len(events) == 0 {
		fmt.Printf("No audit events found for goal/%s\n", name)
		return nil
	}

	return printAuditEvents(events)
}

func runAuditTrace(cmd *cobra.Command, args []string) error {
	resourceType := audit.ResourceType(args[0])
	name := args[1]

	logger, err := getAuditLogger()
	if err != nil {
		return err
	}

	events, err := logger.Trace(resourceType, name)
	if err != nil {
		return err
	}

	if len(events) == 0 {
		fmt.Printf("No audit events found for %s/%s\n", resourceType, name)
		return nil
	}

	return printAuditEvents(events)
}

func runAuditExport(cmd *cobra.Command, args []string) error {
	logger, err := getAuditLogger()
	if err != nil {
		return err
	}

	data, err := logger.Export(auditExportFormat)
	if err != nil {
		return err
	}

	fmt.Print(string(data))
	return nil
}

func printAuditEvents(events []audit.AuditEvent) error {
	if output == "json" {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(events)
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
	fmt.Fprintln(w, "TIMESTAMP\tEVENT\tRESOURCE\tNAME\tDETAILS")
	for _, e := range events {
		details := e.Details
		if len(details) > 50 {
			details = details[:47] + "..."
		}
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n",
			e.Timestamp.Format("2006-01-02 15:04:05"),
			e.EventType, e.ResourceType, e.ResourceName, details,
		)
	}
	return w.Flush()
}
