<template>
  <div class="pd">
    <!-- header: identity + actions -->
    <div class="pd__head">
      <div class="pd__id">
        <div class="pd__name">{{ info?.name || name }}</div>
        <div class="pd__meta">
          <span class="badge" :class="badgeClass">{{ info?.state || '…' }}</span>
          <span v-if="info?.pid" class="pd__pid">PID {{ info.pid }}</span>
          <span v-if="info?.state === 'RUNNING'" class="pd__pid">up {{ fmtUptime(info.uptime) }}</span>
        </div>
      </div>
      <div class="pd__actions">
        <button class="btn" :disabled="!info || info.state === 'RUNNING'" @click="emit('action', name, 'start')">
          Start
        </button>
        <button class="btn" :disabled="!info || info.state === 'STOPPED'" @click="emit('action', name, 'stop')">
          Stop
        </button>
        <button class="btn btn--danger" @click="emit('action', name, 'restart')">Restart</button>
      </div>
    </div>

    <!-- tabs -->
    <div class="pd__tabs" role="tablist">
      <button
        class="pd__tab"
        :class="{ 'pd__tab--active': tab === 'details' }"
        @click="tab = 'details'"
      >
        Details
      </button>
      <button
        class="pd__tab"
        :class="{ 'pd__tab--active': tab === 'logs' }"
        @click="tab = 'logs'"
      >
        Logs
        <span v-if="tab === 'logs' && logEntries.length" class="pd__tab-count">{{ logEntries.length }}</span>
      </button>
      <span class="pd__tabs-right">
        <template v-if="tab === 'details' && loadError">{{ loadError }}</template>
        <template v-else-if="tab === 'details' && loading">loading…</template>
        <template v-else-if="tab === 'logs'">
          {{ lastFetch ? 'updated ' + lastFetch.toLocaleTimeString() : '' }}
        </template>
      </span>
    </div>

    <!-- details -->
    <div v-if="tab === 'details'" class="pd__body">
      <div v-if="loading && !info" class="empty">Loading…</div>
      <template v-else-if="info">
        <!-- Basic info -->
        <section class="pd__section">
          <h3 class="pd__h">Basic info</h3>
          <dl class="pd__kv">
            <div><dt>State</dt><dd><span class="badge" :class="badgeClass">{{ info.state }}</span></dd></div>
            <div><dt>PID</dt><dd class="tnum">{{ info.pid || '—' }}</dd></div>
            <div><dt>Uptime</dt><dd class="tnum">{{ info.state === 'RUNNING' ? fmtUptime(info.uptime) : '—' }}</dd></div>
            <div><dt>Exit code</dt><dd class="tnum">{{ info.exit_code }}</dd></div>
            <div><dt>Retries</dt><dd class="tnum">{{ info.retry_count }}</dd></div>
            <div><dt>Autostart</dt><dd>{{ info.autostart ? 'yes' : 'no' }}</dd></div>
            <div><dt>Autorestart</dt><dd>{{ info.autorestart }}</dd></div>
          </dl>
        </section>

        <!-- Command -->
        <section class="pd__section">
          <h3 class="pd__h">Command</h3>
          <pre class="pd__cmd" :title="info.command">{{ info.command }}</pre>
          <dl class="pd__kv">
            <div><dt>Directory</dt><dd>{{ cfg.directory || '—' }}</dd></div>
            <div><dt>User</dt><dd>{{ cfg.user || '(current user)' }}</dd></div>
            <div><dt>Start secs</dt><dd>{{ cfg.startsecs ?? '—' }}</dd></div>
            <div><dt>Start retries</dt><dd>{{ cfg.startretries ?? '—' }}</dd></div>
            <div><dt>Stop signal</dt><dd>{{ cfg.stopsignal || '—' }}</dd></div>
            <div><dt>Stop wait</dt><dd>{{ fmtDur((cfg.stopwaitsecs ?? 0) * 1e9) }}</dd></div>
            <div><dt>Restart delay</dt><dd>{{ fmtDur(cfg.restart_delay) }}</dd></div>
            <div><dt>Backoff limit</dt><dd>{{ fmtDur(cfg.backoff_limit) }}</dd></div>
            <div v-if="cfg.depends_on?.length"><dt>Depends on</dt><dd>{{ cfg.depends_on.join(', ') }}</dd></div>
          </dl>
        </section>

        <!-- Environment -->
        <section class="pd__section" v-if="envEntries.length || cfg.envfile">
          <h3 class="pd__h">Environment</h3>
          <dl class="pd__kv" v-if="envEntries.length">
            <div v-for="[k, v] in envEntries" :key="k">
              <dt class="pd__env-k">{{ k }}</dt>
              <dd><code class="pd__val">{{ v }}</code></dd>
            </div>
          </dl>
          <dl class="pd__kv" v-if="cfg.envfile">
            <div><dt>Env file</dt><dd>{{ cfg.envfile }}</dd></div>
          </dl>
        </section>

        <!-- Logging -->
        <section class="pd__section" v-if="cfg.stdout?.path || cfg.stderr?.path">
          <h3 class="pd__h">Logging</h3>
          <dl class="pd__kv">
            <template v-if="cfg.stdout?.path">
              <div><dt>Stdout</dt><dd class="pd__path">{{ cfg.stdout.path }}</dd></div>
              <div><dt>Stdout max</dt><dd>{{ cfg.stdout.maxbytes }} × {{ cfg.stdout.backups }}</dd></div>
            </template>
            <template v-if="cfg.stderr?.path">
              <div><dt>Stderr</dt><dd class="pd__path">{{ cfg.stderr.path }}</dd></div>
              <div><dt>Stderr max</dt><dd>{{ cfg.stderr.maxbytes }} × {{ cfg.stderr.backups }}</dd></div>
            </template>
          </dl>
        </section>

        <!-- Health check -->
        <section class="pd__section" v-if="cfg.healthcheck">
          <h3 class="pd__h">Health check</h3>
          <dl class="pd__kv">
            <div><dt>Type</dt><dd>{{ cfg.healthcheck.type }}</dd></div>
            <div v-if="cfg.healthcheck.url"><dt>URL</dt><dd class="pd__path">{{ cfg.healthcheck.url }}</dd></div>
            <div v-if="cfg.healthcheck.address"><dt>Address</dt><dd>{{ cfg.healthcheck.address }}</dd></div>
            <div><dt>Interval</dt><dd>{{ fmtDur(cfg.healthcheck.interval) }}</dd></div>
            <div><dt>Timeout</dt><dd>{{ fmtDur(cfg.healthcheck.timeout) }}</dd></div>
            <div><dt>Retries</dt><dd>{{ cfg.healthcheck.retries }}</dd></div>
          </dl>
        </section>

        <!-- Labels -->
        <section class="pd__section" v-if="labelEntries.length">
          <h3 class="pd__h">Labels</h3>
          <div class="pd__labels">
            <span v-for="[k, v] in labelEntries" :key="k" class="pd__label">
              {{ k }}<em>{{ v }}</em>
            </span>
          </div>
        </section>
      </template>
      <div v-else class="empty">{{ loadError || 'Process not found.' }}</div>
    </div>

    <!-- logs -->
    <div v-else class="pd__body pd__body--logs">
      <div class="pd__logbar">
        <label class="chk">
          <input type="checkbox" v-model="autoScroll" /> follow
        </label>
        <button class="btn" @click="logStore.refresh()">Refresh</button>
      </div>
      <div class="pd__logbox" ref="logBox">
        <div v-if="logEntries.length === 0" class="empty">
          No output captured yet.<br />
          <span style="font-size: 12px">Lines appear as soon as the process writes to stdout/stderr.</span>
        </div>
        <div v-for="(e, i) in logEntries" :key="e.timestamp + '-' + i" class="log-line log-lines">
          <span class="log-line__ts">{{ fmtTs(e.timestamp) }}</span>
          <span class="log-line__src" :class="{ 'log-line__src--err': e.stream === 'stderr' }">{{ e.stream }}</span>
          <span class="log-line__text">{{ e.line }}</span>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, watch, nextTick } from 'vue'
import { getProcess, type ProcessInfo, type ProcessConfig } from '@/api'
import { useLogs } from '@/composables/useLogs'

const props = defineProps<{
  name: string
  initialTab?: 'details' | 'logs'
}>()

const emit = defineEmits<{
  (e: 'action', name: string, action: 'start' | 'stop' | 'restart'): void
}>()

const tab = ref<'details' | 'logs'>(props.initialTab ?? 'details')
const info = ref<ProcessInfo | null>(null)
const cfg = ref<ProcessConfig>({} as ProcessConfig)
const loading = ref(false)
const loadError = ref('')

const cfgRef = ref<string | null>(props.name)

async function fetchDetail() {
  loading.value = true
  loadError.value = ''
  try {
    const { data } = await getProcess(props.name)
    info.value = data.data
    cfg.value = (data.config ?? {}) as ProcessConfig
  } catch (e: any) {
    loadError.value = e?.message ?? 'Failed to load process'
    info.value = null
  } finally {
    loading.value = false
  }
}

const envEntries = computed(() => Object.entries(cfg.value.environment ?? {}))
const labelEntries = computed(() => Object.entries(info.value?.labels ?? cfg.value.labels ?? {}))

function badgeClass(state?: string) {
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

function fmtUptime(u?: string) {
  return (u ?? '').replace(/\.\d+s$/, 's')
}

/** Format a nanosecond duration (Go json marshals time.Duration as ns). 0/absent → —. */
function fmtDur(ns?: number) {
  if (ns == null || isNaN(ns) || ns === 0) return '—'
  const s = ns / 1e9
  if (s < 1) return `${Math.round(ns / 1e6)}ms`
  if (s < 60) return `${s % 1 === 0 ? s : s.toFixed(1)}s`
  if (s < 3600) return `${(s / 60).toFixed(s / 60 < 10 ? 1 : 0)}m`
  return `${(s / 3600).toFixed(1)}h`
}

function fmtTs(ms: number) {
  const d = new Date(ms)
  return d.toLocaleTimeString([], { hour12: false }) + '.' + String(d.getMilliseconds()).padStart(3, '0')
}

/* logs */
const autoScroll = ref(true)
const logBox = ref<HTMLElement | null>(null)
const logStore = useLogs({
  process: cfgRef,
  interval: 2500,
  maxLines: 500,
  onEntry: () => {
    if (!autoScroll.value) return
    nextTick(() => {
      if (logBox.value) logBox.value.scrollTop = logBox.value.scrollHeight
    })
  },
})
const { entries: logEntries, lastFetch, start: startLogs, stop: stopLogs } = logStore

watch(tab, v => {
  if (v === 'logs') startLogs()
  else stopLogs()
})

watch(() => props.name, () => {
  cfgRef.value = props.name
  info.value = null
  fetchDetail()
})

onMounted(() => {
  fetchDetail()
  if (tab.value === 'logs') startLogs()
})
</script>

<style scoped>
.pd {
  display: flex;
  flex-direction: column;
  min-height: 0;
}

/* header */
.pd__head {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 14px;
  flex-wrap: wrap;
  margin-bottom: 14px;
}
.pd__name {
  font-size: 17px;
  font-weight: 650;
  color: var(--ink);
  word-break: break-all;
}
.pd__meta {
  display: flex;
  align-items: center;
  gap: 10px;
  margin-top: 7px;
  flex-wrap: wrap;
}
.pd__pid {
  font-size: 12px;
  color: var(--ink-3);
  font-variant-numeric: tabular-nums;
}
.pd__actions {
  display: flex;
  gap: 7px;
  flex-shrink: 0;
}

/* tabs */
.pd__tabs {
  display: flex;
  align-items: center;
  gap: 4px;
  border-bottom: 1px solid var(--line);
  margin-bottom: 16px;
}
.pd__tab {
  appearance: none;
  background: none;
  border: none;
  border-bottom: 2px solid transparent;
  margin-bottom: -1px;
  padding: 8px 14px;
  font-size: 13px;
  font-weight: 550;
  color: var(--ink-3);
  cursor: pointer;
  display: flex;
  align-items: center;
  gap: 7px;
  transition: color 0.15s, border-color 0.15s;
}
.pd__tab:hover { color: var(--ink-2); }
.pd__tab--active {
  color: var(--accent);
  border-bottom-color: var(--accent);
}
.pd__tab-count {
  font-size: 10.5px;
  background: var(--surface-3);
  color: var(--ink-3);
  border-radius: 999px;
  padding: 1px 7px;
  font-variant-numeric: tabular-nums;
}
.pd__tabs-right {
  margin-left: auto;
  font-size: 11.5px;
  color: var(--ink-3);
}

/* body */
.pd__body {
  min-height: 0;
}
.pd__body--logs {
  display: flex;
  flex-direction: column;
}

/* sections — same grouping as the TUI detail view */
.pd__section {
  margin-bottom: 22px;
}
.pd__section:last-child { margin-bottom: 4px; }
.pd__h {
  margin: 0 0 10px;
  font-size: 11.5px;
  font-weight: 650;
  letter-spacing: 0.07em;
  text-transform: uppercase;
  color: var(--ink-3);
}
.pd__kv {
  margin: 0;
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(240px, 1fr));
  gap: 9px 26px;
}
.pd__kv > div {
  display: grid;
  grid-template-columns: 118px 1fr;
  gap: 10px;
  align-items: baseline;
  min-width: 0;
}
.pd__kv dt {
  font-size: 12.5px;
  color: var(--ink-3);
}
.pd__kv dd {
  margin: 0;
  font-size: 13px;
  color: var(--ink);
  word-break: break-all;
  min-width: 0;
}
.tnum { font-variant-numeric: tabular-nums; }
.pd__env-k { font-family: ui-monospace, Consolas, monospace; font-size: 12px !important; }
.pd__val {
  font-family: ui-monospace, Consolas, monospace;
  font-size: 12px;
  color: var(--ink-2);
  background: var(--plane);
  border: 1px solid var(--line);
  border-radius: 4px;
  padding: 1px 7px;
  word-break: break-all;
}
.pd__path {
  font-family: ui-monospace, Consolas, monospace;
  font-size: 12px;
  color: var(--ink-2);
}

.pd__cmd {
  margin: 0 0 12px;
  padding: 10px 12px;
  background: var(--plane);
  border: 1px solid var(--line);
  border-radius: var(--r-sm);
  font-family: ui-monospace, Consolas, monospace;
  font-size: 12px;
  line-height: 1.6;
  color: var(--accent);
  white-space: pre-wrap;
  word-break: break-all;
  max-height: 96px;
  overflow-y: auto;
}

.pd__labels {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
}
.pd__label {
  display: inline-flex;
  align-items: center;
  gap: 7px;
  font-size: 12px;
  color: var(--ink-3);
  background: var(--surface-2);
  border: 1px solid var(--line);
  border-radius: var(--r-sm);
  padding: 4px 10px;
}
.pd__label em {
  font-style: normal;
  font-weight: 600;
  color: var(--ink);
}

/* logs tab */
.pd__logbar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 9px;
}
.chk {
  display: flex;
  align-items: center;
  gap: 6px;
  font-size: 12.5px;
  color: var(--ink-3);
  cursor: pointer;
  user-select: none;
}
.chk input { accent-color: var(--accent); }

.pd__logbox {
  background: var(--plane);
  border: 1px solid var(--line);
  border-radius: var(--r-sm);
  height: min(52vh, 480px);
  overflow-y: auto;
  padding: 10px 12px;
}
.log-line {
  display: flex;
  gap: 10px;
  padding: 1.5px 0;
  word-break: break-all;
}
.log-line:hover { background: var(--surface-2); }
.log-line__ts {
  color: var(--line-2);
  flex-shrink: 0;
  font-variant-numeric: tabular-nums;
}
.log-line__src { color: var(--ink-3); flex-shrink: 0; width: 52px; }
.log-line__src--err { color: var(--serious, #ec835a); }
.log-line__text { color: var(--ink-2); min-width: 0; }
</style>
