package display

import (
	"fmt"
	"io"
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
	Phases      []ProgressRow
	Warnings    []string // warning messages (page N — description)
	Errors      []string // error messages
	LogPath     string
	LogSize     int64 // bytes
}

// RenderStatus writes the status dashboard to w.
func RenderStatus(w io.Writer, data StatusData, colors StatusColors) {
	fmt.Fprintln(w)

	// Book info
	if data.BookTitle != "" {
		title := data.BookTitle
		if data.BookAuthor != "" {
			title += " \u2014 " + data.BookAuthor
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
		fmt.Fprintf(w, "  Langs: %s \u2192 %s\n",
			strings.Join(data.SourceLangs, ", "),
			strings.Join(data.TargetLangs, ", "))
	}
	fmt.Fprintln(w)

	// Phase rows — uses same shared renderer as live dashboard
	for _, row := range data.Phases {
		fmt.Fprintln(w, RenderProgressLine(row, colors))
		if weLine := RenderWarnErrorLine(row.Warnings, row.Failed, colors); weLine != "" {
			fmt.Fprintln(w, weLine)
		}
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
