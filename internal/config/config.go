package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/joho/godotenv"
)

type Config struct {
	TelegramBotToken      string
	AllowedUsers          []string
	ClaudeModel           string
	AssistantName         string
	Timezone              string
	DataDir               string
	MaxConcurrentSessions int
	SessionIdleTimeout    int
	DailyResetHour        int
	Debug                 bool
}

func Load() (*Config, error) {
	// Load .env file if present (ignore error if missing)
	_ = godotenv.Load()

	token := os.Getenv("TELEGRAM_BOT_TOKEN")
	if token == "" {
		return nil, fmt.Errorf("TELEGRAM_BOT_TOKEN is required")
	}

	allowedRaw := os.Getenv("ALLOWED_TELEGRAM_USERS")
	if allowedRaw == "" {
		return nil, fmt.Errorf("ALLOWED_TELEGRAM_USERS is required")
	}
	allowed := parseCSV(allowedRaw)

	cfg := &Config{
		TelegramBotToken:      token,
		AllowedUsers:          allowed,
		ClaudeModel:           envOrDefault("CLAUDE_MODEL", "claude-sonnet-4-6"),
		AssistantName:         envOrDefault("ASSISTANT_NAME", "AutoCat"),
		Timezone:              envOrDefault("TIMEZONE", "Asia/Shanghai"),
		DataDir:               envOrDefault("DATA_DIR", "./data"),
		MaxConcurrentSessions: envOrDefaultInt("MAX_CONCURRENT_SESSIONS", 2),
		SessionIdleTimeout:    envOrDefaultInt("SESSION_IDLE_TIMEOUT", 300),
		DailyResetHour:        envOrDefaultInt("DAILY_RESET_HOUR", 4),
		Debug:                 envOrDefault("DEBUG", "false") == "true",
	}

	if cfg.ClaudeModel != "claude-sonnet-4-6" && cfg.ClaudeModel != "claude-opus-4-6" {
		return nil, fmt.Errorf("CLAUDE_MODEL must be claude-sonnet-4-6 or claude-opus-4-6")
	}

	if cfg.DailyResetHour < 0 || cfg.DailyResetHour > 23 {
		return nil, fmt.Errorf("DAILY_RESET_HOUR must be 0-23")
	}

	return cfg, nil
}

func (c *Config) IsUserAllowed(userID int64) bool {
	id := strconv.FormatInt(userID, 10)
	for _, allowed := range c.AllowedUsers {
		if allowed == id {
			return true
		}
	}
	return false
}

func parseCSV(s string) []string {
	parts := strings.Split(s, ",")
	result := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			result = append(result, p)
		}
	}
	return result
}

// EnvOrDefault returns the value of an env var, or the fallback.
func EnvOrDefault(key, fallback string) string {
	return envOrDefault(key, fallback)
}

func envOrDefault(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func envOrDefaultInt(key string, fallback int) int {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return fallback
	}
	return n
}
