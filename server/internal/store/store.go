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

-- Cluster (primary side): nodes that joined this instance, and the
-- single-use tokens that let them join.
CREATE TABLE IF NOT EXISTS cluster_nodes (
	id          TEXT PRIMARY KEY,
	name        TEXT NOT NULL,
	url         TEXT NOT NULL DEFAULT '',
	role        TEXT NOT NULL CHECK(role IN ('node','replica')),
	secret      TEXT NOT NULL,
	last_seen   DATETIME,
	last_status TEXT NOT NULL DEFAULT '{}',
	created_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS cluster_tokens (
	token_hash TEXT PRIMARY KEY,
	role       TEXT NOT NULL CHECK(role IN ('node','replica')),
	expires_at DATETIME NOT NULL,
	created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
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
	if err := addColumns(db); err != nil {
		db.Close()
		return nil, fmt.Errorf("migrate schema: %w", err)
	}

	return &Store{DB: db}, nil
}

// columnMigrations add columns to tables that existed before the column
// did. CREATE TABLE IF NOT EXISTS won't touch an existing table, so new
// columns on old databases arrive here instead; each is idempotent.
var columnMigrations = []struct{ table, column, ddl string }{
	// JSON: the workflow's trigger (schedule or event) and on-failure alert.
	// Empty means the legacy daily trigger_time/trigger_tz.
	{"workflows", "options", `ALTER TABLE workflows ADD COLUMN options TEXT NOT NULL DEFAULT ''`},
}

func addColumns(db *sql.DB) error {
	for _, m := range columnMigrations {
		var n int
		if err := db.QueryRow(`SELECT COUNT(*) FROM pragma_table_info(?) WHERE name = ?`, m.table, m.column).Scan(&n); err != nil {
			return err
		}
		if n == 0 {
			if _, err := db.Exec(m.ddl); err != nil {
				return fmt.Errorf("add %s.%s: %w", m.table, m.column, err)
			}
		}
	}
	return nil
}

func (s *Store) Close() error {
	return s.DB.Close()
}
