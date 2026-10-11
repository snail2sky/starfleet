<template>
  <div class="logs">
    <h1>Logs</h1>
    <div class="log-controls">
      <el-select v-model="selectedProcess" placeholder="Select process" clearable>
        <el-option v-for="p in processes" :key="p.name" :label="p.name" :value="p.name" />
      </el-select>
      <el-button @click="connectWs" :type="connected ? 'success' : 'primary'">
        {{ connected ? 'Connected' : 'Connect' }}
      </el-button>
      <el-button @click="clearLogs">Clear</el-button>
    </div>
    <div class="log-container" ref="logContainer">
      <div v-for="(entry, i) in logEntries" :key="i" class="log-line" :class="entry.stream">
        <span class="log-process">{{ entry.process }}</span>
        <span class="log-stream">{{ entry.stream }}</span>
        <span class="log-text">{{ entry.line }}</span>
      </div>
      <div v-if="logEntries.length === 0" class="log-empty">
        No log entries. Connect to start streaming.
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted, nextTick } from 'vue'
import { getProcesses, type ProcessInfo } from '@/api'

interface LogEntry {
  process: string
  stream: string
  line: string
}

const processes = ref<ProcessInfo[]>([])
const selectedProcess = ref('')
const logEntries = ref<LogEntry[]>([])
const connected = ref(false)
const logContainer = ref<HTMLElement | null>(null)
let ws: WebSocket | null = null

function connectWs() {
  if (ws) {
    ws.close()
    ws = null
    connected.value = false
    return
  }

  const protocol = location.protocol === 'https:' ? 'wss:' : 'ws:'
  ws = new WebSocket(`${protocol}//${location.host}/api/v1/ws/logs`)

  ws.onopen = () => {
    connected.value = true
  }

  ws.onmessage = (event) => {
    try {
      const msg = JSON.parse(event.data)
      if (msg.type === 'log') {
        const entry = msg.payload as LogEntry
        if (!selectedProcess.value || entry.process === selectedProcess.value) {
          logEntries.value.push(entry)
          if (logEntries.value.length > 1000) {
            logEntries.value = logEntries.value.slice(-500)
          }
          nextTick(() => {
            if (logContainer.value) {
              logContainer.value.scrollTop = logContainer.value.scrollHeight
            }
          })
        }
      }
    } catch (e) {
      // ignore parse errors
    }
  }

  ws.onclose = () => {
    connected.value = false
    ws = null
  }

  ws.onerror = () => {
    connected.value = false
  }
}

function clearLogs() {
  logEntries.value = []
}

onMounted(async () => {
  try {
    const { data } = await getProcesses()
    processes.value = data.data
  } catch (e) {
    console.error('Failed to fetch processes:', e)
  }
})
</script>

<style scoped>
.logs h1 { color: #38bdf8; margin-bottom: 20px; }
.log-controls { display: flex; gap: 10px; margin-bottom: 15px; }
.log-container {
  background: #1e293b;
  border: 1px solid #334155;
  border-radius: 8px;
  padding: 15px;
  height: calc(100vh - 250px);
  overflow-y: auto;
  font-family: 'Courier New', monospace;
  font-size: 13px;
}
.log-line { margin: 2px 0; line-height: 1.4; }
.log-process { color: #a78bfa; margin-right: 8px; }
.log-stream { color: #64748b; margin-right: 8px; font-size: 11px; }
.log-text { color: #e2e8f0; }
.log-line.stderr .log-text { color: #f87171; }
.log-empty { color: #64748b; text-align: center; margin-top: 50px; }
</style>