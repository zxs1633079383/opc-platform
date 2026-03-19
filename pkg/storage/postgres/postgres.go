package postgres

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	_ "github.com/lib/pq"
	v1 "github.com/zlc-ai/opc-platform/api/v1"
	"github.com/zlc-ai/opc-platform/pkg/storage"
)

// pgStore implements storage.Store using PostgreSQL.
type pgStore struct {
	db *sql.DB
}

// New creates a new PostgreSQL-backed Store.
// dsn example: "postgres://user:pass@localhost:5432/opc?sslmode=disable"
func New(dsn string) (storage.Store, error) {
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		return nil, fmt.Errorf("open postgres: %w", err)
	}

	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(5 * time.Minute)

	if err := db.Ping(); err != nil {
		db.Close()
		return nil, fmt.Errorf("ping postgres: %w", err)
	}

	s := &pgStore{db: db}
	if err := s.migrate(); err != nil {
		db.Close()
		return nil, fmt.Errorf("migrate: %w", err)
	}
	return s, nil
}

func (s *pgStore) migrate() error {
	migrations := []string{
		`CREATE TABLE IF NOT EXISTS agents (
			name       TEXT PRIMARY KEY,
			type       TEXT NOT NULL,
			phase      TEXT NOT NULL DEFAULT 'Created',
			spec_yaml  TEXT NOT NULL DEFAULT '',
			restarts   INTEGER NOT NULL DEFAULT 0,
			message    TEXT NOT NULL DEFAULT '',
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		)`,
		`CREATE TABLE IF NOT EXISTS tasks (
			id         TEXT PRIMARY KEY,
			agent_name TEXT NOT NULL REFERENCES agents(name),
			message    TEXT NOT NULL,
			status     TEXT NOT NULL DEFAULT 'Pending',
			result     TEXT NOT NULL DEFAULT '',
			error      TEXT NOT NULL DEFAULT '',
			tokens_in  INTEGER NOT NULL DEFAULT 0,
			tokens_out INTEGER NOT NULL DEFAULT 0,
			cost       DOUBLE PRECISION NOT NULL DEFAULT 0,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			started_at TIMESTAMPTZ,
			ended_at   TIMESTAMPTZ
		)`,
		`CREATE INDEX IF NOT EXISTS idx_tasks_agent ON tasks(agent_name)`,
		`CREATE INDEX IF NOT EXISTS idx_tasks_status ON tasks(status)`,
		`CREATE TABLE IF NOT EXISTS workflows (
			name       TEXT PRIMARY KEY,
			spec_yaml  TEXT NOT NULL DEFAULT '',
			schedule   TEXT NOT NULL DEFAULT '',
			enabled    BOOLEAN NOT NULL DEFAULT TRUE,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		)`,
		`CREATE TABLE IF NOT EXISTS workflow_runs (
			id            TEXT PRIMARY KEY,
			workflow_name TEXT NOT NULL REFERENCES workflows(name),
			status        TEXT NOT NULL DEFAULT 'Pending',
			steps_json    TEXT NOT NULL DEFAULT '{}',
			started_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			ended_at      TIMESTAMPTZ
		)`,
		`CREATE INDEX IF NOT EXISTS idx_workflow_runs_name ON workflow_runs(workflow_name)`,
		`CREATE TABLE IF NOT EXISTS goals (
			id                  TEXT PRIMARY KEY,
			name                TEXT NOT NULL,
			description         TEXT NOT NULL DEFAULT '',
			phase               TEXT NOT NULL DEFAULT 'active',
			spec_yaml           TEXT NOT NULL DEFAULT '',
			decomposition_plan  TEXT NOT NULL DEFAULT '',
			decompose_cost      DOUBLE PRECISION NOT NULL DEFAULT 0,
			created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			updated_at          TIMESTAMPTZ NOT NULL DEFAULT NOW()
		)`,
	}

	for _, m := range migrations {
		if _, err := s.db.Exec(m); err != nil {
			return fmt.Errorf("exec migration: %w", err)
		}
	}
	return nil
}

func (s *pgStore) Close() error {
	return s.db.Close()
}

// --- Agent operations ---

func (s *pgStore) CreateAgent(ctx context.Context, agent v1.AgentRecord) error {
	now := time.Now()
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO agents (name, type, phase, spec_yaml, restarts, message, created_at, updated_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`,
		agent.Name, string(agent.Type), string(agent.Phase), agent.SpecYAML,
		agent.Restarts, agent.Message, now, now,
	)
	if err != nil {
		return fmt.Errorf("create agent %q: %w", agent.Name, err)
	}
	return nil
}

func (s *pgStore) GetAgent(ctx context.Context, name string) (v1.AgentRecord, error) {
	row := s.db.QueryRowContext(ctx,
		`SELECT name, type, phase, spec_yaml, restarts, message, created_at, updated_at
		 FROM agents WHERE name = $1`, name,
	)
	return scanAgent(row)
}

func (s *pgStore) ListAgents(ctx context.Context) ([]v1.AgentRecord, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT name, type, phase, spec_yaml, restarts, message, created_at, updated_at
		 FROM agents ORDER BY created_at`)
	if err != nil {
		return nil, fmt.Errorf("list agents: %w", err)
	}
	defer rows.Close()

	var agents []v1.AgentRecord
	for rows.Next() {
		a, err := scanAgentRows(rows)
		if err != nil {
			return nil, err
		}
		agents = append(agents, a)
	}
	return agents, rows.Err()
}

func (s *pgStore) UpdateAgent(ctx context.Context, agent v1.AgentRecord) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE agents SET type=$1, phase=$2, spec_yaml=$3, restarts=$4, message=$5, updated_at=$6
		 WHERE name=$7`,
		string(agent.Type), string(agent.Phase), agent.SpecYAML,
		agent.Restarts, agent.Message, time.Now(), agent.Name,
	)
	if err != nil {
		return fmt.Errorf("update agent %q: %w", agent.Name, err)
	}
	return nil
}

func (s *pgStore) DeleteAgent(ctx context.Context, name string) error {
	res, err := s.db.ExecContext(ctx, `DELETE FROM agents WHERE name = $1`, name)
	if err != nil {
		return fmt.Errorf("delete agent %q: %w", name, err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("agent %q not found", name)
	}
	return nil
}

// --- Task operations ---

func (s *pgStore) CreateTask(ctx context.Context, task v1.TaskRecord) error {
	now := time.Now()
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO tasks (id, agent_name, message, status, result, error, tokens_in, tokens_out, cost, created_at, updated_at, started_at, ended_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)`,
		task.ID, task.AgentName, task.Message, string(task.Status),
		task.Result, task.Error, task.TokensIn, task.TokensOut, task.Cost,
		now, now, task.StartedAt, task.EndedAt,
	)
	if err != nil {
		return fmt.Errorf("create task %q: %w", task.ID, err)
	}
	return nil
}

func (s *pgStore) GetTask(ctx context.Context, id string) (v1.TaskRecord, error) {
	row := s.db.QueryRowContext(ctx,
		`SELECT id, agent_name, message, status, result, error, tokens_in, tokens_out, cost, created_at, updated_at, started_at, ended_at
		 FROM tasks WHERE id = $1`, id,
	)
	return scanTask(row)
}

func (s *pgStore) ListTasks(ctx context.Context) ([]v1.TaskRecord, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, agent_name, message, status, result, error, tokens_in, tokens_out, cost, created_at, updated_at, started_at, ended_at
		 FROM tasks ORDER BY created_at DESC`)
	if err != nil {
		return nil, fmt.Errorf("list tasks: %w", err)
	}
	defer rows.Close()

	var tasks []v1.TaskRecord
	for rows.Next() {
		t, err := scanTaskRows(rows)
		if err != nil {
			return nil, err
		}
		tasks = append(tasks, t)
	}
	return tasks, rows.Err()
}

func (s *pgStore) ListTasksByAgent(ctx context.Context, agentName string) ([]v1.TaskRecord, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, agent_name, message, status, result, error, tokens_in, tokens_out, cost, created_at, updated_at, started_at, ended_at
		 FROM tasks WHERE agent_name = $1 ORDER BY created_at DESC`, agentName)
	if err != nil {
		return nil, fmt.Errorf("list tasks for agent %q: %w", agentName, err)
	}
	defer rows.Close()

	var tasks []v1.TaskRecord
	for rows.Next() {
		t, err := scanTaskRows(rows)
		if err != nil {
			return nil, err
		}
		tasks = append(tasks, t)
	}
	return tasks, rows.Err()
}

func (s *pgStore) UpdateTask(ctx context.Context, task v1.TaskRecord) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE tasks SET status=$1, result=$2, error=$3, tokens_in=$4, tokens_out=$5, cost=$6, updated_at=$7, started_at=$8, ended_at=$9
		 WHERE id=$10`,
		string(task.Status), task.Result, task.Error,
		task.TokensIn, task.TokensOut, task.Cost,
		time.Now(), task.StartedAt, task.EndedAt, task.ID,
	)
	if err != nil {
		return fmt.Errorf("update task %q: %w", task.ID, err)
	}
	return nil
}

// --- Workflow operations ---

func (s *pgStore) CreateWorkflow(ctx context.Context, wf v1.WorkflowRecord) error {
	now := time.Now()
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO workflows (name, spec_yaml, schedule, enabled, created_at, updated_at)
		 VALUES ($1, $2, $3, $4, $5, $6)`,
		wf.Name, wf.SpecYAML, wf.Schedule, wf.Enabled, now, now,
	)
	if err != nil {
		return fmt.Errorf("create workflow %q: %w", wf.Name, err)
	}
	return nil
}

func (s *pgStore) GetWorkflow(ctx context.Context, name string) (v1.WorkflowRecord, error) {
	row := s.db.QueryRowContext(ctx,
		`SELECT name, spec_yaml, schedule, enabled, created_at, updated_at
		 FROM workflows WHERE name = $1`, name,
	)
	var wf v1.WorkflowRecord
	err := row.Scan(&wf.Name, &wf.SpecYAML, &wf.Schedule, &wf.Enabled, &wf.CreatedAt, &wf.UpdatedAt)
	if err != nil {
		return wf, fmt.Errorf("get workflow %q: %w", name, err)
	}
	return wf, nil
}

func (s *pgStore) ListWorkflows(ctx context.Context) ([]v1.WorkflowRecord, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT name, spec_yaml, schedule, enabled, created_at, updated_at
		 FROM workflows ORDER BY created_at`)
	if err != nil {
		return nil, fmt.Errorf("list workflows: %w", err)
	}
	defer rows.Close()

	var workflows []v1.WorkflowRecord
	for rows.Next() {
		var wf v1.WorkflowRecord
		if err := rows.Scan(&wf.Name, &wf.SpecYAML, &wf.Schedule, &wf.Enabled, &wf.CreatedAt, &wf.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan workflow: %w", err)
		}
		workflows = append(workflows, wf)
	}
	return workflows, rows.Err()
}

func (s *pgStore) UpdateWorkflow(ctx context.Context, wf v1.WorkflowRecord) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE workflows SET spec_yaml=$1, schedule=$2, enabled=$3, updated_at=$4
		 WHERE name=$5`,
		wf.SpecYAML, wf.Schedule, wf.Enabled, time.Now(), wf.Name,
	)
	if err != nil {
		return fmt.Errorf("update workflow %q: %w", wf.Name, err)
	}
	return nil
}

func (s *pgStore) DeleteWorkflow(ctx context.Context, name string) error {
	res, err := s.db.ExecContext(ctx, `DELETE FROM workflows WHERE name = $1`, name)
	if err != nil {
		return fmt.Errorf("delete workflow %q: %w", name, err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("workflow %q not found", name)
	}
	return nil
}

// --- Workflow run operations ---

func (s *pgStore) CreateWorkflowRun(ctx context.Context, run v1.WorkflowRunRecord) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO workflow_runs (id, workflow_name, status, steps_json, started_at, ended_at)
		 VALUES ($1, $2, $3, $4, $5, $6)`,
		run.ID, run.WorkflowName, string(run.Status), run.StepsJSON, run.StartedAt, run.EndedAt,
	)
	if err != nil {
		return fmt.Errorf("create workflow run %q: %w", run.ID, err)
	}
	return nil
}

func (s *pgStore) GetWorkflowRun(ctx context.Context, id string) (v1.WorkflowRunRecord, error) {
	row := s.db.QueryRowContext(ctx,
		`SELECT id, workflow_name, status, steps_json, started_at, ended_at
		 FROM workflow_runs WHERE id = $1`, id,
	)
	var run v1.WorkflowRunRecord
	var status string
	var endedAt sql.NullTime
	err := row.Scan(&run.ID, &run.WorkflowName, &status, &run.StepsJSON, &run.StartedAt, &endedAt)
	if err != nil {
		return run, fmt.Errorf("get workflow run %q: %w", id, err)
	}
	run.Status = v1.WorkflowStatus(status)
	if endedAt.Valid {
		run.EndedAt = &endedAt.Time
	}
	return run, nil
}

func (s *pgStore) ListWorkflowRuns(ctx context.Context, workflowName string) ([]v1.WorkflowRunRecord, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, workflow_name, status, steps_json, started_at, ended_at
		 FROM workflow_runs WHERE workflow_name = $1 ORDER BY started_at DESC`, workflowName)
	if err != nil {
		return nil, fmt.Errorf("list workflow runs: %w", err)
	}
	defer rows.Close()

	var runs []v1.WorkflowRunRecord
	for rows.Next() {
		var run v1.WorkflowRunRecord
		var status string
		var endedAt sql.NullTime
		if err := rows.Scan(&run.ID, &run.WorkflowName, &status, &run.StepsJSON, &run.StartedAt, &endedAt); err != nil {
			return nil, fmt.Errorf("scan workflow run: %w", err)
		}
		run.Status = v1.WorkflowStatus(status)
		if endedAt.Valid {
			run.EndedAt = &endedAt.Time
		}
		runs = append(runs, run)
	}
	return runs, rows.Err()
}

func (s *pgStore) UpdateWorkflowRun(ctx context.Context, run v1.WorkflowRunRecord) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE workflow_runs SET status=$1, steps_json=$2, ended_at=$3
		 WHERE id=$4`,
		string(run.Status), run.StepsJSON, run.EndedAt, run.ID,
	)
	if err != nil {
		return fmt.Errorf("update workflow run %q: %w", run.ID, err)
	}
	return nil
}

// --- Goal operations ---

func (s *pgStore) CreateGoal(ctx context.Context, goal v1.GoalRecord) error {
	now := time.Now()
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO goals (id, name, description, phase, spec_yaml, decomposition_plan, decompose_cost, created_at, updated_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`,
		goal.ID, goal.Name, goal.Description, string(goal.Phase),
		goal.SpecYAML, goal.DecompositionPlan, goal.DecomposeCost, now, now,
	)
	if err != nil {
		return fmt.Errorf("create goal %q: %w", goal.ID, err)
	}
	return nil
}

func (s *pgStore) GetGoal(ctx context.Context, id string) (v1.GoalRecord, error) {
	row := s.db.QueryRowContext(ctx,
		`SELECT id, name, description, status, phase, spec_yaml, decomposition_plan, decompose_cost,
		 tokens_in, tokens_out, cost, created_at, updated_at
		 FROM goals WHERE id = $1`, id,
	)
	var g v1.GoalRecord
	var phase string
	err := row.Scan(&g.ID, &g.Name, &g.Description, &g.Status, &phase, &g.SpecYAML,
		&g.DecompositionPlan, &g.DecomposeCost,
		&g.TokensIn, &g.TokensOut, &g.Cost, &g.CreatedAt, &g.UpdatedAt)
	if err != nil {
		return g, fmt.Errorf("get goal %q: %w", id, err)
	}
	g.Phase = v1.GoalPhase(phase)
	return g, nil
}

func (s *pgStore) ListGoals(ctx context.Context) ([]v1.GoalRecord, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, name, description, status, phase, spec_yaml, decomposition_plan, decompose_cost,
		 tokens_in, tokens_out, cost, created_at, updated_at
		 FROM goals ORDER BY created_at DESC`)
	if err != nil {
		return nil, fmt.Errorf("list goals: %w", err)
	}
	defer rows.Close()

	var goals []v1.GoalRecord
	for rows.Next() {
		var g v1.GoalRecord
		var phase string
		if err := rows.Scan(&g.ID, &g.Name, &g.Description, &g.Status, &phase, &g.SpecYAML,
			&g.DecompositionPlan, &g.DecomposeCost,
			&g.TokensIn, &g.TokensOut, &g.Cost, &g.CreatedAt, &g.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan goal: %w", err)
		}
		g.Phase = v1.GoalPhase(phase)
		goals = append(goals, g)
	}
	return goals, rows.Err()
}

func (s *pgStore) UpdateGoal(ctx context.Context, goal v1.GoalRecord) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE goals SET name=$1, description=$2, status=$3, phase=$4, spec_yaml=$5,
		 decomposition_plan=$6, decompose_cost=$7,
		 tokens_in=$8, tokens_out=$9, cost=$10, updated_at=$11
		 WHERE id=$12`,
		goal.Name, goal.Description, goal.Status, string(goal.Phase), goal.SpecYAML,
		goal.DecompositionPlan, goal.DecomposeCost,
		goal.TokensIn, goal.TokensOut, goal.Cost, time.Now(), goal.ID,
	)
	if err != nil {
		return fmt.Errorf("update goal %q: %w", goal.ID, err)
	}
	return nil
}

// --- scan helpers ---

type scanner interface {
	Scan(dest ...any) error
}

func scanAgent(s scanner) (v1.AgentRecord, error) {
	var a v1.AgentRecord
	var agentType, phase string
	err := s.Scan(&a.Name, &agentType, &phase, &a.SpecYAML, &a.Restarts, &a.Message, &a.CreatedAt, &a.UpdatedAt)
	if err != nil {
		return a, fmt.Errorf("scan agent: %w", err)
	}
	a.Type = v1.AgentType(agentType)
	a.Phase = v1.AgentPhase(phase)
	return a, nil
}

func scanAgentRows(rows *sql.Rows) (v1.AgentRecord, error) {
	return scanAgent(rows)
}

func scanTask(s scanner) (v1.TaskRecord, error) {
	var t v1.TaskRecord
	var status string
	var startedAt, endedAt sql.NullTime
	err := s.Scan(&t.ID, &t.AgentName, &t.Message, &status,
		&t.Result, &t.Error, &t.TokensIn, &t.TokensOut, &t.Cost,
		&t.CreatedAt, &t.UpdatedAt, &startedAt, &endedAt)
	if err != nil {
		return t, fmt.Errorf("scan task: %w", err)
	}
	t.Status = v1.TaskStatus(status)
	if startedAt.Valid {
		t.StartedAt = &startedAt.Time
	}
	if endedAt.Valid {
		t.EndedAt = &endedAt.Time
	}
	return t, nil
}

func scanTaskRows(rows *sql.Rows) (v1.TaskRecord, error) {
	return scanTask(rows)
}

// ---- Goal extended ----
func (s *pgStore) DeleteGoal(ctx context.Context, id string) error { _, err := s.db.ExecContext(ctx, "DELETE FROM goals WHERE id = $1", id); return err }
func (s *pgStore) GoalStats(ctx context.Context, goalID string) (v1.HierarchyStats, error) {
	var st v1.HierarchyStats
	err := s.db.QueryRowContext(ctx, `SELECT COALESCE(SUM(tokens_in),0),COALESCE(SUM(tokens_out),0),COALESCE(SUM(cost),0),COUNT(*),COALESCE(SUM(CASE WHEN status='Completed' THEN 1 ELSE 0 END),0),COALESCE(SUM(CASE WHEN status='Failed' THEN 1 ELSE 0 END),0) FROM tasks WHERE goal_id=$1`, goalID).
		Scan(&st.TotalTokensIn, &st.TotalTokensOut, &st.TotalCost, &st.TaskCount, &st.CompletedTasks, &st.FailedTasks)
	return st, err
}

// ---- Project ----
func (s *pgStore) CreateProject(ctx context.Context, p v1.ProjectRecord) error {
	now := time.Now()
	_, err := s.db.ExecContext(ctx, `INSERT INTO projects (id,name,goal_id,description,status,spec_yaml,tokens_in,tokens_out,cost,created_at,updated_at) VALUES ($1,$2,$3,$4,$5,$6,0,0,0,$7,$8)`, p.ID, p.Name, p.GoalID, p.Description, p.Status, p.SpecYAML, now, now)
	return err
}
func (s *pgStore) GetProject(ctx context.Context, id string) (v1.ProjectRecord, error) {
	var p v1.ProjectRecord
	err := s.db.QueryRowContext(ctx, `SELECT id,name,goal_id,description,status,COALESCE(spec_yaml,''),tokens_in,tokens_out,cost,created_at,updated_at FROM projects WHERE id=$1`, id).
		Scan(&p.ID, &p.Name, &p.GoalID, &p.Description, &p.Status, &p.SpecYAML, &p.TokensIn, &p.TokensOut, &p.Cost, &p.CreatedAt, &p.UpdatedAt)
	return p, err
}
func (s *pgStore) ListProjects(ctx context.Context) ([]v1.ProjectRecord, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id,name,goal_id,description,status,COALESCE(spec_yaml,''),tokens_in,tokens_out,cost,created_at,updated_at FROM projects ORDER BY created_at DESC`)
	if err != nil { return nil, err }; defer rows.Close()
	var r []v1.ProjectRecord; for rows.Next() { var p v1.ProjectRecord; rows.Scan(&p.ID, &p.Name, &p.GoalID, &p.Description, &p.Status, &p.SpecYAML, &p.TokensIn, &p.TokensOut, &p.Cost, &p.CreatedAt, &p.UpdatedAt); r = append(r, p) }; return r, nil
}
func (s *pgStore) ListProjectsByGoal(ctx context.Context, goalID string) ([]v1.ProjectRecord, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id,name,goal_id,description,status,COALESCE(spec_yaml,''),tokens_in,tokens_out,cost,created_at,updated_at FROM projects WHERE goal_id=$1 ORDER BY created_at`, goalID)
	if err != nil { return nil, err }; defer rows.Close()
	var r []v1.ProjectRecord; for rows.Next() { var p v1.ProjectRecord; rows.Scan(&p.ID, &p.Name, &p.GoalID, &p.Description, &p.Status, &p.SpecYAML, &p.TokensIn, &p.TokensOut, &p.Cost, &p.CreatedAt, &p.UpdatedAt); r = append(r, p) }; return r, nil
}
func (s *pgStore) UpdateProject(ctx context.Context, p v1.ProjectRecord) error {
	_, err := s.db.ExecContext(ctx, `UPDATE projects SET name=$1,description=$2,status=$3,tokens_in=$4,tokens_out=$5,cost=$6,updated_at=$7 WHERE id=$8`, p.Name, p.Description, p.Status, p.TokensIn, p.TokensOut, p.Cost, time.Now(), p.ID); return err
}
func (s *pgStore) DeleteProject(ctx context.Context, id string) error { _, err := s.db.ExecContext(ctx, "DELETE FROM projects WHERE id=$1", id); return err }
func (s *pgStore) ProjectStats(ctx context.Context, projectID string) (v1.HierarchyStats, error) {
	var st v1.HierarchyStats
	err := s.db.QueryRowContext(ctx, `SELECT COALESCE(SUM(tokens_in),0),COALESCE(SUM(tokens_out),0),COALESCE(SUM(cost),0),COUNT(*),COALESCE(SUM(CASE WHEN status='Completed' THEN 1 ELSE 0 END),0),COALESCE(SUM(CASE WHEN status='Failed' THEN 1 ELSE 0 END),0) FROM tasks WHERE project_id=$1`, projectID).
		Scan(&st.TotalTokensIn, &st.TotalTokensOut, &st.TotalCost, &st.TaskCount, &st.CompletedTasks, &st.FailedTasks)
	return st, err
}

// ---- Issue ----
func (s *pgStore) CreateIssue(ctx context.Context, i v1.IssueRecord) error {
	now := time.Now()
	_, err := s.db.ExecContext(ctx, `INSERT INTO issues (id,name,project_id,description,agent_ref,status,spec_yaml,tokens_in,tokens_out,cost,created_at,updated_at) VALUES ($1,$2,$3,$4,$5,$6,$7,0,0,0,$8,$9)`, i.ID, i.Name, i.ProjectID, i.Description, i.AgentRef, i.Status, i.SpecYAML, now, now)
	return err
}
func (s *pgStore) GetIssue(ctx context.Context, id string) (v1.IssueRecord, error) {
	var i v1.IssueRecord
	err := s.db.QueryRowContext(ctx, `SELECT id,name,project_id,description,COALESCE(agent_ref,''),status,COALESCE(spec_yaml,''),tokens_in,tokens_out,cost,created_at,updated_at FROM issues WHERE id=$1`, id).
		Scan(&i.ID, &i.Name, &i.ProjectID, &i.Description, &i.AgentRef, &i.Status, &i.SpecYAML, &i.TokensIn, &i.TokensOut, &i.Cost, &i.CreatedAt, &i.UpdatedAt)
	return i, err
}
func (s *pgStore) ListIssues(ctx context.Context) ([]v1.IssueRecord, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id,name,project_id,description,COALESCE(agent_ref,''),status,COALESCE(spec_yaml,''),tokens_in,tokens_out,cost,created_at,updated_at FROM issues ORDER BY created_at DESC`)
	if err != nil { return nil, err }; defer rows.Close()
	var r []v1.IssueRecord; for rows.Next() { var i v1.IssueRecord; rows.Scan(&i.ID, &i.Name, &i.ProjectID, &i.Description, &i.AgentRef, &i.Status, &i.SpecYAML, &i.TokensIn, &i.TokensOut, &i.Cost, &i.CreatedAt, &i.UpdatedAt); r = append(r, i) }; return r, nil
}
func (s *pgStore) ListIssuesByProject(ctx context.Context, projectID string) ([]v1.IssueRecord, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id,name,project_id,description,COALESCE(agent_ref,''),status,COALESCE(spec_yaml,''),tokens_in,tokens_out,cost,created_at,updated_at FROM issues WHERE project_id=$1 ORDER BY created_at`, projectID)
	if err != nil { return nil, err }; defer rows.Close()
	var r []v1.IssueRecord; for rows.Next() { var i v1.IssueRecord; rows.Scan(&i.ID, &i.Name, &i.ProjectID, &i.Description, &i.AgentRef, &i.Status, &i.SpecYAML, &i.TokensIn, &i.TokensOut, &i.Cost, &i.CreatedAt, &i.UpdatedAt); r = append(r, i) }; return r, nil
}
func (s *pgStore) UpdateIssue(ctx context.Context, i v1.IssueRecord) error {
	_, err := s.db.ExecContext(ctx, `UPDATE issues SET name=$1,description=$2,agent_ref=$3,status=$4,tokens_in=$5,tokens_out=$6,cost=$7,updated_at=$8 WHERE id=$9`, i.Name, i.Description, i.AgentRef, i.Status, i.TokensIn, i.TokensOut, i.Cost, time.Now(), i.ID); return err
}
func (s *pgStore) DeleteIssue(ctx context.Context, id string) error { _, err := s.db.ExecContext(ctx, "DELETE FROM issues WHERE id=$1", id); return err }
