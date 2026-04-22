<template>
  <div class="container">
    <div class="header" style="border-bottom:none;">
      <h1 style="font-size:18px;font-weight:600;color:#fff;letter-spacing:-0.3px;">Instances</h1>
      <button
        v-if="authStore.isAdmin()"
        class="btn-omni"
        :disabled="isConfigured"
        style="margin-left:auto"
        @click="openModal()"
      >Add Omni Instance</button>
    </div>

    <!-- Configured -->
    <template v-if="isConfigured">
      <div class="info-row">
        <div class="info-card">
          <div class="info-card-header">
            <span class="info-card-title">Omni Instance</span>
            <span :class="healthBadgeClass">{{ healthLabel }}</span>
          </div>
          <div class="info-card-value">
            <a v-if="state?.omniEndpoint" :href="state.omniEndpoint" target="_blank" style="color:#ff8b59;text-decoration:none">{{ state.omniEndpoint }}</a>
            <span v-else style="color:#7d7d85">Not configured</span>
          </div>
          <div class="info-card-sub">
            <span v-if="state?.omniConfigured">Version: <b style="color:#9fa1a6">{{ state.omniVersion || '?' }}</b></span>
          </div>
          <div v-if="testResult" :style="testResultStyle" style="font-size:12px;margin:4px 0 0;padding:6px 10px;border-radius:6px;">{{ testResult }}</div>
          <div v-if="authStore.isAdmin()" class="info-card-actions">
            <button class="btn-omni" :disabled="testPending" @click="testConnection">
              {{ testPending ? 'Testing…' : 'Test Connection' }}
            </button>
            <button class="btn-omni" :disabled="!!state?.omniEnvLocked" :title="state?.omniEnvLocked ? 'Configured via environment variables' : ''" @click="openModal()">Edit</button>
            <button class="btn-omni" :disabled="!!state?.omniEnvLocked" :title="state?.omniEnvLocked ? 'Configured via environment variables' : ''" @click="deleteInstance">Delete</button>
          </div>
        </div>
      </div>
    </template>

    <!-- Not configured -->
    <div v-else class="placeholder-page">
      <div class="placeholder-icon">⚡</div>
      <div class="placeholder-title">Omni Instance</div>
      <div class="placeholder-sub">No Omni instance configured — add one using the button above</div>
    </div>

    <!-- Add / Edit modal -->
    <div v-if="showModal" class="repo-modal-wrap show" @click.self="closeModal">
      <div class="repo-modal-box">
        <div class="repo-modal-title">{{ isEdit ? 'Edit Omni Instance' : 'Add Omni Instance' }}</div>
        <div class="repo-form-group">
          <label class="repo-form-label">Endpoint URL <span v-if="!isEdit" style="color:#f87171">*</span></label>
          <input class="repo-form-input" v-model="form.endpoint" type="text" placeholder="https://your-omni-instance.example.com" :disabled="isEdit" :style="isEdit ? { opacity: '0.5', cursor: 'not-allowed' } : {}" />
          <div v-if="isEdit" style="font-size:11px;color:#7d7d85;margin-top:4px;">Endpoint cannot be changed — delete the instance to use a different URL</div>
        </div>
        <div class="repo-form-group">
          <label class="repo-form-label">Service Account Key</label>
          <input class="repo-form-input" v-model="form.key" type="password" placeholder="Paste service account key" />
          <div v-if="state?.omniHasStoredKey" style="font-size:11px;color:#7d7d85;margin-top:4px;">Stored — leave blank to keep current key</div>
        </div>
        <div v-if="modalTestResult" :style="modalTestResultStyle" style="display:block;margin-top:4px;font-size:12px;padding:6px 10px;border-radius:6px;">{{ modalTestResult }}</div>
        <div v-if="formError" class="repo-form-error">{{ formError }}</div>
        <div class="repo-form-actions">
          <button class="btn-omni" @click="closeModal">Cancel</button>
          <button class="btn-omni" :disabled="modalTestPending" @click="testModalConnection">{{ modalTestPending ? 'Testing…' : 'Test Connection' }}</button>
          <button class="btn-omni" :disabled="savePending" @click="save">{{ savePending ? 'Saving…' : 'Save' }}</button>
        </div>
      </div>
    </div>

    <!-- Confirm delete modal -->
    <div v-if="showConfirm" class="repo-modal-wrap show" @click.self="showConfirm = false">
      <div class="repo-modal-box">
        <div class="repo-modal-title">Delete Omni Instance</div>
        <div style="font-size:13px;color:#e8e8e9;margin-bottom:14px;line-height:1.4;">
          This will remove the Omni instance configuration and clear all clusters and machine classes. Type the endpoint URL to confirm:
        </div>
        <input class="repo-form-input" v-model="confirmInput" :placeholder="state?.omniEndpoint || 'endpoint URL'" style="margin-bottom:14px;" />
        <div class="repo-form-actions" style="margin-top:0">
          <button class="btn-omni" @click="showConfirm = false">Cancel</button>
          <button class="btn-omni" :disabled="confirmInput !== (state?.omniEndpoint || '')" @click="confirmDelete">Delete</button>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, ref, reactive } from 'vue'
import { useAppStore } from '@/stores/appStore'
import { useAuthStore } from '@/stores/authStore'

const appStore = useAppStore()
const authStore = useAuthStore()

const state = computed(() => appStore.state)
const isConfigured = computed(() => !!(state.value?.omniConfigured || state.value?.omniEnvLocked))

// Health badge
const healthBadgeClass = computed(() => {
  const s = state.value?.omniHealth?.status
  if (s === 'ok' || s === 'healthy') return 'badge badge-success'
  if (s === 'failed') return 'badge badge-failed'
  return 'badge badge-idle'
})
const healthLabel = computed(() => {
  const s = state.value?.omniHealth?.status
  if (s === 'ok' || s === 'healthy') return 'Healthy'
  if (s === 'failed') return 'Failed'
  return s || 'Unknown'
})

// Card test connection
const testPending = ref(false)
const testResult = ref('')
const testResultOk = ref(false)
const testResultStyle = computed(() => ({
  background: testResultOk.value ? '#052e16' : '#3f1515',
  color: testResultOk.value ? '#4ade80' : '#f87171',
}))

async function testConnection() {
  testPending.value = true
  testResult.value = ''
  try {
    const r = await fetch('/api/omni-instance/refresh', { method: 'POST' })
    const d = await r.json().catch(() => ({}))
    testResultOk.value = r.ok
    testResult.value = r.ok ? 'Connection successful' : (d.error || 'Test failed')
    setTimeout(() => { testResult.value = '' }, 5000)
  } catch (e: unknown) {
    testResultOk.value = false
    testResult.value = 'Network error'
  } finally {
    testPending.value = false
  }
}

// Modal
const showModal = ref(false)
const isEdit = ref(false)
const form = reactive({ endpoint: '', key: '' })
const formError = ref('')
const savePending = ref(false)
const modalTestResult = ref('')
const modalTestOk = ref(false)
const modalTestPending = ref(false)
const modalTestResultStyle = computed(() => ({
  background: modalTestOk.value ? '#052e16' : '#3f1515',
  color: modalTestOk.value ? '#4ade80' : '#f87171',
}))

function openModal() {
  isEdit.value = !!state.value?.omniConfigured
  form.endpoint = state.value?.omniEndpoint || ''
  form.key = ''
  formError.value = ''
  modalTestResult.value = ''
  showModal.value = true
}

function closeModal() {
  showModal.value = false
}

async function testModalConnection() {
  if (!form.endpoint || !form.key) {
    modalTestResult.value = 'Endpoint and service account key are required for testing'
    modalTestOk.value = false
    return
  }
  modalTestPending.value = true
  modalTestResult.value = ''
  try {
    const r = await fetch('/api/omni-instance/test', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ endpoint: form.endpoint, serviceAccountKey: form.key }),
    })
    const d = await r.json().catch(() => ({}))
    modalTestOk.value = r.ok
    modalTestResult.value = r.ok ? 'Connection successful' : (d.error || 'Test failed')
  } catch {
    modalTestOk.value = false
    modalTestResult.value = 'Network error'
  } finally {
    modalTestPending.value = false
  }
}

async function save() {
  formError.value = ''
  if (!isEdit.value && !form.endpoint) { formError.value = 'Endpoint is required'; return }
  if (!form.key && !state.value?.omniHasStoredKey) { formError.value = 'Service account key is required'; return }
  savePending.value = true
  try {
    const body: Record<string, string> = { endpoint: form.endpoint }
    if (form.key) body.serviceAccountKey = form.key
    const r = await fetch('/api/omni-instance', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(body),
    })
    const d = await r.json().catch(() => ({}))
    if (!r.ok) { formError.value = 'Save failed: ' + (d.error || r.status); return }
    closeModal()
  } catch {
    formError.value = 'Network error'
  } finally {
    savePending.value = false
  }
}

// Delete
const showConfirm = ref(false)
const confirmInput = ref('')

function deleteInstance() {
  confirmInput.value = ''
  showConfirm.value = true
}

async function confirmDelete() {
  showConfirm.value = false
  await fetch('/api/omni-instance', { method: 'DELETE' })
}
</script>
