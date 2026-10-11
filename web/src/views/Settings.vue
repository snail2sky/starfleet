<template>
  <div>
    <header class="page-head">
      <div>
        <h1 class="page-title">Settings</h1>
        <p class="page-sub">Effective configuration (secrets masked) and runtime system info</p>
      </div>
      <div class="page-head__actions">
        <button class="btn" @click="fetchConfig">Reload view</button>
        <button class="btn btn--primary" @click="handleReload">Hot-reload config</button>
      </div>
    </header>

    <div class="grid-2">
      <!-- System info -->
      <section class="panel">
        <div class="panel__head">System</div>
        <div class="panel__body">
          <dl class="kv">
            <div><dt>Hostname</dt><dd>{{ overview?.host?.hostname ?? '—' }}</dd></div>
            <div><dt>Platform</dt><dd>{{ platformText }}</dd></div>
            <div><dt>Kernel</dt><dd>{{ overview?.host?.kernel ?? '—' }}</dd></div>
            <div><dt>CPU cores</dt><dd class="tnum">{{ overview?.cpu?.cores ?? '—' }}</dd></div>
            <div><dt>Memory</dt><dd class="tnum">{{ memText }}</dd></div>
            <div><dt>Go runtime</dt><dd>{{ overview?.host?.go_version ?? '—' }}</dd></div>
          </dl>
        </div>
      </section>

      <!-- Config file info -->
      <section class="panel">
        <div class="panel__head">Configuration</div>
        <div class="panel__body">
          <dl class="kv">
            <div><dt>Listen</dt><dd>{{ cfgText('server.host', '—') }}:{{ cfgText('server.port', '—') }}</dd></div>
            <div><dt>Auth</dt><dd>{{ cfgText('server.auth.enabled', false) ? 'enabled' : 'disabled' }}</dd></div>
            <div><dt>Processes</dt><dd class="tnum">{{ procCount }}</dd></div>
            <div><dt>Schedules</dt><dd class="tnum">{{ schedCount }}</dd></div>
            <div><dt>Includes</dt><dd>{{ includeText }}</dd></div>
            <div><dt>Reload</dt><dd>hot-reload watches the config file every 5s</dd></div>
          </dl>
          <div class="hint">
            SIGHUP also reloads: <code>kill -HUP $(pgrep -x starfleet)</code> ·
            API: <code>POST /api/v1/config/reload</code>
          </div>
        </div>
      </section>
    </div>

    <h2 class="section-title">Raw configuration</h2>
    <div class="panel">
      <pre class="cfg-pre">{{ configText }}</pre>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { ElMessage } from 'element-plus'
import { getConfig, reloadConfig, getSystemOverview, type SystemOverview } from '@/api'

const configText = ref('Loading…')
const rawConfig = ref<any>(null)
const overview = ref<SystemOverview | null>(null)

const platformText = computed(() => {
  const h = overview.value?.host
  return h ? `${h.platform} ${h.os}` : '—'
})
const memText = computed(() => {
  const m = overview.value?.memory
  if (!m) return '—'
  const fmt = (n: number) => (n / 1024 / 1024).toFixed(0) + ' MB'
  return `${fmt(m.used)} / ${fmt(m.total)} (${m.percent.toFixed(1)}%)`
})
const procCount = computed(() => Object.keys(rawConfig.value?.processes ?? {}).length)
const schedCount = computed(() => Object.keys(rawConfig.value?.schedules ?? {}).length)
const includeText = computed(() => {
  const inc = rawConfig.value?.includes
  if (!inc || inc.length === 0) return 'conf.d/ (auto-included)'
  return inc.join(', ')
})

function cfgText(path: string, fallback: any) {
  return path.split('.').reduce((o, k) => (o == null ? o : o[k]), rawConfig.value) ?? fallback
}

async function fetchConfig() {
  try {
    const { data } = await getConfig()
    rawConfig.value = data.data
    configText.value = JSON.stringify(data.data, null, 2)
  } catch {
    configText.value = 'Failed to load configuration'
  }
}

async function handleReload() {
  try {
    await reloadConfig()
    ElMessage.success('Configuration reloaded')
    await fetchConfig()
  } catch {
    ElMessage.error('Reload failed — check the server log')
  }
}

onMounted(async () => {
  fetchConfig()
  try {
    const { data } = await getSystemOverview()
    overview.value = data.data
  } catch {
    /* offline — fields show fallback */
  }
})
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
.page-head__actions { display: flex; gap: 8px; }

.grid-2 {
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
  grid-template-columns: 130px 1fr;
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
.tnum { font-variant-numeric: tabular-nums; }

.hint {
  margin-top: 16px;
  padding: 10px 12px;
  background: var(--plane);
  border: 1px solid var(--line);
  border-radius: var(--r-sm);
  font-size: 12px;
  color: var(--ink-3);
  line-height: 1.8;
}
.hint code {
  color: var(--accent);
  font-family: ui-monospace, Consolas, monospace;
}

.cfg-pre {
  margin: 0;
  padding: 16px 18px;
  max-height: 46vh;
  overflow: auto;
  font-family: ui-monospace, Consolas, monospace;
  font-size: 12.5px;
  line-height: 1.6;
  color: var(--ink-2);
  background: var(--surface);
}
</style>
