package tui

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/snail2sky/starfleet/internal/config"
	"github.com/snail2sky/starfleet/internal/engine"
)

// Manager is the interface TUI uses to manage processes.
// LocalManager talks directly to engine.Manager; RemoteManager
// talks to a running starfleet server's REST API.
type Manager interface {
	List() []engine.ProcessInfo
	Get(name string) (*engine.ProcessInfo, error)
	Config(name string) (config.ProcessConfig, error)
	Start(name string) error
	Stop(name string) error
	Restart(name string) error
	// Logs returns the most recent log lines for a process.
	Logs(name string, lines int) []engine.LogEntry
	Mode() string // "local" or "remote"
}

// LocalManager manages processes in the same process (standalone TUI).
type LocalManager struct {
	mgr *engine.Manager
}

// NewLocalManager wraps a local engine.Manager.
func NewLocalManager(mgr *engine.Manager) *LocalManager {
	return &LocalManager{mgr: mgr}
}

func (l *LocalManager) List() []engine.ProcessInfo { return l.mgr.List() }
func (l *LocalManager) Get(name string) (*engine.ProcessInfo, error) {
	return l.mgr.Get(name)
}
func (l *LocalManager) Config(name string) (config.ProcessConfig, error) {
	return l.mgr.GetConfig(name)
}
func (l *LocalManager) Start(name string) error  { return l.mgr.Start(name) }
func (l *LocalManager) Stop(name string) error   { return l.mgr.Stop(name) }
func (l *LocalManager) Restart(name string) error { return l.mgr.Restart(name) }

// Logs reads captured lines from the in-process log collector.
func (l *LocalManager) Logs(name string, lines int) []engine.LogEntry {
	return engine.DefaultCollector().LastN(name, lines)
}

func (l *LocalManager) Mode() string { return "local" }

// RemoteManager talks to a running starfleet server via REST API.
type RemoteManager struct {
	base  string
	http  *http.Client
}

// NewRemoteManager creates a manager backed by the server API.
func NewRemoteManager(baseURL string) *RemoteManager {
	return &RemoteManager{
		base: strings.TrimRight(baseURL, "/"),
		http: &http.Client{Timeout: 15 * time.Second},
	}
}

// ProbeServer checks whether a starfleet server is reachable.
func ProbeServer(baseURL string) bool {
	client := &http.Client{Timeout: 2 * time.Second}
	resp, err := client.Get(strings.TrimRight(baseURL, "/") + "/health")
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	return resp.StatusCode == http.StatusOK
}

func (r *RemoteManager) Mode() string { return "remote" }

func (r *RemoteManager) List() []engine.ProcessInfo {
	var out struct {
		Data []engine.ProcessInfo `json:"data"`
	}
	if err := r.get("/api/v1/processes", &out); err != nil {
		return nil
	}
	return out.Data
}

func (r *RemoteManager) Get(name string) (*engine.ProcessInfo, error) {
	var out struct {
		Data engine.ProcessInfo `json:"data"`
	}
	if err := r.get("/api/v1/processes/"+name, &out); err != nil {
		return nil, err
	}
	return &out.Data, nil
}

func (r *RemoteManager) Config(name string) (config.ProcessConfig, error) {
	var out struct {
		Config config.ProcessConfig `json:"config"`
	}
	if err := r.get("/api/v1/processes/"+name, &out); err != nil {
		return config.ProcessConfig{}, err
	}
	return out.Config, nil
}

func (r *RemoteManager) Start(name string) error  { return r.post("/api/v1/processes/" + name + "/start") }
func (r *RemoteManager) Stop(name string) error   { return r.post("/api/v1/processes/" + name + "/stop") }
func (r *RemoteManager) Restart(name string) error { return r.post("/api/v1/processes/" + name + "/restart") }

// Logs fetches recent log lines from the server API.
func (r *RemoteManager) Logs(name string, lines int) []engine.LogEntry {
	var out struct {
		Data []engine.LogEntry `json:"data"`
	}
	path := fmt.Sprintf("/api/v1/processes/%s/logs?lines=%d", name, lines)
	if err := r.get(path, &out); err != nil {
		return nil
	}
	return out.Data
}

func (r *RemoteManager) get(path string, v interface{}) error {
	resp, err := r.http.Get(r.base + path)
	if err != nil {
		return fmt.Errorf("api request %s: %w", path, err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 400 {
		return fmt.Errorf("api %s: status %d: %s", path, resp.StatusCode, strings.TrimSpace(string(body)))
	}
	return json.Unmarshal(body, v)
}

func (r *RemoteManager) post(path string) error {
	resp, err := r.http.Post(r.base+path, "application/json", nil)
	if err != nil {
		return fmt.Errorf("api request %s: %w", path, err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 400 {
		return fmt.Errorf("api %s: status %d: %s", path, resp.StatusCode, strings.TrimSpace(string(body)))
	}
	return nil
}