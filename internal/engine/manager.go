package engine

import (
	"fmt"
	"log/slog"
	"sort"
	"sync"

	"github.com/snail2sky/starfleet/internal/config"
)

// Manager manages all configured processes.
type Manager struct {
	mu        sync.RWMutex
	processes map[string]*Process
	configs   map[string]config.ProcessConfig
	autos     []string // processes that should auto-start
	logger    *slog.Logger
}

// NewManager creates a new process manager.
func NewManager() *Manager {
	return &Manager{
		processes: make(map[string]*Process),
		configs:   make(map[string]config.ProcessConfig),
		logger:    slog.With("component", "manager"),
	}
}

// LoadFromConfig loads process definitions from configuration.
func (m *Manager) LoadFromConfig(cfg *config.Config) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.configs = make(map[string]config.ProcessConfig)
	m.autos = nil
	for name, pcfg := range cfg.Processes {
		m.configs[name] = pcfg
		if pcfg.Autostart {
			m.autos = append(m.autos, name)
		}
	}
	m.logger.Info("loaded process configs", "count", len(m.configs), "autostart", len(m.autos))
}

// StartAll starts all autostart processes.
func (m *Manager) StartAll() {
	m.mu.RLock()
	autos := make([]string, len(m.autos))
	copy(autos, m.autos)
	m.mu.RUnlock()

	for _, name := range autos {
		if err := m.Start(name); err != nil {
			m.logger.Error("failed to autostart process", "name", name, "error", err)
		}
	}
}

// StopAll stops all running processes.
func (m *Manager) StopAll() {
	m.mu.RLock()
	names := make([]string, 0, len(m.processes))
	for name := range m.processes {
		names = append(names, name)
	}
	m.mu.RUnlock()

	for _, name := range names {
		if err := m.Stop(name); err != nil {
			m.logger.Error("failed to stop process", "name", name, "error", err)
		}
	}
}

// Start starts a named process.
func (m *Manager) Start(name string) error {
	m.mu.Lock()
	p, exists := m.processes[name]
	if !exists {
		cfg, cfgExists := m.configs[name]
		if !cfgExists {
			m.mu.Unlock()
			return fmt.Errorf("process %q not found", name)
		}
		p = NewProcess(name, cfg)
		m.processes[name] = p
	}
	m.mu.Unlock()
	return p.Start()
}

// Stop stops a named process.
func (m *Manager) Stop(name string) error {
	m.mu.RLock()
	p, exists := m.processes[name]
	m.mu.RUnlock()
	if !exists {
		return fmt.Errorf("process %q not found", name)
	}
	return p.Stop()
}

// Restart restarts a named process.
func (m *Manager) Restart(name string) error {
	m.mu.RLock()
	p, exists := m.processes[name]
	m.mu.RUnlock()
	if !exists {
		return fmt.Errorf("process %q not found", name)
	}
	return p.Restart()
}

// GetProcess returns a process by name.
func (m *Manager) GetProcess(name string) (*Process, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	p, ok := m.processes[name]
	return p, ok
}

// ProcessInfo holds runtime information about a process.
type ProcessInfo struct {
	Name        string            `json:"name"`
	State       string            `json:"state"`
	PID         int               `json:"pid"`
	Uptime      string            `json:"uptime"`
	ExitCode    int               `json:"exit_code"`
	RetryCount  int               `json:"retry_count"`
	Command     string            `json:"command"`
	Directory   string            `json:"directory"`
	Autostart   bool              `json:"autostart"`
	Autorestart string            `json:"autorestart"`
	Labels      map[string]string `json:"labels"`
}

// SetAutostart toggles whether a process starts with starfleet (boot).
// It does not affect a currently running process; use Start/Stop for that.
func (m *Manager) SetAutostart(name string, enabled bool) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	cfg, ok := m.configs[name]
	if !ok {
		return fmt.Errorf("process %q not found", name)
	}
	cfg.Autostart = enabled
	m.configs[name] = cfg
	return nil
}

// GetConfig returns the full configuration of a process.
func (m *Manager) GetConfig(name string) (config.ProcessConfig, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	cfg, ok := m.configs[name]
	if !ok {
		return config.ProcessConfig{}, fmt.Errorf("process %q not found", name)
	}
	return cfg, nil
}

// List returns info about all configured processes.
// List returns info about all configured processes, sorted by name.
// A stable order matters: Go map iteration is randomized, and clients
// (TUI/WebUI) re-render this list on every refresh — unsorted output
// would shuffle rows every second.
func (m *Manager) List() []ProcessInfo {
	m.mu.RLock()
	defer m.mu.RUnlock()
	names := make([]string, 0, len(m.configs))
	for name := range m.configs {
		names = append(names, name)
	}
	sort.Strings(names)
	list := make([]ProcessInfo, 0, len(names))
	for _, name := range names {
		cfg := m.configs[name]
		info := ProcessInfo{
			Name:        name,
			State:       StateStopped.String(),
			Command:     cfg.Command,
			Directory:   cfg.Directory,
			Autostart:   cfg.Autostart,
			Autorestart: cfg.Autorestart,
			Labels:      cfg.Labels,
		}
		if p, ok := m.processes[name]; ok {
			info.State = p.State().String()
			info.PID = p.PID()
			info.Uptime = p.Uptime().String()
			info.ExitCode = p.ExitCode()
			info.RetryCount = p.RetryCount()
		}
		list = append(list, info)
	}
	return list
}

// Get returns info about a single process.
func (m *Manager) Get(name string) (*ProcessInfo, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	cfg, ok := m.configs[name]
	if !ok {
		return nil, fmt.Errorf("process %q not found", name)
	}
	info := &ProcessInfo{
		Name:        name,
		State:       StateStopped.String(),
		Command:     cfg.Command,
		Directory:   cfg.Directory,
		Autostart:   cfg.Autostart,
		Autorestart: cfg.Autorestart,
		Labels:      cfg.Labels,
	}
	if p, ok := m.processes[name]; ok {
		info.State = p.State().String()
		info.PID = p.PID()
		info.Uptime = p.Uptime().String()
		info.ExitCode = p.ExitCode()
		info.RetryCount = p.RetryCount()
	}
	return info, nil
}

// ReloadConfig applies a new configuration, starting/stopping/restarting processes as needed.
func (m *Manager) ReloadConfig(newCfg *config.Config) error {
	m.mu.Lock()
	oldConfigs := m.configs
	m.configs = make(map[string]config.ProcessConfig)
	m.autos = nil
	for name, pcfg := range newCfg.Processes {
		m.configs[name] = pcfg
		if pcfg.Autostart {
			m.autos = append(m.autos, name)
		}
	}
	m.mu.Unlock()

	// Stop removed processes.
	for name := range oldConfigs {
		if _, exists := newCfg.Processes[name]; !exists {
			m.logger.Info("stopping removed process", "name", name)
			if err := m.Stop(name); err != nil {
				m.logger.Error("failed to stop removed process", "name", name, "error", err)
			}
			m.mu.Lock()
			delete(m.processes, name)
			m.mu.Unlock()
		}
	}

	// Start or restart changed processes.
	for name, newPCfg := range newCfg.Processes {
		oldPCfg, existed := oldConfigs[name]
		if !existed {
			// New process.
			if newPCfg.Autostart {
				m.logger.Info("starting new process", "name", name)
				if err := m.Start(name); err != nil {
					m.logger.Error("failed to start new process", "name", name, "error", err)
				}
			}
			continue
		}

		// Check if config changed.
		if processConfigChanged(oldPCfg, newPCfg) {
			m.logger.Info("restarting changed process", "name", name)
			// Update the live process config.
			if p, ok := m.GetProcess(name); ok {
				p.mu.Lock()
				p.Config = newPCfg
				p.mu.Unlock()
				if p.State() == StateRunning {
					if err := m.Restart(name); err != nil {
						m.logger.Error("failed to restart process", "name", name, "error", err)
					}
				}
			}
		}
	}

	m.logger.Info("config reloaded", "processes", len(newCfg.Processes))
	return nil
}

// processConfigChanged checks if two process configs are different.
func processConfigChanged(a, b config.ProcessConfig) bool {
	return a.Command != b.Command ||
		a.Directory != b.Directory ||
		a.User != b.User ||
		a.Autostart != b.Autostart ||
		a.Autorestart != b.Autorestart ||
		a.StartSecs != b.StartSecs ||
		a.StartRetries != b.StartRetries ||
		a.StopSignal != b.StopSignal ||
		a.StopWaitSecs != b.StopWaitSecs
}
