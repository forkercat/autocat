package security

import (
	"strings"
	"testing"
)

func TestSanitizeInput_NullBytes(t *testing.T) {
	input := "hello\x00world"
	got := SanitizeInput(input)
	if strings.Contains(got, "\x00") {
		t.Error("expected null bytes to be removed")
	}
	if got != "helloworld" {
		t.Errorf("expected 'helloworld', got %q", got)
	}
}

func TestSanitizeInput_Truncation(t *testing.T) {
	input := strings.Repeat("a", 20000)
	got := SanitizeInput(input)
	if len(got) > maxMessageLength+20 { // +20 for the truncation notice
		t.Errorf("expected truncation, got length %d", len(got))
	}
	if !strings.Contains(got, "[Message truncated]") {
		t.Error("expected truncation notice")
	}
}

func TestSanitizeInput_Normal(t *testing.T) {
	input := "Hello, how are you?"
	got := SanitizeInput(input)
	if got != input {
		t.Errorf("expected %q, got %q", input, got)
	}
}

func TestRateLimiter_AllowsUnderLimit(t *testing.T) {
	rl := NewRateLimiter()
	defer rl.Stop()

	for i := 0; i < rateLimitMax; i++ {
		if !rl.Allow("user1") {
			t.Errorf("expected Allow() to return true on request %d", i)
		}
	}
}

func TestRateLimiter_BlocksOverLimit(t *testing.T) {
	rl := NewRateLimiter()
	defer rl.Stop()

	for i := 0; i < rateLimitMax; i++ {
		rl.Allow("user1")
	}

	if rl.Allow("user1") {
		t.Error("expected Allow() to return false when over limit")
	}
}

func TestRateLimiter_IndependentUsers(t *testing.T) {
	rl := NewRateLimiter()
	defer rl.Stop()

	// Fill up user1
	for i := 0; i < rateLimitMax; i++ {
		rl.Allow("user1")
	}

	// user2 should still be allowed
	if !rl.Allow("user2") {
		t.Error("expected user2 to be allowed independently of user1")
	}
}

func TestRateLimiter_StopDoesNotPanic(t *testing.T) {
	rl := NewRateLimiter()
	rl.Stop()
	// Should not panic or deadlock
}
