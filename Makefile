.PHONY: all build test lint lint-install clean frontend dev run

VERSION ?= dev
COMMIT  ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo "none")
DATE    ?= $(shell date -u +"%Y-%m-%dT%H:%M:%SZ")
LDFLAGS  = -ldflags "-s -w -X main.version=$(VERSION) -X main.commit=$(COMMIT) -X main.date=$(DATE)"

all: frontend build

frontend:
	cd web && npm ci && npx vite build
	rm -rf internal/api/dist
	cp -r web/dist internal/api/dist

build:
	CGO_ENABLED=0 go build $(LDFLAGS) -o bin/starfleet ./cmd/starfleet/

build-linux:
	GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build $(LDFLAGS) -o bin/starfleet-linux-amd64 ./cmd/starfleet/
	GOOS=linux GOARCH=arm64 CGO_ENABLED=0 go build $(LDFLAGS) -o bin/starfleet-linux-arm64 ./cmd/starfleet/

test:
	go test ./... -race -count=1

test-coverage:
	go test ./... -race -coverprofile=coverage.out
	go tool cover -html=coverage.out -o coverage.html

# Use the first golangci-lint found: PATH, then the default go install dir.
GOLANGCI ?= $(shell command -v golangci-lint 2>/dev/null || echo "$(HOME)/go/bin/golangci-lint")

lint:
	$(GOLANGCI) run ./...

lint-install:
	go install github.com/golangci/golangci-lint/cmd/golangci-lint@v1.64.8

clean:
	rm -rf bin/ web/dist/ internal/api/dist/ coverage.out coverage.html

dev:
	cd web && npm run dev &

run: build
	./bin/starfleet -config configs/starfleet.yml

docker-build:
	docker build -t starfleet:latest .

docker-run:
	docker compose -f deploy/docker-compose.yml up -d