package cluster

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"go.uber.org/zap"
)

// Discovery defines the interface for node discovery mechanisms.
type Discovery interface {
	Register(node *Node) error
	Deregister(nodeID string) error
	Discover() ([]*Node, error)
}

// StaticDiscovery uses a pre-configured list of node addresses.
type StaticDiscovery struct {
	mu    sync.RWMutex
	nodes []*Node
}

// NewStaticDiscovery creates a discovery instance from a static list of addresses.
func NewStaticDiscovery(addresses []string) *StaticDiscovery {
	nodes := make([]*Node, 0, len(addresses))
	for i, addr := range addresses {
		nodes = append(nodes, NewNode(
			fmt.Sprintf("static-%d", i),
			addr,
			NodeRoleWorker,
		))
	}
	return &StaticDiscovery{nodes: nodes}
}

// Register adds a node to the static list.
func (s *StaticDiscovery) Register(node *Node) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, n := range s.nodes {
		if n.NodeID == node.NodeID {
			return nil // already registered
		}
	}
	s.nodes = append(s.nodes, node)
	return nil
}

// Deregister removes a node from the static list.
func (s *StaticDiscovery) Deregister(nodeID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	for i, n := range s.nodes {
		if n.NodeID == nodeID {
			s.nodes = append(s.nodes[:i], s.nodes[i+1:]...)
			return nil
		}
	}
	return nil
}

// Discover returns the current list of known nodes.
func (s *StaticDiscovery) Discover() ([]*Node, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]*Node, len(s.nodes))
	copy(result, s.nodes)
	return result, nil
}

// HTTPDiscovery registers and discovers nodes via the master's HTTP API.
type HTTPDiscovery struct {
	masterAddr string
	client     *http.Client
	logger     *zap.SugaredLogger
}

// NewHTTPDiscovery creates a discovery instance that uses HTTP API registration.
func NewHTTPDiscovery(masterAddr string, logger *zap.SugaredLogger) *HTTPDiscovery {
	return &HTTPDiscovery{
		masterAddr: masterAddr,
		client: &http.Client{
			Timeout: 5 * time.Second,
		},
		logger: logger,
	}
}

// registerRequest is the JSON body sent to the master for registration.
type registerRequest struct {
	NodeID  string `json:"nodeId"`
	Address string `json:"address"`
}

// Register sends a registration request to the master node.
func (h *HTTPDiscovery) Register(node *Node) error {
	body, err := json.Marshal(registerRequest{
		NodeID:  node.NodeID,
		Address: node.Address,
	})
	if err != nil {
		return fmt.Errorf("marshal register request: %w", err)
	}

	url := fmt.Sprintf("%s/api/v1/cluster/nodes", h.masterAddr)
	resp, err := h.client.Post(url, "application/json", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("register with master: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return fmt.Errorf("register failed: status %d", resp.StatusCode)
	}

	return nil
}

// Deregister sends a deregistration request to the master node.
func (h *HTTPDiscovery) Deregister(nodeID string) error {
	url := fmt.Sprintf("%s/api/v1/cluster/nodes/%s", h.masterAddr, nodeID)
	req, err := http.NewRequest(http.MethodDelete, url, nil)
	if err != nil {
		return fmt.Errorf("create deregister request: %w", err)
	}

	resp, err := h.client.Do(req)
	if err != nil {
		return fmt.Errorf("deregister from master: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		return fmt.Errorf("deregister failed: status %d", resp.StatusCode)
	}

	return nil
}

// Discover fetches the list of nodes from the master.
func (h *HTTPDiscovery) Discover() ([]*Node, error) {
	url := fmt.Sprintf("%s/api/v1/cluster/nodes", h.masterAddr)
	resp, err := h.client.Get(url)
	if err != nil {
		return nil, fmt.Errorf("discover nodes: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("discover failed: status %d", resp.StatusCode)
	}

	var nodes []*Node
	if err := json.NewDecoder(resp.Body).Decode(&nodes); err != nil {
		return nil, fmt.Errorf("decode nodes: %w", err)
	}

	return nodes, nil
}
