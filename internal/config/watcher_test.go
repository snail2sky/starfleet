package config

import (
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

func TestWatcherDetectsConfDirChanges(t *testing.T) {
	dir := t.TempDir()
	main := filepath.Join(dir, "starfleet.yml")
	if err := os.WriteFile(main, []byte("server:\n  port: 18080\n"), 0644); err != nil {
		t.Fatal(err)
	}

	var mu sync.Mutex
	var reloads int
	var lastCfg *Config
	w := NewWatcher(main, 50*time.Millisecond, func(newCfg *Config) error {
		mu.Lock()
		reloads++
		lastCfg = newCfg
		mu.Unlock()
		return nil
	})
	w.Start()
	defer w.Stop()

	// Baseline check happens on the first tick without firing a reload.
	time.Sleep(120 * time.Millisecond)
	mu.Lock()
	if reloads != 0 {
		t.Errorf("first tick must not fire a reload, got %d", reloads)
	}
	mu.Unlock()

	// A NEW fragment in conf.d must be detected even though the main
	// config file itself never changed.
	if err := os.MkdirAll(filepath.Join(dir, "conf.d"), 0755); err != nil {
		t.Fatal(err)
	}
	frag := "processes:\n  new-proc:\n    command: echo hi\n"
	if err := os.WriteFile(filepath.Join(dir, "conf.d", "new.yml"), []byte(frag), 0644); err != nil {
		t.Fatal(err)
	}

	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		mu.Lock()
		got := reloads
		cfg := lastCfg
		mu.Unlock()
		if got > 0 {
			if _, ok := cfg.Processes["new-proc"]; !ok {
				t.Error("reloaded config is missing the new conf.d process")
			}
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatal("watcher did not detect the new conf.d fragment in time")
}

func TestWatcherDetectsMainFileEdits(t *testing.T) {
	dir := t.TempDir()
	main := filepath.Join(dir, "starfleet.yml")
	if err := os.WriteFile(main, []byte("server:\n  port: 18081\n"), 0644); err != nil {
		t.Fatal(err)
	}

	var mu sync.Mutex
	var reloads int
	w := NewWatcher(main, 50*time.Millisecond, func(newCfg *Config) error {
		mu.Lock()
		reloads++
		mu.Unlock()
		return nil
	})
	w.Start()
	defer w.Stop()

	time.Sleep(120 * time.Millisecond)

	edited := "server:\n  port: 19090\n"
	if err := os.WriteFile(main, []byte(edited), 0644); err != nil {
		t.Fatal(err)
	}

	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		mu.Lock()
		got := reloads
		mu.Unlock()
		if got > 0 {
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatal("watcher did not detect the main config edit")
}
