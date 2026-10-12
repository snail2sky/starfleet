import axios from 'axios'

const api = axios.create({
  baseURL: '/api/v1',
  timeout: 10000,
})

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