<template>
  <nav class="sidebar" :class="{ 'sidebar-mobile-open': props.mobileOpen }">
    <div class="sidebar-nav">
      <RouterLink
        to="/clusters"
        class="sidebar-item"
        :class="{ active: route.path.startsWith('/clusters') }"
      >
        <span class="sidebar-item-icon" v-html="clustersIconSVG"></span>
        <span class="sidebar-item-label">Clusters</span>
      </RouterLink>

      <RouterLink
        to="/machineclasses"
        class="sidebar-item"
        :class="{ active: route.path === '/machineclasses' }"
      >
        <span class="sidebar-item-icon" v-html="codeBracketIconSVG"></span>
        <span class="sidebar-item-label">Machine Classes</span>
      </RouterLink>


      <RouterLink
        to="/logs"
        class="sidebar-item"
        :class="{ active: route.path === '/logs' }"
      >
        <span class="sidebar-item-icon"><svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" stroke-width="1.5" stroke="currentColor" aria-hidden="true" class="sidebar-nav-icon"><path stroke-linecap="round" stroke-linejoin="round" d="M19.5 14.25v-2.625a3.375 3.375 0 0 0-3.375-3.375h-1.5A1.125 1.125 0 0 1 13.5 7.125v-1.5a3.375 3.375 0 0 0-3.375-3.375H8.25m2.25 0H5.625c-.621 0-1.125.504-1.125 1.125v17.25c0 .621.504 1.125 1.125 1.125h12.75c.621 0 1.125-.504 1.125-1.125V11.25a9 9 0 0 0-9-9Z"></path></svg></span>
        <span class="sidebar-item-label">Logs</span>
      </RouterLink>

      <RouterLink
        to="/audit"
        class="sidebar-item"
        :class="{ active: route.path === '/audit' }"
      >
        <span class="sidebar-item-icon"><svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" stroke-width="1.5" stroke="currentColor" aria-hidden="true" class="sidebar-nav-icon"><path stroke-linecap="round" stroke-linejoin="round" d="M9 12h3.75M9 15h3.75M9 18h3.75m3 .75H18a2.25 2.25 0 0 0 2.25-2.25V6.108c0-1.135-.845-2.098-1.976-2.192a48.424 48.424 0 0 0-1.123-.08m-5.801 0c-.065.21-.1.433-.1.664 0 .414.336.75.75.75h4.5a.75.75 0 0 0 .75-.75 2.25 2.25 0 0 0-.1-.664m-5.8 0A2.251 2.251 0 0 1 13.5 2.25H15c1.012 0 1.867.668 2.15 1.586m-5.8 0c-.376.023-.75.05-1.124.08C9.095 4.01 8.25 4.973 8.25 6.108V8.25m0 0H4.875c-.621 0-1.125.504-1.125 1.125v11.25c0 .621.504 1.125 1.125 1.125h9.75c.621 0 1.125-.504 1.125-1.125V9.375c0-.621-.504-1.125-1.125-1.125H8.25ZM6.75 12h.008v.008H6.75V12Zm0 3h.008v.008H6.75V15Zm0 3h.008v.008H6.75V18Z"></path></svg></span>
        <span class="sidebar-item-label">Audit</span>
      </RouterLink>

      <!-- Settings collapsible group -->
      <button
        class="sidebar-item"
        :class="{ active: isSettingsActive }"
        style="width:100%;background:none;border:none;cursor:pointer;text-align:left;"
        @click="toggleSettings"
      >
        <span class="sidebar-item-icon">⚙</span>
        <span class="sidebar-item-label">Settings</span>
        <div class="sidebar-group-arrow-btn">
          <svg :class="['sidebar-group-arrow', { 'rotate-180': !(settingsOpen || isSettingsActive) }]" viewBox="0 0 24 24" fill="currentColor" xmlns="http://www.w3.org/2000/svg"><path fill-rule="evenodd" clip-rule="evenodd" d="M14.5856 14.5859C15.4765 14.5859 15.9227 13.5088 15.2927 12.8788L12.707 10.293C12.3164 9.90252 11.6833 9.90252 11.2927 10.293L8.70696 12.8788C8.077 13.5088 8.52316 14.5859 9.41407 14.5859L14.5856 14.5859Z"></path></svg>
        </div>
      </button>

      <div class="sidebar-subgroup" :class="{ open: settingsOpen }">
        <RouterLink
          to="/instances"
          class="sidebar-item sidebar-subitem"
          :class="{ active: route.path === '/instances' }"
        >
          <span class="sidebar-item-icon">⬡</span>
          <span class="sidebar-item-label">Instances</span>
        </RouterLink>

        <RouterLink
          to="/repos"
          class="sidebar-item sidebar-subitem"
          :class="{ active: route.path === '/repos' }"
        >
          <span class="sidebar-item-icon" v-html="reposIconSVG"></span>
          <span class="sidebar-item-label">Repos</span>
        </RouterLink>

        <RouterLink
          v-if="!authStore.authDisabled && authStore.isAdmin()"
          to="/users"
          class="sidebar-item sidebar-subitem"
          :class="{ active: route.path === '/users' }"
        >
          <span class="sidebar-item-icon" v-html="usersIconSVG"></span>
          <span class="sidebar-item-label">Users</span>
        </RouterLink>
      </div>
    </div>

    <div class="sidebar-footer">
      <div v-if="authStore.username" class="sidebar-user">
        <div class="sidebar-user-icon" v-html="profileIconSVG"></div>
        <div class="sidebar-user-text">
          <div class="sidebar-user-label">Logged in as:</div>
          <div class="sidebar-user-name">{{ authStore.username }}</div>
        </div>
        <div class="sidebar-user-menu" ref="menuRef">
          <button class="sidebar-user-menu-btn" @click.stop="userMenuOpen = !userMenuOpen">
            <svg viewBox="0 0 24 24" fill="currentColor" xmlns="http://www.w3.org/2000/svg">
              <path d="M7 12C7 13.1046 6.10457 14 5 14C3.89543 14 3 13.1046 3 12C3 10.8954 3.89543 10 5 10C6.10457 10 7 10.8954 7 12Z"></path>
              <path d="M14 12C14 13.1046 13.1046 14 12 14C10.8954 14 10 13.1046 10 12C10 10.8954 10.8954 10 12 10C13.1046 10 14 10.8954 14 12Z"></path>
              <path d="M21 12C21 13.1046 20.1046 14 19 14C17.8954 14 17 13.1046 17 12C17 10.8954 17.8954 10 19 10C20.1046 10 21 10.8954 21 12Z"></path>
            </svg>
          </button>
          <div v-if="userMenuOpen" class="sidebar-user-dropdown">
            <a href="/logout" class="sidebar-user-dropdown-item">
              <span>Log Out</span>
            </a>
          </div>
        </div>
      </div>
      <a v-else href="/logout" class="sidebar-item" style="color:#7d7d85;">
        <span class="sidebar-item-icon">⏻</span>
        <span class="sidebar-item-label">Sign out</span>
      </a>
    </div>
  </nav>
</template>

<script setup lang="ts">
import { ref, computed, watch, onMounted, onUnmounted } from 'vue'
import { useRoute, RouterLink } from 'vue-router'
import { useAppStore } from '@/stores/appStore'
import { useAuthStore } from '@/stores/authStore'
import {
  clustersIconSVG,
  reposIconSVG,
  codeBracketIconSVG,
  usersIconSVG,
  profileIconSVG,
} from '@/assets/icons'

const props = defineProps<{ mobileOpen?: boolean }>()
const emit = defineEmits<{ close: [] }>()

const route = useRoute()
const appStore = useAppStore()
const authStore = useAuthStore()

const settingsRoutes = ['/instances', '/repos', '/users']
const isSettingsActive = computed(() => settingsRoutes.includes(route.path))

const settingsOpen = ref(
  localStorage.getItem('sidebar-settings-open') === '1' || settingsRoutes.includes(route.path)
)

function toggleSettings() {
  settingsOpen.value = !settingsOpen.value
  localStorage.setItem('sidebar-settings-open', settingsOpen.value ? '1' : '0')
}

// Close sidebar on navigation (mobile); open settings group when navigating into a settings route
watch(() => route.path, (path) => {
  emit('close')
  if (settingsRoutes.includes(path)) settingsOpen.value = true
})

// User menu
const userMenuOpen = ref(false)
const menuRef = ref<HTMLElement | null>(null)

function onClickOutside(e: MouseEvent) {
  if (menuRef.value && !menuRef.value.contains(e.target as Node)) {
    userMenuOpen.value = false
  }
}
onMounted(() => document.addEventListener('click', onClickOutside))
onUnmounted(() => document.removeEventListener('click', onClickOutside))
</script>
