package state

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// persistedState is the struct written to the state file.
type persistedState struct {
	LastReconcile       ReconcileInfo       `json:"lastReconcile"`
	RepoClusterMap      map[string][]string `json:"repoClusterMap,omitempty"`
	RepoMachineClassMap map[string][]string `json:"repoMachineClassMap,omitempty"`
	Clusters            []ResourceInfo      `json:"clusters,omitempty"`
	MachineClasses      []ResourceInfo      `json:"machineClasses,omitempty"`
	ClustersEnabled     bool                `json:"clustersEnabled"`
}

// save persists state to disk (best-effort, ignores errors).
func (s *AppState) save() {
	if s.stateFile != "" {
		_ = s.SaveToFile(s.stateFile)
	}
}

// Save persists state to disk (public method for external callers).
func (s *AppState) Save() {
	s.save()
}

// SaveToFile persists state to disk. Full cluster and MC data is stored so
// the UI can display last-known state during the window between server start
// and the first reconcile completing.
func (s *AppState) SaveToFile(path string) error {
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

	ps := persistedState{
		LastReconcile:       s.LastReconcile,
		RepoClusterMap:      s.RepoClusterMap,
		RepoMachineClassMap: s.RepoMachineClassMap,
		Clusters:            filteredClusters,
		MachineClasses:      filteredMCs,
		ClustersEnabled:     s.ClustersEnabled,
	}

	s.mu.RUnlock()

	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}

	data, err := json.MarshalIndent(ps, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(path, data, 0600)
}

// LoadFromFile restores state from disk. Full cluster and MC data is restored
// so the UI shows last-known state immediately on startup.
func (s *AppState) LoadFromFile(path string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, err := os.Stat(path); os.IsNotExist(err) {
		return nil
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	var ps persistedState
	if err := json.Unmarshal(data, &ps); err != nil {
		return err
	}

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

	return nil
}
