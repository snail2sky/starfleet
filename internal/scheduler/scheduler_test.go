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
