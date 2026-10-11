<template>
  <div class="processes">
    <h1>Processes</h1>
    <el-table :data="processes" stripe style="width: 100%">
      <el-table-column prop="name" label="Name" width="180" />
      <el-table-column label="State" width="120">
        <template #default="{ row }">
          <el-tag :type="stateType(row.state)" size="small">{{ row.state }}</el-tag>
        </template>
      </el-table-column>
      <el-table-column prop="pid" label="PID" width="80" />
      <el-table-column prop="uptime" label="Uptime" width="120" />
      <el-table-column prop="command" label="Command" show-overflow-tooltip />
      <el-table-column prop="autorestart" label="Autorestart" width="120" />
      <el-table-column label="Actions" width="240">
        <template #default="{ row }">
          <el-button-group size="small">
            <el-button @click="processStore.start(row.name)" :disabled="row.state === 'RUNNING'">Start</el-button>
            <el-button @click="processStore.stop(row.name)" :disabled="row.state === 'STOPPED'">Stop</el-button>
            <el-button @click="processStore.restart(row.name)">Restart</el-button>
          </el-button-group>
        </template>
      </el-table-column>
    </el-table>
  </div>
</template>

<script setup lang="ts">
import { onMounted } from 'vue'
import { useProcessStore } from '@/stores/process'

const processStore = useProcessStore()
const { processes } = processStore

function stateType(state: string) {
  switch (state) {
    case 'RUNNING': return 'success'
    case 'STOPPED': return 'info'
    case 'FATAL': return 'danger'
    case 'BACKOFF': return 'warning'
    default: return 'info'
  }
}

onMounted(() => processStore.fetchProcesses())
</script>

<style scoped>
.processes h1 { color: #38bdf8; margin-bottom: 20px; }
</style>