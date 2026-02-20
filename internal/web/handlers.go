package web

import (
	"encoding/json"
	"fmt"
	"net/http"

	"omni-cd/internal/omni"
)

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
