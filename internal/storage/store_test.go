package storage

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"omni-cd/internal/auth"
	"omni-cd/internal/config"
	"omni-cd/internal/maintenance"
	"omni-cd/internal/omniinstance"
	"omni-cd/internal/state"
)

// testStoreContract exercises every Store method. Each backend runs it with a
// factory that returns a fresh, empty store; reopen returns a second handle on
// the same underlying data to verify persistence.
func testStoreContract(t *testing.T, open func(t *testing.T) (fresh Store, reopen func() Store)) {
	t.Run("EmptyStore", func(t *testing.T) {
		s, _ := open(t)
		if ps, err := s.LoadState(); err != nil || ps != nil {
			t.Fatalf("LoadState on empty store = %v, %v; want nil, nil", ps, err)
		}
		if repos, err := s.LoadRepoConfigs(); err != nil || len(repos) != 0 {
			t.Fatalf("LoadRepoConfigs = %v, %v; want empty", repos, err)
		}
		if users, err := s.ListUsers(); err != nil || len(users) != 0 {
			t.Fatalf("ListUsers = %v, %v; want empty", users, err)
		}
		if _, err := s.GetUser("nobody"); !errors.Is(err, auth.ErrNotFound) {
			t.Fatalf("GetUser = %v; want ErrNotFound", err)
		}
		if _, err := s.GetOIDCUser("nobody@example.com"); !errors.Is(err, auth.ErrNotFound) {
			t.Fatalf("GetOIDCUser = %v; want ErrNotFound", err)
		}
		if err := s.DeleteOIDCUser("nobody@example.com"); !errors.Is(err, auth.ErrNotFound) {
			t.Fatalf("DeleteOIDCUser = %v; want ErrNotFound", err)
		}
		if sess, err := s.LoadSessions(); err != nil || len(sess) != 0 {
			t.Fatalf("LoadSessions = %v, %v; want empty", sess, err)
		}
		if cfg, err := s.LoadInstanceConfig(); err != nil || cfg != nil {
			t.Fatalf("LoadInstanceConfig = %v, %v; want nil, nil", cfg, err)
		}
		if err := s.DeleteInstanceConfig(); err != nil {
			t.Fatalf("DeleteInstanceConfig on empty store: %v", err)
		}
	})

	t.Run("State", func(t *testing.T) {
		s, reopen := open(t)
		autoSync := true
		want := state.PersistedState{
			LastReconcile:   state.ReconcileInfo{Type: state.ReconcileHard, Status: state.StatusSuccess},
			RepoClusterMap:  map[string][]string{"repo": {"c1"}},
			Clusters:        []state.ResourceInfo{{ID: "c1", Status: "synced", AutoSync: &autoSync}},
			MachineClasses:  []state.ResourceInfo{{ID: "mc1", Status: "synced"}},
			ClustersEnabled: true,
			MaintenanceWindows: []maintenance.Window{
				{ID: "w2", Name: "Weekend", TimeRange: maintenance.TimeRange{Days: []string{"sat"}, Start: "06:00", End: "10:00"}, Clusters: []string{"c1"}},
				{ID: "w1", Name: "Dev", TimeRange: maintenance.TimeRange{Days: []string{"mon"}, Start: "22:00", End: "02:00"}, Clusters: []string{}, Selector: map[string]string{"env": "dev"}},
			},
		}
		if err := s.SaveState(want); err != nil {
			t.Fatal(err)
		}
		got, err := reopen().LoadState()
		if err != nil || got == nil {
			t.Fatalf("LoadState = %v, %v", got, err)
		}
		if len(got.Clusters) != 1 || got.Clusters[0].ID != "c1" || got.Clusters[0].AutoSync == nil || !*got.Clusters[0].AutoSync {
			t.Errorf("clusters not round-tripped: %+v", got.Clusters)
		}
		if len(got.MachineClasses) != 1 || got.MachineClasses[0].ID != "mc1" {
			t.Errorf("machine classes not round-tripped: %+v", got.MachineClasses)
		}
		if got.LastReconcile.Status != state.StatusSuccess || !got.ClustersEnabled || got.RepoClusterMap["repo"][0] != "c1" {
			t.Errorf("state fields not round-tripped: %+v", got)
		}
		if !reflect.DeepEqual(got.MaintenanceWindows, want.MaintenanceWindows) {
			t.Errorf("maintenance windows = %+v, want %+v (order must be kept)", got.MaintenanceWindows, want.MaintenanceWindows)
		}

		// Saving fewer windows must drop the removed ones.
		want.MaintenanceWindows = want.MaintenanceWindows[1:]
		if err := s.SaveState(want); err != nil {
			t.Fatal(err)
		}
		if got, err := reopen().LoadState(); err != nil || len(got.MaintenanceWindows) != 1 || got.MaintenanceWindows[0].ID != "w1" {
			t.Errorf("after removing a window: %+v, %v", got, err)
		}
	})

	t.Run("RepoConfigs", func(t *testing.T) {
		s, reopen := open(t)
		want := []config.RepoConfig{{Name: "a", URL: "https://example.com/a.git", Branch: "main"}}
		if err := s.SaveRepoConfigs(want); err != nil {
			t.Fatal(err)
		}
		got, err := reopen().LoadRepoConfigs()
		if err != nil || len(got) != 1 || got[0].Name != "a" || got[0].Branch != "main" {
			t.Fatalf("LoadRepoConfigs = %+v, %v", got, err)
		}
	})

	t.Run("Users", func(t *testing.T) {
		s, reopen := open(t)
		if err := s.PutUser(auth.User{Username: "admin", DisplayName: "Admin", PasswordHash: "h1"}); err != nil {
			t.Fatal(err)
		}
		if err := s.PutUser(auth.User{Username: "admin", DisplayName: "Root", PasswordHash: "h2"}); err != nil {
			t.Fatal(err)
		}
		r := reopen()
		users, err := r.ListUsers()
		if err != nil || len(users) != 1 {
			t.Fatalf("ListUsers = %+v, %v; want 1 user (put must replace)", users, err)
		}
		u, err := r.GetUser("admin")
		if err != nil || u.DisplayName != "Root" || u.PasswordHash != "h2" {
			t.Fatalf("GetUser = %+v, %v", u, err)
		}
	})

	t.Run("OIDCUsers", func(t *testing.T) {
		s, reopen := open(t)
		now := time.Now().UTC().Truncate(time.Second)
		for _, u := range []auth.OIDCUser{
			{Email: "a@example.com", Role: "viewer", FirstSeen: now, LastSeen: now},
			{Email: "b@example.com", Role: "viewer", FirstSeen: now, LastSeen: now},
			{Email: "a@example.com", Role: "admin", FirstSeen: now, LastSeen: now},
		} {
			if err := s.PutOIDCUser(u); err != nil {
				t.Fatal(err)
			}
		}
		if err := s.DeleteOIDCUser("b@example.com"); err != nil {
			t.Fatal(err)
		}
		r := reopen()
		users, err := r.ListOIDCUsers()
		if err != nil || len(users) != 1 {
			t.Fatalf("ListOIDCUsers = %+v, %v; want 1 user", users, err)
		}
		u, err := r.GetOIDCUser("a@example.com")
		if err != nil || u.Role != "admin" || !u.FirstSeen.Equal(now) {
			t.Fatalf("GetOIDCUser = %+v, %v", u, err)
		}
	})

	t.Run("Sessions", func(t *testing.T) {
		s, reopen := open(t)
		now := time.Now().UTC().Truncate(time.Second)
		if err := s.SaveSessions(map[string]auth.Session{
			"t1": {LoginTime: now, Username: "admin", AuthMethod: "local"},
			"t2": {LoginTime: now, Username: "a@example.com", AuthMethod: "oidc", Role: "viewer"},
		}); err != nil {
			t.Fatal(err)
		}
		// SaveSessions replaces the whole set.
		if err := s.SaveSessions(map[string]auth.Session{
			"t2": {LoginTime: now, Username: "a@example.com", AuthMethod: "oidc", Role: "viewer"},
		}); err != nil {
			t.Fatal(err)
		}
		got, err := reopen().LoadSessions()
		if err != nil || len(got) != 1 || got["t2"].Role != "viewer" || !got["t2"].LoginTime.Equal(now) {
			t.Fatalf("LoadSessions = %+v, %v", got, err)
		}
	})

	t.Run("InstanceConfig", func(t *testing.T) {
		s, reopen := open(t)
		if err := s.SaveInstanceConfig(omniinstance.InstanceConfig{Endpoint: "https://omni", ServiceAccountKey: "k"}); err != nil {
			t.Fatal(err)
		}
		r := reopen()
		cfg, err := r.LoadInstanceConfig()
		if err != nil || cfg == nil || cfg.Endpoint != "https://omni" || cfg.ServiceAccountKey != "k" {
			t.Fatalf("LoadInstanceConfig = %+v, %v", cfg, err)
		}
		if err := r.DeleteInstanceConfig(); err != nil {
			t.Fatal(err)
		}
		if cfg, err := r.LoadInstanceConfig(); err != nil || cfg != nil {
			t.Fatalf("after delete LoadInstanceConfig = %+v, %v; want nil, nil", cfg, err)
		}
	})
}

func TestFileStore(t *testing.T) {
	testStoreContract(t, func(t *testing.T) (Store, func() Store) {
		dir := t.TempDir()
		return NewFileStore(dir), func() Store { return NewFileStore(dir) }
	})
}

func TestSQLiteStore(t *testing.T) {
	testStoreContract(t, func(t *testing.T) (Store, func() Store) {
		path := filepath.Join(t.TempDir(), DatabaseFile)
		s, err := OpenSQLite(path)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = s.Close() })
		return s, func() Store {
			r, err := OpenSQLite(path) // also re-runs migrate on an up-to-date DB
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = r.Close() })
			return r
		}
	})
}

func TestSQLiteStore_NilMapsRoundTrip(t *testing.T) {
	s, err := OpenSQLite(filepath.Join(t.TempDir(), DatabaseFile))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close() //nolint:errcheck
	if err := s.SaveState(state.PersistedState{}); err != nil {
		t.Fatal(err)
	}
	ps, err := s.LoadState()
	if err != nil || ps == nil {
		t.Fatalf("LoadState = %v, %v", ps, err)
	}
	if ps.RepoClusterMap != nil || ps.RepoMachineClassMap != nil || ps.Clusters != nil {
		t.Errorf("nil fields should stay nil: %+v", ps)
	}
}

func TestOpen_ImportsLegacyFiles(t *testing.T) {
	dir := t.TempDir()
	now := time.Now().UTC().Truncate(time.Second)

	// Seed the pre-SQLite JSON layout.
	legacy := NewFileStore(dir)
	must(t, legacy.PutUser(auth.User{Username: "admin", PasswordHash: "h"}))
	must(t, legacy.PutOIDCUser(auth.OIDCUser{Email: "a@example.com", Role: "admin", FirstSeen: now, LastSeen: now}))
	must(t, legacy.SaveRepoConfigs([]config.RepoConfig{{Name: "r1", URL: "https://example.com/r1.git"}}))
	must(t, legacy.SaveInstanceConfig(omniinstance.InstanceConfig{Endpoint: "https://omni", ServiceAccountKey: "k"}))
	must(t, legacy.SaveState(state.PersistedState{ClustersEnabled: true, Clusters: []state.ResourceInfo{{ID: "c1"}},
		MaintenanceWindows: []maintenance.Window{{ID: "w1", Name: "Dev", TimeRange: maintenance.TimeRange{Days: []string{"mon"}, Start: "06:00", End: "10:00"}, Selector: map[string]string{"env": "dev"}}}}))
	must(t, legacy.SaveSessions(map[string]auth.Session{"tok": {LoginTime: now, Username: "admin", AuthMethod: "local"}}))

	s, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	if u, err := s.GetUser("admin"); err != nil || u.PasswordHash != "h" {
		t.Errorf("user not imported: %+v, %v", u, err)
	}
	if u, err := s.GetOIDCUser("a@example.com"); err != nil || u.Role != "admin" {
		t.Errorf("OIDC user not imported: %+v, %v", u, err)
	}
	if repos, err := s.LoadRepoConfigs(); err != nil || len(repos) != 1 || repos[0].Name != "r1" {
		t.Errorf("repos not imported: %+v, %v", repos, err)
	}
	if cfg, err := s.LoadInstanceConfig(); err != nil || cfg == nil || cfg.Endpoint != "https://omni" {
		t.Errorf("instance config not imported: %+v, %v", cfg, err)
	}
	if ps, err := s.LoadState(); err != nil || ps == nil || len(ps.Clusters) != 1 || !ps.ClustersEnabled ||
		len(ps.MaintenanceWindows) != 1 || ps.MaintenanceWindows[0].Selector["env"] != "dev" {
		t.Errorf("state not imported: %+v, %v", ps, err)
	}
	if sess, err := s.LoadSessions(); err != nil || sess["tok"].Username != "admin" {
		t.Errorf("sessions not imported: %+v, %v", sess, err)
	}

	for _, rel := range []string{usersFile, oidcUsersFile, reposFile, instanceFile, stateFile, sessionsFile} {
		if _, err := os.Stat(filepath.Join(dir, rel)); !os.IsNotExist(err) {
			t.Errorf("%s should have been renamed", rel)
		}
		if _, err := os.Stat(filepath.Join(dir, rel+".migrated")); err != nil {
			t.Errorf("%s.migrated missing: %v", rel, err)
		}
	}

	// A change made after the import must survive a reopen when a JSON file
	// older than the import is left behind (e.g. one that could not be renamed).
	must(t, s.PutUser(auth.User{Username: "admin", PasswordHash: "new"}))
	must(t, s.Close())
	must(t, legacy.PutUser(auth.User{Username: "admin", PasswordHash: "stale"}))
	old := time.Now().Add(-time.Hour)
	must(t, os.Chtimes(filepath.Join(dir, usersFile), old, old))

	s2, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	if u, _ := s2.GetUser("admin"); u.PasswordHash != "new" {
		t.Errorf("stale file imported: password hash = %q, want %q", u.PasswordHash, "new")
	}
	if _, err := os.Stat(filepath.Join(dir, usersFile)); err != nil {
		t.Errorf("a skipped file must be left in place: %v", err)
	}
	must(t, s2.Close())

	// Files written after the import come from an older version (rollback and
	// upgrade again) and hold newer data, so they are imported again.
	must(t, os.Remove(filepath.Join(dir, usersFile)))
	must(t, legacy.SaveState(state.PersistedState{ClustersEnabled: true,
		MaintenanceWindows: []maintenance.Window{{ID: "w2", Name: "Rollback", TimeRange: maintenance.TimeRange{Days: []string{"sun"}, Start: "01:00", End: "02:00"}, Clusters: []string{"c1"}}}}))
	future := time.Now().Add(time.Hour)
	must(t, os.Chtimes(filepath.Join(dir, stateFile), future, future))

	s3, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer s3.Close() //nolint:errcheck
	if ps, err := s3.LoadState(); err != nil || len(ps.MaintenanceWindows) != 1 || ps.MaintenanceWindows[0].Name != "Rollback" {
		t.Errorf("state written after the import not imported: %+v, %v", ps, err)
	}
	if u, _ := s3.GetUser("admin"); u.PasswordHash != "new" {
		t.Errorf("re-import touched data without a newer file: password hash = %q", u.PasswordHash)
	}
	if _, err := os.Stat(filepath.Join(dir, stateFile)); !os.IsNotExist(err) {
		t.Errorf("re-imported %s should have been renamed", stateFile)
	}
}

func TestOpen_CorruptFileFailsWithoutRenaming(t *testing.T) {
	for _, rel := range []string{usersFile, stateFile} {
		t.Run(rel, func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, rel)
			must(t, os.MkdirAll(filepath.Dir(path), 0700))
			must(t, os.WriteFile(path, []byte("{not json"), 0600))

			if s, err := Open(dir); err == nil {
				_ = s.Close()
				t.Fatalf("Open should fail on an unreadable %s", rel)
			}
			if _, err := os.Stat(path); err != nil {
				t.Errorf("%s must be left in place: %v", rel, err)
			}
		})
	}
}

func TestOpenSQLite_FilePermissions(t *testing.T) {
	path := filepath.Join(t.TempDir(), DatabaseFile)
	s, err := OpenSQLite(path)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close() //nolint:errcheck
	fi, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if perm := fi.Mode().Perm(); perm != 0600 {
		t.Errorf("database permissions = %o, want 600", perm)
	}
}

func must(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}
