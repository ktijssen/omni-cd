package state

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"omni-cd/internal/maintenance"
)

// Errors returned by the maintenance window mutators. Handlers map
// ErrMaintenanceWindowNotFound to 404 and the others to 400.
var (
	ErrMaintenanceWindowNotFound = errors.New("maintenance window not found")
	ErrMaintenanceWindowName     = errors.New("a maintenance window with this name already exists")
)

// MaintenanceStatusView is a maintenance.Status plus the times pre-formatted
// in the maintenance timezone, so the UI never has to do timezone math.
type MaintenanceStatusView struct {
	Open           bool      `json:"open"`
	CurrentEnd     time.Time `json:"currentEnd,omitzero"`
	NextStart      time.Time `json:"nextStart,omitzero"`
	CurrentEndText string    `json:"currentEndText,omitempty"` // e.g. "Wed 02:00"
	NextStartText  string    `json:"nextStartText,omitempty"`  // e.g. "Sat 06:00"
}

// MaintenanceWindowView is a window with its current status, for the UI.
type MaintenanceWindowView struct {
	maintenance.Window
	Status MaintenanceStatusView `json:"status"`
	// MatchedClusters are the clusters in state the window applies to right
	// now: listed by name or matching the selector. Sorted.
	MatchedClusters []string `json:"matchedClusters"`
}

// SetMaintenanceLocation sets the location maintenance window times are read
// in. Defaults to time.Local (the TZ environment variable, UTC when unset).
func (s *AppState) SetMaintenanceLocation(loc *time.Location) {
	s.mu.Lock()
	s.maintenanceLoc = loc
	s.mu.Unlock()
}

// loc returns the maintenance location. Caller holds s.mu.
func (s *AppState) loc() *time.Location {
	if s.maintenanceLoc != nil {
		return s.maintenanceLoc
	}
	return time.Local
}

// maintenanceTimezone names the maintenance location for display: the IANA
// name when known (TZ=Europe/Amsterdam), otherwise the zone abbreviation.
// Caller holds s.mu.
func (s *AppState) maintenanceTimezone() string {
	if s.maintenanceLoc != nil {
		return s.maintenanceLoc.String()
	}
	if tz := strings.TrimPrefix(os.Getenv("TZ"), ":"); tz != "" {
		return tz
	}
	name, _ := time.Now().In(time.Local).Zone()
	return name
}

func (s *AppState) statusView(ranges []maintenance.TimeRange, now time.Time) MaintenanceStatusView {
	loc := s.loc()
	st := maintenance.Evaluate(ranges, now, loc)
	v := MaintenanceStatusView{Open: st.Open, CurrentEnd: st.CurrentEnd, NextStart: st.NextStart}
	if !st.CurrentEnd.IsZero() {
		v.CurrentEndText = st.CurrentEnd.In(loc).Format("Mon 15:04")
	}
	if !st.NextStart.IsZero() {
		v.NextStartText = st.NextStart.In(loc).Format("Mon 15:04")
	}
	return v
}

// rangesForCluster returns the time range of every window that includes c
// (by name or label selector);
// a cluster in several windows may sync when any of them is open. Returns nil
// when no window includes the cluster. Caller holds s.mu.
func (s *AppState) rangesForCluster(c ResourceInfo) []maintenance.TimeRange {
	var out []maintenance.TimeRange
	for _, w := range s.maintenanceWindows {
		if w.Matches(c.ID, c.Labels, c.TemplateLabels) {
			out = append(out, w.TimeRange)
		}
	}
	return out
}

// GetMaintenanceWindows returns a copy of all maintenance windows.
func (s *AppState) GetMaintenanceWindows() []maintenance.Window {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]maintenance.Window, len(s.maintenanceWindows))
	for i, w := range s.maintenanceWindows {
		out[i] = cloneWindow(w)
	}
	return out
}

// CreateMaintenanceWindow validates w, assigns it an ID, stores it and
// persists state. Every cluster must currently be in state.
func (s *AppState) CreateMaintenanceWindow(w maintenance.Window) (maintenance.Window, error) {
	w = normalizeWindow(w)
	if err := w.Validate(); err != nil {
		return maintenance.Window{}, err
	}
	id, err := newWindowID()
	if err != nil {
		return maintenance.Window{}, err
	}
	w.ID = id

	s.mu.Lock()
	if err := s.checkWindowLocked(w, nil); err != nil {
		s.mu.Unlock()
		return maintenance.Window{}, err
	}
	s.maintenanceWindows = append(s.maintenanceWindows, cloneWindow(w))
	s.mu.Unlock()
	s.save()
	s.notifyChange()
	return w, nil
}

// UpdateMaintenanceWindow replaces the window with w.ID. Clusters must be in
// state, except ones the window already listed (so a window can still be
// edited after one of its clusters was removed).
func (s *AppState) UpdateMaintenanceWindow(w maintenance.Window) error {
	w = normalizeWindow(w)
	if err := w.Validate(); err != nil {
		return err
	}
	s.mu.Lock()
	idx := s.windowIndexLocked(w.ID)
	if idx < 0 {
		s.mu.Unlock()
		return ErrMaintenanceWindowNotFound
	}
	if err := s.checkWindowLocked(w, &s.maintenanceWindows[idx]); err != nil {
		s.mu.Unlock()
		return err
	}
	s.maintenanceWindows[idx] = cloneWindow(w)
	s.mu.Unlock()
	s.save()
	s.notifyChange()
	return nil
}

// DeleteMaintenanceWindow removes the window with id and returns its name.
func (s *AppState) DeleteMaintenanceWindow(id string) (string, error) {
	s.mu.Lock()
	idx := s.windowIndexLocked(id)
	if idx < 0 {
		s.mu.Unlock()
		return "", ErrMaintenanceWindowNotFound
	}
	name := s.maintenanceWindows[idx].Name
	s.maintenanceWindows = append(s.maintenanceWindows[:idx:idx], s.maintenanceWindows[idx+1:]...)
	s.mu.Unlock()
	s.save()
	s.notifyChange()
	return name, nil
}

func (s *AppState) windowIndexLocked(id string) int {
	for i, w := range s.maintenanceWindows {
		if w.ID == id {
			return i
		}
	}
	return -1
}

// checkWindowLocked enforces unique names and known clusters. prev is the
// stored version when updating, nil when creating. Caller holds s.mu.
func (s *AppState) checkWindowLocked(w maintenance.Window, prev *maintenance.Window) error {
	for _, other := range s.maintenanceWindows {
		if other.ID != w.ID && strings.EqualFold(other.Name, w.Name) {
			return ErrMaintenanceWindowName
		}
	}
	known := make(map[string]bool, len(s.Clusters))
	for _, c := range s.Clusters {
		known[c.ID] = true
	}
	for _, id := range w.Clusters {
		if !known[id] && (prev == nil || !prev.HasCluster(id)) {
			return fmt.Errorf("unknown cluster %q", id)
		}
	}
	return nil
}

// AutoApplyBlockReason returns why automatic changes to a cluster are not
// allowed at time now, or "" when they are. Unknown clusters are blocked,
// matching GetClusterAutoSync's default of auto-sync off.
func (s *AppState) AutoApplyBlockReason(id string, now time.Time) string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, c := range s.Clusters {
		if c.ID == id {
			return s.autoApplyBlockReasonLocked(c, now)
		}
	}
	return "auto-sync disabled"
}

// AutoApplyBlockedClusters returns the IDs of clusters in state whose
// automatic changes are not allowed at time now, mapped to the reason.
func (s *AppState) AutoApplyBlockedClusters(now time.Time) map[string]string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make(map[string]string)
	for _, c := range s.Clusters {
		if reason := s.autoApplyBlockReasonLocked(c, now); reason != "" {
			out[c.ID] = reason
		}
	}
	return out
}

func (s *AppState) autoApplyBlockReasonLocked(c ResourceInfo, now time.Time) string {
	if c.AutoSync == nil || !*c.AutoSync {
		return "auto-sync disabled"
	}
	ranges := s.rangesForCluster(c)
	if ranges == nil {
		return ""
	}
	st := s.statusView(ranges, now)
	switch {
	case st.Open:
		return ""
	case st.NextStartText != "":
		return fmt.Sprintf("outside maintenance window, next window %s %s", st.NextStartText, s.maintenanceTimezone())
	default:
		return "outside maintenance window"
	}
}

// OpenMaintenanceWindows returns the IDs of clusters with auto-sync enabled
// that are in at least one maintenance window, and whose windows are open at
// time now.
func (s *AppState) OpenMaintenanceWindows(now time.Time) map[string]bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make(map[string]bool)
	for _, c := range s.Clusters {
		if c.AutoSync == nil || !*c.AutoSync {
			continue
		}
		if ranges := s.rangesForCluster(c); ranges != nil && maintenance.Evaluate(ranges, now, s.loc()).Open {
			out[c.ID] = true
		}
	}
	return out
}

// RequestClusterApplyPass makes the next reconcile run the cluster apply path
// even when no repo changed, so pending changes are applied once a
// maintenance window opens.
func (s *AppState) RequestClusterApplyPass() {
	s.mu.Lock()
	s.clusterApplyPass = true
	s.mu.Unlock()
}

// ClusterApplyPassPending reports whether an apply pass is still waiting
// for a reconcile to pick it up.
func (s *AppState) ClusterApplyPassPending() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.clusterApplyPass
}

// TakeClusterApplyPass reports whether an apply pass was requested and clears
// the request.
func (s *AppState) TakeClusterApplyPass() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	requested := s.clusterApplyPass
	s.clusterApplyPass = false
	return requested
}

// withMaintenanceStatus fills MaintenanceWindows/MaintenanceStatus on an
// already-cloned cluster slice. Caller holds s.mu.
func (s *AppState) withMaintenanceStatus(clusters []ResourceInfo, now time.Time) []ResourceInfo {
	if len(s.maintenanceWindows) == 0 {
		return clusters
	}
	for i := range clusters {
		var names []string
		var ranges []maintenance.TimeRange
		for _, w := range s.maintenanceWindows {
			if w.Matches(clusters[i].ID, clusters[i].Labels, clusters[i].TemplateLabels) {
				names = append(names, w.Name)
				ranges = append(ranges, w.TimeRange)
			}
		}
		if names != nil {
			st := s.statusView(ranges, now)
			clusters[i].MaintenanceWindows = names
			clusters[i].MaintenanceStatus = &st
		}
	}
	return clusters
}

// maintenanceWindowViews returns every window with its status. Caller holds s.mu.
func (s *AppState) maintenanceWindowViews(now time.Time) []MaintenanceWindowView {
	out := make([]MaintenanceWindowView, len(s.maintenanceWindows))
	for i, w := range s.maintenanceWindows {
		matched := []string{}
		for _, c := range s.Clusters {
			if w.Matches(c.ID, c.Labels, c.TemplateLabels) {
				matched = append(matched, c.ID)
			}
		}
		sort.Strings(matched)
		out[i] = MaintenanceWindowView{
			Window:          cloneWindow(w),
			Status:          s.statusView([]maintenance.TimeRange{w.TimeRange}, now),
			MatchedClusters: matched,
		}
	}
	return out
}

func normalizeWindow(w maintenance.Window) maintenance.Window {
	w = cloneWindow(w)
	w.Name = strings.TrimSpace(w.Name)
	return w
}

func cloneWindow(w maintenance.Window) maintenance.Window {
	// Clusters is never nil so it serialises as [] (the UI iterates it);
	// windows saved with only a selector store it as null.
	out := maintenance.Window{ID: w.ID, Name: w.Name, Clusters: append([]string{}, w.Clusters...)}
	out.TimeRange = maintenance.TimeRange{Days: append([]string(nil), w.Days...), Start: w.Start, End: w.End}
	if len(w.Selector) > 0 {
		out.Selector = cloneStringMap(w.Selector)
	}
	return out
}

func newWindowID() (string, error) {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
