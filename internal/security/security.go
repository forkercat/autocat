package security

import (
	"strings"
	"sync"
	"time"
)

const (
	maxMessageLength = 10000
	rateLimitWindow  = 60 * time.Second
	rateLimitMax     = 30
)

// SanitizeInput cleans user input.
func SanitizeInput(input string) string {
	// Remove null bytes
	s := strings.ReplaceAll(input, "\x00", "")

	// Truncate overly long messages
	if len(s) > maxMessageLength {
		s = s[:maxMessageLength] + "\n[Message truncated]"
	}

	return s
}

// RateLimiter tracks per-user request rates.
type RateLimiter struct {
	mu      sync.Mutex
	windows map[string][]time.Time
	done    chan struct{}
}

// NewRateLimiter creates a new rate limiter with background cleanup.
func NewRateLimiter() *RateLimiter {
	rl := &RateLimiter{
		windows: make(map[string][]time.Time),
		done:    make(chan struct{}),
	}
	go rl.cleanupLoop()
	return rl
}

// Allow returns true if the user is within rate limits.
func (rl *RateLimiter) Allow(userID string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	cutoff := now.Add(-rateLimitWindow)

	timestamps := rl.windows[userID]
	valid := make([]time.Time, 0, len(timestamps))
	for _, t := range timestamps {
		if t.After(cutoff) {
			valid = append(valid, t)
		}
	}

	if len(valid) >= rateLimitMax {
		return false
	}

	rl.windows[userID] = append(valid, now)
	return true
}

// Stop terminates the background cleanup goroutine.
func (rl *RateLimiter) Stop() {
	close(rl.done)
}

func (rl *RateLimiter) cleanupLoop() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-rl.done:
			return
		case <-ticker.C:
			rl.cleanup()
		}
	}
}

func (rl *RateLimiter) cleanup() {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	cutoff := time.Now().Add(-rateLimitWindow)
	for userID, timestamps := range rl.windows {
		valid := make([]time.Time, 0, len(timestamps))
		for _, t := range timestamps {
			if t.After(cutoff) {
				valid = append(valid, t)
			}
		}
		if len(valid) == 0 {
			delete(rl.windows, userID)
		} else {
			rl.windows[userID] = valid
		}
	}
}
