<template>
  <div>
    <header class="page-head">
      <div>
        <h1 class="page-title">Schedules</h1>
        <p class="page-sub">Cron-driven tasks · trigger manually or inspect execution history</p>
      </div>
      <button class="btn" @click="fetchSchedules">Refresh</button>
    </header>

    <!-- Today's activity: the daily glance view (TUI: press d) -->
    <div class="panel" v-if="daily">
      <div class="panel__head">
        Today <span class="daily__date">{{ daily.date }}</span>
        <span class="daily__stats">
          <b class="tnum">{{ daily.total }} runs</b>
          <span class="daily__ok tnum">{{ daily.ok }} ok</span>
          <span class="tnum" :class="daily.failed ? 'daily__fail' : ''">{{ daily.failed }} failed</span>
        </span>
      </div>
      <div class="panel__body" v-if="daily.total === 0">
        <div class="hint">No runs yet today — use <b>Run now</b> below or wait for the next cron fire.</div>
      </div>
      <el-table v-else :data="daily.recent" max-height="260" size="small">
        <el-table-column label="Time" width="150">
          <template #default="{ row }">
            <span class="tnum">{{ fmtTime(row.start_time) }}</span>
          </template>
        </el-table-column>
        <el-table-column label="Job" min-width="150">
          <template #default="{ row }"><span class="cell-name">{{ row.name }}</span></template>
        </el-table-column>
        <el-table-column label="Result" width="100">
          <template #default="{ row }">
            <span class="badge" :class="row.exit_code === 0 && !row.error ? 'badge--running' : 'badge--crit'">
              {{ row.exit_code === 0 && !row.error ? 'OK' : 'FAILED' }}
            </span>
          </template>
        </el-table-column>
        <el-table-column label="Duration" width="110">
          <template #default="{ row }"><span class="tnum">{{ fmtDuration(row.duration) }}</span></template>
        </el-table-column>
        <el-table-column label="Exit" width="70">
          <template #default="{ row }"><span class="tnum">{{ row.exit_code }}</span></template>
        </el-table-column>
        <el-table-column label="Error / Output" min-width="240" show-overflow-tooltip>
          <template #default="{ row }">
            <span :class="{ 'cell-err': row.error }">{{ row.error || row.output || '—' }}</span>
          </template>
        </el-table-column>
      </el-table>
    </div>

    <div class="panel">
      <el-table :data="schedules" row-key="name" v-loading="loading">
        <el-table-column label="Job" min-width="170">
          <template #default="{ row }">
            <span class="cell-name">{{ row.name }}</span>
          </template>
        </el-table-column>
        <el-table-column label="Schedule" width="140">
          <template #default="{ row }">
            <code class="cell-cron">{{ row.cron }}</code>
          </template>
        </el-table-column>
        <el-table-column label="Type" width="80">
          <template #default="{ row }">
            <span class="type-chip">{{ row.type || 'exec' }}</span>
          </template>
        </el-table-column>
        <el-table-column label="Enabled" width="100">
          <template #default="{ row }">
            <button
              class="toggle"
              :class="{ 'toggle--on': row.enabled }"
              :title="row.enabled ? 'Scheduled — click to pause' : 'Paused — click to schedule'"
              :disabled="toggling === row.name"
              @click="toggleEnabled(row)"
            >
              <span class="toggle__dot"></span>
              {{ row.enabled ? 'ON' : 'OFF' }}
            </button>
          </template>
        </el-table-column>
        <el-table-column label="Next run" width="170">
          <template #default="{ row }">
            <span class="tnum next-run">{{ row.next_run || '—' }}</span>
          </template>
        </el-table-column>
        <el-table-column label="Today" width="130">
          <template #default="{ row }">
            <span v-if="todayMap[row.name]" class="today">
              <b class="tnum" :class="todayMap[row.name].failed ? 'daily__fail' : 'daily__ok'">
                {{ todayMap[row.name].runs }}<template v-if="todayMap[row.name].failed"> ({{ todayMap[row.name].failed }}✗)</template>
              </b>
              <span class="today__last tnum">{{ fmtLastRun(todayMap[row.name]) }}</span>
            </span>
            <span v-else class="today--none">—</span>
          </template>
        </el-table-column>
        <el-table-column label="Timeout" width="95">
          <template #default="{ row }">
            <span class="tnum">{{ row.timeout }}</span>
          </template>
        </el-table-column>
        <el-table-column label="Command" min-width="220" show-overflow-tooltip>
          <template #default="{ row }">
            <code class="cell-cmd">{{ row.command || row.url }}</code>
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
import { ref, computed, onMounted, onUnmounted } from 'vue'
import { ElMessage } from 'element-plus'
import {
  getSchedules,
  runSchedule,
  setScheduleEnabled,
  getScheduleHistory,
  getScheduleDaily,
  type ScheduleInfo,
  type HistoryEntry,
  type DailyReport,
} from '@/api'

const schedules = ref<ScheduleInfo[]>([])
const daily = ref<DailyReport | null>(null)
const history = ref<HistoryEntry[]>([])
const historyJob = ref('')
const historyVisible = ref(false)
const loading = ref(false)
const toggling = ref('')

// Auto-refresh keeps Next-run accurate and self-heals an empty table.
let refreshTimer: ReturnType<typeof setInterval> | null = null

async function fetchSchedules() {
  loading.value = true
  try {
    const { data } = await getSchedules()
    schedules.value = data.data ?? []
    fetchDaily() // keep the Today view in the same refresh cycle
  } catch {
    // Silent on background refresh; the manual Refresh button surfaces errors.
    if (!schedules.value.length) {
      ElMessage.error('Failed to fetch schedules')
    }
  } finally {
    loading.value = false
  }
}

async function fetchDaily() {
  try {
    const { data } = await getScheduleDaily()
    daily.value = data.data ?? null
  } catch {
    // background refresh — keep the last report
  }
}

/** name -> today's stat, for the Today column. */
const todayMap = computed<Record<string, DailyReport['jobs'][number]>>(() => {
  const m: Record<string, DailyReport['jobs'][number]> = {}
  for (const j of daily.value?.jobs ?? []) m[j.name] = j
  return m
})

function fmtTime(iso: string): string {
  const d = new Date(iso)
  if (isNaN(d.getTime())) return '—'
  return d.toLocaleString([], {
    month: '2-digit', day: '2-digit', hour: '2-digit', minute: '2-digit', second: '2-digit',
  })
}

function fmtLastRun(j: DailyReport['jobs'][number]): string {
  const d = new Date(j.last_start)
  if (isNaN(d.getTime()) || j.last_start.startsWith('0001-01-01')) return ''
  return d.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' })
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

/** Enable/disable a schedule; the server persists it into the config file. */
async function toggleEnabled(row: ScheduleInfo) {
  toggling.value = row.name
  try {
    await setScheduleEnabled(row.name, !row.enabled)
    await fetchSchedules()
    ElMessage.success(
      row.enabled ? `"${row.name}" paused` : `"${row.name}" scheduled`,
    )
  } catch (e: any) {
    ElMessage.error(
      e?.response?.data?.warning || `Failed to change "${row.name}"`,
    )
  } finally {
    toggling.value = ''
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

/* enabled toggle (mirrors the Processes page Boot switch) */
.toggle {
  display: inline-flex;
  align-items: center;
  gap: 7px;
  height: 24px;
  padding: 0 10px;
  font-size: 11px;
  font-weight: 600;
  letter-spacing: 0.04em;
  border-radius: 999px;
  border: 1px solid var(--line);
  background: var(--surface-2);
  color: var(--ink-3);
  cursor: pointer;
  transition: background 0.15s, color 0.15s, border-color 0.15s;
}
.toggle:disabled { opacity: 0.5; cursor: wait; }
.toggle__dot {
  width: 6px;
  height: 6px;
  border-radius: 50%;
  background: currentColor;
}
.toggle--on {
  color: var(--good);
  background: var(--good-soft);
  border-color: rgba(12, 163, 12, 0.35);
}

/* daily overview panel */
.daily__date {
  font-size: 12px;
  font-weight: 500;
  color: var(--ink-3);
  margin-left: 8px;
}
.daily__stats {
  float: right;
  display: flex;
  gap: 14px;
  font-size: 12.5px;
}
.daily__ok { color: var(--good); font-weight: 600; }
.daily__fail { color: var(--crit); font-weight: 600; }

/* Today column */
.today {
  display: inline-flex;
  align-items: baseline;
  gap: 8px;
}
.today b { font-size: 12.5px; }
.today__last { font-size: 11px; color: var(--ink-3); }
.today--none { color: var(--ink-3); }
</style>
