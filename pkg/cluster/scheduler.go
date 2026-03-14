package cluster

import (
	"fmt"
	"sync"
)

// AffinityRule defines a preference for scheduling a specific agent type to certain nodes.
type AffinityRule struct {
	AgentType     string   `json:"agentType" yaml:"agentType"`
	PreferredNodes []string `json:"preferredNodes" yaml:"preferredNodes"`
}

// Scheduler selects the best node for deploying an agent based on load and affinity rules.
type Scheduler struct {
	mu       sync.RWMutex
	manager  *Manager
	affinity map[string][]string // agentType -> preferred nodeIDs
}

// NewScheduler creates a new Scheduler backed by the given cluster Manager.
func NewScheduler(manager *Manager) *Scheduler {
	return &Scheduler{
		manager:  manager,
		affinity: make(map[string][]string),
	}
}

// AddAffinity registers a preference for scheduling agentType to the given node IDs.
func (s *Scheduler) AddAffinity(agentType string, nodeIDs []string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.affinity[agentType] = nodeIDs
}

// RemoveAffinity removes affinity rules for the given agent type.
func (s *Scheduler) RemoveAffinity(agentType string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	delete(s.affinity, agentType)
}

// SelectNode picks the best node for the given agent type.
// It first checks affinity rules, then falls back to least-loaded scheduling.
func (s *Scheduler) SelectNode(agentType string) (*Node, error) {
	nodes := s.manager.ListNodes()

	ready := filterReady(nodes)
	if len(ready) == 0 {
		return nil, fmt.Errorf("no ready nodes available")
	}

	s.mu.RLock()
	preferred, hasAffinity := s.affinity[agentType]
	s.mu.RUnlock()

	// Try affinity nodes first.
	if hasAffinity {
		affinityNodes := filterByIDs(ready, preferred)
		if len(affinityNodes) > 0 {
			selected := leastLoaded(affinityNodes)
			return &selected, nil
		}
		// All preferred nodes are unavailable; fall through to general pool.
	}

	selected := leastLoaded(ready)
	return &selected, nil
}

// filterReady returns only nodes with status Ready.
func filterReady(nodes []Node) []Node {
	result := make([]Node, 0, len(nodes))
	for _, n := range nodes {
		if n.Status == NodeStatusReady {
			result = append(result, n)
		}
	}
	return result
}

// filterByIDs returns nodes whose NodeID is in the given set.
func filterByIDs(nodes []Node, ids []string) []Node {
	idSet := make(map[string]struct{}, len(ids))
	for _, id := range ids {
		idSet[id] = struct{}{}
	}

	result := make([]Node, 0, len(ids))
	for _, n := range nodes {
		if _, ok := idSet[n.NodeID]; ok {
			result = append(result, n)
		}
	}
	return result
}

// leastLoaded returns the node with the fewest running agents.
// Caller must ensure nodes is non-empty.
func leastLoaded(nodes []Node) Node {
	best := nodes[0]
	for _, n := range nodes[1:] {
		if n.AgentCount < best.AgentCount {
			best = n
		}
	}
	return best
}
