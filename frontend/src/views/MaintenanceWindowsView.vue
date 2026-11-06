<template>
  <div class="container">
    <div class="header" style="border-bottom:none;">
      <h1 style="font-size:18px;font-weight:600;color:#fff;letter-spacing:-0.3px;">Maintenance Windows</h1>
      <span class="mw-tz" title="Window times are read in the server's timezone (set with the TZ environment variable)">
        Server time: {{ timezone }}
      </span>
      <button
        v-if="authStore.isAdmin()"
        class="btn-omni"
        style="margin-left:auto"
        @click="editing = { window: undefined }"
      >Add Maintenance Window</button>
    </div>

    <div v-if="windows.length === 0" class="placeholder-page">
      <div class="placeholder-icon mw-placeholder-icon" v-html="clockIconSVG"></div>
      <div class="placeholder-title">Maintenance Windows</div>
      <div class="placeholder-sub">
        No maintenance windows — clusters with auto-sync on are synced at any time.<br />
        Add a window to only apply changes to a set of clusters during set hours.
      </div>
    </div>

    <div v-else class="info-row">
      <div v-for="w in windows" :key="w.id" class="info-card">
        <div class="info-card-header">
          <span class="info-card-title">{{ w.name }}</span>
          <span :class="['badge', w.status.open ? 'badge-success' : 'badge-outofsync']">{{ w.status.open ? 'Open' : 'Closed' }}</span>
        </div>
        <div class="info-card-value">{{ maintenanceSummary(w.status) }}</div>
        <div class="info-card-sub">
          <div class="mw-range">
            <span class="mw-range-days">{{ daysLabel(w.days) }}</span>
            <span>{{ w.start }}–{{ w.end }}<span v-if="w.end < w.start" class="mw-next-day"> (+1 day)</span></span>
          </div>
          <div v-if="w.selector && Object.keys(w.selector).length" class="mw-selector">
            <span class="mw-selector-label">Labels:</span>
            <span
              v-for="[k, v] in Object.entries(w.selector).sort(([a], [b]) => a.localeCompare(b))"
              :key="k"
              class="cluster-label"
            >{{ k }}<template v-if="v">: <span class="cluster-label-value">{{ v }}</span></template></span>
          </div>
          <div v-for="h in selectorHints(w)" :key="h" class="mw-hint">{{ h }}</div>
          <div class="mw-chips" :aria-label="`Clusters in ${w.name}`">
            <span v-if="!windowClusters(w).length" class="mw-none">No clusters match yet</span>
            <RouterLink
              v-for="id in windowClusters(w)"
              :key="id"
              :to="`/clusters/${id}`"
              class="mw-chip"
              :class="{ off: autoSyncOff(id), gone: !known(id) }"
              :title="!known(id) ? 'Cluster no longer known to Omni CD' : autoSyncOff(id) ? 'Auto-sync is off — nothing is applied automatically' : ''"
            >{{ id }}</RouterLink>
          </div>
        </div>
        <div v-if="authStore.isAdmin()" class="info-card-actions">
          <button class="btn-omni" @click="editing = { window: w }">Edit</button>
          <button class="btn-omni" @click="deleteTarget = w">Delete</button>
        </div>
      </div>
    </div>

    <MaintenanceWindowEditor
      v-if="editing"
      :window="editing.window"
      :clusters="managedClusters"
      :timezone="timezone"
      @close="editing = null"
    />

    <!-- Delete confirm -->
    <div v-if="deleteTarget" class="repo-modal-wrap show" @click.self="deleteTarget = null">
      <div class="repo-modal-box">
        <div class="repo-modal-title">Remove Maintenance Window</div>
        <div style="font-size:13px;color:#e8e8e9;margin-bottom:14px;line-height:1.4;">
          Remove <b>{{ deleteTarget.name }}</b>? Clusters that are in no other window will have changes
          from Git applied at any time while auto-sync is on.
        </div>
        <div v-if="deleteError" class="repo-form-error" style="margin:0 0 12px">{{ deleteError }}</div>
        <div class="repo-form-actions" style="margin-top:0">
          <button class="btn-omni" @click="deleteTarget = null">Cancel</button>
          <button class="btn-omni" :disabled="deleting" @click="doDelete">{{ deleting ? 'Removing…' : 'Remove' }}</button>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { RouterLink } from 'vue-router'
import { useAppStore } from '@/stores/appStore'
import { useAuthStore } from '@/stores/authStore'
import type { MaintenanceWindow, MaintenanceWindowView, ResourceInfo } from '@/types'
import { clockIconSVG } from '@/assets/icons'
import MaintenanceWindowEditor from '@/components/maintenance/MaintenanceWindowEditor.vue'
import { daysLabel, maintenanceSummary, selectorHint } from '@/components/maintenance/maintenance'

const appStore = useAppStore()
const authStore = useAuthStore()

const clusters = computed<ResourceInfo[]>(() => appStore.state?.clusters ?? [])
const timezone = computed(() => appStore.state?.maintenanceTimezone || 'UTC')
const windows = computed<MaintenanceWindowView[]>(() =>
  [...(appStore.state?.maintenanceWindows ?? [])].sort((a, b) => a.name.localeCompare(b.name)),
)

/** Clusters a window can include: managed by Omni CD and not being deleted. */
const managedClusters = computed(() =>
  clusters.value
    .filter(c => !['unmanaged', 'orphaned', 'deleting'].includes(c.status))
    .sort((a, b) => a.id.localeCompare(b.id)),
)

/** Warnings for selector labels that match no cluster (e.g. key "dev" instead of env=dev). */
function selectorHints(w: MaintenanceWindowView): string[] {
  return Object.entries(w.selector ?? {})
    .map(([k, v]) => selectorHint(clusters.value, k, v))
    .filter(Boolean)
}

/** Clusters shown on a card: currently matched, plus listed ones that vanished. */
function windowClusters(w: MaintenanceWindowView): string[] {
  const all = new Set([...(w.matchedClusters ?? []), ...(w.clusters ?? [])])
  return [...all].sort((a, b) => a.localeCompare(b))
}

const byId = computed(() => new Map(clusters.value.map(c => [c.id, c])))
function known(id: string): boolean { return byId.value.has(id) }
function autoSyncOff(id: string): boolean { return byId.value.get(id)?.autoSync === false }

const editing = ref<{ window?: MaintenanceWindow } | null>(null)

const deleteTarget = ref<MaintenanceWindowView | null>(null)
const deleting = ref(false)
const deleteError = ref('')
watch(deleteTarget, () => { deleteError.value = '' })

async function doDelete() {
  if (!deleteTarget.value) return
  deleting.value = true
  try {
    const r = await fetch('/api/maintenance-windows', {
      method: 'DELETE',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ id: deleteTarget.value.id }),
    })
    if (!r.ok) {
      const text = (await r.text()).trim()
      try { deleteError.value = JSON.parse(text).error || text } catch { deleteError.value = text || r.statusText }
      return
    }
    deleteTarget.value = null
  } catch (e) {
    deleteError.value = 'Request failed: ' + (e instanceof Error ? e.message : String(e))
  } finally {
    deleting.value = false
  }
}
</script>

<style scoped>
.mw-tz { margin-left: 12px; font-size: 12px; color: #7d7d85; }
.mw-placeholder-icon { display: flex; justify-content: center; color: #5b5c64; }
.mw-placeholder-icon :deep(svg) { width: 40px; height: 40px; }
.mw-range { display: flex; gap: 10px; color: #e8e8e9; }
.mw-range-days { min-width: 140px; color: #9fa1a6; }
.mw-next-day { color: #7d7d85; }
.mw-chips { display: flex; flex-wrap: wrap; gap: 6px; margin-top: 10px; }
.mw-chip {
  font-size: 12px; line-height: 1; padding: 5px 8px; border-radius: 4px; text-decoration: none;
  border: 1px solid #3d4059; background: #1e2130; color: #c4c4c9;
}
.mw-chip:hover { border-color: #ff8b59; color: #ff8b59; }
.mw-chip.off { color: #7d7d85; }
.mw-chip.gone { color: #f87171; border-style: dashed; }
.mw-selector { display: flex; flex-wrap: wrap; align-items: center; gap: 4px; margin-top: 8px; }
.mw-selector-label { color: #7d7d85; margin-right: 2px; }
.mw-none { font-size: 12px; color: #5b5c64; }
.mw-hint { margin-top: 6px; font-size: 12px; color: #fb923c; line-height: 1.5; }
</style>
