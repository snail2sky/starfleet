<template>
  <div>
    <header class="page-head">
      <div>
        <h1 class="page-title">Schedules</h1>
        <p class="page-sub">Cron-driven tasks · trigger manually or inspect execution history</p>
      </div>
      <button class="btn" @click="fetchSchedules">Refresh</button>
    </header>

    <div class="panel">
      <el-table :data="schedules" row-key="name" v-loading="loading">
        <el-table-column label="Job" min-width="170">
          <template #default="{ row }">
            <span class="cell-name">{{ row.name }}</span>
          </template>
        </el-table-column>
        <el-table-column label="Schedule" width="150">
          <template #default="{ row }">
            <code class="cell-cron">{{ row.cron }}</code>
          </template>
        </el-table-column>
        <el-table-column label="Type" width="90">
          <template #default="{ row }">
            <span class="type-chip">{{ row.type || 'exec' }}</span>
          </template>
        </el-table-column>
        <el-table-column label="Command" min-width="240" show-overflow-tooltip>
          <template #default="{ row }">
            <code class="cell-cmd">{{ row.command || row.url }}</code>
          </template>
        </el-table-column>
        <el-table-column label="Status" width="100">
          <template #default="{ row }">
            <span class="badge" :class="row.enabled ? 'badge--running' : 'badge--idle'">
              {{ row.enabled ? 'ENABLED' : 'PAUSED' }}
            </span>
          </template>
        </el-table-column>
        <el-table-column label="Next run" width="170">
          <template #default="{ row }">
            <span class="tnum next-run">{{ row.next_run || '—' }}</span>
          </template>
        </el-table-column>
        <el-table-column label="Timeout" width="95">
          <template #default="{ row }">
            <span class="tnum">{{ row.timeout }}</span>
          </template>
        </el-table-column>
        <el-table-column label="Actions" width="210" fixed="right">
          <template #default="{ row }">
            <div class="cell-actions">
              <button class="btn btn--primary" @click="runJob(row.name)">Run now</button>
              <button class="btn" @click="viewHistory(row.name)">History</button>
            </div>
          </template>
        </el-table-column>
      </el-table>

      <div v-if="schedules.length === 0 && !loading" class="empty">
        <strong>No scheduled tasks.</strong><br />
        Add entries under <code>schedules:</code> in your config — standard cron expressions or
        <code>@every 1h</code> style intervals.
      </div>
    </div>

    <el-dialog v-model="historyVisible" :title="`History — ${historyJob}`" width="82%" top="6vh">
      <div class="hist-summary">
        {{ history.length }} recorded run<span v-if="history.length !== 1">s</span>
      </div>
      <el-table :data="history" max-height="55vh">
        <el-table-column label="Time" width="180">
          <template #default="{ row }">
            <span class="tnum">{{ new Date(row.start_time).toLocaleString() }}</span>
          </template>
        </el-table-column>
        <el-table-column label="Result" width="110">
          <template #default="{ row }">
            <span class="badge" :class="row.exit_code === 0 && !row.error ? 'badge--running' : 'badge--crit'">
              {{ row.exit_code === 0 && !row.error ? 'OK' : 'FAILED' }}
            </span>
          </template>
        </el-table-column>
        <el-table-column label="Duration" width="110">
          <template #default="{ row }">
            <span class="tnum">{{ fmtDuration(row.duration) }}</span>
          </template>
        </el-table-column>
        <el-table-column label="Error" min-width="160" show-overflow-tooltip>
          <template #default="{ row }">
            <span :class="{ 'cell-err': row.error }">{{ row.error || '—' }}</span>
          </template>
        </el-table-column>
        <el-table-column label="Output" min-width="260" show-overflow-tooltip>
          <template #default="{ row }">
            <code class="cell-cmd">{{ row.output || '—' }}</code>
          </template>
        </el-table-column>
      </el-table>
      <template #footer>
        <button class="btn" @click="historyVisible = false">Close</button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted, onUnmounted } from 'vue'
import { ElMessage } from 'element-plus'
import {
  getSchedules,
  runSchedule,
  getScheduleHistory,
  type ScheduleInfo,
  type HistoryEntry,
} from '@/api'

const schedules = ref<ScheduleInfo[]>([])
const history = ref<HistoryEntry[]>([])
const historyJob = ref('')
const historyVisible = ref(false)
const loading = ref(false)

// Auto-refresh keeps Next-run accurate and self-heals an empty table.
let refreshTimer: ReturnType<typeof setInterval> | null = null

async function fetchSchedules() {
  loading.value = true
  try {
    const { data } = await getSchedules()
    schedules.value = data.data ?? []
  } catch {
    // Silent on background refresh; the manual Refresh button surfaces errors.
    if (!schedules.value.length) {
      ElMessage.error('Failed to fetch schedules')
    }
  } finally {
    loading.value = false
  }
}

async function runJob(name: string) {
  try {
    await runSchedule(name)
    ElMessage.success(`Triggered "${name}"`)
    await fetchSchedules()
  } catch {
    ElMessage.error(`Failed to trigger "${name}"`)
  }
}

async function viewHistory(name: string) {
  try {
    const { data } = await getScheduleHistory(name, 100)
    history.value = data.data
    historyJob.value = name
    historyVisible.value = true
  } catch {
    ElMessage.error(`Failed to fetch history for "${name}"`)
  }
}

function fmtDuration(ns: number): string {
  const ms = ns / 1_000_000
  if (ms < 1000) return `${ms.toFixed(0)}ms`
  if (ms < 60_000) return `${(ms / 1000).toFixed(1)}s`
  return `${(ms / 60_000).toFixed(1)}m`
}

onMounted(() => {
  fetchSchedules()
  refreshTimer = setInterval(fetchSchedules, 30_000)
})
onUnmounted(() => {
  if (refreshTimer) clearInterval(refreshTimer)
})
</script>

<style scoped>
.page-head {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 16px;
  margin-bottom: 20px;
}
.page-title {
  margin: 0;
  font-size: 22px;
  font-weight: 650;
  color: var(--ink);
}
.page-sub {
  margin: 5px 0 0;
  font-size: 13px;
  color: var(--ink-3);
}

.cell-name { font-weight: 600; color: var(--ink); }
.cell-cron,
.cell-cmd {
  font-family: ui-monospace, Consolas, monospace;
  font-size: 11.5px;
  color: var(--ink-3);
}
.cell-cron { color: var(--accent); }
.type-chip {
  font-size: 11px;
  font-weight: 600;
  text-transform: uppercase;
  letter-spacing: 0.05em;
  color: var(--ink-3);
  background: var(--surface-2);
  border: 1px solid var(--line);
  border-radius: 4px;
  padding: 2px 7px;
}
.cell-actions { display: flex; gap: 6px; }
.tnum { font-variant-numeric: tabular-nums; color: var(--ink-2); }
.next-run { color: var(--accent); }
.cell-err { color: var(--crit); }

.hist-summary {
  font-size: 12.5px;
  color: var(--ink-3);
  margin-bottom: 12px;
}
</style>
