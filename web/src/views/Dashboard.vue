<template>
  <div>
    <header class="page-head">
      <div>
        <h1 class="page-title">Overview</h1>
        <p class="page-sub">System resources and managed processes at a glance</p>
      </div>
      <button class="btn" @click="refreshAll" :disabled="loading">
        {{ loading ? 'Refreshing…' : 'Refresh' }}
      </button>
    </header>

    <!-- KPI row: stat tiles with meters -->
    <section class="kpi">
      <div class="tile">
        <div class="tile__label">CPU</div>
        <div class="tile__value">
          {{ cpuPct.toFixed(0) }}<span class="tile__unit">%</span>
        </div>
        <div class="tile__sub">{{ overview?.cpu?.cores ?? '—' }} cores · load {{ loadText }}</div>
        <div class="meter" :class="meterClass(cpuPct)">
          <div class="meter__fill" :style="{ width: cpuPct + '%' }"></div>
        </div>
      </div>

      <div class="tile">
        <div class="tile__label">Memory</div>
        <div class="tile__value">
          {{ memUsed }}<span class="tile__unit"> used</span>
        </div>
        <div class="tile__sub">of {{ memTotal }} · {{ memPct.toFixed(1) }}%</div>
        <div class="meter" :class="meterClass(memPct)">
          <div class="meter__fill" :style="{ width: memPct + '%' }"></div>
        </div>
      </div>

      <div class="tile">
        <div class="tile__label">Disk</div>
        <div class="tile__value">
          {{ diskUsed }}<span class="tile__unit"> used</span>
        </div>
        <div class="tile__sub">of {{ diskTotal }} · {{ diskPct.toFixed(1) }}%</div>
        <div class="meter" :class="meterClass(diskPct)">
          <div class="meter__fill" :style="{ width: diskPct + '%' }"></div>
        </div>
      </div>

      <div class="tile">
        <div class="tile__label">Processes</div>
        <div class="tile__value">
          {{ runningCount }}<span class="tile__unit"> / {{ processes.length }} up</span>
        </div>
        <div class="tile__sub">managed by starfleet</div>
        <div class="proc-chips">
          <span v-if="runningCount > 0" class="badge badge--running">{{ runningCount }} running</span>
          <span v-if="stoppedCount > 0" class="badge badge--idle">{{ stoppedCount }} stopped</span>
          <span v-if="badCount > 0" class="badge badge--crit">{{ badCount }} failed</span>
        </div>
      </div>
    </section>

    <!-- Process cards -->
    <h2 class="section-title">Managed processes</h2>
    <div v-if="processes.length === 0" class="panel empty">
      <strong>No processes configured.</strong><br />
      Add entries under <code>processes:</code> in your config (see <code>conf.d/</code>) — hot-reload picks
      them up automatically.
    </div>

    <div v-else class="proc-grid">
      <article
        v-for="p in processes"
        :key="p.name"
        class="proc-card"
        role="button"
        tabindex="0"
        @click="openDetail(p.name, 'details')"
        @keydown.enter="openDetail(p.name, 'details')"
      >
        <div class="proc-card__top">
          <div class="proc-card__name" :title="p.name">{{ p.name }}</div>
          <span class="badge" :class="badgeClass(p.state)">{{ p.state }}</span>
        </div>
        <div class="proc-card__cmd" :title="p.command">{{ p.command }}</div>
        <dl class="proc-card__meta">
          <div><dt>PID</dt><dd>{{ p.pid || '—' }}</dd></div>
          <div><dt>Uptime</dt><dd>{{ fmtUptime(p) }}</dd></div>
          <div><dt>Retries</dt><dd>{{ p.retry_count }}</dd></div>
        </dl>
        <div class="proc-card__actions" @click.stop>
          <button class="btn" :disabled="p.state === 'RUNNING'" @click="act(p.name, 'start')">Start</button>
          <button class="btn" :disabled="p.state === 'STOPPED'" @click="act(p.name, 'stop')">Stop</button>
          <button class="btn" @click="act(p.name, 'restart')">Restart</button>
          <button class="btn btn--primary" @click="openDetail(p.name, 'logs')">Logs</button>
        </div>
      </article>
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
import { getSystemOverview, type SystemOverview } from '@/api'
import { ElMessage } from 'element-plus'
import ProcessDetail from '@/components/ProcessDetail.vue'

const processStore = useProcessStore()
// storeToRefs keeps reactivity — destructuring the store directly gives a
// stale snapshot and the list never updates after fetch.
const { processes, loading } = storeToRefs(processStore)

const overview = ref<SystemOverview | null>(null)

/* ---------- system metrics ---------- */
const cpuPct = computed(() => overview.value?.cpu?.percent?.[0] ?? 0)
const memPct = computed(() => overview.value?.memory?.percent ?? 0)
const diskPct = computed(() => overview.value?.disk?.percent ?? 0)
const memUsed = computed(() => fmtBytes(overview.value?.memory?.used ?? 0))
const memTotal = computed(() => fmtBytes(overview.value?.memory?.total ?? 0))
const diskUsed = computed(() => fmtBytes(overview.value?.disk?.used ?? 0))
const diskTotal = computed(() => fmtBytes(overview.value?.disk?.total ?? 0))
const loadText = computed(() => {
  const l = overview.value?.load
  return l ? `${l.load1.toFixed(2)} / ${l.load5.toFixed(2)}` : '—'
})

/* severity thresholds → meter class (accent → warning → critical) */
function meterClass(pct: number) {
  if (pct >= 85) return 'meter meter--crit'
  if (pct >= 70) return 'meter meter--warn'
  return 'meter meter--ok'
}

/* ---------- process counts ---------- */
const runningCount = computed(() => processes.value.filter(p => p.state === 'RUNNING').length)
const stoppedCount = computed(() =>
  processes.value.filter(p => p.state === 'STOPPED' || p.state === 'EXITED').length,
)
const badCount = computed(() =>
  processes.value.filter(p => p.state === 'FATAL' || p.state === 'BACKOFF').length,
)

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

function fmtUptime(p: { state: string; uptime: string }) {
  if (p.state !== 'RUNNING' || p.uptime === '0s') return '—'
  return p.uptime.replace(/\.\d+s$/, 's')
}

function fmtBytes(n: number): string {
  if (!n) return '0'
  const units = ['B', 'KB', 'MB', 'GB', 'TB']
  const i = Math.min(Math.floor(Math.log(n) / Math.log(1024)), units.length - 1)
  return `${(n / Math.pow(1024, i)).toFixed(i > 1 ? 1 : 0)} ${units[i]}`
}

/* ---------- actions ---------- */
async function act(name: string, action: 'start' | 'stop' | 'restart') {
  await processStore[action](name)
  refreshAll()
}

async function onAction(name: string, action: 'start' | 'stop' | 'restart') {
  try {
    await processStore[action](name)
    await refreshAll()
    ElMessage.success(`${action} — ${name}`)
  } catch (e: any) {
    ElMessage.error(e?.message ?? `${action} failed`)
  }
}

async function refreshAll() {
  await Promise.all([processStore.fetchProcesses(), fetchOverview()])
}

async function fetchOverview() {
  try {
    const { data } = await getSystemOverview()
    overview.value = data.data
  } catch {
    /* server unreachable — tiles show fallback */
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

onMounted(refreshAll)
</script>

<style scoped>
.page-head {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 16px;
  margin-bottom: 22px;
}
.page-title {
  margin: 0;
  font-size: 22px;
  font-weight: 650;
  color: var(--ink);
  letter-spacing: -0.01em;
}
.page-sub {
  margin: 5px 0 0;
  font-size: 13px;
  color: var(--ink-3);
}

/* KPI row */
.kpi {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(230px, 1fr));
  gap: 14px;
}
.tile__value {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.tile__unit {
  font-size: 14px;
  font-weight: 500;
  color: var(--ink-3);
  margin-left: 2px;
}
.proc-chips {
  display: flex;
  gap: 6px;
  flex-wrap: wrap;
  margin-top: 12px;
}

/* process cards */
.proc-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(330px, 1fr));
  gap: 14px;
}
.proc-card {
  background: var(--surface);
  border: 1px solid var(--ring);
  border-radius: var(--r-md);
  padding: 16px 17px 14px;
  display: flex;
  flex-direction: column;
  gap: 10px;
  min-width: 0;
  cursor: pointer;
  transition: border-color 0.15s, transform 0.15s;
}
.proc-card:hover {
  border-color: var(--line-2);
  transform: translateY(-1px);
}
.proc-card:focus-visible {
  outline: 2px solid var(--accent);
  outline-offset: 2px;
}
.proc-card__top {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 10px;
}
.proc-card__name {
  font-size: 15px;
  font-weight: 650;
  color: var(--ink);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  min-width: 0;
}
.proc-card__cmd {
  font-family: ui-monospace, Consolas, monospace;
  font-size: 11.5px;
  color: var(--ink-3);
  background: var(--plane);
  border: 1px solid var(--line);
  border-radius: var(--r-sm);
  padding: 7px 9px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.proc-card__meta {
  display: flex;
  gap: 22px;
  margin: 0;
}
.proc-card__meta div {
  display: flex;
  flex-direction: column;
  gap: 2px;
}
.proc-card__meta dt {
  font-size: 10.5px;
  text-transform: uppercase;
  letter-spacing: 0.06em;
  color: var(--ink-3);
}
.proc-card__meta dd {
  margin: 0;
  font-size: 13px;
  color: var(--ink-2);
  font-variant-numeric: tabular-nums;
}
.proc-card__actions {
  display: flex;
  gap: 7px;
  margin-top: auto;
  padding-top: 4px;
}
.proc-card__actions .btn {
  flex: 1;
  min-width: 0;
  padding: 0 6px;
}
</style>
