package web

import (
	"encoding/json"
	"net/http"
	"strings"
)

// handleUsers serves GET /api/users → list of users (no hashes).
func (s *Server) handleUsers(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if s.authDisabled {
		http.NotFound(w, r)
		return
	}
	if s.authStore == nil {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode([]struct{}{})
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(s.authStore.List())
}

// handleUpdateProfile handles POST /api/users/update-profile.
// Body: {"newDisplayName": "..."}
func (s *Server) handleUpdateProfile(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if s.authDisabled {
		http.NotFound(w, r)
		return
	}
	if s.authStore == nil {
		http.Error(w, "Auth not configured", http.StatusInternalServerError)
		return
	}

	var body struct {
		NewDisplayName string `json:"newDisplayName"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}
	body.NewDisplayName = strings.TrimSpace(body.NewDisplayName)

	username := s.sessionIdentity(r)
	if username == "" {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	if err := s.authStore.UpdateProfile(username, body.NewDisplayName); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}

	// Update the session display name.
	if cookie, cerr := r.Cookie(sessionCookieName); cerr == nil {
		if v, ok := s.sessions.Load(cookie.Value); ok {
			old := v.(sessionInfo)
			displayName := body.NewDisplayName
			if displayName == "" {
				displayName = old.DisplayName
			}
			s.sessions.Store(cookie.Value, sessionInfo{
				LoginTime:   old.LoginTime,
				Username:    old.Username,
				DisplayName: displayName,
				AuthMethod:  old.AuthMethod,
				Role:        old.Role,
			})
			s.saveSessions()
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

// handleChangePassword handles POST /api/users/change-password.
// Body: {"currentPassword": "...", "newPassword": "..."}
func (s *Server) handleChangePassword(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if s.authDisabled {
		http.NotFound(w, r)
		return
	}
	if s.authStore == nil {
		http.Error(w, "Auth not configured", http.StatusInternalServerError)
		return
	}

	var body struct {
		CurrentPassword string `json:"currentPassword"`
		NewPassword     string `json:"newPassword"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	username := s.sessionIdentity(r)
	if username == "" {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	if err := s.authStore.ChangePassword(username, body.CurrentPassword, body.NewPassword); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}
