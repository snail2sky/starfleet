# 🚀 Starfleet

A modern process management and scheduling platform built with Go and Vue 3.

[![CI](https://github.com/snail2sky/starfleet/actions/workflows/ci.yml/badge.svg)](https://github.com/snail2sky/starfleet/actions/workflows/ci.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/snail2sky/starfleet)](https://goreportcard.com/report/github.com/snail2sky/starfleet)
[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)

Starfleet is a modern alternative to supervisord, designed for managing long-running processes and scheduled tasks with a beautiful web dashboard, real-time log streaming, health checks, and hot-reload capabilities.

## Features

- **Process Management** — Start, stop, restart processes with full lifecycle control
- **Scheduled Tasks** — Cron-based scheduling with execution history
- **Web Dashboard** — Beautiful dark-themed UI built with Vue 3 + Element Plus
- **Real-time Logs** — WebSocket-based log streaming from all processes
- **Health Checks** — HTTP/TCP/process health monitoring with auto-restart
- **Hot Reload** — Update configuration without downtime
- **Event Notifications** — Webhook notifications for process events
- **Single Binary** — Frontend embedded in Go binary, zero dependencies
- **Cross Platform** — Linux (amd64/arm64), macOS support

## Quick Start

### Install

```bash
# Download latest release
curl -sL https://github.com/snail2sky/starfleet/releases/latest/download/starfleet-linux-amd64 -o starfleet
chmod +x starfleet

# Or build from source
git clone https://github.com/snail2sky/starfleet.git
cd starfleet
make
```

### Configure

```bash
# Copy default config
cp configs/starfleet.yml /etc/starfleet/starfleet.yml

# Edit configuration
vim /etc/starfleet/starfleet.yml
```

### Run

```bash
# Start with config
./starfleet -config /etc/starfleet/starfleet.yml

# Or install as systemd service
sudo cp deploy/systemd/starfleet.service /etc/systemd/system/
sudo systemctl enable --now starfleet
```

Open http://localhost:8080 in your browser.

## Configuration

```yaml
server:
  host: "0.0.0.0"
  port: 8080
  auth:
    enabled: true
    username: admin
    password: "$2a$10$..."

processes:
  nginx:
    command: "nginx -g 'daemon off;'"
    directory: /etc/nginx
    autostart: true
    autorestart: "true"
    startsecs: 3
    startretries: 5
    stdout:
      path: /var/log/nginx/stdout.log
      maxbytes: "100M"
      backups: 5
    healthcheck:
      type: http
      url: "http://127.0.0.1/health"
      interval: 30s

schedules:
  backup:
    cron: "0 3 * * *"
    command: "/opt/backup.sh"
    timeout: 300s
    enabled: true

notifications:
  webhook:
    enabled: true
    url: "https://hooks.slack.com/services/..."
    events: [process_exit, process_fatal]
```

## API

### Processes

| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/api/v1/processes` | List all processes |
| GET | `/api/v1/processes/:name` | Get process details |
| POST | `/api/v1/processes/:name/start` | Start process |
| POST | `/api/v1/processes/:name/stop` | Stop process |
| POST | `/api/v1/processes/:name/restart` | Restart process |

### Schedules

| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/api/v1/schedules` | List all schedules |
| GET | `/api/v1/schedules/:name` | Get schedule details |
| POST | `/api/v1/schedules/:name/run` | Trigger execution |
| GET | `/api/v1/schedules/:name/history` | Execution history |

### System

| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/api/v1/system/overview` | System resource overview |
| GET | `/api/v1/system/health` | Health check |
| GET | `/api/v1/config` | View configuration |
| POST | `/api/v1/config/reload` | Hot reload config |

### WebSocket

| Endpoint | Description |
|----------|-------------|
| `/api/v1/ws/logs` | Real-time log stream |
| `/api/v1/ws/events` | Real-time event stream |

## Development

```bash
# Prerequisites: Go 1.23+, Node.js 20+

# Install dependencies
make frontend

# Run tests
make test

# Build
make build

# Lint
make lint

# Run locally
make run
```

## Docker

```bash
docker compose -f deploy/docker-compose.yml up -d
```

## Architecture

```
┌─────────────────────────────────────────────────┐
│                  Vue 3 Frontend                 │
│        Dashboard │ Processes │ Schedules │ Logs  │
├─────────────────────────────────────────────────┤
│                  Gin HTTP API                   │
│           REST │ WebSocket │ Static Files        │
├──────────┬──────────┬───────────┬───────────────┤
│ Process  │ Scheduler│  Health   │    Event       │
│ Manager  │  (cron)  │  Checker  │    Bus         │
├──────────┴──────────┴───────────┴───────────────┤
│              os/exec + gopsutil                  │
└─────────────────────────────────────────────────┘
```

## License

MIT License - see [LICENSE](LICENSE) for details.