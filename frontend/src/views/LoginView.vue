<template>
  <div class="auth-wrap">
    <div class="auth-logo">
      <span style="width:28px;height:28px;flex-shrink:0;display:flex;align-items:center;justify-content:center;" v-html="omniLogoSVG"></span>
      <span class="auth-logo-text">Omni <span>CD</span></span>
    </div>
    <div class="auth-card">
      <div class="auth-title">Sign in</div>
      <div class="auth-sub">Enter your credentials to continue</div>
      <div v-if="error" class="auth-error">{{ error }}</div>
      <template v-if="localAuth">
        <form @submit.prevent="submit">
          <div class="auth-field">
            <label for="username">Username</label>
            <input id="username" v-model="username" type="text" placeholder="admin" autocomplete="username" autofocus required />
          </div>
          <div class="auth-field">
            <label for="password">Password</label>
            <input id="password" v-model="password" type="password" placeholder="••••••••" autocomplete="current-password" required />
          </div>
          <button class="auth-btn auth-btn-submit" type="submit" :disabled="pending">
            {{ pending ? 'Signing in…' : 'Sign in' }}
          </button>
        </form>
        <div v-if="oidcEnabled" class="auth-divider"><span>or</span></div>
      </template>
      <a v-if="oidcEnabled" href="/auth/login" class="auth-btn auth-btn-oidc">Continue with SSO</a>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { omniLogoSVG } from '@/assets/icons'

const router = useRouter()
const username = ref('')
const password = ref('')
const error = ref('')
const pending = ref(false)
const localAuth = ref(false)
const oidcEnabled = ref(false)

onMounted(async () => {
  try {
    const res = await fetch('/api/login-config')
    if (res.ok) {
      const data = await res.json()
      localAuth.value = data.localAuth
      oidcEnabled.value = data.oidcEnabled
    }
  } catch {
    localAuth.value = true
  }
})

async function submit() {
  error.value = ''
  pending.value = true
  try {
    const body = new URLSearchParams()
    body.append('username', username.value)
    body.append('password', password.value)
    const res = await fetch('/login', {
      method: 'POST',
      headers: { 'Content-Type': 'application/x-www-form-urlencoded' },
      body: body.toString(),
    })
    const data = await res.json().catch(() => ({}))
    if (res.ok) {
      window.location.href = '/'
    } else {
      error.value = data.error || 'Invalid username or password'
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
.auth-btn {
  display: block;
  width: 100%;
  background: #1e2130;
  border: 1px solid #3d4059;
  border-radius: 8px;
  color: #c4c4c9;
  font-size: 13px;
  font-weight: 500;
  padding: 10px;
  cursor: pointer;
  margin-top: 8px;
  transition: border-color 0.2s, background 0.2s, color 0.2s;
  text-align: center;
  text-decoration: none;
  font-family: inherit;
  box-sizing: border-box;
}
.auth-btn:hover { border-color: #ff8b59; background: #ff8b59; color: #fff; }
.auth-btn:disabled { opacity: 0.5; cursor: not-allowed; }
.auth-btn-submit { background: #ff8b59; border-color: #ff8b59; color: #fff; }
.auth-btn-submit:hover:not(:disabled) { background: #d97040; border-color: #d97040; color: #fff; }
.auth-btn-submit:disabled { background: #ff8b59; opacity: 0.5; }
.auth-btn-oidc { margin-top: 0; background: #ff8b59; border-color: #ff8b59; color: #fff; }
.auth-btn-oidc:hover { background: #d97040; border-color: #d97040; color: #fff; }
.auth-divider {
  display: flex;
  align-items: center;
  gap: 10px;
  margin: 14px 0;
  color: #3f4050;
  font-size: 11px;
}
.auth-divider::before,
.auth-divider::after {
  content: '';
  flex: 1;
  height: 1px;
  background: #2c2e38;
}
.auth-footer {
  margin-top: 20px;
  font-size: 11px;
  color: #3f4050;
}
</style>
