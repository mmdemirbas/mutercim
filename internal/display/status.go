package display

import (
	"fmt"
	"io"
)

// StatusData holds all data needed to render the status dashboard.
type StatusData struct {
	BookTitle   string
	BookAuthor  string
	InputName   string
	InputPages  int // total pages from images dir
	PageRange   string
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

	// Header — uses same shared renderer as live dashboard
	RenderHeader(w, HeaderData{
		BookTitle:   data.BookTitle,
		BookAuthor:  data.BookAuthor,
		InputName:   data.InputName,
		InputPages:  data.InputPages,
		PageRange:   data.PageRange,
		SourceLangs: data.SourceLangs,
		TargetLangs: data.TargetLangs,
	}, colors)

	// Phase rows — uses same shared renderer as live dashboard
	for _, row := range data.Phases {
		fmt.Fprintln(w, RenderProgressLine(row, colors))
		if weLine := RenderWarnErrorLine(row.Warnings, row.Failed, colors); weLine != "" {
			fmt.Fprintln(w, weLine)
		}
	}
	fmt.Fprintln(w)

	// Warnings
	if len(data.Warnings) > 0 {
		fmt.Fprintf(w, "  %s %s\n", colors.Yellow("\u26a0"), colors.Yellow(fmt.Sprintf("%d warnings", len(data.Warnings))))
		renderMessages(w, data.Warnings, 10, colors.Dim)
		fmt.Fprintln(w)
	}

	// Errors
	if len(data.Errors) > 0 {
		fmt.Fprintf(w, "  %s %s\n", colors.Red("\u2717"), colors.Red(fmt.Sprintf("%d errors", len(data.Errors))))
		renderMessages(w, data.Errors, 0, colors.Dim) // show all errors
		fmt.Fprintln(w)
	}

	// Log file info
	if data.LogPath != "" {
		size := formatBytes(data.LogSize)
		fmt.Fprintf(w, "  %s %s %s\n", colors.Cyan("Log:"), data.LogPath, colors.Dim("("+size+")"))
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
