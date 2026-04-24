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

    <!-- Tab body -->
    <div
      class="cluster-detail-body"
      :class="{
        'graph-mode': activeTab === 'graph',
        'mc-live-mode': activeTab === 'template' && templateSubTab === 'live',
      }"
    >
      <template v-if="activeTab === 'graph'">
        <ClusterGraph :cluster="cluster" />
      </template>

      <template v-else-if="activeTab === 'template'">
        <!-- Sub-tab bar -->
        <div class="cluster-detail-tabs-bar">
          <button class="cluster-detail-tab" :class="{ active: templateSubTab === 'live' }" @click="templateSubTab = 'live'">Live</button>
          <button class="cluster-detail-tab" :class="{ active: templateSubTab === 'diff' }" @click="templateSubTab = 'diff'">Diff</button>
        </div>
        <!-- Live sub-tab -->
        <template v-if="templateSubTab === 'live'">
          <div v-if="cluster.liveContent" style="white-space:normal;word-break:normal;">
            <div v-if="liveFolds.size > 0" style="display:flex;align-items:center;justify-content:flex-end;gap:6px;padding:6px 14px;border-bottom:1px solid #2c2e38;background:#15161e;position:sticky;top:0;z-index:1;">
              <button @click="expandAllFolds" class="btn-sort" style="font-size:11px;padding:2px 8px;">Expand all</button>
              <button @click="collapseAllFolds" class="btn-sort" style="font-size:11px;padding:2px 8px;">Collapse all</button>
            </div>
            <div class="sbs-table-single">
              <template v-for="(line, i) in liveContentLines" :key="i">
                <div
                  v-if="!hiddenLines.has(i)"
                  class="sbs-cell"
                  style="display:flex;align-items:baseline;padding-left:4px;"
                  :style="liveFolds.has(i) ? { cursor: 'pointer' } : {}"
                  @click="liveFolds.has(i) && toggleFold(i)"
                >
                  <span style="width:14px;flex-shrink:0;font-size:9px;text-align:center;user-select:none;color:#ff8b59;">
                    <template v-if="liveFolds.has(i)">{{ collapsedFolds.has(i) ? '▶' : '▼' }}</template>
                  </span>
                  <span class="sbs-ln" :style="{ minWidth: lineNumberWidth, textAlign: 'right', display: 'inline-block' }">{{ i + 1 }}.</span>
                  <span style="white-space:pre;flex:1;">{{ line }}</span>
                  <span v-if="liveFolds.has(i) && collapsedFolds.has(i)" style="color:#5b5c64;font-size:11px;padding-left:10px;flex-shrink:0;">··· {{ liveFolds.get(i)!.lineCount }} lines</span>
                </div>
              </template>
            </div>
          </div>
          <div v-else style="color:#7d7d85;text-align:center;padding:40px;font-size:14px;">No live state available</div>
        </template>
        <!-- Diff sub-tab -->
        <template v-else>
          <div v-if="cluster.diff">
            <DiffViewer :diff="cluster.diff" />
          </div>
          <div v-else style="color:#7d7d85;text-align:center;padding:40px;font-size:14px;">
            {{ cluster.status === 'unmanaged' ? 'Cluster template exists in Omni but is not managed by OmniCD.' : 'No diff available' }}
          </div>
        </template>
      </template>

      <template v-else-if="activeTab === 'manifests'">
        <div style="white-space:normal;word-break:normal;font-family:Roboto,sans-serif;">
          <div v-if="manifestsLoading" style="text-align:center;padding:40px;color:#7d7d85;">
            <span class="spinner" style="width:16px;height:16px;"></span> Loading manifest status…
          </div>
          <div v-else-if="manifestsError" style="padding:16px 24px;">
            <div style="background:#3f1515;border:1px solid #7f1d1d;border-radius:8px;padding:12px 16px;font-size:13px;color:#f87171;">⚠ {{ manifestsError }}</div>
          </div>
          <template v-else-if="manifestStatus">
            <!-- Header -->
            <div style="display:flex;align-items:center;gap:20px;padding:12px 24px;border-bottom:1px solid #1f222e;">
              <span style="font-size:13px;color:#9fa1a6;">Total: <span style="color:#e8e8e9;font-weight:600;">{{ manifestStatus.total }}</span></span>
              <span style="font-size:13px;" :style="manifestStatus.outOfSync === 0 ? 'color:#4ade80' : 'color:#fb923c'">
                In Sync: <span style="font-weight:600;">{{ manifestStatus.total - manifestStatus.outOfSync }}</span>
              </span>
              <span v-if="manifestStatus.outOfSync > 0" style="font-size:13px;color:#fb923c;">Out of Sync: <span style="font-weight:600;">{{ manifestStatus.outOfSync }}</span></span>
              <span v-if="manifestStatus.lastError" style="font-size:12px;color:#f87171;flex:1;min-width:0;overflow:hidden;text-overflow:ellipsis;white-space:nowrap;" :title="manifestStatus.lastError">⚠ {{ manifestStatus.lastError }}</span>
            </div>
            <!-- No groups -->
            <div v-if="Object.keys(manifestStatus.groups).length === 0" style="text-align:center;color:#7d7d85;padding:32px;font-size:13px;">
              No manifest groups found for this cluster.
            </div>
            <!-- Groups -->
            <template v-for="(group, groupName) in manifestStatus.groups" :key="groupName">
              <!-- Group row -->
              <div
                @click="toggleManifestGroup(groupName as string)"
                style="display:flex;align-items:center;gap:10px;cursor:pointer;padding:12px 24px;border-bottom:1px solid #1f222e;user-select:none;"
              >
                <span style="font-size:10px;color:#5b5c64;flex-shrink:0;width:10px;">{{ expandedGroups.has(groupName as string) ? '▼' : '▶' }}</span>
                <span style="font-size:13px;font-weight:500;color:#e8e8e9;font-family:'Roboto Mono','SF Mono','Fira Code',monospace;flex-shrink:0;">{{ groupName }}</span>
                <span style="font-size:13px;color:#5b5c64;flex-shrink:0;">·</span>
                <span style="font-size:13px;font-weight:500;flex-shrink:0;" :style="{ color: manifestPhaseColorStr(group.phase) }">{{ manifestPhaseName(group.phase) }}</span>
                <span style="font-size:13px;color:#5b5c64;flex-shrink:0;">·</span>
                <span style="font-size:13px;color:#7d7d85;flex-shrink:0;">Mode: <span style="color:#c4c4c9;">{{ group.mode === 'one-time' ? 'One-Time' : 'Full' }}</span></span>
                <span style="font-size:13px;color:#5b5c64;flex-shrink:0;">·</span>
                <span style="font-size:13px;color:#7d7d85;flex-shrink:0;">{{ manifestSyncCount(group) }}/{{ Object.keys(group.manifests).length }} in sync</span>
              </div>
              <!-- Manifest table -->
              <table v-if="expandedGroups.has(groupName as string)" class="audit-table" style="table-layout:fixed;width:100%;">
                <colgroup>
                  <col style="width:20%" />
                  <col style="width:40%" />
                  <col style="width:20%" />
                  <col style="width:20%" />
                </colgroup>
                <thead>
                  <tr>
                    <th>Kind</th>
                    <th>Name</th>
                    <th>Namespace</th>
                    <th style="text-align:right;">Status</th>
                  </tr>
                </thead>
                <tbody>
                  <tr v-for="(m, mKey) in group.manifests" :key="mKey">
                    <td style="overflow:hidden;text-overflow:ellipsis;white-space:nowrap;font-family:'Roboto Mono','SF Mono','Fira Code',monospace;" :title="m.group ? m.kind + '.' + m.group : m.kind">
                      {{ m.kind }}<span v-if="m.group" class="audit-kind">.{{ m.group }}</span>
                    </td>
                    <td style="overflow:hidden;text-overflow:ellipsis;white-space:nowrap;font-family:'Roboto Mono','SF Mono','Fira Code',monospace;" :title="m.name">{{ m.name }}</td>
                    <td style="overflow:hidden;text-overflow:ellipsis;white-space:nowrap;font-family:'Roboto Mono','SF Mono','Fira Code',monospace;" :title="m.namespace || undefined">{{ m.namespace || '—' }}</td>
                    <td style="text-align:right;white-space:nowrap;font-weight:500;" :style="{ color: manifestPhaseColorStr(m.phase) }">{{ manifestPhaseName(m.phase) }}</td>
                  </tr>
                </tbody>
              </table>
            </template>
          </template>
          <div v-else style="text-align:center;padding:60px 24px;">
            <span style="font-size:14px;color:#7d7d85;">No Kubernetes manifests configured for this cluster</span>
          </div>
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
import { computed, ref, watch } from 'vue'
import { useRoute, useRouter, RouterLink } from 'vue-router'
import { useAppStore } from '@/stores/appStore'
import { useAuthStore } from '@/stores/authStore'
import ClusterGraph from '@/components/clusters/ClusterGraph.vue'
import DiffViewer from '@/components/clusters/DiffViewer.vue'
import { syncedIconSVG, outOfSyncIconSVG, failedIconSVG } from '@/assets/icons'
import type { ClusterManifestStatus, ManifestGroupStatus } from '@/types'

const route = useRoute()
const router = useRouter()
const appStore = useAppStore()
const authStore = useAuthStore()

const state = computed(() => appStore.state)

const cluster = computed(() => {
  const id = decodeURIComponent(route.params.id as string)
  return state.value?.clusters.find(c => c.id === id) ?? null
})

const activeTab = computed(() => (route.params.tab as string) || 'graph')
const templateSubTab = ref<'live' | 'diff'>('live')

// Manifests tab state
const manifestStatus = ref<ClusterManifestStatus | null>(null)
const manifestsLoading = ref(false)
const manifestsError = ref('')
const expandedGroups = ref<Set<string>>(new Set())

async function loadManifests() {
  const id = cluster.value?.id
  if (!id) return
  manifestsLoading.value = true
  manifestsError.value = ''
  try {
    const r = await fetch(`/api/cluster-manifests?id=${encodeURIComponent(id)}`)
    if (r.status === 204) {
      manifestStatus.value = null
      return
    }
    if (!r.ok) {
      const d = await r.json().catch(() => ({}))
      manifestsError.value = d.error || 'Failed to load manifest status'
      return
    }
    manifestStatus.value = await r.json()
    expandedGroups.value = new Set()
  } catch {
    manifestsError.value = 'Network error'
  } finally {
    manifestsLoading.value = false
  }
}

watch(() => cluster.value?.id, (id) => {
  if (id) loadManifests()
}, { immediate: true })

function toggleManifestGroup(name: string) {
  if (expandedGroups.value.has(name)) {
    expandedGroups.value.delete(name)
  } else {
    expandedGroups.value.add(name)
  }
}

function manifestPhaseName(phase: string): string {
  switch (phase) {
    case 'applied':     return 'Applied'
    case 'progressing': return 'Progressing'
    case 'pending':     return 'Pending'
    case 'deleting':    return 'Deleting'
    default:            return 'Unknown'
  }
}

function manifestPhaseColorStr(phase: string): string {
  switch (phase) {
    case 'applied':     return '#4ade80'
    case 'progressing': return '#fbbf24'
    case 'pending':     return '#9fa1a6'
    case 'deleting':    return '#f87171'
    default:            return '#7d7d85'
  }
}

function manifestSyncCount(group: ManifestGroupStatus): number {
  return Object.values(group.manifests).filter(m => m.phase === 'applied').length
}

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

const liveContentLines = computed(() => {
  const content = cluster.value?.liveContent || ''
  return content.replace(/\\n/g, '\n').split('\n')
})

interface LiveFoldInfo { bodyStart: number; bodyEnd: number; lineCount: number }

const liveFolds = computed((): Map<number, LiveFoldInfo> => {
  const lines = liveContentLines.value
  const folds = new Map<number, LiveFoldInfo>()
  for (let i = 0; i < lines.length; i++) {
    // Bare YAML key with no inline value at any indentation level
    const m = lines[i].match(/^(\s*)[a-zA-Z][\w.-]*:\s*$/)
    if (!m) continue
    const indent = m[1].length
    // Peek at the next non-empty line — must be more indented than this key
    let peek = i + 1
    while (peek < lines.length && lines[peek].trim() === '') peek++
    if (peek >= lines.length || lines[peek].trim() === '---') continue
    const peekIndent = (lines[peek].match(/^(\s*)/) ?? ['', ''])[1].length
    if (peekIndent <= indent) continue
    // Scan to end of body: first non-empty line at same or lesser indentation
    let end = i + 1
    while (end < lines.length) {
      const l = lines[end]
      if (l.trim() === '---') break
      if (l.trim() !== '' && (l.match(/^(\s*)/) ?? ['', ''])[1].length <= indent) break
      end++
    }
    // Trim trailing blank lines
    while (end > i + 1 && lines[end - 1].trim() === '') end--
    if (end > i + 1) folds.set(i, { bodyStart: i + 1, bodyEnd: end, lineCount: end - i - 1 })
  }
  return folds
})

const collapsedFolds = ref<Set<number>>(new Set())

const lineNumberWidth = computed(() => {
  const digits = String(liveContentLines.value.length).length
  return `${digits + 1}ch` // +1 for the trailing dot
})

const hiddenLines = computed((): Set<number> => {
  const hidden = new Set<number>()
  for (const [idx, fold] of liveFolds.value) {
    if (collapsedFolds.value.has(idx)) {
      for (let i = fold.bodyStart; i < fold.bodyEnd; i++) hidden.add(i)
    }
  }
  return hidden
})

watch(() => cluster.value?.id, () => { collapsedFolds.value = new Set() })

function toggleFold(i: number) {
  if (collapsedFolds.value.has(i)) collapsedFolds.value.delete(i)
  else collapsedFolds.value.add(i)
}
function expandAllFolds() { collapsedFolds.value = new Set() }
function collapseAllFolds() { collapsedFolds.value = new Set(liveFolds.value.keys()) }

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
    loadManifests()
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
