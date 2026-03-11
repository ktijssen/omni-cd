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
	lastSeen    time.Time
}

// isSecure returns true when the request arrived over TLS or via an
// HTTPS-terminated reverse proxy (X-Forwarded-Proto: https).
func isSecure(r *http.Request) bool {
	return r.TLS != nil || r.Header.Get("X-Forwarded-Proto") == "https"
}

// clientIP extracts the real client IP. X-Forwarded-For is only trusted when
// the connection originates from a private/loopback address (i.e. a local proxy).
func clientIP(r *http.Request) string {
	remoteHost, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		remoteHost = r.RemoteAddr
	}
	if fwd := r.Header.Get("X-Forwarded-For"); fwd != "" && isPrivateAddr(remoteHost) {
		if i := strings.IndexByte(fwd, ','); i >= 0 {
			return strings.TrimSpace(fwd[:i])
		}
		return strings.TrimSpace(fwd)
	}
	return remoteHost
}

// isPrivateAddr reports whether addr is a loopback or RFC-1918/4193 private address.
func isPrivateAddr(addr string) bool {
	ip := net.ParseIP(addr)
	if ip == nil {
		return false
	}
	for _, cidr := range []string{
		"127.0.0.0/8", "::1/128",
		"10.0.0.0/8", "172.16.0.0/12", "192.168.0.0/16",
		"fc00::/7",
	} {
		_, network, _ := net.ParseCIDR(cidr)
		if network.Contains(ip) {
			return true
		}
	}
	return false
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
		// No users configured yet — force first-time setup regardless of OIDC.
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
		// OIDC users with "none" role are authenticated but not authorised.
		// Redirect them to /unauthorized for any page, or 403 for API/ws.
		if role := s.sessionRole(r); role == "none" {
			if strings.HasPrefix(r.URL.Path, "/api/") || r.URL.Path == "/ws" {
				http.Error(w, "Forbidden", http.StatusForbidden)
				return
			}
			if r.URL.Path != "/unauthorized" {
				http.Redirect(w, r, "/unauthorized", http.StatusFound)
				return
			}
		}
		next(w, r)
	}
}

// sessionInfo holds per-session metadata.
type sessionInfo struct {
	LoginTime   time.Time
	Username    string // login identifier: username for local, email for OIDC
	DisplayName string // shown in the sidebar
	AuthMethod  string // "local" or "oidc"
	Role        string // "admin", "viewer", "none" — empty means local-auth default (admin)
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

// sessionIdentity returns the login identifier for the session cookie in r, or empty string.
func (s *Server) sessionIdentity(r *http.Request) string {
	cookie, err := r.Cookie(sessionCookieName)
	if err != nil {
		return ""
	}
	v, ok := s.sessions.Load(cookie.Value)
	if !ok {
		return ""
	}
	return v.(sessionInfo).Username
}

// sessionRole returns the role for the session in r.
// Local-auth sessions have an empty Role field and are always treated as "admin".
func (s *Server) sessionRole(r *http.Request) string {
	cookie, err := r.Cookie(sessionCookieName)
	if err != nil {
		return ""
	}
	v, ok := s.sessions.Load(cookie.Value)
	if !ok {
		return ""
	}
	info := v.(sessionInfo)
	if info.AuthMethod == "oidc" {
		return info.Role
	}
	return "admin" // local-auth users are always admin
}

// roleAtLeast returns true when role meets or exceeds minRole in the hierarchy.
// Hierarchy (ascending): none < viewer < admin.
func roleAtLeast(role, minRole string) bool {
	levels := map[string]int{"none": 0, "viewer": 1, "admin": 2}
	return levels[role] >= levels[minRole]
}

// requireRole wraps a handler and enforces a minimum role level.
// It calls requireAuth first, so unauthenticated requests are redirected to /login.
// If the session role is below minRole, API requests get 403 and page requests
// are redirected to /unauthorized.
func (s *Server) requireRole(minRole string, next http.HandlerFunc) http.HandlerFunc {
	return s.requireAuth(func(w http.ResponseWriter, r *http.Request) {
		if s.authDisabled {
			next(w, r)
			return
		}
		role := s.sessionRole(r)
		if !roleAtLeast(role, minRole) {
			if strings.HasPrefix(r.URL.Path, "/api/") || r.URL.Path == "/ws" {
				http.Error(w, "Forbidden", http.StatusForbidden)
				return
			}
			http.Redirect(w, r, "/unauthorized", http.StatusFound)
			return
		}
		next(w, r)
	})
}

// loginBucketFor returns the rate-limit bucket for the given IP, creating it if needed.
func (s *Server) loginBucketFor(ip string) *loginBucket {
	v, _ := s.loginBuckets.LoadOrStore(ip, &loginBucket{lastSeen: time.Now()})
	bucket := v.(*loginBucket)
	bucket.mu.Lock()
	bucket.lastSeen = time.Now()
	bucket.mu.Unlock()
	return bucket
}

// cleanupLoginBuckets periodically removes stale rate-limit buckets to prevent
// unbounded memory growth when many distinct IPs attempt logins.
func (s *Server) cleanupLoginBuckets() {
	ticker := time.NewTicker(30 * time.Minute)
	defer ticker.Stop()
	for range ticker.C {
		cutoff := time.Now().Add(-30 * time.Minute)
		s.loginBuckets.Range(func(key, value any) bool {
			bucket := value.(*loginBucket)
			bucket.mu.Lock()
			stale := bucket.lastSeen.Before(cutoff)
			bucket.mu.Unlock()
			if stale {
				s.loginBuckets.Delete(key)
			}
			return true
		})
	}
}

// renderLoginPage builds the full login HTML, injecting an optional error banner and the form/SSO button.
func (s *Server) renderLoginPage(errorHTML string) string {
	page := loginHTML
	if errorHTML != "" {
		page = strings.ReplaceAll(page, "<!--ERROR-->", errorHTML)
	}
	page = strings.ReplaceAll(page, "<!--LOCAL_FORM-->", localFormHTML)
	if s.oidcEnabled() {
		page = strings.ReplaceAll(page, "<!--OIDC_BUTTON-->", `<div class="login-divider"><span>or</span></div><a href="/auth/login" class="sso-btn">Sign in with SSO</a>`)
	} else {
		page = strings.ReplaceAll(page, "<!--OIDC_BUTTON-->", "")
	}
	return page
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
		fmt.Fprint(w, s.renderLoginPage(""))

	case http.MethodPost:
		r.Body = http.MaxBytesReader(w, r.Body, 1<<20) // 1 MB limit
		ip := clientIP(r)
		bucket := s.loginBucketFor(ip)

		bucket.mu.Lock()
		if time.Now().Before(bucket.lockedUntil) {
			bucket.mu.Unlock()
			slog.Warn("Login blocked — too many failed attempts", "ip", ip, "component", "Auth")
			w.Header().Set("Content-Type", "text/html")
			w.WriteHeader(http.StatusTooManyRequests)
			fmt.Fprint(w, s.renderLoginPage(`<div class="login-error">Too many failed attempts — try again in 15 minutes</div>`))
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
			s.sessions.Store(token, sessionInfo{LoginTime: time.Now(), Username: username, DisplayName: displayName})
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

		slog.Warn("Failed login attempt", "username", username, "ip", ip, "component", "Auth")
		w.Header().Set("Content-Type", "text/html")
		w.WriteHeader(http.StatusUnauthorized)
		fmt.Fprint(w, s.renderLoginPage(`<div class="login-error">Invalid username or password</div>`))

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
