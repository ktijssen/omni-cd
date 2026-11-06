-- Initial schema. Timestamps are stored as RFC 3339 (nano) strings in UTC.

-- Singleton row holding scalar app state.
CREATE TABLE app_state (
    id                     INTEGER PRIMARY KEY CHECK (id = 1),
    last_reconcile         TEXT    NOT NULL, -- JSON state.ReconcileInfo
    clusters_enabled       INTEGER NOT NULL,
    repo_cluster_map       TEXT,             -- JSON map[string][]string
    repo_machine_class_map TEXT              -- JSON map[string][]string
);

-- Last-known clusters and machine classes. The full ResourceInfo is kept as
-- JSON; position preserves the order the UI displays them in.
CREATE TABLE resources (
    kind     TEXT    NOT NULL CHECK (kind IN ('cluster', 'machineclass')),
    position INTEGER NOT NULL,
    id       TEXT    NOT NULL,
    data     TEXT    NOT NULL, -- JSON state.ResourceInfo
    PRIMARY KEY (kind, position)
);

-- Repositories added via the UI (config-file repos are never stored here).
CREATE TABLE repos (
    position INTEGER PRIMARY KEY,
    name     TEXT NOT NULL,
    data     TEXT NOT NULL -- JSON config.RepoConfig
);

CREATE TABLE users (
    username      TEXT PRIMARY KEY,
    display_name  TEXT NOT NULL DEFAULT '',
    password_hash TEXT NOT NULL
);

CREATE TABLE oidc_users (
    email        TEXT PRIMARY KEY,
    display_name TEXT NOT NULL DEFAULT '',
    role         TEXT NOT NULL CHECK (role IN ('admin', 'viewer', 'none')),
    first_seen   TEXT NOT NULL,
    last_seen    TEXT NOT NULL
);

CREATE TABLE sessions (
    token        TEXT PRIMARY KEY,
    login_time   TEXT NOT NULL,
    username     TEXT NOT NULL,
    display_name TEXT NOT NULL DEFAULT '',
    auth_method  TEXT NOT NULL,
    role         TEXT NOT NULL DEFAULT ''
);

-- Singleton row holding the Omni connection entered via the UI.
CREATE TABLE omni_instance (
    id                  INTEGER PRIMARY KEY CHECK (id = 1),
    endpoint            TEXT NOT NULL,
    service_account_key TEXT NOT NULL
);

-- Key/value bookkeeping for the storage layer itself.
CREATE TABLE meta (
    key   TEXT PRIMARY KEY,
    value TEXT NOT NULL
);
