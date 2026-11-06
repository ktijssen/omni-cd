package state_test

import (
	"encoding/json"
	"errors"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"omni-cd/internal/maintenance"
	"omni-cd/internal/omni"
	"omni-cd/internal/state"
)

// satMorning is open Saturdays 06:00–10:00. 2026-11-07 is a Saturday.
var satMorning = maintenance.TimeRange{Days: []string{"sat"}, Start: "06:00", End: "10:00"}

var (
	insideWindow  = time.Date(2026, 11, 7, 7, 0, 0, 0, time.UTC)
	outsideWindow = time.Date(2026, 11, 4, 12, 0, 0, 0, time.UTC)
)

// newMaintenanceState returns state in UTC with the given clusters, all with
// auto-sync on.
func newMaintenanceState(t *testing.T, path string, clusters ...string) *state.AppState {
	t.Helper()
	s := state.New(100, "", true, path)
	s.SetMaintenanceLocation(time.UTC)
	var res []state.ResourceInfo
	for _, id := range clusters {
		res = append(res, state.ResourceInfo{ID: id, Status: "success"})
	}
	s.SetClusters(res)
	for _, id := range clusters {
		s.SetClusterAutoSync(id, true)
	}
	return s
}

func mustCreate(t *testing.T, s *state.AppState, w maintenance.Window) maintenance.Window {
	t.Helper()
	created, err := s.CreateMaintenanceWindow(w)
	if err != nil {
		t.Fatalf("CreateMaintenanceWindow: %v", err)
	}
	return created
}

func TestMaintenanceWindow_AppliesToEveryListedCluster(t *testing.T) {
	s := newMaintenanceState(t, "", "a", "b", "free")
	mustCreate(t, s, maintenance.Window{Name: "weekend", Clusters: []string{"a", "b"}, TimeRange: satMorning})

	blocked := s.AutoApplyBlockedClusters(outsideWindow)
	for _, id := range []string{"a", "b"} {
		if r, ok := blocked[id]; !ok || !strings.HasPrefix(r, "outside maintenance window, next window Sat 06:00 UTC") {
			t.Errorf("%s outside window: reason %q, blocked %v", id, r, ok)
		}
	}
	if _, ok := blocked["free"]; ok {
		t.Error("a cluster in no window should not be blocked")
	}
	if got := s.AutoApplyBlockedClusters(insideWindow); len(got) != 0 {
		t.Errorf("inside window nothing should be blocked, got %v", got)
	}
}

func TestMaintenanceWindow_ClusterInSeveralWindowsUsesUnion(t *testing.T) {
	s := newMaintenanceState(t, "", "a")
	mustCreate(t, s, maintenance.Window{Name: "sat", Clusters: []string{"a"}, TimeRange: satMorning})
	mustCreate(t, s, maintenance.Window{Name: "wed", Clusters: []string{"a"},
		TimeRange: maintenance.TimeRange{Days: []string{"wed"}, Start: "11:00", End: "13:00"}})

	// outsideWindow is Wed 12:00 — inside the second window.
	if r := s.AutoApplyBlockReason("a", outsideWindow); r != "" {
		t.Errorf("open in one of its windows should be allowed, got %q", r)
	}
	if r := s.AutoApplyBlockReason("a", time.Date(2026, 11, 5, 12, 0, 0, 0, time.UTC)); r == "" {
		t.Error("Thursday is in neither window and should be blocked")
	}
}

func TestMaintenanceWindow_AutoSyncOffWins(t *testing.T) {
	s := newMaintenanceState(t, "", "a")
	mustCreate(t, s, maintenance.Window{Name: "w", Clusters: []string{"a"}, TimeRange: satMorning})
	s.SetClusterAutoSync("a", false)
	if r := s.AutoApplyBlockReason("a", insideWindow); r != "auto-sync disabled" {
		t.Errorf("reason = %q, want auto-sync disabled", r)
	}
	if r := s.AutoApplyBlockReason("unknown", insideWindow); r == "" {
		t.Error("unknown cluster should be blocked")
	}
}

func TestMaintenanceWindow_Validation(t *testing.T) {
	s := newMaintenanceState(t, "", "a", "b")
	created := mustCreate(t, s, maintenance.Window{Name: "Weekend", Clusters: []string{"a"}, TimeRange: satMorning})
	if created.ID == "" {
		t.Fatal("created window has no ID")
	}

	if _, err := s.CreateMaintenanceWindow(maintenance.Window{Name: " weekend ", Clusters: []string{"b"}, TimeRange: satMorning}); !errors.Is(err, state.ErrMaintenanceWindowName) {
		t.Errorf("duplicate name (case/space-insensitive): err = %v", err)
	}
	if _, err := s.CreateMaintenanceWindow(maintenance.Window{Name: "x", Clusters: []string{"nope"}, TimeRange: satMorning}); err == nil || !strings.Contains(err.Error(), "unknown cluster") {
		t.Errorf("unknown cluster: err = %v", err)
	}
	if _, err := s.CreateMaintenanceWindow(maintenance.Window{Name: "x", TimeRange: satMorning}); err == nil {
		t.Error("window without clusters should be rejected")
	}
	if err := s.UpdateMaintenanceWindow(maintenance.Window{ID: "missing", Name: "x", Clusters: []string{"a"}, TimeRange: satMorning}); !errors.Is(err, state.ErrMaintenanceWindowNotFound) {
		t.Errorf("update unknown id: err = %v", err)
	}
	if _, err := s.DeleteMaintenanceWindow("missing"); !errors.Is(err, state.ErrMaintenanceWindowNotFound) {
		t.Errorf("delete unknown id: err = %v", err)
	}
}

func TestMaintenanceWindow_UpdateKeepsVanishedCluster(t *testing.T) {
	s := newMaintenanceState(t, "", "a", "b")
	w := mustCreate(t, s, maintenance.Window{Name: "w", Clusters: []string{"a", "b"}, TimeRange: satMorning})

	// Cluster b disappears from state (e.g. removed from Git).
	s.SetClusters([]state.ResourceInfo{{ID: "a"}})

	w.Name = "renamed"
	if err := s.UpdateMaintenanceWindow(w); err != nil {
		t.Fatalf("editing a window whose cluster vanished should work: %v", err)
	}
	w.Clusters = append(w.Clusters, "c")
	if err := s.UpdateMaintenanceWindow(w); err == nil {
		t.Error("adding a new unknown cluster should still be rejected")
	}
}

func TestMaintenanceWindow_PersistsAndDeletes(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.json")
	s := newMaintenanceState(t, path, "a")
	w := mustCreate(t, s, maintenance.Window{Name: "w", Clusters: []string{"a"}, TimeRange: satMorning})

	reloaded := state.New(100, "", true, path)
	got := reloaded.GetMaintenanceWindows()
	if len(got) != 1 || got[0].ID != w.ID || got[0].Name != "w" || got[0].Clusters[0] != "a" {
		t.Fatalf("windows after reload = %+v", got)
	}
	if c := reloaded.GetClusters()[0]; c.MaintenanceStatus != nil || c.MaintenanceWindows != nil {
		t.Error("computed per-cluster fields must not be persisted")
	}

	if name, err := s.DeleteMaintenanceWindow(w.ID); err != nil || name != "w" {
		t.Fatalf("DeleteMaintenanceWindow = %q, %v", name, err)
	}
	if r := s.AutoApplyBlockReason("a", outsideWindow); r != "" {
		t.Errorf("after delete the cluster should sync any time, got %q", r)
	}
}

func TestMaintenanceWindow_GetReturnsCopy(t *testing.T) {
	s := newMaintenanceState(t, "", "a")
	mustCreate(t, s, maintenance.Window{Name: "w", Clusters: []string{"a"}, TimeRange: satMorning})
	got := s.GetMaintenanceWindows()
	got[0].Clusters[0] = "hacked"
	got[0].Days[0] = "sun"
	again := s.GetMaintenanceWindows()[0]
	if again.Clusters[0] != "a" || again.Days[0] != "sat" {
		t.Error("GetMaintenanceWindows must return a deep copy")
	}
}

func TestSnapshot_MaintenanceFields(t *testing.T) {
	s := newMaintenanceState(t, "", "a", "free")
	mustCreate(t, s, maintenance.Window{Name: "weekend", Clusters: []string{"a"}, TimeRange: satMorning})

	snap := s.Snapshot()
	if snap.MaintenanceTimezone != "UTC" {
		t.Errorf("MaintenanceTimezone = %q, want UTC", snap.MaintenanceTimezone)
	}
	if len(snap.MaintenanceWindows) != 1 || snap.MaintenanceWindows[0].Name != "weekend" {
		t.Fatalf("snapshot windows = %+v", snap.MaintenanceWindows)
	}
	if st := snap.MaintenanceWindows[0].Status; !st.Open && st.NextStartText == "" {
		t.Errorf("closed window should carry NextStartText: %+v", st)
	}
	for _, c := range snap.Clusters {
		switch c.ID {
		case "a":
			if len(c.MaintenanceWindows) != 1 || c.MaintenanceWindows[0] != "weekend" || c.MaintenanceStatus == nil {
				t.Errorf("cluster a: windows=%v status=%+v", c.MaintenanceWindows, c.MaintenanceStatus)
			}
		case "free":
			if c.MaintenanceWindows != nil || c.MaintenanceStatus != nil {
				t.Errorf("cluster in no window should have no maintenance fields: %+v", c)
			}
		}
	}
	if s.GetClusters()[0].MaintenanceStatus != nil {
		t.Error("computed status leaked into state")
	}
}

func TestOpenMaintenanceWindowsAndApplyPass(t *testing.T) {
	s := newMaintenanceState(t, "", "on", "off")
	s.SetClusterAutoSync("off", false)
	mustCreate(t, s, maintenance.Window{Name: "w", Clusters: []string{"on", "off"}, TimeRange: satMorning})

	open := s.OpenMaintenanceWindows(insideWindow)
	if !open["on"] || open["off"] {
		t.Errorf("OpenMaintenanceWindows = %v; want only 'on'", open)
	}
	if len(s.OpenMaintenanceWindows(outsideWindow)) != 0 {
		t.Error("no windows should be open outside")
	}

	if s.TakeClusterApplyPass() {
		t.Error("no apply pass requested yet")
	}
	s.RequestClusterApplyPass()
	if !s.ClusterApplyPassPending() || !s.TakeClusterApplyPass() {
		t.Error("requested apply pass should be pending and taken once")
	}
	if s.ClusterApplyPassPending() || s.TakeClusterApplyPass() {
		t.Error("apply pass should be cleared after Take")
	}
}

func TestMaintenanceWindow_LabelSelector(t *testing.T) {
	s := newMaintenanceState(t, "", "prod-eu", "prod-us", "dev")
	s.UpdateClusterReadyStatuses(map[string]omni.ClusterStatus{
		"prod-eu": {Labels: map[string]string{"env": "prod"}},
		"prod-us": {Labels: map[string]string{"env": "prod"}},
		"dev":     {Labels: map[string]string{"env": "dev"}},
	})
	w := mustCreate(t, s, maintenance.Window{Name: "prod", TimeRange: satMorning, Selector: map[string]string{"env": "prod"}})

	blocked := s.AutoApplyBlockedClusters(outsideWindow)
	if _, ok := blocked["prod-eu"]; !ok {
		t.Error("prod-eu matches env=prod and should be blocked outside the window")
	}
	if _, ok := blocked["dev"]; ok {
		t.Error("dev does not match and should not be blocked")
	}

	snap := s.Snapshot()
	if got := snap.MaintenanceWindows[0].MatchedClusters; strings.Join(got, ",") != "prod-eu,prod-us" {
		t.Errorf("MatchedClusters = %v", got)
	}

	// A cluster labelled later joins the window automatically.
	s.UpdateClusterReadyStatuses(map[string]omni.ClusterStatus{
		"prod-eu": {Labels: map[string]string{"env": "prod"}},
		"prod-us": {Labels: map[string]string{"env": "prod"}},
		"dev":     {Labels: map[string]string{"env": "prod"}},
	})
	if r := s.AutoApplyBlockReason("dev", outsideWindow); r == "" {
		t.Error("dev relabelled env=prod should now be in the window")
	}

	// Removing the selector leaves only explicitly listed clusters (none).
	w.Selector = nil
	w.Clusters = []string{"prod-us"}
	if err := s.UpdateMaintenanceWindow(w); err != nil {
		t.Fatal(err)
	}
	if r := s.AutoApplyBlockReason("prod-eu", outsideWindow); r != "" {
		t.Errorf("prod-eu is no longer in the window, got %q", r)
	}
}

func TestMaintenanceWindow_SelectorMatchesGitTemplateLabels(t *testing.T) {
	s := newMaintenanceState(t, "", "existing")
	mustCreate(t, s, maintenance.Window{Name: "prod", TimeRange: satMorning, Selector: map[string]string{"env": "prod"}})

	// A new cluster that is only in Git ("missing" from Omni, so no live
	// labels) must still wait for the window.
	tmpl := "kind: Cluster\nname: new-prod\nlabels:\n  env: prod\n"
	s.UpsertClusterInfo("new-prod", state.ResourceInfo{ID: "new-prod", Status: "missing", FileContent: tmpl})
	s.SetClusterAutoSync("new-prod", true)
	if r := s.AutoApplyBlockReason("new-prod", outsideWindow); !strings.HasPrefix(r, "outside maintenance window") {
		t.Errorf("new-prod (labelled only in Git) reason = %q; want blocked", r)
	}

	// A reconcile without FileContent keeps the template labels.
	s.SetClusters([]state.ResourceInfo{{ID: "existing"}, {ID: "new-prod"}})
	if got := s.GetClusters()[1].TemplateLabels; got["env"] != "prod" {
		t.Errorf("template labels lost: %v", got)
	}
}

func TestSnapshot_SelectorOnlyWindowSerialisesClustersAsList(t *testing.T) {
	s := newMaintenanceState(t, "", "a")
	mustCreate(t, s, maintenance.Window{Name: "labels only", TimeRange: satMorning, Selector: map[string]string{"env": "prod"}})
	b, err := json.Marshal(s.Snapshot().MaintenanceWindows)
	if err != nil {
		t.Fatal(err)
	}
	js := string(b)
	if !strings.Contains(js, `"clusters":[]`) || strings.Contains(js, `"clusters":null`) {
		t.Errorf("clusters must serialise as [] — got %s", js)
	}
	if strings.Contains(js, "0001-01-01") {
		t.Errorf("zero timestamps must be omitted — got %s", js)
	}
}
