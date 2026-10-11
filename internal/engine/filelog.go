package engine

import (
	"bufio"
	"fmt"
	"log/slog"
	"os"
	"strconv"
	"strings"
)

// parseMaxBytes parses a size like "50M", "1024", "2G", "4096000000".
// Empty string defaults to 50M (the documented default).
func parseMaxBytes(s string) int64 {
	s = strings.TrimSpace(s)
	if s == "" {
		return 50 * 1024 * 1024
	}
	mult := int64(1)
	upper := strings.ToUpper(s)
	switch {
	case strings.HasSuffix(upper, "K"), strings.HasSuffix(upper, "KB"):
		mult = 1024
		s = strings.TrimSuffix(strings.TrimSuffix(upper, "B"), "K")
	case strings.HasSuffix(upper, "M"), strings.HasSuffix(upper, "MB"):
		mult = 1024 * 1024
		s = strings.TrimSuffix(strings.TrimSuffix(upper, "B"), "M")
	case strings.HasSuffix(upper, "G"), strings.HasSuffix(upper, "GB"):
		mult = 1024 * 1024 * 1024
		s = strings.TrimSuffix(strings.TrimSuffix(upper, "B"), "G")
	default:
		s = upper
	}
	n, err := strconv.ParseInt(strings.TrimSpace(s), 10, 64)
	if err != nil || n <= 0 {
		return 0 // unlimited / unparsable
	}
	return n * mult
}

// fileLog appends process output to a file with size-based rotation
// (supervisord's stdout_logfile + maxbytes + backups equivalent).
// All methods are safe on a nil receiver (path unset → no-op).
type fileLog struct {
	path    string
	max     int64
	backups int
	f       *os.File
	bw      *bufio.Writer
	written int64
}

// newFileLog opens path for appending. Returns nil when path is empty or
// the file cannot be opened (a bad log path must not prevent the process
// from starting — the line still reaches the in-memory ring).
func newFileLog(path string, maxBytes string, backups int) *fileLog {
	if path == "" {
		return nil
	}
	if backups <= 0 {
		backups = 10
	}
	fl := &fileLog{path: path, max: parseMaxBytes(maxBytes), backups: backups}
	if err := fl.open(); err != nil {
		slog.Warn("cannot open log file, continuing without file logging",
			"path", path, "error", err)
		return nil
	}
	return fl
}

func (fl *fileLog) open() error {
	f, err := os.OpenFile(fl.path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return err
	}
	fl.f = f
	fl.bw = bufio.NewWriterSize(f, 8192)
	fl.written = 0
	// Count existing size so rotation also works across restarts.
	if st, err := f.Stat(); err == nil {
		fl.written = st.Size()
	}
	return nil
}

// writeLine appends one log line and rotates when the size limit is hit.
func (fl *fileLog) writeLine(line string) {
	if fl == nil || fl.f == nil {
		return
	}
	n, err := fl.bw.WriteString(line + "\n")
	if err != nil {
		return
	}
	fl.written += int64(n)
	_ = fl.bw.Flush()
	if fl.max > 0 && fl.written >= fl.max {
		fl.rotate()
	}
}

// rotate shifts path.1..path.N and starts a fresh file (supervisord naming).
func (fl *fileLog) rotate() {
	_ = fl.bw.Flush()
	_ = fl.f.Close()

	// Drop the oldest backup, then shift every file up one slot.
	_ = os.Remove(fmt.Sprintf("%s.%d", fl.path, fl.backups))
	for i := fl.backups - 1; i >= 1; i-- {
		_ = os.Rename(fmt.Sprintf("%s.%d", fl.path, i), fmt.Sprintf("%s.%d", fl.path, i+1))
	}
	_ = os.Rename(fl.path, fl.path+".1")

	if err := fl.open(); err != nil {
		slog.Warn("log rotation failed, file logging disabled", "path", fl.path, "error", err)
		fl.f = nil
	}
}

// close flushes and closes the file.
func (fl *fileLog) close() {
	if fl == nil || fl.f == nil {
		return
	}
	_ = fl.bw.Flush()
	_ = fl.f.Close()
	fl.f = nil
}
