package adapter

import (
	"context"

	v1 "github.com/zlc-ai/opc-platform/api/v1"
)

// Adapter is the unified interface for all Agent types.
type Adapter interface {
	// Type returns the agent type this adapter handles.
	Type() v1.AgentType

	// Start launches the agent process.
	Start(ctx context.Context, spec v1.AgentSpec) error

	// Stop gracefully shuts down the agent.
	Stop(ctx context.Context) error

	// Health returns the current health status.
	Health() v1.HealthStatus

	// Execute runs a task synchronously and returns the result.
	Execute(ctx context.Context, task v1.TaskRecord) (ExecuteResult, error)

	// Stream runs a task and returns a channel of output chunks.
	Stream(ctx context.Context, task v1.TaskRecord) (<-chan Chunk, error)

	// Status returns the current agent status.
	Status() v1.AgentPhase

	// Metrics returns runtime metrics.
	Metrics() v1.AgentMetrics
}

// ExecuteResult contains the output of a task execution.
type ExecuteResult struct {
	Output    string `json:"output"`
	TokensIn  int    `json:"tokensIn"`
	TokensOut int    `json:"tokensOut"`
}

// Chunk represents a streaming output chunk.
type Chunk struct {
	Content string `json:"content"`
	Done    bool   `json:"done"`
	Error   error  `json:"-"`
}

// Registry maps agent types to adapter factories.
type Registry struct {
	factories map[v1.AgentType]Factory
}

// Factory creates a new Adapter instance.
type Factory func() Adapter

// NewRegistry creates a new adapter registry.
func NewRegistry() *Registry {
	return &Registry{factories: make(map[v1.AgentType]Factory)}
}

// Register adds an adapter factory for a given agent type.
func (r *Registry) Register(agentType v1.AgentType, factory Factory) {
	r.factories[agentType] = factory
}

// Create returns a new Adapter for the given agent type.
func (r *Registry) Create(agentType v1.AgentType) (Adapter, error) {
	factory, ok := r.factories[agentType]
	if !ok {
		return nil, &UnsupportedAgentTypeError{Type: agentType}
	}
	return factory(), nil
}

// UnsupportedAgentTypeError is returned when an unknown agent type is requested.
type UnsupportedAgentTypeError struct {
	Type v1.AgentType
}

func (e *UnsupportedAgentTypeError) Error() string {
	return "unsupported agent type: " + string(e.Type)
}
