package store

import "context"

// SetNickname assigns a user-facing alias to a container or pod. runtime is
// "container" (Docker/Podman, keyed by container ID) or "k8s" (keyed by
// "<context>/<namespace>/<pod>").
func (s *Store) SetNickname(ctx context.Context, runtime, resourceKey, nickname string) error {
	_, err := s.DB.ExecContext(ctx, `
		INSERT INTO nicknames (runtime, resource_key, nickname, updated_at)
		VALUES (?, ?, ?, CURRENT_TIMESTAMP)
		ON CONFLICT (runtime, resource_key) DO UPDATE SET nickname = excluded.nickname, updated_at = CURRENT_TIMESTAMP
	`, runtime, resourceKey, nickname)
	return err
}

func (s *Store) DeleteNickname(ctx context.Context, runtime, resourceKey string) error {
	_, err := s.DB.ExecContext(ctx, `DELETE FROM nicknames WHERE runtime = ? AND resource_key = ?`, runtime, resourceKey)
	return err
}

// Nicknames returns every nickname for a runtime as a resourceKey->nickname
// map, so callers can merge them into a container/pod list in one query.
func (s *Store) Nicknames(ctx context.Context, runtime string) (map[string]string, error) {
	rows, err := s.DB.QueryContext(ctx, `SELECT resource_key, nickname FROM nicknames WHERE runtime = ?`, runtime)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make(map[string]string)
	for rows.Next() {
		var key, nick string
		if err := rows.Scan(&key, &nick); err != nil {
			return nil, err
		}
		out[key] = nick
	}
	return out, rows.Err()
}
