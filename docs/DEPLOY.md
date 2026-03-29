# AutoCat Deployment Guide

Complete step-by-step guide for deploying AutoCat on a Linux ARM64 EC2 instance.
The same steps apply to a local Mac Mini (skip the EC2-specific parts).

---

## Prerequisites

| Item | Notes |
|---|---|
| EC2 instance | Ubuntu 22.04 LTS, ARM64 (e.g. `t4g.small`) |
| Security group | Outbound 443 (HTTPS) open; inbound SSH (22) from your IP |
| Telegram Bot Token | Create via [@BotFather](https://t.me/BotFather) |
| Your Telegram User ID | Get from [@userinfobot](https://t.me/userinfobot) |
| GitHub access | SSH key or HTTPS token for pushing to `forkercat/claude-config` |

---

## Step 1 — Connect to EC2

```bash
ssh ubuntu@<YOUR_EC2_IP>
```

---

## Step 2 — Install System Dependencies

```bash
sudo apt-get update
sudo apt-get install -y git gcc build-essential curl ca-certificates
```

---

## Step 3 — Install Go 1.22

```bash
wget https://go.dev/dl/go1.22.5.linux-arm64.tar.gz
sudo tar -C /usr/local -xzf go1.22.5.linux-arm64.tar.gz
rm go1.22.5.linux-arm64.tar.gz

echo 'export PATH=$PATH:/usr/local/go/bin' >> ~/.bashrc
source ~/.bashrc

go version  # should print: go version go1.22.5 linux/arm64
```

---

## Step 4 — Install Claude CLI

```bash
npm install -g @anthropic-ai/claude-code
# or via curl installer if available:
# curl -fsSL https://claude.ai/install.sh | sh

claude --version
```

---

## Step 5 — Set Up GitHub SSH Key (for claude-config sync)

This allows the EC2 instance to pull updates to `~/.claude` from `forkercat/claude-config`.

```bash
ssh-keygen -t ed25519 -C "autocat-ec2" -f ~/.ssh/id_ed25519 -N ""
cat ~/.ssh/id_ed25519.pub
```

Copy the output and add it to GitHub:
**GitHub → Settings → SSH and GPG keys → New SSH key**

Test the connection:
```bash
ssh -T git@github.com
# Expected: Hi forkercat! You've successfully authenticated...
```

---

## Step 6 — Clone claude-config to ~/.claude (Plan A)

This syncs your Claude CLI settings and project memories from your Mac to the EC2 instance.

```bash
# If ~/.claude already exists from a previous claude login, back it up first:
mv ~/.claude ~/.claude.bak 2>/dev/null || true

git clone git@github.com:forkercat/claude-config.git ~/.claude
```

Your EC2 instance now shares the same `settings.json`, plugin config, and project memories as your Mac.

---

## Step 7 — Authenticate Claude CLI

```bash
claude login
# Follow the browser/OAuth flow to authenticate
claude --version  # verify auth works
```

> **Note:** Authentication tokens are stored in `~/.claude` but are excluded from the
> `claude-config` repo (they are machine-specific). You must run `claude login` on
> each new machine.

---

## Step 8 — Clone and Build AutoCat

```bash
git clone https://github.com/forkercat/autocat.git ~/autocat
cd ~/autocat

# Install Go dependencies
go mod download

# Build (CGO required for SQLite)
CGO_ENABLED=1 go build -o autocat ./cmd/autocat

./autocat --help  # quick sanity check
```

---

## Step 9 — Configure Environment

```bash
cp .env.example .env
nano .env   # or vim .env
```

Fill in the required values:

```env
# Required
TELEGRAM_BOT_TOKEN=1234567890:ABCdefGHIjklMNOpqrSTUvwxYZ
ALLOWED_TELEGRAM_USERS=123456789   # your Telegram user ID

# Recommended
CLAUDE_MODEL=claude-sonnet-4-6
ASSISTANT_NAME=AutoCat
TIMEZONE=Asia/Shanghai
DATA_DIR=/home/ubuntu/autocat/data
DAILY_RESET_HOUR=4
DEBUG=false
```

---

## Step 10 — Run a Quick Test

```bash
cd ~/autocat
./autocat
```

Open Telegram, find your bot, and send `/start`. You should get a welcome message.
Press `Ctrl+C` to stop.

---

## Step 11 — Install as a Systemd Service

```bash
# Edit the service file to match your paths
sed -i 's|/home/ubuntu/autocat|'"$HOME/autocat"'|g' scripts/autocat.service
sed -i 's|User=ubuntu|User='"$USER"'|g' scripts/autocat.service

sudo cp scripts/autocat.service /etc/systemd/system/autocat.service
sudo systemctl daemon-reload
sudo systemctl enable autocat
sudo systemctl start autocat
```

Check status:
```bash
sudo systemctl status autocat
journalctl -u autocat -f   # follow live logs
```

---

## Keeping Things Up to Date

### Update AutoCat (new features)

```bash
cd ~/autocat
git pull
CGO_ENABLED=1 go build -o autocat ./cmd/autocat
sudo systemctl restart autocat
```

### Sync claude-config (settings/memory from Mac)

When you update `~/.claude` on your Mac and push to `forkercat/claude-config`,
pull it on EC2:

```bash
cd ~/.claude
git pull
```

> No restart needed — AutoCat reads settings at startup only. Restart if you changed
> `settings.json` or want new memories available immediately.

---

## Deploying to Mac Mini (Future)

The steps are identical except:

1. Skip Step 2 (macOS has clang/gcc via Xcode Command Line Tools)
2. Step 3: Download `go1.22.x.darwin-arm64.pkg` from https://go.dev/dl/
3. Step 4: `brew install node && npm install -g @anthropic-ai/claude-code`
4. Step 11: Use a `launchd` plist instead of systemd:

```bash
# ~/Library/LaunchAgents/com.autocat.plist
cat > ~/Library/LaunchAgents/com.autocat.plist << 'EOF'
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
  <key>Label</key>
  <string>com.autocat</string>
  <key>ProgramArguments</key>
  <array>
    <string>/Users/YOUR_USER/autocat/autocat</string>
  </array>
  <key>WorkingDirectory</key>
  <string>/Users/YOUR_USER/autocat</string>
  <key>RunAtLoad</key>
  <true/>
  <key>KeepAlive</key>
  <true/>
  <key>StandardOutPath</key>
  <string>/Users/YOUR_USER/autocat/data/autocat.log</string>
  <key>StandardErrorPath</key>
  <string>/Users/YOUR_USER/autocat/data/autocat.error.log</string>
</dict>
</plist>
EOF

launchctl load ~/Library/LaunchAgents/com.autocat.plist
```

---

## Troubleshooting

| Problem | Fix |
|---|---|
| `claude: command not found` | Add npm global bin to PATH: `export PATH=$PATH:$(npm bin -g)` |
| `CGO_ENABLED` build fails | Install gcc: `sudo apt-get install gcc` |
| Telegram bot not responding | Check `ALLOWED_TELEGRAM_USERS` matches your actual user ID |
| `Permission denied (publickey)` on git | Re-add SSH key to GitHub (Step 5) |
| Claude returns auth error | Re-run `claude login` |
| Scheduled tasks not running | Check timezone in `.env`; verify cron expression with a cron validator |
