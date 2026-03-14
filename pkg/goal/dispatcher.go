package goal

import (
	"fmt"
	"sync"

	"github.com/zlc-ai/opc-platform/pkg/federation"
	"go.uber.org/zap"
)

// Dispatcher distributes goals and issues across federated companies.
type Dispatcher struct {
	mu         sync.RWMutex
	goals      map[string]*Goal
	federation *federation.FederationController
	logger     *zap.SugaredLogger
}

// NewDispatcher creates a dispatcher backed by the given federation controller.
func NewDispatcher(fed *federation.FederationController, logger *zap.SugaredLogger) *Dispatcher {
	return &Dispatcher{
		goals:      make(map[string]*Goal),
		federation: fed,
		logger:     logger,
	}
}

// StoreGoal saves a goal in the dispatcher's goal registry.
func (d *Dispatcher) StoreGoal(g *Goal) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.goals[g.ID] = g
}

// ListGoals returns all stored goals.
func (d *Dispatcher) ListGoals() []*Goal {
	d.mu.RLock()
	defer d.mu.RUnlock()

	result := make([]*Goal, 0, len(d.goals))
	for _, g := range d.goals {
		result = append(result, g)
	}
	return result
}

// GetGoal returns a goal by ID.
func (d *Dispatcher) GetGoal(id string) (*Goal, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	g, ok := d.goals[id]
	if !ok {
		return nil, fmt.Errorf("goal %q not found", id)
	}
	return g, nil
}

// Dispatch distributes a goal's projects to their target companies.
func (d *Dispatcher) Dispatch(goal *Goal) error {
	if goal == nil {
		return fmt.Errorf("goal is nil")
	}

	d.mu.Lock()
	defer d.mu.Unlock()

	for _, project := range goal.Projects {
		company, err := d.federation.GetCompany(project.CompanyID)
		if err != nil {
			d.logger.Warnw("company not found, skipping project",
				"company_id", project.CompanyID,
				"project_id", project.ID,
			)
			continue
		}

		if company.Status != federation.CompanyStatusOnline {
			return fmt.Errorf("company %s is not online (status: %s)", company.Name, company.Status)
		}

		transport := d.federation.Transport()
		payload := map[string]interface{}{
			"goal_id":    goal.ID,
			"project_id": project.ID,
			"tasks":      project.Tasks,
		}
		if _, err := transport.Send(company.Endpoint, "POST", "/api/v1/dispatch", payload); err != nil {
			return fmt.Errorf("failed to dispatch to company %s: %w", company.Name, err)
		}

		d.logger.Infow("dispatched project to company",
			"project_id", project.ID,
			"company_id", company.ID,
		)
	}

	goal.Status = GoalInProgress
	return nil
}

// InjectContext sends context from one issue to another across companies.
func (d *Dispatcher) InjectContext(fromIssue, toIssue string, context map[string]interface{}) error {
	if fromIssue == "" || toIssue == "" {
		return fmt.Errorf("fromIssue and toIssue must not be empty")
	}

	d.mu.RLock()
	defer d.mu.RUnlock()

	payload := map[string]interface{}{
		"from_issue": fromIssue,
		"to_issue":   toIssue,
		"context":    context,
	}

	companies := d.federation.ListCompanies()
	for _, company := range companies {
		if company.Status != federation.CompanyStatusOnline {
			continue
		}
		transport := d.federation.Transport()
		if _, err := transport.Send(company.Endpoint, "POST", "/api/v1/context", payload); err != nil {
			d.logger.Warnw("failed to inject context to company",
				"company_id", company.ID,
				"error", err,
			)
			continue
		}
	}

	d.logger.Infow("injected cross-company context",
		"from_issue", fromIssue,
		"to_issue", toIssue,
	)
	return nil
}

// GetIssueQueue returns all pending issues for a given company.
func (d *Dispatcher) GetIssueQueue(companyID string) []*Issue {
	d.mu.RLock()
	defer d.mu.RUnlock()

	if _, err := d.federation.GetCompany(companyID); err != nil {
		d.logger.Warnw("company not found", "company_id", companyID)
		return nil
	}

	heartbeat := federation.NewHeartbeatMonitor(d.federation, d.logger)
	pending := heartbeat.ListPendingIssues()

	var issues []*Issue
	for _, p := range pending {
		if p.CompanyID == companyID {
			issues = append(issues, &Issue{
				ID:   p.IssueID,
				Name: p.Reason,
			})
		}
	}
	return issues
}
