package reconciler

import (
	"fmt"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"omni-cd/internal/omni"
	"omni-cd/internal/state"
)

// ============================================================
// Clusters — Apply
// ============================================================

// ApplyClusters validates and syncs all cluster templates from the given directory.
// Each subdirectory must contain a cluster.yaml file. Templates that fail validation
// are skipped, leaving the existing cluster intact.
// Only syncs when there is an actual diff to avoid unnecessary updates.
// forceClusterIDs is passed in by the caller (read once before iterating repos)
// so it is not consumed/cleared on the first repo and lost for subsequent ones.
// crossRepoDuplicates maps cluster IDs that are defined in >1 repo to a description
// of the conflicting repo names; those clusters are marked outofsync and skipped.
func (r *Reconciler) ApplyClusters(dir string, forceClusterIDs map[string]bool, crossRepoDuplicates map[string]string, repoSHA, repoAuthor, repoMessage string) {
	// Derive the repo name from the work-dir convention /tmp/repo-<name>/...
	repoName := repoNameFromDir(dir)

	templates, err := findClusterTemplates(dir)
	if err != nil {
		r.logWarn("Directory not found, skipping", "component", "Clusters", "path", dir)
		return
	}

	if len(templates) == 0 {
		r.logWarn("No cluster templates found", "component", "Clusters")
		return
	}

	if len(forceClusterIDs) > 0 {
		// Only proceed if at least one forced ID exists in this repo.
		anyInRepo := false
		var inThisRepo []string
		for _, tmpl := range templates {
			if name := extractClusterName(tmpl); forceClusterIDs[name] {
				anyInRepo = true
				inThisRepo = append(inThisRepo, name)
			}
		}
		if !anyInRepo {
			// None of the forced clusters are in this repo — skip silently.
			return
		}
		r.logInfo("Force syncing clusters", "component", "Clusters", "clusters", strings.Join(inThisRepo, ", "))
	} else {
		r.logInfo("Syncing cluster templates", "component", "Clusters", "count", len(templates))
	}

	// Batch fetch all live cluster states once
	allLiveStates, _ := omni.GetAllLiveClusters()
	allHostnames, _ := omni.GetAllMachineHostnames()

	var (
		mu        sync.Mutex
		wg        sync.WaitGroup
		resources []state.ResourceInfo
		synced    int
		failed    int
	)

	// Detect duplicate cluster IDs: first within this repo, then across repos.
	nameToFiles := make(map[string][]string)
	for _, tmpl := range templates {
		name := extractClusterName(tmpl)
		if name != "" {
			nameToFiles[name] = append(nameToFiles[name], tmpl)
		}
	}
	duplicates := make(map[string]bool)
	for name, files := range nameToFiles {
		if len(files) > 1 {
			duplicates[name] = true
			relFiles := make([]string, len(files))
			repoRoot := filepath.Dir(dir)
			for i, f := range files {
				relFiles[i] = strings.TrimPrefix(f, repoRoot)
			}
			errMsg := fmt.Sprintf("Conflicting cluster templates: %s", strings.Join(relFiles, ", "))
			r.logError("Duplicate cluster ID found, skipping sync", "component", "Clusters", "cluster", name, "files", strings.Join(relFiles, ", "))
			r.state.UpsertClusterStatus(name, "failed")
			liveContent := allLiveStates[name]
			talos, k8s, cp, wk, clusterExts, machExts := clusterDetailFromLive(liveContent)
			cp, wk = hydratePoolMachines(name, cp, wk)
			resources = append(resources, state.ResourceInfo{
				ID:                name,
				Type:              "Cluster",
				Status:            "failed",
				Error:             errMsg,
				LiveContent:       liveContent,
				TalosVersion:      talos,
				KubernetesVersion: k8s,
				ControlPlane:      cp,
				Workers:           wk,
				ClusterExtensions: clusterExts,
				MachineExtensions: machExts,
				MachineHostnames:  extractHostnames(allHostnames, cp, wk),
			})
			failed++
		}
	}
	// Block clusters that are also defined in another repo.
	for id, repos := range crossRepoDuplicates {
		if _, alreadyDupe := duplicates[id]; alreadyDupe {
			continue // already handled by within-repo detection above
		}
		// Check if this cluster is in this repo.
		if _, inThisRepo := nameToFiles[id]; !inThisRepo {
			continue
		}
		duplicates[id] = true
		errMsg := fmt.Sprintf("Conflict: cluster '%s' is managed by multiple repositories (%s) — remove it from all but one to resolve", id, repos)
		r.logError("Cross-repo cluster name conflict, skipping sync", "component", "Clusters", "cluster", id, "repos", repos)
		r.state.UpsertClusterStatus(id, "failed")
		liveContent := allLiveStates[id]
		talos, k8s, cp, wk, clusterExts, machExts := clusterDetailFromLive(liveContent)
		cp, wk = hydratePoolMachines(id, cp, wk)
		resources = append(resources, state.ResourceInfo{
			ID:                id,
			Type:              "Cluster",
			Status:            "failed",
			Error:             errMsg,
			LiveContent:       liveContent,
			TalosVersion:      talos,
			KubernetesVersion: k8s,
			ControlPlane:      cp,
			Workers:           wk,
			ClusterExtensions: clusterExts,
			MachineExtensions: machExts,
			MachineHostnames:  extractHostnames(allHostnames, cp, wk),
		})
		failed++
	}

	for _, tmpl := range templates {
		name := extractClusterName(tmpl)
		if name == "" {
			r.logWarn("No cluster name found in template, skipping", "component", "Clusters", "file", tmpl)
			continue
		}

		// Skip clusters with conflicting templates
		if duplicates[name] {
			continue
		}

		// If force-syncing specific clusters, skip any that are not in the set
		if len(forceClusterIDs) > 0 && !forceClusterIDs[name] {
			continue
		}

		wg.Add(1)
		go func(tmplPath, clusterName string) {
			defer wg.Done()

			// Read file content for UI display
			fileContent := readFileContent(tmplPath)

			// Validate the template before syncing to prevent broken configs
			if err := omni.ClusterTemplateValidate(tmplPath); err != nil {
				r.logError("Cluster template validation failed", "component", "Clusters", "cluster", clusterName, "error", err)
				r.state.UpsertClusterStatus(clusterName, "failed")
				mu.Lock()
				resources = append(resources, state.ResourceInfo{
					ID:          clusterName,
					Type:        "Cluster",
					Status:      "failed",
					FileContent: fileContent,
					Error:       err.Error(),
				})
				failed++
				mu.Unlock()
				return
			}

			// Check if there are any changes to apply
			diffOutput, _ := omni.ClusterTemplateDiff(tmplPath)
			isForceSync := forceClusterIDs[clusterName]

			if !isForceSync && (diffOutput == "" || strings.Contains(diffOutput, "no changes")) {
				r.logDebug("Cluster up to date", "component", "Clusters", "cluster", clusterName)
				liveContent := allLiveStates[clusterName]
				if liveContent == "" {
					liveContent, _ = omni.GetLiveCluster(clusterName)
				}
				talos, k8s, cp, wk, clusterExts, machExts := clusterDetailFromLive(liveContent)
				cp, wk = hydratePoolMachines(clusterName, cp, wk)
				mu.Lock()
				resources = append(resources, state.ResourceInfo{
					ID:                clusterName,
					Type:              "Cluster",
					Status:            "success",
					FileContent:       fileContent,
					LiveContent:       liveContent,
					TalosVersion:      talos,
					KubernetesVersion: k8s,
					ControlPlane:      cp,
					Workers:           wk,
					ClusterExtensions: clusterExts,
					MachineExtensions: machExts,
					MachineHostnames:  extractHostnames(allHostnames, cp, wk),
				})
				mu.Unlock()
				return
			}

			// Per-cluster auto-sync: if disabled and this is not a manual force-sync,
			// record the diff as out-of-sync but do NOT apply it.
			if !isForceSync && !r.state.GetClusterAutoSync(clusterName) {
				r.logInfo("Cluster out of sync (auto sync disabled, use Sync button to apply)", "component", "Clusters", "cluster", clusterName)
				r.state.UpsertClusterStatus(clusterName, "outofsync")
				liveContentAS := allLiveStates[clusterName]
				if liveContentAS == "" {
					liveContentAS, _ = omni.GetLiveCluster(clusterName)
				}
				talosAS, k8sAS, cpAS, wkAS, clusterExtsAS, machExtsAS := clusterDetailFromLive(liveContentAS)
				cpAS, wkAS = hydratePoolMachines(clusterName, cpAS, wkAS)
				mu.Lock()
				resources = append(resources, state.ResourceInfo{
					ID:                clusterName,
					Type:              "Cluster",
					Status:            "outofsync",
					Diff:              diffOutput,
					FileContent:       fileContent,
					LiveContent:       liveContentAS,
					TalosVersion:      talosAS,
					KubernetesVersion: k8sAS,
					ControlPlane:      cpAS,
					Workers:           wkAS,
					ClusterExtensions: clusterExtsAS,
					MachineExtensions: machExtsAS,
					MachineHostnames:  extractHostnames(allHostnames, cpAS, wkAS),
				})
				mu.Unlock()
				return
			}

			// There is a diff or force sync — log it and sync
			if isForceSync {
				r.logInfo("Force syncing cluster", "component", "Clusters", "cluster", clusterName)
			} else {
				r.logInfo("Cluster out of sync", "component", "Clusters", "cluster", clusterName)
				r.state.UpsertClusterStatus(clusterName, "outofsync")
			}
			r.logInfo("Syncing cluster", "component", "Clusters", "cluster", clusterName)
			r.state.UpsertClusterStatus(clusterName, "syncing")

			if err := omni.ClusterTemplateSync(tmplPath); err != nil {
				r.logError("Cluster sync failed", "component", "Clusters", "cluster", clusterName, "error", err)
				r.state.UpsertClusterStatus(clusterName, "failed")
				liveContent := allLiveStates[clusterName]
				if liveContent == "" {
					liveContent, _ = omni.GetLiveCluster(clusterName)
				}
				talos, k8s, cp, wk, clusterExts, machExts := clusterDetailFromLive(liveContent)
				cp, wk = hydratePoolMachines(clusterName, cp, wk)
				mu.Lock()
				resources = append(resources, state.ResourceInfo{
					ID:                clusterName,
					Type:              "Cluster",
					Status:            "failed",
					Diff:              diffOutput,
					FileContent:       fileContent,
					LiveContent:       liveContent,
					Error:             err.Error(),
					LastSyncResult:    "failed",
					LastSyncError:     err.Error(),
					LastSyncTime:      time.Now().UTC(),
					LastSyncSHA:       repoSHA,
					LastSyncAuthor:    repoAuthor,
					LastSyncMessage:   repoMessage,
					TalosVersion:      talos,
					KubernetesVersion: k8s,
					ControlPlane:      cp,
					Workers:           wk,
					ClusterExtensions: clusterExts,
					MachineExtensions: machExts,
					MachineHostnames:  extractHostnames(allHostnames, cp, wk),
				})
				failed++
				mu.Unlock()
			} else {
				r.logInfo("Cluster synced", "component", "Clusters", "cluster", clusterName)
				r.state.UpsertClusterStatus(clusterName, "success")
				// Always fetch fresh after sync — the pre-fetched cache is stale
				liveContent, _ := omni.GetLiveCluster(clusterName)
				talos, k8s, cp, wk, clusterExts, machExts := clusterDetailFromLive(liveContent)
				cp, wk = hydratePoolMachines(clusterName, cp, wk)
				mu.Lock()
				resources = append(resources, state.ResourceInfo{
					ID:                clusterName,
					Type:              "Cluster",
					Status:            "success",
					FileContent:       fileContent,
					LiveContent:       liveContent,
					TalosVersion:      talos,
					KubernetesVersion: k8s,
					ControlPlane:      cp,
					Workers:           wk,
					ClusterExtensions: clusterExts,
					MachineExtensions: machExts,
					MachineHostnames:  extractHostnames(allHostnames, cp, wk),
					LastSyncResult:    "ok",
					LastSyncTime:      time.Now().UTC(),
					LastSyncSHA:       repoSHA,
					LastSyncAuthor:    repoAuthor,
					LastSyncMessage:   repoMessage,
				})
				synced++
				mu.Unlock()
			}
		}(tmpl, name)
	}

	wg.Wait()

	// Stamp the source repo name on every resource produced by this apply pass
	// so the UI can correlate clusters with their git repo's sync health.
	// Conflicted clusters (duplicates map) belong to multiple repos — leave
	// their RepoName empty so no single repo is shown as the owner.
	if repoName != "" {
		var clusterIDs []string
		for i := range resources {
			if !duplicates[resources[i].ID] {
				resources[i].RepoName = repoName
				clusterIDs = append(clusterIDs, resources[i].ID)
			}
		}
		// Persist the repo→cluster mapping so it survives future sync failures.
		r.state.SetRepoClusters(repoName, clusterIDs)
	}

	// Always merge with existing cluster states to preserve unmanaged clusters
	existing := r.state.GetClusters()

	// Create a map of updated clusters for quick lookup
	updatedMap := make(map[string]state.ResourceInfo)
	for _, res := range resources {
		updatedMap[res.ID] = res
	}

	// Build the final list
	final := make([]state.ResourceInfo, 0, len(existing)+len(resources))
	processedIDs := make(map[string]bool)

	// First, update existing clusters
	for _, existingCluster := range existing {
		if updated, found := updatedMap[existingCluster.ID]; found {
			// Preserve last sync result/time/SHA if this refresh didn't actually sync
			if updated.LastSyncResult == "" {
				updated.LastSyncResult = existingCluster.LastSyncResult
				updated.LastSyncError = existingCluster.LastSyncError
				updated.LastSyncTime = existingCluster.LastSyncTime
				updated.LastSyncSHA = existingCluster.LastSyncSHA
				updated.LastSyncAuthor = existingCluster.LastSyncAuthor
				updated.LastSyncMessage = existingCluster.LastSyncMessage
			}
			if updated.CreatedAt.IsZero() {
				updated.CreatedAt = omni.GetClusterCreatedAt(updated.ID)
			}
			// Track when the cluster entered outofsync — preserve if already set, stamp if newly entered
			if updated.Status == "outofsync" {
				if existingCluster.Status == "outofsync" && !existingCluster.SyncStatusSince.IsZero() {
					updated.SyncStatusSince = existingCluster.SyncStatusSince
				} else {
					updated.SyncStatusSince = time.Now().UTC()
				}
			}
			// This cluster was processed, use the new state
			final = append(final, updated)
			processedIDs[updated.ID] = true
		} else if len(forceClusterIDs) == 0 {
			// Not force-syncing, preserve all existing clusters
			final = append(final, existingCluster)
			processedIDs[existingCluster.ID] = true
		} else if !forceClusterIDs[existingCluster.ID] {
			// Force-syncing specific clusters — preserve those not in the target set
			final = append(final, existingCluster)
			processedIDs[existingCluster.ID] = true
		}
	}

	// Add any new clusters that weren't in existing state
	for _, res := range resources {
		if !processedIDs[res.ID] {
			final = append(final, res)
		}
	}

	r.state.SetClusters(final)

	if len(forceClusterIDs) > 0 {
		r.logInfo("Force sync complete", "component", "Clusters", "synced", synced, "failed", failed)
	} else {
		r.logInfo("Cluster apply result", "component", "Clusters", "synced", synced, "failed", failed)
	}
	// NOTE: Save() is intentionally omitted here. doReconcile checkpoints state
	// after all repos are processed, avoiding N disk writes per cycle.
}

// ============================================================
// Clusters — Single Cluster Refresh (git diff only)
// ============================================================

// RefreshSingleCluster re-reads and diffs one cluster template from disk without
// touching any other clusters or machine classes. It does not sync (apply) any
// changes — it only updates the cluster's display state (status, diff, live content).
func (r *Reconciler) RefreshSingleCluster(dir, id string) {
	// Skip clusters whose deletion is actively in progress in this process.
	// This covers the window between RemoveCluster() and omni.DeleteCluster()
	// returning, during which Omni may not yet report the cluster as TearingDown.
	if _, pending := r.pendingDeletes.Load(id); pending {
		r.logDebug("Skipping refresh: cluster delete in progress", "component", "Clusters", "cluster", id)
		return
	}

	// Always skip clusters currently tearing down in Omni, regardless of what
	// our local state says. This prevents watcher events fired during teardown
	// (e.g. MachineSet Destroyed) from re-adding a deleted cluster to the UI —
	// especially important for orphaned clusters that were removed from state
	// before the Omni delete call returned.
	if omni.IsClusterTearingDown(id) {
		r.logDebug("Skipping refresh: cluster is tearing down in Omni", "component", "Clusters", "cluster", id)
		return
	}

	// If local state says deleting but Omni no longer reports teardown, the
	// delete may have silently failed or the cluster was recreated. Clear the
	// local deleting status so the refresh can re-evaluate correctly.
	for _, c := range r.state.GetClusters() {
		if c.ID == id && c.Status == "deleting" {
			r.logInfo("Cluster no longer tearing down in Omni, resuming refresh", "component", "Clusters", "cluster", id)
			r.state.UpdateClusterStatus(id, "outofsync")
		}
	}

	templates, err := findClusterTemplates(dir)
	if err != nil {
		r.logWarn("Directory not found, skipping", "component", "Clusters", "path", dir)
		return
	}

	// Find the template for this cluster
	var tmplPath string
	for _, t := range templates {
		if extractClusterName(t) == id {
			tmplPath = t
			break
		}
	}

	if tmplPath == "" {
		// No git template — check if this is an unmanaged cluster and refresh its live data.
		if omni.IsClusterTemplateManaged(id) {
			r.logWarn("No template found for managed cluster", "component", "Clusters", "cluster", id)
			r.state.UpsertClusterStatus(id, "orphaned")
			return
		}
		// Unmanaged cluster — populate live info.
		r.logInfo("Refreshing unmanaged cluster from Omni", "component", "Clusters", "cluster", id)
		liveContent, err := omni.GetLiveCluster(id)
		if err != nil {
			r.logError("Failed to fetch live cluster data", "component", "Clusters", "cluster", id, "error", err)
			return
		}
		talos, k8s, cp, wk, clusterExts, machExts := clusterDetailFromLive(liveContent)
		cp, wk = hydratePoolMachines(id, cp, wk)
		allHostnames, _ := omni.GetAllMachineHostnames()
		r.state.UpsertClusterInfo(id, state.ResourceInfo{
			ID:                id,
			Type:              "Cluster",
			Status:            "unmanaged",
			LiveContent:       liveContent,
			TalosVersion:      talos,
			KubernetesVersion: k8s,
			ControlPlane:      cp,
			Workers:           wk,
			ClusterExtensions: clusterExts,
			MachineExtensions: machExts,
			MachineHostnames:  extractHostnames(allHostnames, cp, wk),
		})
		r.state.Save()
		return
	}

	r.logInfo("Refreshing cluster from git", "component", "Clusters", "cluster", id)

	fileContent := readFileContent(tmplPath)

	if err := omni.ClusterTemplateValidate(tmplPath); err != nil {
		r.logError("Cluster template validation failed", "component", "Clusters", "cluster", id, "error", err)
		liveContent, _ := omni.GetLiveCluster(id)
		talos, k8s, cp, wk, clusterExts, machExts := clusterDetailFromLive(liveContent)
		cp, wk = hydratePoolMachines(id, cp, wk)
		allHostnames, _ := omni.GetAllMachineHostnames()
		r.state.UpsertClusterInfo(id, state.ResourceInfo{
			ID:                id,
			Type:              "Cluster",
			Status:            "failed",
			FileContent:       fileContent,
			LiveContent:       liveContent,
			Error:             err.Error(),
			TalosVersion:      talos,
			KubernetesVersion: k8s,
			ControlPlane:      cp,
			Workers:           wk,
			ClusterExtensions: clusterExts,
			MachineExtensions: machExts,
			MachineHostnames:  extractHostnames(allHostnames, cp, wk),
		})
		r.state.Save()
		return
	}

	diffOutput, _ := omni.ClusterTemplateDiff(tmplPath)
	liveContent, _ := omni.GetLiveCluster(id)
	talos, k8s, cp, wk, clusterExts, machExts := clusterDetailFromLive(liveContent)
	cp, wk = hydratePoolMachines(id, cp, wk)
	allHostnames, _ := omni.GetAllMachineHostnames()

	status := "success"
	if diffOutput != "" && !strings.Contains(diffOutput, "no changes") {
		status = "outofsync"
		r.logInfo("Cluster out of sync", "component", "Clusters", "cluster", id)
	} else {
		r.logDebug("Cluster in sync", "component", "Clusters", "cluster", id)
	}

	// If the cluster was previously in a failed state due to a sync error (not a
	// validation error — that is handled above) and the diff is still non-empty,
	// preserve the failed status and error so the user can see why it failed.
	// The state will self-heal to "success" on the next full reconcile if the
	// underlying issue is resolved.
	lastSyncResult := ""
	lastSyncError := ""
	var lastSyncTime time.Time
	lastSyncSHA := ""
	lastSyncAuthor := ""
	lastSyncMessage := ""
	var syncStatusSince time.Time
	for _, c := range r.state.GetClusters() {
		if c.ID == id {
			lastSyncResult = c.LastSyncResult
			lastSyncError = c.LastSyncError
			lastSyncTime = c.LastSyncTime
			lastSyncSHA = c.LastSyncSHA
			lastSyncAuthor = c.LastSyncAuthor
			lastSyncMessage = c.LastSyncMessage
			if c.Status == "outofsync" {
				syncStatusSince = c.SyncStatusSince
			}
			if status == "outofsync" && c.Status == "failed" && c.LastSyncError != "" {
				status = "failed"
				r.logWarn("Preserving failed status from previous sync error", "component", "Clusters", "cluster", id)
			}
			break
		}
	}

	r.state.UpsertClusterInfo(id, state.ResourceInfo{
		ID:                id,
		Type:              "Cluster",
		Status:            status,
		Diff:              diffOutput,
		FileContent:       fileContent,
		LiveContent:       liveContent,
		LastSyncResult:    lastSyncResult,
		LastSyncError:     lastSyncError,
		LastSyncTime:      lastSyncTime,
		LastSyncSHA:       lastSyncSHA,
		LastSyncAuthor:    lastSyncAuthor,
		LastSyncMessage:   lastSyncMessage,
		SyncStatusSince:    func() time.Time {
			if status != "outofsync" {
				return time.Time{}
			}
			if !syncStatusSince.IsZero() {
				return syncStatusSince
			}
			return time.Now().UTC()
		}(),
		TalosVersion:      talos,
		KubernetesVersion: k8s,
		ControlPlane:      cp,
		Workers:           wk,
		ClusterExtensions: clusterExts,
		MachineExtensions: machExts,
		MachineHostnames:  extractHostnames(allHostnames, cp, wk),
		CreatedAt:         omni.GetClusterCreatedAt(id),
	})
	r.state.Save()
}

// ============================================================
// Clusters — Diff Only (when sync is disabled)
// ============================================================

// DiffClusters runs validate + diff on all cluster templates without syncing.
// Clusters with diffs are reported as "outofsync" in the state.
// This allows operators to see drift even when sync is disabled.
func (r *Reconciler) DiffClusters(dir string) {
	templates, err := findClusterTemplates(dir)
	if err != nil {
		r.logWarn("Directory not found, skipping", "component", "Clusters", "path", dir)
		// Still need to collect unmanaged clusters even if directory doesn't exist
		r.collectUnmanagedClusters([]string{dir}, nil)
		return
	}
	if len(templates) == 0 {
		r.logWarn("No cluster templates found", "component", "Clusters")
		// Still need to collect unmanaged clusters even if no templates found
		r.collectUnmanagedClusters([]string{dir}, nil)
		return
	}

	r.logInfo("Checking cluster templates for drift (sync disabled)", "component", "Clusters", "count", len(templates))

	// Batch fetch all live cluster states once
	allLiveStates, _ := omni.GetAllLiveClusters()
	allHostnames, _ := omni.GetAllMachineHostnames()

	var resources []state.ResourceInfo
	inSync, outOfSync, errCount := 0, 0, 0

	for _, tmpl := range templates {
		name := extractClusterName(tmpl)
		if name == "" {
			r.logWarn("No cluster name found in template, skipping", "component", "Clusters", "file", tmpl)
			continue
		}

		// Read file content for UI display
		fileContent := readFileContent(tmpl)

		// Validate the template
		if err := omni.ClusterTemplateValidate(tmpl); err != nil {
			r.logError("Cluster template validation failed", "component", "Clusters", "cluster", name, "error", err)
			liveContent := allLiveStates[name]
			if liveContent == "" {
				liveContent, _ = omni.GetLiveCluster(name)
			}
			talos, k8s, cp, wk, clusterExts, machExts := clusterDetailFromLive(liveContent)
			cp, wk = hydratePoolMachines(name, cp, wk)
			resources = append(resources, state.ResourceInfo{
				ID:                name,
				Type:              "Cluster",
				Status:            "failed",
				FileContent:       fileContent,
				LiveContent:       liveContent,
				TalosVersion:      talos,
				KubernetesVersion: k8s,
				ControlPlane:      cp,
				Workers:           wk,
				ClusterExtensions: clusterExts,
				MachineExtensions: machExts,
				MachineHostnames:  extractHostnames(allHostnames, cp, wk),
			})
			errCount++
			continue
		}

		// Check if there are any changes
		diffOutput, _ := omni.ClusterTemplateDiff(tmpl)
		liveContent := allLiveStates[name]
		if liveContent == "" {
			liveContent, _ = omni.GetLiveCluster(name)
		}
		talos, k8s, cp, wk, clusterExts, machExts := clusterDetailFromLive(liveContent)
		cp, wk = hydratePoolMachines(name, cp, wk)
		if diffOutput == "" || strings.Contains(diffOutput, "no changes") {
			r.logDebug("Cluster in sync", "component", "Clusters", "cluster", name)
			resources = append(resources, state.ResourceInfo{
				ID:                name,
				Type:              "Cluster",
				Status:            "success",
				FileContent:       fileContent,
				LiveContent:       liveContent,
				TalosVersion:      talos,
				KubernetesVersion: k8s,
				ControlPlane:      cp,
				Workers:           wk,
				ClusterExtensions: clusterExts,
				MachineExtensions: machExts,
				MachineHostnames:  extractHostnames(allHostnames, cp, wk),
			})
			inSync++
		} else {
			r.logInfo("Cluster out of sync (sync disabled, skipping...)", "component", "Clusters", "cluster", name)
			resources = append(resources, state.ResourceInfo{
				ID:                name,
				Type:              "Cluster",
				Status:            "outofsync",
				Diff:              diffOutput,
				FileContent:       fileContent,
				LiveContent:       liveContent,
				TalosVersion:      talos,
				KubernetesVersion: k8s,
				ControlPlane:      cp,
				Workers:           wk,
				ClusterExtensions: clusterExts,
				MachineExtensions: machExts,
				MachineHostnames:  extractHostnames(allHostnames, cp, wk),
			})
			outOfSync++
		}
	}

	// Merge with existing cluster states to preserve unmanaged clusters
	existing := r.state.GetClusters()

	// Create a map of updated clusters for quick lookup
	updatedMap := make(map[string]state.ResourceInfo)
	for _, res := range resources {
		updatedMap[res.ID] = res
	}

	// Build the final list
	final := make([]state.ResourceInfo, 0, len(existing)+len(resources))
	processedIDs := make(map[string]bool)

	// First, update existing clusters
	for _, existingCluster := range existing {
		if updated, found := updatedMap[existingCluster.ID]; found {
			// This cluster was processed, use the new state
			final = append(final, updated)
			processedIDs[updated.ID] = true
		} else {
			// Preserve existing clusters (including unmanaged)
			final = append(final, existingCluster)
			processedIDs[existingCluster.ID] = true
		}
	}

	// Add any new clusters that weren't in existing state
	for _, res := range resources {
		if !processedIDs[res.ID] {
			final = append(final, res)
		}
	}

	r.state.SetClusters(final)
	r.logInfo("Cluster diff result", "component", "Clusters", "in_sync", inSync, "out_of_sync", outOfSync, "failed", errCount)

	// Also detect unmanaged clusters
	r.collectUnmanagedClusters([]string{dir}, nil)

	// Save state to disk
	r.state.Save()
}

// ============================================================
// Clusters — Delete
// ============================================================

// DeleteSingleCluster deletes a specific cluster from Omni by ID.
// It runs independently of the global reconcile lock — only that cluster's
// state entry is updated while the deletion is in progress.
func (r *Reconciler) DeleteSingleCluster(id string) {
	// Check the local app state instead of making a blocking Omni API call.
	// omni.IsClusterTemplateManaged would block if another Omni operation is
	// already in flight (e.g. a concurrent delete), causing this to time out
	// and silently skip the deletion. Instead, trust the state: any cluster
	// tracked with a non-'unmanaged' status was created by our reconciler.
	clusters := r.state.GetClusters()
	var found bool
	var isOrphaned bool
	for _, c := range clusters {
		if c.ID == id {
			if c.Status == "unmanaged" {
				r.logWarn("Cluster is unmanaged (not created by templates), skipping delete", "component", "Clusters", "cluster", id)
				return
			}
			found = true
			isOrphaned = c.Status == "orphaned"
			break
		}
	}
	if !found {
		r.logWarn("Cluster not found in state, skipping delete", "component", "Clusters", "cluster", id)
		return
	}

	// Mark as deleting immediately so the card shows the deleting state.
	// The watcher removes the card once Omni confirms the cluster is gone.
	r.state.UpdateClusterStatus(id, "deleting")
	r.state.Save()

	r.logWarn("Deleting cluster", "component", "Clusters", "cluster", id)

	if err := omni.DeleteCluster(id); err != nil {
		r.logError("Cluster delete failed", "component", "Clusters", "cluster", id, "error", err)
		// Restore the previous status on failure.
		if isOrphaned {
			r.state.UpdateClusterStatus(id, "orphaned")
		} else {
			r.state.UpdateClusterStatus(id, "outofsync")
		}
		r.state.Save()
		return
	}

	r.logInfo("Cluster delete initiated in Omni", "component", "Clusters", "cluster", id)
	// Keep 'deleting' status — the watcher's UpdateTearingDownStatuses will
	// remove the card once the cluster disappears from Omni's live list.
}

// DeleteClusters deletes clusters from Omni that no longer exist in Git.
// Only clusters with the omni.sidero.dev/managed-by-cluster-templates
// annotation are considered. Manually created clusters are never touched.
// Unmanaged clusters are added to state with "unmanaged" status for visibility.
func (r *Reconciler) DeleteClusters(dir string) {
	desiredIDs := collectClusterIDs(dir)

	allIDs, err := omni.GetClusterIDs()
	if err != nil {
		r.logError("Failed to list clusters", "component", "Clusters", "error", err)
		return
	}

	r.logInfo("Checking for template-managed clusters to delete", "component", "Clusters")

	// Build a set of cluster IDs where auto-sync is disabled so we skip deletion
	// for them (user must use the manual Sync button to delete).
	autoSyncDisabled := make(map[string]bool)
	for _, c := range r.state.GetClusters() {
		if c.AutoSync != nil && !*c.AutoSync {
			autoSyncDisabled[c.ID] = true
		}
	}

	// Only delete clusters that omni-cd has previously owned (appeared in a git
	// repo at some point). Template-managed clusters created externally (outside
	// any omni-cd repo) should never be auto-deleted by this instance.
	allTracked := r.state.GetAllTrackedClusterIDs()

	// Track unmanaged clusters to preserve in state
	var (
		unmanaged []state.ResourceInfo
		// notOwned holds template-managed clusters that omni-cd has never synced
		// from a git repo — preserve them in state without deleting.
		notOwned []string
		mu       sync.Mutex
		wg       sync.WaitGroup
		deleted  int
		failed   int
	)

	for _, id := range allIDs {
		// If cluster is in Git, keep it
		if contains(desiredIDs, id) {
			continue
		}

		// Skip clusters from repos that are currently failing to sync.
		if r.protectedClusters[id] {
			continue
		}

		// Only delete clusters managed by cluster templates.
		if !omni.IsClusterTemplateManaged(id) {
			r.logDebug("Cluster not managed by templates, ignoring", "component", "Clusters", "cluster", id)
			unmanaged = append(unmanaged, state.ResourceInfo{
				ID:        id,
				Type:      "Cluster",
				Status:    "unmanaged",
				CreatedAt: omni.GetClusterCreatedAt(id),
			})
			continue
		}

		// Template-managed but never synced by this omni-cd instance — treat as
		// externally managed; show as outofsync but never delete.
		if !allTracked[id] {
			r.logDebug("Cluster managed by templates but not tracked by omni-cd, skipping delete", "component", "Clusters", "cluster", id)
			notOwned = append(notOwned, id)
			continue
		}

		// Skip deletion for clusters where auto-sync is disabled — the user
		// must manually sync them via the Sync button.
		if autoSyncDisabled[id] {
			r.logInfo("Cluster not in Git but auto sync is disabled, skipping delete", "component", "Clusters", "cluster", id)
			continue
		}

		r.state.UpdateClusterStatus(id, "deleting")
		wg.Add(1)
		go func(clusterID string) {
			defer wg.Done()
			r.logWarn("Cluster not in Git, deleting", "component", "Clusters", "cluster", clusterID)
			if err := omni.DeleteCluster(clusterID); err != nil {
				r.logError("Cluster delete failed", "component", "Clusters", "cluster", clusterID, "error", err)
				mu.Lock()
				failed++
				mu.Unlock()
			} else {
				r.logInfo("Cluster deleted", "component", "Clusters", "cluster", clusterID)
				mu.Lock()
				deleted++
				mu.Unlock()
			}
		}(id)
	}

	// Snapshot protected clusters so the goroutine has a stable copy.
	protectedSnapshotSingle := r.protectedClusters

	// Wait for all deletions in the background so the reconcile loop is not blocked.
	go func() {
		wg.Wait()

		// Update state - merge unmanaged clusters with managed ones from git
		existing := r.state.GetClusters()

		// Build a lookup of existing rich data for unmanaged clusters so we
		// don't lose TalosVersion / KubernetesVersion / node-group info that
		// was populated by ApplyClusters / collectUnmanagedClusters.
		existingMap := make(map[string]state.ResourceInfo, len(existing))
		for _, c := range existing {
			existingMap[c.ID] = c
		}

		// Build final list: keep clusters from git + unmanaged clusters
		desiredSet := make(map[string]bool)
		for _, id := range desiredIDs {
			desiredSet[id] = true
		}
		for id := range protectedSnapshotSingle {
			desiredSet[id] = true
		}
		// Preserve clusters that were skipped because auto-sync is disabled —
		// they must stay in state as outofsync, not be silently dropped.
		// Exclude clusters that are in the unmanaged slice — those are handled
		// below and adding them here would create a duplicate entry.
		mu.Lock()
		unmanagedSetSingle := make(map[string]bool, len(unmanaged))
		for _, u := range unmanaged {
			unmanagedSetSingle[u.ID] = true
		}
		mu.Unlock()
		for id := range autoSyncDisabled {
			if !unmanagedSetSingle[id] {
				desiredSet[id] = true
			}
		}
		// Preserve template-managed clusters not owned by omni-cd so they stay
		// in state for collectUnmanagedClusters to classify as outofsync.
		for _, id := range notOwned {
			desiredSet[id] = true
		}
		var final []state.ResourceInfo
		for _, cluster := range existing {
			if desiredSet[cluster.ID] {
				final = append(final, cluster)
			}
		}
		// Then add unmanaged clusters, preserving existing rich data.
		mu.Lock()
		for _, u := range unmanaged {
			if rich, ok := existingMap[u.ID]; ok {
				rich.Status = "unmanaged" // ensure status is correct
				if rich.CreatedAt.IsZero() {
					rich.CreatedAt = u.CreatedAt
				}
				final = append(final, rich)
			} else {
				final = append(final, u)
			}
		}
		d, f := deleted, failed
		mu.Unlock()

		r.state.SetClusters(final)

		if d == 0 && f == 0 {
			r.logInfo("No clusters to delete", "component", "Clusters")
		} else {
			r.logInfo("Cluster delete result", "component", "Clusters", "deleted", d, "failed", f)
		}
	}()
}

// DeleteClustersAll is the multi-repo variant of DeleteClusters.
// It collects desired IDs from all provided directories and deletes any
// template-managed cluster that is not present in any of them.
// liveIDs is an optional pre-fetched cluster ID list from Omni; pass nil to
// fetch internally (avoids a duplicate API call when the caller already has it).
func (r *Reconciler) DeleteClustersAll(dirs []string, liveIDs []string) {
	var desiredIDs []string
	for _, dir := range dirs {
		desiredIDs = append(desiredIDs, collectClusterIDs(dir)...)
	}

	var allIDs []string
	if liveIDs != nil {
		allIDs = liveIDs
	} else {
		var err error
		allIDs, err = omni.GetClusterIDs()
		if err != nil {
			r.logError("Failed to list clusters", "component", "Clusters", "error", err)
			return
		}
	}

	r.logInfo("Checking for template-managed clusters to delete (multi-repo)", "component", "Clusters")

	autoSyncDisabled := make(map[string]bool)
	for _, c := range r.state.GetClusters() {
		if c.AutoSync != nil && !*c.AutoSync {
			autoSyncDisabled[c.ID] = true
		}
	}

	// Only delete clusters that omni-cd has previously owned.
	allTrackedAll := r.state.GetAllTrackedClusterIDs()

	var (
		unmanaged []state.ResourceInfo
		// notOwned holds template-managed clusters omni-cd has never synced.
		notOwned []string
		mu       sync.Mutex
		wg       sync.WaitGroup
		deleted  int
		failed   int
	)

	for _, id := range allIDs {
		if contains(desiredIDs, id) {
			continue
		}
		// Skip clusters whose repo is currently failing to sync — they are not
		// "missing from git", they just can't be verified right now.
		if r.protectedClusters[id] {
			continue
		}
		if !omni.IsClusterTemplateManaged(id) {
			r.logDebug("Cluster not managed by templates, ignoring", "component", "Clusters", "cluster", id)
			unmanaged = append(unmanaged, state.ResourceInfo{
				ID:        id,
				Type:      "Cluster",
				Status:    "unmanaged",
				CreatedAt: omni.GetClusterCreatedAt(id),
			})
			continue
		}
		// Template-managed but never synced by this omni-cd instance.
		if !allTrackedAll[id] {
			r.logDebug("Cluster managed by templates but not tracked by omni-cd, skipping delete", "component", "Clusters", "cluster", id)
			notOwned = append(notOwned, id)
			continue
		}
		if autoSyncDisabled[id] {
			r.logInfo("Cluster not in Git but auto sync is disabled, skipping delete", "component", "Clusters", "cluster", id)
			continue
		}
		r.state.UpdateClusterStatus(id, "deleting")
		wg.Add(1)
		go func(clusterID string) {
			defer wg.Done()
			r.logWarn("Cluster not in Git, deleting", "component", "Clusters", "cluster", clusterID)
			if err := omni.DeleteCluster(clusterID); err != nil {
				r.logError("Cluster delete failed", "component", "Clusters", "cluster", clusterID, "error", err)
				mu.Lock()
				failed++
				mu.Unlock()
			} else {
				r.logInfo("Cluster deleted", "component", "Clusters", "cluster", clusterID)
				mu.Lock()
				deleted++
				mu.Unlock()
			}
		}(id)
	}

	// Snapshot protected clusters before launching the goroutine so that a
	// subsequent reconcile calling SetProtectedResources cannot race with it.
	protectedSnapshot := r.protectedClusters

	go func() {
		wg.Wait()
		existing := r.state.GetClusters()
		existingMap := make(map[string]state.ResourceInfo, len(existing))
		for _, c := range existing {
			existingMap[c.ID] = c
		}
		desiredSet := make(map[string]bool)
		for _, id := range desiredIDs {
			desiredSet[id] = true
		}
		// Also protect clusters from repos that are currently failing to sync.
		for id := range protectedSnapshot {
			desiredSet[id] = true
		}
		// Preserve clusters that were skipped because auto-sync is disabled —
		// they must stay in state as outofsync, not be silently dropped.
		// Exclude clusters that are in the unmanaged slice — those are handled
		// below and adding them here would create a duplicate entry.
		mu.Lock()
		unmanagedSet := make(map[string]bool, len(unmanaged))
		for _, u := range unmanaged {
			unmanagedSet[u.ID] = true
		}
		mu.Unlock()
		for id := range autoSyncDisabled {
			if !unmanagedSet[id] {
				desiredSet[id] = true
			}
		}
		// Preserve template-managed clusters not owned by omni-cd.
		for _, id := range notOwned {
			desiredSet[id] = true
		}
		var final []state.ResourceInfo
		for _, cluster := range existing {
			if desiredSet[cluster.ID] {
				final = append(final, cluster)
			}
		}
		mu.Lock()
		for _, u := range unmanaged {
			if rich, ok := existingMap[u.ID]; ok {
				rich.Status = "unmanaged"
				if rich.CreatedAt.IsZero() {
					rich.CreatedAt = u.CreatedAt
				}
				final = append(final, rich)
			} else {
				final = append(final, u)
			}
		}
		d, f := deleted, failed
		mu.Unlock()
		r.state.SetClusters(final)
		if d == 0 && f == 0 {
			r.logInfo("No clusters to delete", "component", "Clusters")
		} else {
			r.logInfo("Cluster delete result", "component", "Clusters", "deleted", d, "failed", f)
		}
	}()
}

// ============================================================
// Clusters — Detect Unmanaged
// ============================================================

// CollectUnmanagedClusters is the exported entry point for post-reconcile
// unmanaged-cluster detection across all repo dirs. Call this once after all
// per-repo ApplyClusters calls are done so every repo's clusters are in scope.
// liveIDs is the pre-fetched list of all cluster IDs in Omni; if nil the
// function fetches it internally (allows reuse of a single API call when the
// caller already has the list).
func (r *Reconciler) CollectUnmanagedClusters(dirs []string, liveIDs []string) {
	r.collectUnmanagedClusters(dirs, liveIDs)
}

// collectUnmanagedClusters finds clusters in Omni that are not managed by
// cluster templates and adds them to state with "unmanaged" status.
// Also removes clusters from state that are no longer in git or Omni.
// liveIDs is an optional pre-fetched cluster ID list; pass nil to fetch inside.
func (r *Reconciler) collectUnmanagedClusters(dirs []string, liveIDs []string) {
	// Collect unique cluster IDs from all provided dirs.
	seen := make(map[string]bool)
	var desiredIDs []string
	for _, dir := range dirs {
		for _, id := range collectClusterIDs(dir) {
			if !seen[id] {
				seen[id] = true
				desiredIDs = append(desiredIDs, id)
			}
		}
	}
	// Treat clusters from failing repos as "desired" so they are not re-classified
	// as outofsync or unmanaged while their repo is temporarily unreachable.
	for id := range r.protectedClusters {
		if !seen[id] {
			seen[id] = true
			desiredIDs = append(desiredIDs, id)
		}
	}

	// Used to distinguish clusters omni-cd previously owned from externally-created ones.
	allTracked := r.state.GetAllTrackedClusterIDs()
	allHostnames, _ := omni.GetAllMachineHostnames()

	// Fetch cluster IDs with phase info so we can detect TearingDown clusters.
	var allIDs []string
	tearingDownMap := make(map[string]bool)
	if liveIDs != nil {
		allIDs = liveIDs
		// Still need phase info — fetch it separately when IDs were pre-supplied.
		if _, td, err := omni.GetClusterIDsWithPhases(); err == nil {
			tearingDownMap = td
		}
	} else {
		var err error
		allIDs, tearingDownMap, err = omni.GetClusterIDsWithPhases()
		if err != nil {
			return
		}
	}

	existing := r.state.GetClusters()

	// Create maps for quick lookup
	desiredMap := make(map[string]bool)
	for _, id := range desiredIDs {
		desiredMap[id] = true
	}

	omniMap := make(map[string]bool)
	for _, id := range allIDs {
		omniMap[id] = true
	}

	// Build final state:
	// 1. Keep clusters that are in git (already processed by Apply/Diff)
	// 2. Mark managed clusters not in git as "outofsync" (removed from git but still managed)
	// 3. Mark unmanaged clusters (manually created) as "unmanaged"
	// 4. Remove clusters that are no longer in Omni
	var final []state.ResourceInfo

	// Keep clusters from state that are still in git
	for _, cluster := range existing {
		if desiredMap[cluster.ID] {
			if tearingDownMap[cluster.ID] {
				// Cluster is being torn down in Omni (phase=TearingDown) — mark as deleting.
				cluster.Status = "deleting"
			} else if !omniMap[cluster.ID] {
				switch cluster.Status {
				case "deleting":
					// Deletion completed but template still in git — revert to outofsync
					// so the cluster stays visible and can be re-synced or removed from git.
					cluster.Status = "outofsync"
					cluster.LiveContent = ""
				case "success", "applied":
					// Cluster was running but disappeared from Omni externally — mark as deleting.
					cluster.Status = "deleting"
				}
			}
			final = append(final, cluster)
		} else if !omniMap[cluster.ID] {
			// Skip - this cluster has been deleted
		} else {
			// In Omni but not in git - check if it's template-managed
			// Never overwrite a 'deleting' or 'orphaned' status — 'deleting' means
			// a delete is in flight; 'orphaned' means the repo was deleted while
			// auto-sync was off and the cluster must stay visible until the user
			// explicitly deletes it.
			if cluster.Status == "deleting" || cluster.Status == "orphaned" {
				final = append(final, cluster)
				continue
			}
			isManaged := omni.IsClusterTemplateManaged(cluster.ID)
			if isManaged {
				if allTracked[cluster.ID] {
					cluster.Status = "outofsync"
					cluster.Diff = "Cluster template is removed from git. Enable Auto-Sync or Sync to delete this cluster."
				} else {
					cluster.Status = "orphaned"
					cluster.Diff = "Cluster template exists in Omni but is not managed by this omni-cd instance."
				}
			} else {
				cluster.Status = "unmanaged"
				cluster.Diff = ""
			}
			// Refresh live data when version is unknown OR when machines are not yet populated.
			needsMachines := len(cluster.ControlPlane.Machines) == 0
			if cluster.TalosVersion == "" || needsMachines {
				if liveContent, err := omni.GetLiveCluster(cluster.ID); err == nil && liveContent != "" {
					talos, k8s, cp, workers, clusterExts, machExts := clusterDetailFromLive(liveContent)
					cp, workers = hydratePoolMachines(cluster.ID, cp, workers)
					cluster.LiveContent = liveContent
					cluster.TalosVersion = talos
					cluster.KubernetesVersion = k8s
					cluster.ControlPlane = cp
					cluster.Workers = workers
					cluster.ClusterExtensions = clusterExts
					cluster.MachineExtensions = machExts
					cluster.MachineHostnames = extractHostnames(allHostnames, cp, workers)
				}
			}
			final = append(final, cluster)
		}
	}

	// Add newly discovered clusters (in Omni, not in state yet)
	existingMap := make(map[string]bool)
	for _, res := range existing {
		existingMap[res.ID] = true
	}

	for _, id := range allIDs {
		// Skip if already processed above
		if existingMap[id] {
			continue
		}

		// Skip if has a template in Git (will be added by Apply/Diff)
		if desiredMap[id] {
			continue
		}

		// Skip clusters whose deletion is actively in progress in this process.
		if _, pending := r.pendingDeletes.Load(id); pending {
			continue
		}

		// Skip clusters that are currently tearing down in Omni — they were
		// likely just deleted (e.g. an orphaned cluster the user triggered a
		// delete on) and will disappear from Omni on their own. Re-adding them
		// here would cause them to flicker back into the UI as outofsync/orphaned.
		if tearingDownMap[id] {
			continue
		}

		// Check if this is a managed or unmanaged cluster
		isManaged := omni.IsClusterTemplateManaged(id)
		status := "unmanaged"
		diffMsg := ""
		if isManaged {
			if allTracked[id] {
				status = "outofsync"
				diffMsg = "Cluster template is removed from git. Enable Auto-Sync or Sync to delete this cluster."
			} else {
				status = "orphaned"
				diffMsg = "Cluster template exists in Omni but is not managed by this omni-cd instance."
			}
		}
		liveContent, _ := omni.GetLiveCluster(id)
		talos, k8s, cp, workers, clusterExts, machExts := clusterDetailFromLive(liveContent)
		cp, workers = hydratePoolMachines(id, cp, workers)
		final = append(final, state.ResourceInfo{
			ID:                id,
			Type:              "Cluster",
			Status:            status,
			Diff:              diffMsg,
			LiveContent:       liveContent,
			TalosVersion:      talos,
			KubernetesVersion: k8s,
			ControlPlane:      cp,
			Workers:           workers,
			ClusterExtensions: clusterExts,
			MachineExtensions: machExts,
			MachineHostnames:  extractHostnames(allHostnames, cp, workers),
		})
	}

	r.state.SetClusters(final)
}

// DetectCrossRepoDuplicates returns a map of cluster IDs that appear in more
// than one repo directory, mapping each duplicate ID to a comma-separated list
// of the repo names that define it.
func (r *Reconciler) DetectCrossRepoDuplicates(dirs []string) map[string]string {
	return detectCrossRepoDuplicates(dirs)
}

// forceDeleteClusters deletes all template-managed clusters declared in the
// given directories whose auto-sync is enabled. Called when a repo is removed.
func (r *Reconciler) forceDeleteClusters(dirs []string) {
	var clusterIDs []string
	for _, dir := range dirs {
		clusterIDs = append(clusterIDs, collectClusterIDs(dir)...)
	}

	autoSyncDisabled := make(map[string]bool)
	for _, c := range r.state.GetClusters() {
		if c.AutoSync != nil && !*c.AutoSync {
			autoSyncDisabled[c.ID] = true
		}
	}

	allTracked := r.state.GetAllTrackedClusterIDs()

	for _, id := range clusterIDs {
		if autoSyncDisabled[id] {
			r.logInfo("Repo deleted but cluster auto-sync disabled, leaving as orphaned", "component", "Clusters", "cluster", id)
			r.state.UpdateClusterStatus(id, "orphaned")
			continue
		}
		if !omni.IsClusterTemplateManaged(id) {
			continue
		}
		if !allTracked[id] {
			r.logDebug("Cluster managed by templates but not tracked by omni-cd, skipping force-delete", "component", "Clusters", "cluster", id)
			continue
		}
		r.logWarn("Force-deleting cluster from deleted repo", "component", "Clusters", "cluster", id)
		r.state.UpdateClusterStatus(id, "deleting")
		if err := omni.DeleteCluster(id); err != nil {
			r.logError("Force-delete of cluster failed", "component", "Clusters", "cluster", id, "error", err)
		} else {
			r.logInfo("Cluster force-deleted", "component", "Clusters", "cluster", id)
			r.state.RemoveCluster(id)
		}
	}
}
