# AutoCat

A lightweight personal AI assistant powered by Claude CLI with Telegram integration, scheduled tasks, and memory system.

## Project Structure

```
cmd/autocat/main.go        - Entry point, orchestrates all components
internal/
  config/config.go          - Configuration loading from .env with validation
  db/db.go                  - SQLite database initialization and migrations
  claude/claude.go          - Claude CLI subprocess invocation
  telegram/bot.go           - Telegram bot with command handling and chat
  session/session.go        - Session lifecycle (create, resume, daily reset)
  memory/memory.go          - Memory storage, retrieval, and auto-extraction
  scheduler/scheduler.go    - Cron-based task scheduler
  tasks/templates.go        - Built-in task templates (briefing, stocks, etc.)
  security/security.go      - Rate limiting, input sanitization, user allowlist
```

## Key Design Decisions

- **Claude CLI** is invoked as a subprocess (not via API) — requires `claude` to be installed and authenticated on the host
- **SQLite** for all persistence (messages, sessions, tasks, memory)
- **Single binary** — no container isolation, relies on host security + user allowlist
- **Cron scheduler** uses 6-field cron expressions (with seconds)
- Messages and code are in English; the assistant responds in the user's language

## Build & Run

```bash
make build    # Build binary
make run      # Build and run
make dev      # Run with go run (no build step)
```

## Adding Features

- New task templates: add to `internal/tasks/templates.go`
- New Telegram commands: add case to `handleCommand()` in `internal/telegram/bot.go`
- New memory categories: just use any string — no enum needed
- Database schema changes: add to `migrate()` in `internal/db/db.go`
