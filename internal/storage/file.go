package storage

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sync"

	"omni-cd/internal/auth"
	"omni-cd/internal/config"
	"omni-cd/internal/omniinstance"
	"omni-cd/internal/state"
)

// File paths relative to the data directory. These match the layout used
// before the storage abstraction existed, so existing volumes keep working.
const (
	stateFile     = "state/state.json"
	reposFile     = "config/repos.json"
	usersFile     = "auth/users.json"
	oidcUsersFile = "config/oidc-users.json"
	sessionsFile  = "auth/sessions.json"
	instanceFile  = "config/instances.json"
)

// FileStore persists data as JSON files under a data directory. Each record
// type lives in its own file; every write replaces the whole file atomically
// (temp file + rename) so a crash never leaves a truncated file behind.
type FileStore struct {
	dir string
	mu  sync.Mutex // serialises writes (and read-modify-write cycles) across all files
}

var _ Store = (*FileStore)(nil)

// NewFileStore returns a FileStore rooted at dir (normally /data).
func NewFileStore(dir string) *FileStore {
	return &FileStore{dir: dir}
}

// Close is a no-op; FileStore holds no open handles.
func (f *FileStore) Close() error { return nil }

// ── state.Store ──────────────────────────────────────────────────────────────

func (f *FileStore) LoadState() (*state.PersistedState, error) {
	var ps state.PersistedState
	found, err := f.readJSON(stateFile, &ps)
	if err != nil || !found {
		return nil, err
	}
	return &ps, nil
}

func (f *FileStore) SaveState(ps state.PersistedState) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.writeJSON(stateFile, ps, true, 0750)
}

func (f *FileStore) LoadRepoConfigs() ([]config.RepoConfig, error) {
	var repos []config.RepoConfig
	_, err := f.readJSON(reposFile, &repos)
	return repos, err
}

func (f *FileStore) SaveRepoConfigs(repos []config.RepoConfig) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.writeJSON(reposFile, repos, false, 0700)
}

// ── auth.UserRepository ──────────────────────────────────────────────────────

func (f *FileStore) ListUsers() ([]auth.User, error) {
	var users []auth.User
	_, err := f.readJSON(usersFile, &users)
	return users, err
}

func (f *FileStore) GetUser(username string) (auth.User, error) {
	users, err := f.ListUsers()
	if err != nil {
		return auth.User{}, err
	}
	for _, u := range users {
		if u.Username == username {
			return u, nil
		}
	}
	return auth.User{}, auth.ErrNotFound
}

func (f *FileStore) PutUser(u auth.User) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	users, err := f.ListUsers()
	if err != nil {
		return err
	}
	replaced := false
	for i := range users {
		if users[i].Username == u.Username {
			users[i] = u
			replaced = true
			break
		}
	}
	if !replaced {
		users = append(users, u)
	}
	return f.writeJSON(usersFile, users, false, 0700)
}

// ── auth.OIDCUserRepository ──────────────────────────────────────────────────

func (f *FileStore) ListOIDCUsers() ([]auth.OIDCUser, error) {
	var users []auth.OIDCUser
	_, err := f.readJSON(oidcUsersFile, &users)
	return users, err
}

func (f *FileStore) GetOIDCUser(email string) (auth.OIDCUser, error) {
	users, err := f.ListOIDCUsers()
	if err != nil {
		return auth.OIDCUser{}, err
	}
	for _, u := range users {
		if u.Email == email {
			return u, nil
		}
	}
	return auth.OIDCUser{}, auth.ErrNotFound
}

func (f *FileStore) PutOIDCUser(u auth.OIDCUser) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	users, err := f.ListOIDCUsers()
	if err != nil {
		return err
	}
	replaced := false
	for i := range users {
		if users[i].Email == u.Email {
			users[i] = u
			replaced = true
			break
		}
	}
	if !replaced {
		users = append(users, u)
	}
	return f.writeJSON(oidcUsersFile, users, true, 0700)
}

func (f *FileStore) DeleteOIDCUser(email string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	users, err := f.ListOIDCUsers()
	if err != nil {
		return err
	}
	for i := range users {
		if users[i].Email == email {
			users = append(users[:i], users[i+1:]...)
			return f.writeJSON(oidcUsersFile, users, true, 0700)
		}
	}
	return auth.ErrNotFound
}

// ── auth.SessionRepository ───────────────────────────────────────────────────

func (f *FileStore) LoadSessions() (map[string]auth.Session, error) {
	sessions := map[string]auth.Session{}
	_, err := f.readJSON(sessionsFile, &sessions)
	return sessions, err
}

func (f *FileStore) SaveSessions(sessions map[string]auth.Session) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.writeJSON(sessionsFile, sessions, false, 0700)
}

// ── omniinstance.Store ───────────────────────────────────────────────────────

func (f *FileStore) LoadInstanceConfig() (*omniinstance.InstanceConfig, error) {
	var cfg omniinstance.InstanceConfig
	found, err := f.readJSON(instanceFile, &cfg)
	if err != nil || !found {
		return nil, err
	}
	return &cfg, nil
}

func (f *FileStore) SaveInstanceConfig(cfg omniinstance.InstanceConfig) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.writeJSON(instanceFile, cfg, true, 0700)
}

func (f *FileStore) DeleteInstanceConfig() error {
	err := os.Remove(filepath.Join(f.dir, instanceFile))
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	return err
}

// ── helpers ──────────────────────────────────────────────────────────────────

// readJSON decodes the file at rel into v. It reports found=false (and leaves
// v untouched) when the file does not exist.
func (f *FileStore) readJSON(rel string, v any) (found bool, err error) {
	data, err := os.ReadFile(filepath.Join(f.dir, rel))
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if err := json.Unmarshal(data, v); err != nil {
		return false, err
	}
	return true, nil
}

// writeJSON encodes v and atomically replaces the file at rel with it.
// Callers must hold f.mu so concurrent writers never share the temp file.
func (f *FileStore) writeJSON(rel string, v any, indent bool, dirPerm os.FileMode) error {
	var (
		data []byte
		err  error
	)
	if indent {
		data, err = json.MarshalIndent(v, "", "  ")
	} else {
		data, err = json.Marshal(v)
	}
	if err != nil {
		return err
	}

	path := filepath.Join(f.dir, rel)
	if err := os.MkdirAll(filepath.Dir(path), dirPerm); err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0600); err != nil {
		return err
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	return nil
}
