package ipwatch

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"os/exec"
	"strings"
	"sync"
	"time"
)

// Tracker turns probe rounds into presence intervals persisted in SQLite.
type Tracker struct {
	db           *sql.DB
	offlineAfter int // consecutive missed rounds before marking offline

	mu     sync.Mutex
	misses map[string]int   // ip -> consecutive misses
	open   map[string]int64 // ip -> open presence row id
}

// NewTracker creates a Tracker. offlineAfter is how many consecutive
// failed rounds a device may miss before it is marked offline (debounces
// Wi-Fi sleep flicker).
func NewTracker(db *sql.DB, offlineAfter int) (*Tracker, error) {
	if offlineAfter < 1 {
		offlineAfter = 3
	}
	t := &Tracker{
		db:           db,
		offlineAfter: offlineAfter,
		misses:       make(map[string]int),
		open:         make(map[string]int64),
	}
	// Re-adopt presence rows left open by a previous run.
	rows, err := t.db.Query(`SELECT id, ip FROM presence WHERE offline_at IS NULL`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var id int64
		var ip string
		if err := rows.Scan(&id, &ip); err != nil {
			return nil, err
		}
		t.open[ip] = id
	}
	return t, rows.Err()
}

// Round processes one full scan cycle: alive results, MAC map (ip->mac),
// and additional online IPs observed in an active neighbor state.
// safe to call sequentially from a single goroutine.
func (t *Tracker) Round(ctx context.Context, now time.Time, alive []PingResult, macs map[string]string, neighActive map[string]bool) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	seen := make(map[string]PingResult, len(alive)+len(neighActive))
	for _, r := range alive {
		if r.Alive {
			seen[r.IP] = r
		}
	}
	// Neighbor evidence counts as alive too (catches ICMP-shy devices
	// that are actively exchanging packets with the gateway).
	for ip := range neighActive {
		if _, ok := seen[ip]; !ok {
			seen[ip] = PingResult{IP: ip, Alive: true}
		}
	}

	ts := now.UTC().Format(time.RFC3339)

	// Persist raw sightings for this round.
	tx, err := t.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	stmt, err := tx.Prepare(`INSERT INTO sightings (ts, ip, mac, source, rtt_ms) VALUES (?, ?, ?, ?, ?)`)
	if err != nil {
		tx.Rollback()
		return err
	}
	pingAlive := map[string]bool{}
	for _, r := range alive {
		if r.Alive {
			pingAlive[r.IP] = true
		}
	}
	for ip, r := range seen {
		source := "neigh"
		var rtt sql.NullFloat64
		if pingAlive[ip] {
			source = "ping"
			if r.RTT > 0 {
				rtt = sql.NullFloat64{Float64: float64(r.RTT) / 1e6, Valid: true}
			}
		}
		if _, err := stmt.Exec(ts, ip, nullStr(macs[ip]), source, rtt); err != nil {
			tx.Rollback()
			return err
		}
	}
	stmt.Close()

	// Device inventory upserts.
	devStmt, err := tx.Prepare(`INSERT INTO devices (ip, mac, first_seen, last_seen)
		VALUES (?, ?, ?, ?)
		ON CONFLICT(ip) DO UPDATE SET
		  mac = COALESCE(excluded.mac, devices.mac),
		  last_seen = excluded.last_seen`)
	if err != nil {
		tx.Rollback()
		return err
	}
	for ip := range seen {
		if _, err := devStmt.Exec(ip, nullStr(macs[ip]), ts, ts); err != nil {
			tx.Rollback()
			return err
		}
	}
	devStmt.Close()

	// Transitions: newly online.
	for ip := range seen {
		if _, ok := t.open[ip]; ok {
			t.misses[ip] = 0
			// Enrich the open episode with MAC once known. Must go
			// through tx — the pool is capped at one connection and an
			// open transaction owns it (db.Exec here would deadlock).
			if mac, ok := macs[ip]; ok {
				if _, err := tx.Exec(`UPDATE presence SET mac = ? WHERE id = ?`, mac, t.open[ip]); err != nil {
					tx.Rollback()
					return err
				}
			}
			continue
		}
		res, err := tx.Exec(`INSERT INTO presence (ip, mac, online_at) VALUES (?, ?, ?)`,
			ip, nullStr(macs[ip]), ts)
		if err != nil {
			tx.Rollback()
			return err
		}
		id, _ := res.LastInsertId()
		t.open[ip] = id
		t.misses[ip] = 0
		slog.Info("device online", "ip", ip, "mac", macs[ip])
	}

	// Transitions: missed this round; offline after offlineAfter misses.
	for ip := range t.open {
		if _, ok := seen[ip]; ok {
			continue
		}
		t.misses[ip]++
		if t.misses[ip] >= t.offlineAfter {
			if _, err := tx.Exec(`UPDATE presence SET offline_at = ? WHERE id = ? AND offline_at IS NULL`,
				ts, t.open[ip]); err != nil {
				tx.Rollback()
				return err
			}
			slog.Info("device offline", "ip", ip, "misses", t.misses[ip])
			delete(t.open, ip)
			delete(t.misses, ip)
		}
	}
	return tx.Commit()
}

// PruneSightings deletes sightings older than retain (0 = keep forever).
func (t *Tracker) PruneSightings(ctx context.Context, retain time.Duration) (int64, error) {
	if retain <= 0 {
		return 0, nil
	}
	cutoff := time.Now().UTC().Add(-retain).Format(time.RFC3339)
	res, err := t.db.ExecContext(ctx, `DELETE FROM sightings WHERE ts < ?`, cutoff)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

// Device is one inventoried host.
type Device struct {
	IP        string
	MAC       string
	FirstSeen string
	LastSeen  string
	Online    bool
}

// Summary lists inventoried devices (newest last_seen first).
func (t *Tracker) Summary() ([]Device, error) {
	rows, err := t.db.Query(`
		SELECT d.ip, COALESCE(d.mac, ''), d.first_seen, d.last_seen,
		       EXISTS(SELECT 1 FROM presence p WHERE p.ip = d.ip AND p.offline_at IS NULL)
		FROM devices d ORDER BY d.last_seen DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Device
	for rows.Next() {
		var d Device
		var online bool
		if err := rows.Scan(&d.IP, &d.MAC, &d.FirstSeen, &d.LastSeen, &online); err != nil {
			return nil, err
		}
		d.Online = online
		out = append(out, d)
	}
	return out, rows.Err()
}

// TodayPresence returns presence intervals overlapping today (server local),
// newest first — the analysis view behind -summary's episode list.
func (t *Tracker) TodayPresence() ([]PresenceRow, error) {
	day := time.Now().UTC().Format("2006-01-02")
	rows, err := t.db.Query(`
		SELECT id, ip, COALESCE(mac,''), online_at, COALESCE(offline_at,'')
		FROM presence
		WHERE substr(online_at,1,10) >= ? OR (offline_at IS NULL)
		ORDER BY online_at DESC LIMIT 200`, day)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []PresenceRow
	for rows.Next() {
		var p PresenceRow
		if err := rows.Scan(&p.ID, &p.IP, &p.MAC, &p.OnlineAt, &p.OfflineAt); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// PresenceRow is one online→offline episode.
type PresenceRow struct {
	ID        int64
	IP        string
	MAC       string
	OnlineAt  string
	OfflineAt string // empty = currently online
}

func nullStr(s string) any {
	if s == "" {
		return nil
	}
	return s
}

// NeighOutput shells out to `ip -4 neigh` (cached per call site).
func NeighOutput() (string, error) {
	out, err := exec.Command("ip", "-4", "neigh").CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("ip neigh: %v (%s)", err, strings.TrimSpace(string(out)))
	}
	return string(out), nil
}
