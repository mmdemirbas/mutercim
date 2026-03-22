package display

import (
	"context"
	"io"
	"os"
	"time"
)

// Phase identifies a pipeline phase for display purposes.
type Phase string

const (
	PhaseCut       Phase = "CUT"
	PhaseLayout    Phase = "LAYOUT"
	PhaseRead      Phase = "READ"
	PhaseSolve     Phase = "SOLVE"
	PhaseTranslate Phase = "TRANS"
	PhaseWrite     Phase = "WRITE"
)

// PageResult describes the outcome of processing one page.
type PageResult struct {
	Phase         Phase
	PageNum       int
	Total         int
	Completed     int
	Failed        int
	Warnings      int
	Entries       int
	Footnotes     int
	Lang          string // target language code (translate/write phases)
	Input         string // input stem
	Err           error  // non-nil if this page failed
	LayoutTool    string // "doclayout-yolo", "surya", or "ai-only" (read phase only)
	LayoutMs      int    // milliseconds spent in layout detection
	LayoutRegions int    // number of regions the layout tool detected
	LayoutError   string // non-empty if layout tool failed and fell back to ai-only
}

// StatusLine describes an in-progress operation shown below the active phase bar.
type StatusLine struct {
	Text      string        // e.g. "reading page 248 via gemini/gemini-2.5-flash-lite"
	StartedAt time.Time     // when the operation started (for elapsed timer)
	Countdown time.Duration // if > 0, show countdown instead of elapsed
}

// Display controls terminal progress output.
type Display interface {
	// SetHeader sets the metadata header (book, input, langs) shown above progress bars.
	SetHeader(header HeaderData)
	// SetStatus sets the in-progress status line below the active phase bar.
	// Pass empty StatusLine to clear. The elapsed timer updates automatically.
	SetStatus(status StatusLine)
	// StartPhase begins tracking a new phase.
	StartPhase(phase Phase, input string, total int, lang string)
	// Update records one page's result and refreshes the display.
	Update(result PageResult)
	// FinishPhase marks a phase as complete and collapses it to a summary line.
	FinishPhase(phase Phase, input string, lang string)
	// Finish prints a final summary. Called on normal exit or Ctrl+C.
	Finish()
}

// New creates a Display appropriate for the output writer.
// If out is a TTY, returns a live ANSI progress display.
// Otherwise returns a line-per-page display.
// nowFunc is injectable for testing; pass nil for time.Now.
func New(out io.Writer, nowFunc func() time.Time) Display {
	if nowFunc == nil {
		nowFunc = time.Now
	}
	if IsTTY(out) {
		return newTTYDisplay(out, nowFunc)
	}
	return newLineDisplay(out)
}

// IsTTY returns true if the writer is a terminal.
func IsTTY(w io.Writer) bool {
	f, ok := w.(*os.File)
	if !ok {
		return false
	}
	stat, err := f.Stat()
	if err != nil {
		return false
	}
	return (stat.Mode() & os.ModeCharDevice) != 0
}

type contextKey struct{}

// WithDisplay stores a Display on the context.
func WithDisplay(ctx context.Context, d Display) context.Context {
	return context.WithValue(ctx, contextKey{}, d)
}

// FromContext retrieves the Display from the context, or nil.
func FromContext(ctx context.Context) Display {
	d, _ := ctx.Value(contextKey{}).(Display)
	return d
}
