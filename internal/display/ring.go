package display

import (
	"fmt"
	"time"
)

// LogLevel classifies a log entry for display formatting.
type LogLevel int

const (
	// LogNormal is a regular log entry.
	LogNormal LogLevel = iota
	// LogWarning is a warning log entry (shown with ⚠).
	LogWarning
	// LogError is an error log entry (shown with ✗).
	LogError
)

// LogEntry is a single entry in the log tail ring buffer.
type LogEntry struct {
	Time    time.Time
	Message string
	Level   LogLevel
}

// RingBuffer is a fixed-size circular buffer of log entries.
type RingBuffer struct {
	entries []LogEntry
	size    int
	head    int // next write position
	count   int
}

// NewRingBuffer creates a ring buffer with the given capacity.
func NewRingBuffer(size int) *RingBuffer {
	if size <= 0 {
		size = 1
	}
	return &RingBuffer{
		entries: make([]LogEntry, size),
		size:    size,
	}
}

// Push adds an entry, evicting the oldest if the buffer is full.
func (r *RingBuffer) Push(entry LogEntry) {
	r.entries[r.head] = entry
	r.head = (r.head + 1) % r.size
	if r.count < r.size {
		r.count++
	}
}

// Entries returns all stored entries in chronological order (oldest first).
func (r *RingBuffer) Entries() []LogEntry {
	if r.count == 0 {
		return nil
	}
	result := make([]LogEntry, r.count)
	start := 0
	if r.count == r.size {
		start = r.head // oldest entry is at head when full
	}
	for i := 0; i < r.count; i++ {
		result[i] = r.entries[(start+i)%r.size]
	}
	return result
}

// Len returns the number of entries currently stored.
func (r *RingBuffer) Len() int {
	return r.count
}

// FormatLogEntry formats a log entry for display.
// The line is truncated to maxWidth visible characters with "..." if needed.
func FormatLogEntry(entry LogEntry, maxWidth int, colors StatusColors) string {
	ts := entry.Time.Format("15:04:05")
	msg := entry.Message

	suffix := ""
	suffixWidth := 0
	switch entry.Level {
	case LogWarning:
		suffix = "  " + colors.yellow("\u26a0")
		suffixWidth = 3 // "  ⚠"
	case LogError:
		suffix = "  " + colors.red("\u2717")
		suffixWidth = 3 // "  ✗"
	}

	// Truncate message if line would exceed maxWidth.
	// "HH:MM:SS " prefix = 9 visible chars.
	if maxWidth > 0 {
		lineWidth := 9 + len([]rune(msg)) + suffixWidth
		if lineWidth > maxWidth {
			allowed := maxWidth - 9 - suffixWidth - 3 // 3 for "..."
			if allowed < 0 {
				allowed = 0
			}
			runes := []rune(msg)
			if allowed < len(runes) {
				msg = string(runes[:allowed]) + "..."
			}
		}
	}

	return fmt.Sprintf("%s %s%s", ts, msg, suffix)
}
