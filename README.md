# AutoCat

A lightweight, single-binary personal AI assistant. It connects [Claude CLI](https://docs.anthropic.com/en/docs/claude-code) to Telegram and runs scheduled tasks — daily briefings, stock analysis, news digests, English lessons, and more — with a built-in memory system that learns from your conversations.

Inspired by [NanoClaw](https://github.com/qwibitai/nanoclaw) and [ClaudeClaw](https://github.com/earlyaidopters/claudeclaw).

## How It Works

```
 You (Telegram)
      |
      v
 Telegram Bot ──> Security (allowlist, rate limit, audit)
      |
      v
 Message Handler ──> Memory (augments context)
      |
      v
 Claude CLI (subprocess, model of your choice)
      |
      v
 Response ──> Telegram
      |
 Cron Scheduler ──> Scheduled tasks (briefing, stocks, news...)
      |
 SQLite ──> Messages, sessions, tasks, memories
```

Claude CLI runs as a subprocess — not via API. This means you authenticate once with `claude login` and AutoCat inherits that session. No API keys to manage.

## Features

- **Telegram Bot** — chat with Claude via Telegram with full conversation context and memory augmentation
- **Scheduled Tasks** — 6 built-in cron-based templates (daily briefing, stock analysis, weekly finance, news digest, English learning, memory extraction), plus custom tasks
- **Memory System** — automatic extraction of key facts from conversations; memories are injected into future prompts as context
- **Session Management** — sessions auto-rotate daily at a configurable hour; each session preserves Claude CLI resume capability
- **Security** — Telegram user allowlist, per-user rate limiting (30 req/min), input sanitization, structured audit logging
- **Google Workspace** — optional integration via `gws` CLI for Gmail inbox triage, Google Calendar agenda, and Google Tasks (auto-injected into daily briefings)
- **Skills** — reusable prompt templates (`/tr`, `/sum`, `/cr`, `/eli5`, etc.) for common tasks
- **Custom Instructions** — per-chat personalized instructions injected into every prompt
- **Observability** — JSON metrics endpoint (`/metrics`) and health check (`/health`) on a configurable HTTP port
- **Single Binary** — one `go build`, one binary, zero runtime dependencies beyond Claude CLI and SQLite

## Quick Start

**Prerequisites:** Go 1.22+, [Claude CLI](https://docs.anthropic.com/en/docs/claude-code) authenticated, a [Telegram Bot Token](https://t.me/BotFather).

```bash
git clone https://github.com/forkercat/autocat.git
cd autocat
cp .env.example .env
# Edit .env: set TELEGRAM_BOT_TOKEN and ALLOWED_TELEGRAM_USERS

make build   # or: CGO_ENABLED=1 go build -o autocat ./cmd/autocat
./autocat
```

Open Telegram, find your bot, send `/start`.

## Configuration

All configuration is via environment variables or a `.env` file in the working directory.

| Variable | Description | Default |
|---|---|---|
| `TELEGRAM_BOT_TOKEN` | Telegram Bot API token | *required* |
| `ALLOWED_TELEGRAM_USERS` | Comma-separated Telegram user IDs allowed to interact | *required* |
| `CLAUDE_MODEL` | `claude-sonnet-4-6` or `claude-opus-4-6` | `claude-sonnet-4-6` |
| `ASSISTANT_NAME` | Display name used in prompts and greetings | `AutoCat` |
| `TIMEZONE` | IANA timezone for cron schedules and daily reset | `America/Los_Angeles` |
| `DATA_DIR` | Directory for SQLite database | `./data` |
| `DAILY_RESET_HOUR` | Hour (0-23) to auto-end all sessions | `4` |
| `MAX_CONCURRENT_SESSIONS` | Max parallel Claude CLI invocations | `2` |
| `SESSION_IDLE_TIMEOUT` | Session idle timeout in seconds | `300` |
| `GWS_ENABLED` | Enable Google Workspace CLI integration | `false` |
| `METRICS_ADDR` | Address for metrics/health HTTP server | `:9090` |
| `DEBUG` | Enable verbose logging | `false` |

## Telegram Commands

| Command | Description |
|---|---|
| `/start` | Welcome message and command list |
| `/new` | End current session and start a fresh one |
| `/model` | Switch model mid-session (`/model sonnet` or `/model opus`) |
| `/skill` | List and invoke skills (`/skill translate hello` or `/tr hello`) |
| `/instructions` | Set custom instructions injected into every prompt |
| `/tasks` | List all scheduled tasks with status |
| `/addtask` | Show available task templates |
| `/enable <n>` | Enable a built-in task template by number |
| `/disable <id>` | Disable a task by its database ID |
| `/memory` | Show the 10 most recent memories |
| `/status` | Current model, session ID, task count, timezone |
| `/help` | Full command reference |

Any non-command message is sent to Claude as a chat message within the current session.

## Built-in Task Templates

| # | Name | Schedule | What it does |
|---|---|---|---|
| 1 | Daily Briefing | 8:00 AM daily | Priorities, reminders, motivational quote |
| 2 | Stock Analysis | 9:30 AM weekdays | Market indices, holdings analysis, alerts |
| 3 | Weekly Finance | 10:00 AM Sunday | Weekly market review, upcoming economic events |
| 4 | News Digest | 7:00 AM daily | Top stories across tech, finance, world news |
| 5 | English Learning | 7:30 AM daily | Vocabulary, idiom, grammar tip, mini exercise |
| 6 | Memory Extract | 11:00 PM daily | Consolidate key facts from today's conversations |

All times are in your configured `TIMEZONE`. Cron expressions use 6 fields (with seconds).

## Built-in Skills

Skills are reusable prompt templates invoked on demand. Use `/skill` to list them, or invoke directly via alias:

| Skill | Alias | Description |
|---|---|---|
| `translate` | `/tr` | Translate between Chinese and English |
| `summarize` | `/sum` | Summarize text into key points |
| `explain` | `/eli5` | Explain a concept simply |
| `codereview` | `/cr` | Review code for issues |
| `rewrite` | `/rw` | Rewrite text for clarity |
| `bullets` | `/bp` | Convert text to bullet points |

Example: `/tr What is the meaning of life?` or `/skill translate What is the meaning of life?`

## Custom Instructions

Set per-chat instructions that are injected into every Claude prompt:

```
/instructions Always respond in English. I'm a senior Go developer.
/instructions clear    # remove instructions
/instructions          # view current instructions
```

## Google Workspace Integration

Optional integration with Gmail, Google Calendar, and Google Tasks via the [Google Workspace CLI (`gws`)](https://github.com/googleworkspace/cli).

**Setup:**

```bash
npm install -g @googleworkspace/cli
gws auth login
```

Then set `GWS_ENABLED=true` in `.env`.

**Commands:**

| Command | Description |
|---|---|
| `/gmail` | Inbox triage — unread email summary |
| `/calendar` (`/cal`) | Today's agenda |
| `/gtasks` | Google Tasks list |

GWS data is also automatically injected into the **daily briefing** and **weekly finance** scheduled tasks when enabled.

## Deployment

See **[docs/DEPLOY.md](docs/DEPLOY.md)** for a complete step-by-step guide covering EC2 (ARM64), systemd, Docker, and Mac Mini (launchd).

Quick reference:

```bash
# Bare metal
make build && ./autocat

# Systemd (Linux)
sudo cp scripts/autocat.service /etc/systemd/system/
sudo systemctl enable --now autocat

# Docker
make docker && docker compose up -d
```

## Observability

AutoCat exposes a lightweight HTTP server (default `:9090`):

- `GET /health` — returns `ok` (use for load balancer health checks)
- `GET /metrics` — returns JSON counters:

```json
{
  "uptime": "2h30m15s",
  "messages_received": 142,
  "messages_sent": 138,
  "claude_invocations": 138,
  "claude_errors": 2,
  "tasks_executed": 12,
  "tasks_failed": 0,
  "unauthorized": 3,
  "rate_limited": 0
}
```

## Development

```bash
make dev      # Run with `go run` (hot reload with file watcher not included)
make test     # Run all tests
make lint     # Run go vet
```

See [CLAUDE.md](CLAUDE.md) for project structure, design decisions, and guidelines for adding features.

## License

MIT
