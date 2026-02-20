package state

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"time"
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

// GitInfo holds information about the current git state.
type GitInfo struct {
	Repo          string    `json:"repo"`
	Branch        string    `json:"branch"`
	SHA           string    `json:"sha"`
	ShortSHA      string    `json:"shortSha"`
	CommitMessage string    `json:"commitMessage"`
	LastSync      time.Time `json:"lastSync"`
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
	Name         string `json:"name,omitempty"`
	Count        int    `json:"count"`
	MachineClass string `json:"machineClass,omitempty"`
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
	TalosVersion      string      `json:"talosVersion,omitempty"`
	KubernetesVersion string      `json:"kubernetesVersion,omitempty"`
	ControlPlane      NodeGroup   `json:"controlPlane,omitempty"`
	Workers           []NodeGroup `json:"workers,omitempty"`
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
	OmniEndpoint    string         `json:"omniEndpoint"`
	OmniVersion     string         `json:"omniVersion"`
	OmnictlVersion  string         `json:"omnictlVersion"`
	VersionMismatch bool           `json:"versionMismatch"`
	OmniHealth      OmniHealth     `json:"omniHealth"`
	Git             GitInfo        `json:"git"`
	LastReconcile   ReconcileInfo  `json:"lastReconcile"`
	MachineClasses  []ResourceInfo `json:"machineClasses"`
	Clusters        []ResourceInfo `json:"clusters"`
	ClustersEnabled bool           `json:"clustersEnabled"`
	Logs            []LogEntry     `json:"logs"`
}

// AppState holds all shared state for the application.
type AppState struct {
	mu              sync.RWMutex
	OmniEndpoint    string         `json:"omniEndpoint"`
	OmniVersion     string         `json:"omniVersion"`
	OmnictlVersion  string         `json:"omnictlVersion"`
	VersionMismatch bool           `json:"versionMismatch"`
	OmniHealth      OmniHealth     `json:"omniHealth"`
	Git             GitInfo        `json:"git"`
	LastReconcile   ReconcileInfo  `json:"lastReconcile"`
	MachineClasses  []ResourceInfo `json:"machineClasses"`
	Clusters        []ResourceInfo `json:"clusters"`
	ClustersEnabled bool           `json:"clustersEnabled"`
	ForceClusterID  string         // Cluster ID to force sync (not exported to JSON)
	Logs            []LogEntry     `json:"logs"`
	maxLogs         int
	stateFile       string        // Path to state file (not exported to JSON)
	changeCh        chan struct{}  // Closed/sent on every state mutation
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

// SetVersions sets the Omni and omnictl version strings and mismatch flag.
func (s *AppState) SetVersions(omniVersion, omnictlVersion string, mismatch bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.OmniVersion = omniVersion
	s.OmnictlVersion = omnictlVersion
	s.VersionMismatch = mismatch
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

// UpdateGit updates the git information.
func (s *AppState) UpdateGit(info GitInfo) {
	s.mu.Lock()
	s.Git = info
	s.mu.Unlock()
	s.notifyChange()
}

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

// GetClusters returns a copy of the current cluster list.
func (s *AppState) GetClusters() []ResourceInfo {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]ResourceInfo, len(s.Clusters))
	copy(out, s.Clusters)
	return out
}

// SetClusters replaces the cluster list.
func (s *AppState) SetClusters(resources []ResourceInfo) {
	s.mu.Lock()
	s.Clusters = resources
	s.mu.Unlock()
	s.notifyChange()
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
			s.Clusters[i].Status = status
			s.mu.Unlock()
			s.notifyChange()
			return
		}
	}
	s.Clusters = append(s.Clusters, ResourceInfo{
		ID:     id,
		Type:   "Cluster",
		Status: status,
	})
	s.mu.Unlock()
	s.notifyChange()
}

// GetClustersEnabled returns the current clusters enabled state.
func (s *AppState) GetClustersEnabled() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.ClustersEnabled
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

// SetForceClusterID sets a specific cluster to force sync on next reconcile.
func (s *AppState) SetForceClusterID(id string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.ForceClusterID = id
}

// GetForceClusterID returns and clears the force cluster ID.
func (s *AppState) GetForceClusterID() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	id := s.ForceClusterID
	s.ForceClusterID = ""
	return id
}

// HasForceClusterID checks if a force cluster ID is set without clearing it.
func (s *AppState) HasForceClusterID() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.ForceClusterID != ""
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
	return SnapshotData{
		OmniEndpoint:    s.OmniEndpoint,
		OmniVersion:     s.OmniVersion,
		OmnictlVersion:  s.OmnictlVersion,
		VersionMismatch: s.VersionMismatch,
		OmniHealth:      s.OmniHealth,
		Git:             s.Git,
		LastReconcile:   s.LastReconcile,
		MachineClasses:  s.MachineClasses,
		Clusters:        s.Clusters,
		ClustersEnabled: s.ClustersEnabled,
		Logs:            s.Logs,
	}
}

// SaveToFile persists the state to a JSON file.
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

	// Create snapshot without logs and git info (they're transient)
	snapshot := SnapshotData{
		OmniEndpoint:    s.OmniEndpoint,
		OmniVersion:     s.OmniVersion,
		OmnictlVersion:  s.OmnictlVersion,
		VersionMismatch: s.VersionMismatch,
		// Git intentionally omitted
		LastReconcile:   s.LastReconcile,
		MachineClasses:  filteredMCs,
		Clusters:        filteredClusters,
		ClustersEnabled: s.ClustersEnabled,
		// Logs intentionally omitted
	}

	s.mu.RUnlock()

	// Create directory if it doesn't exist
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	// Marshal state to JSON
	data, err := json.MarshalIndent(snapshot, "", "  ")
	if err != nil {
		return err
	}

	// Write to file
	return os.WriteFile(path, data, 0644)
}

// LoadFromFile loads state from a JSON file if it exists.
func (s *AppState) LoadFromFile(path string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Check if file exists
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return nil // File doesn't exist, not an error
	}

	// Read file
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	// Unmarshal into a temporary struct
	var loaded AppState
	if err := json.Unmarshal(data, &loaded); err != nil {
		return err
	}

	// Restore persisted fields
	s.ClustersEnabled = loaded.ClustersEnabled
	// Don't restore Git - it's transient
	s.LastReconcile = loaded.LastReconcile
	s.MachineClasses = loaded.MachineClasses
	s.Clusters = loaded.Clusters
	s.OmniVersion = loaded.OmniVersion
	s.OmnictlVersion = loaded.OmnictlVersion
	s.VersionMismatch = loaded.VersionMismatch
	// Don't restore Logs - they're transient

	return nil
}
