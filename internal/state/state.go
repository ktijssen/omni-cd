package state

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"omni-cd/internal/config"
	"omni-cd/internal/omni"
)

// ReconcileType identifies the type of reconciliation.
type ReconcileType string

const (
	ReconcileSoft ReconcileType = "soft"
	ReconcileHard ReconcileType = "hard"
)

// ReconcileStatus represents the status of a reconciliation.
type ReconcileStatus string

const (
	StatusIdle    ReconcileStatus = "idle"
	StatusRunning ReconcileStatus = "running"
	StatusSuccess ReconcileStatus = "success"
	StatusFailed  ReconcileStatus = "failed"
)

// OmniHealth holds the result of the last Omni connectivity check.
type OmniHealth struct {
	Status    string    `json:"status"`
	LastCheck time.Time `json:"lastCheck"`
	Error     string    `json:"error,omitempty"`
}

// RepoConfigView is a safe, token-free view of a RepoConfig sent to the browser.
type RepoConfigView struct {
	Name         string `json:"name"`
	URL          string `json:"url"`
	Branch       string `json:"branch"`
	HasToken     bool   `json:"hasToken"`
	ClustersPath string `json:"clustersPath"`
	MCPath       string `json:"mcPath"`
}

// GitInfo holds information about the current git state.
type GitInfo struct {
	Name          string    `json:"name,omitempty"`
	Repo          string    `json:"repo"`
	Branch        string    `json:"branch"`
	SHA           string    `json:"sha"`
	ShortSHA      string    `json:"shortSha"`
	CommitMessage string    `json:"commitMessage"`
	LastSync      time.Time `json:"lastSync"`
	SyncError     string    `json:"syncError,omitempty"`
}

// ReconcileInfo holds information about the last reconciliation.
type ReconcileInfo struct {
	Type       ReconcileType   `json:"type"`
	Status     ReconcileStatus `json:"status"`
	StartedAt  time.Time       `json:"startedAt"`
	FinishedAt time.Time       `json:"finishedAt"`
}

// ResourceInfo holds information about a managed resource.
// NodeGroup holds information about a group of nodes (control plane or a workers pool).
type NodeGroup struct {
	Name         string   `json:"name,omitempty"`
	Count        int      `json:"count"`
	MachineClass string   `json:"machineClass,omitempty"`
	Machines     []string `json:"machines,omitempty"`
	Extensions   []string `json:"extensions,omitempty"`
}

type ResourceInfo struct {
	ID            string `json:"id"`
	Type          string `json:"type"`
	Status        string `json:"status"`
	ProvisionType string `json:"provisionType,omitempty"`
	Diff          string `json:"diff,omitempty"`
	FileContent   string `json:"fileContent,omitempty"`
	LiveContent   string `json:"liveContent,omitempty"`
	Error         string `json:"error,omitempty"`
	// Cluster-specific detail (populated from live template export)
	TalosVersion       string      `json:"talosVersion,omitempty"`
	KubernetesVersion  string      `json:"kubernetesVersion,omitempty"`
	ControlPlane       NodeGroup   `json:"controlPlane,omitempty"`
	Workers            []NodeGroup `json:"workers,omitempty"`
	ClusterReady       string      `json:"clusterReady,omitempty"`
	KubernetesAPIReady string      `json:"kubernetesApiReady,omitempty"`
	ControlplaneReady  string      `json:"controlplaneReady,omitempty"`
	ClusterPhase       string      `json:"clusterPhase,omitempty"`
	// Live health fields (populated by background polling, not the reconciler)
	MachinesHealthy int       `json:"machinesHealthy,omitempty"`
	MachinesTotal   int       `json:"machinesTotal,omitempty"`
	EtcdStatus      string    `json:"etcdStatus,omitempty"`
	WireGuardStatus string    `json:"wireGuardStatus,omitempty"`
	LastBackupTime  time.Time `json:"lastBackupTime,omitempty"`
	BackupEnabled   bool      `json:"backupEnabled,omitempty"`
	// Extensions parsed from the cluster template
	ClusterExtensions []string            `json:"clusterExtensions,omitempty"`
	MachineExtensions map[string][]string `json:"machineExtensions,omitempty"`
	// Machine UUID -> hostname (populated for clusters with individual machines)
	MachineHostnames map[string]string `json:"machineHostnames,omitempty"`
	// AutoSync controls whether this cluster is automatically applied during reconcile.
	// nil or true = auto; false = manual (diff-only until user explicitly syncs).
	AutoSync *bool `json:"autoSync,omitempty"`
	// RepoName is the name of the git repo this cluster was sourced from.
	RepoName string `json:"repoName,omitempty"`
}

// LogEntry holds a single log entry.
type LogEntry struct {
	Timestamp time.Time `json:"timestamp"`
	Level     string    `json:"level"`
	Label     string    `json:"label"`
	Message   string    `json:"message"`
}

// SnapshotData holds a point-in-time copy of AppState for JSON serialization.
type SnapshotData struct {
	ServerStartedAt     time.Time           `json:"serverStartedAt"`
	OmniEndpoint        string              `json:"omniEndpoint"`
	OmniVersion         string              `json:"omniVersion"`
	OmniHealth          OmniHealth          `json:"omniHealth"`
	Git                 GitInfo             `json:"git"`
	Repos               []GitInfo           `json:"repos,omitempty"`
	RepoConfigs         []RepoConfigView    `json:"repoConfigs,omitempty"`
	LastReconcile       ReconcileInfo       `json:"lastReconcile"`
	MachineClasses      []ResourceInfo      `json:"machineClasses"`
	Clusters            []ResourceInfo      `json:"clusters"`
	ClustersEnabled     bool                `json:"clustersEnabled"`
	RepoClusterMap      map[string][]string `json:"repoClusterMap,omitempty"`
	RepoMachineClassMap map[string][]string `json:"repoMachineClassMap,omitempty"`
	Logs                []LogEntry          `json:"logs"`
}

// AppState holds all shared state for the application.
type AppState struct {
	mu                  sync.RWMutex
	OmniEndpoint        string              `json:"omniEndpoint"`
	OmniVersion         string              `json:"omniVersion"`
	OmniHealth          OmniHealth          `json:"omniHealth"`
	Git                 GitInfo             `json:"git"`
	Repos               []GitInfo           `json:"repos,omitempty"`
	RepoConfigs         []config.RepoConfig // mutable repo configs — never serialised to browser
	LastReconcile       ReconcileInfo       `json:"lastReconcile"`
	MachineClasses      []ResourceInfo      `json:"machineClasses"`
	Clusters            []ResourceInfo      `json:"clusters"`
	ClustersEnabled     bool                `json:"clustersEnabled"`
	RepoClusterMap      map[string][]string `json:"repoClusterMap,omitempty"`      // repoName → clusterIDs, persisted
	RepoMachineClassMap map[string][]string `json:"repoMachineClassMap,omitempty"` // repoName → mcIDs, persisted
	forceClusterIDs     map[string]bool     // Cluster IDs queued for force sync — accessed via Add/GetAndClear
	forceMCIDs          map[string]bool     // Machine class IDs queued for force sync — accessed via Add/GetAndClear
	pendingRepoDeletes  []config.RepoConfig // Repos deleted via UI that need resource cleanup
	ServerStartedAt     time.Time           // Set once at process start, never persisted
	Logs                []LogEntry          `json:"logs"`
	maxLogs             int
	stateFile           string        // Path to state file (not exported to JSON)
	repoFile            string        // Path to repos.json (not exported to JSON)
	changeCh            chan struct{} // Closed/sent on every state mutation
}

// New creates a new AppState with a max log buffer size.
func New(maxLogs int, omniEndpoint string, clustersEnabled bool, stateFile string) *AppState {
	s := &AppState{
		maxLogs:         maxLogs,
		OmniEndpoint:    omniEndpoint,
		ClustersEnabled: clustersEnabled,
		MachineClasses:  []ResourceInfo{},
		Clusters:        []ResourceInfo{},
		Logs:            []LogEntry{},
		stateFile:       stateFile,
		changeCh:        make(chan struct{}, 1),
		ServerStartedAt: time.Now().UTC(),
		LastReconcile: ReconcileInfo{
			Status: StatusIdle,
		},
	}

	// Load persisted state if available
	if stateFile != "" {
		_ = s.LoadFromFile(stateFile) // Ignore errors, use defaults
	}

	return s
}

// notifyChange does a non-blocking send on changeCh to signal a state mutation.
// Must be called while NOT holding mu (the receiver in the web layer will re-read state).
func (s *AppState) notifyChange() {
	select {
	case s.changeCh <- struct{}{}:
	default:
	}
}

// ChangeCh returns a channel that receives a value whenever the state changes.
func (s *AppState) ChangeCh() <-chan struct{} {
	return s.changeCh
}

// SetVersions sets the Omni version string.
func (s *AppState) SetVersions(omniVersion string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.OmniVersion = omniVersion
}

// SetOmniHealth updates the Omni connectivity health status.
func (s *AppState) SetOmniHealth(status, errMsg string) {
	s.mu.Lock()
	s.OmniHealth = OmniHealth{
		Status:    status,
		LastCheck: time.Now().UTC(),
		Error:     errMsg,
	}
	s.mu.Unlock()
	s.notifyChange()
}

// UpdateGit updates the git information for the primary (first) repo.
func (s *AppState) UpdateGit(info GitInfo) {
	s.mu.Lock()
	s.Git = info
	s.mu.Unlock()
	s.notifyChange()
}

// UpdateRepos updates the full list of repo git infos and keeps Git in sync
// with the first repo for backward compatibility.
func (s *AppState) UpdateRepos(infos []GitInfo) {
	s.mu.Lock()
	s.Repos = infos
	if len(infos) > 0 {
		s.Git = infos[0]
	}
	s.mu.Unlock()
	s.notifyChange()
}

// ── Repo Config management ──────────────────────────────────────────────────

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

// AddRepoConfig adds a new repo.  Returns an error if the name already exists.
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
				rc.Token = "" // explicit clear
			} else if rc.Token == "" {
				rc.Token = r.Token // preserve existing
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

// SaveRepoConfigs persists the current repo configs to repoFile as JSON.
// Tokens are stored in this file — keep the file permissions tight.
// SetRepoClusters records which cluster IDs are managed by the named repo.
// Called after each successful ApplyClusters pass.
func (s *AppState) SetRepoClusters(repoName string, clusterIDs []string) {
	s.mu.Lock()
	if s.RepoClusterMap == nil {
		s.RepoClusterMap = make(map[string][]string)
	}
	s.RepoClusterMap[repoName] = clusterIDs
	s.mu.Unlock()
}

// GetRepoClusters returns the last known cluster IDs for the named repo.
func (s *AppState) GetRepoClusters(repoName string) []string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.RepoClusterMap[repoName]
}

// GetAllTrackedClusterIDs returns the union of all cluster IDs that have ever
// been managed by any repo known to this omni-cd instance. Used to distinguish
// omni-cd-owned clusters from template-managed clusters created externally.
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
func (s *AppState) GetRepoMachineClasses(repoName string) []string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.RepoMachineClassMap[repoName]
}

// GetAllTrackedMachineClassIDs returns the union of all machine class IDs that
// have ever been managed by any repo known to this omni-cd instance.
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
// doesn't already have one. Used to restore the association when a repo
// is failing to sync and ApplyClusters doesn't run.
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

// AddPendingRepoDelete records a repo config so its clusters and machine classes
// can be force-deleted on the next reconcile cycle.
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

func (s *AppState) SaveRepoConfigs() error {
	s.mu.RLock()
	path := s.repoFile
	repos := make([]config.RepoConfig, len(s.RepoConfigs))
	copy(repos, s.RepoConfigs)
	s.mu.RUnlock()
	if path == "" {
		return nil // persistence not configured
	}
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	data, err := json.Marshal(repos)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0600)
}

// LoadRepoConfigs reads repo configs from repoFile if it exists.
// Returns nil (no error) if the file does not exist.
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

// ── End Repo Config management ──────────────────────────────────────────────

// SetReconcileStarted marks a reconciliation as started.
func (s *AppState) SetReconcileStarted(t ReconcileType) {
	s.mu.Lock()
	s.LastReconcile = ReconcileInfo{
		Type:      t,
		Status:    StatusRunning,
		StartedAt: time.Now().UTC(),
	}
	s.mu.Unlock()
	s.notifyChange()
}

// SetReconcileFinished marks a reconciliation as finished.
func (s *AppState) SetReconcileFinished(success bool) {
	s.mu.Lock()
	if success {
		s.LastReconcile.Status = StatusSuccess
	} else {
		s.LastReconcile.Status = StatusFailed
	}
	s.LastReconcile.FinishedAt = time.Now().UTC()
	s.mu.Unlock()
	s.notifyChange()
}

// SetMachineClasses replaces the machine class list.
func (s *AppState) SetMachineClasses(resources []ResourceInfo) {
	s.mu.Lock()
	s.MachineClasses = resources
	s.mu.Unlock()
	s.notifyChange()
}

// GetMachineClasses returns a copy of the current machine class list.
func (s *AppState) GetMachineClasses() []ResourceInfo {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]ResourceInfo, len(s.MachineClasses))
	copy(out, s.MachineClasses)
	return out
}

// MergeMachineClasses updates or inserts resources by ID without removing
// entries not present in the slice. Use this when processing one repo at a time
// so results from different repos accumulate rather than overwrite each other.
func (s *AppState) MergeMachineClasses(resources []ResourceInfo) {
	s.mu.Lock()
	// Preserve user-configured AutoSync preferences so a reconcile cycle
	// cannot reset them (the reconciler never sets AutoSync on ResourceInfo).
	autoSyncMap := make(map[string]*bool, len(s.MachineClasses))
	for _, mc := range s.MachineClasses {
		autoSyncMap[mc.ID] = mc.AutoSync
	}

	existing := make(map[string]int, len(s.MachineClasses))
	for i, mc := range s.MachineClasses {
		existing[mc.ID] = i
	}
	for _, res := range resources {
		// Carry forward the stored AutoSync value; if this is a brand-new MC
		// (not yet in state) leave AutoSync as nil so GetMachineClassAutoSync
		// returns the default (disabled).
		if prev, had := autoSyncMap[res.ID]; had {
			res.AutoSync = prev
		}
		if idx, ok := existing[res.ID]; ok {
			s.MachineClasses[idx] = res
		} else {
			s.MachineClasses = append(s.MachineClasses, res)
			existing[res.ID] = len(s.MachineClasses) - 1
		}
	}
	s.mu.Unlock()
	s.notifyChange()
}

// GetClusters returns a copy of the current cluster list.
func (s *AppState) GetClusters() []ResourceInfo {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]ResourceInfo, len(s.Clusters))
	copy(out, s.Clusters)
	return out
}

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
}

// SetClusters replaces the cluster list, preserving transient fields (e.g.
// ClusterReady, KubernetesAPIReady) that are populated by background polling
// rather than the reconciler itself. It also preserves the per-cluster AutoSync
// preference so a reconcile cycle cannot reset user-configured values.
func (s *AppState) SetClusters(resources []ResourceInfo) {
	s.mu.Lock()
	// Build a lookup of existing transient values so we don't lose them when
	// the reconciler overwrites the list with freshly-built structs.
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
			// Never let a stale snapshot from a concurrent reconcile clobber a
			// mid-deletion status.  The deleting goroutine races with ApplyClusters
			// which snapshots clusters, builds a 'final' slice, and then calls
			// SetClusters — if the snapshot was taken before 'deleting' was set,
			// the incoming status would be stale.  Restore the current status here.
			if statusMap[resources[i].ID] == "deleting" {
				resources[i].Status = "deleting"
			}
			// Preserve user-configured AutoSync preference; nil means it was
			// never explicitly set (e.g. added by UpsertClusterStatus mid-reconcile),
			// so apply the default of disabled.
			if autoSyncMap[resources[i].ID] != nil {
				resources[i].AutoSync = autoSyncMap[resources[i].ID]
			} else {
				f := false
				resources[i].AutoSync = &f
			}
		} else {
			// New clusters default to auto-sync disabled.
			f := false
			resources[i].AutoSync = &f
		}
	}
	s.Clusters = resources
	s.mu.Unlock()
	s.notifyChange()
}

// UpdateMachineClassStatus updates the status of a single machine class by ID.
// If the machine class is not found, it is a no-op.
func (s *AppState) UpdateMachineClassStatus(id, status string) {
	s.mu.Lock()
	for i := range s.MachineClasses {
		if s.MachineClasses[i].ID == id {
			s.MachineClasses[i].Status = status
			s.mu.Unlock()
			s.notifyChange()
			return
		}
	}
	s.mu.Unlock()
}

// RemoveMachineClass removes a machine class from state by ID. No-op if not found.
func (s *AppState) RemoveMachineClass(id string) {
	s.mu.Lock()
	for i, mc := range s.MachineClasses {
		if mc.ID == id {
			s.MachineClasses = append(s.MachineClasses[:i], s.MachineClasses[i+1:]...)
			s.mu.Unlock()
			s.notifyChange()
			return
		}
	}
	s.mu.Unlock()
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
// If the cluster is not found, it is a no-op.
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

// UpsertClusterStatus updates the status of a cluster by ID.
// If the cluster is not yet tracked in state, a minimal entry is added so it
// becomes visible in the UI immediately.
func (s *AppState) UpsertClusterStatus(id, status string) {
	s.mu.Lock()
	for i := range s.Clusters {
		if s.Clusters[i].ID == id {
			// Never overwrite a mid-deletion status with anything else.
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

// UpsertClusterInfo replaces the full ResourceInfo for one cluster, or appends it
// if the cluster is not yet in state. Ready-status fields and AutoSync are preserved
// from the existing entry so a refresh does not clear live health data or user prefs.
func (s *AppState) UpsertClusterInfo(id string, info ResourceInfo) {
	s.mu.Lock()
	for i := range s.Clusters {
		if s.Clusters[i].ID == id {
			// Never overwrite a mid-deletion status with anything else.
			if s.Clusters[i].Status == "deleting" {
				s.mu.Unlock()
				return
			}
			// Preserve live-health fields populated by the background poller.
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
			// Preserve repo name if the incoming update doesn't supply one
			if info.RepoName == "" {
				info.RepoName = s.Clusters[i].RepoName
			}
			s.Clusters[i] = info
			s.mu.Unlock()
			s.notifyChange()
			return
		}
	}
	// New cluster — default auto-sync to disabled.
	if info.AutoSync == nil {
		disabled := false
		info.AutoSync = &disabled
	}
	s.Clusters = append(s.Clusters, info)
	s.mu.Unlock()
	s.notifyChange()
}

// UpdateClusterReadyStatuses updates the ClusterReady and KubernetesAPIReady fields
// for each cluster from a status map. Clusters not present in the map are marked as "unknown".
// Returns true if any value changed so the caller can decide whether to persist to disk.
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

// UpdateTearingDownStatuses marks clusters whose phase is TearingDown in Omni
// as "deleting", and reverts clusters that were "deleting" but are no longer
// found in Omni at all back to "outofsync" (template still exists in git).
// Returns true if any cluster status changed.
func (s *AppState) UpdateTearingDownStatuses(allIDs []string, tearingDown map[string]bool) bool {
	omniMap := make(map[string]bool, len(allIDs))
	for _, id := range allIDs {
		omniMap[id] = true
	}
	s.mu.Lock()
	changed := false
	for i := range s.Clusters {
		c := &s.Clusters[i]
		// When a cluster is tearing down the clusterPhase already shows "destroying";
		// do not overwrite the sync status with "deleting".
		// If it was previously marked deleting and is now gone from Omni, revert.
		if c.Status == "deleting" && !omniMap[c.ID] {
			c.Status = "outofsync"
			c.LiveContent = ""
			changed = true
		}
	}
	s.mu.Unlock()
	if changed {
		s.notifyChange()
	}
	return changed
}

// GetClustersEnabled returns the current clusters enabled state.
// Deprecated: use per-cluster AutoSync instead.
func (s *AppState) GetClustersEnabled() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.ClustersEnabled
}

// GetClusterAutoSync returns true if the cluster should be automatically applied
// during reconcile. Returns false (disabled) for unknown clusters or when AutoSync is nil.
func (s *AppState) GetClusterAutoSync(id string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, c := range s.Clusters {
		if c.ID == id {
			if c.AutoSync != nil {
				return *c.AutoSync
			}
			return false // nil = default disabled
		}
	}
	return false // unknown cluster defaults to disabled
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

	// Auto-save after toggle
	s.save()

	return newState
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

// AddForceMCID queues a machine class ID to be force-synced on the next reconcile.
// Multiple IDs can be queued between reconcile cycles — none will be lost.
func (s *AppState) AddForceMCID(id string) {
	s.mu.Lock()
	if s.forceMCIDs == nil {
		s.forceMCIDs = make(map[string]bool)
	}
	s.forceMCIDs[id] = true
	s.mu.Unlock()
}

// GetAndClearForceMCIDs returns all queued force-sync MC IDs and atomically clears the set.
func (s *AppState) GetAndClearForceMCIDs() map[string]bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	ids := s.forceMCIDs
	s.forceMCIDs = nil
	return ids
}

// AddForceClusterID queues a cluster ID to be force-synced on the next reconcile.
// Multiple IDs can be queued between reconcile cycles — none will be lost.
func (s *AppState) AddForceClusterID(id string) {
	s.mu.Lock()
	if s.forceClusterIDs == nil {
		s.forceClusterIDs = make(map[string]bool)
	}
	s.forceClusterIDs[id] = true
	s.mu.Unlock()
}

// GetAndClearForceClusterIDs returns all queued force-sync IDs and atomically clears the set.
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

// GetMachineClassAutoSync returns true if the machine class is automatically applied
// during reconcile. Returns false (disabled) for unknown or unconfigured MCs so that
// newly-discovered machine classes require an explicit opt-in before being applied.
func (s *AppState) GetMachineClassAutoSync(id string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, mc := range s.MachineClasses {
		if mc.ID == id {
			if mc.AutoSync != nil {
				return *mc.AutoSync
			}
			return false // nil = not yet set, default to disabled
		}
	}
	return false // unknown MC defaults to disabled
}

// IsMachineClassAutoSyncExplicitlyDisabled returns true only when the machine class
// AutoSync has been explicitly set to false by the user. Returns false for unknown MCs
// or when AutoSync is nil (never configured). Used to allow force-reconciles to apply
// MCs that haven't been explicitly opted out.
func (s *AppState) IsMachineClassAutoSyncExplicitlyDisabled(id string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, mc := range s.MachineClasses {
		if mc.ID == id {
			return mc.AutoSync != nil && !*mc.AutoSync
		}
	}
	return false // unknown MC — not explicitly disabled
}

// SetMachineClassAutoSync sets the per-MC AutoSync preference and persists state.
func (s *AppState) SetMachineClassAutoSync(id string, enabled bool) {
	s.mu.Lock()
	for i := range s.MachineClasses {
		if s.MachineClasses[i].ID == id {
			s.MachineClasses[i].AutoSync = &enabled
			s.mu.Unlock()
			s.save()
			s.notifyChange()
			return
		}
	}
	s.mu.Unlock()
}

// AddLog appends a log entry, trimming old entries if needed.
func (s *AppState) AddLog(level, label, message string) {
	s.mu.Lock()
	entry := LogEntry{
		Timestamp: time.Now().UTC(),
		Level:     level,
		Label:     label,
		Message:   message,
	}
	s.Logs = append(s.Logs, entry)
	if len(s.Logs) > s.maxLogs {
		s.Logs = s.Logs[len(s.Logs)-s.maxLogs:]
	}
	s.mu.Unlock()
	s.notifyChange()
}

// Snapshot returns a copy of the current state for JSON serialization.
func (s *AppState) Snapshot() SnapshotData {
	s.mu.RLock()
	defer s.mu.RUnlock()
	// Build masked repo config views
	repoViews := make([]RepoConfigView, len(s.RepoConfigs))
	for i, rc := range s.RepoConfigs {
		repoViews[i] = RepoConfigView{
			Name:         rc.Name,
			URL:          rc.URL,
			Branch:       rc.Branch,
			HasToken:     rc.Token != "",
			ClustersPath: rc.ClustersPath,
			MCPath:       rc.MCPath,
		}
	}
	return SnapshotData{
		ServerStartedAt:     s.ServerStartedAt,
		OmniEndpoint:        s.OmniEndpoint,
		OmniVersion:         s.OmniVersion,
		OmniHealth:          s.OmniHealth,
		Git:                 s.Git,
		Repos:               s.Repos,
		RepoConfigs:         repoViews,
		LastReconcile:       s.LastReconcile,
		MachineClasses:      s.MachineClasses,
		Clusters:            s.Clusters,
		ClustersEnabled:     s.ClustersEnabled,
		RepoClusterMap:      s.RepoClusterMap,
		RepoMachineClassMap: s.RepoMachineClassMap,
		Logs:                s.Logs,
	}
}

// persistedState is the struct written to the state file.
type persistedState struct {
	LastReconcile       ReconcileInfo       `json:"lastReconcile"`
	RepoClusterMap      map[string][]string `json:"repoClusterMap,omitempty"`
	RepoMachineClassMap map[string][]string `json:"repoMachineClassMap,omitempty"`
	Clusters            []ResourceInfo      `json:"clusters,omitempty"`
	MachineClasses      []ResourceInfo      `json:"machineClasses,omitempty"`
	ClustersEnabled     bool                `json:"clustersEnabled"` // legacy: migrate old global flag to per-cluster AutoSync
}

// SaveToFile persists state to disk. Full cluster and MC data is stored so
// the UI can display last-known state during the window between server start
// and the first reconcile completing. RepoClusterMap and RepoMachineClassMap
// are included so resource-protection tracking survives restarts.
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

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	data, err := json.MarshalIndent(ps, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(path, data, 0600)
}

// LoadFromFile restores state from disk. Full cluster and MC data is restored
// so the UI shows last-known state immediately on startup. RepoClusterMap and
// RepoMachineClassMap are restored so resource-protection is effective before
// the first reconcile completes.
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
		// Migration: if the old global ClustersEnabled was false, apply AutoSync=false
		// to any cluster that never had an explicit preference set.
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
