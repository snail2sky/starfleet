<template>
  <div class="schedules">
    <h1>Schedules</h1>
    <el-table :data="schedules" stripe style="width: 100%">
      <el-table-column prop="name" label="Name" width="180" />
      <el-table-column prop="cron" label="Cron" width="150" />
      <el-table-column prop="type" label="Type" width="80" />
      <el-table-column prop="command" label="Command" show-overflow-tooltip />
      <el-table-column label="Enabled" width="100">
        <template #default="{ row }">
          <el-tag :type="row.enabled ? 'success' : 'info'" size="small">
            {{ row.enabled ? 'Yes' : 'No' }}
          </el-tag>
        </template>
      </el-table-column>
      <el-table-column prop="timeout" label="Timeout" width="100" />
      <el-table-column label="Actions" width="200">
        <template #default="{ row }">
          <el-button size="small" @click="runJob(row.name)">Run Now</el-button>
          <el-button size="small" @click="viewHistory(row.name)">History</el-button>
        </template>
      </el-table-column>
    </el-table>

    <el-dialog v-model="historyVisible" title="Execution History" width="80%">
      <el-table :data="history" stripe>
        <el-table-column prop="start_time" label="Time" width="180">
          <template #default="{ row }">{{ new Date(row.start_time).toLocaleString() }}</template>
        </el-table-column>
        <el-table-column prop="exit_code" label="Exit Code" width="100" />
        <el-table-column label="Duration" width="120">
          <template #default="{ row }">{{ (row.duration / 1000000).toFixed(0) }}ms</template>
        </el-table-column>
        <el-table-column prop="error" label="Error" show-overflow-tooltip />
        <el-table-column prop="output" label="Output" show-overflow-tooltip />
      </el-table>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { ElMessage } from 'element-plus'
import { getSchedules, runSchedule, getScheduleHistory, type ScheduleInfo, type HistoryEntry } from '@/api'

const schedules = ref<ScheduleInfo[]>([])
const history = ref<HistoryEntry[]>([])
const historyVisible = ref(false)

async function fetchSchedules() {
  try {
    const { data } = await getSchedules()
    schedules.value = data.data
  } catch (e) {
    console.error('Failed to fetch schedules:', e)
  }
}

async function runJob(name: string) {
  try {
    await runSchedule(name)
    ElMessage.success(`Job "${name}" triggered`)
  } catch (e) {
    ElMessage.error(`Failed to trigger job "${name}"`)
  }
}

async function viewHistory(name: string) {
  try {
    const { data } = await getScheduleHistory(name)
    history.value = data.data
    historyVisible.value = true
  } catch (e) {
    ElMessage.error(`Failed to fetch history for "${name}"`)
  }
}

onMounted(fetchSchedules)
</script>

<style scoped>
.schedules h1 { color: #38bdf8; margin-bottom: 20px; }
</style>