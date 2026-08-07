package state

import "time"

// SetMachineClasses replaces the machine class list.
func (s *AppState) SetMachineClasses(resources []ResourceInfo) {
	s.mu.Lock()
	s.MachineClasses = resources
	s.mu.Unlock()
	s.notifyChange()
}

// GetMachineClasses returns a copy of the current machine class list.
func (s *AppState) GetMachineClasses() []ResourceInfo {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]ResourceInfo, len(s.MachineClasses))
	copy(out, s.MachineClasses)
	return out
}

// MergeMachineClasses updates or inserts resources by ID without removing
// entries not present in the slice. Preserves AutoSync, LastSync*, SyncStatusSince,
// and CreatedAt from existing state when the incoming resource does not set them.
func (s *AppState) MergeMachineClasses(resources []ResourceInfo) {
	s.mu.Lock()
	existing := make(map[string]int, len(s.MachineClasses))
	for i, mc := range s.MachineClasses {
		existing[mc.ID] = i
	}
	for _, res := range resources {
		if idx, ok := existing[res.ID]; ok {
			prev := s.MachineClasses[idx]
			// Preserve AutoSync preference.
			res.AutoSync = prev.AutoSync
			// Preserve last-sync fields when this pass didn't produce a sync result.
			if res.LastSyncResult == "" {
				res.LastSyncResult = prev.LastSyncResult
				res.LastSyncError = prev.LastSyncError
				res.LastSyncTime = prev.LastSyncTime
				res.LastSyncSHA = prev.LastSyncSHA
				res.LastSyncAuthor = prev.LastSyncAuthor
				res.LastSyncMessage = prev.LastSyncMessage
				// Preserve the error when no apply was attempted (diff-only or auto-sync
				// disabled). Clear it only when the resource is now fully in sync.
				if IsPendingSync(res.Status) && res.Error == "" {
					res.Error = prev.Error
				}
			}
			// Preserve CreatedAt if the incoming resource doesn't set it.
			if res.CreatedAt.IsZero() {
				res.CreatedAt = prev.CreatedAt
			}
			// Carry forward SyncStatusSince while the resource stays pending sync
			// (outofsync or missing).
			if IsPendingSync(res.Status) {
				if IsPendingSync(prev.Status) && !prev.SyncStatusSince.IsZero() {
					res.SyncStatusSince = prev.SyncStatusSince
				} else if res.SyncStatusSince.IsZero() {
					res.SyncStatusSince = time.Now().UTC()
				}
			}
			s.MachineClasses[idx] = res
		} else {
			s.MachineClasses = append(s.MachineClasses, res)
			existing[res.ID] = len(s.MachineClasses) - 1
		}
	}
	s.mu.Unlock()
	s.notifyChange()
}

// UpdateMachineClassStatus updates the status of a single machine class by ID.
func (s *AppState) UpdateMachineClassStatus(id, status string) {
	s.mu.Lock()
	for i := range s.MachineClasses {
		if s.MachineClasses[i].ID == id {
			s.MachineClasses[i].Status = status
			s.mu.Unlock()
			s.notifyChange()
			return
		}
	}
	s.mu.Unlock()
}

// RemoveMachineClass removes a machine class from state by ID. No-op if not found.
func (s *AppState) RemoveMachineClass(id string) {
	s.mu.Lock()
	for i, mc := range s.MachineClasses {
		if mc.ID == id {
			s.MachineClasses = append(s.MachineClasses[:i], s.MachineClasses[i+1:]...)
			s.mu.Unlock()
			s.notifyChange()
			return
		}
	}
	s.mu.Unlock()
}

// GetMachineClassAutoSync returns true if the machine class is automatically applied.
func (s *AppState) GetMachineClassAutoSync(id string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, mc := range s.MachineClasses {
		if mc.ID == id {
			if mc.AutoSync != nil {
				return *mc.AutoSync
			}
			return false
		}
	}
	return false
}

// IsMachineClassAutoSyncExplicitlyDisabled returns true only when AutoSync has
// been explicitly set to false by the user.
func (s *AppState) IsMachineClassAutoSyncExplicitlyDisabled(id string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, mc := range s.MachineClasses {
		if mc.ID == id {
			return mc.AutoSync != nil && !*mc.AutoSync
		}
	}
	return false
}

// SetMachineClassAutoSync sets the per-MC AutoSync preference and persists state.
func (s *AppState) SetMachineClassAutoSync(id string, enabled bool) {
	s.mu.Lock()
	for i := range s.MachineClasses {
		if s.MachineClasses[i].ID == id {
			s.MachineClasses[i].AutoSync = &enabled
			s.mu.Unlock()
			s.save()
			s.notifyChange()
			return
		}
	}
	s.mu.Unlock()
}

// AddForceMCID queues a machine class ID to be force-synced on the next reconcile.
func (s *AppState) AddForceMCID(id string) {
	s.mu.Lock()
	if s.forceMCIDs == nil {
		s.forceMCIDs = make(map[string]bool)
	}
	s.forceMCIDs[id] = true
	s.mu.Unlock()
}

// GetAndClearForceMCIDs returns all queued force-sync MC IDs and clears the set.
// The returned map is a copy — safe to iterate concurrently with future
// AddForceMCID writers.
func (s *AppState) GetAndClearForceMCIDs() map[string]bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := cloneStringBoolMap(s.forceMCIDs)
	s.forceMCIDs = nil
	return out
}
