package web

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"omni-cd/internal/state"

	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true // Allow all origins for simplicity
	},
}

// Server serves the web UI and API endpoints.
type Server struct {
	appState    *state.AppState
	triggerHard chan struct{}
	triggerSoft chan struct{}
	port        string
	version     string
	clients     map[*websocket.Conn]bool
	clientsMu   sync.RWMutex
	broadcast   chan []byte
}

// New creates a new web server.
func New(appState *state.AppState, triggerHard chan struct{}, triggerSoft chan struct{}, port string, version string) *Server {
	s := &Server{
		appState:    appState,
		triggerHard: triggerHard,
		triggerSoft: triggerSoft,
		port:        port,
		version:     version,
		clients:     make(map[*websocket.Conn]bool),
		broadcast:   make(chan []byte, 256),
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

	// WebSocket endpoint
	mux.HandleFunc("/ws", s.handleWebSocket)

	// API endpoints
	mux.HandleFunc("/api/state", s.handleState)
	mux.HandleFunc("/api/reconcile", s.handleReconcile)
	mux.HandleFunc("/api/check", s.handleCheck)
	mux.HandleFunc("/api/clusters-toggle", s.handleClustersToggle)
	mux.HandleFunc("/api/force-cluster", s.handleForceCluster)
	mux.HandleFunc("/api/export-cluster", s.handleExportCluster)

	// Serve the UI
	mux.HandleFunc("/clusters", s.handleUI)
	mux.HandleFunc("/", s.handleUI)

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
	if snapshot.VersionMismatch {
		hash = hash * 37
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
