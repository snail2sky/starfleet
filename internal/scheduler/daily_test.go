package scheduler

import (
	"github.com/snail2sky/starfleet/internal/event"
	"testing"
	"time"
)

func TestDailyAggregation(t *testing.T) {
	s := NewScheduler(event.NewBus(8))
	now := time.Now()
	day := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	yesterday := day.Add(-24 * time.Hour)

	add := func(name string, at time.Time, exit int, errMsg string) {
		s.history.Add(HistoryEntry{
			Name: name, StartTime: at, EndTime: at.Add(time.Second),
			Duration: time.Second, ExitCode: exit, Error: errMsg,
		})
	}

	// Today: alpha 3 runs (1 failed), beta 1 run. Plus a yesterday entry
	// that must NOT count.
	add("beta", day.Add(1*time.Hour), 0, "")
	add("alpha", day.Add(2*time.Hour), 0, "")
	add("alpha", day.Add(3*time.Hour), 1, "")
	add("alpha", day.Add(4*time.Hour), 0, "")
	add("alpha", yesterday, 1, "old failure")

	r := s.Daily()
	if r.Date != now.Format("2006-01-02") {
		t.Errorf("date = %q", r.Date)
	}
	if r.Total != 4 || r.OK != 3 || r.Failed != 1 {
		t.Errorf("totals = %d/%d/%d, want total=4 ok=3 failed=1", r.Total, r.OK, r.Failed)
	}
	if len(r.Jobs) != 2 {
		t.Fatalf("jobs = %d, want 2", len(r.Jobs))
	}
	// Sorted by name: alpha first.
	if r.Jobs[0].Name != "alpha" || r.Jobs[1].Name != "beta" {
		t.Errorf("job order = %s, %s", r.Jobs[0].Name, r.Jobs[1].Name)
	}
	a := r.Jobs[0]
	if a.Runs != 3 || a.Failed != 1 || a.OK != 2 {
		t.Errorf("alpha stats = %+v", a)
	}
	if !a.LastStart.Equal(day.Add(4 * time.Hour)) {
		t.Errorf("alpha last run = %v, want newest today entry", a.LastStart)
	}
	if a.LastExit != 0 {
		t.Errorf("alpha last exit = %d, want 0", a.LastExit)
	}
	if len(r.Recent) != 4 {
		t.Errorf("recent = %d entries, want 4 (yesterday excluded)", len(r.Recent))
	}
	// Newest first.
	if !r.Recent[0].StartTime.After(r.Recent[1].StartTime) {
		t.Error("recent must be newest-first")
	}
}

func TestDailyEmpty(t *testing.T) {
	s := NewScheduler(event.NewBus(8))
	r := s.Daily()
	if r.Total != 0 || len(r.Jobs) != 0 || len(r.Recent) != 0 {
		t.Errorf("expected empty report, got %+v", r)
	}
}
