package config

import (
	"os"
	"testing"
)

func clearEnv() {
	for _, key := range []string{
		"TELEGRAM_BOT_TOKEN", "ALLOWED_TELEGRAM_USERS", "CLAUDE_MODEL",
		"ASSISTANT_NAME", "TIMEZONE", "DATA_DIR", "MAX_CONCURRENT_SESSIONS",
		"SESSION_IDLE_TIMEOUT", "DAILY_RESET_HOUR", "DEBUG",
	} {
		os.Unsetenv(key)
	}
}

func setRequiredEnv() {
	os.Setenv("TELEGRAM_BOT_TOKEN", "test-token-123")
	os.Setenv("ALLOWED_TELEGRAM_USERS", "111,222")
}

func TestLoad_MissingToken(t *testing.T) {
	clearEnv()
	_, err := Load()
	if err == nil {
		t.Fatal("expected error for missing TELEGRAM_BOT_TOKEN")
	}
}

func TestLoad_MissingAllowedUsers(t *testing.T) {
	clearEnv()
	os.Setenv("TELEGRAM_BOT_TOKEN", "tok")
	_, err := Load()
	if err == nil {
		t.Fatal("expected error for missing ALLOWED_TELEGRAM_USERS")
	}
}

func TestLoad_InvalidModel(t *testing.T) {
	clearEnv()
	setRequiredEnv()
	os.Setenv("CLAUDE_MODEL", "gpt-4")
	_, err := Load()
	if err == nil {
		t.Fatal("expected error for invalid CLAUDE_MODEL")
	}
}

func TestLoad_InvalidResetHour(t *testing.T) {
	clearEnv()
	setRequiredEnv()
	os.Setenv("DAILY_RESET_HOUR", "25")
	_, err := Load()
	if err == nil {
		t.Fatal("expected error for DAILY_RESET_HOUR=25")
	}
}

func TestLoad_Defaults(t *testing.T) {
	clearEnv()
	setRequiredEnv()
	cfg, err := Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.ClaudeModel != "claude-sonnet-4-6" {
		t.Errorf("expected default model claude-sonnet-4-6, got %s", cfg.ClaudeModel)
	}
	if cfg.AssistantName != "AutoCat" {
		t.Errorf("expected default name AutoCat, got %s", cfg.AssistantName)
	}
	if cfg.Timezone != "America/Los_Angeles" {
		t.Errorf("expected default timezone America/Los_Angeles, got %s", cfg.Timezone)
	}
	if cfg.DailyResetHour != 4 {
		t.Errorf("expected default reset hour 4, got %d", cfg.DailyResetHour)
	}
	if cfg.MaxConcurrentSessions != 2 {
		t.Errorf("expected default max sessions 2, got %d", cfg.MaxConcurrentSessions)
	}
	if cfg.Debug {
		t.Error("expected debug to be false by default")
	}
}

func TestLoad_CustomValues(t *testing.T) {
	clearEnv()
	setRequiredEnv()
	os.Setenv("CLAUDE_MODEL", "claude-opus-4-6")
	os.Setenv("DAILY_RESET_HOUR", "6")
	os.Setenv("DEBUG", "true")
	os.Setenv("ASSISTANT_NAME", "MyCat")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.ClaudeModel != "claude-opus-4-6" {
		t.Errorf("expected claude-opus-4-6, got %s", cfg.ClaudeModel)
	}
	if cfg.DailyResetHour != 6 {
		t.Errorf("expected reset hour 6, got %d", cfg.DailyResetHour)
	}
	if !cfg.Debug {
		t.Error("expected debug true")
	}
	if cfg.AssistantName != "MyCat" {
		t.Errorf("expected MyCat, got %s", cfg.AssistantName)
	}
}

func TestIsUserAllowed(t *testing.T) {
	cfg := &Config{AllowedUsers: []string{"111", "222"}}

	if !cfg.IsUserAllowed(111) {
		t.Error("expected user 111 to be allowed")
	}
	if !cfg.IsUserAllowed(222) {
		t.Error("expected user 222 to be allowed")
	}
	if cfg.IsUserAllowed(333) {
		t.Error("expected user 333 to be denied")
	}
}

func TestParseCSV(t *testing.T) {
	tests := []struct {
		input string
		want  []string
	}{
		{"a,b,c", []string{"a", "b", "c"}},
		{" a , b , c ", []string{"a", "b", "c"}},
		{"single", []string{"single"}},
		{",,,", nil},
		{"a,,b", []string{"a", "b"}},
	}

	for _, tt := range tests {
		got := parseCSV(tt.input)
		if len(got) != len(tt.want) {
			t.Errorf("parseCSV(%q) = %v, want %v", tt.input, got, tt.want)
			continue
		}
		for i := range got {
			if got[i] != tt.want[i] {
				t.Errorf("parseCSV(%q)[%d] = %q, want %q", tt.input, i, got[i], tt.want[i])
			}
		}
	}
}

func TestEnvOrDefaultInt(t *testing.T) {
	os.Setenv("TEST_INT", "42")
	if v := envOrDefaultInt("TEST_INT", 0); v != 42 {
		t.Errorf("expected 42, got %d", v)
	}

	os.Setenv("TEST_INT", "invalid")
	if v := envOrDefaultInt("TEST_INT", 7); v != 7 {
		t.Errorf("expected fallback 7, got %d", v)
	}

	os.Unsetenv("TEST_INT")
	if v := envOrDefaultInt("TEST_INT", 99); v != 99 {
		t.Errorf("expected fallback 99, got %d", v)
	}
}
