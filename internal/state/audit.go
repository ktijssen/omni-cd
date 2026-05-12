package state

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const maxAuditEntries = 500

// AuditEntry records a single user-initiated action.
type AuditEntry struct {
	Timestamp time.Time `json:"timestamp"`
	User      string    `json:"user"`     // display name, "system", or "webhook"
	Action    string    `json:"action"`   // "sync", "refresh", "delete", "auto-sync-on", "auto-sync-off", "repo-add", "repo-update", "repo-delete", "omni-update", "global-sync-on", "global-sync-off"
	Resource  string    `json:"resource"` // cluster/MC name, repo name, or ""
	Kind      string    `json:"kind"`     // "cluster", "machineclass", "repo", "omni", "global"
}

// AppendAudit adds an entry to the in-memory ring buffer and the daily audit file.
func (s *AppState) AppendAudit(entry AuditEntry) {
	if entry.Timestamp.IsZero() {
		entry.Timestamp = time.Now().UTC()
	}
	s.mu.Lock()
	s.auditLog = append(s.auditLog, entry)
	if len(s.auditLog) > maxAuditEntries {
		s.auditLog = s.auditLog[len(s.auditLog)-maxAuditEntries:]
	}
	if s.auditDir != "" {
		today := entry.Timestamp.Format("2006-01-02")
		if today != s.auditFileDate {
			s.rotateAuditFile(today)
		}
		if s.auditFile != nil {
			if data, err := json.Marshal(entry); err == nil {
				// Audit data is compliance-sensitive — write failure must be
				// loud, never swallowed.
				if _, werr := s.auditFile.Write(append(data, '\n')); werr != nil {
					slog.Error("Failed to write audit entry", "error", werr, "component", "State")
				}
			} else {
				slog.Error("Failed to marshal audit entry", "error", err, "component", "State")
			}
		}
	}
	s.mu.Unlock()
}

// GetAuditLog returns a copy of the in-memory audit log, newest first.
func (s *AppState) GetAuditLog() []AuditEntry {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]AuditEntry, len(s.auditLog))
	for i, e := range s.auditLog {
		out[len(s.auditLog)-1-i] = e
	}
	return out
}

// SetAuditDir configures the audit log directory. Call once after New().
func (s *AppState) SetAuditDir(dir string, retentionDays int) {
	if err := os.MkdirAll(dir, 0750); err != nil {
		slog.Error("Failed to create audit dir", "error", err, "component", "State")
		return
	}
	today := time.Now().UTC().Format("2006-01-02")
	s.mu.Lock()
	s.auditDir = dir
	s.auditRetentionDays = retentionDays
	s.rotateAuditFile(today)
	s.mu.Unlock()

	// Restore recent entries from today's file.
	path := filepath.Join(dir, "audit-"+today+".jsonlog")
	if entries := readLastNAuditEntries(path, maxAuditEntries); len(entries) > 0 {
		s.mu.Lock()
		s.auditLog = append(entries, s.auditLog...)
		if len(s.auditLog) > maxAuditEntries {
			s.auditLog = s.auditLog[len(s.auditLog)-maxAuditEntries:]
		}
		s.mu.Unlock()
	}

	go s.cleanOldAuditFiles()
}

// rotateAuditFile opens a new audit file for the given date. Must be called with s.mu held.
func (s *AppState) rotateAuditFile(date string) {
	if s.auditFile != nil {
		if err := s.auditFile.Close(); err != nil {
			slog.Error("Failed to close audit file during rotation", "error", err, "component", "State")
		}
		s.auditFile = nil
	}
	path := filepath.Join(s.auditDir, "audit-"+date+".jsonlog")
	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		slog.Error("Failed to open audit file", "error", err, "component", "State")
		return
	}
	s.auditFile = f
	s.auditFileDate = date
}

// cleanOldAuditFiles deletes audit files older than auditRetentionDays.
func (s *AppState) cleanOldAuditFiles() {
	s.mu.RLock()
	dir := s.auditDir
	days := s.auditRetentionDays
	s.mu.RUnlock()
	if dir == "" || days <= 0 {
		return
	}
	cutoff := time.Now().UTC().AddDate(0, 0, -days)
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if !strings.HasPrefix(name, "audit-") || !strings.HasSuffix(name, ".jsonlog") {
			continue
		}
		dateStr := strings.TrimSuffix(strings.TrimPrefix(name, "audit-"), ".jsonlog")
		t, err := time.Parse("2006-01-02", dateStr)
		if err != nil {
			continue
		}
		if t.Before(cutoff) {
			os.Remove(filepath.Join(dir, name))
		}
	}
}

func readLastNAuditEntries(path string, n int) []AuditEntry {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	lines := bytes.Split(bytes.TrimRight(data, "\n"), []byte("\n"))
	if len(lines) > n {
		lines = lines[len(lines)-n:]
	}
	var entries []AuditEntry
	for _, line := range lines {
		if len(line) == 0 {
			continue
		}
		var e AuditEntry
		if err := json.Unmarshal(line, &e); err == nil {
			entries = append(entries, e)
		}
	}
	return entries
}

// auditUser returns a display-safe actor label: display name if available,
// "webhook" for webhook-triggered actions, or "system" for automated reconciles.
func auditUser(displayName string) string {
	if displayName == "" {
		return "system"
	}
	if strings.EqualFold(displayName, "webhook") {
		return "webhook"
	}
	return displayName
}
