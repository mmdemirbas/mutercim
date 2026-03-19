package display

import (
	"fmt"
	"io"
	"strings"
	"sync"
)

// LineDisplay writes one line per page to the output writer.
// Used when output is not a TTY (piped, redirected, CI).
type LineDisplay struct {
	out io.Writer
	mu  sync.Mutex
	// Track phase summaries for Finish()
	phases []phaseSummary
}

type phaseSummary struct {
	phase     Phase
	input     string
	lang      string
	total     int
	completed int
	failed    int
	warnings  int
}

func newLineDisplay(out io.Writer) *LineDisplay {
	return &LineDisplay{out: out}
}

// StartPhase records that a phase has started.
func (d *LineDisplay) StartPhase(phase Phase, input string, total int, lang string) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.phases = append(d.phases, phaseSummary{
		phase: phase, input: input, lang: lang, total: total,
	})
}

// Update writes a single line describing the page result.
func (d *LineDisplay) Update(result PageResult) {
	d.mu.Lock()
	defer d.mu.Unlock()

	// Update summary tracking
	if idx := d.findPhase(result.Phase, result.Input); idx >= 0 {
		d.phases[idx].completed = result.Completed
		d.phases[idx].failed = result.Failed
		d.phases[idx].warnings += result.Warnings
	}

	label := formatLineLabel(result.Phase, result.Lang)

	if result.Err != nil {
		fmt.Fprintf(d.out, "[%s] %s page %d/%d FAILED: %v\n",
			label, result.Input, result.PageNum, result.Total, result.Err)
		return
	}

	var details []string
	if result.Entries > 0 {
		details = append(details, fmt.Sprintf("%d entries", result.Entries))
	}
	if result.Footnotes > 0 {
		details = append(details, fmt.Sprintf("%d footnotes", result.Footnotes))
	}

	detail := ""
	if len(details) > 0 {
		detail = " — " + strings.Join(details, ", ")
	}

	fmt.Fprintf(d.out, "[%s] %s %d/%d%s\n",
		label, result.Input, result.Completed, result.Total, detail)
}

// FinishPhase writes a phase completion summary line.
func (d *LineDisplay) FinishPhase(phase Phase, input string) {
	d.mu.Lock()
	defer d.mu.Unlock()

	if idx := d.findPhase(phase, input); idx >= 0 {
		s := d.phases[idx]
		fmt.Fprintf(d.out, "[%s] %s done: %d completed, %d failed, %d warnings\n",
			formatLineLabel(s.phase, s.lang), s.input, s.completed, s.failed, s.warnings)
	}
}

// Finish writes a final summary of all phases.
func (d *LineDisplay) Finish() {
	d.mu.Lock()
	defer d.mu.Unlock()

	for _, s := range d.phases {
		if s.completed > 0 || s.failed > 0 {
			fmt.Fprintf(d.out, "[%s] %s: %d/%d completed, %d failed\n",
				formatLineLabel(s.phase, s.lang), s.input, s.completed, s.total, s.failed)
		}
	}
}

func (d *LineDisplay) findPhase(phase Phase, input string) int {
	for i := len(d.phases) - 1; i >= 0; i-- {
		if d.phases[i].phase == phase && d.phases[i].input == input {
			return i
		}
	}
	return -1
}

func formatLineLabel(phase Phase, lang string) string {
	if lang != "" {
		return string(phase) + ":" + lang
	}
	return string(phase)
}
