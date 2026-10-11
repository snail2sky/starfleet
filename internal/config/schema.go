// Package config defines the configuration schema for Starfleet.
package config

import (
	"time"
)

// Config is the root configuration structure.
type Config struct {
	Server        ServerConfig                `yaml:"server"`
	Processes     map[string]ProcessConfig    `yaml:"processes"`
	Schedules     map[string]ScheduleConfig   `yaml:"schedules"`
	Logging       LoggingConfig               `yaml:"logging"`
	Notifications NotificationsConfig         `yaml:"notifications"`
}

// ServerConfig is the HTTP server configuration.
type ServerConfig struct {
	Host         string        `yaml:"host"`
	Port         int           `yaml:"port"`
	ReadTimeout  time.Duration `yaml:"read_timeout"`
	WriteTimeout time.Duration `yaml:"write_timeout"`
	Auth         AuthConfig    `yaml:"auth"`
}

// AuthConfig is the authentication configuration.
type AuthConfig struct {
	Enabled  bool   `yaml:"enabled"`
	Username string `yaml:"username"`
	Password string `yaml:"password"` // bcrypt hash
	Secret   string `yaml:"secret"`   // JWT secret
}

// ProcessConfig defines a managed process.
type ProcessConfig struct {
	Command       string            `yaml:"command"`
	Directory     string            `yaml:"directory"`
	User          string            `yaml:"user"`
	Autostart     bool              `yaml:"autostart"`
	Autorestart   string            `yaml:"autorestart"` // "true", "false", "unexpected"
	StartSecs     int               `yaml:"startsecs"`
	StartRetries  int               `yaml:"startretries"`
	StopSignal    string            `yaml:"stopsignal"`
	StopWaitSecs  int               `yaml:"stopwaitsecs"`
	ExitCodes     []int             `yaml:"exitcodes"`
	Environment   map[string]string `yaml:"environment"`
	EnvFile       string            `yaml:"envfile"`
	StdoutLog     LogConfig         `yaml:"stdout"`
	StderrLog     LogConfig         `yaml:"stderr"`
	HealthCheck   *HealthCheckConfig `yaml:"healthcheck"`
	Labels        map[string]string `yaml:"labels"`
	DependsOn     []string          `yaml:"depends_on"`
	RestartDelay  time.Duration     `yaml:"restart_delay"`
	BackoffLimit  time.Duration     `yaml:"backoff_limit"`
}

// LogConfig defines log file settings.
type LogConfig struct {
	Path      string `yaml:"path"`
	MaxBytes  string `yaml:"maxbytes"`
	Backups   int    `yaml:"backups"`
}

// HealthCheckConfig defines a health check.
type HealthCheckConfig struct {
	Type     string        `yaml:"type"` // "http", "tcp", "process"
	URL      string        `yaml:"url"`
	Address  string        `yaml:"address"`
	Interval time.Duration `yaml:"interval"`
	Timeout  time.Duration `yaml:"timeout"`
	Retries  int           `yaml:"retries"`
}

// ScheduleConfig defines a scheduled task.
type ScheduleConfig struct {
	Cron     string        `yaml:"cron"`
	Command  string        `yaml:"command"`
	Type     string        `yaml:"type"` // "exec" or "http"
	URL      string        `yaml:"url"`
	Method   string        `yaml:"method"`
	Timeout  time.Duration `yaml:"timeout"`
	Enabled  bool          `yaml:"enabled"`
	Labels   map[string]string `yaml:"labels"`
}

// LoggingConfig defines the logging configuration.
type LoggingConfig struct {
	Level  string `yaml:"level"`
	Format string `yaml:"format"`
	Output string `yaml:"output"`
}

// NotificationsConfig defines notification channels.
type NotificationsConfig struct {
	Webhook WebhookConfig `yaml:"webhook"`
	Email   EmailConfig   `yaml:"email"`
}

// WebhookConfig defines webhook notification settings.
type WebhookConfig struct {
	Enabled bool     `yaml:"enabled"`
	URL     string   `yaml:"url"`
	Events  []string `yaml:"events"`
}

// EmailConfig defines email notification settings.
type EmailConfig struct {
	Enabled  bool   `yaml:"enabled"`
	SMTPHost string `yaml:"smtp_host"`
	SMTPPort int    `yaml:"smtp_port"`
	From     string `yaml:"from"`
	To       string `yaml:"to"`
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