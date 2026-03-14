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
	}

	for _, m := range migrations {
		if _, err := s.db.Exec(m); err != nil {
			return fmt.Errorf("exec migration: %w", err)
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
		`INSERT INTO tasks (id, agent_name, message, status, result, error, tokens_in, tokens_out, cost, created_at, updated_at, started_at, ended_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		task.ID, task.AgentName, task.Message, string(task.Status),
		task.Result, task.Error, task.TokensIn, task.TokensOut, task.Cost,
		now, now, task.StartedAt, task.EndedAt,
	)
	if err != nil {
		return fmt.Errorf("create task %q: %w", task.ID, err)
	}
	return nil
}

func (s *sqliteStore) GetTask(ctx context.Context, id string) (v1.TaskRecord, error) {
	row := s.db.QueryRowContext(ctx,
		`SELECT id, agent_name, message, status, result, error, tokens_in, tokens_out, cost, created_at, updated_at, started_at, ended_at
		 FROM tasks WHERE id = ?`, id,
	)
	return scanTask(row)
}

func (s *sqliteStore) ListTasks(ctx context.Context) ([]v1.TaskRecord, error) {
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

func (s *sqliteStore) ListTasksByAgent(ctx context.Context, agentName string) ([]v1.TaskRecord, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, agent_name, message, status, result, error, tokens_in, tokens_out, cost, created_at, updated_at, started_at, ended_at
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
		`UPDATE tasks SET status=?, result=?, error=?, tokens_in=?, tokens_out=?, cost=?, updated_at=?, started_at=?, ended_at=?
		 WHERE id=?`,
		string(task.Status), task.Result, task.Error,
		task.TokensIn, task.TokensOut, task.Cost,
		time.Now(), task.StartedAt, task.EndedAt, task.ID,
	)
	if err != nil {
		return fmt.Errorf("update task %q: %w", task.ID, err)
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
