package store

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/base64"
	"errors"
	"fmt"
	"time"

	"golang.org/x/crypto/bcrypt"
)

type Role string

const (
	RoleAdmin    Role = "admin"
	RoleOperator Role = "operator"
	RoleViewer   Role = "viewer"
)

func (r Role) Valid() bool {
	return r == RoleAdmin || r == RoleOperator || r == RoleViewer
}

type User struct {
	ID        int64
	Username  string
	Role      Role
	CreatedAt time.Time
}

var ErrNotFound = errors.New("not found")
var ErrConflict = errors.New("already exists")

// UserCount is used to decide whether the first-run setup wizard should
// still be shown.
func (s *Store) UserCount(ctx context.Context) (int, error) {
	var n int
	err := s.DB.QueryRowContext(ctx, `SELECT COUNT(*) FROM users`).Scan(&n)
	return n, err
}

func (s *Store) CreateUser(ctx context.Context, username, password string, role Role) (User, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return User{}, fmt.Errorf("hash password: %w", err)
	}

	res, err := s.DB.ExecContext(ctx,
		`INSERT INTO users (username, password_hash, role) VALUES (?, ?, ?)`,
		username, string(hash), string(role))
	if err != nil {
		return User{}, ErrConflict
	}
	id, _ := res.LastInsertId()
	return User{ID: id, Username: username, Role: role, CreatedAt: time.Now()}, nil
}

func (s *Store) VerifyPassword(ctx context.Context, username, password string) (User, error) {
	var u User
	var hash string
	err := s.DB.QueryRowContext(ctx,
		`SELECT id, username, password_hash, role, created_at FROM users WHERE username = ?`, username,
	).Scan(&u.ID, &u.Username, &hash, &u.Role, &u.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		// Still run bcrypt so login timing doesn't reveal whether the
		// username exists.
		_ = bcrypt.CompareHashAndPassword([]byte("$2a$10$invalidinvalidinvalidinvalidinvalidinvalidinvalidinva"), []byte(password))
		return User{}, ErrNotFound
	}
	if err != nil {
		return User{}, err
	}
	if err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)); err != nil {
		return User{}, ErrNotFound
	}
	return u, nil
}

func (s *Store) ListUsers(ctx context.Context) ([]User, error) {
	rows, err := s.DB.QueryContext(ctx, `SELECT id, username, role, created_at FROM users ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var users []User
	for rows.Next() {
		var u User
		if err := rows.Scan(&u.ID, &u.Username, &u.Role, &u.CreatedAt); err != nil {
			return nil, err
		}
		users = append(users, u)
	}
	return users, rows.Err()
}

func (s *Store) SetUserRole(ctx context.Context, id int64, role Role) error {
	res, err := s.DB.ExecContext(ctx, `UPDATE users SET role = ? WHERE id = ?`, string(role), id)
	if err != nil {
		return err
	}
	return checkRowsAffected(res)
}

func (s *Store) SetUserPassword(ctx context.Context, id int64, password string) error {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("hash password: %w", err)
	}
	res, err := s.DB.ExecContext(ctx, `UPDATE users SET password_hash = ? WHERE id = ?`, string(hash), id)
	if err != nil {
		return err
	}
	return checkRowsAffected(res)
}

func (s *Store) DeleteUser(ctx context.Context, id int64) error {
	res, err := s.DB.ExecContext(ctx, `DELETE FROM users WHERE id = ?`, id)
	if err != nil {
		return err
	}
	return checkRowsAffected(res)
}

func checkRowsAffected(res sql.Result) error {
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

// --- Sessions ---

type Session struct {
	Token     string
	UserID    int64
	ExpiresAt time.Time
}

const sessionTTL = 7 * 24 * time.Hour

func newToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

func (s *Store) CreateSession(ctx context.Context, userID int64) (Session, error) {
	token, err := newToken()
	if err != nil {
		return Session{}, err
	}
	sess := Session{Token: token, UserID: userID, ExpiresAt: time.Now().Add(sessionTTL)}
	_, err = s.DB.ExecContext(ctx,
		`INSERT INTO sessions (token, user_id, expires_at) VALUES (?, ?, ?)`,
		sess.Token, sess.UserID, sess.ExpiresAt)
	if err != nil {
		return Session{}, err
	}
	return sess, nil
}

// UserBySession resolves a session token to its user, treating an expired
// or unknown token as "not found" rather than a distinct error, since
// callers only care whether the caller is authenticated.
func (s *Store) UserBySession(ctx context.Context, token string) (User, error) {
	var u User
	err := s.DB.QueryRowContext(ctx, `
		SELECT u.id, u.username, u.role, u.created_at
		FROM sessions s JOIN users u ON u.id = s.user_id
		WHERE s.token = ? AND s.expires_at > CURRENT_TIMESTAMP
	`, token).Scan(&u.ID, &u.Username, &u.Role, &u.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return User{}, ErrNotFound
	}
	return u, err
}

func (s *Store) DeleteSession(ctx context.Context, token string) error {
	_, err := s.DB.ExecContext(ctx, `DELETE FROM sessions WHERE token = ?`, token)
	return err
}

// DeleteExpiredSessions is run periodically to keep the table from growing
// without bound.
func (s *Store) DeleteExpiredSessions(ctx context.Context) error {
	_, err := s.DB.ExecContext(ctx, `DELETE FROM sessions WHERE expires_at <= CURRENT_TIMESTAMP`)
	return err
}
