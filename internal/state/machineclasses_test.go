package state_test

import (
	"testing"
	"time"

	"omni-cd/internal/state"
)

func TestMergeMachineClasses_PreservesAutoSync(t *testing.T) {
	s := newState()
	enabled := true
	s.SetMachineClasses([]state.ResourceInfo{
		{ID: "mc1", Status: "success", AutoSync: &enabled},
	})
	s.MergeMachineClasses([]state.ResourceInfo{
		{ID: "mc1", Status: "outofsync"},
	})
	mcs := s.GetMachineClasses()
	if len(mcs) != 1 {
		t.Fatalf("expected 1 MC, got %d", len(mcs))
	}
	if mcs[0].AutoSync == nil || !*mcs[0].AutoSync {
		t.Errorf("AutoSync should be preserved as true, got %v", mcs[0].AutoSync)
	}
}

func TestMergeMachineClasses_PreservesLastSyncWhenNoResult(t *testing.T) {
	s := newState()
	s.SetMachineClasses([]state.ResourceInfo{
		{
			ID:             "mc1",
			Status:         "success",
			LastSyncResult: "ok",
			LastSyncSHA:    "sha1",
			LastSyncAuthor: "bob",
		},
	})
	// Incoming has no LastSyncResult — previous values must be preserved.
	s.MergeMachineClasses([]state.ResourceInfo{
		{ID: "mc1", Status: "outofsync"},
	})
	mc := s.GetMachineClasses()[0]
	if mc.LastSyncResult != "ok" {
		t.Errorf("LastSyncResult: got %q, want %q", mc.LastSyncResult, "ok")
	}
	if mc.LastSyncSHA != "sha1" {
		t.Errorf("LastSyncSHA: got %q, want %q", mc.LastSyncSHA, "sha1")
	}
	if mc.LastSyncAuthor != "bob" {
		t.Errorf("LastSyncAuthor: got %q, want %q", mc.LastSyncAuthor, "bob")
	}
}

func TestMergeMachineClasses_ClearsLastSyncWhenResultPresent(t *testing.T) {
	s := newState()
	s.SetMachineClasses([]state.ResourceInfo{
		{ID: "mc1", Status: "success", LastSyncResult: "ok", LastSyncSHA: "old-sha"},
	})
	// Incoming has a new sync result — old values must not be preserved.
	s.MergeMachineClasses([]state.ResourceInfo{
		{ID: "mc1", Status: "failed", LastSyncResult: "failed", LastSyncSHA: "new-sha", LastSyncError: "oops"},
	})
	mc := s.GetMachineClasses()[0]
	if mc.LastSyncSHA != "new-sha" {
		t.Errorf("LastSyncSHA: got %q, want %q", mc.LastSyncSHA, "new-sha")
	}
	if mc.LastSyncError != "oops" {
		t.Errorf("LastSyncError: got %q, want %q", mc.LastSyncError, "oops")
	}
}

func TestMergeMachineClasses_AddsNew(t *testing.T) {
	s := newState()
	s.MergeMachineClasses([]state.ResourceInfo{
		{ID: "mc-new", Status: "outofsync"},
	})
	mcs := s.GetMachineClasses()
	if len(mcs) != 1 {
		t.Fatalf("expected 1 MC after adding new, got %d", len(mcs))
	}
	if mcs[0].ID != "mc-new" {
		t.Errorf("ID: got %q, want %q", mcs[0].ID, "mc-new")
	}
}

func TestRemoveMachineClass(t *testing.T) {
	s := newState()
	s.SetMachineClasses([]state.ResourceInfo{
		{ID: "mc1", Status: "success"},
		{ID: "mc2", Status: "success"},
	})
	s.RemoveMachineClass("mc1")
	mcs := s.GetMachineClasses()
	if len(mcs) != 1 {
		t.Fatalf("expected 1 MC after removal, got %d", len(mcs))
	}
	if mcs[0].ID != "mc2" {
		t.Errorf("remaining MC should be mc2, got %q", mcs[0].ID)
	}
	// No-op on unknown ID.
	s.RemoveMachineClass("unknown")
	if len(s.GetMachineClasses()) != 1 {
		t.Error("removing unknown ID should be a no-op")
	}
}

func TestAddForceMCID_GetAndClear(t *testing.T) {
	s := newState()
	s.AddForceMCID("mc1")
	s.AddForceMCID("mc2")

	ids := s.GetAndClearForceMCIDs()
	if !ids["mc1"] || !ids["mc2"] {
		t.Errorf("expected both mc1 and mc2 in force queue, got %v", ids)
	}
	ids2 := s.GetAndClearForceMCIDs()
	if len(ids2) != 0 {
		t.Errorf("force queue should be empty after clear, got %v", ids2)
	}
}

func TestMergeMachineClasses_SyncStatusSince_SetOnFirstOutofsync(t *testing.T) {
	s := newState()
	// Establish the MC in a non-outofsync state first.
	s.SetMachineClasses([]state.ResourceInfo{
		{ID: "mc1", Status: "success"},
	})
	before := time.Now()
	// Transition to outofsync — SyncStatusSince must be set.
	s.MergeMachineClasses([]state.ResourceInfo{
		{ID: "mc1", Status: "outofsync"},
	})
	mc := s.GetMachineClasses()[0]
	if mc.SyncStatusSince.IsZero() {
		t.Error("SyncStatusSince should be set when MC transitions to outofsync")
	}
	if mc.SyncStatusSince.Before(before) {
		t.Error("SyncStatusSince should be >= time before the call")
	}
}

func TestMergeMachineClasses_SyncStatusSince_PreservedOnSubsequentOutofsync(t *testing.T) {
	s := newState()
	since := time.Now().UTC().Add(-2 * time.Hour)
	s.SetMachineClasses([]state.ResourceInfo{
		{ID: "mc1", Status: "outofsync", SyncStatusSince: since},
	})
	// Merge again with outofsync — SyncStatusSince must not change.
	s.MergeMachineClasses([]state.ResourceInfo{
		{ID: "mc1", Status: "outofsync"},
	})
	mc := s.GetMachineClasses()[0]
	if !mc.SyncStatusSince.Equal(since) {
		t.Errorf("SyncStatusSince should be preserved: got %v, want %v", mc.SyncStatusSince, since)
	}
}
