// ipwatch records LAN device presence (online/offline intervals plus raw
// sightings) into SQLite for later analysis. Designed to run as a
// starfleet-managed process: JSON logs to stdout, graceful SIGTERM.
//
//	ipwatch -subnet 192.168.1.0/24 -db /data0/apps/ipwatch/ipwatch.db
//	ipwatch -db ... -summary      # list devices instead of scanning
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/snail2sky/starfleet/internal/ipwatch"
)

func main() {
	var (
		subnet       = flag.String("subnet", "192.168.1.0/24", "IPv4 subnet to sweep (/24 or smaller)")
		interval     = flag.Duration("interval", 15*time.Second, "time between sweeps")
		offlineAfter = flag.Int("offline-after", 3, "missed rounds before a device is marked offline")
		dbPath       = flag.String("db", "ipwatch.db", "SQLite database path")
		workers      = flag.Int("workers", 32, "parallel pings per sweep")
		retain       = flag.Duration("retain", 30*24*time.Hour, "raw sightings retention (0 = keep forever)")
		summary      = flag.Bool("summary", false, "print device inventory and exit")
		once         = flag.Bool("once", false, "run a single sweep and exit")
		webAddr      = flag.String("web", ":8095", "dashboard listen address (empty disables the web UI)")
	)
	flag.Parse()

	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo})))

	db, err := ipwatch.OpenDB(*dbPath)
	if err != nil {
		fatal("open db", err)
	}
	defer db.Close()
	tracker, err := ipwatch.NewTracker(db, *offlineAfter)
	if err != nil {
		fatal("init tracker", err)
	}

	if *summary {
		printSummary(tracker)
		return
	}

	targets, err := ipwatch.ExpandSubnet(*subnet)
	if err != nil {
		fatal("subnet", err)
	}
	slog.Info("ipwatch starting",
		"subnet", *subnet, "hosts", len(targets),
		"interval", interval.String(), "db", *dbPath)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	// One pruning pass at startup, then once per day.
	prune := func() {
		n, err := tracker.PruneSightings(ctx, *retain)
		if err != nil {
			slog.Error("prune sightings", "error", err)
		} else if n > 0 {
			slog.Info("pruned old sightings", "rows", n, "retain", retain.String())
		}
	}
	prune()

	sweep := func() {
		start := time.Now()
		neighOut, err := ipwatch.NeighOutput()
		var macs map[string]string
		var neighActive map[string]bool
		if err != nil {
			slog.Warn("neigh table unavailable", "error", err)
			macs, neighActive = map[string]string{}, map[string]bool{}
		} else {
			macs, neighActive = ipwatch.ParseNeigh(neighOut)
		}
		alive := ipwatch.PingSweep(ctx, targets, *workers)
		if err := tracker.Round(ctx, time.Now(), alive, macs, neighActive); err != nil {
			slog.Error("round failed", "error", err)
			return
		}
		up := 0
		for _, r := range alive {
			if r.Alive {
				up++
			}
		}
		slog.Info("sweep done",
			"ping_alive", up, "neigh_active", len(neighActive),
			"took", time.Since(start).Round(time.Millisecond).String())
	}

	sweep()
	if *once {
		return
	}

	// Optional read-only dashboard (embedded single-page UI + JSON API).
	if *webAddr != "" {
		srv := &http.Server{Addr: *webAddr, Handler: ipwatch.NewWebHandler(db)}
		go func() {
			slog.Info("dashboard listening", "addr", *webAddr)
			if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
				slog.Error("dashboard server failed", "error", err)
			}
		}()
		go func() {
			<-ctx.Done()
			shCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			defer cancel()
			_ = srv.Shutdown(shCtx)
		}()
	}

	ticker := time.NewTicker(*interval)
	defer ticker.Stop()
	day := time.NewTicker(24 * time.Hour)
	defer day.Stop()
	for {
		select {
		case <-ctx.Done():
			slog.Info("ipwatch stopping")
			return
		case <-ticker.C:
			sweep()
		case <-day.C:
			prune()
		}
	}
}

func printSummary(tr *ipwatch.Tracker) {
	devices, err := tr.Summary()
	if err != nil {
		fatal("summary", err)
	}
	fmt.Printf("%-16s %-18s %-9s %s\n", "IP", "MAC", "STATE", "LAST SEEN")
	for _, d := range devices {
		state := "offline"
		if d.Online {
			state = "ONLINE"
		}
		fmt.Printf("%-16s %-18s %-9s %s\n", d.IP, d.MAC, state, d.LastSeen)
	}
	fmt.Printf("\n%d device(s). Episodes today:\n", len(devices))
	rows, err := tr.TodayPresence()
	if err != nil {
		fatal("presence", err)
	}
	for _, p := range rows {
		end := p.OfflineAt
		if end == "" {
			end = "(online)"
		}
		fmt.Printf("  %-16s %-18s %s → %s\n", p.IP, p.MAC, p.OnlineAt, end)
	}
}

func fatal(msg string, err error) {
	slog.Error(msg, "error", err)
	fmt.Fprintln(os.Stderr, "ipwatch:", msg+":", err)
	os.Exit(1)
}
