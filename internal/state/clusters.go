package state

import (
	"time"

	"omni-cd/internal/omni"
)

// transientClusterFields holds polling-derived fields that must survive a
// SetClusters call (which uses freshly-built structs from the reconciler).
type transientClusterFields struct {
	ClusterReady       string
	KubernetesAPIReady string
	ControlplaneReady  string
	ClusterPhase       string
	MachinesHealthy    int
	MachinesTotal      int
	EtcdStatus         string
	WireGuardStatus    string
	LastBackupTime     time.Time
	BackupEnabled      bool
	LastSyncResult     string
	LastSyncError      string
	LastSyncTime       time.Time
	LastSyncSHA        string
	LastSyncAuthor     string
	LastSyncMessage    string
	SyncStatusSince    time.Time
	CreatedAt          time.Time
	Error              string
}

// GetClusters returns a copy of the current cluster list.
func (s *AppState) GetClusters() []ResourceInfo {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]ResourceInfo, len(s.Clusters))
	copy(out, s.Clusters)
	return out
}

// SetClusters replaces the cluster list, preserving transient fields and
// per-cluster AutoSync preferences.
func (s *AppState) SetClusters(resources []ResourceInfo) {
	s.mu.Lock()
	existing := make(map[string]transientClusterFields, len(s.Clusters))
	autoSyncMap := make(map[string]*bool, len(s.Clusters))
	statusMap := make(map[string]string, len(s.Clusters))
	for _, c := range s.Clusters {
		existing[c.ID] = transientClusterFields{
			ClusterReady:       c.ClusterReady,
			KubernetesAPIReady: c.KubernetesAPIReady,
			ControlplaneReady:  c.ControlplaneReady,
			ClusterPhase:       c.ClusterPhase,
			MachinesHealthy:    c.MachinesHealthy,
			MachinesTotal:      c.MachinesTotal,
			EtcdStatus:         c.EtcdStatus,
			WireGuardStatus:    c.WireGuardStatus,
			LastBackupTime:     c.LastBackupTime,
			BackupEnabled:      c.BackupEnabled,
			LastSyncResult:     c.LastSyncResult,
			LastSyncError:      c.LastSyncError,
			LastSyncTime:       c.LastSyncTime,
			LastSyncSHA:        c.LastSyncSHA,
			LastSyncAuthor:     c.LastSyncAuthor,
			LastSyncMessage:    c.LastSyncMessage,
			SyncStatusSince:    c.SyncStatusSince,
			CreatedAt:          c.CreatedAt,
			Error:              c.Error,
		}
		autoSyncMap[c.ID] = c.AutoSync
		statusMap[c.ID] = c.Status
	}
	for i := range resources {
		if f, ok := existing[resources[i].ID]; ok {
			resources[i].ClusterReady = f.ClusterReady
			resources[i].KubernetesAPIReady = f.KubernetesAPIReady
			resources[i].ControlplaneReady = f.ControlplaneReady
			resources[i].ClusterPhase = f.ClusterPhase
			resources[i].MachinesHealthy = f.MachinesHealthy
			resources[i].MachinesTotal = f.MachinesTotal
			resources[i].EtcdStatus = f.EtcdStatus
			resources[i].WireGuardStatus = f.WireGuardStatus
			resources[i].LastBackupTime = f.LastBackupTime
			resources[i].BackupEnabled = f.BackupEnabled
			if resources[i].LastSyncResult == "" {
				resources[i].LastSyncResult = f.LastSyncResult
				resources[i].LastSyncError = f.LastSyncError
				resources[i].LastSyncTime = f.LastSyncTime
				resources[i].LastSyncSHA = f.LastSyncSHA
				resources[i].LastSyncAuthor = f.LastSyncAuthor
				resources[i].LastSyncMessage = f.LastSyncMessage
				// Preserve the error when no apply was attempted (diff-only or auto-sync
				// disabled). Clear it only when the resource is now fully in sync.
				if resources[i].Status == "outofsync" && resources[i].Error == "" {
					resources[i].Error = f.Error
				}
			}
			if resources[i].SyncStatusSince.IsZero() {
				resources[i].SyncStatusSince = f.SyncStatusSince
			}
			if resources[i].CreatedAt.IsZero() {
				resources[i].CreatedAt = f.CreatedAt
			}
			if statusMap[resources[i].ID] == "deleting" {
				resources[i].Status = "deleting"
			}
			if autoSyncMap[resources[i].ID] != nil {
				resources[i].AutoSync = autoSyncMap[resources[i].ID]
			} else {
				f := false
				resources[i].AutoSync = &f
			}
		} else {
			f := false
			resources[i].AutoSync = &f
		}
	}
	s.Clusters = resources
	s.mu.Unlock()
	s.notifyChange()
}

// RemoveCluster removes a cluster from state by ID. No-op if not found.
func (s *AppState) RemoveCluster(id string) {
	s.mu.Lock()
	for i, c := range s.Clusters {
		if c.ID == id {
			s.Clusters = append(s.Clusters[:i], s.Clusters[i+1:]...)
			s.mu.Unlock()
			s.notifyChange()
			return
		}
	}
	s.mu.Unlock()
}

// UpdateClusterStatus updates the status of a single cluster by ID.
func (s *AppState) UpdateClusterStatus(id, status string) {
	s.mu.Lock()
	for i := range s.Clusters {
		if s.Clusters[i].ID == id {
			s.Clusters[i].Status = status
			s.mu.Unlock()
			s.notifyChange()
			return
		}
	}
	s.mu.Unlock()
}

// UpsertClusterStatus updates the status of a cluster by ID, adding a minimal
// entry if it is not yet tracked.
func (s *AppState) UpsertClusterStatus(id, status string) {
	s.mu.Lock()
	for i := range s.Clusters {
		if s.Clusters[i].ID == id {
			if s.Clusters[i].Status == "deleting" {
				s.mu.Unlock()
				return
			}
			s.Clusters[i].Status = status
			s.mu.Unlock()
			s.notifyChange()
			return
		}
	}
	disabled := false
	s.Clusters = append(s.Clusters, ResourceInfo{
		ID:       id,
		Type:     "Cluster",
		Status:   status,
		AutoSync: &disabled,
	})
	s.mu.Unlock()
	s.notifyChange()
}

// UpsertClusterInfo replaces the full ResourceInfo for one cluster, or appends
// it if the cluster is not yet in state. Live health fields and AutoSync are preserved.
func (s *AppState) UpsertClusterInfo(id string, info ResourceInfo) {
	s.mu.Lock()
	for i := range s.Clusters {
		if s.Clusters[i].ID == id {
			if s.Clusters[i].Status == "deleting" {
				s.mu.Unlock()
				return
			}
			info.ClusterReady = s.Clusters[i].ClusterReady
			info.KubernetesAPIReady = s.Clusters[i].KubernetesAPIReady
			info.ControlplaneReady = s.Clusters[i].ControlplaneReady
			info.ClusterPhase = s.Clusters[i].ClusterPhase
			info.MachinesHealthy = s.Clusters[i].MachinesHealthy
			info.MachinesTotal = s.Clusters[i].MachinesTotal
			info.EtcdStatus = s.Clusters[i].EtcdStatus
			info.WireGuardStatus = s.Clusters[i].WireGuardStatus
			info.LastBackupTime = s.Clusters[i].LastBackupTime
			info.BackupEnabled = s.Clusters[i].BackupEnabled
			info.AutoSync = s.Clusters[i].AutoSync
			if info.LastSyncResult == "" {
				info.LastSyncResult = s.Clusters[i].LastSyncResult
				info.LastSyncError = s.Clusters[i].LastSyncError
				info.LastSyncTime = s.Clusters[i].LastSyncTime
				info.LastSyncSHA = s.Clusters[i].LastSyncSHA
				info.LastSyncAuthor = s.Clusters[i].LastSyncAuthor
				info.LastSyncMessage = s.Clusters[i].LastSyncMessage
				if info.Status == "outofsync" && info.Error == "" {
					info.Error = s.Clusters[i].Error
				}
			}
			if info.SyncStatusSince.IsZero() {
				info.SyncStatusSince = s.Clusters[i].SyncStatusSince
			}
			if info.CreatedAt.IsZero() {
				info.CreatedAt = s.Clusters[i].CreatedAt
			}
			if info.RepoName == "" {
				info.RepoName = s.Clusters[i].RepoName
			}
			s.Clusters[i] = info
			s.mu.Unlock()
			s.notifyChange()
			return
		}
	}
	if info.AutoSync == nil {
		disabled := false
		info.AutoSync = &disabled
	}
	s.Clusters = append(s.Clusters, info)
	s.mu.Unlock()
	s.notifyChange()
}

// UpdateClusterReadyStatuses updates live health fields for all clusters.
// Returns true if any value changed.
func (s *AppState) UpdateClusterReadyStatuses(statuses map[string]omni.ClusterStatus) bool {
	s.mu.Lock()
	changed := false
	for i := range s.Clusters {
		var newReady, newAPIReady, newCPReady, newPhase string
		var newMachinesHealthy, newMachinesTotal int
		var newEtcd, newWireGuard string
		var newLastBackup time.Time
		var newBackupEnabled bool
		if st, ok := statuses[s.Clusters[i].ID]; ok {
			if st.Ready {
				newReady = "ready"
			} else {
				newReady = "not-ready"
			}
			if st.KubernetesAPIReady {
				newAPIReady = "ready"
			} else {
				newAPIReady = "not-ready"
			}
			if st.ControlPlaneReady {
				newCPReady = "ready"
			} else {
				newCPReady = "not-ready"
			}
			newMachinesHealthy = st.MachinesHealthy
			newMachinesTotal = st.MachinesTotal
			newEtcd = st.EtcdStatus
			newWireGuard = st.WireGuardStatus
			newLastBackup = st.LastBackupTime
			newBackupEnabled = st.BackupEnabled
			newPhase = st.Phase
		} else {
			newReady = "unknown"
			newAPIReady = "unknown"
			newCPReady = "unknown"
			newEtcd = "unknown"
			newWireGuard = "unknown"
			newPhase = "unknown"
		}
		c := &s.Clusters[i]
		if c.ClusterReady != newReady || c.KubernetesAPIReady != newAPIReady ||
			c.ControlplaneReady != newCPReady || c.ClusterPhase != newPhase ||
			c.MachinesHealthy != newMachinesHealthy || c.MachinesTotal != newMachinesTotal ||
			c.EtcdStatus != newEtcd || c.WireGuardStatus != newWireGuard ||
			!c.LastBackupTime.Equal(newLastBackup) || c.BackupEnabled != newBackupEnabled {
			c.ClusterReady = newReady
			c.KubernetesAPIReady = newAPIReady
			c.ControlplaneReady = newCPReady
			c.ClusterPhase = newPhase
			c.MachinesHealthy = newMachinesHealthy
			c.MachinesTotal = newMachinesTotal
			c.EtcdStatus = newEtcd
			c.WireGuardStatus = newWireGuard
			c.LastBackupTime = newLastBackup
			c.BackupEnabled = newBackupEnabled
			changed = true
		}
	}
	s.mu.Unlock()
	if changed {
		s.notifyChange()
	}
	return changed
}

// UpdateTearingDownStatuses marks clusters as "deleting" when tearing down in Omni.
// Returns true if any cluster status changed.
func (s *AppState) UpdateTearingDownStatuses(allIDs []string, tearingDown map[string]bool) bool {
	omniMap := make(map[string]bool, len(allIDs))
	for _, id := range allIDs {
		omniMap[id] = true
	}
	s.mu.Lock()
	changed := false
	filtered := s.Clusters[:0]
	for _, c := range s.Clusters {
		// Remove clusters that have fully disappeared from Omni.
		if !omniMap[c.ID] && (c.Status == "deleting" || c.Status == "orphaned" || c.Status == "unmanaged") {
			changed = true
			continue
		}
		filtered = append(filtered, c)
	}
	s.Clusters = filtered
	s.mu.Unlock()
	if changed {
		s.notifyChange()
	}
	return changed
}

// GetClustersEnabled returns the current clusters enabled state.
func (s *AppState) GetClustersEnabled() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.ClustersEnabled
}

// GetClusterAutoSync returns true if the cluster should be automatically applied.
func (s *AppState) GetClusterAutoSync(id string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, c := range s.Clusters {
		if c.ID == id {
			if c.AutoSync != nil {
				return *c.AutoSync
			}
			return false
		}
	}
	return false
}

// SetClusterAutoSync sets the per-cluster AutoSync preference and persists state.
func (s *AppState) SetClusterAutoSync(id string, enabled bool) {
	s.mu.Lock()
	for i := range s.Clusters {
		if s.Clusters[i].ID == id {
			s.Clusters[i].AutoSync = &enabled
			s.mu.Unlock()
			s.save()
			s.notifyChange()
			return
		}
	}
	s.mu.Unlock()
}

// SetClustersEnabled sets the clusters enabled state.
func (s *AppState) SetClustersEnabled(enabled bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.ClustersEnabled = enabled
}

// ToggleClustersEnabled flips the clusters enabled state and returns the new value.
func (s *AppState) ToggleClustersEnabled() bool {
	s.mu.Lock()
	s.ClustersEnabled = !s.ClustersEnabled
	newState := s.ClustersEnabled
	s.mu.Unlock()
	s.save()
	return newState
}

// AddForceClusterID queues a cluster ID to be force-synced on the next reconcile.
func (s *AppState) AddForceClusterID(id string) {
	s.mu.Lock()
	if s.forceClusterIDs == nil {
		s.forceClusterIDs = make(map[string]bool)
	}
	s.forceClusterIDs[id] = true
	s.mu.Unlock()
}

// GetAndClearForceClusterIDs returns all queued force-sync IDs and clears the set.
func (s *AppState) GetAndClearForceClusterIDs() map[string]bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	ids := s.forceClusterIDs
	s.forceClusterIDs = nil
	return ids
}

// HasForceClusterIDs returns true if any cluster IDs are queued for force sync.
func (s *AppState) HasForceClusterIDs() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.forceClusterIDs) > 0
}
