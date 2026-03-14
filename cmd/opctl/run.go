package main

import (
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"
	v1 "github.com/zlc-ai/opc-platform/api/v1"
	"github.com/zlc-ai/opc-platform/internal/config"
	"github.com/zlc-ai/opc-platform/pkg/workflow"
)

var runCmd = &cobra.Command{
	Use:   "run",
	Short: "Execute a task or workflow",
}

var runTaskCmd = &cobra.Command{
	Use:   "--agent <name> <message>",
	Short: "Execute a task against an agent",
	RunE:  runRunTask,
}

var runWorkflowCmd = &cobra.Command{
	Use:   "workflow <name>",
	Short: "Execute a workflow",
	Args:  cobra.ExactArgs(1),
	RunE:  runRunWorkflow,
}

var (
	runAgentName string
	runStream    bool
)

func init() {
	runTaskCmd.Flags().StringVar(&runAgentName, "agent", "", "agent name (required)")
	runTaskCmd.MarkFlagRequired("agent")
	runTaskCmd.Flags().BoolVar(&runStream, "stream", false, "enable streaming output")

	runCmd.AddCommand(runTaskCmd)
	runCmd.AddCommand(runWorkflowCmd)
	rootCmd.AddCommand(runCmd)

	// Also allow `opctl run --agent <name> "message"` directly.
	rootCmd.AddCommand(&cobra.Command{
		Use:    "exec --agent <name> <message>",
		Short:  "Execute a task (alias for run)",
		Hidden: true,
		RunE:   runRunTask,
	})
}

func runRunTask(cmd *cobra.Command, args []string) error {
	if runAgentName == "" {
		return fmt.Errorf("--agent flag is required")
	}
	if len(args) == 0 {
		return fmt.Errorf("message is required")
	}

	message := strings.Join(args, " ")

	ctrl, cleanup, err := getController()
	if err != nil {
		return err
	}
	defer cleanup()

	// Create task record.
	taskID := generateTaskID()
	task := v1.TaskRecord{
		ID:        taskID,
		AgentName: runAgentName,
		Message:   message,
		Status:    v1.TaskStatusPending,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	if err := ctrl.Store().CreateTask(cmd.Context(), task); err != nil {
		return fmt.Errorf("create task: %w", err)
	}

	fmt.Printf("task/%s created (agent: %s)\n", taskID, runAgentName)

	if runStream {
		ch, err := ctrl.StreamTask(cmd.Context(), task)
		if err != nil {
			return err
		}
		for chunk := range ch {
			if chunk.Error != nil {
				return chunk.Error
			}
			fmt.Print(chunk.Content)
		}
		fmt.Println()
	} else {
		result, err := ctrl.ExecuteTask(cmd.Context(), task)
		if err != nil {
			return err
		}
		fmt.Println(result.Output)
		fmt.Printf("\n--- Tokens: in=%d out=%d ---\n", result.TokensIn, result.TokensOut)
	}

	return nil
}

func runRunWorkflow(cmd *cobra.Command, args []string) error {
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

	spec, err := workflow.ParseWorkflow([]byte(wf.SpecYAML))
	if err != nil {
		return fmt.Errorf("parse workflow: %w", err)
	}

	logger := config.Logger
	if logger == nil {
		config.InitLogger(false)
		logger = config.Logger
	}

	engine := workflow.NewEngine(ctrl, ctrl.Store(), logger)

	fmt.Printf("Running workflow/%s...\n", name)

	run, err := engine.Execute(cmd.Context(), spec)
	if err != nil {
		fmt.Printf("workflow/%s failed: %v\n", name, err)
		if run != nil {
			printWorkflowRun(run)
		}
		return err
	}

	printWorkflowRun(run)
	return nil
}

func printWorkflowRun(run *workflow.WorkflowRun) {
	fmt.Printf("\nWorkflow Run: %s\n", run.ID)
	fmt.Printf("Status: %s\n", run.Status)
	fmt.Printf("Started: %s\n", run.StartedAt.Format(time.RFC3339))
	if run.EndedAt != nil {
		fmt.Printf("Duration: %s\n", run.EndedAt.Sub(run.StartedAt))
	}
	fmt.Println("\nSteps:")
	for _, sr := range run.Steps {
		fmt.Printf("  %s: %s", sr.Name, sr.Status)
		if sr.Error != "" {
			fmt.Printf(" (error: %s)", sr.Error)
		}
		fmt.Println()
	}
}

func generateTaskID() string {
	return fmt.Sprintf("task-%d", time.Now().UnixNano()/1e6)
}
