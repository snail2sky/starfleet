package engine

import (
	"bufio"
	"io"
	"log/slog"
	"sync"
	"time"
)

// LogEntry represents a single log line.
type LogEntry struct {
	Process   string `json:"process"`
	Stream    string `json:"stream"` // "stdout" or "stderr"
	Line      string `json:"line"`
	Timestamp int64  `json:"timestamp"` // unix milliseconds
}

// LogRing is a thread-safe ring buffer for log entries.
type LogRing struct {
	mu      sync.RWMutex
	buf     []LogEntry
	size    int
	head    int
	count   int
	maxSize int
}

// NewLogRing creates a new ring buffer with the given capacity.
func NewLogRing(capacity int) *LogRing {
	return &LogRing{
		buf:     make([]LogEntry, capacity),
		maxSize: capacity,
	}
}

// Append adds a log entry to the ring buffer.
func (r *LogRing) Append(entry LogEntry) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.buf[r.head] = entry
	r.head = (r.head + 1) % r.maxSize
	if r.count < r.maxSize {
		r.count++
	}
}

// LastN returns the last N log entries in chronological order.
func (r *LogRing) LastN(n int) []LogEntry {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if n > r.count {
		n = r.count
	}
	result := make([]LogEntry, n)
	start := (r.head - n + r.maxSize) % r.maxSize
	for i := 0; i < n; i++ {
		result[i] = r.buf[(start+i)%r.maxSize]
	}
	return result
}

// All returns all log entries in chronological order.
func (r *LogRing) All() []LogEntry {
	return r.LastN(r.count)
}

// Count returns the number of entries in the buffer.
func (r *LogRing) Count() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.count
}

// LogCollector collects logs from all processes.
type LogCollector struct {
	mu      sync.RWMutex
	rings   map[string]*LogRing
	maxSize int
}

// NewLogCollector creates a new log collector.
func NewLogCollector(maxSize int) *LogCollector {
	return &LogCollector{
		rings:   make(map[string]*LogRing),
		maxSize: maxSize,
	}
}

// GetRing returns the log ring for a process, creating one if needed.
func (lc *LogCollector) GetRing(process string) *LogRing {
	lc.mu.Lock()
	defer lc.mu.Unlock()
	if ring, ok := lc.rings[process]; ok {
		return ring
	}
	ring := NewLogRing(lc.maxSize)
	lc.rings[process] = ring
	return ring
}

// LastN returns the last N log entries for a process.
func (lc *LogCollector) LastN(process string, n int) []LogEntry {
	return lc.GetRing(process).LastN(n)
}

// AllProcesses returns log entries from all processes, interleaved by timestamp.
func (lc *LogCollector) AllProcesses(n int) []LogEntry {
	lc.mu.RLock()
	defer lc.mu.RUnlock()
	var all []LogEntry
	for _, ring := range lc.rings {
		all = append(all, ring.All()...)
	}
	// Simple sort by timestamp.
	sortLogEntries(all)
	if len(all) > n {
		return all[len(all)-n:]
	}
	return all
}

func sortLogEntries(entries []LogEntry) {
	for i := 1; i < len(entries); i++ {
		for j := i; j > 0 && entries[j].Timestamp < entries[j-1].Timestamp; j-- {
			entries[j], entries[j-1] = entries[j-1], entries[j]
		}
	}
}

// LogCallback is called when a new log line is received.
type LogCallback func(entry LogEntry)

var logCallbacks []LogCallback
var logCallbacksMu sync.Mutex

// defaultCollector stores the most recent log lines for the API.
// Keyed by process name, ring-buffered.
var defaultCollector = NewLogCollector(2000)

// DefaultCollector returns the process-wide log collector.
func DefaultCollector() *LogCollector {
	return defaultCollector
}

// RegisterLogCallback registers a callback for new log entries.
func RegisterLogCallback(cb LogCallback) {
	logCallbacksMu.Lock()
	defer logCallbacksMu.Unlock()
	logCallbacks = append(logCallbacks, cb)
}

// collectLogs reads from an io.Reader, stores lines in the default
// collector, and notifies callbacks (WebSocket, etc.).
func collectLogs(reader io.Reader, process, stream string) {
	scanner := bufio.NewScanner(reader)
	scanner.Buffer(make([]byte, 64*1024), 1024*1024) // allow long lines
	for scanner.Scan() {
		entry := LogEntry{
			Process:   process,
			Stream:    stream,
			Line:      scanner.Text(),
			Timestamp: time.Now().UnixMilli(),
		}
		// Store in ring buffer for REST API queries.
		defaultCollector.GetRing(process).Append(entry)
		// Notify callbacks (WebSocket, etc.).
		logCallbacksMu.Lock()
		for _, cb := range logCallbacks {
			cb(entry)
		}
		logCallbacksMu.Unlock()
	}
	if err := scanner.Err(); err != nil {
		slog.Debug("log scanner error", "process", process, "stream", stream, "error", err)
	}
}
