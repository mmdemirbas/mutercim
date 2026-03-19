package display

import (
	"fmt"
	"io"
	"os"
	"strings"
)

// StatusData holds all data needed to render the status dashboard.
type StatusData struct {
	BookTitle   string
	BookAuthor  string
	InputName   string
	InputPages  int // total pages from images dir
	SourceLangs []string
	TargetLangs []string
	Phases      []PhaseRow
	Warnings    []string // warning messages (page N — description)
	Errors      []string // error messages
	LogPath     string
	LogSize     int64 // bytes
}

// PhaseRow describes one row in the status table.
type PhaseRow struct {
	Name      string // pages, read, solve, translate, write
	Completed int
	Failed    int
	Total     int // 0 means not applicable yet
	Warnings  int
	Lang      string // for translate/write per-lang rows
	Done      bool   // true if phase fully completed
}

// ANSI color codes.
const (
	colorReset  = "\033[0m"
	colorGreen  = "\033[32m"
	colorYellow = "\033[33m"
	colorRed    = "\033[31m"
	colorDim    = "\033[2m"
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

// RenderStatus writes the status dashboard to w.
func RenderStatus(w io.Writer, data StatusData, colors StatusColors) {
	fmt.Fprintln(w)

	// Book info
	if data.BookTitle != "" {
		title := data.BookTitle
		if data.BookAuthor != "" {
			title += " — " + data.BookAuthor
		}
		fmt.Fprintf(w, "  Book:  %s\n", title)
	}
	if data.InputName != "" {
		pages := ""
		if data.InputPages > 0 {
			pages = fmt.Sprintf(" (%d pages)", data.InputPages)
		}
		fmt.Fprintf(w, "  Input: %s%s\n", data.InputName, pages)
	}
	if len(data.SourceLangs) > 0 && len(data.TargetLangs) > 0 {
		fmt.Fprintf(w, "  Langs: %s → %s\n",
			strings.Join(data.SourceLangs, ", "),
			strings.Join(data.TargetLangs, ", "))
	}
	fmt.Fprintln(w)

	// Phase table header
	fmt.Fprintf(w, "  %-18s %-22s %s\n", "Phase", "Progress", "Status")
	fmt.Fprintf(w, "  %-18s %-22s %s\n", "─────", "────────", "──────")

	// Phase rows
	for _, row := range data.Phases {
		renderPhaseRow(w, row, colors)
	}
	fmt.Fprintln(w)

	// Warnings
	fmt.Fprintf(w, "  Warnings: %d\n", len(data.Warnings))
	renderMessages(w, data.Warnings, 10, colors.yellow)
	fmt.Fprintln(w)

	// Errors
	fmt.Fprintf(w, "  Errors: %d\n", len(data.Errors))
	renderMessages(w, data.Errors, 0, colors.red) // show all errors
	fmt.Fprintln(w)

	// Log file info
	if data.LogPath != "" {
		size := formatBytes(data.LogSize)
		fmt.Fprintf(w, "  Log: %s (%s)\n", data.LogPath, size)
		fmt.Fprintln(w)
	}
}

func renderPhaseRow(w io.Writer, row PhaseRow, colors StatusColors) {
	name := row.Name
	if row.Lang != "" {
		name += " [" + row.Lang + "]"
	}

	bar := ProgressBar(row.Completed, row.Total)
	counts := "—"
	if row.Total > 0 {
		counts = fmt.Sprintf("%d/%d", row.Completed, row.Total)
	}

	status := ""
	if row.Done {
		status = colors.green("done")
	} else if row.Total == 0 {
		status = colors.dim("pending")
	} else if row.Completed == 0 && row.Failed == 0 {
		status = colors.dim("pending")
	} else {
		pct := row.Completed * 100 / row.Total
		status = fmt.Sprintf("%d%%", pct)
	}

	if row.Warnings > 0 {
		status += "  " + colors.yellow(fmt.Sprintf("%d warnings", row.Warnings))
	}
	if row.Failed > 0 {
		status += "  " + colors.red(fmt.Sprintf("%d errors", row.Failed))
	}

	// Color the progress bar
	if row.Done {
		bar = colors.green(bar)
	} else if row.Total == 0 || (row.Completed == 0 && row.Failed == 0) {
		bar = colors.dim(bar)
	}

	fmt.Fprintf(w, "  %-18s %s  %-10s %s\n", name, bar, counts, status)
}

func renderMessages(w io.Writer, msgs []string, limit int, colorFn func(string) string) {
	if len(msgs) == 0 {
		return
	}
	show := msgs
	remaining := 0
	if limit > 0 && len(msgs) > limit {
		show = msgs[:limit]
		remaining = len(msgs) - limit
	}
	for _, msg := range show {
		fmt.Fprintf(w, "    %s\n", colorFn(msg))
	}
	if remaining > 0 {
		fmt.Fprintf(w, "    ... and %d more (see reports/)\n", remaining)
	}
}

func formatBytes(b int64) string {
	if b == 0 {
		return "empty"
	}
	if b < 1024 {
		return fmt.Sprintf("%d B", b)
	}
	if b < 1024*1024 {
		return fmt.Sprintf("%.1f KB", float64(b)/1024)
	}
	return fmt.Sprintf("%.1f MB", float64(b)/(1024*1024))
}
