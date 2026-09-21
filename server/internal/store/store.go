// Package store is ServerDash's persistence layer: users, sessions,
// nicknames, automation rules, and scripts, all in a single embedded
// SQLite database file.
package store

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite"
)

type Store struct {
	DB *sql.DB
}

const schema = `
CREATE TABLE IF NOT EXISTS users (
	id            INTEGER PRIMARY KEY AUTOINCREMENT,
	username      TEXT UNIQUE NOT NULL,
	password_hash TEXT NOT NULL,
	role          TEXT NOT NULL CHECK(role IN ('admin','operator','viewer')),
	created_at    DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS sessions (
	token      TEXT PRIMARY KEY,
	user_id    INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
	created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
	expires_at DATETIME NOT NULL
);

CREATE TABLE IF NOT EXISTS nicknames (
	runtime      TEXT NOT NULL,
	resource_key TEXT NOT NULL,
	nickname     TEXT NOT NULL,
	updated_at   DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
	PRIMARY KEY (runtime, resource_key)
);

CREATE TABLE IF NOT EXISTS automation_rules (
	id         TEXT PRIMARY KEY,
	enabled    INTEGER NOT NULL DEFAULT 0,
	config     TEXT NOT NULL DEFAULT '{}',
	schedule   TEXT,
	updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS scripts (
	id         INTEGER PRIMARY KEY AUTOINCREMENT,
	name       TEXT NOT NULL,
	content    TEXT NOT NULL,
	schedule   TEXT,
	enabled    INTEGER NOT NULL DEFAULT 1,
	created_by INTEGER REFERENCES users(id) ON DELETE SET NULL,
	created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
	updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS script_runs (
	id          INTEGER PRIMARY KEY AUTOINCREMENT,
	script_id   INTEGER NOT NULL REFERENCES scripts(id) ON DELETE CASCADE,
	started_at  DATETIME NOT NULL,
	finished_at DATETIME,
	exit_code   INTEGER,
	output      TEXT NOT NULL DEFAULT '',
	triggered_by TEXT NOT NULL DEFAULT 'manual'
);

CREATE TABLE IF NOT EXISTS settings (
	key   TEXT PRIMARY KEY,
	value TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS workflows (
	id           INTEGER PRIMARY KEY AUTOINCREMENT,
	name         TEXT NOT NULL,
	trigger_time TEXT NOT NULL,             -- "HH:MM", 24-hour
	trigger_tz   TEXT NOT NULL DEFAULT 'UTC', -- IANA zone, e.g. America/Chicago
	blocks       TEXT NOT NULL,             -- JSON array of block objects, run in order
	enabled      INTEGER NOT NULL DEFAULT 1,
	created_by   INTEGER REFERENCES users(id) ON DELETE SET NULL,
	created_at   DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
	updated_at   DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS workflow_runs (
	id           INTEGER PRIMARY KEY AUTOINCREMENT,
	workflow_id  INTEGER NOT NULL REFERENCES workflows(id) ON DELETE CASCADE,
	started_at   DATETIME NOT NULL,
	finished_at  DATETIME,
	success      INTEGER,
	log          TEXT NOT NULL DEFAULT '',
	triggered_by TEXT NOT NULL DEFAULT 'manual'
);
`

// Open creates (if needed) and migrates the SQLite database at path.
func Open(path string) (*Store, error) {
	if dir := filepath.Dir(path); dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return nil, fmt.Errorf("create data directory %s: %w", dir, err)
		}
	}

	db, err := sql.Open("sqlite", path+"?_pragma=busy_timeout(5000)&_pragma=foreign_keys(1)")
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}
	// SQLite only supports one writer at a time; a single connection avoids
	// SQLITE_BUSY errors from concurrent writers within this process.
	db.SetMaxOpenConns(1)

	if _, err := db.Exec(schema); err != nil {
		db.Close()
		return nil, fmt.Errorf("migrate schema: %w", err)
	}

	return &Store{DB: db}, nil
}

func (s *Store) Close() error {
	return s.DB.Close()
}
