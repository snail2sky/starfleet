package engine

import (
	"testing"

	"github.com/snail2sky/starfleet/internal/config"
)

func TestListIsSortedByName(t *testing.T) {
	m := NewManager()
	cfg := &config.Config{Processes: map[string]config.ProcessConfig{
		"zeta":  {Command: "echo z"},
		"alpha": {Command: "echo a"},
		"mid":   {Command: "echo m"},
	}}
	m.LoadFromConfig(cfg)

	// Call repeatedly: map iteration is randomized, so every call must
	// independently return the same sorted order.
	for i := 0; i < 10; i++ {
		list := m.List()
		if len(list) != 3 {
			t.Fatalf("expected 3 processes, got %d", len(list))
		}
		if list[0].Name != "alpha" || list[1].Name != "mid" || list[2].Name != "zeta" {
			t.Fatalf("iteration %d not sorted: %s, %s, %s", i, list[0].Name, list[1].Name, list[2].Name)
		}
	}
}

func TestSetAutostart(t *testing.T) {
	m := NewManager()
	cfg := &config.Config{Processes: map[string]config.ProcessConfig{
		"svc": {Command: "echo svc", Autostart: true},
	}}
	m.LoadFromConfig(cfg)

	if err := m.SetAutostart("svc", false); err != nil {
		t.Fatalf("disable: %v", err)
	}
	got, _ := m.GetConfig("svc")
	if got.Autostart {
		t.Error("autostart should be false after disable")
	}
	if err := m.SetAutostart("svc", true); err != nil {
		t.Fatalf("enable: %v", err)
	}
	got, _ = m.GetConfig("svc")
	if !got.Autostart {
		t.Error("autostart should be true after enable")
	}
	if err := m.SetAutostart("nope", true); err == nil {
		t.Error("expected error for unknown process")
	}
}
