package cluster

import (
	"context"
	"fmt"
	"sync"
	"time"

	"go.uber.org/zap"
)

const (
	defaultHeartbeatInterval = 10 * time.Second
	defaultHeartbeatTimeout  = 30 * time.Second
)

// Manager manages the cluster state and node membership.
type Manager struct {
	mu sync.RWMutex

	localNode *Node
	nodes     map[string]*Node
	logger    *zap.SugaredLogger

	heartbeatInterval time.Duration
	heartbeatTimeout  time.Duration

	discovery Discovery
	cancel    context.CancelFunc
}

// NewManager creates a new cluster manager.
func NewManager(logger *zap.SugaredLogger) *Manager {
	return &Manager{
		nodes:             make(map[string]*Node),
		logger:            logger,
		heartbeatInterval: defaultHeartbeatInterval,
		heartbeatTimeout:  defaultHeartbeatTimeout,
	}
}

// Init initializes this node as a master node and starts the cluster.
func (m *Manager) Init(nodeID, listenAddr string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.localNode != nil {
		return fmt.Errorf("node already initialized")
	}

	node := NewNode(nodeID, listenAddr, NodeRoleMaster)
	m.localNode = node
	m.nodes[nodeID] = node

	m.logger.Infow("cluster initialized as master",
		"nodeId", nodeID,
		"address", listenAddr,
	)

	return nil
}

// Join joins an existing cluster by contacting the master node.
func (m *Manager) Join(nodeID, localAddr, masterAddr string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.localNode != nil {
		return fmt.Errorf("node already initialized")
	}

	node := NewNode(nodeID, localAddr, NodeRoleWorker)
	m.localNode = node
	m.nodes[nodeID] = node

	if m.discovery != nil {
		if err := m.discovery.Register(node); err != nil {
			return fmt.Errorf("register with master: %w", err)
		}
	}

	m.logger.Infow("joined cluster",
		"nodeId", nodeID,
		"localAddr", localAddr,
		"masterAddr", masterAddr,
	)

	return nil
}

// Leave gracefully removes the local node from the cluster.
func (m *Manager) Leave() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.localNode == nil {
		return fmt.Errorf("node not initialized")
	}

	m.localNode.SetStatus(NodeStatusLeaving)

	if m.discovery != nil {
		if err := m.discovery.Deregister(m.localNode.NodeID); err != nil {
			m.logger.Warnw("failed to deregister from cluster", "error", err)
		}
	}

	if m.cancel != nil {
		m.cancel()
	}

	nodeID := m.localNode.NodeID
	delete(m.nodes, nodeID)
	m.localNode = nil

	m.logger.Infow("left cluster", "nodeId", nodeID)

	return nil
}

// AddNode registers a remote node in the local node table.
func (m *Manager) AddNode(node *Node) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.nodes[node.NodeID] = node
	m.logger.Infow("node added", "nodeId", node.NodeID, "address", node.Address)
}

// RemoveNode removes a node from the local node table.
func (m *Manager) RemoveNode(nodeID string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	delete(m.nodes, nodeID)
	m.logger.Infow("node removed", "nodeId", nodeID)
}

// ListNodes returns a snapshot of all known nodes.
func (m *Manager) ListNodes() []Node {
	m.mu.RLock()
	defer m.mu.RUnlock()

	nodes := make([]Node, 0, len(m.nodes))
	for _, n := range m.nodes {
		nodes = append(nodes, n.Snapshot())
	}
	return nodes
}

// GetNode returns a snapshot of a specific node.
func (m *Manager) GetNode(nodeID string) (Node, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	n, ok := m.nodes[nodeID]
	if !ok {
		return Node{}, false
	}
	return n.Snapshot(), true
}

// LocalNode returns the local node's snapshot.
func (m *Manager) LocalNode() (Node, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.localNode == nil {
		return Node{}, false
	}
	return m.localNode.Snapshot(), true
}

// SetDiscovery sets the discovery mechanism used for node registration.
func (m *Manager) SetDiscovery(d Discovery) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.discovery = d
}

// StartHeartbeat starts a background goroutine that periodically sends heartbeats
// and marks unresponsive nodes as NotReady.
func (m *Manager) StartHeartbeat(ctx context.Context) {
	ctx, m.cancel = context.WithCancel(ctx)

	go func() {
		ticker := time.NewTicker(m.heartbeatInterval)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				m.checkHeartbeats()
			}
		}
	}()
}

// checkHeartbeats marks nodes that have missed their heartbeat deadline.
func (m *Manager) checkHeartbeats() {
	m.mu.RLock()
	defer m.mu.RUnlock()

	for _, n := range m.nodes {
		if n == m.localNode {
			n.UpdateHeartbeat(n.CPUUsage, n.MemoryUsage, n.AgentCount)
			continue
		}
		if !n.IsHealthy(m.heartbeatTimeout) {
			n.SetStatus(NodeStatusNotReady)
			m.logger.Warnw("node heartbeat timeout",
				"nodeId", n.NodeID,
				"lastHeartbeat", n.LastHeartbeat,
			)
		}
	}
}

// NodeCount returns the total number of known nodes.
func (m *Manager) NodeCount() int {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return len(m.nodes)
}

// IsMaster returns true if the local node is a master.
func (m *Manager) IsMaster() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.localNode == nil {
		return false
	}
	return m.localNode.Role == NodeRoleMaster
}
