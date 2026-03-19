package display

import (
	"fmt"
	"io"
	"strings"
	"sync"
	"time"
)

const barWidth = 20

// TTYDisplay renders live ANSI progress bars to a terminal.
type TTYDisplay struct {
	out          io.Writer
	now          func() time.Time
	mu           sync.Mutex
	phaseOrder   []phaseKey
	phases       map[phaseKey]*phaseState
	currentLines int
}

type phaseKey struct {
	phase Phase
	input string
}

type phaseState struct {
	total     int
	completed int
	failed    int
	warnings  int
	lang      string
	startTime time.Time
	finished  bool
	// Rolling window for rate calculation
	durations [10]time.Duration
	durCount  int
	durIndex  int
	lastTime  time.Time
	// Last page details
	lastPage      int
	lastEntries   int
	lastFootnotes int
	finishTime    time.Time
}

func newTTYDisplay(out io.Writer, nowFunc func() time.Time) *TTYDisplay {
	return &TTYDisplay{
		out:    out,
		now:    nowFunc,
		phases: make(map[phaseKey]*phaseState),
	}
}

// StartPhase begins tracking a new phase.
func (d *TTYDisplay) StartPhase(phase Phase, input string, total int, lang string) {
	d.mu.Lock()
	defer d.mu.Unlock()

	key := phaseKey{phase, input}
	now := d.now()
	d.phases[key] = &phaseState{
		total:     total,
		lang:      lang,
		startTime: now,
		lastTime:  now,
	}
	d.phaseOrder = append(d.phaseOrder, key)
	d.render()
}

// Update records a page result and refreshes the display.
func (d *TTYDisplay) Update(result PageResult) {
	d.mu.Lock()
	defer d.mu.Unlock()

	key := phaseKey{result.Phase, result.Input}
	ps, ok := d.phases[key]
	if !ok {
		return
	}

	now := d.now()

	// Record duration for rate calculation
	dur := now.Sub(ps.lastTime)
	ps.durations[ps.durIndex] = dur
	ps.durIndex = (ps.durIndex + 1) % 10
	if ps.durCount < 10 {
		ps.durCount++
	}
	ps.lastTime = now

	ps.completed = result.Completed
	ps.failed = result.Failed
	ps.warnings += result.Warnings
	ps.lastPage = result.PageNum
	ps.lastEntries = result.Entries
	ps.lastFootnotes = result.Footnotes

	d.render()
}

// FinishPhase collapses the phase to a one-line summary.
func (d *TTYDisplay) FinishPhase(phase Phase, input string) {
	d.mu.Lock()
	defer d.mu.Unlock()

	key := phaseKey{phase, input}
	if ps, ok := d.phases[key]; ok {
		ps.finished = true
		ps.finishTime = d.now()
	}
	d.render()
}

// Finish prints a final summary of all phases.
func (d *TTYDisplay) Finish() {
	d.mu.Lock()
	defer d.mu.Unlock()

	d.clearLines()
	for _, key := range d.phaseOrder {
		ps := d.phases[key]
		label := FormatLabel(key.phase, ps.lang)
		elapsed := ""
		if !ps.finishTime.IsZero() {
			elapsed = "  " + formatDuration(ps.finishTime.Sub(ps.startTime))
		} else if ps.completed > 0 || ps.failed > 0 {
			elapsed = "  " + formatDuration(d.now().Sub(ps.startTime))
		}

		var parts []string
		if ps.warnings > 0 {
			parts = append(parts, fmt.Sprintf("%d warnings", ps.warnings))
		}
		if ps.failed > 0 {
			parts = append(parts, fmt.Sprintf("%d errors", ps.failed))
		}
		suffix := ""
		if len(parts) > 0 {
			suffix = "  " + strings.Join(parts, " \u00b7 ")
		}

		if ps.finished {
			fmt.Fprintf(d.out, "%s  %s %d/%d \u2713%s%s\n",
				label, ProgressBar(ps.completed, ps.total), ps.completed, ps.total, suffix, elapsed)
		} else {
			fmt.Fprintf(d.out, "%s  %s %d/%d%s%s\n",
				label, ProgressBar(ps.completed, ps.total), ps.completed, ps.total, suffix, elapsed)
		}
	}
	d.currentLines = 0
}

func (d *TTYDisplay) render() {
	d.clearLines()

	lines := 0
	for _, key := range d.phaseOrder {
		ps := d.phases[key]
		label := FormatLabel(key.phase, ps.lang)

		if ps.finished {
			elapsed := formatDuration(ps.finishTime.Sub(ps.startTime))
			var parts []string
			if ps.warnings > 0 {
				parts = append(parts, fmt.Sprintf("%d warnings", ps.warnings))
			}
			if ps.failed > 0 {
				parts = append(parts, fmt.Sprintf("%d errors", ps.failed))
			}
			suffix := ""
			if len(parts) > 0 {
				suffix = "  " + strings.Join(parts, " \u00b7 ")
			}
			fmt.Fprintf(d.out, "%s  %s %d/%d \u2713%s  %s\n",
				label, ProgressBar(ps.completed, ps.total), ps.completed, ps.total, suffix, elapsed)
			lines++
			continue
		}

		// Active phase: 3 lines
		pct := ""
		rateStr := ""
		etaStr := ""
		if ps.total > 0 {
			pct = fmt.Sprintf("  %d%%", ps.completed*100/ps.total)
		}
		rate := ps.rate()
		if rate > 0 {
			rateStr = fmt.Sprintf("  %.0fp/min", rate)
			remaining := ps.total - ps.completed
			eta := time.Duration(float64(remaining)/rate*60) * time.Second
			etaStr = fmt.Sprintf("  ETA %s", formatDuration(eta))
		}
		fmt.Fprintf(d.out, "%s  %s %d/%d%s%s%s\n",
			label, ProgressBar(ps.completed, ps.total), ps.completed, ps.total, pct, rateStr, etaStr)
		lines++

		// Detail line
		if ps.lastPage > 0 {
			var details []string
			if ps.lastEntries > 0 {
				details = append(details, fmt.Sprintf("%d entries", ps.lastEntries))
			}
			if ps.lastFootnotes > 0 {
				details = append(details, fmt.Sprintf("%d footnotes", ps.lastFootnotes))
			}
			detail := ""
			if len(details) > 0 {
				detail = " \u2014 " + strings.Join(details, ", ")
			}
			fmt.Fprintf(d.out, "%12s  page %d%s\n", "", ps.lastPage, detail)
			lines++
		}

		// Warnings/errors line
		if ps.warnings > 0 || ps.failed > 0 {
			var parts []string
			if ps.warnings > 0 {
				parts = append(parts, fmt.Sprintf("\u26a0 %d warnings", ps.warnings))
			}
			if ps.failed > 0 {
				parts = append(parts, fmt.Sprintf("\u2717 %d errors", ps.failed))
			}
			fmt.Fprintf(d.out, "%12s  %s\n", "", strings.Join(parts, "  "))
			lines++
		}
	}

	d.currentLines = lines
}

func (d *TTYDisplay) clearLines() {
	for i := 0; i < d.currentLines; i++ {
		fmt.Fprint(d.out, "\033[A\033[2K")
	}
	d.currentLines = 0
}

func (ps *phaseState) rate() float64 {
	n := ps.durCount
	if n == 0 {
		return 0
	}
	var total time.Duration
	for i := 0; i < n; i++ {
		total += ps.durations[i]
	}
	avg := total / time.Duration(n)
	if avg == 0 {
		return 0
	}
	return 60.0 / avg.Seconds()
}

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
