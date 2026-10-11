package scheduler

import (
	"testing"
	"time"
)

func TestHistoryAddAndGet(t *testing.T) {
	h := NewHistory(100)
	entry := HistoryEntry{
		Name:      "test",
		StartTime: time.Now(),
		ExitCode:  0,
	}
	h.Add(entry)
	entries := h.Get("test", 10)
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
	if entries[0].Name != "test" {
		t.Errorf("expected name test, got %s", entries[0].Name)
	}
}

func TestHistoryMaxSize(t *testing.T) {
	h := NewHistory(3)
	for i := 0; i < 5; i++ {
		h.Add(HistoryEntry{Name: "test", StartTime: time.Now(), ExitCode: i})
		time.Sleep(10 * time.Millisecond)
	}
	entries := h.Get("test", 10)
	if len(entries) != 3 {
		t.Errorf("expected 3 entries, got %d", len(entries))
	}
	// Should have the last 3 (exit codes 2, 3, 4).
	if entries[0].ExitCode != 2 {
		t.Errorf("expected exit code 2, got %d", entries[0].ExitCode)
	}
}

func TestHistoryGetLimit(t *testing.T) {
	h := NewHistory(100)
	for i := 0; i < 5; i++ {
		h.Add(HistoryEntry{Name: "test", StartTime: time.Now()})
		time.Sleep(10 * time.Millisecond)
	}
	entries := h.Get("test", 2)
	if len(entries) != 2 {
		t.Errorf("expected 2 entries, got %d", len(entries))
	}
}

func TestHistoryGetNonexistent(t *testing.T) {
	h := NewHistory(100)
	entries := h.Get("nonexistent", 10)
	if entries == nil {
		t.Error("expected empty slice, got nil")
	}
	if len(entries) != 0 {
		t.Errorf("expected 0 entries, got %d", len(entries))
	}
}

func TestHistoryAll(t *testing.T) {
	h := NewHistory(100)
	h.Add(HistoryEntry{Name: "a", StartTime: time.Now()})
	time.Sleep(10 * time.Millisecond)
	h.Add(HistoryEntry{Name: "b", StartTime: time.Now()})
	all := h.All(0)
	if len(all) != 2 {
		t.Errorf("expected 2 entries, got %d", len(all))
	}
	// Most recent first.
	if all[0].Name != "b" {
		t.Errorf("expected b first, got %s", all[0].Name)
	}
}
