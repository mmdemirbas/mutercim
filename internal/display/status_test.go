package display

import (
	"bytes"
	"fmt"
	"strings"
	"testing"
)

func TestRenderStatus_AllDone(t *testing.T) {
	var buf bytes.Buffer
	data := StatusData{
		InputName:   "book.pdf",
		InputPages:  612,
		SourceLangs: []string{"ar"},
		TargetLangs: []string{"tr"},
		Phases: []ProgressRow{
			{Phase: PhaseCut, Completed: 612, Total: 612, Done: true},
			{Phase: PhaseRead, Completed: 612, Total: 612, Done: true},
			{Phase: PhaseSolve, Completed: 612, Total: 612, Done: true},
			{Phase: PhaseTranslate, Completed: 612, Total: 612, Done: true, Lang: "tr"},
			{Phase: PhaseWrite, Completed: 612, Total: 612, Done: true, Lang: "tr"},
		},
		LogPath: "mutercim.log",
		LogSize: 1258000,
	}

	RenderStatus(&buf, data, StatusColors{Enabled: false})
	out := buf.String()

	for _, want := range []string{
		"book.pdf (612 pages)",
		"ar",
		"tr",
		"612/612",
		"\u2713", // checkmark replaces "done"
		"mutercim.log",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("output should contain %q, got:\n%s", want, out)
		}
	}
}

func TestRenderStatus_Partial(t *testing.T) {
	var buf bytes.Buffer
	data := StatusData{
		InputName:  "vol1.pdf",
		InputPages: 100,
		Phases: []ProgressRow{
			{Phase: PhaseCut, Completed: 100, Total: 100, Done: true},
			{Phase: PhaseRead, Completed: 60, Total: 100, Warnings: 2},
			{Phase: PhaseSolve, Completed: 0, Total: 60},
		},
		Warnings: []string{
			"page 41 — hadith number gap",
			"page 87 — low confidence",
		},
	}

	RenderStatus(&buf, data, StatusColors{Enabled: false})
	out := buf.String()

	if !strings.Contains(out, "60/100") {
		t.Errorf("should show 60/100, got:\n%s", out)
	}
	if !strings.Contains(out, "60%") {
		t.Errorf("should show 60%%, got:\n%s", out)
	}
	if !strings.Contains(out, "2 warnings") {
		t.Errorf("should show 2 warnings, got:\n%s", out)
	}
	if !strings.Contains(out, "2 warnings") {
		t.Errorf("should show 2 warnings, got:\n%s", out)
	}
	if !strings.Contains(out, "page 41") {
		t.Errorf("should show warning details, got:\n%s", out)
	}
}

func TestRenderStatus_EmptyWorkspace(t *testing.T) {
	var buf bytes.Buffer
	data := StatusData{
		Phases: []ProgressRow{
			{Phase: PhaseCut, Total: 0},
			{Phase: PhaseRead, Total: 0},
		},
	}

	RenderStatus(&buf, data, StatusColors{Enabled: false})
	out := buf.String()

	// Pending phases show em-dash for counts (no "pending" text)
	if !strings.Contains(out, "\u2014") {
		t.Errorf("should show em-dash for pending, got:\n%s", out)
	}
}

func TestRenderStatus_WarningTruncation(t *testing.T) {
	var buf bytes.Buffer
	warnings := make([]string, 15)
	for i := range warnings {
		warnings[i] = fmt.Sprintf("page %d — some warning", i+1)
	}
	data := StatusData{
		Warnings: warnings,
		Phases:   []ProgressRow{{Phase: PhaseRead, Completed: 15, Total: 100, Warnings: 15}},
	}

	RenderStatus(&buf, data, StatusColors{Enabled: false})
	out := buf.String()

	if !strings.Contains(out, "... and 5 more") {
		t.Errorf("should truncate to 10 and show '... and 5 more', got:\n%s", out)
	}
	if !strings.Contains(out, "page 10") {
		t.Errorf("should show page 10 (last of first 10), got:\n%s", out)
	}
	if strings.Contains(out, "page 11 —") {
		t.Error("should NOT show page 11 (truncated)")
	}
}

func TestRenderStatus_NoColor(t *testing.T) {
	var buf bytes.Buffer
	data := StatusData{
		Phases: []ProgressRow{
			{Phase: PhaseRead, Completed: 50, Total: 100, Warnings: 1, Done: false},
		},
		Warnings: []string{"page 1 — test warning"},
	}

	// NO_COLOR: no ANSI escape codes
	RenderStatus(&buf, data, StatusColors{Enabled: false})
	out := buf.String()

	if strings.Contains(out, "\033[") {
		t.Errorf("NO_COLOR should produce no ANSI codes, got:\n%q", out)
	}
}

func TestRenderStatus_WithColors(t *testing.T) {
	var buf bytes.Buffer
	data := StatusData{
		Phases: []ProgressRow{
			{Phase: PhaseRead, Completed: 100, Total: 100, Done: true},
			{Phase: PhaseSolve, Total: 0},
		},
	}

	RenderStatus(&buf, data, StatusColors{Enabled: true})
	out := buf.String()

	if !strings.Contains(out, "\033[32m") {
		t.Error("should contain green ANSI code for done phase")
	}
	if !strings.Contains(out, "\033[2m") {
		t.Error("should contain dim ANSI code for pending phase")
	}
}

func TestRenderStatus_TranslatePerLanguage(t *testing.T) {
	var buf bytes.Buffer
	data := StatusData{
		SourceLangs: []string{"ar"},
		TargetLangs: []string{"tr", "en"},
		Phases: []ProgressRow{
			{Phase: PhaseTranslate, Completed: 50, Total: 100, Lang: "tr"},
			{Phase: PhaseTranslate, Completed: 0, Total: 100, Lang: "en"},
		},
	}

	RenderStatus(&buf, data, StatusColors{Enabled: false})
	out := buf.String()

	if !strings.Contains(out, "[tr]") {
		t.Errorf("should show [tr], got:\n%s", out)
	}
	if !strings.Contains(out, "[en]") {
		t.Errorf("should show [en], got:\n%s", out)
	}
}

func TestRenderStatus_TotalCascade(t *testing.T) {
	// Read produced 370/612, so solve total should be 370
	var buf bytes.Buffer
	data := StatusData{
		Phases: []ProgressRow{
			{Phase: PhaseCut, Completed: 612, Total: 612, Done: true},
			{Phase: PhaseRead, Completed: 370, Total: 612, Warnings: 2},
			{Phase: PhaseSolve, Completed: 0, Total: 370},
			{Phase: PhaseTranslate, Completed: 0, Total: 370, Lang: "tr"},
		},
	}

	RenderStatus(&buf, data, StatusColors{Enabled: false})
	out := buf.String()

	// Verify solve shows 0/370 not 0/612
	lines := strings.Split(out, "\n")
	for _, line := range lines {
		if strings.Contains(line, "SOLVE") && strings.Contains(line, "0/370") {
			return // found it
		}
	}
	t.Errorf("solve should show 0/370, got:\n%s", out)
}

func TestRenderStatus_ColumnAlignment(t *testing.T) {
	var buf bytes.Buffer
	data := StatusData{
		Phases: []ProgressRow{
			{Phase: PhaseCut, Completed: 5, Total: 5, Done: true},
			{Phase: PhaseRead, Completed: 3, Total: 5},
			{Phase: PhaseTranslate, Completed: 100, Total: 1000, Lang: "tr"},
		},
	}

	RenderStatus(&buf, data, StatusColors{Enabled: false})
	lines := strings.Split(buf.String(), "\n")

	// Find all phase lines (contain progress bar chars)
	var barPositions []int
	for _, line := range lines {
		if idx := strings.Index(line, "\u2588"); idx >= 0 {
			barPositions = append(barPositions, idx)
		} else if idx := strings.Index(line, "\u2591"); idx >= 0 {
			barPositions = append(barPositions, idx)
		}
	}

	// All bars should start at the same column
	if len(barPositions) < 2 {
		t.Fatalf("expected at least 2 phase lines with bars, got %d", len(barPositions))
	}
	for i := 1; i < len(barPositions); i++ {
		if barPositions[i] != barPositions[0] {
			t.Errorf("bar alignment mismatch: line 0 at col %d, line %d at col %d",
				barPositions[0], i, barPositions[i])
		}
	}
}

func TestRenderStatus_Errors(t *testing.T) {
	var buf bytes.Buffer
	data := StatusData{
		Phases: []ProgressRow{
			{Phase: PhaseRead, Completed: 8, Total: 10, Failed: 2},
		},
		Errors: []string{
			"page 5 — provider timeout after 3 retries",
			"page 9 — invalid JSON response",
		},
	}

	RenderStatus(&buf, data, StatusColors{Enabled: false})
	out := buf.String()

	if !strings.Contains(out, "2 errors") {
		t.Errorf("should show 2 errors, got:\n%s", out)
	}
	if !strings.Contains(out, "page 5") {
		t.Errorf("should show error details, got:\n%s", out)
	}
	if !strings.Contains(out, "page 9") {
		t.Errorf("should show all errors, got:\n%s", out)
	}
	if !strings.Contains(out, "2 errors") {
		t.Errorf("should show '2 errors' in phase row, got:\n%s", out)
	}
}

func TestFormatBytes(t *testing.T) {
	tests := []struct {
		bytes int64
		want  string
	}{
		{0, "empty"},
		{500, "500 B"},
		{1536, "1.5 KB"},
		{1258000, "1.2 MB"},
	}
	for _, tt := range tests {
		got := formatBytes(tt.bytes)
		if got != tt.want {
			t.Errorf("formatBytes(%d) = %q, want %q", tt.bytes, got, tt.want)
		}
	}
}
