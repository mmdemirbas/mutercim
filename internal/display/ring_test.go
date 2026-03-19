package display

import (
	"strings"
	"testing"
	"time"
)

func TestRingBuffer_PushThreeIntoSizeFive(t *testing.T) {
	rb := NewRingBuffer(5)
	for i := 0; i < 3; i++ {
		rb.Push(LogEntry{Message: "msg" + string(rune('A'+i))})
	}

	if rb.Len() != 3 {
		t.Errorf("Len() = %d, want 3", rb.Len())
	}

	entries := rb.Entries()
	if len(entries) != 3 {
		t.Fatalf("Entries() len = %d, want 3", len(entries))
	}
	for i, want := range []string{"msgA", "msgB", "msgC"} {
		if entries[i].Message != want {
			t.Errorf("entries[%d].Message = %q, want %q", i, entries[i].Message, want)
		}
	}
}

func TestRingBuffer_PushEightIntoSizeFive_OldestDropped(t *testing.T) {
	rb := NewRingBuffer(5)
	for i := 0; i < 8; i++ {
		rb.Push(LogEntry{Message: "msg" + string(rune('A'+i))})
	}

	if rb.Len() != 5 {
		t.Errorf("Len() = %d, want 5", rb.Len())
	}

	entries := rb.Entries()
	if len(entries) != 5 {
		t.Fatalf("Entries() len = %d, want 5", len(entries))
	}
	// Should contain items D,E,F,G,H (oldest 3 dropped)
	for i, want := range []string{"msgD", "msgE", "msgF", "msgG", "msgH"} {
		if entries[i].Message != want {
			t.Errorf("entries[%d].Message = %q, want %q", i, entries[i].Message, want)
		}
	}
}

func TestRingBuffer_Empty(t *testing.T) {
	rb := NewRingBuffer(5)
	if rb.Len() != 0 {
		t.Errorf("Len() = %d, want 0", rb.Len())
	}
	if entries := rb.Entries(); entries != nil {
		t.Errorf("Entries() = %v, want nil", entries)
	}
}

func TestRingBuffer_ExactlyFull(t *testing.T) {
	rb := NewRingBuffer(3)
	for i := 0; i < 3; i++ {
		rb.Push(LogEntry{Message: "msg" + string(rune('A'+i))})
	}

	if rb.Len() != 3 {
		t.Errorf("Len() = %d, want 3", rb.Len())
	}

	entries := rb.Entries()
	for i, want := range []string{"msgA", "msgB", "msgC"} {
		if entries[i].Message != want {
			t.Errorf("entries[%d].Message = %q, want %q", i, entries[i].Message, want)
		}
	}
}

func TestRingBuffer_SizeOne(t *testing.T) {
	rb := NewRingBuffer(1)
	rb.Push(LogEntry{Message: "first"})
	rb.Push(LogEntry{Message: "second"})

	entries := rb.Entries()
	if len(entries) != 1 {
		t.Fatalf("Entries() len = %d, want 1", len(entries))
	}
	if entries[0].Message != "second" {
		t.Errorf("entry = %q, want %q", entries[0].Message, "second")
	}
}

func TestRingBuffer_ZeroSize(t *testing.T) {
	rb := NewRingBuffer(0)
	// Should default to size 1
	rb.Push(LogEntry{Message: "test"})
	if rb.Len() != 1 {
		t.Errorf("Len() = %d, want 1", rb.Len())
	}
}

func TestFormatLogEntry_Normal(t *testing.T) {
	entry := LogEntry{
		Time:    time.Date(2024, 1, 15, 9, 14, 1, 0, time.UTC),
		Message: "page 245 \u2014 4 entries, 2 footnotes",
		Level:   LogNormal,
	}
	got := FormatLogEntry(entry, 80, StatusColors{Enabled: false})

	if !strings.HasPrefix(got, "09:14:01") {
		t.Errorf("should start with timestamp, got: %q", got)
	}
	if !strings.Contains(got, "4 entries") {
		t.Errorf("should contain '4 entries', got: %q", got)
	}
	// Should not have suffix
	if strings.Contains(got, "\u26a0") || strings.Contains(got, "\u2717") {
		t.Errorf("normal entry should not have warning/error suffix, got: %q", got)
	}
}

func TestFormatLogEntry_Warning(t *testing.T) {
	entry := LogEntry{
		Time:    time.Date(2024, 1, 15, 9, 14, 4, 0, time.UTC),
		Message: "page 247 \u2014 hadith number gap",
		Level:   LogWarning,
	}
	got := FormatLogEntry(entry, 80, StatusColors{Enabled: false})

	if !strings.Contains(got, "\u26a0") {
		t.Errorf("warning entry should contain ⚠, got: %q", got)
	}
}

func TestFormatLogEntry_Error(t *testing.T) {
	entry := LogEntry{
		Time:    time.Date(2024, 1, 15, 9, 14, 6, 0, time.UTC),
		Message: "page 249 \u2014 failed after 3 retries",
		Level:   LogError,
	}
	got := FormatLogEntry(entry, 80, StatusColors{Enabled: false})

	if !strings.Contains(got, "\u2717") {
		t.Errorf("error entry should contain ✗, got: %q", got)
	}
}

func TestFormatLogEntry_Truncation(t *testing.T) {
	entry := LogEntry{
		Time:    time.Date(2024, 1, 15, 9, 14, 1, 0, time.UTC),
		Message: strings.Repeat("a", 100),
		Level:   LogNormal,
	}
	got := FormatLogEntry(entry, 40, StatusColors{Enabled: false})

	// "09:14:01 " = 9 chars, so message should be truncated to ~28 chars + "..."
	runes := []rune(got)
	if len(runes) > 40 {
		t.Errorf("truncated line should be at most 40 runes, got %d: %q", len(runes), got)
	}
	if !strings.HasSuffix(got, "...") {
		t.Errorf("truncated line should end with '...', got: %q", got)
	}
}

func TestFormatLogEntry_TruncationWithSuffix(t *testing.T) {
	entry := LogEntry{
		Time:    time.Date(2024, 1, 15, 9, 14, 1, 0, time.UTC),
		Message: strings.Repeat("b", 100),
		Level:   LogWarning,
	}
	got := FormatLogEntry(entry, 40, StatusColors{Enabled: false})

	// Should still contain suffix
	if !strings.Contains(got, "\u26a0") {
		t.Errorf("truncated warning should still have ⚠, got: %q", got)
	}
	if !strings.Contains(got, "...") {
		t.Errorf("truncated line should contain '...', got: %q", got)
	}
}

func TestFormatLogEntry_TimestampFormat(t *testing.T) {
	entry := LogEntry{
		Time:    time.Date(2024, 3, 5, 14, 7, 3, 0, time.UTC),
		Message: "test",
		Level:   LogNormal,
	}
	got := FormatLogEntry(entry, 80, StatusColors{Enabled: false})

	if !strings.HasPrefix(got, "14:07:03") {
		t.Errorf("should use HH:MM:SS format, got: %q", got)
	}
}

func TestFormatLogEntry_NoTruncationWhenShort(t *testing.T) {
	entry := LogEntry{
		Time:    time.Date(2024, 1, 15, 9, 14, 1, 0, time.UTC),
		Message: "page 1",
		Level:   LogNormal,
	}
	got := FormatLogEntry(entry, 80, StatusColors{Enabled: false})

	if strings.Contains(got, "...") {
		t.Errorf("short line should not be truncated, got: %q", got)
	}
	want := "09:14:01 page 1"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}
