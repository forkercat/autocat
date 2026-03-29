# AutoCat — Developer Guide

A lightweight personal AI assistant powered by Claude CLI with Telegram integration, scheduled tasks, and memory system.

## Project Structure

```
cmd/autocat/main.go             Entry point — wires all components, runs event loop
internal/
  config/config.go              .env loading, validation, typed Config struct
  config/config_test.go         Tests for parsing, defaults, validation edge cases
  db/db.go                      SQLite init, connection pool, schema migrations
  claude/claude.go              Claude CLI subprocess invocation (single + multi-turn)
  telegram/bot.go               Telegram long-polling bot, command router, chat handler
  telegram/split_test.go        Tests for message chunking logic
  session/session.go            Session CRUD (transactional create, daily reset)
  session/session_test.go       Tests with in-memory SQLite
  memory/memory.go              Memory CRUD, context formatting, Claude-based extraction
  personalize/personalize.go    Per-chat custom instructions (CRUD on personalization table)
  skills/skills.go              Built-in reusable prompt templates (translate, summarize, etc.)
  gws/gws.go                    Google Workspace CLI (gws) subprocess wrapper
  scheduler/scheduler.go        Cron scheduler with cancellable context and WaitGroup
  tasks/templates.go            Built-in task templates (briefing, stocks, news, etc.)
  security/security.go          Rate limiter, input sanitization
  security/audit.go             Structured audit logging for security events
  security/security_test.go     Tests for sanitization, rate limiting, stop behavior
  metrics/metrics.go            JSON /metrics + /health HTTP endpoints
docs/DEPLOY.md                  Step-by-step deployment guide (EC2, Mac Mini, Docker)
scripts/
  autocat.service               Systemd unit file with security hardening
  setup.sh                      Local setup script
  deploy.sh                     Cross-compile helper for linux/arm64
```

## Key Design Decisions

1. **Claude CLI as subprocess** — not the Anthropic API. Requires `claude` installed and authenticated on the host (`claude login`). This simplifies auth (OAuth, no API keys) and gives access to Claude CLI features like session resume.

2. **SQLite for everything** — messages, sessions, scheduled tasks, task run logs, memory, personalization. Single file, no external database to manage. Connection pool configured (25 open, 5 idle, 5min lifetime).

3. **Single binary** — `go build` produces one binary. No container isolation. Security relies on the Telegram user allowlist + OS-level permissions.

4. **6-field cron** — uses `robfig/cron/v3` with seconds field enabled. Example: `0 0 8 * * *` = daily at 08:00:00.

5. **Transactional session rotation** — session create (end old + insert new) is wrapped in a SQL transaction to prevent concurrent requests from creating multiple active sessions.

6. **Graceful shutdown** — both the Telegram bot and scheduler track in-flight work via `sync.WaitGroup`. On SIGINT/SIGTERM, the bot stops accepting new messages, waits for handlers to finish, and the scheduler cancels its context and waits for running tasks.

7. **Audit logging** — security-relevant events (unauthorized access, rate limiting, commands) are logged in a structured `[AUDIT]` format for grep-ability.

8. **External CLIs as subprocess** — both Claude CLI and Google Workspace CLI (`gws`) are invoked as subprocesses. GWS integration is opt-in (`GWS_ENABLED=true`) and gracefully degrades if `gws` is not installed.

## Build & Run

```bash
make build        # CGO_ENABLED=1 go build -o autocat ./cmd/autocat
make run          # Build then run
make dev          # go run (no binary)
make test         # go test ./...
make lint         # go vet ./...

# Cross-compile for deployment
make build-linux-arm64
make build-linux-amd64
```

## Adding Features

### New task template
Add a function to `internal/tasks/templates.go` returning a `Template` struct, and include it in `Builtin()`. It will automatically appear in `/addtask`.

### New Telegram command
Add a `case` to `handleCommand()` in `internal/telegram/bot.go`. Follow the existing pattern (parse args, call internal logic, reply with `b.replyText`).

### New memory category
Just use any string as the category in `memory.Save()`. No enum or schema change needed.

### Database schema change
Add new `CREATE TABLE` or `CREATE INDEX` statements to `migrate()` in `internal/db/db.go`. SQLite `IF NOT EXISTS` makes migrations idempotent.

### New skill
Add a function to `internal/skills/skills.go` returning a `Skill` struct, and include it in `Builtin()`. Use `{input}` as the placeholder in the prompt template. It will be auto-discovered by `/skill` and as a direct command alias.

### New GWS command
Add a wrapper function to `internal/gws/gws.go`, then add a `case` to `handleCommand()` in `bot.go` guarded by `b.cfg.GWSEnabled`. Use `b.summarizeAndReply()` to pass raw gws output through Claude for formatting.

### New metrics counter
Add an `atomic.Int64` field to the `Counters` struct in `internal/metrics/metrics.go`, add it to the `snapshot` struct and `Snapshot()` method, then increment with `metrics.Get().YourCounter.Add(1)`.

## Code Conventions

- All code and comments in English
- The assistant responds in the user's language (controlled via system prompt)
- Errors: return `fmt.Errorf("context: %w", err)` with wrapping; log at the point of handling, not at every level
- Logging: `[INFO]`, `[WARN]`, `[ERROR]` prefixes; security events use `[AUDIT]`
- Tests: table-driven where appropriate, `_test.go` alongside source files
