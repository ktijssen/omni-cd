package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	"omni-cd/internal/auth"
	"omni-cd/internal/config"
	"omni-cd/internal/git"
	oidcconfig "omni-cd/internal/oidc"
	"omni-cd/internal/omni"
	"omni-cd/internal/omniinstance"
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
		fmt.Fprintf(os.Stderr, "Failed to load config: %v\n", err)
		os.Exit(1)
	}

	// Configure slog with JSON handler and configured log level
	logLevel := parseLogLevel(cfg.LogLevel)
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: logLevel,
	})))

	logInfo("Starting OmniCD")
	migrateDataFiles()

	// Load (or create) the auth store. ADMIN_PASSWORD is only applied on first
	// boot (empty store) so a password set via the setup UI is never overwritten.
	authStore, err := auth.New("/data/auth/users.json")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to load auth store: %v\n", err)
		os.Exit(1)
	}
	if cfg.AdminPassword != "" && authStore.IsEmpty() {
		if err := authStore.SetUser("admin", "Admin", cfg.AdminPassword); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to set admin password from ENV: %v\n", err)
			os.Exit(1)
		}
		logInfo("Admin account set from environment variables (first boot)")
	}
	logInfo("Refresh reconcile interval", "interval", cfg.RefreshInterval)

	// Resolve Omni credentials: ENV vars take precedence over the persisted file.
	instanceFile := omniinstance.DefaultPath
	effectiveEndpoint := cfg.OmniEndpoint
	effectiveKey := cfg.OmniServiceAccountKey
	if cfg.OmniEnvLocked {
		logInfo("Omni credentials loaded from environment variables", "endpoint", effectiveEndpoint)
	} else {
		if inst, err := omniinstance.Load(instanceFile); err != nil {
			logError("Failed to load instances.json", "error", err)
		} else if inst != nil {
			effectiveEndpoint = inst.Endpoint
			effectiveKey = inst.ServiceAccountKey
			logInfo("Omni credentials loaded from config file", "endpoint", effectiveEndpoint, "path", instanceFile)
		} else {
			logInfo("No Omni credentials found — waiting for configuration via UI")
		}
	}
	configured := effectiveEndpoint != "" && effectiveKey != ""

	// Shared state for the web UI — always created so the setup UI works
	// even before Omni is configured.
	stateFile := "/data/state/state.json"
	appState = state.New(500, effectiveEndpoint, true, stateFile)
	appState.SetEnvLocked(cfg.OmniEnvLocked)
	appState.SetHasStoredKey(!cfg.OmniEnvLocked && effectiveKey != "")
	appState.LogLevel = strings.ToUpper(cfg.LogLevel)
	logDebug("State file configured", "path", stateFile)

	logDir := "/data/logs"
	appState.SetLogDir(logDir, cfg.LogRetentionDays)
	logInfo("Log rotation configured", "dir", logDir, "retention_days", cfg.LogRetentionDays)

	// Set up repo persistence unconditionally so repos can be saved even before
	// Omni is configured. Without this, SaveRepoConfigs() is a no-op.
	repoFile := "/data/config/repos.json"
	appState.SetRepoFile(repoFile)
	if err := appState.LoadRepoConfigs(); err != nil {
		logError("Failed to load persisted repo configs", "error", err)
	}

	// Channels for web UI to trigger reconciles
	triggerHard := make(chan struct{}, 1)
	triggerSoft := make(chan struct{}, 1)
	triggerRefreshCluster := make(chan string, 1)
	triggerDeleteCluster := make(chan string, 10)
	triggerDeleteMC := make(chan string, 10)
	triggerMCRefresh := make(chan struct{}, 1)
	triggerMCRefreshSingle := make(chan string, 1)
	triggerRepoChange := make(chan struct{}, 1)
	triggerOmniConfigured := make(chan struct{}, 1)

	// Initialise OIDC from environment variables (optional).
	var oidcRT *web.OIDCRuntime
	if cfg.OIDC != nil {
		oidcCfg := &oidcconfig.Config{
			IssuerURL:    cfg.OIDC.IssuerURL,
			ClientID:     cfg.OIDC.ClientID,
			ClientSecret: cfg.OIDC.ClientSecret,
			RedirectURL:  cfg.OIDC.RedirectURL,
			Scopes:       cfg.OIDC.Scopes,
			GroupsClaim:  cfg.OIDC.GroupsClaim,
			AdminGroups:  cfg.OIDC.AdminGroups,
			AdminEmails:  cfg.OIDC.AdminEmails,
			ViewerGroups: cfg.OIDC.ViewerGroups,
			ViewerEmails: cfg.OIDC.ViewerEmails,
			DefaultRole:  cfg.OIDC.DefaultRole,
			Insecure:     cfg.OIDC.Insecure,
		}
		if rt, err := web.InitOIDCRuntime(oidcCfg); err != nil {
			logError("Failed to initialise OIDC provider", "error", err)
		} else {
			oidcRT = rt
			logInfo("OIDC provider initialised", "issuer", oidcCfg.IssuerURL)
		}
	}

	// Start the web UI server early — needed for the setup UI in unconfigured mode.
	webServer := web.New(appState, triggerHard, triggerSoft, triggerRefreshCluster, triggerDeleteCluster, triggerDeleteMC, triggerMCRefresh, triggerMCRefreshSingle, triggerRepoChange, triggerOmniConfigured, instanceFile, logDir, cfg.WebPort, version, authStore, cfg.AuthDisabled, oidcRT)
	webServer.Start()

	// Set up graceful shutdown
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)

	// loadOmniStartupData seeds repo configs from cfg.Repos (if none loaded from
	// disk) and fetches the Omni version after Omni is ready. Repo file setup is
	// done unconditionally above so SaveRepoConfigs() always writes to disk.
	loadOmniStartupData := func() {
		if len(appState.GetRepoConfigs()) == 0 {
			appState.SetRepoConfigs(cfg.Repos)
			if err := appState.SaveRepoConfigs(); err != nil {
				logError("Failed to persist initial repo configs", "error", err)
			}
		}
		omniVersion := omni.GetOmniVersion()
		appState.SetVersions(omniVersion)
		logDebug("Omni version", "version", omniVersion)
	}

	if configured {
		if err := omni.Init(effectiveEndpoint, effectiveKey); err != nil {
			if cfg.OmniEnvLocked {
				logError("Failed to initialise Omni client, retrying in background", "error", err)
				appState.SetOmniHealth("failed", err.Error())
				go retryOmniConnect(effectiveEndpoint, effectiveKey, appState, triggerOmniConfigured)
			} else {
				logError("Failed to initialise Omni client, waiting for UI configuration", "error", err)
			}
			configured = false
		} else if err := omni.CheckConnectivity(); err != nil {
			if cfg.OmniEnvLocked {
				logError("Omni temporarily unavailable, retrying in background", "error", err)
				appState.SetOmniHealth("failed", err.Error())
				go retryOmniConnect(effectiveEndpoint, effectiveKey, appState, triggerOmniConfigured)
			} else {
				logError("Omni connectivity check failed, waiting for UI configuration", "error", err)
				appState.SetOmniHealth("failed", err.Error())
			}
			configured = false
		} else {
			logInfo("Omni authenticated successfully")
			appState.SetOmniConfigured(true)
			appState.SetOmniHealth("healthy", "")
			loadOmniStartupData()
		}
	}

	if !configured {
		logInfo("Omni not configured — waiting for configuration via UI")
		select {
		case <-triggerOmniConfigured:
			// omni.Init was already called by the save handler (UI flow) or by
			// retryOmniConnect (env-locked retry flow).
			loadOmniStartupData()
		case <-stop:
			logInfo("Shutting down gracefully")
			return
		}
	}

	// buildMultiClient constructs a fresh MultiClient from the current repo configs
	// stored in appState.  Always call this inside the reconcile lock or at a quiet
	// moment to avoid racing with an in-flight reconcile.
	buildMultiClient := func() *git.MultiClient {
		rcs := appState.GetRepoConfigs()
		cfgCopy := *cfg
		cfgCopy.Repos = rcs
		return git.NewMulti(&cfgCopy, appState)
	}

	rec := reconciler.New(appState)

	// refreshClusterMu serialises single-cluster refresh goroutines so that
	// concurrent SyncAll() calls cannot race with each other.
	var refreshClusterMu sync.Mutex

	// latestClusterDirs / latestMCDirs hold the most recently reconciled dirs.
	// Updated each reconcile so watchers can trigger targeted refreshes without
	// a full git pull.
	var clusterDirsMu sync.RWMutex
	var latestClusterDirs []string
	setClusterDirs := func(dirs []string) {
		clusterDirsMu.Lock()
		latestClusterDirs = dirs
		clusterDirsMu.Unlock()
	}
	getClusterDirs := func() []string {
		clusterDirsMu.RLock()
		defer clusterDirsMu.RUnlock()
		return latestClusterDirs
	}

	var mcDirsMu sync.RWMutex
	var latestMCDirs []string
	setMCDirs := func(dirs []string) {
		mcDirsMu.Lock()
		latestMCDirs = dirs
		mcDirsMu.Unlock()
	}
	getMCDirs := func() []string {
		mcDirsMu.RLock()
		defer mcDirsMu.RUnlock()
		return latestMCDirs
	}

	// pollClusterStatuses fetches etcd backup status (not covered by the watch)
	// as a 60s safety-net fallback. ClusterStatus and ControlPlaneStatus are
	// streamed in real-time by WatchClusters.
	pollClusterStatuses := func() {
		statuses, err := omni.GetAllClusterReadyStatuses()
		if err == nil && appState.UpdateClusterReadyStatuses(statuses) {
			appState.Save()
		}
	}

	// startWatcher runs WatchClusters with automatic reconnect on failure.
	// Falls back to a 60-second safety-net ticker so no state is permanently missed.
	startWatcher := func() {
		// Safety-net: re-poll every 60s in case the watch misses an event.
		go func() {
			ticker := time.NewTicker(60 * time.Second)
			defer ticker.Stop()
			for range ticker.C {
				pollClusterStatuses()
			}
		}()

		// Dedicated health-check: poll CheckConnectivity frequently when Omni is
		// up (10s) so outages are detected quickly, and less often when it is
		// already down (3m) to avoid hammering an unavailable endpoint.
		go func() {
			for {
				if err := omni.Ping(); err != nil {
					appState.SetOmniHealth("failed", err.Error())
					time.Sleep(3 * time.Minute)
				} else {
					appState.SetOmniHealth("healthy", "")
					time.Sleep(10 * time.Second)
				}
			}
		}()

		// Shared debounce for cluster config changes from any Omni watch source.
		var pendingMu sync.Mutex
		pending := make(map[string]bool)
		var debounceTimer *time.Timer

		flush := func() {
			pendingMu.Lock()
			ids := make([]string, 0, len(pending))
			for id := range pending {
				ids = append(ids, id)
			}
			pending = make(map[string]bool)
			pendingMu.Unlock()

			dirs := getClusterDirs()

			// No repos configured — trigger a soft reconcile so CollectUnmanagedClusters
			// can detect clusters created or modified directly in Omni.
			if len(dirs) == 0 {
				logInfo("Omni cluster change detected, triggering reconcile (no repos configured)")
				select {
				case triggerSoft <- struct{}{}:
				default:
				}
				return
			}

			anySynced := false
			for _, id := range ids {
				if appState.GetClusterAutoSync(id) {
					// Auto-sync enabled: queue a force-sync and trigger a hard reconcile.
					// The reconcile will pull git, apply MCs, then force-apply this cluster.
					logInfo("Omni config change detected, auto-syncing cluster", "cluster", id)
					appState.AddForceClusterID(id)
					anySynced = true
				} else {
					foundDir := ""
					for _, dir := range dirs {
						if _, err := os.Stat(dir + "/" + id); err == nil {
							foundDir = dir
							break
						}
					}
					if foundDir == "" && len(dirs) > 0 {
						foundDir = dirs[0]
					}
					if foundDir != "" {
						logInfo("Omni config change detected, refreshing cluster", "cluster", id)
						go rec.RefreshSingleCluster(foundDir, id)
					}
				}
			}
			if anySynced {
				select {
				case triggerHard <- struct{}{}:
				default:
				}
			}
		}

		addPending := func(clusterID string) {
			pendingMu.Lock()
			pending[clusterID] = true
			if debounceTimer != nil {
				debounceTimer.Reset(2 * time.Second)
			} else {
				debounceTimer = time.AfterFunc(2*time.Second, flush)
			}
			pendingMu.Unlock()
		}

		// WatchClusters: real-time ClusterStatus + ControlPlaneStatus + Cluster phases.
		// Cluster Updated events are forwarded to addPending so config changes made
		// in Omni trigger a targeted refresh without a second Cluster gRPC stream.
		// Stream failures/recoveries update omniHealth directly — but only once
		// omniConfigured is true (i.e. past startup) to avoid racing with the
		// initial doReconcile which is the authoritative source during startup.
		go func() {
			connected := false
			for {
				ctx, cancel := context.WithCancel(context.Background())
				err := omni.WatchClusters(ctx, func(statuses map[string]omni.ClusterStatus, allIDs []string, tearingDown map[string]bool) {
					if !connected {
						connected = true
						if appState.IsOmniConfigured() {
							appState.SetOmniHealth("healthy", "")
						}
					}
					omni.CacheClusterSnapshot(allIDs, tearingDown)
					changed := appState.UpdateClusterReadyStatuses(statuses)
					changed = appState.UpdateTearingDownStatuses(allIDs, tearingDown) || changed
					if changed {
						appState.Save()
					}
				}, addPending)
				cancel()
				if err != nil {
					if connected && appState.IsOmniConfigured() {
						appState.SetOmniHealth("failed", err.Error())
					}
					connected = false
					logError("Cluster watch failed, retrying in 5s", "error", err)
				}
				time.Sleep(5 * time.Second)
			}
		}()

		// WatchClusterConfigChanges: MachineSet changes (Cluster changes handled above).
		go func() {
			for {
				ctx, cancel := context.WithCancel(context.Background())
				err := omni.WatchClusterConfigChanges(ctx, addPending)
				cancel()
				if err != nil {
					logError("Cluster config watch failed, retrying in 5s", "error", err)
				}
				time.Sleep(5 * time.Second)
			}
		}()

		// WatchMachineSetNodes: fires when Omni assigns/removes a machine from a
		// MachineSet (e.g. during cluster creation / pool scaling). Forwarded to
		// addPending so the topology graph updates in real time without waiting
		// for the next full reconcile cycle.
		go func() {
			for {
				ctx, cancel := context.WithCancel(context.Background())
				err := omni.WatchMachineSetNodes(ctx, addPending)
				cancel()
				if err != nil {
					logError("MachineSetNode watch failed, retrying in 5s", "error", err)
				}
				time.Sleep(5 * time.Second)
			}
		}()

		// WatchMachineClasses: maintain in-memory cache of MC id -> YAML content.
		// When content changes (not just bootstrap), apply auto-sync-enabled MCs
		// immediately and diff the rest so the UI reflects the change right away.
		go func() {
			var prevSnapshot map[string]string
			for {
				ctx, cancel := context.WithCancel(context.Background())
				err := omni.WatchMachineClasses(ctx, func(content map[string]string) {
					omni.CacheMachineClassSnapshot(content)

					// Detect real changes (skip the initial bootstrap delivery).
					if prevSnapshot != nil {
						changed := len(content) != len(prevSnapshot)
						if !changed {
							for id, v := range content {
								if prevSnapshot[id] != v {
									changed = true
									break
								}
							}
						}
						if changed {
							// ApplyMachineClasses respects per-MC auto-sync: MCs with
							// auto-sync ON are applied immediately to fix the drift;
							// MCs with auto-sync OFF are recorded as outofsync only.
							// Passing nil for crossRepoDuplicates and forceMCIDs is
							// intentional — this is a reactive apply, not a full reconcile.
							dirs := getMCDirs()
							for _, dir := range dirs {
								go rec.ApplyMachineClasses(dir, nil, nil, false, "", "", "")
							}
						}
					}
					// Copy snapshot for next comparison.
					prevSnapshot = make(map[string]string, len(content))
					for k, v := range content {
						prevSnapshot[k] = v
					}
				})
				cancel()
				if err != nil {
					logError("Machine class watch failed, retrying in 5s", "error", err)
				}
				time.Sleep(5 * time.Second)
			}
		}()
	}
	startWatcher()
	// Run immediately on start (hard reconcile)
	doReconcile(buildMultiClient(), rec, cfg, true, setClusterDirs, setMCDirs)
	// Poll right after the initial reconcile so ClusterReady is populated
	// as soon as the cluster list exists in state (avoids waiting a full
	// 5-second tick for the first badge to appear).
	go pollClusterStatuses()

	// Start refresh timer after first reconcile completes
	refreshTimer := time.NewTimer(cfg.RefreshInterval)
	defer refreshTimer.Stop()

	// reconcileCh serialises reconcile runs: the single-slot buffer ensures at
	// most one reconcile is running and one is queued at any time. Excess
	// triggers are dropped instead of spawning unbounded goroutines.
	reconcileCh := make(chan bool, 1)
	go func() {
		for force := range reconcileCh {
			doReconcile(buildMultiClient(), rec, cfg, force, setClusterDirs, setMCDirs)
			pollClusterStatuses()
		}
	}()

	runReconcile := func(force bool) {
		select {
		case reconcileCh <- force:
		default:
			logInfo("Reconcile already in progress, skipping trigger")
		}
	}

	for {
		select {
		case <-refreshTimer.C:
			refreshTimer.Reset(cfg.RefreshInterval)
			runReconcile(false)
		case <-triggerHard:
			logInfo("Sync reconcile triggered", "trigger", "web UI")
			runReconcile(true)
		case <-triggerSoft:
			logInfo("Git check triggered", "trigger", "web UI")
			runReconcile(false)
		case clusterID := <-triggerRefreshCluster:
			logInfo("Cluster refresh triggered", "trigger", "web UI", "cluster", clusterID)
			go func(id string) {
				if !refreshClusterMu.TryLock() {
					logInfo("Cluster refresh already in progress, skipping", "cluster", id)
					return
				}
				defer refreshClusterMu.Unlock()
				for _, r := range buildMultiClient().SyncAll() {
					if r.Err != nil {
						logError("Git sync failed during cluster refresh", "repo", r.Name, "error", r.Err)
					}
				}
				dirs := getClusterDirs()
				foundDir := ""
				for _, dir := range dirs {
					if _, err := os.Stat(dir + "/" + id); err == nil {
						foundDir = dir
						break
					}
				}
				if foundDir == "" && len(dirs) > 0 {
					foundDir = dirs[0]
				}
				if foundDir != "" {
					rec.RefreshSingleCluster(foundDir, id)
				}
			}(clusterID)
		case clusterID := <-triggerDeleteCluster:
			logInfo("Cluster delete triggered", "trigger", "web UI", "cluster", clusterID)
			go func(id string) {
				rec.DeleteSingleCluster(id)
			}(clusterID)
		case mcID := <-triggerDeleteMC:
			logInfo("Machine class delete triggered", "trigger", "web UI", "machineclass", mcID)
			go func(id string) {
				rec.DeleteSingleMachineClass(id)
				// Re-diff after deletion so the MC immediately shows as Out of Sync
				// (still in Git but no longer in Omni) without waiting for the next
				// scheduled reconcile.
				gc := buildMultiClient()
				results := gc.SyncAll()
				currentRepos := appState.GetRepoConfigs()
				repoByName := make(map[string]config.RepoConfig, len(currentRepos))
				for _, rc := range currentRepos {
					repoByName[rc.Name] = rc
				}
				for _, r := range results {
					if r.Err != nil {
						continue
					}
					rc := repoByName[r.Name]
					rec.DiffMachineClasses(r.RepoDir + "/" + rc.MCPath)
				}
			}(mcID)
		case mcID := <-triggerMCRefreshSingle:
			logInfo("Single machine class refresh triggered", "trigger", "web UI", "machineclass", mcID)
			go func(id string) {
				gc := buildMultiClient()
				results := gc.SyncAll()
				for _, r := range results {
					if r.Err != nil {
						logError("Git sync failed during MC refresh", "repo", r.Name, "error", r.Err)
						return
					}
				}
				currentRepos := appState.GetRepoConfigs()
				repoByNameMC := make(map[string]config.RepoConfig, len(currentRepos))
				for _, rc := range currentRepos {
					repoByNameMC[rc.Name] = rc
				}
				for _, r := range results {
					rc := repoByNameMC[r.Name]
					rec.DiffMachineClasses(r.RepoDir + "/" + rc.MCPath)
				}
			}(mcID)
		case <-triggerMCRefresh:
			logInfo("Machine class refresh triggered", "trigger", "web UI")
			go func() {
				gc := buildMultiClient()
				results := gc.SyncAll()
				for _, r := range results {
					if r.Err != nil {
						logError("Git sync failed during MC refresh", "repo", r.Name, "error", r.Err)
						return
					}
				}
				currentRepos := appState.GetRepoConfigs()
				repoByName := make(map[string]config.RepoConfig, len(currentRepos))
				for _, rc := range currentRepos {
					repoByName[rc.Name] = rc
				}
				for _, r := range results {
					rc := repoByName[r.Name]
					rec.DiffMachineClasses(r.RepoDir + "/" + rc.MCPath)
				}
			}()
		case <-triggerRepoChange:
			logInfo("Repo configuration changed, triggering reconcile", "trigger", "web UI")
			runReconcile(true)
		case <-stop:
			logInfo("Shutting down gracefully")
			return
		}
	}
}

// doReconcile performs a git sync + reconcile cycle across all configured repos.
func doReconcile(gitClient *git.MultiClient, rec *reconciler.Reconciler, cfg *config.Config, force bool, setClusterDirs func([]string), setMCDirs func([]string)) {
	if force {
		logInfo("Reconcile started", "type", "sync")
		appState.SetReconcileStarted(state.ReconcileHard)
	} else {
		logInfo("Reconcile started", "type", "refresh")
		appState.SetReconcileStarted(state.ReconcileSoft)
	}

	// Process repos deleted via the UI — force-delete their managed clusters
	// and machine classes. Runs unconditionally so cleanup is not blocked by
	// git sync errors or an empty repo list.
	if pending := appState.TakePendingRepoDeletes(); len(pending) > 0 {
		for _, rc := range pending {
			workDir := "/tmp/repo-" + rc.Name
			rec.ForceDeleteFromDirs(
				[]string{workDir + "/" + rc.ClustersPath},
				[]string{workDir + "/" + rc.MCPath},
			)
			os.RemoveAll(workDir)
			// Clusters skipped by ForceDeleteFromDirs due to auto-sync being
			// disabled are no longer owned by any repo — mark them orphaned
			// immediately so the UI reflects the correct state right away.
			for _, c := range appState.GetClusters() {
				if c.RepoName == rc.Name && c.AutoSync != nil && !*c.AutoSync {
					appState.UpdateClusterStatus(c.ID, "orphaned")
					logInfo("Cluster marked orphaned after repo deletion (auto-sync disabled)", "component", "Clusters", "cluster", c.ID)
				}
			}
			// Clear the repo→cluster and repo→MC maps so the next reconcile
			// treats those clusters as untracked (orphaned) rather than
			// out-of-sync (which would happen if they remained in allTracked).
			appState.SetRepoClusters(rc.Name, []string{})
			appState.SetRepoMachineClasses(rc.Name, []string{})
		}

		// Prune state entries for clusters and machine classes that were only in
		// deleted repos and never actually applied to Omni (e.g., AutoSync was
		// disabled the entire time so they were git-only drafts). Without this,
		// those entries linger in the UI after the repo is removed because the
		// normal collectUnmanaged pass may be skipped (early return when no repos
		// remain) or may not see them in any git dir.
		if liveClusterIDs, err := omni.GetClusterIDs(); err == nil {
			liveClusterSet := make(map[string]bool, len(liveClusterIDs))
			for _, id := range liveClusterIDs {
				liveClusterSet[id] = true
			}
			allTrackedClusters := appState.GetAllTrackedClusterIDs()
			existing := appState.GetClusters()
			var kept []state.ResourceInfo
			for _, c := range existing {
				// Never prune entries that have been intentionally left behind after
				// repo deletion — they carry the new "orphaned" status the user
				// expects to see. collectUnmanagedClusters will drop them naturally
				// once Omni confirms they are gone.
				if c.Status == "orphaned" || c.Status == "unmanaged" {
					kept = append(kept, c)
					continue
				}
				// Drop entries that were never applied to Omni (not tracked, not live).
				if !allTrackedClusters[c.ID] && !liveClusterSet[c.ID] {
					logInfo("Removing stale cluster entry after repo deletion", "component", "Clusters", "cluster", c.ID)
					continue
				}
				// Drop completed deletions: cluster was mid-delete but is now gone from Omni.
				if c.Status == "deleting" && !liveClusterSet[c.ID] {
					logInfo("Cluster deletion completed, removing from state", "component", "Clusters", "cluster", c.ID)
					continue
				}
				kept = append(kept, c)
			}
			if len(kept) != len(existing) {
				appState.SetClusters(kept)
			}
		}
		if liveMCIDs, err := omni.GetMachineClassIDs(); err == nil {
			liveMCSet := make(map[string]bool, len(liveMCIDs))
			for _, id := range liveMCIDs {
				liveMCSet[id] = true
			}
			allTrackedMCs := appState.GetAllTrackedMachineClassIDs()
			existingMCs := appState.GetMachineClasses()
			var keptMCs []state.ResourceInfo
			for _, mc := range existingMCs {
				// Never prune entries intentionally left behind as "unmanaged".
				if mc.Status == "unmanaged" {
					keptMCs = append(keptMCs, mc)
					continue
				}
				if !allTrackedMCs[mc.ID] && !liveMCSet[mc.ID] {
					logInfo("Removing stale machine class entry after repo deletion", "component", "MachineClasses", "id", mc.ID)
					continue
				}
				keptMCs = append(keptMCs, mc)
			}
			if len(keptMCs) != len(existingMCs) {
				appState.SetMachineClasses(keptMCs)
			}
		}
	}

	// Check Omni connectivity
	if err := omni.CheckConnectivity(); err != nil {
		logError("Omni connectivity check failed", "error", err)
		appState.SetOmniHealth("failed", err.Error())
	} else {
		appState.SetOmniHealth("healthy", "")
	}

	// No repos configured — still discover unmanaged resources from Omni so the
	// UI shows what exists, marked as unmanaged.
	if len(appState.GetRepoConfigs()) == 0 {
		logInfo("No repositories configured, collecting unmanaged resources from Omni")
		appState.ClearRepoMaps()
		liveClusterIDs, _, _ := omni.GetCachedClusterIDsWithPhases()
		if len(liveClusterIDs) == 0 {
			if ids, err := omni.GetClusterIDs(); err == nil {
				liveClusterIDs = ids
			}
		}
		rec.CollectUnmanagedClusters(nil, liveClusterIDs)
		liveMCStates, _ := omni.GetCachedMachineClasses()
		rec.CollectUnmanagedMachineClasses(nil, liveMCStates)
		appState.SetReconcileFinished(true)
		appState.Save()
		logInfo("Reconcile finished")
		return
	}

	results := gitClient.SyncAll()

	// Log errors for failed repos but do not abort — healthy repos should still
	// be reconciled. Failed repos already have SyncError set in state so the UI
	// shows the 'connecting' badge and the clusters show 'refreshing'.
	var okResults []git.RepoSyncResult
	var protectedClusterIDs, protectedMCIDs []string
	for _, r := range results {
		if r.Err != nil {
			logError("Git sync failed, skipping repo for this cycle", "repo", r.Name, "error", r.Err)
			// Collect last-known resources for this repo so the delete/collect
			// pass treats them as "still in git" and doesn't delete or mark them
			// out-of-sync while the repo is temporarily unreachable.
			protectedClusterIDs = append(protectedClusterIDs, appState.GetRepoClusters(r.Name)...)
			protectedMCIDs = append(protectedMCIDs, appState.GetRepoMachineClasses(r.Name)...)
			// Ensure repoName is set on each cluster so the UI shows 'refreshing'.
			appState.StampRepoNameOnClusters(r.Name, appState.GetRepoClusters(r.Name))
		} else {
			okResults = append(okResults, r)
		}
	}
	rec.SetProtectedResources(protectedClusterIDs, protectedMCIDs)

	// If every repo failed there is nothing to reconcile.
	if len(okResults) == 0 {
		logError("All git syncs failed, skipping reconcile")
		appState.SetReconcileFinished(false)
		appState.Save()
		return
	}

	// Check if any (successful) repo changed, or a force-sync was requested.
	forceClusterIDs := appState.GetAndClearForceClusterIDs()
	forceMCIDs := appState.GetAndClearForceMCIDs()
	anyChanged := force || len(forceClusterIDs) > 0 || len(forceMCIDs) > 0
	for _, r := range okResults {
		if r.Changed {
			anyChanged = true
		}
	}

	// Build per-repo RepoConfig lookup using current live configs from state
	currentRepos := appState.GetRepoConfigs()
	repoByName := make(map[string]config.RepoConfig, len(currentRepos))
	for _, rc := range currentRepos {
		repoByName[rc.Name] = rc
	}

	// Build directory lists for all ok repos — needed for both the changed path
	// and the unconditional unmanaged-detection pass below.
	var allClusterDirs, allMCDirs []string
	for _, r := range okResults {
		rc := repoByName[r.Name]
		allClusterDirs = append(allClusterDirs, r.RepoDir+"/"+rc.ClustersPath)
		allMCDirs = append(allMCDirs, r.RepoDir+"/"+rc.MCPath)
	}
	// Publish dirs so the Omni config-change watcher can trigger targeted refreshes.
	setClusterDirs(allClusterDirs)
	setMCDirs(allMCDirs)

	// Pre-compute cross-repo duplicate maps so each apply call can reject
	// resources that are defined in more than one repository.
	crossRepoDuplicates := rec.DetectCrossRepoDuplicates(allClusterDirs)
	crossRepoDuplicatesMC := rec.DetectCrossRepoDuplicatesMC(allMCDirs)

	// Fetch live cluster IDs once — reused by delete and unmanaged passes.
	// Use the watch cache when available to avoid an API call.
	liveClusterIDs, _, cacheOK := omni.GetCachedClusterIDsWithPhases()
	if !cacheOK {
		liveClusterIDs, _ = omni.GetClusterIDs()
	}

	if anyChanged {
		// ---- Apply phase (dependencies first) ----
		// 1. Machine classes must exist before clusters can reference them
		for _, r := range okResults {
			rc := repoByName[r.Name]
			rec.ApplyMachineClasses(r.RepoDir+"/"+rc.MCPath, crossRepoDuplicatesMC, forceMCIDs, force, r.Info.SHA, r.Info.CommitAuthor, r.Info.CommitMessage)
		}

		// 2. Cluster templates — per-cluster AutoSync controls apply vs diff-only
		for _, r := range okResults {
			rc := repoByName[r.Name]
			rec.ApplyClusters(r.RepoDir+"/"+rc.ClustersPath, forceClusterIDs, crossRepoDuplicates, r.Info.SHA, r.Info.CommitAuthor, r.Info.CommitMessage)
		}

		// Checkpoint: persist state immediately after all clusters have been
		// applied so a crash before the next full save doesn't lose status.
		appState.Save()

		// ---- Delete phase (dependents first, across all repos combined) ----

		// 3. Remove clusters not in any Git repo
		rec.DeleteClustersAll(allClusterDirs, liveClusterIDs)

	} else {
		logDebug("Repository up to date, running diff-only pass")
		for _, r := range okResults {
			rc := repoByName[r.Name]
			rec.DiffMachineClasses(r.RepoDir + "/" + rc.MCPath)
		}
		for _, r := range okResults {
			rc := repoByName[r.Name]
			rec.DiffClusters(r.RepoDir + "/" + rc.ClustersPath)
		}
	}

	// Always discover unmanaged/out-of-sync resources, even when git has not
	// changed, so that clusters and machine classes created directly in Omni
	// appear in the UI on the very next reconcile cycle.

	// 5. Collect unmanaged/out-of-sync clusters across ALL repos in one pass.
	//    Must be after ApplyClusters for all repos so no repo's clusters are
	//    incorrectly flagged as unmanaged by a single-repo view.
	//    Re-read the cache after apply so newly created clusters (picked up by
	//    the watch) are included and not incorrectly flagged as externally deleted.
	if anyChanged {
		if freshIDs, _, ok := omni.GetCachedClusterIDsWithPhases(); ok {
			liveClusterIDs = freshIDs
		} else if freshIDs, err := omni.GetClusterIDs(); err == nil {
			liveClusterIDs = freshIDs
		}
	}
	rec.CollectUnmanagedClusters(allClusterDirs, liveClusterIDs)

	// 6. Same treatment for machine classes.
	liveMCStates, _ := omni.GetCachedMachineClasses()
	rec.CollectUnmanagedMachineClasses(allMCDirs, liveMCStates)

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

// retryOmniConnect retries Init+CheckConnectivity every 10 seconds until Omni
// is reachable, then signals main() to proceed via triggerOmniConfigured.
// It is only used for the env-locked startup path where the app must not exit
// just because Omni is temporarily unavailable.
func retryOmniConnect(endpoint, key string, appState *state.AppState, trigger chan struct{}) {
	for {
		time.Sleep(10 * time.Second)
		if err := omni.Init(endpoint, key); err != nil {
			logError("Omni init retry failed", "error", err)
			appState.SetOmniHealth("failed", err.Error())
			continue
		}
		if err := omni.CheckConnectivity(); err != nil {
			logError("Omni connectivity retry failed", "error", err)
			appState.SetOmniHealth("failed", err.Error())
			continue
		}
		logInfo("Omni connectivity restored")
		appState.SetOmniConfigured(true)
		appState.SetOmniHealth("healthy", "")
		select {
		case trigger <- struct{}{}:
		default:
		}
		return
	}
}

// migrateDataFiles moves files from the old flat /data/ layout to the new
// structured layout. Only moves a file when the old path exists and the new
// path does not, so it is safe to run on every startup.
func migrateDataFiles() {
	migrations := []struct{ old, new string }{
		{"/data/omni-cd-state.json", "/data/state/state.json"},
		{"/data/omni-instance.json", "/data/config/instances.json"},
		{"/data/repos.json", "/data/config/repos.json"},
	}
	for _, m := range migrations {
		if _, err := os.Stat(m.old); os.IsNotExist(err) {
			continue // old file not present, nothing to migrate
		}
		if _, err := os.Stat(m.new); err == nil {
			continue // new file already exists, skip
		}
		if err := os.MkdirAll(filepath.Dir(m.new), 0750); err != nil {
			logError("Migration: failed to create directory", "path", filepath.Dir(m.new), "error", err)
			continue
		}
		if err := os.Rename(m.old, m.new); err != nil {
			logError("Migration: failed to move file", "from", m.old, "to", m.new, "error", err)
			continue
		}
		logInfo("Migration: moved file to new location", "from", m.old, "to", m.new)
	}
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
