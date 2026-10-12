<template>
  <div>
    <header class="page-head">
      <div>
        <h1 class="page-title">Overview</h1>
        <p class="page-sub">System health at a glance — processes live on their own pages</p>
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
        <div class="tile__label">Host uptime</div>
        <div class="tile__value">{{ hostUptime }}</div>
        <div class="tile__sub">{{ overview?.host?.hostname ?? '—' }}</div>
      </div>
    </section>

    <!-- System details -->
    <section class="sys-grid">
      <div class="panel">
        <div class="panel__head">System</div>
        <div class="panel__body">
          <dl class="kv">
            <div><dt>Hostname</dt><dd>{{ overview?.host?.hostname ?? '—' }}</dd></div>
            <div><dt>Platform</dt><dd>{{ overview?.host?.platform ?? '—' }} {{ overview?.host?.os ?? '' }}</dd></div>
            <div><dt>Kernel</dt><dd class="mono">{{ overview?.host?.kernel ?? '—' }}</dd></div>
            <div><dt>Go runtime</dt><dd class="mono">{{ overview?.host?.go_version ?? '—' }}</dd></div>
          </dl>
        </div>
      </div>

      <div class="panel">
        <div class="panel__head">Load</div>
        <div class="panel__body">
          <dl class="kv">
            <div><dt>1 minute</dt><dd class="mono">{{ overview?.load?.load1?.toFixed(2) ?? '—' }}</dd></div>
            <div><dt>5 minutes</dt><dd class="mono">{{ overview?.load?.load5?.toFixed(2) ?? '—' }}</dd></div>
            <div><dt>15 minutes</dt><dd class="mono">{{ overview?.load?.load15?.toFixed(2) ?? '—' }}</dd></div>
            <div><dt>CPU cores</dt><dd class="mono">{{ overview?.cpu?.cores ?? '—' }}</dd></div>
          </dl>
        </div>
      </div>
    </section>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { getSystemOverview, type SystemOverview } from '@/api'

const overview = ref<SystemOverview | null>(null)
const loading = ref(false)

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

const hostUptime = computed(() => {
  const secs = overview.value?.host?.uptime ?? 0
  if (!secs) return '—'
  const d = Math.floor(secs / 86400)
  const h = Math.floor((secs % 86400) / 3600)
  const m = Math.floor((secs % 3600) / 60)
  if (d > 0) return `${d}d ${h}h`
  if (h > 0) return `${h}h ${m}m`
  return `${m}m`
})

/* severity thresholds → meter class (accent → warning → critical) */
function meterClass(pct: number) {
  if (pct >= 85) return 'meter meter--crit'
  if (pct >= 70) return 'meter meter--warn'
  return 'meter meter--ok'
}

function fmtBytes(n: number): string {
  if (!n) return '0'
  const units = ['B', 'KB', 'MB', 'GB', 'TB']
  const i = Math.min(Math.floor(Math.log(n) / Math.log(1024)), units.length - 1)
  return `${(n / Math.pow(1024, i)).toFixed(i > 1 ? 1 : 0)} ${units[i]}`
}

async function refreshAll() {
  loading.value = true
  try {
    const { data } = await getSystemOverview()
    overview.value = data.data
  } catch {
    /* server unreachable — tiles show fallback */
  } finally {
    loading.value = false
  }
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
  margin-bottom: 16px;
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

/* system panels */
.sys-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(340px, 1fr));
  gap: 14px;
}
.panel__head {
  padding: 13px 17px;
  border-bottom: 1px solid var(--line);
  font-size: 11.5px;
  font-weight: 600;
  letter-spacing: 0.07em;
  text-transform: uppercase;
  color: var(--ink-3);
}
.panel__body { padding: 15px 17px 17px; }
.kv {
  margin: 0;
  display: flex;
  flex-direction: column;
  gap: 11px;
}
.kv div {
  display: grid;
  grid-template-columns: 120px 1fr;
  gap: 12px;
  align-items: baseline;
}
.kv dt {
  font-size: 12.5px;
  color: var(--ink-3);
}
.kv dd {
  margin: 0;
  font-size: 13px;
  color: var(--ink);
  word-break: break-all;
}
.mono {
  font-family: ui-monospace, Consolas, monospace;
  font-size: 12px;
}
</style>