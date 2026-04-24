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

      <!-- Cluster sub-nav -->
      <template v-if="clusterSubNav">
        <div class="sidebar-sep" style="margin-top:6px;"></div>
        <div class="sidebar-cluster-label">{{ clusterSubNav.name }}</div>
        <RouterLink :to="`/clusters/${clusterSubNav.id}/graph`" class="sidebar-item sidebar-subitem" :class="{ active: clusterSubNav.tab === 'graph' }">
          <span class="sidebar-item-icon"><svg width="16" height="16" viewBox="0 0 16 16" fill="currentColor" xmlns="http://www.w3.org/2000/svg"><path fill-rule="evenodd" clip-rule="evenodd" d="M7.07755 1H9.5C9.77614 1 10 1.22386 10 1.5C10 1.77614 9.77614 2 9.5 2H7.1C6.11166 2 5.40047 2.00039 4.84192 2.04602C4.28921 2.09118 3.93014 2.17814 3.63803 2.32698C3.07354 2.6146 2.6146 3.07354 2.32698 3.63803C2.17814 3.93014 2.09118 4.28921 2.04602 4.84192C2.00039 5.40047 2 6.11166 2 7.1V8.9C2 9.88834 2.00039 10.5995 2.04602 11.1581C2.09118 11.7108 2.17814 12.0699 2.32698 12.362C2.6146 12.9265 3.07354 13.3854 3.63803 13.673C3.93014 13.8219 4.28921 13.9088 4.84192 13.954C5.40047 13.9996 6.11166 14 7.1 14H8.9C9.88834 14 10.5995 13.9996 11.1581 13.954C11.7108 13.9088 12.0699 13.8219 12.362 13.673C12.9265 13.3854 13.3854 12.9265 13.673 12.362C13.8219 12.0699 13.9088 11.7108 13.954 11.1581C13.9996 10.5995 14 9.88834 14 8.9V7.5C14 7.22386 14.2239 7 14.5 7C14.7761 7 15 7.22386 15 7.5V8.92246C15 9.88354 15 10.6355 14.9507 11.2395C14.9004 11.8541 14.7967 12.3594 14.564 12.816C14.1805 13.5686 13.5686 14.1805 12.816 14.564C12.3594 14.7967 11.8541 14.9004 11.2395 14.9507C10.6355 15 9.88354 15 8.92246 15H7.07754C6.11646 15 5.36451 15 4.76049 14.9507C4.14594 14.9004 3.64062 14.7967 3.18404 14.564C2.43139 14.1805 1.81947 13.5686 1.43597 12.816C1.20334 12.3594 1.09956 11.8541 1.04935 11.2395C0.999995 10.6355 0.999997 9.88354 1 8.92245V7.07755C0.999997 6.11646 0.999995 5.36451 1.04935 4.76049C1.09956 4.14594 1.20334 3.64062 1.43597 3.18404C1.81947 2.43139 2.43139 1.81947 3.18404 1.43597C3.64062 1.20334 4.14594 1.09956 4.76049 1.04935C5.36451 0.999995 6.11646 0.999997 7.07755 1Z"/><path fill-rule="evenodd" clip-rule="evenodd" d="M14 4C14.5523 4 15 3.55228 15 3C15 2.44772 14.5523 2 14 2C13.4477 2 13 2.44772 13 3C13 3.55228 13.4477 4 14 4ZM14 5C15.1046 5 16 4.10457 16 3C16 1.89543 15.1046 1 14 1C12.8954 1 12 1.89543 12 3C12 4.10457 12.8954 5 14 5Z"/><path fill-rule="evenodd" clip-rule="evenodd" d="M10.8536 6.14645C11.0488 6.34171 11.0488 6.65829 10.8536 6.85355L8.85357 8.85355C8.67763 9.0295 8.39908 9.0493 8.20002 8.9L6.58771 7.69077L4.89045 9.81235C4.71795 10.028 4.4033 10.0629 4.18767 9.89043C3.97204 9.71793 3.93708 9.40328 4.10958 9.18765L6.10958 6.68765C6.27829 6.47677 6.58397 6.43796 6.80002 6.6L8.4531 7.83981L10.1465 6.14645C10.3417 5.95118 10.6583 5.95118 10.8536 6.14645Z"/></svg></span>
          <span class="sidebar-item-label">Topology</span>
        </RouterLink>
        <RouterLink :to="`/clusters/${clusterSubNav.id}/template`" class="sidebar-item sidebar-subitem" :class="{ active: clusterSubNav.tab === 'template' }">
          <span class="sidebar-item-icon"><svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" stroke-width="1.5" stroke="currentColor" aria-hidden="true" class="sidebar-nav-icon"><path stroke-linecap="round" stroke-linejoin="round" d="M19.5 14.25v-2.625a3.375 3.375 0 0 0-3.375-3.375h-1.5A1.125 1.125 0 0 1 13.5 7.125v-1.5a3.375 3.375 0 0 0-3.375-3.375H8.25m0 12.75h7.5m-7.5 3H12M10.5 2.25H5.625c-.621 0-1.125.504-1.125 1.125v17.25c0 .621.504 1.125 1.125 1.125h12.75c.621 0 1.125-.504 1.125-1.125V11.25a9 9 0 0 0-9-9Z"/></svg></span>
          <span class="sidebar-item-label">Cluster Template</span>
        </RouterLink>
        <RouterLink :to="`/clusters/${clusterSubNav.id}/manifests`" class="sidebar-item sidebar-subitem" :class="{ active: clusterSubNav.tab === 'manifests' }">
          <span class="sidebar-item-icon"><svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" stroke-width="1.5" stroke="currentColor" aria-hidden="true" class="sidebar-nav-icon"><path stroke-linecap="round" stroke-linejoin="round" d="M19.5 14.25v-2.625a3.375 3.375 0 0 0-3.375-3.375h-1.5A1.125 1.125 0 0 1 13.5 7.125v-1.5a3.375 3.375 0 0 0-3.375-3.375H8.25m0 12.75h7.5m-7.5 3H12M10.5 2.25H5.625c-.621 0-1.125.504-1.125 1.125v17.25c0 .621.504 1.125 1.125 1.125h12.75c.621 0 1.125-.504 1.125-1.125V11.25a9 9 0 0 0-9-9Z"/></svg></span>
          <span class="sidebar-item-label">Manifests</span>
        </RouterLink>
        <RouterLink v-if="clusterSubNav.hasError" :to="`/clusters/${clusterSubNav.id}/error`" class="sidebar-item sidebar-subitem" :class="{ active: clusterSubNav.tab === 'error' }" style="color:#f87171;">
          <span class="sidebar-item-icon" style="font-size:13px;">⚠</span>
          <span class="sidebar-item-label">Error</span>
        </RouterLink>
      </template>
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

const clusterSubNav = computed(() => {
  const id = route.params.id as string
  if (!id) return null
  const encoded = encodeURIComponent(id)
  const cluster = appStore.state?.clusters.find(c => c.id === id)
  return {
    id: encoded,
    name: id,
    tab: (route.params.tab as string) || 'graph',
    hasError: !!(cluster?.error || cluster?.lastSyncError),
  }
})

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
