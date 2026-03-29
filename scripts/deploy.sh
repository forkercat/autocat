#!/usr/bin/env bash
# Deploy autocat on a Linux ARM EC2 instance
set -euo pipefail

echo "=== AutoCat Deployment ==="

# Build for the target platform
GOOS=${GOOS:-linux}
GOARCH=${GOARCH:-arm64}

echo "Building for $GOOS/$GOARCH..."
CGO_ENABLED=1 GOOS=$GOOS GOARCH=$GOARCH go build -o autocat-$GOOS-$GOARCH ./cmd/autocat
echo "Built: autocat-$GOOS-$GOARCH"

echo ""
echo "To deploy to your EC2 instance:"
echo "  1. scp autocat-$GOOS-$GOARCH .env your-ec2:~/autocat/"
echo "  2. ssh your-ec2"
echo "  3. cd ~/autocat && chmod +x autocat-$GOOS-$GOARCH"
echo "  4. claude login  # authenticate Claude CLI"
echo "  5. ./autocat-$GOOS-$GOARCH"
echo ""
echo "For systemd service, create /etc/systemd/system/autocat.service"
