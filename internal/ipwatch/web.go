package ipwatch

import (
	"database/sql"
	"embed"
	"encoding/json"
	"fmt"
	"io/fs"
	"net/http"
	"sort"
	"time"
)

//go:embed dashboard.html
var dashFS embed.FS

// NewWebHandler serves the dashboard and its JSON API (read-only).
func NewWebHandler(db *sql.DB) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/state", func(w http.ResponseWriter, r *http.Request) {
		serveJSON(w, dbState(db))
	})
	mux.HandleFunc("/api/timeline", func(w http.ResponseWriter, r *http.Request) {
		hours := 24
		if h := r.URL.Query().Get("hours"); h != "" {
			_, _ = fmt.Sscanf(h, "%d", &hours)
		}
		if hours < 1 || hours > 24*30 {
			hours = 24
		}
		serveJSON(w, dbTimeline(db, hours))
	})
	mux.HandleFunc("/api/events", func(w http.ResponseWriter, r *http.Request) {
		serveJSON(w, dbEvents(db, 50))
	})
	page, _ := fs.ReadFile(dashFS, "dashboard.html")
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Cache-Control", "no-store")
		_, _ = w.Write(page)
	})
	return mux
}

/* ---------- API shapes ---------- */

type webDevice struct {
	IP        string `json:"ip"`
	MAC       string `json:"mac"`
	Online    bool   `json:"online"`
	Since     string `json:"since,omitempty"`  // online_at when online
	FirstSeen string `json:"first_seen"`
	LastSeen  string `json:"last_seen"`
	TodayRuns int    `json:"episodes_today"`
}

type webState struct {
	Now         string      `json:"now"`
	Online      int         `json:"online"`
	Total       int         `json:"total"`
	LastSweep   string      `json:"last_sweep"`
	Devices     []webDevice `json:"devices"`
}

type webSeg struct {
	StartMs int64 `json:"start"`          // epoch ms
	EndMs   int64 `json:"end"`            // 0 = still online
}

type webTimeline struct {
	FromMs  int64        `json:"from"`
	ToMs    int64        `json:"to"`
	Rows    []webTlRow   `json:"rows"`
}

type webTlRow struct {
	IP     string   `json:"ip"`
	MAC    string   `json:"mac"`
	Online bool     `json:"online"`
	Segs   []webSeg `json:"segs"`
}

type webEvent struct {
	At   string `json:"at"`
	Type string `json:"type"` // online | offline
	IP   string `json:"ip"`
	MAC  string `json:"mac"`
}

/* ---------- queries ---------- */

func dbState(db *sql.DB) *webState {
	st := &webState{Now: time.Now().Format(time.RFC3339), Devices: []webDevice{}}
	rows, err := db.Query(`
		SELECT d.ip, COALESCE(d.mac,''), d.first_seen, d.last_seen,
		       (SELECT p.online_at FROM presence p
		         WHERE p.ip = d.ip AND p.offline_at IS NULL LIMIT 1),
		       (SELECT COUNT(*) FROM presence p
		         WHERE p.ip = d.ip AND p.offline_at IS NULL
		           AND substr(p.online_at,1,10) = ?)
		FROM devices d ORDER BY d.last_seen DESC`, time.Now().Format("2006-01-02"))
	if err != nil {
		return st
	}
	defer rows.Close()
	for rows.Next() {
		var d webDevice
		var since sql.NullString
		if err := rows.Scan(&d.IP, &d.MAC, &d.FirstSeen, &d.LastSeen, &since, &d.TodayRuns); err != nil {
			continue
		}
		d.Online = since.Valid
		if since.Valid {
			d.Since = since.String
			st.Online++
		}
		st.Devices = append(st.Devices, d)
	}
	st.Total = len(st.Devices)
	_ = db.QueryRow(`SELECT COALESCE(MAX(last_seen),'') FROM devices`).Scan(&st.LastSweep)
	return st
}

func dbTimeline(db *sql.DB, hours int) *webTimeline {
	now := time.Now()
	from := now.Add(-time.Duration(hours) * time.Hour)
	tl := &webTimeline{
		FromMs: from.UnixMilli(),
		ToMs:   now.UnixMilli(),
		Rows:   []webTlRow{},
	}
	// Presence overlapping the window; clamp to window bounds.
	rows, err := db.Query(`
		SELECT ip, COALESCE(mac,''), online_at, COALESCE(offline_at,'')
		FROM presence
		WHERE online_at <= ? AND (offline_at IS NULL OR offline_at >= ?)
		ORDER BY ip, online_at`,
		now.UTC().Format(time.RFC3339), from.UTC().Format(time.RFC3339))
	if err != nil {
		return tl
	}
	defer rows.Close()

	byIP := map[string]*webTlRow{}
	var order []string
	for rows.Next() {
		var ip, mac, on, off string
		if err := rows.Scan(&ip, &mac, &on, &off); err != nil {
			continue
		}
		row := byIP[ip]
		if row == nil {
			row = &webTlRow{IP: ip, MAC: mac, Segs: []webSeg{}}
			byIP[ip] = row
			order = append(order, ip)
		}
		if mac != "" && row.MAC == "" {
			row.MAC = mac
		}
		start := parseRFC3339(on)
		end := int64(0)
		if off != "" {
			end = parseRFC3339(off)
		} else {
			row.Online = true
		}
		// Clamp to window.
		if start < tl.FromMs {
			start = tl.FromMs
		}
		if end != 0 && end > tl.ToMs {
			end = tl.ToMs
		}
		if end != 0 && end <= start {
			continue // fully outside / zero-length
		}
		row.Segs = append(row.Segs, webSeg{StartMs: start, EndMs: end})
	}
	sort.Strings(order)
	for _, ip := range order {
		tl.Rows = append(tl.Rows, *byIP[ip])
	}
	return tl
}

func dbEvents(db *sql.DB, limit int) []webEvent {
	now := time.Now()
	from := now.Add(-24 * time.Hour).UTC().Format(time.RFC3339)
	rows, err := db.Query(`
		SELECT at, type, ip, mac FROM (
		  SELECT online_at AS at, 'online' AS type, ip, COALESCE(mac,'') AS mac FROM presence WHERE online_at >= ?
		  UNION ALL
		  SELECT offline_at AS at, 'offline' AS type, ip, COALESCE(mac,'') AS mac FROM presence WHERE offline_at IS NOT NULL AND offline_at >= ?
		) ORDER BY at DESC LIMIT ?`, from, from, limit)
	if err != nil {
		return []webEvent{}
	}
	defer rows.Close()
	out := []webEvent{}
	for rows.Next() {
		var e webEvent
		if err := rows.Scan(&e.At, &e.Type, &e.IP, &e.MAC); err == nil {
			out = append(out, e)
		}
	}
	return out
}

func parseRFC3339(s string) int64 {
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		return 0
	}
	return t.UnixMilli()
}

func serveJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	_ = json.NewEncoder(w).Encode(v)
}
