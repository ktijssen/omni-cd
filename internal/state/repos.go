package state

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"omni-cd/internal/config"
)

// SetRepoFile sets the path used by SaveRepoConfigs / LoadRepoConfigs.
func (s *AppState) SetRepoFile(path string) {
	s.mu.Lock()
	s.repoFile = path
	s.mu.Unlock()
}

// SetRepoConfigs replaces the full repo config list (used at startup).
func (s *AppState) SetRepoConfigs(repos []config.RepoConfig) {
	s.mu.Lock()
	s.RepoConfigs = repos
	s.mu.Unlock()
	s.notifyChange()
}

// GetRepoConfigs returns a copy of the current repo config list.
func (s *AppState) GetRepoConfigs() []config.RepoConfig {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]config.RepoConfig, len(s.RepoConfigs))
	copy(out, s.RepoConfigs)
	return out
}

// AddRepoConfig adds a new repo. Returns an error if the name already exists.
func (s *AppState) AddRepoConfig(rc config.RepoConfig) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, r := range s.RepoConfigs {
		if r.Name == rc.Name {
			return fmt.Errorf("repo %q already exists", rc.Name)
		}
	}
	s.RepoConfigs = append(s.RepoConfigs, rc)
	return nil
}

// UpdateRepoConfig replaces the repo with the given name.
// If rc.Token is empty the existing token is preserved.
// If rc.Token is the sentinel "\x00clear\x00" the token is removed.
func (s *AppState) UpdateRepoConfig(name string, rc config.RepoConfig) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i, r := range s.RepoConfigs {
		if r.Name == name {
			if rc.Token == "\x00clear\x00" {
				rc.Token = ""
			} else if rc.Token == "" {
				rc.Token = r.Token
			}
			s.RepoConfigs[i] = rc
			return nil
		}
	}
	return fmt.Errorf("repo %q not found", name)
}

// DeleteRepoConfig removes the repo with the given name.
func (s *AppState) DeleteRepoConfig(name string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i, r := range s.RepoConfigs {
		if r.Name == name {
			s.RepoConfigs = append(s.RepoConfigs[:i], s.RepoConfigs[i+1:]...)
			return nil
		}
	}
	return fmt.Errorf("repo %q not found", name)
}

// SetRepoClusters records which cluster IDs are managed by the named repo.
func (s *AppState) SetRepoClusters(repoName string, clusterIDs []string) {
	s.mu.Lock()
	if s.RepoClusterMap == nil {
		s.RepoClusterMap = make(map[string][]string)
	}
	s.RepoClusterMap[repoName] = clusterIDs
	s.mu.Unlock()
}

// GetRepoClusters returns the last known cluster IDs for the named repo.
// The returned slice is a copy — safe to iterate concurrently with writers.
func (s *AppState) GetRepoClusters(repoName string) []string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	v, ok := s.RepoClusterMap[repoName]
	if !ok {
		return nil
	}
	return append([]string(nil), v...)
}

// GetAllTrackedClusterIDs returns the union of all cluster IDs managed by any repo.
func (s *AppState) GetAllTrackedClusterIDs() map[string]bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make(map[string]bool)
	for _, ids := range s.RepoClusterMap {
		for _, id := range ids {
			out[id] = true
		}
	}
	return out
}

// ClearRepoMaps resets the repo→cluster and repo→machine-class maps.
func (s *AppState) ClearRepoMaps() {
	s.mu.Lock()
	s.RepoClusterMap = nil
	s.RepoMachineClassMap = nil
	s.mu.Unlock()
}

// SetRepoMachineClasses records which machine class IDs are managed by the named repo.
func (s *AppState) SetRepoMachineClasses(repoName string, mcIDs []string) {
	s.mu.Lock()
	if s.RepoMachineClassMap == nil {
		s.RepoMachineClassMap = make(map[string][]string)
	}
	s.RepoMachineClassMap[repoName] = mcIDs
	s.mu.Unlock()
}

// GetRepoMachineClasses returns the last known machine class IDs for the named repo.
// The returned slice is a copy — safe to iterate concurrently with writers.
func (s *AppState) GetRepoMachineClasses(repoName string) []string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	v, ok := s.RepoMachineClassMap[repoName]
	if !ok {
		return nil
	}
	return append([]string(nil), v...)
}

// GetAllTrackedMachineClassIDs returns the union of all machine class IDs managed by any repo.
func (s *AppState) GetAllTrackedMachineClassIDs() map[string]bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make(map[string]bool)
	for _, ids := range s.RepoMachineClassMap {
		for _, id := range ids {
			out[id] = true
		}
	}
	return out
}

// StampRepoNameOnClusters sets RepoName on each cluster in the list that
// doesn't already have one.
func (s *AppState) StampRepoNameOnClusters(repoName string, clusterIDs []string) {
	if len(clusterIDs) == 0 {
		return
	}
	idSet := make(map[string]bool, len(clusterIDs))
	for _, id := range clusterIDs {
		idSet[id] = true
	}
	s.mu.Lock()
	changed := false
	for i := range s.Clusters {
		if idSet[s.Clusters[i].ID] && s.Clusters[i].RepoName == "" {
			s.Clusters[i].RepoName = repoName
			changed = true
		}
	}
	s.mu.Unlock()
	if changed {
		s.notifyChange()
	}
}

// AddPendingRepoDelete records a repo config so its resources can be
// force-deleted on the next reconcile cycle.
func (s *AppState) AddPendingRepoDelete(rc config.RepoConfig) {
	s.mu.Lock()
	s.pendingRepoDeletes = append(s.pendingRepoDeletes, rc)
	s.mu.Unlock()
}

// TakePendingRepoDeletes atomically returns and clears all pending repo deletes.
func (s *AppState) TakePendingRepoDeletes() []config.RepoConfig {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := s.pendingRepoDeletes
	s.pendingRepoDeletes = nil
	return out
}

// SaveRepoConfigs persists the current repo configs to repoFile as JSON.
//
// The file write uses a temp + rename so a crash mid-write leaves either the
// old file intact or the new file complete — never a truncated repos.json
// (which would lose every configured repository on next startup).
func (s *AppState) SaveRepoConfigs() error {
	s.mu.RLock()
	path := s.repoFile
	repos := make([]config.RepoConfig, len(s.RepoConfigs))
	copy(repos, s.RepoConfigs)
	s.mu.RUnlock()
	if path == "" {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	data, err := json.Marshal(repos)
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0600); err != nil {
		return err
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	return nil
}

// LoadRepoConfigs reads repo configs from repoFile if it exists.
func (s *AppState) LoadRepoConfigs() error {
	s.mu.RLock()
	path := s.repoFile
	s.mu.RUnlock()
	if path == "" {
		return nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	var repos []config.RepoConfig
	if err := json.Unmarshal(data, &repos); err != nil {
		return err
	}
	s.mu.Lock()
	s.RepoConfigs = repos
	s.mu.Unlock()
	s.notifyChange()
	return nil
}
