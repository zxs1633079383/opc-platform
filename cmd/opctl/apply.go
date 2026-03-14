package main

import (
	"context"
	"fmt"
	"os"

	"github.com/spf13/cobra"
	v1 "github.com/zlc-ai/opc-platform/api/v1"
	"github.com/zlc-ai/opc-platform/pkg/workflow"
	"gopkg.in/yaml.v3"
)

var applyCmd = &cobra.Command{
	Use:   "apply -f <file>",
	Short: "Apply a configuration from a YAML file",
	Long:  `Create or update resources defined in a YAML file. Supports AgentSpec, Workflow, Goal, Project, Task, and Issue kinds.`,
	RunE:  runApply,
}

var applyFile string

func init() {
	applyCmd.Flags().StringVarP(&applyFile, "file", "f", "", "path to YAML file (required)")
	applyCmd.MarkFlagRequired("file")
	rootCmd.AddCommand(applyCmd)
}

func runApply(cmd *cobra.Command, args []string) error {
	data, err := os.ReadFile(applyFile)
	if err != nil {
		return fmt.Errorf("read file %q: %w", applyFile, err)
	}

	// Parse kind to determine resource type.
	var res v1.Resource
	if err := yaml.Unmarshal(data, &res); err != nil {
		return fmt.Errorf("parse YAML: %w", err)
	}

	if res.APIVersion != v1.APIVersion {
		return fmt.Errorf("unsupported apiVersion %q, expected %q", res.APIVersion, v1.APIVersion)
	}

	switch res.Kind {
	case v1.KindAgentSpec:
		return applyAgentSpec(data)
	case v1.KindWorkflow:
		return applyWorkflow(data)
	default:
		return fmt.Errorf("unsupported kind %q", res.Kind)
	}
}

func applyAgentSpec(data []byte) error {
	var spec v1.AgentSpec
	if err := yaml.Unmarshal(data, &spec); err != nil {
		return fmt.Errorf("parse AgentSpec: %w", err)
	}

	if spec.Metadata.Name == "" {
		return fmt.Errorf("metadata.name is required")
	}
	if spec.Spec.Type == "" {
		return fmt.Errorf("spec.type is required")
	}

	ctrl, cleanup, err := getController()
	if err != nil {
		return err
	}
	defer cleanup()

	if err := ctrl.Apply(context.Background(), spec); err != nil {
		return err
	}

	fmt.Printf("agent/%s configured\n", spec.Metadata.Name)
	return nil
}

func applyWorkflow(data []byte) error {
	spec, err := workflow.ParseWorkflow(data)
	if err != nil {
		return err
	}

	ctrl, cleanup, err := getController()
	if err != nil {
		return err
	}
	defer cleanup()

	record := v1.WorkflowRecord{
		Name:     spec.Metadata.Name,
		SpecYAML: string(data),
		Schedule: spec.Spec.Schedule,
		Enabled:  true,
	}

	// Try update first, then create.
	existing, getErr := ctrl.Store().GetWorkflow(context.Background(), spec.Metadata.Name)
	if getErr == nil {
		existing.SpecYAML = string(data)
		existing.Schedule = spec.Spec.Schedule
		if err := ctrl.Store().UpdateWorkflow(context.Background(), existing); err != nil {
			return err
		}
		fmt.Printf("workflow/%s updated\n", spec.Metadata.Name)
	} else {
		if err := ctrl.Store().CreateWorkflow(context.Background(), record); err != nil {
			return err
		}
		fmt.Printf("workflow/%s created\n", spec.Metadata.Name)
	}

	return nil
}
