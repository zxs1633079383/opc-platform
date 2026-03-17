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
	Description  string   `json:"description,omitempty"`
	Tasks        []*Task  `json:"tasks,omitempty"`
	Dependencies []string `json:"dependencies,omitempty"`
}

// Task represents a unit of work within a project.
type Task struct {
	ID          string   `json:"id"`
	ProjectID   string   `json:"projectId"`
	Name        string   `json:"name"`
	Description string   `json:"description,omitempty"`
	AssignAgent string   `json:"assignAgent,omitempty"`
	Complexity  string   `json:"complexity,omitempty"` // low | medium | high
	DependsOn   []string `json:"dependsOn,omitempty"`
	Issues      []*Issue `json:"issues,omitempty"`
}

// Issue represents the smallest executable unit, assigned to an agent.
type Issue struct {
	ID            string                 `json:"id"`
	TaskID        string                 `json:"taskId"`
	Name          string                 `json:"name"`
	Description   string                 `json:"description,omitempty"`
	AssignedAgent string                 `json:"assignedAgent,omitempty"`
	Context       map[string]interface{} `json:"context,omitempty"`
	AuditEvents   []string               `json:"auditEvents,omitempty"`
}

// AIDecomposeResult holds the structured output from AI goal decomposition.
type AIDecomposeResult struct {
	Projects []AIProject `json:"projects"`
}

// AIProject represents a project in the AI decomposition output.
type AIProject struct {
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Tasks       []AITask `json:"tasks"`
}

// AITask represents a task in the AI decomposition output.
type AITask struct {
	Name        string    `json:"name"`
	Description string    `json:"description"`
	AssignAgent string    `json:"assignAgent"`
	Complexity  string    `json:"complexity"` // low | medium | high
	DependsOn   []string  `json:"dependsOn,omitempty"`
	Issues      []AIIssue `json:"issues,omitempty"`
}

// AIIssue represents an issue in the AI decomposition output.
type AIIssue struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}
