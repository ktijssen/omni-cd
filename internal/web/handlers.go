package web

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"omni-cd/internal/config"
	"omni-cd/internal/git"
	"omni-cd/internal/omni"
	"omni-cd/internal/omniinstance"
	"omni-cd/internal/state"
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

// handleWebhook accepts push events from GitHub (and in future GitLab) and
// triggers a soft reconcile. The endpoint is public but protected by an
// HMAC-SHA256 signature when WEBHOOK_SECRET is set.
func (s *Server) handleWebhook(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20)) // 1 MiB limit
	if err != nil {
		http.Error(w, "Failed to read body", http.StatusBadRequest)
		return
	}

	// Validate HMAC signature when a secret is configured.
	if s.webhookSecret != "" {
		sig := r.Header.Get("X-Hub-Signature-256")
		if sig == "" {
			http.Error(w, "Missing signature", http.StatusUnauthorized)
			return
		}
		mac := hmac.New(sha256.New, []byte(s.webhookSecret))
		mac.Write(body)
		expected := "sha256=" + hex.EncodeToString(mac.Sum(nil))
		if !hmac.Equal([]byte(sig), []byte(expected)) {
			http.Error(w, "Invalid signature", http.StatusUnauthorized)
			return
		}
	}

	// Only act on push events; ignore everything else silently.
	event := r.Header.Get("X-GitHub-Event")
	if event != "push" {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "ignored", "event": event})
		return
	}

	slog.Info("Webhook push event received, triggering soft reconcile", "component", "Web")
	s.appState.AppendAudit(state.AuditEntry{User: "webhook", Action: "refresh", Kind: "global"})
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

// handleReconcile triggers a hard reconcile.
func (s *Server) handleReconcile(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	select {
	case s.triggerHard <- struct{}{}:
		s.appState.AppendAudit(state.AuditEntry{User: s.sessionIdentity(r), Action: "sync", Kind: "global"})
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
		s.appState.AppendAudit(state.AuditEntry{User: s.sessionIdentity(r), Action: "refresh", Kind: "global"})
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
	action := "global-sync-on"
	if !newState {
		action = "global-sync-off"
	}
	s.appState.AppendAudit(state.AuditEntry{User: s.sessionIdentity(r), Action: action, Kind: "global"})
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"clustersEnabled": newState,
	})
}

// handleRefreshCluster triggers a git-only refresh for a single cluster.
func (s *Server) handleRefreshCluster(w http.ResponseWriter, r *http.Request) {
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

	select {
	case s.triggerRefreshCluster <- req.ID:
		s.appState.AppendAudit(state.AuditEntry{User: s.sessionIdentity(r), Action: "refresh", Resource: req.ID, Kind: "cluster"})
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "triggered"})
	default:
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusConflict)
		json.NewEncoder(w).Encode(map[string]string{"status": "already running"})
	}
}

// handleRefreshSingleMC triggers a git pull + diff-only refresh for a single machine class.
func (s *Server) handleRefreshSingleMC(w http.ResponseWriter, r *http.Request) {
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
		http.Error(w, "Machine class ID is required", http.StatusBadRequest)
		return
	}
	select {
	case s.triggerMCRefreshSingle <- req.ID:
		s.appState.AppendAudit(state.AuditEntry{User: s.sessionIdentity(r), Action: "refresh", Resource: req.ID, Kind: "machineclass"})
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "triggered"})
	default:
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusConflict)
		json.NewEncoder(w).Encode(map[string]string{"status": "already running"})
	}
}

// handleRefreshMC triggers a diff-only refresh of machine classes (no apply).
func (s *Server) handleRefreshMC(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	select {
	case s.triggerMCRefresh <- struct{}{}:
		s.appState.AppendAudit(state.AuditEntry{User: s.sessionIdentity(r), Action: "refresh", Kind: "machineclass"})
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "triggered"})
	default:
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusConflict)
		json.NewEncoder(w).Encode(map[string]string{"status": "already running"})
	}
}

// handleSetClusterAutoSync sets the per-cluster AutoSync preference.
func (s *Server) handleSetClusterAutoSync(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		ID       string `json:"id"`
		AutoSync bool   `json:"autoSync"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}
	if req.ID == "" {
		http.Error(w, "Cluster ID is required", http.StatusBadRequest)
		return
	}
	s.appState.SetClusterAutoSync(req.ID, req.AutoSync)
	autoSyncAction := "auto-sync-on"
	if !req.AutoSync {
		autoSyncAction = "auto-sync-off"
	}
	s.appState.AppendAudit(state.AuditEntry{User: s.sessionIdentity(r), Action: autoSyncAction, Resource: req.ID, Kind: "cluster"})
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"status": "ok", "id": req.ID, "autoSync": req.AutoSync})
}

// handleDeleteCluster triggers deletion of a specific cluster without blocking the global reconcile.
func (s *Server) handleDeleteCluster(w http.ResponseWriter, r *http.Request) {
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

	// Reject duplicate delete requests — if the cluster is already being
	// deleted there is nothing to do and we don't want to queue another
	// operation on top of the in-flight one.
	for _, c := range s.appState.GetClusters() {
		if c.ID == req.ID && c.Status == "deleting" {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusConflict)
			json.NewEncoder(w).Encode(map[string]string{"status": "already deleting"})
			return
		}
	}

	select {
	case s.triggerDeleteCluster <- req.ID:
	default:
	}
	s.appState.AppendAudit(state.AuditEntry{User: s.sessionIdentity(r), Action: "delete", Resource: req.ID, Kind: "cluster"})
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "triggered"})
}

// handleDeleteMachineClass triggers deletion of a specific machine class from Omni.
func (s *Server) handleDeleteMachineClass(w http.ResponseWriter, r *http.Request) {
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
		http.Error(w, "Machine class ID is required", http.StatusBadRequest)
		return
	}

	select {
	case s.triggerDeleteMC <- req.ID:
	default:
	}
	s.appState.AppendAudit(state.AuditEntry{User: s.sessionIdentity(r), Action: "delete", Resource: req.ID, Kind: "machineclass"})
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "triggered"})
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

	s.appState.AddForceClusterID(req.ID)
	s.appState.AppendAudit(state.AuditEntry{User: s.sessionIdentity(r), Action: "sync", Resource: req.ID, Kind: "cluster"})
	select {
	case s.triggerHard <- struct{}{}:
	default:
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status": "ok",
		"id":     req.ID,
	})
}

// handleSyncMachineClass queues a machine class ID for force-sync on the next reconcile.
func (s *Server) handleSyncMachineClass(w http.ResponseWriter, r *http.Request) {
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
		http.Error(w, "Machine class ID is required", http.StatusBadRequest)
		return
	}

	s.appState.AddForceMCID(req.ID)
	s.appState.AppendAudit(state.AuditEntry{User: s.sessionIdentity(r), Action: "sync", Resource: req.ID, Kind: "machineclass"})
	select {
	case s.triggerHard <- struct{}{}:
	default:
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status": "ok",
		"id":     req.ID,
	})
}

// handleSetMCAutoSync sets the per-machine-class AutoSync preference.
func (s *Server) handleSetMCAutoSync(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		ID       string `json:"id"`
		AutoSync bool   `json:"autoSync"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}
	if req.ID == "" {
		http.Error(w, "Machine class ID is required", http.StatusBadRequest)
		return
	}
	s.appState.SetMachineClassAutoSync(req.ID, req.AutoSync)
	mcAutoSyncAction := "auto-sync-on"
	if !req.AutoSync {
		mcAutoSyncAction = "auto-sync-off"
	}
	s.appState.AppendAudit(state.AuditEntry{User: s.sessionIdentity(r), Action: mcAutoSyncAction, Resource: req.ID, Kind: "machineclass"})
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"status": "ok", "id": req.ID, "autoSync": req.AutoSync})
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

	// Sanitize cluster ID for use in Content-Disposition to prevent header injection.
	safeID := strings.Map(func(r rune) rune {
		if r < 0x20 || r == 0x7f || r == '"' || r == ';' || r == '\\' {
			return -1
		}
		return r
	}, req.ID)

	s.appState.AppendAudit(state.AuditEntry{User: s.sessionIdentity(r), Action: "export", Resource: req.ID, Kind: "cluster"})

	// Return YAML content with appropriate headers for download
	w.Header().Set("Content-Type", "application/x-yaml")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s.yaml"`, safeID))
	w.Write([]byte(yamlContent))
}

// handleClusterManifests returns the KubernetesManifestGroup sync status for a cluster.
func (s *Server) handleClusterManifests(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	id := r.URL.Query().Get("id")
	if id == "" {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "id is required"})
		return
	}
	status, err := omni.GetClusterManifestStatus(id)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}
	if status == nil {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	json.NewEncoder(w).Encode(status)
}

// handleRepos handles CRUD operations for git repository configurations.
//
//	POST   /api/repos         — add a new repo
//	PUT    /api/repos         — update an existing repo (name in body)
//	DELETE /api/repos         — delete a repo (name in body)
func (s *Server) handleRepos(w http.ResponseWriter, r *http.Request) {
	type repoRequest struct {
		// Used by all methods
		Name string `json:"name"`
		// Used by POST and PUT
		URL          string `json:"url"`
		Branch       string `json:"branch"`
		Token        string `json:"token"`      // "" = keep existing (PUT), non-empty = set new
		ClearToken   bool   `json:"clearToken"` // true = remove any token (PUT only)
		ClustersPath string `json:"clustersPath"`
		MCPath       string `json:"mcPath"`
	}

	w.Header().Set("Content-Type", "application/json")

	switch r.Method {
	case http.MethodPost:
		var req repoRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "invalid request body", http.StatusBadRequest)
			return
		}
		req.Name = strings.TrimSpace(req.Name)
		req.URL = strings.TrimSpace(req.URL)
		if req.Name == "" {
			http.Error(w, "name is required", http.StatusBadRequest)
			return
		}
		if req.URL == "" {
			http.Error(w, "url is required", http.StatusBadRequest)
			return
		}
		if req.Branch == "" {
			req.Branch = "main"
		}
		if req.ClustersPath == "" {
			req.ClustersPath = "clusters"
		}
		if req.MCPath == "" {
			req.MCPath = "machine-classes"
		}
		rc := config.RepoConfig{
			Name:         req.Name,
			URL:          req.URL,
			Branch:       req.Branch,
			Token:        req.Token,
			ClustersPath: req.ClustersPath,
			MCPath:       req.MCPath,
		}
		if err := s.appState.AddRepoConfig(rc); err != nil {
			w.WriteHeader(http.StatusConflict)
			json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
			return
		}
		if err := s.appState.SaveRepoConfigs(); err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]string{"error": "failed to persist: " + err.Error()})
			return
		}
		s.signalRepoChange()
		s.appState.AppendAudit(state.AuditEntry{User: s.sessionIdentity(r), Action: "repo-add", Resource: rc.Name, Kind: "repo"})
		json.NewEncoder(w).Encode(map[string]string{"status": "created", "name": rc.Name})

	case http.MethodPut:
		var req repoRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "invalid request body", http.StatusBadRequest)
			return
		}
		req.Name = strings.TrimSpace(req.Name)
		req.URL = strings.TrimSpace(req.URL)
		if req.Name == "" {
			http.Error(w, "name is required", http.StatusBadRequest)
			return
		}
		if req.URL == "" {
			http.Error(w, "url is required", http.StatusBadRequest)
			return
		}
		if req.Branch == "" {
			req.Branch = "main"
		}
		if req.ClustersPath == "" {
			req.ClustersPath = "clusters"
		}
		if req.MCPath == "" {
			req.MCPath = "machine-classes"
		}
		token := req.Token
		if req.ClearToken {
			token = "" // will be stored as empty — UpdateRepoConfig checks "" only when not clearing
		}
		rc := config.RepoConfig{
			Name:         req.Name,
			URL:          req.URL,
			Branch:       req.Branch,
			Token:        token,
			ClustersPath: req.ClustersPath,
			MCPath:       req.MCPath,
		}
		// Pass ClearToken intent via a sentinel: UpdateRepoConfig preserves empty
		// token unless we pass a non-empty one or explicitly clear.
		if req.ClearToken {
			// Use a sentinel so UpdateRepoConfig knows to clear
			rc.Token = "\x00clear\x00"
		}
		if err := s.appState.UpdateRepoConfig(req.Name, rc); err != nil {
			w.WriteHeader(http.StatusNotFound)
			json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
			return
		}
		if err := s.appState.SaveRepoConfigs(); err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]string{"error": "failed to persist: " + err.Error()})
			return
		}
		s.signalRepoChange()
		s.appState.AppendAudit(state.AuditEntry{User: s.sessionIdentity(r), Action: "repo-update", Resource: rc.Name, Kind: "repo"})
		json.NewEncoder(w).Encode(map[string]string{"status": "updated", "name": rc.Name})

	case http.MethodDelete:
		var req struct {
			Name string `json:"name"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "invalid request body", http.StatusBadRequest)
			return
		}
		req.Name = strings.TrimSpace(req.Name)
		if req.Name == "" {
			http.Error(w, "name is required", http.StatusBadRequest)
			return
		}
		// Capture the repo config before deletion so we can force-delete its
		// clusters and machine classes on the next reconcile cycle.
		for _, rc := range s.appState.GetRepoConfigs() {
			if rc.Name == req.Name {
				s.appState.AddPendingRepoDelete(rc)
				break
			}
		}
		if err := s.appState.DeleteRepoConfig(req.Name); err != nil {
			w.WriteHeader(http.StatusNotFound)
			json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
			return
		}
		if err := s.appState.SaveRepoConfigs(); err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]string{"error": "failed to persist: " + err.Error()})
			return
		}
		s.signalRepoChange()
		s.appState.AppendAudit(state.AuditEntry{User: s.sessionIdentity(r), Action: "repo-delete", Resource: req.Name, Kind: "repo"})
		json.NewEncoder(w).Encode(map[string]string{"status": "deleted", "name": req.Name})

	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

// signalRepoChange sends a non-blocking signal to rebuild the git client when
// repos are added, updated, or deleted from the UI.
func (s *Server) signalRepoChange() {
	select {
	case s.triggerRepoChange <- struct{}{}:
	default:
	}
}

// handleTestRepo tests connectivity for a given repo URL/branch/token without
// persisting anything. Used by the "Test Connection" button in the UI.
func (s *Server) handleTestRepo(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		URL    string `json:"url"`
		Branch string `json:"branch"`
		Token  string `json:"token"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	req.URL = strings.TrimSpace(req.URL)
	if req.URL == "" {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "url is required"})
		return
	}
	if req.Branch == "" {
		req.Branch = "main"
	}

	if err := git.TestConnection(req.URL, req.Branch, req.Token); err != nil {
		w.WriteHeader(http.StatusUnprocessableEntity)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}

	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

// handleOmniInstance handles GET and POST for the Omni instance configuration.
// GET returns the current endpoint and whether a key is stored (never the key itself).
// POST saves new credentials after verifying connectivity. Returns 403 when ENV-locked.
func (s *Server) handleOmniInstance(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	if r.Method == http.MethodGet {
		snap := s.appState.Snapshot()
		json.NewEncoder(w).Encode(map[string]interface{}{
			"endpoint":   snap.OmniEndpoint,
			"hasKey":     snap.OmniHasStoredKey,
			"envLocked":  snap.OmniEnvLocked,
			"configured": snap.OmniConfigured,
		})
		return
	}

	if r.Method == http.MethodDelete {
		if s.appState.IsEnvLocked() {
			w.WriteHeader(http.StatusForbidden)
			json.NewEncoder(w).Encode(map[string]string{"error": "omni instance is configured via environment variables"})
			return
		}
		if err := os.Remove(s.omniInstanceFile); err != nil && !os.IsNotExist(err) {
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]string{"error": "failed to delete config: " + err.Error()})
			return
		}
		s.appState.SetOmniEndpoint("")
		s.appState.SetHasStoredKey(false)
		s.appState.SetOmniConfigured(false)
		s.appState.SetClusters(nil)
		s.appState.SetMachineClasses(nil)
		omni.ClearCache()
		s.appState.Save()
		s.appState.AppendAudit(state.AuditEntry{User: s.sessionIdentity(r), Action: "omni-delete", Kind: "omni"})
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
		return
	}

	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if s.appState.IsEnvLocked() {
		w.WriteHeader(http.StatusForbidden)
		json.NewEncoder(w).Encode(map[string]string{"error": "omni instance is configured via environment variables"})
		return
	}

	var req struct {
		Endpoint          string `json:"endpoint"`
		ServiceAccountKey string `json:"serviceAccountKey"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}
	req.Endpoint = strings.TrimSpace(req.Endpoint)
	req.ServiceAccountKey = strings.TrimSpace(req.ServiceAccountKey)
	if req.Endpoint == "" || req.ServiceAccountKey == "" {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "endpoint and serviceAccountKey are required"})
		return
	}

	if err := omni.TestConnectivity(req.Endpoint, req.ServiceAccountKey); err != nil {
		w.WriteHeader(http.StatusUnprocessableEntity)
		json.NewEncoder(w).Encode(map[string]string{"error": "connectivity check failed: " + err.Error()})
		return
	}

	if err := omni.Init(req.Endpoint, req.ServiceAccountKey); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": "failed to initialise client: " + err.Error()})
		return
	}

	if err := omniinstance.Save(s.omniInstanceFile, omniinstance.InstanceConfig{Endpoint: req.Endpoint, ServiceAccountKey: req.ServiceAccountKey}); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": "failed to save config: " + err.Error()})
		return
	}

	s.appState.SetClusters(nil)
	s.appState.SetMachineClasses(nil)
	s.appState.SetOmniEndpoint(req.Endpoint)
	s.appState.SetHasStoredKey(true)
	s.appState.SetOmniConfigured(true)
	omni.ClearCache()
	s.appState.Save()
	s.appState.AppendAudit(state.AuditEntry{User: s.sessionIdentity(r), Action: "omni-update", Kind: "omni"})

	// Signal main() to start the reconciler for the first time (non-blocking).
	select {
	case s.triggerOmniConfigured <- struct{}{}:
	default:
	}
	// Trigger a hard reconcile in case the reconciler is already running
	// (e.g. instance was removed and re-added).
	select {
	case s.triggerHard <- struct{}{}:
	default:
	}

	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

// handleTestOmniInstance tests connectivity for a given endpoint+key without
// persisting anything or touching the global client state.
func (s *Server) handleTestOmniInstance(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if s.appState.IsEnvLocked() {
		w.WriteHeader(http.StatusForbidden)
		json.NewEncoder(w).Encode(map[string]string{"error": "omni instance is configured via environment variables"})
		return
	}

	var req struct {
		Endpoint          string `json:"endpoint"`
		ServiceAccountKey string `json:"serviceAccountKey"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if req.Endpoint == "" || req.ServiceAccountKey == "" {
		http.Error(w, "endpoint and serviceAccountKey are required", http.StatusBadRequest)
		return
	}

	if err := omni.TestConnectivity(req.Endpoint, req.ServiceAccountKey); err != nil {
		w.WriteHeader(http.StatusUnprocessableEntity)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}

	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

// handleRefreshOmniConnection re-checks connectivity and refreshes the Omni version in state.
func (s *Server) handleRefreshOmniConnection(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if !s.appState.IsOmniConfigured() {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "omni instance not configured"})
		return
	}
	if err := omni.CheckConnectivity(); err != nil {
		s.appState.SetOmniHealth("failed", err.Error())
		w.WriteHeader(http.StatusServiceUnavailable)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}
	s.appState.SetOmniHealth("healthy", "")
	omniVersion := omni.GetOmniVersion()
	s.appState.SetVersions(omniVersion)
	json.NewEncoder(w).Encode(map[string]string{"status": "ok", "version": omniVersion})
}

// handleLogFiles returns a JSON list of available daily log files, newest first.
func (s *Server) handleLogFiles(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	entries, err := os.ReadDir(s.logDir)
	if err != nil {
		if os.IsNotExist(err) {
			json.NewEncoder(w).Encode([]interface{}{})
			return
		}
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}
	type fileInfo struct {
		Date     string `json:"date"`
		Filename string `json:"filename"`
		Size     int64  `json:"size"`
	}
	var files []fileInfo
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if !strings.HasPrefix(name, "omni-cd-") || !strings.HasSuffix(name, ".jsonlog") {
			continue
		}
		dateStr := strings.TrimSuffix(strings.TrimPrefix(name, "omni-cd-"), ".jsonlog")
		if _, err := time.Parse("2006-01-02", dateStr); err != nil {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		files = append(files, fileInfo{Date: dateStr, Filename: name, Size: info.Size()})
	}
	// Reverse so newest is first.
	for i, j := 0, len(files)-1; i < j; i, j = i+1, j-1 {
		files[i], files[j] = files[j], files[i]
	}
	if files == nil {
		files = []fileInfo{}
	}
	json.NewEncoder(w).Encode(files)
}

// handleLogDownload streams a daily log file as a download.
// Query param: date=YYYY-MM-DD
func (s *Server) handleLogDownload(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	date := r.URL.Query().Get("date")
	if date == "" {
		date = time.Now().UTC().Format("2006-01-02")
	}
	if _, err := time.Parse("2006-01-02", date); err != nil {
		http.Error(w, "invalid date format", http.StatusBadRequest)
		return
	}
	filename := "omni-cd-" + date + ".jsonlog"
	path := filepath.Join(s.logDir, filename)
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, filename))
	w.Header().Set("Content-Type", "application/x-ndjson")
	http.ServeFile(w, r, path)
}

// handleAudit returns the in-memory audit log, newest first.
func (s *Server) handleAudit(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	entries := s.appState.GetAuditLog()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(entries)
}

// handleAuditFiles returns a JSON list of available daily audit files, newest first.
func (s *Server) handleAuditFiles(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	dirEntries, err := os.ReadDir(s.auditDir)
	if err != nil {
		if os.IsNotExist(err) {
			json.NewEncoder(w).Encode([]interface{}{})
			return
		}
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}
	type fileInfo struct {
		Date string `json:"date"`
		Size int64  `json:"size"`
	}
	var files []fileInfo
	for _, e := range dirEntries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if !strings.HasPrefix(name, "audit-") || !strings.HasSuffix(name, ".jsonlog") {
			continue
		}
		dateStr := strings.TrimSuffix(strings.TrimPrefix(name, "audit-"), ".jsonlog")
		if _, err := time.Parse("2006-01-02", dateStr); err != nil {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		files = append(files, fileInfo{Date: dateStr, Size: info.Size()})
	}
	for i, j := 0, len(files)-1; i < j; i, j = i+1, j-1 {
		files[i], files[j] = files[j], files[i]
	}
	if files == nil {
		files = []fileInfo{}
	}
	json.NewEncoder(w).Encode(files)
}

// handleAuditDownload streams a daily audit file as a download.
// Query param: date=YYYY-MM-DD
func (s *Server) handleAuditDownload(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	date := r.URL.Query().Get("date")
	if date == "" {
		date = time.Now().UTC().Format("2006-01-02")
	}
	if _, err := time.Parse("2006-01-02", date); err != nil {
		http.Error(w, "invalid date format", http.StatusBadRequest)
		return
	}
	filename := "audit-" + date + ".jsonlog"
	path := filepath.Join(s.auditDir, filename)
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, filename))
	w.Header().Set("Content-Type", "application/x-ndjson")
	http.ServeFile(w, r, path)
}

// handleMe returns the current user's identity and auth configuration.
func (s *Server) handleMe(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	type meResponse struct {
		Username     string `json:"username"`
		Role         string `json:"role"`
		AuthDisabled bool   `json:"authDisabled"`
		OIDCEnabled  bool   `json:"oidcEnabled"`
	}
	username := s.sessionUsername(r)
	role := s.sessionRole(r)
	if role == "" {
		role = "admin"
	}
	resp := meResponse{
		Username:     username,
		Role:         role,
		AuthDisabled: s.authDisabled,
		OIDCEnabled:  s.oidcEnabled(),
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}
