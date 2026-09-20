package store

import (
	"context"
	"database/sql"
	"errors"
	"time"
)

type Script struct {
	ID        int64         `json:"id"`
	Name      string        `json:"name"`
	Content   string        `json:"content"`
	Schedule  string        `json:"schedule"` // cron expression; empty means manual-run-only
	Enabled   bool          `json:"enabled"`
	CreatedBy sql.NullInt64 `json:"createdBy"`
	CreatedAt time.Time     `json:"createdAt"`
	UpdatedAt time.Time     `json:"updatedAt"`
}

type ScriptRun struct {
	ID          int64         `json:"id"`
	ScriptID    int64         `json:"scriptId"`
	StartedAt   time.Time     `json:"startedAt"`
	FinishedAt  sql.NullTime  `json:"finishedAt"`
	ExitCode    sql.NullInt64 `json:"exitCode"`
	Output      string        `json:"output"`
	TriggeredBy string        `json:"triggeredBy"`
}

func (s *Store) CreateScript(ctx context.Context, name, content, schedule string, createdBy int64) (Script, error) {
	now := time.Now()
	res, err := s.DB.ExecContext(ctx, `
		INSERT INTO scripts (name, content, schedule, enabled, created_by, created_at, updated_at)
		VALUES (?, ?, ?, 1, ?, ?, ?)
	`, name, content, nullIfEmpty(schedule), createdBy, now, now)
	if err != nil {
		return Script{}, err
	}
	id, _ := res.LastInsertId()
	return Script{ID: id, Name: name, Content: content, Schedule: schedule, Enabled: true, CreatedAt: now, UpdatedAt: now}, nil
}

func (s *Store) UpdateScript(ctx context.Context, id int64, name, content, schedule string, enabled bool) error {
	res, err := s.DB.ExecContext(ctx, `
		UPDATE scripts SET name = ?, content = ?, schedule = ?, enabled = ?, updated_at = CURRENT_TIMESTAMP
		WHERE id = ?
	`, name, content, nullIfEmpty(schedule), boolToInt(enabled), id)
	if err != nil {
		return err
	}
	return checkRowsAffected(res)
}

func (s *Store) DeleteScript(ctx context.Context, id int64) error {
	res, err := s.DB.ExecContext(ctx, `DELETE FROM scripts WHERE id = ?`, id)
	if err != nil {
		return err
	}
	return checkRowsAffected(res)
}

func (s *Store) GetScript(ctx context.Context, id int64) (Script, error) {
	var sc Script
	var enabled int
	var schedule sql.NullString
	err := s.DB.QueryRowContext(ctx, `
		SELECT id, name, content, schedule, enabled, created_by, created_at, updated_at
		FROM scripts WHERE id = ?
	`, id).Scan(&sc.ID, &sc.Name, &sc.Content, &schedule, &enabled, &sc.CreatedBy, &sc.CreatedAt, &sc.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return Script{}, ErrNotFound
	}
	if err != nil {
		return Script{}, err
	}
	sc.Schedule = schedule.String
	sc.Enabled = enabled != 0
	return sc, nil
}

func (s *Store) ListScripts(ctx context.Context) ([]Script, error) {
	rows, err := s.DB.QueryContext(ctx, `
		SELECT id, name, content, schedule, enabled, created_by, created_at, updated_at
		FROM scripts ORDER BY name
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var scripts []Script
	for rows.Next() {
		var sc Script
		var enabled int
		var schedule sql.NullString
		if err := rows.Scan(&sc.ID, &sc.Name, &sc.Content, &schedule, &enabled, &sc.CreatedBy, &sc.CreatedAt, &sc.UpdatedAt); err != nil {
			return nil, err
		}
		sc.Schedule = schedule.String
		sc.Enabled = enabled != 0
		scripts = append(scripts, sc)
	}
	return scripts, rows.Err()
}

// maxScriptOutputBytes caps how much of a script's combined output is kept,
// so a runaway script can't grow the database unbounded.
const maxScriptOutputBytes = 200_000

func (s *Store) StartScriptRun(ctx context.Context, scriptID int64, triggeredBy string) (int64, error) {
	res, err := s.DB.ExecContext(ctx, `
		INSERT INTO script_runs (script_id, started_at, output, triggered_by) VALUES (?, ?, '', ?)
	`, scriptID, time.Now(), triggeredBy)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

func (s *Store) FinishScriptRun(ctx context.Context, runID int64, exitCode int, output string) error {
	if len(output) > maxScriptOutputBytes {
		output = output[:maxScriptOutputBytes] + "\n... output truncated ..."
	}
	_, err := s.DB.ExecContext(ctx, `
		UPDATE script_runs SET finished_at = ?, exit_code = ?, output = ? WHERE id = ?
	`, time.Now(), exitCode, output, runID)
	return err
}

func (s *Store) ListScriptRuns(ctx context.Context, scriptID int64, limit int) ([]ScriptRun, error) {
	rows, err := s.DB.QueryContext(ctx, `
		SELECT id, script_id, started_at, finished_at, exit_code, output, triggered_by
		FROM script_runs WHERE script_id = ? ORDER BY started_at DESC LIMIT ?
	`, scriptID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var runs []ScriptRun
	for rows.Next() {
		var r ScriptRun
		if err := rows.Scan(&r.ID, &r.ScriptID, &r.StartedAt, &r.FinishedAt, &r.ExitCode, &r.Output, &r.TriggeredBy); err != nil {
			return nil, err
		}
		runs = append(runs, r)
	}
	return runs, rows.Err()
}
