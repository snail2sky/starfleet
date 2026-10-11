package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// Load reads and parses a YAML configuration file.
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config %s: %w", path, err)
	}
	return Parse(data)
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