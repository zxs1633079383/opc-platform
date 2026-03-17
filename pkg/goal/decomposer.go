package goal

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"go.uber.org/zap"
)

// Decomposer defines the interface for goal decomposition strategies.
type Decomposer interface {
	Decompose(ctx context.Context, req DecomposeRequest) (*DecomposeResult, error)
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

// StaticDecomposer breaks down a Goal into Projects, Tasks, and Issues
// using a deterministic, template-based approach (one project per company).
type StaticDecomposer struct {
	logger *zap.SugaredLogger
}

// NewStaticDecomposer creates a new StaticDecomposer.
func NewStaticDecomposer(logger *zap.SugaredLogger) *StaticDecomposer {
	return &StaticDecomposer{logger: logger}
}

// NewDecomposer creates a new StaticDecomposer (backward-compatible alias).
func NewDecomposer(logger *zap.SugaredLogger) *StaticDecomposer {
	return NewStaticDecomposer(logger)
}

// Decompose breaks a goal into projects (one per target company),
// each containing a default task with a default issue.
func (d *StaticDecomposer) Decompose(ctx context.Context, req DecomposeRequest) (*DecomposeResult, error) {
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
