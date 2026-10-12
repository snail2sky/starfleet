<template>
  <div>
    <header class="page-head">
      <div>
        <h1 class="page-title">Processes</h1>
        <p class="page-sub">Click a row for full details · same information as the TUI detail view</p>
      </div>
      <div class="page-head__actions">
        <input
          class="search"
          v-model="filter"
          placeholder="Filter by name…"
          aria-label="Filter processes"
        />
        <button class="btn" @click="processStore.fetchProcesses()">Refresh</button>
      </div>
    </header>

    <div class="panel">
      <el-table
        :data="filtered"
        row-key="name"
        @row-click="(row: any) => openDetail(String(row?.name ?? ''), 'details')"
        :row-class-name="() => 'row-clickable'"
        v-loading="loading"
      >
        <el-table-column label="Process" min-width="170">
          <template #default="{ row }">
            <div class="cell-name">
              <span class="cell-name__dot" :class="dotClass(row.state)"></span>
              {{ row.name }}
            </div>
          </template>
        </el-table-column>
        <el-table-column label="State" width="120">
          <template #default="{ row }">
            <span class="badge" :class="badgeClass(row.state)">{{ row.state }}</span>
          </template>
        </el-table-column>
        <el-table-column label="PID" width="80">
          <template #default="{ row }">
            <span class="tnum">{{ row.pid || '—' }}</span>
          </template>
        </el-table-column>
        <el-table-column label="Uptime" width="140">
          <template #default="{ row }">
            <span class="tnum">{{ fmtUptime(row) }}</span>
          </template>
        </el-table-column>
        <el-table-column label="Autostart" width="105">
          <template #default="{ row }">
            <button
              class="toggle"
              :class="{ 'toggle--on': row.autostart }"
              :title="row.autostart ? 'Starts with starfleet — click to disable' : 'Manual start only — click to enable'"
              :disabled="toggling === row.name"
              @click.stop="toggleAutostart(row)"
            >
              <span class="toggle__dot"></span>
              {{ row.autostart ? 'ON' : 'OFF' }}
            </button>
          </template>
        </el-table-column>
        <el-table-column label="Retries" width="85">
          <template #default="{ row }">
            <span class="tnum">{{ row.retry_count }}</span>
          </template>
        </el-table-column>
        <el-table-column label="Command" min-width="220" show-overflow-tooltip>
          <template #default="{ row }">
            <code class="cell-cmd">{{ row.command }}</code>
          </template>
        </el-table-column>
        <el-table-column label="Actions" width="280" fixed="right">
          <template #default="{ row }">
            <div class="cell-actions" @click.stop>
              <button class="btn" :disabled="row.state === 'RUNNING'" @click="act(row.name, 'start')">
                Start
              </button>
              <button class="btn" :disabled="row.state === 'STOPPED'" @click="act(row.name, 'stop')">
                Stop
              </button>
              <button class="btn" @click="act(row.name, 'restart')">Restart</button>
              <button class="btn btn--primary" @click="openDetail(row.name, 'logs')">Logs</button>
            </div>
          </template>
        </el-table-column>
      </el-table>

      <div v-if="filtered.length === 0 && !loading" class="empty">
        <strong>No processes match.</strong><br />
        Define them under <code>processes:</code> in your config — changes hot-reload automatically.
      </div>
    </div>

    <!-- Process detail dialog (TUI-equivalent) -->
    <el-dialog
      v-model="detailVisible"
      :title="detailName"
      width="min(860px, 94%)"
      top="5vh"
      destroy-on-close
      @closed="detailName = ''"
    >
      <ProcessDetail
        v-if="detailName"
        :name="detailName"
        :initial-tab="detailTab"
        @action="onAction"
      />
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { storeToRefs } from 'pinia'
import { useProcessStore } from '@/stores/process'
import { ElMessage } from 'element-plus'
import ProcessDetail from '@/components/ProcessDetail.vue'
import { setProcessAutostart, type ProcessInfo } from '@/api'

const processStore = useProcessStore()
const { processes, loading } = storeToRefs(processStore)
const toggling = ref<string>('')

const filter = ref('')
const filtered = computed(() => {
  const q = filter.value.trim().toLowerCase()
  if (!q) return processes.value
  return processes.value.filter(p => p.name.toLowerCase().includes(q))
})

/* ---------- state helpers ---------- */
function badgeClass(state: string) {
  switch (state) {
    case 'RUNNING':
      return 'badge--running'
    case 'FATAL':
      return 'badge--crit'
    case 'BACKOFF':
    case 'STARTING':
    case 'STOPPING':
      return 'badge--warn'
    default:
      return 'badge--idle'
  }
}
function dotClass(state: string) {
  if (state === 'RUNNING') return 'is-run'
  if (state === 'FATAL') return 'is-crit'
  if (state === 'BACKOFF' || state === 'STARTING' || state === 'STOPPING') return 'is-warn'
  return 'is-idle'
}
function fmtUptime(p: ProcessInfo) {
  if (p.state !== 'RUNNING' || p.uptime === '0s') return '—'
  return p.uptime.replace(/\.\d+s$/, 's')
}

async function act(name: string, action: 'start' | 'stop' | 'restart') {
  await processStore[action](name)
  processStore.fetchProcesses()
}

/** Toggle boot-start; the server persists it into the process's config file. */
async function toggleAutostart(row: ProcessInfo) {
  toggling.value = row.name
  try {
    await setProcessAutostart(row.name, !row.autostart)
    await processStore.fetchProcesses()
    ElMessage.success(
      row.autostart
        ? `${row.name} will no longer start on boot`
        : `${row.name} now starts on boot`,
    )
  } catch (e: any) {
    const warning = e?.response?.data?.warning
    ElMessage.error(warning || `Failed to change autostart for ${row.name}`)
  } finally {
    toggling.value = ''
  }
}

/* ---------- detail dialog ---------- */
const detailVisible = ref(false)
const detailName = ref('')
const detailTab = ref<'details' | 'logs'>('details')

function openDetail(name: string, tab: 'details' | 'logs' = 'details') {
  if (typeof name !== 'string' || !name) return
  detailName.value = name
  detailTab.value = tab
  detailVisible.value = true
}

async function onAction(name: string, action: 'start' | 'stop' | 'restart') {
  try {
    await processStore[action](name)
    await processStore.fetchProcesses()
    ElMessage.success(`${action} — ${name}`)
  } catch (e: any) {
    ElMessage.error(e?.message ?? `${action} failed`)
  }
}

onMounted(() => processStore.fetchProcesses())
</script>

<style scoped>
.page-head {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 16px;
  margin-bottom: 20px;
  flex-wrap: wrap;
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
.page-head__actions {
  display: flex;
  gap: 8px;
  align-items: center;
}
.search {
  height: 30px;
  width: 200px;
  padding: 0 11px;
  background: var(--surface-2);
  border: 1px solid var(--line);
  border-radius: var(--r-sm);
  color: var(--ink-2);
  font-size: 13px;
  outline: none;
}
.search:focus { border-color: var(--accent); }
.search::placeholder { color: var(--ink-3); }

/* table cells */
:deep(.row-clickable) { cursor: pointer; }
.cell-name {
  display: flex;
  align-items: center;
  gap: 9px;
  font-weight: 600;
  color: var(--ink);
}
.cell-name__dot {
  width: 7px;
  height: 7px;
  border-radius: 50%;
  flex-shrink: 0;
}
.cell-name__dot.is-run { background: var(--good); box-shadow: 0 0 6px rgba(12, 163, 12, 0.6); }
.cell-name__dot.is-crit { background: var(--crit); box-shadow: 0 0 6px rgba(208, 59, 59, 0.6); }
.cell-name__dot.is-warn { background: var(--warn); }
.cell-name__dot.is-idle { background: var(--line-2); }

.cell-cmd {
  font-family: ui-monospace, Consolas, monospace;
  font-size: 11.5px;
  color: var(--ink-3);
}
.cell-actions {
  display: flex;
  gap: 6px;
}

/* autostart toggle */
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
.tnum { font-variant-numeric: tabular-nums; color: var(--ink-2); }
</style>
