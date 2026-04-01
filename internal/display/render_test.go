package display

import (
	"bytes"
	"fmt"
	"strings"
	"testing"
	"time"
)

func TestRenderProgressLine_Done(t *testing.T) {
	row := ProgressRow{
		Phase: PhaseCut, Completed: 600, Total: 600, Done: true,
		Elapsed: 5 * time.Minute,
	}
	line := RenderProgressLine(row, StatusColors{Enabled: false})

	for _, want := range []string{"CUT", "600/600", "\u2713", "5m"} {
		if !strings.Contains(line, want) {
			t.Errorf("done line should contain %q, got: %q", want, line)
		}
	}
}

func TestRenderProgressLine_Active_WithRateETA(t *testing.T) {
	row := ProgressRow{
		Phase: PhaseRead, Completed: 247, Total: 600,
		Rate: 14.0, ETA: 25 * time.Minute,
	}
	line := RenderProgressLine(row, StatusColors{Enabled: false})

	for _, want := range []string{"READ", "247/600", "41%", "14p/min", "ETA 25m"} {
		if !strings.Contains(line, want) {
			t.Errorf("active line should contain %q, got: %q", want, line)
		}
	}
}

func TestRenderProgressLine_Pending(t *testing.T) {
	row := ProgressRow{Phase: PhaseSolve, Total: 0}
	line := RenderProgressLine(row, StatusColors{Enabled: false})

	if !strings.Contains(line, "SOLVE") {
		t.Errorf("pending line should contain SOLVE, got: %q", line)
	}
	if !strings.Contains(line, "\u2014") {
		t.Errorf("pending line should contain em-dash, got: %q", line)
	}
	// Should not contain percentage or checkmark
	if strings.Contains(line, "%") || strings.Contains(line, "\u2713") {
		t.Errorf("pending line should not contain %% or checkmark, got: %q", line)
	}
}

func TestRenderProgressLine_PendingWithTotal(t *testing.T) {
	row := ProgressRow{Phase: PhaseRead, Total: 100}
	line := RenderProgressLine(row, StatusColors{Enabled: false})

	if !strings.Contains(line, "0/100") {
		t.Errorf("pending line should contain 0/100, got: %q", line)
	}
	// No percentage for 0 completed + 0 failed
	if strings.Contains(line, "0%") {
		t.Errorf("pending line should not contain 0%%, got: %q", line)
	}
}

func TestRenderProgressLine_NoColor(t *testing.T) {
	row := ProgressRow{
		Phase: PhaseRead, Completed: 100, Total: 100, Done: true,
	}
	line := RenderProgressLine(row, StatusColors{Enabled: false})

	if strings.Contains(line, "\033[") {
		t.Errorf("no-color line should not contain ANSI codes, got: %q", line)
	}
}

func TestRenderProgressLine_WithColors(t *testing.T) {
	row := ProgressRow{
		Phase: PhaseRead, Completed: 100, Total: 100, Done: true,
	}
	line := RenderProgressLine(row, StatusColors{Enabled: true})

	if !strings.Contains(line, colorGreen) {
		t.Errorf("colored done line should contain green ANSI code, got: %q", line)
	}
}

func TestRenderProgressLine_DimPending(t *testing.T) {
	row := ProgressRow{Phase: PhaseSolve, Total: 0}
	line := RenderProgressLine(row, StatusColors{Enabled: true})

	if !strings.Contains(line, colorDim) {
		t.Errorf("colored pending line should contain dim ANSI code, got: %q", line)
	}
}

func TestRenderWarnErrorLine_Both(t *testing.T) {
	line := RenderWarnErrorLine(3, 2, StatusColors{Enabled: false})

	if !strings.Contains(line, "3 warnings") {
		t.Errorf("should contain '3 warnings', got: %q", line)
	}
	if !strings.Contains(line, "2 errors") {
		t.Errorf("should contain '2 errors', got: %q", line)
	}
	if !strings.Contains(line, "\u26a0") {
		t.Errorf("should contain ⚠, got: %q", line)
	}
	if !strings.Contains(line, "\u2717") {
		t.Errorf("should contain ✗, got: %q", line)
	}
}

func TestRenderWarnErrorLine_WarningsOnly(t *testing.T) {
	line := RenderWarnErrorLine(5, 0, StatusColors{Enabled: false})

	if !strings.Contains(line, "5 warnings") {
		t.Errorf("should contain '5 warnings', got: %q", line)
	}
	if strings.Contains(line, "errors") {
		t.Errorf("should not contain 'errors', got: %q", line)
	}
}

func TestRenderWarnErrorLine_ErrorsOnly(t *testing.T) {
	line := RenderWarnErrorLine(0, 3, StatusColors{Enabled: false})

	if strings.Contains(line, "warnings") {
		t.Errorf("should not contain 'warnings', got: %q", line)
	}
	if !strings.Contains(line, "3 errors") {
		t.Errorf("should contain '3 errors', got: %q", line)
	}
}

func TestRenderWarnErrorLine_None(t *testing.T) {
	line := RenderWarnErrorLine(0, 0, StatusColors{Enabled: false})

	if line != "" {
		t.Errorf("no warnings/errors should produce empty string, got: %q", line)
	}
}

func TestRenderWarnErrorLine_Colored(t *testing.T) {
	line := RenderWarnErrorLine(1, 1, StatusColors{Enabled: true})

	if !strings.Contains(line, colorYellow) {
		t.Errorf("should contain yellow for warnings, got: %q", line)
	}
	if !strings.Contains(line, colorRed) {
		t.Errorf("should contain red for errors, got: %q", line)
	}
}

func TestSharedRender_StatusAndLiveProduceSameProgressSection(t *testing.T) {
	colors := StatusColors{Enabled: false}

	// Data that both status command and live dashboard would have
	testRows := []ProgressRow{
		{Phase: PhaseCut, Completed: 600, Total: 600, Done: true},
		{Phase: PhaseRead, Completed: 247, Total: 600, Warnings: 2, Failed: 1},
		{Phase: PhaseSolve, Total: 0}, // pending
	}

	// Simulate status command rendering
	var statusBuf bytes.Buffer
	for _, row := range testRows {
		_, _ = fmt.Fprintln(&statusBuf, RenderProgressLine(row, colors))
		if weLine := RenderWarnErrorLine(row.Warnings, row.Failed, colors); weLine != "" {
			_, _ = fmt.Fprintln(&statusBuf, weLine)
		}
	}

	// Simulate live dashboard rendering (same code path)
	var liveBuf bytes.Buffer
	for _, row := range testRows {
		fmt.Fprintln(&liveBuf, RenderProgressLine(row, colors))
		if weLine := RenderWarnErrorLine(row.Warnings, row.Failed, colors); weLine != "" {
			fmt.Fprintln(&liveBuf, weLine)
		}
	}

	if statusBuf.String() != liveBuf.String() {
		t.Errorf("status and live should produce identical progress sections:\nstatus:\n%s\nlive:\n%s",
			statusBuf.String(), liveBuf.String())
	}

	// Verify expected content
	out := statusBuf.String()
	for _, want := range []string{"CUT", "READ", "SOLVE", "600/600", "247/600", "\u2713", "\u2014"} {
		if !strings.Contains(out, want) {
			t.Errorf("output should contain %q, got:\n%s", want, out)
		}
	}
}

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
		{PhaseCut, "", "         CUT"},
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

func TestFormatDuration(t *testing.T) {
	tests := []struct {
		d    time.Duration
		want string
	}{
		{30 * time.Second, "30s"},
		{5 * time.Minute, "5m"},
		{2*time.Hour + 30*time.Minute, "2.5h"},
	}
	for _, tt := range tests {
		got := formatDuration(tt.d)
		if got != tt.want {
			t.Errorf("formatDuration(%v) = %q, want %q", tt.d, got, tt.want)
		}
	}
}

func TestRenderHeader_ColonAlignment(t *testing.T) {
	var buf bytes.Buffer
	RenderHeader(&buf, HeaderData{
		Inputs:     []string{"/workspace/input/vol1.pdf"},
		InputPages: 100,
	}, StatusColors{Enabled: false})

	lines := strings.Split(buf.String(), "\n")
	// Find colon positions in non-empty lines
	var colonPositions []int
	for _, line := range lines {
		if idx := strings.Index(line, ":"); idx >= 0 && strings.TrimSpace(line) != "" {
			colonPositions = append(colonPositions, idx)
		}
	}
	if len(colonPositions) < 1 {
		t.Fatalf("expected at least 1 line with colons, got %d", len(colonPositions))
	}
	for i := 1; i < len(colonPositions); i++ {
		if colonPositions[i] != colonPositions[0] {
			t.Errorf("colon alignment mismatch: line 0 at col %d, line %d at col %d\noutput:\n%s",
				colonPositions[0], i, colonPositions[i], buf.String())
		}
	}
}

func TestRenderHeader_WithPageRange(t *testing.T) {
	var buf bytes.Buffer
	RenderHeader(&buf, HeaderData{
		Inputs:     []string{"/workspace/input/vol1.pdf"},
		InputPages: 612,
		PageRange:  "1-50",
	}, StatusColors{Enabled: false})

	out := buf.String()
	if !strings.Contains(out, "pages 1-50 of 612") {
		t.Errorf("should show page range, got:\n%s", out)
	}
}

func TestRenderHeader_AllPagesNoRange(t *testing.T) {
	var buf bytes.Buffer
	RenderHeader(&buf, HeaderData{
		Inputs:     []string{"/workspace/input/vol1.pdf"},
		InputPages: 612,
	}, StatusColors{Enabled: false})

	out := buf.String()
	if !strings.Contains(out, "612 pages") {
		t.Errorf("should show total pages when no range, got:\n%s", out)
	}
	if strings.Contains(out, "of 612") {
		t.Errorf("should not show 'of N' format when no range, got:\n%s", out)
	}
}

func TestRenderHeader_Empty(t *testing.T) {
	var buf bytes.Buffer
	lines := RenderHeader(&buf, HeaderData{}, StatusColors{Enabled: false})

	if lines != 0 {
		t.Errorf("empty header should produce 0 lines, got %d", lines)
	}
	if buf.Len() != 0 {
		t.Errorf("empty header should produce no output, got: %q", buf.String())
	}
}

func TestFormatStatusLine_InProgress(t *testing.T) {
	line := FormatStatusLine("reading page 248 via gemini/gemini-2.5-flash-lite", 4200*time.Millisecond, 0, StatusColors{Enabled: false})

	if !strings.Contains(line, "reading page 248") {
		t.Errorf("should contain operation text, got: %q", line)
	}
	if !strings.Contains(line, "4.2s") {
		t.Errorf("should show elapsed with one decimal, got: %q", line)
	}
	if !strings.Contains(line, "\u2192") {
		t.Errorf("should contain arrow symbol, got: %q", line)
	}
}

func TestFormatStatusLine_Retry(t *testing.T) {
	line := FormatStatusLine("reading page 248 via gemini \u2014 retry 2/3 (429)", 6100*time.Millisecond, 0, StatusColors{Enabled: false})

	if !strings.Contains(line, "retry 2/3") {
		t.Errorf("should contain retry info, got: %q", line)
	}
	if !strings.Contains(line, "6.1s") {
		t.Errorf("should show elapsed, got: %q", line)
	}
}

func TestFormatStatusLine_Failover(t *testing.T) {
	line := FormatStatusLine("reading page 248 via groq/llama \u2014 failover from gemini", 1300*time.Millisecond, 0, StatusColors{Enabled: false})

	if !strings.Contains(line, "failover from gemini") {
		t.Errorf("should contain failover info, got: %q", line)
	}
	if !strings.Contains(line, "1.3s") {
		t.Errorf("should show elapsed, got: %q", line)
	}
}

func TestFormatStatusLine_Backoff_Countdown(t *testing.T) {
	// 1 second elapsed out of 4 second countdown → 3s remaining
	line := FormatStatusLine("reading page 248 \u2014 retry 1/3 (429)", 1*time.Second, 4*time.Second, StatusColors{Enabled: false})

	if !strings.Contains(line, "3s") {
		t.Errorf("should show 3s remaining, got: %q", line)
	}
	// Should NOT show decimal for countdown
	if strings.Contains(line, "3.0s") {
		t.Errorf("countdown should use %%0.fs (no decimal), got: %q", line)
	}
}

func TestFormatStatusLine_Backoff_Expired(t *testing.T) {
	// Elapsed exceeds countdown → show 0s
	line := FormatStatusLine("waiting", 10*time.Second, 4*time.Second, StatusColors{Enabled: false})

	if !strings.Contains(line, "0s") {
		t.Errorf("expired countdown should show 0s, got: %q", line)
	}
}

func TestRenderHeader_LineCount(t *testing.T) {
	var buf bytes.Buffer
	lines := RenderHeader(&buf, HeaderData{
		Inputs:     []string{"/workspace/input/input.pdf"},
		InputPages: 10,
	}, StatusColors{Enabled: false})

	// 1 content line + 1 blank separator = 2
	if lines != 2 {
		t.Errorf("expected 2 lines (1 content + 1 blank), got %d\noutput:\n%s", lines, buf.String())
	}
}
