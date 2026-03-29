package gws

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"time"
)

// IsInstalled checks if the gws CLI is available on the system.
func IsInstalled() bool {
	_, err := exec.LookPath("gws")
	return err == nil
}

// Run executes a gws command and returns the raw output.
func Run(ctx context.Context, args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "gws", args...)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return "", fmt.Errorf("gws timed out")
		}
		return "", fmt.Errorf("gws error: %s", truncate(stderr.String(), 500))
	}

	return stdout.String(), nil
}

// GmailTriage returns an unread inbox summary.
func GmailTriage(ctx context.Context) (string, error) {
	return Run(ctx, "gmail", "+triage")
}

// GmailRead reads a specific email by message ID.
func GmailRead(ctx context.Context, messageID string) (string, error) {
	return Run(ctx, "gmail", "+read", "--message-id", messageID)
}

// CalendarAgenda returns today's upcoming events.
func CalendarAgenda(ctx context.Context, timezone string) (string, error) {
	if timezone != "" {
		return Run(ctx, "calendar", "+agenda", "--timezone", timezone)
	}
	return Run(ctx, "calendar", "+agenda")
}

// TasksList returns all tasks from the default task list.
func TasksList(ctx context.Context) (string, error) {
	return Run(ctx, "tasks", "tasklists", "list")
}

// TasksGet returns tasks from a specific task list.
func TasksGet(ctx context.Context, taskListID string) (string, error) {
	return Run(ctx, "tasks", "tasks", "list", "--tasklist", taskListID)
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
