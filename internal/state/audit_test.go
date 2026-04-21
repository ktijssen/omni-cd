package state

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestAppendAudit_RingBuffer(t *testing.T) {
	s := New(100, "", true, "")
	// Add more than maxAuditEntries (500) entries.
	for i := 0; i < 600; i++ {
		s.AppendAudit(AuditEntry{Action: "sync", Kind: "cluster"})
	}
	s.mu.RLock()
	n := len(s.auditLog)
	s.mu.RUnlock()
	if n != maxAuditEntries {
		t.Errorf("ring buffer: got %d entries, want %d", n, maxAuditEntries)
	}
}

func TestGetAuditLog_NewestFirst(t *testing.T) {
	s := New(100, "", true, "")
	t1 := time.Now().UTC().Add(-2 * time.Hour)
	t2 := time.Now().UTC().Add(-1 * time.Hour)
	t3 := time.Now().UTC()
	s.AppendAudit(AuditEntry{Timestamp: t1, Action: "first", Kind: "cluster"})
	s.AppendAudit(AuditEntry{Timestamp: t2, Action: "second", Kind: "cluster"})
	s.AppendAudit(AuditEntry{Timestamp: t3, Action: "third", Kind: "cluster"})

	entries := s.GetAuditLog()
	if len(entries) != 3 {
		t.Fatalf("expected 3 entries, got %d", len(entries))
	}
	if entries[0].Action != "third" {
		t.Errorf("first result entry should be 'third' (newest), got %q", entries[0].Action)
	}
	if entries[2].Action != "first" {
		t.Errorf("last result entry should be 'first' (oldest), got %q", entries[2].Action)
	}
}

func TestSetAuditDir_WritesAndReadsFile(t *testing.T) {
	dir := t.TempDir()

	s1 := New(100, "", true, "")
	s1.SetAuditDir(dir, 30)
	s1.AppendAudit(AuditEntry{Action: "sync", Kind: "cluster", User: "alice"})

	// A second AppState loading the same dir should restore the entry.
	s2 := New(100, "", true, "")
	s2.SetAuditDir(dir, 30)

	s2.mu.RLock()
	entries := make([]AuditEntry, len(s2.auditLog))
	copy(entries, s2.auditLog)
	s2.mu.RUnlock()

	found := false
	for _, e := range entries {
		if e.Action == "sync" && e.User == "alice" {
			found = true
			break
		}
	}
	if !found {
		t.Error("audit entry written by s1 should be restored when s2 loads the same dir")
	}
}

func TestCleanOldAuditFiles(t *testing.T) {
	dir := t.TempDir()
	s := New(100, "", true, "")
	s.mu.Lock()
	s.auditDir = dir
	s.auditRetentionDays = 7
	s.mu.Unlock()

	oldDate1 := time.Now().UTC().AddDate(0, 0, -10).Format("2006-01-02")
	oldDate2 := time.Now().UTC().AddDate(0, 0, -8).Format("2006-01-02")
	todayDate := time.Now().UTC().Format("2006-01-02")

	for _, date := range []string{oldDate1, oldDate2, todayDate} {
		path := filepath.Join(dir, "audit-"+date+".jsonlog")
		if err := os.WriteFile(path, []byte{}, 0600); err != nil {
			t.Fatal(err)
		}
	}

	s.cleanOldAuditFiles()

	dirEntries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(dirEntries) != 1 {
		names := make([]string, len(dirEntries))
		for i, e := range dirEntries {
			names[i] = e.Name()
		}
		t.Errorf("expected 1 file to remain, got %d: %v", len(dirEntries), names)
	}
	if len(dirEntries) > 0 && dirEntries[0].Name() != "audit-"+todayDate+".jsonlog" {
		t.Errorf("remaining file should be today's, got %q", dirEntries[0].Name())
	}
}

func TestReadLastNAuditEntries(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "audit-test.jsonlog")

	entries := []AuditEntry{
		{Action: "sync", User: "alice", Kind: "cluster"},
		{Action: "delete", User: "bob", Kind: "cluster"},
		{Action: "refresh", User: "charlie", Kind: "machineclass"},
	}
	var lines []byte
	for _, e := range entries {
		data, _ := json.Marshal(e)
		lines = append(lines, data...)
		lines = append(lines, '\n')
	}
	if err := os.WriteFile(path, lines, 0600); err != nil {
		t.Fatal(err)
	}

	// Read all 3 entries.
	got := readLastNAuditEntries(path, 3)
	if len(got) != 3 {
		t.Fatalf("expected 3 entries, got %d", len(got))
	}

	// Read only the last 2.
	got2 := readLastNAuditEntries(path, 2)
	if len(got2) != 2 {
		t.Fatalf("expected 2 entries with limit=2, got %d", len(got2))
	}
	if got2[0].Action != "delete" {
		t.Errorf("first of last-2 should be 'delete', got %q", got2[0].Action)
	}
	if got2[1].Action != "refresh" {
		t.Errorf("second of last-2 should be 'refresh', got %q", got2[1].Action)
	}

	// Non-existent file returns nil.
	if readLastNAuditEntries(filepath.Join(dir, "nonexistent.jsonlog"), 10) != nil {
		t.Error("non-existent file should return nil")
	}
}
