#!/usr/bin/env bash
set -euo pipefail

echo "=== AutoCat Setup ==="

# Check prerequisites
command -v go >/dev/null 2>&1 || { echo "Error: Go 1.22+ is required. Install from https://go.dev/dl/"; exit 1; }
command -v claude >/dev/null 2>&1 || { echo "Error: Claude CLI is required. Install from https://claude.ai/download"; exit 1; }

# Check Go version
GO_VERSION=$(go version | grep -oP 'go\K[0-9]+\.[0-9]+')
echo "Go version: $GO_VERSION"

# Create data directory
mkdir -p data
echo "Created data/ directory"

# Copy .env if not exists
if [ ! -f .env ]; then
    cp .env.example .env
    echo "Created .env from .env.example - please edit it with your settings"
else
    echo ".env already exists, skipping"
fi

# Install dependencies
echo "Downloading Go dependencies..."
go mod download

# Build
echo "Building autocat..."
CGO_ENABLED=1 go build -o autocat ./cmd/autocat
echo "Build successful: ./autocat"

echo ""
echo "=== Setup complete ==="
echo ""
echo "Next steps:"
echo "  1. Edit .env with your Telegram bot token and allowed user IDs"
echo "  2. Authenticate Claude CLI: claude login"
echo "  3. Run: ./autocat"
echo ""
