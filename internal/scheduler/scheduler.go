// Package scheduler provides cron-based task scheduling.
package scheduler

import (
	"context"
	"fmt"
	"log/slog"
	"os/exec"
	"reflect"
	"sort"
	"sync"
	"time"

	"github.com/robfig/cron/v3"
	"github.com/snail2sky/starfleet/internal/config"
	"github.com/snail2sky/starfleet/internal/event"
)

// Scheduler manages scheduled tasks.
type Scheduler struct {
	mu      sync.RWMutex
	cron    *cron.Cron
	jobs    map[string]*Job
	history *History
	bus     *event.Bus
	logger  *slog.Logger
}

// NewScheduler creates a new scheduler.
func NewScheduler(bus *event.Bus) *Scheduler {
	return &Scheduler{
		cron:    cron.New(),
		jobs:    make(map[string]*Job),
		history: NewHistory(1000),
		bus:     bus,
		logger:  slog.With("component", "scheduler"),
	}
}

// LoadFromConfig loads schedule definitions from configuration.
func (s *Scheduler) LoadFromConfig(cfg *config.Config) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for name, scfg := range cfg.Schedules {
		if _, exists := s.jobs[name]; exists {
			continue // already loaded
		}
		if !scfg.Enabled {
			continue
		}
		job := NewJob(name, scfg, s.history, s.bus)
		entryID, err := s.cron.AddJob(scfg.Cron, job)
		if err != nil {
			return fmt.Errorf("schedule %q: %w", name, err)
		}
		job.entryID = entryID
		s.jobs[name] = job
		s.logger.Info("scheduled task added", "name", name, "cron", scfg.Cron)
	}
	return nil
}

// Start starts the scheduler.
func (s *Scheduler) Start() {
	s.cron.Start()
	s.logger.Info("scheduler started")
}

// Stop stops the scheduler.
func (s *Scheduler) Stop() {
	ctx := s.cron.Stop()
	<-ctx.Done()
	s.logger.Info("scheduler stopped")
}

// List returns all scheduled jobs sorted by name (stable order for UI
// refreshes — Go map iteration is randomized).
func (s *Scheduler) List() []JobInfo {
	s.mu.RLock()
	defer s.mu.RUnlock()
	names := make([]string, 0, len(s.jobs))
	for name := range s.jobs {
		names = append(names, name)
	}
	sort.Strings(names)
	list := make([]JobInfo, 0, len(names))
	for _, name := range names {
		job := s.jobs[name]
		info := job.Info()
		s.fillNextRun(&info, job)
		list = append(list, info)
	}
	return list
}

// Get returns a single job by name.
func (s *Scheduler) Get(name string) (*JobInfo, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	job, ok := s.jobs[name]
	if !ok {
		return nil, fmt.Errorf("schedule %q not found", name)
	}
	info := job.Info()
	s.fillNextRun(&info, job)
	return &info, nil
}

// fillNextRun sets NextRun from the cron entry's next fire time.
// Callers must hold at least a read lock.
func (s *Scheduler) fillNextRun(info *JobInfo, job *Job) {
	entry := s.cron.Entry(job.entryID)
	if entry.ID > 0 && !entry.Next.IsZero() {
		info.NextRun = entry.Next.Format("2006-01-02 15:04:05")
	}
}

// Run triggers a job immediately.
func (s *Scheduler) Run(name string) error {
	s.mu.RLock()
	job, ok := s.jobs[name]
	s.mu.RUnlock()
	if !ok {
		return fmt.Errorf("schedule %q not found", name)
	}
	go job.Run()
	return nil
}

// History returns execution history for a job.
func (s *Scheduler) History(name string, limit int) []HistoryEntry {
	return s.history.Get(name, limit)
}

// ReloadConfig applies new schedule configuration: removes schedules that
// disappeared, re-creates schedules whose definition changed (e.g. a new
// cron expression), and registers newly added ones.
func (s *Scheduler) ReloadConfig(cfg *config.Config) error {
	s.mu.Lock()
	for name, job := range s.jobs {
		want, exists := cfg.Schedules[name]
		if !exists || !reflect.DeepEqual(job.Config, want) {
			s.cron.Remove(job.entryID)
			delete(s.jobs, name)
			if exists {
				s.logger.Info("schedule changed, recreating", "name", name)
			} else {
				s.logger.Info("removed schedule", "name", name)
			}
		}
	}
	s.mu.Unlock()

	// Add (or re-add) all enabled schedules.
	return s.LoadFromConfig(cfg)
}

// Job represents a scheduled task.
type Job struct {
	Name    string
	Config  config.ScheduleConfig
	history *History
	bus     *event.Bus
	entryID cron.EntryID
	logger  *slog.Logger
}

// NewJob creates a new Job.
func NewJob(name string, cfg config.ScheduleConfig, history *History, bus *event.Bus) *Job {
	return &Job{
		Name:    name,
		Config:  cfg,
		history: history,
		bus:     bus,
		logger:  slog.With("schedule", name),
	}
}

// Run executes the job (called by cron).
func (j *Job) Run() {
	j.logger.Info("schedule job starting")
	entry := HistoryEntry{
		Name:      j.Name,
		StartTime: time.Now(),
	}

	switch j.Config.Type {
	case "http":
		entry = j.runHTTP(entry)
	default: // "exec"
		entry = j.runExec(entry)
	}

	entry.EndTime = time.Now()
	entry.Duration = entry.EndTime.Sub(entry.StartTime)
	j.history.Add(entry)

	if entry.Error != "" {
		j.logger.Error("schedule job failed", "error", entry.Error, "duration", entry.Duration)
	} else {
		j.logger.Info("schedule job completed", "duration", entry.Duration)
	}

	if j.bus != nil {
		j.bus.Publish(event.Event{
			Type:    event.EventScheduleComplete,
			Process: j.Name,
			Data: map[string]interface{}{
				"success":  entry.Error == "",
				"duration": entry.Duration.String(),
			},
		})
	}
}

func (j *Job) runExec(entry HistoryEntry) HistoryEntry {
	timeout := j.Config.Timeout
	if timeout == 0 {
		timeout = 60 * time.Second
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "sh", "-c", j.Config.Command)
	output, err := cmd.CombinedOutput()
	entry.Output = string(output)
	if err != nil {
		entry.Error = err.Error()
		entry.ExitCode = -1
		if exitErr, ok := err.(*exec.ExitError); ok {
			entry.ExitCode = exitErr.ExitCode()
		}
	} else {
		entry.ExitCode = 0
	}
	return entry
}

func (j *Job) runHTTP(entry HistoryEntry) HistoryEntry {
	// HTTP job execution is a placeholder for now.
	entry.Error = "http job type not yet implemented"
	entry.ExitCode = -1
	return entry
}

// Info returns job information.
func (j *Job) Info() JobInfo {
	return JobInfo{
		Name:    j.Name,
		Cron:    j.Config.Cron,
		Type:    j.Config.Type,
		Command: j.Config.Command,
		URL:     j.Config.URL,
		Enabled: j.Config.Enabled,
		Timeout: j.Config.Timeout.String(),
		Labels:  j.Config.Labels,
	}
}

// JobInfo holds scheduled job information.
type JobInfo struct {
	Name    string            `json:"name"`
	Cron    string            `json:"cron"`
	Type    string            `json:"type"`
	Command string            `json:"command"`
	URL     string            `json:"url,omitempty"`
	Enabled bool              `json:"enabled"`
	Timeout string            `json:"timeout"`
	NextRun string            `json:"next_run,omitempty"` // next fire time, local tz
	Labels  map[string]string `json:"labels"`
}
