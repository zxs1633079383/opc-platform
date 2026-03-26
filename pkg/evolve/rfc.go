package evolve

import "time"

// RFC represents an improvement proposal.
type RFC struct {
	ID              string    `json:"id"`
	Title           string    `json:"title"`
	Problem         string    `json:"problem"`
	Solution        string    `json:"solution"`
	ExpectedBenefit string    `json:"expectedBenefit"`
	Risk            string    `json:"risk"`
	Status          string    `json:"status"` // "pending" | "approved" | "rejected"
	CreatedAt       time.Time `json:"createdAt"`
}
