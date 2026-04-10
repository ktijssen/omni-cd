<template>
  <div class="container">
    <div class="header">
      <h1 style="font-size:18px;font-weight:600;color:#fff;letter-spacing:-0.3px;">Logs</h1>
      <div class="header-buttons" style="margin-left:auto;display:flex;align-items:center;gap:8px">
        <button class="btn-omni" @click="openLogFilesModal">Show Logs</button>
        <button class="btn-omni" @click="downloadTodaysLogs">Download Today's Logs</button>
      </div>
    </div>

    <!-- Filters -->
    <div class="logs-filters">
      <input
        v-model="logsSearch"
        type="text"
        placeholder="Search logs..."
        style="background:#1e2130;border:1px solid #3d4059;border-radius:4px;color:#c4c4c9;font-size:13px;padding:6px 12px;outline:none;width:200px;font-family:inherit;transition:border-color 0.2s;"
        @focus="($event.target as HTMLInputElement).style.borderColor='#ff8b59'"
        @blur="($event.target as HTMLInputElement).style.borderColor='#3d4059'"
      />
      <div class="filter-dropdown-wrap">
        <button class="filter-select-btn" :class="{ active: !!logsComponentFilter }" @click="activeDropdown = activeDropdown === 'component' ? null : 'component'">
          <span class="filter-select-label">
            <label>Component</label>
            <span>{{ logsComponentFilter || 'All' }}</span>
          </span>
          <svg width="16" height="16" viewBox="0 0 16 16" fill="currentColor" style="flex-shrink:0;transition:transform 0.2s" :style="{ transform: activeDropdown === 'component' ? 'rotate(180deg)' : 'none' }"><path fill-rule="evenodd" clip-rule="evenodd" d="M3.14645 5.14645C3.34171 4.95118 3.65829 4.95118 3.85355 5.14645L8 9.29289L12.1464 5.14645C12.3417 4.95118 12.6583 4.95118 12.8536 5.14645C13.0488 5.34171 13.0488 5.65829 12.8536 5.85355L8.35355 10.3536C8.15829 10.5488 7.84171 10.5488 7.64645 10.3536L3.14645 5.85355C2.95118 5.65829 2.95118 5.34171 3.14645 5.14645Z"/></svg>
        </button>
        <div v-if="activeDropdown === 'component'" class="cluster-list-menu" style="min-width:160px">
          <button class="cluster-list-menu-item" :class="{ active: logsComponentFilter === '' }" @click="logsComponentFilter = ''; activeDropdown = null">{{ logsComponentFilter === '' ? '✓ ' : '\u00a0\u00a0 ' }}All</button>
          <button v-for="c in uniqueComponents" :key="c" class="cluster-list-menu-item" :class="{ active: logsComponentFilter === c }" @click="logsComponentFilter = c; activeDropdown = null">{{ logsComponentFilter === c ? '✓ ' : '\u00a0\u00a0 ' }}{{ c }}</button>
        </div>
      </div>
      <div class="filter-dropdown-wrap">
        <button class="filter-select-btn" @click="activeDropdown = activeDropdown === 'order' ? null : 'order'">
          <span class="filter-select-label">
            <label>Order</label>
            <span>{{ logsOrder === 'newest' ? 'Newest first' : 'Oldest first' }}</span>
          </span>
          <svg width="16" height="16" viewBox="0 0 16 16" fill="currentColor" style="flex-shrink:0;transition:transform 0.2s" :style="{ transform: activeDropdown === 'order' ? 'rotate(180deg)' : 'none' }"><path fill-rule="evenodd" clip-rule="evenodd" d="M3.14645 5.14645C3.34171 4.95118 3.65829 4.95118 3.85355 5.14645L8 9.29289L12.1464 5.14645C12.3417 4.95118 12.6583 4.95118 12.8536 5.14645C13.0488 5.34171 13.0488 5.65829 12.8536 5.85355L8.35355 10.3536C8.15829 10.5488 7.84171 10.5488 7.64645 10.3536L3.14645 5.85355C2.95118 5.65829 2.95118 5.34171 3.14645 5.14645Z"/></svg>
        </button>
        <div v-if="activeDropdown === 'order'" class="cluster-list-menu" style="min-width:140px">
          <button class="cluster-list-menu-item" :class="{ active: logsOrder === 'oldest' }" @click="logsOrder = 'oldest'; activeDropdown = null">{{ logsOrder === 'oldest' ? '✓ ' : '\u00a0\u00a0 ' }}Oldest first</button>
          <button class="cluster-list-menu-item" :class="{ active: logsOrder === 'newest' }" @click="logsOrder = 'newest'; activeDropdown = null">{{ logsOrder === 'newest' ? '✓ ' : '\u00a0\u00a0 ' }}Newest first</button>
        </div>
      </div>
      <div class="filter-dropdown-wrap">
        <button class="filter-select-btn" :class="{ active: !!logsLevelFilter }" @click="activeDropdown = activeDropdown === 'level' ? null : 'level'">
          <span class="filter-select-label">
            <label>Level</label>
            <span>{{ logsLevelFilter || 'All' }}</span>
          </span>
          <svg width="16" height="16" viewBox="0 0 16 16" fill="currentColor" style="flex-shrink:0;transition:transform 0.2s" :style="{ transform: activeDropdown === 'level' ? 'rotate(180deg)' : 'none' }"><path fill-rule="evenodd" clip-rule="evenodd" d="M3.14645 5.14645C3.34171 4.95118 3.65829 4.95118 3.85355 5.14645L8 9.29289L12.1464 5.14645C12.3417 4.95118 12.6583 4.95118 12.8536 5.14645C13.0488 5.34171 13.0488 5.65829 12.8536 5.85355L8.35355 10.3536C8.15829 10.5488 7.84171 10.5488 7.64645 10.3536L3.14645 5.85355C2.95118 5.65829 2.95118 5.34171 3.14645 5.14645Z"/></svg>
        </button>
        <div v-if="activeDropdown === 'level'" class="cluster-list-menu" style="min-width:120px">
          <button class="cluster-list-menu-item" :class="{ active: logsLevelFilter === '' }" @click="logsLevelFilter = ''; activeDropdown = null">{{ logsLevelFilter === '' ? '✓ ' : '\u00a0\u00a0 ' }}All</button>
          <button v-for="lv in levelButtons" :key="lv" class="cluster-list-menu-item" :class="{ active: logsLevelFilter === lv }" @click="logsLevelFilter = lv; activeDropdown = null">{{ logsLevelFilter === lv ? '✓ ' : '\u00a0\u00a0 ' }}{{ lv }}</button>
        </div>
      </div>
      <button
        v-if="logsSearch || logsLevelFilter || logsComponentFilter"
        class="btn-omni"
        @click="clearFilters"
      >✕ Clear</button>
      <span style="font-size:11px;color:#5b5c64;margin-left:4px">{{ filteredLogs.length }} / {{ allLogs.length }}</span>
    </div>

    <!-- Table -->
    <div style="margin-top:8px;overflow-x:auto;">
      <table class="audit-table">
        <thead>
          <tr>
            <th style="width:1%;white-space:nowrap">Time</th>
            <th style="width:1%;white-space:nowrap">Level</th>
            <th style="width:1%;white-space:nowrap">Component</th>
            <th>Message</th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="(entry, i) in displayedLogs" :key="i">
            <td class="audit-ts">{{ formatTs(entry.timestamp) }}</td>
            <td><span :class="levelClass(entry.level)" style="font-size:11px">{{ entry.level || 'INFO' }}</span></td>
            <td class="audit-kind">{{ entry.label || '—' }}</td>
            <td style="color:#e8e8e9">{{ parseMsg(entry.message) }}</td>
          </tr>
          <tr v-if="displayedLogs.length === 0">
            <td colspan="4" style="text-align:center;color:#5b5c64;padding:24px">
              {{ allLogs.length > 0 ? 'No logs match the current filters' : 'No logs yet' }}
            </td>
          </tr>
        </tbody>
      </table>
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
                  <button class="btn-omni" @click="downloadLogFile(f.date)">Download</button>
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
import { computed, ref, onMounted, onUnmounted } from 'vue'
import { useAppStore } from '@/stores/appStore'
import type { LogEntry } from '@/types'

const appStore = useAppStore()

const logsSearch = ref('')
const logsLevelFilter = ref('')
const logsComponentFilter = ref('')
const logsOrder = ref<'oldest' | 'newest'>('oldest')
const activeDropdown = ref<'component' | 'order' | 'level' | null>(null)

function closeMenu(e: MouseEvent) {
  if (!(e.target as HTMLElement).closest('.filter-dropdown-wrap')) activeDropdown.value = null
}
onMounted(() => document.addEventListener('click', closeMenu))
onUnmounted(() => document.removeEventListener('click', closeMenu))

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
    if (search && !parseMsg(l.message).toLowerCase().includes(search) && !(l.label || '').toLowerCase().includes(search)) return false
    return true
  })
})

const displayedLogs = computed(() =>
  logsOrder.value === 'newest' ? [...filteredLogs.value].reverse() : filteredLogs.value
)


function clearFilters() {
  logsSearch.value = ''
  logsLevelFilter.value = ''
  logsComponentFilter.value = ''
}

function formatTs(ts: string): string {
  if (!ts) return ''
  const d = new Date(ts)
  if (isNaN(d.getTime())) return ts
  return d.toLocaleString()
}

function parseMsg(raw: string): string {
  try {
    const parsed = JSON.parse(raw)
    if (parsed.msg) return parsed.msg
  } catch { /* not JSON */ }
  return raw
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
