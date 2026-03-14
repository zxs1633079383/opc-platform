package main

import (
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"
	v1 "github.com/zlc-ai/opc-platform/api/v1"
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

var (
	runAgentName string
	runStream    bool
)

func init() {
	runTaskCmd.Flags().StringVar(&runAgentName, "agent", "", "agent name (required)")
	runTaskCmd.MarkFlagRequired("agent")
	runTaskCmd.Flags().BoolVar(&runStream, "stream", false, "enable streaming output")

	runCmd.AddCommand(runTaskCmd)
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

func generateTaskID() string {
	return fmt.Sprintf("task-%d", time.Now().UnixNano()/1e6)
}
