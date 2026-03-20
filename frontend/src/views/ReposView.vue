<template>
  <div class="container">
    <div class="header" style="border-bottom:none;">
      <h1 style="font-size:18px;font-weight:600;color:#fff;letter-spacing:-0.3px;">Repositories</h1>
      <button
        v-if="authStore.isAdmin()"
        class="btn-sort btn-primary"
        :disabled="!state?.omniConfigured && !state?.omniEnvLocked"
        :title="(!state?.omniConfigured && !state?.omniEnvLocked) ? 'Configure an Omni instance before adding a repository' : ''"
        @click="openRepoModal(null)"
        style="margin-left:auto"
      >Add Repository</button>
    </div>

    <!-- Repos -->
    <div v-if="repoConfigs.length === 0" class="placeholder-page">
      <div class="placeholder-icon">⎇</div>
      <div class="placeholder-title">Git Repositories</div>
      <div class="placeholder-sub">No repositories configured — add one using the button above</div>
    </div>

    <div v-else class="info-row">
      <div
        v-for="rc in repoConfigs"
        :key="rc.name"
        class="info-card"
      >
        <div class="info-card-header">
          <span class="info-card-title">{{ rc.name }}</span>
          <span :class="['badge', repoHealthBadgeClass(rc.name)]">{{ repoHealthLabel(rc.name) }}</span>
        </div>
        <div class="info-card-value">
          <a
            v-if="repoUrl(rc.name)"
            :href="repoUrl(rc.name)"
            target="_blank"
            style="color:#ff8b59;text-decoration:none"
          >{{ repoUrl(rc.name) }}</a>
          <span v-else>—</span>
        </div>
        <div class="info-card-sub">
          Branch: <b style="color:#9fa1a6">{{ repoBranch(rc.name) }}</b>
          <template v-if="repoShortSha(rc.name)">
            &nbsp;·&nbsp; SHA <b style="color:#9fa1a6">{{ repoShortSha(rc.name) }}</b>
          </template>
          <template v-if="rc.hasToken">
            &nbsp;·&nbsp; <span style="color:#4ade80;font-size:11px">🔑 token set</span>
          </template>
          <br />
          <template v-if="repoCommitMessage(rc.name)">{{ repoCommitMessage(rc.name) }}<br /></template>
          <template v-if="repoLastSync(rc.name)">Last sync: {{ ago(repoLastSync(rc.name)) }}</template>
          <span v-else style="color:#5b5c64">Never synced</span>
        </div>

        <!-- Test result -->
        <div
          v-if="testResults[rc.name]"
          style="font-size:12px;margin:4px 0 0;padding:6px 10px;border-radius:6px;"
          :style="{ background: testResults[rc.name].ok ? '#052e16' : '#3f1515', color: testResults[rc.name].ok ? '#4ade80' : '#f87171' }"
        >{{ testResults[rc.name].text }}</div>

        <div v-if="repoSyncError(rc.name)" style="color:#f87171;font-size:12px;margin-top:4px;">
          {{ repoSyncError(rc.name) }}
        </div>

        <div v-if="authStore.isAdmin()" class="info-card-actions">
          <button
            class="btn-sort btn-primary"
            :disabled="testingRepo === rc.name"
            @click="testRepoConnection(rc.name)"
          >{{ testingRepo === rc.name ? 'Testing...' : 'Test Connection' }}</button>
          <button class="btn-sort btn-primary" @click="openRepoModal(rc)">Edit</button>
          <button class="btn-sort btn-primary" @click="deleteRepo(rc.name)">Delete</button>
        </div>
      </div>
    </div>

    <!-- Delete confirm modal -->
    <div v-if="deleteConfirm" class="repo-modal-wrap show" @click.self="deleteConfirm = null">
      <div class="repo-modal-box">
        <div class="repo-modal-title">Remove Repository</div>
        <div style="font-size:13px;color:#e8e8e9;margin-bottom:14px;line-height:1.4;white-space:pre-line;" v-html="deleteConfirm.message"></div>
        <div style="font-size:13px;color:#9fa1a6;margin-bottom:8px;">{{ deleteConfirm.inputPrompt }}</div>
        <input class="repo-form-input" v-model="deleteConfirmInput" :placeholder="deleteConfirm.requireInput" style="margin-bottom:14px;" />
        <div class="repo-form-actions" style="margin-top:0">
          <button class="btn-sort btn-primary" @click="deleteConfirm = null">Cancel</button>
          <button class="btn-sort btn-primary" :disabled="deleteConfirmInput !== deleteConfirm.requireInput" @click="doDeleteRepo">OK</button>
        </div>
      </div>
    </div>

    <!-- Repo modal -->
    <div v-if="repoModalOpen" class="repo-modal-wrap show">
      <div class="repo-modal-box">
        <div class="repo-modal-title">{{ editingRepo ? 'Edit Repository' : 'Add Repository' }}</div>
        <div class="repo-form-group">
          <label class="repo-form-label">Name <span style="color:#f87171">*</span></label>
          <input v-model="repoForm.name" class="repo-form-input" :disabled="!!editingRepo" placeholder="my-repo" />
        </div>
        <div class="repo-form-group">
          <label class="repo-form-label">URL <span style="color:#f87171">*</span></label>
          <input v-model="repoForm.url" class="repo-form-input" placeholder="https://github.com/org/repo.git" />
        </div>
        <div class="repo-form-group">
          <label class="repo-form-label">Branch</label>
          <input v-model="repoForm.branch" class="repo-form-input" placeholder="main" />
        </div>
        <div class="repo-form-group">
          <label class="repo-form-label">Access Token</label>
          <div class="repo-token-row">
            <input type="checkbox" id="rm-set-token" v-model="repoForm.setToken" />
            <label for="rm-set-token" style="font-size:12px;color:#9fa1a6">
              {{ editingRepo ? (repoForm.setToken ? 'Change token' : (editingRepo.hasToken ? 'Token set — check to change' : 'Set access token')) : 'Set access token' }}
            </label>
          </div>
          <input
            v-if="repoForm.setToken"
            v-model="repoForm.token"
            class="repo-form-input"
            type="password"
            placeholder="Personal access token"
          />
        </div>
        <div class="repo-form-group">
          <label class="repo-form-label">Clusters Path</label>
          <input v-model="repoForm.clustersPath" class="repo-form-input" placeholder="clusters" />
        </div>
        <div class="repo-form-group">
          <label class="repo-form-label">Machine Classes Path</label>
          <input v-model="repoForm.mcPath" class="repo-form-input" placeholder="machineclasses" />
        </div>
        <div v-if="repoFormError" class="repo-form-error">{{ repoFormError }}</div>
        <div
          v-if="repoTestResult"
          style="font-size:12px;margin-bottom:8px;padding:6px 10px;border-radius:6px;"
          :style="{ background: repoTestResult.ok ? '#052e16' : '#3f1515', color: repoTestResult.ok ? '#4ade80' : '#f87171' }"
        >{{ repoTestResult.text }}</div>
        <div class="repo-form-actions">
          <button class="btn-sort btn-primary" @click="closeRepoModal">Cancel</button>
          <button class="btn-sort btn-primary" :disabled="modalTesting" @click="testModalConnection">
            {{ modalTesting ? 'Testing...' : 'Test Connection' }}
          </button>
          <button class="btn-sort btn-primary" @click="saveRepo">Save</button>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, ref, reactive } from 'vue'
import { useAppStore } from '@/stores/appStore'
import { useAuthStore } from '@/stores/authStore'
import type { RepoConfigView, GitInfo } from '@/types'

const appStore = useAppStore()
const authStore = useAuthStore()

const state = computed(() => appStore.state)
const repoConfigs = computed(() => (state.value?.repoConfigs ?? []).slice().sort((a: RepoConfigView, b: RepoConfigView) => (a.name || '').localeCompare(b.name || '')))

function getGitInfo(name: string): GitInfo | null {
  const repos = state.value?.repos
  if (repos) return repos.find((r: GitInfo) => r.name === name) || null
  return null
}

function repoUrl(name: string): string { return getGitInfo(name)?.repo || '' }
function repoBranch(name: string): string { return getGitInfo(name)?.branch || 'main' }
function repoShortSha(name: string): string { return getGitInfo(name)?.shortSha || '' }
function repoCommitMessage(name: string): string { return getGitInfo(name)?.commitMessage || '' }
function repoLastSync(name: string): string { return getGitInfo(name)?.lastSync || '' }
function repoSyncError(name: string): string { return getGitInfo(name)?.syncError || '' }

function repoHealthLabel(name: string): string {
  const info = getGitInfo(name)
  if (!info) return 'Not synced'
  if (info.syncError) return 'Failed'
  if (!info.sha || !info.lastSync) return 'Disconnected'
  const minsAgo = Math.floor((Date.now() - new Date(info.lastSync).getTime()) / 60000)
  if (minsAgo > 10) return 'Stale'
  return 'Healthy'
}

function repoHealthBadgeClass(name: string): string {
  const label = repoHealthLabel(name)
  if (label === 'Healthy') return 'badge-success'
  if (label === 'Failed') return 'badge-connecting'
  if (label === 'Stale' || label === 'Degraded') return 'badge-outofsync'
  if (label === 'Disconnected') return 'badge-failed'
  return 'badge-idle'
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

// Repo modal
const repoModalOpen = ref(false)
const editingRepo = ref<RepoConfigView | null>(null)
const repoFormError = ref('')
const repoTestResult = ref<{ ok: boolean; text: string } | null>(null)
const modalTesting = ref(false)
const testingRepo = ref<string | null>(null)
const testResults = reactive<Record<string, { ok: boolean; text: string }>>({})

const repoForm = reactive({
  name: '',
  url: '',
  branch: 'main',
  clustersPath: '',
  mcPath: '',
  token: '',
  setToken: false,
})

function openRepoModal(rc: RepoConfigView | null) {
  editingRepo.value = rc
  repoFormError.value = ''
  repoTestResult.value = null
  if (rc) {
    repoForm.name = rc.name
    repoForm.url = rc.url
    repoForm.branch = rc.branch
    repoForm.clustersPath = rc.clustersPath
    repoForm.mcPath = rc.mcPath
    repoForm.token = ''
    repoForm.setToken = false
  } else {
    repoForm.name = ''
    repoForm.url = ''
    repoForm.branch = 'main'
    repoForm.clustersPath = ''
    repoForm.mcPath = ''
    repoForm.token = ''
    repoForm.setToken = false
  }
  repoModalOpen.value = true
}

function closeRepoModal() {
  repoModalOpen.value = false
  editingRepo.value = null
  repoFormError.value = ''
  repoTestResult.value = null
}

async function testModalConnection() {
  modalTesting.value = true
  repoTestResult.value = null
  try {
    const r = await fetch('/api/repos/test', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ url: repoForm.url, branch: repoForm.branch, token: repoForm.setToken ? repoForm.token : undefined }),
    })
    const text = r.ok ? 'Connection successful' : (await r.text() || 'Test failed')
    repoTestResult.value = { ok: r.ok, text }
  } catch (e: unknown) {
    repoTestResult.value = { ok: false, text: 'Network error: ' + (e instanceof Error ? e.message : String(e)) }
  } finally {
    modalTesting.value = false
  }
}

async function testRepoConnection(name: string) {
  const rc = state.value?.repoConfigs?.find(r => r.name === name)
  if (!rc) return
  testingRepo.value = name
  delete testResults[name]
  try {
    const res = await fetch('/api/repos/test', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ url: rc.url, branch: rc.branch }),
    })
    const d = await res.json().catch(() => ({}))
    const text = res.ok ? 'Connection successful' : (d.error || 'Test failed')
    testResults[name] = { ok: res.ok, text }
    setTimeout(() => { delete testResults[name] }, 5000)
  } catch (e: unknown) {
    testResults[name] = { ok: false, text: 'Network error: ' + (e instanceof Error ? e.message : String(e)) }
    setTimeout(() => { delete testResults[name] }, 5000)
  } finally {
    testingRepo.value = null
  }
}

async function saveRepo() {
  repoFormError.value = ''
  if (!repoForm.name.trim()) { repoFormError.value = 'Name is required'; return }
  if (!repoForm.url.trim()) { repoFormError.value = 'URL is required'; return }
  const method = editingRepo.value ? 'PUT' : 'POST'
  const body: Record<string, string> = {
    name: repoForm.name,
    url: repoForm.url,
    branch: repoForm.branch,
    clustersPath: repoForm.clustersPath,
    mcPath: repoForm.mcPath,
  }
  if (repoForm.setToken && repoForm.token) body.token = repoForm.token
  const res = await fetch('/api/repos', {
    method,
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(body),
  })
  if (!res.ok) {
    repoFormError.value = (await res.text()) || 'Failed to save repo'
    return
  }
  closeRepoModal()
}

// Delete confirm modal
interface DeleteConfirm { message: string; inputPrompt: string; requireInput: string; name: string }
const deleteConfirm = ref<DeleteConfirm | null>(null)
const deleteConfirmInput = ref('')

function deleteRepo(name: string) {
  const clusterIDs: string[] = (state.value?.repoClusterMap?.[name]) ?? []
  const mcIDs: string[] = (state.value?.repoMachineClassMap?.[name]) ?? []
  const clusterSection = clusterIDs.length ? '\n• Clusters\n' + clusterIDs.map(id => '  • ' + id).join('\n') + '\n' : ''
  const mcSection = mcIDs.length ? '\n• Machine Classes\n' + mcIDs.map(id => '  • ' + id).join('\n') : ''
  deleteConfirm.value = {
    name,
    message: `Are you sure you want to remove <b>${name}</b>?\n\nDeleting this repository will remove associated Clusters &amp; MachineClasses from Omni, where Auto-Sync is enabled. The remaining resources will remain in Omni, but are marked as "Unmanaged" or "Orphaned" in OmniCD.` +
      (clusterSection || mcSection ? '\n' + clusterSection + mcSection : '') +
      '\n\n<i>Note: MachineClasses with Auto-Sync enabled that are associated with any Cluster will not be deleted.</i>',
    inputPrompt: `Please type '${name}' to confirm deletion of the repository`,
    requireInput: name,
  }
  deleteConfirmInput.value = ''
}

async function doDeleteRepo() {
  const name = deleteConfirm.value?.name
  deleteConfirm.value = null
  if (!name) return
  await fetch('/api/repos', {
    method: 'DELETE',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ name }),
  })
}
</script>
