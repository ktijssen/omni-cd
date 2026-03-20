<template>
  <div class="auth-wrap">
    <div class="auth-logo">
      <span v-html="omniLogoSVG" style="width:28px;height:28px;flex-shrink:0;display:flex;align-items:center;justify-content:center;"></span>
      <span class="auth-logo-text">Omni <span>CD</span></span>
    </div>
    <div class="auth-card">
      <div class="auth-title">Create your Admin account</div>
      <div class="auth-sub">Set up access to Omni CD</div>
      <div v-if="error" class="auth-error">{{ error }}</div>
      <form @submit.prevent="submit">
        <div class="auth-field">
          <label>Username</label>
          <input type="text" value="admin" disabled />
        </div>
        <div class="auth-field">
          <label for="password">Password</label>
          <input id="password" v-model="password" type="password" placeholder="••••••••" autocomplete="new-password" autofocus required @input="onPasswordInput" />
        </div>
        <div class="auth-field">
          <label for="confirm">Confirm Password</label>
          <input id="confirm" v-model="confirm" type="password" placeholder="••••••••" autocomplete="new-password" required />
          <div v-if="confirm" class="confirm-msg" :class="passwordsMatch ? 'match' : 'mismatch'">
            {{ passwordsMatch ? '✓ Passwords match' : '✗ Passwords do not match' }}
          </div>
        </div>
        <div class="pw-checks">
          <div :class="['pw-check', checks.len && 'met']">
            <span class="pw-icon">{{ checks.len ? '✓' : '✗' }}</span>12 characters or more
          </div>
          <div :class="['pw-check', checks.upper && 'met']">
            <span class="pw-icon">{{ checks.upper ? '✓' : '✗' }}</span>Uppercase letter
          </div>
          <div :class="['pw-check', checks.num && 'met']">
            <span class="pw-icon">{{ checks.num ? '✓' : '✗' }}</span>Number
          </div>
          <div :class="['pw-check', checks.special && 'met']">
            <span class="pw-icon">{{ checks.special ? '✓' : '✗' }}</span>Special character
          </div>
        </div>
        <button class="auth-btn" type="submit" :disabled="pending || !allChecksMet">
          {{ pending ? 'Setting up…' : 'Set Password' }}
        </button>
      </form>
    </div>
    <div class="auth-footer">Omni CD · First-time setup</div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, reactive } from 'vue'
import { useRouter } from 'vue-router'
import { omniLogoSVG } from '@/assets/icons'

const router = useRouter()
const password = ref('')
const confirm = ref('')
const error = ref('')
const pending = ref(false)

const checks = reactive({ len: false, upper: false, num: false, special: false })

const passwordsMatch = computed(() => password.value === confirm.value)
const allChecksMet = computed(() => checks.len && checks.upper && checks.num && checks.special && passwordsMatch.value && confirm.value.length > 0)

function onPasswordInput() {
  const v = password.value
  checks.len = v.length >= 12
  checks.upper = /[A-Z]/.test(v)
  checks.num = /[0-9]/.test(v)
  checks.special = /[^a-zA-Z0-9]/.test(v)
}

onMounted(async () => {
  try {
    const res = await fetch('/api/setup-status')
    if (res.ok) {
      const data = await res.json()
      if (!data.needed) {
        router.replace('/login')
      }
    }
  } catch {
    // proceed with form
  }
})

async function submit() {
  error.value = ''
  if (!allChecksMet.value) return
  pending.value = true
  try {
    const res = await fetch('/setup', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ password: password.value, confirm: confirm.value }),
    })
    const data = await res.json().catch(() => ({}))
    if (res.ok) {
      router.replace('/login')
    } else {
      error.value = data.error || 'Setup failed'
    }
  } catch {
    error.value = 'Network error — please try again'
  } finally {
    pending.value = false
  }
}
</script>

<style scoped>
.auth-wrap {
  min-height: 100vh;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  background: #101118;
  padding: 24px;
}
.auth-logo {
  display: flex;
  align-items: center;
  gap: 10px;
  margin-bottom: 28px;
}
.auth-logo-text {
  font-size: 20px;
  font-weight: 700;
  color: #fff;
  letter-spacing: -0.4px;
}
.auth-logo-text span { color: #ff8b59; }
.auth-card {
  width: 100%;
  max-width: 360px;
  background: #1f222e;
  border: 1px solid #2c2e38;
  border-radius: 14px;
  padding: 28px 28px 24px;
}
.auth-title {
  font-size: 15px;
  font-weight: 600;
  color: #fff;
  margin-bottom: 4px;
}
.auth-sub {
  font-size: 12px;
  color: #5b5c64;
  margin-bottom: 22px;
}
.auth-error {
  background: rgba(248, 113, 113, 0.08);
  border: 1px solid rgba(248, 113, 113, 0.25);
  border-radius: 8px;
  color: #f87171;
  font-size: 13px;
  padding: 9px 13px;
  margin-bottom: 16px;
}
.auth-field {
  margin-bottom: 14px;
}
.auth-field label {
  display: block;
  font-size: 11px;
  font-weight: 500;
  color: #7d7d85;
  text-transform: uppercase;
  letter-spacing: 0.06em;
  margin-bottom: 6px;
}
.auth-field input {
  width: 100%;
  background: #101118;
  border: 1px solid #2c2e38;
  border-radius: 8px;
  color: #e8e8e9;
  font-size: 14px;
  padding: 9px 13px;
  outline: none;
  transition: border-color 0.15s;
  box-sizing: border-box;
  font-family: inherit;
}
.auth-field input:focus { border-color: #ff8b59; }
.auth-field input::placeholder { color: #3f4050; }
.auth-field input:disabled { opacity: 0.4; cursor: not-allowed; }
.confirm-msg {
  font-size: 12px;
  margin-top: 5px;
}
.confirm-msg.match { color: #4ade80; }
.confirm-msg.mismatch { color: #f87171; }
.pw-checks {
  display: flex;
  flex-direction: column;
  gap: 5px;
  margin: 0 0 18px;
}
.pw-check {
  display: flex;
  align-items: center;
  gap: 7px;
  font-size: 12px;
  color: #3f4050;
  transition: color 0.15s;
}
.pw-check.met { color: #4ade80; }
.pw-icon { font-size: 11px; width: 12px; flex-shrink: 0; }
.auth-btn {
  display: block;
  width: 100%;
  background: #ff8b59;
  border: 1px solid #ff8b59;
  border-radius: 8px;
  color: #fff;
  font-size: 13px;
  font-weight: 600;
  padding: 10px;
  cursor: pointer;
  margin-top: 4px;
  transition: background 0.15s, border-color 0.15s;
  text-align: center;
  font-family: inherit;
  box-sizing: border-box;
}
.auth-btn:hover:not(:disabled) { background: #ffb899; border-color: #ffb899; color: #fff; }
.auth-btn:disabled { opacity: 0.4; cursor: not-allowed; }
.auth-footer {
  margin-top: 20px;
  font-size: 11px;
  color: #3f4050;
}
</style>
