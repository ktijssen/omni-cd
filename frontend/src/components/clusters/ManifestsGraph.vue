<template>
  <div class="manifest-graph">
    <div class="cluster-graph-toolbar">
      <button class="cluster-graph-zoom-btn" title="Collapse all" @click="collapseAll">&#8991;</button>
      <button class="cluster-graph-zoom-btn" title="Expand all" @click="expandAll">&#8990;</button>
      <span class="graph-toolbar-sep"></span>
      <button class="cluster-graph-zoom-btn" @click="zoom('out')">&#8722;</button>
      <span class="graph-zoom-level">{{ Math.round(zoomLevel * 100) }}%</span>
      <button class="cluster-graph-zoom-btn" @click="zoom('reset')">&#8635;</button>
      <button class="cluster-graph-zoom-btn" @click="zoom('in')">&#43;</button>
    </div>
    <div
      ref="canvasRef"
      class="manifest-graph-canvas"
      @wheel.prevent="onWheel"
      @mousedown="onDragStart"
      @mousemove="onDragMove"
      @mouseup="onDragEnd"
      @mouseleave="onDragEnd"
    >
      <div ref="innerRef" class="cluster-graph-inner" :style="innerStyle">
        <!-- Group nodes column -->
        <div :style="{ position: 'relative', width: '220px', height: `${maxH}px`, flexShrink: 0 }">
          <div
            v-for="(g, gi) in groupEntries"
            :key="g.name"
            :style="{ position: 'absolute', top: `${Math.round(groupYs[gi] - NH / 2)}px`, left: 0 }"
          >
            <div class="dag-node-wrap">
              <div class="dag-node" style="width:220px">
                <div class="dag-node-icon" :style="{ color: phaseColor(g.group.phase) }" v-html="manifestIconSVG"></div>
                <div class="dag-node-body">
                  <div class="dag-node-kind">Group</div>
                  <div class="dag-node-name" :title="g.name">{{ g.name }}</div>
                  <div class="dag-node-meta">Mode: {{ g.group.mode === 'one-time' ? 'One-Time' : 'Full' }}</div>
                  <div class="dag-node-meta">In Sync: {{ syncedCount(g) }}/{{ g.manifests.length }}</div>
                </div>
                <button v-if="g.manifests.length > 0" class="dag-fold-btn" @click.stop="toggleGroupCollapse(g.name)">
                  {{ isGroupCollapsed(g.name) ? '+' : '−' }}
                </button>
              </div>
            </div>
          </div>
        </div>

        <template v-if="visibleKindEntries.length > 0">
          <!-- Edges: group -> kind -->
          <svg :width="EW" :height="maxH" overflow="visible" style="flex-shrink:0;">
            <g v-for="c in groupKindConnections" :key="c.key">
              <path :d="c.d" stroke="#2c2e38" stroke-width="1.5" fill="none" stroke-dasharray="4,3" />
              <polygon :points="c.arrowPoints" fill="#5b5c64" />
            </g>
          </svg>

          <!-- Kind nodes column -->
          <div :style="{ position: 'relative', width: '200px', height: `${maxH}px`, flexShrink: 0 }">
            <div
              v-for="(k, ki) in visibleKindEntries"
              :key="k.key"
              :style="{ position: 'absolute', top: `${Math.round(kindYs[ki] - NH / 2)}px`, left: 0 }"
            >
              <div class="dag-node-wrap">
                <div class="dag-node" style="width:200px">
                  <div class="dag-node-icon" :style="{ color: phaseColor(kindPhase(k)) }" v-html="k8sIconSVG"></div>
                  <div class="dag-node-body">
                    <div class="dag-node-kind">Kind</div>
                    <div class="dag-node-name" :title="pluralizeKind(k.kind)">{{ pluralizeKind(k.kind) }}</div>
                    <div class="dag-node-meta">In Sync: {{ kindSyncedCount(k) }}/{{ k.manifests.length }}</div>
                  </div>
                  <button class="dag-fold-btn" @click.stop="toggleKindCollapse(k.key)">
                    {{ isKindCollapsed(k.key) ? '+' : '−' }}
                  </button>
                </div>
              </div>
            </div>
          </div>

          <template v-if="visibleManifests.length > 0">
            <!-- Edges: kind -> manifest -->
            <svg :width="EW" :height="maxH" overflow="visible" style="flex-shrink:0;">
              <g v-for="c in connections" :key="c.key">
                <path :d="c.d" stroke="#2c2e38" stroke-width="1.5" fill="none" stroke-dasharray="4,3" />
                <polygon :points="c.arrowPoints" fill="#5b5c64" />
              </g>
            </svg>

            <!-- Manifest nodes column -->
            <div :style="{ position: 'relative', width: '260px', height: `${maxH}px`, flexShrink: 0 }">
              <div
                v-for="m in visibleManifests"
                :key="m.key"
                :style="{ position: 'absolute', top: `${Math.round(manifestYs[m.flatIdx] - NH / 2)}px`, left: 0 }"
              >
                <div class="dag-node-wrap">
                  <div
                    v-if="m.overflowCount"
                    class="dag-node dag-node-overflow"
                    style="width:260px;cursor:pointer;"
                    @click="emit('show-full-group', groupEntries[m.gi].name)"
                  >
                    <div class="dag-node-body">
                      <div class="dag-node-kind">&ctdot; +{{ m.overflowCount }} more</div>
                      <div class="dag-node-name">Switch to List view &rarr;</div>
                    </div>
                  </div>
                  <div v-else class="dag-node" style="width:260px">
                    <div class="dag-node-icon" :style="{ color: phaseColor(m.status.phase) }" v-html="k8sIconSVG"></div>
                    <div class="dag-node-body">
                      <div class="dag-node-kind">{{ m.status.kind }}</div>
                      <div class="dag-node-name" :title="m.status.name">{{ m.status.name }}</div>
                      <div class="dag-node-meta">Namespace: {{ m.status.namespace || 'cluster-scoped' }}</div>
                      <div class="dag-node-meta">
                        Status: <span :style="{ color: phaseColor(m.status.phase) }">{{ phaseName(m.status.phase) }}</span>
                      </div>
                    </div>
                  </div>
                </div>
              </div>
            </div>
          </template>
        </template>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, nextTick, onMounted, ref } from 'vue'
import type { ManifestGroupStatus, ManifestStatus } from '@/types'
import { manifestIconSVG, k8sIconSVG } from '@/assets/icons'

const props = defineProps<{
  groups: Record<string, ManifestGroupStatus>
  forceExpand?: boolean
}>()

const emit = defineEmits<{ 'show-full-group': [name: string] }>()

const NH = 100, EW = 60
const INTRA_GAP = 8, INTER_GAP = 32
const MAX_VISIBLE_PER_GROUP = 40

interface GroupEntry {
  name: string
  group: ManifestGroupStatus
  manifests: { key: string; status: ManifestStatus }[]
}

const groupEntries = computed<GroupEntry[]>(() =>
  Object.entries(props.groups).map(([name, group]) => ({
    name,
    group,
    manifests: Object.entries(group.manifests).map(([key, status]) => ({ key, status })),
  })),
)

function syncedCount(g: GroupEntry): number {
  return g.manifests.filter(m => m.status.phase === 'applied').length
}

function pluralizeKind(kind: string): string {
  if (/(s|x|ch|sh)$/i.test(kind)) return `${kind}es`
  if (/[^aeiou]y$/i.test(kind)) return `${kind.slice(0, -1)}ies`
  return `${kind}s`
}

interface KindEntry { key: string; gi: number; kind: string; manifests: { key: string; status: ManifestStatus }[] }

const kindEntriesAll = computed<KindEntry[]>(() => {
  const result: KindEntry[] = []
  groupEntries.value.forEach((g, gi) => {
    const byKind = new Map<string, { key: string; status: ManifestStatus }[]>()
    g.manifests.forEach(m => {
      const kind = m.status.kind || 'Unknown'
      if (!byKind.has(kind)) byKind.set(kind, [])
      byKind.get(kind)!.push(m)
    })
    Array.from(byKind.keys()).sort().forEach(kind => {
      result.push({ key: `${g.name}::${kind}`, gi, kind, manifests: byKind.get(kind)! })
    })
  })
  return result
})

function kindSyncedCount(k: KindEntry): number {
  return k.manifests.filter(m => m.status.phase === 'applied').length
}

function kindPhase(k: KindEntry): string {
  if (k.manifests.some(m => m.status.phase === 'deleting')) return 'deleting'
  if (k.manifests.some(m => m.status.phase === 'progressing')) return 'progressing'
  if (k.manifests.every(m => m.status.phase === 'applied')) return 'applied'
  if (k.manifests.some(m => m.status.phase === 'pending')) return 'pending'
  return 'unknown'
}

const collapsedGroups = ref<Set<string>>(
  new Set(groupEntries.value.filter(g => g.manifests.length > 0).map(g => g.name)),
)
const collapsedKinds = ref<Set<string>>(new Set(kindEntriesAll.value.map(k => k.key)))

function isGroupCollapsed(name: string): boolean {
  if (props.forceExpand) return false
  return collapsedGroups.value.has(name)
}

function toggleGroupCollapse(name: string) {
  const next = new Set(collapsedGroups.value)
  if (next.has(name)) next.delete(name)
  else next.add(name)
  collapsedGroups.value = next
}

function isKindCollapsed(key: string): boolean {
  if (props.forceExpand) return false
  return collapsedKinds.value.has(key)
}

function toggleKindCollapse(key: string) {
  const next = new Set(collapsedKinds.value)
  if (next.has(key)) next.delete(key)
  else next.add(key)
  collapsedKinds.value = next
}

function collapseAll() {
  collapsedGroups.value = new Set(groupEntries.value.filter(g => g.manifests.length > 0).map(g => g.name))
  collapsedKinds.value = new Set(kindEntriesAll.value.map(k => k.key))
}

function expandAll() {
  collapsedGroups.value = new Set()
  collapsedKinds.value = new Set()
}

interface VisibleKindEntry extends KindEntry { flatIdx: number }

const visibleKindEntries = computed<VisibleKindEntry[]>(() => {
  const result: VisibleKindEntry[] = []
  kindEntriesAll.value.forEach(k => {
    const g = groupEntries.value[k.gi]
    if (isGroupCollapsed(g.name)) return
    result.push({ ...k, flatIdx: result.length })
  })
  return result
})

interface ManifestInfo { key: string; status: ManifestStatus; gi: number; ki: number; flatIdx: number; overflowCount?: number }

const allManifests = computed<ManifestInfo[]>(() => {
  const result: ManifestInfo[] = []
  visibleKindEntries.value.forEach((k, ki) => {
    if (isKindCollapsed(k.key)) return
    const cap = props.forceExpand ? k.manifests.length : MAX_VISIBLE_PER_GROUP
    const visible = k.manifests.slice(0, cap)
    visible.forEach(m => result.push({ ...m, gi: k.gi, ki, flatIdx: result.length }))
    const overflow = k.manifests.length - visible.length
    if (overflow > 0) {
      result.push({ key: `${k.key}:__overflow`, status: {} as ManifestStatus, gi: k.gi, ki, flatIdx: result.length, overflowCount: overflow })
    }
  })
  return result
})

const visibleManifests = computed(() => allManifests.value)

function getManifestsForKind(ki: number): ManifestInfo[] {
  return allManifests.value.filter(m => m.ki === ki)
}

// Single top-to-bottom tree layout pass: each node's height is exactly the stacked
// height of its visible children (or NH if collapsed/childless), and each node's Y is
// the midpoint of the range its children occupy. This keeps every level in one shared
// coordinate space, so a node's position never has to be reconciled against an
// independently-computed sibling (which previously caused stray gaps when expanded
// and collapsed siblings were mixed at the same level).
interface LayoutResult { groupYs: number[]; kindYs: number[]; manifestYs: number[]; maxH: number }

const layout = computed<LayoutResult>(() => {
  const groupYs: number[] = new Array(groupEntries.value.length).fill(0)
  const kindYs: number[] = new Array(visibleKindEntries.value.length).fill(0)
  const manifestYs: number[] = new Array(allManifests.value.length).fill(0)

  const kindIndicesByGroup = new Map<number, number[]>()
  visibleKindEntries.value.forEach((k, ki) => {
    if (!kindIndicesByGroup.has(k.gi)) kindIndicesByGroup.set(k.gi, [])
    kindIndicesByGroup.get(k.gi)!.push(ki)
  })

  let cursor = 0
  groupEntries.value.forEach((g, gi) => {
    const kis = isGroupCollapsed(g.name) ? [] : (kindIndicesByGroup.get(gi) ?? [])
    const groupStart = cursor
    if (kis.length > 0) {
      kis.forEach((ki, kIdx) => {
        const k = visibleKindEntries.value[ki]
        const kindStart = cursor
        const ms = isKindCollapsed(k.key) ? [] : getManifestsForKind(ki)
        if (ms.length > 0) {
          ms.forEach(m => {
            manifestYs[m.flatIdx] = cursor + NH / 2
            cursor += NH + INTRA_GAP
          })
          cursor -= INTRA_GAP
        } else {
          cursor += NH
        }
        kindYs[ki] = (kindStart + cursor) / 2
        if (kIdx < kis.length - 1) cursor += INTRA_GAP
      })
    } else {
      cursor += NH
    }
    groupYs[gi] = (groupStart + cursor) / 2
    if (gi < groupEntries.value.length - 1) cursor += INTER_GAP
  })

  return { groupYs, kindYs, manifestYs, maxH: Math.max(cursor, NH) }
})

const groupYs = computed(() => layout.value.groupYs)
const kindYs = computed(() => layout.value.kindYs)
const manifestYs = computed(() => layout.value.manifestYs)
const maxH = computed(() => layout.value.maxH)

interface Conn { key: string; d: string; arrowPoints: string }

const groupKindConnections = computed<Conn[]>(() => {
  const hw = EW * 0.5
  return visibleKindEntries.value.map((k, ki) => {
    const y0 = groupYs.value[k.gi]
    const y1 = kindYs.value[ki]
    return {
      key: `${k.gi}:${ki}`,
      d: `M0,${y0.toFixed(1)} H${hw.toFixed(1)} V${y1.toFixed(1)} H${EW}`,
      arrowPoints: `${EW},${y1.toFixed(1)} ${EW - 5},${(y1 - 3).toFixed(1)} ${EW - 5},${(y1 + 3).toFixed(1)}`,
    }
  })
})

const connections = computed<Conn[]>(() => {
  const hw = EW * 0.5
  return allManifests.value.map(m => {
    const y0 = kindYs.value[m.ki]
    const y1 = manifestYs.value[m.flatIdx]
    return {
      key: `${m.ki}:${m.flatIdx}`,
      d: `M0,${y0.toFixed(1)} H${hw.toFixed(1)} V${y1.toFixed(1)} H${EW}`,
      arrowPoints: `${EW},${y1.toFixed(1)} ${EW - 5},${(y1 - 3).toFixed(1)} ${EW - 5},${(y1 + 3).toFixed(1)}`,
    }
  })
})

function phaseName(phase: string): string {
  switch (phase) {
    case 'applied':     return 'Applied'
    case 'progressing': return 'Progressing'
    case 'pending':     return 'Pending'
    case 'deleting':    return 'Deleting'
    default:            return 'Unknown'
  }
}

function phaseColor(phase: string): string {
  switch (phase) {
    case 'applied':     return '#4ade80'
    case 'progressing': return '#fbbf24'
    case 'pending':     return '#9fa1a6'
    case 'deleting':    return '#f87171'
    default:            return '#7d7d85'
  }
}

// Pan/zoom (mirrors ClusterGraph)
const zoomLevel = ref(1)
const translateX = ref(0)
const translateY = ref(0)
const canvasRef = ref<HTMLElement>()
const innerRef = ref<HTMLElement>()
const isDragging = ref(false)
let dragStart = { x: 0, y: 0, tx: 0, ty: 0 }

const innerStyle = computed(() => ({
  transform: `translate(${translateX.value.toFixed(1)}px,${translateY.value.toFixed(1)}px) scale(${zoomLevel.value})`,
  transformOrigin: 'top left',
  alignItems: 'flex-start',
  gap: '0',
}))

function zoom(dir: 'in' | 'out' | 'reset') {
  if (dir === 'in') zoomLevel.value = Math.min(2.5, +(zoomLevel.value + 0.15).toFixed(2))
  else if (dir === 'out') zoomLevel.value = Math.max(0.25, +(zoomLevel.value - 0.15).toFixed(2))
  else {
    zoomLevel.value = 1
    translateX.value = 0
    translateY.value = 0
    nextTick(centreGraph)
  }
}

function centreGraph() {
  const canvas = canvasRef.value
  const inner = innerRef.value
  if (!canvas || !inner) return
  const cw = canvas.offsetWidth
  const ch = canvas.offsetHeight
  const iw = inner.offsetWidth
  const ih = inner.offsetHeight
  translateX.value = Math.max(0, (cw - iw) / 2)
  translateY.value = Math.max(0, (ch - ih) / 2)
}

function onWheel(e: WheelEvent) {
  const canvas = canvasRef.value
  if (!canvas) return
  const z = zoomLevel.value
  const newZ = Math.min(2.5, Math.max(0.25, +(z + (e.deltaY < 0 ? 0.08 : -0.08)).toFixed(2)))
  const rect = canvas.getBoundingClientRect()
  const mx = e.clientX - rect.left
  const my = e.clientY - rect.top
  translateX.value = mx - (mx - translateX.value) / z * newZ
  translateY.value = my - (my - translateY.value) / z * newZ
  zoomLevel.value = newZ
}

function onDragStart(e: MouseEvent) {
  if (e.button !== 0) return
  isDragging.value = true
  dragStart = { x: e.clientX, y: e.clientY, tx: translateX.value, ty: translateY.value }
  ;(e.currentTarget as HTMLElement).style.cursor = 'grabbing'
}

function onDragMove(e: MouseEvent) {
  if (!isDragging.value) return
  translateX.value = dragStart.tx + (e.clientX - dragStart.x)
  translateY.value = dragStart.ty + (e.clientY - dragStart.y)
}

function onDragEnd(e: MouseEvent) {
  isDragging.value = false
  ;(e.currentTarget as HTMLElement).style.cursor = 'grab'
}

onMounted(() => {
  nextTick(centreGraph)
})
</script>
