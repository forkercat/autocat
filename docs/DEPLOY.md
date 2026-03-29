# Deployment Guide

Step-by-step guide for deploying AutoCat on a Linux ARM64 EC2 instance. The same steps apply to a Mac Mini — see [Mac Mini differences](#deploying-to-mac-mini) at the bottom.

## Prerequisites

Before you begin, make sure you have:

- **EC2 instance** — Ubuntu 22.04+ LTS, ARM64 (e.g. `t4g.small`). Security group: outbound 443 (HTTPS) open, inbound SSH (22) from your IP.
- **Telegram Bot Token** — create one via [@BotFather](https://t.me/BotFather)
- **Your Telegram User ID** — get it from [@userinfobot](https://t.me/userinfobot)
- **GitHub access** — SSH key or HTTPS token for pulling from `forkercat/claude-config` (optional, for settings sync)

---

## Step 1 — Connect to EC2

```bash
ssh ubuntu@<YOUR_EC2_IP>
```

## Step 2 — Install system dependencies

```bash
sudo apt-get update
sudo apt-get install -y git gcc build-essential curl ca-certificates
```

## Step 3 — Install Go

```bash
# Check https://go.dev/dl/ for the latest version
wget https://go.dev/dl/go1.22.5.linux-arm64.tar.gz
sudo tar -C /usr/local -xzf go1.22.5.linux-arm64.tar.gz
rm go1.22.5.linux-arm64.tar.gz

echo 'export PATH=$PATH:/usr/local/go/bin' >> ~/.bashrc
source ~/.bashrc

go version
# Expected: go version go1.22.5 linux/arm64
```

## Step 4 — Install Claude CLI

```bash
# Option A: via npm (recommended)
sudo apt-get install -y nodejs npm
npm install -g @anthropic-ai/claude-code

# Option B: via curl installer
curl -fsSL https://claude.ai/install.sh | sh

# Verify
claude --version
```

## Step 5 — Set up GitHub SSH key (optional)

Required only if you want to sync your Claude CLI settings via the `forkercat/claude-config` repo.

```bash
ssh-keygen -t ed25519 -C "autocat-ec2" -f ~/.ssh/id_ed25519 -N ""
cat ~/.ssh/id_ed25519.pub
```

Add the public key to GitHub: **Settings > SSH and GPG keys > New SSH key**.

```bash
ssh -T git@github.com
# Expected: Hi forkercat! You've successfully authenticated...
```

## Step 6 — Sync Claude CLI settings (optional)

Clone your `claude-config` repo to `~/.claude` so the EC2 instance shares the same plugin settings and project memories as your Mac.

```bash
# Back up any existing config from a prior `claude login`
mv ~/.claude ~/.claude.bak 2>/dev/null || true

git clone git@github.com:forkercat/claude-config.git ~/.claude
```

## Step 7 — Authenticate Claude CLI

```bash
claude login
# Follow the browser/OAuth flow

claude --version   # verify it works
```

> **Note:** Auth tokens are machine-specific and excluded from the `claude-config` repo. You must run `claude login` on every new machine.

## Step 8 — Clone and build AutoCat

```bash
git clone https://github.com/forkercat/autocat.git ~/autocat
cd ~/autocat

go mod download
CGO_ENABLED=1 go build -o autocat ./cmd/autocat

./autocat --help   # sanity check
```

## Step 9 — Configure

```bash
cp .env.example .env
nano .env
```

Minimum required configuration:

```env
TELEGRAM_BOT_TOKEN=1234567890:ABCdefGHIjklMNOpqrSTUvwxYZ
ALLOWED_TELEGRAM_USERS=123456789
```

Recommended additional settings:

```env
CLAUDE_MODEL=claude-sonnet-4-6
ASSISTANT_NAME=AutoCat
TIMEZONE=Asia/Shanghai
DATA_DIR=/home/ubuntu/autocat/data
DAILY_RESET_HOUR=4
METRICS_ADDR=:9090
```

See the full configuration reference in [README.md](../README.md#configuration).

## Step 10 — Test

```bash
cd ~/autocat
./autocat
```

Open Telegram, find your bot, send `/start`. You should get a welcome message. Send a chat message to verify Claude responds. Press `Ctrl+C` to stop.

## Step 11 — Install as a systemd service

```bash
# Adjust paths and user in the service file
sed -i "s|/home/ubuntu/autocat|$HOME/autocat|g" scripts/autocat.service
sed -i "s|User=ubuntu|User=$USER|g" scripts/autocat.service

sudo cp scripts/autocat.service /etc/systemd/system/autocat.service
sudo systemctl daemon-reload
sudo systemctl enable autocat
sudo systemctl start autocat
```

Verify:

```bash
sudo systemctl status autocat        # should show "active (running)"
journalctl -u autocat -f             # follow live logs
curl -s localhost:9090/health         # should return "ok"
curl -s localhost:9090/metrics | jq . # view counters
```

---

## Updating

### Update AutoCat code

```bash
cd ~/autocat
git pull
CGO_ENABLED=1 go build -o autocat ./cmd/autocat
sudo systemctl restart autocat
```

### Sync Claude settings from your Mac

After pushing changes to `forkercat/claude-config` from your Mac:

```bash
cd ~/.claude && git pull
```

AutoCat reads configuration at startup. Restart the service if you changed `settings.json`.

---

## Deploying to Mac Mini

The process is identical with these differences:

| Step | EC2 (Linux) | Mac Mini (macOS) |
|---|---|---|
| 2 | `apt-get install gcc ...` | Install Xcode Command Line Tools: `xcode-select --install` |
| 3 | Download `linux-arm64` tarball | Download `darwin-arm64.pkg` from [go.dev/dl](https://go.dev/dl/) |
| 4 | `npm install -g` | `brew install node && npm install -g @anthropic-ai/claude-code` |
| 11 | systemd service | launchd plist (see below) |

### launchd plist for Mac Mini

```bash
cat > ~/Library/LaunchAgents/com.autocat.plist << 'EOF'
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN"
  "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
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

# Replace YOUR_USER with your username, then load:
launchctl load ~/Library/LaunchAgents/com.autocat.plist
```

---

## Troubleshooting

| Problem | Cause | Fix |
|---|---|---|
| `claude: command not found` | npm global bin not in PATH | `export PATH=$PATH:$(npm bin -g)` and add to `~/.bashrc` |
| Build fails with CGO error | Missing C compiler | `sudo apt-get install gcc build-essential` |
| Bot doesn't respond to messages | User ID not in allowlist | Check `ALLOWED_TELEGRAM_USERS` matches your actual Telegram user ID |
| `Permission denied (publickey)` | SSH key not configured | Re-do Step 5; ensure the key is added to GitHub |
| Claude returns auth error | Token expired or missing | Run `claude login` again |
| Scheduled tasks not firing | Wrong timezone or cron syntax | Verify `TIMEZONE` in `.env`; test cron expressions at [crontab.guru](https://crontab.guru/) (note: AutoCat uses 6-field cron with a leading seconds field) |
| Metrics endpoint unreachable | Port not open or wrong address | Check `METRICS_ADDR` in `.env`; ensure security group allows the port if accessing remotely |
