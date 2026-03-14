package federation

import (
	"sync"
	"time"

	"go.uber.org/zap"
)

const (
	defaultHeartbeatInterval = 30 * time.Second
	heartbeatTimeout         = 10 * time.Second
)

// HeartbeatMonitor periodically checks the status of federated companies.
type HeartbeatMonitor struct {
	mu         sync.RWMutex
	controller *FederationController
	interval   time.Duration
	stopCh     chan struct{}
	running    bool
	logger     *zap.SugaredLogger

	// pendingIssues tracks issues waiting for human intervention.
	pendingIssues []PendingIssue
}

// PendingIssue represents an issue that is blocked and awaiting action.
type PendingIssue struct {
	IssueID   string    `json:"issueId"`
	CompanyID string    `json:"companyId"`
	Reason    string    `json:"reason"`
	Since     time.Time `json:"since"`
}

// NewHeartbeatMonitor creates a new HeartbeatMonitor.
func NewHeartbeatMonitor(controller *FederationController, logger *zap.SugaredLogger) *HeartbeatMonitor {
	return &HeartbeatMonitor{
		controller:    controller,
		interval:      defaultHeartbeatInterval,
		stopCh:        make(chan struct{}),
		logger:        logger,
		pendingIssues: make([]PendingIssue, 0),
	}
}

// Start begins the heartbeat monitoring loop.
func (hm *HeartbeatMonitor) Start() {
	hm.mu.Lock()
	if hm.running {
		hm.mu.Unlock()
		return
	}
	hm.running = true
	hm.mu.Unlock()

	go hm.loop()
	hm.logger.Infow("heartbeat monitor started", "interval", hm.interval)
}

// Stop halts the heartbeat monitoring loop.
func (hm *HeartbeatMonitor) Stop() {
	hm.mu.Lock()
	defer hm.mu.Unlock()

	if !hm.running {
		return
	}

	close(hm.stopCh)
	hm.running = false
	hm.logger.Infow("heartbeat monitor stopped")
}

// AddPendingIssue adds an issue to the pending queue.
func (hm *HeartbeatMonitor) AddPendingIssue(issue PendingIssue) {
	hm.mu.Lock()
	defer hm.mu.Unlock()

	if issue.Since.IsZero() {
		issue.Since = time.Now().UTC()
	}
	hm.pendingIssues = append(hm.pendingIssues, issue)
	hm.logger.Infow("pending issue added", "issueId", issue.IssueID, "companyId", issue.CompanyID)
}

// RemovePendingIssue removes an issue from the pending queue.
func (hm *HeartbeatMonitor) RemovePendingIssue(issueID string) {
	hm.mu.Lock()
	defer hm.mu.Unlock()

	for i, issue := range hm.pendingIssues {
		if issue.IssueID == issueID {
			hm.pendingIssues = append(hm.pendingIssues[:i], hm.pendingIssues[i+1:]...)
			return
		}
	}
}

// ListPendingIssues returns all pending issues.
func (hm *HeartbeatMonitor) ListPendingIssues() []PendingIssue {
	hm.mu.RLock()
	defer hm.mu.RUnlock()

	result := make([]PendingIssue, len(hm.pendingIssues))
	copy(result, hm.pendingIssues)
	return result
}

func (hm *HeartbeatMonitor) loop() {
	ticker := time.NewTicker(hm.interval)
	defer ticker.Stop()

	for {
		select {
		case <-hm.stopCh:
			return
		case <-ticker.C:
			hm.checkAll()
		}
	}
}

func (hm *HeartbeatMonitor) checkAll() {
	companies := hm.controller.ListCompanies()
	transport := hm.controller.Transport()

	for _, company := range companies {
		if err := transport.Ping(company.Endpoint); err != nil {
			hm.logger.Warnw("company heartbeat failed",
				"company", company.Name,
				"endpoint", company.Endpoint,
				"error", err,
			)
			if err := hm.controller.UpdateCompanyStatus(company.ID, CompanyStatusOffline); err != nil {
				hm.logger.Errorw("failed to update company status", "company", company.Name, "error", err)
			}
		} else {
			if company.Status != CompanyStatusOnline {
				if err := hm.controller.UpdateCompanyStatus(company.ID, CompanyStatusOnline); err != nil {
					hm.logger.Errorw("failed to update company status", "company", company.Name, "error", err)
				}
				hm.logger.Infow("company came online", "company", company.Name)
			}
		}
	}
}
