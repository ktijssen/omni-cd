package web

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"log/slog"
	"net"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"
)

const (
	sessionCookieName = "omnicd_session"
	sessionDuration   = 24 * time.Hour
	maxLoginFailures  = 5
	loginLockDuration = 15 * time.Minute
	sessionFile       = "/data/auth/sessions.json"
)

// loadSessions reads persisted sessions from disk and populates the in-memory map,
// skipping any that have already expired.
func (s *Server) loadSessions() {
	data, err := os.ReadFile(sessionFile)
	if os.IsNotExist(err) {
		return
	}
	if err != nil {
		slog.Warn("Could not read session file", "err", err, "component", "Auth")
		return
	}
	var saved map[string]sessionInfo
	if err := json.Unmarshal(data, &saved); err != nil {
		slog.Warn("Could not parse session file", "err", err, "component", "Auth")
		return
	}
	loaded := 0
	for token, info := range saved {
		if time.Since(info.LoginTime) < sessionDuration {
			s.sessions.Store(token, info)
			loaded++
		}
	}
	slog.Info("Loaded persisted sessions", "count", loaded, "component", "Auth")
}

// saveSessions writes all active, non-expired sessions to disk so they survive restarts.
func (s *Server) saveSessions() {
	m := make(map[string]sessionInfo)
	s.sessions.Range(func(key, value any) bool {
		info := value.(sessionInfo)
		if time.Since(info.LoginTime) < sessionDuration {
			m[key.(string)] = info
		}
		return true
	})
	data, err := json.Marshal(m)
	if err != nil {
		slog.Warn("Could not marshal sessions", "err", err, "component", "Auth")
		return
	}
	if err := os.MkdirAll("/data/auth", 0700); err != nil {
		slog.Warn("Could not create auth dir", "err", err, "component", "Auth")
		return
	}
	if err := os.WriteFile(sessionFile, data, 0600); err != nil {
		slog.Warn("Could not write session file", "err", err, "component", "Auth")
	}
}

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
		// No local users configured and OIDC is not active — force first-time setup.
		if s.authStore != nil && s.authStore.IsEmpty() && !s.oidcEnabled() {
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

// webhookBucketFor returns the rate-limit bucket for the given IP used to
// throttle bad-signature webhook attempts. Distinct from loginBuckets so the
// two surfaces have independent lockouts.
func (s *Server) webhookBucketFor(ip string) *loginBucket {
	v, _ := s.webhookBuckets.LoadOrStore(ip, &loginBucket{lastSeen: time.Now()})
	bucket := v.(*loginBucket)
	bucket.mu.Lock()
	bucket.lastSeen = time.Now()
	bucket.mu.Unlock()
	return bucket
}

// recordWebhookFailure increments the failure counter for an IP and, after
// 10 consecutive bad signatures, applies a 15-minute lockout.
func (s *Server) recordWebhookFailure(bucket *loginBucket, ip, reason string) {
	bucket.mu.Lock()
	bucket.failures++
	failures := bucket.failures
	if failures >= 10 {
		bucket.lockedUntil = time.Now().Add(15 * time.Minute)
		bucket.failures = 0
	}
	bucket.mu.Unlock()
	slog.Warn("Webhook auth failure", "component", "Web", "ip", ip, "reason", reason, "failures", failures)
}

// cleanupLoginBuckets periodically removes stale rate-limit buckets to prevent
// unbounded memory growth when many distinct IPs attempt logins or send
// webhook requests.
func (s *Server) cleanupLoginBuckets() {
	ticker := time.NewTicker(30 * time.Minute)
	defer ticker.Stop()
	for range ticker.C {
		cutoff := time.Now().Add(-30 * time.Minute)
		purge := func(m *sync.Map) {
			m.Range(func(key, value any) bool {
				bucket := value.(*loginBucket)
				bucket.mu.Lock()
				stale := bucket.lastSeen.Before(cutoff)
				bucket.mu.Unlock()
				if stale {
					m.Delete(key)
				}
				return true
			})
		}
		purge(&s.loginBuckets)
		purge(&s.webhookBuckets)
	}
}
