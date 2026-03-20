<template>
  <div class="container">
    <div class="header">
      <h1 style="font-size:18px;font-weight:600;color:#fff;letter-spacing:-0.3px;">Logs</h1>
      <div class="header-buttons" style="margin-left:auto;display:flex;align-items:center;gap:8px">
        <button class="btn-sort btn-primary" @click="openLogFilesModal">Show Logs</button>
        <button class="btn-sort btn-primary" @click="downloadTodaysLogs">Download Today's Logs</button>
      </div>
    </div>

    <!-- Filters -->
    <div class="logs-filters">
      <input
        v-model="logsSearch"
        type="text"
        placeholder="Search logs..."
        style="background:#13141c;border:1px solid #2c2e38;border-radius:4px;color:#e8e8e9;font-size:12px;padding:3px 8px;outline:none;width:180px;font-family:inherit;"
      />
      <select
        v-model="logsComponentFilter"
        style="background:#13141c;border:1px solid #2c2e38;border-radius:4px;color:#e8e8e9;font-size:12px;padding:3px 8px;outline:none;font-family:inherit;cursor:pointer"
      >
        <option value="">All components</option>
        <option v-for="c in uniqueComponents" :key="c" :value="c">{{ c }}</option>
      </select>
      <select
        v-model="logsOrder"
        style="background:#13141c;border:1px solid #2c2e38;border-radius:4px;color:#e8e8e9;font-size:12px;padding:3px 8px;outline:none;font-family:inherit;cursor:pointer"
      >
        <option value="oldest">Oldest first</option>
        <option value="newest">Newest first</option>
      </select>
      <button
        v-for="lv in levelButtons"
        :key="lv"
        class="btn-sort btn-primary"
        :class="{ active: logsLevelFilter === lv }"
        @click="toggleLevel(lv)"
      >{{ lv }}</button>
      <button
        v-if="logsSearch || logsLevelFilter || logsComponentFilter"
        class="btn-sort"
        @click="clearFilters"
      >✕ Clear</button>
      <span style="font-size:11px;color:#5b5c64;margin-left:4px">{{ filteredLogs.length }} / {{ allLogs.length }}</span>
    </div>

    <!-- Log body -->
    <div class="logs-page" style="height:calc(100vh - 160px);padding:0 0 12px;margin-top:8px">
      <div class="logs-page-body" id="logs-page-container">
        <div
          v-for="(entry, i) in displayedLogs"
          :key="i"
          class="log-entry"
        >
          <span class="log-ts">{{ formatTs(entry.timestamp) }}</span>
          &nbsp;
          <span :class="levelClass(entry.level)" style="font-size:10px;min-width:36px;display:inline-block">{{ entry.level || 'INFO' }}</span>
          &nbsp;
          <span v-if="entry.label" class="log-label">[{{ entry.label }}]</span>
          <span v-if="entry.label">&nbsp;</span>
          <span class="log-msg">{{ entry.message }}</span>
        </div>
        <div v-if="displayedLogs.length === 0" class="log-entry" style="color:#5b5c64">
          {{ allLogs.length > 0 ? 'No logs match the current filters' : 'No logs yet' }}
        </div>
      </div>
    </div>

    <!-- Log files modal -->
    <div v-if="showLogFilesModal" class="modal show" @click.self="showLogFilesModal = false">
      <div class="modal-content" style="max-width:560px" @click.stop>
        <div class="logs-modal-header">
          <div class="logs-modal-title">Log Files</div>
          <button class="modal-close" @click="showLogFilesModal = false">&times;</button>
        </div>
        <div style="padding:16px 24px;min-height:80px">
          <div v-if="logFilesLoading" style="color:#7d7d85;text-align:center;padding:24px">Loading...</div>
          <table v-else-if="logFiles.length > 0" style="width:100%;border-collapse:collapse;font-size:12px">
            <thead>
              <tr>
                <th style="text-align:left;color:#7d7d85;font-weight:400;padding:4px 8px">Date</th>
                <th style="text-align:right;color:#7d7d85;font-weight:400;padding:4px 8px">Size</th>
                <th style="padding:4px 8px"></th>
              </tr>
            </thead>
            <tbody>
              <tr v-for="f in logFiles" :key="f.date" style="border-top:1px solid #1f222e">
                <td style="padding:6px 8px;color:#e8e8e9">{{ f.date }}</td>
                <td style="padding:6px 8px;color:#7d7d85;text-align:right">{{ formatBytes(f.size) }}</td>
                <td style="padding:6px 8px;text-align:right">
                  <button class="btn-sort btn-primary" @click="downloadLogFile(f.date)">Download</button>
                </td>
              </tr>
            </tbody>
          </table>
          <div v-else style="color:#7d7d85;text-align:center;padding:24px">No log files found</div>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, ref } from 'vue'
import { useAppStore } from '@/stores/appStore'
import type { LogEntry } from '@/types'

const appStore = useAppStore()

const logsSearch = ref('')
const logsLevelFilter = ref('')
const logsComponentFilter = ref('')
const logsOrder = ref<'oldest' | 'newest'>('oldest')

const allLogs = computed((): LogEntry[] => appStore.state?.logs ?? [])

const uniqueComponents = computed(() => {
  const set = new Set<string>()
  allLogs.value.forEach(l => { if (l.label) set.add(l.label) })
  return [...set].sort()
})

const levelButtons = computed(() => {
  const base = ['INFO', 'WARN', 'ERROR']
  if (appStore.state?.logLevel === 'DEBUG') return ['DEBUG', ...base]
  return base
})

const filteredLogs = computed(() => {
  const search = logsSearch.value.toLowerCase()
  return allLogs.value.filter(l => {
    if (logsLevelFilter.value && l.level !== logsLevelFilter.value) return false
    if (logsComponentFilter.value && l.label !== logsComponentFilter.value) return false
    if (search && !l.message.toLowerCase().includes(search) && !(l.label || '').toLowerCase().includes(search)) return false
    return true
  })
})

const displayedLogs = computed(() =>
  logsOrder.value === 'newest' ? [...filteredLogs.value].reverse() : filteredLogs.value
)

function toggleLevel(lv: string) {
  logsLevelFilter.value = logsLevelFilter.value === lv ? '' : lv
}

function clearFilters() {
  logsSearch.value = ''
  logsLevelFilter.value = ''
  logsComponentFilter.value = ''
}

function formatTs(ts: string): string {
  if (!ts) return ''
  const d = new Date(ts)
  if (isNaN(d.getTime())) return ts
  return d.toLocaleTimeString()
}

function levelClass(level: string): string {
  if (level === 'DEBUG') return 'log-debug'
  if (level === 'WARN') return 'log-warn'
  if (level === 'ERROR') return 'log-error'
  return 'log-info'
}

// Log files modal
const showLogFilesModal = ref(false)
const logFilesLoading = ref(false)
const logFiles = ref<{ date: string; size: number }[]>([])

async function openLogFilesModal() {
  showLogFilesModal.value = true
  logFilesLoading.value = true
  logFiles.value = []
  try {
    const r = await fetch('/api/logs/files')
    logFiles.value = r.ok ? await r.json() : []
  } catch { logFiles.value = [] }
  logFilesLoading.value = false
}

function downloadTodaysLogs() {
  const today = new Date().toISOString().slice(0, 10)
  window.location.href = '/api/logs/download?date=' + today
}

function downloadLogFile(date: string) {
  window.location.href = '/api/logs/download?date=' + date
}

function formatBytes(b: number): string {
  if (b < 1024) return b + ' B'
  if (b < 1024 * 1024) return (b / 1024).toFixed(1) + ' KB'
  return (b / (1024 * 1024)).toFixed(1) + ' MB'
}
</script>
