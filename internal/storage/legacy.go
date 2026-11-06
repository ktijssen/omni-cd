package storage

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"time"
)

// DatabaseFile is the SQLite database's name inside the data directory.
const DatabaseFile = "omni-cd.db"

// legacyImportKey records in the meta table when the pre-SQLite JSON files
// were last imported.
const legacyImportKey = "legacy_json_import"

// Open opens the SQLite store in dataDir and, on first use, imports the JSON
// files written by earlier versions. Imported files are renamed to
// <name>.migrated so they stay available for rollback but are not read again.
func Open(dataDir string) (*SQLiteStore, error) {
	s, err := OpenSQLite(filepath.Join(dataDir, DatabaseFile))
	if err != nil {
		return nil, err
	}
	if err := s.importLegacyFiles(dataDir); err != nil {
		_ = s.Close()
		return nil, fmt.Errorf("import legacy JSON files from %s: %w", dataDir, err)
	}
	return s, nil
}

// importLegacyFiles copies everything a FileStore in dir holds into s.
//
// Every write is an upsert or a full replace, so if the process dies midway
// the import simply runs again on next start. Files holding data that cannot
// be rebuilt (users, roles, repos, Omni credentials) must parse or the import
// fails, leaving the files untouched. The state file must parse too, as it
// holds the maintenance windows; sessions are best-effort since a fresh login
// recreates them.
//
// A file written after the last import was written by an older version, as
// happens when rolling back to an older image and upgrading again, so it is
// imported again. Files not modified since the last import (one that could not
// be renamed) are skipped so they never overwrite newer data in the database.
func (s *SQLiteStore) importLegacyFiles(dir string) error {
	var since time.Time
	if v, done, err := s.getMeta(legacyImportKey); err != nil {
		return err
	} else if done {
		if since, err = time.Parse(time.RFC3339Nano, v); err != nil {
			return fmt.Errorf("meta %s: %w", legacyImportKey, err)
		}
	}

	legacy := NewFileStore(dir)
	var present []string
	exists := func(rel string) bool {
		fi, err := os.Stat(filepath.Join(dir, rel))
		if err != nil || !fi.ModTime().After(since) {
			return false
		}
		present = append(present, rel)
		return true
	}

	if exists(usersFile) {
		users, err := legacy.ListUsers()
		if err != nil {
			return fmt.Errorf("%s: %w", usersFile, err)
		}
		for _, u := range users {
			if err := s.PutUser(u); err != nil {
				return err
			}
		}
		slog.Info("Imported users", "count", len(users), "component", "Storage")
	}

	if exists(oidcUsersFile) {
		users, err := legacy.ListOIDCUsers()
		if err != nil {
			return fmt.Errorf("%s: %w", oidcUsersFile, err)
		}
		for _, u := range users {
			if err := s.PutOIDCUser(u); err != nil {
				return fmt.Errorf("%s: user %q: %w", oidcUsersFile, u.Email, err)
			}
		}
		slog.Info("Imported OIDC users", "count", len(users), "component", "Storage")
	}

	if exists(reposFile) {
		repos, err := legacy.LoadRepoConfigs()
		if err != nil {
			return fmt.Errorf("%s: %w", reposFile, err)
		}
		if err := s.SaveRepoConfigs(repos); err != nil {
			return err
		}
		slog.Info("Imported repositories", "count", len(repos), "component", "Storage")
	}

	if exists(instanceFile) {
		cfg, err := legacy.LoadInstanceConfig()
		if err != nil {
			return fmt.Errorf("%s: %w", instanceFile, err)
		}
		if cfg != nil {
			if err := s.SaveInstanceConfig(*cfg); err != nil {
				return err
			}
		}
		slog.Info("Imported Omni instance config", "component", "Storage")
	}

	if exists(stateFile) {
		ps, err := legacy.LoadState()
		if err != nil {
			return fmt.Errorf("%s: %w", stateFile, err)
		}
		if ps != nil {
			if err := s.SaveState(*ps); err != nil {
				return err
			}
			slog.Info("Imported app state", "clusters", len(ps.Clusters), "machineClasses", len(ps.MachineClasses),
				"maintenanceWindows", len(ps.MaintenanceWindows), "component", "Storage")
		}
	}

	if exists(sessionsFile) {
		if sessions, err := legacy.LoadSessions(); err != nil {
			slog.Warn("Skipping unreadable legacy sessions file", "file", sessionsFile, "error", err, "component", "Storage")
		} else if err := s.SaveSessions(sessions); err != nil {
			return err
		} else {
			slog.Info("Imported sessions", "count", len(sessions), "component", "Storage")
		}
	}

	if len(present) == 0 && !since.IsZero() {
		return nil
	}
	if !since.IsZero() {
		slog.Warn("Imported JSON files written after the last import, probably by an older version",
			"files", present, "component", "Storage")
	}
	if err := s.setMeta(legacyImportKey, time.Now().UTC().Format(time.RFC3339Nano)); err != nil {
		return err
	}

	for _, rel := range present {
		path := filepath.Join(dir, rel)
		if err := os.Rename(path, path+".migrated"); err != nil {
			slog.Warn("Could not rename imported file", "file", path, "error", err, "component", "Storage")
		}
	}
	return nil
}
