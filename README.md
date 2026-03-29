# AutoCat

Lightweight personal AI assistant powered by Claude CLI with Telegram integration, scheduled tasks, and memory system.

Inspired by [NanoClaw](https://github.com/qwibitai/nanoclaw) and [ClaudeClaw](https://github.com/earlyaidopters/claudeclaw).

## Features

- **Telegram Bot** — chat with Claude via Telegram with full conversation context
- **Scheduled Tasks** — cron-based tasks for daily briefings, stock analysis, news digests, English learning, and more
- **Memory System** — automatic memory extraction from conversations, with memory-augmented context for future chats
- **Session Management** — automatic daily session rotation with conversation history
- **Security** — user allowlist, rate limiting, input sanitization
- **Single Binary** — easy deployment on Linux ARM64 (EC2) or macOS (Mac Mini)

## Prerequisites

- Go 1.22+
- [Claude CLI](https://claude.ai/download) installed and authenticated (`claude login`)
- Telegram Bot Token (from [@BotFather](https://t.me/BotFather))

## Quick Start

```bash
git clone https://github.com/wjunhao/autocat.git
cd autocat

# Setup
cp .env.example .env
# Edit .env with your TELEGRAM_BOT_TOKEN and ALLOWED_TELEGRAM_USERS

# Build and run
make build
./autocat
```

## Configuration

All configuration is via environment variables (or `.env` file):

| Variable | Description | Default |
|---|---|---|
| `TELEGRAM_BOT_TOKEN` | Telegram Bot API token | (required) |
| `ALLOWED_TELEGRAM_USERS` | Comma-separated allowed user IDs | (required) |
| `CLAUDE_MODEL` | `claude-sonnet-4-6` or `claude-opus-4-6` | `claude-sonnet-4-6` |
| `ASSISTANT_NAME` | Bot display name | `AutoCat` |
| `TIMEZONE` | Timezone for schedules | `Asia/Shanghai` |
| `DATA_DIR` | Data directory path | `./data` |
| `DAILY_RESET_HOUR` | Hour to auto-reset sessions (0-23) | `4` |
| `DEBUG` | Enable debug logging | `false` |

## Telegram Commands

| Command | Description |
|---|---|
| `/start` | Welcome message |
| `/new` | Start a new conversation session |
| `/tasks` | List all scheduled tasks |
| `/addtask` | Show available task templates |
| `/enable <n>` | Enable a task template by number |
| `/disable <id>` | Disable a task by ID |
| `/memory` | View recent memories |
| `/status` | Show bot status |
| `/help` | Show help |

## Built-in Task Templates

1. **Daily Briefing** — morning summary with priorities (8:00 AM)
2. **Stock Analysis** — market overview and portfolio analysis (9:30 AM weekdays)
3. **Weekly Finance** — weekly financial review (10:00 AM Sunday)
4. **News Digest** — curated daily news (7:00 AM)
5. **English Learning** — vocabulary, idioms, grammar practice (7:30 AM)
6. **Daily Memory Extract** — consolidate memories from conversations (11:00 PM)

## Deployment

### Bare metal (EC2 / Mac Mini)

```bash
make build
# Copy binary + .env to your server
# Authenticate Claude CLI on the server: claude login
./autocat
```

### Systemd service

```bash
sudo cp scripts/autocat.service /etc/systemd/system/
sudo systemctl enable autocat
sudo systemctl start autocat
```

### Docker

```bash
make docker
docker compose up -d
```

## Architecture

```
Telegram Bot ←→ Message Handler ←→ Claude CLI (subprocess)
                     ↕                    ↕
                  SQLite DB          Session Resume
                     ↕
              Cron Scheduler → Claude CLI → Telegram
                     ↕
              Memory System
```

## License

MIT
