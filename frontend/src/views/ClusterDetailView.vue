<template>
  <div v-if="!cluster" class="container">
    <div class="placeholder-page">
      <div class="placeholder-title">Cluster not found</div>
      <div class="placeholder-sub">
        <RouterLink to="/clusters" class="breadcrumb-link">← Back to clusters</RouterLink>
      </div>
    </div>
  </div>
  <div v-else class="cluster-detail-page">
    <!-- Header -->
    <div class="cluster-detail-header-wrap">
      <div class="header" style="margin-bottom:8px;border-bottom:none">
        <h1>
          <nav class="breadcrumb">
            <RouterLink to="/clusters" class="breadcrumb-link">Clusters</RouterLink>
            <span class="breadcrumb-sep">/</span>
            <span class="breadcrumb-current">{{ cluster.id }}</span>
          </nav>
        </h1>
      </div>
      <div class="header-buttons" style="margin-bottom:8px">
        <a
          v-if="state?.omniEndpoint"
          class="btn-omni"
          style="text-decoration:none"
          :href="omniClusterUrl"
          target="_blank"
        >&#8599; Open in Omni</a>
        <template v-if="authStore.isAdmin()">
          <button
            class="btn-omni"
            :disabled="!!actionPending"
            @click="refreshCluster"
          >↺ {{ actionPending === 'refresh' ? 'Refreshing...' : 'Refresh' }}</button>
          <button
            v-if="cluster.status !== 'deleting' && (cluster.status === 'unmanaged' || cluster.status === 'orphaned')"
            class="btn-omni"
            @click="exportCluster"
          >↓ Export</button>
          <button
            v-if="cluster.status !== 'deleting' && cluster.status !== 'unmanaged' && cluster.status !== 'orphaned'"
            class="btn-omni"
            :disabled="!!actionPending"
            @click="syncCluster"
          >⇅ {{ actionPending === 'sync' ? 'Syncing...' : 'Sync' }}</button>
          <button
            v-if="cluster.status !== 'deleting' && cluster.status !== 'unmanaged' && cluster.status !== 'orphaned'"
            class="btn-omni auto-sync"
            :class="{ active: cluster.autoSync !== false }"
            @click="toggleAutoSync"
          >{{ cluster.autoSync === false ? '○ Auto-Sync: Off' : '● Auto-Sync: On' }}</button>
          <button
            v-if="cluster.status !== 'deleting' && cluster.status !== 'unmanaged'"
            class="btn-omni"
            @click="deleteCluster"
          >✕ Delete</button>
        </template>
      </div>
    </div>

    <!-- Detail strip -->
    <div class="cluster-detail-strip">
      <template v-if="activePhase">
        <div class="detail-strip-item">
          <div class="detail-strip-label">Phase</div>
          <div class="detail-strip-value" v-html="phaseBadge(cluster.clusterPhase)"></div>
        </div>
        <div class="detail-strip-sep"></div>
      </template>
      <div class="detail-strip-item">
        <div class="detail-strip-label">Cluster</div>
        <div class="detail-strip-value" v-html="cpBadge(cluster.clusterReady)"></div>
      </div>
      <div class="detail-strip-sep"></div>
      <div class="detail-strip-item">
        <div class="detail-strip-label">Controlplane</div>
        <div class="detail-strip-value" v-html="cpBadge(cluster.controlplaneReady)"></div>
      </div>
      <div class="detail-strip-sep"></div>
      <div class="detail-strip-item">
        <div class="detail-strip-label">K8S API</div>
        <div class="detail-strip-value" v-html="cpBadge(cluster.kubernetesApiReady)"></div>
      </div>
      <div class="detail-strip-sep"></div>
      <div class="detail-strip-item">
        <div class="detail-strip-label">ETCD</div>
        <div class="detail-strip-value" v-html="statusBadge(cluster.etcdStatus)"></div>
      </div>
      <div class="detail-strip-sep"></div>
      <div class="detail-strip-item">
        <div class="detail-strip-label">Wireguard</div>
        <div class="detail-strip-value" v-html="statusBadge(cluster.wireGuardStatus)"></div>
      </div>
      <div class="detail-strip-sep"></div>
      <div class="detail-strip-item">
        <div class="detail-strip-label">Machines</div>
        <div class="detail-strip-value" v-html="machinesBadge"></div>
      </div>
      <div class="detail-strip-sep"></div>
      <div class="detail-strip-item" style="min-width:160px" :title="syncStatusTooltip">
        <div class="detail-strip-label">Sync Status</div>
        <div class="detail-strip-value" v-html="syncStatusBadge"></div>
        <div v-if="syncSinceStr" style="font-size:11px;color:#7d7d85;margin-top:2px;white-space:nowrap;overflow:hidden;text-overflow:ellipsis;width:100%">Since: {{ syncSinceStr }}</div>
        <div v-if="repoAuthor && cluster.status !== 'unmanaged'" style="font-size:11px;color:#7d7d85;margin-top:2px;white-space:nowrap;overflow:hidden;text-overflow:ellipsis;width:100%">Author: {{ repoAuthor }}</div>
        <div v-if="repoMessage && cluster.status !== 'unmanaged'" style="font-size:11px;color:#7d7d85;margin-top:2px;white-space:nowrap;overflow:hidden;text-overflow:ellipsis;width:100%">Message: {{ repoMessage }}</div>
      </div>
      <div class="detail-strip-sep"></div>
      <div class="detail-strip-item" style="min-width:140px" :title="lastSyncResultTooltip">
        <div class="detail-strip-label">Last Sync Result</div>
        <div class="detail-strip-value" v-html="lastSyncResultBadge"></div>
        <div v-if="lastSyncDateStr && cluster.status !== 'unmanaged'" style="font-size:11px;color:#7d7d85;margin-top:2px;white-space:nowrap">{{ lastSyncDateStr }}</div>
        <div v-if="lastSyncAuthor && cluster.status !== 'unmanaged'" style="font-size:11px;color:#7d7d85;margin-top:2px;white-space:nowrap">Author: {{ lastSyncAuthor }}</div>
        <div v-if="lastSyncMessage && cluster.status !== 'unmanaged'" style="font-size:11px;color:#7d7d85;margin-top:2px;white-space:nowrap;overflow:hidden;text-overflow:ellipsis;width:100%">Message: {{ lastSyncMessage }}</div>
      </div>
      <template v-if="cluster.status === 'unmanaged' && createdAgo">
        <div class="detail-strip-sep"></div>
        <div class="detail-strip-item">
          <div class="detail-strip-label">Created At</div>
          <div class="detail-strip-value" style="font-size:12px;color:#e8e8e9;">{{ new Date(cluster.createdAt!).toLocaleString() }}</div>
          <div style="font-size:11px;color:#7d7d85;margin-top:2px;">{{ createdAgo }}</div>
        </div>
      </template>
    </div>

    <!-- Tab bar -->
    <div class="cluster-detail-tabs-bar">
      <button
        v-for="tab in tabs"
        :key="tab"
        class="cluster-detail-tab"
        :class="{ active: activeTab === tab }"
        @click="activeTab = tab"
      >
        {{ tabLabels[tab] || tab }}
      </button>
    </div>

    <!-- Tab body -->
    <div
      class="cluster-detail-body"
      :class="{
        'graph-mode': activeTab === 'graph',
        'mc-live-mode': activeTab === 'live',
      }"
    >
      <template v-if="activeTab === 'graph'">
        <ClusterGraph :cluster="cluster" />
      </template>

      <template v-else-if="activeTab === 'live'">
        <div v-if="cluster.liveContent" style="padding:24px;">
          <div class="sbs-table-single">
            <div
              v-for="(line, i) in liveLines"
              :key="i"
              class="sbs-cell"
            ><span class="sbs-ln">{{ i + 1 }}.</span>{{ line }}</div>
          </div>
        </div>
        <div v-else style="color:#7d7d85;text-align:center;padding:40px;">No live state available</div>
      </template>

      <template v-else-if="activeTab === 'diff'">
        <div v-if="cluster.diff">
          <DiffViewer :diff="cluster.diff" />
        </div>
        <div v-else style="color:#7d7d85;text-align:center;padding:40px;">
          {{ cluster.status === 'unmanaged' ? 'No diff — this cluster has no template in Git.' : 'No diff available' }}
        </div>
      </template>

      <template v-else-if="activeTab === 'error'">
        <div style="padding:24px;color:#f87171;white-space:pre-wrap;">{{ cluster.error || cluster.lastSyncError }}</div>
      </template>
    </div>

    <!-- Sync error modal -->
    <div v-if="showSyncError" class="modal show" @click.self="showSyncError = false">
      <div class="modal-content confirm-modal" @click.stop>
        <div class="modal-header">
          <div class="modal-title">Sync Error</div>
          <button class="modal-close" @click="showSyncError = false">&times;</button>
        </div>
        <div class="modal-body confirm-body">
          <pre style="margin:0;white-space:pre-wrap;color:#f87171;font-size:12px;font-family:'SF Mono','Fira Code',monospace">{{ cluster.lastSyncError }}</pre>
        </div>
      </div>
    </div>

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
import { computed, ref } from 'vue'
import { useRoute, useRouter, RouterLink } from 'vue-router'
import { useAppStore } from '@/stores/appStore'
import { useAuthStore } from '@/stores/authStore'
import ClusterGraph from '@/components/clusters/ClusterGraph.vue'
import DiffViewer from '@/components/clusters/DiffViewer.vue'
import { syncedIconSVG, outOfSyncIconSVG, failedIconSVG } from '@/assets/icons'

const route = useRoute()
const router = useRouter()
const appStore = useAppStore()
const authStore = useAuthStore()

const state = computed(() => appStore.state)

const cluster = computed(() => {
  const id = decodeURIComponent(route.params.id as string)
  return state.value?.clusters.find(c => c.id === id) ?? null
})

const tabs = computed(() => {
  const t = ['graph', 'live', 'diff']
  if (cluster.value?.error || cluster.value?.lastSyncError) t.push('error')
  return t
})

const tabLabels: Record<string, string> = {
  graph: 'Topology',
  live: 'Live',
  diff: 'Diff',
  error: 'Error',
}

const activeTab = ref('graph')
const actionPending = ref<string | null>(null)
const showSyncError = ref(false)

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

const activePhase = computed(() => {
  const phase = cluster.value?.clusterPhase || ''
  const activePhases = ['scaling-up', 'scaling-down', 'destroying', 'reconfiguring']
  return activePhases.includes(phase) ? phase : ''
})

const clusterRepo = computed(() => {
  const c = cluster.value
  if (!c) return null
  if (c.repoName && state.value?.repos) {
    return state.value.repos.find(r => r.name === c.repoName) || state.value.git || null
  }
  return state.value?.git || null
})

const machinesBadge = computed(() => {
  const c = cluster.value
  if (!c) return '<span style="color:#5b5c64">—</span>'
  const healthy = c.machinesHealthy || 0
  const total = c.machinesTotal || 0
  if (total === 0) return '<span style="color:#5b5c64">—</span>'
  if (healthy === total) return `<span style="color:#4ade80">${healthy} / ${total}</span>`
  return `<span style="color:#fb923c">${healthy} / ${total}</span>`
})

const repoAuthor = computed(() => clusterRepo.value?.commitAuthor || '')
const repoMessage = computed(() => clusterRepo.value?.commitMessage || '')

const syncStatusBadge = computed(() => {
  const c = cluster.value
  if (!c) return '<span style="color:#5b5c64">—</span>'
  const repo = clusterRepo.value
  const repoDisconnected = !!(repo && repo.syncError)
  if (c.status === 'unmanaged') return '<span style="color:#5b5c64">—</span>'
  if (repoDisconnected) return '<span class="spinner" style="width:10px;height:10px;display:inline-block;vertical-align:middle"></span>'
  const branchSha = repo
    ? ((repo.branch || '') + (repo.shortSha ? ' (' + repo.shortSha + ')' : ''))
    : ''
  const repoUrl = repo?.repo || ''
  const commitUrl = repoUrl && repo?.sha ? repoUrl.replace(/\/+$/, '') + '/commit/' + repo.sha : ''
  const branchShaHtml = branchSha
    ? (commitUrl
        ? `<a href="${escHtml(commitUrl)}" target="_blank" rel="noopener" style="color:#ff8b59;text-decoration:none" onclick="event.stopPropagation()">${escHtml(branchSha)}</a>`
        : `<span style="color:#ff8b59">${escHtml(branchSha)}</span>`)
    : ''
  const hasSyncError = !!(c.error || c.lastSyncError)
  const badge = (c.status === 'outofsync' && hasSyncError)
    ? `<span style="color:#f87171">${failedIconSVG} Sync Failed</span>`
    : syncBadge(c.status || '')
  return branchShaHtml ? badge + ' from ' + branchShaHtml : badge
})

const syncSinceStr = computed(() => {
  const c = cluster.value
  if (!c) return ''
  if (c.status === 'outofsync' && c.syncStatusSince) return relTime(c.syncStatusSince)
  if ((c.status === 'success' || c.status === 'applied') && c.lastSyncTime) return relTime(c.lastSyncTime)
  return ''
})

const syncStatusTooltip = computed(() => {
  const parts: string[] = []
  if (syncSinceStr.value) parts.push('Since: ' + syncSinceStr.value)
  if (repoAuthor.value) parts.push('Author: ' + repoAuthor.value)
  if (repoMessage.value) parts.push('Message: ' + repoMessage.value)
  return parts.join('\n')
})

const lastSyncResultBadge = computed(() => {
  const c = cluster.value
  if (!c) return '<span style="color:#5b5c64">—</span>'
  if (c.status === 'unmanaged') return '<span style="color:#5b5c64">—</span>'
  if (!c.lastSyncResult) return '<span style="color:#5b5c64">—</span>'
  const repo = clusterRepo.value
  const repoUrl = repo?.repo ? repo.repo.replace(/\/+$/, '') : ''
  const shortSHA = c.lastSyncSHA ? c.lastSyncSHA.slice(0, 8) : ''
  const shaHtml = shortSHA
    ? ' to ' + (repoUrl
        ? `<a href="${escHtml(repoUrl + '/commit/' + c.lastSyncSHA)}" target="_blank" rel="noopener" style="color:#ff8b59;text-decoration:none" onclick="event.stopPropagation()">${escHtml(shortSHA)}</a>`
        : `<span style="color:#ff8b59">${escHtml(shortSHA)}</span>`)
    : ''
  if (c.lastSyncResult === 'ok') {
    return `<span style="color:#4ade80">${syncedIconSVG} Sync OK</span>${shaHtml}`
  }
  return `<span style="color:#f87171">${failedIconSVG} Sync Failed</span>`
})

const lastSyncDateStr = computed(() => {
  const c = cluster.value
  if (!c?.lastSyncTime) return ''
  const dt = new Date(c.lastSyncTime)
  if (isNaN(dt.getTime()) || dt.getFullYear() <= 1) return ''
  const verb = c.lastSyncResult === 'ok' ? 'Succeeded ' : 'Failed '
  return verb + relTime(c.lastSyncTime)
})
const lastSyncAuthor = computed(() => cluster.value?.lastSyncAuthor || '')
const createdAgo = computed(() => {
  const d = cluster.value?.createdAt
  if (!d) return ''
  const dt = new Date(d)
  if (isNaN(dt.getTime()) || dt.getFullYear() <= 1) return ''
  return relTime(d)
})
const lastSyncMessage = computed(() => cluster.value?.lastSyncMessage || '')
const lastSyncResultTooltip = computed(() => {
  const parts: string[] = []
  if (lastSyncDateStr.value) parts.push(lastSyncDateStr.value)
  if (lastSyncAuthor.value) parts.push('Author: ' + lastSyncAuthor.value)
  if (lastSyncMessage.value) parts.push('Message: ' + lastSyncMessage.value)
  return parts.join('\n')
})

const liveLines = computed(() => {
  const content = cluster.value?.liveContent || ''
  return content.replace(/\\n/g, '\n').split('\n')
})

const omniClusterUrl = computed(() => {
  const ep = (state.value?.omniEndpoint || '').replace(/\/$/, '')
  return ep + '/clusters/' + (cluster.value?.id || '')
})

function cpBadge(val?: string): string {
  if (val === 'ready') return `<span style="color:#4ade80">${syncedIconSVG} Ready</span>`
  if (val === 'not-ready') return `<span style="color:#f87171">${failedIconSVG} Not Ready</span>`
  return '<span style="color:#5b5c64">—</span>'
}
function statusBadge(val?: string): string {
  if (val === 'ok' || val === 'ready') return `<span style="color:#4ade80">${syncedIconSVG} OK</span>`
  if (val === 'not-ready') return `<span style="color:#f87171">${failedIconSVG} Not Ready</span>`
  return '<span style="color:#5b5c64">—</span>'
}
function syncBadge(status: string): string {
  if (status === 'success' || status === 'applied') return `<span style="color:#4ade80">${syncedIconSVG} Synced</span>`
  if (status === 'outofsync') return `<span style="color:#fb923c">${outOfSyncIconSVG} Out of Sync</span>`
  if (status === 'orphaned') return '<span style="color:#a78bfa">● Orphaned</span>'
  if (status === 'failed') return `<span style="color:#f87171">${failedIconSVG} Failed</span>`
  if (status === 'syncing') return '<span style="color:#2dd4bf">● Syncing</span>'
  if (status === 'unmanaged') return '<span style="color:#5b5c64">Unmanaged</span>'
  return '<span style="color:#5b5c64">—</span>'
}
function phaseBadge(phase?: string): string {
  if (phase === 'scaling-up') return '<span style="color:#60a5fa">↑ Scaling Up</span>'
  if (phase === 'scaling-down') return '<span style="color:#f59e0b">↓ Scaling Down</span>'
  if (phase === 'destroying') return `<span style="color:#f43f5e">${failedIconSVG} Destroying</span>`
  if (phase === 'reconfiguring') return '<span style="color:#a78bfa">↻ Reconfiguring</span>'
  if (phase === 'running') return `<span style="color:#4ade80">${syncedIconSVG} Running</span>`
  return '<span style="color:#5b5c64">—</span>'
}

function escHtml(s: string): string {
  return s.replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;')
}

function relTime(d?: string): string {
  if (!d) return ''
  const dt = new Date(d)
  if (isNaN(dt.getTime()) || dt.getFullYear() <= 1) return ''
  const s = Math.floor((Date.now() - dt.getTime()) / 1000)
  let rel: string
  if (s < 10) rel = 'a few seconds ago'
  else if (s < 60) rel = s + ' seconds ago'
  else if (s < 120) rel = '1 minute ago'
  else if (s < 3600) rel = Math.floor(s / 60) + ' minutes ago'
  else if (s < 7200) rel = '1 hour ago'
  else if (s < 86400) rel = Math.floor(s / 3600) + ' hours ago'
  else if (s < 172800) rel = '1 day ago'
  else {
    const days = Math.floor(s / 86400)
    rel = days < 31 ? days + ' days ago' : (days < 365 ? Math.floor(days / 30) + ' months ago' : Math.floor(days / 365) + ' years ago')
  }
  return rel + ' (' + dt.toString().replace(/\s*\(.*\)$/, '') + ')'
}

async function refreshCluster() {
  if (actionPending.value) return
  actionPending.value = 'refresh'
  try {
    await fetch('/api/refresh-cluster', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ id: cluster.value?.id }),
    })
    setTimeout(() => { actionPending.value = null }, 5000)
  } catch { actionPending.value = null }
}

async function syncCluster() {
  if (actionPending.value) return
  actionPending.value = 'sync'
  try {
    await fetch('/api/force-cluster', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ id: cluster.value?.id }),
    })
    setTimeout(() => { actionPending.value = null }, 8000)
  } catch { actionPending.value = null }
}

async function toggleAutoSync() {
  const c = cluster.value
  if (!c) return
  const enabled = c.autoSync === false
  await fetch('/api/set-cluster-autosync', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ id: c.id, autoSync: enabled }),
  })
}

function deleteCluster() {
  const c = cluster.value
  if (!c) return
  confirmInput.value = ''
  confirmModal.value = {
    title: 'Delete Cluster',
    message: `Are you sure you want to delete the Cluster <b>${c.id}</b>?<br><br>Deleting the Cluster will delete all the cluster's managed resources, which can be dangerous.`,
    requireInput: c.id,
    inputPrompt: `Please type '${c.id}' to confirm the deletion of the cluster`,
    onConfirm: async () => {
      confirmModal.value = null
      confirmInput.value = ''
      await fetch('/api/delete-cluster', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ id: c.id }),
      })
      router.push('/clusters')
    },
  }
}

async function exportCluster() {
  const c = cluster.value
  if (!c) return
  const r = await fetch('/api/export-cluster', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ id: c.id }),
  })
  if (!r.ok) { alert('Failed to export cluster: ' + r.statusText); return }
  const yaml = await r.text()
  const blob = new Blob([yaml], { type: 'application/x-yaml' })
  const url = URL.createObjectURL(blob)
  const a = document.createElement('a')
  a.href = url; a.download = c.id + '.yaml'
  document.body.appendChild(a); a.click()
  document.body.removeChild(a); URL.revokeObjectURL(url)
}
</script>
