package git

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"strings"
	"time"

	"omni-cd/internal/config"
	"omni-cd/internal/state"
)

const workDir = "/tmp/repo"

// Client handles Git operations for omni-cd.
type Client struct {
	cfg     *config.Config
	state   *state.AppState
	lastSHA string
}

// New creates a new Git client with shared state.
func New(cfg *config.Config, s *state.AppState) *Client {
	return &Client{cfg: cfg, state: s}
}

// RepoDir returns the local path to the cloned repository.
func (c *Client) RepoDir() string {
	return workDir
}

// Sync performs a fresh shallow clone and returns true if the HEAD SHA
// has changed since the last sync. A fresh clone each cycle avoids
// issues with shallow fetch/reset on some Git versions.
func (c *Client) Sync() (bool, error) {
	repoURL := c.cfg.GitRepo

	// Inject token for private repos
	if c.cfg.GitToken != "" {
		repoURL = strings.Replace(repoURL, "https://", "https://token:"+c.cfg.GitToken+"@", 1)
	}

	// Remove old clone and start fresh
	os.RemoveAll(workDir)

	// Shallow clone the target branch
	cmd := exec.Command("git", "clone",
		"--branch", c.cfg.GitBranch,
		"--single-branch",
		"--depth", "1",
		repoURL, workDir,
		"--quiet",
	)
	if out, err := cmd.CombinedOutput(); err != nil {
		return false, fmt.Errorf("git clone failed: %w\n%s", err, string(out))
	}

	// Get the current HEAD SHA
	current, err := c.headSHA()
	if err != nil {
		return false, fmt.Errorf("failed to get HEAD: %w", err)
	}

	msg := c.commitMessage()

	// Update shared state with git info
	c.state.UpdateGit(state.GitInfo{
		SHA:           current,
		ShortSHA:      short(current),
		CommitMessage: msg,
		Branch:        c.cfg.GitBranch,
		Repo:          c.cfg.GitRepo,
		LastSync:      time.Now().UTC(),
	})

	previous := c.lastSHA
	c.lastSHA = current

	// First run — always treat as changed
	if previous == "" {
		c.logInfo("Cloned repository", "repo", c.cfg.GitRepo, "branch", c.cfg.GitBranch, "sha", short(current))
		return true, nil
	}

	// SHA changed — new commit detected
	if current != previous {
		c.logInfo("New commit detected", "sha", short(current), "message", msg)
		return true, nil
	}

	return false, nil
}

// headSHA returns the current HEAD SHA of the cloned repo.
func (c *Client) headSHA() (string, error) {
	out, err := exec.Command("git", "-C", workDir, "rev-parse", "HEAD").Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

// commitMessage returns the commit message of HEAD.
func (c *Client) commitMessage() string {
	out, err := exec.Command("git", "-C", workDir, "log", "-1", "--format=%s").Output()
	if err != nil {
		return "(unknown)"
	}
	return strings.TrimSpace(string(out))
}

// short returns the first 8 characters of a SHA.
func short(sha string) string {
	if len(sha) > 8 {
		return sha[:8]
	}
	return sha
}

func (c *Client) logDebug(msg string, attrs ...any) {
	// Add component as first attribute
	allAttrs := append([]any{"component", "Git"}, attrs...)
	slog.Debug(msg, allAttrs...)

	// Only add to web UI if this level is enabled
	if c.state != nil && slog.Default().Enabled(nil, slog.LevelDebug) {
		displayMsg := formatLogMessage("DEBUG", msg, allAttrs...)
		c.state.AddLog("DEBUG", "Git", displayMsg)
	}
}

func (c *Client) logInfo(msg string, attrs ...any) {
	// Add component as first attribute
	allAttrs := append([]any{"component", "Git"}, attrs...)
	slog.Info(msg, allAttrs...)

	// Only add to web UI if this level is enabled
	if c.state != nil && slog.Default().Enabled(nil, slog.LevelInfo) {
		displayMsg := formatLogMessage("INFO", msg, allAttrs...)
		c.state.AddLog("INFO", "Git", displayMsg)
	}
}

// formatLogMessage formats a message with key-value pairs as JSON for display
func formatLogMessage(level, msg string, attrs ...any) string {
	// Build a struct to ensure consistent field order
	type logEntry struct {
		Time  string `json:"time"`
		Level string `json:"level"`
		Msg   string `json:"msg"`
	}

	entry := logEntry{
		Time:  time.Now().UTC().Format(time.RFC3339Nano),
		Level: level,
		Msg:   msg,
	}

	// Start with the base fields
	var jsonParts []string
	baseJSON, _ := json.Marshal(entry)
	baseStr := string(baseJSON)
	// Remove closing brace
	baseStr = baseStr[:len(baseStr)-1]
	jsonParts = append(jsonParts, baseStr)

	// Add all attributes in order
	for i := 0; i < len(attrs); i += 2 {
		if i+1 < len(attrs) {
			key := fmt.Sprint(attrs[i])
			valJSON, _ := json.Marshal(attrs[i+1])
			jsonParts = append(jsonParts, fmt.Sprintf(`"%s":%s`, key, string(valJSON)))
		}
	}

	return strings.Join(jsonParts, ",") + "}"
}
