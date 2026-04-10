package state

import (
	"os"
	"sync"
	"time"

	"omni-cd/internal/config"
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
	DownSince time.Time `json:"downSince,omitempty"`
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
	CommitAuthor  string    `json:"commitAuthor,omitempty"`
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

// NodeGroup holds information about a group of nodes (control plane or a workers pool).
type NodeGroup struct {
	Name         string   `json:"name,omitempty"`
	Count        int      `json:"count"`
	MachineClass string   `json:"machineClass,omitempty"`
	Machines     []string `json:"machines,omitempty"`
	Extensions   []string `json:"extensions,omitempty"`
}

// ResourceInfo holds information about a managed resource.
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
	// LastSyncResult is the outcome of the most recent sync attempt: "ok" or "failed".
	LastSyncResult string `json:"lastSyncResult,omitempty"`
	// LastSyncError holds the error message from the most recent failed sync attempt.
	LastSyncError string `json:"lastSyncError,omitempty"`
	// LastSyncTime is the timestamp of the most recent sync attempt.
	LastSyncTime time.Time `json:"lastSyncTime,omitempty"`
	// LastSyncSHA is the git SHA that was applied during the most recent sync attempt.
	LastSyncSHA string `json:"lastSyncSHA,omitempty"`
	// LastSyncAuthor is the commit author at the time of the most recent sync attempt.
	LastSyncAuthor string `json:"lastSyncAuthor,omitempty"`
	// LastSyncMessage is the commit message at the time of the most recent sync attempt.
	LastSyncMessage string `json:"lastSyncMessage,omitempty"`
	// SyncStatusSince is the timestamp when the cluster first entered the outofsync state.
	SyncStatusSince time.Time `json:"syncStatusSince,omitempty"`
	// CreatedAt is the timestamp when the cluster resource was created in Omni.
	CreatedAt time.Time `json:"createdAt,omitempty"`
	// AutoSync controls whether this resource is automatically applied during reconcile.
	AutoSync *bool `json:"autoSync,omitempty"`
	// RepoName is the name of the git repo this resource was sourced from.
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
	AppVersion          string              `json:"appVersion"`
	OmniEndpoint        string              `json:"omniEndpoint"`
	OmniVersion         string              `json:"omniVersion"`
	OmniHealth          OmniHealth          `json:"omniHealth"`
	OmniEnvLocked       bool                `json:"omniEnvLocked"`
	OmniConfigured      bool                `json:"omniConfigured"`
	OmniHasStoredKey    bool                `json:"omniHasStoredKey"`
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
	LogLevel            string              `json:"logLevel"`
}

// AppState holds all shared state for the application.
type AppState struct {
	mu                  sync.RWMutex
	OmniEndpoint        string              `json:"omniEndpoint"`
	OmniVersion         string              `json:"omniVersion"`
	OmniHealth          OmniHealth          `json:"omniHealth"`
	omniEnvLocked       bool                // true when Omni creds come from ENV vars — never changes at runtime
	omniConfigured      bool                // true once omni.Init has succeeded
	omniHasStoredKey    bool                // true when a key is saved in instances.json
	Git                 GitInfo             `json:"git"`
	Repos               []GitInfo           `json:"repos,omitempty"`
	RepoConfigs         []config.RepoConfig // mutable repo configs — never serialised to browser
	LastReconcile       ReconcileInfo       `json:"lastReconcile"`
	MachineClasses      []ResourceInfo      `json:"machineClasses"`
	Clusters            []ResourceInfo      `json:"clusters"`
	ClustersEnabled     bool                `json:"clustersEnabled"`
	RepoClusterMap      map[string][]string `json:"repoClusterMap,omitempty"`
	RepoMachineClassMap map[string][]string `json:"repoMachineClassMap,omitempty"`
	forceClusterIDs     map[string]bool     // Cluster IDs queued for force sync
	forceMCIDs          map[string]bool     // Machine class IDs queued for force sync
	pendingRepoDeletes  []config.RepoConfig // Repos deleted via UI that need resource cleanup
	ServerStartedAt     time.Time           // Set once at process start, never persisted
	AppVersion          string              // Set at startup from build ldflags, never persisted
	auditLog            []AuditEntry        // In-memory ring buffer, never serialised to state.json
	auditDir            string              // Directory for daily audit files
	auditFile           *os.File            // Current day's open audit file
	auditFileDate       string              // "2006-01-02" of auditFile
	auditRetentionDays  int                 // Number of days to keep audit files
	Logs                []LogEntry          `json:"logs"`
	LogLevel            string              // e.g. "DEBUG", "INFO", "WARN", "ERROR"
	maxLogs             int
	logDir              string
	logFile             *os.File
	logFileDate         string // "2006-01-02"
	logRetentionDays    int
	stateFile           string        // Path to state file
	repoFile            string        // Path to repos.json
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
		auditLog:        []AuditEntry{},
		stateFile:       stateFile,
		changeCh:        make(chan struct{}, 1),
		ServerStartedAt: time.Now().UTC(),
		LastReconcile: ReconcileInfo{
			Status: StatusIdle,
		},
	}

	if stateFile != "" {
		_ = s.LoadFromFile(stateFile)
	}

	return s
}

// notifyChange does a non-blocking send on changeCh to signal a state mutation.
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
	downSince := s.OmniHealth.DownSince
	if status == "failed" && downSince.IsZero() {
		downSince = time.Now().UTC()
	} else if status != "failed" {
		downSince = time.Time{}
	}
	s.OmniHealth = OmniHealth{
		Status:    status,
		LastCheck: time.Now().UTC(),
		DownSince: downSince,
		Error:     errMsg,
	}
	s.mu.Unlock()
	s.notifyChange()
}

func (s *AppState) SetEnvLocked(v bool) {
	s.mu.Lock()
	s.omniEnvLocked = v
	s.mu.Unlock()
}

func (s *AppState) IsEnvLocked() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.omniEnvLocked
}

func (s *AppState) SetOmniConfigured(v bool) {
	s.mu.Lock()
	s.omniConfigured = v
	s.mu.Unlock()
	s.notifyChange()
}

func (s *AppState) IsOmniConfigured() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.omniConfigured
}

func (s *AppState) SetHasStoredKey(v bool) {
	s.mu.Lock()
	s.omniHasStoredKey = v
	s.mu.Unlock()
}

func (s *AppState) SetOmniEndpoint(endpoint string) {
	s.mu.Lock()
	s.OmniEndpoint = endpoint
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

// Snapshot returns a copy of the current state for JSON serialization.
func (s *AppState) Snapshot() SnapshotData {
	s.mu.RLock()
	defer s.mu.RUnlock()
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
		AppVersion:          s.AppVersion,
		OmniEndpoint:        s.OmniEndpoint,
		OmniVersion:         s.OmniVersion,
		OmniHealth:          s.OmniHealth,
		OmniEnvLocked:       s.omniEnvLocked,
		OmniConfigured:      s.omniConfigured,
		OmniHasStoredKey:    s.omniHasStoredKey,
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
		LogLevel:            s.LogLevel,
	}
}
