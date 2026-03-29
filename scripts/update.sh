#!/usr/bin/env bash
# Run this script on the EC2 instance to update AutoCat to the latest version.
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
AUTOCAT_DIR="$(dirname "$SCRIPT_DIR")"

echo "=== AutoCat Update ==="
cd "$AUTOCAT_DIR"

echo "Pulling latest code..."
git pull

echo "Building..."
CGO_ENABLED=1 go build -o autocat ./cmd/autocat

echo "Restarting service..."
sudo systemctl restart autocat

echo "Done. Status:"
sudo systemctl status autocat --no-pager -l
