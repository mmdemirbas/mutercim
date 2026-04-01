package display

import (
	"fmt"
	"io"
	"os"
	"strings"
	"time"
)

const barWidth = 20

// PhaseConfig holds static config info for a pipeline phase (shown in dashboard).
type PhaseConfig struct {
	Phase    Phase
	Info     string   // brief info on the progress line (e.g. tool name)
	SubItems []string // numbered list below the progress line (e.g. model chain)
}

// HeaderData holds metadata shown in the header section of both live and status displays.
// All paths (OutputDir, Inputs, Knowledge) should be absolute.
type HeaderData struct {
	InputPages   int           // total pages available
	PageRange    string        // e.g. "1-50", empty means all
	LogLevel     string        // effective log level
	OutputDir    string        // output directory (absolute)
	Inputs       []string      // input paths (absolute)
	Knowledge    []string      // knowledge paths (absolute)
	PhaseConfigs []PhaseConfig // per-phase config summaries
}

// RenderHeader writes the header section and returns the number of lines written.
// Order: log level, output, inputs, knowledge.
func RenderHeader(w io.Writer, h HeaderData, colors StatusColors) int {
	lines := 0
	if h.LogLevel != "" && h.LogLevel != "info" {
		fmt.Fprintf(w, "%s: %s\n", colors.Cyan(fmt.Sprintf("%8s", "Log")), h.LogLevel)
		lines++
	}
	if h.OutputDir != "" {
		fmt.Fprintf(w, "%s: %s\n", colors.Cyan(fmt.Sprintf("%8s", "Output")), h.OutputDir)
		lines++
	}
	if len(h.Inputs) > 0 {
		for i, inp := range h.Inputs {
			label := "Input"
			if len(h.Inputs) > 1 {
				label = fmt.Sprintf("Input %d", i+1)
			}
			info := inp
			// Show page info only on the first (or only) input
			if i == 0 {
				if h.PageRange != "" && h.InputPages > 0 {
					info += colors.dim(fmt.Sprintf(" (pages %s of %d)", h.PageRange, h.InputPages))
				} else if h.InputPages > 0 {
					info += colors.dim(fmt.Sprintf(" (%d pages)", h.InputPages))
				}
			}
			fmt.Fprintf(w, "%s: %s\n", colors.Cyan(fmt.Sprintf("%8s", label)), info)
			lines++
		}
	}
	if len(h.Knowledge) > 0 {
		fmt.Fprintf(w, "%s: %s\n", colors.Cyan(fmt.Sprintf("%8s", "Know")), strings.Join(h.Knowledge, ", "))
		lines++
	}
	if lines > 0 {
		_, _ = fmt.Fprintln(w)
		lines++
	}
	return lines
}

// ProgressRow holds the data needed to render one phase progress line.
// Used by both the live dashboard and the status command.
type ProgressRow struct {
	Phase      Phase
	Lang       string
	Completed  int
	Failed     int
	Total      int
	Warnings   int
	Done       bool
	Skipped    bool          // phase was skipped entirely (e.g. layout tool disabled)
	SkipReason string        // reason shown when Skipped (e.g. "no tool configured")
	Info       string        // brief info shown after progress (e.g. tool name, target langs)
	SubItems   []string      // numbered list shown below the progress line (e.g. model chain)
	Rate       float64       // pages/min, 0 = not available
	ETA        time.Duration // 0 = not available
	Elapsed    time.Duration // for finished phases
}

// ANSI color codes.
const (
	colorReset  = "\033[0m"
	colorBold   = "\033[1m"
	colorDim    = "\033[2m"
	colorGreen  = "\033[32m"
	colorYellow = "\033[33m"
	colorRed    = "\033[31m"
	colorCyan   = "\033[36m"
)

// StatusColors controls whether ANSI colors are used.
type StatusColors struct {
	Enabled bool
}

// NewStatusColors returns colors enabled if out is a TTY and NO_COLOR is not set.
func NewStatusColors(out io.Writer) StatusColors {
	if os.Getenv("NO_COLOR") != "" {
		return StatusColors{Enabled: false}
	}
	return StatusColors{Enabled: IsTTY(out)}
}

func (c StatusColors) green(s string) string {
	if !c.Enabled {
		return s
	}
	return colorGreen + s + colorReset
}

func (c StatusColors) yellow(s string) string {
	if !c.Enabled {
		return s
	}
	return colorYellow + s + colorReset
}

func (c StatusColors) red(s string) string {
	if !c.Enabled {
		return s
	}
	return colorRed + s + colorReset
}

func (c StatusColors) dim(s string) string {
	if !c.Enabled {
		return s
	}
	return colorDim + s + colorReset
}

// Bold returns bold text.
func (c StatusColors) Bold(s string) string {
	if !c.Enabled {
		return s
	}
	return colorBold + s + colorReset
}

// Cyan returns cyan-colored text (used for labels and headers).
func (c StatusColors) Cyan(s string) string {
	if !c.Enabled {
		return s
	}
	return colorCyan + s + colorReset
}

// Green returns green-colored text (exported for use outside display package).
func (c StatusColors) Green(s string) string { return c.green(s) }

// Yellow returns yellow-colored text (exported for use outside display package).
func (c StatusColors) Yellow(s string) string { return c.yellow(s) }

// Red returns red-colored text (exported for use outside display package).
func (c StatusColors) Red(s string) string { return c.red(s) }

// Dim returns dim text (exported for use outside display package).
func (c StatusColors) Dim(s string) string { return c.dim(s) }

// ProgressBar renders a progress bar of barWidth characters.
func ProgressBar(completed, total int) string {
	if total <= 0 {
		return strings.Repeat("\u2591", barWidth)
	}
	filled := completed * barWidth / total
	if filled > barWidth {
		filled = barWidth
	}
	return strings.Repeat("\u2588", filled) + strings.Repeat("\u2591", barWidth-filled)
}

// FormatLabel right-aligns a phase label with optional language tag.
func FormatLabel(phase Phase, lang string) string {
	label := string(phase)
	if lang != "" {
		label += " [" + lang + "]"
	}
	// Width 12 accommodates "TRANS [tr]" with padding
	return fmt.Sprintf("%12s", label)
}

func formatDuration(d time.Duration) string {
	if d < time.Minute {
		return fmt.Sprintf("%.0fs", d.Seconds())
	}
	if d < time.Hour {
		return fmt.Sprintf("%.0fm", d.Minutes())
	}
	return fmt.Sprintf("%.1fh", d.Hours())
}

// RenderProgressLine formats one phase progress line.
// Both the live dashboard and status command use this for consistent output.
func RenderProgressLine(row ProgressRow, colors StatusColors) string {
	label := FormatLabel(row.Phase, row.Lang)

	// Skipped phase — show dashes instead of progress bar
	if row.Skipped {
		dashes := strings.Repeat("\u2500", barWidth)
		reason := row.SkipReason
		if reason == "" {
			reason = "skipped"
		}
		return fmt.Sprintf("  %s  %s  %s", label, colors.dim(dashes), colors.dim(reason))
	}

	bar := ProgressBar(row.Completed, row.Total)

	// Color the bar
	if row.Done {
		bar = colors.green(bar)
	} else if row.Total == 0 || (row.Completed == 0 && row.Failed == 0) {
		bar = colors.dim(bar)
	}

	// Counts
	counts := "\u2014" // em-dash for pending
	if row.Total > 0 {
		counts = fmt.Sprintf("%d/%d", row.Completed, row.Total)
	}

	// Build status parts
	var parts []string
	if row.Done {
		parts = append(parts, colors.green("\u2713"))
	} else if row.Total > 0 && (row.Completed > 0 || row.Failed > 0) {
		pct := row.Completed * 100 / row.Total
		parts = append(parts, fmt.Sprintf("%d%%", pct))
	}

	if row.Rate > 0 {
		parts = append(parts, fmt.Sprintf("%.0fp/min", row.Rate))
	}
	if row.ETA > 0 {
		parts = append(parts, "ETA "+formatDuration(row.ETA))
	}
	if row.Elapsed > 0 {
		parts = append(parts, formatDuration(row.Elapsed))
	}

	suffix := ""
	if len(parts) > 0 {
		suffix = "  " + strings.Join(parts, "  ")
	}

	line := fmt.Sprintf("%s  %s %s%s", label, bar, counts, suffix)
	if row.Info != "" {
		line += "  " + colors.dim(row.Info)
	}
	return line
}

// RenderSubItems formats the numbered list below a progress row.
func RenderSubItems(items []string, colors StatusColors) []string {
	var lines []string
	for i, item := range items {
		lines = append(lines, fmt.Sprintf("%12s  %s %s",
			"", colors.dim(fmt.Sprintf("%d.", i+1)), item))
	}
	return lines
}

// FormatStatusLine renders the in-progress status line below a phase bar.
// elapsed is the time since the operation started.
// If countdown > 0, shows remaining time instead of elapsed.
func FormatStatusLine(text string, elapsed time.Duration, countdown time.Duration, colors StatusColors) string {
	timeStr := ""
	if countdown > 0 {
		remaining := countdown - elapsed
		if remaining < 0 {
			remaining = 0
		}
		timeStr = fmt.Sprintf("%.0fs", remaining.Seconds())
	} else {
		timeStr = fmt.Sprintf("%.1fs", elapsed.Seconds())
	}
	return fmt.Sprintf("%12s  %s %s ... %s", "", colors.dim("\u2192"), text, timeStr)
}

// RenderWarnErrorLine formats the ⚠/✗ detail line below a progress row.
// Returns empty string if there are no warnings or errors.
func RenderWarnErrorLine(warnings, failed int, colors StatusColors) string {
	if warnings == 0 && failed == 0 {
		return ""
	}
	var parts []string
	if warnings > 0 {
		parts = append(parts, colors.yellow(fmt.Sprintf("\u26a0 %d warnings", warnings)))
	}
	if failed > 0 {
		parts = append(parts, colors.red(fmt.Sprintf("\u2717 %d errors", failed)))
	}
	return fmt.Sprintf("%12s  %s", "", strings.Join(parts, "  "))
}
