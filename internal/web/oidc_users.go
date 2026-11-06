package web

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"omni-cd/internal/auth"
	appstate "omni-cd/internal/state"
)

// oidcUser is the web package's name for a persisted OIDC user.
type oidcUser = auth.OIDCUser

// oidcUserStore wraps an OIDCUserRepository with the role bookkeeping used by
// the login flow. The mutex serialises read-modify-write updates.
type oidcUserStore struct {
	mu   sync.Mutex
	repo auth.OIDCUserRepository
}

func newOIDCUserStore(repo auth.OIDCUserRepository) *oidcUserStore {
	return &oidcUserStore{repo: repo}
}

// upsert adds the user if new (using the provided role) or updates their
// LastSeen / DisplayName if they already exist. Returns the effective role.
func (s *oidcUserStore) upsert(email, displayName, role string) string {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now()
	u, err := s.repo.GetOIDCUser(email)
	switch {
	case err == nil:
		u.LastSeen = now
		if displayName != "" {
			u.DisplayName = displayName
		}
	case errors.Is(err, auth.ErrNotFound):
		u = oidcUser{Email: email, DisplayName: displayName, Role: role, FirstSeen: now, LastSeen: now}
	default:
		// Fail closed: without the stored role we cannot tell whether an
		// admin demoted this user, so grant no access.
		slog.Error("Could not read OIDC user", "email", email, "error", err, "component", "OIDC")
		return "none"
	}
	if err := s.repo.PutOIDCUser(u); err != nil {
		slog.Warn("Could not save OIDC user", "email", email, "error", err, "component", "OIDC")
	}
	return u.Role
}

// isEmpty reports whether no OIDC user has logged in yet. A read error is
// treated as non-empty so a broken store never auto-promotes someone to admin.
func (s *oidcUserStore) isEmpty() bool {
	users, err := s.repo.ListOIDCUsers()
	if err != nil {
		slog.Error("Could not list OIDC users", "error", err, "component", "OIDC")
		return false
	}
	return len(users) == 0
}

func (s *oidcUserStore) list() []oidcUser {
	users, err := s.repo.ListOIDCUsers()
	if err != nil {
		slog.Error("Could not list OIDC users", "error", err, "component", "OIDC")
		return nil
	}
	return users
}

// setRole updates the stored role for an existing user. Returns false if the
// user was not found or could not be saved.
func (s *oidcUserStore) setRole(email, role string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	u, err := s.repo.GetOIDCUser(email)
	if err != nil {
		if !errors.Is(err, auth.ErrNotFound) {
			slog.Error("Could not read OIDC user", "email", email, "error", err, "component", "OIDC")
		}
		return false
	}
	u.Role = role
	if err := s.repo.PutOIDCUser(u); err != nil {
		slog.Error("Could not save OIDC user", "email", email, "error", err, "component", "OIDC")
		return false
	}
	return true
}

// delete removes a user by email. Returns false if the user was not found or
// could not be deleted.
func (s *oidcUserStore) delete(email string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.repo.DeleteOIDCUser(email); err != nil {
		if !errors.Is(err, auth.ErrNotFound) {
			slog.Error("Could not delete OIDC user", "email", email, "error", err, "component", "OIDC")
		}
		return false
	}
	return true
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
