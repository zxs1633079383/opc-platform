package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	v1 "github.com/zlc-ai/opc-platform/api/v1"
)

// DefaultDaemonAddr is the default address for the opctl daemon.
const DefaultDaemonAddr = "http://localhost:9527"

// Client communicates with the opctl daemon over HTTP.
type Client struct {
	baseURL    string
	httpClient *http.Client
}

// New creates a new daemon client.
func New(baseURL string) *Client {
	return &Client{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// apiError represents an error response from the daemon.
type apiError struct {
	Error string `json:"error"`
}

// runTaskRequest is the request body for RunTask.
type runTaskRequest struct {
	Agent   string `json:"agent"`
	Message string `json:"message"`
}

// runTaskResponse is the response body from RunTask.
type runTaskResponse struct {
	TaskID    string `json:"taskId"`
	Output    string `json:"output"`
	TokensIn  int    `json:"tokensIn"`
	TokensOut int    `json:"tokensOut"`
}

// applyResponse is the response body from Apply.
type applyResponse struct {
	Message string `json:"message"`
}

// doRequest builds and executes an HTTP request, returning the raw response.
func (c *Client) doRequest(ctx context.Context, method, path string, body io.Reader, contentType string) (*http.Response, error) {
	url := c.baseURL + path

	req, err := http.NewRequestWithContext(ctx, method, url, body)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}

	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("sending request to daemon: %w", err)
	}

	return resp, nil
}

// checkResponse reads the response body and checks for error status codes.
// Returns the raw body bytes, or an error if the status code indicates failure.
func checkResponse(resp *http.Response) ([]byte, error) {
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading response body: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		var apiErr apiError
		if json.Unmarshal(respBody, &apiErr) == nil && apiErr.Error != "" {
			return nil, fmt.Errorf("daemon error (HTTP %d): %s", resp.StatusCode, apiErr.Error)
		}
		return nil, fmt.Errorf("daemon error (HTTP %d): %s", resp.StatusCode, string(respBody))
	}

	return respBody, nil
}

// doJSON executes an HTTP request with optional JSON body and decodes the JSON response.
// If body is non-nil, it is marshalled to JSON and sent as the request body.
// If result is non-nil, the response body is decoded into it.
func (c *Client) doJSON(ctx context.Context, method, path string, body interface{}, result interface{}) error {
	var bodyReader io.Reader
	var contentType string

	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("marshalling request body: %w", err)
		}
		bodyReader = bytes.NewReader(data)
		contentType = "application/json"
	}

	resp, err := c.doRequest(ctx, method, path, bodyReader, contentType)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	respBody, err := checkResponse(resp)
	if err != nil {
		return err
	}

	if result != nil && len(respBody) > 0 {
		if err := json.Unmarshal(respBody, result); err != nil {
			return fmt.Errorf("decoding response: %w", err)
		}
	}

	return nil
}

// Ping checks if the daemon is reachable.
func (c *Client) Ping() error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	resp, err := c.doRequest(ctx, http.MethodGet, "/api/health", nil, "")
	if err != nil {
		return fmt.Errorf("daemon is not reachable: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("daemon returned status %d", resp.StatusCode)
	}

	return nil
}

// Apply sends raw YAML to the daemon for processing.
// Returns the response message (e.g. "agent/coder configured").
func (c *Client) Apply(ctx context.Context, yamlData []byte) (string, error) {
	resp, err := c.doRequest(ctx, http.MethodPost, "/api/apply", bytes.NewReader(yamlData), "application/x-yaml")
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	respBody, err := checkResponse(resp)
	if err != nil {
		return "", err
	}

	var result applyResponse
	if err := json.Unmarshal(respBody, &result); err != nil {
		// If the response isn't JSON, return it as-is.
		return string(respBody), nil
	}

	return result.Message, nil
}

// ListAgents returns all agents.
func (c *Client) ListAgents(ctx context.Context) ([]v1.AgentRecord, error) {
	var agents []v1.AgentRecord
	if err := c.doJSON(ctx, http.MethodGet, "/api/agents", nil, &agents); err != nil {
		return nil, err
	}
	return agents, nil
}

// GetAgent returns a single agent by name.
func (c *Client) GetAgent(ctx context.Context, name string) (v1.AgentRecord, error) {
	var agent v1.AgentRecord
	if err := c.doJSON(ctx, http.MethodGet, "/api/agents/"+name, nil, &agent); err != nil {
		return v1.AgentRecord{}, err
	}
	return agent, nil
}

// DeleteAgent deletes an agent by name.
func (c *Client) DeleteAgent(ctx context.Context, name string) error {
	return c.doJSON(ctx, http.MethodDelete, "/api/agents/"+name, nil, nil)
}

// StartAgent starts an agent by name.
func (c *Client) StartAgent(ctx context.Context, name string) error {
	return c.doJSON(ctx, http.MethodPost, "/api/agents/"+name+"/start", nil, nil)
}

// StopAgent stops an agent by name.
func (c *Client) StopAgent(ctx context.Context, name string) error {
	return c.doJSON(ctx, http.MethodPost, "/api/agents/"+name+"/stop", nil, nil)
}

// RestartAgent restarts an agent by name.
func (c *Client) RestartAgent(ctx context.Context, name string) error {
	return c.doJSON(ctx, http.MethodPost, "/api/agents/"+name+"/restart", nil, nil)
}

// RunTask executes a task on the specified agent. Returns the task ID and output.
func (c *Client) RunTask(ctx context.Context, agentName, message string) (taskID string, output string, err error) {
	reqBody := runTaskRequest{
		Agent:   agentName,
		Message: message,
	}

	var result runTaskResponse
	if err := c.doJSON(ctx, http.MethodPost, "/api/run", reqBody, &result); err != nil {
		return "", "", err
	}

	return result.TaskID, result.Output, nil
}

// ListTasks returns all tasks.
func (c *Client) ListTasks(ctx context.Context) ([]v1.TaskRecord, error) {
	var tasks []v1.TaskRecord
	if err := c.doJSON(ctx, http.MethodGet, "/api/tasks", nil, &tasks); err != nil {
		return nil, err
	}
	return tasks, nil
}

// GetTask returns a single task by ID.
func (c *Client) GetTask(ctx context.Context, id string) (v1.TaskRecord, error) {
	var task v1.TaskRecord
	if err := c.doJSON(ctx, http.MethodGet, "/api/tasks/"+id, nil, &task); err != nil {
		return v1.TaskRecord{}, err
	}
	return task, nil
}

// ListWorkflows returns all workflows.
func (c *Client) ListWorkflows(ctx context.Context) ([]v1.WorkflowRecord, error) {
	var workflows []v1.WorkflowRecord
	if err := c.doJSON(ctx, http.MethodGet, "/api/workflows", nil, &workflows); err != nil {
		return nil, err
	}
	return workflows, nil
}

// DeleteWorkflow deletes a workflow by name.
func (c *Client) DeleteWorkflow(ctx context.Context, name string) error {
	return c.doJSON(ctx, http.MethodDelete, "/api/workflows/"+name, nil, nil)
}

// Status returns the cluster status summary.
func (c *Client) Status(ctx context.Context) (map[string]interface{}, error) {
	var status map[string]interface{}
	if err := c.doJSON(ctx, http.MethodGet, "/api/status", nil, &status); err != nil {
		return nil, err
	}
	return status, nil
}

// AgentMetrics returns metrics for all agents.
func (c *Client) AgentMetrics(ctx context.Context) (map[string]v1.AgentMetrics, error) {
	var metrics map[string]v1.AgentMetrics
	if err := c.doJSON(ctx, http.MethodGet, "/api/metrics/agents", nil, &metrics); err != nil {
		return nil, err
	}
	return metrics, nil
}

// Health returns the health status for all agents.
func (c *Client) Health(ctx context.Context) (map[string]v1.HealthStatus, error) {
	var health map[string]v1.HealthStatus
	if err := c.doJSON(ctx, http.MethodGet, "/api/health", nil, &health); err != nil {
		return nil, err
	}
	return health, nil
}
