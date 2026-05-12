package web

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"

	appstate "omni-cd/internal/state"
)

const oidcUsersPath = "/data/config/oidc-users.json"

// oidcUser records an OIDC-authenticated user and their assigned role.
type oidcUser struct {
	Email       string    `json:"email"`
	DisplayName string    `json:"displayName"`
	Role        string    `json:"role"` // admin | viewer | none
	FirstSeen   time.Time `json:"firstSeen"`
	LastSeen    time.Time `json:"lastSeen"`
}

// oidcUserStore is a file-backed list of OIDC users with assignable roles.
type oidcUserStore struct {
	mu    sync.Mutex
	path  string
	users []oidcUser
}

func loadOIDCUserStore(path string) *oidcUserStore {
	s := &oidcUserStore{path: path}
	data, err := os.ReadFile(path)
	if err == nil {
		if err := json.Unmarshal(data, &s.users); err != nil {
			slog.Warn("Could not parse OIDC users file, starting with empty list", "error", err, "component", "OIDC")
		}
	}
	return s
}

// upsert adds the user if new (using the provided role) or updates their
// LastSeen / DisplayName if they already exist. Returns the effective role.
func (s *oidcUserStore) upsert(email, displayName, role string) string {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now()
	for i, u := range s.users {
		if u.Email == email {
			s.users[i].LastSeen = now
			if displayName != "" {
				s.users[i].DisplayName = displayName
			}
			s.save()
			return s.users[i].Role
		}
	}
	s.users = append(s.users, oidcUser{
		Email:       email,
		DisplayName: displayName,
		Role:        role,
		FirstSeen:   now,
		LastSeen:    now,
	})
	s.save()
	return role
}

func (s *oidcUserStore) isEmpty() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.users) == 0
}

func (s *oidcUserStore) list() []oidcUser {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]oidcUser, len(s.users))
	copy(out, s.users)
	return out
}

// setRole updates the stored role for an existing user. Returns false if the
// user was not found.
func (s *oidcUserStore) setRole(email, role string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i, u := range s.users {
		if u.Email == email {
			s.users[i].Role = role
			s.save()
			return true
		}
	}
	return false
}

// delete removes a user by email. Returns false if the user was not found.
func (s *oidcUserStore) delete(email string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i, u := range s.users {
		if u.Email == email {
			s.users = append(s.users[:i], s.users[i+1:]...)
			s.save()
			return true
		}
	}
	return false
}

func (s *oidcUserStore) save() {
	if err := os.MkdirAll(filepath.Dir(s.path), 0700); err != nil {
		slog.Warn("Failed to create directory for OIDC users", "error", err, "component", "OIDC")
		return
	}
	data, err := json.MarshalIndent(s.users, "", "  ")
	if err != nil {
		slog.Warn("Could not marshal OIDC users", "error", err, "component", "OIDC")
		return
	}
	if err := os.WriteFile(s.path, data, 0600); err != nil {
		slog.Warn("Could not write OIDC users file", "error", err, "component", "OIDC")
	}
}

// handleOIDCUsers serves GET /api/users/oidc (list),
// PATCH /api/users/oidc (update role), and DELETE /api/users/oidc (remove user).
func (s *Server) handleOIDCUsers(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	if r.Method == http.MethodPatch {
		var body struct {
			Email string `json:"email"`
			Role  string `json:"role"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Email == "" {
			http.Error(w, "Invalid request body", http.StatusBadRequest)
			return
		}
		if body.Role != "admin" && body.Role != "viewer" && body.Role != "none" {
			http.Error(w, "Invalid role; must be admin, viewer, or none", http.StatusBadRequest)
			return
		}
		if s.oidcUsers == nil || !s.oidcUsers.setRole(body.Email, body.Role) {
			http.Error(w, "User not found", http.StatusNotFound)
			return
		}
		s.appState.AppendAudit(appstate.AuditEntry{User: s.sessionIdentity(r), Action: "oidc-role-update", Resource: body.Email, Kind: "user"})
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
		return
	}

	if r.Method == http.MethodDelete {
		var body struct {
			Email string `json:"email"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Email == "" {
			http.Error(w, "Invalid request body", http.StatusBadRequest)
			return
		}
		if s.oidcUsers == nil || !s.oidcUsers.delete(body.Email) {
			http.Error(w, "User not found", http.StatusNotFound)
			return
		}
		// Invalidate any live sessions belonging to the deleted user.
		s.sessions.Range(func(key, value any) bool {
			info := value.(sessionInfo)
			if info.AuthMethod == "oidc" && info.Username == body.Email {
				s.sessions.Delete(key)
			}
			return true
		})
		s.saveSessions()
		slog.Info("SSO user deleted", "email", body.Email, "component", "OIDC")
		s.appState.AppendAudit(appstate.AuditEntry{User: s.sessionIdentity(r), Action: "oidc-user-delete", Resource: body.Email, Kind: "user"})
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
		return
	}

	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var users []oidcUser
	if s.oidcUsers != nil {
		users = s.oidcUsers.list()
	}
	if users == nil {
		users = []oidcUser{}
	}
	writeJSON(w, http.StatusOK, users)
}
