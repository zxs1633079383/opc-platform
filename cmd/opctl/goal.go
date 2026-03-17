package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/google/uuid"
	"github.com/spf13/cobra"
	"github.com/zlc-ai/opc-platform/pkg/client"
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
	goalCreateName             string
	goalCreateDescription      string
	goalCreateCompanies        string
	goalCreateAutoDecompose    bool
	goalCreateAutoApprove      bool
	goalMaxCost                int
	goalMaxAgents              int
	goalMaxTasks               int
	goalReviseFile             string
)

func init() {
	goalCreateCmd.Flags().StringVar(&goalCreateName, "name", "", "goal name (required)")
	goalCreateCmd.Flags().StringVar(&goalCreateDescription, "description", "", "goal description")
	goalCreateCmd.Flags().StringVar(&goalCreateCompanies, "companies", "", "target company IDs, comma-separated (required)")
	goalCreateCmd.Flags().BoolVar(&goalCreateAutoDecompose, "auto-decompose", false, "enable AI auto-decomposition")
	goalCreateCmd.Flags().BoolVar(&goalCreateAutoApprove, "auto-approve", false, "auto-approve after decomposition")
	goalCreateCmd.Flags().IntVar(&goalMaxCost, "max-cost", 0, "max cost in dollars")
	goalCreateCmd.Flags().IntVar(&goalMaxAgents, "max-agents", 0, "max number of agents")
	goalCreateCmd.Flags().IntVar(&goalMaxTasks, "max-tasks", 0, "max number of tasks per project")
	goalCreateCmd.MarkFlagRequired("name")
	goalCreateCmd.MarkFlagRequired("companies")

	goalReviseCmd.Flags().StringVar(&goalReviseFile, "file", "", "path to revised plan JSON file (required)")
	goalReviseCmd.MarkFlagRequired("file")

	goalCmd.AddCommand(goalCreateCmd)
	goalCmd.AddCommand(goalListCmd)
	goalCmd.AddCommand(goalStatusCmd)
	goalCmd.AddCommand(goalTraceCmd)
	goalCmd.AddCommand(goalInterveneCmd)
	goalCmd.AddCommand(goalPlanCmd)
	goalCmd.AddCommand(goalApproveCmd)
	goalCmd.AddCommand(goalReviseCmd)

	rootCmd.AddCommand(goalCmd)
}

func runGoalCreate(cmd *cobra.Command, args []string) error {
	logger := ensureLogger()

	companies := strings.Split(goalCreateCompanies, ",")
	for i := range companies {
		companies[i] = strings.TrimSpace(companies[i])
	}

	// If auto-decompose is enabled, delegate to the daemon API.
	if goalCreateAutoDecompose {
		return runGoalCreateViaDaemon(cmd.Context())
	}

	goalID := uuid.New().String()[:8]

	decomposer := goal.NewDecomposer(logger)
	result, err := decomposer.Decompose(cmd.Context(), goal.DecomposeRequest{
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

	dispatcher := goal.NewDispatcher(fc, logger)
	if err := dispatcher.Dispatch(g); err != nil {
		return fmt.Errorf("dispatch goal: %w", err)
	}

	fmt.Printf("Goal created: id=%s name=%s companies=%v projects=%d\n",
		g.ID, g.Name, g.TargetCompanies, len(g.Projects))
	return nil
}

func runGoalCreateViaDaemon(ctx context.Context) error {
	c := client.New(daemonAddr())

	approval := "required"
	if goalCreateAutoApprove {
		approval = "auto"
	}

	req := map[string]interface{}{
		"name":          goalCreateName,
		"description":   goalCreateDescription,
		"autoDecompose": true,
		"approval":      approval,
	}

	if goalMaxCost > 0 || goalMaxAgents > 0 || goalMaxTasks > 0 {
		constraints := map[string]int{}
		if goalMaxCost > 0 {
			constraints["maxCostDollars"] = goalMaxCost
		}
		if goalMaxAgents > 0 {
			constraints["maxAgents"] = goalMaxAgents
		}
		if goalMaxTasks > 0 {
			constraints["maxTasksPerProject"] = goalMaxTasks
		}
		req["constraints"] = constraints
	}

	var result map[string]interface{}
	if err := c.DoJSON(ctx, "POST", "/api/goals", req, &result); err != nil {
		return fmt.Errorf("create goal via daemon: %w", err)
	}

	data, _ := json.MarshalIndent(result, "", "  ")
	fmt.Println(string(data))
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
	dispatcher := goal.NewDispatcher(fc, logger)
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
	dispatcher := goal.NewDispatcher(fc, logger)
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
	dispatcher := goal.NewDispatcher(fc, logger)
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

// --- plan subcommand ---

var goalPlanCmd = &cobra.Command{
	Use:   "plan [goal-id]",
	Short: "Show the decomposition plan for a goal",
	Args:  cobra.ExactArgs(1),
	RunE:  runGoalPlan,
}

func runGoalPlan(cmd *cobra.Command, args []string) error {
	c := client.New(daemonAddr())

	var result json.RawMessage
	if err := c.DoJSON(cmd.Context(), "GET", "/api/goals/"+args[0]+"/plan", nil, &result); err != nil {
		return fmt.Errorf("get goal plan: %w", err)
	}

	// Pretty-print the plan.
	var pretty bytes.Buffer
	if err := json.Indent(&pretty, result, "", "  "); err != nil {
		fmt.Println(string(result))
		return nil
	}
	fmt.Println(pretty.String())
	return nil
}

// --- approve subcommand ---

var goalApproveCmd = &cobra.Command{
	Use:   "approve [goal-id]",
	Short: "Approve a planned goal for execution",
	Args:  cobra.ExactArgs(1),
	RunE:  runGoalApprove,
}

func runGoalApprove(cmd *cobra.Command, args []string) error {
	c := client.New(daemonAddr())

	var result map[string]interface{}
	if err := c.DoJSON(cmd.Context(), "POST", "/api/goals/"+args[0]+"/approve", nil, &result); err != nil {
		return fmt.Errorf("approve goal: %w", err)
	}

	data, _ := json.MarshalIndent(result, "", "  ")
	fmt.Println(string(data))
	return nil
}

// --- revise subcommand ---

var goalReviseCmd = &cobra.Command{
	Use:   "revise [goal-id]",
	Short: "Revise the decomposition plan for a goal",
	Args:  cobra.ExactArgs(1),
	RunE:  runGoalRevise,
}

func runGoalRevise(cmd *cobra.Command, args []string) error {
	c := client.New(daemonAddr())

	planData, err := os.ReadFile(goalReviseFile)
	if err != nil {
		return fmt.Errorf("read plan file: %w", err)
	}

	var plan json.RawMessage
	if err := json.Unmarshal(planData, &plan); err != nil {
		return fmt.Errorf("parse plan JSON: %w", err)
	}

	req := map[string]interface{}{
		"plan": plan,
	}

	var result map[string]interface{}
	if err := c.DoJSON(cmd.Context(), "POST", "/api/goals/"+args[0]+"/revise", req, &result); err != nil {
		return fmt.Errorf("revise goal: %w", err)
	}

	data, _ := json.MarshalIndent(result, "", "  ")
	fmt.Println(string(data))
	return nil
}

// daemonAddr returns the daemon address to connect to.
func daemonAddr() string {
	if addr := os.Getenv("OPC_DAEMON_ADDR"); addr != "" {
		return addr
	}
	return client.DefaultDaemonAddr
}
