package reconciler

import (
	"fmt"
	"path/filepath"
	"strings"

	"omni-cd/internal/omni"
	"omni-cd/internal/state"
)

// ============================================================
// Machine Classes — Apply / Diff (shared implementation)
// ============================================================

// ApplyMachineClasses applies all machine class YAML files from the given
// directory. Delegates to processMachineClasses with applyChanges=true.
// forceMCIDs (may be nil) is a set of MC IDs that must be applied even when
// their per-MC auto-sync preference is disabled.
// force=true means the reconcile was explicitly requested (e.g. repo added, Sync
// button); MCs whose AutoSync has never been explicitly set (nil) are applied.
func (r *Reconciler) ApplyMachineClasses(dir string, crossRepoDuplicates map[string]string, forceMCIDs map[string]bool, force bool) {
	r.processMachineClasses(dir, true, crossRepoDuplicates, forceMCIDs, force)
}

// DiffMachineClasses checks all machine class YAML files for drift without
// applying any changes. Delegates to processMachineClasses with applyChanges=false.
func (r *Reconciler) DiffMachineClasses(dir string) {
	r.processMachineClasses(dir, false, nil, nil, false)
}

// processMachineClasses is the shared implementation for ApplyMachineClasses and
// DiffMachineClasses.
//
//   - applyChanges=true:  apply diffs to Omni when auto-sync is enabled for that MC.
//   - applyChanges=false: report diffs as "outofsync" only (diff-only refresh).
//
// crossRepoDuplicates (may be nil) maps MC IDs that appear in multiple repos to a
// human-readable list of those repo names; those MCs are blocked from being applied.
// forceMCIDs (may be nil) is a set of MC IDs to apply regardless of auto-sync setting.
func (r *Reconciler) processMachineClasses(dir string, applyChanges bool, crossRepoDuplicates map[string]string, forceMCIDs map[string]bool, force bool) {
	repoName := repoNameFromDir(dir)

	files, err := findYAMLFiles(dir)
	if err != nil {
		r.logWarn("Directory not found, skipping", "component", "MachineClasses", "path", dir)
		return
	}
	if len(files) == 0 {
		r.logWarn("No YAML files found", "component", "MachineClasses", "path", dir)
		return
	}

	if applyChanges {
		idCount := 0
		for _, f := range files {
			idCount += len(extractAllIDs(f))
		}
		r.logInfo("Syncing machine classes", "component", "MachineClasses", "count", idCount)
	} else {
		r.logInfo("Checking machine classes for drift (refresh only)", "component", "MachineClasses")
	}

	// Detect duplicate IDs within this directory.
	idToFiles := make(map[string][]string)
	for _, f := range files {
		for _, id := range extractAllIDs(f) {
			idToFiles[id] = append(idToFiles[id], f)
		}
	}
	duplicateIDs := make(map[string]bool)
	repoRoot := filepath.Dir(dir)
	var resources []state.ResourceInfo
	inSync, outOfSync, applied, failed := 0, 0, 0, 0

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

	// Also block MCs that conflict across repos (#9).
	for id, repos := range crossRepoDuplicates {
		if duplicateIDs[id] {
			continue // already handled above
		}
		if _, inThisRepo := idToFiles[id]; !inThisRepo {
			continue
		}
		duplicateIDs[id] = true
		errMsg := fmt.Sprintf("Conflict: machine class '%s' is managed by multiple repositories (%s) — remove it from all but one to resolve", id, repos)
		r.logError("Cross-repo machine class conflict, skipping sync", "component", "MachineClasses", "id", id, "repos", repos)
		resources = append(resources, state.ResourceInfo{
			ID:     id,
			Type:   "MachineClass",
			Status: "failed",
			Error:  errMsg,
		})
		failed++
	}

	// Batch fetch all live machine class states once.
	allLiveStates, _ := omni.GetAllLiveMachineClasses()

	for _, file := range files {
		ids := extractAllIDs(file)
		// Filter out duplicate IDs (within-repo or cross-repo conflicts).
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

		diffOutput, dryRunErr := omni.MachineClassDryRun(file)
		fileContent := readFileContent(file)

		// Validation failure — always fail regardless of applyChanges.
		if dryRunErr != nil {
			r.logError("Machine class validation failed", "component", "MachineClasses", "ids", strings.Join(ids, ", "), "error", dryRunErr)
			for _, id := range ids {
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

		// No diff — always success.
		if diffOutput == "" || strings.Contains(diffOutput, "no changes") {
			r.logDebug("Machine classes up to date", "component", "MachineClasses", "ids", strings.Join(ids, ", "))
			for _, id := range ids {
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
			inSync += len(ids)
			continue
		}

		// There is a diff. Check per-MC auto-sync preference (#8).
		// All IDs in this file share the same physical template, so use the
		// first ID's setting as representative for the file.
		// forceMCIDs bypasses the auto-sync gate for explicitly requested syncs.
		// force=true (e.g. repo added, Sync button) also applies MCs whose AutoSync
		// has never been explicitly set (nil), so a newly added repo's MCs are applied
		// immediately without requiring the user to enable AutoSync for each one.
		mcForced := forceMCIDs != nil && forceMCIDs[ids[0]]
		mcAutoSyncEnabled := !applyChanges || r.state.GetMachineClassAutoSync(ids[0]) || mcForced ||
			(force && !r.state.IsMachineClassAutoSyncExplicitlyDisabled(ids[0]))

		shouldApply := applyChanges && mcAutoSyncEnabled
		if shouldApply {
			if err := omni.Apply(file); err != nil {
				r.logError("Machine class apply failed", "component", "MachineClasses", "ids", strings.Join(ids, ", "), "error", err)
				for _, id := range ids {
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
		} else {
			// applyChanges=false (diff-only refresh) or auto-sync disabled for this MC.
			if applyChanges {
				r.logWarn("Machine class out of sync (auto-sync disabled)", "component", "MachineClasses", "ids", strings.Join(ids, ", "))
			} else {
				r.logWarn("Machine class out of sync (refresh only, skipping apply)", "component", "MachineClasses", "ids", strings.Join(ids, ", "))
			}
			for _, id := range ids {
				liveContent := allLiveStates[id]
				if liveContent == "" {
					liveContent, _ = omni.GetLiveMachineClass(id)
				}
				resources = append(resources, state.ResourceInfo{
					ID:            id,
					Type:          "MachineClass",
					Status:        "outofsync",
					ProvisionType: provisionType,
					Diff:          diffOutput,
					FileContent:   fileContent,
					LiveContent:   liveContent,
				})
			}
			outOfSync += len(ids)
		}
	}

	// Stamp RepoName on all resources so the UI can correlate them with their repo.
	if repoName != "" {
		for i := range resources {
			resources[i].RepoName = repoName
		}
	}

	// Record which machine class IDs are managed by this repo so failing-sync
	// cycles can protect them from deletion.
	if repoName != "" {
		mcIDs := make([]string, 0, len(resources))
		for _, res := range resources {
			mcIDs = append(mcIDs, res.ID)
		}
		r.state.SetRepoMachineClasses(repoName, mcIDs)
	}

	r.state.MergeMachineClasses(resources)
	if applyChanges {
		r.logInfo("Machine classes result", "component", "MachineClasses", "synced", applied, "out_of_sync", outOfSync, "failed", failed)
	} else {
		r.logInfo("Machine class diff result", "component", "MachineClasses", "in_sync", inSync, "out_of_sync", outOfSync, "failed", failed)
	}
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

	allTrackedMC := r.state.GetAllTrackedMachineClassIDs()

	deleted, failed := 0, 0
	for _, id := range existingIDs {
		if contains(desiredIDs, id) {
			continue
		}
		// Only delete MCs that omni-cd has previously owned.
		if !allTrackedMC[id] {
			r.logDebug("Machine class not tracked by omni-cd, skipping delete", "component", "MachineClasses", "id", id)
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

// DeleteMachineClassesAll is the multi-repo variant of DeleteMachineClasses.
// It collects desired IDs from all provided directories and deletes any
// machine class that is not present in any of them.
func (r *Reconciler) DeleteMachineClassesAll(dirs []string) {
	var desiredIDs []string
	for _, dir := range dirs {
		desiredIDs = append(desiredIDs, collectMachineClassIDs(dir)...)
	}

	existingIDs, err := omni.GetMachineClassIDs()
	if err != nil {
		r.logError("Failed to list machine classes", "component", "MachineClasses", "error", err)
		return
	}

	r.logInfo("Checking for machine classes to delete (multi-repo)", "component", "MachineClasses")

	allTrackedMCAll := r.state.GetAllTrackedMachineClassIDs()

	deleted, failed := 0, 0
	for _, id := range existingIDs {
		if contains(desiredIDs, id) {
			continue
		}
		// Skip machine classes whose repo is currently failing to sync.
		if r.protectedMCs[id] {
			continue
		}
		// Only delete MCs that omni-cd has previously owned.
		if !allTrackedMCAll[id] {
			r.logDebug("Machine class not tracked by omni-cd, skipping delete", "component", "MachineClasses", "id", id)
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

// CollectUnmanagedMachineClasses is the exported entry point for post-reconcile
// unmanaged-machine-class detection across all repo dirs. Call once after all
// per-repo ApplyMachineClasses calls are done.
// allLiveStates is a pre-fetched id->YAML map; pass nil to fetch from the API.
func (r *Reconciler) CollectUnmanagedMachineClasses(dirs []string, allLiveStates map[string]string) {
	// Build the set of IDs managed by any repo.
	managedIDs := make(map[string]bool)
	for _, dir := range dirs {
		for _, id := range collectMachineClassIDs(dir) {
			managedIDs[id] = true
		}
	}
	// Treat MCs from failing repos as managed so they are not marked unmanaged.
	for id := range r.protectedMCs {
		managedIDs[id] = true
	}

	if allLiveStates == nil {
		var err error
		allLiveStates, err = omni.GetAllLiveMachineClasses()
		if err != nil {
			r.logError("Failed to list live machine classes for unmanaged detection", "component", "MachineClasses", "error", err)
			return
		}
	}

	// Start from current state; drop any that have disappeared from Omni.
	existing := r.state.GetMachineClasses()
	var final []state.ResourceInfo
	for _, mc := range existing {
		if _, stillInOmni := allLiveStates[mc.ID]; !stillInOmni && !managedIDs[mc.ID] {
			// Gone from Omni and not in git — remove from state.
			continue
		}
		if !managedIDs[mc.ID] {
			// In Omni but not in any git repo — mark unmanaged and disable auto-sync.
			mc.Status = "unmanaged"
			mc.Diff = ""
			disabled := false
			mc.AutoSync = &disabled
		}
		final = append(final, mc)
	}

	// Add newly discovered unmanaged MCs not yet in state.
	existingMap := make(map[string]bool, len(existing))
	for _, mc := range existing {
		existingMap[mc.ID] = true
	}
	for id, liveContent := range allLiveStates {
		if !existingMap[id] && !managedIDs[id] {
			final = append(final, state.ResourceInfo{
				ID:            id,
				Type:          "MachineClass",
				Status:        "unmanaged",
				ProvisionType: detectProvisionTypeFromString(liveContent),
				LiveContent:   liveContent,
			})
		}
	}

	r.state.SetMachineClasses(final)
}

// DeleteSingleMachineClass deletes a specific machine class from Omni by ID.
// It only acts on machine classes tracked by omni-cd (unmanaged or previously managed).
func (r *Reconciler) DeleteSingleMachineClass(id string) {
	mcs := r.state.GetMachineClasses()
	var found bool
	for _, mc := range mcs {
		if mc.ID == id {
			found = true
			break
		}
	}
	if !found {
		r.logWarn("Machine class not found in state, skipping delete", "component", "MachineClasses", "id", id)
		return
	}

	r.logWarn("Deleting machine class", "component", "MachineClasses", "id", id)
	output, err := omni.DeleteMachineClass(id)
	if err != nil {
		if strings.Contains(output, "still in use") {
			r.logWarn("Machine class still in use, cannot delete", "component", "MachineClasses", "id", id)
		} else {
			r.logError("Machine class delete failed", "component", "MachineClasses", "id", id, "output", output, "error", err)
		}
		return
	}

	r.logInfo("Machine class deleted", "component", "MachineClasses", "id", id)

	// Remove from state
	existing := r.state.GetMachineClasses()
	var final []state.ResourceInfo
	for _, mc := range existing {
		if mc.ID != id {
			final = append(final, mc)
		}
	}
	r.state.SetMachineClasses(final)
	r.state.Save()
}

// DetectCrossRepoDuplicatesMC returns a map of machine-class IDs that appear
// in more than one repo directory, mapping each duplicate ID to a
// comma-separated list of the repo names that define it.
func (r *Reconciler) DetectCrossRepoDuplicatesMC(dirs []string) map[string]string {
	return detectCrossRepoDuplicatesMC(dirs)
}

// forceDeleteMachineClasses deletes all machine classes declared in the given
// directories whose auto-sync is enabled. Called when a repo is removed.
func (r *Reconciler) forceDeleteMachineClasses(dirs []string) {
	var mcIDs []string
	for _, dir := range dirs {
		mcIDs = append(mcIDs, collectMachineClassIDs(dir)...)
	}
	r.logInfo("Force-delete: machine classes found in deleted repo dirs", "component", "MachineClasses", "count", len(mcIDs), "ids", mcIDs, "dirs", dirs)
	for _, id := range mcIDs {
		if !r.state.GetMachineClassAutoSync(id) {
			r.logInfo("Repo deleted but MC auto-sync disabled, leaving as unmanaged", "component", "MachineClasses", "id", id)
			r.state.UpdateMachineClassStatus(id, "unmanaged")
			continue
		}
		r.logWarn("Force-deleting machine class from deleted repo", "component", "MachineClasses", "id", id)
		output, err := omni.DeleteMachineClass(id)
		if err != nil {
			if strings.Contains(output, "still in use") {
				r.logWarn("Machine class still in use, skipping force-delete", "component", "MachineClasses", "id", id)
				r.state.UpdateMachineClassStatus(id, "unmanaged")
			} else {
				r.logError("Force-delete of machine class failed", "component", "MachineClasses", "id", id, "output", output)
			}
		} else {
			r.logInfo("Machine class force-deleted", "component", "MachineClasses", "id", id)
			r.state.RemoveMachineClass(id)
		}
	}
}
