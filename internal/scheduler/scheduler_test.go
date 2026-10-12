package scheduler

import (
	"testing"
	"time"

	"github.com/snail2sky/starfleet/internal/config"
	"github.com/snail2sky/starfleet/internal/event"
)

func TestListShowsNextRun(t *testing.T) {
	bus := event.NewBus(4)
	defer bus.Close()
	s := NewScheduler(bus)

	cfg := config.DefaultConfig()
	cfg.Schedules["every-second"] = config.ScheduleConfig{
		Cron:    "@every 1h",
		Command: "echo hi",
		Type:    "exec",
		Enabled: true,
	}
	if err := s.LoadFromConfig(cfg); err != nil {
		t.Fatalf("load: %v", err)
	}
	s.Start()
	defer s.Stop()

	// robfig computes Next after Start; give it a moment.
	time.Sleep(100 * time.Millisecond)

	jobs := s.List()
	if len(jobs) != 1 {
		t.Fatalf("expected 1 job, got %d", len(jobs))
	}
	if jobs[0].NextRun == "" {
		t.Error("expected NextRun to be populated for a scheduled job")
	}
	if _, err := time.ParseInLocation("2006-01-02 15:04:05", jobs[0].NextRun, time.Local); err != nil {
		t.Errorf("NextRun not parseable: %q", jobs[0].NextRun)
	}

	info, err := s.Get("every-second")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if info.NextRun == "" {
		t.Error("expected Get() to fill NextRun too")
	}
}

func TestRunTriggersJobAndRecordsHistory(t *testing.T) {
	bus := event.NewBus(4)
	defer bus.Close()
	s := NewScheduler(bus)

	cfg := config.DefaultConfig()
	cfg.Schedules["manual"] = config.ScheduleConfig{
		Cron:    "@every 24h",
		Command: "echo manual-run",
		Type:    "exec",
		Enabled: true,
	}
	if err := s.LoadFromConfig(cfg); err != nil {
		t.Fatalf("load: %v", err)
	}
	s.Start()
	defer s.Stop()

	if err := s.Run("manual"); err != nil {
		t.Fatalf("run: %v", err)
	}

	// Poll for the history entry (exec is async).
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if hist := s.History("manual", 10); len(hist) > 0 {
			if hist[0].Error != "" {
				t.Errorf("job failed: %s", hist[0].Error)
			}
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatal("timed out waiting for job history entry")
}

func TestReloadConfigUpdatesChangedSchedules(t *testing.T) {
	bus := event.NewBus(4)
	defer bus.Close()
	s := NewScheduler(bus)

	cfg := config.DefaultConfig()
	cfg.Schedules["job"] = config.ScheduleConfig{
		Cron: "@every 1h", Command: "echo one", Type: "exec", Enabled: true,
	}
	if err := s.LoadFromConfig(cfg); err != nil {
		t.Fatalf("load: %v", err)
	}
	if got := mustJob(t, s, "job").Cron; got != "@every 1h" {
		t.Fatalf("initial cron: %s", got)
	}

	// Change the cron expression + add a new schedule + drop nothing.
	cfg2 := config.DefaultConfig()
	cfg2.Schedules["job"] = config.ScheduleConfig{
		Cron: "@every 2h", Command: "echo two", Type: "exec", Enabled: true,
	}
	cfg2.Schedules["extra"] = config.ScheduleConfig{
		Cron: "0 4 * * *", Command: "echo extra", Type: "exec", Enabled: true,
	}
	if err := s.ReloadConfig(cfg2); err != nil {
		t.Fatalf("reload: %v", err)
	}

	if got := mustJob(t, s, "job").Cron; got != "@every 2h" {
		t.Errorf("changed cron not applied: got %s", got)
	}
	if got := mustJob(t, s, "job").Command; got != "echo two" {
		t.Errorf("changed command not applied: got %s", got)
	}
	mustJob(t, s, "extra")
	if len(s.List()) != 2 {
		t.Errorf("expected 2 schedules, got %d", len(s.List()))
	}

	// Removing a schedule takes it away.
	cfg3 := config.DefaultConfig()
	cfg3.Schedules["job"] = cfg2.Schedules["job"]
	if err := s.ReloadConfig(cfg3); err != nil {
		t.Fatalf("reload3: %v", err)
	}
	if _, err := s.Get("extra"); err == nil {
		t.Error("removed schedule still present")
	}
}

func mustJob(t *testing.T, s *Scheduler, name string) JobInfo {
	t.Helper()
	info, err := s.Get(name)
	if err != nil {
		t.Fatalf("get %s: %v", name, err)
	}
	return *info
}
