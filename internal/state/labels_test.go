package state_test

import (
	"maps"
	"testing"

	"omni-cd/internal/omni"
	"omni-cd/internal/state"
)

func TestClusterLabels_FromStatusAndPreserved(t *testing.T) {
	s := newState()
	s.SetClusters([]state.ResourceInfo{{ID: "c1", Status: "success"}})

	if !s.UpdateClusterReadyStatuses(map[string]omni.ClusterStatus{"c1": {Ready: true, Labels: map[string]string{"env": "prod"}}}) {
		t.Fatal("first status update should report a change")
	}
	if got := s.GetClusters()[0].Labels; !maps.Equal(got, map[string]string{"env": "prod"}) {
		t.Fatalf("labels = %v", got)
	}

	// Same status, only a label value changed: must still count as a change.
	if !s.UpdateClusterReadyStatuses(map[string]omni.ClusterStatus{"c1": {Ready: true, Labels: map[string]string{"env": "staging"}}}) {
		t.Error("a label change should report a change")
	}
	if s.UpdateClusterReadyStatuses(map[string]omni.ClusterStatus{"c1": {Ready: true, Labels: map[string]string{"env": "staging"}}}) {
		t.Error("identical update should not report a change")
	}

	// A reconcile rebuilds the cluster list without live fields.
	s.SetClusters([]state.ResourceInfo{{ID: "c1", Status: "outofsync"}})
	if got := s.GetClusters()[0].Labels; got["env"] != "staging" {
		t.Errorf("SetClusters dropped labels: %v", got)
	}
	s.UpsertClusterInfo("c1", state.ResourceInfo{ID: "c1", Status: "success"})
	if got := s.GetClusters()[0].Labels; got["env"] != "staging" {
		t.Errorf("UpsertClusterInfo dropped labels: %v", got)
	}

	// Removing all labels in Omni clears them.
	if !s.UpdateClusterReadyStatuses(map[string]omni.ClusterStatus{"c1": {Ready: true}}) {
		t.Error("removing labels should report a change")
	}
	if got := s.GetClusters()[0].Labels; len(got) != 0 {
		t.Errorf("labels should be cleared, got %v", got)
	}
}
