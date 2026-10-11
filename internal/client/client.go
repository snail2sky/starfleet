// Package client is the HTTP client for a running starfleet server.
// Used by the TUI (remote mode) and the CLI control subcommands.
package client

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/snail2sky/starfleet/internal/config"
	"github.com/snail2sky/starfleet/internal/engine"
	"github.com/snail2sky/starfleet/internal/scheduler"
)

// Client talks to a starfleet server's REST API.
type Client struct {
	base string
	http *http.Client
}

// New creates a client for baseURL (e.g. http://127.0.0.1:8080).
func New(baseURL string) *Client {
	return &Client{
		base: strings.TrimRight(baseURL, "/"),
		http: &http.Client{Timeout: 15 * time.Second},
	}
}

// Probe checks whether a starfleet server is reachable.
func Probe(baseURL string) bool {
	client := &http.Client{Timeout: 2 * time.Second}
	resp, err := client.Get(strings.TrimRight(baseURL, "/") + "/health")
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	return resp.StatusCode == http.StatusOK
}

// Base returns the base URL this client targets.
func (c *Client) Base() string { return c.base }

/* ---------- processes ---------- */

// List returns all managed processes.
func (c *Client) List() []engine.ProcessInfo {
	var out struct {
		Data []engine.ProcessInfo `json:"data"`
	}
	if err := c.get("/api/v1/processes", &out); err != nil {
		return nil
	}
	return out.Data
}

// Get returns one process's runtime info.
func (c *Client) Get(name string) (*engine.ProcessInfo, error) {
	var out struct {
		Data engine.ProcessInfo `json:"data"`
	}
	if err := c.get("/api/v1/processes/"+name, &out); err != nil {
		return nil, err
	}
	return &out.Data, nil
}

// Config returns one process's full configuration.
func (c *Client) Config(name string) (config.ProcessConfig, error) {
	var out struct {
		Config config.ProcessConfig `json:"config"`
	}
	if err := c.get("/api/v1/processes/"+name, &out); err != nil {
		return config.ProcessConfig{}, err
	}
	return out.Config, nil
}

// Start starts a process.
func (c *Client) Start(name string) error {
	return c.post("/api/v1/processes/" + name + "/start")
}

// Stop stops a process.
func (c *Client) Stop(name string) error {
	return c.post("/api/v1/processes/" + name + "/stop")
}

// Restart restarts a process.
func (c *Client) Restart(name string) error {
	return c.post("/api/v1/processes/" + name + "/restart")
}

/* ---------- logs ---------- */

// Logs returns recent log lines for a process.
func (c *Client) Logs(name string, lines int) []engine.LogEntry {
	var out struct {
		Data []engine.LogEntry `json:"data"`
	}
	path := fmt.Sprintf("/api/v1/processes/%s/logs?lines=%d", name, lines)
	if err := c.get(path, &out); err != nil {
		return nil
	}
	return out.Data
}

// ServerLogs returns starfleet's own recent log lines.
func (c *Client) ServerLogs(lines int) []engine.LogEntry {
	var out struct {
		Data []engine.LogEntry `json:"data"`
	}
	if err := c.get(fmt.Sprintf("/api/v1/server/logs?lines=%d", lines), &out); err != nil {
		return nil
	}
	return out.Data
}

/* ---------- schedules ---------- */

// Schedules returns all scheduled jobs.
func (c *Client) Schedules() ([]scheduler.JobInfo, error) {
	var out struct {
		Data []scheduler.JobInfo `json:"data"`
	}
	if err := c.get("/api/v1/schedules", &out); err != nil {
		return nil, err
	}
	return out.Data, nil
}

// RunSchedule triggers a scheduled job immediately.
func (c *Client) RunSchedule(name string) error {
	return c.post("/api/v1/schedules/" + name + "/run")
}

// ScheduleHistory returns execution history for a job.
func (c *Client) ScheduleHistory(name string, limit int) ([]scheduler.HistoryEntry, error) {
	var out struct {
		Data []scheduler.HistoryEntry `json:"data"`
	}
	path := fmt.Sprintf("/api/v1/schedules/%s/history?limit=%d", name, limit)
	if err := c.get(path, &out); err != nil {
		return nil, err
	}
	return out.Data, nil
}

/* ---------- config ---------- */

// Reload asks the server to hot-reload its configuration.
func (c *Client) Reload() error {
	return c.post("/api/v1/config/reload")
}

/* ---------- transport ---------- */

func (c *Client) get(path string, v interface{}) error {
	resp, err := c.http.Get(c.base + path)
	if err != nil {
		return fmt.Errorf("api request %s: %w", path, err)
	}
	return decode(resp, v)
}

func (c *Client) post(path string) error {
	resp, err := c.http.Post(c.base+path, "application/json", nil)
	if err != nil {
		return fmt.Errorf("api request %s: %w", path, err)
	}
	return decode(resp, nil)
}

func decode(resp *http.Response, v interface{}) error {
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 400 {
		return fmt.Errorf("status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	if v != nil {
		return json.Unmarshal(body, v)
	}
	return nil
}
