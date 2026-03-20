package web

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"omni-cd/internal/auth"
)

// handleSetupStatus returns JSON describing whether first-time setup is still needed.
func (s *Server) handleSetupStatus(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	needed := s.authStore != nil && s.authStore.IsEmpty() && !s.oidcEnabled()
	json.NewEncoder(w).Encode(map[string]bool{"needed": needed})
}

// handleSetup serves GET /setup (setup page) and POST /setup (create admin account).
// Once any user exists the POST returns 404 to prevent re-initialisation.
func (s *Server) handleSetup(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		if s.authDisabled {
			http.Redirect(w, r, "/", http.StatusFound)
			return
		}
		serveIndexHTML(w)

	case http.MethodPost:
		if s.authStore == nil || !s.authStore.IsEmpty() {
			http.NotFound(w, r)
			return
		}
		r.Body = http.MaxBytesReader(w, r.Body, 1<<20) // 1 MB limit
		var body struct {
			Password string `json:"password"`
			Confirm  string `json:"confirm"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{"error": "Invalid request"})
			return
		}
		if err := auth.ValidatePasswordStrength(body.Password); err != nil {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
			return
		}
		if body.Password != body.Confirm {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{"error": "Passwords do not match"})
			return
		}
		if err := s.authStore.SetUser("admin", "Admin", body.Password); err != nil {
			http.Error(w, "Failed to save credentials", http.StatusInternalServerError)
			return
		}
		slog.Info("Admin account created via setup", "component", "Auth")
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]bool{"ok": true})

	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}
