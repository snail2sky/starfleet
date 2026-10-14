package ipwatch

import (
	"context"
	"path/filepath"
	"testing"
	"time"
)

func TestParseNeigh(t *testing.T) {
	out := `192.168.1.1 dev wlan0 lladdr aa:bb:cc:dd:ee:01 REACHABLE
192.168.1.5 dev wlan0 lladdr aa:bb:cc:dd:ee:02 STALE
192.168.1.9 dev wlan0 INCOMPLETE
192.168.1.20 dev wlan0 lladdr aa:bb:cc:dd:ee:03 DELAY
`
	macs, active := ParseNeigh(out)
	if macs["192.168.1.1"] != "aa:bb:cc:dd:ee:01" {
		t.Errorf("mac for .1 = %q", macs["192.168.1.1"])
	}
	if macs["192.168.1.5"] != "aa:bb:cc:dd:ee:02" {
		t.Errorf("stale entries must still contribute MACs, got %q", macs["192.168.1.5"])
	}
	if active["192.168.1.1"] != true || active["192.168.1.20"] != true {
		t.Error("REACHABLE/DELAY must be active")
	}
	if active["192.168.1.5"] {
		t.Error("STALE must NOT count as online evidence")
	}
	if active["192.168.1.9"] {
		t.Error("INCOMPLETE must NOT count as online evidence")
	}
}

func TestExpandSubnet(t *testing.T) {
	hosts, err := ExpandSubnet("10.0.7.0/24")
	if err != nil {
		t.Fatal(err)
	}
	if len(hosts) != 254 || hosts[0] != "10.0.7.1" || hosts[253] != "10.0.7.254" {
		t.Fatalf("hosts[0]=%q len=%d", hosts[0], len(hosts))
	}
	if _, err := ExpandSubnet("10.0.0.0/8"); err == nil {
		t.Error("expected error for oversized subnet")
	}
	if _, err := ExpandSubnet("banana"); err == nil {
		t.Error("expected error for garbage input")
	}
}

func TestTrackerTransitions(t *testing.T) {
	db, err := OpenDB(filepath.Join(t.TempDir(), "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	tr, err := NewTracker(db, 2) // offline after 2 misses
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	mac := map[string]string{"192.168.1.10": "aa:bb:cc:dd:ee:ff"}

	round := func(offset time.Duration, results ...PingResult) {
		if err := tr.Round(ctx, time.Now().Add(offset), results, mac, nil); err != nil {
			t.Fatal(err)
		}
	}

	// Round 1: online.
	round(0, PingResult{IP: "192.168.1.10", Alive: true, RTT: 3 * time.Millisecond})
	// Round 2: still online (no new episode).
	round(time.Second, PingResult{IP: "192.168.1.10", Alive: true})
	// Round 3-4: two misses -> offline.
	round(2*time.Second)
	round(3 * time.Second)

	rows, err := tr.TodayPresence()
	if err != nil {
		t.Fatal(err)
	}
	closed := 0
	for _, p := range rows {
		if p.IP != "192.168.1.10" {
			continue
		}
		if p.OfflineAt != "" {
			closed++
			if p.MAC != "aa:bb:cc:dd:ee:ff" {
				t.Errorf("presence mac = %q", p.MAC)
			}
		}
	}
	if closed != 1 {
		t.Fatalf("expected exactly 1 closed episode, got %d (rows=%+v)", closed, rows)
	}

	// Round 5: back online -> a NEW episode opens.
	round(4*time.Second, PingResult{IP: "192.168.1.10", Alive: true})
	devices, err := tr.Summary()
	if err != nil {
		t.Fatal(err)
	}
	online := 0
	for _, d := range devices {
		if d.IP == "192.168.1.10" && d.Online {
			online++
		}
	}
	if online != 1 {
		t.Fatalf("expected device online again, got %d", online)
	}

	// Sightings were persisted for every alive round.
	var n int
	if err := db.QueryRow(`SELECT COUNT(*) FROM sightings WHERE ip='192.168.1.10'`).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 3 {
		t.Errorf("sightings = %d, want 3 alive rounds", n)
	}
}

func TestTrackerRestartAdoptsOpenPresence(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "t.db")
	db, err := OpenDB(path)
	if err != nil {
		t.Fatal(err)
	}
	tr, _ := NewTracker(db, 3)
	err = tr.Round(context.Background(), time.Now(),
		[]PingResult{{IP: "192.168.1.7", Alive: true}}, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	db.Close()

	// Reopen: the still-open episode must be adopted, not duplicated.
	db2, err := OpenDB(path)
	if err != nil {
		t.Fatal(err)
	}
	defer db2.Close()
	tr2, err := NewTracker(db2, 3)
	if err != nil {
		t.Fatal(err)
	}
	if err := tr2.Round(context.Background(), time.Now(),
		[]PingResult{{IP: "192.168.1.7", Alive: true}}, nil, nil); err != nil {
		t.Fatal(err)
	}
	var open int
	if err := db2.QueryRow(`SELECT COUNT(*) FROM presence WHERE ip='192.168.1.7' AND offline_at IS NULL`).Scan(&open); err != nil {
		t.Fatal(err)
	}
	if open != 1 {
		t.Fatalf("open episodes = %d, want 1 (restart must not duplicate)", open)
	}
}
