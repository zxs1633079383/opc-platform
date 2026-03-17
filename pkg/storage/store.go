package storage

import (
	"context"

	v1 "github.com/zlc-ai/opc-platform/api/v1"
)

// Store defines the interface for persisting OPC Platform state.
type Store interface {
	// Agent operations.
	CreateAgent(ctx context.Context, agent v1.AgentRecord) error
	GetAgent(ctx context.Context, name string) (v1.AgentRecord, error)
	ListAgents(ctx context.Context) ([]v1.AgentRecord, error)
	UpdateAgent(ctx context.Context, agent v1.AgentRecord) error
	DeleteAgent(ctx context.Context, name string) error

	// Task operations.
	CreateTask(ctx context.Context, task v1.TaskRecord) error
	GetTask(ctx context.Context, id string) (v1.TaskRecord, error)
	ListTasks(ctx context.Context) ([]v1.TaskRecord, error)
	ListTasksByAgent(ctx context.Context, agentName string) ([]v1.TaskRecord, error)
	UpdateTask(ctx context.Context, task v1.TaskRecord) error

	// Workflow operations.
	CreateWorkflow(ctx context.Context, wf v1.WorkflowRecord) error
	GetWorkflow(ctx context.Context, name string) (v1.WorkflowRecord, error)
	ListWorkflows(ctx context.Context) ([]v1.WorkflowRecord, error)
	UpdateWorkflow(ctx context.Context, wf v1.WorkflowRecord) error
	DeleteWorkflow(ctx context.Context, name string) error

	// Workflow run operations.
	CreateWorkflowRun(ctx context.Context, run v1.WorkflowRunRecord) error
	GetWorkflowRun(ctx context.Context, id string) (v1.WorkflowRunRecord, error)
	ListWorkflowRuns(ctx context.Context, workflowName string) ([]v1.WorkflowRunRecord, error)
	UpdateWorkflowRun(ctx context.Context, run v1.WorkflowRunRecord) error

	// Goal operations.
	CreateGoal(ctx context.Context, goal v1.GoalRecord) error
	GetGoal(ctx context.Context, id string) (v1.GoalRecord, error)
	ListGoals(ctx context.Context) ([]v1.GoalRecord, error)
	UpdateGoal(ctx context.Context, goal v1.GoalRecord) error
	DeleteGoal(ctx context.Context, id string) error
	GoalStats(ctx context.Context, goalID string) (v1.HierarchyStats, error)

	// Project operations.
	CreateProject(ctx context.Context, project v1.ProjectRecord) error
	GetProject(ctx context.Context, id string) (v1.ProjectRecord, error)
	ListProjects(ctx context.Context) ([]v1.ProjectRecord, error)
	ListProjectsByGoal(ctx context.Context, goalID string) ([]v1.ProjectRecord, error)
	UpdateProject(ctx context.Context, project v1.ProjectRecord) error
	DeleteProject(ctx context.Context, id string) error
	ProjectStats(ctx context.Context, projectID string) (v1.HierarchyStats, error)

	// Issue operations.
	CreateIssue(ctx context.Context, issue v1.IssueRecord) error
	GetIssue(ctx context.Context, id string) (v1.IssueRecord, error)
	ListIssues(ctx context.Context) ([]v1.IssueRecord, error)
	ListIssuesByProject(ctx context.Context, projectID string) ([]v1.IssueRecord, error)
	UpdateIssue(ctx context.Context, issue v1.IssueRecord) error
	DeleteIssue(ctx context.Context, id string) error

	// Close releases all resources.
	Close() error
}
