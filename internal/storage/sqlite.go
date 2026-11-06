package storage

import (
	"context"
	"database/sql"
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	_ "modernc.org/sqlite" // registers the pure-Go "sqlite" driver

	"omni-cd/internal/auth"
	"omni-cd/internal/config"
	"omni-cd/internal/maintenance"
	"omni-cd/internal/omniinstance"
	"omni-cd/internal/state"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

// SQLiteStore persists data in a single SQLite database file.
type SQLiteStore struct {
	db *sql.DB
}

var _ Store = (*SQLiteStore)(nil)

// OpenSQLite opens (creating if needed) the database at path and applies any
// pending schema migrations.
func OpenSQLite(path string) (*SQLiteStore, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return nil, err
	}
	// Pre-create the file so it gets 0600; SQLite gives the -wal and -shm
	// side files the same permissions as the main database file.
	f, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE, 0600)
	if err != nil {
		return nil, err
	}
	_ = f.Close()

	q := url.Values{}
	q.Add("_pragma", "busy_timeout(5000)")
	q.Add("_pragma", "journal_mode(WAL)")
	q.Add("_pragma", "synchronous(NORMAL)")
	q.Add("_pragma", "foreign_keys(1)")
	db, err := sql.Open("sqlite", "file:"+path+"?"+q.Encode())
	if err != nil {
		return nil, err
	}
	// One connection serialises all access, which avoids SQLITE_BUSY between
	// writers and is plenty for this workload.
	db.SetMaxOpenConns(1)

	s := &SQLiteStore{db: db}
	if err := s.migrate(); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("migrate %s: %w", path, err)
	}
	return s, nil
}

// Close closes the database.
func (s *SQLiteStore) Close() error { return s.db.Close() }

// migrate applies every migrations/NNNN_*.sql file whose number is above the
// database's user_version, each in its own transaction.
func (s *SQLiteStore) migrate() error {
	var current int
	if err := s.db.QueryRow(`PRAGMA user_version`).Scan(&current); err != nil {
		return err
	}

	entries, err := fs.ReadDir(migrationsFS, "migrations")
	if err != nil {
		return err
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Name() < entries[j].Name() })

	for _, e := range entries {
		prefix, _, ok := strings.Cut(e.Name(), "_")
		if !ok {
			return fmt.Errorf("migration %q: name must be NNNN_description.sql", e.Name())
		}
		version, err := strconv.Atoi(prefix)
		if err != nil {
			return fmt.Errorf("migration %q: %w", e.Name(), err)
		}
		if version <= current {
			continue
		}
		body, err := migrationsFS.ReadFile("migrations/" + e.Name())
		if err != nil {
			return err
		}
		err = s.inTx(func(tx *sql.Tx) error {
			if _, err := tx.Exec(string(body)); err != nil {
				return err
			}
			_, err := tx.Exec(fmt.Sprintf(`PRAGMA user_version = %d`, version))
			return err
		})
		if err != nil {
			return fmt.Errorf("migration %q: %w", e.Name(), err)
		}
		current = version
	}
	return nil
}

func (s *SQLiteStore) inTx(fn func(tx *sql.Tx) error) error {
	tx, err := s.db.BeginTx(context.Background(), nil)
	if err != nil {
		return err
	}
	if err := fn(tx); err != nil {
		_ = tx.Rollback()
		return err
	}
	return tx.Commit()
}

// ── state.Store ──────────────────────────────────────────────────────────────

func (s *SQLiteStore) LoadState() (*state.PersistedState, error) {
	var (
		lastReconcile, clusterMap, mcMap sql.NullString
		clustersEnabled                  bool
	)
	err := s.db.QueryRow(`SELECT last_reconcile, clusters_enabled, repo_cluster_map, repo_machine_class_map
		FROM app_state WHERE id = 1`).Scan(&lastReconcile, &clustersEnabled, &clusterMap, &mcMap)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	ps := &state.PersistedState{ClustersEnabled: clustersEnabled}
	if err := unmarshalNullable(lastReconcile, &ps.LastReconcile); err != nil {
		return nil, fmt.Errorf("last_reconcile: %w", err)
	}
	if err := unmarshalNullable(clusterMap, &ps.RepoClusterMap); err != nil {
		return nil, fmt.Errorf("repo_cluster_map: %w", err)
	}
	if err := unmarshalNullable(mcMap, &ps.RepoMachineClassMap); err != nil {
		return nil, fmt.Errorf("repo_machine_class_map: %w", err)
	}
	if ps.Clusters, err = s.loadResources("cluster"); err != nil {
		return nil, err
	}
	if ps.MachineClasses, err = s.loadResources("machineclass"); err != nil {
		return nil, err
	}
	if ps.MaintenanceWindows, err = s.loadMaintenanceWindows(); err != nil {
		return nil, err
	}
	return ps, nil
}

func (s *SQLiteStore) loadMaintenanceWindows() ([]maintenance.Window, error) {
	rows, err := s.db.Query(`SELECT data FROM maintenance_windows ORDER BY position`)
	if err != nil {
		return nil, err
	}
	defer rows.Close() //nolint:errcheck
	var out []maintenance.Window
	for rows.Next() {
		var data string
		if err := rows.Scan(&data); err != nil {
			return nil, err
		}
		var w maintenance.Window
		if err := json.Unmarshal([]byte(data), &w); err != nil {
			return nil, fmt.Errorf("maintenance window: %w", err)
		}
		out = append(out, w)
	}
	return out, rows.Err()
}

func (s *SQLiteStore) loadResources(kind string) ([]state.ResourceInfo, error) {
	rows, err := s.db.Query(`SELECT data FROM resources WHERE kind = ? ORDER BY position`, kind)
	if err != nil {
		return nil, err
	}
	defer rows.Close() //nolint:errcheck
	var out []state.ResourceInfo
	for rows.Next() {
		var data string
		if err := rows.Scan(&data); err != nil {
			return nil, err
		}
		var r state.ResourceInfo
		if err := json.Unmarshal([]byte(data), &r); err != nil {
			return nil, fmt.Errorf("%s resource: %w", kind, err)
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

func (s *SQLiteStore) SaveState(ps state.PersistedState) error {
	lastReconcile, err := json.Marshal(ps.LastReconcile)
	if err != nil {
		return err
	}
	clusterMap, err := marshalNullable(ps.RepoClusterMap)
	if err != nil {
		return err
	}
	mcMap, err := marshalNullable(ps.RepoMachineClassMap)
	if err != nil {
		return err
	}
	return s.inTx(func(tx *sql.Tx) error {
		if _, err := tx.Exec(`INSERT INTO app_state (id, last_reconcile, clusters_enabled, repo_cluster_map, repo_machine_class_map)
			VALUES (1, ?, ?, ?, ?)
			ON CONFLICT (id) DO UPDATE SET
				last_reconcile = excluded.last_reconcile,
				clusters_enabled = excluded.clusters_enabled,
				repo_cluster_map = excluded.repo_cluster_map,
				repo_machine_class_map = excluded.repo_machine_class_map`,
			string(lastReconcile), ps.ClustersEnabled, clusterMap, mcMap); err != nil {
			return err
		}
		if _, err := tx.Exec(`DELETE FROM resources`); err != nil {
			return err
		}
		stmt, err := tx.Prepare(`INSERT INTO resources (kind, position, id, data) VALUES (?, ?, ?, ?)`)
		if err != nil {
			return err
		}
		defer stmt.Close() //nolint:errcheck
		for kind, list := range map[string][]state.ResourceInfo{"cluster": ps.Clusters, "machineclass": ps.MachineClasses} {
			for i, r := range list {
				data, err := json.Marshal(r)
				if err != nil {
					return err
				}
				if _, err := stmt.Exec(kind, i, r.ID, string(data)); err != nil {
					return err
				}
			}
		}
		return saveMaintenanceWindows(tx, ps.MaintenanceWindows)
	})
}

func saveMaintenanceWindows(tx *sql.Tx, windows []maintenance.Window) error {
	if _, err := tx.Exec(`DELETE FROM maintenance_windows`); err != nil {
		return err
	}
	stmt, err := tx.Prepare(`INSERT INTO maintenance_windows (id, position, name, data) VALUES (?, ?, ?, ?)`)
	if err != nil {
		return err
	}
	defer stmt.Close() //nolint:errcheck
	for i, w := range windows {
		data, err := json.Marshal(w)
		if err != nil {
			return err
		}
		if _, err := stmt.Exec(w.ID, i, w.Name, string(data)); err != nil {
			return fmt.Errorf("maintenance window %q: %w", w.Name, err)
		}
	}
	return nil
}

func (s *SQLiteStore) LoadRepoConfigs() ([]config.RepoConfig, error) {
	rows, err := s.db.Query(`SELECT data FROM repos ORDER BY position`)
	if err != nil {
		return nil, err
	}
	defer rows.Close() //nolint:errcheck
	var out []config.RepoConfig
	for rows.Next() {
		var data string
		if err := rows.Scan(&data); err != nil {
			return nil, err
		}
		var r config.RepoConfig
		if err := json.Unmarshal([]byte(data), &r); err != nil {
			return nil, fmt.Errorf("repo: %w", err)
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

func (s *SQLiteStore) SaveRepoConfigs(repos []config.RepoConfig) error {
	return s.inTx(func(tx *sql.Tx) error {
		if _, err := tx.Exec(`DELETE FROM repos`); err != nil {
			return err
		}
		for i, r := range repos {
			data, err := json.Marshal(r)
			if err != nil {
				return err
			}
			if _, err := tx.Exec(`INSERT INTO repos (position, name, data) VALUES (?, ?, ?)`, i, r.Name, string(data)); err != nil {
				return err
			}
		}
		return nil
	})
}

// ── auth.UserRepository ──────────────────────────────────────────────────────

func (s *SQLiteStore) ListUsers() ([]auth.User, error) {
	rows, err := s.db.Query(`SELECT username, display_name, password_hash FROM users ORDER BY rowid`)
	if err != nil {
		return nil, err
	}
	defer rows.Close() //nolint:errcheck
	var out []auth.User
	for rows.Next() {
		var u auth.User
		if err := rows.Scan(&u.Username, &u.DisplayName, &u.PasswordHash); err != nil {
			return nil, err
		}
		out = append(out, u)
	}
	return out, rows.Err()
}

func (s *SQLiteStore) GetUser(username string) (auth.User, error) {
	u := auth.User{Username: username}
	err := s.db.QueryRow(`SELECT display_name, password_hash FROM users WHERE username = ?`, username).
		Scan(&u.DisplayName, &u.PasswordHash)
	if errors.Is(err, sql.ErrNoRows) {
		return auth.User{}, auth.ErrNotFound
	}
	return u, err
}

func (s *SQLiteStore) PutUser(u auth.User) error {
	_, err := s.db.Exec(`INSERT INTO users (username, display_name, password_hash) VALUES (?, ?, ?)
		ON CONFLICT (username) DO UPDATE SET
			display_name = excluded.display_name,
			password_hash = excluded.password_hash`,
		u.Username, u.DisplayName, u.PasswordHash)
	return err
}

// ── auth.OIDCUserRepository ──────────────────────────────────────────────────

func (s *SQLiteStore) ListOIDCUsers() ([]auth.OIDCUser, error) {
	rows, err := s.db.Query(`SELECT email, display_name, role, first_seen, last_seen FROM oidc_users ORDER BY rowid`)
	if err != nil {
		return nil, err
	}
	defer rows.Close() //nolint:errcheck
	var out []auth.OIDCUser
	for rows.Next() {
		var (
			u                   auth.OIDCUser
			firstSeen, lastSeen string
		)
		if err := rows.Scan(&u.Email, &u.DisplayName, &u.Role, &firstSeen, &lastSeen); err != nil {
			return nil, err
		}
		if u.FirstSeen, err = parseTime(firstSeen); err != nil {
			return nil, err
		}
		if u.LastSeen, err = parseTime(lastSeen); err != nil {
			return nil, err
		}
		out = append(out, u)
	}
	return out, rows.Err()
}

func (s *SQLiteStore) GetOIDCUser(email string) (auth.OIDCUser, error) {
	var (
		u                   = auth.OIDCUser{Email: email}
		firstSeen, lastSeen string
	)
	err := s.db.QueryRow(`SELECT display_name, role, first_seen, last_seen FROM oidc_users WHERE email = ?`, email).
		Scan(&u.DisplayName, &u.Role, &firstSeen, &lastSeen)
	if errors.Is(err, sql.ErrNoRows) {
		return auth.OIDCUser{}, auth.ErrNotFound
	}
	if err != nil {
		return auth.OIDCUser{}, err
	}
	if u.FirstSeen, err = parseTime(firstSeen); err != nil {
		return auth.OIDCUser{}, err
	}
	if u.LastSeen, err = parseTime(lastSeen); err != nil {
		return auth.OIDCUser{}, err
	}
	return u, nil
}

func (s *SQLiteStore) PutOIDCUser(u auth.OIDCUser) error {
	_, err := s.db.Exec(`INSERT INTO oidc_users (email, display_name, role, first_seen, last_seen) VALUES (?, ?, ?, ?, ?)
		ON CONFLICT (email) DO UPDATE SET
			display_name = excluded.display_name,
			role = excluded.role,
			first_seen = excluded.first_seen,
			last_seen = excluded.last_seen`,
		u.Email, u.DisplayName, u.Role, formatTime(u.FirstSeen), formatTime(u.LastSeen))
	return err
}

func (s *SQLiteStore) DeleteOIDCUser(email string) error {
	res, err := s.db.Exec(`DELETE FROM oidc_users WHERE email = ?`, email)
	if err != nil {
		return err
	}
	if n, err := res.RowsAffected(); err != nil {
		return err
	} else if n == 0 {
		return auth.ErrNotFound
	}
	return nil
}

// ── auth.SessionRepository ───────────────────────────────────────────────────

func (s *SQLiteStore) LoadSessions() (map[string]auth.Session, error) {
	rows, err := s.db.Query(`SELECT token, login_time, username, display_name, auth_method, role FROM sessions`)
	if err != nil {
		return nil, err
	}
	defer rows.Close() //nolint:errcheck
	out := map[string]auth.Session{}
	for rows.Next() {
		var (
			token, loginTime string
			sess             auth.Session
		)
		if err := rows.Scan(&token, &loginTime, &sess.Username, &sess.DisplayName, &sess.AuthMethod, &sess.Role); err != nil {
			return nil, err
		}
		if sess.LoginTime, err = parseTime(loginTime); err != nil {
			return nil, err
		}
		out[token] = sess
	}
	return out, rows.Err()
}

func (s *SQLiteStore) SaveSessions(sessions map[string]auth.Session) error {
	return s.inTx(func(tx *sql.Tx) error {
		if _, err := tx.Exec(`DELETE FROM sessions`); err != nil {
			return err
		}
		for token, sess := range sessions {
			if _, err := tx.Exec(`INSERT INTO sessions (token, login_time, username, display_name, auth_method, role)
				VALUES (?, ?, ?, ?, ?, ?)`,
				token, formatTime(sess.LoginTime), sess.Username, sess.DisplayName, sess.AuthMethod, sess.Role); err != nil {
				return err
			}
		}
		return nil
	})
}

// ── omniinstance.Store ───────────────────────────────────────────────────────

func (s *SQLiteStore) LoadInstanceConfig() (*omniinstance.InstanceConfig, error) {
	var cfg omniinstance.InstanceConfig
	err := s.db.QueryRow(`SELECT endpoint, service_account_key FROM omni_instance WHERE id = 1`).
		Scan(&cfg.Endpoint, &cfg.ServiceAccountKey)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &cfg, nil
}

func (s *SQLiteStore) SaveInstanceConfig(cfg omniinstance.InstanceConfig) error {
	_, err := s.db.Exec(`INSERT INTO omni_instance (id, endpoint, service_account_key) VALUES (1, ?, ?)
		ON CONFLICT (id) DO UPDATE SET
			endpoint = excluded.endpoint,
			service_account_key = excluded.service_account_key`,
		cfg.Endpoint, cfg.ServiceAccountKey)
	return err
}

func (s *SQLiteStore) DeleteInstanceConfig() error {
	_, err := s.db.Exec(`DELETE FROM omni_instance WHERE id = 1`)
	return err
}

// ── meta ─────────────────────────────────────────────────────────────────────

func (s *SQLiteStore) getMeta(key string) (string, bool, error) {
	var v string
	err := s.db.QueryRow(`SELECT value FROM meta WHERE key = ?`, key).Scan(&v)
	if errors.Is(err, sql.ErrNoRows) {
		return "", false, nil
	}
	return v, err == nil, err
}

func (s *SQLiteStore) setMeta(key, value string) error {
	_, err := s.db.Exec(`INSERT INTO meta (key, value) VALUES (?, ?)
		ON CONFLICT (key) DO UPDATE SET value = excluded.value`, key, value)
	return err
}

// ── helpers ──────────────────────────────────────────────────────────────────

func formatTime(t time.Time) string { return t.UTC().Format(time.RFC3339Nano) }

func parseTime(s string) (time.Time, error) { return time.Parse(time.RFC3339Nano, s) }

// marshalNullable encodes v as JSON, mapping a nil map to SQL NULL so it
// round-trips back to nil.
func marshalNullable(v map[string][]string) (any, error) {
	if v == nil {
		return nil, nil
	}
	b, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	return string(b), nil
}

func unmarshalNullable(s sql.NullString, v any) error {
	if !s.Valid {
		return nil
	}
	return json.Unmarshal([]byte(s.String), v)
}
