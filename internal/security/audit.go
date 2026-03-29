package security

import (
	"fmt"
	"log"
	"time"
)

// AuditEvent types
const (
	AuditLogin          = "login"
	AuditUnauthorized   = "unauthorized"
	AuditRateLimited    = "rate_limited"
	AuditCommand        = "command"
	AuditChat           = "chat"
	AuditTaskExecuted   = "task_executed"
	AuditSessionCreated = "session_created"
	AuditSessionReset   = "session_reset"
)

// AuditLog records a security-relevant event.
// Format: [AUDIT] timestamp | event | user | details
func AuditLog(event string, userID string, details string) {
	ts := time.Now().Format("2006-01-02T15:04:05Z07:00")
	msg := fmt.Sprintf("[AUDIT] %s | %s | user=%s | %s", ts, event, userID, details)
	log.Println(msg)
}
