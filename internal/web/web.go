package web

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"sync"
	"time"

	"omni-cd/internal/auth"
	"omni-cd/internal/metrics"
	"omni-cd/internal/state"

	"github.com/gorilla/websocket"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// withTimeout wraps a handler so each request must complete within d, after
// which a 503 is sent. Use for handlers that may stream large bodies
// (log/audit downloads) so a slow client cannot pin a connection.
func withTimeout(d time.Duration, h http.Handler) http.Handler {
	return http.TimeoutHandler(h, d, "request timed out")
}

// upgrader is the WebSocket upgrade configuration.
//
// CheckOrigin permits requests with no Origin header (non-browser clients,
// e.g. CLI tools). This is safe because the /ws route is wrapped with
// requireRole("viewer", ...) in Start(), which runs requireAuth and rejects
// the request with 403 before the upgrade is attempted. A non-browser client
// without a valid session cookie cannot reach this code path.
var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		origin := r.Header.Get("Origin")
		if origin == "" {
			return true
		}
		u, err := url.ParseRequestURI(origin)
		if err != nil {
			return false
		}
		// Only allow same-origin requests.
		return u.Host == r.Host
	},
}

// Server serves the web UI and API endpoints.
type Server struct {
	appState               *state.AppState
	triggerHard            chan struct{}
	triggerSoft            chan struct{}
	triggerRefreshCluster  chan string
	triggerDeleteCluster   chan string
	triggerDeleteMC        chan string
	triggerMCRefresh       chan struct{}
	triggerMCRefreshSingle chan string
	triggerRepoChange      chan struct{}
	triggerOmniConfigured  chan struct{}
	omniInstanceFile       string
	logDir                 string
	auditDir               string
	port                   string
	version                string
	webhookSecret          string
	clients                map[*websocket.Conn]bool
	clientsMu              sync.RWMutex
	broadcast              chan []byte
	authStore              *auth.Store
	authDisabled           bool
	sessions               sync.Map // token (string) -> sessionInfo
	loginBuckets           sync.Map // IP (string) -> *loginBucket
	webhookBuckets         sync.Map // IP (string) -> *loginBucket — reused for bad-signature lockouts
	metricsCollector       *metrics.Collector
	metricsPort            string
	// OIDC
	oidcRT     *OIDCRuntime
	oidcMu     sync.RWMutex
	oidcStates sync.Map // state token (string) -> oidcStateEntry
	oidcUsers  *oidcUserStore
}

// Options bundles every dependency the web server needs. Using a struct
// instead of positional arguments keeps the main.go call site readable and
// lets new fields be added without breaking existing callers.
type Options struct {
	AppState               *state.AppState
	TriggerHard            chan struct{}
	TriggerSoft            chan struct{}
	TriggerRefreshCluster  chan string
	TriggerDeleteCluster   chan string
	TriggerDeleteMC        chan string
	TriggerMCRefresh       chan struct{}
	TriggerMCRefreshSingle chan string
	TriggerRepoChange      chan struct{}
	TriggerOmniConfigured  chan struct{}
	OmniInstanceFile       string
	LogDir                 string
	AuditDir               string
	Port                   string
	Version                string
	AuthStore              *auth.Store
	AuthDisabled           bool
	OIDC                   *OIDCRuntime
	WebhookSecret          string
	Metrics                *metrics.Collector
	MetricsPort            string
}

// New creates a new web server from the given options.
func New(opts Options) *Server {
	s := &Server{
		appState:               opts.AppState,
		triggerHard:            opts.TriggerHard,
		triggerSoft:            opts.TriggerSoft,
		triggerRefreshCluster:  opts.TriggerRefreshCluster,
		triggerDeleteCluster:   opts.TriggerDeleteCluster,
		triggerDeleteMC:        opts.TriggerDeleteMC,
		triggerMCRefresh:       opts.TriggerMCRefresh,
		triggerMCRefreshSingle: opts.TriggerMCRefreshSingle,
		triggerRepoChange:      opts.TriggerRepoChange,
		triggerOmniConfigured:  opts.TriggerOmniConfigured,
		omniInstanceFile:       opts.OmniInstanceFile,
		logDir:                 opts.LogDir,
		auditDir:               opts.AuditDir,
		port:                   opts.Port,
		version:                opts.Version,
		clients:                make(map[*websocket.Conn]bool),
		broadcast:              make(chan []byte, 256),
		authStore:              opts.AuthStore,
		authDisabled:           opts.AuthDisabled,
		oidcRT:                 opts.OIDC,
		webhookSecret:          opts.WebhookSecret,
		oidcUsers:              loadOIDCUserStore(oidcUsersPath),
		metricsCollector:       opts.Metrics,
		metricsPort:            opts.MetricsPort,
	}

	// Load persisted sessions so users stay logged in across restarts.
	s.loadSessions()

	// Start broadcast handler
	go s.handleBroadcasts()

	// Start state change monitor
	go s.monitorStateChanges()

	// Periodically purge expired OIDC state tokens to prevent unbounded growth.
	go s.cleanupOIDCStates()

	// Periodically purge stale login rate-limit buckets to prevent unbounded growth.
	go s.cleanupLoginBuckets()

	return s
}

// Start starts the web server in a goroutine.
func (s *Server) Start() {
	// Register and expose Prometheus metrics on a dedicated port.
	if s.metricsCollector != nil && s.metricsPort != "" {
		reg := prometheus.NewRegistry()
		reg.MustRegister(s.metricsCollector)
		metricsMux := http.NewServeMux()
		metricsMux.Handle("/metrics", promhttp.HandlerFor(reg, promhttp.HandlerOpts{}))
		metricsAddr := ":" + s.metricsPort
		slog.Info("Metrics server listening", "address", metricsAddr, "component", "Web")
		go func() {
			if err := http.ListenAndServe(metricsAddr, metricsMux); err != nil && err != http.ErrServerClosed {
				slog.Error("Metrics server failed", "error", err, "component", "Web")
			}
		}()
	}

	mux := http.NewServeMux()

	// Public routes — no auth required
	mux.HandleFunc("/setup", s.handleSetup)
	mux.HandleFunc("/login", s.handleLogin)
	mux.HandleFunc("/logout", s.handleLogout)
	mux.HandleFunc("/unauthorized", s.handleUnauthorized)
	mux.HandleFunc("/api/login-config", s.handleLoginConfig)
	mux.HandleFunc("/api/setup-status", s.handleSetupStatus)
	mux.HandleFunc("/assets/", s.handleStaticAsset)

	// OIDC SSO routes — public (the handlers check OIDC is enabled)
	mux.HandleFunc("/auth/login", s.handleOIDCLogin)
	mux.HandleFunc("/auth/callback", s.handleOIDCCallback)

	// Webhook endpoint — public, protected by HMAC secret
	mux.HandleFunc("/api/webhook", s.handleWebhook)

	// WebSocket endpoint — viewer+
	mux.HandleFunc("/ws", s.requireRole("viewer", s.handleWebSocket))

	// Read-only API endpoints — viewer+
	mux.HandleFunc("/api/me", s.requireRole("viewer", s.handleMe))
	mux.HandleFunc("/api/state", s.requireRole("viewer", s.handleState))
	mux.HandleFunc("/api/logs/files", s.requireRole("viewer", s.handleLogFiles))
	// Downloads can serve large daily log/audit files; wrap with a per-request
	// timeout so a slow client cannot pin a connection past WriteTimeout.
	mux.Handle("/api/logs/download", withTimeout(30*time.Second, s.requireRole("viewer", s.handleLogDownload)))
	mux.HandleFunc("/api/audit", s.requireRole("viewer", s.handleAudit))
	mux.HandleFunc("/api/audit/files", s.requireRole("viewer", s.handleAuditFiles))
	mux.Handle("/api/audit/download", withTimeout(30*time.Second, s.requireRole("viewer", s.handleAuditDownload)))

	// Write API endpoints — admin only
	mux.HandleFunc("/api/reconcile", s.requireRole("admin", s.handleReconcile))
	mux.HandleFunc("/api/check", s.requireRole("admin", s.handleCheck))
	mux.HandleFunc("/api/clusters-toggle", s.requireRole("admin", s.handleClustersToggle))
	mux.HandleFunc("/api/force-cluster", s.requireRole("admin", s.handleForceCluster))
	mux.HandleFunc("/api/refresh-cluster", s.requireRole("admin", s.handleRefreshCluster))
	mux.HandleFunc("/api/delete-cluster", s.requireRole("admin", s.handleDeleteCluster))
	mux.HandleFunc("/api/set-cluster-autosync", s.requireRole("admin", s.handleSetClusterAutoSync))
	mux.HandleFunc("/api/export-cluster", s.requireRole("admin", s.handleExportCluster))
	mux.HandleFunc("/api/cluster-manifests", s.requireRole("viewer", s.handleClusterManifests))
	mux.HandleFunc("/api/repos", s.requireRole("admin", s.handleRepos))
	mux.HandleFunc("/api/repos/test", s.requireRole("admin", s.handleTestRepo))
	mux.HandleFunc("/api/refresh-mc", s.requireRole("admin", s.handleRefreshMC))
	mux.HandleFunc("/api/refresh-single-mc", s.requireRole("admin", s.handleRefreshSingleMC))
	mux.HandleFunc("/api/delete-machineclass", s.requireRole("admin", s.handleDeleteMachineClass))
	mux.HandleFunc("/api/sync-machineclass", s.requireRole("admin", s.handleSyncMachineClass))
	mux.HandleFunc("/api/set-mc-autosync", s.requireRole("admin", s.handleSetMCAutoSync))
	mux.HandleFunc("/api/omni-instance", s.requireRole("admin", s.handleOmniInstance))
	mux.HandleFunc("/api/omni-instance/test", s.requireRole("admin", s.handleTestOmniInstance))
	mux.HandleFunc("/api/omni-instance/refresh", s.requireRole("admin", s.handleRefreshOmniConnection))
	mux.HandleFunc("/api/users", s.requireRole("admin", s.handleUsers))
	mux.HandleFunc("/api/users/change-password", s.requireRole("admin", s.handleChangePassword))
	mux.HandleFunc("/api/users/update-profile", s.requireRole("admin", s.handleUpdateProfile))
	mux.HandleFunc("/api/users/oidc", s.requireRole("admin", s.handleOIDCUsers))
	mux.HandleFunc("/api/oidc-config", s.requireRole("admin", s.handleOIDCConfigAPI))

	// Serve the UI (all protected, viewer+)
	mux.HandleFunc("/clusters/", s.requireRole("viewer", s.handleUI))
	mux.HandleFunc("/clusters", s.requireRole("viewer", s.handleUI))
	mux.HandleFunc("/machineclasses", s.requireRole("viewer", s.handleUI))
	mux.HandleFunc("/repos", s.requireRole("viewer", s.handleUI))
	mux.HandleFunc("/logs", s.requireRole("viewer", s.handleUI))
	mux.HandleFunc("/audit", s.requireRole("viewer", s.handleUI))
	mux.HandleFunc("/users", s.requireRole("admin", func(w http.ResponseWriter, r *http.Request) {
		if s.authDisabled {
			http.Redirect(w, r, "/clusters", http.StatusFound)
			return
		}
		s.handleUI(w, r)
	}))
	mux.HandleFunc("/", s.requireRole("viewer", s.handleUI))

	addr := fmt.Sprintf(":%s", s.port)
	slog.Info("Web UI listening", "address", addr, "component", "Web")

	srv := &http.Server{
		Addr:         addr,
		Handler:      securityHeadersMiddleware(mux),
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 60 * time.Second,
		IdleTimeout:  120 * time.Second,
	}
	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("Web server failed", "error", err, "component", "Web")
		}
	}()
}

// handleWebSocket upgrades HTTP connection to WebSocket and manages client lifecycle.
func (s *Server) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		slog.Error("WebSocket upgrade failed", "error", err, "component", "Web")
		return
	}
	defer conn.Close()

	// Send the initial state BEFORE registering the connection with the
	// broadcaster. gorilla/websocket forbids concurrent WriteMessage calls
	// on the same conn, so the initial write must complete before the
	// broadcaster goroutine can pick this conn up from s.clients.
	snapshot := s.appState.Snapshot()
	data, err := json.Marshal(snapshot)
	if err != nil {
		slog.Error("Failed to marshal initial WS snapshot", "error", err, "component", "Web")
		return
	}
	if err := conn.WriteMessage(websocket.TextMessage, data); err != nil {
		slog.Debug("Failed to send initial WS snapshot", "error", err, "component", "Web")
		return
	}

	// Register client
	s.clientsMu.Lock()
	s.clients[conn] = true
	s.clientsMu.Unlock()

	slog.Debug("WebSocket client connected", "component", "Web")

	// Wait for client disconnect
	for {
		if _, _, err := conn.ReadMessage(); err != nil {
			break
		}
	}

	// Unregister client
	s.clientsMu.Lock()
	delete(s.clients, conn)
	s.clientsMu.Unlock()

	slog.Debug("WebSocket client disconnected", "component", "Web")
}

// handleBroadcasts sends state updates to all connected WebSocket clients.
func (s *Server) handleBroadcasts() {
	for message := range s.broadcast {
		// Collect failed connections while holding the read lock continuously.
		// Releasing and re-acquiring the lock mid-iteration allows concurrent
		// handleWebSocket calls to modify s.clients, which corrupts the ongoing
		// map iteration and can cause a panic or silent data loss.
		var failed []*websocket.Conn
		s.clientsMu.RLock()
		for client := range s.clients {
			if err := client.WriteMessage(websocket.TextMessage, message); err != nil {
				client.Close()
				failed = append(failed, client)
			}
		}
		s.clientsMu.RUnlock()
		// Remove all failed connections in a single write-lock pass, after the
		// iteration has fully completed.
		if len(failed) > 0 {
			s.clientsMu.Lock()
			for _, client := range failed {
				delete(s.clients, client)
			}
			s.clientsMu.Unlock()
		}
	}
}

// monitorStateChanges broadcasts state immediately on any mutation, with a
// 1-second ticker as a fallback to catch any updates that may be missed.
func (s *Server) monitorStateChanges() {
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()
	changeCh := s.appState.ChangeCh()

	var lastHash uint64
	maybeBroadcast := func() {
		snapshot := s.appState.Snapshot()
		currentHash := s.hashState(snapshot)
		if currentHash != lastHash {
			lastHash = currentHash
			if data, err := json.Marshal(snapshot); err == nil {
				select {
				case s.broadcast <- data:
				default:
				}
			}
		}
	}

	for {
		select {
		case <-changeCh:
			maybeBroadcast()
		case <-ticker.C:
			maybeBroadcast()
		}
	}
}

// hashState creates a hash of the state for change detection.
func (s *Server) hashState(snapshot state.SnapshotData) uint64 {
	var hash uint64
	hash = uint64(len(snapshot.MachineClasses))
	hash = hash*31 + uint64(len(snapshot.Clusters))
	hash = hash*31 + uint64(len(snapshot.Logs))
	if snapshot.ClustersEnabled {
		hash = hash * 31
	}
	// Hash per-cluster AutoSync so toggling it triggers an immediate UI refresh.
	for _, c := range snapshot.Clusters {
		if c.AutoSync != nil && !*c.AutoSync {
			hash = hash*31 + 7
		}
	}
	if len(snapshot.LastReconcile.Status) > 0 {
		hash = hash*31 + uint64(snapshot.LastReconcile.Status[0])
	}
	for _, b := range []byte(snapshot.OmniHealth.Status) {
		hash = hash*31 + uint64(b)
	}
	hash = hash*31 + uint64(len(snapshot.Git.SHA))
	// Include per-resource statuses so a status-only change is detected.
	for _, c := range snapshot.Clusters {
		for _, b := range []byte(c.Status) {
			hash = hash*31 + uint64(b)
		}
		if c.ClusterReady != "" {
			for _, b := range []byte(c.ClusterReady) {
				hash = hash*31 + uint64(b)
			}
		}
		if c.KubernetesAPIReady != "" {
			for _, b := range []byte(c.KubernetesAPIReady) {
				hash = hash*31 + uint64(b)
			}
		}
		if c.ClusterPhase != "" {
			for _, b := range []byte(c.ClusterPhase) {
				hash = hash*31 + uint64(b)
			}
		}
		// Include machine topology so additions/removals during cluster creation
		// are detected and broadcast to the UI without waiting for a full reconcile.
		// Hash the worker group count explicitly so adding/removing an entire
		// worker group (MachineSet) is always detected, even when its machine
		// list happens to be empty.
		hash = hash*31 + uint64(len(c.ControlPlane.Machines))
		hash = hash*31 + uint64(len(c.Workers))
		for _, wg := range c.Workers {
			hash = hash*31 + uint64(len(wg.Machines))
		}
		// Include liveContent length so manifest additions/removals (which don't
		// change status or topology) still trigger a WebSocket broadcast.
		hash = hash*31 + uint64(len(c.LiveContent))
	}
	for _, m := range snapshot.MachineClasses {
		for _, b := range []byte(m.Status) {
			hash = hash*31 + uint64(b)
		}
		if m.AutoSync != nil && *m.AutoSync {
			hash = hash*31 + 1
		}
	}
	return hash
}

// securityHeadersMiddleware adds standard security response headers to every reply.
//
// CSP rationale: the frontend is built and served from the same origin, so
// 'self' is the baseline. Bundled CSS includes inline style attributes from
// the React build, so style-src needs 'unsafe-inline'. WebSocket upgrades use
// the same origin (ws:// or wss://) — covered by 'self' for connect-src.
//
// HSTS is only emitted when the request appears to be HTTPS (direct TLS or
// behind an X-Forwarded-Proto: https proxy) to avoid pinning HSTS on
// development plaintext.
func securityHeadersMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-XSS-Protection", "1; mode=block")
		w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
		w.Header().Set("Content-Security-Policy",
			"default-src 'self'; "+
				"script-src 'self'; "+
				"style-src 'self' 'unsafe-inline'; "+
				"img-src 'self' data:; "+
				"font-src 'self' data:; "+
				"connect-src 'self' ws: wss:; "+
				"frame-ancestors 'none'; "+
				"base-uri 'self'; "+
				"form-action 'self'")
		if isSecure(r) {
			w.Header().Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
		}
		next.ServeHTTP(w, r)
	})
}

// BroadcastState sends current state to all connected WebSocket clients.
func (s *Server) BroadcastState() {
	snapshot := s.appState.Snapshot()
	if data, err := json.Marshal(snapshot); err == nil {
		select {
		case s.broadcast <- data:
		default:
			// Channel full, skip
		}
	}
}
