<template>
  <div class="container">
    <div class="header" style="border-bottom:none;">
      <h1 style="font-size:18px;font-weight:600;color:#fff;letter-spacing:-0.3px;">Clusters</h1>
      <div class="header-buttons">
        <template v-if="authStore.isAdmin()">
          <span v-if="isRunning" class="spinner"></span>
          <button class="btn-omni" :disabled="isRunning" @click="triggerCheck">
            {{ gitRunning ? 'Refreshing...' : 'Refresh' }}
          </button>
          <button class="btn-omni" :disabled="isRunning" @click="triggerReconcile">
            {{ syncRunning ? 'Syncing...' : 'Sync' }}
          </button>
        </template>
        <input
          v-model="clusterSearch"
          type="text"
          placeholder="Search clusters..."
          style="background:#1e2130;border:1px solid #3d4059;border-radius:4px;color:#c4c4c9;font-size:13px;padding:6px 12px;outline:none;width:200px;font-family:inherit;transition:border-color 0.2s;margin-left:8px;"
          @focus="($event.target as HTMLInputElement).style.borderColor='#ff8b59'"
          @blur="($event.target as HTMLInputElement).style.borderColor='#3d4059'"
        />
      </div>
    </div>

    <!-- Omni not configured -->
    <div v-if="!state?.omniConfigured" class="placeholder-page">
      <div class="placeholder-icon">🔌</div>
      <div class="placeholder-title">Please configure an Omni Instance first in Settings/Instances</div>
    </div>

    <!-- Clusters disabled -->
    <div v-else-if="!state?.clustersEnabled" class="placeholder-page">
      <div class="placeholder-icon">⏸</div>
      <div class="placeholder-title">Clusters management disabled</div>
      <div class="placeholder-sub">Enable clusters management in the configuration.</div>
    </div>

    <template v-else>
      <!-- Health bar -->
      <div v-if="healthTotal > 0" class="cluster-health-bar-wrap">
        <div class="cluster-health-bar" :class="{ 'has-filter': clusterStatusFilter !== null }">
          <div
            v-if="countReady"
            class="cluster-health-bar-seg cluster-health-bar-seg--ready"
            :class="{ active: clusterStatusFilter === 'ready' }"
            :style="{ width: (countReady / healthTotal * 100).toFixed(1) + '%' }"
            :title="countReady + ' ready'"
            @click="setClusterFilter('ready')"
          ></div>
          <div
            v-if="countNotReady"
            class="cluster-health-bar-seg cluster-health-bar-seg--notready"
            :class="{ active: clusterStatusFilter === 'not-ready' }"
            :style="{ width: (countNotReady / healthTotal * 100).toFixed(1) + '%' }"
            :title="countNotReady + ' not ready'"
            @click="setClusterFilter('not-ready')"
          ></div>
          <div
            v-if="countScalingUp"
            class="cluster-health-bar-seg cluster-health-bar-seg--scalingup"
            :class="{ active: clusterStatusFilter === 'scaling-up' }"
            :style="{ width: (countScalingUp / healthTotal * 100).toFixed(1) + '%' }"
            :title="countScalingUp + ' scaling up'"
            @click="setClusterFilter('scaling-up')"
          ></div>
          <div
            v-if="countScalingDown"
            class="cluster-health-bar-seg cluster-health-bar-seg--scalingdown"
            :class="{ active: clusterStatusFilter === 'scaling-down' }"
            :style="{ width: (countScalingDown / healthTotal * 100).toFixed(1) + '%' }"
            :title="countScalingDown + ' scaling down'"
            @click="setClusterFilter('scaling-down')"
          ></div>
          <div
            v-if="countDestroying"
            class="cluster-health-bar-seg cluster-health-bar-seg--destroying"
            :class="{ active: clusterStatusFilter === 'destroying' }"
            :style="{ width: (countDestroying / healthTotal * 100).toFixed(1) + '%' }"
            :title="countDestroying + ' destroying'"
            @click="setClusterFilter('destroying')"
          ></div>
          <div
            v-if="countReconfiguring"
            class="cluster-health-bar-seg cluster-health-bar-seg--reconfiguring"
            :class="{ active: clusterStatusFilter === 'reconfiguring' }"
            :style="{ width: (countReconfiguring / healthTotal * 100).toFixed(1) + '%' }"
            :title="countReconfiguring + ' reconfiguring'"
            @click="setClusterFilter('reconfiguring')"
          ></div>
        </div>
        <div class="cluster-health-summary">
          {{ allClusters.length }} clusters
          <template v-if="countReady">
            &nbsp;·&nbsp;<span
              style="color:#4ade80;cursor:pointer"
              :style="{ fontWeight: clusterStatusFilter === 'ready' ? '700' : 'normal' }"
              @click="setClusterFilter('ready')"
            >{{ countReady }} Ready</span>
          </template>
          <template v-if="countNotReady">
            &nbsp;·&nbsp;<span
              style="color:#f87171;cursor:pointer"
              :style="{ fontWeight: clusterStatusFilter === 'not-ready' ? '700' : 'normal' }"
              @click="setClusterFilter('not-ready')"
            >{{ countNotReady }} Not Ready</span>
          </template>
          <template v-if="countScalingUp">
            &nbsp;·&nbsp;<span
              style="color:#60a5fa;cursor:pointer"
              :style="{ fontWeight: clusterStatusFilter === 'scaling-up' ? '700' : 'normal' }"
              @click="setClusterFilter('scaling-up')"
            >{{ countScalingUp }} Scaling Up</span>
          </template>
          <template v-if="countScalingDown">
            &nbsp;·&nbsp;<span
              style="color:#f59e0b;cursor:pointer"
              :style="{ fontWeight: clusterStatusFilter === 'scaling-down' ? '700' : 'normal' }"
              @click="setClusterFilter('scaling-down')"
            >{{ countScalingDown }} Scaling Down</span>
          </template>
          <template v-if="countDestroying">
            &nbsp;·&nbsp;<span
              style="color:#f43f5e;cursor:pointer"
              :style="{ fontWeight: clusterStatusFilter === 'destroying' ? '700' : 'normal' }"
              @click="setClusterFilter('destroying')"
            >{{ countDestroying }} Destroying</span>
          </template>
          <template v-if="countReconfiguring">
            &nbsp;·&nbsp;<span
              style="color:#a78bfa;cursor:pointer"
              :style="{ fontWeight: clusterStatusFilter === 'reconfiguring' ? '700' : 'normal' }"
              @click="setClusterFilter('reconfiguring')"
            >{{ countReconfiguring }} Reconfiguring</span>
          </template>
          <template v-if="clusterStatusFilter !== null">
            &nbsp;<span style="cursor:pointer;color:#9fa1a6;text-decoration:underline;font-size:11px" @click="clearClusterFilter">clear</span>
          </template>
        </div>
      </div>

      <!-- Toolbar: sync filter buttons + sort + page size -->
      <div style="display:flex;align-items:center;gap:8px;padding:0 0 12px;flex-wrap:wrap;">
        <button
          v-for="def in visibleSyncDefs"
          :key="def.key"
          class="btn-omni"
          :class="{ active: !!clusterSyncFilters[def.key] }"
          @click="toggleSyncFilter(def.key)"
        >{{ def.label }}</button>
        <div style="margin-left:auto;display:flex;align-items:center;gap:8px">
          <button class="btn-omni active" @click="toggleSort">{{ sortAZ ? 'A→Z' : 'Z→A' }}</button>
          <div class="page-size-bar">
            <button
              v-for="n in [5, 10, 15, 20, 0]"
              :key="n"
              class="btn-omni"
              :class="{ active: pageSize === n }"
              @click="setPageSize(n)"
            >{{ n === 0 ? 'All' : n }}</button>
          </div>
        </div>
      </div>

      <!-- No clusters -->
      <div v-if="displayClusters.length === 0 && allClusters.length === 0" class="placeholder-page">
        <div class="placeholder-title">No clusters found</div>
        <div class="placeholder-sub">Clusters defined in your git repo will appear here.</div>
      </div>
      <div v-else-if="displayClusters.length === 0" style="padding:24px;color:#5b5c64">No clusters match the current filters</div>

      <template v-else>
        <!-- Cluster grid -->
        <div class="cluster-grid">
          <div
            v-for="cluster in pageClusters"
            :key="cluster.id"
            class="cluster-card clickable"
            :data-status="cluster.status || 'idle'"
            v-bind="clusterCardAttrs(cluster)"
            @click="goToCluster(cluster.id)"
          >
            <div class="cluster-card-accent"></div>
            <div class="cluster-card-body">
              <!-- Header -->
              <div class="cluster-card-header">
                <div style="display:flex;align-items:center;gap:6px;min-width:0;flex-wrap:wrap;">
                  <span class="cluster-card-title">{{ cluster.id }}</span>
                  <a
                    v-if="state?.omniEndpoint"
                    class="btn-omni"
                    style="font-size:11px;padding:1px 6px;text-decoration:none"
                    :href="omniClusterUrl(cluster.id)"
                    target="_blank"
                    @click.stop
                    title="Open in Omni"
                  >&#8599; Open in Omni</a>
                </div>
                <span class="cluster-card-status" :style="{ color: mgmtColor(cluster), flexShrink: '0' }">{{ mgmtBadge(cluster) }}</span>
              </div>

              <div class="cluster-card-divider"></div>

              <!-- Meta grid -->
              <div class="cluster-card-meta">
                <div class="cluster-card-meta-pair">
                  <span class="cluster-card-meta-label">Sync Status:</span>
                  <span class="cluster-card-meta-value" :style="{ color: syncColor(cluster) }" v-html="syncText(cluster)"></span>
                </div>
                <div class="cluster-card-meta-pair">
                  <span class="cluster-card-meta-label">Cluster Health:</span>
                  <span class="cluster-card-meta-value" :style="{ color: healthColor(cluster) }" v-html="healthText(cluster)"></span>
                </div>
                <div class="cluster-card-meta-pair">
                  <span class="cluster-card-meta-label">Talos Version:</span>
                  <span class="cluster-card-meta-value">
                    <span v-if="cluster.talosVersion">{{ cluster.talosVersion }}</span>
                    <span v-else style="color:#5b5c64">—</span>
                  </span>
                </div>
                <div class="cluster-card-meta-pair">
                  <span class="cluster-card-meta-label">Kubernetes Version:</span>
                  <span class="cluster-card-meta-value">
                    <span v-if="cluster.kubernetesVersion">{{ cluster.kubernetesVersion }}</span>
                    <span v-else style="color:#5b5c64">—</span>
                  </span>
                </div>
                <div class="cluster-card-meta-pair">
                  <span class="cluster-card-meta-label">Repository:</span>
                  <span class="cluster-card-meta-value">
                    <span v-if="cluster.repoName">{{ cluster.repoName }}</span>
                    <span v-else style="color:#5b5c64">—</span>
                  </span>
                </div>
                <div class="cluster-card-meta-pair">
                  <span class="cluster-card-meta-label">Branch:</span>
                  <span class="cluster-card-meta-value">
                    <span v-if="clusterRepo(cluster)?.branch">{{ clusterRepo(cluster)!.branch }}</span>
                    <span v-else style="color:#5b5c64">—</span>
                  </span>
                </div>
                <div class="cluster-card-meta-pair">
                  <span class="cluster-card-meta-label">Created At:</span>
                  <span class="cluster-card-meta-value">
                    <span v-if="!isZeroTime(cluster.createdAt)">
                      {{ fmtDateTime(cluster.createdAt) }}
                      <span style="color:#7d7d85">({{ ago(cluster.createdAt) }})</span>
                    </span>
                    <span v-else style="color:#5b5c64">—</span>
                  </span>
                </div>
                <div class="cluster-card-meta-pair">
                  <span class="cluster-card-meta-label">Last Sync:</span>
                  <span class="cluster-card-meta-value">
                    <span v-if="!isZeroTime(cluster.lastSyncTime)">
                      {{ fmtDateTime(cluster.lastSyncTime) }}
                      <span style="color:#7d7d85">({{ ago(cluster.lastSyncTime) }})</span>
                    </span>
                    <span v-else style="color:#5b5c64">—</span>
                  </span>
                </div>
              </div>

              <div class="cluster-card-divider"></div>

              <!-- Pool rows -->
              <div
                v-for="(sec, idx) in clusterSections(cluster)"
                :key="idx"
                class="cluster-pool-row"
              >
                <div class="cluster-pool-row-label">{{ sec.label }}</div>
                <div class="cluster-pool-row-count">{{ sec.count }}</div>
                <div class="cluster-pool-row-mc">
                  <span v-if="sec.mc">{{ sec.mc }}</span>
                  <span v-else style="color:#2c2e38">—</span>
                </div>
              </div>

              <div class="cluster-card-divider" style="margin-top:8px"></div>

              <!-- Actions -->
              <div v-if="authStore.isAdmin()" class="cluster-card-actions" @click.stop>
                <button
                  class="btn-omni"
                  :disabled="!!clusterActionPending[cluster.id]"
                  @click="refreshCluster(cluster)"
                  title="Re-read live state from Omni"
                >↺ {{ clusterActionPending[cluster.id] === 'refresh' ? 'Refreshing...' : 'Refresh' }}</button>
                <button
                  v-if="cluster.status !== 'deleting' && (cluster.status === 'unmanaged' || cluster.status === 'orphaned')"
                  class="btn-omni"
                  @click="exportCluster(cluster)"
                  title="Export cluster as YAML template"
                >↓ Export</button>
                <button
                  v-if="cluster.status !== 'deleting' && cluster.status !== 'unmanaged' && cluster.status !== 'orphaned'"
                  class="btn-omni"
                  :disabled="!!clusterActionPending[cluster.id]"
                  @click="syncCluster(cluster)"
                  title="Force sync this cluster from Git"
                >⇅ {{ clusterActionPending[cluster.id] === 'sync' ? 'Syncing...' : 'Sync' }}</button>
                <button
                  v-if="cluster.status !== 'deleting' && cluster.status !== 'unmanaged' && cluster.status !== 'orphaned'"
                  class="btn-omni auto-sync"
                  :class="{ active: cluster.autoSync !== false }"
                  @click="toggleAutoSync(cluster)"
                  title="Toggle per-cluster auto sync"
                >{{ cluster.autoSync === false ? '○ Auto-Sync: Off' : '● Auto-Sync: On' }}</button>
                <button
                  v-if="cluster.status !== 'deleting' && cluster.status !== 'unmanaged'"
                  class="btn-omni"
                  @click="deleteCluster(cluster)"
                  title="Delete this cluster from Omni"
                >✕ Delete</button>
              </div>
            </div>
          </div>
        </div>

        <!-- Pagination -->
        <div v-if="pageSize > 0 && displayClusters.length > pageSize" class="pagination">
          <button class="page-btn" :disabled="currentPage === 1" @click="currentPage--">&laquo;</button>
          <button
            v-for="p in totalPages"
            :key="p"
            class="page-btn"
            :class="{ active: p === currentPage }"
            @click="currentPage = p"
          >{{ p }}</button>
          <button class="page-btn" :disabled="currentPage === totalPages" @click="currentPage++">&raquo;</button>
        </div>
      </template>
    </template>

    <!-- Confirm modal -->
    <div v-if="confirmModal" class="modal show" @click.self="confirmModal = null">
      <div class="modal-content confirm-modal" @click.stop>
        <div class="modal-header">
          <div class="modal-title">{{ confirmModal.title }}</div>
          <button class="modal-close" @click="confirmModal = null">&times;</button>
        </div>
        <div class="modal-body confirm-body">
          <div class="confirm-message" v-html="confirmModal.message"></div>
          <div v-if="confirmModal.requireInput" class="confirm-input-prompt">{{ confirmModal.inputPrompt }}</div>
          <input
            v-if="confirmModal.requireInput"
            v-model="confirmInput"
            class="confirm-input"
            type="text"
            :placeholder="confirmModal.requireInput"
          />
          <div class="confirm-actions">
            <button class="btn-omni" @click="confirmModal = null">Cancel</button>
            <button
              class="btn-omni"
              :disabled="!!(confirmModal.requireInput && confirmInput !== confirmModal.requireInput)"
              @click="doConfirm"
            >OK</button>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, ref, reactive } from 'vue'
import { useRouter } from 'vue-router'
import { useAppStore } from '@/stores/appStore'
import { useAuthStore } from '@/stores/authStore'
import type { ResourceInfo, GitInfo, NodeGroup } from '@/types'
import { syncedIconSVG, outOfSyncIconSVG, failedIconSVG } from '@/assets/icons'

const router = useRouter()
const appStore = useAppStore()
const authStore = useAuthStore()

const state = computed(() => appStore.state)
const allClusters = computed(() => {
  const list = (state.value?.clusters ?? []).slice()
  return sortAZ.value
    ? list.sort((a, b) => a.id.localeCompare(b.id))
    : list.sort((a, b) => b.id.localeCompare(a.id))
})

// Health bar counts
const countReady = computed(() =>
  allClusters.value.filter(c => {
    const phase = c.clusterPhase || ''
    if (phase && phase !== 'running') return false
    if (!c.clusterReady || c.clusterReady === 'unknown') return false
    return c.clusterReady === 'ready' && c.kubernetesApiReady === 'ready'
  }).length
)
const countNotReady = computed(() =>
  allClusters.value.filter(c => {
    const phase = c.clusterPhase || ''
    if (phase && phase !== 'running') return false
    if (!c.clusterReady || c.clusterReady === 'unknown') return false
    return c.clusterReady === 'not-ready' || c.kubernetesApiReady === 'not-ready'
  }).length
)
const countScalingUp = computed(() => allClusters.value.filter(c => c.clusterPhase === 'scaling-up').length)
const countScalingDown = computed(() => allClusters.value.filter(c => c.clusterPhase === 'scaling-down').length)
const countDestroying = computed(() => allClusters.value.filter(c => c.clusterPhase === 'destroying').length)
const countReconfiguring = computed(() => allClusters.value.filter(c => c.clusterPhase === 'reconfiguring').length)
const healthTotal = computed(() =>
  countReady.value + countNotReady.value + countScalingUp.value +
  countScalingDown.value + countDestroying.value + countReconfiguring.value
)

// State
const clusterStatusFilter = ref<string | null>(null)
const clusterSyncFilters = reactive<Record<string, boolean>>({})
const clusterSearch = ref('')
const sortAZ = ref(true)
const pageSize = ref(10)
const currentPage = ref(1)
const clusterActionPending = reactive<Record<string, string>>({})
const reconcileRunning = ref(false)
const gitRefreshing = ref(false)

const syncRunning = computed(() => reconcileRunning.value || state.value?.lastReconcile?.status === 'running')
const gitRunning = computed(() => gitRefreshing.value)
const isRunning = computed(() => syncRunning.value || gitRunning.value)

const syncDefs = [
  { key: 'synced',    label: 'Synced' },
  { key: 'outofsync', label: 'Out of Sync' },
  { key: 'failed',    label: 'Failed' },
  { key: 'unmanaged', label: 'Unmanaged' },
  { key: 'orphaned',  label: 'Orphaned' },
]

const visibleSyncDefs = computed(() => {
  const presentKeys = new Set<string>()
  allClusters.value.forEach(c => {
    const st = c.status || ''
    const key = (st === 'success' || st === 'applied' || st === 'synced') ? 'synced'
      : st === 'outofsync' ? 'outofsync'
      : st === 'failed' ? 'failed'
      : st === 'unmanaged' ? 'unmanaged'
      : st === 'orphaned' ? 'orphaned'
      : null
    if (key) presentKeys.add(key)
  })
  return syncDefs.filter(d => presentKeys.has(d.key))
})

const displayClusters = computed(() => {
  const activeSyncKeys = Object.keys(clusterSyncFilters).filter(k => clusterSyncFilters[k])
  let result = allClusters.value.filter(c => {
    const st = c.status || ''
    const phase = c.clusterPhase || ''
    if (clusterStatusFilter.value) {
      if (clusterStatusFilter.value === 'ready') {
        if (!(c.clusterReady === 'ready' && c.kubernetesApiReady === 'ready') || (phase && phase !== 'running')) return false
      } else if (clusterStatusFilter.value === 'not-ready') {
        if (!((c.clusterReady === 'not-ready' || c.kubernetesApiReady === 'not-ready') && (!phase || phase === 'running'))) return false
      } else if (clusterStatusFilter.value === 'scaling-up') {
        if (phase !== 'scaling-up') return false
      } else if (clusterStatusFilter.value === 'scaling-down') {
        if (phase !== 'scaling-down') return false
      } else if (clusterStatusFilter.value === 'destroying') {
        if (phase !== 'destroying') return false
      } else if (clusterStatusFilter.value === 'reconfiguring') {
        if (phase !== 'reconfiguring') return false
      }
    }
    if (activeSyncKeys.length > 0) {
      const syncKey = (st === 'success' || st === 'applied' || st === 'synced') ? 'synced'
        : st === 'outofsync' ? 'outofsync'
        : st === 'failed' ? 'failed'
        : st === 'unmanaged' ? 'unmanaged'
        : st === 'orphaned' ? 'orphaned'
        : null
      if (!syncKey || !clusterSyncFilters[syncKey]) return false
    }
    return true
  })
  if (clusterSearch.value.trim()) {
    const q = clusterSearch.value.trim().toLowerCase()
    result = result.filter(c => c.id.toLowerCase().includes(q))
  }
  return result
})

const totalPages = computed(() =>
  pageSize.value === 0 ? 1 : Math.max(1, Math.ceil(displayClusters.value.length / pageSize.value))
)
const pageClusters = computed(() => {
  if (pageSize.value === 0) return displayClusters.value
  const start = (currentPage.value - 1) * pageSize.value
  return displayClusters.value.slice(start, start + pageSize.value)
})

// Confirm modal
interface ConfirmModal {
  title: string
  message: string
  requireInput?: string
  inputPrompt?: string
  onConfirm: () => void
}
const confirmModal = ref<ConfirmModal | null>(null)
const confirmInput = ref('')

function doConfirm() {
  if (confirmModal.value?.requireInput && confirmInput.value !== confirmModal.value.requireInput) return
  confirmModal.value?.onConfirm()
}

// Helpers
function setClusterFilter(key: string) {
  clusterStatusFilter.value = clusterStatusFilter.value === key ? null : key
  currentPage.value = 1
}
function clearClusterFilter() {
  clusterStatusFilter.value = null
  currentPage.value = 1
}
function toggleSyncFilter(key: string) {
  clusterSyncFilters[key] = !clusterSyncFilters[key]
  currentPage.value = 1
}
function toggleSort() {
  sortAZ.value = !sortAZ.value
  currentPage.value = 1
}
function setPageSize(n: number) {
  pageSize.value = n
  currentPage.value = 1
}

function clusterRepo(c: ResourceInfo): GitInfo | null {
  if (!c.repoName || !state.value?.repos) return null
  return state.value.repos.find(r => r.name === c.repoName) || null
}

function clusterCardAttrs(c: ResourceInfo): Record<string, string> {
  const attrs: Record<string, string> = {}
  const activePhases = ['scaling-up', 'scaling-down', 'destroying', 'reconfiguring']
  if (c.clusterPhase && activePhases.includes(c.clusterPhase)) {
    attrs['data-phase'] = c.clusterPhase
  }
  if (c.clusterReady === 'not-ready' || c.kubernetesApiReady === 'not-ready') {
    attrs['data-health'] = 'not-ready'
  }
  return attrs
}

function mgmtBadge(c: ResourceInfo): string {
  if (c.status === 'unmanaged') return 'Unmanaged'
  if (c.status === 'orphaned') return 'Orphaned'
  return 'Managed'
}
function mgmtColor(c: ResourceInfo): string {
  if (c.status === 'unmanaged') return '#5b5c64'
  if (c.status === 'orphaned') return '#a78bfa'
  return '#7d7d85'
}

function syncText(c: ResourceInfo): string {
  const repoSyncErrors: Record<string, boolean> = {}
  ;(state.value?.repos ?? []).forEach(r => { if (r.syncError) repoSyncErrors[r.name || ''] = true })
  if (c.repoName && repoSyncErrors[c.repoName] && c.status !== 'unmanaged') {
    return '<span class="spinner" style="width:10px;height:10px;display:inline-block;vertical-align:middle"></span> Syncing'
  }
  if (c.status === 'unmanaged' || c.status === 'orphaned') return '—'
  if (c.status === 'outofsync') return outOfSyncIconSVG + ' Out of Sync'
  if (c.status === 'failed') return failedIconSVG + ' Failed'
  if (c.status === 'syncing') return '● Syncing'
  if (c.status === 'success' || c.status === 'applied') return syncedIconSVG + ' Synced'
  return '—'
}
function syncColor(c: ResourceInfo): string {
  if (c.status === 'unmanaged' || c.status === 'orphaned') return '#5b5c64'
  if (c.status === 'outofsync') return '#fb923c'
  if (c.status === 'failed') return '#f87171'
  if (c.status === 'syncing') return '#2dd4bf'
  if (c.status === 'success' || c.status === 'applied') return '#4ade80'
  return '#5b5c64'
}
function healthText(c: ResourceInfo): string {
  const phase = c.clusterPhase || ''
  if (phase === 'scaling-up') return '↑ Scaling Up'
  if (phase === 'scaling-down') return '↓ Scaling Down'
  if (phase === 'destroying') return '✕ Destroying'
  if (phase === 'reconfiguring') return '↻ Reconfiguring'
  if (c.clusterReady === 'ready' && c.kubernetesApiReady === 'ready') return syncedIconSVG + ' Ready'
  if (c.clusterReady === 'not-ready' || c.kubernetesApiReady === 'not-ready') return failedIconSVG + ' Not Ready'
  return '—'
}
function healthColor(c: ResourceInfo): string {
  const phase = c.clusterPhase || ''
  if (phase === 'scaling-up') return '#60a5fa'
  if (phase === 'scaling-down') return '#f59e0b'
  if (phase === 'destroying') return '#f43f5e'
  if (phase === 'reconfiguring') return '#a78bfa'
  if (c.clusterReady === 'ready' && c.kubernetesApiReady === 'ready') return '#4ade80'
  if (c.clusterReady === 'not-ready' || c.kubernetesApiReady === 'not-ready') return '#f87171'
  return '#5b5c64'
}

function clusterSections(c: ResourceInfo) {
  const cp: NodeGroup = c.controlPlane ?? { count: 0 }
  const workers = Array.isArray(c.workers) ? c.workers : (c.workers ? [c.workers] : [])
  const sections: { label: string; count: number; mc: string }[] = [
    { label: 'Controlplane', count: cp.count || 0, mc: cp.machineClass || '' },
  ]
  workers.forEach(wk => {
    sections.push({ label: wk.name || 'Workers', count: wk.count || 0, mc: wk.machineClass || '' })
  })
  if (workers.length === 0) sections.push({ label: 'Workers', count: 0, mc: '' })
  return sections
}

function isZeroTime(d?: string): boolean {
  if (!d) return true
  const dt = new Date(d)
  if (isNaN(dt.getTime())) return true
  return dt.getFullYear() <= 1
}

function fmtDateTime(d?: string): string {
  if (!d) return '—'
  const dt = new Date(d)
  const pad = (n: number) => (n < 10 ? '0' + n : '' + n)
  return dt.getFullYear() + '-' + pad(dt.getMonth() + 1) + '-' + pad(dt.getDate()) +
    ' ' + pad(dt.getHours()) + ':' + pad(dt.getMinutes())
}

function ago(d?: string): string {
  if (!d) return ''
  const dt = new Date(d)
  if (isNaN(dt.getTime())) return ''
  const s = Math.floor((Date.now() - dt.getTime()) / 1000)
  if (s < 5) return 'just now'
  if (s < 60) return s + 's ago'
  if (s < 3600) return Math.floor(s / 60) + 'm ago'
  if (s < 86400) return Math.floor(s / 3600) + 'h ago'
  const days = Math.floor(s / 86400)
  if (days < 31) return days + 'd ago'
  if (days < 365) return Math.floor(days / 30) + 'mo ago'
  return Math.floor(days / 365) + 'y ago'
}

function omniClusterUrl(id: string): string {
  const ep = (state.value?.omniEndpoint || '').replace(/\/$/, '')
  return ep + '/clusters/' + id
}

function goToCluster(id: string) {
  router.push(`/clusters/${encodeURIComponent(id)}`)
}

async function triggerCheck() {
  gitRefreshing.value = true
  try { await fetch('/api/check', { method: 'POST' }) } finally { gitRefreshing.value = false }
}

async function triggerReconcile() {
  reconcileRunning.value = true
  try { await fetch('/api/reconcile', { method: 'POST' }) } finally { reconcileRunning.value = false }
}

async function refreshCluster(cluster: ResourceInfo) {
  if (clusterActionPending[cluster.id]) return
  clusterActionPending[cluster.id] = 'refresh'
  try {
    await fetch('/api/refresh-cluster', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ id: cluster.id }),
    })
    setTimeout(() => { delete clusterActionPending[cluster.id] }, 5000)
  } catch { delete clusterActionPending[cluster.id] }
}

async function syncCluster(cluster: ResourceInfo) {
  if (clusterActionPending[cluster.id]) return
  clusterActionPending[cluster.id] = 'sync'
  try {
    await fetch('/api/force-cluster', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ id: cluster.id }),
    })
    await fetch('/api/reconcile', { method: 'POST' })
    setTimeout(() => { delete clusterActionPending[cluster.id] }, 8000)
  } catch { delete clusterActionPending[cluster.id] }
}

async function toggleAutoSync(cluster: ResourceInfo) {
  const enabled = cluster.autoSync === false
  await fetch('/api/set-cluster-autosync', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ id: cluster.id, autoSync: enabled }),
  })
}

function deleteCluster(cluster: ResourceInfo) {
  confirmInput.value = ''
  confirmModal.value = {
    title: 'Delete Cluster',
    message: `Are you sure you want to delete the Cluster <b>${cluster.id}</b>?<br><br>Deleting the Cluster will delete all the cluster's managed resources, which can be dangerous.<br>Be sure you understand the effects of deleting this resource before continuing.`,
    requireInput: cluster.id,
    inputPrompt: `Please type '${cluster.id}' to confirm the deletion of the cluster`,
    onConfirm: async () => {
      confirmModal.value = null
      confirmInput.value = ''
      await fetch('/api/delete-cluster', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ id: cluster.id }),
      })
    },
  }
}

async function exportCluster(cluster: ResourceInfo) {
  const r = await fetch('/api/export-cluster', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ id: cluster.id }),
  })
  if (!r.ok) { alert('Failed to export cluster: ' + r.statusText); return }
  const yaml = await r.text()
  const blob = new Blob([yaml], { type: 'application/x-yaml' })
  const url = URL.createObjectURL(blob)
  const a = document.createElement('a')
  a.href = url; a.download = cluster.id + '.yaml'
  document.body.appendChild(a); a.click()
  document.body.removeChild(a); URL.revokeObjectURL(url)
}
</script>
