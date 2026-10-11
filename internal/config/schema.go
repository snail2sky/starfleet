// Package config defines the configuration schema for Starfleet.
package config

import (
	"time"
)

// Config is the root configuration structure.
type Config struct {
	Includes      []string                  `yaml:"includes" json:"includes,omitempty"`
	Server        ServerConfig              `yaml:"server" json:"server"`
	Processes     map[string]ProcessConfig  `yaml:"processes" json:"processes"`
	Schedules     map[string]ScheduleConfig `yaml:"schedules" json:"schedules"`
	Logging       LoggingConfig             `yaml:"logging" json:"logging"`
	Notifications NotificationsConfig       `yaml:"notifications" json:"notifications"`
}

// ServerConfig is the HTTP server configuration.
type ServerConfig struct {
	Host         string        `yaml:"host" json:"host"`
	Port         int           `yaml:"port" json:"port"`
	ReadTimeout  time.Duration `yaml:"read_timeout" json:"read_timeout"`
	WriteTimeout time.Duration `yaml:"write_timeout" json:"write_timeout"`
	Auth         AuthConfig    `yaml:"auth" json:"auth"`
}

// AuthConfig is the authentication configuration.
type AuthConfig struct {
	Enabled  bool   `yaml:"enabled" json:"enabled"`
	Username string `yaml:"username" json:"username"`
	Password string `yaml:"password" json:"password"` // bcrypt hash
	Secret   string `yaml:"secret" json:"secret"`     // JWT secret
}

// ProcessConfig defines a managed process.
type ProcessConfig struct {
	Command      string             `yaml:"command" json:"command"`
	Directory    string             `yaml:"directory" json:"directory"`
	User         string             `yaml:"user" json:"user"`
	Autostart    bool               `yaml:"autostart" json:"autostart"`
	Autorestart  string             `yaml:"autorestart" json:"autorestart"` // "true", "false", "unexpected"
	StartSecs    int                `yaml:"startsecs" json:"startsecs"`
	StartRetries int                `yaml:"startretries" json:"startretries"`
	StopSignal   string             `yaml:"stopsignal" json:"stopsignal"`
	StopWaitSecs int                `yaml:"stopwaitsecs" json:"stopwaitsecs"`
	ExitCodes    []int              `yaml:"exitcodes" json:"exitcodes,omitempty"`
	Environment  map[string]string  `yaml:"environment" json:"environment,omitempty"`
	EnvFile      string             `yaml:"envfile" json:"envfile,omitempty"`
	StdoutLog    LogConfig          `yaml:"stdout" json:"stdout"`
	StderrLog    LogConfig          `yaml:"stderr" json:"stderr"`
	HealthCheck  *HealthCheckConfig `yaml:"healthcheck" json:"healthcheck,omitempty"`
	Labels       map[string]string  `yaml:"labels" json:"labels,omitempty"`
	DependsOn    []string           `yaml:"depends_on" json:"depends_on,omitempty"`
	RestartDelay time.Duration      `yaml:"restart_delay" json:"restart_delay"`
	BackoffLimit time.Duration      `yaml:"backoff_limit" json:"backoff_limit"`
}

// LogConfig defines log file settings.
type LogConfig struct {
	Path     string `yaml:"path" json:"path"`
	MaxBytes string `yaml:"maxbytes" json:"maxbytes"`
	Backups  int    `yaml:"backups" json:"backups"`
}

// HealthCheckConfig defines a health check.
type HealthCheckConfig struct {
	Type     string        `yaml:"type" json:"type"` // "http", "tcp", "process"
	URL      string        `yaml:"url" json:"url,omitempty"`
	Address  string        `yaml:"address" json:"address,omitempty"`
	Interval time.Duration `yaml:"interval" json:"interval"`
	Timeout  time.Duration `yaml:"timeout" json:"timeout"`
	Retries  int           `yaml:"retries" json:"retries"`
}

// ScheduleConfig defines a scheduled task.
type ScheduleConfig struct {
	Cron    string            `yaml:"cron" json:"cron"`
	Command string            `yaml:"command" json:"command,omitempty"`
	Type    string            `yaml:"type" json:"type"` // "exec" or "http"
	URL     string            `yaml:"url" json:"url,omitempty"`
	Method  string            `yaml:"method" json:"method,omitempty"`
	Timeout time.Duration     `yaml:"timeout" json:"timeout"`
	Enabled bool              `yaml:"enabled" json:"enabled"`
	Labels  map[string]string `yaml:"labels" json:"labels,omitempty"`
}

// LoggingConfig defines the logging configuration.
type LoggingConfig struct {
	Level  string `yaml:"level" json:"level"`
	Format string `yaml:"format" json:"format"`
	Output string `yaml:"output" json:"output"`
}

// NotificationsConfig defines notification channels.
type NotificationsConfig struct {
	Webhook WebhookConfig `yaml:"webhook" json:"webhook"`
	Email   EmailConfig   `yaml:"email" json:"email"`
}

// WebhookConfig defines webhook notification settings.
type WebhookConfig struct {
	Enabled bool     `yaml:"enabled" json:"enabled"`
	URL     string   `yaml:"url" json:"url,omitempty"`
	Events  []string `yaml:"events" json:"events,omitempty"`
}

// EmailConfig defines email notification settings.
type EmailConfig struct {
	Enabled  bool   `yaml:"enabled" json:"enabled"`
	SMTPHost string `yaml:"smtp_host" json:"smtp_host,omitempty"`
	SMTPPort int    `yaml:"smtp_port" json:"smtp_port,omitempty"`
	From     string `yaml:"from" json:"from,omitempty"`
	To       string `yaml:"to" json:"to,omitempty"`
}

// DefaultConfig returns a Config with sensible defaults.
func DefaultConfig() *Config {
	return &Config{
		Server: ServerConfig{
			Host:         "0.0.0.0",
			Port:         8080,
			ReadTimeout:  30 * time.Second,
			WriteTimeout: 30 * time.Second,
			Auth: AuthConfig{
				Enabled: false,
			},
		},
		Processes: make(map[string]ProcessConfig),
		Schedules: make(map[string]ScheduleConfig),
		Logging: LoggingConfig{
			Level:  "info",
			Format: "json",
			Output: "stdout",
		},
		Notifications: NotificationsConfig{},
	}
}

// DefaultProcessConfig returns a ProcessConfig with sensible defaults.
func DefaultProcessConfig() ProcessConfig {
	return ProcessConfig{
		Autostart:    true,
		Autorestart:  "unexpected",
		StartSecs:    1,
		StartRetries: 3,
		StopSignal:   "SIGTERM",
		StopWaitSecs: 10,
		ExitCodes:    []int{0},
		RestartDelay: 5 * time.Second,
		BackoffLimit: 300 * time.Second,
		StdoutLog: LogConfig{
			MaxBytes: "50M",
			Backups:  10,
		},
		StderrLog: LogConfig{
			MaxBytes: "50M",
			Backups:  10,
		},
	}
}

// DefaultScheduleConfig returns a ScheduleConfig with sensible defaults.
func DefaultScheduleConfig() ScheduleConfig {
	return ScheduleConfig{
		Type:    "exec",
		Method:  "GET",
		Timeout: 60 * time.Second,
		Enabled: true,
	}
}
