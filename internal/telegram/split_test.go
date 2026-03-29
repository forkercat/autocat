package telegram

import (
	"strings"
	"testing"
)

func TestSplitMessage_Short(t *testing.T) {
	chunks := splitMessage("hello", 100)
	if len(chunks) != 1 || chunks[0] != "hello" {
		t.Errorf("expected [hello], got %v", chunks)
	}
}

func TestSplitMessage_ExactLength(t *testing.T) {
	text := strings.Repeat("a", 100)
	chunks := splitMessage(text, 100)
	if len(chunks) != 1 {
		t.Errorf("expected 1 chunk, got %d", len(chunks))
	}
}

func TestSplitMessage_SplitAtNewline(t *testing.T) {
	text := strings.Repeat("a", 50) + "\n" + strings.Repeat("b", 50)
	chunks := splitMessage(text, 60)
	if len(chunks) != 2 {
		t.Fatalf("expected 2 chunks, got %d: %v", len(chunks), chunks)
	}
	if chunks[0] != strings.Repeat("a", 50) {
		t.Errorf("chunk[0] = %q, expected 50 a's", chunks[0])
	}
	if chunks[1] != strings.Repeat("b", 50) {
		t.Errorf("chunk[1] = %q, expected 50 b's", chunks[1])
	}
}

func TestSplitMessage_HardCutNoNewline(t *testing.T) {
	text := strings.Repeat("x", 200)
	chunks := splitMessage(text, 100)
	if len(chunks) != 2 {
		t.Fatalf("expected 2 chunks, got %d", len(chunks))
	}
	if len(chunks[0]) != 100 {
		t.Errorf("expected chunk[0] length 100, got %d", len(chunks[0]))
	}
	if len(chunks[1]) != 100 {
		t.Errorf("expected chunk[1] length 100, got %d", len(chunks[1]))
	}
}

func TestSplitMessage_NoInfiniteLoop(t *testing.T) {
	// Regression test: no newline, all content — must not loop forever
	text := strings.Repeat("z", 500)
	chunks := splitMessage(text, 100)
	total := 0
	for _, c := range chunks {
		total += len(c)
	}
	if total != 500 {
		t.Errorf("expected total length 500, got %d", total)
	}
	if len(chunks) != 5 {
		t.Errorf("expected 5 chunks, got %d", len(chunks))
	}
}

func TestSplitMessage_Empty(t *testing.T) {
	chunks := splitMessage("", 100)
	if len(chunks) != 1 || chunks[0] != "" {
		t.Errorf("expected [\"\"], got %v", chunks)
	}
}

func TestSplitMessage_NewlineAtStart(t *testing.T) {
	// Regression: newline at position 0 should not cause idx=0 infinite loop
	text := "\n" + strings.Repeat("a", 200)
	chunks := splitMessage(text, 100)
	total := 0
	for _, c := range chunks {
		total += len(c)
	}
	// Content after trimming newlines
	if total == 0 {
		t.Error("expected non-empty content")
	}
}
