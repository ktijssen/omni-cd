package git

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"omni-cd/internal/config"
	"omni-cd/internal/state"
)

// Client handles Git operations for a single repository.
type Client struct {
	repoConfig config.RepoConfig
	workDir    string
	state      *state.AppState
	lastSHA    string
	mu         sync.Mutex // serialises concurrent Sync() calls on the same working directory
}

// New creates a git.Client for the primary (first) configured repository.
// This preserves backwards compatibility for callers that only use one repo.
func New(cfg *config.Config, s *state.AppState) *Client {
	rc := cfg.Repos[0]
	return &Client{
		repoConfig: rc,
		workDir:    "/tmp/repo-" + rc.Name,
		state:      s,
	}
}

// RepoDir returns the local path to the cloned repository.
func (c *Client) RepoDir() string {
	return c.workDir
}

// Sync performs a fresh shallow clone and returns true if the HEAD SHA
// has changed since the last sync. A fresh clone each cycle avoids
// issues with shallow fetch/reset on some Git versions.
// The method serialises concurrent callers via a per-repo mutex so that a
// single-cluster refresh goroutine and a full reconcile cannot clone/read
// the working directory simultaneously.
func (c *Client) Sync() (bool, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	repoURL := c.repoConfig.URL

	// Remove old clone and start fresh
	os.RemoveAll(c.workDir)

	// Shallow clone the target branch
	cmd := exec.Command("git", "clone",
		"--branch", c.repoConfig.Branch,
		"--single-branch",
		"--depth", "1",
		repoURL, c.workDir,
		"--quiet",
	)

	// Inject credentials via a temporary HOME so the token never appears in the
	// URL (which leaks via git error messages and /proc process listings).
	if c.repoConfig.Token != "" {
		u, err := url.Parse(repoURL)
		if err != nil {
			return false, fmt.Errorf("invalid repo URL: %w", err)
		}
		tmpHome, err := os.MkdirTemp("", "git-auth-*")
		if err != nil {
			return false, fmt.Errorf("failed to create credentials dir: %w", err)
		}
		defer os.RemoveAll(tmpHome)
		netrc := fmt.Sprintf("machine %s login token password %s\n", u.Hostname(), c.repoConfig.Token)
		if err := os.WriteFile(filepath.Join(tmpHome, ".netrc"), []byte(netrc), 0600); err != nil {
			return false, fmt.Errorf("failed to write git credentials: %w", err)
		}
		cmd.Env = append(os.Environ(), "HOME="+tmpHome, "GIT_TERMINAL_PROMPT=0")
	}

	if out, err := cmd.CombinedOutput(); err != nil {
		return false, fmt.Errorf("git clone failed: %w\n%s", err, string(out))
	}

	// Get the current HEAD SHA
	current, err := c.headSHA()
	if err != nil {
		return false, fmt.Errorf("failed to get HEAD: %w", err)
	}

	msg := c.commitMessage()
	author := c.commitAuthor()

	// Update shared state with git info
	c.state.UpdateGit(state.GitInfo{
		Name:          c.repoConfig.Name,
		SHA:           current,
		ShortSHA:      short(current),
		CommitMessage: msg,
		CommitAuthor:  author,
		Branch:        c.repoConfig.Branch,
		Repo:          c.repoConfig.URL,
		LastSync:      time.Now().UTC(),
	})

	previous := c.lastSHA
	c.lastSHA = current

	// First run — always treat as changed
	if previous == "" {
		c.logInfo(fmt.Sprintf("Cloned repository %s (branch: %s, sha: %s)", c.repoConfig.URL, c.repoConfig.Branch, short(current)))
		return true, nil
	}

	// SHA changed — new commit detected
	if current != previous {
		c.logInfo(fmt.Sprintf("New commit detected: %s — %s", short(current), msg))
		return true, nil
	}

	return false, nil
}

// headSHA returns the current HEAD SHA of the cloned repo.
func (c *Client) headSHA() (string, error) {
	out, err := exec.Command("git", "-C", c.workDir, "rev-parse", "HEAD").Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

// commitMessage returns the subject line of the HEAD commit.
func (c *Client) commitMessage() string {
	out, err := exec.Command("git", "-C", c.workDir, "log", "-1", "--format=%s").Output()
	if err != nil {
		return "(unknown)"
	}
	return strings.TrimSpace(string(out))
}

// commitAuthor returns the author name of the HEAD commit.
func (c *Client) commitAuthor() string {
	out, err := exec.Command("git", "-C", c.workDir, "log", "-1", "--format=%an").Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

// ── MultiClient ───────────────────────────────────────────────────────────────

// RepoSyncResult holds the outcome of syncing one repository.
type RepoSyncResult struct {
	Name    string
	Changed bool
	Err     error
	RepoDir string
	Info    state.GitInfo
}

// MultiClient manages multiple git Clients, one per configured repository.
type MultiClient struct {
	clients []*Client
}

// NewMulti creates a MultiClient for all repos in cfg.
func NewMulti(cfg *config.Config, s *state.AppState) *MultiClient {
	clients := make([]*Client, len(cfg.Repos))
	for i, rc := range cfg.Repos {
		clients[i] = &Client{
			repoConfig: rc,
			workDir:    "/tmp/repo-" + rc.Name,
			state:      s,
		}
	}
	return &MultiClient{clients: clients}
}

// SyncAll clones/refreshes all repos sequentially and returns per-repo results.
// State is updated for each repo immediately after it completes.
func (m *MultiClient) SyncAll() []RepoSyncResult {
	results := make([]RepoSyncResult, 0, len(m.clients))
	infos := make([]state.GitInfo, 0, len(m.clients))

	for _, c := range m.clients {
		changed, err := c.Sync()
		info := state.GitInfo{
			Name:          c.repoConfig.Name,
			SHA:           c.lastSHA,
			ShortSHA:      short(c.lastSHA),
			CommitMessage: c.commitMessage(),
			CommitAuthor:  c.commitAuthor(),
			Branch:        c.repoConfig.Branch,
			Repo:          c.repoConfig.URL,
			LastSync:      time.Now().UTC(),
		}
		if err != nil {
			info.SyncError = err.Error()
		}
		results = append(results, RepoSyncResult{
			Name:    c.repoConfig.Name,
			Changed: changed,
			Err:     err,
			RepoDir: c.workDir,
			Info:    info,
		})
		infos = append(infos, info)
	}

	// Publish all git infos at once so the UI sees a consistent snapshot
	if len(m.clients) > 0 {
		m.clients[0].state.UpdateRepos(infos)
	}
	return results
}

// RepoDirFor returns the workDir for the named repo, or "" if not found.
func (m *MultiClient) RepoDirFor(name string) string {
	for _, c := range m.clients {
		if c.repoConfig.Name == name {
			return c.workDir
		}
	}
	return ""
}

// AllRepoDirs returns a slice of workDir paths for all repos.
func (m *MultiClient) AllRepoDirs() []string {
	dirs := make([]string, len(m.clients))
	for i, c := range m.clients {
		dirs[i] = c.workDir
	}
	return dirs
}

// TestConnection verifies that the given repo URL, branch, and optional token
// are reachable by running git ls-remote. No local files are written.
func TestConnection(repoURL, branch, token string) error {
	if branch == "" {
		branch = "main"
	}
	cmd := exec.Command("git", "ls-remote", "--heads", repoURL, "refs/heads/"+branch)
	cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0")

	if token != "" {
		u, err := url.Parse(repoURL)
		if err != nil {
			return fmt.Errorf("invalid repo URL: %w", err)
		}
		tmpHome, err := os.MkdirTemp("", "git-auth-*")
		if err != nil {
			return fmt.Errorf("failed to create credentials dir: %w", err)
		}
		defer os.RemoveAll(tmpHome)
		netrc := fmt.Sprintf("machine %s login token password %s\n", u.Hostname(), token)
		if err := os.WriteFile(filepath.Join(tmpHome, ".netrc"), []byte(netrc), 0600); err != nil {
			return fmt.Errorf("failed to write git credentials: %w", err)
		}
		cmd.Env = append(os.Environ(), "HOME="+tmpHome, "GIT_TERMINAL_PROMPT=0")
	}

	out, err := cmd.CombinedOutput()
	if err != nil {
		msg := strings.TrimSpace(string(out))
		if msg == "" {
			msg = err.Error()
		}
		msg = strings.TrimPrefix(msg, "fatal: ")
		return fmt.Errorf("%s", msg)
	}
	if strings.TrimSpace(string(out)) == "" {
		return fmt.Errorf("branch %q not found in repository", branch)
	}
	return nil
}

// ── Helpers ───────────────────────────────────────────────────────────────────

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
