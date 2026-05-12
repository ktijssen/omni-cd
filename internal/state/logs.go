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

// AddLog appends a log entry to the in-memory ring buffer and the daily log file.
func (s *AppState) AddLog(level, label, message string) {
	s.mu.Lock()
	entry := LogEntry{
		Timestamp: time.Now().UTC(),
		Level:     level,
		Label:     label,
		Message:   message,
	}
	s.Logs = append(s.Logs, entry)
	if len(s.Logs) > s.maxLogs {
		s.Logs = s.Logs[len(s.Logs)-s.maxLogs:]
	}
	if s.logDir != "" {
		today := entry.Timestamp.Format("2006-01-02")
		if today != s.logFileDate {
			s.rotateLogFile(today)
		}
		if s.logFile != nil {
			if data, err := json.Marshal(entry); err == nil {
				if _, werr := s.logFile.Write(append(data, '\n')); werr != nil {
					slog.Error("Failed to write log entry", "error", werr, "component", "State")
				}
			} else {
				slog.Error("Failed to marshal log entry", "error", err, "component", "State")
			}
		}
	}
	s.mu.Unlock()
	s.notifyChange()
}

// SetLogDir configures daily log file rotation. Call once after New().
func (s *AppState) SetLogDir(dir string, retentionDays int) {
	if err := os.MkdirAll(dir, 0750); err != nil {
		slog.Error("Failed to create log dir", "error", err, "component", "State")
		return
	}
	today := time.Now().UTC().Format("2006-01-02")
	s.mu.Lock()
	s.logDir = dir
	s.logRetentionDays = retentionDays
	s.rotateLogFile(today)
	s.mu.Unlock()

	// Populate ring buffer from today's file so history survives restarts.
	path := filepath.Join(dir, "omni-cd-"+today+".jsonlog")
	if entries := readLastNLogEntries(path, s.maxLogs); len(entries) > 0 {
		s.mu.Lock()
		s.Logs = append(entries, s.Logs...)
		if len(s.Logs) > s.maxLogs {
			s.Logs = s.Logs[len(s.Logs)-s.maxLogs:]
		}
		s.mu.Unlock()
	}

	go s.cleanOldLogFiles()
}

// rotateLogFile opens a new log file for the given date. Must be called with s.mu held.
func (s *AppState) rotateLogFile(date string) {
	if s.logFile != nil {
		if err := s.logFile.Close(); err != nil {
			slog.Error("Failed to close log file during rotation", "error", err, "component", "State")
		}
		s.logFile = nil
	}
	path := filepath.Join(s.logDir, "omni-cd-"+date+".jsonlog")
	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		slog.Error("Failed to open log file", "error", err, "component", "State")
		return
	}
	s.logFile = f
	s.logFileDate = date
}

// cleanOldLogFiles deletes log files older than logRetentionDays.
func (s *AppState) cleanOldLogFiles() {
	s.mu.RLock()
	dir := s.logDir
	days := s.logRetentionDays
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
		if !strings.HasPrefix(name, "omni-cd-") || !strings.HasSuffix(name, ".jsonlog") {
			continue
		}
		dateStr := strings.TrimSuffix(strings.TrimPrefix(name, "omni-cd-"), ".jsonlog")
		t, err := time.Parse("2006-01-02", dateStr)
		if err != nil {
			continue
		}
		if t.Before(cutoff) {
			os.Remove(filepath.Join(dir, name))
		}
	}
}

// readLastNLogEntries reads the last n JSONL entries from a log file.
func readLastNLogEntries(path string, n int) []LogEntry {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	lines := bytes.Split(bytes.TrimRight(data, "\n"), []byte("\n"))
	if len(lines) > n {
		lines = lines[len(lines)-n:]
	}
	var entries []LogEntry
	for _, line := range lines {
		if len(line) == 0 {
			continue
		}
		var e LogEntry
		if err := json.Unmarshal(line, &e); err == nil {
			entries = append(entries, e)
		}
	}
	return entries
}
