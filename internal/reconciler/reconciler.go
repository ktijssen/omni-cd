package reconciler

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"omni-cd/internal/omni"
	"omni-cd/internal/state"
)

// Reconciler handles the apply and delete phases for machine classes
// and cluster templates.
type Reconciler struct {
	state *state.AppState
}

// New creates a new Reconciler with shared state.
func New(appState *state.AppState) *Reconciler {
	return &Reconciler{state: appState}
}

// ============================================================
// Machine Classes — Apply
// ============================================================

// ApplyMachineClasses applies all machine class YAML files from the given directory.
// This is idempotent — existing classes are updated, new ones are created.
// Files can contain multiple machine classes separated by ---.
func (r *Reconciler) ApplyMachineClasses(dir string) {
	files, err := findYAMLFiles(dir)
	if err != nil {
		r.logWarn("Directory not found, skipping", "component", "MachineClasses", "path", dir)
		return
	}
	if len(files) == 0 {
		r.logWarn("No YAML files found", "component", "MachineClasses", "path", dir)
		return
	}

	// Count total IDs across all files
	idCount := 0
	for _, f := range files {
		idCount += len(extractAllIDs(f))
	}

	r.logInfo("Syncing machine classes", "component", "MachineClasses", "count", idCount)

	// Detect duplicate IDs across files
	idToFiles := make(map[string][]string)
	for _, f := range files {
		for _, id := range extractAllIDs(f) {
			idToFiles[id] = append(idToFiles[id], f)
		}
	}
	duplicateIDs := make(map[string]bool)
	repoRoot := filepath.Dir(dir)
	var resources []state.ResourceInfo
	applied, failed := 0, 0
	for id, idFiles := range idToFiles {
		if len(idFiles) > 1 {
			duplicateIDs[id] = true
			relFiles := make([]string, len(idFiles))
			for i, f := range idFiles {
				relFiles[i] = strings.TrimPrefix(f, repoRoot)
			}
			errMsg := fmt.Sprintf("Conflicting machine class templates: %s", strings.Join(relFiles, ", "))
			r.logError("Duplicate machine class ID found, skipping sync", "component", "MachineClasses", "id", id, "files", strings.Join(relFiles, ", "))
			resources = append(resources, state.ResourceInfo{
				ID:     id,
				Type:   "MachineClass",
				Status: "outofsync",
				Error:  errMsg,
			})
			failed++
		}
	}

	// Batch fetch all live machine class states once
	allLiveStates, _ := omni.GetAllLiveMachineClasses()

	for _, file := range files {
		ids := extractAllIDs(file)
		// Filter out duplicate IDs
		var nonDupIDs []string
		for _, id := range ids {
			if !duplicateIDs[id] {
				nonDupIDs = append(nonDupIDs, id)
			}
		}
		if len(nonDupIDs) == 0 {
			continue
		}
		ids = nonDupIDs
		provisionType := detectProvisionType(file)

		// Get dry-run diff to check if changes are needed
		diffOutput, dryRunErr := omni.MachineClassDryRun(file)
		fileContent := readFileContent(file)

		// If dry-run failed (validation error), mark as failed
		if dryRunErr != nil {
			r.logError("Machine class validation failed", "component", "MachineClasses", "ids", strings.Join(ids, ", "), "error", dryRunErr)
			for _, id := range ids {
				// Get from batch or fallback to individual fetch
				liveContent := allLiveStates[id]
				if liveContent == "" {
					liveContent, _ = omni.GetLiveMachineClass(id)
				}
				resources = append(resources, state.ResourceInfo{
					ID:            id,
					Type:          "MachineClass",
					Status:        "failed",
					ProvisionType: provisionType,
					FileContent:   fileContent,
					LiveContent:   liveContent,
					Error:         dryRunErr.Error(),
				})
			}
			failed += len(ids)
			continue
		}

		// Check if there are any changes to apply
		if diffOutput == "" || strings.Contains(diffOutput, "no changes") {
			r.logDebug("Machine classes up to date", "component", "MachineClasses", "ids", strings.Join(ids, ", "))
			for _, id := range ids {
				// Get from batch or fallback to individual fetch
				liveContent := allLiveStates[id]
				if liveContent == "" {
					liveContent, _ = omni.GetLiveMachineClass(id)
				}
				resources = append(resources, state.ResourceInfo{
					ID:            id,
					Type:          "MachineClass",
					Status:        "success",
					ProvisionType: provisionType,
					FileContent:   fileContent,
					LiveContent:   liveContent,
				})
			}
			applied += len(ids)
			continue
		}

		// There is a diff — apply it
		if err := omni.Apply(file); err != nil {
			r.logError("Machine class apply failed", "component", "MachineClasses", "ids", strings.Join(ids, ", "), "error", err)
			for _, id := range ids {
				// Get from batch or fallback to individual fetch
				liveContent := allLiveStates[id]
				if liveContent == "" {
					liveContent, _ = omni.GetLiveMachineClass(id)
				}
				resources = append(resources, state.ResourceInfo{
					ID:            id,
					Type:          "MachineClass",
					Status:        "failed",
					ProvisionType: provisionType,
					Diff:          diffOutput,
					FileContent:   fileContent,
					LiveContent:   liveContent,
					Error:         err.Error(),
				})
			}
			failed += len(ids)
		} else {
			r.logInfo("Machine classes applied", "component", "MachineClasses", "ids", strings.Join(ids, ", "))
			for _, id := range ids {
				// Get from batch or fallback to individual fetch
				liveContent := allLiveStates[id]
				if liveContent == "" {
					liveContent, _ = omni.GetLiveMachineClass(id)
				}
				resources = append(resources, state.ResourceInfo{
					ID:            id,
					Type:          "MachineClass",
					Status:        "success",
					ProvisionType: provisionType,
					Diff:          diffOutput,
					FileContent:   fileContent,
					LiveContent:   liveContent,
				})
			}
			applied += len(ids)
		}
	}

	r.state.SetMachineClasses(resources)
	r.logInfo("Machine classes result", "component", "MachineClasses", "synced", applied, "failed", failed)
}

// ============================================================
// Machine Classes — Delete
// ============================================================

// DeleteMachineClasses deletes machine classes from Omni that no longer exist in Git.
// If a machine class is still in use by a cluster, the delete is skipped with a warning.
func (r *Reconciler) DeleteMachineClasses(dir string) {
	desiredIDs := collectMachineClassIDs(dir)

	existingIDs, err := omni.GetMachineClassIDs()
	if err != nil {
		r.logError("Failed to list machine classes", "component", "MachineClasses", "error", err)
		return
	}

	r.logInfo("Checking for machine classes to delete", "component", "MachineClasses")

	deleted, failed := 0, 0
	for _, id := range existingIDs {
		if contains(desiredIDs, id) {
			continue
		}

		r.logWarn("Machine class not in Git, deleting", "component", "MachineClasses", "id", id)
		output, err := omni.DeleteMachineClass(id)
		if err != nil {
			if strings.Contains(output, "still in use") {
				r.logWarn("Machine class still in use, skipping delete", "component", "MachineClasses", "id", id)
			} else {
				r.logError("Machine class delete failed", "component", "MachineClasses", "id", id, "output", output)
				failed++
			}
		} else {
			r.logInfo("Machine class deleted", "component", "MachineClasses", "id", id)
			deleted++
		}
	}

	if deleted == 0 && failed == 0 {
		r.logInfo("No machine classes to delete", "component", "MachineClasses")
	} else {
		r.logInfo("Machine class delete result", "component", "MachineClasses", "deleted", deleted, "failed", failed)
	}
}

// ============================================================
// Clusters — Apply
// ============================================================

// ApplyClusters validates and syncs all cluster templates from the given directory.
// Each subdirectory must contain a cluster.yaml file. Templates that fail validation
// are skipped, leaving the existing cluster intact.
// Only syncs when there is an actual diff to avoid unnecessary updates.
func (r *Reconciler) ApplyClusters(dir string) {
	// Check if we're force-syncing a specific cluster BEFORE checking templates
	forceClusterID := r.state.GetForceClusterID()

	templates, err := findClusterTemplates(dir)
	if err != nil {
		// If force-syncing and no templates directory exists, delete the cluster
		if forceClusterID != "" && omni.IsClusterTemplateManaged(forceClusterID) {
			r.logWarn("Cluster not in Git (no templates directory), deleting", "component", "Clusters", "cluster", forceClusterID)
			if err := omni.DeleteCluster(forceClusterID); err != nil {
				r.logError("Cluster delete failed", "component", "Clusters", "cluster", forceClusterID, "error", err)
				return
			}
			r.logInfo("Cluster deleted", "component", "Clusters", "cluster", forceClusterID)
			r.collectUnmanagedClusters(dir)
			return
		}
		r.logWarn("Directory not found, skipping", "component", "Clusters", "path", dir)
		return
	}

	if len(templates) == 0 {
		// If force-syncing and no templates found, delete the cluster
		if forceClusterID != "" && omni.IsClusterTemplateManaged(forceClusterID) {
			r.logWarn("Cluster not in Git (no templates), deleting", "component", "Clusters", "cluster", forceClusterID)
			if err := omni.DeleteCluster(forceClusterID); err != nil {
				r.logError("Cluster delete failed", "component", "Clusters", "cluster", forceClusterID, "error", err)
				return
			}
			r.logInfo("Cluster deleted", "component", "Clusters", "cluster", forceClusterID)
			r.collectUnmanagedClusters(dir)
			return
		}
		r.logWarn("No cluster templates found", "component", "Clusters")
		return
	}

	if forceClusterID != "" {
		r.logInfo("Force syncing cluster", "component", "Clusters", "cluster", forceClusterID)

		// Check if the cluster exists in Git templates
		clusterInGit := false
		for _, tmpl := range templates {
			if extractClusterName(tmpl) == forceClusterID {
				clusterInGit = true
				break
			}
		}

		// If cluster is not in Git but is managed, delete it
		if !clusterInGit && omni.IsClusterTemplateManaged(forceClusterID) {
			r.logWarn("Cluster not in Git, deleting", "component", "Clusters", "cluster", forceClusterID)
			if err := omni.DeleteCluster(forceClusterID); err != nil {
				r.logError("Cluster delete failed", "component", "Clusters", "cluster", forceClusterID, "error", err)
				return
			}
			r.logInfo("Cluster deleted", "component", "Clusters", "cluster", forceClusterID)
			// Remove from state
			r.collectUnmanagedClusters(dir)
			return
		}
	} else {
		r.logInfo("Syncing cluster templates", "component", "Clusters", "count", len(templates))
	}

	// Batch fetch all live cluster states once
	allLiveStates, _ := omni.GetAllLiveClusters()

	var (
		mu        sync.Mutex
		wg        sync.WaitGroup
		resources []state.ResourceInfo
		synced    int
		failed    int
	)

	// Detect duplicate cluster IDs across templates before processing.
	// Two cluster.yaml files with the same cluster name would conflict —
	// mark both as out of sync with an error and skip them.
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
			r.state.UpsertClusterStatus(name, "outofsync")
			resources = append(resources, state.ResourceInfo{
				ID:     name,
				Type:   "Cluster",
				Status: "outofsync",
				Error:  errMsg,
			})
			failed++
		}
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

		// If force-syncing a specific cluster, skip others
		if forceClusterID != "" && name != forceClusterID {
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
			isForceSync := forceClusterID != "" && clusterName == forceClusterID

			if !isForceSync && (diffOutput == "" || strings.Contains(diffOutput, "no changes")) {
				r.logDebug("Cluster up to date", "component", "Clusters", "cluster", clusterName)
				liveContent := allLiveStates[clusterName]
				if liveContent == "" {
					liveContent, _ = omni.GetLiveCluster(clusterName)
				}
				talos, k8s, cp, wk, clusterExts, machExts := clusterDetailFromLive(liveContent)
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
				})
				mu.Unlock()
				return
			}

			// There is a diff or force sync — log it and sync
			if isForceSync {
				r.logWarn("Force syncing cluster", "component", "Clusters", "cluster", clusterName)
			} else {
				r.logWarn("Cluster out of sync", "component", "Clusters", "cluster", clusterName)
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
				mu.Lock()
				resources = append(resources, state.ResourceInfo{
					ID:                clusterName,
					Type:              "Cluster",
					Status:            "failed",
					Diff:              diffOutput,
					FileContent:       fileContent,
					LiveContent:       liveContent,
					Error:             err.Error(),
					TalosVersion:      talos,
					KubernetesVersion: k8s,
					ControlPlane:      cp,
					Workers:           wk,
					ClusterExtensions: clusterExts,
					MachineExtensions: machExts,
				})
				failed++
				mu.Unlock()
			} else {
				r.logInfo("Cluster synced", "component", "Clusters", "cluster", clusterName)
				r.state.UpsertClusterStatus(clusterName, "success")
				// Always fetch fresh after sync — the pre-fetched cache is stale
				liveContent, _ := omni.GetLiveCluster(clusterName)
				talos, k8s, cp, wk, clusterExts, machExts := clusterDetailFromLive(liveContent)
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
				})
				synced++
				mu.Unlock()
			}
		}(tmpl, name)
	}

	wg.Wait()

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
			// This cluster was processed, use the new state
			final = append(final, updated)
			processedIDs[updated.ID] = true
		} else if forceClusterID == "" {
			// Not force-syncing, preserve all existing clusters
			final = append(final, existingCluster)
			processedIDs[existingCluster.ID] = true
		} else if existingCluster.ID != forceClusterID {
			// Force-syncing, but this isn't the target cluster, so preserve it
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

	if forceClusterID != "" {
		r.logInfo("Force sync complete", "component", "Clusters", "synced", synced, "failed", failed)
	} else {
		r.logInfo("Cluster apply result", "component", "Clusters", "synced", synced, "failed", failed)
	}

	// Always collect unmanaged clusters to ensure they're visible
	r.collectUnmanagedClusters(dir)

	// Save state to disk
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
		r.collectUnmanagedClusters(dir)
		return
	}
	if len(templates) == 0 {
		r.logWarn("No cluster templates found", "component", "Clusters")
		// Still need to collect unmanaged clusters even if no templates found
		r.collectUnmanagedClusters(dir)
		return
	}

	r.logInfo("Checking cluster templates for drift (sync disabled)", "component", "Clusters", "count", len(templates))

	// Batch fetch all live cluster states once
	allLiveStates, _ := omni.GetAllLiveClusters()

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
			})
			inSync++
		} else {
			r.logWarn("Cluster out of sync (sync disabled, skipping...)", "component", "Clusters", "cluster", name)
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
	r.collectUnmanagedClusters(dir)

	// Save state to disk
	r.state.Save()
}

// ============================================================
// Clusters — Delete
// ============================================================

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

	// Track unmanaged clusters to preserve in state
	var (
		unmanaged []state.ResourceInfo
		mu        sync.Mutex
		wg        sync.WaitGroup
		deleted   int
		failed    int
	)

	for _, id := range allIDs {
		// If cluster is in Git, keep it
		if contains(desiredIDs, id) {
			continue
		}

		// Only delete clusters managed by cluster templates.
		if !omni.IsClusterTemplateManaged(id) {
			r.logDebug("Cluster not managed by templates, ignoring", "component", "Clusters", "cluster", id)
			unmanaged = append(unmanaged, state.ResourceInfo{
				ID:     id,
				Type:   "Cluster",
				Status: "unmanaged",
			})
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

	wg.Wait()

	// Update state - merge unmanaged clusters with managed ones from git
	existing := r.state.GetClusters()

	// Build final list: keep clusters from git + unmanaged clusters
	desiredSet := make(map[string]bool)
	for _, id := range desiredIDs {
		desiredSet[id] = true
	}

	var final []state.ResourceInfo
	// First, keep all clusters that are in git (managed)
	for _, cluster := range existing {
		if desiredSet[cluster.ID] {
			final = append(final, cluster)
		}
	}
	// Then add unmanaged clusters
	final = append(final, unmanaged...)

	r.state.SetClusters(final)

	if deleted == 0 && failed == 0 {
		r.logInfo("No clusters to delete", "component", "Clusters")
	} else {
		r.logInfo("Cluster delete result", "component", "Clusters", "deleted", deleted, "failed", failed)
	}
}

// ============================================================
// Clusters — Detect Unmanaged
// ============================================================

// collectUnmanagedClusters finds clusters in Omni that are not managed by
// cluster templates and adds them to state with "unmanaged" status.
// Also removes clusters from state that are no longer in git or Omni.
func (r *Reconciler) collectUnmanagedClusters(dir string) {
	desiredIDs := collectClusterIDs(dir)

	allIDs, err := omni.GetClusterIDs()
	if err != nil {
		return
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
			final = append(final, cluster)
		} else if !omniMap[cluster.ID] {
			// Skip - this cluster has been deleted
		} else {
			// In Omni but not in git - check if it's template-managed
			isManaged := omni.IsClusterTemplateManaged(cluster.ID)
			if isManaged {
				cluster.Status = "outofsync"
				cluster.Diff = "Cluster template removed from git. Force sync to delete this cluster."
			} else {
				cluster.Status = "unmanaged"
				cluster.Diff = ""
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

		// Check if this is a managed or unmanaged cluster
		isManaged := omni.IsClusterTemplateManaged(id)
		if isManaged {
			final = append(final, state.ResourceInfo{
				ID:     id,
				Type:   "Cluster",
				Status: "outofsync",
				Diff:   "Cluster template removed from git. Force sync to delete this cluster.",
			})
		} else {
			final = append(final, state.ResourceInfo{
				ID:     id,
				Type:   "Cluster",
				Status: "unmanaged",
			})
		}
	}

	r.state.SetClusters(final)
}

// ============================================================
// Helpers
// ============================================================

// findYAMLFiles returns all .yaml and .yml files in a directory.
func findYAMLFiles(dir string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}

	var files []string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		n := e.Name()
		if strings.HasSuffix(n, ".yaml") || strings.HasSuffix(n, ".yml") {
			files = append(files, filepath.Join(dir, n))
		}
	}
	return files, nil
}

// findClusterTemplates returns all cluster.yaml files found in
// subdirectories of the given directory.
func findClusterTemplates(dir string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}

	var templates []string
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		tmpl := filepath.Join(dir, e.Name(), "cluster.yaml")
		if _, err := os.Stat(tmpl); err == nil {
			templates = append(templates, tmpl)
		}
	}
	return templates, nil
}

// clusterDetailFromLive parses a live cluster template export and returns
// the populated NodeGroup and version fields for a ResourceInfo.
func clusterDetailFromLive(liveContent string) (talos, k8s string, cp state.NodeGroup, workers []state.NodeGroup, clusterExts []string, machExts map[string][]string) {
	info := omni.ParseClusterTemplate(liveContent)
	talos = info.TalosVersion
	k8s = info.KubernetesVersion
	cp = state.NodeGroup{Name: info.ControlPlaneName, Count: info.ControlPlaneCount, MachineClass: info.ControlPlaneMachineClass, Machines: info.ControlPlaneMachines, Extensions: info.ControlPlaneExtensions}
	for _, wg := range info.WorkerGroups {
		workers = append(workers, state.NodeGroup{Name: wg.Name, Count: wg.Count, MachineClass: wg.MachineClass, Machines: wg.Machines, Extensions: wg.Extensions})
	}
	clusterExts = info.ClusterExtensions
	machExts = info.MachineExtensions
	return
}

// extractAllIDs extracts all resource ids from a YAML file.
// Supports multi-document YAML files separated by ---.
func extractAllIDs(file string) []string {
	data, err := os.ReadFile(file)
	if err != nil {
		return nil
	}

	var ids []string
	for _, line := range strings.Split(string(data), "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "id:") {
			parts := strings.Fields(trimmed)
			if len(parts) >= 2 {
				ids = append(ids, parts[1])
			}
		}
	}
	return ids
}

// extractClusterName extracts the cluster name from the 'name' field
// in a cluster.yaml template file.
func extractClusterName(file string) string {
	data, err := os.ReadFile(file)
	if err != nil {
		return ""
	}

	for _, line := range strings.Split(string(data), "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "name:") {
			parts := strings.Fields(trimmed)
			if len(parts) >= 2 {
				return parts[1]
			}
		}
	}
	return ""
}

// collectMachineClassIDs returns all desired machine class IDs from the Git repo.
func collectMachineClassIDs(dir string) []string {
	files, err := findYAMLFiles(dir)
	if err != nil {
		return nil
	}

	var ids []string
	for _, f := range files {
		ids = append(ids, extractAllIDs(f)...)
	}
	return ids
}

// collectClusterIDs returns all desired cluster names from the Git repo.
func collectClusterIDs(dir string) []string {
	templates, err := findClusterTemplates(dir)
	if err != nil {
		return nil
	}

	var ids []string
	for _, t := range templates {
		if name := extractClusterName(t); name != "" {
			ids = append(ids, name)
		}
	}
	return ids
}

// contains checks if a string slice contains a value.
func contains(slice []string, val string) bool {
	for _, s := range slice {
		if s == val {
			return true
		}
	}
	return false
}

// ============================================================
// Logging — writes to both stdout and shared state
// ============================================================

func (r *Reconciler) logDebug(msg string, attrs ...any) {
	// Extract component from attrs or default to empty
	component := extractComponent(attrs...)
	allAttrs := append([]any{"component", component}, attrs...)
	slog.Debug(msg, allAttrs...)

	// Only add to web UI if this level is enabled
	if slog.Default().Enabled(nil, slog.LevelDebug) {
		displayMsg := formatLogMessage("DEBUG", msg, attrs...)
		r.state.AddLog("DEBUG", component, displayMsg)
	}
}

func (r *Reconciler) logInfo(msg string, attrs ...any) {
	// Extract component from attrs or default to empty
	component := extractComponent(attrs...)
	allAttrs := append([]any{"component", component}, attrs...)
	slog.Info(msg, allAttrs...)

	// Only add to web UI if this level is enabled
	if slog.Default().Enabled(nil, slog.LevelInfo) {
		displayMsg := formatLogMessage("INFO", msg, attrs...)
		r.state.AddLog("INFO", component, displayMsg)
	}
}

func (r *Reconciler) logWarn(msg string, attrs ...any) {
	// Extract component from attrs or default to empty
	component := extractComponent(attrs...)
	allAttrs := append([]any{"component", component}, attrs...)
	slog.Warn(msg, allAttrs...)

	// Only add to web UI if this level is enabled
	if slog.Default().Enabled(nil, slog.LevelWarn) {
		displayMsg := formatLogMessage("WARN", msg, attrs...)
		r.state.AddLog("WARN", component, displayMsg)
	}
}

func (r *Reconciler) logError(msg string, attrs ...any) {
	// Extract component from attrs or default to empty
	component := extractComponent(attrs...)
	allAttrs := append([]any{"component", component}, attrs...)
	slog.Error(msg, allAttrs...)

	// Only add to web UI if this level is enabled
	if slog.Default().Enabled(nil, slog.LevelError) {
		displayMsg := formatLogMessage("ERROR", msg, attrs...)
		r.state.AddLog("ERROR", component, displayMsg)
	}
}

// extractComponent extracts the "component" value from attrs
func extractComponent(attrs ...any) string {
	for i := 0; i < len(attrs); i += 2 {
		if i+1 < len(attrs) {
			if key, ok := attrs[i].(string); ok && key == "component" {
				if val, ok := attrs[i+1].(string); ok {
					return val
				}
			}
		}
	}
	return ""
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

// readFileContent reads and returns the content of a file, or empty string on error.
func readFileContent(file string) string {
	data, err := os.ReadFile(file)
	if err != nil {
		return ""
	}
	return string(data)
}

// detectProvisionType determines if a machine class uses auto or manual provisioning.
// Auto provision files contain "providerid:" in the autoprovision spec.
func detectProvisionType(file string) string {
	data, err := os.ReadFile(file)
	if err != nil {
		return ""
	}

	content := string(data)
	// Auto provision has "providerid:" field
	if strings.Contains(content, "providerid:") {
		return "auto"
	}
	// Manual provision uses matchlabels or has autoprovision: null
	return "manual"
}
