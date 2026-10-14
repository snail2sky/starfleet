# Starfleet - Project Documentation

## Overview
Starfleet is a process management and scheduling platform (Go backend, Vue 3
frontend): a supervisord replacement with a web dashboard, a TUI, a CLI, real-time
logs, health checks and hot reload. Current release: **v1.1.0**.

## Tech Stack
- **Backend**: Go 1.24+, Gin, robfig/cron/v3, gopsutil/v4, coder/websocket,
  google/shlex, spf13/cobra
- **Frontend**: Vue 3 + `<script setup>`, Vite, Pinia, Vue Router, Element Plus
- **Terminal UI**: bubbletea / bubbles / lipgloss
- **Testing**: stdlib testing + table-driven tests; `stretchr/testify` not used
- **CI/CD**: GitHub Actions (ci.yml per push; release.yml per `v*` tag → 5 platform binaries)

## Project Structure
- `cmd/starfleet/` — CLI (cobra): main.go (server/TUI/validate/status), control.go (start/stop/restart/reload/logs/run/schedules)
- `cmd/ipwatch/` — standalone LAN presence tracker: sweeps a /24 (parallel ping + `ip neigh`), records online/offline intervals + raw sightings in SQLite (`internal/ipwatch/`); deployed as a starfleet-managed process
- `internal/engine/` — process state machine, manager, log collector, file log rotation, health checks
- `internal/config/` — YAML loading (conf_dir/conf.d/includes), watcher, in-place field persistence (yaml.Node)
- `internal/scheduler/` — cron jobs, enable/disable, history ring
- `internal/api/` — Gin routes, handlers, WS hub, embedded SPA (static.go)
- `internal/client/` — HTTP client shared by TUI (remote) and CLI control commands
- `internal/selflog/` — starfleet's own logs mirrored into a ring buffer
- `internal/tui/` — bubbletea UI (local + remote managers behind one interface)
- `internal/event/`, `internal/notify/` — event bus, webhooks
- `web/` — Vue 3 frontend; build output is embedded at `internal/api/dist`
- `configs/` — example config + `conf.d/` sample fragment
- `deploy/` — Dockerfile, docker-compose, systemd unit, `stf` wrapper
- `docs/configuration.md` — **authoritative** config reference

## Build Commands
```bash
make            # frontend + binary into bin/
make frontend   # build web → internal/api/dist
make test       # go test ./... -race
make lint
make run        # build + run with configs/starfleet.yml
```

## Key Design Decisions
- Process state machine: STOPPED → STARTING → RUNNING → STOPPING → FATAL/BACKOFF/EXITED
- supervisord-compatible retry semantics: only runs shorter than `startsecs`
  accumulate toward FATAL; a process that started OK restarts forever with
  capped backoff
- Single binary, frontend embedded via `go:embed` (keep
  `internal/api/dist/.gitkeep` so fresh clones compile)
- Cache policy: index.html `no-store`, hashed `/assets/*` `immutable`
- Config hot reload: fingerprint of all loaded files; reloads **both**
  processes and schedules; `server.auth` hot-applies via `middleware.AuthState`
  (atomic), `GET /config` reads disk fresh, un-reloadable server fields
  (host/port/web/logging/notifications) log a `WARN ... require a restart`
- Auth: `server.auth` guards `/api/v1` with Basic **or** Bearer JWT
  (`POST /api/v1/auth/login`, WebUI login page); `/health` reports the
  `auth` flag unauthenticated; CLI/TUI creds: `--user/--password` >
  `$STARFLEET_USER/$STARFLEET_PASSWORD` > config `auth.*`; empty
  `secret` auto-generates a random JWT key (forgery-safe); login is
  rate-limited per IP (10 fails / 5 min); CORS + WebSocket are
  same-origin only; `SetTrustedProxies(nil)` (no XFF spoofing)
- Enable/disable and autorestart policy changes write the single value back
  to the defining config file (yaml.Node round-trip keeps comments)
- Stable ordering everywhere (never iterate maps for display lists)

## Testing
```bash
go test ./... -race
go test ./internal/config/ -v        # config loading + persistence
go test ./internal/scheduler/ -v    # cron + enable/disable
```

## Common Tasks
- New process field: `internal/config/schema.go` → update
  `docs/configuration.md` + `TestFullConfigFields` **in the same commit**
- New API endpoint: handler in `internal/api/handler/`, register in `router.go`,
  add client method in `internal/client/client.go`
- New TUI feature: keys in `internal/tui/model.go`, keep the web UI in parity
  (README parity table)
- Release: tag `v1.1.1` etc. — `release.yml` builds the matrix