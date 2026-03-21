package display

import (
	"fmt"
	"io"
	"strings"
)

// StatusData holds all data needed to render the status dashboard.
type StatusData struct {
	BookTitle   string
	InputName   string
	InputPages  int // total pages from images dir
	PageRange   string
	SourceLangs []string
	TargetLangs []string
	LayoutTool  string   // configured layout tool name (e.g. "doclayout-yolo", "ai-only")
	ReadModels  []string // ordered model chain (e.g. ["gemini/gemini-2.5-flash-lite", "groq/llama-3.2-90b"])
	TransModels []string // ordered model chain
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
		InputName:   data.InputName,
		InputPages:  data.InputPages,
		PageRange:   data.PageRange,
		SourceLangs: data.SourceLangs,
		TargetLangs: data.TargetLangs,
	}, colors)

	// Config summary — layout tool and model chains
	if data.LayoutTool != "" || len(data.ReadModels) > 0 || len(data.TransModels) > 0 {
		if data.LayoutTool != "" {
			fmt.Fprintf(w, "%s: %s\n", colors.Cyan(fmt.Sprintf("%6s", "Layout")), data.LayoutTool)
		}
		if len(data.ReadModels) > 0 {
			fmt.Fprintf(w, "%s: %s\n", colors.Cyan(fmt.Sprintf("%6s", "Read")),
				strings.Join(data.ReadModels, " \u2192 "))
		}
		if len(data.TransModels) > 0 {
			fmt.Fprintf(w, "%s: %s\n", colors.Cyan(fmt.Sprintf("%6s", "Trans")),
				strings.Join(data.TransModels, " \u2192 "))
		}
		fmt.Fprintln(w)
	}

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
