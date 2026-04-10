<template>
  <RouterView v-if="isPublicRoute" />
  <template v-else>
    <div v-if="!appStore.state" class="loading-overlay">
      <div class="loading-spinner-large"></div>
      <div class="loading-overlay-title">Omni CD</div>
      <div class="loading-overlay-sub">Connecting...</div>
    </div>
    <template v-else>
      <!-- Omni alert bar -->
      <div
        v-if="appStore.state.omniHealth?.status === 'failed'"
        class="omni-alert-bar visible"
      >
        <span class="omni-alert-bar-icon">⚠</span>
        <span class="omni-alert-bar-msg">
          Omni is unreachable
          <span v-if="appStore.state.omniHealth.downSince"> since {{ new Date(appStore.state.omniHealth.downSince).toLocaleString() }}</span>
        </span>
        <RouterLink to="/instances" class="omni-alert-bar-link">View details</RouterLink>
      </div>
      <div class="layout">
        <header class="app-header">
          <button class="sidebar-toggle" :aria-expanded="sidebarOpen" aria-controls="app-sidebar" @click="sidebarOpen = !sidebarOpen">
            <span class="sidebar-toggle-icon">
              <!-- Hamburger -->
              <svg xmlns="http://www.w3.org/2000/svg" width="16" height="16" stroke="currentColor" stroke-linecap="round" :style="{ opacity: sidebarOpen ? 0 : 1 }" aria-label="open sidebar"><path d="M2.5 12.5h11M2.5 8h11m-11-4.5h11"></path></svg>
              <!-- Close -->
              <svg width="16" height="16" viewBox="0 0 16 16" fill="currentColor" xmlns="http://www.w3.org/2000/svg" :style="{ opacity: sidebarOpen ? 1 : 0, transform: 'rotate(-90deg)' }" aria-label="close sidebar"><path d="M4.85355 4.14645C4.65829 3.95118 4.34171 3.95118 4.14645 4.14645C3.95118 4.34171 3.95118 4.65829 4.14645 4.85355L7.29289 8L4.14645 11.1464C3.95118 11.3417 3.95118 11.6583 4.14645 11.8536C4.34171 12.0488 4.65829 12.0488 4.85355 11.8536L8 8.70711L11.1464 11.8536C11.3417 12.0488 11.6583 12.0488 11.8536 11.8536C12.0488 11.6583 12.0488 11.3417 11.8536 11.1464L8.70711 8L11.8536 4.85355C12.0488 4.65829 12.0488 4.34171 11.8536 4.14645C11.6583 3.95118 11.3417 3.95118 11.1464 4.14645L8 7.29289L4.85355 4.14645Z"></path></svg>
            </span>
          </button>
          <span class="sidebar-item-icon" v-html="omniLogoSVG" style="width:28px;height:28px;flex-shrink:0;display:flex;align-items:center;justify-content:center;"></span>
          <div class="sidebar-logo-text">
            Omni <span>CD</span>
            <div v-if="appStore.state?.appVersion" class="sidebar-logo-version">{{ appStore.state.appVersion }}</div>
          </div>
          <a href="https://github.com/ktijssen/omni-cd" target="_blank" rel="noopener noreferrer" class="header-icon-link" style="margin-left:auto;" aria-label="GitHub">
            <svg xmlns="http://www.w3.org/2000/svg" width="26" height="26" viewBox="0 0 24 24" fill="currentColor"><path d="M12 2C6.477 2 2 6.477 2 12c0 4.418 2.865 8.166 6.839 9.489.5.092.682-.217.682-.482 0-.237-.009-.868-.013-1.703-2.782.604-3.369-1.342-3.369-1.342-.454-1.154-1.11-1.462-1.11-1.462-.908-.62.069-.608.069-.608 1.003.07 1.531 1.03 1.531 1.03.892 1.529 2.341 1.087 2.91.831.092-.646.35-1.086.636-1.336-2.22-.253-4.555-1.11-4.555-4.943 0-1.091.39-1.984 1.029-2.683-.103-.253-.446-1.27.098-2.647 0 0 .84-.269 2.75 1.025A9.578 9.578 0 0 1 12 6.836a9.59 9.59 0 0 1 2.504.337c1.909-1.294 2.747-1.025 2.747-1.025.546 1.377.202 2.394.1 2.647.64.699 1.028 1.592 1.028 2.683 0 3.842-2.339 4.687-4.566 4.935.359.309.678.919.678 1.852 0 1.336-.012 2.415-.012 2.741 0 .267.18.578.688.48C19.138 20.163 22 16.418 22 12c0-5.523-4.477-10-10-10z"/></svg>
          </a>
        </header>
        <div class="layout-body">
          <!-- Mobile backdrop -->
          <div v-if="sidebarOpen" class="sidebar-backdrop" @click="sidebarOpen = false"></div>
          <AppSidebar id="app-sidebar" :mobile-open="sidebarOpen" @close="sidebarOpen = false" />
          <main class="main-content">
            <RouterView />
          </main>
        </div>
      </div>
    </template>
  </template>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, onUnmounted } from 'vue'
import { RouterView, RouterLink, useRoute } from 'vue-router'
import { useAppStore } from '@/stores/appStore'
import { useAuthStore } from '@/stores/authStore'
import AppSidebar from '@/components/layout/AppSidebar.vue'
import { omniLogoSVG } from '@/assets/icons'

const appStore = useAppStore()
const authStore = useAuthStore()
const route = useRoute()

const isPublicRoute = computed(() => !!route.meta.public)
const sidebarOpen = ref(false)

onMounted(async () => {
  await authStore.fetchMe()
  appStore.connect()
})

onUnmounted(() => {
  appStore.disconnect()
})
</script>
