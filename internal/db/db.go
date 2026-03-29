package db

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

var instance *sql.DB

// Init opens (or creates) the SQLite database and runs migrations.
func Init(dataDir string) (*sql.DB, error) {
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		return nil, fmt.Errorf("create data dir: %w", err)
	}

	dbPath := filepath.Join(dataDir, "autocat.db")
	db, err := sql.Open("sqlite3", dbPath+"?_journal_mode=WAL&_foreign_keys=ON")
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}

	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("ping database: %w", err)
	}

	// Connection pool settings
	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(5 * time.Minute)

	if err := migrate(db); err != nil {
		return nil, fmt.Errorf("migrate database: %w", err)
	}

	instance = db
	return db, nil
}

// Get returns the global database instance.
func Get() *sql.DB {
	return instance
}

func migrate(db *sql.DB) error {
	schema := `
	CREATE TABLE IF NOT EXISTS messages (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		chat_id TEXT NOT NULL,
		sender TEXT NOT NULL,
		sender_name TEXT DEFAULT '',
		content TEXT NOT NULL,
		role TEXT NOT NULL CHECK(role IN ('user', 'assistant')),
		timestamp INTEGER NOT NULL,
		session_id TEXT
	);

	CREATE TABLE IF NOT EXISTS sessions (
		id TEXT PRIMARY KEY,
		chat_id TEXT NOT NULL,
		claude_session_id TEXT,
		started_at INTEGER NOT NULL,
		ended_at INTEGER,
		status TEXT NOT NULL DEFAULT 'active' CHECK(status IN ('active', 'ended'))
	);

	CREATE TABLE IF NOT EXISTS scheduled_tasks (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		name TEXT NOT NULL UNIQUE,
		chat_id TEXT NOT NULL,
		prompt TEXT NOT NULL,
		cron_expression TEXT NOT NULL,
		enabled INTEGER NOT NULL DEFAULT 1,
		last_run INTEGER,
		next_run INTEGER,
		created_at INTEGER NOT NULL,
		updated_at INTEGER NOT NULL
	);

	CREATE TABLE IF NOT EXISTS task_runs (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		task_id INTEGER NOT NULL REFERENCES scheduled_tasks(id),
		started_at INTEGER NOT NULL,
		finished_at INTEGER,
		status TEXT NOT NULL DEFAULT 'running' CHECK(status IN ('running', 'success', 'error')),
		result TEXT,
		error TEXT
	);

	CREATE TABLE IF NOT EXISTS memory (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		chat_id TEXT NOT NULL,
		category TEXT NOT NULL,
		content TEXT NOT NULL,
		source TEXT,
		created_at INTEGER NOT NULL,
		expires_at INTEGER
	);

	CREATE TABLE IF NOT EXISTS personalization (
		key TEXT PRIMARY KEY,
		value TEXT NOT NULL,
		updated_at INTEGER NOT NULL
	);

	CREATE INDEX IF NOT EXISTS idx_messages_chat_ts ON messages(chat_id, timestamp);
	CREATE INDEX IF NOT EXISTS idx_messages_session ON messages(session_id);
	CREATE INDEX IF NOT EXISTS idx_memory_chat_cat ON memory(chat_id, category);
	CREATE INDEX IF NOT EXISTS idx_sessions_chat ON sessions(chat_id);
	CREATE INDEX IF NOT EXISTS idx_scheduled_next ON scheduled_tasks(next_run);
	`
	_, err := db.Exec(schema)
	return err
}
