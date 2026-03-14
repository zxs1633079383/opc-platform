package cluster

import (
	"sync"
	"time"
)

// NodeStatus represents the current state of a cluster node.
type NodeStatus string

const (
	NodeStatusReady    NodeStatus = "Ready"
	NodeStatusNotReady NodeStatus = "NotReady"
	NodeStatusLeaving  NodeStatus = "Leaving"
)

// NodeRole represents the role of a node in the cluster.
type NodeRole string

const (
	NodeRoleMaster NodeRole = "master"
	NodeRoleWorker NodeRole = "worker"
)

// Node represents a single node in the OPC cluster.
type Node struct {
	mu sync.RWMutex

	NodeID        string     `json:"nodeId" yaml:"nodeId"`
	Address       string     `json:"address" yaml:"address"`
	Role          NodeRole   `json:"role" yaml:"role"`
	Status        NodeStatus `json:"status" yaml:"status"`
	AgentCount    int        `json:"agentCount" yaml:"agentCount"`
	CPUUsage      float64    `json:"cpuUsage" yaml:"cpuUsage"`
	MemoryUsage   float64    `json:"memoryUsage" yaml:"memoryUsage"`
	LastHeartbeat time.Time  `json:"lastHeartbeat" yaml:"lastHeartbeat"`
	JoinedAt      time.Time  `json:"joinedAt" yaml:"joinedAt"`
}

// NewNode creates a new Node with the given ID and address.
func NewNode(id, address string, role NodeRole) *Node {
	now := time.Now()
	return &Node{
		NodeID:        id,
		Address:       address,
		Role:          role,
		Status:        NodeStatusReady,
		LastHeartbeat: now,
		JoinedAt:      now,
	}
}

// UpdateHeartbeat refreshes the node's heartbeat timestamp and resource usage.
func (n *Node) UpdateHeartbeat(cpuUsage, memoryUsage float64, agentCount int) {
	n.mu.Lock()
	defer n.mu.Unlock()

	n.LastHeartbeat = time.Now()
	n.CPUUsage = cpuUsage
	n.MemoryUsage = memoryUsage
	n.AgentCount = agentCount
	n.Status = NodeStatusReady
}

// IsHealthy returns true if the node's last heartbeat is within the given timeout.
func (n *Node) IsHealthy(timeout time.Duration) bool {
	n.mu.RLock()
	defer n.mu.RUnlock()

	return time.Since(n.LastHeartbeat) < timeout
}

// SetStatus updates the node status.
func (n *Node) SetStatus(status NodeStatus) {
	n.mu.Lock()
	defer n.mu.Unlock()

	n.Status = status
}

// Snapshot returns a copy of the node's current state (safe for concurrent reads).
func (n *Node) Snapshot() Node {
	n.mu.RLock()
	defer n.mu.RUnlock()

	return Node{
		NodeID:        n.NodeID,
		Address:       n.Address,
		Role:          n.Role,
		Status:        n.Status,
		AgentCount:    n.AgentCount,
		CPUUsage:      n.CPUUsage,
		MemoryUsage:   n.MemoryUsage,
		LastHeartbeat: n.LastHeartbeat,
		JoinedAt:      n.JoinedAt,
	}
}
