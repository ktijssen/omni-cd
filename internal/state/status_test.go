package state_test

import (
	"testing"

	"omni-cd/internal/state"
)

func TestIsPendingSync(t *testing.T) {
	pending := []string{"outofsync", "missing"}
	for _, st := range pending {
		if !state.IsPendingSync(st) {
			t.Errorf("IsPendingSync(%q) = false, want true", st)
		}
	}

	notPending := []string{"success", "applied", "failed", "syncing", "unmanaged", "orphaned", "deleting", ""}
	for _, st := range notPending {
		if state.IsPendingSync(st) {
			t.Errorf("IsPendingSync(%q) = true, want false", st)
		}
	}
}
