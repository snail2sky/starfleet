# Starfleet 配置文件参考

本文档是 starfleet 配置格式的**唯一权威参考**。任何配置字段的新增 / 修改 / 删除，
必须同步更新本文档、单元测试（`internal/config/`）与示例配置（`configs/`）。

---

## 1. 配置文件组织

### 1.1 加载顺序（后者覆盖前者）

```
主配置文件（starfleet.yml）
  → includes: 列出的文件/目录（按声明顺序）
  → conf_dir: 托管配置目录（未设置时自动探测主配置旁的 conf.d/）
```

同名进程 / 调度任务按此顺序**后者覆盖前者**，便于"基础配置 + 局部覆盖"的写法。

### 1.2 三种组织方式

**方式一：单文件**（适合进程少的场景）

```yaml
# starfleet.yml —— 服务器 + 进程 + 调度全写在一个文件
server: { port: 8080 }
processes:
  web: { command: "nginx -g 'daemon off;'" }
```

**方式二：conf.d 目录自动发现**（默认约定）

```
/etc/starfleet/
├── starfleet.yml        # 只写 server / logging / notifications
└── conf.d/              # 自动加载此目录下所有 .yml / .yaml
    ├── nginx.yml        # 只写 processes: …
    ├── redis.yml
    └── schedules.yml    # 只写 schedules: …
```

只要 `conf.d/` 与主配置同级且存在，就会被自动加载，**无需任何声明**。

**方式三：conf_dir 显式指定**（推荐用于托管配置与主配置分离的部署）

```yaml
# starfleet.yml
conf_dir: /data0/apps/starfleet/managed   # 绝对路径；相对路径基于主配置所在目录
```

- `conf_dir` 设置后**必须存在**，不存在则启动报错（避免拼写错误静默失效）；
- 相对路径相对于主配置文件所在目录解析；
- 会**取代** conf.d 自动发现。

### 1.3 includes 显式引入

```yaml
includes:
  - extra.yml              # 相对主配置的文件
  - /etc/starfleet/sites/  # 也可以是目录（递归加载其中 .yml/.yaml）
```

### 1.4 热加载

主配置、includes、conf_dir 中任何文件变更后，starfleet **5 秒内自动检测并应用**：

- 新增进程 → 自动启动（autostart=true 时）
- 删除进程 → 优雅停止
- 修改进程定义 → 重启该进程（未变更的进程不受影响）
- 修改 `server.auth` → 认证立即生效（无需重启）
- 也可手动触发：`starfleet reload`、`kill -HUP <pid>`、`POST /api/v1/config/reload`

---

## 2. server — 服务器配置

| 字段 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| `host` | string | `0.0.0.0` | API/WebUI 监听地址 |
| `port` | int | `8080` | 监听端口（1–65535，非法值启动报错） |
| `read_timeout` | duration | `30s` | HTTP 读超时（Go duration 语法：`30s`、`2m`） |
| `write_timeout` | duration | `30s` | HTTP 写超时 |
| `auth.enabled` | bool | `false` | 是否启用 API Basic Auth |
| `auth.username` | string | — | 认证用户名 |
| `auth.password` | string | — | 认证密码 |
| `auth.secret` | string | — | JWT 签名密钥 |
| `web.enabled` | bool | `true` | 是否提供内置 Web UI。设为 `false` 则只运行 API + TUI（无头模式，适合纯命令行/远程管控） |

```yaml
server:
  host: "0.0.0.0"
  port: 8080
  read_timeout: 30s
  write_timeout: 30s
  auth:
    enabled: true
    username: admin
    password: "your-password"
    secret: "至少32字符的随机串"
```

启用 `auth.enabled` 后：

- **WebUI**：打开页面会跳转到登录页，凭证通过
  `POST /api/v1/auth/login` 换取 24h JWT，后续请求携带
  `Authorization: Bearer <token>`；`GET /health`（无认证）会返回
  `{"auth": true/false}`，前端据此决定是否需要登录。
- **API**：接受 Basic Auth 或 Bearer JWT 两种方式（`/health` 与登录端点除外）。
- **CLI/TUI**：自动读取配置中的 `auth.username`/`auth.password`；
  也可以用 `--user` / `--password` 参数或 `STARFLEET_USER` /
  `STARFLEET_PASSWORD` 环境变量覆盖（优先级：参数 > 环境变量 > 配置）。
- **热加载**：`auth.*` 修改后 `starfleet reload`（或最长 5s 的配置 watcher）
  **立即生效**；`server.host`/`server.port`/`server.web.enabled`、`logging`、
  `notifications` 的变更无法热应用，reload 时会打 `WARN ... require a
  restart` 日志，需要重启服务。

---

## 3. processes — 进程定义

每个键是**进程名**（全局唯一，建议用 `a-z0-9-`）。

### 3.1 字段总表

| 字段 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| `command` | string | **必填** | 启动命令，POSIX shell 词法解析（支持单/双引号、反斜杠转义、空参数） |
| `directory` | string | — | 工作目录（chdir 后执行） |
| `user` | string | 当前用户 | 以哪个用户运行（等价 supervisord 的 `user=`）。切换到**其他**用户要求 starfleet 以 root 运行；与 starfleet 同用户时无特权要求 |
| `autostart` | bool | `true` | starfleet 启动时是否自动拉起 |
| `autorestart` | string | `unexpected` | `true`=总是重启 / `false`=不重启 / `unexpected`=非预期退出码才重启。可在 WebUI 点击单元格、TUI 按 `R` 循环修改（即时生效并写回配置文件） |
| `startsecs` | int | `1` | 启动后运行满该秒数才算成功 |
| `startretries` | int | `3` | **连续启动失败**（进程未运行满 startsecs 就退出）达到该次数进入 FATAL。成功启动后再崩溃的进程按 `autorestart` 无限重试（退避递增，上限 backoff_limit）— 与 supervisord 语义一致，网络抖动型服务不会被放弃 |
| `stopsignal` | string | `SIGTERM` | 停止时发送的信号（SIGTERM/SIGINT/SIGQUIT/SIGHUP/SIGKILL/SIGUSR1/SIGUSR2） |
| `stopwaitsecs` | int | `10` | 等待优雅退出秒数，超时发 SIGKILL |
| `exitcodes` | []int | `[0]` | 预期的正常退出码（配合 `autorestart: unexpected`） |
| `environment` | map | — | 环境变量 `KEY: value` |
| `envfile` | string | — | 环境变量文件路径 |
| `restart_delay` | duration | `5s` | 重启退避基数（第 N 次重试延迟 = N × restart_delay） |
| `backoff_limit` | duration | `300s` | 退避延迟上限 |
| `stdout` | object | — | stdout 日志配置（见 3.2） |
| `stderr` | object | — | stderr 日志配置（见 3.2） |
| `healthcheck` | object | — | 健康检查（见 3.3） |
| `depends_on` | []string | — | 依赖的进程名 |
| `labels` | map | — | 自定义标签（前端分组/筛选用） |

### 3.2 日志配置（stdout / stderr 各自独立）

| 字段 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| `path` | string | — | 日志文件路径。进程输出同时写入该文件（超过 `maxbytes` 按 supervisord 规则轮转为 `.1`…`.N`），并进入内存环形缓冲（可在 WebUI/TUI/`starfleet logs` 查看） |
| `maxbytes` | string | `50M` | 单文件大小上限（支持 `K/M/G`） |
| `backups` | int | `10` | 轮转保留份数 |

### 3.3 健康检查（healthcheck）

| 字段 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| `type` | string | **必填** | `http` / `tcp` / `process` |
| `url` | string | — | http 检查的 URL |
| `address` | string | — | tcp 检查的 `host:port` |
| `interval` | duration | — | 检查间隔 |
| `timeout` | duration | — | 单次检查超时 |
| `retries` | int | — | 连续失败达该次数触发告警事件 |

### 3.4 进程状态机

```
STOPPED ──start()──→ STARTING ──→ RUNNING
                        │            │
                未满 startsecs  运行后退出
                就退出(启动失败)   （含网络抖动）
                        ↓            ↓
               startFailures++    BACKOFF（无限重试，
                        │          退避递增至 backoff_limit）
                        ↓
              FATAL（连续启动失败 ≥ startretries）
```

- **FATAL 只由"启动失败"触发**：进程必须活过 `startsecs` 才算启动成功；
  成功启动后又退出的进程会一直按退避策略重启（这是 frpc 等网络服务需要的行为）。
- 从 FATAL / STOPPED 手动 `start` 会重置失败计数。

### 3.5 完整示例

```yaml
processes:
  prometheus:
    command: "/data0/apps/prometheus/prometheus --config.file=/etc/prom.yml"
    directory: /data0/apps/prometheus
    user: prometheus
    autostart: true
    autorestart: "true"
    startsecs: 3
    startretries: 5
    stopsignal: SIGTERM
    stopwaitsecs: 15
    environment:
      GOMAXPROCS: "2"
    stdout:
      path: /var/log/prometheus/out.log
      maxbytes: "100M"
      backups: 10
    stderr:
      path: /var/log/prometheus/err.log
      maxbytes: "50M"
      backups: 5
    healthcheck:
      type: http
      url: "http://127.0.0.1:9090/-/healthy"
      interval: 30s
      timeout: 5s
      retries: 3
    labels:
      group: monitoring
```

---

## 4. schedules — 定时调度任务

每个键是**任务名**（全局唯一）。

| 字段 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| `cron` | string | **必填** | 标准 5 段 cron（`分 时 日 月 周`）或 `@every 1h` 语法 |
| `type` | string | `exec` | `exec`=执行命令 / `http`=发 HTTP 请求 |
| `command` | string | exec 必填 | 要执行的 shell 命令 |
| `url` | string | http 必填 | HTTP 请求地址 |
| `method` | string | `GET` | HTTP 方法 |
| `timeout` | duration | `60s` | 单次执行超时 |
| `enabled` | bool | `true` | 是否启用（false=暂停调度，保留定义） |
| `labels` | map | — | 自定义标签 |

```yaml
schedules:
  db_backup:
    cron: "0 3 * * *"          # 每天 03:00
    command: "/opt/backup.sh"
    timeout: 300s
    enabled: true

  health_ping:
    cron: "@every 5m"          # 每 5 分钟
    type: http
    url: "http://127.0.0.1:8080/health"
    method: GET
    timeout: 10s
```

执行历史通过 `starfleet schedules` / WebUI Schedules 页 / `GET /api/v1/schedules/:name/history` 查看，
手动触发：`starfleet run <name>`。

---

## 5. logging — starfleet 自身日志

| 字段 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| `level` | string | `info` | `debug` / `info` / `warn` / `error` |
| `format` | string | `json` | `json` / `text` |
| `output` | string | `stdout` | `stdout` / `stderr` / 文件路径 |

自身日志始终同时保留在内存环形缓冲中，可随时通过
WebUI Logs 页（选择 `★ starfleet (server)`）、TUI（按 `0`）、
`starfleet logs`、`GET /api/v1/server/logs` 查看。

---

## 6. notifications — 事件通知

### 6.1 webhook

| 字段 | 类型 | 说明 |
|------|------|------|
| `enabled` | bool | 是否启用 |
| `url` | string | POST JSON 到该地址 |
| `events` | []string | 触发事件：`process_started` / `process_exited` / `process_fatal` / `health_check_fail` / `schedule_complete` / `config_reloaded` |

### 6.2 email

| 字段 | 类型 | 说明 |
|------|------|------|
| `enabled` | bool | 是否启用 |
| `smtp_host` / `smtp_port` | string / int | SMTP 服务器 |
| `from` / `to` | string | 发件人 / 收件人 |

---

## 7. 完整示例（单文件）

```yaml
# starfleet.yml
conf_dir: managed           # 托管配置目录（可选，见 1.2 方式三）
includes: []                # 额外引入的文件（可选，见 1.3）

server:
  host: "0.0.0.0"
  port: 8080
  auth:
    enabled: false

processes:
  my-app:
    command: "/opt/app/server.js --port 3000"
    directory: /opt/app
    autostart: true
    autorestart: "true"
    startsecs: 5
    environment:
      NODE_ENV: production
    stdout:
      path: /var/log/my-app/out.log
      maxbytes: "100M"
      backups: 5
    healthcheck:
      type: http
      url: "http://127.0.0.1:3000/health"
      interval: 30s
      timeout: 5s
      retries: 3

schedules:
  cleanup:
    cron: "0 4 * * 0"
    command: "find /tmp -mtime +7 -delete"
    timeout: 120s
    enabled: true

logging:
  level: info
  format: json
  output: stdout

notifications:
  webhook:
    enabled: false
    url: "https://hooks.example.com/x"
    events: [process_fatal, health_check_fail]
```

---

## 8. 配置校验

启动前可离线校验（列出全部进程/调度并检查字段合法性）。
`validate` 会**明示托管配置的来源**（conf_dir 或自动发现的 conf.d）以及
实际加载的每一个配置文件，方便排查"某个进程定义来自哪里"：

```bash
starfleet validate -c starfleet.yml
```

```
✓ configuration is valid: /etc/starfleet/starfleet.yml
  server:    0.0.0.0:8080
  auth:      false
  conf dir:  /etc/starfleet/conf.d (auto-discovered conf.d)
  loaded 3 config file(s):
    - /etc/starfleet/starfleet.yml
    - /etc/starfleet/sites/extra.yml
    - /etc/starfleet/conf.d/nginx.yml
  processes: 2
    - nginx                     auto    autorestart=true
  schedules: 1
    - cleanup                   enabled  0 4 * * 0
```

同样的信息可通过 `GET /api/v1/config` 查看（`loaded_files` / `conf_dir_resolved` 字段）。

校验规则（启动也会执行）：

- `server.port` 必须在 1–65535
- 每个进程的 `command` 必填；`autorestart` ∈ {`true`,`false`,`unexpected`}
- `healthcheck.type` ∈ {`http`,`tcp`,`process`}
- 每个调度的 `cron` 必填；`exec` 类型必须有 `command`，`http` 类型必须有 `url`

---

## 9. 与 supervisord 字段对照（迁移参考）

| supervisord | starfleet | 备注 |
|-------------|-----------|------|
| `command` | `command` | starfleet 用 POSIX shell 词法解析 |
| `directory` | `directory` | 相同 |
| `user` | `user` | 相同 |
| `autostart` | `autostart` | 相同 |
| `autorestart` | `autorestart` | 显式字符串 `true/false/unexpected` |
| `startsecs` | `startsecs` | 相同 |
| `startretries` | `startretries` | 相同 |
| `stopsignal` | `stopsignal` | 相同 |
| `stopwaitsecs` | `stopwaitsecs` | 相同 |
| `exitcodes` | `exitcodes` | 数组 |
| `environment` | `environment` | supervisord 是逗号分隔字符串，starfleet 是 map |
| `stdout_logfile` | `stdout.path` | starfleet 用嵌套对象 |
| `stdout_logfile_maxbytes` | `stdout.maxbytes` | 支持 `100M` 写法 |
| `stdout_logfile_backups` | `stdout.backups` | 相同 |
| `stderr_logfile*` | `stderr.*` | 同上 |
| — | `healthcheck` | supervisord 无 |
| — | `depends_on` | supervisord 无 |
| — | `labels` | supervisord 无 |
