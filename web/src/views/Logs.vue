<template>
  <div>
    <header class="page-head">
      <div>
        <h1 class="page-title">Logs</h1>
        <p class="page-sub">Auto-refreshes every {{ interval / 1000 }}s · buffered in memory (last 500 lines)</p>
      </div>
      <div class="page-head__actions">
        <select class="select" v-model="selected" aria-label="Filter by process">
          <option value="">All processes</option>
          <option v-for="p in processes" :key="p.name" :value="p.name">{{ p.name }}</option>
        </select>
        <select class="select" v-model="stream" aria-label="Filter by stream">
          <option value="">stdout + stderr</option>
          <option value="stdout">stdout only</option>
          <option value="stderr">stderr only</option>
        </select>
        <label class="chk">
          <input type="checkbox" v-model="autoScroll" /> follow
        </label>
        <button class="btn" @click="refreshNow">Refresh now</button>
      </div>
    </header>

    <div class="status-strip">
      <span class="dot" :class="error ? 'dot--crit' : 'dot--ok'"></span>
      <span class="status-strip__text">
        <template v-if="error">Connection error: {{ error }} — retrying…</template>
        <template v-else>
          {{ visible.length }} lines
          <template v-if="lastFetch"> · updated {{ lastFetch.toLocaleTimeString() }}</template>
          <template v-if="loading"> · fetching…</template>
        </template>
      </span>
      <span class="status-strip__right">{{ selected || 'all processes' }}</span>
    </div>

    <div class="log-box" ref="logBox">
      <div v-if="visible.length === 0" class="empty">
        <strong>No log output yet.</strong><br />
        Logs appear here as soon as managed processes write to stdout/stderr.
      </div>
      <div v-for="(e, i) in visible" :key="e.timestamp + '-' + i" class="log-line log-lines">
        <span class="log-line__ts">{{ fmtTs(e.timestamp) }}</span>
        <span class="log-line__proc">{{ e.process }}</span>
        <span
          class="log-line__src"
          :class="{ 'log-line__src--err': e.stream === 'stderr' }"
          >{{ e.stream }}</span
        >
        <span class="log-line__text">{{ e.line }}</span>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, nextTick } from 'vue'
import { storeToRefs } from 'pinia'
import { useProcessStore } from '@/stores/process'
import { useLogs } from '@/composables/useLogs'

const interval = 2000
const processStore = useProcessStore()
const { processes } = storeToRefs(processStore)

const selected = ref<string | null>(null)
const stream = ref('')
const autoScroll = ref(true)
const logBox = ref<HTMLElement | null>(null)

const { entries, loading, error, lastFetch, start, refresh } = useLogs({
  process: selected,
  interval,
  onEntry: () => {
    if (!autoScroll.value) return
    nextTick(() => {
      if (logBox.value) logBox.value.scrollTop = logBox.value.scrollHeight
    })
  },
})

const visible = computed(() =>
  stream.value ? entries.value.filter(e => e.stream === stream.value) : entries.value,
)

function refreshNow() {
  refresh()
}

function fmtTs(ms: number): string {
  const d = new Date(ms)
  return (
    d.toLocaleTimeString([], { hour12: false }) +
    '.' +
    String(d.getMilliseconds()).padStart(3, '0')
  )
}

onMounted(() => {
  processStore.fetchProcesses()
  start()
})
</script>

<style scoped>
.page-head {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 16px;
  margin-bottom: 16px;
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
  align-items: center;
  gap: 9px;
  flex-wrap: wrap;
}

.select {
  height: 30px;
  padding: 0 9px;
  background: var(--surface-2);
  border: 1px solid var(--line);
  border-radius: var(--r-sm);
  color: var(--ink-2);
  font-size: 13px;
  outline: none;
  cursor: pointer;
}
.select:focus { border-color: var(--accent); }

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

/* status strip */
.status-strip {
  display: flex;
  align-items: center;
  gap: 10px;
  background: var(--surface);
  border: 1px solid var(--ring);
  border-radius: var(--r-sm);
  padding: 8px 13px;
  margin-bottom: 12px;
  font-size: 12.5px;
  color: var(--ink-3);
}
.status-strip__text { flex: 1; }
.status-strip__right {
  color: var(--ink-3);
  font-family: ui-monospace, Consolas, monospace;
  font-size: 12px;
}
.dot {
  width: 7px;
  height: 7px;
  border-radius: 50%;
  flex-shrink: 0;
}
.dot--ok { background: var(--good); box-shadow: 0 0 6px rgba(12, 163, 12, 0.6); }
.dot--crit { background: var(--crit); box-shadow: 0 0 6px rgba(208, 59, 59, 0.6); }

/* log box */
.log-box {
  background: var(--surface);
  border: 1px solid var(--ring);
  border-radius: var(--r-md);
  height: calc(100vh - 250px);
  min-height: 300px;
  overflow-y: auto;
  padding: 12px 14px;
}
.log-line {
  display: flex;
  gap: 11px;
  padding: 1.5px 0;
  word-break: break-all;
  border-bottom: 1px solid transparent;
}
.log-line:hover { background: var(--surface-2); }
.log-line__ts {
  color: var(--line-2);
  flex-shrink: 0;
  font-variant-numeric: tabular-nums;
}
.log-line__proc {
  color: var(--accent);
  flex-shrink: 0;
  width: 110px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.log-line__src { color: var(--ink-3); flex-shrink: 0; width: 52px; }
.log-line__src--err { color: var(--serious, #ec835a); }
.log-line__text { color: var(--ink-2); }
</style>
