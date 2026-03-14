package federation

import (
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"
)

// Intervention records a human intervention on an issue.
type Intervention struct {
	ID        string
	IssueID   string
	Action    string // approve, reject, modify
	Reason    string
	ActorID   string
	Timestamp time.Time
}

// interventionRegistry stores interventions and approval gates.
type interventionRegistry struct {
	mu            sync.RWMutex
	interventions map[string]*Intervention // keyed by intervention ID
	approvalGates map[string]bool          // keyed by task ID; true = gate set
	taskApprovals map[string]*Intervention // keyed by task ID
}

func newInterventionRegistry() *interventionRegistry {
	return &interventionRegistry{
		interventions: make(map[string]*Intervention),
		approvalGates: make(map[string]bool),
		taskApprovals: make(map[string]*Intervention),
	}
}

// registry is a package-level intervention registry.
var registry = newInterventionRegistry()

// InterventionAction represents the type of intervention action.
type InterventionAction string

// InterventionRequest is the input for handling an intervention.
type InterventionRequest struct {
	IssueID string
	Action  InterventionAction
	Reason  string
}

// InterventionResult is the output of handling an intervention.
type InterventionResult struct {
	IssueID string
	Action  InterventionAction
	Status  string
	Message string
}

// InterventionHandler processes intervention requests.
type InterventionHandler struct {
	controller *FederationController
	logger     *zap.SugaredLogger
}

// NewInterventionHandler creates a new InterventionHandler.
func NewInterventionHandler(logger *zap.SugaredLogger, controller *FederationController) *InterventionHandler {
	return &InterventionHandler{
		controller: controller,
		logger:     logger,
	}
}

// Handle processes an intervention request.
func (h *InterventionHandler) Handle(req InterventionRequest) (*InterventionResult, error) {
	action := string(req.Action)
	if err := h.controller.Intervene(req.IssueID, action, req.Reason, "operator"); err != nil {
		return nil, err
	}

	return &InterventionResult{
		IssueID: req.IssueID,
		Action:  req.Action,
		Status:  "applied",
		Message: fmt.Sprintf("intervention %s applied to issue %s", action, req.IssueID),
	}, nil
}

// Intervene records a human intervention on an issue.
func (fc *FederationController) Intervene(issueID, action, reason, actorID string) error {
	if issueID == "" {
		return fmt.Errorf("issueID must not be empty")
	}
	validActions := map[string]bool{"approve": true, "reject": true, "modify": true}
	if !validActions[action] {
		return fmt.Errorf("invalid action %q: must be approve, reject, or modify", action)
	}

	intervention := &Intervention{
		ID:        uuid.New().String(),
		IssueID:   issueID,
		Action:    action,
		Reason:    reason,
		ActorID:   actorID,
		Timestamp: time.Now(),
	}

	registry.mu.Lock()
	registry.interventions[intervention.ID] = intervention
	registry.mu.Unlock()

	fc.logger.Infow("human intervention recorded",
		"intervention_id", intervention.ID,
		"issue_id", issueID,
		"action", action,
		"actor_id", actorID,
	)
	return nil
}

// SetApprovalGate marks a task as requiring human approval before proceeding.
func (fc *FederationController) SetApprovalGate(taskID string) error {
	if taskID == "" {
		return fmt.Errorf("taskID must not be empty")
	}

	registry.mu.Lock()
	registry.approvalGates[taskID] = true
	registry.mu.Unlock()

	fc.logger.Infow("approval gate set", "task_id", taskID)
	return nil
}

// ApproveTask approves a task that has an approval gate.
func (fc *FederationController) ApproveTask(taskID, approverID string) error {
	if taskID == "" {
		return fmt.Errorf("taskID must not be empty")
	}

	registry.mu.Lock()
	defer registry.mu.Unlock()

	if !registry.approvalGates[taskID] {
		return fmt.Errorf("no approval gate set for task %s", taskID)
	}

	intervention := &Intervention{
		ID:        uuid.New().String(),
		IssueID:   taskID,
		Action:    "approve",
		ActorID:   approverID,
		Timestamp: time.Now(),
	}
	registry.taskApprovals[taskID] = intervention
	delete(registry.approvalGates, taskID)

	fc.logger.Infow("task approved",
		"task_id", taskID,
		"approver_id", approverID,
	)
	return nil
}

// RejectTask rejects a task that has an approval gate.
func (fc *FederationController) RejectTask(taskID, approverID, reason string) error {
	if taskID == "" {
		return fmt.Errorf("taskID must not be empty")
	}

	registry.mu.Lock()
	defer registry.mu.Unlock()

	if !registry.approvalGates[taskID] {
		return fmt.Errorf("no approval gate set for task %s", taskID)
	}

	intervention := &Intervention{
		ID:        uuid.New().String(),
		IssueID:   taskID,
		Action:    "reject",
		Reason:    reason,
		ActorID:   approverID,
		Timestamp: time.Now(),
	}
	registry.taskApprovals[taskID] = intervention
	delete(registry.approvalGates, taskID)

	fc.logger.Infow("task rejected",
		"task_id", taskID,
		"approver_id", approverID,
		"reason", reason,
	)
	return nil
}
