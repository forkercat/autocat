package session

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"fmt"
	"log"
	"time"
)

// Session represents a conversation session.
type Session struct {
	ID              string
	ChatID          string
	ClaudeSessionID sql.NullString
	StartedAt       int64
	EndedAt         sql.NullInt64
	Status          string
}

// GetActive returns the active session for a chat, creating one if none exists.
func GetActive(db *sql.DB, chatID string) (*Session, error) {
	row := db.QueryRow(
		"SELECT id, chat_id, claude_session_id, started_at, ended_at, status FROM sessions WHERE chat_id = ? AND status = 'active' ORDER BY started_at DESC LIMIT 1",
		chatID,
	)
	s := &Session{}
	err := row.Scan(&s.ID, &s.ChatID, &s.ClaudeSessionID, &s.StartedAt, &s.EndedAt, &s.Status)
	if err == sql.ErrNoRows {
		return Create(db, chatID)
	}
	if err != nil {
		return nil, fmt.Errorf("query active session: %w", err)
	}
	return s, nil
}

// Create starts a new session, ending any active ones for the chat.
// Uses a transaction to ensure atomicity.
func Create(db *sql.DB, chatID string) (*Session, error) {
	tx, err := db.Begin()
	if err != nil {
		return nil, fmt.Errorf("begin transaction: %w", err)
	}
	defer tx.Rollback() // no-op if already committed

	now := time.Now().UnixMilli()
	id := generateID(now)

	// End existing active sessions
	if _, err := tx.Exec(
		"UPDATE sessions SET status = 'ended', ended_at = ? WHERE chat_id = ? AND status = 'active'",
		now, chatID,
	); err != nil {
		return nil, fmt.Errorf("end active sessions: %w", err)
	}

	if _, err := tx.Exec(
		"INSERT INTO sessions (id, chat_id, started_at, status) VALUES (?, ?, ?, 'active')",
		id, chatID, now,
	); err != nil {
		return nil, fmt.Errorf("create session: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit session: %w", err)
	}

	log.Printf("[INFO] New session created: %s for chat %s", id, chatID)

	return &Session{
		ID:        id,
		ChatID:    chatID,
		StartedAt: now,
		Status:    "active",
	}, nil
}

// UpdateClaudeSessionID stores the Claude CLI session ID for resume.
func UpdateClaudeSessionID(db *sql.DB, sessionID, claudeSessionID string) error {
	_, err := db.Exec("UPDATE sessions SET claude_session_id = ? WHERE id = ?", claudeSessionID, sessionID)
	return err
}

// End marks a session as ended.
func End(db *sql.DB, sessionID string) error {
	_, err := db.Exec(
		"UPDATE sessions SET status = 'ended', ended_at = ? WHERE id = ?",
		time.Now().UnixMilli(), sessionID,
	)
	return err
}

// EndAllActive ends all active sessions (used for daily reset).
func EndAllActive(db *sql.DB) (int64, error) {
	result, err := db.Exec(
		"UPDATE sessions SET status = 'ended', ended_at = ? WHERE status = 'active'",
		time.Now().UnixMilli(),
	)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}

func generateID(ts int64) string {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		// Fallback to timestamp-only ID if crypto/rand fails
		return fmt.Sprintf("session_%d", ts)
	}
	return fmt.Sprintf("session_%d_%s", ts, hex.EncodeToString(b))
}
