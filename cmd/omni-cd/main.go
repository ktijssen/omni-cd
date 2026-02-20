package main

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"omni-cd/internal/config"
	"omni-cd/internal/git"
	"omni-cd/internal/omni"
	"omni-cd/internal/reconciler"
	"omni-cd/internal/state"
	"omni-cd/internal/web"
)

// version is set at build time via -ldflags "-X main.version=v1.0.0"
var version = "dev"

var appState *state.AppState

func main() {
	// Load config first to get log level
	cfg, err := config.Load()
	if err != nil {
		// Can't use logInfo yet as slog isn't configured
		fmt.Fprintf(os.Stderr, "Failed to load config: %v\n", err)
		os.Exit(1)
	}

	// Configure slog with JSON handler and configured log level
	logLevel := parseLogLevel(cfg.LogLevel)
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: logLevel,
	})))

	logInfo("Starting OmniCD")
	logInfo("Watching repository", "repo", cfg.GitRepo, "branch", cfg.GitBranch)
	logInfo("Machine classes path", "path", cfg.MCPath)
	logInfo("Cluster templates path", "path", cfg.ClustersPath)
	logInfo("Cluster sync configuration", "enabled", cfg.ClustersEnabled)
	logInfo("Refresh reconcile interval", "interval", cfg.RefreshInterval)
	logInfo("Sync reconcile interval", "interval", cfg.SyncInterval)

	// Verify omnictl connectivity
	if err := omni.CheckConnectivity(); err != nil {
		logInfo("omnictl authentication failed", "error", err)
		os.Exit(1)
	}
	logInfo("omnictl authenticated successfully")

	// Shared state for the web UI
	stateFile := "/data/omni-cd-state.json"
	appState = state.New(500, cfg.OmniEndpoint, cfg.ClustersEnabled, stateFile)
	logDebug("State file configured", "path", stateFile)

	// Fetch and store version info
	omniVersion := omni.GetOmniVersion()
	omnictlVersion := omni.GetOmnictlVersion()
	versionMismatch := omni.CompareVersions(omniVersion, omnictlVersion)
	appState.SetVersions(omniVersion, omnictlVersion, versionMismatch)
	logDebug("Omni version", "version", omniVersion)
	logDebug("omnictl version", "version", omnictlVersion)
	if versionMismatch {
		logError("Version mismatch detected", "omni_version", omniVersion, "omnictl_version", omnictlVersion)
		logError("Sync disabled due to version mismatch")
	}

	// Channels for web UI to trigger reconciles
	triggerHard := make(chan struct{}, 1)
	triggerSoft := make(chan struct{}, 1)

	// Start the web UI server
	webServer := web.New(appState, triggerHard, triggerSoft, cfg.WebPort, version)
	webServer.Start()

	// Set up graceful shutdown
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)

	gitClient := git.New(cfg, appState)
	rec := reconciler.New(appState)

	// pollClusterStatuses fetches live ready-status for every cluster and
	// writes it into AppState.  Defined here so it can be called both from
	// the background ticker AND synchronously at the end of each reconcile
	// (when s.Clusters is guaranteed to be populated).
	pollClusterStatuses := func() {
		statuses, err := omni.GetAllClusterReadyStatuses()
		if err == nil {
			appState.UpdateClusterReadyStatuses(statuses)
		}
	}

	// Start background polling every 5 seconds for live ready-status updates
	// between reconcile cycles.
	go func() {
		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()
		for range ticker.C {
			pollClusterStatuses()
		}
	}()

	// Run immediately on start (hard reconcile)
	doReconcile(gitClient, rec, cfg, true)
	// Poll right after the initial reconcile so ClusterReady is populated
	// as soon as the cluster list exists in state (avoids waiting a full
	// 5-second tick for the first badge to appear).
	go pollClusterStatuses()

	// Start refresh timer after first reconcile completes
	refreshTimer := time.NewTimer(cfg.RefreshInterval)
	syncTicker := time.NewTicker(cfg.SyncInterval)
	defer refreshTimer.Stop()
	defer syncTicker.Stop()

	for {
		select {
		case <-refreshTimer.C:
			doReconcile(gitClient, rec, cfg, false)
			go pollClusterStatuses()
			refreshTimer.Reset(cfg.RefreshInterval)
		case <-syncTicker.C:
			logInfo("Sync reconcile triggered", "trigger", "scheduled")
			doReconcile(gitClient, rec, cfg, true)
			go pollClusterStatuses()
			refreshTimer.Reset(cfg.RefreshInterval)
		case <-triggerHard:
			logInfo("Sync reconcile triggered", "trigger", "web UI")
			doReconcile(gitClient, rec, cfg, true)
			go pollClusterStatuses()
			refreshTimer.Reset(cfg.RefreshInterval)
		case <-triggerSoft:
			logInfo("Git check triggered", "trigger", "web UI")
			doReconcile(gitClient, rec, cfg, false)
			go pollClusterStatuses()
			refreshTimer.Reset(cfg.RefreshInterval)
		case <-stop:
			logInfo("Shutting down gracefully")
			return
		}
	}
}

// doReconcile performs a single git sync + reconcile cycle.
func doReconcile(gitClient *git.Client, rec *reconciler.Reconciler, cfg *config.Config, force bool) {
	// Block everything when version mismatch
	if appState.Snapshot().VersionMismatch {
		logError("All operations disabled due to version mismatch")
		return
	}

	if force {
		logInfo("Reconcile started", "type", "sync")
		appState.SetReconcileStarted(state.ReconcileHard)
	} else {
		logInfo("Reconcile started", "type", "refresh")
		appState.SetReconcileStarted(state.ReconcileSoft)
	}

	// Check Omni connectivity
	if err := omni.CheckConnectivity(); err != nil {
		logError("Omni connectivity check failed", "error", err)
		appState.SetOmniHealth("failed", err.Error())
	} else {
		appState.SetOmniHealth("healthy", "")
	}

	changed, err := gitClient.Sync()
	if err != nil {
		logError("Git sync failed", "error", err)
		appState.SetReconcileFinished(false)
		appState.Save()
		return
	}

	if changed || force {
		repoDir := gitClient.RepoDir()

		// If version mismatch, do nothing
		if appState.Snapshot().VersionMismatch {
			logError("All operations disabled due to version mismatch")
		} else {
			// ---- Apply phase (dependencies first) ----
			// 1. Machine classes must exist before clusters can reference them
			rec.ApplyMachineClasses(repoDir + "/" + cfg.MCPath)

			// 2. Cluster templates (only if enabled or force sync requested)
			if appState.GetClustersEnabled() || appState.HasForceClusterID() {
				rec.ApplyClusters(repoDir + "/" + cfg.ClustersPath)
			} else {
				rec.DiffClusters(repoDir + "/" + cfg.ClustersPath)
			}

			// ---- Delete phase (dependents first) ----
			// 3. Remove clusters before their machine classes (only if enabled)
			if appState.GetClustersEnabled() {
				rec.DeleteClusters(repoDir + "/" + cfg.ClustersPath)
			} else {
				logInfo("Cluster sync disabled, skipping cluster delete")
			}

			// 4. Machine classes can now be safely removed
			rec.DeleteMachineClasses(repoDir + "/" + cfg.MCPath)
		}
	} else {
		// No git change and not a forced reconcile.
		// Still run cluster diff if sync is disabled so we detect drift.
		if !appState.GetClustersEnabled() {
			rec.DiffClusters(gitClient.RepoDir() + "/" + cfg.ClustersPath)
		}
		logDebug("Repository up to date, no reconciliation needed")
	}

	appState.SetReconcileFinished(true)
	appState.Save()
	logInfo("Reconcile finished")
}

func logDebug(msg string, attrs ...any) {
	// Add component as first attribute
	allAttrs := append([]any{"component", "Main"}, attrs...)
	slog.Debug(msg, allAttrs...)

	// Only add to web UI if this level is enabled
	if appState != nil && slog.Default().Enabled(nil, slog.LevelDebug) {
		displayMsg := formatLogMessage("DEBUG", msg, allAttrs...)
		appState.AddLog("DEBUG", "Main", displayMsg)
	}
}

func logInfo(msg string, attrs ...any) {
	// Add component as first attribute
	allAttrs := append([]any{"component", "Main"}, attrs...)
	slog.Info(msg, allAttrs...)

	// Only add to web UI if this level is enabled
	if appState != nil && slog.Default().Enabled(nil, slog.LevelInfo) {
		displayMsg := formatLogMessage("INFO", msg, allAttrs...)
		appState.AddLog("INFO", "Main", displayMsg)
	}
}

func logError(msg string, attrs ...any) {
	// Add component as first attribute
	allAttrs := append([]any{"component", "Main"}, attrs...)
	slog.Error(msg, allAttrs...)

	// Only add to web UI if this level is enabled
	if appState != nil && slog.Default().Enabled(nil, slog.LevelError) {
		displayMsg := formatLogMessage("ERROR", msg, allAttrs...)
		appState.AddLog("ERROR", "Main", displayMsg)
	}
}

// formatLogMessage formats a message with key-value pairs as JSON for display
func formatLogMessage(level, msg string, attrs ...any) string {
	// Build a struct to ensure consistent field order
	type logEntry struct {
		Time  string `json:"time"`
		Level string `json:"level"`
		Msg   string `json:"msg"`
	}

	entry := logEntry{
		Time:  time.Now().UTC().Format(time.RFC3339Nano),
		Level: level,
		Msg:   msg,
	}

	// Start with the base fields
	var jsonParts []string
	baseJSON, _ := json.Marshal(entry)
	baseStr := string(baseJSON)
	// Remove closing brace
	baseStr = baseStr[:len(baseStr)-1]
	jsonParts = append(jsonParts, baseStr)

	// Add all attributes in order
	for i := 0; i < len(attrs); i += 2 {
		if i+1 < len(attrs) {
			key := fmt.Sprint(attrs[i])
			valJSON, _ := json.Marshal(attrs[i+1])
			jsonParts = append(jsonParts, fmt.Sprintf(`"%s":%s`, key, string(valJSON)))
		}
	}

	return strings.Join(jsonParts, ",") + "}"
}

// parseLogLevel converts a string log level to slog.Level
func parseLogLevel(level string) slog.Level {
	switch strings.ToUpper(level) {
	case "DEBUG":
		return slog.LevelDebug
	case "INFO":
		return slog.LevelInfo
	case "WARN", "WARNING":
		return slog.LevelWarn
	case "ERROR":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}
