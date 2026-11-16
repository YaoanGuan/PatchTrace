package graph

import (
	"database/sql"
	"fmt"
)

// schemaVersion is bumped whenever the on-disk schema changes. The value is
// stored in SQLite's user_version pragma.
const schemaVersion = 1

const schemaV1 = `
CREATE TABLE IF NOT EXISTS projects (
    id                TEXT PRIMARY KEY,
    title             TEXT NOT NULL,
    origin            TEXT NOT NULL,
    goal              TEXT NOT NULL,
    status            TEXT NOT NULL,
    bootstrap_enabled INTEGER NOT NULL DEFAULT 1,
    created_at        TEXT NOT NULL,
    reason_worker     TEXT,
    reason_trigger    TEXT,
    reason_started_at TEXT,
    reason_heartbeat_at TEXT,
    reason_expire_at  TEXT
);

CREATE TABLE IF NOT EXISTS nodes (
    id           TEXT NOT NULL,
    project_id   TEXT NOT NULL,
    kind         TEXT NOT NULL,
    type         TEXT NOT NULL DEFAULT '',
    content      TEXT NOT NULL,
    state        TEXT NOT NULL DEFAULT '',
    from_json    TEXT NOT NULL DEFAULT '[]',
    to_id        TEXT,
    creator      TEXT NOT NULL DEFAULT '',
    worker       TEXT,
    lease_until  TEXT,
    created_at   TEXT NOT NULL,
    concluded_at TEXT,
    PRIMARY KEY (project_id, id)
);
CREATE INDEX IF NOT EXISTS idx_nodes_project_kind ON nodes(project_id, kind);

CREATE TABLE IF NOT EXISTS edges (
    project_id TEXT NOT NULL,
    from_id    TEXT NOT NULL,
    to_id      TEXT NOT NULL,
    kind       TEXT NOT NULL,
    PRIMARY KEY (project_id, from_id, to_id, kind)
);
CREATE INDEX IF NOT EXISTS idx_edges_project ON edges(project_id);

CREATE TABLE IF NOT EXISTS counters (
    project_id TEXT NOT NULL,
    name       TEXT NOT NULL,
    value      INTEGER NOT NULL,
    PRIMARY KEY (project_id, name)
);
`

func migrate(db *sql.DB) error {
	if _, err := db.Exec(`PRAGMA busy_timeout=5000`); err != nil {
		return fmt.Errorf("graph: set busy_timeout: %w", err)
	}
	var journalMode string
	if err := db.QueryRow(`PRAGMA journal_mode=WAL`).Scan(&journalMode); err != nil {
		return fmt.Errorf("graph: enable WAL: %w", err)
	}

	var version int
	if err := db.QueryRow(`PRAGMA user_version`).Scan(&version); err != nil {
		return fmt.Errorf("graph: read schema version: %w", err)
	}
	if version >= schemaVersion {
		return nil
	}

	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("graph: begin migration: %w", err)
	}
	defer tx.Rollback()

	if _, err := tx.Exec(schemaV1); err != nil {
		return fmt.Errorf("graph: apply schema v%d: %w", schemaVersion, err)
	}
	if _, err := tx.Exec(fmt.Sprintf(`PRAGMA user_version = %d`, schemaVersion)); err != nil {
		return fmt.Errorf("graph: record schema version: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("graph: commit migration: %w", err)
	}
	return nil
}
