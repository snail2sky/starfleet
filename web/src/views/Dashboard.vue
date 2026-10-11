<template>
  <div class="dashboard">
    <h1>Dashboard</h1>

    <!-- System Overview Cards -->
    <el-row :gutter="20" class="stat-cards">
      <el-col :span="6">
        <el-card shadow="hover">
          <template #header><span>CPU</span></template>
          <div class="stat-value">{{ overview?.cpu?.percent?.[0]?.toFixed(1) || 0 }}%</div>
          <el-progress :percentage="overview?.cpu?.percent?.[0] || 0" :stroke-width="8" />
          <div class="stat-detail">{{ overview?.cpu?.cores || 0 }} cores</div>
        </el-card>
      </el-col>
      <el-col :span="6">
        <el-card shadow="hover">
          <template #header><span>Memory</span></template>
          <div class="stat-value">{{ formatBytes(overview?.memory?.used || 0) }}</div>
          <el-progress :percentage="overview?.memory?.percent || 0" :stroke-width="8" status="warning" />
          <div class="stat-detail">/ {{ formatBytes(overview?.memory?.total || 0) }}</div>
        </el-card>
      </el-col>
      <el-col :span="6">
        <el-card shadow="hover">
          <template #header><span>Disk</span></template>
          <div class="stat-value">{{ formatBytes(overview?.disk?.used || 0) }}</div>
          <el-progress :percentage="overview?.disk?.percent || 0" :stroke-width="8" status="exception" />
          <div class="stat-detail">/ {{ formatBytes(overview?.disk?.total || 0) }}</div>
        </el-card>
      </el-col>
      <el-col :span="6">
        <el-card shadow="hover">
          <template #header><span>Load Average</span></template>
          <div class="stat-value">{{ overview?.load?.load1?.toFixed(2) || '0.00' }}</div>
          <div class="stat-detail">
            5m: {{ overview?.load?.load5?.toFixed(2) || '0.00' }} |
            15m: {{ overview?.load?.load15?.toFixed(2) || '0.00' }}
          </div>
        </el-card>
      </el-col>
    </el-row>

    <!-- Process Status Cards -->
    <h2>Processes</h2>
    <el-row :gutter="16">
      <el-col :span="6" v-for="p in processes" :key="p.name">
        <el-card shadow="hover" class="process-card">
          <div class="process-header">
            <span class="process-name">{{ p.name }}</span>
            <el-tag :type="stateType(p.state)" size="small">{{ p.state }}</el-tag>
          </div>
          <div class="process-detail">
            <div v-if="p.pid">PID: {{ p.pid }}</div>
            <div v-if="p.uptime !== '0s'">Uptime: {{ p.uptime }}</div>
          </div>
          <div class="process-actions">
            <el-button-group size="small">
              <el-button @click="processStore.start(p.name)" :disabled="p.state === 'RUNNING'">Start</el-button>
              <el-button @click="processStore.stop(p.name)" :disabled="p.state === 'STOPPED'">Stop</el-button>
              <el-button @click="processStore.restart(p.name)">Restart</el-button>
            </el-button-group>
          </div>
        </el-card>
      </el-col>
    </el-row>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { getSystemOverview, type SystemOverview } from '@/api'
import { useProcessStore } from '@/stores/process'

const processStore = useProcessStore()
const { processes } = processStore
const overview = ref<SystemOverview | null>(null)

function stateType(state: string) {
  switch (state) {
    case 'RUNNING': return 'success'
    case 'STOPPED': return 'info'
    case 'FATAL': return 'danger'
    case 'BACKOFF': return 'warning'
    default: return 'info'
  }
}

function formatBytes(bytes: number): string {
  if (bytes === 0) return '0 B'
  const k = 1024
  const sizes = ['B', 'KB', 'MB', 'GB', 'TB']
  const i = Math.floor(Math.log(bytes) / Math.log(k))
  return (bytes / Math.pow(k, i)).toFixed(1) + ' ' + sizes[i]
}

onMounted(async () => {
  processStore.fetchProcesses()
  try {
    const { data } = await getSystemOverview()
    overview.value = data.data
  } catch (e) {
    console.error('Failed to fetch overview:', e)
  }
})
</script>

<style scoped>
.dashboard h1 { color: #38bdf8; margin-bottom: 20px; }
.dashboard h2 { color: #94a3b8; margin: 20px 0 10px; }
.stat-cards { margin-bottom: 20px; }
.stat-value { font-size: 24px; font-weight: bold; color: #e2e8f0; margin: 10px 0; }
.stat-detail { color: #64748b; font-size: 12px; margin-top: 5px; }
.process-card { margin-bottom: 16px; }
.process-header { display: flex; justify-content: space-between; align-items: center; margin-bottom: 10px; }
.process-name { font-weight: bold; color: #38bdf8; }
.process-detail { color: #94a3b8; font-size: 12px; margin-bottom: 10px; }
.process-actions { text-align: right; }
</style>