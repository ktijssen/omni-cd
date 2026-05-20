<template>
  <div class="container">
    <div class="header" style="border-bottom:none;flex-direction:column;align-items:stretch;gap:8px;">
      <h1 style="font-size:18px;font-weight:600;color:#fff;letter-spacing:-0.3px;white-space:nowrap;">Clusters</h1>
      <div style="display:flex;align-items:center;gap:8px;">
        <template v-if="authStore.isAdmin()">
          <span v-if="isRunning" class="spinner"></span>
          <button class="btn-omni" :disabled="isRunning" @click="openSelectModal('refresh')">Refresh</button>
          <button class="btn-omni" :disabled="isRunning" @click="openSelectModal('sync')">Sync</button>
        </template>
        <input
          v-model="clusterSearch"
          type="text"
          placeholder="Search clusters..."
          style="background:#1e2130;border:1px solid #3d4059;border-radius:4px;color:#c4c4c9;font-size:13px;padding:6px 12px;outline:none;width:200px;font-family:inherit;transition:border-color 0.2s;"
          @focus="($event.target as HTMLInputElement).style.borderColor='#ff8b59'"
          @blur="($event.target as HTMLInputElement).style.borderColor='#3d4059'"
        />
        <div class="page-size-bar" style="margin-left:auto;">
          <button class="btn-omni" :class="{ active: viewMode === 'grid' }" @click="viewMode = 'grid'" title="Tile view">⊞</button>
          <button class="btn-omni" :class="{ active: viewMode === 'list' }" @click="viewMode = 'list'" title="List view">☰</button>
        </div>
      </div>
    </div>

    <!-- Omni not configured -->
    <div v-if="!state?.omniConfigured" class="placeholder-page">
      <div class="placeholder-icon">🔌</div>
      <div class="placeholder-title">Please configure an Omni Instance first in Settings/Instances</div>
    </div>

    <!-- Clusters disabled -->
    <div v-else-if="!state?.clustersEnabled" class="placeholder-page">
      <div class="placeholder-icon">⏸</div>
      <div class="placeholder-title">Clusters management disabled</div>
      <div class="placeholder-sub">Enable clusters management in the configuration.</div>
    </div>

    <template v-else>
      <!-- Health bar -->
      <div v-if="healthTotal > 0" class="cluster-health-bar-wrap">
        <div class="cluster-health-bar" :class="{ 'has-filter': clusterStatusFilter !== null }">
          <div
            v-if="countReady"
            class="cluster-health-bar-seg cluster-health-bar-seg--ready"
            :class="{ active: clusterStatusFilter === 'ready' }"
            :style="{ width: (countReady / healthTotal * 100).toFixed(1) + '%' }"
            :title="countReady + ' ready'"
            @click="setClusterFilter('ready')"
          ></div>
          <div
            v-if="countNotReady"
            class="cluster-health-bar-seg cluster-health-bar-seg--notready"
            :class="{ active: clusterStatusFilter === 'not-ready' }"
            :style="{ width: (countNotReady / healthTotal * 100).toFixed(1) + '%' }"
            :title="countNotReady + ' not ready'"
            @click="setClusterFilter('not-ready')"
          ></div>
          <div
            v-if="countScalingUp"
            class="cluster-health-bar-seg cluster-health-bar-seg--scalingup"
            :class="{ active: clusterStatusFilter === 'scaling-up' }"
            :style="{ width: (countScalingUp / healthTotal * 100).toFixed(1) + '%' }"
            :title="countScalingUp + ' scaling up'"
            @click="setClusterFilter('scaling-up')"
          ></div>
          <div
            v-if="countScalingDown"
            class="cluster-health-bar-seg cluster-health-bar-seg--scalingdown"
            :class="{ active: clusterStatusFilter === 'scaling-down' }"
            :style="{ width: (countScalingDown / healthTotal * 100).toFixed(1) + '%' }"
            :title="countScalingDown + ' scaling down'"
            @click="setClusterFilter('scaling-down')"
          ></div>
          <div
            v-if="countDestroying"
            class="cluster-health-bar-seg cluster-health-bar-seg--destroying"
            :class="{ active: clusterStatusFilter === 'destroying' }"
            :style="{ width: (countDestroying / healthTotal * 100).toFixed(1) + '%' }"
            :title="countDestroying + ' destroying'"
            @click="setClusterFilter('destroying')"
          ></div>
          <div
            v-if="countReconfiguring"
            class="cluster-health-bar-seg cluster-health-bar-seg--reconfiguring"
            :class="{ active: clusterStatusFilter === 'reconfiguring' }"
            :style="{ width: (countReconfiguring / healthTotal * 100).toFixed(1) + '%' }"
            :title="countReconfiguring + ' reconfiguring'"
            @click="setClusterFilter('reconfiguring')"
          ></div>
        </div>
        <div class="cluster-health-summary">
          {{ allClusters.length }} clusters
          <template v-if="countReady">
            &nbsp;·&nbsp;<span
              style="color:#4ade80;cursor:pointer"
              :style="{ fontWeight: clusterStatusFilter === 'ready' ? '700' : 'normal' }"
              @click="setClusterFilter('ready')"
            >{{ countReady }} Ready</span>
          </template>
          <template v-if="countNotReady">
            &nbsp;·&nbsp;<span
              style="color:#f87171;cursor:pointer"
              :style="{ fontWeight: clusterStatusFilter === 'not-ready' ? '700' : 'normal' }"
              @click="setClusterFilter('not-ready')"
            >{{ countNotReady }} Not Ready</span>
          </template>
          <template v-if="countScalingUp">
            &nbsp;·&nbsp;<span
              style="color:#60a5fa;cursor:pointer"
              :style="{ fontWeight: clusterStatusFilter === 'scaling-up' ? '700' : 'normal' }"
              @click="setClusterFilter('scaling-up')"
            >{{ countScalingUp }} Scaling Up</span>
          </template>
          <template v-if="countScalingDown">
            &nbsp;·&nbsp;<span
              style="color:#f59e0b;cursor:pointer"
              :style="{ fontWeight: clusterStatusFilter === 'scaling-down' ? '700' : 'normal' }"
              @click="setClusterFilter('scaling-down')"
            >{{ countScalingDown }} Scaling Down</span>
          </template>
          <template v-if="countDestroying">
            &nbsp;·&nbsp;<span
              style="color:#f43f5e;cursor:pointer"
              :style="{ fontWeight: clusterStatusFilter === 'destroying' ? '700' : 'normal' }"
              @click="setClusterFilter('destroying')"
            >{{ countDestroying }} Destroying</span>
          </template>
          <template v-if="countReconfiguring">
            &nbsp;·&nbsp;<span
              style="color:#a78bfa;cursor:pointer"
              :style="{ fontWeight: clusterStatusFilter === 'reconfiguring' ? '700' : 'normal' }"
              @click="setClusterFilter('reconfiguring')"
            >{{ countReconfiguring }} Reconfiguring</span>
          </template>
          <template v-if="clusterStatusFilter !== null">
            &nbsp;<span style="cursor:pointer;color:#9fa1a6;text-decoration:underline;font-size:11px" @click="clearClusterFilter">clear</span>
          </template>
        </div>
      </div>

      <!-- Toolbar: sort + filter + page size -->
      <div style="display:flex;align-items:center;gap:8px;padding:0 0 12px;flex-wrap:wrap;">
        <div style="margin-left:auto;display:flex;align-items:center;gap:8px">
          <div class="filter-dropdown-wrap">
            <button class="filter-select-btn" :class="{ active: activeSyncFilterCount > 0 }" @click="activeDropdown = activeDropdown === 'filter' ? null : 'filter'">
              <span class="filter-select-label">
                <label>Status</label>
                <span>{{ activeSyncFilterCount > 0 ? activeSyncFilterLabels : 'All' }}</span>
              </span>
              <svg width="16" height="16" viewBox="0 0 16 16" fill="currentColor" style="flex-shrink:0;transition:transform 0.2s" :style="{ transform: activeDropdown === 'filter' ? 'rotate(180deg)' : 'none' }"><path fill-rule="evenodd" clip-rule="evenodd" d="M3.14645 5.14645C3.34171 4.95118 3.65829 4.95118 3.85355 5.14645L8 9.29289L12.1464 5.14645C12.3417 4.95118 12.6583 4.95118 12.8536 5.14645C13.0488 5.34171 13.0488 5.65829 12.8536 5.85355L8.35355 10.3536C8.15829 10.5488 7.84171 10.5488 7.64645 10.3536L3.14645 5.85355C2.95118 5.65829 2.95118 5.34171 3.14645 5.14645Z"/></svg>
            </button>
            <div v-if="activeDropdown === 'filter'" class="cluster-list-menu" style="min-width:180px;right:0;left:auto;">
              <div class="cluster-list-menu-section-label">Sync Status</div>
              <button
                v-for="def in visibleSyncDefs"
                :key="def.key"
                class="cluster-list-menu-item"
                :class="{ active: !!clusterSyncFilters[def.key] }"
                @click="toggleSyncFilter(def.key)"
              >{{ !!clusterSyncFilters[def.key] ? '✓ ' : '\u00a0\u00a0 ' }}{{ def.label }}</button>
              <div class="cluster-list-menu-divider"></div>
              <div class="cluster-list-menu-section-label">Health</div>
              <button
                v-for="def in healthDefs"
                :key="def.key"
                class="cluster-list-menu-item"
                :class="{ active: clusterStatusFilter === def.key }"
                @click="setClusterFilter(def.key)"
              >{{ clusterStatusFilter === def.key ? '✓ ' : '\u00a0\u00a0 ' }}{{ def.label }}</button>
              <div v-if="activeSyncFilterCount > 0" class="cluster-list-menu-divider"></div>
              <button v-if="activeSyncFilterCount > 0" class="cluster-list-menu-item" style="color:#9fa1a6" @click="clearSyncFilters(); activeDropdown = null">Clear filters</button>
            </div>
          </div>
          <div class="filter-dropdown-wrap">
            <button class="filter-select-btn" @click="activeDropdown = activeDropdown === 'sort' ? null : 'sort'">
              <span class="filter-select-label">
                <label>Sort</label>
                <span>{{ sortLabel }}</span>
              </span>
              <svg width="16" height="16" viewBox="0 0 16 16" fill="currentColor" style="flex-shrink:0;transition:transform 0.2s" :style="{ transform: activeDropdown === 'sort' ? 'rotate(180deg)' : 'none' }"><path fill-rule="evenodd" clip-rule="evenodd" d="M3.14645 5.14645C3.34171 4.95118 3.65829 4.95118 3.85355 5.14645L8 9.29289L12.1464 5.14645C12.3417 4.95118 12.6583 4.95118 12.8536 5.14645C13.0488 5.34171 13.0488 5.65829 12.8536 5.85355L8.35355 10.3536C8.15829 10.5488 7.84171 10.5488 7.64645 10.3536L3.14645 5.85355C2.95118 5.65829 2.95118 5.34171 3.14645 5.14645Z"/></svg>
            </button>
            <div v-if="activeDropdown === 'sort'" class="cluster-list-menu" style="min-width:180px;right:0;left:auto;">
              <button
                v-for="def in sortDefs"
                :key="def.key"
                class="cluster-list-menu-item"
                :class="{ active: sortKey === def.key }"
                @click="setSort(def.key)"
              >{{ sortKey === def.key ? '✓ ' : '\u00a0\u00a0 ' }}{{ def.label }}</button>
            </div>
          </div>
          <div class="filter-dropdown-wrap">
            <button class="filter-select-btn" @click="activeDropdown = activeDropdown === 'pageSize' ? null : 'pageSize'">
              <span class="filter-select-label">
                <label>Show</label>
                <span>{{ pageSize === 0 ? 'All' : pageSize }}</span>
              </span>
              <svg width="16" height="16" viewBox="0 0 16 16" fill="currentColor" style="flex-shrink:0;transition:transform 0.2s" :style="{ transform: activeDropdown === 'pageSize' ? 'rotate(180deg)' : 'none' }"><path fill-rule="evenodd" clip-rule="evenodd" d="M3.14645 5.14645C3.34171 4.95118 3.65829 4.95118 3.85355 5.14645L8 9.29289L12.1464 5.14645C12.3417 4.95118 12.6583 4.95118 12.8536 5.14645C13.0488 5.34171 13.0488 5.65829 12.8536 5.85355L8.35355 10.3536C8.15829 10.5488 7.84171 10.5488 7.64645 10.3536L3.14645 5.85355C2.95118 5.65829 2.95118 5.34171 3.14645 5.14645Z"/></svg>
            </button>
            <div v-if="activeDropdown === 'pageSize'" class="cluster-list-menu" style="min-width:100px;right:0;left:auto;">
              <button
                v-for="n in [5, 10, 15, 20, 0]"
                :key="n"
                class="cluster-list-menu-item"
                :class="{ active: pageSize === n }"
                @click="setPageSize(n); activeDropdown = null"
              >{{ pageSize === n ? '✓ ' : '\u00a0\u00a0 ' }}{{ n === 0 ? 'All' : n }}</button>
            </div>
          </div>
        </div>
      </div>

      <!-- No clusters -->
      <div v-if="displayClusters.length === 0 && allClusters.length === 0" class="placeholder-page">
        <div class="placeholder-title">No clusters found</div>
        <div class="placeholder-sub">Clusters defined in your git repo will appear here.</div>
      </div>
      <div v-else-if="displayClusters.length === 0" style="padding:24px;color:#5b5c64">No clusters match the current filters</div>

      <template v-else>
        <!-- Cluster grid -->
        <div v-if="viewMode === 'grid'" class="cluster-grid">
          <div
            v-for="cluster in pageClusters"
            :key="cluster.id"
            class="cluster-card clickable"
            :data-status="cluster.status || 'idle'"
            v-bind="clusterCardAttrs(cluster)"
            @click="goToCluster(cluster.id)"
          >
            <div class="cluster-card-accent"></div>
            <div class="cluster-card-body">
              <!-- Header -->
              <div class="cluster-card-header">
                <div style="display:flex;align-items:center;gap:6px;min-width:0;flex-wrap:wrap;">
                  <span class="cluster-card-title">{{ cluster.id }}</span>
                  <a
                    v-if="state?.omniEndpoint"
                    class="btn-omni"
                    style="font-size:11px;padding:1px 6px;text-decoration:none"
                    :href="omniClusterUrl(cluster.id)"
                    target="_blank"
                    @click.stop
                    title="Open in Omni"
                  >&#8599; Open in Omni</a>
                </div>
                <span class="cluster-card-status" :style="{ color: mgmtColor(cluster), flexShrink: '0' }">{{ mgmtBadge(cluster) }}</span>
              </div>

              <div class="cluster-card-divider"></div>

              <!-- Meta grid -->
              <div class="cluster-card-meta">
                <div class="cluster-card-meta-pair">
                  <span class="cluster-card-meta-label">Sync Status:</span>
                  <span class="cluster-card-meta-value" :style="{ color: syncColor(cluster) }" v-html="syncText(cluster)"></span>
                </div>
                <div class="cluster-card-meta-pair">
                  <span class="cluster-card-meta-label">Cluster Health:</span>
                  <span class="cluster-card-meta-value" :style="{ color: healthColor(cluster) }" v-html="healthText(cluster)"></span>
                </div>
                <div class="cluster-card-meta-pair">
                  <span class="cluster-card-meta-label">Talos Version:</span>
                  <span class="cluster-card-meta-value">
                    <span v-if="cluster.talosVersion">{{ cluster.talosVersion }}</span>
                    <span v-else style="color:#5b5c64">—</span>
                  </span>
                </div>
                <div class="cluster-card-meta-pair">
                  <span class="cluster-card-meta-label">Kubernetes Version:</span>
                  <span class="cluster-card-meta-value">
                    <span v-if="cluster.kubernetesVersion">{{ cluster.kubernetesVersion }}</span>
                    <span v-else style="color:#5b5c64">—</span>
                  </span>
                </div>
                <div class="cluster-card-meta-pair">
                  <span class="cluster-card-meta-label">Repository:</span>
                  <span class="cluster-card-meta-value">
                    <span v-if="cluster.repoName">{{ cluster.repoName }}</span>
                    <span v-else style="color:#5b5c64">—</span>
                  </span>
                </div>
                <div class="cluster-card-meta-pair">
                  <span class="cluster-card-meta-label">Branch:</span>
                  <span class="cluster-card-meta-value">
                    <span v-if="clusterRepo(cluster)?.branch">{{ clusterRepo(cluster)!.branch }}</span>
                    <span v-else style="color:#5b5c64">—</span>
                  </span>
                </div>
                <div class="cluster-card-meta-pair">
                  <span class="cluster-card-meta-label">Created At:</span>
                  <span class="cluster-card-meta-value">
                    <span v-if="!isZeroTime(cluster.createdAt)">
                      {{ fmtDateTime(cluster.createdAt) }}
                      <span style="color:#7d7d85">({{ ago(cluster.createdAt) }})</span>
                    </span>
                    <span v-else style="color:#5b5c64">—</span>
                  </span>
                </div>
                <div class="cluster-card-meta-pair">
                  <span class="cluster-card-meta-label">Last Sync:</span>
                  <span class="cluster-card-meta-value">
                    <span v-if="!isZeroTime(cluster.lastSyncTime)">
                      {{ fmtDateTime(cluster.lastSyncTime) }}
                      <span style="color:#7d7d85">({{ ago(cluster.lastSyncTime) }})</span>
                    </span>
                    <span v-else style="color:#5b5c64">—</span>
                  </span>
                </div>
              </div>

              <div class="cluster-card-divider"></div>

              <!-- Pool rows -->
              <div
                v-for="(sec, idx) in clusterSections(cluster)"
                :key="idx"
                class="cluster-pool-row"
              >
                <div class="cluster-pool-row-label">{{ sec.label }}</div>
                <div class="cluster-pool-row-count">{{ sec.count }}</div>
                <div class="cluster-pool-row-mc">
                  <span v-if="sec.mc">{{ sec.mc }}</span>
                  <span v-else style="color:#2c2e38">—</span>
                </div>
              </div>

              <div class="cluster-card-divider" style="margin-top:8px"></div>

              <!-- Actions -->
              <div v-if="authStore.isAdmin()" class="cluster-card-actions" @click.stop>
                <button
                  class="btn-omni"
                  :disabled="!!clusterActionPending[cluster.id]"
                  @click="refreshCluster(cluster)"
                  title="Re-read live state from Omni"
                >↺ {{ clusterActionPending[cluster.id] === 'refresh' ? 'Refreshing...' : 'Refresh' }}</button>
                <button
                  v-if="cluster.status !== 'deleting' && (cluster.status === 'unmanaged' || cluster.status === 'orphaned')"
                  class="btn-omni"
                  @click="exportCluster(cluster)"
                  title="Export cluster as YAML template"
                >↓ Export</button>
                <button
                  v-if="cluster.status !== 'deleting' && cluster.status !== 'unmanaged' && cluster.status !== 'orphaned'"
                  class="btn-omni"
                  :disabled="!!clusterActionPending[cluster.id]"
                  @click="syncCluster(cluster)"
                  title="Force sync this cluster from Git"
                >⇅ {{ clusterActionPending[cluster.id] === 'sync' ? 'Syncing...' : 'Sync' }}</button>
                <button
                  v-if="cluster.status !== 'deleting' && cluster.status !== 'unmanaged' && cluster.status !== 'orphaned'"
                  class="btn-omni auto-sync"
                  :class="{ active: cluster.autoSync !== false }"
                  @click="toggleAutoSync(cluster)"
                  title="Toggle per-cluster auto sync"
                >{{ cluster.autoSync === false ? '○ Auto-Sync: Off' : '● Auto-Sync: On' }}</button>
                <button
                  v-if="cluster.status !== 'deleting'"
                  class="btn-omni"
                  @click="deleteCluster(cluster)"
                  title="Delete this cluster from Omni"
                >✕ Delete</button>
              </div>
            </div>
          </div>
        </div>

        <!-- Cluster list -->
        <table v-else class="cluster-list-table">
          <thead>
            <tr>
              <th>Name</th>
              <th>Status</th>
              <th>Sync</th>
              <th>Health</th>
              <th>Talos</th>
              <th>Kubernetes</th>
              <th>Repo</th>
              <th>Last Sync</th>
              <th v-if="authStore.isAdmin()"></th>
            </tr>
          </thead>
          <tbody>
            <tr
              v-for="cluster in pageClusters"
              :key="cluster.id"
              class="cluster-list-row"
              :data-status="cluster.status || 'idle'"
              @click="goToCluster(cluster.id)"
            >
              <td class="cluster-list-name">
                {{ cluster.id }}
                <a
                  v-if="state?.omniEndpoint"
                  class="btn-omni"
                  style="font-size:11px;padding:1px 6px;text-decoration:none;margin-left:6px"
                  :href="omniClusterUrl(cluster.id)"
                  target="_blank"
                  @click.stop
                  title="Open in Omni"
                >&#8599;</a>
              </td>
              <td :style="{ color: mgmtColor(cluster) }">{{ mgmtBadge(cluster) }}</td>
              <td :style="{ color: syncColor(cluster) }" v-html="syncText(cluster)"></td>
              <td :style="{ color: healthColor(cluster) }" v-html="healthText(cluster)"></td>
              <td style="color:#c4c4c9">{{ cluster.talosVersion || '—' }}</td>
              <td style="color:#c4c4c9">{{ cluster.kubernetesVersion || '—' }}</td>
              <td style="color:#9fa1a6">{{ cluster.repoName || '—' }}</td>
              <td style="color:#9fa1a6">
                <span v-if="!isZeroTime(cluster.lastSyncTime)" :title="fmtDateTime(cluster.lastSyncTime)">{{ ago(cluster.lastSyncTime) }}</span>
                <span v-else>—</span>
              </td>
              <td v-if="authStore.isAdmin()" class="cluster-list-actions" @click.stop>
                <div class="cluster-list-menu-wrap">
                  <button class="cluster-list-menu-btn" @click="openMenuId = openMenuId === cluster.id ? null : cluster.id">⋮</button>
                  <div v-if="openMenuId === cluster.id" class="cluster-list-menu">
                    <button
                      class="cluster-list-menu-item"
                      :disabled="!!clusterActionPending[cluster.id]"
                      @click="refreshCluster(cluster); openMenuId = null"
                    >↺ {{ clusterActionPending[cluster.id] === 'refresh' ? 'Refreshing...' : 'Refresh' }}</button>
                    <button
                      v-if="cluster.status !== 'deleting' && (cluster.status === 'unmanaged' || cluster.status === 'orphaned')"
                      class="cluster-list-menu-item"
                      @click="exportCluster(cluster); openMenuId = null"
                    >↓ Export</button>
                    <button
                      v-if="cluster.status !== 'deleting' && cluster.status !== 'unmanaged' && cluster.status !== 'orphaned'"
                      class="cluster-list-menu-item"
                      :disabled="!!clusterActionPending[cluster.id]"
                      @click="syncCluster(cluster); openMenuId = null"
                    >⇅ {{ clusterActionPending[cluster.id] === 'sync' ? 'Syncing...' : 'Sync' }}</button>
                    <button
                      v-if="cluster.status !== 'deleting' && cluster.status !== 'unmanaged' && cluster.status !== 'orphaned'"
                      class="cluster-list-menu-item"
                      :class="{ active: cluster.autoSync !== false }"
                      @click="toggleAutoSync(cluster); openMenuId = null"
                    >{{ cluster.autoSync === false ? '○ Auto-Sync: Off' : '● Auto-Sync: On' }}</button>
                    <div v-if="cluster.status !== 'deleting'" class="cluster-list-menu-divider"></div>
                    <button
                      v-if="cluster.status !== 'deleting'"
                      class="cluster-list-menu-item danger"
                      @click="deleteCluster(cluster); openMenuId = null"
                    >✕ Delete</button>
                  </div>
                </div>
              </td>
            </tr>
          </tbody>
        </table>

        <!-- Pagination -->
        <div v-if="pageSize > 0 && displayClusters.length > pageSize" class="pagination">
          <button class="page-btn" :disabled="currentPage === 1" @click="currentPage--">&laquo;</button>
          <button
            v-for="p in totalPages"
            :key="p"
            class="page-btn"
            :class="{ active: p === currentPage }"
            @click="currentPage = p"
          >{{ p }}</button>
          <button class="page-btn" :disabled="currentPage === totalPages" @click="currentPage++">&raquo;</button>
        </div>
      </template>
    </template>

    <!-- Select modal (Refresh / Sync) -->
    <div v-if="selectModal" class="modal show" @click.self="selectModal = null">
      <div class="modal-content confirm-modal" style="width:520px;max-height:70vh;display:flex;flex-direction:column;" @click.stop>
        <div class="modal-header">
          <div class="modal-title">
            {{ selectModal.type === 'refresh' ? 'Refresh Clusters' : 'Sync Clusters' }}
          </div>
          <button class="modal-close" @click="selectModal = null">&times;</button>
        </div>
        <div class="modal-body" style="padding:16px 20px;overflow-y:auto;flex:1;font-family:inherit;white-space:normal;word-break:normal;">
          <div style="display:flex;align-items:center;gap:8px;padding-bottom:10px;border-bottom:1px solid #2c2e38;margin-bottom:8px;">
            <button class="btn-omni" style="padding:2px 10px;font-size:12px;" @click="toggleSelectModalAll">All</button>
            <button class="btn-omni" style="padding:2px 10px;font-size:12px;" @click="selectModalItems = new Set(allClusters.filter(c => c.status === 'outofsync').map(c => c.id))">Out of Sync</button>
            <button class="btn-omni" style="padding:2px 10px;font-size:12px;" @click="selectModalItems = new Set()">None</button>
          </div>
          <template v-for="cluster in allClusters" :key="cluster.id">
            <label
              v-if="selectModal?.type === 'refresh' || (cluster.status !== 'unmanaged' && cluster.status !== 'orphaned')"
              style="display:flex;align-items:center;gap:8px;padding:5px 0;cursor:pointer;font-size:13px;"
            >
              <input type="checkbox" :checked="selectModalItems.has(cluster.id)" @change="toggleSelectModalItem(cluster.id)" style="accent-color:#ff8b59;" />
              <span style="flex:1;color:#e8e8e9;">{{ cluster.id }}</span>
              <span :style="{ color: syncColor(cluster), fontSize: '11px' }" v-html="syncText(cluster)"></span>
            </label>
            <div
              v-else
              style="display:flex;align-items:center;gap:8px;padding:5px 0;font-size:13px;opacity:0.4;cursor:default;padding-left:20px;"
            >
              <span style="flex:1;color:#e8e8e9;">{{ cluster.id }}</span>
              <span :style="{ color: syncColor(cluster), fontSize: '11px' }" v-html="syncText(cluster)"></span>
            </div>
          </template>
        </div>
        <div class="confirm-actions" style="padding:12px 20px;border-top:1px solid #2c2e38;">
          <button class="btn-omni" @click="selectModal = null">Cancel</button>
          <button class="btn-omni" :disabled="selectModalItems.size === 0 || selectModalRunning" @click="doSelectModalAction">
            {{ selectModalRunning ? 'Running...' : selectModal.type === 'refresh' ? 'Refresh' : 'Sync' }}
          </button>
        </div>
      </div>
    </div>

    <!-- Confirm modal -->
    <div v-if="confirmModal" class="modal show" @click.self="confirmModal = null">
      <div class="modal-content confirm-modal" @click.stop>
        <div class="modal-header">
          <div class="modal-title">{{ confirmModal.title }}</div>
          <button class="modal-close" @click="confirmModal = null">&times;</button>
        </div>
        <div class="modal-body confirm-body">
          <div class="confirm-message" v-html="confirmModal.message"></div>
          <div v-if="confirmModal.requireInput" class="confirm-input-prompt">{{ confirmModal.inputPrompt }}</div>
          <input
            v-if="confirmModal.requireInput"
            v-model="confirmInput"
            class="confirm-input"
            type="text"
            :placeholder="confirmModal.requireInput"
          />
          <div class="confirm-actions">
            <button class="btn-omni" @click="confirmModal = null">Cancel</button>
            <button
              class="btn-omni"
              :disabled="!!(confirmModal.requireInput && confirmInput !== confirmModal.requireInput)"
              @click="doConfirm"
            >OK</button>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, ref, reactive, watch, onMounted, onUnmounted } from 'vue'
import { useRouter } from 'vue-router'
import { useAppStore } from '@/stores/appStore'
import { useAuthStore } from '@/stores/authStore'
import type { ResourceInfo, GitInfo, NodeGroup } from '@/types'
import { syncedIconSVG, outOfSyncIconSVG, failedIconSVG } from '@/assets/icons'

const router = useRouter()
const appStore = useAppStore()
const authStore = useAuthStore()

const state = computed(() => appStore.state)
const allClusters = computed(() => {
  const list = (state.value?.clusters ?? []).slice()
  const ts = (v?: string) => v ? new Date(v).getTime() : 0
  switch (sortKey.value) {
    case 'name-desc':     return list.sort((a, b) => b.id.localeCompare(a.id))
    case 'lastsync-desc': return list.sort((a, b) => ts(b.lastSyncTime as string) - ts(a.lastSyncTime as string))
    case 'lastsync-asc':  return list.sort((a, b) => ts(a.lastSyncTime as string) - ts(b.lastSyncTime as string))
    case 'created-desc':  return list.sort((a, b) => ts(b.createdAt as string) - ts(a.createdAt as string))
    case 'created-asc':   return list.sort((a, b) => ts(a.createdAt as string) - ts(b.createdAt as string))
    default:              return list.sort((a, b) => a.id.localeCompare(b.id))
  }
})

// Health bar counts
const countReady = computed(() =>
  allClusters.value.filter(c => {
    const phase = c.clusterPhase || ''
    if (phase && phase !== 'running') return false
    if (!c.clusterReady || c.clusterReady === 'unknown') return false
    return c.clusterReady === 'ready' && c.kubernetesApiReady === 'ready'
  }).length
)
const countNotReady = computed(() =>
  allClusters.value.filter(c => {
    const phase = c.clusterPhase || ''
    if (phase && phase !== 'running') return false
    if (!c.clusterReady || c.clusterReady === 'unknown') return false
    return c.clusterReady === 'not-ready' || c.kubernetesApiReady === 'not-ready'
  }).length
)
const countScalingUp = computed(() => allClusters.value.filter(c => c.clusterPhase === 'scaling-up').length)
const countScalingDown = computed(() => allClusters.value.filter(c => c.clusterPhase === 'scaling-down').length)
const countDestroying = computed(() => allClusters.value.filter(c => c.clusterPhase === 'destroying').length)
const countReconfiguring = computed(() => allClusters.value.filter(c => c.clusterPhase === 'reconfiguring').length)
const healthTotal = computed(() =>
  countReady.value + countNotReady.value + countScalingUp.value +
  countScalingDown.value + countDestroying.value + countReconfiguring.value
)

// State
const clusterStatusFilter = ref<string | null>(null)
const clusterSyncFilters = reactive<Record<string, boolean>>({})
const clusterSearch = ref('')
type SortKey = 'name-asc' | 'name-desc' | 'lastsync-desc' | 'lastsync-asc' | 'created-desc' | 'created-asc'
const sortKey = ref<SortKey>('name-asc')
const activeDropdown = ref<'filter' | 'sort' | 'pageSize' | null>(null)
const sortDefs: { key: SortKey; label: string }[] = [
  { key: 'name-asc',      label: 'Name A→Z' },
  { key: 'name-desc',     label: 'Name Z→A' },
  { key: 'lastsync-desc', label: 'Last Sync (newest)' },
  { key: 'lastsync-asc',  label: 'Last Sync (oldest)' },
  { key: 'created-desc',  label: 'Created (newest)' },
  { key: 'created-asc',   label: 'Created (oldest)' },
]
const sortLabel = computed(() => sortDefs.find(d => d.key === sortKey.value)?.label ?? 'Sort')
const pageSize = ref(10)
const viewMode = ref<'grid' | 'list'>(
  (localStorage.getItem('clustersViewMode') as 'grid' | 'list') || 'grid'
)
watch(viewMode, v => localStorage.setItem('clustersViewMode', v))
const openMenuId = ref<string | null>(null)
const currentPage = ref(1)
const clusterActionPending = reactive<Record<string, string>>({})
const isRunning = computed(() => state.value?.lastReconcile?.status === 'running')


const syncDefs = [
  { key: 'failed',    label: 'Failed' },
  { key: 'managed',   label: 'Managed' },
  { key: 'outofsync', label: 'Out of Sync' },
  { key: 'orphaned',  label: 'Orphaned' },
  { key: 'synced',    label: 'Synced' },
  { key: 'unmanaged', label: 'Unmanaged' },
]

const healthDefs = [
  { key: 'ready',         label: 'Ready' },
  { key: 'not-ready',     label: 'Not Ready' },
  { key: 'scaling-up',    label: 'Scaling Up' },
  { key: 'scaling-down',  label: 'Scaling Down' },
  { key: 'destroying',    label: 'Destroying' },
  { key: 'reconfiguring', label: 'Reconfiguring' },
]

const visibleSyncDefs = computed(() => {
  const presentKeys = new Set<string>()
  allClusters.value.forEach(c => {
    const st = c.status || ''
    const key = (st === 'success' || st === 'applied' || st === 'synced') ? 'synced'
      : st === 'outofsync' ? 'outofsync'
      : st === 'failed' ? 'failed'
      : st === 'unmanaged' ? 'unmanaged'
      : st === 'orphaned' ? 'orphaned'
      : null
    if (key) { presentKeys.add(key); presentKeys.add('managed') }
  })
  return syncDefs.filter(d => presentKeys.has(d.key))
})

const activeSyncFilterCount = computed(() =>
  Object.values(clusterSyncFilters).filter(Boolean).length + (clusterStatusFilter.value ? 1 : 0)
)
const activeSyncFilterLabels = computed(() => {
  const parts = syncDefs.filter(d => clusterSyncFilters[d.key]).map(d => d.label)
  if (clusterStatusFilter.value) {
    const h = healthDefs.find(d => d.key === clusterStatusFilter.value)
    if (h) parts.push(h.label)
  }
  return parts.join(', ')
})

const displayClusters = computed(() => {
  const activeSyncKeys = Object.keys(clusterSyncFilters).filter(k => clusterSyncFilters[k])
  let result = allClusters.value.filter(c => {
    const st = c.status || ''
    const phase = c.clusterPhase || ''
    if (clusterStatusFilter.value) {
      if (clusterStatusFilter.value === 'ready') {
        if (!(c.clusterReady === 'ready' && c.kubernetesApiReady === 'ready') || (phase && phase !== 'running')) return false
      } else if (clusterStatusFilter.value === 'not-ready') {
        if (!((c.clusterReady === 'not-ready' || c.kubernetesApiReady === 'not-ready') && (!phase || phase === 'running'))) return false
      } else if (clusterStatusFilter.value === 'scaling-up') {
        if (phase !== 'scaling-up') return false
      } else if (clusterStatusFilter.value === 'scaling-down') {
        if (phase !== 'scaling-down') return false
      } else if (clusterStatusFilter.value === 'destroying') {
        if (phase !== 'destroying') return false
      } else if (clusterStatusFilter.value === 'reconfiguring') {
        if (phase !== 'reconfiguring') return false
      }
    }
    if (activeSyncKeys.length > 0) {
      const isManaged = st !== 'unmanaged' && st !== 'orphaned'
      const syncKey = (st === 'success' || st === 'applied' || st === 'synced') ? 'synced'
        : st === 'outofsync' ? 'outofsync'
        : st === 'failed' ? 'failed'
        : st === 'unmanaged' ? 'unmanaged'
        : st === 'orphaned' ? 'orphaned'
        : null
      const matches =
        (clusterSyncFilters['managed'] && isManaged) ||
        (syncKey && clusterSyncFilters[syncKey])
      if (!matches) return false
    }
    return true
  })
  if (clusterSearch.value.trim()) {
    const q = clusterSearch.value.trim().toLowerCase()
    result = result.filter(c => c.id.toLowerCase().includes(q))
  }
  return result
})

const totalPages = computed(() =>
  pageSize.value === 0 ? 1 : Math.max(1, Math.ceil(displayClusters.value.length / pageSize.value))
)
const pageClusters = computed(() => {
  if (pageSize.value === 0) return displayClusters.value
  const start = (currentPage.value - 1) * pageSize.value
  return displayClusters.value.slice(start, start + pageSize.value)
})

// Confirm modal
interface ConfirmModal {
  title: string
  message: string
  requireInput?: string
  inputPrompt?: string
  onConfirm: () => void
}
const confirmModal = ref<ConfirmModal | null>(null)
const confirmInput = ref('')

function doConfirm() {
  if (confirmModal.value?.requireInput && confirmInput.value !== confirmModal.value.requireInput) return
  confirmModal.value?.onConfirm()
}

// Select modal (global Refresh / Sync)
interface SelectModal { type: 'refresh' | 'sync' }
const selectModal = ref<SelectModal | null>(null)
const selectModalItems = ref<Set<string>>(new Set())
const selectModalRunning = ref(false)

const selectableCluster = (c: ResourceInfo) =>
  selectModal.value?.type === 'refresh' || (c.status !== 'unmanaged' && c.status !== 'orphaned')
const selectModalAllChecked = computed(() => {
  const selectable = allClusters.value.filter(selectableCluster)
  return selectable.length > 0 && selectable.every(c => selectModalItems.value.has(c.id))
})
function openSelectModal(type: 'refresh' | 'sync') {
  selectModalItems.value = new Set()
  selectModal.value = { type }
}
function toggleSelectModalItem(id: string) {
  const s = new Set(selectModalItems.value)
  s.has(id) ? s.delete(id) : s.add(id)
  selectModalItems.value = s
}
function toggleSelectModalAll() {
  if (selectModalAllChecked.value) {
    selectModalItems.value = new Set()
  } else {
    selectModalItems.value = new Set(allClusters.value.filter(selectableCluster).map(c => c.id))
  }
}
async function doSelectModalAction() {
  if (!selectModal.value) return
  selectModalRunning.value = true
  const ids = Array.from(selectModalItems.value)
  if (selectModal.value.type === 'refresh') {
    await Promise.all(ids.map(id => {
      const c = allClusters.value.find(x => x.id === id)
      return c ? refreshCluster(c) : Promise.resolve()
    }))
  } else {
    await Promise.all(ids.map(id => {
      const c = allClusters.value.find(x => x.id === id)
      return c ? syncCluster(c) : Promise.resolve()
    }))
  }
  selectModalRunning.value = false
  selectModal.value = null
}

// Helpers
function setClusterFilter(key: string) {
  clusterStatusFilter.value = clusterStatusFilter.value === key ? null : key
  currentPage.value = 1
}
function clearClusterFilter() {
  clusterStatusFilter.value = null
  currentPage.value = 1
}
function toggleSyncFilter(key: string) {
  clusterSyncFilters[key] = !clusterSyncFilters[key]
  currentPage.value = 1
}
function clearSyncFilters() {
  Object.keys(clusterSyncFilters).forEach(k => { clusterSyncFilters[k] = false })
  clusterStatusFilter.value = null
  currentPage.value = 1
}
function setSort(key: SortKey) {
  sortKey.value = key
  activeDropdown.value = null
  currentPage.value = 1
}
function setPageSize(n: number) {
  pageSize.value = n
  currentPage.value = 1
}

function clusterRepo(c: ResourceInfo): GitInfo | null {
  if (!c.repoName || !state.value?.repos) return null
  return state.value.repos.find(r => r.name === c.repoName) || null
}

function clusterCardAttrs(c: ResourceInfo): Record<string, string> {
  const attrs: Record<string, string> = {}
  const activePhases = ['scaling-up', 'scaling-down', 'destroying', 'reconfiguring']
  if (c.clusterPhase && activePhases.includes(c.clusterPhase)) {
    attrs['data-phase'] = c.clusterPhase
  }
  if (c.clusterReady === 'not-ready' || c.kubernetesApiReady === 'not-ready') {
    attrs['data-health'] = 'not-ready'
  }
  return attrs
}

function mgmtBadge(c: ResourceInfo): string {
  if (c.status === 'unmanaged') return 'Unmanaged'
  if (c.status === 'orphaned') return 'Orphaned'
  return 'Managed'
}
function mgmtColor(c: ResourceInfo): string {
  if (c.status === 'unmanaged') return '#5b5c64'
  if (c.status === 'orphaned') return '#a78bfa'
  return '#7d7d85'
}

function syncText(c: ResourceInfo): string {
  const repoSyncErrors: Record<string, boolean> = {}
  ;(state.value?.repos ?? []).forEach(r => { if (r.syncError) repoSyncErrors[r.name || ''] = true })
  if (c.repoName && repoSyncErrors[c.repoName] && c.status !== 'unmanaged') {
    return '<span class="spinner" style="width:10px;height:10px;display:inline-block;vertical-align:middle"></span> Syncing'
  }
  if (c.status === 'unmanaged' || c.status === 'orphaned') return '—'
  if (c.status === 'outofsync' && (c.error || c.lastSyncError)) return failedIconSVG + ' Sync Failed'
  if (c.status === 'outofsync') return outOfSyncIconSVG + ' Out of Sync'
  if (c.status === 'failed') return failedIconSVG + ' Failed'
  if (c.status === 'syncing') return '● Syncing'
  if (c.status === 'success' || c.status === 'applied') return syncedIconSVG + ' Synced'
  return '—'
}
function syncColor(c: ResourceInfo): string {
  if (c.status === 'unmanaged' || c.status === 'orphaned') return '#5b5c64'
  if (c.status === 'outofsync' && (c.error || c.lastSyncError)) return '#f87171'
  if (c.status === 'outofsync') return '#fb923c'
  if (c.status === 'failed') return '#f87171'
  if (c.status === 'syncing') return '#2dd4bf'
  if (c.status === 'success' || c.status === 'applied') return '#4ade80'
  return '#5b5c64'
}
function healthText(c: ResourceInfo): string {
  const phase = c.clusterPhase || ''
  if (phase === 'scaling-up') return '↑ Scaling Up'
  if (phase === 'scaling-down') return '↓ Scaling Down'
  if (phase === 'destroying') return '✕ Destroying'
  if (phase === 'reconfiguring') return '↻ Reconfiguring'
  if (c.clusterReady === 'ready' && c.kubernetesApiReady === 'ready') return syncedIconSVG + ' Ready'
  if (c.clusterReady === 'not-ready' || c.kubernetesApiReady === 'not-ready') return failedIconSVG + ' Not Ready'
  return '—'
}
function healthColor(c: ResourceInfo): string {
  const phase = c.clusterPhase || ''
  if (phase === 'scaling-up') return '#60a5fa'
  if (phase === 'scaling-down') return '#f59e0b'
  if (phase === 'destroying') return '#f43f5e'
  if (phase === 'reconfiguring') return '#a78bfa'
  if (c.clusterReady === 'ready' && c.kubernetesApiReady === 'ready') return '#4ade80'
  if (c.clusterReady === 'not-ready' || c.kubernetesApiReady === 'not-ready') return '#f87171'
  return '#5b5c64'
}

function clusterSections(c: ResourceInfo) {
  const cp: NodeGroup = c.controlPlane ?? { count: 0 }
  const workers = Array.isArray(c.workers) ? c.workers : (c.workers ? [c.workers] : [])
  const sections: { label: string; count: number; mc: string }[] = [
    { label: 'Controlplane', count: cp.count || 0, mc: cp.machineClass || '' },
  ]
  workers.forEach(wk => {
    sections.push({ label: wk.name || 'Workers', count: wk.count || 0, mc: wk.machineClass || '' })
  })
  if (workers.length === 0) sections.push({ label: 'Workers', count: 0, mc: '' })
  return sections
}

function isZeroTime(d?: string): boolean {
  if (!d) return true
  const dt = new Date(d)
  if (isNaN(dt.getTime())) return true
  return dt.getFullYear() <= 1
}

function fmtDateTime(d?: string): string {
  if (!d) return '—'
  const dt = new Date(d)
  const pad = (n: number) => (n < 10 ? '0' + n : '' + n)
  return dt.getFullYear() + '-' + pad(dt.getMonth() + 1) + '-' + pad(dt.getDate()) +
    ' ' + pad(dt.getHours()) + ':' + pad(dt.getMinutes())
}

function ago(d?: string): string {
  if (!d) return ''
  const dt = new Date(d)
  if (isNaN(dt.getTime())) return ''
  const s = Math.floor((Date.now() - dt.getTime()) / 1000)
  if (s < 5) return 'just now'
  if (s < 60) return s + 's ago'
  if (s < 3600) return Math.floor(s / 60) + 'm ago'
  if (s < 86400) return Math.floor(s / 3600) + 'h ago'
  const days = Math.floor(s / 86400)
  if (days < 31) return days + 'd ago'
  if (days < 365) return Math.floor(days / 30) + 'mo ago'
  return Math.floor(days / 365) + 'y ago'
}

function omniClusterUrl(id: string): string {
  const ep = (state.value?.omniEndpoint || '').replace(/\/$/, '')
  return ep + '/clusters/' + id
}

function closeMenu(e: MouseEvent) {
  const t = e.target as HTMLElement
  if (!t.closest('.cluster-list-menu-wrap')) openMenuId.value = null
  if (!t.closest('.filter-dropdown-wrap')) activeDropdown.value = null
}
onMounted(() => document.addEventListener('click', closeMenu))
onUnmounted(() => document.removeEventListener('click', closeMenu))

function goToCluster(id: string) {
  router.push(`/clusters/${encodeURIComponent(id)}`)
}

async function refreshCluster(cluster: ResourceInfo) {
  if (clusterActionPending[cluster.id]) return
  clusterActionPending[cluster.id] = 'refresh'
  try {
    await fetch('/api/refresh-cluster', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ id: cluster.id }),
    })
    setTimeout(() => { delete clusterActionPending[cluster.id] }, 5000)
  } catch { delete clusterActionPending[cluster.id] }
}

async function syncCluster(cluster: ResourceInfo) {
  if (clusterActionPending[cluster.id]) return
  clusterActionPending[cluster.id] = 'sync'
  try {
    await fetch('/api/force-cluster', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ id: cluster.id }),
    })
    setTimeout(() => { delete clusterActionPending[cluster.id] }, 8000)
  } catch { delete clusterActionPending[cluster.id] }
}

async function toggleAutoSync(cluster: ResourceInfo) {
  const enabled = cluster.autoSync === false
  await fetch('/api/set-cluster-autosync', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ id: cluster.id, autoSync: enabled }),
  })
}

function deleteCluster(cluster: ResourceInfo) {
  confirmInput.value = ''
  confirmModal.value = {
    title: 'Delete Cluster',
    message: `Are you sure you want to delete the Cluster <b>${cluster.id}</b>?<br><br>Deleting the Cluster will delete all the cluster's managed resources, which can be dangerous.<br>Be sure you understand the effects of deleting this resource before continuing.`,
    requireInput: cluster.id,
    inputPrompt: `Please type '${cluster.id}' to confirm the deletion of the cluster`,
    onConfirm: async () => {
      confirmModal.value = null
      confirmInput.value = ''
      await fetch('/api/delete-cluster', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ id: cluster.id }),
      })
    },
  }
}

async function exportCluster(cluster: ResourceInfo) {
  const r = await fetch('/api/export-cluster', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ id: cluster.id }),
  })
  if (!r.ok) { alert('Failed to export cluster: ' + r.statusText); return }
  const yaml = await r.text()
  const blob = new Blob([yaml], { type: 'application/x-yaml' })
  const url = URL.createObjectURL(blob)
  const a = document.createElement('a')
  a.href = url; a.download = cluster.id + '.yaml'
  document.body.appendChild(a); a.click()
  document.body.removeChild(a); URL.revokeObjectURL(url)
}
</script>
