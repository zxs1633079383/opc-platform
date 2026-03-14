package goal

import "time"

// GoalStatus represents the lifecycle status of a goal.
type GoalStatus string

const (
	GoalPending    GoalStatus = "Pending"
	GoalInProgress GoalStatus = "InProgress"
	GoalCompleted  GoalStatus = "Completed"
	GoalFailed     GoalStatus = "Failed"
)

// Goal represents a high-level strategic objective that spans multiple companies.
type Goal struct {
	ID              string     `json:"id"`
	Name            string     `json:"name"`
	Description     string     `json:"description"`
	TargetCompanies []string   `json:"targetCompanies"`
	Projects        []*Project `json:"projects,omitempty"`
	Status          GoalStatus `json:"status"`
	CreatedBy       string     `json:"createdBy"`
	CreatedAt       time.Time  `json:"createdAt"`
}

// Project represents a deliverable within a goal, scoped to a single company.
type Project struct {
	ID           string   `json:"id"`
	GoalID       string   `json:"goalId"`
	CompanyID    string   `json:"companyId"`
	Name         string   `json:"name"`
	Tasks        []*Task  `json:"tasks,omitempty"`
	Dependencies []string `json:"dependencies,omitempty"`
}

// Task represents a unit of work within a project.
type Task struct {
	ID        string   `json:"id"`
	ProjectID string   `json:"projectId"`
	Name      string   `json:"name"`
	Issues    []*Issue `json:"issues,omitempty"`
}

// Issue represents the smallest executable unit, assigned to an agent.
type Issue struct {
	ID            string                 `json:"id"`
	TaskID        string                 `json:"taskId"`
	Name          string                 `json:"name"`
	AssignedAgent string                 `json:"assignedAgent,omitempty"`
	Context       map[string]interface{} `json:"context,omitempty"`
	AuditEvents   []string               `json:"auditEvents,omitempty"`
}
