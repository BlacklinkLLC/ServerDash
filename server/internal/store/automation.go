package store

import (
	"context"
	"database/sql"
	"errors"
)

type AutomationRule struct {
	ID       string `json:"id"`
	Enabled  bool   `json:"enabled"`
	Config   string `json:"config"`   // raw JSON, shape depends on ID
	Schedule string `json:"schedule"` // cron expression; empty for rules that don't use one
}

// well-known rule IDs
const (
	RuleAutoRestartUnhealthy = "auto_restart_unhealthy"
	RulePrune                = "prune"
	RuleBackup               = "backup"
)

func (s *Store) GetAutomationRule(ctx context.Context, id string) (AutomationRule, error) {
	var r AutomationRule
	var enabled int
	var schedule sql.NullString
	err := s.DB.QueryRowContext(ctx,
		`SELECT id, enabled, config, schedule FROM automation_rules WHERE id = ?`, id,
	).Scan(&r.ID, &enabled, &r.Config, &schedule)
	if errors.Is(err, sql.ErrNoRows) {
		// Rules default to disabled/empty until explicitly configured.
		return AutomationRule{ID: id, Config: "{}"}, nil
	}
	if err != nil {
		return AutomationRule{}, err
	}
	r.Enabled = enabled != 0
	r.Schedule = schedule.String
	return r, nil
}

func (s *Store) ListAutomationRules(ctx context.Context) ([]AutomationRule, error) {
	rows, err := s.DB.QueryContext(ctx, `SELECT id, enabled, config, schedule FROM automation_rules ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var rules []AutomationRule
	for rows.Next() {
		var r AutomationRule
		var enabled int
		var schedule sql.NullString
		if err := rows.Scan(&r.ID, &enabled, &r.Config, &schedule); err != nil {
			return nil, err
		}
		r.Enabled = enabled != 0
		r.Schedule = schedule.String
		rules = append(rules, r)
	}
	return rules, rows.Err()
}

func (s *Store) UpsertAutomationRule(ctx context.Context, r AutomationRule) error {
	_, err := s.DB.ExecContext(ctx, `
		INSERT INTO automation_rules (id, enabled, config, schedule, updated_at)
		VALUES (?, ?, ?, ?, CURRENT_TIMESTAMP)
		ON CONFLICT (id) DO UPDATE SET enabled = excluded.enabled, config = excluded.config,
			schedule = excluded.schedule, updated_at = CURRENT_TIMESTAMP
	`, r.ID, boolToInt(r.Enabled), r.Config, nullIfEmpty(r.Schedule))
	return err
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

func nullIfEmpty(s string) sql.NullString {
	return sql.NullString{String: s, Valid: s != ""}
}
