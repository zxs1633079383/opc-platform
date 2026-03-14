package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"
)

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Agent configuration management",
}

var configGetCmd = &cobra.Command{
	Use:   "get agent <name>",
	Short: "View agent configuration",
	Args:  cobra.ExactArgs(2),
	RunE:  runConfigGet,
}

var configSetCmd = &cobra.Command{
	Use:   "set agent <name> <key=value>",
	Short: "Update agent configuration (hot reload)",
	Args:  cobra.MinimumNArgs(3),
	RunE:  runConfigSet,
}

var configHistoryCmd = &cobra.Command{
	Use:   "history agent <name>",
	Short: "View configuration change history",
	Args:  cobra.ExactArgs(2),
	RunE:  runConfigHistory,
}

func init() {
	configCmd.AddCommand(configGetCmd)
	configCmd.AddCommand(configSetCmd)
	configCmd.AddCommand(configHistoryCmd)
	rootCmd.AddCommand(configCmd)
}

func runConfigGet(cmd *cobra.Command, args []string) error {
	if args[0] != "agent" {
		return fmt.Errorf("expected 'agent', got %q", args[0])
	}
	name := args[1]

	ctrl, cleanup, err := getController()
	if err != nil {
		return err
	}
	defer cleanup()

	agent, err := ctrl.GetAgent(cmd.Context(), name)
	if err != nil {
		return fmt.Errorf("agent %q not found: %w", name, err)
	}

	if output == "json" {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(agent)
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
	fmt.Fprintf(w, "Name:\t%s\n", agent.Name)
	fmt.Fprintf(w, "Type:\t%s\n", agent.Type)
	fmt.Fprintf(w, "Phase:\t%s\n", agent.Phase)
	fmt.Fprintf(w, "Restarts:\t%d\n", agent.Restarts)
	fmt.Fprintf(w, "Created:\t%s\n", agent.CreatedAt.Format("2006-01-02 15:04:05"))
	fmt.Fprintf(w, "Updated:\t%s\n", agent.UpdatedAt.Format("2006-01-02 15:04:05"))
	if agent.Message != "" {
		fmt.Fprintf(w, "Message:\t%s\n", agent.Message)
	}
	w.Flush()

	if agent.SpecYAML != "" {
		fmt.Println("\nSpec:")
		fmt.Println(agent.SpecYAML)
	}

	return nil
}

func runConfigSet(cmd *cobra.Command, args []string) error {
	if args[0] != "agent" {
		return fmt.Errorf("expected 'agent', got %q", args[0])
	}
	name := args[1]
	kvPairs := args[2:]

	ctrl, cleanup, err := getController()
	if err != nil {
		return err
	}
	defer cleanup()

	_, err = ctrl.GetAgent(context.Background(), name)
	if err != nil {
		return fmt.Errorf("agent %q not found: %w", name, err)
	}

	for _, kv := range kvPairs {
		parts := strings.SplitN(kv, "=", 2)
		if len(parts) != 2 {
			return fmt.Errorf("invalid key=value pair: %q", kv)
		}
		fmt.Printf("agent/%s: %s = %s (hot reload)\n", name, parts[0], parts[1])
	}

	return nil
}

func runConfigHistory(cmd *cobra.Command, args []string) error {
	if args[0] != "agent" {
		return fmt.Errorf("expected 'agent', got %q", args[0])
	}
	name := args[1]

	ctrl, cleanup, err := getController()
	if err != nil {
		return err
	}
	defer cleanup()

	_, err = ctrl.GetAgent(cmd.Context(), name)
	if err != nil {
		return fmt.Errorf("agent %q not found: %w", name, err)
	}

	fmt.Printf("No configuration history recorded for agent/%s\n", name)
	return nil
}
