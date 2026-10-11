package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseValidConfig(t *testing.T) {
	yaml := `
server:
  host: "127.0.0.1"
  port: 9090
processes:
  nginx:
    command: "nginx -g daemon off;"
    directory: "/etc/nginx"
    autostart: true
    autorestart: "true"
    startsecs: 2
    startretries: 5
    stopsignal: SIGTERM
    stopwaitsecs: 10
    environment:
      ENV: production
    stdout:
      path: /var/log/nginx/stdout.log
      maxbytes: "100M"
      backups: 5
schedules:
  cleanup:
    cron: "0 3 * * *"
    command: "find /tmp -mtime +7 -delete"
    timeout: 60s
    enabled: true
logging:
  level: debug
  format: json
`
	cfg, err := Parse([]byte(yaml))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Server.Host != "127.0.0.1" {
		t.Errorf("expected host 127.0.0.1, got %s", cfg.Server.Host)
	}
	if cfg.Server.Port != 9090 {
		t.Errorf("expected port 9090, got %d", cfg.Server.Port)
	}
	if len(cfg.Processes) != 1 {
		t.Errorf("expected 1 process, got %d", len(cfg.Processes))
	}
	nginx, ok := cfg.Processes["nginx"]
	if !ok {
		t.Fatal("expected nginx process")
	}
	if nginx.Command != "nginx -g daemon off;" {
		t.Errorf("unexpected command: %s", nginx.Command)
	}
	if nginx.StartSecs != 2 {
		t.Errorf("expected startsecs 2, got %d", nginx.StartSecs)
	}
	if nginx.Environment["ENV"] != "production" {
		t.Errorf("expected ENV=production, got %s", nginx.Environment["ENV"])
	}
	if len(cfg.Schedules) != 1 {
		t.Errorf("expected 1 schedule, got %d", len(cfg.Schedules))
	}
	if cfg.Logging.Level != "debug" {
		t.Errorf("expected level debug, got %s", cfg.Logging.Level)
	}
}

func TestParseInvalidYAML(t *testing.T) {
	_, err := Parse([]byte(`invalid: [yaml: broken`))
	if err == nil {
		t.Fatal("expected error for invalid YAML")
	}
}

func TestValidateMissingCommand(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Processes["bad"] = ProcessConfig{
		Command: "",
	}
	err := cfg.Validate()
	if err == nil {
		t.Fatal("expected error for missing command")
	}
}

func TestValidateInvalidAutorestart(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Processes["bad"] = ProcessConfig{
		Command:     "echo hello",
		Autorestart: "maybe",
	}
	err := cfg.Validate()
	if err == nil {
		t.Fatal("expected error for invalid autorestart")
	}
}

func TestValidateInvalidPort(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Server.Port = 99999
	err := cfg.Validate()
	if err == nil {
		t.Fatal("expected error for invalid port")
	}
}

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()
	if cfg.Server.Host != "0.0.0.0" {
		t.Errorf("expected host 0.0.0.0, got %s", cfg.Server.Host)
	}
	if cfg.Server.Port != 8080 {
		t.Errorf("expected port 8080, got %d", cfg.Server.Port)
	}
}

func TestLoadFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.yml")
	content := `
server:
  port: 3000
processes:
  echo:
    command: "echo hello"
`
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Server.Port != 3000 {
		t.Errorf("expected port 3000, got %d", cfg.Server.Port)
	}
}

func TestLoadMissingFile(t *testing.T) {
	_, err := Load("/nonexistent/config.yml")
	if err == nil {
		t.Fatal("expected error for missing file")
	}
}

func TestSanitize(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Server.Auth.Password = "secret123"
	cfg.Server.Auth.Secret = "jwt-key"
	sanitized := cfg.Sanitize()
	if sanitized.Server.Auth.Password != "***" {
		t.Errorf("expected password masked, got %s", sanitized.Server.Auth.Password)
	}
	if sanitized.Server.Auth.Secret != "***" {
		t.Errorf("expected secret masked, got %s", sanitized.Server.Auth.Secret)
	}
}

func TestSaveAndLoad(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "saved.yml")
	cfg := DefaultConfig()
	cfg.Server.Port = 4000
	cfg.Processes["test"] = ProcessConfig{
		Command:   "echo test",
		Autostart: true,
	}
	if err := Save(cfg, path); err != nil {
		t.Fatalf("save error: %v", err)
	}
	loaded, err := Load(path)
	if err != nil {
		t.Fatalf("load error: %v", err)
	}
	if loaded.Server.Port != 4000 {
		t.Errorf("expected port 4000, got %d", loaded.Server.Port)
	}
	if _, ok := loaded.Processes["test"]; !ok {
		t.Error("expected test process in loaded config")
	}
}