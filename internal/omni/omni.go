package omni

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// ============================================================
// Connectivity
// ============================================================

// CheckConnectivity verifies that omnictl can communicate with the Omni instance.
func CheckConnectivity() error {
	return run("omnictl", "get", "sysversion")
}

// ============================================================
// Version
// ============================================================

// GetOmnictlVersion returns the omnictl client version string.
func GetOmnictlVersion() string {
	cmd := exec.Command("omnictl", "--version")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "unknown"
	}
	// Format: "omnictl version v1.5.1 (API Version: 2)"
	output := strings.TrimSpace(string(out))
	if strings.Contains(output, "version") {
		parts := strings.Fields(output)
		for i, p := range parts {
			if p == "version" && i+1 < len(parts) {
				return parts[i+1]
			}
		}
	}
	return output
}

// GetOmniVersion returns the Omni server version string.
func GetOmniVersion() string {
	cmd := exec.Command("omnictl", "get", "sysversion", "-o", "yaml")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "unknown"
	}
	for _, line := range strings.Split(string(out), "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "backendversion:") {
			parts := strings.Fields(trimmed)
			if len(parts) >= 2 {
				return parts[1]
			}
		}
	}
	return "unknown"
}

// CompareVersions checks if the Omni server version is higher than omnictl.
// Returns true if there is a mismatch where server > client.
func CompareVersions(omniVersion, omnictlVersion string) bool {
	ov := normalizeVersion(omniVersion)
	cv := normalizeVersion(omnictlVersion)
	if ov == "" || cv == "" {
		return false
	}
	return ov > cv
}

// normalizeVersion strips the "v" prefix and any suffix after the semver.
func normalizeVersion(v string) string {
	v = strings.TrimPrefix(v, "v")
	// Take only major.minor.patch
	parts := strings.SplitN(v, ".", 3)
	if len(parts) < 3 {
		return v
	}
	// Remove anything after patch (e.g. "-rc1")
	patch := parts[2]
	if idx := strings.IndexAny(patch, "-+"); idx >= 0 {
		patch = patch[:idx]
	}
	return parts[0] + "." + parts[1] + "." + patch
}

// ============================================================
// Machine Classes
// ============================================================

// Apply applies a YAML file to Omni using omnictl apply.
// Supports multi-document YAML files separated by ---.
func Apply(file string) error {
	return run("omnictl", "apply", "-f", file)
}

// MachineClassDryRun runs a dry-run apply and returns the diff output.
// For machine classes, we compare the file spec with live state since omnictl
// doesn't provide a proper diff command for machine classes.
func MachineClassDryRun(file string) (string, error) {
	// First, try to run dry-run to validate the file
	cmd := exec.Command("omnictl", "apply", "-f", file, "--dry-run")
	out, err := cmd.CombinedOutput()
	output := string(out)

	// If there's an error, it's likely a validation error
	if err != nil {
		// Return the error, not as diff content
		return "", fmt.Errorf("%w: %s", err, strings.TrimSpace(output))
	}

	// Check if output indicates no changes
	if strings.Contains(output, "no changes") || strings.TrimSpace(output) == "" {
		return "", nil
	}

	// Check if it's creating a new resource
	if strings.Contains(output, "Creating resource") {
		// New resource - extract and return the spec
		lines := strings.Split(output, "\n")
		resourceLines := []string{}
		inResource := false

		for _, line := range lines {
			trimmed := strings.TrimSpace(line)

			// Skip status messages
			if strings.HasPrefix(trimmed, "Processing ") ||
				strings.HasPrefix(trimmed, "Syncing resources from:") ||
				strings.HasPrefix(trimmed, "Creating resource ") {
				continue
			}

			// Once we see metadata: or spec:, we're in the resource
			if strings.HasPrefix(trimmed, "metadata:") || strings.HasPrefix(trimmed, "spec:") {
				inResource = true
			}

			if inResource {
				// Skip metadata fields that always change
				if strings.HasPrefix(trimmed, "version:") ||
					strings.HasPrefix(trimmed, "created:") ||
					strings.HasPrefix(trimmed, "updated:") {
					continue
				}
				resourceLines = append(resourceLines, line)
			}
		}

		return strings.TrimSpace(strings.Join(resourceLines, "\n")), nil
	}

	// For updates, omnictl always dumps the full resource regardless of whether
	// anything actually changed. Compare the spec section of the file vs live
	// state to decide whether a real apply is needed.
	fileData, err := os.ReadFile(file)
	if err != nil {
		return output, nil // can't read file, trust dry-run
	}
	// Collect IDs mentioned in the file
	var fileIDs []string
	for _, line := range strings.Split(string(fileData), "\n") {
		t := strings.TrimSpace(line)
		if strings.HasPrefix(t, "id:") {
			parts := strings.Fields(t)
			if len(parts) >= 2 {
				fileIDs = append(fileIDs, parts[1])
			}
		}
	}
	for _, id := range fileIDs {
		liveContent, liveErr := GetLiveMachineClass(id)
		if liveErr != nil {
			return output, nil // resource missing live → needs apply
		}
		fileDoc := findYAMLDocForID(string(fileData), id)
		if extractSpecSection(fileDoc) != extractSpecSection(liveContent) {
			return output, nil // real change detected
		}
	}
	return "", nil // all specs match, nothing to do
}

// findYAMLDocForID returns the YAML document (within a multi-doc file) whose
// metadata.id matches id, or the whole content if no match is found.
func findYAMLDocForID(content, id string) string {
	for _, doc := range strings.Split(content, "\n---") {
		for _, line := range strings.Split(doc, "\n") {
			if strings.TrimSpace(line) == "id: "+id {
				return doc
			}
		}
	}
	return content
}

// extractSpecSection returns the trimmed, normalised lines of the spec: block
// from a single YAML document so two semantically identical specs compare equal
// regardless of indentation or blank lines.
func extractSpecSection(yaml string) string {
	lines := strings.Split(yaml, "\n")
	inSpec := false
	var result []string
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if !inSpec {
			if trimmed == "spec:" {
				inSpec = true
			}
			continue
		}
		// Stop when we hit another top-level key (no leading whitespace, not blank)
		if trimmed != "" && !strings.HasPrefix(line, " ") && !strings.HasPrefix(line, "\t") {
			break
		}
		if trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return strings.Join(result, "\n")
}

// GetMachineClassIDs returns all machine class IDs currently registered in Omni.
func GetMachineClassIDs() ([]string, error) {
	return getResourceIDs("omnictl", "get", "machineclasses", "-o", "yaml")
}

// GetLiveMachineClass gets the live machine class state from Omni.
// Returns the YAML content of the current machine class configuration.
func GetLiveMachineClass(id string) (string, error) {
	cmd := exec.Command("omnictl", "get", "machineclass", id, "-o", "yaml")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("%w: %s", err, strings.TrimSpace(string(out)))
	}
	return string(out), nil
}

// GetAllLiveMachineClasses fetches all machine classes in one call.
// Returns a map of machine class ID -> YAML content.
func GetAllLiveMachineClasses() (map[string]string, error) {
	cmd := exec.Command("omnictl", "get", "machineclasses", "-o", "yaml")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("%w: %s", err, strings.TrimSpace(string(out)))
	}
	return parseMultiDocYAML(string(out)), nil
}

// DeleteMachineClass deletes a machine class from Omni by id.
// Returns the command output so callers can check for "still in use" errors.
func DeleteMachineClass(id string) (string, error) {
	cmd := exec.Command("omnictl", "delete", "machineclasses", id)
	out, err := cmd.CombinedOutput()
	output := strings.TrimSpace(string(out))
	if err != nil {
		return output, fmt.Errorf("%w: %s", err, output)
	}
	return output, nil
}

// ============================================================
// Cluster Templates
// ============================================================

// ClusterTemplateValidate validates a cluster template file
// before syncing to prevent broken configs from being applied.
func ClusterTemplateValidate(file string) error {
	return runInDir(filepath.Dir(file), "omnictl", "cluster", "template", "validate", "-f", filepath.Base(file))
}

// ClusterTemplateSync syncs a cluster template to Omni.
// Handles both creating new clusters and updating existing ones.
func ClusterTemplateSync(file string) error {
	return runInDir(filepath.Dir(file), "omnictl", "cluster", "template", "sync", "-f", filepath.Base(file))
}

// ClusterTemplateDiff returns the diff output for a cluster template.
// Returns empty string if there are no changes.
func ClusterTemplateDiff(file string) (string, error) {
	cmd := exec.Command("omnictl", "cluster", "template", "diff", "-f", filepath.Base(file))
	cmd.Dir = filepath.Dir(file)
	out, err := cmd.CombinedOutput()
	output := strings.TrimSpace(string(out))
	if err != nil {
		// diff may return non-zero when there are differences, that's expected
		return output, nil
	}
	return output, nil
}

// ============================================================
// Clusters
// ============================================================

// GetClusterIDs returns all cluster IDs currently registered in Omni.
func GetClusterIDs() ([]string, error) {
	return getResourceIDs("omnictl", "get", "clusters", "-o", "yaml")
}

// ClusterStatus holds relevant status fields from omnictl get clusterstatus.
type ClusterStatus struct {
	Ready              bool
	KubernetesAPIReady bool
}

// GetAllClusterReadyStatuses fetches status fields for every cluster in one call.
// Returns a map of cluster ID -> ClusterStatus.
func GetAllClusterReadyStatuses() (map[string]ClusterStatus, error) {
	cmd := exec.Command("omnictl", "get", "clusterstatus", "-o", "yaml")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("%w: %s", err, strings.TrimSpace(string(out)))
	}
	return parseClusterStatuses(string(out)), nil
}

// parseClusterStatuses parses multi-doc YAML from omnictl get clusterstatus.
func parseClusterStatuses(yamlContent string) map[string]ClusterStatus {
	result := make(map[string]ClusterStatus)
	docs := strings.Split(yamlContent, "\n---\n")
	for _, doc := range docs {
		doc = strings.TrimSpace(doc)
		if doc == "" || doc == "---" {
			continue
		}
		var id string
		var st ClusterStatus
		inSpec := false
		for _, line := range strings.Split(doc, "\n") {
			trimmed := strings.TrimSpace(line)
			switch {
			case trimmed == "metadata:":
				inSpec = false
			case trimmed == "spec:":
				inSpec = true
			case !inSpec && strings.HasPrefix(trimmed, "id:"):
				parts := strings.Fields(trimmed)
				if len(parts) >= 2 {
					id = parts[1]
				}
			case inSpec && strings.HasPrefix(trimmed, "ready:"):
				parts := strings.Fields(trimmed)
				if len(parts) >= 2 {
					st.Ready = parts[1] == "true"
				}
			case inSpec && strings.HasPrefix(trimmed, "kubernetesapiready:"):
				parts := strings.Fields(trimmed)
				if len(parts) >= 2 {
					st.KubernetesAPIReady = parts[1] == "true"
				}
			}
		}
		if id != "" {
			result[id] = st
		}
	}
	return result
}

// DeleteCluster deletes a cluster from Omni by id.
func DeleteCluster(id string) error {
	return run("omnictl", "cluster", "delete", id)
}

// ExportCluster exports a cluster configuration as a cluster template YAML.
// Returns the YAML content that can be used as a cluster template.
func ExportCluster(id string) (string, error) {
	cmd := exec.Command("omnictl", "cluster", "template", "export", "-c", id)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("%w: %s", err, strings.TrimSpace(string(out)))
	}
	return string(out), nil
}

// GetLiveCluster gets the live cluster state from Omni (same as ExportCluster).
// Returns the YAML content of the current cluster configuration.
func GetLiveCluster(id string) (string, error) {
	return ExportCluster(id)
}

// GetAllLiveClusters fetches all cluster templates in parallel.
// Returns a map of cluster name -> YAML content.
func GetAllLiveClusters() (map[string]string, error) {
	ids, err := GetClusterIDs()
	if err != nil {
		return nil, err
	}

	if len(ids) == 0 {
		return make(map[string]string), nil
	}

	// Export clusters in parallel for better performance
	type result struct {
		id      string
		content string
	}

	resultChan := make(chan result, len(ids))
	for _, id := range ids {
		go func(clusterID string) {
			content, _ := ExportCluster(clusterID)
			resultChan <- result{id: clusterID, content: content}
		}(id)
	}

	// Collect results - use the cluster ID from GetClusterIDs, not from YAML parsing
	resultMap := make(map[string]string)
	for i := 0; i < len(ids); i++ {
		r := <-resultChan
		if r.content != "" {
			resultMap[r.id] = r.content
		}
	}

	return resultMap, nil
}

// IsClusterTemplateManaged checks if a cluster has the
// omni.sidero.dev/managed-by-cluster-templates annotation.
// This annotation is only visible when querying individual clusters.
func IsClusterTemplateManaged(id string) bool {
	cmd := exec.Command("omnictl", "get", "cluster", id, "-o", "yaml")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return false
	}
	return strings.Contains(string(out), "omni.sidero.dev/managed-by-cluster-templates")
}

// ============================================================
// Cluster Template Parsing
// ============================================================

// WorkerGroup holds information about a single Workers pool from a cluster template.
type WorkerGroup struct {
	Name         string
	Count        int
	MachineClass string
	Machines     []string
	Extensions   []string
}

// ClusterTemplateInfo holds parsed information from an exported cluster template.
type ClusterTemplateInfo struct {
	TalosVersion             string
	KubernetesVersion        string
	ControlPlaneName         string
	ControlPlaneCount        int
	ControlPlaneMachineClass string
	ControlPlaneMachines     []string
	ControlPlaneExtensions   []string
	WorkerGroups             []WorkerGroup
	// Extensions defined at the cluster level (kind: Cluster → systemExtensions)
	ClusterExtensions []string
	// Extensions defined per individual machine (kind: Machine → systemExtensions, keyed by machine name/UUID)
	MachineExtensions map[string][]string
}

// ParseClusterTemplate parses a cluster template YAML and extracts version and
// node group information. It supports two formats:
//   - The legacy `kind: ControlPlane / Workers` template format
//   - The Omni resource format (`type: MachineSets.omni.sidero.dev`) produced
//     by `omnictl cluster template render`
func ParseClusterTemplate(yamlContent string) ClusterTemplateInfo {
	// Detect format: resource format uses "type: MachineSets.omni.sidero.dev"
	for _, line := range strings.Split(yamlContent, "\n") {
		t := strings.TrimSpace(line)
		if strings.HasPrefix(t, "type: MachineSets.omni.sidero.dev") ||
			strings.HasPrefix(t, "type: Clusters.omni.sidero.dev") ||
			strings.HasPrefix(t, "type: ExtensionsConfigurations.omni.sidero.dev") {
			return parseResourceFormat(yamlContent)
		}
	}
	return parseLegacyFormat(yamlContent)
}

// parseLegacyFormat handles the `kind: Cluster / ControlPlane / Workers` template format.
func parseLegacyFormat(yamlContent string) ClusterTemplateInfo {
	var info ClusterTemplateInfo

	// Scan full content for version fields.
	var inTalos, inKubernetes bool
	for _, line := range strings.Split(yamlContent, "\n") {
		trimmed := strings.TrimSpace(line)
		if info.TalosVersion == "" && strings.HasPrefix(trimmed, "talosVersion:") {
			info.TalosVersion = strings.TrimSpace(strings.TrimPrefix(trimmed, "talosVersion:"))
		}
		if info.KubernetesVersion == "" && strings.HasPrefix(trimmed, "kubernetesVersion:") {
			info.KubernetesVersion = strings.TrimSpace(strings.TrimPrefix(trimmed, "kubernetesVersion:"))
		}
		if trimmed == "talos:" {
			inTalos = true
			inKubernetes = false
			continue
		}
		if trimmed == "kubernetes:" {
			inKubernetes = true
			inTalos = false
			continue
		}
		if strings.HasPrefix(trimmed, "version:") {
			v := strings.TrimSpace(strings.TrimPrefix(trimmed, "version:"))
			if inTalos && info.TalosVersion == "" {
				info.TalosVersion = v
			}
			if inKubernetes && info.KubernetesVersion == "" {
				info.KubernetesVersion = v
			}
			inTalos = false
			inKubernetes = false
			continue
		}
		if trimmed != "" && !strings.HasPrefix(trimmed, "#") {
			inTalos = false
			inKubernetes = false
		}
	}

	machineExtensions := make(map[string][]string)

	for _, doc := range strings.Split(yamlContent, "\n---") {
		doc = strings.TrimSpace(doc)
		if doc == "" {
			continue
		}

		var kind, docName string
		var inMachines, inMachineClass, inSystemExtensions bool
		var machineClassSize int
		var machineClassName string
		var machines, systemExtensions []string

		for _, line := range strings.Split(doc, "\n") {
			trimmed := strings.TrimSpace(line)
			if strings.HasPrefix(trimmed, "kind:") {
				kind = strings.TrimSpace(strings.TrimPrefix(trimmed, "kind:"))
				inMachines, inMachineClass, inSystemExtensions = false, false, false
				continue
			}
			if !inMachineClass && len(line) > 0 && line[0] != ' ' && line[0] != '\t' && strings.HasPrefix(trimmed, "name:") {
				docName = strings.TrimSpace(strings.TrimPrefix(trimmed, "name:"))
				continue
			}
			switch kind {
			case "ControlPlane", "Workers":
				if strings.HasPrefix(trimmed, "machines:") {
					inMachines = true
					inMachineClass, inSystemExtensions = false, false
				} else if strings.HasPrefix(trimmed, "machineClass:") {
					inMachineClass = true
					inMachines, inSystemExtensions = false, false
				} else if strings.HasPrefix(trimmed, "systemExtensions:") {
					inSystemExtensions = true
					inMachines, inMachineClass = false, false
				} else if inMachines && strings.HasPrefix(trimmed, "- ") {
					machines = append(machines, strings.TrimSpace(strings.TrimPrefix(trimmed, "- ")))
				} else if inSystemExtensions && strings.HasPrefix(trimmed, "- ") {
					systemExtensions = append(systemExtensions, strings.TrimSpace(strings.TrimPrefix(trimmed, "- ")))
				} else if inMachineClass && strings.HasPrefix(trimmed, "name:") {
					machineClassName = strings.TrimSpace(strings.TrimPrefix(trimmed, "name:"))
				} else if strings.HasPrefix(trimmed, "size:") {
					parts := strings.Fields(trimmed)
					if len(parts) >= 2 {
						machineClassSize, _ = strconv.Atoi(parts[1])
					}
				}
			case "Cluster", "Machine":
				// Reset section on any top-level non-list key
				if len(line) > 0 && line[0] != ' ' && line[0] != '\t' && !strings.HasPrefix(trimmed, "- ") {
					if strings.HasPrefix(trimmed, "systemExtensions:") {
						inSystemExtensions = true
					} else {
						inSystemExtensions = false
					}
				} else if inSystemExtensions && strings.HasPrefix(trimmed, "- ") {
					systemExtensions = append(systemExtensions, strings.TrimSpace(strings.TrimPrefix(trimmed, "- ")))
				}
			}
		}

		switch kind {
		case "ControlPlane":
			if info.ControlPlaneName == "" {
				info.ControlPlaneName = "control-planes"
			}
			if machineClassName != "" {
				info.ControlPlaneMachineClass = machineClassName
				info.ControlPlaneCount = machineClassSize
			} else {
				info.ControlPlaneMachines = machines
				info.ControlPlaneCount = len(machines)
			}
			info.ControlPlaneExtensions = systemExtensions
		case "Workers":
			wg := WorkerGroup{Name: docName}
			if machineClassName != "" {
				wg.MachineClass = machineClassName
				wg.Count = machineClassSize
			} else {
				wg.Machines = machines
				wg.Count = len(machines)
			}
			wg.Extensions = systemExtensions
			info.WorkerGroups = append(info.WorkerGroups, wg)
		case "Cluster":
			if len(systemExtensions) > 0 {
				info.ClusterExtensions = append(info.ClusterExtensions, systemExtensions...)
			}
		case "Machine":
			if docName != "" && len(systemExtensions) > 0 {
				machineExtensions[docName] = append(machineExtensions[docName], systemExtensions...)
			}
		}
	}

	if len(machineExtensions) > 0 {
		info.MachineExtensions = machineExtensions
	}

	return info
}

// parseResourceFormat handles the Omni resource format produced by
// `omnictl cluster template render`, which uses typed documents with
// metadata labels to express roles and relationships.
func parseResourceFormat(yamlContent string) ClusterTemplateInfo {
	var info ClusterTemplateInfo

	type msEntry struct {
		id           string
		clusterID    string
		isCP         bool
		isWorker     bool
		machineClass string
		count        int
	}
	type extEntry struct {
		machineSetID string
		extensions   []string
	}

	var msEntries []msEntry
	var extEntries []extEntry

	for _, doc := range strings.Split(yamlContent, "\n---") {
		doc = strings.TrimSpace(doc)
		if doc == "" {
			continue
		}

		var docType, docID, clusterID, machineSetID string
		var isCP, isWorker bool
		var machineClass string
		var machineCount int
		var extensions []string
		var section, subSection string

		for _, line := range strings.Split(doc, "\n") {
			if strings.TrimSpace(line) == "" {
				continue
			}
			indent := len(line) - len(strings.TrimLeft(line, " \t"))
			trimmed := strings.TrimSpace(line)

			if indent == 0 {
				switch trimmed {
				case "metadata:":
					section = "metadata"
				case "spec:":
					section = "spec"
				default:
					section = ""
				}
				subSection = ""
				continue
			}

			if indent == 2 {
				switch section {
				case "metadata":
					switch {
					case trimmed == "labels:":
						subSection = "labels"
					case strings.HasPrefix(trimmed, "type:"):
						docType = strings.TrimSpace(strings.TrimPrefix(trimmed, "type:"))
						subSection = ""
					case strings.HasPrefix(trimmed, "id:"):
						docID = strings.TrimSpace(strings.TrimPrefix(trimmed, "id:"))
						subSection = ""
					default:
						subSection = ""
					}
				case "spec":
					switch {
					case strings.HasPrefix(trimmed, "machineallocation:"):
						subSection = "machineallocation"
					case strings.HasPrefix(trimmed, "extensions:"):
						subSection = "extensions"
					case strings.HasPrefix(trimmed, "talosversion:"):
						if info.TalosVersion == "" {
							info.TalosVersion = strings.TrimSpace(strings.TrimPrefix(trimmed, "talosversion:"))
						}
						subSection = ""
					case strings.HasPrefix(trimmed, "kubernetesversion:"):
						if info.KubernetesVersion == "" {
							info.KubernetesVersion = strings.TrimSpace(strings.TrimPrefix(trimmed, "kubernetesversion:"))
						}
						subSection = ""
					default:
						subSection = ""
					}
				}
				continue
			}

			if indent == 4 {
				switch subSection {
				case "labels":
					switch {
					case strings.HasPrefix(trimmed, "omni.sidero.dev/cluster:"):
						clusterID = strings.TrimSpace(strings.TrimPrefix(trimmed, "omni.sidero.dev/cluster:"))
					case strings.HasPrefix(trimmed, "omni.sidero.dev/role-controlplane"):
						isCP = true
					case strings.HasPrefix(trimmed, "omni.sidero.dev/role-worker"):
						isWorker = true
					case strings.HasPrefix(trimmed, "omni.sidero.dev/machine-set:"):
						machineSetID = strings.TrimSpace(strings.TrimPrefix(trimmed, "omni.sidero.dev/machine-set:"))
					}
				case "machineallocation":
					switch {
					case strings.HasPrefix(trimmed, "name:"):
						machineClass = strings.TrimSpace(strings.TrimPrefix(trimmed, "name:"))
					case strings.HasPrefix(trimmed, "machinecount:"):
						parts := strings.Fields(trimmed)
						if len(parts) >= 2 {
							machineCount, _ = strconv.Atoi(parts[1])
						}
					}
				case "extensions":
					if strings.HasPrefix(trimmed, "- ") {
						extensions = append(extensions, strings.TrimSpace(strings.TrimPrefix(trimmed, "- ")))
					}
				}
			}
		}

		switch {
		case strings.Contains(docType, "MachineSets"):
			msEntries = append(msEntries, msEntry{
				id: docID, clusterID: clusterID,
				isCP: isCP, isWorker: isWorker,
				machineClass: machineClass, count: machineCount,
			})
		case strings.Contains(docType, "ExtensionsConfigurations"):
			if machineSetID != "" {
				extEntries = append(extEntries, extEntry{
					machineSetID: machineSetID,
					extensions:   extensions,
				})
			}
		}
	}

	// Build extension lookup by machine-set ID
	extMap := make(map[string][]string)
	for _, e := range extEntries {
		extMap[e.machineSetID] = e.extensions
	}

	for _, ms := range msEntries {
		exts := extMap[ms.id]
		// Strip cluster-id prefix from the display name (e.g. "cluster-6-workers" → "workers")
		displayName := ms.id
		if ms.clusterID != "" && strings.HasPrefix(displayName, ms.clusterID+"-") {
			displayName = displayName[len(ms.clusterID)+1:]
		}
		switch {
		case ms.isCP:
			info.ControlPlaneName = displayName
			info.ControlPlaneCount = ms.count
			info.ControlPlaneMachineClass = ms.machineClass
			info.ControlPlaneExtensions = exts
		case ms.isWorker:
			info.WorkerGroups = append(info.WorkerGroups, WorkerGroup{
				Name:         displayName,
				Count:        ms.count,
				MachineClass: ms.machineClass,
				Extensions:   exts,
			})
		}
	}

	return info
}

// ============================================================
// Helpers
// ============================================================

// run executes a command and returns an error with output if it fails.
func run(name string, args ...string) error {
	cmd := exec.Command(name, args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%w: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

// runInDir executes a command in a specific directory and returns an error with output if it fails.
func runInDir(dir, name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%w: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

// runWithRetry executes a command with retry logic for transient errors.
func runWithRetry(name string, args ...string) ([]byte, error) {
	maxRetries := 3
	baseDelay := 500 * time.Millisecond

	for attempt := 0; attempt < maxRetries; attempt++ {
		cmd := exec.Command(name, args...)
		out, err := cmd.CombinedOutput()

		if err == nil {
			return out, nil
		}

		// Check if it's a transient error (missing content-type, connection issues)
		output := strings.TrimSpace(string(out))
		isTransient := strings.Contains(output, "missing HTTP content-type") ||
			strings.Contains(output, "connection refused") ||
			strings.Contains(output, "connection reset") ||
			strings.Contains(output, "timeout")

		if !isTransient || attempt == maxRetries-1 {
			return out, fmt.Errorf("%w: %s", err, output)
		}

		// Wait before retrying with exponential backoff
		delay := baseDelay * time.Duration(1<<uint(attempt))
		time.Sleep(delay)
	}

	return nil, fmt.Errorf("max retries exceeded")
}

// getResourceIDs executes a command and parses YAML output for id fields.
func getResourceIDs(name string, args ...string) ([]string, error) {
	out, err := runWithRetry(name, args...)
	if err != nil {
		return nil, err
	}
	return parseIDs(string(out)), nil
}

// parseMultiDocYAML splits YAML output by document separator and extracts individual resources.
// Returns a map of resource ID -> full YAML document.
// For cluster templates, uses "name:" field instead of "id:".
func parseMultiDocYAML(yamlContent string) map[string]string {
	result := make(map[string]string)

	// Split by YAML document separator
	docs := strings.Split(yamlContent, "\n---\n")

	for _, doc := range docs {
		doc = strings.TrimSpace(doc)
		if doc == "" || doc == "---" {
			continue
		}

		// Extract ID from the document (try both "id:" and "name:")
		id := extractIDFromYAML(doc)
		if id != "" {
			result[id] = doc
		}
	}

	return result
}

// extractIDFromYAML extracts the resource ID from a YAML document.
// Looks for "id: <value>" or "name: <value>" patterns (for cluster templates).
func extractIDFromYAML(yamlDoc string) string {
	lines := strings.Split(yamlDoc, "\n")
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		// Check for id: first (machine classes)
		if strings.HasPrefix(trimmed, "id:") {
			parts := strings.Fields(trimmed)
			if len(parts) >= 2 {
				return parts[1]
			}
		}
		// Check for name: (cluster templates)
		if strings.HasPrefix(trimmed, "name:") {
			parts := strings.Fields(trimmed)
			if len(parts) >= 2 {
				return parts[1]
			}
		}
	}
	return ""
}

// parseIDs extracts all id values from YAML output lines like "  id: some-id".
func parseIDs(yaml string) []string {
	var ids []string
	for _, line := range strings.Split(yaml, "\n") {
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
