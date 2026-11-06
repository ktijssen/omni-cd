<template>
  <div class="repo-modal-wrap show" @click.self="emit('close')">
    <div class="repo-modal-box mw-box" role="dialog" aria-labelledby="mw-title">
      <div id="mw-title" class="repo-modal-title">{{ isEdit ? 'Edit Maintenance Window' : 'Add Maintenance Window' }}</div>
      <p class="mw-help">
        While auto-sync is on, changes from Git (including creating or deleting a cluster) are
        applied to these clusters only while the window is open. The manual <b>Sync</b> button always works.
      </p>

      <div class="repo-form-group">
        <label class="repo-form-label" for="mw-name">Name <span style="color:#f87171">*</span></label>
        <input id="mw-name" v-model="name" class="repo-form-input" maxlength="64" placeholder="e.g. Weekend production" />
      </div>

      <div class="repo-form-group">
        <div class="repo-form-label">
          Select by label
          <span class="mw-count">clusters with all of these labels, now and in the future</span>
        </div>
        <div v-for="(row, i) in selectorRows" :key="i" class="mw-sel-row">
          <input
            v-model.trim="row.key"
            class="repo-form-input mw-sel-input"
            list="mw-label-keys"
            placeholder="key, e.g. env"
            :aria-label="`Label ${i + 1} key`"
            autocomplete="off"
            @change="splitPair(row)"
          />
          <span class="mw-sel-eq">=</span>
          <input
            v-model.trim="row.value"
            class="repo-form-input mw-sel-input"
            :list="`mw-label-values-${i}`"
            placeholder="value"
            :aria-label="`Label ${i + 1} value`"
            autocomplete="off"
          />
          <datalist :id="`mw-label-values-${i}`">
            <option v-for="v in valuesFor(row.key)" :key="v" :value="v" />
          </datalist>
          <button type="button" class="mw-remove" :aria-label="`Remove label ${i + 1}`" @click="selectorRows.splice(i, 1)">✕</button>
          <div v-if="selectorHint(clusters, row.key, row.value)" class="mw-sel-hint">
            {{ selectorHint(clusters, row.key, row.value) }}
            <button
              v-for="k in suggestedKeys(row.key)"
              :key="k"
              type="button"
              class="mw-link"
              @click="row.value = row.key; row.key = k"
            >Use {{ k }} = {{ row.key }}</button>
          </div>
        </div>
        <datalist id="mw-label-keys">
          <option v-for="k in labelKeys" :key="k" :value="k" />
        </datalist>
        <button type="button" class="mw-link" @click="selectorRows.push({ key: '', value: '' })">+ Add label</button>
        <div v-if="hasSelector" class="mw-sel-preview">
          <template v-if="labelMatched.length">Matches {{ labelMatched.length }}: {{ labelMatched.join(', ') }}</template>
          <template v-else>No clusters match yet — clusters that get these labels later are included automatically.</template>
        </div>
      </div>

      <div class="repo-form-group">
        <div class="mw-label-row">
          <span class="repo-form-label" style="margin:0">
            {{ hasSelector ? 'Also include clusters' : 'Clusters' }}
            <span class="mw-count">{{ selected.length }} ticked</span>
          </span>
          <span class="mw-bulk">
            <button type="button" class="mw-link" :disabled="!visibleOptions.length" @click="selectVisible(true)">Select all</button>
            <button type="button" class="mw-link" :disabled="!selected.length" @click="selectVisible(false)">None</button>
          </span>
        </div>
        <input
          v-if="options.length > 8"
          v-model.trim="filter"
          class="repo-form-input mw-filter"
          placeholder="Filter clusters…"
          aria-label="Filter clusters"
        />
        <div class="mw-cluster-list" role="group" aria-label="Clusters">
          <label
            v-for="id in visibleOptions"
            :key="id"
            class="mw-cluster"
            :class="{ 'via-label': viaLabelOnly(id) }"
            :title="viaLabelOnly(id) ? 'Included by the label selector' : ''"
          >
            <input v-if="viaLabelOnly(id)" type="checkbox" checked disabled />
            <input v-else v-model="selected" type="checkbox" :value="id" />
            <span class="mw-cluster-name">{{ id }}</span>
            <span v-if="!knownIds.has(id)" class="mw-missing" title="This cluster is no longer known to Omni CD">not found</span>
            <span v-else-if="labelMatchedSet.has(id)" class="mw-via">via label</span>
          </label>
          <div v-if="!options.length" class="mw-empty">No managed clusters yet.</div>
          <div v-else-if="!visibleOptions.length" class="mw-empty">No clusters match “{{ filter }}”.</div>
        </div>
      </div>

      <div class="repo-form-group">
        <div class="repo-form-label">
          Days <span style="color:#f87171">*</span>
        </div>
        <div class="mw-days" role="group" aria-label="Days">
          <button
            v-for="d in WEEKDAYS"
            :key="d.key"
            type="button"
            class="mw-day"
            :class="{ active: days.includes(d.key) }"
            :aria-pressed="days.includes(d.key)"
            @click="toggleDay(d.key)"
          >{{ d.label }}</button>
        </div>
      </div>

      <div class="repo-form-group">
        <div class="repo-form-label">
          Time <span style="color:#f87171">*</span>
          <span class="mw-count">server time ({{ timezone || 'UTC' }})</span>
        </div>
        <div class="mw-times">
          <div class="mw-time">
            <span>From</span>
            <TimeInput24 v-model="start" label="Start" />
          </div>
          <div class="mw-time">
            <span>To</span>
            <TimeInput24 v-model="end" label="End" />
          </div>
          <span v-if="end < start" class="mw-overnight">ends next day</span>
        </div>
      </div>

      <div v-if="error" class="repo-form-error">{{ error }}</div>

      <div class="repo-form-actions">
        <button class="btn-omni" type="button" :disabled="saving" @click="emit('close')">Cancel</button>
        <button class="btn-omni" type="button" :disabled="saving" @click="save">
          {{ saving ? 'Saving…' : 'Save' }}
        </button>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, ref } from 'vue'
import type { MaintenanceWindow, ResourceInfo, Weekday } from '@/types'
import { WEEKDAYS, matchesSelector, selectorHint } from './maintenance'
import TimeInput24 from './TimeInput24.vue'

const props = withDefaults(defineProps<{
  /** The window being edited; omit to add a new one. */
  window?: MaintenanceWindow
  /** Managed clusters that can be selected, with their labels. */
  clusters: ResourceInfo[]
  /** Server timezone the times are read in. */
  timezone?: string
}>(), { window: undefined, timezone: 'UTC' })
const emit = defineEmits<{ close: [] }>()

const isEdit = computed(() => !!props.window)
const name = ref(props.window?.name ?? '')
const selected = ref<string[]>([...(props.window?.clusters ?? [])])
const days = ref<Weekday[]>(props.window ? [...props.window.days] : [])
const start = ref(props.window?.start ?? '06:00')
const end = ref(props.window?.end ?? '10:00')
const selectorRows = ref<{ key: string; value: string }[]>(
  Object.entries(props.window?.selector ?? {}).map(([key, value]) => ({ key, value })),
)
const filter = ref('')
const saving = ref(false)
const error = ref('')

const knownIds = computed(() => new Set(props.clusters.map(c => c.id)))

// Known clusters plus any already-selected ones that have since disappeared,
// so they can still be seen and unticked.
const options = computed(() => {
  const all = new Set([...knownIds.value, ...(props.window?.clusters ?? [])])
  return [...all].sort((a, b) => a.localeCompare(b))
})

/** Label key/value suggestions from the clusters' live and Git labels. */
const labelValues = computed(() => {
  const m = new Map<string, Set<string>>()
  for (const c of props.clusters) {
    for (const [k, v] of [...Object.entries(c.labels ?? {}), ...Object.entries(c.templateLabels ?? {})]) {
      if (!m.has(k)) m.set(k, new Set())
      m.get(k)!.add(v)
    }
  }
  return m
})
const labelKeys = computed(() => [...labelValues.value.keys()].sort())
function valuesFor(key: string): string[] {
  return [...(labelValues.value.get(key) ?? [])].sort()
}

/** The selector as entered; rows with an empty key are ignored. */
const selector = computed<Record<string, string>>(() => {
  const out: Record<string, string> = {}
  for (const r of selectorRows.value) if (r.key) out[r.key] = r.value
  return out
})
const hasSelector = computed(() => Object.keys(selector.value).length > 0)
const labelMatched = computed(() =>
  hasSelector.value
    ? props.clusters.filter(c => matchesSelector(c, selector.value)).map(c => c.id).sort((a, b) => a.localeCompare(b))
    : [],
)
const labelMatchedSet = computed(() => new Set(labelMatched.value))
/** Included by the selector and not ticked explicitly: shown ticked + locked. */
/**
 * "env=dev" or "env: dev" typed or pasted into the key box is split into key
 * and value. Runs on change (blur/Enter) and before saving — not per
 * keystroke, which would split at "env=" and send the rest back to the key.
 */
function splitPair(row: { key: string; value: string }) {
  const m = /^([^=:\s]+)\s*[=:]\s*(.*)$/.exec(row.key)
  if (m && !row.value) {
    row.key = m[1]
    row.value = m[2].trim()
  }
}

/** Label keys whose value equals text — for "Did you mean env = dev?". */
function suggestedKeys(text: string): string[] {
  if (!text || labelValues.value.has(text)) return []
  return [...labelValues.value.entries()].filter(([, vs]) => vs.has(text)).map(([k]) => k).sort()
}

function viaLabelOnly(id: string): boolean {
  return labelMatchedSet.value.has(id) && !selected.value.includes(id)
}
const visibleOptions = computed(() => {
  const f = filter.value.toLowerCase()
  return f ? options.value.filter(id => id.toLowerCase().includes(f)) : options.value
})

function selectVisible(on: boolean) {
  const visible = new Set(visibleOptions.value)
  const rest = selected.value.filter(id => !visible.has(id))
  selected.value = on ? [...rest, ...visibleOptions.value] : rest
}

function toggleDay(d: Weekday) {
  const i = days.value.indexOf(d)
  if (i >= 0) days.value.splice(i, 1)
  else days.value.push(d)
}

function validate(): string {
  selectorRows.value.forEach(splitPair)
  if (!name.value.trim()) return 'Enter a name.'
  const keys = selectorRows.value.filter(r => r.key || r.value)
  if (keys.some(r => !r.key)) return 'Every label needs a key.'
  if (new Set(keys.map(r => r.key)).size !== keys.length) return 'Each label key can only be used once.'
  if (keys.some(r => r.key.startsWith('omni.sidero.dev/'))) return 'omni.sidero.dev/ labels are managed by Omni and cannot be used.'
  if (!selected.value.length && !hasSelector.value) return 'Select at least one cluster or label.'
  if (!days.value.length) return 'Pick at least one day.'
  if (start.value === end.value) return 'Start and end time must differ.'
  return ''
}

async function save() {
  error.value = validate()
  if (error.value) return
  const body: MaintenanceWindow = {
    id: props.window?.id ?? '',
    name: name.value.trim(),
    clusters: options.value.filter(id => selected.value.includes(id)),
    selector: hasSelector.value ? selector.value : undefined,
    days: WEEKDAYS.map(d => d.key).filter(d => days.value.includes(d)),
    start: start.value,
    end: end.value,
  }
  saving.value = true
  try {
    const r = await fetch('/api/maintenance-windows', {
      method: isEdit.value ? 'PUT' : 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(body),
    })
    if (!r.ok) {
      const text = (await r.text()).trim()
      try { error.value = JSON.parse(text).error || text } catch { error.value = text || r.statusText }
      return
    }
    emit('close')
  } catch (e) {
    error.value = 'Request failed: ' + (e instanceof Error ? e.message : String(e))
  } finally {
    saving.value = false
  }
}
</script>

<style scoped>
.mw-box { width: 540px; }
.mw-help { font-size: 12px; line-height: 1.5; color: #9fa1a6; margin: -8px 0 16px; }
.mw-label-row { display: flex; align-items: baseline; justify-content: space-between; margin-bottom: 6px; }
.mw-count { margin-left: 6px; color: #5b5c64; font-size: 11px; }
.mw-bulk { display: flex; gap: 10px; }
.mw-link { background: none; border: none; padding: 0; font-size: 12px; color: #ff8b59; cursor: pointer; font-family: inherit; }
.mw-link:disabled { color: #5b5c64; cursor: default; }
.mw-filter { margin-bottom: 6px; }
.mw-cluster-list { max-height: 180px; overflow-y: auto; border: 1px solid #2c2e38; border-radius: 6px; background: #13141c; padding: 4px; }
.mw-cluster { display: flex; align-items: center; gap: 8px; padding: 5px 8px; border-radius: 4px; font-size: 13px; color: #e8e8e9; cursor: pointer; }
.mw-cluster:hover { background: #1f222e; }
.mw-cluster input { accent-color: #ff8b59; cursor: pointer; }
.mw-cluster-name { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.mw-missing { margin-left: auto; font-size: 11px; color: #f87171; }
.mw-empty { padding: 8px; font-size: 12px; color: #5b5c64; }
.mw-cluster.via-label { color: #9fa1a6; cursor: default; }
.mw-cluster.via-label input { cursor: default; }
.mw-via { margin-left: auto; font-size: 11px; color: #ff8b59; }
.mw-sel-row { display: flex; flex-wrap: wrap; align-items: center; gap: 6px; margin-bottom: 6px; }
.mw-sel-hint { flex-basis: 100%; display: flex; flex-wrap: wrap; gap: 4px 10px; font-size: 12px; color: #fb923c; line-height: 1.5; }
.mw-sel-input { flex: 1 1 0; min-width: 0; font-family: ui-monospace, SFMono-Regular, monospace; }
.mw-sel-eq { color: #7d7d85; }
.mw-sel-preview { margin-top: 6px; font-size: 12px; color: #9fa1a6; line-height: 1.5; }
.mw-remove { background: none; border: none; color: #7d7d85; cursor: pointer; font-size: 13px; padding: 2px 6px; border-radius: 4px; }
.mw-remove:hover { color: #f87171; background: #2c2e38; }
.mw-days { display: flex; flex-wrap: wrap; gap: 6px; }
.mw-day {
  min-width: 44px; padding: 5px 8px; font-size: 12px; border-radius: 4px; cursor: pointer;
  border: 1px solid #3d4059; background: #1e2130; color: #9fa1a6; font-family: inherit;
}
.mw-day:hover { border-color: #ff8b59; color: #e8e8e9; }
.mw-day.active { border-color: #ff8b59; background: rgba(255, 139, 89, 0.15); color: #ff8b59; }
.mw-day:focus-visible, .mw-link:focus-visible { outline: 2px solid #ff8b59; outline-offset: 2px; }
.mw-times { display: flex; flex-wrap: wrap; align-items: flex-end; gap: 12px; }
.mw-time { display: flex; flex-direction: column; gap: 4px; font-size: 12px; color: #9fa1a6; }
.mw-overnight { font-size: 12px; color: #fb923c; padding-bottom: 8px; }
</style>
