// Package ipwatch records LAN device presence: who is online, since when,
// and for how long. It periodically sweeps the subnet with parallel pings
// and enriches results with MAC addresses from the kernel neighbor table,
// storing intervals and raw sightings in SQLite for later analysis.
package ipwatch

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite"
)

const schema = `
CREATE TABLE IF NOT EXISTS devices (
  ip         TEXT PRIMARY KEY,
  mac        TEXT,
  first_seen TEXT NOT NULL,          -- RFC3339 UTC
  last_seen  TEXT NOT NULL
);
CREATE TABLE IF NOT EXISTS presence (
  id         INTEGER PRIMARY KEY,
  ip         TEXT NOT NULL,
  mac        TEXT,
  online_at  TEXT NOT NULL,          -- RFC3339 UTC
  offline_at TEXT                    -- NULL = currently online
);
CREATE INDEX IF NOT EXISTS idx_presence_ip ON presence(ip);
CREATE INDEX IF NOT EXISTS idx_presence_open ON presence(offline_at);
CREATE TABLE IF NOT EXISTS sightings (
  id     INTEGER PRIMARY KEY,
  ts     TEXT NOT NULL,              -- RFC3339 UTC
  ip     TEXT NOT NULL,
  mac    TEXT,
  source TEXT NOT NULL,              -- ping | neigh
  rtt_ms REAL
);
CREATE INDEX IF NOT EXISTS idx_sightings_ts ON sightings(ts);
CREATE INDEX IF NOT EXISTS idx_sightings_ip ON sightings(ip);
`

// OpenDB opens (creating if needed) the SQLite database at path.
func OpenDB(path string) (*sql.DB, error) {
	if dir := filepath.Dir(path); dir != "." && dir != "" {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return nil, fmt.Errorf("create db dir: %w", err)
		}
	}
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	// Single writer daemon: keep it simple and safe.
	db.SetMaxOpenConns(1)
	if _, err := db.Exec(schema); err != nil {
		db.Close()
		return nil, fmt.Errorf("init schema: %w", err)
	}
	return db, nil
}
