package web

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
)

const (
	sessionCookieName  = "omnicd_session"
	sessionDuration    = 24 * time.Hour
	maxLoginFailures   = 5
	loginLockDuration  = 15 * time.Minute
)

// loginBucket tracks failed login attempts for one IP address.
type loginBucket struct {
	mu          sync.Mutex
	failures    int
	lockedUntil time.Time
}

// isSecure returns true when the request arrived over TLS or via an
// HTTPS-terminated reverse proxy (X-Forwarded-Proto: https).
func isSecure(r *http.Request) bool {
	return r.TLS != nil || r.Header.Get("X-Forwarded-Proto") == "https"
}

// clientIP extracts the real client IP, preferring X-Forwarded-For when present.
func clientIP(r *http.Request) string {
	if fwd := r.Header.Get("X-Forwarded-For"); fwd != "" {
		if i := strings.IndexByte(fwd, ','); i >= 0 {
			return strings.TrimSpace(fwd[:i])
		}
		return strings.TrimSpace(fwd)
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

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
// Auth is skipped entirely when authDisabled is true.
// When no users are configured yet, all requests are redirected to /setup.
func (s *Server) requireAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if s.authDisabled {
			next(w, r)
			return
		}
		// No users configured yet — force first-time setup.
		if s.authStore != nil && s.authStore.IsEmpty() {
			if strings.HasPrefix(r.URL.Path, "/api/") || r.URL.Path == "/ws" {
				http.Error(w, "Unauthorized", http.StatusUnauthorized)
				return
			}
			http.Redirect(w, r, "/setup", http.StatusFound)
			return
		}
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

// sessionInfo holds per-session metadata.
type sessionInfo struct {
	LoginTime   time.Time
	Email       string // used for API calls (e.g. change-password)
	DisplayName string // shown in the sidebar
}

// validSession returns true if the token is an active, unexpired session.
func (s *Server) validSession(token string) bool {
	if token == "" {
		return false
	}
	v, ok := s.sessions.Load(token)
	if !ok {
		return false
	}
	if time.Since(v.(sessionInfo).LoginTime) >= sessionDuration {
		s.sessions.Delete(token)
		return false
	}
	return true
}

// sessionUsername returns the display name for the session cookie in r, or empty string.
func (s *Server) sessionUsername(r *http.Request) string {
	cookie, err := r.Cookie(sessionCookieName)
	if err != nil {
		return ""
	}
	v, ok := s.sessions.Load(cookie.Value)
	if !ok {
		return ""
	}
	return v.(sessionInfo).DisplayName
}

// sessionEmail returns the email for the session cookie in r, or empty string.
func (s *Server) sessionEmail(r *http.Request) string {
	cookie, err := r.Cookie(sessionCookieName)
	if err != nil {
		return ""
	}
	v, ok := s.sessions.Load(cookie.Value)
	if !ok {
		return ""
	}
	return v.(sessionInfo).Email
}

// loginBucketFor returns the rate-limit bucket for the given IP, creating it if needed.
func (s *Server) loginBucketFor(ip string) *loginBucket {
	v, _ := s.loginBuckets.LoadOrStore(ip, &loginBucket{})
	return v.(*loginBucket)
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
		ip := clientIP(r)
		bucket := s.loginBucketFor(ip)

		bucket.mu.Lock()
		if time.Now().Before(bucket.lockedUntil) {
			bucket.mu.Unlock()
			slog.Warn("Login blocked — too many failed attempts", "ip", ip, "component", "Auth")
			w.Header().Set("Content-Type", "text/html")
			w.WriteHeader(http.StatusTooManyRequests)
			fmt.Fprint(w, strings.ReplaceAll(loginHTML, "<!--ERROR-->",
				`<div class="login-error">Too many failed attempts — try again in 15 minutes</div>`))
			return
		}
		bucket.mu.Unlock()

		email := r.FormValue("email")
		password := r.FormValue("password")

		if s.authStore != nil && s.authStore.Validate(email, password) {
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
			displayName := s.authStore.GetDisplayName(email)
			s.sessions.Store(token, sessionInfo{LoginTime: time.Now(), Email: email, DisplayName: displayName})
			http.SetCookie(w, &http.Cookie{
				Name:     sessionCookieName,
				Value:    token,
				Path:     "/",
				HttpOnly: true,
				Secure:   isSecure(r),
				SameSite: http.SameSiteLaxMode,
				MaxAge:   int(sessionDuration.Seconds()),
			})
			slog.Info("User logged in", "email", email, "ip", ip, "component", "Auth")
			http.Redirect(w, r, "/", http.StatusFound)
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

		slog.Warn("Failed login attempt", "email", email, "ip", ip, "component", "Auth")
		w.Header().Set("Content-Type", "text/html")
		w.WriteHeader(http.StatusUnauthorized)
		fmt.Fprint(w, strings.ReplaceAll(loginHTML, "<!--ERROR-->",
			`<div class="login-error">Invalid username or password</div>`))

	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// handleLogout clears the session cookie and redirects to /login (or / when auth is disabled).
func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
	if cookie, err := r.Cookie(sessionCookieName); err == nil {
		s.sessions.Delete(cookie.Value)
		slog.Info("User logged out", "remote", r.RemoteAddr, "component", "Auth")
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
