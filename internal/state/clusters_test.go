package state_test

import (
	"testing"
	"time"

	"omni-cd/internal/state"
)

func newState() *state.AppState {
	return state.New(100, "", true, "")
}

func TestSetClusters_PreservesTransientFields(t *testing.T) {
	s := newState()
	// First SetClusters establishes the cluster (AutoSync starts as false for new clusters).
	s.SetClusters([]state.ResourceInfo{
		{
			ID:                 "c1",
			Status:             "success",
			ClusterReady:       "ready",
			KubernetesAPIReady: "ready",
			ControlplaneReady:  "ready",
			EtcdStatus:         "ok",
			WireGuardStatus:    "ok",
			MachinesHealthy:    3,
			MachinesTotal:      3,
			BackupEnabled:      true,
			LastSyncResult:     "ok",
			LastSyncSHA:        "abc123",
			LastSyncAuthor:     "alice",
		},
	})
	// Explicitly enable AutoSync so the second SetClusters has something to preserve.
	s.SetClusterAutoSync("c1", true)

	// Reconciler sends a fresh struct without transient fields.
	s.SetClusters([]state.ResourceInfo{
		{ID: "c1", Status: "success"},
	})

	clusters := s.GetClusters()
	if len(clusters) != 1 {
		t.Fatalf("expected 1 cluster, got %d", len(clusters))
	}
	c := clusters[0]

	strChecks := []struct {
		name, got, want string
	}{
		{"ClusterReady", c.ClusterReady, "ready"},
		{"KubernetesAPIReady", c.KubernetesAPIReady, "ready"},
		{"ControlplaneReady", c.ControlplaneReady, "ready"},
		{"EtcdStatus", c.EtcdStatus, "ok"},
		{"WireGuardStatus", c.WireGuardStatus, "ok"},
		{"LastSyncResult", c.LastSyncResult, "ok"},
		{"LastSyncSHA", c.LastSyncSHA, "abc123"},
		{"LastSyncAuthor", c.LastSyncAuthor, "alice"},
	}
	for _, ch := range strChecks {
		if ch.got != ch.want {
			t.Errorf("%s: got %q, want %q", ch.name, ch.got, ch.want)
		}
	}
	if c.MachinesHealthy != 3 {
		t.Errorf("MachinesHealthy: got %d, want 3", c.MachinesHealthy)
	}
	if !c.BackupEnabled {
		t.Error("BackupEnabled: want true, got false")
	}
	if c.AutoSync == nil || !*c.AutoSync {
		t.Errorf("AutoSync: want true, got %v", c.AutoSync)
	}
}

func TestSetClusters_NewClusterGetsAutoSyncFalse(t *testing.T) {
	s := newState()
	s.SetClusters([]state.ResourceInfo{
		{ID: "brand-new", Status: "outofsync"},
	})
	clusters := s.GetClusters()
	if len(clusters) != 1 {
		t.Fatalf("expected 1 cluster, got %d", len(clusters))
	}
	if clusters[0].AutoSync == nil || *clusters[0].AutoSync {
		t.Errorf("new cluster should have AutoSync=false, got %v", clusters[0].AutoSync)
	}
}

func TestSetClusters_PreservesDeletingStatus(t *testing.T) {
	s := newState()
	s.SetClusters([]state.ResourceInfo{
		{ID: "c1", Status: "deleting"},
	})
	// Reconciler sends "outofsync" — status must not change.
	s.SetClusters([]state.ResourceInfo{
		{ID: "c1", Status: "outofsync"},
	})
	clusters := s.GetClusters()
	if len(clusters) == 0 {
		t.Fatal("expected cluster to still be in state")
	}
	if clusters[0].Status != "deleting" {
		t.Errorf("status should remain 'deleting', got %q", clusters[0].Status)
	}
}

func TestSetClusters_ClearsLastSyncWhenResultPresent(t *testing.T) {
	s := newState()
	s.SetClusters([]state.ResourceInfo{
		{ID: "c1", Status: "success", LastSyncResult: "ok", LastSyncSHA: "old-sha"},
	})
	// Incoming has a new sync result — old values must not be preserved.
	s.SetClusters([]state.ResourceInfo{
		{ID: "c1", Status: "failed", LastSyncResult: "failed", LastSyncSHA: "new-sha", LastSyncError: "boom"},
	})
	c := s.GetClusters()[0]
	if c.LastSyncSHA != "new-sha" {
		t.Errorf("LastSyncSHA: got %q, want %q", c.LastSyncSHA, "new-sha")
	}
	if c.LastSyncError != "boom" {
		t.Errorf("LastSyncError: got %q, want %q", c.LastSyncError, "boom")
	}
}

func TestSetClusters_SyncStatusSince_Preserved(t *testing.T) {
	s := newState()
	since := time.Now().UTC().Add(-time.Hour)
	s.SetClusters([]state.ResourceInfo{
		{ID: "c1", Status: "outofsync", SyncStatusSince: since},
	})
	// Fresh struct with zero SyncStatusSince — should inherit the existing value.
	s.SetClusters([]state.ResourceInfo{
		{ID: "c1", Status: "outofsync"},
	})
	c := s.GetClusters()[0]
	if !c.SyncStatusSince.Equal(since) {
		t.Errorf("SyncStatusSince should be preserved: got %v, want %v", c.SyncStatusSince, since)
	}
}

func TestUpdateTearingDownStatuses_RemovesGone(t *testing.T) {
	s := newState()
	s.SetClusters([]state.ResourceInfo{
		{ID: "gone-orphaned", Status: "orphaned"},
		{ID: "gone-deleting", Status: "deleting"},
		{ID: "gone-unmanaged", Status: "unmanaged"},
		{ID: "still-here", Status: "success"},
	})
	// Only "still-here" remains in Omni; the others have disappeared.
	s.UpdateTearingDownStatuses([]string{"still-here"}, nil)
	clusters := s.GetClusters()
	if len(clusters) != 1 {
		t.Errorf("expected 1 cluster remaining, got %d", len(clusters))
	}
	if len(clusters) > 0 && clusters[0].ID != "still-here" {
		t.Errorf("remaining cluster should be 'still-here', got %q", clusters[0].ID)
	}
}

func TestUpdateTearingDownStatuses_KeepsManaged(t *testing.T) {
	s := newState()
	s.SetClusters([]state.ResourceInfo{
		{ID: "managed", Status: "success"},
	})
	// Managed cluster disappears from allIDs — must NOT be removed.
	s.UpdateTearingDownStatuses([]string{}, nil)
	clusters := s.GetClusters()
	if len(clusters) != 1 {
		t.Errorf("managed cluster must not be removed; got %d clusters", len(clusters))
	}
}

func TestUpsertClusterInfo_PreservesHealthFields(t *testing.T) {
	s := newState()
	s.SetClusters([]state.ResourceInfo{
		{
			ID:                 "c1",
			Status:             "success",
			ClusterReady:       "ready",
			KubernetesAPIReady: "ready",
			MachinesHealthy:    2,
			MachinesTotal:      2,
		},
	})
	s.UpsertClusterInfo("c1", state.ResourceInfo{ID: "c1", Status: "outofsync"})
	c := s.GetClusters()[0]
	if c.ClusterReady != "ready" {
		t.Errorf("ClusterReady should be preserved, got %q", c.ClusterReady)
	}
	if c.MachinesHealthy != 2 {
		t.Errorf("MachinesHealthy should be preserved, got %d", c.MachinesHealthy)
	}
}

func TestUpsertClusterInfo_SkipsDeletingCluster(t *testing.T) {
	s := newState()
	s.SetClusters([]state.ResourceInfo{
		{ID: "c1", Status: "deleting"},
	})
	s.UpsertClusterInfo("c1", state.ResourceInfo{ID: "c1", Status: "success"})
	c := s.GetClusters()[0]
	if c.Status != "deleting" {
		t.Errorf("deleting cluster should not be updated; got status %q", c.Status)
	}
}

func TestUpsertClusterInfo_AppendsNew(t *testing.T) {
	s := newState()
	s.UpsertClusterInfo("new-cluster", state.ResourceInfo{ID: "new-cluster", Status: "outofsync"})
	clusters := s.GetClusters()
	if len(clusters) != 1 {
		t.Fatalf("expected 1 cluster, got %d", len(clusters))
	}
	if clusters[0].AutoSync == nil || *clusters[0].AutoSync {
		t.Errorf("new cluster should have AutoSync=false, got %v", clusters[0].AutoSync)
	}
}

func TestAddForceClusterID_GetAndClear(t *testing.T) {
	s := newState()
	s.AddForceClusterID("c1")
	s.AddForceClusterID("c2")

	ids := s.GetAndClearForceClusterIDs()
	if !ids["c1"] || !ids["c2"] {
		t.Errorf("expected both c1 and c2 in force queue, got %v", ids)
	}

	ids2 := s.GetAndClearForceClusterIDs()
	if len(ids2) != 0 {
		t.Errorf("force queue should be empty after clear, got %v", ids2)
	}
}

func TestHasForceClusterIDs(t *testing.T) {
	s := newState()
	if s.HasForceClusterIDs() {
		t.Error("should be false when queue is empty")
	}
	s.AddForceClusterID("c1")
	if !s.HasForceClusterIDs() {
		t.Error("should be true after adding an ID")
	}
	s.GetAndClearForceClusterIDs()
	if s.HasForceClusterIDs() {
		t.Error("should be false after clearing")
	}
}

func TestMarkClusterOrphaned_ClearsRepoFieldsPreservesIntrinsic(t *testing.T) {
	s := newState()
	now := time.Now().UTC()
	enabled := true
	s.SetClusters([]state.ResourceInfo{
		{
			ID:   "c1",
			Type: "Cluster",
			// repo-derived metadata that must be cleared
			RepoName:        "infra-prod",
			Status:          "success",
			LastSyncResult:  "ok",
			LastSyncError:   "previous error",
			LastSyncTime:    now,
			LastSyncSHA:     "abc12345",
			LastSyncAuthor:  "alice",
			LastSyncMessage: "deploy cluster",
			SyncStatusSince: now,
			Diff:            "+ something",
			FileContent:     "kind: Cluster\n",
			Error:           "stale error",
			// intrinsic fields that must survive
			TalosVersion:       "v1.7.0",
			KubernetesVersion:  "v1.30.0",
			MachinesHealthy:    3,
			MachinesTotal:      3,
			ClusterReady:       "ready",
			KubernetesAPIReady: "ready",
			ControlplaneReady:  "ready",
			ClusterPhase:       "running",
			EtcdStatus:         "ok",
			WireGuardStatus:    "ok",
			LiveContent:        "kind: Cluster\nname: c1\n",
			CreatedAt:          now,
			BackupEnabled:      true,
		},
	})
	// AutoSync is set after SetClusters because SetClusters resets new clusters to false.
	s.SetClusterAutoSync("c1", true)
	_ = enabled

	s.MarkClusterOrphaned("c1")

	cs := s.GetClusters()
	if len(cs) != 1 {
		t.Fatalf("expected 1 cluster, got %d", len(cs))
	}
	c := cs[0]

	if c.Status != "orphaned" {
		t.Errorf("Status: got %q, want %q", c.Status, "orphaned")
	}

	// Repo-derived fields must be zero.
	zeros := []struct {
		name, got string
	}{
		{"RepoName", c.RepoName},
		{"LastSyncResult", c.LastSyncResult},
		{"LastSyncError", c.LastSyncError},
		{"LastSyncSHA", c.LastSyncSHA},
		{"LastSyncAuthor", c.LastSyncAuthor},
		{"LastSyncMessage", c.LastSyncMessage},
		{"Diff", c.Diff},
		{"FileContent", c.FileContent},
		{"Error", c.Error},
	}
	for _, z := range zeros {
		if z.got != "" {
			t.Errorf("%s: got %q, want empty", z.name, z.got)
		}
	}
	if !c.LastSyncTime.IsZero() {
		t.Errorf("LastSyncTime: got %v, want zero", c.LastSyncTime)
	}
	if !c.SyncStatusSince.IsZero() {
		t.Errorf("SyncStatusSince: got %v, want zero", c.SyncStatusSince)
	}

	// Intrinsic fields must survive.
	if c.TalosVersion != "v1.7.0" {
		t.Errorf("TalosVersion clobbered: got %q", c.TalosVersion)
	}
	if c.KubernetesVersion != "v1.30.0" {
		t.Errorf("KubernetesVersion clobbered: got %q", c.KubernetesVersion)
	}
	if c.MachinesHealthy != 3 {
		t.Errorf("MachinesHealthy clobbered: got %d", c.MachinesHealthy)
	}
	if c.ClusterReady != "ready" {
		t.Errorf("ClusterReady clobbered: got %q", c.ClusterReady)
	}
	if c.LiveContent == "" {
		t.Error("LiveContent was unexpectedly cleared")
	}
	if c.CreatedAt.IsZero() {
		t.Error("CreatedAt was unexpectedly cleared")
	}
	if c.AutoSync == nil || !*c.AutoSync {
		t.Errorf("AutoSync preference lost: %v", c.AutoSync)
	}
}

func TestMarkClusterOrphaned_UnknownIDIsNoOp(t *testing.T) {
	s := newState()
	s.SetClusters([]state.ResourceInfo{
		{ID: "c1", Type: "Cluster", Status: "success", RepoName: "infra"},
	})

	s.MarkClusterOrphaned("does-not-exist")

	cs := s.GetClusters()
	if len(cs) != 1 {
		t.Fatalf("expected 1 cluster, got %d", len(cs))
	}
	if cs[0].Status != "success" || cs[0].RepoName != "infra" {
		t.Errorf("unrelated cluster mutated: %+v", cs[0])
	}
}

func TestUpdateTearingDownStatuses_KeepsMissing(t *testing.T) {
	s := newState()
	s.SetClusters([]state.ResourceInfo{
		{ID: "gone-missing", Status: "missing"},
		{ID: "still-here", Status: "success"},
	})
	// A "missing" cluster is absent from Omni by definition — it is defined in git
	// but never created, so it must survive the prune that removes clusters which
	// have disappeared from Omni.
	s.UpdateTearingDownStatuses([]string{"still-here"}, nil)
	clusters := s.GetClusters()
	if len(clusters) != 2 {
		t.Fatalf("expected 2 clusters remaining, got %d", len(clusters))
	}
	found := map[string]string{}
	for _, c := range clusters {
		found[c.ID] = c.Status
	}
	if found["gone-missing"] != "missing" {
		t.Errorf("missing cluster should be kept with status 'missing', got %q", found["gone-missing"])
	}
}

func TestSetClusters_PreservesErrorWhenMissing(t *testing.T) {
	s := newState()
	s.SetClusters([]state.ResourceInfo{
		{ID: "c1", Status: "failed", Error: "boom", LastSyncResult: "failed"},
	})
	// Diff-only pass reports "missing" and attempted no apply — the error explaining
	// the previous failed sync must survive so the UI keeps its Sync Failed badge.
	s.SetClusters([]state.ResourceInfo{
		{ID: "c1", Status: "missing"},
	})
	c := s.GetClusters()[0]
	if c.Error != "boom" {
		t.Errorf("Error should be preserved: got %q, want %q", c.Error, "boom")
	}
}

func TestSetClusters_PreservesDeletingOverMissing(t *testing.T) {
	s := newState()
	s.SetClusters([]state.ResourceInfo{
		{ID: "c1", Status: "deleting"},
	})
	s.SetClusters([]state.ResourceInfo{
		{ID: "c1", Status: "missing"},
	})
	c := s.GetClusters()[0]
	if c.Status != "deleting" {
		t.Errorf("status should remain 'deleting', got %q", c.Status)
	}
}

func TestUpsertClusterInfo_PreservesErrorWhenMissing(t *testing.T) {
	s := newState()
	s.SetClusters([]state.ResourceInfo{
		{ID: "c1", Status: "failed", Error: "boom", LastSyncResult: "failed"},
	})
	s.UpsertClusterInfo("c1", state.ResourceInfo{ID: "c1", Status: "missing"})
	c := s.GetClusters()[0]
	if c.Error != "boom" {
		t.Errorf("Error should be preserved: got %q, want %q", c.Error, "boom")
	}
}

func TestSetClusters_SyncStatusSince_PreservedAcrossOutofsyncToMissing(t *testing.T) {
	s := newState()
	since := time.Now().UTC().Add(-time.Hour)
	s.SetClusters([]state.ResourceInfo{
		{ID: "c1", Status: "outofsync", SyncStatusSince: since},
	})
	// The cluster was drifted, then deleted from Omni. It has still been pending
	// since the original timestamp — the clock must not reset.
	s.SetClusters([]state.ResourceInfo{
		{ID: "c1", Status: "missing"},
	})
	c := s.GetClusters()[0]
	if !c.SyncStatusSince.Equal(since) {
		t.Errorf("SyncStatusSince should be preserved: got %v, want %v", c.SyncStatusSince, since)
	}
}
