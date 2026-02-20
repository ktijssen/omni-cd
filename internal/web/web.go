package web

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"

	"omni-cd/internal/omni"
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

// handleState returns the current application state as JSON.
func (s *Server) handleState(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	snapshot := s.appState.Snapshot()
	json.NewEncoder(w).Encode(snapshot)
}

// handleReconcile triggers a hard reconcile.
func (s *Server) handleReconcile(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Block sync when version mismatch
	snap := s.appState.Snapshot()
	if snap.VersionMismatch {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusConflict)
		json.NewEncoder(w).Encode(map[string]string{"status": "blocked", "reason": "version mismatch"})
		return
	}

	select {
	case s.triggerHard <- struct{}{}:
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "triggered"})
	default:
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusConflict)
		json.NewEncoder(w).Encode(map[string]string{"status": "already running"})
	}
}

// handleCheck triggers a soft reconcile (git check).
func (s *Server) handleCheck(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	select {
	case s.triggerSoft <- struct{}{}:
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "triggered"})
	default:
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusConflict)
		json.NewEncoder(w).Encode(map[string]string{"status": "already running"})
	}
}

// handleClustersToggle toggles cluster sync on/off at runtime.
func (s *Server) handleClustersToggle(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	newState := s.appState.ToggleClustersEnabled()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"clustersEnabled": newState,
	})
}

// handleForceCluster sets a specific cluster to force sync.
func (s *Server) handleForceCluster(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		ID string `json:"id"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.ID == "" {
		http.Error(w, "Cluster ID is required", http.StatusBadRequest)
		return
	}

	s.appState.SetForceClusterID(req.ID)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status": "ok",
		"id":     req.ID,
	})
}

// handleExportCluster exports an unmanaged cluster as a YAML template.
func (s *Server) handleExportCluster(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		ID string `json:"id"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.ID == "" {
		http.Error(w, "Cluster ID is required", http.StatusBadRequest)
		return
	}

	// Export the cluster template
	yamlContent, err := omni.ExportCluster(req.ID)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to export cluster: %v", err), http.StatusInternalServerError)
		return
	}

	// Return YAML content with appropriate headers for download
	w.Header().Set("Content-Type", "application/x-yaml")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%s.yaml", req.ID))
	w.Write([]byte(yamlContent))
}

// handleUI serves the embedded UI.
func (s *Server) handleUI(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html")
	fmt.Fprint(w, strings.ReplaceAll(uiHTML, "{{APP_VERSION}}", s.version))
}

const uiHTML = `<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="UTF-8">
<meta name="viewport" content="width=device-width, initial-scale=1.0">
<title>Omni CD</title>
<style>
  * { margin: 0; padding: 0; box-sizing: border-box; }
  body {
    font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif;
    background: #1b1b1d;
    color: #e4e4e7;
    min-height: 100vh;
  }
  .container { max-width: 1200px; margin: 0 auto; padding: 24px; }

  .header {
    display: flex;
    justify-content: space-between;
    align-items: center;
    margin-bottom: 32px;
    padding-bottom: 16px;
    border-bottom: 1px solid #27272a;
  }
  .header h1 {
    font-size: 24px;
    font-weight: 700;
    color: #fff;
    letter-spacing: -0.5px;
    display: flex;
    align-items: center;
    gap: 10px;
  }
  .header h1 span { color: #FB326E; margin: 0; padding: 0; }
  .logo { width: 28px; height: 28px; }
  .header-buttons { display: flex; align-items: center; gap: 10px; }
  .btn-check {
    background: #FB326E;
    color: #fff;
    border: none;
    padding: 10px 20px;
    border-radius: 8px;
    font-size: 14px;
    font-weight: 600;
    cursor: pointer;
    transition: all 0.2s;
  }
  .btn-check:hover { background: #e0285f; }
  .btn-check:active { background: #c92255; }
  .btn-check:disabled { background: #27272a; color: #52525b; cursor: not-allowed; }
  .btn-reconcile {
    background: #FB326E;
    color: #fff;
    border: none;
    padding: 10px 20px;
    border-radius: 8px;
    font-size: 14px;
    font-weight: 600;
    cursor: pointer;
    transition: all 0.2s;
  }
  .btn-reconcile:hover { background: #e0285f; }
  .btn-reconcile:active { background: #c92255; }
  .btn-reconcile:disabled { background: #27272a; color: #52525b; cursor: not-allowed; }

  .status-bar {
    display: grid;
    grid-template-columns: repeat(auto-fit, minmax(250px, 1fr));
    gap: 16px;
    margin-bottom: 24px;
  }
  .status-card {
    background: #27272a;
    border: 1px solid #3f3f46;
    border-radius: 12px;
    padding: 20px;
  }
  .status-card .label {
    font-size: 11px;
    text-transform: uppercase;
    letter-spacing: 1px;
    color: #71717a;
    margin-bottom: 8px;
  }
  .status-card .value {
    font-size: 18px;
    font-weight: 600;
    color: #fff;
    word-break: break-all;
  }
  .status-card .sub {
    font-size: 12px;
    color: #a1a1aa;
    margin-top: 4px;
  }

  .badge {
    display: inline-block;
    padding: 4px 10px;
    border-radius: 6px;
    font-size: 12px;
    font-weight: 600;
    margin-left: 3px;
  }
  .badge-success { background: #14532d; color: #4ade80; }
  .badge-running { background: #1e3a5f; color: #60a5fa; }
  .badge-failed { background: #451a1e; color: #f87171; }
  .badge-outofsync { background: #431407; color: #fb923c; }
  .badge-unmanaged { background: #27272a; color: #71717a; border: 1px solid #3f3f46; }
  .badge-deleting { background: #4d1500; color: #fb7a37; }
  .badge-syncing { background: #0d2d2a; color: #2dd4bf; }
  .badge-idle { background: #3f3f46; color: #a1a1aa; }

  .provision-type {
    display: inline-block;
    padding: 2px 8px;
    border-radius: 4px;
    font-size: 10px;
    font-weight: 600;
    text-transform: uppercase;
    margin-right: 8px;
    border: 1px solid;
  }
  .provision-type.auto {
    background: #1e3a5f;
    color: #60a5fa;
    border-color: #60a5fa;
  }
  .provision-type.manual {
    background: #431407;
    color: #fb923c;
    border-color: #fb923c;
  }

  .version-warning {
    background: #431407;
    border: 1px solid #fb923c;
    border-radius: 8px;
    padding: 8px 14px;
    color: #fb923c;
    font-size: 12px;
    font-weight: 500;
    display: flex;
    align-items: center;
    gap: 6px;
    white-space: nowrap;
  }
  .version-warning .warn-icon { font-size: 14px; }

  /* Toggle switch */
  .toggle-switch {
    position: relative;
    width: 36px;
    height: 20px;
    background: #3f3f46;
    border-radius: 10px;
    cursor: pointer;
    transition: background 0.2s;
    border: none;
    padding: 0;
    flex-shrink: 0;
  }
  .toggle-switch.on { background: #FB326E; }
  .toggle-switch .toggle-knob {
    position: absolute;
    top: 2px;
    left: 2px;
    width: 16px;
    height: 16px;
    background: #fff;
    border-radius: 50%;
    transition: transform 0.2s;
  }
  .toggle-switch.on .toggle-knob { transform: translateX(16px); }
  .panel-header-right {
    display: flex;
    align-items: center;
    gap: 8px;
  }
  .toggle-status { font-size: 11px; font-weight: 600; }
  .toggle-status.on { color: #4ade80; }
  .toggle-status.off { color: #f87171; }

  .panels {
    display: grid;
    grid-template-columns: 1fr 1fr;
    gap: 16px;
    margin-bottom: 24px;
  }
  @media (max-width: 768px) { .panels { grid-template-columns: 1fr; } }
  .panel {
    background: #27272a;
    border: 1px solid #3f3f46;
    border-radius: 12px;
    overflow: hidden;
  }
  .panel-header {
    padding: 16px 20px;
    border-bottom: 1px solid #3f3f46;
    font-size: 14px;
    font-weight: 600;
    color: #fff;
    display: flex;
    justify-content: space-between;
    align-items: center;
  }
  .panel-header .count {
    background: #3f3f46;
    padding: 2px 8px;
    border-radius: 4px;
    font-size: 12px;
    color: #a1a1aa;
  }
  .resource-list { padding: 8px 0; }
  .resource-item {
    padding: 10px 20px;
    display: flex;
    justify-content: space-between;
    align-items: center;
    font-size: 13px;
    border-bottom: 1px solid #1b1b1d;
  }
  .resource-item:last-child { border-bottom: none; }
  .resource-id { font-family: 'SF Mono', 'Fira Code', monospace; color: #e4e4e7; }
  .resource-id.clickable { cursor: pointer; }
  .resource-id.clickable:hover { color: #FB326E; }
  .resource-right { display: flex; align-items: center; gap: 8px; }
  .btn-diff {
    background: none;
    border: 1px solid #3f3f46;
    color: #a1a1aa;
    padding: 2px 8px;
    border-radius: 4px;
    font-size: 11px;
    cursor: pointer;
    font-family: 'SF Mono', 'Fira Code', monospace;
  }
  .btn-diff:hover { border-color: #fb923c; color: #fb923c; }
  .btn-sync {
    background: none;
    border: 1px solid #ca8a04;
    color: #fbbf24;
    padding: 2px 8px;
    border-radius: 4px;
    font-size: 11px;
    cursor: pointer;
    font-family: 'SF Mono', 'Fira Code', monospace;
  }
  .btn-sync:hover { border-color: #fbbf24; background: rgba(251, 191, 36, 0.1); }
  .btn-export {
    background: none;
    border: 1px solid #0891b2;
    color: #22d3ee;
    padding: 2px 8px;
    border-radius: 4px;
    font-size: 11px;
    cursor: pointer;
    font-family: 'SF Mono', 'Fira Code', monospace;
    margin-right: 8px;
  }
  .btn-export:hover { border-color: #22d3ee; background: rgba(34, 211, 238, 0.1); }
  .btn-sort { background: none; border: 1px solid #3f3f46; color: #71717a; padding: 2px 8px; border-radius: 4px; font-size: 11px; cursor: pointer; font-family: 'SF Mono', 'Fira Code', monospace; }
  .btn-sort:hover { border-color: #a1a1aa; color: #a1a1aa; }
  .btn-sort.active { border-color: #FB326E; color: #FB326E; background: rgba(251, 50, 110, 0.1); }
  .diff-viewer {
    background: #18181b;
    border-top: 1px solid #3f3f46;
    padding: 12px 20px;
    font-family: 'SF Mono', 'Fira Code', monospace;
    font-size: 12px;
    line-height: 1.6;
    white-space: pre-wrap;
    word-break: break-all;
    max-height: 300px;
    overflow-y: auto;
    color: #a1a1aa;
  }
  .diff-viewer::-webkit-scrollbar { width: 6px; }
  .diff-viewer::-webkit-scrollbar-track { background: #18181b; }
  .diff-viewer::-webkit-scrollbar-thumb { background: #3f3f46; border-radius: 3px; }
  .diff-add { color: #4ade80; }
  .diff-del { color: #f87171; }
  .diff-hdr { color: #60a5fa; }

  .logs-panel {
    background: #27272a;
    border: 1px solid #3f3f46;
    border-radius: 12px;
    overflow: hidden;
  }
  .logs-panel.collapsed .logs-container {
    display: none;
  }
  .logs-header {
    padding: 16px 20px;
    border-bottom: 1px solid #3f3f46;
    font-size: 14px;
    font-weight: 600;
    color: #fff;
    display: flex;
    justify-content: space-between;
    align-items: center;
    cursor: pointer;
    user-select: none;
  }
  .logs-header:hover {
    background: rgba(255, 255, 255, 0.02);
  }
  .logs-toggle {
    font-size: 18px;
    color: #a1a1aa;
    transition: transform 0.2s;
  }
  .logs-panel.collapsed .logs-toggle {
    transform: rotate(-90deg);
  }
  .logs-panel.collapsed .logs-header {
    border-bottom: none;
  }
  .logs-container {
    height: 400px;
    overflow-y: auto;
    padding: 12px 0;
    font-family: 'SF Mono', 'Fira Code', monospace;
    font-size: 12px;
    line-height: 1.6;
  }
  .logs-container::-webkit-scrollbar { width: 6px; }
  .logs-container::-webkit-scrollbar-track { background: #1b1b1d; }
  .logs-container::-webkit-scrollbar-thumb { background: #3f3f46; border-radius: 3px; }
  .log-entry { padding: 2px 20px; }
  .log-entry:hover { background: #323235; }
  .log-ts { color: #52525b; }
  .log-info { color: #e4e4e7; }
  .log-warn { color: #facc15; }
  .log-error { color: #f87171; }
  .log-label { color: #a1a1aa; }
  .log-msg { color: #e4e4e7; }

  .refresh-indicator {
    font-size: 11px;
    color: #52525b;
    text-align: center;
    padding: 16px;
  }

  /* Modal */
  .modal {
    display: none;
    position: fixed;
    z-index: 1000;
    left: 0;
    top: 0;
    width: 100%;
    height: 100%;
    background-color: rgba(0, 0, 0, 0.7);
    animation: fadeIn 0.2s;
  }
  .modal.show { display: flex; align-items: center; justify-content: center; }
  @keyframes fadeIn {
    from { opacity: 0; }
    to { opacity: 1; }
  }
  .modal-content {
    background: #27272a;
    border: 1px solid #3f3f46;
    border-radius: 12px;
    max-width: 900px;
    width: 90%;
    max-height: 85vh;
    display: flex;
    flex-direction: column;
    animation: slideIn 0.2s;
  }
  @keyframes slideIn {
    from { transform: translateY(-20px); opacity: 0; }
    to { transform: translateY(0); opacity: 1; }
  }
  .modal-header {
    padding: 20px 24px 0;
    display: flex;
    justify-content: space-between;
    align-items: center;
  }
  .modal-title {
    font-size: 16px;
    font-weight: 600;
    color: #fff;
    font-family: 'SF Mono', 'Fira Code', monospace;
  }
  .modal-close {
    background: none;
    border: none;
    color: #a1a1aa;
    font-size: 24px;
    cursor: pointer;
    padding: 0;
    width: 32px;
    height: 32px;
    display: flex;
    align-items: center;
    justify-content: center;
    border-radius: 6px;
    transition: all 0.2s;
  }
  .modal-close:hover { background: #3f3f46; color: #fff; }
  .modal-tabs {
    display: flex;
    gap: 4px;
    padding: 0 24px;
    margin-top: 16px;
    border-bottom: 1px solid #3f3f46;
  }
  .modal-tab {
    background: none;
    border: none;
    color: #a1a1aa;
    padding: 10px 16px;
    font-size: 13px;
    font-weight: 500;
    cursor: pointer;
    border-bottom: 2px solid transparent;
    transition: all 0.2s;
  }
  .modal-tab:hover { color: #e4e4e7; }
  .modal-tab.active {
    color: #FB326E;
    border-bottom-color: #FB326E;
  }
  .modal-body {
    padding: 24px;
    overflow-y: auto;
    flex: 1;
    font-family: 'SF Mono', 'Fira Code', monospace;
    font-size: 13px;
    line-height: 1.6;
    color: #e4e4e7;
    white-space: pre-wrap;
    word-break: break-all;
  }
  .modal-body::-webkit-scrollbar { width: 8px; }
  .modal-body::-webkit-scrollbar-track { background: #1b1b1d; }
  .modal-body::-webkit-scrollbar-thumb { background: #3f3f46; border-radius: 4px; }

  /* Confirmation modal */
  .confirm-modal {
    max-width: 500px;
  }
  .confirm-body {
    padding: 32px 24px;
    text-align: center;
    white-space: normal;
  }
  .confirm-icon {
    font-size: 48px;
    margin-bottom: 16px;
  }
  .confirm-message {
    font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif;
    font-size: 14px;
    line-height: 1.6;
    color: #e4e4e7;
    margin-bottom: 24px;
    white-space: pre-line;
  }
  .confirm-actions {
    display: flex;
    gap: 12px;
    justify-content: center;
  }
  .btn-cancel, .btn-confirm {
    padding: 8px 24px;
    border-radius: 6px;
    font-size: 14px;
    font-weight: 500;
    cursor: pointer;
    border: none;
    transition: all 0.2s;
  }
  .btn-cancel {
    background: #3f3f46;
    color: #e4e4e7;
  }
  .btn-cancel:hover {
    background: #52525b;
  }
  .btn-confirm {
    background: #FB326E;
    color: #fff;
  }
  .btn-confirm:hover {
    background: #e91e63;
  }

  /* Pagination */
  .pagination {
    display: flex;
    justify-content: center;
    align-items: center;
    gap: 4px;
    padding: 12px 20px;
    border-top: 1px solid #1b1b1d;
  }
  .page-btn {
    background: none;
    border: 1px solid #3f3f46;
    color: #a1a1aa;
    padding: 4px 10px;
    border-radius: 4px;
    font-size: 12px;
    cursor: pointer;
    min-width: 32px;
    transition: all 0.2s;
  }
  .page-btn:hover:not(:disabled) {
    border-color: #FB326E;
    color: #FB326E;
  }
  .page-btn.active {
    background: #FB326E;
    border-color: #FB326E;
    color: #fff;
  }
  .page-btn:disabled {
    opacity: 0.3;
    cursor: not-allowed;
  }
</style>
</head>
<body>
<div class="container" id="app"></div>
<script>
(function() {
  var app = document.getElementById('app');
  var appVersion = '{{APP_VERSION}}';
  var state = null;
  var autoScroll = true;
  var machineClassPage = 1;
  var machineClassSortAZ = true;
  var clusterPage = 1;
  var clusterSortAZ = true;
  var pageSize = 5;
  var logsCollapsed = true;
  var ws = null;
  var wsReconnectDelay = 1000;
  var wsReconnectTimer = null;

  function ts(d) {
    if (!d) return '-';
    var dt = new Date(d);
    if (isNaN(dt)) return '-';
    return dt.toLocaleTimeString();
  }

  function ago(d) {
    if (!d) return '';
    var dt = new Date(d);
    if (isNaN(dt)) return '';
    var s = Math.floor((Date.now() - dt.getTime()) / 1000);
    if (s < 5) return 'just now';
    if (s < 60) return s + 's ago';
    if (s < 3600) return Math.floor(s / 60) + 'm ago';
    return Math.floor(s / 3600) + 'h ago';
  }

  var currentModal = null;
  var confirmModal = null;

  function showClusterModal(id) {
    if (!state || !state.clusters) return;
    var cluster = state.clusters.find(function(c) { return c.id === id; });
    if (!cluster) return;
    currentModal = {
      id: id,
      fileContent: cluster.fileContent || '',
      liveContent: cluster.liveContent || '',
      diff: cluster.diff || '',
      error: cluster.error || '',
      activeTab: cluster.error ? 'error' : 'live',
      type: 'cluster'
    };
    render();
  }

  function showMachineClassModal(id) {
    if (!state || !state.machineClasses) return;
    var mc = state.machineClasses.find(function(m) { return m.id === id; });
    if (!mc) return;
    currentModal = {
      id: id,
      fileContent: mc.fileContent || '',
      liveContent: mc.liveContent || '',
      diff: mc.diff || '',
      error: mc.error || '',
      activeTab: mc.error ? 'error' : 'live',
      type: 'machineclass'
    };
    render();
  }

  function setModalTab(tab) {
    if (currentModal) {
      currentModal.activeTab = tab;
      render();
    }
  }

  function closeModal() {
    currentModal = null;
    render();
  }

  function formatDiff(raw) {
    if (!raw) return '';
    var text = raw.replace(/\\n/g, '\n');
    return text.split('\n').map(function(line) {
      if (line.startsWith('+')) return '<span class="diff-add">' + escHtml(line) + '</span>';
      if (line.startsWith('-')) return '<span class="diff-del">' + escHtml(line) + '</span>';
      if (line.startsWith('@@') || line.startsWith('---') || line.startsWith('+++')) return '<span class="diff-hdr">' + escHtml(line) + '</span>';
      return escHtml(line);
    }).join('\n');
  }

  function escHtml(s) {
    return s.replace(/&/g,'&amp;').replace(/</g,'&lt;').replace(/>/g,'&gt;');
  }

  window.__showClusterModal = showClusterModal;
  window.__setModalTab = setModalTab;
  window.__closeModal = closeModal;

  function badgeClass(st) {
    if (!st) return 'badge-idle';
    if (st === 'success' || st === 'applied' || st === 'synced') return 'badge-success';
    if (st === 'running') return 'badge-running';
    if (st === 'failed') return 'badge-failed';
    if (st === 'outofsync' || st === 'out of sync') return 'badge-outofsync';
    if (st === 'unmanaged') return 'badge-unmanaged';
    if (st === 'syncing') return 'badge-syncing';
    if (st === 'deleting') return 'badge-deleting';
    return 'badge-idle';
  }

  function getOmniHealth(s) {
    if (!s || !s.omniHealth || !s.omniHealth.lastCheck) return { status: 'unknown', label: 'Unknown' };
    if (s.omniHealth.status === 'healthy') return { status: 'healthy', label: 'Healthy' };
    if (s.omniHealth.status === 'failed') return { status: 'failed', label: 'Unreachable' };
    return { status: 'unknown', label: 'Unknown' };
  }

  function getGitHealth(s) {
    if (!s || !s.git) return { status: 'unknown', label: 'Unknown' };

    // Check if we have valid git data
    if (!s.git.sha || !s.git.lastSync) {
      return { status: 'disconnected', label: 'Disconnected' };
    }

    // Check if last sync was recent (within 10 minutes)
    var lastSync = new Date(s.git.lastSync);
    var now = Date.now();
    var minutesSinceSync = Math.floor((now - lastSync.getTime()) / 1000 / 60);

    // If sync is very old, something might be wrong
    if (minutesSinceSync > 10) {
      return { status: 'stale', label: 'Stale' };
    }

    // Check if last reconcile failed
    if (s.lastReconcile && s.lastReconcile.status === 'failed') {
      return { status: 'degraded', label: 'Degraded' };
    }

    return { status: 'healthy', label: 'Healthy' };
  }

  function gitHealthBadgeClass(status) {
    if (status === 'healthy') return 'badge-success';
    if (status === 'degraded') return 'badge-outofsync';
    if (status === 'stale') return 'badge-outofsync';
    if (status === 'disconnected') return 'badge-failed';
    return 'badge-idle';
  }

  function logClass(level) {
    if (level === 'WARN') return 'log-warn';
    if (level === 'ERROR') return 'log-error';
    return 'log-info';
  }

  async function fetchState() {
    try {
      var r = await fetch('/api/state');
      state = await r.json();
      // Don't re-render if modal is open to prevent flashing
      if (!currentModal && !confirmModal) {
        render();
      }
    } catch(e) {}
  }

  async function checkGit() {
    try {
      var r = await fetch('/api/check', { method: 'POST' });
      var d = await r.json();
      if (d.status === 'already running') alert('Reconcile already in progress');
      fetchState();
    } catch(e) {
      alert('Failed to trigger git check');
    }
  }

  async function triggerReconcile() {
    try {
      var r = await fetch('/api/reconcile', { method: 'POST' });
      var d = await r.json();
      if (d.status === 'already running') alert('Reconcile already in progress');
      fetchState();
    } catch(e) {
      alert('Failed to trigger reconcile');
    }
  }

  async function toggleClusters() {
    try {
      var r = await fetch('/api/clusters-toggle', { method: 'POST' });
      var d = await r.json();
      fetchState();
    } catch(e) {
      alert('Failed to toggle clusters');
    }
  }

  function forceSync(clusterId, event) {
    // Prevent event bubbling
    event.stopPropagation();

    // Show confirmation modal
    confirmModal = {
      clusterId: clusterId,
      title: 'Force Sync Cluster',
      message: 'Are you sure you want to force sync cluster "' + clusterId + '"?\n\nThis will immediately sync the cluster with the configuration from Git.',
      onConfirm: function() {
        confirmModal = null;
        render();
        doForceSync(clusterId);
      }
    };
    render();
  }

  async function doForceSync(clusterId) {
    try {
      // First, set the cluster ID to force sync
      await fetch('/api/force-cluster', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ id: clusterId })
      });

      // Then trigger reconcile
      var r = await fetch('/api/reconcile', { method: 'POST' });
      var d = await r.json();
      if (d.status === 'blocked') {
        alert('Sync blocked: ' + d.reason);
      } else if (d.status === 'already running') {
        alert('Reconcile already in progress');
      } else {
        fetchState();
      }
    } catch(e) {
      alert('Failed to trigger sync');
    }
  }

  function closeConfirmModal() {
    confirmModal = null;
    render();
  }

  async function exportCluster(clusterId, event) {
    // Prevent event bubbling
    event.stopPropagation();

    try {
      var r = await fetch('/api/export-cluster', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ id: clusterId })
      });

      if (!r.ok) {
        alert('Failed to export cluster: ' + r.statusText);
        return;
      }

      // Get the YAML content
      var yamlContent = await r.text();
      
      // Create a blob and download link
      var blob = new Blob([yamlContent], { type: 'application/x-yaml' });
      var url = window.URL.createObjectURL(blob);
      var a = document.createElement('a');
      a.href = url;
      a.download = clusterId + '.yaml';
      document.body.appendChild(a);
      a.click();
      document.body.removeChild(a);
      window.URL.revokeObjectURL(url);
    } catch(e) {
      alert('Failed to export cluster: ' + e.message);
    }
  }

  function confirmAction() {
    if (confirmModal && confirmModal.onConfirm) {
      confirmModal.onConfirm();
    }
  }

  function changeMachineClassPage(page) {
    machineClassPage = page;
    render();
  }

  function changeClusterPage(page) {
    clusterPage = page;
    render();
  }

  function toggleMachineClassSort() {
    machineClassSortAZ = !machineClassSortAZ;
    machineClassPage = 1;
    render();
  }

  function toggleClusterSort() {
    clusterSortAZ = !clusterSortAZ;
    clusterPage = 1;
    render();
  }

  function toggleLogs() {
    logsCollapsed = !logsCollapsed;
    render();
  }

  function paginateItems(items, page) {
    var start = (page - 1) * pageSize;
    var end = start + pageSize;
    return items.slice(start, end);
  }

  function renderPagination(items, currentPage, onPageChange) {
    var totalPages = Math.ceil(items.length / pageSize);
    if (totalPages <= 1) return '';

    var pages = '';
    for (var i = 1; i <= totalPages; i++) {
      pages += '<button class="page-btn ' + (i === currentPage ? 'active' : '') + '" onclick="' + onPageChange + '(' + i + ')">' + i + '</button>';
    }

    return '<div class="pagination">' +
      '<button class="page-btn" onclick="' + onPageChange + '(' + (currentPage - 1) + ')" ' + (currentPage === 1 ? 'disabled' : '') + '>&laquo;</button>' +
      pages +
      '<button class="page-btn" onclick="' + onPageChange + '(' + (currentPage + 1) + ')" ' + (currentPage === totalPages ? 'disabled' : '') + '>&raquo;</button>' +
    '</div>';
  }

  function render() {
    if (!state) {
      app.innerHTML = '<div style="text-align:center;padding:60px;color:#52525b">Loading...</div>';
      return;
    }
    var s = state;
    var isRunning = s.lastReconcile && s.lastReconcile.status === 'running';
    var mismatch = s.versionMismatch;
    var syncDisabled = isRunning || mismatch;

    app.innerHTML =
      '<div class="header">' +
        '<h1>' +
          '<img class="logo" src="https://mintlify.s3.us-west-1.amazonaws.com/siderolabs-fe86397c/images/omni.svg" alt="Omni">' +
          '<div>Omni <span style="color:#FB326E">CD</span></div></h1>' +
        '<div class="header-buttons">' +
          (mismatch ?
            '<div class="version-warning">' +
              '<span class="warn-icon">&#9888;</span>' +
              'Omni ' + s.omniVersion + ' &gt; omnictl ' + s.omnictlVersion +
            '</div>' : '') +
          '<button class="btn-check" onclick="window.__checkGit()" ' +
            (isRunning ? 'disabled' : '') + '>Refresh</button>' +
          '<button class="btn-reconcile" onclick="window.__triggerReconcile()" ' +
            (syncDisabled ? 'disabled' : '') + '>' +
            (isRunning ? 'Syncing...' : 'Sync') +
          '</button>' +
        '</div>' +
      '</div>' +

      (function() {
        var omniHealth = getOmniHealth(s);
        return '<div class="status-bar">' +
        '<div class="status-card">' +
          '<div class="label" style="display:flex;justify-content:space-between;align-items:center">' +
            '<span>Omni Instance</span>' +
            '<span class="badge ' + gitHealthBadgeClass(omniHealth.status) + '">' + omniHealth.label + '</span>' +
          '</div>' +
          '<div class="value">' + (s.omniEndpoint ? '<a href="' + s.omniEndpoint + '" target="_blank" style="color:#FB326E;text-decoration:none">' + s.omniEndpoint + '</a>' : '-') + '</div>' +
          '<div class="sub">Omni: ' + (s.omniVersion || 'unknown') + ' &middot; omnictl: ' + (s.omnictlVersion || 'unknown') + '</div>' +
          '<div class="sub">Last check: ' + ago(s.omniHealth && s.omniHealth.lastCheck) + '</div>' +
          (s.omniHealth && s.omniHealth.error ? '<div class="sub" style="color:#f87171">' + s.omniHealth.error + '</div>' : '') +
        '</div>' +
        '<div class="status-card">' +
          '<div class="label">Resources</div>' +
          '<div class="value">' +
            (s.machineClasses ? s.machineClasses.length : 0) + ' MachineClasses &middot; ' +
            (s.clusters ? s.clusters.length : 0) + ' Clusters</div>' +
          '<div class="sub">Managed by Omni CD</div>' +
        '</div>' +
      '</div>';
      })() +

      (function() {
        var gitHealth = getGitHealth(s);
        return '<div class="status-bar">' +
          '<div class="status-card">' +
            '<div class="label" style="display:flex;justify-content:space-between;align-items:center">' +
              '<span>Git Status</span>' +
              '<span class="badge ' + gitHealthBadgeClass(gitHealth.status) + '">' + gitHealth.label + '</span>' +
            '</div>' +
            '<div class="value">Repository: ' + (s.git.repo ? '<a href="' + s.git.repo + '" target="_blank" style="color:#FB326E;text-decoration:none">' + s.git.repo + '</a>' : '-') + '</div>' +
            '<div class="sub">Branch: ' + (s.git.branch || '-') + '</div>' +
            '<div class="sub">Commit: ' + (s.git.shortSha || '-') + (s.git.commitMessage ? ' - ' + s.git.commitMessage : '') + '</div>' +
            '<div class="sub">Last sync: ' + ago(s.git.lastSync) + '</div>' +
          '</div>' +
          '<div class="status-card">' +
            '<div class="label" style="display:flex;justify-content:space-between;align-items:center">' +
              '<span>Last Reconciliation</span>' +
              '<span class="badge ' + badgeClass(s.lastReconcile.status) + '">' + (s.lastReconcile.status || 'idle') + '</span>' +
            '</div>' +
            '<div class="value">Type: ' + (s.lastReconcile.type === 'soft' ? 'Refresh' : s.lastReconcile.type === 'hard' ? 'Sync' : '-') + '</div>' +
            '<div class="sub">Started: ' + ts(s.lastReconcile.startedAt) + '</div>' +
            '<div class="sub">Finished: ' + ts(s.lastReconcile.finishedAt) + '</div>' +
          '</div>' +
        '</div>';
      })() +

      '<div class="panels">' +
        '<div class="panel">' +
          '<div class="panel-header">Machine Classes ' +
            '<div class="panel-header-right">' +
              '<button class="btn-sort active" onclick="window.__toggleMachineClassSort()">' + (machineClassSortAZ ? 'A→Z' : 'Z→A') + '</button>' +
              '<span class="count">' + (s.machineClasses ? s.machineClasses.length : 0) + '</span>' +
            '</div>' +
          '</div>' +
          '<div class="resource-list">' +
            (s.machineClasses && s.machineClasses.length > 0
              ? paginateItems(s.machineClasses.slice().sort(function(a, b) { return machineClassSortAZ ? a.id.localeCompare(b.id) : b.id.localeCompare(a.id); }), machineClassPage).map(function(r) {
                  var displayStatus = r.status === 'success' ? 'synced' : r.status;
                  var provisionBadge = r.provisionType ? '<span class="provision-type ' + r.provisionType + '">' + r.provisionType + '</span>' : '';
                  var hasDiff = r.diff && r.diff.length > 0;
                  var hasFile = r.fileContent && r.fileContent.length > 0;
                  var hasDetails = hasDiff || hasFile;
                  return '<div class="resource-item">' +
                    '<span class="resource-id' + (hasDetails ? ' clickable' : '') + '"' +
                      (hasDetails ? ' onclick="window.__showMachineClassModal(\'' + r.id + '\')"' : '') + '>' + r.id +
                    '</span><div class="resource-right">' + provisionBadge + '<span class="badge ' + badgeClass(r.status) + '">' +
                    displayStatus + '</span></div></div>';
                }).join('')
              : '<div class="resource-item" style="color:#52525b">No machine classes</div>') +
          '</div>' +
          (s.machineClasses && s.machineClasses.length > pageSize ? renderPagination(s.machineClasses.slice().sort(function(a, b) { return machineClassSortAZ ? a.id.localeCompare(b.id) : b.id.localeCompare(a.id); }), machineClassPage, 'window.__changeMachineClassPage') : '') +
        '</div>' +
        '<div class="panel">' +
          '<div class="panel-header">Clusters ' +
            '<div class="panel-header-right">' +
              '<button class="btn-sort active" onclick="window.__toggleClusterSort()">' + (clusterSortAZ ? 'A→Z' : 'Z→A') + '</button>' +
              '<span class="toggle-status ' + (s.clustersEnabled ? 'on' : 'off') + '">Auto Sync</span>' +
              '<button class="toggle-switch ' + (s.clustersEnabled ? 'on' : '') + '" onclick="window.__toggleClusters()">' +
                '<div class="toggle-knob"></div>' +
              '</button>' +
              '<span class="count">' + (s.clusters ? s.clusters.length : 0) + '</span>' +
            '</div>' +
          '</div>' +
          '<div class="resource-list">' +
            (s.clusters && s.clusters.length > 0
              ? paginateItems(s.clusters.slice().sort(function(a, b) { return clusterSortAZ ? a.id.localeCompare(b.id) : b.id.localeCompare(a.id); }), clusterPage).map(function(r) {
                  var hasDiff = r.diff && r.diff.length > 0;
                  var hasFile = r.fileContent && r.fileContent.length > 0;
                  var isFailed = r.status === 'failed';
                  var hasError = r.error && r.error.length > 0;
                  var hasDetails = hasFile || hasDiff || isFailed || hasError;
                  var isOutOfSync = r.status === 'outofsync';
                  var isUnmanaged = r.status === 'unmanaged';
                  
                  // Build status badges - show both failed and out of sync if applicable
                  var badges = '';
                  if (isFailed && hasDiff) {
                    // Show both out of sync and failed
                    badges = '<span class="badge badge-outofsync">out of sync</span>' +
                             '<span class="badge badge-failed">failed</span>';
                  } else if (r.status === 'outofsync') {
                    badges = '<span class="badge badge-outofsync">out of sync</span>';
                  } else if (r.status === 'success') {
                    badges = '<span class="badge badge-success">synced</span>';
                  } else if (isFailed) {
                    badges = '<span class="badge badge-failed">failed</span>';
                  } else if (isUnmanaged) {
                    badges = '<span class="badge badge-unmanaged">unmanaged</span>';
                  } else if (r.status === 'deleting') {
                    badges = '<span class="badge badge-deleting">deleting</span>';
                  } else if (r.status === 'syncing') {
                    badges = '<span class="badge badge-syncing">syncing</span>';
                  } else {
                    badges = '<span class="badge badge-idle">' + r.status + '</span>';
                  }
                  
                  return '<div class="resource-item">' +
                    '<span class="resource-id' + (hasDetails ? ' clickable' : '') + '"' +
                      (hasDetails ? ' onclick="window.__showClusterModal(\'' + r.id + '\')"' : '') + '>' +
                      r.id +
                    '</span>' +
                    '<div class="resource-right">' +
                      (isUnmanaged ? '<button class="btn-export" onclick="window.__exportCluster(\'' + r.id + '\', event)">export</button>' : '') +
                      (isOutOfSync && !isFailed ? '<button class="btn-sync" onclick="window.__forceSync(\'' + r.id + '\', event)">force sync</button>' : '') +
                      badges +
                    '</div>' +
                  '</div>';
                }).join('')
              : '<div class="resource-item" style="color:#52525b">No clusters</div>') +
          '</div>' +
          (s.clusters && s.clusters.length > pageSize ? renderPagination(s.clusters.slice().sort(function(a, b) { return clusterSortAZ ? a.id.localeCompare(b.id) : b.id.localeCompare(a.id); }), clusterPage, 'window.__changeClusterPage') : '') +
        '</div>' +
      '</div>' +

      '<div class="logs-panel' + (logsCollapsed ? ' collapsed' : '') + '">' +
        '<div class="logs-header" onclick="window.__toggleLogs()">' +
          '<span>Logs</span>' +
          '<span class="logs-toggle">▼</span>' +
        '</div>' +
        '<div class="logs-container" id="logs">' +
          (s.logs && s.logs.length > 0
            ? s.logs.map(function(l) {
                return '<div class="log-entry">' +
                  '<span class="log-msg">' + l.message + '</span></div>';
              }).join('')
            : '<div class="log-entry" style="color:#52525b">No logs yet</div>') +
        '</div>' +
      '</div>' +

      '<div class="refresh-indicator">Omni CD ' + appVersion + ' · Real-time updates</div>' +

      '<div class="modal ' + (currentModal ? 'show' : '') + '" onclick="if(event.target === this) window.__closeModal()">' +
        '<div class="modal-content" onclick="event.stopPropagation()">' +
          '<div class="modal-header">' +
            '<div class="modal-title">' + (currentModal ? currentModal.id : '') + '</div>' +
            '<button class="modal-close" onclick="window.__closeModal()">&times;</button>' +
          '</div>' +
          (currentModal ?
            '<div class="modal-tabs">' +
              (currentModal.error ? '<button class="modal-tab ' + (currentModal.activeTab === 'error' ? 'active' : '') + '" onclick="window.__setModalTab(\'error\')">Error</button>' : '') +
              '<button class="modal-tab ' + (currentModal.activeTab === 'live' ? 'active' : '') + '" onclick="window.__setModalTab(\'live\')">Live</button>' +
              (currentModal.type === 'cluster' ? '<button class="modal-tab ' + (currentModal.activeTab === 'diff' ? 'active' : '') + '" onclick="window.__setModalTab(\'diff\')">Diff</button>' : '') +
            '</div>' : '') +
          '<div class="modal-body">' +
            (currentModal ?
              (currentModal.activeTab === 'error' ? '<div style="color:#f87171;white-space:pre-wrap;">' + escHtml(currentModal.error) + '</div>' :
               currentModal.activeTab === 'live' ? (currentModal.liveContent ? '<pre style="margin:0;white-space:pre-wrap;">' + escHtml(currentModal.liveContent) + '</pre>' : '<div style="color:#71717a;text-align:center;padding:40px;">No live state available</div>') :
               currentModal.activeTab === 'diff' ? (currentModal.diff ? '<pre style="margin:0;white-space:pre-wrap;">' + formatDiff(currentModal.diff) + '</pre>' : '<div style="color:#71717a;text-align:center;padding:40px;">No diff available</div>') :
               '<div style="color:#71717a;text-align:center;padding:40px;">No content available</div>')
            : '') +
          '</div>' +
        '</div>' +
      '</div>' +

      '<div class="modal ' + (confirmModal ? 'show' : '') + '" onclick="if(event.target === this) window.__closeConfirmModal()">' +
        '<div class="modal-content confirm-modal" onclick="event.stopPropagation()">' +
          '<div class="modal-header">' +
            '<div class="modal-title">' + (confirmModal ? confirmModal.title : '') + '</div>' +
            '<button class="modal-close" onclick="window.__closeConfirmModal()">&times;</button>' +
          '</div>' +
          '<div class="modal-body confirm-body">' +
            '<div class="confirm-icon">⚠️</div>' +
            '<div class="confirm-message">' + (confirmModal ? confirmModal.message : '') + '</div>' +
            '<div class="confirm-actions">' +
              '<button class="btn-cancel" onclick="window.__closeConfirmModal()">Cancel</button>' +
              '<button class="btn-confirm" onclick="window.__confirmAction()">Confirm</button>' +
            '</div>' +
          '</div>' +
        '</div>' +
      '</div>';

    if (autoScroll) {
      var el = document.getElementById('logs');
      if (el) el.scrollTop = el.scrollHeight;
    }
  }

  window.__triggerReconcile = triggerReconcile;
  window.__checkGit = checkGit;
  window.__toggleClusters = toggleClusters;
  window.__forceSync = forceSync;
  window.__exportCluster = exportCluster;
  window.__closeConfirmModal = closeConfirmModal;
  window.__confirmAction = confirmAction;
  window.__changeMachineClassPage = changeMachineClassPage;
  window.__changeClusterPage = changeClusterPage;
  window.__toggleMachineClassSort = toggleMachineClassSort;
  window.__toggleClusterSort = toggleClusterSort;
  window.__toggleLogs = toggleLogs;
  window.__showMachineClassModal = showMachineClassModal;

  // WebSocket connection
  function connectWebSocket() {
    var protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
    var wsUrl = protocol + '//' + window.location.host + '/ws';
    
    try {
      ws = new WebSocket(wsUrl);
      
      ws.onopen = function() {
        console.log('WebSocket connected');
        wsReconnectDelay = 1000; // Reset reconnect delay on successful connection
      };
      
      ws.onmessage = function(event) {
        try {
          state = JSON.parse(event.data);
          render();
        } catch(e) {
          console.error('Failed to parse WebSocket message:', e);
        }
      };
      
      ws.onclose = function() {
        console.log('WebSocket disconnected, reconnecting...');
        ws = null;
        // Exponential backoff with max 10 seconds
        wsReconnectDelay = Math.min(wsReconnectDelay * 1.5, 10000);
        wsReconnectTimer = setTimeout(connectWebSocket, wsReconnectDelay);
      };
      
      ws.onerror = function(error) {
        console.error('WebSocket error:', error);
      };
    } catch(e) {
      console.error('Failed to create WebSocket:', e);
      wsReconnectTimer = setTimeout(connectWebSocket, wsReconnectDelay);
    }
  }

  // Close modal on ESC key
  document.addEventListener('keydown', function(e) {
    if (e.key === 'Escape') {
      if (confirmModal) {
        closeConfirmModal();
      } else if (currentModal) {
        closeModal();
      }
    }
  });

  // Start WebSocket connection
  connectWebSocket();
  
  // Fallback polling (only if WebSocket is disconnected)
  setInterval(function() {
    if (!ws || ws.readyState !== WebSocket.OPEN) {
      fetchState();
    }
  }, 5000);
})();
</script>
</body>
</html>`
