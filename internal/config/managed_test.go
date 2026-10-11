package config

import (
	"os"
	"path/filepath"
	"testing"
)

/* ---------- conf_dir / includes / managed-config loading ---------- */

func TestLoadConfDir(t *testing.T) {
	dir := t.TempDir()
	// main config with an explicit relative conf_dir
	main := "server:\n  port: 9001\nconf_dir: managed\n"
	if err := os.WriteFile(filepath.Join(dir, "starfleet.yml"), []byte(main), 0644); err != nil {
		t.Fatal(err)
	}
	// managed fragment holding a process + schedule
	managed := filepath.Join(dir, "managed")
	if err := os.MkdirAll(managed, 0755); err != nil {
		t.Fatal(err)
	}
	frag := "processes:\n  web:\n    command: /usr/bin/web\nschedules:\n  nightly:\n    cron: \"0 3 * * *\"\n    command: /opt/nightly.sh\n"
	if err := os.WriteFile(filepath.Join(managed, "app.yml"), []byte(frag), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(filepath.Join(dir, "starfleet.yml"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Server.Port != 9001 {
		t.Errorf("expected port 9001, got %d", cfg.Server.Port)
	}
	if _, ok := cfg.Processes["web"]; !ok {
		t.Error("expected process 'web' from conf_dir")
	}
	if _, ok := cfg.Schedules["nightly"]; !ok {
		t.Error("expected schedule 'nightly' from conf_dir")
	}
}

func TestLoadConfDirMissing(t *testing.T) {
	dir := t.TempDir()
	main := "conf_dir: /nonexistent/starfleet-managed\n"
	if err := os.WriteFile(filepath.Join(dir, "starfleet.yml"), []byte(main), 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(filepath.Join(dir, "starfleet.yml")); err == nil {
		t.Fatal("expected error for missing explicit conf_dir")
	}
}

func TestLoadAutoConfD(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "starfleet.yml"), []byte("server:\n  port: 9002\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dir, "conf.d"), 0755); err != nil {
		t.Fatal(err)
	}
	frag := "processes:\n  auto-proc:\n    command: echo hi\n"
	if err := os.WriteFile(filepath.Join(dir, "conf.d", "p.yml"), []byte(frag), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(filepath.Join(dir, "starfleet.yml"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, ok := cfg.Processes["auto-proc"]; !ok {
		t.Error("expected auto-discovered conf.d process")
	}
}

func TestConfDirOverridesIncludes(t *testing.T) {
	dir := t.TempDir()
	// main: includes extra.yml (defines proc v1) + conf_dir managed (defines proc v2)
	main := "includes:\n  - extra.yml\nconf_dir: managed\n"
	if err := os.WriteFile(filepath.Join(dir, "starfleet.yml"), []byte(main), 0644); err != nil {
		t.Fatal(err)
	}
	extra := "processes:\n  dup:\n    command: from-includes\n"
	if err := os.WriteFile(filepath.Join(dir, "extra.yml"), []byte(extra), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dir, "managed"), 0755); err != nil {
		t.Fatal(err)
	}
	managed := "processes:\n  dup:\n    command: from-conf-dir\n"
	if err := os.WriteFile(filepath.Join(dir, "managed", "dup.yml"), []byte(managed), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(filepath.Join(dir, "starfleet.yml"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got := cfg.Processes["dup"].Command
	if got != "from-conf-dir" {
		t.Errorf("conf_dir should win over includes: got %q", got)
	}
}

/* ---------- full-schema coverage (keep in sync with docs/configuration.md) ---------- */

func TestFullConfigFields(t *testing.T) {
	yaml := `
includes:
  - extra.yml
conf_dir: managed
server:
  host: "192.168.1.50"
  port: 9443
  read_timeout: 45s
  write_timeout: 60s
  auth:
    enabled: true
    username: admin
    password: "$2a$10$hash"
    secret: "0123456789abcdef0123456789abcdef"
processes:
  app:
    command: "/opt/app --flag 'quoted arg'"
    directory: /opt/app
    user: appuser
    autostart: false
    autorestart: unexpected
    startsecs: 5
    startretries: 7
    stopsignal: SIGQUIT
    stopwaitsecs: 20
    exitcodes: [0, 2]
    environment:
      A: "1"
      B: "two"
    envfile: /opt/app/env
    stdout:
      path: /var/log/app.out
      maxbytes: "200M"
      backups: 7
    stderr:
      path: /var/log/app.err
      maxbytes: "80M"
      backups: 3
    healthcheck:
      type: http
      url: "http://127.0.0.1:8080/health"
      interval: 15s
      timeout: 3s
      retries: 4
    labels:
      group: backend
    depends_on: [redis]
    restart_delay: 10s
    backoff_limit: 600s
schedules:
  job-exec:
    cron: "*/5 * * * *"
    command: /opt/job.sh
    type: exec
    timeout: 90s
    enabled: true
    labels:
      team: ops
  job-http:
    cron: "@every 1h"
    type: http
    url: "http://127.0.0.1:9090/webhook"
    method: POST
    timeout: 10s
    enabled: false
logging:
  level: debug
  format: text
  output: /var/log/starfleet.log
notifications:
  webhook:
    enabled: true
    url: "https://hooks.example.com/x"
    events: [process_fatal, health_check_fail]
  email:
    enabled: true
    smtp_host: smtp.example.com
    smtp_port: 587
    from: sf@example.com
    to: ops@example.com
`
	cfg, err := Parse([]byte(yaml))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	// root level
	if len(cfg.Includes) != 1 || cfg.Includes[0] != "extra.yml" {
		t.Errorf("includes: got %#v", cfg.Includes)
	}
	if cfg.ConfDir != "managed" {
		t.Errorf("conf_dir: got %q", cfg.ConfDir)
	}

	// server
	s := cfg.Server
	if s.Host != "192.168.1.50" || s.Port != 9443 {
		t.Errorf("server: got %s:%d", s.Host, s.Port)
	}
	if s.ReadTimeout != 45*1e9 || s.WriteTimeout != 60*1e9 {
		t.Errorf("server timeouts: got %v / %v", s.ReadTimeout, s.WriteTimeout)
	}
	if !s.Auth.Enabled || s.Auth.Username != "admin" || s.Auth.Password != "$2a$10$hash" || s.Auth.Secret == "" {
		t.Errorf("auth: got %#v", s.Auth)
	}

	// process (every field)
	p := cfg.Processes["app"]
	if p.Command != "/opt/app --flag 'quoted arg'" {
		t.Errorf("command: got %q", p.Command)
	}
	if p.Directory != "/opt/app" || p.User != "appuser" {
		t.Errorf("dir/user: got %q / %q", p.Directory, p.User)
	}
	if p.Autostart || p.Autorestart != "unexpected" {
		t.Errorf("autostart/autorestart: got %v / %q", p.Autostart, p.Autorestart)
	}
	if p.StartSecs != 5 || p.StartRetries != 7 {
		t.Errorf("startsecs/startretries: got %d / %d", p.StartSecs, p.StartRetries)
	}
	if p.StopSignal != "SIGQUIT" || p.StopWaitSecs != 20 {
		t.Errorf("stopsignal/stopwaitsecs: got %q / %d", p.StopSignal, p.StopWaitSecs)
	}
	if len(p.ExitCodes) != 2 || p.ExitCodes[1] != 2 {
		t.Errorf("exitcodes: got %#v", p.ExitCodes)
	}
	if p.Environment["A"] != "1" || p.Environment["B"] != "two" {
		t.Errorf("environment: got %#v", p.Environment)
	}
	if p.EnvFile != "/opt/app/env" {
		t.Errorf("envfile: got %q", p.EnvFile)
	}
	if p.StdoutLog.Path != "/var/log/app.out" || p.StdoutLog.MaxBytes != "200M" || p.StdoutLog.Backups != 7 {
		t.Errorf("stdout log: got %#v", p.StdoutLog)
	}
	if p.StderrLog.Path != "/var/log/app.err" || p.StderrLog.Backups != 3 {
		t.Errorf("stderr log: got %#v", p.StderrLog)
	}
	hc := p.HealthCheck
	if hc == nil || hc.Type != "http" || hc.URL != "http://127.0.0.1:8080/health" ||
		hc.Interval != 15*1e9 || hc.Timeout != 3*1e9 || hc.Retries != 4 {
		t.Errorf("healthcheck: got %#v", hc)
	}
	if p.Labels["group"] != "backend" {
		t.Errorf("labels: got %#v", p.Labels)
	}
	if len(p.DependsOn) != 1 || p.DependsOn[0] != "redis" {
		t.Errorf("depends_on: got %#v", p.DependsOn)
	}
	if p.RestartDelay != 10*1e9 || p.BackoffLimit != 600*1e9 {
		t.Errorf("restart_delay/backoff_limit: got %v / %v", p.RestartDelay, p.BackoffLimit)
	}

	// schedules (exec + http)
	se := cfg.Schedules["job-exec"]
	if se.Cron != "*/5 * * * *" || se.Command != "/opt/job.sh" || se.Type != "exec" ||
		se.Timeout != 90*1e9 || !se.Enabled || se.Labels["team"] != "ops" {
		t.Errorf("job-exec: got %#v", se)
	}
	sh := cfg.Schedules["job-http"]
	if sh.Cron != "@every 1h" || sh.Type != "http" || sh.URL != "http://127.0.0.1:9090/webhook" ||
		sh.Method != "POST" || sh.Timeout != 10*1e9 || sh.Enabled {
		t.Errorf("job-http: got %#v", sh)
	}

	// logging + notifications
	if cfg.Logging.Level != "debug" || cfg.Logging.Format != "text" || cfg.Logging.Output != "/var/log/starfleet.log" {
		t.Errorf("logging: got %#v", cfg.Logging)
	}
	n := cfg.Notifications
	if !n.Webhook.Enabled || n.Webhook.URL == "" || len(n.Webhook.Events) != 2 {
		t.Errorf("webhook: got %#v", n.Webhook)
	}
	if !n.Email.Enabled || n.Email.SMTPHost != "smtp.example.com" || n.Email.SMTPPort != 587 ||
		n.Email.From != "sf@example.com" || n.Email.To != "ops@example.com" {
		t.Errorf("email: got %#v", n.Email)
	}

	// Save -> Parse must preserve the core data.
	tmp := filepath.Join(t.TempDir(), "out.yml")
	if err := Save(cfg, tmp); err != nil {
		t.Fatalf("save: %v", err)
	}
	raw, err := os.ReadFile(tmp)
	if err != nil {
		t.Fatal(err)
	}
	reloaded, err := Parse(raw)
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	if reloaded.Processes["app"].Command != cfg.Processes["app"].Command ||
		reloaded.Schedules["job-http"].URL != cfg.Schedules["job-http"].URL ||
		reloaded.Server.Port != cfg.Server.Port {
		t.Error("save/load round trip changed data")
	}
}

func TestLoadedFilesTracking(t *testing.T) {
	dir := t.TempDir()
	main := "includes:\n  - extra.yml\nconf_dir: managed\n"
	if err := os.WriteFile(filepath.Join(dir, "starfleet.yml"), []byte(main), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "extra.yml"), []byte("processes:\n  a:\n    command: echo a\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dir, "managed"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "managed", "b.yml"), []byte("processes:\n  b:\n    command: echo b\n"), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(filepath.Join(dir, "starfleet.yml"))
	if err != nil {
		t.Fatalf("load: %v", err)
	}

	// main file first, then includes, then conf_dir fragments.
	if len(cfg.LoadedFiles) != 3 {
		t.Fatalf("expected 3 loaded files, got %#v", cfg.LoadedFiles)
	}
	if filepath.Base(cfg.LoadedFiles[0]) != "starfleet.yml" {
		t.Errorf("first loaded file should be the main config, got %s", cfg.LoadedFiles[0])
	}
	if filepath.Base(cfg.LoadedFiles[1]) != "extra.yml" {
		t.Errorf("second loaded file should be the include, got %s", cfg.LoadedFiles[1])
	}
	if filepath.Base(cfg.LoadedFiles[2]) != "b.yml" {
		t.Errorf("third loaded file should be the conf_dir fragment, got %s", cfg.LoadedFiles[2])
	}

	// Resolved conf dir must point at the absolute managed directory.
	wantResolved := filepath.Join(dir, "managed")
	if cfg.ConfDirResolved != wantResolved {
		t.Errorf("ConfDirResolved: want %s, got %s", wantResolved, cfg.ConfDirResolved)
	}
}
