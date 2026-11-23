package store

import (
	"context"
	"database/sql"
	"errors"
)

func (s *Store) ListManagedContainers(ctx context.Context) ([]string, error) {
	rows, err := s.DB.QueryContext(ctx, `SELECT data FROM managed_containers ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []string
	for rows.Next() {
		var data string
		if err := rows.Scan(&data); err != nil {
			return nil, err
		}
		out = append(out, data)
	}
	return out, rows.Err()
}

func (s *Store) GetManagedContainer(ctx context.Context, id string) (string, bool, error) {
	var data string
	err := s.DB.QueryRowContext(ctx, `SELECT data FROM managed_containers WHERE id = ?`, id).Scan(&data)
	if err == nil {
		return data, true, nil
	}
	if errors.Is(err, sql.ErrNoRows) {
		return "", false, nil
	}
	return "", false, err
}

func (s *Store) SetManagedContainer(ctx context.Context, id, name, data string) error {
	_, err := s.DB.ExecContext(ctx, `
		INSERT INTO managed_containers (id, name, data, updated_at) VALUES (?, ?, ?, CURRENT_TIMESTAMP)
		ON CONFLICT (id) DO UPDATE SET name = excluded.name, data = excluded.data, updated_at = CURRENT_TIMESTAMP
	`, id, name, data)
	return err
}

func (s *Store) DeleteManagedContainer(ctx context.Context, id string) error {
	_, err := s.DB.ExecContext(ctx, `DELETE FROM managed_containers WHERE id = ?`, id)
	return err
}
