-- Maintenance windows that gate auto-sync. The full maintenance.Window is kept
-- as JSON; position preserves the order they were created in.
CREATE TABLE maintenance_windows (
    id       TEXT    PRIMARY KEY,
    position INTEGER NOT NULL UNIQUE,
    name     TEXT    NOT NULL,
    data     TEXT    NOT NULL -- JSON maintenance.Window
);
