package main

import (
	"fmt"
	"time"

	"github.com/spf13/cobra"
)

var logsCmd = &cobra.Command{
	Use:   "logs <task-id>",
	Short: "View task logs and output",
	Args:  cobra.ExactArgs(1),
	RunE:  runLogs,
}

func init() {
	rootCmd.AddCommand(logsCmd)
}

func runLogs(cmd *cobra.Command, args []string) error {
	taskID := args[0]

	ctrl, cleanup, err := getController()
	if err != nil {
		return err
	}
	defer cleanup()

	task, err := ctrl.Store().GetTask(cmd.Context(), taskID)
	if err != nil {
		return fmt.Errorf("task %q not found", taskID)
	}

	fmt.Printf("Task:    %s\n", task.ID)
	fmt.Printf("Agent:   %s\n", task.AgentName)
	fmt.Printf("Status:  %s\n", task.Status)
	fmt.Printf("Created: %s\n", task.CreatedAt.Format(time.RFC3339))
	if task.StartedAt != nil {
		fmt.Printf("Started: %s\n", task.StartedAt.Format(time.RFC3339))
	}
	if task.EndedAt != nil {
		fmt.Printf("Ended:   %s\n", task.EndedAt.Format(time.RFC3339))
		if task.StartedAt != nil {
			fmt.Printf("Duration: %s\n", task.EndedAt.Sub(*task.StartedAt).Round(time.Millisecond))
		}
	}
	if task.TokensIn > 0 || task.TokensOut > 0 {
		fmt.Printf("Tokens:  in=%d out=%d\n", task.TokensIn, task.TokensOut)
	}
	if task.Cost > 0 {
		fmt.Printf("Cost:    $%.4f\n", task.Cost)
	}

	fmt.Println("\n--- Message ---")
	fmt.Println(task.Message)

	if task.Result != "" {
		fmt.Println("\n--- Output ---")
		fmt.Println(task.Result)
	}

	if task.Error != "" {
		fmt.Println("\n--- Error ---")
		fmt.Println(task.Error)
	}

	return nil
}
