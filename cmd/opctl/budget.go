package main

import (
	"fmt"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
	"github.com/zlc-ai/opc-platform/internal/config"
	"github.com/zlc-ai/opc-platform/pkg/cost"
)

var budgetCmd = &cobra.Command{
	Use:   "budget",
	Short: "Budget management",
}

var budgetSetCmd = &cobra.Command{
	Use:   "set",
	Short: "Set budget limits",
	RunE:  runBudgetSet,
}

var (
	budgetDaily   string
	budgetMonthly string
)

func init() {
	budgetSetCmd.Flags().StringVar(&budgetDaily, "daily", "", "daily budget limit (e.g., $10)")
	budgetSetCmd.Flags().StringVar(&budgetMonthly, "monthly", "", "monthly budget limit (e.g., $200)")

	budgetCmd.AddCommand(budgetSetCmd)
	rootCmd.AddCommand(budgetCmd)
}

func parseDollar(s string) (float64, error) {
	s = strings.TrimPrefix(s, "$")
	return strconv.ParseFloat(s, 64)
}

func runBudgetSet(cmd *cobra.Command, args []string) error {
	config.InitLogger(false)
	dir := filepath.Join(config.GetConfigDir(), "cost")
	tracker := cost.NewTracker(dir, config.Logger)

	budget := cost.BudgetConfig{AlertPct: 0.8}

	if budgetDaily != "" {
		v, err := parseDollar(budgetDaily)
		if err != nil {
			return fmt.Errorf("invalid daily budget: %w", err)
		}
		budget.DailyLimit = v
	}

	if budgetMonthly != "" {
		v, err := parseDollar(budgetMonthly)
		if err != nil {
			return fmt.Errorf("invalid monthly budget: %w", err)
		}
		budget.MonthlyLimit = v
	}

	tracker.SetBudget(budget)

	if budgetDaily != "" {
		fmt.Printf("Daily budget set to $%.2f\n", budget.DailyLimit)
	}
	if budgetMonthly != "" {
		fmt.Printf("Monthly budget set to $%.2f\n", budget.MonthlyLimit)
	}

	return nil
}
