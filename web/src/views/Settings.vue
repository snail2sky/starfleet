<template>
  <div class="settings">
    <h1>Settings</h1>

    <el-card class="config-card">
      <template #header>
        <div class="card-header">
          <span>Configuration</span>
          <el-button type="primary" @click="handleReload">Reload Config</el-button>
        </div>
      </template>
      <el-input
        v-model="configText"
        type="textarea"
        :rows="20"
        readonly
        class="config-editor"
      />
    </el-card>

    <el-card class="info-card">
      <template #header><span>System Info</span></template>
      <el-descriptions :column="2" border>
        <el-descriptions-item label="Hostname">{{ overview?.host?.hostname }}</el-descriptions-item>
        <el-descriptions-item label="OS">{{ overview?.host?.platform }} {{ overview?.host?.os }}</el-descriptions-item>
        <el-descriptions-item label="Kernel">{{ overview?.host?.kernel }}</el-descriptions-item>
        <el-descriptions-item label="Go Version">{{ overview?.host?.go_version }}</el-descriptions-item>
        <el-descriptions-item label="CPU Cores">{{ overview?.cpu?.cores }}</el-descriptions-item>
        <el-descriptions-item label="Memory">{{ formatBytes(overview?.memory?.total || 0) }}</el-descriptions-item>
      </el-descriptions>
    </el-card>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { ElMessage } from 'element-plus'
import { getConfig, reloadConfig, getSystemOverview, type SystemOverview } from '@/api'

const configText = ref('')
const overview = ref<SystemOverview | null>(null)

function formatBytes(bytes: number): string {
  if (bytes === 0) return '0 B'
  const k = 1024
  const sizes = ['B', 'KB', 'MB', 'GB', 'TB']
  const i = Math.floor(Math.log(bytes) / Math.log(k))
  return (bytes / Math.pow(k, i)).toFixed(1) + ' ' + sizes[i]
}

async function fetchConfig() {
  try {
    const { data } = await getConfig()
    configText.value = JSON.stringify(data.data, null, 2)
  } catch (e) {
    ElMessage.error('Failed to fetch config')
  }
}

async function handleReload() {
  try {
    await reloadConfig()
    ElMessage.success('Configuration reloaded')
    await fetchConfig()
  } catch (e) {
    ElMessage.error('Failed to reload config')
  }
}

onMounted(async () => {
  fetchConfig()
  try {
    const { data } = await getSystemOverview()
    overview.value = data.data
  } catch (e) {
    console.error('Failed to fetch overview:', e)
  }
})
</script>

<style scoped>
.settings h1 { color: #38bdf8; margin-bottom: 20px; }
.config-card, .info-card { margin-bottom: 20px; }
.card-header { display: flex; justify-content: space-between; align-items: center; }
.config-editor { font-family: 'Courier New', monospace; }
</style>