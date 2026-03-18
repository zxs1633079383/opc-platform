package federation

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/zlc-ai/opc-platform/internal/config"
	"go.uber.org/zap"
)

const federationFileName = "federation.json"

// FederationController manages the federation of OPC companies.
type FederationController struct {
	mu        sync.RWMutex
	companies map[string]*Company
	transport Transport
	logger    *zap.SugaredLogger
	stateDir  string
}

// NewControllerForTest creates a FederationController with a custom state dir
// and transport. Intended for testing only.
func NewControllerForTest(stateDir string, transport Transport, logger *zap.SugaredLogger) *FederationController {
	return &FederationController{
		companies: make(map[string]*Company),
		transport: transport,
		logger:    logger,
		stateDir:  stateDir,
	}
}

// NewController creates a new FederationController.
func NewController(logger *zap.SugaredLogger) *FederationController {
	stateDir := filepath.Join(config.GetStateDir(), "federation")
	if err := os.MkdirAll(stateDir, 0o755); err != nil {
		logger.Warnw("failed to create federation state dir", "error", err)
	}

	fc := &FederationController{
		companies: make(map[string]*Company),
		transport: NewHTTPTransport(logger),
		logger:    logger,
		stateDir:  stateDir,
	}

	if err := fc.loadState(); err != nil {
		logger.Warnw("failed to load federation state", "error", err)
	}

	return fc
}

// RegisterCompany adds a new company to the federation.
func (fc *FederationController) RegisterCompany(reg CompanyRegistration) (*Company, error) {
	fc.mu.Lock()
	defer fc.mu.Unlock()

	// Check for duplicate name.
	for _, c := range fc.companies {
		if c.Name == reg.Name {
			return nil, fmt.Errorf("company %q already registered", reg.Name)
		}
	}

	// Probe endpoint to set initial status.
	initialStatus := CompanyStatusOffline
	if err := fc.transport.Ping(reg.Endpoint); err == nil {
		initialStatus = CompanyStatusOnline
	}

	company := &Company{
		ID:           uuid.New().String()[:8],
		Name:         reg.Name,
		Endpoint:     reg.Endpoint,
		DashboardURL: reg.DashboardURL,
		Type:         CompanyType(reg.Type),
		Status:       initialStatus,
		Agents:       reg.Agents,
		APIKey:       GenerateAPIKey(),
		JoinedAt:     time.Now().UTC(),
	}

	fc.companies[company.ID] = company

	if err := fc.saveState(); err != nil {
		return nil, fmt.Errorf("persist federation state: %w", err)
	}

	fc.logger.Infow("company registered",
		"id", company.ID,
		"name", company.Name,
		"endpoint", company.Endpoint,
		"type", company.Type,
	)

	return company, nil
}

// UnregisterCompany removes a company from the federation.
func (fc *FederationController) UnregisterCompany(id string) error {
	fc.mu.Lock()
	defer fc.mu.Unlock()

	if _, ok := fc.companies[id]; !ok {
		return fmt.Errorf("company %q not found", id)
	}

	delete(fc.companies, id)

	if err := fc.saveState(); err != nil {
		return fmt.Errorf("persist federation state: %w", err)
	}

	fc.logger.Infow("company unregistered", "id", id)
	return nil
}

// GetCompany returns a company by ID.
func (fc *FederationController) GetCompany(id string) (*Company, error) {
	fc.mu.RLock()
	defer fc.mu.RUnlock()

	c, ok := fc.companies[id]
	if !ok {
		return nil, fmt.Errorf("company %q not found", id)
	}
	return c, nil
}

// FindCompanyByName returns a company by name.
func (fc *FederationController) FindCompanyByName(name string) (*Company, error) {
	fc.mu.RLock()
	defer fc.mu.RUnlock()

	for _, c := range fc.companies {
		if c.Name == name {
			return c, nil
		}
	}
	return nil, fmt.Errorf("company %q not found", name)
}

// ListCompanies returns all registered companies.
func (fc *FederationController) ListCompanies() []*Company {
	fc.mu.RLock()
	defer fc.mu.RUnlock()

	result := make([]*Company, 0, len(fc.companies))
	for _, c := range fc.companies {
		result = append(result, c)
	}
	return result
}

// UpdateCompanyStatus updates the status of a company.
func (fc *FederationController) UpdateCompanyStatus(id string, status CompanyStatus) error {
	fc.mu.Lock()
	defer fc.mu.Unlock()

	c, ok := fc.companies[id]
	if !ok {
		return fmt.Errorf("company %q not found", id)
	}

	c.Status = status

	if err := fc.saveState(); err != nil {
		return fmt.Errorf("persist federation state: %w", err)
	}

	return nil
}

// Transport returns the federation transport.
func (fc *FederationController) Transport() Transport {
	return fc.transport
}

// federationState is the serialized federation state.
type federationState struct {
	Companies map[string]*Company `json:"companies"`
}

func (fc *FederationController) saveState() error {
	state := federationState{
		Companies: fc.companies,
	}

	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal federation state: %w", err)
	}

	path := filepath.Join(fc.stateDir, federationFileName)
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("write federation state: %w", err)
	}

	return nil
}

func (fc *FederationController) loadState() error {
	path := filepath.Join(fc.stateDir, federationFileName)

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("read federation state: %w", err)
	}

	var state federationState
	if err := json.Unmarshal(data, &state); err != nil {
		return fmt.Errorf("unmarshal federation state: %w", err)
	}

	if state.Companies != nil {
		fc.companies = state.Companies
		// Reset all companies to Offline on load; heartbeat will re-probe.
		for _, c := range fc.companies {
			c.Status = CompanyStatusOffline
		}
	}

	return nil
}
