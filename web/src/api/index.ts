import axios from 'axios'

const api = axios.create({
  baseURL: '/api/v1',
  timeout: 10000,
})

/* ---------- auth ---------- */

const TOKEN_KEY = 'starfleet_token'

export const getToken = (): string | null => localStorage.getItem(TOKEN_KEY)
export const setToken = (t: string): void => localStorage.setItem(TOKEN_KEY, t)
export const clearToken = (): void => localStorage.removeItem(TOKEN_KEY)

api.interceptors.request.use((config) => {
  const t = getToken()
  if (t) config.headers.Authorization = `Bearer ${t}`
  return config
})

api.interceptors.response.use(
  (resp) => resp,
  (err) => {
    if (err.response?.status === 401) {
      clearToken()
      if (!window.location.pathname.startsWith('/login')) {
        window.location.assign('/login')
      }
    }
    return Promise.reject(err)
  },
)

// Public (unauthenticated) client for /health.
const pub = axios.create({ baseURL: '/', timeout: 5000 })
let authMode: Promise<boolean> | null = null

/**
 * Whether the server requires login. Resolves false when auth is disabled
 * or the server is unreachable (a login page would not help then).
 * The result is cached for the lifetime of the page.
 */
export function authRequired(): Promise<boolean> {
  if (!authMode) {
    authMode = pub
      .get<{ status: string; auth: boolean }>('/health')
      .then((r) => !!r.data?.auth)
      .catch(() => false)
  }
  return authMode
}

/** Exchange credentials for a token. Throws on wrong credentials. */
export async function login(username: string, password: string): Promise<void> {
  const r = await api.post<{ token: string }>('/auth/login', { username, password })
  setToken(r.data.token)
}

export function logout(): void {
  clearToken()
}

export interface ProcessInfo {
  name: string
  state: string
  pid: number
  uptime: string
  exit_code: number
  retry_count: number
  command: string
  directory: string
  autostart: boolean
  autorestart: string
  labels: Record<string, string>
}

/** Full process configuration (snake_case json tags from Go). */
export interface ProcessConfig {
  command: string
  directory?: string
  user?: string
  autostart?: boolean
  autorestart?: string
  startsecs?: number
  startretries?: number
  stopsignal?: string
  stopwaitsecs?: number
  environment?: Record<string, string>
  envfile?: string
  stdout?: { path: string; maxbytes: string; backups: number }
  stderr?: { path: string; maxbytes: string; backups: number }
  healthcheck?: {
    type: string
    url?: string
    address?: string
    interval: number // nanoseconds
    timeout: number
    retries: number
  }
  labels?: Record<string, string>
  depends_on?: string[]
  restart_delay?: number // nanoseconds
  backoff_limit?: number // nanoseconds
}

export interface SystemOverview {
  host: { hostname: string; os: string; platform: string; kernel: string; uptime: number; go_version: string }
  cpu: { cores: number; percent: number[] }
  memory: { total: number; used: number; available: number; percent: number }
  disk: { total: number; used: number; free: number; percent: number }
  load: { load1: number; load5: number; load15: number }
}

export interface ScheduleInfo {
  name: string
  cron: string
  type: string
  command: string
  url?: string
  enabled: boolean
  timeout: string
  next_run?: string
  labels: Record<string, string>
}

export interface HistoryEntry {
  name: string
  start_time: string
  end_time: string
  duration: number
  exit_code: number
  output: string
  error: string
}

export interface LogEntry {
  process: string
  stream: string // "stdout" | "stderr"
  line: string
  timestamp: number // unix ms
}

// Process API
export const getProcesses = () => api.get<{ data: ProcessInfo[] }>('/processes')
export const getProcess = (name: string) =>
  api.get<{ data: ProcessInfo; config?: ProcessConfig }>(`/processes/${name}`)
export const startProcess = (name: string) => api.post(`/processes/${name}/start`)
export const stopProcess = (name: string) => api.post(`/processes/${name}/stop`)
export const restartProcess = (name: string) => api.post(`/processes/${name}/restart`)
export const setProcessAutostart = (name: string, enabled: boolean) =>
  api.post(`/processes/${name}/${enabled ? 'enable' : 'disable'}`)
export type AutorestartPolicy = 'true' | 'false' | 'unexpected'
export const setProcessAutorestart = (name: string, value: AutorestartPolicy) =>
  api.post(`/processes/${name}/autorestart`, { value })

// Log API
/** Special source selector value: starfleet's own server logs. */
export const SERVER_SOURCE = '__server__'

export const getProcessLogs = (name: string, lines = 200, stream?: string) =>
  api.get<{ data: LogEntry[] }>(
    `/processes/${name}/logs?lines=${lines}` + (stream ? `&stream=${stream}` : ''),
  )
export const getAllLogs = (lines = 200) =>
  api.get<{ data: LogEntry[] }>(`/logs?lines=${lines}`)
export const getServerLogs = (lines = 200) =>
  api.get<{ data: LogEntry[] }>(`/server/logs?lines=${lines}`)

/** Stream values that should render as error (server logs use levels). */
export function isErrStream(stream: string): boolean {
  return stream === 'stderr' || stream === 'ERROR' || stream === 'FATAL' || stream === 'PANIC'
}

// System API
export const getSystemOverview = () => api.get<{ data: SystemOverview }>('/system/overview')
export const getHealth = () => api.get('/system/health')

// Schedule API
export const getSchedules = () => api.get<{ data: ScheduleInfo[] }>('/schedules')
export const getSchedule = (name: string) => api.get<{ data: ScheduleInfo }>(`/schedules/${name}`)
export const runSchedule = (name: string) => api.post(`/schedules/${name}/run`)
export const setScheduleEnabled = (name: string, enabled: boolean) =>
  api.post(`/schedules/${name}/${enabled ? 'enable' : 'disable'}`)
export const getScheduleHistory = (name: string, limit = 50) =>
  api.get<{ data: HistoryEntry[] }>(`/schedules/${name}/history?limit=${limit}`)

// Config API
export const getConfig = () => api.get('/config')
export const reloadConfig = () => api.post('/config/reload')

export default api