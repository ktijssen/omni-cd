package git

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"omni-cd/internal/config"
	"omni-cd/internal/state"
)

// cloneTimeout is the maximum time allowed for a git clone operation.
// If a remote hangs, the clone is killed after this duration so the
// reconcile goroutine can unblock and recover on the next cycle.
const cloneTimeout = 5 * time.Minute

// lsRemoteTimeout is the maximum time allowed for git ls-remote (test connection).
const lsRemoteTimeout = 30 * time.Second

// workDirLocks serialises Sync() calls across all Client instances that share
// the same working directory path. buildMultiClient() constructs a fresh
// MultiClient — with new Client instances and new per-instance mutexes — on
// every call. When two goroutines call buildMultiClient().SyncAll() concurrently
// (e.g. the main reconcile goroutine and a triggerDeleteMC goroutine) they each
// get separate Client objects whose c.mu values are independent, so c.mu alone
// cannot prevent both from operating on /tmp/repo-<name> at the same time.
// This map provides a path-level lock that is shared across all Client instances
// for a given directory.
var workDirLocks sync.Map // string → *sync.Mutex

// workDirMu returns the shared mutex for a given working directory path,
// creating it on first use.
func workDirMu(dir string) *sync.Mutex {
	v, _ := workDirLocks.LoadOrStore(dir, new(sync.Mutex))
	return v.(*sync.Mutex)
}

// RemoveWorkDir deletes the working directory for a repo while holding the
// shared workDir lock. Callers outside this package (e.g. the reconcile
// cleanup path in cmd/omni-cd/main.go) must use this helper rather than
// os.RemoveAll directly so a concurrent Client.Sync() on the same path
// cannot have its directory yanked out from underneath it mid-clone.
func RemoveWorkDir(dir string) error {
	mu := workDirMu(dir)
	mu.Lock()
	defer mu.Unlock()
	return os.RemoveAll(dir)
}

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
	// Acquire the path-level lock before the per-instance lock so that two
	// Client instances created by separate buildMultiClient() calls cannot
	// race on the same /tmp/repo-<name> directory (e.g. one removing it while
	// the other is mid-clone).
	dirMu := workDirMu(c.workDir)
	dirMu.Lock()
	defer dirMu.Unlock()

	c.mu.Lock()
	defer c.mu.Unlock()
	repoURL := c.repoConfig.URL

	// Remove old clone and start fresh. If cleanup fails (permissions, EBUSY
	// on a file still held by a stuck child process), abort early — cloning
	// into a dirty workdir would produce a misleading "destination already
	// exists" error and mask the real problem.
	if err := os.RemoveAll(c.workDir); err != nil {
		return false, fmt.Errorf("failed to clean work directory %q: %w", c.workDir, err)
	}

	// Shallow clone the target branch — bounded by cloneTimeout so a
	// hanging remote cannot block the reconcile goroutine indefinitely.
	ctx, cancel := context.WithTimeout(context.Background(), cloneTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, "git", "clone",
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
		return false, fmt.Errorf("git clone failed for %s: %w\n%s",
			sanitizeGitURL(repoURL), err, scrubGitOutput(string(out)))
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
	ctx, cancel := context.WithTimeout(context.Background(), lsRemoteTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, "git", "ls-remote", "--heads", repoURL, "refs/heads/"+branch)
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
		return fmt.Errorf("%s", scrubGitOutput(msg))
	}
	if strings.TrimSpace(string(out)) == "" {
		return fmt.Errorf("branch %q not found in repository", branch)
	}
	return nil
}

// ── Helpers ───────────────────────────────────────────────────────────────────

// sanitizeGitURL strips userinfo (e.g. https://user:token@host/...) from a git
// URL so it can safely appear in logs or error chains. Returns the input
// unchanged on parse failure.
func sanitizeGitURL(raw string) string {
	if raw == "" {
		return raw
	}
	u, err := url.Parse(raw)
	if err != nil || u.User == nil {
		return raw
	}
	u.User = nil
	return u.String()
}

// scrubGitOutput redacts likely credential material from a git stderr/stdout
// blob before it is logged or wrapped into an error. Catches both URL-embedded
// tokens (https://x-access-token:abc@github.com) and bare-secret-looking
// substrings emitted by some git versions.
func scrubGitOutput(s string) string {
	if s == "" {
		return s
	}
	// Mask any userinfo embedded in URLs (https://user:pass@host -> https://***:***@host).
	reURLAuth := regexp.MustCompile(`(https?://)([^:@\s/]+):([^@\s/]+)@`)
	s = reURLAuth.ReplaceAllString(s, `$1***:***@`)
	// Mask common token prefixes when echoed standalone. Accepts ":", "=" or
	// whitespace between the keyword and the value (e.g. "bearer abc.def.ghi").
	reToken := regexp.MustCompile(`(?i)(token|password|bearer)[\s:=]+\S+`)
	s = reToken.ReplaceAllString(s, `$1=***`)
	return s
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

// formatLogMessage serialises a message with key-value pairs as a single
// JSON object for display in the web UI. attrs is expected to be alternating
// key/value pairs (slog convention); unpaired trailing keys are dropped.
//
// Builds an ordered slice of (key, value) entries and marshals them through
// json.Marshal — keys with embedded quotes/backslashes are handled correctly
// (the previous string-concat implementation could emit invalid JSON for
// such inputs).
func formatLogMessage(level, msg string, attrs ...any) string {
	type kv struct {
		Key string
		Val any
	}
	entries := []kv{
		{Key: "time", Val: time.Now().UTC().Format(time.RFC3339Nano)},
		{Key: "level", Val: level},
		{Key: "msg", Val: msg},
	}
	for i := 0; i+1 < len(attrs); i += 2 {
		entries = append(entries, kv{Key: fmt.Sprint(attrs[i]), Val: attrs[i+1]})
	}

	var buf strings.Builder
	buf.WriteByte('{')
	for i, e := range entries {
		if i > 0 {
			buf.WriteByte(',')
		}
		keyJSON, _ := json.Marshal(e.Key)
		valJSON, err := json.Marshal(e.Val)
		if err != nil {
			valJSON = []byte(`"<unrenderable>"`)
		}
		buf.Write(keyJSON)
		buf.WriteByte(':')
		buf.Write(valJSON)
	}
	buf.WriteByte('}')
	return buf.String()
}
