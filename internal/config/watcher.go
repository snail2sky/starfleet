package config

import (
	"log/slog"
	"os"
	"sync"
	"time"
)

// ReloadFunc is called when config changes are detected.
type ReloadFunc func(newCfg *Config) error

// Watcher watches a config file for changes and triggers reloads.
type Watcher struct {
	path     string
	interval time.Duration
	lastMod  time.Time
	callback ReloadFunc
	stopCh   chan struct{}
	mu       sync.Mutex
}

// NewWatcher creates a new config watcher.
func NewWatcher(path string, interval time.Duration, callback ReloadFunc) *Watcher {
	return &Watcher{
		path:     path,
		interval: interval,
		callback: callback,
		stopCh:   make(chan struct{}),
	}
}

// Start begins watching the config file. It polls at the configured interval.
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
	info, err := os.Stat(w.path)
	if err != nil {
		return
	}
	mod := info.ModTime()
	w.mu.Lock()
	defer w.mu.Unlock()
	if mod.After(w.lastMod) {
		w.lastMod = mod
		newCfg, err := Load(w.path)
		if err != nil {
			slog.Error("config reload failed", "error", err)
			return
		}
		slog.Info("config file changed, reloading")
		if err := w.callback(newCfg); err != nil {
			slog.Error("config reload callback failed", "error", err)
		}
	}
}
