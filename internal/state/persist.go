package state

import (
	"log/slog"

	"omni-cd/internal/config"
	"omni-cd/internal/maintenance"
)

// Store persists AppState across restarts. Implementations live in
// internal/storage.
type Store interface {
	// LoadState returns nil, nil when no state has been stored yet.
	LoadState() (*PersistedState, error)
	SaveState(ps PersistedState) error
	// LoadRepoConfigs returns the UI-managed repo configs (never config-file ones).
	LoadRepoConfigs() ([]config.RepoConfig, error)
	SaveRepoConfigs(repos []config.RepoConfig) error
}

// PersistedState is the subset of AppState that survives restarts.
type PersistedState struct {
	LastReconcile       ReconcileInfo        `json:"lastReconcile"`
	RepoClusterMap      map[string][]string  `json:"repoClusterMap,omitempty"`
	RepoMachineClassMap map[string][]string  `json:"repoMachineClassMap,omitempty"`
	Clusters            []ResourceInfo       `json:"clusters,omitempty"`
	MachineClasses      []ResourceInfo       `json:"machineClasses,omitempty"`
	ClustersEnabled     bool                 `json:"clustersEnabled"`
	MaintenanceWindows  []maintenance.Window `json:"maintenanceWindows,omitempty"`
}

// save persists state (best-effort, logs errors).
func (s *AppState) save() {
	if err := s.saveState(); err != nil {
		slog.Error("Failed to persist state", "error", err, "component", "State")
	}
}

// Save persists state (public method for external callers).
func (s *AppState) Save() {
	s.save()
}

// saveState persists state to the store. Full cluster and MC data is stored so
// the UI can display last-known state during the window between server start
// and the first reconcile completing.
//
// The snapshot is built while RLock is held so concurrent mutations cannot
// tear the slices/maps mid-encode; the store gets its own copy.
func (s *AppState) saveState() error {
	if s.store == nil {
		return nil
	}
	s.mu.RLock()

	// Strip transient in-progress statuses so a crash or restart never
	// leaves resources stuck in "syncing" or "deleting" on next boot.
	filteredClusters := make([]ResourceInfo, 0, len(s.Clusters))
	for _, c := range s.Clusters {
		if c.Status == "syncing" || c.Status == "deleting" {
			continue
		}
		filteredClusters = append(filteredClusters, c)
	}
	filteredMCs := make([]ResourceInfo, 0, len(s.MachineClasses))
	for _, m := range s.MachineClasses {
		if m.Status == "syncing" {
			continue
		}
		filteredMCs = append(filteredMCs, m)
	}

	ps := PersistedState{
		LastReconcile:       s.LastReconcile,
		RepoClusterMap:      cloneStringSliceMap(s.RepoClusterMap),
		RepoMachineClassMap: cloneStringSliceMap(s.RepoMachineClassMap),
		Clusters:            cloneResources(filteredClusters),
		MachineClasses:      cloneResources(filteredMCs),
		ClustersEnabled:     s.ClustersEnabled,
		MaintenanceWindows:  s.maintenanceWindows,
	}
	s.mu.RUnlock()

	return s.store.SaveState(ps)
}

// loadState restores state from the store. Full cluster and MC data is
// restored so the UI shows last-known state immediately on startup.
func (s *AppState) loadState() error {
	if s.store == nil {
		return nil
	}
	ps, err := s.store.LoadState()
	if err != nil || ps == nil {
		return err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	s.LastReconcile = ps.LastReconcile
	s.ClustersEnabled = ps.ClustersEnabled

	if ps.RepoClusterMap != nil {
		s.RepoClusterMap = ps.RepoClusterMap
	}
	if ps.RepoMachineClassMap != nil {
		s.RepoMachineClassMap = ps.RepoMachineClassMap
	}

	if len(ps.Clusters) > 0 {
		s.Clusters = ps.Clusters
		// Migration: apply AutoSync=false to any cluster without an explicit preference.
		for i := range s.Clusters {
			if s.Clusters[i].AutoSync == nil {
				f := false
				s.Clusters[i].AutoSync = &f
			}
		}
	}

	if len(ps.MachineClasses) > 0 {
		s.MachineClasses = ps.MachineClasses
	}

	s.maintenanceWindows = ps.MaintenanceWindows

	return nil
}
