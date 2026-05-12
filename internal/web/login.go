package web

import (
	"fmt"
	"io/fs"
	"log/slog"
	"net/http"
	"time"

	"omni-cd/internal/state"
)

// handleLoginConfig returns JSON describing which auth methods are available.
func (s *Server) handleLoginConfig(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]bool{
		"oidcEnabled": s.oidcEnabled(),
		"localAuth":   s.authStore != nil,
	})
}

// handleLogin serves GET /login (login page) and POST /login (credential check).
func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		// Auth disabled or already authenticated → go home.
		if s.authDisabled {
			http.Redirect(w, r, "/", http.StatusFound)
			return
		}
		if cookie, err := r.Cookie(sessionCookieName); err == nil && s.validSession(cookie.Value) {
			http.Redirect(w, r, "/", http.StatusFound)
			return
		}
		serveIndexHTML(w)

	case http.MethodPost:
		r.Body = http.MaxBytesReader(w, r.Body, 1<<20) // 1 MB limit
		ip := clientIP(r)
		bucket := s.loginBucketFor(ip)

		bucket.mu.Lock()
		if time.Now().Before(bucket.lockedUntil) {
			bucket.mu.Unlock()
			slog.Warn("Login blocked — too many failed attempts", "ip", ip, "component", "Auth")
			writeJSON(w, http.StatusTooManyRequests, map[string]string{"error": "Too many failed attempts — try again in 15 minutes"})
			return
		}
		bucket.mu.Unlock()

		username := r.FormValue("username")
		password := r.FormValue("password")

		if s.authStore != nil && s.authStore.Validate(username, password) {
			// Reset failure counter on success.
			bucket.mu.Lock()
			bucket.failures = 0
			bucket.lockedUntil = time.Time{}
			bucket.mu.Unlock()

			token, err := generateToken()
			if err != nil {
				http.Error(w, "Internal server error", http.StatusInternalServerError)
				return
			}
			displayName := s.authStore.GetDisplayName(username)
			s.sessions.Store(token, sessionInfo{LoginTime: time.Now(), Username: username, DisplayName: displayName, AuthMethod: "local"})
			s.saveSessions()
			http.SetCookie(w, &http.Cookie{
				Name:     sessionCookieName,
				Value:    token,
				Path:     "/",
				HttpOnly: true,
				Secure:   isSecure(r),
				SameSite: http.SameSiteLaxMode,
				MaxAge:   int(sessionDuration.Seconds()),
			})
			slog.Info("User logged in", "username", username, "ip", ip, "component", "Auth")
			s.appState.AppendAudit(state.AuditEntry{User: username, Action: "login", Kind: "session"})
			writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
			return
		}

		// Bad credentials — increment failure counter.
		bucket.mu.Lock()
		bucket.failures++
		if bucket.failures >= maxLoginFailures {
			bucket.lockedUntil = time.Now().Add(loginLockDuration)
			bucket.failures = 0
		}
		bucket.mu.Unlock()

		slog.Warn("Failed login attempt", "username", username, "ip", ip, "component", "Auth")
		s.appState.AppendAudit(state.AuditEntry{User: username, Action: "login-failed", Kind: "session"})
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "Invalid username or password"})

	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// handleLogout clears the session cookie and redirects to /login (or / when auth is disabled).
func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
	if cookie, err := r.Cookie(sessionCookieName); err == nil {
		if info, ok := s.sessions.Load(cookie.Value); ok {
			si := info.(sessionInfo)
			slog.Info("User logged out", "username", si.Username, "remote", r.RemoteAddr, "component", "Auth")
			s.appState.AppendAudit(state.AuditEntry{User: si.Username, Action: "logout", Kind: "session"})
		}
		s.sessions.Delete(cookie.Value)
		s.saveSessions()
	}
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		MaxAge:   -1,
	})
	if s.authDisabled {
		http.Redirect(w, r, "/", http.StatusFound)
		return
	}
	http.Redirect(w, r, "/login", http.StatusFound)
}

// serveIndexHTML reads dist/index.html from the embedded FS and writes it to w.
func serveIndexHTML(w http.ResponseWriter) {
	data, err := distFS.ReadFile("dist/index.html")
	if err != nil {
		http.Error(w, "UI not built. Run: cd frontend && npm run build", http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	fmt.Fprint(w, string(data))
}

// handleStaticAsset serves files under /assets/ from the embedded dist FS without auth.
func (s *Server) handleStaticAsset(w http.ResponseWriter, r *http.Request) {
	sub, err := fs.Sub(distFS, "dist")
	if err != nil {
		http.NotFound(w, r)
		return
	}
	http.FileServerFS(sub).ServeHTTP(w, r)
}
