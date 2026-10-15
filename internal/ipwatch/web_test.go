package ipwatch

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"
)

func TestWebAPI(t *testing.T) {
	db, err := OpenDB(filepath.Join(t.TempDir(), "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	// Seed: one device online since yesterday (still open), one offline.
	y := time.Now().Add(-24 * time.Hour).UTC().Format(time.RFC3339)
	now := time.Now().UTC().Format(time.RFC3339)
	older := time.Now().Add(-2 * time.Hour).UTC().Format(time.RFC3339)
	if _, err := db.Exec(`INSERT INTO devices (ip, mac, first_seen, last_seen)
		VALUES ('192.168.1.5','aa:bb:cc:dd:ee:01', ?, ?), ('192.168.1.6','', ?, ?)`,
		y, now, y, older); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO presence (ip, mac, online_at) VALUES ('192.168.1.5','aa:bb:cc:dd:ee:01', ?)`, y); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO presence (ip, mac, online_at, offline_at) VALUES ('192.168.1.6','', ?, ?)`,
		older, now); err != nil {
		t.Fatal(err)
	}

	h := NewWebHandler(db)

	// Dashboard page is served.
	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/", nil))
	if w.Code != http.StatusOK || w.Body.Len() < 500 {
		t.Fatalf("dashboard page: code=%d len=%d", w.Code, w.Body.Len())
	}

	// /api/state
	w = httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/state", nil))
	var st webState
	if err := json.Unmarshal(w.Body.Bytes(), &st); err != nil {
		t.Fatalf("state json: %v", err)
	}
	if st.Total != 2 || st.Online != 1 {
		t.Errorf("state total=%d online=%d, want 2/1", st.Total, st.Online)
	}

	// /api/timeline — both episodes overlap 24h window.
	w = httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/timeline?hours=24", nil))
	var tl webTimeline
	if err := json.Unmarshal(w.Body.Bytes(), &tl); err != nil {
		t.Fatalf("timeline json: %v", err)
	}
	if len(tl.Rows) != 2 {
		t.Fatalf("timeline rows=%d, want 2", len(tl.Rows))
	}
	for _, row := range tl.Rows {
		if len(row.Segs) == 0 {
			t.Errorf("%s has no segments", row.IP)
		}
	}
	// The still-open episode must have end=0 (live).
	foundLive := false
	for _, row := range tl.Rows {
		for _, s := range row.Segs {
			if row.IP == "192.168.1.5" && s.EndMs == 0 {
				foundLive = true
			}
		}
	}
	if !foundLive {
		t.Error("expected live segment (end=0) for the open presence row")
	}

	// /api/events — one online (yesterday, outside 24h) + one offline (now).
	w = httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/events", nil))
	var evs []webEvent
	if err := json.Unmarshal(w.Body.Bytes(), &evs); err != nil {
		t.Fatalf("events json: %v", err)
	}
	offlines := 0
	for _, e := range evs {
		if e.Type == "offline" {
			offlines++
		}
	}
	if offlines != 1 {
		t.Errorf("offline events=%d, want 1 (yesterday's online_at is outside the window)", offlines)
	}
}
