package metrics

import (
	"encoding/json"
	"log"
	"net/http"
	"sync"
	"sync/atomic"
	"time"
)

// Counters tracks application metrics.
type Counters struct {
	MessagesReceived atomic.Int64
	MessagesSent     atomic.Int64
	ClaudeInvocations atomic.Int64
	ClaudeErrors     atomic.Int64
	TasksExecuted    atomic.Int64
	TasksFailed      atomic.Int64
	Unauthorized     atomic.Int64
	RateLimited      atomic.Int64

	mu       sync.Mutex
	startedAt time.Time
}

// Global metrics instance
var global = &Counters{startedAt: time.Now()}

// Get returns the global metrics counters.
func Get() *Counters {
	return global
}

// snapshot is the JSON-serializable form of Counters.
type snapshot struct {
	Uptime            string `json:"uptime"`
	MessagesReceived  int64  `json:"messages_received"`
	MessagesSent      int64  `json:"messages_sent"`
	ClaudeInvocations int64  `json:"claude_invocations"`
	ClaudeErrors      int64  `json:"claude_errors"`
	TasksExecuted     int64  `json:"tasks_executed"`
	TasksFailed       int64  `json:"tasks_failed"`
	Unauthorized      int64  `json:"unauthorized"`
	RateLimited       int64  `json:"rate_limited"`
}

// Snapshot returns a point-in-time copy of all counters.
func (c *Counters) Snapshot() snapshot {
	return snapshot{
		Uptime:            time.Since(c.startedAt).Round(time.Second).String(),
		MessagesReceived:  c.MessagesReceived.Load(),
		MessagesSent:      c.MessagesSent.Load(),
		ClaudeInvocations: c.ClaudeInvocations.Load(),
		ClaudeErrors:      c.ClaudeErrors.Load(),
		TasksExecuted:     c.TasksExecuted.Load(),
		TasksFailed:       c.TasksFailed.Load(),
		Unauthorized:      c.Unauthorized.Load(),
		RateLimited:       c.RateLimited.Load(),
	}
}

// ServeHTTP implements http.Handler, returning JSON metrics.
func (c *Counters) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(c.Snapshot())
}

// StartServer starts a simple HTTP server for metrics on the given address.
// Returns immediately. Intended to be called from main().
func StartServer(addr string) {
	mux := http.NewServeMux()
	mux.Handle("/metrics", global)
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	})

	server := &http.Server{Addr: addr, Handler: mux}
	go func() {
		log.Printf("[INFO] Metrics server listening on %s", addr)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Printf("[WARN] Metrics server error: %v", err)
		}
	}()
}
