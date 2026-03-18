package federation

import "time"

// CompanyStatus represents the operational status of a company.
type CompanyStatus string

const (
	CompanyStatusOnline  CompanyStatus = "Online"
	CompanyStatusOffline CompanyStatus = "Offline"
	CompanyStatusBusy    CompanyStatus = "Busy"
)

// CompanyType represents the functional type of a company.
type CompanyType string

const (
	CompanyTypeSoftware   CompanyType = "software"
	CompanyTypeOperations CompanyType = "operations"
	CompanyTypeSales      CompanyType = "sales"
	CompanyTypeCustom     CompanyType = "custom"
)

// Company represents an independent OPC instance in the federation.
type Company struct {
	ID           string        `json:"id"`
	Name         string        `json:"name"`
	Endpoint     string        `json:"endpoint"`
	DashboardURL string        `json:"dashboardUrl,omitempty"`
	Type         CompanyType   `json:"type"`
	Status       CompanyStatus `json:"status"`
	Agents       []string      `json:"agents,omitempty"`
	APIKey       string        `json:"apiKey,omitempty"`
	JoinedAt     time.Time     `json:"joinedAt"`
}

// CompanyRegistration is the request payload for registering a company.
type CompanyRegistration struct {
	Name         string      `json:"name"`
	Endpoint     string      `json:"endpoint"`
	DashboardURL string      `json:"dashboardUrl,omitempty"`
	Type         CompanyType `json:"type"`
	Agents       []string    `json:"agents,omitempty"`
}
