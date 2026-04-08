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
          <div class="sidebar-logo-text">Omni <span>CD</span></div>
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
