package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"
	"github.com/zlc-ai/opc-platform/internal/config"
	"github.com/zlc-ai/opc-platform/pkg/cost"
)

var costCmd = &cobra.Command{
	Use:   "cost",
	Short: "Cost tracking and reporting",
}

var costReportCmd = &cobra.Command{
	Use:   "report",
	Short: "Show cost report",
	RunE:  runCostReport,
}

var costExportCmd = &cobra.Command{
	Use:   "export",
	Short: "Export cost data",
	RunE:  runCostExport,
}

var costWatchCmd = &cobra.Command{
	Use:   "watch",
	Short: "Show real-time cost status",
	RunE:  runCostWatch,
}

var (
	costGroupBy string
	costPeriod  string
	costFormat  string
)

func init() {
	costReportCmd.Flags().StringVar(&costGroupBy, "by", "", "group by: agent|goal|project")
	costReportCmd.Flags().StringVar(&costPeriod, "period", "30d", "time period: 1d|7d|30d")

	costExportCmd.Flags().StringVar(&costFormat, "format", "csv", "export format: csv")

	costCmd.AddCommand(costReportCmd)
	costCmd.AddCommand(costExportCmd)
	costCmd.AddCommand(costWatchCmd)
	rootCmd.AddCommand(costCmd)
}

func getCostTracker() *cost.Tracker {
	config.InitLogger(false)
	dir := filepath.Join(config.GetConfigDir(), "cost")
	return cost.NewTracker(dir, config.Logger)
}

func parsePeriod(s string) time.Duration {
	switch s {
	case "1d":
		return 24 * time.Hour
	case "7d":
		return 7 * 24 * time.Hour
	case "30d":
		return 30 * 24 * time.Hour
	default:
		d, err := time.ParseDuration(s)
		if err != nil {
			return 30 * 24 * time.Hour
		}
		return d
	}
}

func runCostReport(cmd *cobra.Command, args []string) error {
	tracker := getCostTracker()
	period := parsePeriod(costPeriod)
	report := tracker.GenerateReport(costGroupBy, period)

	if output == "json" {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(report)
	}

	fmt.Printf("Cost Report (period: %s)\n", costPeriod)
	fmt.Printf("Total Cost:   $%.4f\n", report.TotalCost)
	fmt.Printf("Total Tokens: %d\n", report.TotalTokens)
	fmt.Printf("Events:       %d\n", report.EventCount)

	if len(report.ByAgent) > 0 {
		fmt.Println("\nBy Agent:")
		w := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
		fmt.Fprintln(w, "  AGENT\tCOST")
		for agent, c := range report.ByAgent {
			fmt.Fprintf(w, "  %s\t$%.4f\n", agent, c)
		}
		w.Flush()
	}

	if len(report.ByGoal) > 0 {
		fmt.Println("\nBy Goal:")
		w := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
		fmt.Fprintln(w, "  GOAL\tCOST")
		for goal, c := range report.ByGoal {
			fmt.Fprintf(w, "  %s\t$%.4f\n", goal, c)
		}
		w.Flush()
	}

	return nil
}

func runCostExport(cmd *cobra.Command, args []string) error {
	tracker := getCostTracker()

	data, err := tracker.ExportCSV()
	if err != nil {
		return err
	}

	fmt.Print(string(data))
	return nil
}

func runCostWatch(cmd *cobra.Command, args []string) error {
	tracker := getCostTracker()
	status := tracker.GetBudgetStatus()

	fmt.Println("Budget Status")
	fmt.Printf("Daily:   $%.2f / $%.2f (%.0f%%)\n", status.DailySpent, status.DailyLimit, status.DailyPct*100)
	fmt.Printf("Monthly: $%.2f / $%.2f (%.0f%%)\n", status.MonthlySpent, status.MonthlyLimit, status.MonthlyPct*100)
	if status.Exceeded {
		fmt.Println("\n*** BUDGET EXCEEDED ***")
	}

	return nil
}
