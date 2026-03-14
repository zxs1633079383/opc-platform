package goal

import (
	"fmt"

	"github.com/google/uuid"
	"go.uber.org/zap"
)

// Decomposer breaks down a Goal into Projects, Tasks, and Issues.
type Decomposer struct {
	logger *zap.SugaredLogger
}

// NewDecomposer creates a new Decomposer.
func NewDecomposer(logger *zap.SugaredLogger) *Decomposer {
	return &Decomposer{logger: logger}
}

// DecomposeRequest holds the parameters for goal decomposition.
type DecomposeRequest struct {
	GoalID          string
	GoalName        string
	Description     string
	TargetCompanies []string
}

// DecomposeResult holds the output of goal decomposition.
type DecomposeResult struct {
	Projects []*Project
}

// Decompose breaks a goal into projects (one per target company),
// each containing a default task with a default issue.
// In a future version, this will call an AI model for intelligent decomposition.
func (d *Decomposer) Decompose(req DecomposeRequest) (*DecomposeResult, error) {
	if len(req.TargetCompanies) == 0 {
		return nil, fmt.Errorf("no target companies specified")
	}

	projects := make([]*Project, 0, len(req.TargetCompanies))

	for _, companyID := range req.TargetCompanies {
		projectID := uuid.New().String()[:8]
		taskID := uuid.New().String()[:8]
		issueID := uuid.New().String()[:8]

		project := &Project{
			ID:        projectID,
			GoalID:    req.GoalID,
			CompanyID: companyID,
			Name:      fmt.Sprintf("%s - %s", req.GoalName, companyID),
			Tasks: []*Task{
				{
					ID:        taskID,
					ProjectID: projectID,
					Name:      fmt.Sprintf("Implement: %s", req.GoalName),
					Issues: []*Issue{
						{
							ID:     issueID,
							TaskID: taskID,
							Name:   fmt.Sprintf("Execute: %s", req.GoalName),
							Context: map[string]interface{}{
								"description": req.Description,
								"companyId":   companyID,
							},
						},
					},
				},
			},
		}

		projects = append(projects, project)
	}

	d.logger.Infow("goal decomposed",
		"goalId", req.GoalID,
		"projects", len(projects),
	)

	return &DecomposeResult{Projects: projects}, nil
}
