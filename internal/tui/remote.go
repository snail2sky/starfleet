package tui

import (
	"fmt"

	"github.com/snail2sky/starfleet/internal/client"
	"github.com/snail2sky/starfleet/internal/config"
	"github.com/snail2sky/starfleet/internal/engine"
	"github.com/snail2sky/starfleet/internal/scheduler"
	"github.com/snail2sky/starfleet/internal/selflog"
)

// Manager is the interface TUI uses to manage processes, schedules and logs.
// LocalManager runs in-process; RemoteManager talks to a starfleet server.
type Manager interface {
	List() []engine.ProcessInfo
	Get(name string) (*engine.ProcessInfo, error)
	Config(name string) (config.ProcessConfig, error)
	Start(name string) error
	Stop(name string) error
	Restart(name string) error
	SetAutostart(name string, enabled bool) error
	// Logs returns the most recent log lines for a process.
	Logs(name string, lines int) []engine.LogEntry
	// ServerLogs returns starfleet's own log lines.
	ServerLogs(lines int) []engine.LogEntry
	Schedules() []scheduler.JobInfo
	RunSchedule(name string) error
	SetScheduleEnabled(name string, enabled bool) error
	ScheduleHistory(name string, limit int) []scheduler.HistoryEntry
	Mode() string // "local" or "remote"
}

// LocalManager manages processes in the same process (standalone TUI).
type LocalManager struct {
	mgr   *engine.Manager
	sched *scheduler.Scheduler
}

// NewLocalManager wraps a local engine.Manager and optional scheduler
// (standalone TUI runs both so the schedules tab works offline).
func NewLocalManager(mgr *engine.Manager, sched *scheduler.Scheduler) *LocalManager {
	return &LocalManager{mgr: mgr, sched: sched}
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

// SetAutostart toggles boot-start for a process (runtime only in
// standalone mode — there is no config file to persist to).
func (l *LocalManager) SetAutostart(name string, enabled bool) error {
	return l.mgr.SetAutostart(name, enabled)
}

// Schedules returns locally-configured jobs.
func (l *LocalManager) Schedules() []scheduler.JobInfo {
	if l.sched == nil {
		return nil
	}
	return l.sched.List()
}

// RunSchedule triggers a job immediately.
func (l *LocalManager) RunSchedule(name string) error {
	if l.sched == nil {
		return fmt.Errorf("scheduler not available in standalone TUI")
	}
	return l.sched.Run(name)
}

// SetScheduleEnabled toggles a job (runtime only in standalone mode).
func (l *LocalManager) SetScheduleEnabled(name string, enabled bool) error {
	if l.sched == nil {
		return fmt.Errorf("scheduler not available in standalone TUI")
	}
	return l.sched.SetEnabled(name, enabled)
}

// ScheduleHistory returns local execution history for a job.
func (l *LocalManager) ScheduleHistory(name string, limit int) []scheduler.HistoryEntry {
	if l.sched == nil {
		return nil
	}
	return l.sched.History(name, limit)
}

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

// SetAutostart forwards to the server (persisted to config file there).
func (r *RemoteManager) SetAutostart(name string, enabled bool) error {
	return r.Client.SetAutostart(name, enabled)
}

// Schedules returns jobs from the server.
func (r *RemoteManager) Schedules() []scheduler.JobInfo {
	jobs, err := r.Client.Schedules()
	if err != nil {
		return nil
	}
	return jobs
}

// RunSchedule triggers a job on the server.
func (r *RemoteManager) RunSchedule(name string) error {
	return r.Client.RunSchedule(name)
}

// SetScheduleEnabled toggles a job on the server (persisted there).
func (r *RemoteManager) SetScheduleEnabled(name string, enabled bool) error {
	return r.Client.SetScheduleEnabled(name, enabled)
}

// ScheduleHistory fetches execution history from the server.
func (r *RemoteManager) ScheduleHistory(name string, limit int) []scheduler.HistoryEntry {
	entries, err := r.Client.ScheduleHistory(name, limit)
	if err != nil {
		return nil
	}
	return entries
}

// ProbeServer checks whether a starfleet server is reachable.
func ProbeServer(baseURL string) bool { return client.Probe(baseURL) }
