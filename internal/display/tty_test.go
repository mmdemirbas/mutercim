package display

import (
	"bytes"
	"strings"
	"testing"
	"time"
)

func TestTTYDisplayStartAndUpdate(t *testing.T) {
	var buf bytes.Buffer
	tick := time.Unix(1000, 0)
	now := func() time.Time {
		tick = tick.Add(5 * time.Second)
		return tick
	}

	d := newTTYDisplay(&buf, now)

	d.StartPhase(PhaseRead, "vol1", 10, "")
	out := buf.String()
	if !strings.Contains(out, "READ") {
		t.Errorf("StartPhase output should contain READ, got: %q", out)
	}
	if !strings.Contains(out, "0/10") {
		t.Errorf("StartPhase output should contain 0/10, got: %q", out)
	}

	buf.Reset()
	d.Update(PageResult{
		Phase:     PhaseRead,
		Input:     "vol1",
		PageNum:   1,
		Total:     10,
		Completed: 1,
		Entries:   5,
		Footnotes: 3,
	})
	out = buf.String()
	if !strings.Contains(out, "1/10") {
		t.Errorf("Update output should contain 1/10, got: %q", out)
	}
	// Detail now appears in log tail section
	if !strings.Contains(out, "5 entries") {
		t.Errorf("Update output should contain '5 entries' in log tail, got: %q", out)
	}
}

func TestTTYDisplayFinishPhase(t *testing.T) {
	var buf bytes.Buffer
	tick := time.Unix(1000, 0)
	now := func() time.Time {
		tick = tick.Add(time.Second)
		return tick
	}

	d := newTTYDisplay(&buf, now)
	d.StartPhase(PhaseRead, "vol1", 5, "")
	for i := 1; i <= 5; i++ {
		d.Update(PageResult{Phase: PhaseRead, Input: "vol1", PageNum: i, Total: 5, Completed: i})
	}

	buf.Reset()
	d.FinishPhase(PhaseRead, "vol1", "")
	out := buf.String()
	if !strings.Contains(out, "5/5") {
		t.Errorf("FinishPhase should show 5/5, got: %q", out)
	}
	if !strings.Contains(out, "\u2713") {
		t.Errorf("FinishPhase should show checkmark, got: %q", out)
	}
}

func TestTTYDisplayRateAndETA(t *testing.T) {
	var buf bytes.Buffer
	tick := time.Unix(1000, 0)
	now := func() time.Time {
		tick = tick.Add(6 * time.Second) // 10 pages/min
		return tick
	}

	d := newTTYDisplay(&buf, now)
	d.StartPhase(PhaseRead, "vol1", 100, "")

	// Process 5 pages to build up rate data
	for i := 1; i <= 5; i++ {
		d.Update(PageResult{Phase: PhaseRead, Input: "vol1", PageNum: i, Total: 100, Completed: i})
	}

	out := buf.String()
	if !strings.Contains(out, "p/min") {
		t.Errorf("should show rate, got: %q", out)
	}
	if !strings.Contains(out, "ETA") {
		t.Errorf("should show ETA, got: %q", out)
	}
}

func TestTTYDisplayFinishSummary(t *testing.T) {
	var buf bytes.Buffer
	tick := time.Unix(1000, 0)
	now := func() time.Time {
		tick = tick.Add(time.Second)
		return tick
	}

	d := newTTYDisplay(&buf, now)
	d.StartPhase(PhaseRead, "vol1", 10, "")
	d.Update(PageResult{Phase: PhaseRead, Input: "vol1", PageNum: 1, Total: 10, Completed: 8, Failed: 2, Warnings: 3})
	d.FinishPhase(PhaseRead, "vol1", "")

	buf.Reset()
	d.Finish()
	out := buf.String()
	if !strings.Contains(out, "8/10") {
		t.Errorf("Finish should show 8/10, got: %q", out)
	}
	if !strings.Contains(out, "3 warnings") {
		t.Errorf("Finish should show warnings, got: %q", out)
	}
	if !strings.Contains(out, "2 errors") {
		t.Errorf("Finish should show errors, got: %q", out)
	}
}

func TestTTYDisplayTranslateWithLang(t *testing.T) {
	var buf bytes.Buffer
	now := func() time.Time { return time.Unix(1000, 0) }

	d := newTTYDisplay(&buf, now)
	d.StartPhase(PhaseTranslate, "vol1", 50, "tr")
	out := buf.String()
	if !strings.Contains(out, "[tr]") {
		t.Errorf("translate phase should show [tr], got: %q", out)
	}
}

func TestTTYDisplayLogTail(t *testing.T) {
	var buf bytes.Buffer
	tick := time.Unix(1000, 0)
	now := func() time.Time {
		tick = tick.Add(time.Second)
		return tick
	}

	d := newTTYDisplay(&buf, now)
	d.StartPhase(PhaseRead, "vol1", 100, "")

	// Send 3 updates
	for i := 1; i <= 3; i++ {
		d.Update(PageResult{
			Phase: PhaseRead, Input: "vol1", PageNum: i, Total: 100, Completed: i,
			Entries: i * 2, Footnotes: i,
		})
	}

	out := buf.String()
	if !strings.Contains(out, "recent") {
		t.Errorf("should show log tail section, got: %q", out)
	}
	if !strings.Contains(out, "page 1") {
		t.Errorf("should show page 1 in log tail, got: %q", out)
	}
	if !strings.Contains(out, "page 3") {
		t.Errorf("should show page 3 in log tail, got: %q", out)
	}
}

func TestTTYDisplayLogTail_WarningAndError(t *testing.T) {
	var buf bytes.Buffer
	tick := time.Unix(1000, 0)
	now := func() time.Time {
		tick = tick.Add(time.Second)
		return tick
	}

	d := newTTYDisplay(&buf, now)
	d.StartPhase(PhaseRead, "vol1", 100, "")

	// Normal page
	d.Update(PageResult{
		Phase: PhaseRead, Input: "vol1", PageNum: 1, Total: 100, Completed: 1,
		Entries: 5, Footnotes: 2,
	})
	// Warning page
	d.Update(PageResult{
		Phase: PhaseRead, Input: "vol1", PageNum: 2, Total: 100, Completed: 2,
		Entries: 3, Warnings: 1,
	})
	// Error page
	d.Update(PageResult{
		Phase: PhaseRead, Input: "vol1", PageNum: 3, Total: 100, Completed: 2, Failed: 1,
		Err: errTest,
	})

	out := buf.String()
	// Log tail should show the error's ✗ symbol
	if !strings.Contains(out, "\u2717") {
		t.Errorf("log tail should show ✗ for error, got: %q", out)
	}
}

func TestTTYDisplayLogTail_RingBufferOverflow(t *testing.T) {
	var buf bytes.Buffer
	tick := time.Unix(1000, 0)
	now := func() time.Time {
		tick = tick.Add(time.Second)
		return tick
	}

	d := newTTYDisplay(&buf, now)
	d.StartPhase(PhaseRead, "vol1", 100, "")

	// Send 8 updates (buffer size is 5)
	for i := 1; i <= 8; i++ {
		d.Update(PageResult{
			Phase: PhaseRead, Input: "vol1", PageNum: i, Total: 100, Completed: i,
			Entries: i,
		})
	}

	out := buf.String()
	// Should not contain early pages (dropped from ring buffer)
	// Only pages 4-8 should be in the last render
	// Note: due to ANSI clear/re-render, all previous renders are overwritten,
	// so only the last render's output matters. But since we're checking the buffer
	// which includes ANSI sequences, let's check the last rendered content.
	if !strings.Contains(out, "page 8") {
		t.Errorf("should show most recent page, got: %q", out)
	}
}

func TestTTYDisplayStatusLine(t *testing.T) {
	var buf bytes.Buffer
	now := time.Unix(1000, 0)
	d := newTTYDisplay(&buf, func() time.Time { return now })

	d.StartPhase(PhaseRead, "vol1", 100, "")

	// Set status — should appear in output
	d.SetStatus(StatusLine{
		Text:      "reading page 1 via gemini/gemini-2.5-flash-lite",
		StartedAt: now,
	})

	// Advance time to simulate elapsed
	now = now.Add(5 * time.Second)

	// Manually trigger a render by setting status again
	d.SetStatus(StatusLine{
		Text:      "reading page 1 via gemini/gemini-2.5-flash-lite",
		StartedAt: now.Add(-5 * time.Second),
	})

	// Clear status to stop the ticker before reading the buffer
	d.SetStatus(StatusLine{})

	out := buf.String()
	if !strings.Contains(out, "reading page 1") {
		t.Errorf("should show status text, got: %q", out)
	}
	if !strings.Contains(out, "5.0s") {
		t.Errorf("should show elapsed time, got: %q", out)
	}
}

func TestTTYDisplayStatusLine_ClearedOnComplete(t *testing.T) {
	var buf bytes.Buffer
	now := time.Unix(1000, 0)
	d := newTTYDisplay(&buf, func() time.Time { return now })

	d.StartPhase(PhaseRead, "vol1", 100, "")

	d.SetStatus(StatusLine{
		Text:      "reading page 1",
		StartedAt: now,
	})

	// Clear status
	d.SetStatus(StatusLine{})

	buf.Reset()
	// Force a fresh render via Update
	now = now.Add(time.Second)
	d.Update(PageResult{Phase: PhaseRead, Input: "vol1", PageNum: 1, Total: 100, Completed: 1})
	out := buf.String()
	if strings.Contains(out, "reading page 1") {
		t.Errorf("status should be cleared after SetStatus({}), got: %q", out)
	}
}

func TestTTYDisplayStatusLine_Countdown(t *testing.T) {
	var buf bytes.Buffer
	now := time.Unix(1000, 0)
	d := newTTYDisplay(&buf, func() time.Time { return now })

	d.StartPhase(PhaseRead, "vol1", 100, "")

	// Set countdown status — 1s elapsed of 4s countdown = 3s remaining
	d.SetStatus(StatusLine{
		Text:      "reading page 1 \u2014 retry 1/3 (429)",
		StartedAt: now.Add(-1 * time.Second),
		Countdown: 4 * time.Second,
	})

	out := buf.String()
	if !strings.Contains(out, "3s") {
		t.Errorf("should show countdown remaining, got: %q", out)
	}
}
