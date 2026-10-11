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
  enabled: boolean
  timeout: string
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

// Process API
export const getProcesses = () => api.get<{ data: ProcessInfo[] }>('/processes')
export const getProcess = (name: string) => api.get<{ data: ProcessInfo }>(`/processes/${name}`)
export const startProcess = (name: string) => api.post(`/processes/${name}/start`)
export const stopProcess = (name: string) => api.post(`/processes/${name}/stop`)
export const restartProcess = (name: string) => api.post(`/processes/${name}/restart`)

// System API
export const getSystemOverview = () => api.get<{ data: SystemOverview }>('/system/overview')
export const getHealth = () => api.get('/system/health')

// Schedule API
export const getSchedules = () => api.get<{ data: ScheduleInfo[] }>('/schedules')
export const getSchedule = (name: string) => api.get<{ data: ScheduleInfo }>(`/schedules/${name}`)
export const runSchedule = (name: string) => api.post(`/schedules/${name}/run`)
export const getScheduleHistory = (name: string, limit = 50) =>
  api.get<{ data: HistoryEntry[] }>(`/schedules/${name}/history?limit=${limit}`)

// Config API
export const getConfig = () => api.get('/config')
export const reloadConfig = () => api.post('/config/reload')

export default api