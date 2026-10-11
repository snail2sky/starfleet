import { defineStore } from 'pinia'
import { ref } from 'vue'
import { getProcesses, startProcess, stopProcess, restartProcess, type ProcessInfo } from '@/api'

export const useProcessStore = defineStore('process', () => {
  const processes = ref<ProcessInfo[]>([])
  const loading = ref(false)

  async function fetchProcesses() {
    loading.value = true
    try {
      const { data } = await getProcesses()
      processes.value = data.data
    } catch (e) {
      console.error('Failed to fetch processes:', e)
    } finally {
      loading.value = false
    }
  }

  async function start(name: string) {
    await startProcess(name)
    await fetchProcesses()
  }

  async function stop(name: string) {
    await stopProcess(name)
    await fetchProcesses()
  }

  async function restart(name: string) {
    await restartProcess(name)
    await fetchProcesses()
  }

  return { processes, loading, fetchProcesses, start, stop, restart }
})