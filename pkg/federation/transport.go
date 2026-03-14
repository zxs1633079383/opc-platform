package federation

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"go.uber.org/zap"
)

// Transport defines the interface for inter-company communication.
type Transport interface {
	// Send sends a request to a company endpoint and returns the response.
	Send(endpoint string, method string, path string, body any) ([]byte, error)

	// Ping checks if a company endpoint is reachable.
	Ping(endpoint string) error

	// FetchStatus retrieves the status of a remote company.
	FetchStatus(endpoint string) (*CompanyStatusReport, error)
}

// CompanyStatusReport is the status response from a remote company.
type CompanyStatusReport struct {
	CompanyID   string   `json:"companyId"`
	CompanyName string   `json:"companyName"`
	Status      string   `json:"status"`
	AgentCount  int      `json:"agentCount"`
	Agents      []string `json:"agents,omitempty"`
}

// HTTPTransport implements Transport using HTTP RPC.
type HTTPTransport struct {
	client *http.Client
	logger *zap.SugaredLogger
}

// NewHTTPTransport creates a new HTTPTransport.
func NewHTTPTransport(logger *zap.SugaredLogger) *HTTPTransport {
	return &HTTPTransport{
		client: &http.Client{
			Timeout: 10 * time.Second,
		},
		logger: logger,
	}
}

// Send sends an HTTP request to the given endpoint.
func (t *HTTPTransport) Send(endpoint, method, path string, body any) ([]byte, error) {
	url := fmt.Sprintf("%s%s", endpoint, path)

	var reqBody io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("marshal request body: %w", err)
		}
		reqBody = bytes.NewReader(data)
	}

	req, err := http.NewRequest(method, url, reqBody)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := t.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("send request to %s: %w", url, err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("request to %s returned %d: %s", url, resp.StatusCode, string(respBody))
	}

	return respBody, nil
}

// Ping checks if a company endpoint is reachable.
func (t *HTTPTransport) Ping(endpoint string) error {
	url := fmt.Sprintf("%s/healthz", endpoint)

	resp, err := t.client.Get(url)
	if err != nil {
		return fmt.Errorf("ping %s: %w", endpoint, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("ping %s returned %d", endpoint, resp.StatusCode)
	}

	return nil
}

// FetchStatus retrieves the status of a remote company.
func (t *HTTPTransport) FetchStatus(endpoint string) (*CompanyStatusReport, error) {
	data, err := t.Send(endpoint, http.MethodGet, "/api/v1/status", nil)
	if err != nil {
		return nil, fmt.Errorf("fetch status: %w", err)
	}

	var report CompanyStatusReport
	if err := json.Unmarshal(data, &report); err != nil {
		return nil, fmt.Errorf("unmarshal status: %w", err)
	}

	return &report, nil
}
