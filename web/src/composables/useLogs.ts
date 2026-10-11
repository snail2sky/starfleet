import { ref, onUnmounted, watch, type Ref } from 'vue'
import { getProcessLogs, getAllLogs, getServerLogs, SERVER_SOURCE, type LogEntry } from '@/api'

/**
 * Polls the logs API on an interval and keeps a bounded buffer.
 * The process ref accepts: a process name, '' / null (all processes),
 * or SERVER_SOURCE (starfleet's own logs).
 */
export function useLogs(options: {
  /** Process name, SERVER_SOURCE, or null/'' for logs from all processes */
  process: Ref<string | null>
  /** Poll interval in ms (default 2000) */
  interval?: number
  /** Max lines kept client-side (default 500) */
  maxLines?: number
  /** Lines fetched per request (default 300) */
  fetchLines?: number
  /** Auto-scroll sink: called whenever new entries arrive */
  onEntry?: (entries: LogEntry[]) => void
}) {
  const entries = ref<LogEntry[]>([])
  const loading = ref(false)
  const error = ref('')
  const lastFetch = ref<Date | null>(null)

  const interval = options.interval ?? 2000
  const maxLines = options.maxLines ?? 500
  const fetchLines = options.fetchLines ?? 300

  let timer: ReturnType<typeof setInterval> | null = null
  let fetching = false

  async function refresh() {
    if (fetching) return
    fetching = true
    loading.value = true
    try {
      const proc = options.process.value
      let res
      if (proc === SERVER_SOURCE) {
        res = await getServerLogs(fetchLines)
      } else if (proc) {
        res = await getProcessLogs(proc, fetchLines)
      } else {
        res = await getAllLogs(fetchLines)
      }
      entries.value = res.data.data ?? []
      if (entries.value.length > maxLines) {
        entries.value = entries.value.slice(-maxLines)
      }
      error.value = ''
      lastFetch.value = new Date()
      options.onEntry?.(entries.value)
    } catch (e: any) {
      error.value = e?.message ?? String(e)
    } finally {
      fetching = false
      loading.value = false
    }
  }

  function start() {
    stop()
    refresh()
    timer = setInterval(refresh, interval)
  }

  function stop() {
    if (timer) {
      clearInterval(timer)
      timer = null
    }
  }

  // Re-fetch immediately when the selected process changes.
  watch(options.process, () => {
    entries.value = []
    refresh()
  })

  onUnmounted(stop)

  return { entries, loading, error, lastFetch, refresh, start, stop }
}
