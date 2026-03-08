package reconciler

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"omni-cd/internal/state"
)

// Reconciler handles the apply and delete phases for machine classes
// and cluster templates.
type Reconciler struct {
	state             *state.AppState
	protectedClusters map[string]bool // clusters from failing repos — skip delete/outofsync
	protectedMCs      map[string]bool // machine classes from failing repos — skip delete
	pendingDeletes    sync.Map        // cluster IDs currently being deleted by DeleteSingleCluster
}

// New creates a new Reconciler with shared state.
func New(appState *state.AppState) *Reconciler {
	return &Reconciler{state: appState}
}

// SetProtectedResources records cluster and machine class IDs that belong to
// repos which are currently failing to sync. These resources must not be
// deleted or marked out-of-sync during this reconcile cycle.
func (r *Reconciler) SetProtectedResources(clusterIDs, mcIDs []string) {
	r.protectedClusters = make(map[string]bool, len(clusterIDs))
	for _, id := range clusterIDs {
		r.protectedClusters[id] = true
	}
	r.protectedMCs = make(map[string]bool, len(mcIDs))
	for _, id := range mcIDs {
		r.protectedMCs[id] = true
	}
}

// ForceDeleteFromDirs force-deletes all template-managed clusters and machine
// classes whose IDs are declared in the given directories, bypassing the
// auto-sync check. This is used when a repo is deleted via the UI so its
// previously-managed resources are cleaned up unconditionally.
func (r *Reconciler) ForceDeleteFromDirs(clustersDirs, mcDirs []string) {
	r.forceDeleteClusters(clustersDirs)
	r.forceDeleteMachineClasses(mcDirs)
}

// ============================================================
// Logging — writes to both stdout and shared state
// ============================================================

func (r *Reconciler) logDebug(msg string, attrs ...any) {
	component := extractComponent(attrs...)
	allAttrs := append([]any{"component", component}, attrs...)
	slog.Debug(msg, allAttrs...)

	if slog.Default().Enabled(nil, slog.LevelDebug) {
		displayMsg := formatLogMessage("DEBUG", msg, attrs...)
		r.state.AddLog("DEBUG", component, displayMsg)
	}
}

func (r *Reconciler) logInfo(msg string, attrs ...any) {
	component := extractComponent(attrs...)
	allAttrs := append([]any{"component", component}, attrs...)
	slog.Info(msg, allAttrs...)

	if slog.Default().Enabled(nil, slog.LevelInfo) {
		displayMsg := formatLogMessage("INFO", msg, attrs...)
		r.state.AddLog("INFO", component, displayMsg)
	}
}

func (r *Reconciler) logWarn(msg string, attrs ...any) {
	component := extractComponent(attrs...)
	allAttrs := append([]any{"component", component}, attrs...)
	slog.Warn(msg, allAttrs...)

	if slog.Default().Enabled(nil, slog.LevelWarn) {
		displayMsg := formatLogMessage("WARN", msg, attrs...)
		r.state.AddLog("WARN", component, displayMsg)
	}
}

func (r *Reconciler) logError(msg string, attrs ...any) {
	component := extractComponent(attrs...)
	allAttrs := append([]any{"component", component}, attrs...)
	slog.Error(msg, allAttrs...)

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

	var jsonParts []string
	baseJSON, _ := json.Marshal(entry)
	baseStr := string(baseJSON)
	// Remove closing brace
	baseStr = baseStr[:len(baseStr)-1]
	jsonParts = append(jsonParts, baseStr)

	for i := 0; i < len(attrs); i += 2 {
		if i+1 < len(attrs) {
			key := fmt.Sprint(attrs[i])
			valJSON, _ := json.Marshal(attrs[i+1])
			jsonParts = append(jsonParts, fmt.Sprintf(`"%s":%s`, key, string(valJSON)))
		}
	}

	return strings.Join(jsonParts, ",") + "}"
}
