package scheduler

import (
	"sort"
	"time"
)

// RecentDailyCap bounds the per-response run list of a daily report.
const RecentDailyCap = 100

// DailyJobStat summarizes one schedule's executions today.
type DailyJobStat struct {
	Name         string        `json:"name"`
	Runs         int           `json:"runs"`
	OK           int           `json:"ok"`
	Failed       int           `json:"failed"`
	LastStart    time.Time     `json:"last_start"` // zero when no runs yet
	LastExit     int           `json:"last_exit"`
	LastError    string        `json:"last_error,omitempty"`
	LastDuration time.Duration `json:"last_duration"` // nanoseconds, like HistoryEntry
}

// DailyReport is today's cross-job execution summary (server local time).
type DailyReport struct {
	Date   string         `json:"date"` // YYYY-MM-DD
	Total  int            `json:"total"`
	OK     int            `json:"ok"`
	Failed int            `json:"failed"`
	Jobs   []DailyJobStat `json:"jobs"`   // jobs with >=1 run today, by name
	Recent []HistoryEntry `json:"recent"` // today, newest first, capped
}

// Failed reports whether a history entry counts as a failed run.
func (e HistoryEntry) Failed() bool {
	return e.ExitCode != 0 || e.Error != ""
}

// Daily aggregates today's executions from the in-memory history.
// History is memory-only (per-job ring, cleared on restart), so the
// report covers everything since the last starfleet start.
func (s *Scheduler) Daily() DailyReport {
	now := time.Now()
	dayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())

	report := DailyReport{
		Date:   now.Format("2006-01-02"),
		Jobs:   []DailyJobStat{},
		Recent: []HistoryEntry{},
	}
	byName := map[string]*DailyJobStat{}

	// All() is already sorted newest-first, so Recent stays in that order.
	for _, e := range s.history.All(0) {
		if e.StartTime.Before(dayStart) {
			continue
		}
		report.Total++
		if e.Failed() {
			report.Failed++
		} else {
			report.OK++
		}
		stat := byName[e.Name]
		if stat == nil {
			stat = &DailyJobStat{Name: e.Name}
			byName[e.Name] = stat
		}
		stat.Runs++
		if e.Failed() {
			stat.Failed++
		} else {
			stat.OK++
		}
		// Newest-first iteration: first seen is the last run.
		if stat.LastStart.IsZero() {
			stat.LastStart = e.StartTime
			stat.LastExit = e.ExitCode
			stat.LastError = e.Error
			stat.LastDuration = e.Duration
		}
		if len(report.Recent) < RecentDailyCap {
			report.Recent = append(report.Recent, e)
		}
	}

	for _, stat := range byName {
		report.Jobs = append(report.Jobs, *stat)
	}
	sort.Slice(report.Jobs, func(i, j int) bool {
		return report.Jobs[i].Name < report.Jobs[j].Name
	})
	return report
}
