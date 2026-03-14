package v1

import "time"

// APIVersion is the current API version.
const APIVersion = "opc/v1"

// Kind constants for resource types.
const (
	KindAgentSpec = "AgentSpec"
	KindTask      = "Task"
	KindGoal      = "Goal"
	KindProject   = "Project"
	KindIssue     = "Issue"
	KindWorkflow  = "Workflow"
	KindCostEvent = "CostEvent"
)

// Metadata contains common metadata for all resources.
type Metadata struct {
	Name      string            `yaml:"name" json:"name"`
	Labels    map[string]string `yaml:"labels,omitempty" json:"labels,omitempty"`
	CreatedAt time.Time         `yaml:"createdAt,omitempty" json:"createdAt,omitempty"`
	UpdatedAt time.Time         `yaml:"updatedAt,omitempty" json:"updatedAt,omitempty"`
}

// AgentType represents the type of an Agent.
type AgentType string

const (
	AgentTypeOpenClaw   AgentType = "openclaw"
	AgentTypeClaudeCode AgentType = "claude-code"
	AgentTypeCodex      AgentType = "codex"
	AgentTypeCursor     AgentType = "cursor"
	AgentTypeOpenAI     AgentType = "openai"
	AgentTypeCustom     AgentType = "custom"
)

// AgentPhase represents the lifecycle phase of an Agent.
type AgentPhase string

const (
	AgentPhaseCreated    AgentPhase = "Created"
	AgentPhaseStarting   AgentPhase = "Starting"
	AgentPhaseRunning    AgentPhase = "Running"
	AgentPhaseCompleting AgentPhase = "Completing"
	AgentPhaseCompleted  AgentPhase = "Completed"
	AgentPhaseFailed     AgentPhase = "Failed"
	AgentPhaseRetrying   AgentPhase = "Retrying"
	AgentPhaseTerminated AgentPhase = "Terminated"
	AgentPhaseStopped    AgentPhase = "Stopped"
)

// AgentSpec is the declarative configuration for an Agent.
type AgentSpec struct {
	APIVersion string        `yaml:"apiVersion" json:"apiVersion"`
	Kind       string        `yaml:"kind" json:"kind"`
	Metadata   Metadata      `yaml:"metadata" json:"metadata"`
	Spec       AgentSpecBody `yaml:"spec" json:"spec"`
}

// AgentSpecBody contains the spec fields for an AgentSpec.
type AgentSpecBody struct {
	Type     AgentType     `yaml:"type" json:"type"`
	Replicas int           `yaml:"replicas,omitempty" json:"replicas,omitempty"`
	Runtime  RuntimeConfig `yaml:"runtime,omitempty" json:"runtime,omitempty"`
	Resources ResourceConfig `yaml:"resources,omitempty" json:"resources,omitempty"`
	Context  ContextConfig `yaml:"context,omitempty" json:"context,omitempty"`
	HealthCheck HealthCheckConfig `yaml:"healthCheck,omitempty" json:"healthCheck,omitempty"`
	Recovery RecoveryConfig `yaml:"recovery,omitempty" json:"recovery,omitempty"`

	// Custom agent fields.
	Command  []string          `yaml:"command,omitempty" json:"command,omitempty"`
	Args     []string          `yaml:"args,omitempty" json:"args,omitempty"`
	Env      map[string]string `yaml:"env,omitempty" json:"env,omitempty"`
	Protocol ProtocolConfig    `yaml:"protocol,omitempty" json:"protocol,omitempty"`
}

// RuntimeConfig holds runtime settings.
type RuntimeConfig struct {
	Model     ModelConfig     `yaml:"model,omitempty" json:"model,omitempty"`
	Inference InferenceConfig `yaml:"inference,omitempty" json:"inference,omitempty"`
	Timeout   TimeoutConfig   `yaml:"timeout,omitempty" json:"timeout,omitempty"`
}

// ModelConfig holds model provider settings.
type ModelConfig struct {
	Provider string `yaml:"provider,omitempty" json:"provider,omitempty"`
	Name     string `yaml:"name,omitempty" json:"name,omitempty"`
	Fallback string `yaml:"fallback,omitempty" json:"fallback,omitempty"`
}

// InferenceConfig holds inference parameters.
type InferenceConfig struct {
	Thinking    string  `yaml:"thinking,omitempty" json:"thinking,omitempty"`
	Temperature float64 `yaml:"temperature,omitempty" json:"temperature,omitempty"`
	MaxTokens   int     `yaml:"maxTokens,omitempty" json:"maxTokens,omitempty"`
}

// TimeoutConfig holds timeout settings.
type TimeoutConfig struct {
	Task    string `yaml:"task,omitempty" json:"task,omitempty"`
	Idle    string `yaml:"idle,omitempty" json:"idle,omitempty"`
	Startup string `yaml:"startup,omitempty" json:"startup,omitempty"`
}

// ResourceConfig holds resource quota settings.
type ResourceConfig struct {
	TokenBudget TokenBudgetConfig `yaml:"tokenBudget,omitempty" json:"tokenBudget,omitempty"`
	CostLimit   CostLimitConfig   `yaml:"costLimit,omitempty" json:"costLimit,omitempty"`
	OnExceed    string            `yaml:"onExceed,omitempty" json:"onExceed,omitempty"`
}

// TokenBudgetConfig holds token budget limits.
type TokenBudgetConfig struct {
	PerTask int `yaml:"perTask,omitempty" json:"perTask,omitempty"`
	PerHour int `yaml:"perHour,omitempty" json:"perHour,omitempty"`
	PerDay  int `yaml:"perDay,omitempty" json:"perDay,omitempty"`
}

// CostLimitConfig holds cost limits.
type CostLimitConfig struct {
	PerTask  string `yaml:"perTask,omitempty" json:"perTask,omitempty"`
	PerDay   string `yaml:"perDay,omitempty" json:"perDay,omitempty"`
	PerMonth string `yaml:"perMonth,omitempty" json:"perMonth,omitempty"`
}

// ContextConfig holds context settings.
type ContextConfig struct {
	Workdir string   `yaml:"workdir,omitempty" json:"workdir,omitempty"`
	Skills  []string `yaml:"skills,omitempty" json:"skills,omitempty"`
}

// HealthCheckConfig holds health check settings.
type HealthCheckConfig struct {
	Type     string `yaml:"type,omitempty" json:"type,omitempty"`
	Interval string `yaml:"interval,omitempty" json:"interval,omitempty"`
	Timeout  string `yaml:"timeout,omitempty" json:"timeout,omitempty"`
	Retries  int    `yaml:"retries,omitempty" json:"retries,omitempty"`
}

// RecoveryConfig holds crash recovery settings.
type RecoveryConfig struct {
	Enabled      bool   `yaml:"enabled,omitempty" json:"enabled,omitempty"`
	MaxRestarts  int    `yaml:"maxRestarts,omitempty" json:"maxRestarts,omitempty"`
	RestartDelay string `yaml:"restartDelay,omitempty" json:"restartDelay,omitempty"`
	Backoff      string `yaml:"backoff,omitempty" json:"backoff,omitempty"`
}

// ProtocolConfig holds protocol settings for custom agents.
type ProtocolConfig struct {
	Type   string `yaml:"type,omitempty" json:"type,omitempty"`
	Format string `yaml:"format,omitempty" json:"format,omitempty"`
}

// TaskStatus represents the status of a task.
type TaskStatus string

const (
	TaskStatusPending   TaskStatus = "Pending"
	TaskStatusRunning   TaskStatus = "Running"
	TaskStatusCompleted TaskStatus = "Completed"
	TaskStatusFailed    TaskStatus = "Failed"
	TaskStatusCancelled TaskStatus = "Cancelled"
)

// TaskRecord represents a persisted task execution record.
type TaskRecord struct {
	ID        string     `json:"id"`
	AgentName string     `json:"agentName"`
	Message   string     `json:"message"`
	Status    TaskStatus `json:"status"`
	Result    string     `json:"result,omitempty"`
	Error     string     `json:"error,omitempty"`
	TokensIn  int        `json:"tokensIn,omitempty"`
	TokensOut int        `json:"tokensOut,omitempty"`
	Cost      float64    `json:"cost,omitempty"`
	CreatedAt time.Time  `json:"createdAt"`
	UpdatedAt time.Time  `json:"updatedAt"`
	StartedAt *time.Time `json:"startedAt,omitempty"`
	EndedAt   *time.Time `json:"endedAt,omitempty"`
}

// AgentRecord represents a persisted agent record.
type AgentRecord struct {
	Name      string     `json:"name"`
	Type      AgentType  `json:"type"`
	Phase     AgentPhase `json:"phase"`
	SpecYAML  string     `json:"specYaml"`
	Restarts  int        `json:"restarts"`
	Message   string     `json:"message,omitempty"`
	CreatedAt time.Time  `json:"createdAt"`
	UpdatedAt time.Time  `json:"updatedAt"`
}

// HealthStatus represents the health of an Agent.
type HealthStatus struct {
	Healthy bool   `json:"healthy"`
	Message string `json:"message,omitempty"`
}

// AgentMetrics contains runtime metrics for an Agent.
type AgentMetrics struct {
	TasksCompleted int     `json:"tasksCompleted"`
	TasksFailed    int     `json:"tasksFailed"`
	TasksRunning   int     `json:"tasksRunning"`
	TotalTokensIn  int     `json:"totalTokensIn"`
	TotalTokensOut int     `json:"totalTokensOut"`
	TotalCost      float64 `json:"totalCost"`
	UptimeSeconds  float64 `json:"uptimeSeconds"`
}

// Resource is a generic YAML resource for parsing kind.
type Resource struct {
	APIVersion string   `yaml:"apiVersion"`
	Kind       string   `yaml:"kind"`
	Metadata   Metadata `yaml:"metadata"`
}

// --- Goal/Project hierarchy types ---

// GoalSpec defines a strategic goal.
type GoalSpec struct {
	APIVersion string   `yaml:"apiVersion" json:"apiVersion"`
	Kind       string   `yaml:"kind" json:"kind"`
	Metadata   Metadata `yaml:"metadata" json:"metadata"`
	Spec       GoalBody `yaml:"spec" json:"spec"`
}

// GoalBody contains the spec fields for a Goal.
type GoalBody struct {
	Description     string            `yaml:"description" json:"description"`
	Owner           string            `yaml:"owner,omitempty" json:"owner,omitempty"`
	Deadline        string            `yaml:"deadline,omitempty" json:"deadline,omitempty"`
	SuccessCriteria []SuccessCriterion `yaml:"successCriteria,omitempty" json:"successCriteria,omitempty"`
	Budget          BudgetConfig      `yaml:"budget,omitempty" json:"budget,omitempty"`
}

// SuccessCriterion defines a measurable success metric.
type SuccessCriterion struct {
	Metric string `yaml:"metric" json:"metric"`
	Target int    `yaml:"target" json:"target"`
}

// BudgetConfig defines budget limits.
type BudgetConfig struct {
	Total string `yaml:"total,omitempty" json:"total,omitempty"`
	Alert string `yaml:"alert,omitempty" json:"alert,omitempty"`
}

// ProjectSpec defines a project under a goal.
type ProjectSpec struct {
	APIVersion string      `yaml:"apiVersion" json:"apiVersion"`
	Kind       string      `yaml:"kind" json:"kind"`
	Metadata   Metadata    `yaml:"metadata" json:"metadata"`
	Spec       ProjectBody `yaml:"spec" json:"spec"`
}

// ProjectBody contains the spec fields for a Project.
type ProjectBody struct {
	Description string   `yaml:"description" json:"description"`
	GoalRef     string   `yaml:"goalRef,omitempty" json:"goalRef,omitempty"`
	Agents      []string `yaml:"agents,omitempty" json:"agents,omitempty"`
}

// IssueSpec defines an execution unit under a task.
type IssueSpec struct {
	APIVersion string    `yaml:"apiVersion" json:"apiVersion"`
	Kind       string    `yaml:"kind" json:"kind"`
	Metadata   Metadata  `yaml:"metadata" json:"metadata"`
	Spec       IssueBody `yaml:"spec" json:"spec"`
}

// IssueBody contains the spec fields for an Issue.
type IssueBody struct {
	Description string `yaml:"description" json:"description"`
	TaskRef     string `yaml:"taskRef,omitempty" json:"taskRef,omitempty"`
	AgentRef    string `yaml:"agentRef,omitempty" json:"agentRef,omitempty"`
	Message     string `yaml:"message,omitempty" json:"message,omitempty"`
}

// --- Workflow types ---

// WorkflowSpec defines a multi-step workflow.
type WorkflowSpec struct {
	APIVersion string       `yaml:"apiVersion" json:"apiVersion"`
	Kind       string       `yaml:"kind" json:"kind"`
	Metadata   Metadata     `yaml:"metadata" json:"metadata"`
	Spec       WorkflowBody `yaml:"spec" json:"spec"`
}

// WorkflowBody contains the spec fields for a Workflow.
type WorkflowBody struct {
	Schedule string         `yaml:"schedule,omitempty" json:"schedule,omitempty"`
	Steps    []WorkflowStep `yaml:"steps" json:"steps"`
}

// WorkflowStep defines a single step in a workflow.
type WorkflowStep struct {
	Name      string           `yaml:"name" json:"name"`
	Agent     string           `yaml:"agent" json:"agent"`
	DependsOn []string         `yaml:"dependsOn,omitempty" json:"dependsOn,omitempty"`
	Input     WorkflowInput    `yaml:"input" json:"input"`
	Outputs   []WorkflowOutput `yaml:"outputs,omitempty" json:"outputs,omitempty"`
}

// WorkflowInput defines the input for a workflow step.
type WorkflowInput struct {
	Message string   `yaml:"message" json:"message"`
	Context []string `yaml:"context,omitempty" json:"context,omitempty"`
}

// WorkflowOutput defines a named output from a workflow step.
type WorkflowOutput struct {
	Name string `yaml:"name" json:"name"`
}

// WorkflowStatus represents the status of a workflow run.
type WorkflowStatus string

const (
	WorkflowStatusPending   WorkflowStatus = "Pending"
	WorkflowStatusRunning   WorkflowStatus = "Running"
	WorkflowStatusCompleted WorkflowStatus = "Completed"
	WorkflowStatusFailed    WorkflowStatus = "Failed"
)

// WorkflowRecord represents a persisted workflow definition.
type WorkflowRecord struct {
	Name      string `json:"name"`
	SpecYAML  string `json:"specYaml"`
	Schedule  string `json:"schedule,omitempty"`
	Enabled   bool   `json:"enabled"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

// WorkflowRunRecord represents a persisted workflow execution.
type WorkflowRunRecord struct {
	ID           string         `json:"id"`
	WorkflowName string         `json:"workflowName"`
	Status       WorkflowStatus `json:"status"`
	StepsJSON    string         `json:"stepsJson"`
	StartedAt    time.Time      `json:"startedAt"`
	EndedAt      *time.Time     `json:"endedAt,omitempty"`
}

// --- Dispatcher types ---

// DispatcherSpec defines the dispatcher configuration.
type DispatcherSpec struct {
	APIVersion string         `yaml:"apiVersion" json:"apiVersion"`
	Kind       string         `yaml:"kind" json:"kind"`
	Metadata   Metadata       `yaml:"metadata" json:"metadata"`
	Spec       DispatcherBody `yaml:"spec" json:"spec"`
}

// DispatcherBody contains the spec fields for a Dispatcher.
type DispatcherBody struct {
	Strategy string        `yaml:"strategy" json:"strategy"`
	Routing  []RoutingRule `yaml:"routing,omitempty" json:"routing,omitempty"`
	Fallback FallbackRule  `yaml:"fallback,omitempty" json:"fallback,omitempty"`
}

// RoutingRule defines how to match and route tasks.
type RoutingRule struct {
	Match      MatchCriteria `yaml:"match" json:"match"`
	Agents     []string      `yaml:"agents" json:"agents"`
	Preference string        `yaml:"preference,omitempty" json:"preference,omitempty"`
}

// MatchCriteria defines matching conditions.
type MatchCriteria struct {
	TaskType string            `yaml:"taskType,omitempty" json:"taskType,omitempty"`
	Labels   map[string]string `yaml:"labels,omitempty" json:"labels,omitempty"`
}

// FallbackRule defines the fallback agent.
type FallbackRule struct {
	Agent string `yaml:"agent" json:"agent"`
}
