package scheduler

import (
	"sync"
	"time"
)

// HistoryEntry represents a single execution of a scheduled job.
type HistoryEntry struct {
	Name      string        `json:"name"`
	StartTime time.Time     `json:"start_time"`
	EndTime   time.Time     `json:"end_time"`
	Duration  time.Duration `json:"duration"`
	ExitCode  int           `json:"exit_code"`
	Output    string        `json:"output"`
	Error     string        `json:"error"`
}

// History stores execution history for all scheduled jobs.
type History struct {
	mu      sync.RWMutex
	entries map[string][]HistoryEntry
	maxSize int
}

// NewHistory creates a new History with the given max size per job.
func NewHistory(maxSize int) *History {
	return &History{
		entries: make(map[string][]HistoryEntry),
		maxSize: maxSize,
	}
}

// Add adds an entry to the history.
func (h *History) Add(entry HistoryEntry) {
	h.mu.Lock()
	defer h.mu.Unlock()
	entries := h.entries[entry.Name]
	entries = append(entries, entry)
	if len(entries) > h.maxSize {
		entries = entries[len(entries)-h.maxSize:]
	}
	h.entries[entry.Name] = entries
}

// Get returns the last N entries for a job.
func (h *History) Get(name string, limit int) []HistoryEntry {
	h.mu.RLock()
	defer h.mu.RUnlock()
	entries := h.entries[name]
	if limit > 0 && limit < len(entries) {
		return entries[len(entries)-limit:]
	}
	result := make([]HistoryEntry, len(entries))
	copy(result, entries)
	return result
}

// All returns all entries across all jobs.
func (h *History) All(limit int) []HistoryEntry {
	h.mu.RLock()
	defer h.mu.RUnlock()
	var all []HistoryEntry
	for _, entries := range h.entries {
		all = append(all, entries...)
	}
	// Sort by start time (most recent first).
	for i := 1; i < len(all); i++ {
		for j := i; j > 0 && all[j].StartTime.After(all[j-1].StartTime); j-- {
			all[j], all[j-1] = all[j-1], all[j]
		}
	}
	if limit > 0 && limit < len(all) {
		return all[:limit]
	}
	return all
}