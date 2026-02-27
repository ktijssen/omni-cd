package web

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"
)

const (
	sessionCookieName = "omnicd_session"
	sessionDuration   = 24 * time.Hour
)

// generateToken produces a cryptographically random 32-byte hex session token.
func generateToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// requireAuth wraps a handler and redirects to /login if no valid session is present.
// API calls (paths starting with /api/ or /ws) get a 401 instead of a redirect.
func (s *Server) requireAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie(sessionCookieName)
		if err != nil || !s.validSession(cookie.Value) {
			if strings.HasPrefix(r.URL.Path, "/api/") || r.URL.Path == "/ws" {
				http.Error(w, "Unauthorized", http.StatusUnauthorized)
				return
			}
			http.Redirect(w, r, "/login", http.StatusFound)
			return
		}
		next(w, r)
	}
}

// validSession returns true if the token is an active session.
func (s *Server) validSession(token string) bool {
	if token == "" {
		return false
	}
	_, ok := s.sessions.Load(token)
	return ok
}

// handleLogin serves GET /login (login page) and POST /login (credential check).
func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		// Already authenticated → go home.
		if cookie, err := r.Cookie(sessionCookieName); err == nil && s.validSession(cookie.Value) {
			http.Redirect(w, r, "/", http.StatusFound)
			return
		}
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprint(w, loginHTML)

	case http.MethodPost:
		username := r.FormValue("username")
		password := r.FormValue("password")

		if username == s.adminUsername && password == s.adminPassword {
			token, err := generateToken()
			if err != nil {
				http.Error(w, "Internal server error", http.StatusInternalServerError)
				return
			}
			s.sessions.Store(token, time.Now())
			http.SetCookie(w, &http.Cookie{
				Name:     sessionCookieName,
				Value:    token,
				Path:     "/",
				HttpOnly: true,
				SameSite: http.SameSiteLaxMode,
				MaxAge:   int(sessionDuration.Seconds()),
			})
			slog.Info("User logged in", "username", username, "remote", r.RemoteAddr, "component", "Auth")
			http.Redirect(w, r, "/", http.StatusFound)
			return
		}

		// Bad credentials — redisplay login page with an error message.
		slog.Warn("Failed login attempt", "username", username, "remote", r.RemoteAddr, "component", "Auth")
		w.Header().Set("Content-Type", "text/html")
		w.WriteHeader(http.StatusUnauthorized)
		fmt.Fprint(w, strings.ReplaceAll(loginHTML, "<!--ERROR-->",
			`<div class="login-error">Invalid username or password</div>`))

	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// handleLogout clears the session cookie and redirects to /login.
func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
	if cookie, err := r.Cookie(sessionCookieName); err == nil {
		s.sessions.Delete(cookie.Value)
		slog.Info("User logged out", "username", s.adminUsername, "remote", r.RemoteAddr, "component", "Auth")
	}
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		MaxAge:   -1,
	})
	http.Redirect(w, r, "/login", http.StatusFound)
}
