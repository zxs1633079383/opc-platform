package main

import (
	"fmt"
	"os"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/google/uuid"
	"github.com/spf13/cobra"
	"github.com/zlc-ai/opc-platform/pkg/federation"
	"github.com/zlc-ai/opc-platform/pkg/goal"
)

var goalCmd = &cobra.Command{
	Use:   "goal",
	Short: "Manage goals across companies",
}

// --- create subcommand ---

var goalCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create a new cross-company goal",
	RunE:  runGoalCreate,
}

var (
	goalCreateName        string
	goalCreateDescription string
	goalCreateCompanies   string
)

func init() {
	goalCreateCmd.Flags().StringVar(&goalCreateName, "name", "", "goal name (required)")
	goalCreateCmd.Flags().StringVar(&goalCreateDescription, "description", "", "goal description")
	goalCreateCmd.Flags().StringVar(&goalCreateCompanies, "companies", "", "target company IDs, comma-separated (required)")
	goalCreateCmd.MarkFlagRequired("name")
	goalCreateCmd.MarkFlagRequired("companies")

	goalCmd.AddCommand(goalCreateCmd)
	goalCmd.AddCommand(goalListCmd)
	goalCmd.AddCommand(goalStatusCmd)
	goalCmd.AddCommand(goalTraceCmd)
	goalCmd.AddCommand(goalInterveneCmd)

	rootCmd.AddCommand(goalCmd)
}

func runGoalCreate(cmd *cobra.Command, args []string) error {
	logger := ensureLogger()

	companies := strings.Split(goalCreateCompanies, ",")
	for i := range companies {
		companies[i] = strings.TrimSpace(companies[i])
	}

	goalID := uuid.New().String()[:8]

	decomposer := goal.NewDecomposer(logger)
	result, err := decomposer.Decompose(goal.DecomposeRequest{
		GoalID:          goalID,
		GoalName:        goalCreateName,
		Description:     goalCreateDescription,
		TargetCompanies: companies,
	})
	if err != nil {
		return fmt.Errorf("decompose goal: %w", err)
	}

	fc := federation.NewController(logger)
	g := &goal.Goal{
		ID:              goalID,
		Name:            goalCreateName,
		Description:     goalCreateDescription,
		TargetCompanies: companies,
		Projects:        result.Projects,
		Status:          goal.GoalPending,
		CreatedBy:       "operator",
		CreatedAt:       time.Now().UTC(),
	}

	dispatcher := goal.NewDispatcher(logger, fc)
	if err := dispatcher.Dispatch(g); err != nil {
		return fmt.Errorf("dispatch goal: %w", err)
	}

	fmt.Printf("Goal created: id=%s name=%s companies=%v projects=%d\n",
		g.ID, g.Name, g.TargetCompanies, len(g.Projects))
	return nil
}

// --- list subcommand ---

var goalListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all goals",
	RunE:  runGoalList,
}

func runGoalList(cmd *cobra.Command, args []string) error {
	logger := ensureLogger()

	fc := federation.NewController(logger)
	dispatcher := goal.NewDispatcher(logger, fc)
	goals := dispatcher.ListGoals()

	if len(goals) == 0 {
		fmt.Println("No goals found.")
		return nil
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
	fmt.Fprintln(w, "ID\tNAME\tSTATUS\tCOMPANIES\tPROJECTS")
	for _, g := range goals {
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%d\n",
			g.ID, g.Name, g.Status,
			strings.Join(g.TargetCompanies, ","), len(g.Projects))
	}
	return w.Flush()
}

// --- status subcommand ---

var goalStatusCmd = &cobra.Command{
	Use:   "status [goal-id]",
	Short: "Show goal progress (Goal -> Projects -> Tasks -> Issues)",
	Args:  cobra.ExactArgs(1),
	RunE:  runGoalStatus,
}

func runGoalStatus(cmd *cobra.Command, args []string) error {
	logger := ensureLogger()

	fc := federation.NewController(logger)
	dispatcher := goal.NewDispatcher(logger, fc)
	g, err := dispatcher.GetGoal(args[0])
	if err != nil {
		return err
	}

	fmt.Printf("Goal: %s (%s)\n", g.Name, g.Status)
	fmt.Printf("Description: %s\n", g.Description)
	fmt.Printf("Companies: %s\n", strings.Join(g.TargetCompanies, ", "))
	fmt.Println()

	for _, p := range g.Projects {
		fmt.Printf("  Project: %s (company: %s)\n", p.Name, p.CompanyID)
		for _, t := range p.Tasks {
			fmt.Printf("    Task: %s\n", t.Name)
			for _, i := range t.Issues {
				agent := i.AssignedAgent
				if agent == "" {
					agent = "<unassigned>"
				}
				fmt.Printf("      Issue: %s [agent: %s]\n", i.Name, agent)
			}
		}
	}

	return nil
}

// --- trace subcommand ---

var goalTraceCmd = &cobra.Command{
	Use:   "trace [goal-id]",
	Short: "Trace audit chain for a goal",
	Args:  cobra.ExactArgs(1),
	RunE:  runGoalTrace,
}

func runGoalTrace(cmd *cobra.Command, args []string) error {
	logger := ensureLogger()

	fc := federation.NewController(logger)
	dispatcher := goal.NewDispatcher(logger, fc)
	g, err := dispatcher.GetGoal(args[0])
	if err != nil {
		return err
	}

	fmt.Printf("Audit Trace: %s (%s)\n", g.Name, g.ID)
	fmt.Println(strings.Repeat("=", 40))

	for _, p := range g.Projects {
		for _, t := range p.Tasks {
			for _, i := range t.Issues {
				if len(i.AuditEvents) == 0 {
					continue
				}
				fmt.Printf("\nIssue: %s (%s)\n", i.Name, i.ID)
				for _, event := range i.AuditEvents {
					fmt.Printf("  - %s\n", event)
				}
			}
		}
	}

	return nil
}

// --- intervene subcommand ---

var goalInterveneCmd = &cobra.Command{
	Use:   "intervene",
	Short: "Intervene on a pending issue",
	RunE:  runGoalIntervene,
}

var (
	interveneIssue  string
	interveneAction string
	interveneReason string
)

func init() {
	goalInterveneCmd.Flags().StringVar(&interveneIssue, "issue", "", "issue ID (required)")
	goalInterveneCmd.Flags().StringVar(&interveneAction, "action", "", "action: approve|reject|modify (required)")
	goalInterveneCmd.Flags().StringVar(&interveneReason, "reason", "", "reason for intervention")
	goalInterveneCmd.MarkFlagRequired("issue")
	goalInterveneCmd.MarkFlagRequired("action")
}

func runGoalIntervene(cmd *cobra.Command, args []string) error {
	logger := ensureLogger()

	fc := federation.NewController(logger)
	handler := federation.NewInterventionHandler(logger, fc)

	result, err := handler.Handle(federation.InterventionRequest{
		IssueID: interveneIssue,
		Action:  federation.InterventionAction(interveneAction),
		Reason:  interveneReason,
	})
	if err != nil {
		return fmt.Errorf("intervene: %w", err)
	}

	fmt.Printf("Intervention applied: issue=%s action=%s status=%s\n",
		result.IssueID, result.Action, result.Status)
	if result.Message != "" {
		fmt.Printf("Message: %s\n", result.Message)
	}

	return nil
}
