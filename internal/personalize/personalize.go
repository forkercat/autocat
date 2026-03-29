package personalize

import (
	"database/sql"
	"time"
)

// Get returns a personalization value by composite key (chatID:key).
func Get(db *sql.DB, chatID, key string) (string, error) {
	compositeKey := chatID + ":" + key
	var value string
	err := db.QueryRow("SELECT value FROM personalization WHERE key = ?", compositeKey).Scan(&value)
	if err == sql.ErrNoRows {
		return "", nil
	}
	return value, err
}

// Set upserts a personalization value.
func Set(db *sql.DB, chatID, key, value string) error {
	compositeKey := chatID + ":" + key
	now := time.Now().UnixMilli()
	_, err := db.Exec(
		"INSERT INTO personalization (key, value, updated_at) VALUES (?, ?, ?) ON CONFLICT(key) DO UPDATE SET value = ?, updated_at = ?",
		compositeKey, value, now, value, now,
	)
	return err
}

// Delete removes a personalization entry.
func Delete(db *sql.DB, chatID, key string) error {
	compositeKey := chatID + ":" + key
	_, err := db.Exec("DELETE FROM personalization WHERE key = ?", compositeKey)
	return err
}

// GetInstructions returns the custom instructions for a chat.
func GetInstructions(db *sql.DB, chatID string) (string, error) {
	return Get(db, chatID, "instructions")
}

// SetInstructions saves custom instructions for a chat.
func SetInstructions(db *sql.DB, chatID, instructions string) error {
	return Set(db, chatID, "instructions", instructions)
}

// ClearInstructions removes custom instructions for a chat.
func ClearInstructions(db *sql.DB, chatID string) error {
	return Delete(db, chatID, "instructions")
}
