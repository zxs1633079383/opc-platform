package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	_ "github.com/mattn/go-sqlite3"
	v1 "github.com/zlc-ai/opc-platform/api/v1"
	"github.com/zlc-ai/opc-platform/pkg/storage"
)

// sqliteStore implements storage.Store using SQLite.
type sqliteStore struct {
	db *sql.DB
}

// New creates a new SQLite-backed Store at the given path.
func New(dbPath string) (storage.Store, error) {
	db, err := sql.Open("sqlite3", dbPath+"?_journal_mode=WAL&_busy_timeout=5000")
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}

	s := &sqliteStore{db: db}
	if err := s.migrate(); err != nil {
		db.Close()
		return nil, fmt.Errorf("migrate: %w", err)
	}
	return s, nil
}

func (s *sqliteStore) migrate() error {
	migrations := []string{
		`CREATE TABLE IF NOT EXISTS agents (
			name       TEXT PRIMARY KEY,
			type       TEXT NOT NULL,
			phase      TEXT NOT NULL DEFAULT 'Created',
			spec_yaml  TEXT NOT NULL DEFAULT '',
			restarts   INTEGER NOT NULL DEFAULT 0,
			message    TEXT NOT NULL DEFAULT '',
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS tasks (
			id         TEXT PRIMARY KEY,
			agent_name TEXT NOT NULL,
			message    TEXT NOT NULL,
			status     TEXT NOT NULL DEFAULT 'Pending',
			result     TEXT NOT NULL DEFAULT '',
			error      TEXT NOT NULL DEFAULT '',
			tokens_in  INTEGER NOT NULL DEFAULT 0,
			tokens_out INTEGER NOT NULL DEFAULT 0,
			cost       REAL NOT NULL DEFAULT 0,
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			started_at DATETIME,
			ended_at   DATETIME,
			FOREIGN KEY (agent_name) REFERENCES agents(name)
		)`,
		`CREATE INDEX IF NOT EXISTS idx_tasks_agent ON tasks(agent_name)`,
		`CREATE INDEX IF NOT EXISTS idx_tasks_status ON tasks(status)`,
		`CREATE TABLE IF NOT EXISTS workflows (
			name       TEXT PRIMARY KEY,
			spec_yaml  TEXT NOT NULL DEFAULT '',
			schedule   TEXT NOT NULL DEFAULT '',
			enabled    INTEGER NOT NULL DEFAULT 1,
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS workflow_runs (
			id            TEXT PRIMARY KEY,
			workflow_name TEXT NOT NULL,
			status        TEXT NOT NULL DEFAULT 'Pending',
			steps_json    TEXT NOT NULL DEFAULT '{}',
			started_at    DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			ended_at      DATETIME,
			FOREIGN KEY (workflow_name) REFERENCES workflows(name)
		)`,
		`CREATE INDEX IF NOT EXISTS idx_workflow_runs_name ON workflow_runs(workflow_name)`,
		`CREATE TABLE IF NOT EXISTS goals (
			id                  TEXT PRIMARY KEY,
			name                TEXT NOT NULL,
			description         TEXT NOT NULL DEFAULT '',
			phase               TEXT NOT NULL DEFAULT 'active',
			spec_yaml           TEXT NOT NULL DEFAULT '',
			decomposition_plan  TEXT NOT NULL DEFAULT '',
			decompose_cost      REAL NOT NULL DEFAULT 0,
			created_at          DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at          DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`,
	}

	for _, m := range migrations {
		if _, err := s.db.Exec(m); err != nil {
			return fmt.Errorf("exec migration: %w", err)
		}
	}

	// Schema migrations for existing tables.
	alterMigrations := []string{
		"ALTER TABLE goals ADD COLUMN phase TEXT NOT NULL DEFAULT 'active'",
		"ALTER TABLE goals ADD COLUMN spec_yaml TEXT NOT NULL DEFAULT ''",
		"ALTER TABLE goals ADD COLUMN decomposition_plan TEXT NOT NULL DEFAULT ''",
		"ALTER TABLE goals ADD COLUMN decompose_cost REAL NOT NULL DEFAULT 0",
		"ALTER TABLE goals ADD COLUMN owner TEXT NOT NULL DEFAULT ''",
		"ALTER TABLE goals ADD COLUMN deadline TEXT NOT NULL DEFAULT ''",
		"ALTER TABLE goals ADD COLUMN status TEXT NOT NULL DEFAULT 'active'",
		"ALTER TABLE goals ADD COLUMN tokens_in INTEGER NOT NULL DEFAULT 0",
		"ALTER TABLE goals ADD COLUMN tokens_out INTEGER NOT NULL DEFAULT 0",
		"ALTER TABLE goals ADD COLUMN cost REAL NOT NULL DEFAULT 0",
	}
	for _, m := range alterMigrations {
		s.db.Exec(m) // Ignore errors (column already exists).
	}

	// Create projects and issues tables if missing.
	extraTables := []string{
		`CREATE TABLE IF NOT EXISTS projects (
			id TEXT PRIMARY KEY, name TEXT NOT NULL, goal_id TEXT NOT NULL DEFAULT '',
			description TEXT NOT NULL DEFAULT '', status TEXT NOT NULL DEFAULT 'active',
			spec_yaml TEXT NOT NULL DEFAULT '', tokens_in INTEGER NOT NULL DEFAULT 0,
			tokens_out INTEGER NOT NULL DEFAULT 0, cost REAL NOT NULL DEFAULT 0,
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP)`,
		`CREATE TABLE IF NOT EXISTS issues (
			id TEXT PRIMARY KEY, name TEXT NOT NULL, project_id TEXT NOT NULL DEFAULT '',
			description TEXT NOT NULL DEFAULT '', agent_ref TEXT NOT NULL DEFAULT '',
			status TEXT NOT NULL DEFAULT 'open', spec_yaml TEXT NOT NULL DEFAULT '',
			tokens_in INTEGER NOT NULL DEFAULT 0, tokens_out INTEGER NOT NULL DEFAULT 0,
			cost REAL NOT NULL DEFAULT 0,
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP)`,
	}
	for _, m := range extraTables {
		if _, err := s.db.Exec(m); err != nil {
			return fmt.Errorf("exec extra table migration: %w", err)
		}
	}

	return nil
}

func (s *sqliteStore) Close() error {
	return s.db.Close()
}

// --- Agent operations ---

func (s *sqliteStore) CreateAgent(ctx context.Context, agent v1.AgentRecord) error {
	now := time.Now()
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO agents (name, type, phase, spec_yaml, restarts, message, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		agent.Name, string(agent.Type), string(agent.Phase), agent.SpecYAML,
		agent.Restarts, agent.Message, now, now,
	)
	if err != nil {
		return fmt.Errorf("create agent %q: %w", agent.Name, err)
	}
	return nil
}

func (s *sqliteStore) GetAgent(ctx context.Context, name string) (v1.AgentRecord, error) {
	row := s.db.QueryRowContext(ctx,
		`SELECT name, type, phase, spec_yaml, restarts, message, created_at, updated_at
		 FROM agents WHERE name = ?`, name,
	)
	return scanAgent(row)
}

func (s *sqliteStore) ListAgents(ctx context.Context) ([]v1.AgentRecord, error) {
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

func (s *sqliteStore) UpdateAgent(ctx context.Context, agent v1.AgentRecord) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE agents SET type=?, phase=?, spec_yaml=?, restarts=?, message=?, updated_at=?
		 WHERE name=?`,
		string(agent.Type), string(agent.Phase), agent.SpecYAML,
		agent.Restarts, agent.Message, time.Now(), agent.Name,
	)
	if err != nil {
		return fmt.Errorf("update agent %q: %w", agent.Name, err)
	}
	return nil
}

func (s *sqliteStore) DeleteAgent(ctx context.Context, name string) error {
	res, err := s.db.ExecContext(ctx, `DELETE FROM agents WHERE name = ?`, name)
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

func (s *sqliteStore) CreateTask(ctx context.Context, task v1.TaskRecord) error {
	now := time.Now()
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO tasks (id, agent_name, message, status, result, error, tokens_in, tokens_out, cost, issue_id, project_id, goal_id, created_at, updated_at, started_at, ended_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		task.ID, task.AgentName, task.Message, string(task.Status),
		task.Result, task.Error, task.TokensIn, task.TokensOut, task.Cost,
		task.IssueID, task.ProjectID, task.GoalID,
		now, now, task.StartedAt, task.EndedAt,
	)
	if err != nil {
		return fmt.Errorf("create task %q: %w", task.ID, err)
	}
	return nil
}

func (s *sqliteStore) GetTask(ctx context.Context, id string) (v1.TaskRecord, error) {
	row := s.db.QueryRowContext(ctx,
		`SELECT id, agent_name, message, status, result, error, tokens_in, tokens_out, cost, issue_id, project_id, goal_id, created_at, updated_at, started_at, ended_at
		 FROM tasks WHERE id = ?`, id,
	)
	return scanTask(row)
}

func (s *sqliteStore) ListTasks(ctx context.Context) ([]v1.TaskRecord, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, agent_name, message, status, result, error, tokens_in, tokens_out, cost, issue_id, project_id, goal_id, created_at, updated_at, started_at, ended_at
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

func (s *sqliteStore) ListTasksByAgent(ctx context.Context, agentName string) ([]v1.TaskRecord, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, agent_name, message, status, result, error, tokens_in, tokens_out, cost, issue_id, project_id, goal_id, created_at, updated_at, started_at, ended_at
		 FROM tasks WHERE agent_name = ? ORDER BY created_at DESC`, agentName)
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

func (s *sqliteStore) UpdateTask(ctx context.Context, task v1.TaskRecord) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE tasks SET status=?, result=?, error=?, tokens_in=?, tokens_out=?, cost=?, issue_id=?, project_id=?, goal_id=?, updated_at=?, started_at=?, ended_at=?
		 WHERE id=?`,
		string(task.Status), task.Result, task.Error,
		task.TokensIn, task.TokensOut, task.Cost,
		task.IssueID, task.ProjectID, task.GoalID,
		time.Now(), task.StartedAt, task.EndedAt, task.ID,
	)
	if err != nil {
		return fmt.Errorf("update task %q: %w", task.ID, err)
	}
	return nil
}

// --- Workflow operations ---

func (s *sqliteStore) CreateWorkflow(ctx context.Context, wf v1.WorkflowRecord) error {
	now := time.Now()
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO workflows (name, spec_yaml, schedule, enabled, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?)`,
		wf.Name, wf.SpecYAML, wf.Schedule, wf.Enabled, now, now,
	)
	if err != nil {
		return fmt.Errorf("create workflow %q: %w", wf.Name, err)
	}
	return nil
}

func (s *sqliteStore) GetWorkflow(ctx context.Context, name string) (v1.WorkflowRecord, error) {
	row := s.db.QueryRowContext(ctx,
		`SELECT name, spec_yaml, schedule, enabled, created_at, updated_at
		 FROM workflows WHERE name = ?`, name,
	)
	var wf v1.WorkflowRecord
	err := row.Scan(&wf.Name, &wf.SpecYAML, &wf.Schedule, &wf.Enabled, &wf.CreatedAt, &wf.UpdatedAt)
	if err != nil {
		return wf, fmt.Errorf("get workflow %q: %w", name, err)
	}
	return wf, nil
}

func (s *sqliteStore) ListWorkflows(ctx context.Context) ([]v1.WorkflowRecord, error) {
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

func (s *sqliteStore) UpdateWorkflow(ctx context.Context, wf v1.WorkflowRecord) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE workflows SET spec_yaml=?, schedule=?, enabled=?, updated_at=?
		 WHERE name=?`,
		wf.SpecYAML, wf.Schedule, wf.Enabled, time.Now(), wf.Name,
	)
	if err != nil {
		return fmt.Errorf("update workflow %q: %w", wf.Name, err)
	}
	return nil
}

func (s *sqliteStore) DeleteWorkflow(ctx context.Context, name string) error {
	res, err := s.db.ExecContext(ctx, `DELETE FROM workflows WHERE name = ?`, name)
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

func (s *sqliteStore) CreateWorkflowRun(ctx context.Context, run v1.WorkflowRunRecord) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO workflow_runs (id, workflow_name, status, steps_json, started_at, ended_at)
		 VALUES (?, ?, ?, ?, ?, ?)`,
		run.ID, run.WorkflowName, string(run.Status), run.StepsJSON, run.StartedAt, run.EndedAt,
	)
	if err != nil {
		return fmt.Errorf("create workflow run %q: %w", run.ID, err)
	}
	return nil
}

func (s *sqliteStore) GetWorkflowRun(ctx context.Context, id string) (v1.WorkflowRunRecord, error) {
	row := s.db.QueryRowContext(ctx,
		`SELECT id, workflow_name, status, steps_json, started_at, ended_at
		 FROM workflow_runs WHERE id = ?`, id,
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

func (s *sqliteStore) ListWorkflowRuns(ctx context.Context, workflowName string) ([]v1.WorkflowRunRecord, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, workflow_name, status, steps_json, started_at, ended_at
		 FROM workflow_runs WHERE workflow_name = ? ORDER BY started_at DESC`, workflowName)
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

func (s *sqliteStore) UpdateWorkflowRun(ctx context.Context, run v1.WorkflowRunRecord) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE workflow_runs SET status=?, steps_json=?, ended_at=?
		 WHERE id=?`,
		string(run.Status), run.StepsJSON, run.EndedAt, run.ID,
	)
	if err != nil {
		return fmt.Errorf("update workflow run %q: %w", run.ID, err)
	}
	return nil
}

// --- Goal operations ---

func (s *sqliteStore) CreateGoal(ctx context.Context, goal v1.GoalRecord) error {
	now := time.Now()
	phase := string(goal.Phase)
	if phase == "" { phase = "active" }
	status := goal.Status
	if status == "" { status = "active" }
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO goals (id, name, description, owner, deadline, status, phase, spec_yaml, decomposition_plan, decompose_cost, tokens_in, tokens_out, cost, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		goal.ID, goal.Name, goal.Description, goal.Owner, goal.Deadline, status, phase,
		goal.SpecYAML, goal.DecompositionPlan, goal.DecomposeCost,
		goal.TokensIn, goal.TokensOut, goal.Cost, now, now,
	)
	if err != nil {
		return fmt.Errorf("create goal %q: %w", goal.ID, err)
	}
	return nil
}

func (s *sqliteStore) GetGoal(ctx context.Context, id string) (v1.GoalRecord, error) {
	row := s.db.QueryRowContext(ctx,
		`SELECT id, name, description, phase, spec_yaml, decomposition_plan, decompose_cost, created_at, updated_at
		 FROM goals WHERE id = ?`, id,
	)
	var g v1.GoalRecord
	var phase string
	err := row.Scan(&g.ID, &g.Name, &g.Description, &phase, &g.SpecYAML,
		&g.DecompositionPlan, &g.DecomposeCost, &g.CreatedAt, &g.UpdatedAt)
	if err != nil {
		return g, fmt.Errorf("get goal %q: %w", id, err)
	}
	g.Phase = v1.GoalPhase(phase)
	return g, nil
}

func (s *sqliteStore) ListGoals(ctx context.Context) ([]v1.GoalRecord, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, name, description, phase, spec_yaml, decomposition_plan, decompose_cost, created_at, updated_at
		 FROM goals ORDER BY created_at DESC`)
	if err != nil {
		return nil, fmt.Errorf("list goals: %w", err)
	}
	defer rows.Close()

	var goals []v1.GoalRecord
	for rows.Next() {
		var g v1.GoalRecord
		var phase string
		if err := rows.Scan(&g.ID, &g.Name, &g.Description, &phase, &g.SpecYAML,
			&g.DecompositionPlan, &g.DecomposeCost, &g.CreatedAt, &g.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan goal: %w", err)
		}
		g.Phase = v1.GoalPhase(phase)
		goals = append(goals, g)
	}
	return goals, rows.Err()
}

func (s *sqliteStore) UpdateGoal(ctx context.Context, goal v1.GoalRecord) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE goals SET name=?, description=?, phase=?, spec_yaml=?, decomposition_plan=?, decompose_cost=?, updated_at=?
		 WHERE id=?`,
		goal.Name, goal.Description, string(goal.Phase), goal.SpecYAML,
		goal.DecompositionPlan, goal.DecomposeCost, time.Now(), goal.ID,
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
		&t.IssueID, &t.ProjectID, &t.GoalID,
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

// ---- Goal extended operations ----

func (s *sqliteStore) DeleteGoal(ctx context.Context, id string) error {
	_, err := s.db.ExecContext(ctx, "DELETE FROM goals WHERE id = ?", id)
	return err
}

func (s *sqliteStore) GoalStats(ctx context.Context, goalID string) (v1.HierarchyStats, error) {
	var stats v1.HierarchyStats
	row := s.db.QueryRowContext(ctx, `
		SELECT COALESCE(SUM(tokens_in),0), COALESCE(SUM(tokens_out),0), COALESCE(SUM(cost),0),
		       COUNT(*), COALESCE(SUM(CASE WHEN status='Completed' THEN 1 ELSE 0 END),0),
		       COALESCE(SUM(CASE WHEN status='Failed' THEN 1 ELSE 0 END),0)
		FROM tasks WHERE goal_id = ?`, goalID)
	err := row.Scan(&stats.TotalTokensIn, &stats.TotalTokensOut, &stats.TotalCost,
		&stats.TaskCount, &stats.CompletedTasks, &stats.FailedTasks)
	return stats, err
}

// ---- Project operations ----

func (s *sqliteStore) CreateProject(ctx context.Context, p v1.ProjectRecord) error {
	now := time.Now()
	_, err := s.db.ExecContext(ctx, `INSERT INTO projects (id, name, goal_id, description, status, spec_yaml, tokens_in, tokens_out, cost, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, 0, 0, 0, ?, ?)`, p.ID, p.Name, p.GoalID, p.Description, p.Status, p.SpecYAML, now, now)
	return err
}

func (s *sqliteStore) GetProject(ctx context.Context, id string) (v1.ProjectRecord, error) {
	var p v1.ProjectRecord
	err := s.db.QueryRowContext(ctx, `SELECT id, name, goal_id, description, status, COALESCE(spec_yaml,''), tokens_in, tokens_out, cost, created_at, updated_at FROM projects WHERE id = ?`, id).
		Scan(&p.ID, &p.Name, &p.GoalID, &p.Description, &p.Status, &p.SpecYAML, &p.TokensIn, &p.TokensOut, &p.Cost, &p.CreatedAt, &p.UpdatedAt)
	return p, err
}

func (s *sqliteStore) ListProjects(ctx context.Context) ([]v1.ProjectRecord, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id, name, goal_id, description, status, COALESCE(spec_yaml,''), tokens_in, tokens_out, cost, created_at, updated_at FROM projects ORDER BY created_at DESC`)
	if err != nil { return nil, err }
	defer rows.Close()
	var result []v1.ProjectRecord
	for rows.Next() {
		var p v1.ProjectRecord
		if err := rows.Scan(&p.ID, &p.Name, &p.GoalID, &p.Description, &p.Status, &p.SpecYAML, &p.TokensIn, &p.TokensOut, &p.Cost, &p.CreatedAt, &p.UpdatedAt); err != nil { continue }
		result = append(result, p)
	}
	return result, nil
}

func (s *sqliteStore) ListProjectsByGoal(ctx context.Context, goalID string) ([]v1.ProjectRecord, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id, name, goal_id, description, status, COALESCE(spec_yaml,''), tokens_in, tokens_out, cost, created_at, updated_at FROM projects WHERE goal_id = ? ORDER BY created_at`, goalID)
	if err != nil { return nil, err }
	defer rows.Close()
	var result []v1.ProjectRecord
	for rows.Next() {
		var p v1.ProjectRecord
		if err := rows.Scan(&p.ID, &p.Name, &p.GoalID, &p.Description, &p.Status, &p.SpecYAML, &p.TokensIn, &p.TokensOut, &p.Cost, &p.CreatedAt, &p.UpdatedAt); err != nil { continue }
		result = append(result, p)
	}
	return result, nil
}

func (s *sqliteStore) UpdateProject(ctx context.Context, p v1.ProjectRecord) error {
	_, err := s.db.ExecContext(ctx, `UPDATE projects SET name=?, description=?, status=?, tokens_in=?, tokens_out=?, cost=?, updated_at=? WHERE id=?`,
		p.Name, p.Description, p.Status, p.TokensIn, p.TokensOut, p.Cost, time.Now(), p.ID)
	return err
}

func (s *sqliteStore) DeleteProject(ctx context.Context, id string) error {
	_, err := s.db.ExecContext(ctx, "DELETE FROM projects WHERE id = ?", id)
	return err
}

func (s *sqliteStore) ProjectStats(ctx context.Context, projectID string) (v1.HierarchyStats, error) {
	var stats v1.HierarchyStats
	row := s.db.QueryRowContext(ctx, `
		SELECT COALESCE(SUM(tokens_in),0), COALESCE(SUM(tokens_out),0), COALESCE(SUM(cost),0),
		       COUNT(*), COALESCE(SUM(CASE WHEN status='Completed' THEN 1 ELSE 0 END),0),
		       COALESCE(SUM(CASE WHEN status='Failed' THEN 1 ELSE 0 END),0)
		FROM tasks WHERE project_id = ?`, projectID)
	err := row.Scan(&stats.TotalTokensIn, &stats.TotalTokensOut, &stats.TotalCost,
		&stats.TaskCount, &stats.CompletedTasks, &stats.FailedTasks)
	return stats, err
}

// ---- Issue operations ----

func (s *sqliteStore) CreateIssue(ctx context.Context, i v1.IssueRecord) error {
	now := time.Now()
	_, err := s.db.ExecContext(ctx, `INSERT INTO issues (id, name, project_id, description, agent_ref, status, spec_yaml, tokens_in, tokens_out, cost, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, 0, 0, 0, ?, ?)`, i.ID, i.Name, i.ProjectID, i.Description, i.AgentRef, i.Status, i.SpecYAML, now, now)
	return err
}

func (s *sqliteStore) GetIssue(ctx context.Context, id string) (v1.IssueRecord, error) {
	var i v1.IssueRecord
	err := s.db.QueryRowContext(ctx, `SELECT id, name, project_id, description, COALESCE(agent_ref,''), status, COALESCE(spec_yaml,''), tokens_in, tokens_out, cost, created_at, updated_at FROM issues WHERE id = ?`, id).
		Scan(&i.ID, &i.Name, &i.ProjectID, &i.Description, &i.AgentRef, &i.Status, &i.SpecYAML, &i.TokensIn, &i.TokensOut, &i.Cost, &i.CreatedAt, &i.UpdatedAt)
	return i, err
}

func (s *sqliteStore) ListIssues(ctx context.Context) ([]v1.IssueRecord, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id, name, project_id, description, COALESCE(agent_ref,''), status, COALESCE(spec_yaml,''), tokens_in, tokens_out, cost, created_at, updated_at FROM issues ORDER BY created_at DESC`)
	if err != nil { return nil, err }
	defer rows.Close()
	var result []v1.IssueRecord
	for rows.Next() {
		var i v1.IssueRecord
		if err := rows.Scan(&i.ID, &i.Name, &i.ProjectID, &i.Description, &i.AgentRef, &i.Status, &i.SpecYAML, &i.TokensIn, &i.TokensOut, &i.Cost, &i.CreatedAt, &i.UpdatedAt); err != nil { continue }
		result = append(result, i)
	}
	return result, nil
}

func (s *sqliteStore) ListIssuesByProject(ctx context.Context, projectID string) ([]v1.IssueRecord, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id, name, project_id, description, COALESCE(agent_ref,''), status, COALESCE(spec_yaml,''), tokens_in, tokens_out, cost, created_at, updated_at FROM issues WHERE project_id = ? ORDER BY created_at`, projectID)
	if err != nil { return nil, err }
	defer rows.Close()
	var result []v1.IssueRecord
	for rows.Next() {
		var i v1.IssueRecord
		if err := rows.Scan(&i.ID, &i.Name, &i.ProjectID, &i.Description, &i.AgentRef, &i.Status, &i.SpecYAML, &i.TokensIn, &i.TokensOut, &i.Cost, &i.CreatedAt, &i.UpdatedAt); err != nil { continue }
		result = append(result, i)
	}
	return result, nil
}

func (s *sqliteStore) UpdateIssue(ctx context.Context, i v1.IssueRecord) error {
	_, err := s.db.ExecContext(ctx, `UPDATE issues SET name=?, description=?, agent_ref=?, status=?, tokens_in=?, tokens_out=?, cost=?, updated_at=? WHERE id=?`,
		i.Name, i.Description, i.AgentRef, i.Status, i.TokensIn, i.TokensOut, i.Cost, time.Now(), i.ID)
	return err
}

func (s *sqliteStore) DeleteIssue(ctx context.Context, id string) error {
	_, err := s.db.ExecContext(ctx, "DELETE FROM issues WHERE id = ?", id)
	return err
}
