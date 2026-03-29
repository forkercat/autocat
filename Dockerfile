# Multi-stage build for autocat
# Supports linux/amd64 and linux/arm64

FROM golang:1.22-bookworm AS builder

RUN apt-get update && apt-get install -y gcc libc6-dev && rm -rf /var/lib/apt/lists/*

WORKDIR /build
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=1 go build -o autocat ./cmd/autocat

# Runtime stage
FROM debian:bookworm-slim

RUN apt-get update && \
    apt-get install -y --no-install-recommends ca-certificates curl && \
    rm -rf /var/lib/apt/lists/*

# Install Claude CLI
RUN curl -fsSL https://claude.ai/install.sh | sh || true

RUN useradd -m -s /bin/bash autocat
USER autocat
WORKDIR /home/autocat

COPY --from=builder /build/autocat /usr/local/bin/autocat

# Data volume
VOLUME /home/autocat/data

ENV DATA_DIR=/home/autocat/data

ENTRYPOINT ["autocat"]
