package display

import (
	"bytes"
	"strings"
	"testing"
	"time"
)

func TestProgressBar(t *testing.T) {
	tests := []struct {
		name      string
		completed int
		total     int
		wantFull  int
		wantEmpty int
	}{
		{"zero", 0, 100, 0, 20},
		{"half", 50, 100, 10, 10},
		{"full", 100, 100, 20, 0},
		{"quarter", 25, 100, 5, 15},
		{"zero total", 0, 0, 0, 20},
		{"over total", 200, 100, 20, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			bar := ProgressBar(tt.completed, tt.total)
			full := strings.Count(bar, "\u2588")
			empty := strings.Count(bar, "\u2591")
			if full != tt.wantFull {
				t.Errorf("full blocks = %d, want %d (bar: %q)", full, tt.wantFull, bar)
			}
			if empty != tt.wantEmpty {
				t.Errorf("empty blocks = %d, want %d (bar: %q)", empty, tt.wantEmpty, bar)
			}
			// Total width should always be barWidth
			if full+empty != barWidth {
				t.Errorf("total width = %d, want %d", full+empty, barWidth)
			}
		})
	}
}

func TestFormatLabel(t *testing.T) {
	tests := []struct {
		phase Phase
		lang  string
		want  string
	}{
		{PhaseRead, "", "        READ"},
		{PhasePages, "", "       PAGES"},
		{PhaseTranslate, "", "       TRANS"},
		{PhaseTranslate, "tr", "  TRANS [tr]"},
		{PhaseWrite, "en", "  WRITE [en]"},
	}

	for _, tt := range tests {
		got := FormatLabel(tt.phase, tt.lang)
		if got != tt.want {
			t.Errorf("FormatLabel(%q, %q) = %q, want %q", tt.phase, tt.lang, got, tt.want)
		}
	}
}

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
	if !strings.Contains(out, "5 entries") {
		t.Errorf("Update output should contain '5 entries', got: %q", out)
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
	d.FinishPhase(PhaseRead, "vol1")
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
	d.FinishPhase(PhaseRead, "vol1")

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
