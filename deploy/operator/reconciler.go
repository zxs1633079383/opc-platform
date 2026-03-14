package operator

import (
	"context"
	"fmt"
	"log"
	"time"
)

// ResourceState represents the observed state of a custom resource.
type ResourceState struct {
	Name       string
	Namespace  string
	Kind       string
	Generation int64
	Spec       map[string]any
	Status     map[string]any
}

// ReconcileResult tells the controller what to do after reconciliation.
type ReconcileResult struct {
	Requeue      bool
	RequeueAfter time.Duration
}

// Reconciler defines the interface for reconciling custom resources.
type Reconciler interface {
	Reconcile(ctx context.Context, resource ResourceState) (ReconcileResult, error)
}

// AgentReconciler reconciles Agent custom resources.
type AgentReconciler struct{}

func (r *AgentReconciler) Reconcile(ctx context.Context, resource ResourceState) (ReconcileResult, error) {
	log.Printf("reconciling Agent %s/%s (generation: %d)", resource.Namespace, resource.Name, resource.Generation)

	agentType, _ := resource.Spec["type"].(string)
	if agentType == "" {
		return ReconcileResult{}, fmt.Errorf("agent type is required")
	}

	currentPhase, _ := resource.Status["phase"].(string)
	if currentPhase == "" {
		resource.Status["phase"] = "Created"
		return ReconcileResult{Requeue: true}, nil
	}

	switch currentPhase {
	case "Created":
		resource.Status["phase"] = "Starting"
		return ReconcileResult{Requeue: true, RequeueAfter: 5 * time.Second}, nil
	case "Starting":
		resource.Status["phase"] = "Running"
		return ReconcileResult{RequeueAfter: 30 * time.Second}, nil
	case "Running":
		return ReconcileResult{RequeueAfter: 60 * time.Second}, nil
	case "Failed":
		restarts, _ := resource.Status["restarts"].(float64)
		maxRestarts := 3
		if recovery, ok := resource.Spec["recovery"].(map[string]any); ok {
			if mr, ok := recovery["maxRestarts"].(float64); ok {
				maxRestarts = int(mr)
			}
		}
		if int(restarts) < maxRestarts {
			resource.Status["restarts"] = restarts + 1
			resource.Status["phase"] = "Starting"
			return ReconcileResult{Requeue: true, RequeueAfter: 10 * time.Second}, nil
		}
		resource.Status["phase"] = "Terminated"
		return ReconcileResult{}, nil
	}

	return ReconcileResult{}, nil
}

// WorkflowReconciler reconciles Workflow custom resources.
type WorkflowReconciler struct{}

func (r *WorkflowReconciler) Reconcile(ctx context.Context, resource ResourceState) (ReconcileResult, error) {
	log.Printf("reconciling Workflow %s/%s", resource.Namespace, resource.Name)

	schedule, _ := resource.Spec["schedule"].(string)
	if schedule != "" {
		resource.Status["phase"] = "Scheduled"
	} else {
		resource.Status["phase"] = "Ready"
	}

	return ReconcileResult{RequeueAfter: 60 * time.Second}, nil
}

// OPCClusterReconciler reconciles OPCCluster custom resources.
type OPCClusterReconciler struct{}

func (r *OPCClusterReconciler) Reconcile(ctx context.Context, resource ResourceState) (ReconcileResult, error) {
	log.Printf("reconciling OPCCluster %s/%s", resource.Namespace, resource.Name)

	resource.Status["phase"] = "Running"
	resource.Status["ready"] = true

	return ReconcileResult{RequeueAfter: 30 * time.Second}, nil
}
