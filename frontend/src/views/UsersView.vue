<template>
  <div class="container">
    <div class="header" style="border-bottom:none;">
      <h1 style="font-size:18px;font-weight:600;color:#fff;letter-spacing:-0.3px;">Users</h1>
    </div>

    <!-- Local admin account section -->
    <div v-if="!localUsersLoaded" style="color:#7d7d85;font-size:13px;padding:24px 0;">Loading...</div>
    <template v-else-if="localUsers.length > 0">
      <div style="font-size:13px;font-weight:600;color:#9fa1a6;text-transform:uppercase;letter-spacing:0.06em;margin-bottom:12px;">Local Admin Account</div>
      <div style="display:flex;align-items:center;gap:12px;background:#15161e;border:1px solid #2c2e38;border-radius:10px;padding:14px 16px;max-width:480px;">
        <div style="width:18px;height:18px;opacity:0.5;flex-shrink:0;color:#9fa1a6;font-size:16px;">👤</div>
        <div style="flex:1;min-width:0;">
          <div style="font-size:13px;font-weight:600;color:#fff;white-space:nowrap;overflow:hidden;text-overflow:ellipsis;">
            {{ localUsers[0].displayName || localUsers[0].username }}
          </div>
        </div>
        <div style="display:flex;gap:8px;">
          <button class="btn-omni" @click="openEditProfile">Edit Profile</button>
          <button class="btn-omni" @click="openChangePassword">Change Password</button>
        </div>
      </div>
    </template>

    <!-- SSO users section -->
    <div v-if="authStore.oidcEnabled" style="margin-top:28px">
      <div style="font-size:13px;font-weight:600;color:#9fa1a6;text-transform:uppercase;letter-spacing:0.06em;margin-bottom:12px;">SSO Users</div>
      <div style="background:#15161e;border:1px solid #2c2e38;border-radius:10px;overflow:hidden;max-width:560px">
        <div v-if="!oidcUsersLoaded" style="padding:12px 14px;font-size:13px;color:#7d7d85;">Loading...</div>
        <div v-else-if="oidcUsers.length === 0" style="padding:12px 14px;font-size:13px;color:#7d7d85;">No SSO users have logged in yet.</div>
        <div
          v-else
          v-for="user in oidcUsers"
          :key="user.email"
          style="display:flex;align-items:center;gap:12px;padding:10px 14px;border-bottom:1px solid #2c2e38;"
        >
          <div style="flex:1;min-width:0;">
            <div style="font-size:13px;font-weight:600;color:#fff;white-space:nowrap;overflow:hidden;text-overflow:ellipsis;">
              {{ user.displayName || user.email }}
            </div>
            <div style="font-size:12px;color:#7d7d85;">{{ user.email }}</div>
            <div style="font-size:11px;color:#5b5c64;margin-top:2px">Last seen: {{ formatDate(user.lastSeen) }}</div>
          </div>
          <span :style="{ fontSize: '12px', fontWeight: '600', color: roleColor(user.role), marginRight: '8px' }">{{ roleLabel(user.role) }}</span>
          <button class="btn-omni" @click="openEditOIDCUser(user)">Edit Role</button>
          <button class="btn-omni" @click="openDeleteOIDCUser(user)">Delete</button>
        </div>
      </div>
    </div>

    <!-- Change Password modal -->
    <div v-if="showChangePassword" class="repo-modal-wrap show">
      <div class="repo-modal-box">
        <div class="repo-modal-title">Change Password</div>
        <div style="color:#f87171;font-size:13px;min-height:18px;margin-bottom:8px;">{{ changePwError }}</div>
        <div class="repo-form-group">
          <label class="repo-form-label">Current Password</label>
          <input v-model="changePwForm.current" class="repo-form-input" type="password" placeholder="••••••••" autocomplete="current-password" />
        </div>
        <div class="repo-form-group">
          <label class="repo-form-label">New Password</label>
          <input v-model="changePwForm.newPw" class="repo-form-input" type="password" placeholder="••••••••" autocomplete="new-password" />
        </div>
        <div class="repo-form-group">
          <label class="repo-form-label">Confirm New Password</label>
          <input v-model="changePwForm.confirm" class="repo-form-input" type="password" placeholder="••••••••" autocomplete="new-password" />
          <div v-if="changePwForm.confirm" style="font-size:12px;margin-top:6px;" :style="{ color: changePwForm.confirm === changePwForm.newPw ? '#4ade80' : '#f87171' }">
            {{ changePwForm.confirm === changePwForm.newPw ? '✓ Passwords match' : '✗ Passwords do not match' }}
          </div>
        </div>
        <div class="pw-checks">
          <div class="pw-check" :class="{ met: changePwForm.newPw.length >= 12 }">
            <span class="pw-check-icon">{{ changePwForm.newPw.length >= 12 ? '✓' : '✗' }}</span>12 characters or more
          </div>
          <div class="pw-check" :class="{ met: /[A-Z]/.test(changePwForm.newPw) }">
            <span class="pw-check-icon">{{ /[A-Z]/.test(changePwForm.newPw) ? '✓' : '✗' }}</span>Uppercase letter
          </div>
          <div class="pw-check" :class="{ met: /[0-9]/.test(changePwForm.newPw) }">
            <span class="pw-check-icon">{{ /[0-9]/.test(changePwForm.newPw) ? '✓' : '✗' }}</span>Number
          </div>
          <div class="pw-check" :class="{ met: /[^a-zA-Z0-9]/.test(changePwForm.newPw) }">
            <span class="pw-check-icon">{{ /[^a-zA-Z0-9]/.test(changePwForm.newPw) ? '✓' : '✗' }}</span>Special character
          </div>
        </div>
        <div style="display:flex;gap:8px;justify-content:flex-end;margin-top:8px;">
          <button class="btn-omni" @click="showChangePassword = false">Cancel</button>
          <button class="btn-omni" :disabled="!changePwAllMet" @click="submitChangePassword">Change Password</button>
        </div>
      </div>
    </div>

    <!-- Edit Profile modal -->
    <div v-if="showEditProfile" class="repo-modal-wrap show">
      <div class="repo-modal-box">
        <div class="repo-modal-title">Edit Profile</div>
        <div style="color:#f87171;font-size:13px;min-height:18px;margin-bottom:8px;">{{ editProfileError }}</div>
        <div class="repo-form-group">
          <label class="repo-form-label">Username</label>
          <input class="repo-form-input" :value="localUsers[0]?.username" type="text" readonly style="opacity:0.5;cursor:not-allowed;" autocomplete="username" />
        </div>
        <div class="repo-form-group">
          <label class="repo-form-label">Display Name</label>
          <input v-model="editProfileForm.displayName" class="repo-form-input" type="text" placeholder="Your name" autocomplete="name" />
        </div>
        <div style="display:flex;gap:8px;justify-content:flex-end;margin-top:8px;">
          <button class="btn-omni" @click="showEditProfile = false">Cancel</button>
          <button class="btn-omni" @click="submitEditProfile">Save</button>
        </div>
      </div>
    </div>

    <!-- Edit OIDC user role modal -->
    <div v-if="editOIDCUser" class="repo-modal-wrap show">
      <div class="repo-modal-box">
        <div class="repo-modal-title">Edit SSO User</div>
        <div style="color:#f87171;font-size:13px;min-height:18px;margin-bottom:8px;">{{ editOIDCError }}</div>
        <div class="repo-form-group">
          <label class="repo-form-label">Role</label>
          <div style="display:flex;flex-direction:column;gap:8px;margin-top:4px;">
            <div
              v-for="opt in oidcRoleOptions"
              :key="opt.value"
              style="display:flex;align-items:center;gap:10px;padding:10px 14px;border-radius:8px;border:1px solid;cursor:pointer;transition:border-color 0.15s,background 0.15s;"
              :style="{ borderColor: selectedOIDCRole === opt.value ? opt.color : '#2c2e38', background: selectedOIDCRole === opt.value ? '#1f222e' : '' }"
              @click="selectedOIDCRole = opt.value"
            >
              <div style="width:10px;height:10px;border-radius:50%;flex-shrink:0;" :style="{ background: opt.color }"></div>
              <div>
                <div style="font-size:13px;font-weight:600;" :style="{ color: opt.color }">{{ opt.label }}</div>
                <div style="font-size:11px;color:#7d7d85;">{{ opt.desc }}</div>
              </div>
            </div>
          </div>
        </div>
        <div style="display:flex;gap:8px;justify-content:flex-end;margin-top:16px;">
          <button class="btn-omni" @click="editOIDCUser = null">Cancel</button>
          <button class="btn-omni" @click="submitEditOIDCUser">Save</button>
        </div>
      </div>
    </div>

    <!-- Delete OIDC user modal -->
    <div v-if="deleteOIDCUser" class="repo-modal-wrap show">
      <div class="repo-modal-box">
        <div class="repo-modal-title">Delete SSO User</div>
        <p style="font-size:13px;color:#9fa1a6;margin-bottom:4px;">
          Delete SSO User <span style="color:#fff;font-weight:600;">{{ deleteOIDCUser.email }}</span>?
        </p>
        <p style="font-size:13px;color:#7d7d85;margin-bottom:20px;">
          They will be removed from the user list and their active session will be invalidated.
        </p>
        <div style="color:#f87171;font-size:13px;min-height:18px;margin-bottom:8px;">{{ deleteOIDCError }}</div>
        <div style="display:flex;gap:8px;justify-content:flex-end;">
          <button class="btn-omni" @click="deleteOIDCUser = null">Cancel</button>
          <button class="btn-omni" @click="submitDeleteOIDCUser">Delete</button>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, computed, onMounted } from 'vue'
import { useAuthStore } from '@/stores/authStore'

const authStore = useAuthStore()

interface LocalUser {
  username: string
  displayName?: string
  role?: string
}
interface OIDCUser {
  email: string
  displayName?: string
  role: string
  lastSeen: string
}

const localUsers = ref<LocalUser[]>([])
const localUsersLoaded = ref(false)
const oidcUsers = ref<OIDCUser[]>([])
const oidcUsersLoaded = ref(false)

// Change password
const showChangePassword = ref(false)
const changePwError = ref('')
const changePwForm = reactive({ current: '', newPw: '', confirm: '' })
const changePwAllMet = computed(() =>
  changePwForm.current.length > 0 &&
  changePwForm.newPw.length >= 12 &&
  /[A-Z]/.test(changePwForm.newPw) &&
  /[0-9]/.test(changePwForm.newPw) &&
  /[^a-zA-Z0-9]/.test(changePwForm.newPw) &&
  changePwForm.confirm === changePwForm.newPw &&
  changePwForm.confirm.length > 0
)

// Edit profile
const showEditProfile = ref(false)
const editProfileError = ref('')
const editProfileForm = reactive({ displayName: '' })

// OIDC role options
const oidcRoleOptions = [
  { value: 'admin',  label: 'Admin',     color: '#ff8b59', desc: 'Full access — can change settings and trigger actions' },
  { value: 'viewer', label: 'Viewer',    color: '#22c55e', desc: 'Read-only access — can view clusters and logs' },
  { value: 'none',   label: 'No Access', color: '#7d7d85', desc: 'Cannot log in — redirected to the access denied page' },
]
const editOIDCUser = ref<OIDCUser | null>(null)
const selectedOIDCRole = ref('none')
const editOIDCError = ref('')
const deleteOIDCUser = ref<OIDCUser | null>(null)
const deleteOIDCError = ref('')

function roleLabel(role: string): string {
  if (role === 'admin') return 'Admin'
  if (role === 'viewer') return 'Viewer'
  return 'No Access'
}
function roleColor(role: string): string {
  if (role === 'admin') return '#ff8b59'
  if (role === 'viewer') return '#22c55e'
  return '#7d7d85'
}
function formatDate(d: string): string {
  if (!d) return ''
  return new Date(d).toLocaleString()
}

async function fetchUsers() {
  try {
    const r = await fetch('/api/users')
    if (r.ok) {
      const data = await r.json()
      localUsers.value = Array.isArray(data) ? data : (data.users || [])
    }
  } catch { /* ignore */ }
  localUsersLoaded.value = true
}

async function fetchOIDCUsers() {
  if (!authStore.oidcEnabled) return
  try {
    const r = await fetch('/api/users/oidc')
    if (r.ok) {
      const data = await r.json()
      oidcUsers.value = Array.isArray(data) ? data : (data.users || [])
    }
  } catch { /* ignore */ }
  oidcUsersLoaded.value = true
}

function openChangePassword() {
  changePwForm.current = ''
  changePwForm.newPw = ''
  changePwForm.confirm = ''
  changePwError.value = ''
  showChangePassword.value = true
}

function openEditProfile() {
  editProfileForm.displayName = localUsers.value[0]?.displayName || ''
  editProfileError.value = ''
  showEditProfile.value = true
}

async function submitChangePassword() {
  changePwError.value = ''
  if (changePwForm.newPw !== changePwForm.confirm) {
    changePwError.value = 'New passwords do not match'
    return
  }
  try {
    const r = await fetch('/api/users/change-password', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ currentPassword: changePwForm.current, newPassword: changePwForm.newPw }),
    })
    const d = await r.json()
    if (!r.ok) { changePwError.value = d.error || 'Failed to change password'; return }
    showChangePassword.value = false
  } catch { changePwError.value = 'Request failed' }
}

async function submitEditProfile() {
  editProfileError.value = ''
  try {
    const r = await fetch('/api/users/update-profile', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ newDisplayName: editProfileForm.displayName }),
    })
    const d = await r.json()
    if (!r.ok) { editProfileError.value = d.error || 'Failed to update profile'; return }
    showEditProfile.value = false
    await fetchUsers()
  } catch { editProfileError.value = 'Request failed' }
}

function openEditOIDCUser(user: OIDCUser) {
  editOIDCUser.value = user
  selectedOIDCRole.value = user.role || 'none'
  editOIDCError.value = ''
}

async function submitEditOIDCUser() {
  editOIDCError.value = ''
  if (!editOIDCUser.value) return
  try {
    const r = await fetch('/api/users/oidc', {
      method: 'PATCH',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ email: editOIDCUser.value.email, role: selectedOIDCRole.value }),
    })
    if (!r.ok) { editOIDCError.value = 'Failed to update role'; return }
    editOIDCUser.value = null
    await fetchOIDCUsers()
  } catch { editOIDCError.value = 'Network error' }
}

function openDeleteOIDCUser(user: OIDCUser) {
  deleteOIDCUser.value = user
  deleteOIDCError.value = ''
}

async function submitDeleteOIDCUser() {
  deleteOIDCError.value = ''
  if (!deleteOIDCUser.value) return
  try {
    const r = await fetch('/api/users/oidc', {
      method: 'DELETE',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ email: deleteOIDCUser.value.email }),
    })
    if (!r.ok) { deleteOIDCError.value = 'Failed to delete user'; return }
    deleteOIDCUser.value = null
    await fetchOIDCUsers()
  } catch { deleteOIDCError.value = 'Network error' }
}

onMounted(async () => {
  await Promise.all([fetchUsers(), fetchOIDCUsers()])
})
</script>
