import { defineStore } from 'pinia'
import { ref } from 'vue'
import type { MeResponse } from '@/types'

export const useAuthStore = defineStore('auth', () => {
  const username = ref('')
  const role = ref('admin')
  const authDisabled = ref(false)
  const oidcEnabled = ref(false)
  const loaded = ref(false)

  async function fetchMe() {
    try {
      const res = await fetch('/api/me')
      if (res.ok) {
        const data: MeResponse = await res.json()
        username.value = data.username
        role.value = data.role
        authDisabled.value = data.authDisabled
        oidcEnabled.value = data.oidcEnabled
      }
    } catch {
      // ignore
    } finally {
      loaded.value = true
    }
  }

  function isAdmin() {
    return role.value === 'admin' || authDisabled.value
  }

  return { username, role, authDisabled, oidcEnabled, loaded, fetchMe, isAdmin }
})
