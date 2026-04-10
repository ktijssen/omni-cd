<template>
  <div class="container">
    <div class="header">
      <h1 style="font-size:18px;font-weight:600;color:#fff;letter-spacing:-0.3px;">Audit Log</h1>
      <div class="header-buttons" style="margin-left:auto;display:flex;align-items:center;gap:8px">
        <button class="btn-omni" @click="openFilesModal">Show Files</button>
        <button class="btn-omni" @click="downloadToday">Download Today's Audit</button>
      </div>
    </div>

    <!-- Filters -->
    <div class="logs-filters">
      <input
        v-model="search"
        type="text"
        placeholder="Search..."
        style="background:#1e2130;border:1px solid #3d4059;border-radius:4px;color:#c4c4c9;font-size:13px;padding:6px 12px;outline:none;width:200px;font-family:inherit;transition:border-color 0.2s;"
        @focus="($event.target as HTMLInputElement).style.borderColor='#ff8b59'"
        @blur="($event.target as HTMLInputElement).style.borderColor='#3d4059'"
      />
      <div class="filter-dropdown-wrap">
        <button class="filter-select-btn" :class="{ active: !!kindFilter }" @click="activeDropdown = activeDropdown === 'kind' ? null : 'kind'">
          <span class="filter-select-label">
            <label>Kind</label>
            <span>{{ kindFilter || 'All' }}</span>
          </span>
          <svg width="16" height="16" viewBox="0 0 16 16" fill="currentColor" style="flex-shrink:0;transition:transform 0.2s" :style="{ transform: activeDropdown === 'kind' ? 'rotate(180deg)' : 'none' }"><path fill-rule="evenodd" clip-rule="evenodd" d="M3.14645 5.14645C3.34171 4.95118 3.65829 4.95118 3.85355 5.14645L8 9.29289L12.1464 5.14645C12.3417 4.95118 12.6583 4.95118 12.8536 5.14645C13.0488 5.34171 13.0488 5.65829 12.8536 5.85355L8.35355 10.3536C8.15829 10.5488 7.84171 10.5488 7.64645 10.3536L3.14645 5.85355C2.95118 5.65829 2.95118 5.34171 3.14645 5.14645Z"/></svg>
        </button>
        <div v-if="activeDropdown === 'kind'" class="cluster-list-menu" style="min-width:160px">
          <button class="cluster-list-menu-item" :class="{ active: kindFilter === '' }" @click="kindFilter = ''; activeDropdown = null">{{ kindFilter === '' ? '✓ ' : '\u00a0\u00a0 ' }}All</button>
          <button v-for="k in uniqueKinds" :key="k" class="cluster-list-menu-item" :class="{ active: kindFilter === k }" @click="kindFilter = k; activeDropdown = null">{{ kindFilter === k ? '✓ ' : '\u00a0\u00a0 ' }}{{ k }}</button>
        </div>
      </div>
      <div class="filter-dropdown-wrap">
        <button class="filter-select-btn" :class="{ active: !!actionFilter }" @click="activeDropdown = activeDropdown === 'action' ? null : 'action'">
          <span class="filter-select-label">
            <label>Action</label>
            <span>{{ actionFilter || 'All' }}</span>
          </span>
          <svg width="16" height="16" viewBox="0 0 16 16" fill="currentColor" style="flex-shrink:0;transition:transform 0.2s" :style="{ transform: activeDropdown === 'action' ? 'rotate(180deg)' : 'none' }"><path fill-rule="evenodd" clip-rule="evenodd" d="M3.14645 5.14645C3.34171 4.95118 3.65829 4.95118 3.85355 5.14645L8 9.29289L12.1464 5.14645C12.3417 4.95118 12.6583 4.95118 12.8536 5.14645C13.0488 5.34171 13.0488 5.65829 12.8536 5.85355L8.35355 10.3536C8.15829 10.5488 7.84171 10.5488 7.64645 10.3536L3.14645 5.85355C2.95118 5.65829 2.95118 5.34171 3.14645 5.14645Z"/></svg>
        </button>
        <div v-if="activeDropdown === 'action'" class="cluster-list-menu" style="min-width:160px">
          <button class="cluster-list-menu-item" :class="{ active: actionFilter === '' }" @click="actionFilter = ''; activeDropdown = null">{{ actionFilter === '' ? '✓ ' : '\u00a0\u00a0 ' }}All</button>
          <button v-for="a in uniqueActions" :key="a" class="cluster-list-menu-item" :class="{ active: actionFilter === a }" @click="actionFilter = a; activeDropdown = null">{{ actionFilter === a ? '✓ ' : '\u00a0\u00a0 ' }}{{ a }}</button>
        </div>
      </div>
      <button
        v-if="search || kindFilter || actionFilter"
        class="btn-omni"
        @click="clearFilters"
      >✕ Clear</button>
      <span style="font-size:11px;color:#5b5c64;margin-left:4px">{{ filtered.length }} / {{ entries.length }}</span>
    </div>

    <!-- Table -->
    <div style="margin-top:8px;overflow-x:auto;">
      <div v-if="loading" style="color:#7d7d85;padding:24px;text-align:center">Loading...</div>
      <div v-else-if="error" style="color:#e05c5c;padding:24px;text-align:center">{{ error }}</div>
      <table v-else class="audit-table">
        <thead>
          <tr>
            <th style="width:1%;white-space:nowrap">Time</th>
            <th style="width:1%;white-space:nowrap">User</th>
            <th style="width:1%;white-space:nowrap">Action</th>
            <th style="width:1%;white-space:nowrap">Resource</th>
            <th style="width:1%;white-space:nowrap">Kind</th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="(e, i) in filtered" :key="i">
            <td class="audit-ts">{{ formatTs(e.timestamp) }}</td>
            <td>{{ e.user || '—' }}</td>
            <td>{{ e.action }}</td>
            <td>{{ e.resource || '—' }}</td>
            <td><span class="audit-kind">{{ e.kind }}</span></td>
          </tr>
          <tr v-if="filtered.length === 0">
            <td colspan="5" style="text-align:center;color:#5b5c64;padding:24px">
              {{ entries.length > 0 ? 'No entries match the current filters' : 'No audit entries yet' }}
            </td>
          </tr>
        </tbody>
      </table>
    </div>
    <!-- Audit files modal -->
    <div v-if="showFilesModal" class="modal show" @click.self="showFilesModal = false">
      <div class="modal-content" style="max-width:560px" @click.stop>
        <div class="logs-modal-header">
          <div class="logs-modal-title">Audit Files</div>
          <button class="modal-close" @click="showFilesModal = false">&times;</button>
        </div>
        <div style="padding:16px 24px;min-height:80px">
          <div v-if="filesLoading" style="color:#7d7d85;text-align:center;padding:24px">Loading...</div>
          <table v-else-if="auditFiles.length > 0" style="width:100%;border-collapse:collapse;font-size:12px">
            <thead>
              <tr>
                <th style="text-align:left;color:#7d7d85;font-weight:400;padding:4px 8px">Date</th>
                <th style="text-align:right;color:#7d7d85;font-weight:400;padding:4px 8px">Size</th>
                <th style="padding:4px 8px"></th>
              </tr>
            </thead>
            <tbody>
              <tr v-for="f in auditFiles" :key="f.date" style="border-top:1px solid #1f222e">
                <td style="padding:6px 8px;color:#e8e8e9">{{ f.date }}</td>
                <td style="padding:6px 8px;color:#7d7d85;text-align:right">{{ formatBytes(f.size) }}</td>
                <td style="padding:6px 8px;text-align:right">
                  <button class="btn-omni" @click="downloadFile(f.date)">Download</button>
                </td>
              </tr>
            </tbody>
          </table>
          <div v-else style="color:#7d7d85;text-align:center;padding:24px">No audit files found</div>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, onUnmounted } from 'vue'
import type { AuditEntry } from '@/types'

const entries = ref<AuditEntry[]>([])
const loading = ref(false)
const error = ref('')

const search = ref('')
const kindFilter = ref('')
const actionFilter = ref('')
const activeDropdown = ref<'kind' | 'action' | null>(null)

function closeMenu(e: MouseEvent) {
  if (!(e.target as HTMLElement).closest('.filter-dropdown-wrap')) activeDropdown.value = null
}
onMounted(() => document.addEventListener('click', closeMenu))
onUnmounted(() => document.removeEventListener('click', closeMenu))

async function load() {
  loading.value = true
  error.value = ''
  try {
    const r = await fetch('/api/audit')
    if (!r.ok) throw new Error(`HTTP ${r.status}`)
    entries.value = await r.json()
  } catch (e: unknown) {
    error.value = e instanceof Error ? e.message : 'Failed to load audit log'
  }
  loading.value = false
}

onMounted(load)

const uniqueKinds = computed(() => [...new Set(entries.value.map(e => e.kind).filter(Boolean))].sort())
const uniqueActions = computed(() => [...new Set(entries.value.map(e => e.action).filter(Boolean))].sort())

const filtered = computed(() => {
  const s = search.value.toLowerCase()
  return entries.value.filter(e => {
    if (kindFilter.value && e.kind !== kindFilter.value) return false
    if (actionFilter.value && e.action !== actionFilter.value) return false
    if (s && !e.user.toLowerCase().includes(s) && !e.action.toLowerCase().includes(s) && !e.resource.toLowerCase().includes(s)) return false
    return true
  })
})

function clearFilters() {
  search.value = ''
  kindFilter.value = ''
  actionFilter.value = ''
}

function formatTs(ts: string): string {
  if (!ts) return ''
  const d = new Date(ts)
  if (isNaN(d.getTime())) return ts
  return d.toLocaleString()
}

// Files modal
const showFilesModal = ref(false)
const filesLoading = ref(false)
const auditFiles = ref<{ date: string; size: number }[]>([])

async function openFilesModal() {
  showFilesModal.value = true
  filesLoading.value = true
  auditFiles.value = []
  try {
    const r = await fetch('/api/audit/files')
    auditFiles.value = r.ok ? await r.json() : []
  } catch { auditFiles.value = [] }
  filesLoading.value = false
}

function downloadToday() {
  const today = new Date().toISOString().slice(0, 10)
  window.location.href = '/api/audit/download?date=' + today
}

function downloadFile(date: string) {
  window.location.href = '/api/audit/download?date=' + date
}

function formatBytes(b: number): string {
  if (b < 1024) return b + ' B'
  if (b < 1024 * 1024) return (b / 1024).toFixed(1) + ' KB'
  return (b / (1024 * 1024)).toFixed(1) + ' MB'
}
</script>
