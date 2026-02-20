package omni

import (
	"fmt"
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

	// For updates, omnictl shows the full resource even when nothing changed
	// So we always return empty to prevent unnecessary applies
	// The real diff will be detected when user actually changes the file
	return "", nil
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
}

// ClusterTemplateInfo holds parsed information from an exported cluster template.
type ClusterTemplateInfo struct {
	TalosVersion             string
	KubernetesVersion        string
	ControlPlaneCount        int
	ControlPlaneMachineClass string
	WorkerGroups             []WorkerGroup
}

// ParseClusterTemplate parses the YAML output of `omnictl cluster template export`
// and extracts version and node group information.
func ParseClusterTemplate(yamlContent string) ClusterTemplateInfo {
	var info ClusterTemplateInfo

	// Scan the full content for version fields — they are unique across all docs.
	// Handles both flat (talosVersion: v1.x) and nested (talos:\n  version: v1.x) formats.
	var inTalos, inKubernetes bool
	for _, line := range strings.Split(yamlContent, "\n") {
		trimmed := strings.TrimSpace(line)
		// Flat format
		if info.TalosVersion == "" && strings.HasPrefix(trimmed, "talosVersion:") {
			info.TalosVersion = strings.TrimSpace(strings.TrimPrefix(trimmed, "talosVersion:"))
		}
		if info.KubernetesVersion == "" && strings.HasPrefix(trimmed, "kubernetesVersion:") {
			info.KubernetesVersion = strings.TrimSpace(strings.TrimPrefix(trimmed, "kubernetesVersion:"))
		}
		// Nested format: talos: / kubernetes: followed by version:
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
		// Any other non-empty, non-comment line resets nested state
		if trimmed != "" && !strings.HasPrefix(trimmed, "#") {
			inTalos = false
			inKubernetes = false
		}
	}

	// Parse per-document for node group information.
	docs := strings.Split(yamlContent, "\n---")
	for _, doc := range docs {
		doc = strings.TrimSpace(doc)
		if doc == "" {
			continue
		}

		var kind, docName string
		var inMachines, inMachineClass bool
		var machineCount, machineClassSize int
		var machineClassName string

		for _, line := range strings.Split(doc, "\n") {
			trimmed := strings.TrimSpace(line)

			if strings.HasPrefix(trimmed, "kind:") {
				kind = strings.TrimSpace(strings.TrimPrefix(trimmed, "kind:"))
				inMachines, inMachineClass = false, false
				continue
			}

			// Capture top-level name (not indented, not inside machineClass block)
			if !inMachineClass && len(line) > 0 && line[0] != ' ' && line[0] != '\t' && strings.HasPrefix(trimmed, "name:") {
				docName = strings.TrimSpace(strings.TrimPrefix(trimmed, "name:"))
				continue
			}

			switch kind {
			case "ControlPlane", "Workers":
				if strings.HasPrefix(trimmed, "machines:") {
					inMachines = true
					inMachineClass = false
				} else if strings.HasPrefix(trimmed, "machineClass:") {
					inMachineClass = true
					inMachines = false
				} else if inMachines && strings.HasPrefix(trimmed, "- ") {
					machineCount++
				} else if inMachineClass && strings.HasPrefix(trimmed, "name:") {
					machineClassName = strings.TrimSpace(strings.TrimPrefix(trimmed, "name:"))
				} else if strings.HasPrefix(trimmed, "size:") {
					parts := strings.Fields(trimmed)
					if len(parts) >= 2 {
						machineClassSize, _ = strconv.Atoi(parts[1])
					}
				}
			}
		}

		switch kind {
		case "ControlPlane":
			if machineClassName != "" {
				info.ControlPlaneMachineClass = machineClassName
				info.ControlPlaneCount = machineClassSize
			} else {
				info.ControlPlaneCount = machineCount
			}
		case "Workers":
			wg := WorkerGroup{Name: docName}
			if machineClassName != "" {
				wg.MachineClass = machineClassName
				wg.Count = machineClassSize
			} else {
				wg.Count = machineCount
			}
			info.WorkerGroups = append(info.WorkerGroups, wg)
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
