package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// Load reads and parses a YAML configuration file with include support.
// It supports:
//   - Main config file (starfleet.yml)
//   - includes: list of files/directories to include
//   - Auto-include conf.d/*.yml if conf.d/ exists next to main config
func Load(path string) (*Config, error) {
	cfg, err := loadFile(path)
	if err != nil {
		return nil, err
	}

	// Auto-include conf.d/ if it exists next to the main config.
	dir := filepath.Dir(path)
	confD := filepath.Join(dir, "conf.d")
	if info, statErr := os.Stat(confD); statErr == nil && info.IsDir() {
		if err := cfg.loadDir(confD); err != nil {
			return nil, fmt.Errorf("load conf.d: %w", err)
		}
	}

	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("validate config: %w", err)
	}
	return cfg, nil
}

// loadFile loads a single YAML config file and merges it.
func loadFile(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config %s: %w", path, err)
	}
	cfg := DefaultConfig()
	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("parse config %s: %w", path, err)
	}

	// Process includes.
	for _, inc := range cfg.Includes {
		incPath := inc
		if !filepath.IsAbs(incPath) {
			incPath = filepath.Join(filepath.Dir(path), inc)
		}
		info, err := os.Stat(incPath)
		if err != nil {
			return nil, fmt.Errorf("include %q: %w", inc, err)
		}
		if info.IsDir() {
			if err := cfg.loadDir(incPath); err != nil {
				return nil, fmt.Errorf("include dir %q: %w", inc, err)
			}
		} else {
			sub, err := loadFile(incPath)
			if err != nil {
				return nil, err
			}
			cfg.merge(sub)
		}
	}

	return cfg, nil
}

// loadDir loads all .yml/.yaml files from a directory.
func (c *Config) loadDir(dir string) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if !strings.HasSuffix(name, ".yml") && !strings.HasSuffix(name, ".yaml") {
			continue
		}
		sub, err := loadFile(filepath.Join(dir, name))
		if err != nil {
			return fmt.Errorf("load %s: %w", name, err)
		}
		c.merge(sub)
	}
	return nil
}

// merge merges another config into this one.
// Processes and schedules from the other config override existing ones.
func (c *Config) merge(other *Config) {
	for k, v := range other.Processes {
		if c.Processes == nil {
			c.Processes = make(map[string]ProcessConfig)
		}
		c.Processes[k] = v
	}
	for k, v := range other.Schedules {
		if c.Schedules == nil {
			c.Schedules = make(map[string]ScheduleConfig)
		}
		c.Schedules[k] = v
	}
}

// Parse parses YAML bytes into a Config, applying defaults.
func Parse(data []byte) (*Config, error) {
	cfg := DefaultConfig()
	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("validate config: %w", err)
	}
	return cfg, nil
}

// Validate checks the configuration for common errors.
func (c *Config) Validate() error {
	if c.Server.Port < 1 || c.Server.Port > 65535 {
		return fmt.Errorf("invalid server port: %d", c.Server.Port)
	}
	for name, p := range c.Processes {
		if p.Command == "" {
			return fmt.Errorf("process %q: command is required", name)
		}
		if p.Autorestart != "" && p.Autorestart != "true" && p.Autorestart != "false" && p.Autorestart != "unexpected" {
			return fmt.Errorf("process %q: autorestart must be true, false, or unexpected", name)
		}
		if p.StartSecs < 0 {
			return fmt.Errorf("process %q: startsecs must be >= 0", name)
		}
		if p.StartRetries < 0 {
			return fmt.Errorf("process %q: startretries must be >= 0", name)
		}
		if p.HealthCheck != nil {
			switch p.HealthCheck.Type {
			case "http", "tcp", "process":
			default:
				return fmt.Errorf("process %q: healthcheck type must be http, tcp, or process", name)
			}
		}
	}
	for name, s := range c.Schedules {
		if s.Cron == "" {
			return fmt.Errorf("schedule %q: cron is required", name)
		}
		if s.Type == "exec" && s.Command == "" {
			return fmt.Errorf("schedule %q: command is required for exec type", name)
		}
		if s.Type == "http" && s.URL == "" {
			return fmt.Errorf("schedule %q: url is required for http type", name)
		}
	}
	return nil
}

// Save writes the configuration to a YAML file.
func Save(cfg *Config, path string) error {
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}
	return os.WriteFile(path, data, 0644)
}

// Sanitize returns a copy of the config with sensitive fields masked.
func (c *Config) Sanitize() *Config {
	out := *c
	if out.Server.Auth.Password != "" {
		out.Server.Auth.Password = "***"
	}
	if out.Server.Auth.Secret != "" {
		out.Server.Auth.Secret = "***"
	}
	return &out
}