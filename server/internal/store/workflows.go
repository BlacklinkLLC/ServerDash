package store

import (
	"context"
	"database/sql"
	"errors"
	"time"
)

// Workflow is a scheduled, block-based deployment automation: "at this
// time, in this timezone, run these blocks in order" — e.g. git-pull a
// repo, then rebuild and recreate its container.
type Workflow struct {
	ID          int64
	Name        string
	TriggerTime string // "HH:MM", 24-hour
	TriggerTZ   string // IANA zone
	Blocks      string // raw JSON array
	Options     string // raw JSON: trigger + on-failure alert ("" = legacy daily trigger)
	Enabled     bool
	CreatedBy   sql.NullInt64
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

type WorkflowRun struct {
	ID          int64
	WorkflowID  int64
	StartedAt   time.Time
	FinishedAt  sql.NullTime
	Success     sql.NullBool
	Log         string
	TriggeredBy string
}

func (s *Store) CreateWorkflow(ctx context.Context, name, triggerTime, triggerTZ, blocks, options string, enabled bool, createdBy int64) (Workflow, error) {
	now := time.Now()
	var by any = createdBy
	if createdBy <= 0 {
		by = nil
	}
	res, err := s.DB.ExecContext(ctx, `
		INSERT INTO workflows (name, trigger_time, trigger_tz, blocks, options, enabled, created_by, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, name, triggerTime, triggerTZ, blocks, options, boolToInt(enabled), by, now, now)
	if err != nil {
		return Workflow{}, err
	}
	id, _ := res.LastInsertId()
	return Workflow{
		ID: id, Name: name, TriggerTime: triggerTime, TriggerTZ: triggerTZ, Blocks: blocks, Options: options,
		Enabled: enabled, CreatedAt: now, UpdatedAt: now,
	}, nil
}

func (s *Store) UpdateWorkflow(ctx context.Context, id int64, name, triggerTime, triggerTZ, blocks, options string, enabled bool) error {
	res, err := s.DB.ExecContext(ctx, `
		UPDATE workflows SET name = ?, trigger_time = ?, trigger_tz = ?, blocks = ?, options = ?, enabled = ?, updated_at = CURRENT_TIMESTAMP
		WHERE id = ?
	`, name, triggerTime, triggerTZ, blocks, options, boolToInt(enabled), id)
	if err != nil {
		return err
	}
	return checkRowsAffected(res)
}

func (s *Store) DeleteWorkflow(ctx context.Context, id int64) error {
	res, err := s.DB.ExecContext(ctx, `DELETE FROM workflows WHERE id = ?`, id)
	if err != nil {
		return err
	}
	return checkRowsAffected(res)
}

func (s *Store) GetWorkflow(ctx context.Context, id int64) (Workflow, error) {
	var wf Workflow
	var enabled int
	err := s.DB.QueryRowContext(ctx, `
		SELECT id, name, trigger_time, trigger_tz, blocks, options, enabled, created_by, created_at, updated_at
		FROM workflows WHERE id = ?
	`, id).Scan(&wf.ID, &wf.Name, &wf.TriggerTime, &wf.TriggerTZ, &wf.Blocks, &wf.Options, &enabled, &wf.CreatedBy, &wf.CreatedAt, &wf.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return Workflow{}, ErrNotFound
	}
	if err != nil {
		return Workflow{}, err
	}
	wf.Enabled = enabled != 0
	return wf, nil
}

func (s *Store) ListWorkflows(ctx context.Context) ([]Workflow, error) {
	rows, err := s.DB.QueryContext(ctx, `
		SELECT id, name, trigger_time, trigger_tz, blocks, options, enabled, created_by, created_at, updated_at
		FROM workflows ORDER BY name
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []Workflow
	for rows.Next() {
		var wf Workflow
		var enabled int
		if err := rows.Scan(&wf.ID, &wf.Name, &wf.TriggerTime, &wf.TriggerTZ, &wf.Blocks, &wf.Options, &enabled, &wf.CreatedBy, &wf.CreatedAt, &wf.UpdatedAt); err != nil {
			return nil, err
		}
		wf.Enabled = enabled != 0
		out = append(out, wf)
	}
	return out, rows.Err()
}

const maxWorkflowLogBytes = 200_000

func (s *Store) StartWorkflowRun(ctx context.Context, workflowID int64, triggeredBy string) (int64, error) {
	res, err := s.DB.ExecContext(ctx, `
		INSERT INTO workflow_runs (workflow_id, started_at, log, triggered_by) VALUES (?, ?, '', ?)
	`, workflowID, time.Now(), triggeredBy)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

func (s *Store) FinishWorkflowRun(ctx context.Context, runID int64, success bool, log string) error {
	if len(log) > maxWorkflowLogBytes {
		log = log[:maxWorkflowLogBytes] + "\n... log truncated ..."
	}
	_, err := s.DB.ExecContext(ctx, `
		UPDATE workflow_runs SET finished_at = ?, success = ?, log = ? WHERE id = ?
	`, time.Now(), boolToInt(success), log, runID)
	return err
}

func (s *Store) ListWorkflowRuns(ctx context.Context, workflowID int64, limit int) ([]WorkflowRun, error) {
	rows, err := s.DB.QueryContext(ctx, `
		SELECT id, workflow_id, started_at, finished_at, success, log, triggered_by
		FROM workflow_runs WHERE workflow_id = ? ORDER BY started_at DESC LIMIT ?
	`, workflowID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []WorkflowRun
	for rows.Next() {
		var r WorkflowRun
		if err := rows.Scan(&r.ID, &r.WorkflowID, &r.StartedAt, &r.FinishedAt, &r.Success, &r.Log, &r.TriggeredBy); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// RecentRun is a run plus its workflow's name, for the activity feed.
type RecentRun struct {
	WorkflowRun
	WorkflowName string
}

func (s *Store) ListRecentWorkflowRuns(ctx context.Context, limit int) ([]RecentRun, error) {
	rows, err := s.DB.QueryContext(ctx, `
		SELECT r.id, r.workflow_id, r.started_at, r.finished_at, r.success, r.log, r.triggered_by, w.name
		FROM workflow_runs r JOIN workflows w ON w.id = r.workflow_id
		ORDER BY r.started_at DESC LIMIT ?
	`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []RecentRun{}
	for rows.Next() {
		var r RecentRun
		if err := rows.Scan(&r.ID, &r.WorkflowID, &r.StartedAt, &r.FinishedAt, &r.Success, &r.Log, &r.TriggeredBy, &r.WorkflowName); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}
