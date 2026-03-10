package web

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"

	gooidc "github.com/coreos/go-oidc/v3/oidc"
	oidcconfig "omni-cd/internal/oidc"
	"golang.org/x/oauth2"
)

// OIDCRuntime holds the live OIDC provider and oauth2 config derived from an
// oidcconfig.Config. Access via the Server helpers; the mutex makes it safe to
// hot-swap when config changes.
type OIDCRuntime struct {
	mu         sync.RWMutex
	provider   *gooidc.Provider
	oauth2     *oauth2.Config
	cfg        *oidcconfig.Config
	httpClient *http.Client // non-nil only when cfg.Insecure is true
}

// InitOIDCRuntime initialises a new OIDCRuntime from cfg.
// The OIDC discovery call is made with a 15 s timeout.
func InitOIDCRuntime(cfg *oidcconfig.Config) (*OIDCRuntime, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	var insecureClient *http.Client
	if cfg.Insecure {
		insecureClient = &http.Client{
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{InsecureSkipVerify: true}, //nolint:gosec
			},
		}
		ctx = gooidc.ClientContext(ctx, insecureClient)
	}

	provider, err := gooidc.NewProvider(ctx, cfg.IssuerURL)
	if err != nil {
		return nil, fmt.Errorf("OIDC discovery for %q failed: %w", cfg.IssuerURL, err)
	}

	oauth2Cfg := &oauth2.Config{
		ClientID:     cfg.ClientID,
		ClientSecret: cfg.ClientSecret,
		RedirectURL:  cfg.RedirectURL,
		Endpoint:     provider.Endpoint(),
		Scopes:       cfg.GetScopes(),
	}

	return &OIDCRuntime{provider: provider, oauth2: oauth2Cfg, cfg: cfg, httpClient: insecureClient}, nil
}

// oidcEnabled returns true when a live OIDC runtime is loaded.
func (s *Server) oidcEnabled() bool {
	s.oidcMu.RLock()
	defer s.oidcMu.RUnlock()
	return s.oidcRT != nil
}

// OIDCRuntime returns the live runtime (or nil).
func (s *Server) getOIDCRuntime() *OIDCRuntime {
	s.oidcMu.RLock()
	defer s.oidcMu.RUnlock()
	return s.oidcRT
}

// setOIDCRuntime atomically replaces the live runtime.
func (s *Server) setOIDCRuntime(rt *OIDCRuntime) {
	s.oidcMu.Lock()
	defer s.oidcMu.Unlock()
	s.oidcRT = rt
}

// resolveRole maps a user's email and group list to a role using the OIDC config.
// Precedence: adminEmails > adminGroups > viewerEmails > viewerGroups > defaultRole.
func resolveRole(cfg *oidcconfig.Config, email string, groups []string) string {
	for _, e := range cfg.AdminEmails {
		if strings.EqualFold(e, email) {
			return "admin"
		}
	}
	for _, g := range groups {
		for _, ag := range cfg.AdminGroups {
			if strings.EqualFold(g, ag) {
				return "admin"
			}
		}
	}
	for _, e := range cfg.ViewerEmails {
		if strings.EqualFold(e, email) {
			return "viewer"
		}
	}
	for _, g := range groups {
		for _, vg := range cfg.ViewerGroups {
			if strings.EqualFold(g, vg) {
				return "viewer"
			}
		}
	}
	return cfg.GetDefaultRole()
}

// oidcStateEntry is stored in Server.oidcStates keyed by the state token.
type oidcStateEntry struct {
	CreatedAt   time.Time
	RedirectURL string // redirect_uri used in this specific auth request
}

// deriveRedirectURL builds the callback URL from the incoming request when
// no explicit redirect URL is configured. Respects X-Forwarded-Proto / Host.
func deriveRedirectURL(r *http.Request) string {
	scheme := "https"
	if proto := r.Header.Get("X-Forwarded-Proto"); proto != "" {
		scheme = proto
	} else if r.TLS == nil {
		scheme = "http"
	}
	host := r.Header.Get("X-Forwarded-Host")
	if host == "" {
		host = r.Host
	}
	return scheme + "://" + host + "/auth/callback"
}

// --- OIDC flow handlers ---

// handleOIDCLogin redirects the browser to the IdP's authorisation endpoint.
// GET /auth/login
func (s *Server) handleOIDCLogin(w http.ResponseWriter, r *http.Request) {
	rt := s.getOIDCRuntime()
	if rt == nil {
		http.Error(w, "OIDC not configured", http.StatusServiceUnavailable)
		return
	}

	state, err := generateToken()
	if err != nil {
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	// Determine redirect URL: use configured value or derive from request.
	redirectURL := rt.cfg.RedirectURL
	if redirectURL == "" {
		redirectURL = deriveRedirectURL(r)
	}

	s.oidcStates.Store(state, oidcStateEntry{CreatedAt: time.Now(), RedirectURL: redirectURL})

	authURL := rt.oauth2.AuthCodeURL(state, oauth2.SetAuthURLParam("redirect_uri", redirectURL))
	http.Redirect(w, r, authURL, http.StatusFound)
}

// handleOIDCCallback handles the IdP's redirect back to /auth/callback.
func (s *Server) handleOIDCCallback(w http.ResponseWriter, r *http.Request) {
	rt := s.getOIDCRuntime()
	if rt == nil {
		http.Error(w, "OIDC not configured", http.StatusServiceUnavailable)
		return
	}

	// Surface any error returned by the IdP (e.g. unregistered redirect_uri).
	if errCode := r.URL.Query().Get("error"); errCode != "" {
		desc := r.URL.Query().Get("error_description")
		if desc == "" {
			desc = errCode
		}
		slog.Warn("OIDC provider returned error", "error", errCode, "description", desc, "component", "OIDC")
		http.Error(w, "SSO error: "+desc, http.StatusBadRequest)
		return
	}

	// Validate state.
	state := r.URL.Query().Get("state")
	if state == "" {
		http.Error(w, "Missing state parameter", http.StatusBadRequest)
		return
	}
	raw, ok := s.oidcStates.Load(state)
	if !ok {
		http.Error(w, "Invalid or expired state", http.StatusBadRequest)
		return
	}
	entry := raw.(oidcStateEntry)
	if time.Since(entry.CreatedAt) > 10*time.Minute {
		s.oidcStates.Delete(state)
		http.Error(w, "State expired", http.StatusBadRequest)
		return
	}
	s.oidcStates.Delete(state)

	// Exchange code for tokens.
	code := r.URL.Query().Get("code")
	if code == "" {
		http.Error(w, "Missing code parameter", http.StatusBadRequest)
		return
	}

	ctx := context.Background()
	if rt.httpClient != nil {
		ctx = gooidc.ClientContext(ctx, rt.httpClient)
	}
	// Pass the same redirect_uri that was used in the auth request.
	token, err := rt.oauth2.Exchange(ctx, code, oauth2.SetAuthURLParam("redirect_uri", entry.RedirectURL))
	if err != nil {
		slog.Warn("OIDC token exchange failed", "error", err, "redirect_uri", entry.RedirectURL, "component", "OIDC")
		http.Error(w, "Token exchange failed: "+err.Error(), http.StatusBadRequest)
		return
	}

	// Verify id_token.
	rawIDToken, ok := token.Extra("id_token").(string)
	if !ok {
		http.Error(w, "No id_token in response", http.StatusBadRequest)
		return
	}
	verifier := rt.provider.Verifier(&gooidc.Config{ClientID: rt.cfg.ClientID})
	idToken, err := verifier.Verify(ctx, rawIDToken)
	if err != nil {
		slog.Warn("OIDC id_token verification failed", "error", err, "component", "OIDC")
		http.Error(w, "Token verification failed", http.StatusUnauthorized)
		return
	}

	// Extract standard claims from id_token.
	var claims struct {
		Email         string   `json:"email"`
		Name          string   `json:"name"`
		PreferredName string   `json:"preferred_username"`
		Groups        []string `json:"groups"`
	}
	if err := idToken.Claims(&claims); err != nil {
		http.Error(w, "Failed to parse claims", http.StatusInternalServerError)
		return
	}

	// If groups weren't in the id_token, try the userinfo endpoint.
	if len(claims.Groups) == 0 {
		userInfo, uiErr := rt.provider.UserInfo(ctx, oauth2.StaticTokenSource(token))
		if uiErr == nil {
			var uiClaims map[string]interface{}
			if uiErr = userInfo.Claims(&uiClaims); uiErr == nil {
				groupClaim := rt.cfg.GetGroupsClaim()
				if raw, found := uiClaims[groupClaim]; found {
					if arr, ok2 := raw.([]interface{}); ok2 {
						for _, g := range arr {
							if gs, ok3 := g.(string); ok3 {
								claims.Groups = append(claims.Groups, gs)
							}
						}
					}
				}
			}
		}
	}

	displayName := claims.Name
	if displayName == "" {
		displayName = claims.PreferredName
	}
	if displayName == "" {
		displayName = claims.Email
	}

	rt.mu.RLock()
	cfg := rt.cfg
	rt.mu.RUnlock()

	oidcRole := resolveRole(cfg, claims.Email, claims.Groups)

	// The first OIDC user ever is automatically promoted to admin so the
	// instance is usable without pre-configuring email/group mappings.
	initialRole := oidcRole
	if s.oidcUsers != nil && s.oidcUsers.isEmpty() {
		initialRole = "admin"
		slog.Info("First OIDC login — promoting to admin", "email", claims.Email, "component", "OIDC")
	}

	// Upsert the user. For new users, initialRole is stored. For existing
	// users, the stored role (which the admin may have changed) is returned.
	role := initialRole
	if s.oidcUsers != nil {
		role = s.oidcUsers.upsert(claims.Email, displayName, initialRole)
	}

	sessionToken, err := generateToken()
	if err != nil {
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	s.sessions.Store(sessionToken, sessionInfo{
		LoginTime:   time.Now(),
		Username:    claims.Email,
		DisplayName: displayName,
		AuthMethod:  "oidc",
		Role:        role,
	})
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    sessionToken,
		Path:     "/",
		HttpOnly: true,
		Secure:   isSecure(r),
		SameSite: http.SameSiteLaxMode,
		MaxAge:   int(sessionDuration.Seconds()),
	})

	slog.Info("OIDC user logged in", "email", claims.Email, "role", role, "component", "OIDC")

	if role == "none" {
		http.Redirect(w, r, "/unauthorized", http.StatusFound)
		return
	}
	http.Redirect(w, r, "/", http.StatusFound)
}

// handleUnauthorized serves the /unauthorized page shown to OIDC users with the "none" role.
func (s *Server) handleUnauthorized(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html")
	w.WriteHeader(http.StatusForbidden)
	fmt.Fprint(w, unauthorizedHTML)
}

// --- OIDC config API ---

// handleOIDCConfigAPI serves GET /api/oidc-config (read-only; config is env-only).
func (s *Server) handleOIDCConfigAPI(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	s.handleGetOIDCConfig(w, r)
}

// handleGetOIDCConfig returns the current OIDC config (secret masked). GET /api/oidc-config
func (s *Server) handleGetOIDCConfig(w http.ResponseWriter, r *http.Request) {
	rt := s.getOIDCRuntime()
	w.Header().Set("Content-Type", "application/json")
	if rt == nil {
		json.NewEncoder(w).Encode(map[string]interface{}{"configured": false})
		return
	}
	rt.mu.RLock()
	cfg := *rt.cfg
	rt.mu.RUnlock()

	// Mask the client secret.
	masked := cfg
	if masked.ClientSecret != "" {
		masked.ClientSecret = "••••••••"
	}
	json.NewEncoder(w).Encode(map[string]interface{}{
		"configured": true,
		"config":     masked,
	})
}


// --- helpers ---


// unauthorizedHTML is the page shown to OIDC users whose role is "none".
const unauthorizedHTML = `<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="UTF-8">
<meta name="viewport" content="width=device-width, initial-scale=1.0">
<title>Omni CD · Access Denied</title>
<style>
  * { margin: 0; padding: 0; box-sizing: border-box; }
  body {
    font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif;
    background: #1b1b1d; color: #e4e4e7;
    min-height: 100vh; display: flex; align-items: center; justify-content: center;
  }
  .wrap { text-align: center; max-width: 400px; padding: 24px; }
  .icon { font-size: 48px; margin-bottom: 16px; }
  h1 { font-size: 22px; font-weight: 700; color: #fff; margin-bottom: 8px; }
  p { font-size: 14px; color: #71717a; margin-bottom: 24px; line-height: 1.5; }
  a {
    display: inline-block;
    background: #FB326E; color: #fff;
    border-radius: 8px; padding: 10px 20px;
    font-size: 14px; font-weight: 600; text-decoration: none;
  }
  a:hover { background: #e0285f; }
</style>
</head>
<body>
<div class="wrap">
  <div class="icon">🚫</div>
  <h1>Access Denied</h1>
  <p>Your account does not have permission to access Omni CD.<br>
     Contact your administrator to request access.</p>
  <a href="/logout">Sign out</a>
</div>
</body>
</html>`
