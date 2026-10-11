# Starfleet - Project Documentation

## Overview
Starfleet is a modern process management and scheduling platform built with Go (backend) and Vue 3 (frontend). It's a supervisord alternative with web UI, real-time logs, health checks, and hot-reload.

## Tech Stack
- **Backend**: Go 1.23+, Gin, robfig/cron/v3, gopsutil/v4, coder/websocket
- **Frontend**: Vue 3, Vite, Pinia, Vue Router, Element Plus
- **Testing**: Go testing + testify (planned)
- **CI/CD**: GitHub Actions

## Project Structure
- `cmd/starfleet/` — Main entry point
- `internal/engine/` — Process state machine and manager
- `internal/config/` — YAML configuration loading
- `internal/scheduler/` — Cron job scheduling
- `internal/event/` — In-process event bus
- `internal/api/` — Gin HTTP API, WebSocket, middleware
- `internal/notify/` — Webhook notifications
- `web/` — Vue 3 frontend source
- `configs/` — Example configuration files
- `deploy/` — Docker, systemd deployment files

## Build Commands
```bash
# Full build (frontend + Go)
make all

# Go only (requires frontend pre-built)
make build

# Frontend only
cd web && npm run build

# Run tests
make test

# Lint
make lint
```

## Key Design Decisions
- Process state machine: STOPPED → STARTING → RUNNING → STOPPING → STOPPED/FATAL/BACKOFF
- Config hot-reload via polling (5s interval)
- WebSocket hub for real-time log/event streaming
- Single binary with embedded frontend via go:embed

## Testing
```bash
# All Go tests
go test ./...

# Specific package
go test ./internal/engine/ -v

# With coverage
go test ./... -coverprofile=coverage.out
```

## Common Tasks
- Add new process config: edit `configs/starfleet.yml` under `processes:`
- Add new API endpoint: add handler in `internal/api/handler/`, register in `internal/api/router.go`
- Add new Vue page: create in `web/src/views/`, register in `web/src/router/index.ts`