package session

import (
	"database/sql"
	"os"
	"testing"

	_ "github.com/mattn/go-sqlite3"
)

func setupTestDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite3", ":memory:?_foreign_keys=ON")
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}
	_, err = db.Exec(`
		CREATE TABLE sessions (
			id TEXT PRIMARY KEY,
			chat_id TEXT NOT NULL,
			claude_session_id TEXT,
			started_at INTEGER NOT NULL,
			ended_at INTEGER,
			status TEXT NOT NULL DEFAULT 'active' CHECK(status IN ('active', 'ended'))
		)
	`)
	if err != nil {
		t.Fatalf("create table: %v", err)
	}
	return db
}

func TestMain(m *testing.M) {
	os.Exit(m.Run())
}

func TestCreate_NewSession(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	sess, err := Create(db, "chat1")
	if err != nil {
		t.Fatalf("create session: %v", err)
	}
	if sess.ChatID != "chat1" {
		t.Errorf("expected chatID chat1, got %s", sess.ChatID)
	}
	if sess.Status != "active" {
		t.Errorf("expected status active, got %s", sess.Status)
	}
	if sess.ID == "" {
		t.Error("expected non-empty session ID")
	}
}

func TestCreate_EndsExistingSessions(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	sess1, _ := Create(db, "chat1")
	sess2, _ := Create(db, "chat1")

	if sess1.ID == sess2.ID {
		t.Error("expected different session IDs")
	}

	// sess1 should now be ended
	var status string
	err := db.QueryRow("SELECT status FROM sessions WHERE id = ?", sess1.ID).Scan(&status)
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	if status != "ended" {
		t.Errorf("expected sess1 status ended, got %s", status)
	}

	// sess2 should be active
	err = db.QueryRow("SELECT status FROM sessions WHERE id = ?", sess2.ID).Scan(&status)
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	if status != "active" {
		t.Errorf("expected sess2 status active, got %s", status)
	}
}

func TestGetActive_CreatesIfNone(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	sess, err := GetActive(db, "newchat")
	if err != nil {
		t.Fatalf("get active: %v", err)
	}
	if sess == nil {
		t.Fatal("expected non-nil session")
	}
	if sess.Status != "active" {
		t.Errorf("expected active, got %s", sess.Status)
	}
}

func TestGetActive_ReturnsExisting(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	sess1, _ := Create(db, "chat1")
	sess2, _ := GetActive(db, "chat1")

	if sess1.ID != sess2.ID {
		t.Errorf("expected same session ID, got %s vs %s", sess1.ID, sess2.ID)
	}
}

func TestEndAllActive(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	Create(db, "chat1")
	Create(db, "chat2")

	count, err := EndAllActive(db)
	if err != nil {
		t.Fatalf("end all: %v", err)
	}
	if count != 2 {
		t.Errorf("expected 2 ended, got %d", count)
	}

	// Should now return 0
	count, _ = EndAllActive(db)
	if count != 0 {
		t.Errorf("expected 0 ended on second call, got %d", count)
	}
}

func TestUpdateClaudeSessionID(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	sess, _ := Create(db, "chat1")
	err := UpdateClaudeSessionID(db, sess.ID, "claude-123")
	if err != nil {
		t.Fatalf("update claude session: %v", err)
	}

	var csid sql.NullString
	db.QueryRow("SELECT claude_session_id FROM sessions WHERE id = ?", sess.ID).Scan(&csid)
	if !csid.Valid || csid.String != "claude-123" {
		t.Errorf("expected claude-123, got %v", csid)
	}
}
