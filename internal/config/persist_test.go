package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSetProcessAutostartPreservesComments(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "web.yml")
	original := `# managed by hand — do not delete
processes:
  web:
    command: "nginx -g 'daemon off;'"   # the web server
    autostart: true
    # keep this note
    labels:
      group: web
  other:
    command: "echo hi"
`
	if err := os.WriteFile(path, []byte(original), 0644); err != nil {
		t.Fatal(err)
	}

	if err := SetProcessAutostart(path, "web", false); err != nil {
		t.Fatalf("set: %v", err)
	}

	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	s := string(got)

	if !strings.Contains(s, "autostart: false") {
		t.Errorf("autostart not flipped to false:\n%s", s)
	}
	if strings.Contains(s, "autostart: true") {
		t.Errorf("stale autostart:true remains:\n%s", s)
	}
	// Other content and comments must survive the round-trip.
	for _, want := range []string{
		"# managed by hand — do not delete",
		"command: \"nginx -g 'daemon off;'\"",
		"# the web server",
		"# keep this note",
		"group: web",
		"other:",
	} {
		if !strings.Contains(s, want) {
			t.Errorf("lost content %q:\n%s", want, s)
		}
	}

	// Other processes must be untouched.
	if strings.Contains(s, "autostart: false\n  other") {
		t.Error("autostart leaked into a different process")
	}

	// Re-enable round-trips too.
	if err := SetProcessAutostart(path, "web", true); err != nil {
		t.Fatalf("set back: %v", err)
	}
	s2, _ := os.ReadFile(path)
	if !strings.Contains(string(s2), "autostart: true") {
		t.Errorf("re-enable failed:\n%s", s2)
	}
}

func TestSetProcessAutostartErrors(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "x.yml")
	if err := os.WriteFile(path, []byte("processes:\n  a:\n    command: echo a\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := SetProcessAutostart(path, "missing", true); err == nil {
		t.Error("expected error for unknown process")
	}

	noProcs := filepath.Join(dir, "y.yml")
	if err := os.WriteFile(noProcs, []byte("server:\n  port: 1\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := SetProcessAutostart(noProcs, "a", true); err == nil {
		t.Error("expected error when file has no processes section")
	}
}

func TestProcessSourcesTracking(t *testing.T) {
	dir := t.TempDir()
	main := "processes:\n  shared:\n    command: from-main\nincludes:\n  - extra.yml\n"
	if err := os.WriteFile(filepath.Join(dir, "starfleet.yml"), []byte(main), 0644); err != nil {
		t.Fatal(err)
	}
	extraPath := filepath.Join(dir, "extra.yml")
	if err := os.WriteFile(extraPath, []byte("processes:\n  shared:\n    command: from-extra\n  extra-only:\n    command: echo e\n"), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(filepath.Join(dir, "starfleet.yml"))
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	// "shared" is defined twice; the later file must own it.
	if got := cfg.ProcessSources["shared"]; got != extraPath {
		t.Errorf("shared source = %q, want %q", got, extraPath)
	}
	if got := cfg.ProcessSources["extra-only"]; got != extraPath {
		t.Errorf("extra-only source = %q, want %q", got, extraPath)
	}
	// Enabling/disabling "shared" must edit extra.yml (the effective one).
	if err := SetProcessAutostart(extraPath, "shared", false); err != nil {
		t.Fatalf("persist: %v", err)
	}
	reloaded, err := Load(filepath.Join(dir, "starfleet.yml"))
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	if reloaded.Processes["shared"].Autostart {
		t.Error("shared autostart=false should win over the main file")
	}
}

func TestWebConfigDefaultEnabled(t *testing.T) {
	cfg := DefaultConfig()
	if !cfg.Server.Web.Enabled {
		t.Error("web UI must be enabled by default")
	}
	// An explicit false must survive loading.
	parsed, err := Parse([]byte("server:\n  web:\n    enabled: false\n"))
	if err != nil {
		t.Fatal(err)
	}
	if parsed.Server.Web.Enabled {
		t.Error("web.enabled=false was not honored")
	}
}

func TestSetScheduleEnabled(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "jobs.yml")
	original := `schedules:
  report:
    cron: "*/5 * * * *"
    command: /home/orangepi/bin/report.sh   # every five minutes
    enabled: true
`
	if err := os.WriteFile(path, []byte(original), 0644); err != nil {
		t.Fatal(err)
	}
	if err := SetScheduleEnabled(path, "report", false); err != nil {
		t.Fatalf("disable: %v", err)
	}
	got, _ := os.ReadFile(path)
	s := string(got)
	if !strings.Contains(s, "enabled: false") {
		t.Errorf("enabled not flipped:\n%s", s)
	}
	if !strings.Contains(s, "# every five minutes") {
		t.Errorf("comment lost:\n%s", s)
	}
	if !strings.Contains(s, "cron: \"*/5 * * * *\"") {
		t.Errorf("cron lost:\n%s", s)
	}
	// Re-enable.
	if err := SetScheduleEnabled(path, "report", true); err != nil {
		t.Fatalf("enable: %v", err)
	}
	s2, _ := os.ReadFile(path)
	if !strings.Contains(string(s2), "enabled: true") {
		t.Errorf("re-enable failed:\n%s", s2)
	}
}
