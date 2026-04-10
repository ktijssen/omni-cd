<template>
  <div class="container">
    <div class="header" style="border-bottom:none;flex-direction:column;align-items:stretch;gap:8px;">
      <h1 style="font-size:18px;font-weight:600;color:#fff;letter-spacing:-0.3px;white-space:nowrap;">Machine Classes</h1>
      <div style="display:flex;align-items:center;gap:8px;">
        <template v-if="authStore.isAdmin()">
          <span v-if="isRunning" class="spinner"></span>
          <button class="btn-omni" :disabled="isRunning" @click="doRefreshMC">
            {{ isRunning ? 'Refreshing...' : 'Refresh' }}
          </button>
          <button class="btn-omni" :disabled="isRunning" @click="doSyncAll">
            {{ isRunning ? 'Syncing...' : 'Sync' }}
          </button>
        </template>
        <input
          v-model="mcSearch"
          type="text"
          placeholder="Search machine classes..."
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

    <div v-if="!state?.omniConfigured" class="placeholder-page">
      <div class="placeholder-icon">🔌</div>
      <div class="placeholder-title">Please configure an Omni Instance first in Settings/Instances</div>
    </div>

    <template v-else>
      <!-- Toolbar -->
      <div style="display:flex;align-items:center;gap:8px;padding:0 0 12px;flex-wrap:wrap;">
        <div style="margin-left:auto;display:flex;align-items:center;gap:8px;">
          <!-- Status filter dropdown -->
          <div class="filter-dropdown-wrap">
            <button class="filter-select-btn" :class="{ active: activeFilterCount > 0 }" @click="activeDropdown = activeDropdown === 'filter' ? null : 'filter'">
              <span class="filter-select-label">
                <label>Status</label>
                <span>{{ activeFilterCount > 0 ? activeFilterLabels : 'All' }}</span>
              </span>
              <svg width="16" height="16" viewBox="0 0 16 16" fill="currentColor" style="flex-shrink:0;transition:transform 0.2s" :style="{ transform: activeDropdown === 'filter' ? 'rotate(180deg)' : 'none' }"><path fill-rule="evenodd" clip-rule="evenodd" d="M3.14645 5.14645C3.34171 4.95118 3.65829 4.95118 3.85355 5.14645L8 9.29289L12.1464 5.14645C12.3417 4.95118 12.6583 4.95118 12.8536 5.14645C13.0488 5.34171 13.0488 5.65829 12.8536 5.85355L8.35355 10.3536C8.15829 10.5488 7.84171 10.5488 7.64645 10.3536L3.14645 5.85355C2.95118 5.65829 2.95118 5.34171 3.14645 5.14645Z"/></svg>
            </button>
            <div v-if="activeDropdown === 'filter'" class="cluster-list-menu" style="min-width:160px;right:0;left:auto;">
              <button
                v-for="f in statusFilters"
                :key="f.key"
                class="cluster-list-menu-item"
                :class="{ active: activeFilters.has(f.key) }"
                @click="toggleFilter(f.key)"
              >{{ activeFilters.has(f.key) ? '✓ ' : '\u00a0\u00a0 ' }}{{ f.label }}</button>
              <div v-if="activeFilterCount > 0" class="cluster-list-menu-divider"></div>
              <button v-if="activeFilterCount > 0" class="cluster-list-menu-item" style="color:#9fa1a6" @click="clearFilters(); activeDropdown = null">Clear filters</button>
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

      <div v-if="filteredMCs.length === 0" class="placeholder-page">
        <div class="placeholder-title">No machine classes found</div>
        <div class="placeholder-sub">Machine classes defined in your git repo will appear here.</div>
      </div>

      <template v-else>
        <div v-if="viewMode === 'grid'" class="mc-grid">
          <div
            v-for="mc in pageMCs"
            :key="mc.id"
            class="mc-card"
            :class="{ clickable: hasDetails(mc) }"
            :data-status="mc.status || 'idle'"
            @click="hasDetails(mc) && openDetail(mc)"
          >
            <div class="mc-card-accent"></div>
            <div class="mc-card-header">

              <!-- Title row: name + plain badge -->
              <div class="mc-card-title-row">
                <div style="display:flex;align-items:center;gap:6px;min-width:0;flex-wrap:wrap;">
                  <span class="mc-card-name">{{ mc.id }}</span>
                  <a
                    v-if="state?.omniEndpoint"
                    class="btn-omni"
                    style="font-size:10px;padding:1px 6px;text-decoration:none;flex-shrink:0"
                    :href="state.omniEndpoint.replace(/\/$/, '') + '/machine-classes/' + mc.id"
                    target="_blank"
                    @click.stop
                  >↗ Open in Omni</a>
                </div>
                <span class="mc-card-status" :style="{ color: mc.status === 'unmanaged' ? '#5b5c64' : '#7d7d85' }">
                  {{ mc.status === 'unmanaged' ? 'unmanaged' : 'managed' }}
                </span>
              </div>


              <!-- Meta: Sync Status, Used by, Repository, Branch, Created At, Last Sync -->
              <div class="cluster-card-meta">
                <div class="cluster-card-meta-pair">
                  <span class="cluster-card-meta-label">Sync Status:</span>
                  <span class="cluster-card-meta-value">
                    <span v-if="mc.status === 'unmanaged'" style="color:#5b5c64">—</span>
                    <span v-else :style="{ color: syncStatusColor(mc.status) }" v-html="syncStatusText(mc)"></span>
                  </span>
                </div>
                <div class="cluster-card-meta-pair" style="align-items:flex-start">
                  <span class="cluster-card-meta-label">Used by:</span>
                  <span class="cluster-card-meta-value" style="white-space:normal">
                    <div v-if="clustersUsingMC(mc.id).length > 0" class="mc-used-by">
                      <span
                        v-for="cid in clustersUsingMC(mc.id)"
                        :key="cid"
                        class="mc-used-by-chip"
                        :title="cid"
                        @click.stop="goToCluster(cid)"
                      >{{ cid }}</span>
                    </div>
                    <span v-else class="mc-used-by-none">none</span>
                  </span>
                </div>
                <div class="cluster-card-meta-pair">
                  <span class="cluster-card-meta-label">Repository:</span>
                  <span class="cluster-card-meta-value">
                    <span v-if="mc.repoName">{{ mc.repoName }}</span>
                    <span v-else style="color:#5b5c64">—</span>
                  </span>
                </div>
                <div class="cluster-card-meta-pair">
                  <span class="cluster-card-meta-label">Branch:</span>
                  <span class="cluster-card-meta-value">
                    <span v-if="mcRepoBranch(mc)">{{ mcRepoBranch(mc) }}</span>
                    <span v-else style="color:#5b5c64">—</span>
                  </span>
                </div>
                <div class="cluster-card-meta-pair">
                  <span class="cluster-card-meta-label">Created At:</span>
                  <span class="cluster-card-meta-value">
                    <span v-if="isZeroTime(mc.createdAt)" style="color:#5b5c64">—</span>
                    <span v-else>{{ fmtDateTime(mc.createdAt) }} <span style="color:#7d7d85">({{ ago(mc.createdAt) }})</span></span>
                  </span>
                </div>
                <div class="cluster-card-meta-pair">
                  <span class="cluster-card-meta-label">Last Sync:</span>
                  <span class="cluster-card-meta-value">
                    <span v-if="isZeroTime(mc.lastSyncTime)" style="color:#5b5c64">—</span>
                    <span v-else>{{ fmtDateTime(mc.lastSyncTime) }} <span style="color:#7d7d85">({{ ago(mc.lastSyncTime) }})</span></span>
                  </span>
                </div>
              </div>

              <div class="mc-card-divider"></div>

              <!-- Spec info rows: Mode, Provider, Cores, etc. -->
              <template v-for="spec in [mcSpec(mc)]" :key="mc.id + '-spec'">
                <div class="mc-info-row">
                  <span class="mc-info-label">Mode</span>
                  <span class="mc-info-value" :style="{ color: provisionType(mc) === 'auto' ? '#60a5fa' : '#9fa1a6' }">
                    {{ provisionType(mc) === 'auto' ? 'Auto-Provision' : 'Manual' }}
                  </span>
                </div>
                <div v-if="spec.providerId" class="mc-info-row">
                  <span class="mc-info-label">Provider</span>
                  <span class="mc-info-value">{{ spec.providerId }}</span>
                </div>
                <div v-if="spec.providerData.cores || spec.providerData.vcpu || spec.providerData.cpu" class="mc-info-row">
                  <span class="mc-info-label">Cores</span>
                  <span class="mc-info-value">{{ spec.providerData.cores || spec.providerData.vcpu || spec.providerData.cpu }}</span>
                </div>
                <div v-if="spec.providerData.sockets" class="mc-info-row">
                  <span class="mc-info-label">Sockets</span>
                  <span class="mc-info-value">{{ spec.providerData.sockets }}</span>
                </div>
                <div v-if="spec.providerData.memory || spec.providerData.ram" class="mc-info-row">
                  <span class="mc-info-label">Memory</span>
                  <span class="mc-info-value">{{ spec.providerData.memory || spec.providerData.ram }}</span>
                </div>
                <div v-if="spec.providerData.disk_size || spec.providerData.diskSize || spec.providerData.disk" class="mc-info-row">
                  <span class="mc-info-label">Disk Size</span>
                  <span class="mc-info-value">{{ spec.providerData.disk_size || spec.providerData.diskSize || spec.providerData.disk }}</span>
                </div>
                <div v-if="Object.keys(spec.matchLabels).length > 0" class="mc-info-row" style="align-items:flex-start;margin-top:4px">
                  <span class="mc-info-label">Match Labels</span>
                  <span class="mc-info-value">
                    <div v-for="(val, key) in spec.matchLabels" :key="key" style="padding:1px 0">
                      <span style="color:#7d7d85;margin-right:5px">•</span>{{ key }} = {{ val }}
                    </div>
                  </span>
                </div>
              </template>

              <!-- Actions pinned to bottom -->
              <div style="margin-top:auto">
                <div class="mc-card-divider" style="margin-top:8px"></div>
                <div class="cluster-card-actions" v-if="authStore.isAdmin()">
                  <template v-if="mc.status === 'unmanaged'">
                    <button class="btn-omni" @click.stop="exportMC(mc)">↓ Export</button>
                    <button
                      class="btn-omni"
                      :disabled="clustersUsingMC(mc.id).length > 0"
                      :title="clustersUsingMC(mc.id).length > 0 ? 'In use by: ' + clustersUsingMC(mc.id).join(', ') : ''"
                      @click.stop="deleteMC(mc)"
                    >✕ Delete</button>
                  </template>
                  <template v-else>
                    <button class="btn-omni" :disabled="!!actionPending[mc.id]" @click.stop="refreshSingleMC(mc)">
                      ↺ {{ actionPending[mc.id] === 'refresh' ? 'Refreshing...' : 'Refresh' }}
                    </button>
                    <button class="btn-omni" :disabled="!!actionPending[mc.id]" @click.stop="syncMC(mc)">⇅ {{ actionPending[mc.id] === 'sync' ? 'Syncing...' : 'Sync' }}</button>
                    <button
                      class="btn-omni auto-sync"
                      :class="{ active: mc.autoSync === true }"
                      @click.stop="toggleMCAutoSync(mc, $event)"
                    >{{ mc.autoSync === true ? '● Auto-Sync: On' : '○ Auto-Sync: Off' }}</button>
                    <button
                      class="btn-omni"
                      :disabled="clustersUsingMC(mc.id).length > 0"
                      :title="clustersUsingMC(mc.id).length > 0 ? 'In use by: ' + clustersUsingMC(mc.id).join(', ') : ''"
                      @click.stop="deleteMC(mc)"
                    >✕ Delete</button>
                  </template>
                </div>
              </div>

            </div>
          </div>
        </div>

        <!-- Machine class list -->
        <table v-else class="cluster-list-table">
          <thead>
            <tr>
              <th>Name</th>
              <th>Status</th>
              <th>Sync</th>
              <th>Used By</th>
              <th>Repo</th>
              <th>Last Sync</th>
              <th v-if="authStore.isAdmin()"></th>
            </tr>
          </thead>
          <tbody>
            <tr
              v-for="mc in pageMCs"
              :key="mc.id"
              class="cluster-list-row"
              :class="{ clickable: hasDetails(mc) }"
              :data-status="mc.status || 'idle'"
              @click="hasDetails(mc) && openDetail(mc)"
            >
              <td class="cluster-list-name">
                {{ mc.id }}
                <a
                  v-if="state?.omniEndpoint"
                  class="btn-omni"
                  style="font-size:11px;padding:1px 6px;text-decoration:none;margin-left:6px"
                  :href="state.omniEndpoint.replace(/\/$/, '') + '/machine-classes/' + mc.id"
                  target="_blank"
                  @click.stop
                  title="Open in Omni"
                >&#8599;</a>
              </td>
              <td :style="{ color: mc.status === 'unmanaged' ? '#5b5c64' : '#7d7d85' }">
                {{ mc.status === 'unmanaged' ? 'Unmanaged' : 'Managed' }}
              </td>
              <td>
                <span v-if="mc.status === 'unmanaged'" style="color:#5b5c64">—</span>
                <span v-else :style="{ color: syncStatusColor(mc.status) }" v-html="syncStatusText(mc)"></span>
              </td>
              <td>
                <span v-if="clustersUsingMC(mc.id).length === 0" style="color:#5b5c64">—</span>
                <span v-else class="mc-used-by" style="flex-wrap:wrap;gap:3px">
                  <span
                    v-for="cid in clustersUsingMC(mc.id).slice(0, 3)"
                    :key="cid"
                    class="mc-used-by-chip"
                    @click.stop="goToCluster(cid)"
                  >{{ cid }}</span>
                  <span v-if="clustersUsingMC(mc.id).length > 3" style="color:#7d7d85;font-size:11px">+{{ clustersUsingMC(mc.id).length - 3 }} more</span>
                </span>
              </td>
              <td style="color:#9fa1a6">{{ mc.repoName || '—' }}</td>
              <td style="color:#9fa1a6">
                <span v-if="!isZeroTime(mc.lastSyncTime)" :title="fmtDateTime(mc.lastSyncTime)">{{ ago(mc.lastSyncTime) }}</span>
                <span v-else>—</span>
              </td>
              <td v-if="authStore.isAdmin()" class="cluster-list-actions" @click.stop>
                <div class="cluster-list-menu-wrap">
                  <button class="cluster-list-menu-btn" @click="openMenuId = openMenuId === mc.id ? null : mc.id">⋮</button>
                  <div v-if="openMenuId === mc.id" class="cluster-list-menu">
                    <template v-if="mc.status === 'unmanaged'">
                      <button class="cluster-list-menu-item" @click="exportMC(mc); openMenuId = null">↓ Export</button>
                      <div class="cluster-list-menu-divider"></div>
                      <button
                        class="cluster-list-menu-item danger"
                        :disabled="clustersUsingMC(mc.id).length > 0"
                        :title="clustersUsingMC(mc.id).length > 0 ? 'In use by: ' + clustersUsingMC(mc.id).join(', ') : ''"
                        @click="deleteMC(mc); openMenuId = null"
                      >✕ Delete</button>
                    </template>
                    <template v-else>
                      <button class="cluster-list-menu-item" :disabled="!!actionPending[mc.id]" @click="refreshSingleMC(mc); openMenuId = null">
                        ↺ {{ actionPending[mc.id] === 'refresh' ? 'Refreshing...' : 'Refresh' }}
                      </button>
                      <button class="cluster-list-menu-item" :disabled="!!actionPending[mc.id]" @click="syncMC(mc); openMenuId = null">
                        ⇅ {{ actionPending[mc.id] === 'sync' ? 'Syncing...' : 'Sync' }}
                      </button>
                      <button
                        class="cluster-list-menu-item"
                        :class="{ active: mc.autoSync === true }"
                        @click="toggleMCAutoSync(mc, $event); openMenuId = null"
                      >{{ mc.autoSync === true ? '● Auto-Sync: On' : '○ Auto-Sync: Off' }}</button>
                      <div class="cluster-list-menu-divider"></div>
                      <button
                        class="cluster-list-menu-item danger"
                        :disabled="clustersUsingMC(mc.id).length > 0"
                        :title="clustersUsingMC(mc.id).length > 0 ? 'In use by: ' + clustersUsingMC(mc.id).join(', ') : ''"
                        @click="deleteMC(mc); openMenuId = null"
                      >✕ Delete</button>
                    </template>
                  </div>
                </div>
              </td>
            </tr>
          </tbody>
        </table>

        <!-- Pagination -->
        <div v-if="pageSize > 0 && filteredMCs.length > pageSize" class="pagination">
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

    <!-- Detail modal: Live / Diff -->
    <div v-if="detailModal" class="repo-modal-wrap show" @click.self="detailModal = null">
      <div class="repo-modal-box" style="width:1000px;max-width:95vw;height:90vh;display:flex;flex-direction:column;">
        <div style="display:flex;align-items:center;justify-content:space-between;margin-bottom:14px;">
          <div class="repo-modal-title" style="margin-bottom:0">{{ detailModal.id }}</div>
          <button class="btn-omni" @click="detailModal = null" style="padding:2px 10px;font-size:13px;">✕</button>
        </div>
        <!-- Tabs -->
        <div class="cluster-detail-tabs-bar" style="margin:0 -24px 12px;padding:0 16px;background:#1f222e;">
          <button
            v-for="tab in ['live','diff']"
            :key="tab"
            class="cluster-detail-tab"
            :class="{ active: detailTab === tab }"
            @click="detailTab = tab as 'live' | 'diff'"
          >{{ tab === 'live' ? 'Live' : 'Diff' }}</button>
          <label v-if="detailTab === 'live'" class="mc-ignored-toggle" style="margin-left:auto;">
            <input type="checkbox" class="mc-ignored-cb" v-model="showIgnoredFields" /> Show Ignored Fields
          </label>
        </div>
        <!-- Content -->
        <div style="overflow:auto;flex:1;min-height:0;">
          <template v-if="detailTab === 'live'">
            <div v-if="detailModal.liveContent" style="padding:16px;">
              <div class="sbs-table-single">
                <div
                  v-for="(row, i) in detailLiveRows"
                  :key="i"
                  class="sbs-cell"
                  :class="{ 'sbs-ignored': row.isPlaceholder, 'sbs-meta-dim': row.isDim }"
                >
                  <span class="sbs-ln">{{ row.lineNum ? row.lineNum + '.' : '' }}</span>{{ row.content }}
                </div>
              </div>
            </div>
            <div v-else style="color:#7d7d85;text-align:center;padding:40px;">No live state available</div>
          </template>
          <template v-else>
            <div v-if="detailModal.status !== 'success' && detailModal.status !== 'applied' && detailModal.status !== 'unmanaged' && detailDiffVisible.length" style="padding:16px;">
              <div class="sbs-table">
                <template v-for="(row, i) in detailDiffVisible" :key="i">
                  <template v-if="row.separator">
                    <div class="sbs-cell sbs-hunk-hdr">···</div>
                    <div class="sbs-cell sbs-hunk-hdr">···</div>
                  </template>
                  <template v-else>
                    <div
                      class="sbs-cell"
                      :class="{ 'sbs-del': row.changed && !!row.l && row.l.content.trim() !== '' }"
                    ><span class="sbs-ln">{{ row.seq }}.</span>{{ row.l?.content ?? '' }}</div>
                    <div
                      class="sbs-cell"
                      :class="{ 'sbs-add': row.changed && !!row.r && row.r.content.trim() !== '' }"
                    ><span class="sbs-ln">{{ row.seq }}.</span>{{ row.r?.content ?? '' }}</div>
                  </template>
                </template>
              </div>
            </div>
            <div v-else style="color:#7d7d85;text-align:center;padding:40px;">
              {{ detailModal.status === 'unmanaged' ? 'No diff — this machine class is not managed by Git.' : (detailModal.status === 'success' || detailModal.status === 'applied') ? 'No diff — this machine class is in sync.' : 'No diff available' }}
            </div>
          </template>
        </div>
      </div>
    </div>

    <!-- Confirm delete modal -->
    <div v-if="confirmModal" class="modal-overlay" @click.self="confirmModal = null">
      <div class="modal-box">
        <div class="modal-title">{{ confirmModal.title }}</div>
        <div class="modal-body" v-html="confirmModal.message"></div>
        <div v-if="confirmModal.requireInput" style="font-size:12px;color:#9fa1a6;margin-bottom:6px;">{{ confirmModal.inputPrompt }}</div>
        <input
          v-if="confirmModal.requireInput"
          v-model="confirmInput"
          class="repo-form-input"
          type="text"
          :placeholder="confirmModal.requireInput"
        />
        <div class="modal-actions">
          <button class="btn-omni" @click="confirmModal = null">Cancel</button>
          <button
            class="btn-omni"
            :disabled="!!(confirmModal.requireInput && confirmInput !== confirmModal.requireInput)"
            @click="doConfirm"
          >Confirm</button>
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
import type { ResourceInfo } from '@/types'
import { syncedIconSVG, outOfSyncIconSVG, failedIconSVG } from '@/assets/icons'

const router = useRouter()
const appStore = useAppStore()
const authStore = useAuthStore()

const state = computed(() => appStore.state)
const mcSearch = ref('')
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
const currentPage = ref(1)
const actionPending = reactive<Record<string, string>>({})
const viewMode = ref<'grid' | 'list'>(
  (localStorage.getItem('mcViewMode') as 'grid' | 'list') || 'grid'
)
watch(viewMode, v => localStorage.setItem('mcViewMode', v))
const openMenuId = ref<string | null>(null)

function closeMenu(e: MouseEvent) {
  const t = e.target as HTMLElement
  if (!t.closest('.cluster-list-menu-wrap')) openMenuId.value = null
  if (!t.closest('.filter-dropdown-wrap')) activeDropdown.value = null
}
onMounted(() => document.addEventListener('click', closeMenu))
onUnmounted(() => document.removeEventListener('click', closeMenu))

// Status filters — only show statuses present in the data

const allStatusDefs = [
  { key: 'failed',    label: 'Failed' },
  { key: 'managed',   label: 'Managed' },
  { key: 'outofsync', label: 'Out of Sync' },
  { key: 'synced',    label: 'Synced' },
  { key: 'unmanaged', label: 'Unmanaged' },
]
const statusFilters = computed(() => {
  const present = new Set<string>()
  ;(state.value?.machineClasses ?? []).forEach(m => {
    const key = (m.status === 'success' || m.status === 'applied') ? 'synced' : (m.status || '')
    if (key) { present.add(key); present.add('managed') }
  })
  return allStatusDefs.filter(d => present.has(d.key))
})
const activeFilters = ref(new Set<string>())
const activeFilterCount = computed(() => activeFilters.value.size)
const activeFilterLabels = computed(() =>
  allStatusDefs.filter(d => activeFilters.value.has(d.key)).map(d => d.label).join(', ')
)
function toggleFilter(key: string) {
  if (activeFilters.value.has(key)) activeFilters.value.delete(key)
  else activeFilters.value.add(key)
  activeFilters.value = new Set(activeFilters.value)
  currentPage.value = 1
}
function clearFilters() {
  activeFilters.value = new Set()
  currentPage.value = 1
}

const reconcileRunning = computed(() => state.value?.lastReconcile?.status === 'running')
const refreshPending = ref(false)
const isRunning = computed(() => reconcileRunning.value || refreshPending.value)

const machineClasses = computed(() => {
  const list = (state.value?.machineClasses ?? []).slice()
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

const filteredMCs = computed(() => {
  let list = machineClasses.value
  if (mcSearch.value.trim()) {
    const q = mcSearch.value.trim().toLowerCase()
    list = list.filter(m => m.id.toLowerCase().includes(q))
  }
  if (activeFilters.value.size > 0) {
    list = list.filter(m => {
      const st = m.status || ''
      const isManaged = st !== 'unmanaged'
      const key = (st === 'success' || st === 'applied') ? 'synced' : st
      return (activeFilters.value.has('managed') && isManaged) || activeFilters.value.has(key)
    })
  }
  return list
})

const totalPages = computed(() =>
  pageSize.value === 0 ? 1 : Math.max(1, Math.ceil(filteredMCs.value.length / pageSize.value))
)
const pageMCs = computed(() => {
  if (pageSize.value === 0) return filteredMCs.value
  const start = (currentPage.value - 1) * pageSize.value
  return filteredMCs.value.slice(start, start + pageSize.value)
})

// Detail modal
const detailModal = ref<ResourceInfo | null>(null)
const detailTab = ref<'live' | 'diff'>('live')
const showIgnoredFields = ref(false)

interface McRow {
  lineNum: number | null
  content: string
  isPlaceholder: boolean
  isDim: boolean
}

// Normalize a line for comparison: collapse leading whitespace to a single level
// and strip trailing whitespace so indentation style differences don't create false diffs.
function normalizeLine(s: string): string {
  return s.trimEnd().replace(/^\s+/, ' ')
}

const ALWAYS_SHOW_META = /^\s+(namespace|id|type)\s*:/

function buildMcRows(content: string, showMeta: boolean): McRow[] {
  const text = (content || '').replace(/\\n/g, '\n')
  const lines = text.split('\n')
  let metaStart = -1
  let metaEnd = lines.length
  for (let i = 0; i < lines.length; i++) {
    if (/^metadata:/.test(lines[i])) { metaStart = i; break }
  }
  if (metaStart >= 0) {
    for (let j = metaStart + 1; j < lines.length; j++) {
      if (lines[j].length > 0 && !/^\s/.test(lines[j])) { metaEnd = j; break }
    }
  }
  const rows: McRow[] = []
  let hiddenCount = 0
  for (let i = 0; i < lines.length; i++) {
    const isMeta = metaStart >= 0 && i >= metaStart && i < metaEnd
    const isAlwaysShown = i === metaStart || ALWAYS_SHOW_META.test(lines[i])
    if (isMeta && !isAlwaysShown && !showMeta) {
      hiddenCount++
      continue
    }
    if (hiddenCount > 0) {
      rows.push({ lineNum: null, content: `(${hiddenCount} fields hidden)`, isPlaceholder: true, isDim: false })
      hiddenCount = 0
    }
    rows.push({ lineNum: i + 1, content: lines[i], isPlaceholder: false, isDim: isMeta && !isAlwaysShown })
  }
  if (hiddenCount > 0) {
    rows.push({ lineNum: null, content: `(${hiddenCount} fields hidden)`, isPlaceholder: true, isDim: false })
  }
  return rows
}

function extractDoc(content: string, id: string): string {
  const text = (content || '').replace(/\\n/g, '\n')
  const docs = text.split(/\n---/)
  for (const d of docs) {
    if (d.includes('id: ' + id)) return d
  }
  return text
}

const detailLiveRows = computed(() =>
  buildMcRows(extractDoc(detailModal.value?.liveContent || '', detailModal.value?.id || ''), showIgnoredFields.value)
)

const detailDiffRows = computed(() => {
  const id = detailModal.value?.id || ''
  const live = extractDoc(detailModal.value?.liveContent || '', id)
  const file = extractDoc(detailModal.value?.fileContent || '', id)
  if (!live && !file) return []
  // Always build with showMeta=false so server-injected metadata fields
  // (version, owner, phase, etc.) are never included in the comparison
  const lRows = buildMcRows(live, false).filter(r => !r.isPlaceholder)
  const rRows = buildMcRows(file, false).filter(r => !r.isPlaceholder)
  const lAligned: (McRow | null)[] = []
  const rAligned: (McRow | null)[] = []
  let li = 0, ri = 0
  while (li < lRows.length || ri < rRows.length) {
    const l = li < lRows.length ? lRows[li] : null
    const r = ri < rRows.length ? rRows[ri] : null
    if (l && l.isPlaceholder) {
      lAligned.push(l); rAligned.push(null); li++
    } else if (r && r.isPlaceholder) {
      lAligned.push(null); rAligned.push(r); ri++
    } else {
      lAligned.push(l); rAligned.push(r)
      if (l) li++; if (r) ri++
    }
  }
  return lAligned.map((l, i) => {
    const r = rAligned[i]
    const changed = !!(
      (l && !r) ||
      (!l && r) ||
      (l && r && !l.isPlaceholder && !r.isPlaceholder && normalizeLine(l.content) !== normalizeLine(r.content))
    )
    return { l, r, changed }
  })
})

type DiffViewRow =
  | { separator: true }
  | { separator: false; seq: number; l: McRow | null; r: McRow | null; changed: boolean }

const DIFF_CONTEXT = 2

const detailDiffVisible = computed((): DiffViewRow[] => {
  const rows = detailDiffRows.value
  if (!rows.length) return []
  const show = new Set<number>()
  rows.forEach((row, i) => {
    if (row.changed) {
      for (let j = Math.max(0, i - DIFF_CONTEXT); j <= Math.min(rows.length - 1, i + DIFF_CONTEXT); j++) {
        show.add(j)
      }
    }
  })
  if (!show.size) return []
  const result: DiffViewRow[] = []
  let lastShown = -1
  let seq = 0
  for (const i of Array.from(show).sort((a, b) => a - b)) {
    if (lastShown >= 0 && i > lastShown + 1) result.push({ separator: true })
    seq++
    result.push({ separator: false, seq, ...rows[i] })
    lastShown = i
  }
  return result
})

function openDetail(mc: ResourceInfo) {
  detailModal.value = mc
  detailTab.value = mc.liveContent ? 'live' : 'diff'
  showIgnoredFields.value = false
}

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

function setSort(key: SortKey) { sortKey.value = key; activeDropdown.value = null; currentPage.value = 1 }
function setPageSize(n: number) { pageSize.value = n; currentPage.value = 1 }

function hasDetails(mc: ResourceInfo): boolean {
  return !!(mc.diff || mc.fileContent || mc.liveContent)
}

function provisionType(mc: ResourceInfo): string {
  if (mc.provisionType) return mc.provisionType
  const content = mc.fileContent || mc.liveContent || ''
  return content.toLowerCase().includes('providerid:') ? 'auto' : 'manual'
}

interface MCSpec {
  providerId: string
  matchLabels: Record<string, string>
  providerData: Record<string, string>
}

function mcSpec(mc: ResourceInfo): MCSpec {
  const yaml = mc.fileContent || mc.liveContent || ''
  const text = yaml.replace(/\\n/g, '\n')
  let src = text
  const docs = text.split(/\n---/)
  for (const d of docs) {
    if (d.includes('id: ' + mc.id)) { src = d; break }
  }
  const lines = src.split('\n')
  const result: MCSpec = { matchLabels: {}, providerId: '', providerData: {} }
  let i = 0
  while (i < lines.length) {
    const line = lines[i]
    const trimmed = line.trim()
    if (!trimmed) { i++; continue }
    const indent = line.search(/\S/)

    if (/^matchlabels:/i.test(trimmed)) {
      const base = indent
      i++
      while (i < lines.length) {
        const sl = lines[i]; const st = sl.trim()
        if (!st) { i++; continue }
        if (sl.search(/\S/) <= base) break
        const li = st.match(/^-\s+(.+)$/)
        if (li) {
          const item = li[1].trim()
          const eq = item.match(/^([^=]+?)\s*=\s*(.*)$/)
          if (eq) { result.matchLabels[eq[1].trim()] = eq[2].trim() }
          else { const co = item.match(/^([^:]+?):\s*(.*)$/); if (co) result.matchLabels[co[1].trim()] = co[2].trim() }
        } else {
          const kv = st.match(/^([^:]+?):\s*(.*)$/)
          if (kv) result.matchLabels[kv[1].trim()] = kv[2].trim()
        }
        i++
      }
      continue
    }

    if (/^providerId:/i.test(trimmed)) {
      result.providerId = trimmed.replace(/^providerId:\s*/i, '').trim()
    }

    if (/^providerData:\s*\|/i.test(trimmed)) {
      const base2 = indent
      i++
      while (i < lines.length) {
        const sl2 = lines[i]; const st2 = sl2.trim()
        if (!st2) { i++; continue }
        if (sl2.search(/\S/) <= base2) break
        const kv2 = st2.match(/^([^:]+):\s*(.*)$/)
        if (kv2) result.providerData[kv2[1].trim()] = kv2[2].trim()
        i++
      }
      continue
    }

    i++
  }
  return result
}

function mcRepoBranch(mc: ResourceInfo): string | null {
  if (!mc.repoName || !state.value?.repos) return null
  const repo = state.value.repos.find(r => r.name === mc.repoName)
  return repo?.branch ?? null
}

function syncStatusColor(status: string): string {
  if (status === 'success' || status === 'applied') return '#4ade80'
  if (status === 'failed') return '#f87171'
  if (status === 'outofsync') return '#fb923c'
  if (status === 'syncing') return '#2dd4bf'
  return '#7d7d85'
}

function syncStatusText(mc: ResourceInfo): string {
  const s = mc.status
  if (s === 'success' || s === 'applied') return syncedIconSVG + ' Synced'
  if (s === 'failed') return failedIconSVG + ' Failed'
  if (s === 'outofsync') return outOfSyncIconSVG + ' Out of Sync'
  if (s === 'syncing') return '● Syncing'
  return s || '—'
}

function clustersUsingMC(mcId: string): string[] {
  return (state.value?.clusters ?? [])
    .filter(c => {
      if (c.controlPlane?.machineClass === mcId) return true
      return (c.workers || []).some(w => w.machineClass === mcId)
    })
    .map(c => c.id)
    .sort()
}

function isZeroTime(d?: string): boolean {
  if (!d) return true
  const dt = new Date(d)
  return isNaN(dt.getTime()) || dt.getFullYear() <= 1
}

function fmtDateTime(d?: string): string {
  if (!d) return '—'
  return new Date(d).toLocaleString()
}

function ago(d?: string): string {
  if (!d) return ''
  const diff = Date.now() - new Date(d).getTime()
  const s = Math.floor(diff / 1000)
  if (s < 60) return s + 's ago'
  const m = Math.floor(s / 60)
  if (m < 60) return m + 'm ago'
  const h = Math.floor(m / 60)
  if (h < 24) return h + 'h ago'
  return Math.floor(h / 24) + 'd ago'
}

function goToCluster(id: string) {
  router.push(`/clusters/${encodeURIComponent(id)}`)
}

async function doRefreshMC() {
  refreshPending.value = true
  try {
    await fetch('/api/refresh-mc', { method: 'POST' })
    setTimeout(() => { refreshPending.value = false }, 5000)
  } catch { refreshPending.value = false }
}

async function doSyncAll() {
  await fetch('/api/reconcile', { method: 'POST' })
}

async function refreshSingleMC(mc: ResourceInfo) {
  if (actionPending[mc.id]) return
  actionPending[mc.id] = 'refresh'
  try {
    await fetch('/api/refresh-single-mc', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ id: mc.id }),
    })
    setTimeout(() => { delete actionPending[mc.id] }, 5000)
  } catch { delete actionPending[mc.id] }
}

async function syncMC(mc: ResourceInfo) {
  if (actionPending[mc.id]) return
  actionPending[mc.id] = 'sync'
  try {
    await fetch('/api/sync-machineclass', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ id: mc.id }),
    })
    await fetch('/api/reconcile', { method: 'POST' })
    setTimeout(() => { delete actionPending[mc.id] }, 5000)
  } catch { delete actionPending[mc.id] }
}

async function toggleMCAutoSync(mc: ResourceInfo, event: MouseEvent) {
  (event.currentTarget as HTMLElement).blur()
  await fetch('/api/set-mc-autosync', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ id: mc.id, autoSync: mc.autoSync !== true }),
  })
}

function exportMC(mc: ResourceInfo) {
  const content = mc.fileContent || mc.liveContent || ''
  if (!content) return
  const blob = new Blob([content], { type: 'text/yaml' })
  const a = document.createElement('a')
  a.href = URL.createObjectURL(blob)
  a.download = mc.id + '.yaml'
  a.click()
}

function deleteMC(mc: ResourceInfo) {
  confirmInput.value = ''
  confirmModal.value = {
    title: 'Delete Machine Class',
    message: `Are you sure you want to delete <b>${mc.id}</b>?<br><br>This will permanently remove it from Omni.`,
    requireInput: mc.id,
    inputPrompt: `Type '${mc.id}' to confirm`,
    onConfirm: async () => {
      confirmModal.value = null
      confirmInput.value = ''
      await fetch('/api/delete-machineclass', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ id: mc.id }),
      })
    },
  }
}
</script>
