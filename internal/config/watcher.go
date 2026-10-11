package config

import (
	"fmt"
	"log/slog"
	"os"
	"strings"
	"sync"
	"time"
)

// ReloadFunc is called when config changes are detected.
type ReloadFunc func(newCfg *Config) error

// Watcher polls the configuration every interval and triggers a reload
// when ANY loaded file changes — the main config, an include, or a
// conf_dir fragment (including files being added or removed).
type Watcher struct {
	path     string
	interval time.Duration
	callback ReloadFunc
	stopCh   chan struct{}

	mu      sync.Mutex
	lastFP  string // fingerprint of all loaded files at last check
	started bool
}

// NewWatcher creates a config watcher for path (the main config file).
// The callback receives a freshly loaded Config on change.
func NewWatcher(path string, interval time.Duration, callback ReloadFunc) *Watcher {
	return &Watcher{
		path:     path,
		interval: interval,
		callback: callback,
		stopCh:   make(chan struct{}),
	}
}

// Start begins watching. The first successful load only records the
// baseline (startup already loaded the config itself).
func (w *Watcher) Start() {
	go func() {
		ticker := time.NewTicker(w.interval)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				w.check()
			case <-w.stopCh:
				return
			}
		}
	}()
	slog.Info("config watcher started", "path", w.path, "interval", w.interval)
}

// Stop stops the watcher.
func (w *Watcher) Stop() {
	close(w.stopCh)
}

func (w *Watcher) check() {
	cfg, err := Load(w.path)
	if err != nil {
		slog.Error("config watcher: reload failed", "error", err)
		return
	}
	fp := fingerprint(cfg)

	w.mu.Lock()
	defer w.mu.Unlock()
	if !w.started {
		// Baseline only — the caller already loaded this config at startup.
		w.started = true
		w.lastFP = fp
		return
	}
	if fp == w.lastFP {
		return
	}
	w.lastFP = fp
	slog.Info("config change detected, reloading",
		"files", len(cfg.LoadedFiles), "processes", len(cfg.Processes))
	if err := w.callback(cfg); err != nil {
		slog.Error("config reload callback failed", "error", err)
	}
}

// fingerprint summarizes every loaded file's identity and mtime.
// Adding/removing files (conf.d) or editing content both change it.
func fingerprint(cfg *Config) string {
	var b strings.Builder
	for _, p := range cfg.LoadedFiles {
		if st, err := os.Stat(p); err == nil {
			fmt.Fprintf(&b, "%s:%d;", p, st.ModTime().UnixNano())
		} else {
			fmt.Fprintf(&b, "%s:gone;", p)
		}
	}
	return b.String()
}
