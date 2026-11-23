package store

import (
	"context"
	"database/sql"
	"errors"
)

func (s *Store) GetPreferences(ctx context.Context, userID int64) (string, bool, error) {
	var data string
	err := s.DB.QueryRowContext(ctx, `SELECT data FROM user_preferences WHERE user_id = ?`, userID).Scan(&data)
	if err == nil {
		return data, true, nil
	}
	if errors.Is(err, sql.ErrNoRows) {
		return "", false, nil
	}
	return "", false, err
}

func (s *Store) SetPreferences(ctx context.Context, userID int64, data string) error {
	_, err := s.DB.ExecContext(ctx, `
		INSERT INTO user_preferences (user_id, data, updated_at) VALUES (?, ?, CURRENT_TIMESTAMP)
		ON CONFLICT (user_id) DO UPDATE SET data = excluded.data, updated_at = CURRENT_TIMESTAMP
	`, userID, data)
	return err
}

func (s *Store) DeletePreferences(ctx context.Context, userID int64) error {
	_, err := s.DB.ExecContext(ctx, `DELETE FROM user_preferences WHERE user_id = ?`, userID)
	return err
}
