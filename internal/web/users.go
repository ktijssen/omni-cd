package web

import (
	"encoding/json"
	"net/http"
	"strings"
)

// handleUsers serves:
//   GET  /api/users               → list of users (no hashes)
//   POST /api/users/change-password → change the logged-in user's password
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
// Body: {"newEmail": "...", "newDisplayName": "..."}
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
		NewEmail       string `json:"newEmail"`
		NewDisplayName string `json:"newDisplayName"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	body.NewEmail = strings.TrimSpace(body.NewEmail)
	body.NewDisplayName = strings.TrimSpace(body.NewDisplayName)

	currentEmail := s.sessionEmail(r)
	if currentEmail == "" {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	newEmail, err := s.authStore.UpdateProfile(currentEmail, body.NewEmail, body.NewDisplayName)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}

	// Update the session so subsequent calls use the new email/display name.
	if cookie, cerr := r.Cookie(sessionCookieName); cerr == nil {
		if v, ok := s.sessions.Load(cookie.Value); ok {
			old := v.(sessionInfo)
			displayName := body.NewDisplayName
			if displayName == "" {
				displayName = old.DisplayName
			}
			s.sessions.Store(cookie.Value, sessionInfo{
				LoginTime:   old.LoginTime,
				Email:       newEmail,
				DisplayName: displayName,
			})
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "ok", "email": newEmail})
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

	email := s.sessionEmail(r)
	if email == "" {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	if err := s.authStore.ChangePassword(email, body.CurrentPassword, body.NewPassword); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}
