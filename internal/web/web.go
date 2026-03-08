package web

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"sync"
	"time"

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
	appState              *state.AppState
	triggerHard           chan struct{}
	triggerSoft           chan struct{}
	triggerRefreshCluster chan string
	triggerDeleteCluster  chan string
	triggerDeleteMC       chan string
	triggerMCRefresh         chan struct{}
	triggerMCRefreshSingle   chan string
	triggerRepoChange        chan struct{}
	port                  string
	version               string
	clients               map[*websocket.Conn]bool
	clientsMu             sync.RWMutex
	broadcast             chan []byte
	adminUsername         string
	adminPassword         string
	authDisabled          bool
	sessions              sync.Map // token (string) -> time.Time (login time)
}

// New creates a new web server.
func New(appState *state.AppState, triggerHard chan struct{}, triggerSoft chan struct{}, triggerRefreshCluster chan string, triggerDeleteCluster chan string, triggerDeleteMC chan string, triggerMCRefresh chan struct{}, triggerMCRefreshSingle chan string, triggerRepoChange chan struct{}, port string, version string, adminUsername string, adminPassword string, authDisabled bool) *Server {
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
		port:                  port,
		version:               version,
		clients:               make(map[*websocket.Conn]bool),
		broadcast:             make(chan []byte, 256),
		adminUsername:         adminUsername,
		adminPassword:         adminPassword,
		authDisabled:          authDisabled,
	}

	// Start broadcast handler
	go s.handleBroadcasts()

	// Start state change monitor
	go s.monitorStateChanges()

	return s
}

// Start starts the web server in a goroutine.
func (s *Server) Start() {
	mux := http.NewServeMux()

	// Public routes — no auth required
	mux.HandleFunc("/login", s.handleLogin)
	mux.HandleFunc("/logout", s.handleLogout)

	// WebSocket endpoint (auth checked inside requireAuth)
	mux.HandleFunc("/ws", s.requireAuth(s.handleWebSocket))

	// API endpoints
	mux.HandleFunc("/api/state", s.requireAuth(s.handleState))
	mux.HandleFunc("/api/reconcile", s.requireAuth(s.handleReconcile))
	mux.HandleFunc("/api/check", s.requireAuth(s.handleCheck))
	mux.HandleFunc("/api/clusters-toggle", s.requireAuth(s.handleClustersToggle))
	mux.HandleFunc("/api/force-cluster", s.requireAuth(s.handleForceCluster))
	mux.HandleFunc("/api/refresh-cluster", s.requireAuth(s.handleRefreshCluster))
	mux.HandleFunc("/api/delete-cluster", s.requireAuth(s.handleDeleteCluster))
	mux.HandleFunc("/api/set-cluster-autosync", s.requireAuth(s.handleSetClusterAutoSync))
	mux.HandleFunc("/api/repos", s.requireAuth(s.handleRepos))
	mux.HandleFunc("/api/refresh-mc", s.requireAuth(s.handleRefreshMC))
	mux.HandleFunc("/api/refresh-single-mc", s.requireAuth(s.handleRefreshSingleMC))
	mux.HandleFunc("/api/delete-machineclass", s.requireAuth(s.handleDeleteMachineClass))
	mux.HandleFunc("/api/sync-machineclass", s.requireAuth(s.handleSyncMachineClass))
	mux.HandleFunc("/api/set-mc-autosync", s.requireAuth(s.handleSetMCAutoSync))
	mux.HandleFunc("/api/export-cluster", s.requireAuth(s.handleExportCluster))

	// Serve the UI (all protected)
	mux.HandleFunc("/clusters/", s.requireAuth(s.handleUI))
	mux.HandleFunc("/clusters", s.requireAuth(s.handleUI))
	mux.HandleFunc("/machineclasses", s.requireAuth(s.handleUI))
	mux.HandleFunc("/repos", s.requireAuth(s.handleUI))
	mux.HandleFunc("/users", s.requireAuth(s.handleUI))
	mux.HandleFunc("/", s.requireAuth(s.handleUI))

	addr := fmt.Sprintf(":%s", s.port)
	slog.Info("Web UI listening", "address", addr, "component", "Web")

	go func() {
		if err := http.ListenAndServe(addr, mux); err != nil {
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
