package state_test

import (
	"encoding/json"
	"sync"
	"testing"
	"time"

	"omni-cd/internal/state"
)

// TestSnapshot_ConcurrentMutation proves Snapshot returns deep copies that
// are safe to JSON-marshal while another goroutine mutates the source state.
// Pre-fix this would race under -race and panic with concurrent map iteration.
func TestSnapshot_ConcurrentMutation(t *testing.T) {
	s := newState()
	s.SetClusters([]state.ResourceInfo{
		{
			ID:                "c1",
			Status:            "success",
			ClusterExtensions: []string{"ext1", "ext2"},
			MachineExtensions: map[string][]string{"node1": {"a", "b"}},
			MachineHostnames:  map[string]string{"uuid1": "host1"},
			ControlPlane:      state.NodeGroup{Machines: []string{"m1", "m2"}},
			Workers: []state.NodeGroup{
				{Name: "w1", Machines: []string{"m3", "m4"}, Extensions: []string{"e1"}},
			},
		},
	})
	s.SetRepoClusters("repo1", []string{"c1"})

	deadline := time.Now().Add(200 * time.Millisecond)
	var wg sync.WaitGroup

	// Reader: snapshot + marshal in a tight loop.
	wg.Add(1)
	go func() {
		defer wg.Done()
		for time.Now().Before(deadline) {
			snap := s.Snapshot()
			if _, err := json.Marshal(snap); err != nil {
				t.Errorf("marshal: %v", err)
				return
			}
		}
	}()

	// Writer: mutate the same state in a tight loop.
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; time.Now().Before(deadline); i++ {
			s.UpdateClusterStatus("c1", "syncing")
			s.SetRepoClusters("repo1", []string{"c1", "c2"})
			s.UpdateClusterStatus("c1", "success")
			s.SetRepoClusters("repo1", []string{"c1"})
		}
	}()

	wg.Wait()
}

// TestGetRepoClusters_ReturnsCopy proves the getter returns a slice that the
// caller can safely retain across writer mutations.
func TestGetRepoClusters_ReturnsCopy(t *testing.T) {
	s := newState()
	s.SetRepoClusters("r", []string{"a", "b"})
	got := s.GetRepoClusters("r")
	if len(got) != 2 {
		t.Fatalf("expected 2, got %d", len(got))
	}
	// Mutate the underlying map; the previously returned slice must not change.
	s.SetRepoClusters("r", []string{"x", "y", "z"})
	if got[0] != "a" || got[1] != "b" {
		t.Errorf("returned slice was aliased: got %v", got)
	}
}

// TestGetAndClearForceClusterIDs_ReturnsCopy proves the take-and-clear pattern
// returns a map the caller can iterate without racing future writers.
func TestGetAndClearForceClusterIDs_ReturnsCopy(t *testing.T) {
	s := newState()
	s.AddForceClusterID("c1")
	s.AddForceClusterID("c2")

	taken := s.GetAndClearForceClusterIDs()
	if len(taken) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(taken))
	}

	// Adding new IDs must not bleed into the previously returned map.
	s.AddForceClusterID("c3")
	if _, leaked := taken["c3"]; leaked {
		t.Error("returned map was aliased: post-clear write appeared in returned map")
	}
}
