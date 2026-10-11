package engine

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseMaxBytes(t *testing.T) {
	tests := []struct {
		in   string
		want int64
	}{
		{"", 50 * 1024 * 1024},     // documented default
		{"1024", 1024},             // plain number
		{"4096000000", 4096000000}, // supervisord-style raw integer
		{"50M", 50 * 1024 * 1024},  // suffix
		{"2G", 2 * 1024 * 1024 * 1024},
		{"100k", 100 * 1024},       // lowercase
		{"10MB", 10 * 1024 * 1024}, // with B
		{"not-a-size", 0},          // unparsable → unlimited
		{"0", 0},                   // zero → unlimited
	}
	for _, tt := range tests {
		if got := parseMaxBytes(tt.in); got != tt.want {
			t.Errorf("parseMaxBytes(%q) = %d, want %d", tt.in, got, tt.want)
		}
	}
}

func TestFileLogWritesAndRotates(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "proc.log")

	// Tiny limit forces rotation after a couple of lines.
	fl := newFileLog(path, "100", 3)
	if fl == nil {
		t.Fatal("expected fileLog to open")
	}
	for i := 0; i < 50; i++ {
		fl.writeLine("some log line to exceed the rotation limit quickly")
	}
	fl.close()

	// Current file exists and is below the limit after rotation.
	st, err := os.Stat(path)
	if err != nil {
		t.Fatalf("log file missing: %v", err)
	}
	if st.Size() >= 100 {
		t.Errorf("expected rotation to reset size, got %d bytes", st.Size())
	}

	// At least one backup exists with supervisord naming (.1).
	if _, err := os.Stat(path + ".1"); err != nil {
		t.Errorf("expected rotated backup %s.1: %v", path, err)
	}
}

func TestFileLogNilSafety(t *testing.T) {
	var fl *fileLog
	fl.writeLine("no-op") // must not panic
	fl.close()

	if got := newFileLog("", "1M", 5); got != nil {
		t.Error("empty path should yield nil fileLog")
	}
}

func TestFileLogBadPathIsNil(t *testing.T) {
	// Unwritable path must not panic or block; it degrades to nil.
	got := newFileLog(filepath.Join(t.TempDir(), "no", "such", "dir", "x.log"), "1M", 5)
	if got != nil {
		t.Error("unwritable path should yield nil fileLog")
	}
}
