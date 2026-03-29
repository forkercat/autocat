package memory

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/forkercat/autocat/internal/claude"
	"github.com/forkercat/autocat/internal/config"
)

// Entry represents a memory record.
type Entry struct {
	ID        int64
	ChatID    string
	Category  string
	Content   string
	Source    string
	CreatedAt int64
	ExpiresAt sql.NullInt64
}

// Save stores a memory entry.
func Save(db *sql.DB, chatID, category, content, source string, expiresAt *int64) error {
	now := time.Now().UnixMilli()
	var exp sql.NullInt64
	if expiresAt != nil {
		exp = sql.NullInt64{Int64: *expiresAt, Valid: true}
	}

	_, err := db.Exec(
		"INSERT INTO memory (chat_id, category, content, source, created_at, expires_at) VALUES (?, ?, ?, ?, ?, ?)",
		chatID, category, content, source, now, exp,
	)
	return err
}

// Query retrieves memories for a chat, optionally filtered by category.
func Query(db *sql.DB, chatID string, category string, limit int) ([]Entry, error) {
	query := "SELECT id, chat_id, category, content, source, created_at, expires_at FROM memory WHERE chat_id = ?"
	args := []any{chatID}

	if category != "" {
		query += " AND category = ?"
		args = append(args, category)
	}

	// Exclude expired memories
	now := time.Now().UnixMilli()
	query += " AND (expires_at IS NULL OR expires_at > ?)"
	args = append(args, now)

	query += " ORDER BY created_at DESC"
	if limit > 0 {
		query += fmt.Sprintf(" LIMIT %d", limit)
	}

	rows, err := db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var entries []Entry
	for rows.Next() {
		var e Entry
		var source sql.NullString
		if err := rows.Scan(&e.ID, &e.ChatID, &e.Category, &e.Content, &source, &e.CreatedAt, &e.ExpiresAt); err != nil {
			return nil, err
		}
		if source.Valid {
			e.Source = source.String
		}
		entries = append(entries, e)
	}
	return entries, rows.Err()
}

// Delete removes a memory entry by ID.
func Delete(db *sql.DB, id int64) error {
	_, err := db.Exec("DELETE FROM memory WHERE id = ?", id)
	return err
}

// CleanExpired removes all expired memory entries.
func CleanExpired(db *sql.DB) (int64, error) {
	result, err := db.Exec("DELETE FROM memory WHERE expires_at IS NOT NULL AND expires_at < ?", time.Now().UnixMilli())
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}

// FormatForContext formats recent memories into a string for Claude's system prompt.
func FormatForContext(db *sql.DB, chatID string, maxEntries int) string {
	entries, err := Query(db, chatID, "", maxEntries)
	if err != nil || len(entries) == 0 {
		return ""
	}

	var sb strings.Builder
	sb.WriteString("## Relevant memories\n\n")
	for _, e := range entries {
		sb.WriteString(fmt.Sprintf("- [%s] %s\n", e.Category, e.Content))
	}
	return sb.String()
}

// ExtractAndSave uses Claude to extract key information from a conversation and save as memories.
func ExtractAndSave(ctx context.Context, db *sql.DB, cfg *config.Config, chatID string, conversationText string) error {
	prompt := fmt.Sprintf(`Analyze the following conversation and extract key facts, preferences, or important information that should be remembered for future conversations.

For each memory, output one line in this format:
CATEGORY: content

Valid categories: preference, fact, goal, event, insight

Only extract genuinely important information. If nothing is worth remembering, output NONE.

Conversation:
---
%s
---`, conversationText)

	resp, err := claude.Invoke(ctx, cfg, claude.DefaultSingleTurn(prompt, "You are a memory extraction assistant. Be concise and precise."))
	if err != nil {
		return fmt.Errorf("claude invoke for memory extraction: %w", err)
	}
	if resp.Error != "" {
		return fmt.Errorf("claude error: %s", resp.Error)
	}

	if strings.TrimSpace(resp.Text) == "NONE" {
		return nil
	}

	lines := strings.Split(resp.Text, "\n")
	saved := 0
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || line == "NONE" {
			continue
		}
		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 {
			continue
		}
		category := strings.TrimSpace(strings.ToLower(parts[0]))
		content := strings.TrimSpace(parts[1])
		if content == "" {
			continue
		}

		if err := Save(db, chatID, category, content, "auto-extract", nil); err != nil {
			log.Printf("[WARN] Failed to save memory: %v", err)
			continue
		}
		saved++
	}

	if saved > 0 {
		log.Printf("[INFO] Extracted and saved %d memories for chat %s", saved, chatID)
	}
	return nil
}
