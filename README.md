# 🚀 Starfleet

A modern process management and scheduling platform — a drop-in replacement
for supervisord with a web dashboard, a terminal UI and a scriptable CLI.

[![CI](https://github.com/snail2sky/starfleet/actions/workflows/ci.yml/badge.svg)](https://github.com/snail2sky/starfleet/actions/workflows/ci.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/snail2sky/starfleet)](https://goreportcard.com/report/github.com/snail2sky/starfleet)
[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)
[![Release](https://img.shields.io/github/v/release/snail2sky/starfleet?label=release&color=blue)](https://github.com/snail2sky/starfleet/releases/latest)

Starfleet takes over a box: long-running processes under supervision, cron
jobs on a schedule, logs you can actually read, and configuration you can
change without a restart. Three interfaces share one engine — a web
dashboard, a keyboard-driven TUI, and a CLI that scripts well.

## Features

**Process supervision**
- Full lifecycle: start / stop / restart, per-process user switching
- supervisord-compatible semantics: `startsecs` / `startretries` /
  `autorestart` (`true` / `false` / `unexpected`) / `exitcodes`
- Crash-loop protection with exponential backoff (`restart_delay`, `backoff_limit`)
- Boot-start toggling (`autostart`) from the UI — persisted to the config file
- Log rotation exactly like supervisord (`maxbytes` + `backups`)
- Health checks over HTTP / TCP with automatic restart

**Scheduling**
- Standard 5-field cron and `@every` syntax
- Enable/disable jobs without deleting them, run them on demand
- Execution history with exit codes, duration and captured output
- Two job types: `exec` (shell line, or direct exec via `args`) and `http`
  (method/url/headers/body; `>=400` and transport errors fail the run)
- Per-job `environment` / `directory` / `user` — same semantics as processes;
  the environment overlays (not replaces) the inherited one

**Interfaces**
- Web dashboard (Vue 3, embedded in the binary) — dark UI, auto-refresh,
  login gate when `server.auth` is enabled
- TUI (bubbletea) with the same columns and keyboard shortcuts
- CLI for servers and scripts (auth credentials from config, flags or env)
- Real-time log streaming over WebSocket, plus polling fallbacks

**Operations**
- Hot reload of the whole configuration (file watcher, API, or `SIGHUP`) —
  auth changes apply live, un-reloadable fields warn that a restart is needed
- Split configuration: main file + `conf_dir` / `conf.d` / `includes`
- Webhook notifications on process and schedule events
- Optional web UI (`server.web.enabled: false`) for headless deployments
- JSON system overview plus a Prometheus `/metrics` endpoint (system gauges,
  per-process state, per-schedule enablement)

## Quick Start

### Install

Prebuilt binaries for **linux/amd64, linux/arm64, linux/armv7, darwin/amd64,
darwin/arm64** are attached to every [release](https://github.com/snail2sky/starfleet/releases)
together with `checksums.txt`.

```bash
VER=v1.2.1
curl -sL -o starfleet.tar.gz \
  "https://github.com/snail2sky/starfleet/releases/download/${VER}/starfleet_${VER}_linux_amd64.tar.gz"
echo "<sha256 from checksums.txt>" | sha256sum -c
tar xzf starfleet.tar.gz
sudo install -m 755 starfleet_${VER}_linux_amd64/starfleet /usr/local/bin/
starfleet version
```

Build from source instead (Go 1.24+, Node 20+):

```bash
git clone https://github.com/snail2sky/starfleet.git
cd starfleet
make            # builds the frontend, then the binary into bin/
```

### Configure

```bash
sudo mkdir -p /etc/starfleet/conf.d
sudo cp configs/starfleet.yml /etc/starfleet/starfleet.yml
# put one process per file in conf.d/, see docs/configuration.md
sudo vim /etc/starfleet/starfleet.yml
starfleet validate          # check config before the first run
```

### Run

```bash
# systemd (recommended)
sudo cp deploy/systemd/starfleet.service /etc/systemd/system/
sudo systemctl enable --now starfleet
sudo systemctl status starfleet

# or in the foreground
starfleet serve -c /etc/starfleet/starfleet.yml
```

Open <http://server:8080> for the dashboard, or `starfleet tui` on the box.
Every release tarball also ships a **runnable demo** under `examples/demo/`
(main config + a managed process + a cron schedule) — see its README.

### The `stf` shortcut

`deploy/stf` is a wrapper that bakes in the binary and config paths so you
can skip them:

```bash
sudo install -m 755 deploy/stf /usr/local/bin/stf

stf status        # instead of: starfleet -c /etc/starfleet/starfleet.yml status
stf tui
stf logs -f
```

Edit the two paths at the top of `/usr/local/bin/stf` to match your install.

## CLI

```
starfleet                           # 裸跑 = 终端 UI（连着服务就接管它，没服务就独立运行）
starfleet serve                     # run the API server + managed processes
starfleet -d                        # 兼容旧用法：裸跑 -d 仍等价于 serve --daemon
starfleet tui                       # 与裸跑相同（显式写法）
starfleet validate                  # validate configuration and exit
starfleet status                    # process states of the running server
starfleet start|stop|restart NAME   # process lifecycle
starfleet logs [NAME] [-n N] [-f]   # starfleet's own logs, or one process's
starfleet schedules                 # list scheduled jobs
starfleet run NAME                  # trigger a schedule now
starfleet reload                    # ask the server to hot-reload
starfleet version
```

Global flags: `-c/--config` (config path), `--server` (control commands
against a remote server). Full reference: `--help` on any subcommand.

## Configuration

> 📖 **Full reference: [docs/configuration.md](docs/configuration.md)** — every
> supported field, file layout (`conf_dir` / `conf.d` / `includes`), reload
> behaviour, and a supervisord field mapping.

```yaml
# /etc/starfleet/starfleet.yml
conf_dir: conf.d            # managed process/schedule fragments live here

server:
  host: "0.0.0.0"
  port: 8080
  auth:
    enabled: true
    username: admin
    password: "your-password"   # plain text; WebUI shows a login page
    secret: "at-least-32-chars-random-string"
  web:
    enabled: true           # false = headless (API + TUI only)

processes: {}               # or keep them inline — both work

schedules:
  backup:
    cron: "0 3 * * *"
    command: "/opt/backup.sh"
    timeout: 300s
    enabled: true
    # 与 processes 同语义的可选项：
    # environment: { HTTPS_PROXY: "http://127.0.0.1:7890" }
    # directory: /opt
    # user: deploy
    # args: ["--json"]        # 有 args 时直接 exec（不经 shell）
```

A typical managed config, one file per service in `conf.d/`:

```yaml
# /etc/starfleet/conf.d/nginx.yml
processes:
  nginx:
    command: "nginx -g 'daemon off;'"
    directory: /etc/nginx
    autostart: true
    autorestart: "true"
    startsecs: 3
    stdout:
      path: /var/log/nginx/out.log
      maxbytes: "100M"
      backups: 5
    healthcheck:
      type: http
      url: "http://127.0.0.1/health"
      interval: 30s
```

`starfleet validate` prints where every process came from, so you always
know which file to edit:

```
✓ configuration is valid: /etc/starfleet/starfleet.yml
  conf dir:  /etc/starfleet/conf.d (conf_dir)
  loaded 4 config file(s):
    - /etc/starfleet/starfleet.yml
    - /etc/starfleet/conf.d/nginx.yml
  processes: 2
```

## API

All endpoints live under `/api/v1`. Optional auth via `server.auth`: the
API accepts **HTTP Basic** or a **Bearer JWT** issued by
`POST /api/v1/auth/login` (what the WebUI login page uses). CLI commands
read the same credentials from the config file.

### Processes

| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/processes` | List all processes |
| GET | `/processes/:name` | Runtime info **and** full config |
| POST | `/processes/:name/start` \| `/stop` \| `/restart` | Lifecycle |
| POST | `/processes/:name/enable` \| `/disable` | Boot start, persisted to config |
| GET | `/processes/:name/logs?lines=200&stream=stderr` | Captured output |

### Schedules

| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/schedules` | List jobs (includes next fire time) |
| GET | `/schedules/:name` | One job |
| POST | `/schedules/:name/run` | Trigger now |
| GET | `/schedules/:name/history?limit=50` | Execution history |
| POST | `/schedules/:name/enable` \| `/disable` | Schedule / pause, persisted to config |

### System, logs and config

| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/system/overview` | CPU / memory / disk / load / host |
| GET | `/system/health` | Liveness probe |
| GET | `/logs?lines=200` | Aggregated logs from all processes |
| GET | `/server/logs?lines=200` | Starfleet's own logs |
| GET | `/config` | Effective configuration (secrets masked) |
| POST | `/config/reload` | Hot reload |
| GET | `/metrics` | Prometheus 文本格式（系统 + 进程状态 + 调度开关） |
| WS | `/ws/logs`, `/ws/events` | Real-time streams |

## Web UI ↔ TUI parity

Both interfaces render the same columns in the same order, so switching
between them costs nothing.

| | Web UI | TUI |
|---|--------|-----|
| Process columns | Name · State · Boot · PID · Uptime · Retries · Autorestart · Command | same |
| Schedule columns | Job · Schedule · Type · Enabled · Next run · Timeout · Command | same |
| Full process details | click row → dialog | `Enter` (Processes tab) |
| Process logs | Logs tab | `l` |
| Starfleet's own logs | Logs page (default source) | `0` |
| Start / Stop / Restart | row buttons | `s` / `t` / `r` |
| Toggle boot start | Boot switch | `e` |
| Change autorestart | Autorestart cell (click to cycle) | `R` |
| Schedule details (cron/type/command·url/args/directory/user/最近执行) | click row → dialog | `Enter` (Schedules tab) |
| Trigger a schedule | Run now | `r` |
| Enable/disable schedule | Enabled switch | `e` |
| Execution history | History | `h`（列表或详情页） |
| Today's runs (daily view) | Today panel + column | `d` (Schedules tab) |
| System overview | Overview page | — |
| Config status | Settings page | — |

TUI 通用键：`←` 在子页面（详情/日志/历史/今日）**返回上级**、在列表里切上一个 tab；
`→` 同理；`Tab` / `1` `2` `3` 切标签页；`esc` 返回；`q` 退出。

`Boot` = starts automatically with starfleet (`autostart`);
`Autorestart` = restart policy when the process exits.

## ipwatch（已拆分为独立项目）

`ipwatch`（内网设备在线追踪）已从本仓库拆出，独立发布在
**[snail2sky/ipwatch](https://github.com/snail2sky/ipwatch)**——
它与进程管理无关，不应捆绑在 starfleet 的发布包里。

仍可把它交给 starfleet 托管：

```yaml
processes:
  ipwatch:
    command: "/usr/local/bin/ipwatch -subnet 192.168.1.0/24 -db /data0/apps/ipwatch/ipwatch.db"
    directory: "/data0/apps/ipwatch"
    user: orangepi
    autostart: true
```

## Docker

```bash
docker compose -f deploy/docker-compose.yml up -d
```

## Development

```bash
make frontend   # build the web UI into internal/api/dist
make test       # go test ./... -race
make lint       # golangci-lint
make build      # embed + compile into bin/starfleet
make run        # build then run with configs/starfleet.yml
```

CI runs on every push: lint, unit tests (race detector) and a matrix build
for linux/amd64 + linux/arm64. Tags build release binaries for five
platforms plus `checksums.txt`.

## Architecture

```
┌──────────────────────────────────────────────────────────┐
│                    Interfaces                             │
│      Vue 3 web UI (embedded)   ·   bubbletea TUI   ·   CLI │
├──────────────────────────────────────────────────────────┤
│                 Gin REST + WebSocket                     │
├───────────┬───────────┬─────────────┬────────────────────┤
│  Process  │ Scheduler │Health check │  Event bus +       │
│  engine   │ (robfig)  │  HTTP/TCP   │  self log ring    │
├───────────┴───────────┴─────────────┴────────────────────┤
│   config loader (conf_dir/conf.d/includes) + watcher      │
└──────────────────────────────────────────────────────────┘
```

## Migrating from supervisord

Field-by-field mapping is in
[docs/configuration.md](docs/configuration.md#9-与-supervisord-字段对照迁移参考).
The differences worth knowing:

- `environment` is a YAML map, not a comma-separated string
- `stdout_logfile*` become the nested `stdout:` / `stderr:` objects
- Starfleet writes the configured log files itself and rotates them
- `user:` actually switches uid/gid (starfleet must run as root for that)
- `autorestart` is an explicit string; restart backoff is configurable
- Hot reload replaces restart-to-apply-config

## License

MIT License — see [LICENSE](LICENSE).