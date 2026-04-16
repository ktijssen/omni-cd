package reconciler

import (
	"os"
	"path/filepath"
	"sort"
	"strings"

	"omni-cd/internal/omni"
	"omni-cd/internal/state"
)

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

// extractHostnames filters a global UUID->hostname map to only the machine UUIDs
// present in the given ControlPlane and Workers NodeGroups.
func extractHostnames(all map[string]string, cp state.NodeGroup, workers []state.NodeGroup) map[string]string {
	if len(all) == 0 {
		return nil
	}
	result := make(map[string]string)
	for _, id := range cp.Machines {
		if hn, ok := all[id]; ok {
			result[id] = hn
		}
	}
	for _, wg := range workers {
		for _, id := range wg.Machines {
			if hn, ok := all[id]; ok {
				result[id] = hn
			}
		}
	}
	if len(result) == 0 {
		return nil
	}
	return result
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

// hydratePoolMachines enriches NodeGroups that have no explicit machine list
// by fetching the actual MachineSetNode resources from Omni and populating the
// Machines field with real machine UUIDs.  NodeGroups that already have explicit
// machine lists (manual provisioning) are left untouched.
func hydratePoolMachines(clusterID string, cp state.NodeGroup, workers []state.NodeGroup) (state.NodeGroup, []state.NodeGroup) {
	needsHydration := len(cp.Machines) == 0
	if !needsHydration {
		for _, wg := range workers {
			if len(wg.Machines) == 0 {
				needsHydration = true
				break
			}
		}
	}
	if !needsHydration {
		return cp, workers
	}

	msNodes, err := omni.GetMachineSetNodes(clusterID)
	if err != nil || len(msNodes) == 0 {
		return cp, workers
	}

	if len(cp.Machines) == 0 {
		msID := clusterID + "-" + cp.Name
		if uuids, ok := msNodes[msID]; ok {
			cp.Machines = uuids
		} else {
			// Fallback: case-insensitive name match, then any control-plane key.
			cpNameLower := strings.ToLower(cp.Name)
			for k, v := range msNodes {
				if strings.HasPrefix(k, clusterID+"-") {
					suffix := strings.TrimPrefix(k, clusterID+"-")
					if strings.ToLower(suffix) == cpNameLower {
						cp.Machines = v
						break
					}
				}
			}
			if len(cp.Machines) == 0 {
				for k, v := range msNodes {
					if strings.HasPrefix(k, clusterID+"-") && strings.Contains(k, "control-plane") {
						cp.Machines = v
						break
					}
				}
			}
		}
	}

	// Collect all non-CP machineset keys for this cluster, sorted for
	// deterministic positional assignment when names are absent.
	var workerKeys []string
	for k := range msNodes {
		if strings.HasPrefix(k, clusterID+"-") && !strings.Contains(k, "control-plane") {
			workerKeys = append(workerKeys, k)
		}
	}
	sort.Strings(workerKeys)

	usedKeys := make(map[string]bool)

	// Pass 1: assign by name — exact key, then case-insensitive suffix match.
	for i, wg := range workers {
		if len(workers[i].Machines) > 0 {
			continue
		}
		msID := clusterID + "-" + wg.Name
		if uuids, ok := msNodes[msID]; ok {
			workers[i].Machines = uuids
			usedKeys[msID] = true
			continue
		}
		if wg.Name == "" {
			continue
		}
		nameLower := strings.ToLower(wg.Name)
		for _, k := range workerKeys {
			if !usedKeys[k] {
				suffix := strings.TrimPrefix(k, clusterID+"-")
				if strings.ToLower(suffix) == nameLower {
					workers[i].Machines = msNodes[k]
					usedKeys[k] = true
					break
				}
			}
		}
	}

	// Pass 2: distribute remaining (unmatched) machinesets to workers that
	// still have no machines — covers unnamed pool-based worker groups.
	ri := 0
	for i := range workers {
		if len(workers[i].Machines) > 0 {
			continue
		}
		for ri < len(workerKeys) {
			k := workerKeys[ri]
			ri++
			if !usedKeys[k] {
				workers[i].Machines = msNodes[k]
				usedKeys[k] = true
				break
			}
		}
	}

	// Pass 3: create worker groups for any worker machinesets that were not
	// matched to an existing group. This handles the case where liveContent was
	// empty (ExportCluster failed) so parseLegacyFormat returned zero worker
	// groups — the topology would otherwise show no workers at all.
	for _, k := range workerKeys {
		if !usedKeys[k] {
			uuids := msNodes[k]
			if len(uuids) > 0 {
				name := strings.TrimPrefix(k, clusterID+"-")
				workers = append(workers, state.NodeGroup{
					Name:     name,
					Machines: uuids,
					Count:    len(uuids),
				})
				usedKeys[k] = true
			}
		}
	}

	// Deduplicate UUIDs across groups — CP takes priority, then workers in
	// order. Prevents the same machine appearing in multiple machinesets.
	seen := make(map[string]bool)
	filtered := cp.Machines[:0:0]
	for _, id := range cp.Machines {
		if !seen[id] {
			seen[id] = true
			filtered = append(filtered, id)
		}
	}
	cp.Machines = filtered
	for i := range workers {
		wFiltered := workers[i].Machines[:0:0]
		for _, id := range workers[i].Machines {
			if !seen[id] {
				seen[id] = true
				wFiltered = append(wFiltered, id)
			}
		}
		workers[i].Machines = wFiltered
	}

	// For manually-assigned clusters (no machineallocation count in YAML), derive
	// Count from the actual number of MachineSetNode entries that were hydrated.
	if cp.Count == 0 && len(cp.Machines) > 0 {
		cp.Count = len(cp.Machines)
	}
	for i := range workers {
		if workers[i].Count == 0 && len(workers[i].Machines) > 0 {
			workers[i].Count = len(workers[i].Machines)
		}
	}

	return cp, workers
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

// detectCrossRepoDuplicates scans all cluster directories and returns a map of
// cluster IDs that appear in more than one repo, keyed by ID with the value
// being a comma-separated list of repo names for use in error messages.
func detectCrossRepoDuplicates(dirs []string) map[string]string {
	first := make(map[string]string) // id -> first repo name
	dupes := make(map[string]string) // id -> "repo-a, repo-b, ..."
	for _, dir := range dirs {
		repoName := repoNameFromDir(dir)
		for _, id := range collectClusterIDs(dir) {
			if firstRepo, ok := first[id]; ok {
				if _, already := dupes[id]; !already {
					dupes[id] = firstRepo + ", " + repoName
				} else {
					dupes[id] += ", " + repoName
				}
			} else {
				first[id] = repoName
			}
		}
	}
	return dupes
}

// detectCrossRepoDuplicatesMC scans all machine-class directories and returns a
// map of MC IDs that appear in more than one repo, keyed by ID with the value
// being a comma-separated list of repo names for use in error messages.
func detectCrossRepoDuplicatesMC(dirs []string) map[string]string {
	first := make(map[string]string)
	dupes := make(map[string]string)
	for _, dir := range dirs {
		repoName := repoNameFromDir(dir)
		for _, id := range collectMachineClassIDs(dir) {
			if firstRepo, ok := first[id]; ok {
				if _, already := dupes[id]; !already {
					dupes[id] = firstRepo + ", " + repoName
				} else {
					dupes[id] += ", " + repoName
				}
			} else {
				first[id] = repoName
			}
		}
	}
	return dupes
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
	return detectProvisionTypeFromString(string(data))
}

func detectProvisionTypeFromString(content string) string {
	// Auto provision has "providerid:" field
	if strings.Contains(strings.ToLower(content), "providerid:") {
		return "auto"
	}
	// Manual provision uses matchlabels or has autoprovision: null
	return "manual"
}

// repoNameFromDir extracts the logical repo name from a work-dir path of the
// form /tmp/repo-<name>[/...].
func repoNameFromDir(dir string) string {
	const prefix = "/tmp/repo-"
	if parts := strings.SplitN(dir, prefix, 2); len(parts) == 2 {
		if slash := strings.Index(parts[1], "/"); slash >= 0 {
			return parts[1][:slash]
		}
		return parts[1]
	}
	return ""
}
