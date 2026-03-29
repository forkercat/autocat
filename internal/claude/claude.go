package claude

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"time"

	"github.com/wjunhao/autocat/internal/config"
)

// Response holds the result of a Claude CLI invocation.
type Response struct {
	Text      string
	SessionID string
	Error     string
}

// Invoke calls the Claude CLI with the given prompt.
// Requires `claude` CLI to be installed and authenticated on the host.
func Invoke(ctx context.Context, cfg *config.Config, opts InvokeOptions) (*Response, error) {
	args := []string{
		"--print",
		"--output-format", "json",
		"--model", cfg.ClaudeModel,
		"--max-turns", fmt.Sprintf("%d", opts.MaxTurns),
	}

	if opts.SystemPrompt != "" {
		args = append(args, "--system-prompt", opts.SystemPrompt)
	}

	if opts.SessionID != "" {
		args = append(args, "--resume", opts.SessionID)
	}

	args = append(args, "--prompt", opts.Prompt)

	timeout := opts.Timeout
	if timeout == 0 {
		timeout = 2 * time.Minute
	}

	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "claude", args...)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return &Response{Error: "Claude CLI timed out"}, nil
		}
		return &Response{
			Error: fmt.Sprintf("Claude CLI error: %s", truncate(stderr.String(), 500)),
		}, nil
	}

	// Try to parse JSON response
	var parsed struct {
		Result    string `json:"result"`
		Text      string `json:"text"`
		SessionID string `json:"session_id"`
	}

	raw := stdout.Bytes()
	if err := json.Unmarshal(raw, &parsed); err == nil {
		text := parsed.Result
		if text == "" {
			text = parsed.Text
		}
		if text == "" {
			text = string(raw)
		}
		sid := parsed.SessionID
		if sid == "" {
			sid = opts.SessionID
		}
		return &Response{Text: text, SessionID: sid}, nil
	}

	// Fallback: treat raw output as text
	return &Response{
		Text:      string(bytes.TrimSpace(raw)),
		SessionID: opts.SessionID,
	}, nil
}

// InvokeOptions configures a Claude CLI invocation.
type InvokeOptions struct {
	Prompt       string
	SystemPrompt string
	SessionID    string
	MaxTurns     int
	Timeout      time.Duration
}

// DefaultSingleTurn returns options for a single-turn invocation.
func DefaultSingleTurn(prompt, systemPrompt string) InvokeOptions {
	return InvokeOptions{
		Prompt:       prompt,
		SystemPrompt: systemPrompt,
		MaxTurns:     1,
		Timeout:      2 * time.Minute,
	}
}

// DefaultMultiTurn returns options for a multi-turn invocation.
func DefaultMultiTurn(prompt, systemPrompt, sessionID string) InvokeOptions {
	return InvokeOptions{
		Prompt:       prompt,
		SystemPrompt: systemPrompt,
		SessionID:    sessionID,
		MaxTurns:     5,
		Timeout:      5 * time.Minute,
	}
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
