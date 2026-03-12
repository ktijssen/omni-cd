package web

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"sync"
	"time"

	"omni-cd/internal/auth"
	"omni-cd/internal/state"

	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		origin := r.Header.Get("Origin")
		if origin == "" {
			// No Origin header — non-browser client, allow.
			return true
		}
		u, err := url.ParseRequestURI(origin)
		if err != nil {
			return false
		}
		// Only allow same-origin requests.
		return u.Host == r.Host
	},
}

// Server serves the web UI and API endpoints.
type Server struct {
	appState               *state.AppState
	triggerHard            chan struct{}
	triggerSoft            chan struct{}
	triggerRefreshCluster  chan string
	triggerDeleteCluster   chan string
	triggerDeleteMC        chan string
	triggerMCRefresh       chan struct{}
	triggerMCRefreshSingle chan string
	triggerRepoChange      chan struct{}
	triggerOmniConfigured  chan struct{}
	omniInstanceFile       string
	logDir                 string
	port                   string
	version                string
	clients                map[*websocket.Conn]bool
	clientsMu              sync.RWMutex
	broadcast              chan []byte
	authStore    *auth.Store
	authDisabled bool
	sessions     sync.Map // token (string) -> sessionInfo
	loginBuckets sync.Map // IP (string) -> *loginBucket
	// OIDC
	oidcRT     *OIDCRuntime
	oidcMu     sync.RWMutex
	oidcStates sync.Map // state token (string) -> oidcStateEntry
	oidcUsers  *oidcUserStore
}

// New creates a new web server.
func New(appState *state.AppState, triggerHard chan struct{}, triggerSoft chan struct{}, triggerRefreshCluster chan string, triggerDeleteCluster chan string, triggerDeleteMC chan string, triggerMCRefresh chan struct{}, triggerMCRefreshSingle chan string, triggerRepoChange chan struct{}, triggerOmniConfigured chan struct{}, omniInstanceFile string, logDir string, port string, version string, authStore *auth.Store, authDisabled bool, oidcRT *OIDCRuntime) *Server {
	s := &Server{
		appState:               appState,
		triggerHard:            triggerHard,
		triggerSoft:            triggerSoft,
		triggerRefreshCluster:  triggerRefreshCluster,
		triggerDeleteCluster:   triggerDeleteCluster,
		triggerDeleteMC:        triggerDeleteMC,
		triggerMCRefresh:       triggerMCRefresh,
		triggerMCRefreshSingle: triggerMCRefreshSingle,
		triggerRepoChange:      triggerRepoChange,
		triggerOmniConfigured:  triggerOmniConfigured,
		omniInstanceFile:       omniInstanceFile,
		logDir:                 logDir,
		port:                   port,
		version:                version,
		clients:                make(map[*websocket.Conn]bool),
		broadcast:              make(chan []byte, 256),
		authStore:              authStore,
		authDisabled:           authDisabled,
		oidcRT:    oidcRT,
		oidcUsers: loadOIDCUserStore(oidcUsersPath),
	}

	// Start broadcast handler
	go s.handleBroadcasts()

	// Start state change monitor
	go s.monitorStateChanges()

	// Periodically purge expired OIDC state tokens to prevent unbounded growth.
	go s.cleanupOIDCStates()

	// Periodically purge stale login rate-limit buckets to prevent unbounded growth.
	go s.cleanupLoginBuckets()

	return s
}

// Start starts the web server in a goroutine.
func (s *Server) Start() {
	mux := http.NewServeMux()

	// Public routes — no auth required
	mux.HandleFunc("/setup", s.handleSetup)
	mux.HandleFunc("/login", s.handleLogin)
	mux.HandleFunc("/logout", s.handleLogout)
	mux.HandleFunc("/unauthorized", s.handleUnauthorized)

	// OIDC SSO routes — public (the handlers check OIDC is enabled)
	mux.HandleFunc("/auth/login", s.handleOIDCLogin)
	mux.HandleFunc("/auth/callback", s.handleOIDCCallback)

	// WebSocket endpoint — viewer+
	mux.HandleFunc("/ws", s.requireRole("viewer", s.handleWebSocket))

	// Read-only API endpoints — viewer+
	mux.HandleFunc("/api/state", s.requireRole("viewer", s.handleState))
	mux.HandleFunc("/api/logs/files", s.requireRole("viewer", s.handleLogFiles))
	mux.HandleFunc("/api/logs/download", s.requireRole("viewer", s.handleLogDownload))

	// Write API endpoints — admin only
	mux.HandleFunc("/api/reconcile", s.requireRole("admin", s.handleReconcile))
	mux.HandleFunc("/api/check", s.requireRole("admin", s.handleCheck))
	mux.HandleFunc("/api/clusters-toggle", s.requireRole("admin", s.handleClustersToggle))
	mux.HandleFunc("/api/force-cluster", s.requireRole("admin", s.handleForceCluster))
	mux.HandleFunc("/api/refresh-cluster", s.requireRole("admin", s.handleRefreshCluster))
	mux.HandleFunc("/api/delete-cluster", s.requireRole("admin", s.handleDeleteCluster))
	mux.HandleFunc("/api/set-cluster-autosync", s.requireRole("admin", s.handleSetClusterAutoSync))
	mux.HandleFunc("/api/export-cluster", s.requireRole("admin", s.handleExportCluster))
	mux.HandleFunc("/api/repos", s.requireRole("admin", s.handleRepos))
	mux.HandleFunc("/api/refresh-mc", s.requireRole("admin", s.handleRefreshMC))
	mux.HandleFunc("/api/refresh-single-mc", s.requireRole("admin", s.handleRefreshSingleMC))
	mux.HandleFunc("/api/delete-machineclass", s.requireRole("admin", s.handleDeleteMachineClass))
	mux.HandleFunc("/api/sync-machineclass", s.requireRole("admin", s.handleSyncMachineClass))
	mux.HandleFunc("/api/set-mc-autosync", s.requireRole("admin", s.handleSetMCAutoSync))
	mux.HandleFunc("/api/omni-instance", s.requireRole("admin", s.handleOmniInstance))
	mux.HandleFunc("/api/omni-instance/test", s.requireRole("admin", s.handleTestOmniInstance))
	mux.HandleFunc("/api/omni-instance/refresh", s.requireRole("admin", s.handleRefreshOmniConnection))
	mux.HandleFunc("/api/users", s.requireRole("admin", s.handleUsers))
	mux.HandleFunc("/api/users/change-password", s.requireRole("admin", s.handleChangePassword))
	mux.HandleFunc("/api/users/update-profile", s.requireRole("admin", s.handleUpdateProfile))
	mux.HandleFunc("/api/users/oidc", s.requireRole("admin", s.handleOIDCUsers))
	mux.HandleFunc("/api/oidc-config", s.requireRole("admin", s.handleOIDCConfigAPI))

	// Serve the UI (all protected, viewer+)
	mux.HandleFunc("/clusters/", s.requireRole("viewer", s.handleUI))
	mux.HandleFunc("/clusters", s.requireRole("viewer", s.handleUI))
	mux.HandleFunc("/machineclasses", s.requireRole("viewer", s.handleUI))
	mux.HandleFunc("/repos", s.requireRole("viewer", s.handleUI))
	mux.HandleFunc("/logs", s.requireRole("viewer", s.handleUI))
	mux.HandleFunc("/users", s.requireRole("admin", func(w http.ResponseWriter, r *http.Request) {
		if s.authDisabled {
			http.Redirect(w, r, "/clusters", http.StatusFound)
			return
		}
		s.handleUI(w, r)
	}))
	mux.HandleFunc("/", s.requireRole("viewer", s.handleUI))

	addr := fmt.Sprintf(":%s", s.port)
	slog.Info("Web UI listening", "address", addr, "component", "Web")

	srv := &http.Server{
		Addr:         addr,
		Handler:      securityHeadersMiddleware(mux),
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 60 * time.Second,
		IdleTimeout:  120 * time.Second,
	}
	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("Web server failed", "error", err, "component", "Web")
		}
	}()
}

// handleWebSocket upgrades HTTP connection to WebSocket and manages client lifecycle.
func (s *Server) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		slog.Error("WebSocket upgrade failed", "error", err, "component", "Web")
		return
	}
	defer conn.Close()

	// Register client
	s.clientsMu.Lock()
	s.clients[conn] = true
	s.clientsMu.Unlock()

	slog.Debug("WebSocket client connected", "component", "Web")

	// Send initial state
	snapshot := s.appState.Snapshot()
	if data, err := json.Marshal(snapshot); err == nil {
		conn.WriteMessage(websocket.TextMessage, data)
	}

	// Wait for client disconnect
	for {
		if _, _, err := conn.ReadMessage(); err != nil {
			break
		}
	}

	// Unregister client
	s.clientsMu.Lock()
	delete(s.clients, conn)
	s.clientsMu.Unlock()

	slog.Debug("WebSocket client disconnected", "component", "Web")
}

// handleBroadcasts sends state updates to all connected WebSocket clients.
func (s *Server) handleBroadcasts() {
	for message := range s.broadcast {
		s.clientsMu.RLock()
		for client := range s.clients {
			err := client.WriteMessage(websocket.TextMessage, message)
			if err != nil {
				client.Close()
				s.clientsMu.RUnlock()
				s.clientsMu.Lock()
				delete(s.clients, client)
				s.clientsMu.Unlock()
				s.clientsMu.RLock()
			}
		}
		s.clientsMu.RUnlock()
	}
}

// monitorStateChanges broadcasts state immediately on any mutation, with a
// 1-second ticker as a fallback to catch any updates that may be missed.
func (s *Server) monitorStateChanges() {
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()
	changeCh := s.appState.ChangeCh()

	var lastHash uint64
	maybeBroadcast := func() {
		snapshot := s.appState.Snapshot()
		currentHash := s.hashState(snapshot)
		if currentHash != lastHash {
			lastHash = currentHash
			if data, err := json.Marshal(snapshot); err == nil {
				select {
				case s.broadcast <- data:
				default:
				}
			}
		}
	}

	for {
		select {
		case <-changeCh:
			maybeBroadcast()
		case <-ticker.C:
			maybeBroadcast()
		}
	}
}

// hashState creates a hash of the state for change detection.
func (s *Server) hashState(snapshot state.SnapshotData) uint64 {
	var hash uint64
	hash = uint64(len(snapshot.MachineClasses))
	hash = hash*31 + uint64(len(snapshot.Clusters))
	hash = hash*31 + uint64(len(snapshot.Logs))
	if snapshot.ClustersEnabled {
		hash = hash * 31
	}
	// Hash per-cluster AutoSync so toggling it triggers an immediate UI refresh.
	for _, c := range snapshot.Clusters {
		if c.AutoSync != nil && !*c.AutoSync {
			hash = hash*31 + 7
		}
	}
	if len(snapshot.LastReconcile.Status) > 0 {
		hash = hash*31 + uint64(snapshot.LastReconcile.Status[0])
	}
	hash = hash*31 + uint64(len(snapshot.Git.SHA))
	// Include per-resource statuses so a status-only change is detected.
	for _, c := range snapshot.Clusters {
		for _, b := range []byte(c.Status) {
			hash = hash*31 + uint64(b)
		}
		if c.ClusterReady != "" {
			for _, b := range []byte(c.ClusterReady) {
				hash = hash*31 + uint64(b)
			}
		}
		if c.KubernetesAPIReady != "" {
			for _, b := range []byte(c.KubernetesAPIReady) {
				hash = hash*31 + uint64(b)
			}
		}
		if c.ClusterPhase != "" {
			for _, b := range []byte(c.ClusterPhase) {
				hash = hash*31 + uint64(b)
			}
		}
	}
	for _, m := range snapshot.MachineClasses {
		for _, b := range []byte(m.Status) {
			hash = hash*31 + uint64(b)
		}
	}
	return hash
}

// securityHeadersMiddleware adds standard security response headers to every reply.
func securityHeadersMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-XSS-Protection", "1; mode=block")
		w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
		next.ServeHTTP(w, r)
	})
}

// BroadcastState sends current state to all connected WebSocket clients.
func (s *Server) BroadcastState() {
	snapshot := s.appState.Snapshot()
	if data, err := json.Marshal(snapshot); err == nil {
		select {
		case s.broadcast <- data:
		default:
			// Channel full, skip
		}
	}
}
