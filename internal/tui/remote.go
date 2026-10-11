package tui

import (
	"github.com/snail2sky/starfleet/internal/client"
	"github.com/snail2sky/starfleet/internal/config"
	"github.com/snail2sky/starfleet/internal/engine"
	"github.com/snail2sky/starfleet/internal/selflog"
)

// Manager is the interface TUI uses to manage processes and read logs.
// LocalManager runs in-process; RemoteManager talks to a starfleet server.
type Manager interface {
	List() []engine.ProcessInfo
	Get(name string) (*engine.ProcessInfo, error)
	Config(name string) (config.ProcessConfig, error)
	Start(name string) error
	Stop(name string) error
	Restart(name string) error
	// Logs returns the most recent log lines for a process.
	Logs(name string, lines int) []engine.LogEntry
	// ServerLogs returns starfleet's own log lines.
	ServerLogs(lines int) []engine.LogEntry
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
func (l *LocalManager) Start(name string) error   { return l.mgr.Start(name) }
func (l *LocalManager) Stop(name string) error    { return l.mgr.Stop(name) }
func (l *LocalManager) Restart(name string) error { return l.mgr.Restart(name) }

// Logs reads captured lines from the in-process log collector.
func (l *LocalManager) Logs(name string, lines int) []engine.LogEntry {
	return engine.DefaultCollector().LastN(name, lines)
}

// ServerLogs reads starfleet's own captured log lines.
func (l *LocalManager) ServerLogs(lines int) []engine.LogEntry {
	return selflog.Default().AsLogEntries(lines)
}

func (l *LocalManager) Mode() string { return "local" }

// RemoteManager reads and controls a running starfleet server.
type RemoteManager struct {
	*client.Client
}

// NewRemoteManager creates a manager backed by the server API.
func NewRemoteManager(baseURL string) *RemoteManager {
	return &RemoteManager{Client: client.New(baseURL)}
}

// Mode implements Manager.
func (r *RemoteManager) Mode() string { return "remote" }

// ProbeServer checks whether a starfleet server is reachable.
func ProbeServer(baseURL string) bool { return client.Probe(baseURL) }
