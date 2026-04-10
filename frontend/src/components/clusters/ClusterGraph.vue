<template>
  <div class="cluster-graph">
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
      class="cluster-graph-canvas"
      @wheel.prevent="onWheel"
      @mousedown="onDragStart"
      @mousemove="onDragMove"
      @mouseup="onDragEnd"
      @mouseleave="onDragEnd"
      @click="clearHighlight"
    >
      <div
        ref="innerRef"
        class="cluster-graph-inner"
        :style="innerStyle"
      >
        <!-- Col 1: Git + Omni (collapsible, slides left) -->
        <Transition name="dag-col-left">
          <div v-if="!col1Folded" style="display:flex;align-items:flex-start">
            <div :style="col1WrapStyle">
              <!-- Git node -->
              <div class="dag-node-wrap">
                <div class="dag-node" style="width:220px">
                  <div class="dag-node-icon" v-html="gitIconSVG"></div>
                  <div class="dag-node-body">
                    <div class="dag-node-kind">Git</div>
                    <div class="dag-node-name" :title="gitRepoName">{{ gitRepoName }}</div>
                    <div v-if="gitMeta" class="dag-node-meta" v-html="gitMeta"></div>
                  </div>
                </div>
              </div>
              <!-- Omni node -->
              <div class="dag-node-wrap">
                <div class="dag-node" style="width:220px">
                  <div class="dag-node-icon" v-html="omniIconSVG"></div>
                  <div class="dag-node-body">
                    <div class="dag-node-kind">Omni</div>
                    <div class="dag-node-name">{{ omniEndpointDisplay || 'Omni Instance' }}</div>
                    <div v-if="appState?.omniVersion" class="dag-node-meta">
                      <span>{{ appState.omniVersion }}</span>
                    </div>
                  </div>
                </div>
              </div>
            </div>
            <!-- Edge col1 → col2 -->
            <svg :width="EW" :height="maxH" overflow="visible" style="flex-shrink:0;">
              <path
                v-for="(ly, i) in col1Ys"
                :key="i"
                :d="`M0,${ly.toFixed(1)} H${(EW*0.5).toFixed(1)} V${col2MidY.toFixed(1)} H${EW}`"
                stroke="#2c2e38" stroke-width="1.5" fill="none" stroke-dasharray="4,3"
              />
              <polygon
                v-for="(ly, i) in col1Ys"
                :key="'a'+i"
                :points="`${EW},${col2MidY.toFixed(1)} ${EW-5},${(col2MidY-3).toFixed(1)} ${EW-5},${(col2MidY+3).toFixed(1)}`"
                fill="#5b5c64"
              />
            </svg>
          </div>
        </Transition>

        <!-- Col 2: Cluster node -->
        <div :style="col2WrapStyle">
          <div class="dag-node-wrap">
            <!-- Left fold button -->
            <button class="dag-fold-btn-left" @click.stop="toggleFold('col1')">
              {{ col1Folded ? '+' : '−' }}
            </button>
            <div class="dag-node dag-node-left-btn" style="width:220px">
              <div class="dag-node-icon" v-html="talosIconSVG"></div>
              <div class="dag-node-body">
                <div class="dag-node-kind">Cluster</div>
                <div class="dag-node-name" :title="cluster.id">{{ cluster.id }}</div>
                <div class="dag-node-meta">
                  Talos {{ cluster.talosVersion || '—' }} &middot; K8s {{ cluster.kubernetesVersion || '—' }}
                </div>
              </div>
            </div>
            <!-- Right fold button -->
            <button v-if="nodeGroups.length > 0" class="dag-fold-btn" @click.stop="toggleFold('col3')">
              {{ col3Folded ? '+' : '−' }}
            </button>
          </div>
        </div>

        <!-- Col 3+: MachineSets, machines, extensions (slides right) -->
        <Transition name="dag-col-right">
          <div v-if="!col3Folded && nodeGroups.length > 0" style="display:flex;align-items:flex-start">
            <!-- Edge col2 → col3 -->
            <svg :width="EW" :height="maxH" overflow="visible" style="flex-shrink:0;">
              <g v-for="cc in col2Col3Connections" :key="cc.key">
                <path :d="cc.d" stroke="#2c2e38" stroke-width="1.5" fill="none" stroke-dasharray="4,3"/>
                <polygon :points="cc.arrowPoints" fill="#5b5c64"/>
              </g>
            </svg>

            <!-- Col 3: MachineSets, positioned absolutely -->
            <div :style="{ position: 'relative', width: `${NW}px`, height: `${maxH}px`, flexShrink: 0 }">
              <div
                v-for="(ng, i) in nodeGroups"
                :key="i"
                :style="{ position: 'absolute', top: `${Math.round(col3NodeYs[i] - NH/2)}px`, left: 0 }"
              >
                <div class="dag-node-wrap">
                  <div class="dag-node" :style="{ width: `${NW}px` }">
                    <div class="dag-node-icon" v-html="machinesetIconSVG"></div>
                    <div class="dag-node-body">
                      <div class="dag-node-kind">MachineSet</div>
                      <div class="dag-node-name">{{ ng.label }}</div>
                      <div class="dag-node-meta">
                        {{ ng.isPool ? ng.count : ng.machines.length }}
                        {{ (ng.isPool ? ng.count : ng.machines.length) !== 1 ? 'nodes' : 'node' }}
                        <span v-if="ng.machineClass"> &middot; {{ ng.machineClass }}</span>
                      </div>
                    </div>
                  </div>
                  <button
                    v-if="ng.machines.length > 0"
                    class="dag-fold-btn"
                    @click.stop="toggleFold(`col4:${i}`)"
                  >
                    {{ isGroupFolded(i) ? '+' : '−' }}
                  </button>
                </div>
              </div>
            </div>

            <!-- Edge col3 → col4: only when there are visible machines -->
            <template v-if="visibleMachines.length > 0">
              <svg :width="EW" :height="maxH" overflow="visible" style="flex-shrink:0;">
                <path v-for="mc in machineConnections" :key="`mp-${mc.uuid}`" :d="mc.d" stroke="#2c2e38" stroke-width="1.5" fill="none" stroke-dasharray="4,3"/>
                <polygon v-for="mc in machineConnections" :key="`ma-${mc.uuid}`" :points="mc.arrowPoints" fill="#5b5c64"/>
              </svg>

              <!-- Col 4: Individual machines, grouped (with intra-group gap) -->
              <div :style="{ display: 'flex', flexDirection: 'column', gap: '8px', marginTop: `${col4OffsetY}px`, transition: 'margin-top 0.3s ease' }">
                <TransitionGroup name="mach-fade" tag="div" style="display:contents">
                  <template v-for="(gi, gOrder) in visibleGroupOrder" :key="gi">
                    <div v-if="Number(gOrder) > 0" :style="{ height: `${INTER_GAP - 8}px`, flexShrink: 0 }"></div>
                    <div
                      v-for="machInfo in getMachinesForGroup(gi)"
                      :key="machInfo.uuid"
                      :id="`mn-${graphId}-${machInfo.flatIdx}`"
                      class="dag-node-wrap"
                      @click.stop="highlightMachine(machInfo.flatIdx)"
                    >
                      <div class="dag-node clickable" :class="{ 'ext-hl-selected': isMachineHighlighted(machInfo.flatIdx) }" :style="{ width: `${NW_MACH}px` }">
                        <div class="dag-node-icon" v-html="machineIconForGroup(machInfo.gi)"></div>
                        <div class="dag-node-body">
                          <div class="dag-node-kind">Machine</div>
                          <div class="dag-node-name" :style="{ fontFamily: 'SF Mono, Fira Code, monospace', fontSize: '11px' }" :title="machInfo.uuid">{{ machInfo.uuid }}</div>
                          <div v-if="machInfo.hostname" class="dag-node-meta">{{ machInfo.hostname }}</div>
                        </div>
                      </div>
                    </div>
                  </template>
                </TransitionGroup>
              </div>

              <!-- Edge col4 → extensions -->
              <template v-if="extEdgePaths.length > 0">
                <svg :width="EW" :height="maxH" style="flex-shrink:0;">
                  <template v-for="ep in extEdgePaths" :key="ep.key">
                    <path
                      :class="['ext-edge', extEdgeClassForConn(ep.srcIdx, ep.tgtIdx)]"
                      :d="ep.d"
                      stroke="#2c2e38" stroke-width="1.5" fill="none" stroke-dasharray="4,3"
                    />
                    <polygon
                      :class="['ext-arrow', extEdgeClassForConn(ep.srcIdx, ep.tgtIdx)]"
                      :points="ep.arrowPoints"
                      fill="#5b5c64"
                    />
                  </template>
                </svg>

                <!-- Extensions column -->
                <div :style="extColWrapStyle">
                  <div
                    v-for="(ext, ei) in extNodes"
                    :key="ei"
                    :id="`en-${graphId}-${ei}`"
                    class="dag-node-wrap"
                    @click.stop="highlightExt(ei)"
                  >
                    <div class="dag-node clickable" :class="{ 'ext-hl-selected': isExtHighlighted(ei) }" :style="{ width: `${NW}px` }">
                      <div class="dag-node-icon" v-html="extIconSVG"></div>
                      <div class="dag-node-body">
                        <div class="dag-node-kind">Extension</div>
                        <div class="dag-node-name">{{ ext }}</div>
                      </div>
                    </div>
                  </div>
                </div>
              </template>
            </template>

          </div>
        </Transition>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, ref, onMounted, nextTick } from 'vue'
import { useAppStore } from '@/stores/appStore'
import type { ResourceInfo } from '@/types'
import {
  gitIconSVG, omniIconSVG, talosIconSVG,
  machinesetIconSVG, machinesetPoolInfraIconSVG, machinesetPoolManualIconSVG,
  extIconSVG,
} from '@/assets/icons'

const props = defineProps<{
  cluster: ResourceInfo
}>()

const appStore = useAppStore()
const appState = computed(() => appStore.state)

// Layout constants
const NH = 100, NW = 220, NW_MACH = 330, EW = 60
const INTRA_GAP = 8, INTER_GAP = 32, COL1_GAP = 16

// Stable graph ID
const graphId = computed(() =>
  'g' + props.cluster.id.replace(/[^a-zA-Z0-9]/g, '_')
)

// Fold state — use ref with full object replacement so Vue reliably invalidates all dependents
const foldState = ref<Record<string, boolean>>({})

function toggleFold(key: string) {
  const fullKey = `${graphId.value}:${key}`
  foldState.value = { ...foldState.value, [fullKey]: !foldState.value[fullKey] }
}

function isGroupFolded(gi: number) {
  return !!foldState.value[`${graphId.value}:col4:${gi}`]
}

const col1Folded = computed(() => {
  const v = foldState.value[`${graphId.value}:col1`]
  return v === undefined ? true : v
})
const col3Folded = computed(() => !!foldState.value[`${graphId.value}:col3`])

function collapseAll() {
  const id = graphId.value
  const next = { ...foldState.value }
  next[`${id}:col1`] = true
  next[`${id}:col3`] = true
  nodeGroups.value.forEach((_, i) => { next[`${id}:col4:${i}`] = true })
  foldState.value = next
}

function expandAll() {
  const id = graphId.value
  const next: Record<string, boolean> = {}
  Object.keys(foldState.value).forEach(k => {
    next[k] = k.startsWith(id + ':') ? false : foldState.value[k]
  })
  foldState.value = next
}

// Git / Omni info
const gitInfo = computed(() => {
  const s = appState.value
  if (!s) return null
  const clusterRepoName = props.cluster.repoName
  if (clusterRepoName && s.repos) {
    return s.repos.find(r => r.name === clusterRepoName) || s.git
  }
  return s.git
})

const gitRepoName = computed(() =>
  props.cluster.repoName || gitInfo.value?.name || 'Repository'
)

const gitMeta = computed(() => {
  const g = gitInfo.value
  if (!g) return ''
  const parts: string[] = []
  if (g.branch) parts.push(g.branch)
  return parts.join(' &middot; ')
})

const omniEndpointDisplay = computed(() =>
  (appState.value?.omniEndpoint || '').replace(/^https?:\/\//, '')
)

const omniColor = computed(() => {
  const st = appState.value?.omniHealth?.status
  if (st === 'healthy') return '#fb923c'
  if (st === 'failed') return '#f87171'
  return '#7d7d85'
})


// Node groups
const nodeGroups = computed(() => {
  const c = props.cluster
  const cp = c.controlPlane || { count: 0, machines: [] }
  const workers = Array.isArray(c.workers) ? c.workers : (c.workers ? [c.workers] : [])

  const mcProvMap: Record<string, string> = {}
  ;(appState.value?.machineClasses || []).forEach(mc => {
    mcProvMap[mc.id] = mc.provisionType || 'manual'
  })

  const groups = [
    ...((cp.count || 0) > 0 || (cp.machines || []).length > 0 ? [{
      kind: 'MachineSet',
      label: cp.name || 'control-planes',
      machineClass: cp.machineClass || '',
      provisionType: cp.machineClass ? (mcProvMap[cp.machineClass] || 'manual') : '',
      machines: cp.machines || [],
      count: cp.count || 0,
      exts: cp.extensions || [],
      color: '#ff8b59',
      isPool: (cp.machines || []).length === 0,
    }] : []),
    ...workers
      .filter(wk => (wk.count || 0) > 0 || (wk.machines || []).length > 0)
      .map(wk => ({
        kind: 'MachineSet',
        label: wk.name || 'workers',
        machineClass: wk.machineClass || '',
        provisionType: wk.machineClass ? (mcProvMap[wk.machineClass] || 'manual') : '',
        machines: wk.machines || [],
        count: wk.count || 0,
        exts: wk.extensions || [],
        color: '#8b5cf6',
        isPool: (wk.machines || []).length === 0,
      })),
  ]
  return groups
})

function machineIconForGroup(gi: number): string {
  return nodeGroups.value[gi]?.provisionType === 'auto' ? machinesetPoolInfraIconSVG : machinesetPoolManualIconSVG
}

// Individual machines (flat list with group info)
interface MachineInfo {
  uuid: string
  hostname: string
  gi: number
  flatIdx: number
}

const allMachines = computed<MachineInfo[]>(() => {
  const result: MachineInfo[] = []
  const hostnames = props.cluster.machineHostnames || {}
  nodeGroups.value.forEach((g, gi) => {
    if (isGroupFolded(gi)) return
    g.machines.forEach(uuid => {
      result.push({ uuid, hostname: hostnames[uuid] || '', gi, flatIdx: result.length })
    })
  })
  return result
})

const visibleMachines = computed(() => allMachines.value)
const visibleGroupOrder = computed<number[]>(() => {
  const seen = new Set<number>()
  const order: number[] = []
  allMachines.value.forEach(m => {
    if (!seen.has(m.gi)) { seen.add(m.gi); order.push(m.gi) }
  })
  return order
})

function getMachinesForGroup(gi: number): MachineInfo[] {
  return allMachines.value.filter(m => m.gi === gi)
}

// Machine Y positions (relative to col4 top)
// Layout uses gap:INTRA_GAP on the flex container + a spacer div of (INTER_GAP-INTRA_GAP) between groups.
// Total inter-group spacing = INTRA_GAP + (INTER_GAP-INTRA_GAP) + INTRA_GAP = INTER_GAP + INTRA_GAP = 40px
const machYs = computed(() => {
  const ys: number[] = new Array(allMachines.value.length).fill(0)
  const order = visibleGroupOrder.value
  let yOff = 0
  order.forEach((gi, oi) => {
    if (oi > 0) yOff += INTRA_GAP + (INTER_GAP - INTRA_GAP) + INTRA_GAP  // gap + spacer + gap = 40px
    const machines = getMachinesForGroup(gi)
    machines.forEach((m, li) => {
      ys[m.flatIdx] = yOff + li * (NH + INTRA_GAP) + NH / 2
    })
    yOff += machines.length * NH + Math.max(0, machines.length - 1) * INTRA_GAP
  })
  return ys
})

const col4TotalH = computed(() => {
  const order = visibleGroupOrder.value
  if (order.length === 0) return 0
  let total = 0
  order.forEach((gi, oi) => {
    if (oi > 0) total += INTRA_GAP + (INTER_GAP - INTRA_GAP) + INTRA_GAP  // 40px inter-group
    const size = getMachinesForGroup(gi).length
    total += size * NH + Math.max(0, size - 1) * INTRA_GAP
  })
  return total
})


// Col2→col3 connections — pre-compute full path data so Vue tracks all reactive deps in a computed getter
interface Col2Conn { key: string; d: string; arrowPoints: string }
const col2Col3Connections = computed<Col2Conn[]>(() => {
  const hw = (EW * 0.5).toFixed(1)
  const right = EW.toString()
  const ys = col3NodeYs.value
  const midY = col2MidY.value.toFixed(1)
  return nodeGroups.value.map((_, i) => {
    const y = ys[i].toFixed(1)
    return {
      key: String(i),
      d: `M0,${midY} H${hw} V${y} H${right}`,
      arrowPoints: `${right},${y} ${EW-5},${(ys[i]-3).toFixed(1)} ${EW-5},${(ys[i]+3).toFixed(1)}`,
    }
  })
})

// Machine connections for edges — pre-compute full path data so Vue tracks all reactive deps in a computed getter
interface MachConn { uuid: string; d: string; arrowPoints: string }
const machineConnections = computed<MachConn[]>(() => {
  const hw = EW * 0.5
  return allMachines.value.map(m => {
    const y3 = col3NodeYs.value[m.gi]
    const y4 = machYs.value[m.flatIdx] + col4OffsetY.value
    return {
      uuid: m.uuid,
      d: `M0,${y3.toFixed(1)} H${hw.toFixed(1)} V${y4.toFixed(1)} H${EW}`,
      arrowPoints: `${EW},${y4.toFixed(1)} ${EW-5},${(y4-3).toFixed(1)} ${EW-5},${(y4+3).toFixed(1)}`,
    }
  })
})

// Extensions — only include extensions that have connections to currently visible machines
interface ExtConn { srcIdx: number; tgtIdx: number }

const extData = computed<{ nodes: string[]; conns: ExtConn[] }>(() => {
  const c = props.cluster
  const clusterExts = c.clusterExtensions || []
  const machExtMap = c.machineExtensions || {}

  // Collect raw connections from visible machines only
  const raw: Array<{ srcMachIdx: number; extName: string }> = []
  allMachines.value.forEach(m => {
    clusterExts.forEach(ext => raw.push({ srcMachIdx: m.flatIdx, extName: ext }))
    ;(machExtMap[m.uuid] || []).forEach(ext => raw.push({ srcMachIdx: m.flatIdx, extName: ext }))
  })
  nodeGroups.value.forEach((g, gi) => {
    g.exts.forEach(ext => {
      getMachinesForGroup(gi).forEach(m => raw.push({ srcMachIdx: m.flatIdx, extName: ext }))
    })
  })

  // Only include extensions that actually appear in connections
  const extNameSet = new Set(raw.map(r => r.extName))
  const nodes = [...extNameSet].sort()
  const nameToIdx: Record<string, number> = {}
  nodes.forEach((n, i) => { nameToIdx[n] = i })

  // Deduplicate connections
  const seen = new Set<string>()
  const conns: ExtConn[] = []
  raw.forEach(({ srcMachIdx, extName }) => {
    const tgtIdx = nameToIdx[extName]
    const key = `${srcMachIdx}:${tgtIdx}`
    if (!seen.has(key)) { seen.add(key); conns.push({ srcIdx: srcMachIdx, tgtIdx }) }
  })

  return { nodes, conns }
})

const extNodes = computed(() => extData.value.nodes)
const extConns = computed(() => extData.value.conns)

// Highlight state
const highlightedMachine = ref<number | null>(null)
const highlightedExt = ref<number | null>(null)

function clearHighlight() {
  highlightedMachine.value = null
  highlightedExt.value = null
}

function highlightMachine(mi: number) {
  if (highlightedMachine.value === mi) { clearHighlight(); return }
  highlightedMachine.value = mi
  highlightedExt.value = null
}

function highlightExt(ei: number) {
  if (highlightedExt.value === ei) { clearHighlight(); return }
  highlightedExt.value = ei
  highlightedMachine.value = null
}

function isMachineHighlighted(flatIdx: number): boolean {
  if (highlightedMachine.value === flatIdx) return true
  if (highlightedExt.value !== null) {
    return extConns.value.some(c => c.srcIdx === flatIdx && c.tgtIdx === highlightedExt.value)
  }
  return false
}

function isExtHighlighted(ei: number): boolean {
  if (highlightedExt.value === ei) return true
  if (highlightedMachine.value !== null) {
    return extConns.value.some(c => c.srcIdx === highlightedMachine.value && c.tgtIdx === ei)
  }
  return false
}

function extEdgeClassForConn(srcIdx: number, tgtIdx: number): string {
  if (highlightedMachine.value !== null) {
    return srcIdx === highlightedMachine.value ? 'ext-hl-active' : 'ext-hl-faded'
  }
  if (highlightedExt.value !== null) {
    return tgtIdx === highlightedExt.value ? 'ext-hl-active' : 'ext-hl-faded'
  }
  return ''
}

// Pre-compute extension edge path data so Vue tracks all reactive deps in a computed getter
interface ExtEdgePath { key: string; d: string; arrowPoints: string; srcIdx: number; tgtIdx: number }
const extEdgePaths = computed<ExtEdgePath[]>(() => {
  const n = extNodes.value.length
  const hw = EW * 0.5
  return extConns.value.map(conn => {
    const y4 = machYs.value[conn.srcIdx] + col4OffsetY.value
    const yExt = extMidY(n, conn.tgtIdx)
    return {
      key: `${conn.srcIdx}-${conn.tgtIdx}`,
      d: `M0,${y4.toFixed(1)} H${hw.toFixed(1)} V${yExt.toFixed(1)} H${EW}`,
      arrowPoints: `${EW},${yExt.toFixed(1)} ${EW-5},${(yExt-3).toFixed(1)} ${EW-5},${(yExt+3).toFixed(1)}`,
      srcIdx: conn.srcIdx,
      tgtIdx: conn.tgtIdx,
    }
  })
})

// Layout calculations
function colH(n: number) { return n * NH + Math.max(0, n - 1) * 12 }

const maxH = computed(() => {
  const counts = [2, 1]  // col1 (2 nodes), col2 (1 node)
  if (!col3Folded.value) {
    counts.push(nodeGroups.value.length)
    if (extNodes.value.length > 0) counts.push(extNodes.value.length)
  }
  let h = Math.max(...counts.map(colH), NH)
  if (!col3Folded.value && col4TotalH.value > 0) {
    h = Math.max(h, col4TotalH.value)
    // Ensure col3 nodes with min-spacing (NH+12) fit even when col4OffsetY shifts them down.
    // Derived from: col4OffsetY + NH/2 + (n-1)*minSpacing ≤ h - NH/2
    // → h ≥ 2*NH + 2*(n-1)*minSpacing - col4TotalH
    const n = nodeGroups.value.length
    if (n > 1) h = Math.max(h, 2 * NH + 2 * (n - 1) * (NH + 12) - col4TotalH.value)
  }
  return h
})

const col4OffsetY = computed(() =>
  Math.round(Math.max(0, (maxH.value - col4TotalH.value) / 2))
)

function spreadMidY(n: number, idx: number): number {
  if (n <= 1) return maxH.value / 2
  return idx * (maxH.value - NH) / (n - 1) + NH / 2
}

const col2MidY = computed(() => spreadMidY(1, 0))

const col3NodeYs = computed(() => {
  const ys = nodeGroups.value.map((g, i) => {
    if (!isGroupFolded(i) && g.machines.length > 0) {
      // Align to centre of machine group
      const groupMachines = getMachinesForGroup(i)
      if (groupMachines.length > 0) {
        const firstMach = groupMachines[0]
        const lastMach = groupMachines[groupMachines.length - 1]
        const firstY = machYs.value[firstMach.flatIdx] + col4OffsetY.value
        const lastY = machYs.value[lastMach.flatIdx] + col4OffsetY.value
        return (firstY + lastY) / 2
      }
    }
    return spreadMidY(nodeGroups.value.length, i)
  })
  // Enforce minimum spacing so nodes never overlap
  const minSpacing = NH + 12
  for (let i = 1; i < ys.length; i++) {
    if (ys[i] - ys[i - 1] < minSpacing) ys[i] = ys[i - 1] + minSpacing
  }
  return ys
})

// Col1 Y positions
const col1Ys = computed(() => {
  const n = 2
  const totalH = n * NH + (n - 1) * COL1_GAP
  const startY = Math.max(0, (maxH.value - totalH) / 2)
  return [0, 1].map(i => startY + i * (NH + COL1_GAP) + NH / 2)
})

// Wrap styles
const col1WrapStyle = computed(() => {
  const totalH = 2 * NH + COL1_GAP
  const mt = Math.round(Math.max(0, (maxH.value - totalH) / 2))
  return {
    display: 'flex',
    flexDirection: 'column' as const,
    gap: `${COL1_GAP}px`,
    marginTop: `${mt}px`,
  }
})

const col2WrapStyle = computed(() => {
  const mt = Math.round((maxH.value - NH) / 2)
  return {
    display: 'flex',
    flexDirection: 'column' as const,
    marginTop: `${mt}px`,
  }
})

// Extension Y position (fixed INTRA_GAP spacing, centred in maxH)
function extMidY(n: number, idx: number): number {
  const totalH = n * NH + Math.max(0, n - 1) * INTRA_GAP
  const startY = Math.max(0, (maxH.value - totalH) / 2)
  return startY + idx * (NH + INTRA_GAP) + NH / 2
}

const extColWrapStyle = computed(() => {
  const n = extNodes.value.length
  const totalH = n * NH + Math.max(0, n - 1) * INTRA_GAP
  const mt = Math.round(Math.max(0, (maxH.value - totalH) / 2))
  return { display: 'flex', flexDirection: 'column' as const, gap: `${INTRA_GAP}px`, marginTop: `${mt}px` }
})

// Pan/zoom
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
  // Default col1 to folded
  const id = graphId.value
  if (foldState.value[`${id}:col1`] === undefined) {
    foldState.value = { ...foldState.value, [`${id}:col1`]: true }
  }
  nextTick(centreGraph)
})
</script>
