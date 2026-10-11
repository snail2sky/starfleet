// Package selflog captures starfleet's own structured logs in memory so
// they can be viewed through the API, WebUI and TUI.
package selflog

import (
	"context"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/snail2sky/starfleet/internal/engine"
)

// Entry is one captured log record.
type Entry struct {
	Time    time.Time         `json:"time"`
	Level   string            `json:"level"`
	Message string            `json:"message"`
	Attrs   map[string]string `json:"attrs,omitempty"`
}

// Ring is a fixed-size ring buffer of entries.
type Ring struct {
	mu    sync.RWMutex
	buf   []Entry
	head  int
	count int
	max   int
}

// NewRing creates a ring holding at most max entries.
func NewRing(max int) *Ring {
	return &Ring{buf: make([]Entry, max), max: max}
}

// Append stores an entry, evicting the oldest when full.
func (r *Ring) Append(e Entry) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.buf[r.head] = e
	r.head = (r.head + 1) % r.max
	if r.count < r.max {
		r.count++
	}
}

// LastN returns the newest n entries, oldest first.
func (r *Ring) LastN(n int) []Entry {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if n > r.count {
		n = r.count
	}
	out := make([]Entry, n)
	start := (r.head - n + r.max) % r.max
	for i := 0; i < n; i++ {
		out[i] = r.buf[(start+i)%r.max]
	}
	return out
}

// Count returns how many entries are stored.
func (r *Ring) Count() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.count
}

var defaultRing = NewRing(2000)

// Default returns the process-wide ring.
func Default() *Ring { return defaultRing }

// AsLogEntries converts ring contents into engine.LogEntry (the same shape
// the process-log API returns) so all clients share one log format.
// stream = log level, process = "starfleet".
func (r *Ring) AsLogEntries(n int) []engine.LogEntry {
	entries := r.LastN(n)
	out := make([]engine.LogEntry, 0, len(entries))
	for _, e := range entries {
		var b strings.Builder
		b.WriteString(e.Message)
		for k, v := range e.Attrs {
			b.WriteString(" " + k + "=" + v)
		}
		out = append(out, engine.LogEntry{
			Process:   "starfleet",
			Stream:    e.Level,
			Line:      b.String(),
			Timestamp: e.Time.UnixMilli(),
		})
	}
	return out
}

// Handler wraps a slog.Handler, mirroring every record into a ring while
// passing it through to the wrapped handler (JSON file/stdout).
type Handler struct {
	inner slog.Handler
	ring  *Ring
	attrs []slog.Attr
}

// NewHandler wraps inner with ring capture.
func NewHandler(inner slog.Handler, ring *Ring) *Handler {
	return &Handler{inner: inner, ring: ring}
}

// Enabled implements slog.Handler.
func (h *Handler) Enabled(ctx context.Context, level slog.Level) bool {
	return h.inner.Enabled(ctx, level)
}

// Handle implements slog.Handler.
func (h *Handler) Handle(ctx context.Context, r slog.Record) error {
	entry := Entry{
		Time:    r.Time,
		Level:   r.Level.String(),
		Message: r.Message,
		Attrs:   make(map[string]string, len(h.attrs)+r.NumAttrs()),
	}
	for _, a := range h.attrs {
		entry.Attrs[a.Key] = a.Value.String()
	}
	r.Attrs(func(a slog.Attr) bool {
		entry.Attrs[a.Key] = a.Value.String()
		return true
	})
	h.ring.Append(entry)
	return h.inner.Handle(ctx, r)
}

// WithAttrs implements slog.Handler.
func (h *Handler) WithAttrs(attrs []slog.Attr) slog.Handler {
	merged := make([]slog.Attr, 0, len(h.attrs)+len(attrs))
	merged = append(merged, h.attrs...)
	merged = append(merged, attrs...)
	return &Handler{inner: h.inner.WithAttrs(attrs), ring: h.ring, attrs: merged}
}

// WithGroup implements slog.Handler.
func (h *Handler) WithGroup(name string) slog.Handler {
	return &Handler{inner: h.inner.WithGroup(name), ring: h.ring, attrs: h.attrs}
}
