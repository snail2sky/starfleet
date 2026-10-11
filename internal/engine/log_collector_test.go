package engine

import "testing"

func TestLogRingAppendAndLastN(t *testing.T) {
	ring := NewLogRing(5)
	for i := 0; i < 3; i++ {
		ring.Append(LogEntry{Line: string(rune('a' + i))})
	}
	entries := ring.LastN(2)
	if len(entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(entries))
	}
	if entries[0].Line != "b" {
		t.Errorf("expected 'b', got %q", entries[0].Line)
	}
	if entries[1].Line != "c" {
		t.Errorf("expected 'c', got %q", entries[1].Line)
	}
}

func TestLogRingOverflow(t *testing.T) {
	ring := NewLogRing(3)
	for i := 0; i < 5; i++ {
		ring.Append(LogEntry{Line: string(rune('a' + i))})
	}
	if ring.Count() != 3 {
		t.Errorf("expected count 3, got %d", ring.Count())
	}
	entries := ring.All()
	if len(entries) != 3 {
		t.Fatalf("expected 3 entries, got %d", len(entries))
	}
	// Should have c, d, e (last 3).
	if entries[0].Line != "c" {
		t.Errorf("expected 'c', got %q", entries[0].Line)
	}
	if entries[2].Line != "e" {
		t.Errorf("expected 'e', got %q", entries[2].Line)
	}
}

func TestLogRingLastNMoreThanCount(t *testing.T) {
	ring := NewLogRing(10)
	ring.Append(LogEntry{Line: "a"})
	entries := ring.LastN(5)
	if len(entries) != 1 {
		t.Errorf("expected 1 entry, got %d", len(entries))
	}
}

func TestLogCollectorMultipleProcesses(t *testing.T) {
	lc := NewLogCollector(100)
	ring1 := lc.GetRing("proc1")
	ring2 := lc.GetRing("proc2")
	ring1.Append(LogEntry{Process: "proc1", Line: "hello"})
	ring2.Append(LogEntry{Process: "proc2", Line: "world"})
	all := lc.AllProcesses(10)
	if len(all) != 2 {
		t.Errorf("expected 2 entries, got %d", len(all))
	}
}

func TestLogCollectorGetRingCreatesOnce(t *testing.T) {
	lc := NewLogCollector(100)
	r1 := lc.GetRing("test")
	r2 := lc.GetRing("test")
	if r1 != r2 {
		t.Error("expected same ring for same process")
	}
}