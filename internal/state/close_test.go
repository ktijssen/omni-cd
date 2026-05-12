package state

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// TestClose_FlushesLogAndAudit verifies that Close persists the tail of
// the most recent log and audit entries to disk before the file handle
// is released. Without Close, a process-exit during a buffered write
// can drop the final entry — the audit case is compliance-sensitive.
func TestClose_FlushesLogAndAudit(t *testing.T) {
	logDir := t.TempDir()
	auditDir := t.TempDir()

	s := New(100, "", true, "")
	s.SetLogDir(logDir, 1)
	s.SetAuditDir(auditDir, 1)

	s.AddLog("INFO", "Test", "log-tail-marker")
	s.AppendAudit(AuditEntry{
		Timestamp: time.Now().UTC(),
		User:      "tester",
		Action:    "sync",
		Resource:  "audit-tail-marker",
		Kind:      "cluster",
	})

	s.Close()

	today := time.Now().UTC().Format("2006-01-02")

	logPath := filepath.Join(logDir, "omni-cd-"+today+".jsonlog")
	logData, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("read log file: %v", err)
	}
	if !strings.Contains(string(logData), "log-tail-marker") {
		t.Errorf("log file missing marker; contents: %q", string(logData))
	}

	auditPath := filepath.Join(auditDir, "audit-"+today+".jsonlog")
	auditData, err := os.ReadFile(auditPath)
	if err != nil {
		t.Fatalf("read audit file: %v", err)
	}
	if !strings.Contains(string(auditData), "audit-tail-marker") {
		t.Errorf("audit file missing marker; contents: %q", string(auditData))
	}
}

// TestClose_Idempotent verifies that calling Close twice does not panic
// and the second call is a no-op.
func TestClose_Idempotent(t *testing.T) {
	logDir := t.TempDir()
	s := New(10, "", true, "")
	s.SetLogDir(logDir, 1)
	s.AddLog("INFO", "Test", "msg")
	s.Close()
	s.Close() // must not panic
}
