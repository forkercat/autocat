.PHONY: build run dev clean test lint setup update deploy

build:
	CGO_ENABLED=1 go build -o autocat ./cmd/autocat

run: build
	./autocat

dev:
	go run ./cmd/autocat

clean:
	rm -f autocat autocat-*

test:
	go test ./...

lint:
	go vet ./...

setup:
	bash scripts/setup.sh

# Run on EC2 to pull latest code and restart the service
update:
	bash scripts/update.sh

# Cross-compile for Linux ARM64 (EC2)
build-linux-arm64:
	CGO_ENABLED=1 GOOS=linux GOARCH=arm64 go build -o autocat-linux-arm64 ./cmd/autocat

# Cross-compile for Linux AMD64
build-linux-amd64:
	CGO_ENABLED=1 GOOS=linux GOARCH=amd64 go build -o autocat-linux-amd64 ./cmd/autocat

docker:
	docker build -t autocat .

docker-run:
	docker compose up -d
