package store

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"time"
)

type ClusterNode struct {
	ID         string          `json:"id"`
	Name       string          `json:"name"`
	URL        string          `json:"url"`
	Role       string          `json:"role"`
	Secret     string          `json:"-"`
	LastSeen   *time.Time      `json:"lastSeen"`
	LastStatus json.RawMessage `json:"status"`
	CreatedAt  time.Time       `json:"createdAt"`
}

func hashToken(t string) string {
	sum := sha256.Sum256([]byte(t))
	return hex.EncodeToString(sum[:])
}

// CreateJoinToken stores a hash of a freshly generated join token and
// returns the plaintext once; it can't be recovered afterwards.
func (s *Store) CreateJoinToken(ctx context.Context, role string, ttl time.Duration) (string, time.Time, error) {
	tok, err := newToken()
	if err != nil {
		return "", time.Time{}, err
	}
	exp := time.Now().Add(ttl).UTC()
	_, err = s.DB.ExecContext(ctx, `INSERT INTO cluster_tokens (token_hash, role, expires_at) VALUES (?, ?, ?)`, hashToken(tok), role, exp)
	return tok, exp, err
}

// ConsumeJoinToken validates and deletes a join token in one step, so a
// token can only ever admit one node. It returns the role it grants.
func (s *Store) ConsumeJoinToken(ctx context.Context, tok string) (string, error) {
	var role string
	var exp time.Time
	h := hashToken(tok)
	err := s.DB.QueryRowContext(ctx, `SELECT role, expires_at FROM cluster_tokens WHERE token_hash = ?`, h).Scan(&role, &exp)
	if errors.Is(err, sql.ErrNoRows) {
		return "", ErrNotFound
	}
	if err != nil {
		return "", err
	}
	if _, err := s.DB.ExecContext(ctx, `DELETE FROM cluster_tokens WHERE token_hash = ?`, h); err != nil {
		return "", err
	}
	if time.Now().After(exp) {
		return "", ErrNotFound
	}
	return role, nil
}

func (s *Store) CreateClusterNode(ctx context.Context, n ClusterNode) error {
	_, err := s.DB.ExecContext(ctx, `INSERT INTO cluster_nodes (id, name, url, role, secret) VALUES (?, ?, ?, ?, ?)`,
		n.ID, n.Name, n.URL, n.Role, n.Secret)
	return err
}

func (s *Store) ClusterNodes(ctx context.Context) ([]ClusterNode, error) {
	rows, err := s.DB.QueryContext(ctx, `SELECT id, name, url, role, secret, last_seen, last_status, created_at FROM cluster_nodes ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []ClusterNode{}
	for rows.Next() {
		var n ClusterNode
		var seen sql.NullTime
		var status string
		if err := rows.Scan(&n.ID, &n.Name, &n.URL, &n.Role, &n.Secret, &seen, &status, &n.CreatedAt); err != nil {
			return nil, err
		}
		if seen.Valid {
			t := seen.Time
			n.LastSeen = &t
		}
		n.LastStatus = json.RawMessage(status)
		out = append(out, n)
	}
	return out, rows.Err()
}

func (s *Store) ClusterNode(ctx context.Context, id string) (ClusterNode, error) {
	nodes, err := s.ClusterNodes(ctx)
	if err != nil {
		return ClusterNode{}, err
	}
	for _, n := range nodes {
		if n.ID == id {
			return n, nil
		}
	}
	return ClusterNode{}, ErrNotFound
}

func (s *Store) RecordHeartbeat(ctx context.Context, id, name, url string, status []byte) error {
	_, err := s.DB.ExecContext(ctx, `UPDATE cluster_nodes SET name = ?, url = ?, last_seen = ?, last_status = ? WHERE id = ?`,
		name, url, time.Now().UTC(), string(status), id)
	return err
}

func (s *Store) UpdateClusterNode(ctx context.Context, id, name, url, role string) error {
	res, err := s.DB.ExecContext(ctx, `UPDATE cluster_nodes SET name = ?, url = ?, role = ? WHERE id = ?`, name, url, role, id)
	if err != nil {
		return err
	}
	return checkRowsAffected(res)
}

func (s *Store) DeleteClusterNode(ctx context.Context, id string) error {
	res, err := s.DB.ExecContext(ctx, `DELETE FROM cluster_nodes WHERE id = ?`, id)
	if err != nil {
		return err
	}
	return checkRowsAffected(res)
}

// ReplicatedUser carries a user's bcrypt hash (never a plaintext password)
// from a primary to its replicas.
type ReplicatedUser struct {
	Username     string `json:"username"`
	PasswordHash string `json:"passwordHash"`
	Role         string `json:"role"`
}

func (s *Store) ExportUsers(ctx context.Context) ([]ReplicatedUser, error) {
	rows, err := s.DB.QueryContext(ctx, `SELECT username, password_hash, role FROM users ORDER BY username`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []ReplicatedUser{}
	for rows.Next() {
		var u ReplicatedUser
		if err := rows.Scan(&u.Username, &u.PasswordHash, &u.Role); err != nil {
			return nil, err
		}
		out = append(out, u)
	}
	return out, rows.Err()
}

// ReplaceUsers makes this instance's users exactly match users, keyed by
// username: existing accounts keep their id (so their sessions survive a
// password-unchanged sync), new ones are added, missing ones removed.
func (s *Store) ReplaceUsers(ctx context.Context, users []ReplicatedUser) error {
	if len(users) == 0 {
		return errors.New("refusing to replicate an empty user list")
	}
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	keep := map[string]bool{}
	for _, u := range users {
		if !Role(u.Role).Valid() {
			continue
		}
		keep[u.Username] = true
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO users (username, password_hash, role) VALUES (?, ?, ?)
			ON CONFLICT (username) DO UPDATE SET password_hash = excluded.password_hash, role = excluded.role
		`, u.Username, u.PasswordHash, u.Role); err != nil {
			return err
		}
	}
	rows, err := tx.QueryContext(ctx, `SELECT username FROM users`)
	if err != nil {
		return err
	}
	var drop []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err == nil && !keep[name] {
			drop = append(drop, name)
		}
	}
	rows.Close()
	for _, name := range drop {
		if _, err := tx.ExecContext(ctx, `DELETE FROM users WHERE username = ?`, name); err != nil {
			return err
		}
	}
	return tx.Commit()
}
