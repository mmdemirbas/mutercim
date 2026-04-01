package display

import (
	"fmt"
	"io"
	"strings"
	"sync"
	"time"
)

// TTYDisplay renders live ANSI progress bars to a terminal.
type TTYDisplay struct {
	out          io.Writer
	now          func() time.Time
	mu           sync.Mutex
	phaseOrder   []phaseKey
	phases       map[phaseKey]*phaseState
	currentLines int
	colors       StatusColors
	logBuffer    *RingBuffer
	header       HeaderData
	status       StatusLine
	statusStop   chan struct{}
}

type phaseKey struct {
	phase Phase
	input string
	lang  string
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
	durations  [10]time.Duration
	durCount   int
	durIndex   int
	lastTime   time.Time
	finishTime time.Time
}

func newTTYDisplay(out io.Writer, nowFunc func() time.Time) *TTYDisplay {
	return &TTYDisplay{
		out:       out,
		now:       nowFunc,
		phases:    make(map[phaseKey]*phaseState),
		colors:    NewStatusColors(out),
		logBuffer: NewRingBuffer(5),
	}
}

// SetStatus sets the in-progress status line below the active phase bar.
// Pass empty StatusLine to clear. Starts a 1-second ticker for live elapsed updates.
func (d *TTYDisplay) SetStatus(status StatusLine) {
	d.mu.Lock()
	defer d.mu.Unlock()

	wasActive := d.status.Text != ""
	d.status = status
	isActive := d.status.Text != ""

	if isActive && !wasActive {
		d.startStatusTicker()
	} else if !isActive && wasActive {
		d.stopStatusTicker()
	}

	d.render()
}

func (d *TTYDisplay) startStatusTicker() {
	stop := make(chan struct{})
	d.statusStop = stop
	go func() {
		ticker := time.NewTicker(time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				d.mu.Lock()
				if d.status.Text != "" {
					d.render()
				}
				d.mu.Unlock()
			case <-stop:
				return
			}
		}
	}()
}

func (d *TTYDisplay) stopStatusTicker() {
	if d.statusStop != nil {
		close(d.statusStop)
		d.statusStop = nil
	}
}

// SetHeader sets the metadata header shown above progress bars.
func (d *TTYDisplay) SetHeader(header HeaderData) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.header = header
	d.render()
}

// StartPhase begins tracking a new phase.
func (d *TTYDisplay) StartPhase(phase Phase, input string, total int, lang string) {
	d.mu.Lock()
	defer d.mu.Unlock()

	key := phaseKey{phase, input, lang}
	now := d.now()
	if _, exists := d.phases[key]; exists {
		// Phase already registered (e.g. re-run), just update total
		d.phases[key].total = total
		d.render()
		return
	}
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

	key := phaseKey{result.Phase, result.Input, result.Lang}
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

	// Push log entry
	d.logBuffer.Push(logEntryFromResult(result, now))

	d.render()
}

// FinishPhase collapses the phase to a one-line summary.
func (d *TTYDisplay) FinishPhase(phase Phase, input string, lang string) {
	d.mu.Lock()
	defer d.mu.Unlock()

	key := phaseKey{phase, input, lang}
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

	d.stopStatusTicker()
	d.status = StatusLine{}

	var buf strings.Builder
	if d.currentLines > 0 {
		fmt.Fprintf(&buf, "\033[%dA", d.currentLines)
		buf.WriteString("\033[J")
	}

	renderHeaderTo(&buf, d.header, d.colors)
	for _, key := range d.phaseOrder {
		ps := d.phases[key]
		row := d.buildFinishRow(key, ps)
		fmt.Fprintln(&buf, RenderProgressLine(row, d.colors))
		if weLine := RenderWarnErrorLine(ps.warnings, ps.failed, d.colors); weLine != "" {
			fmt.Fprintln(&buf, weLine)
		}
	}

	io.WriteString(d.out, buf.String())
	d.currentLines = 0
}

func (d *TTYDisplay) render() {
	// Build the entire frame into a buffer, then write it as a single
	// operation. This prevents partial writes from corrupting the display
	// when the terminal processes escape sequences between small writes.
	var buf strings.Builder

	// Clear previous output
	if d.currentLines > 0 {
		fmt.Fprintf(&buf, "\033[%dA", d.currentLines)
		buf.WriteString("\033[J")
	}

	lines := renderHeaderTo(&buf, d.header, d.colors)
	statusRendered := false
	for _, key := range d.phaseOrder {
		ps := d.phases[key]
		row := d.buildLiveRow(key, ps)
		fmt.Fprintln(&buf, RenderProgressLine(row, d.colors))
		lines++

		// Sub-items (model list, etc.) — show only before phase starts or when finished
		if ps.completed == 0 && ps.failed == 0 && !ps.finished {
			for _, sub := range RenderSubItems(row.SubItems, d.colors) {
				fmt.Fprintln(&buf, sub)
				lines++
			}
		}

		// Status line under the active (non-finished) phase
		if !statusRendered && !ps.finished && d.status.Text != "" {
			elapsed := d.now().Sub(d.status.StartedAt)
			fmt.Fprintln(&buf, FormatStatusLine(d.status.Text, elapsed, d.status.Countdown, d.colors))
			lines++
			statusRendered = true
		}

		if weLine := RenderWarnErrorLine(ps.warnings, ps.failed, d.colors); weLine != "" {
			fmt.Fprintln(&buf, weLine)
			lines++
		}
	}

	// Log tail
	entries := d.logBuffer.Entries()
	if len(entries) > 0 {
		fmt.Fprintln(&buf)
		fmt.Fprintln(&buf, "  \u2500\u2500\u2500 recent \u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500")
		lines += 2
		for _, e := range entries {
			fmt.Fprintf(&buf, "  %s\n", FormatLogEntry(e, 80, d.colors))
			lines++
		}
	}

	d.currentLines = lines

	// Single atomic write to the terminal
	io.WriteString(d.out, buf.String())
}

// renderHeaderTo writes the header to a buffer and returns the line count.
// Mirrors RenderHeader but writes to a strings.Builder.
func renderHeaderTo(buf *strings.Builder, h HeaderData, colors StatusColors) int {
	lines := 0
	if h.LogLevel != "" && h.LogLevel != "info" {
		fmt.Fprintf(buf, "%s: %s\n", colors.Cyan(fmt.Sprintf("%8s", "Log")), h.LogLevel)
		lines++
	}
	if h.OutputDir != "" {
		fmt.Fprintf(buf, "%s: %s\n", colors.Cyan(fmt.Sprintf("%8s", "Output")), h.OutputDir)
		lines++
	}
	if len(h.Inputs) > 0 {
		for i, inp := range h.Inputs {
			label := "Input"
			if len(h.Inputs) > 1 {
				label = fmt.Sprintf("Input %d", i+1)
			}
			info := inp
			if i == 0 {
				if h.PageRange != "" && h.InputPages > 0 {
					info += colors.dim(fmt.Sprintf(" (pages %s of %d)", h.PageRange, h.InputPages))
				} else if h.InputPages > 0 {
					info += colors.dim(fmt.Sprintf(" (%d pages)", h.InputPages))
				}
			}
			fmt.Fprintf(buf, "%s: %s\n", colors.Cyan(fmt.Sprintf("%8s", label)), info)
			lines++
		}
	}
	if len(h.Knowledge) > 0 {
		fmt.Fprintf(buf, "%s: %s\n", colors.Cyan(fmt.Sprintf("%8s", "Know")), strings.Join(h.Knowledge, ", "))
		lines++
	}
	if lines > 0 {
		fmt.Fprintln(buf)
		lines++
	}
	return lines
}

// phaseConfigFor returns the PhaseConfig matching the given phase, if any.
func (d *TTYDisplay) phaseConfigFor(phase Phase) *PhaseConfig {
	for i := range d.header.PhaseConfigs {
		if d.header.PhaseConfigs[i].Phase == phase {
			return &d.header.PhaseConfigs[i]
		}
	}
	return nil
}

// buildLiveRow creates a ProgressRow for live rendering (shows rate/ETA for active phases).
func (d *TTYDisplay) buildLiveRow(key phaseKey, ps *phaseState) ProgressRow {
	row := ProgressRow{
		Phase:     key.phase,
		Lang:      ps.lang,
		Completed: ps.completed,
		Failed:    ps.failed,
		Total:     ps.total,
		Warnings:  ps.warnings,
		Done:      ps.finished,
	}

	// Add per-phase config info from header
	if pc := d.phaseConfigFor(key.phase); pc != nil {
		row.Info = pc.Info
		row.SubItems = pc.SubItems
	}

	if ps.finished {
		row.Elapsed = ps.finishTime.Sub(ps.startTime)
	} else {
		rate := ps.rate()
		if rate > 0 {
			row.Rate = rate
			remaining := ps.total - ps.completed
			row.ETA = time.Duration(float64(remaining)/rate*60) * time.Second
		}
	}

	return row
}

// buildFinishRow creates a ProgressRow for the final summary (shows elapsed for all phases).
func (d *TTYDisplay) buildFinishRow(key phaseKey, ps *phaseState) ProgressRow {
	row := ProgressRow{
		Phase:     key.phase,
		Lang:      ps.lang,
		Completed: ps.completed,
		Failed:    ps.failed,
		Total:     ps.total,
		Warnings:  ps.warnings,
		Done:      ps.finished,
	}

	if pc := d.phaseConfigFor(key.phase); pc != nil {
		row.Info = pc.Info
	}

	if !ps.finishTime.IsZero() {
		row.Elapsed = ps.finishTime.Sub(ps.startTime)
	} else if ps.completed > 0 || ps.failed > 0 {
		row.Elapsed = d.now().Sub(ps.startTime)
	}

	return row
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

func logEntryFromResult(result PageResult, now time.Time) LogEntry {
	level := LogNormal
	var msg string

	switch {
	case result.Err != nil:
		level = LogError
		if result.LayoutError != "" {
			msg = fmt.Sprintf("page %d \u2014 %s \u2717 fallback \u2014 %v", result.PageNum, result.LayoutTool, result.Err)
		} else {
			msg = fmt.Sprintf("page %d \u2014 %v", result.PageNum, result.Err)
		}
	case result.Phase == PhaseRead && result.LayoutTool != "":
		// Read phase: show layout tool info and region type breakdown
		var regionParts []string
		if result.Entries > 0 {
			regionParts = append(regionParts, fmt.Sprintf("%d entry", result.Entries))
		}
		if result.Footnotes > 0 {
			regionParts = append(regionParts, fmt.Sprintf("%d footnote", result.Footnotes))
		}

		totalRegions := result.Entries + result.Footnotes
		regionDetail := ""
		if totalRegions > 0 && len(regionParts) > 0 {
			regionDetail = fmt.Sprintf("%d regions (%s)", totalRegions, strings.Join(regionParts, ", "))
		} else if totalRegions > 0 {
			regionDetail = fmt.Sprintf("%d regions", totalRegions)
		}

		switch {
		case result.LayoutError != "":
			msg = fmt.Sprintf("page %d \u2014 %s \u2717 fallback", result.PageNum, result.LayoutTool)
			level = LogWarning
		case result.LayoutTool == "ai-only":
			msg = fmt.Sprintf("page %d \u2014 ai-only", result.PageNum)
		default:
			msg = fmt.Sprintf("page %d \u2014 %s %dms", result.PageNum, result.LayoutTool, result.LayoutMs)
		}
		if regionDetail != "" {
			msg += " \u2014 " + regionDetail
		}
		if result.Warnings > 0 {
			level = LogWarning
		}
	default:
		var details []string
		if result.Entries > 0 {
			details = append(details, fmt.Sprintf("%d entries", result.Entries))
		}
		if result.Footnotes > 0 {
			details = append(details, fmt.Sprintf("%d footnotes", result.Footnotes))
		}
		if len(details) > 0 {
			msg = fmt.Sprintf("page %d \u2014 %s", result.PageNum, strings.Join(details, ", "))
		} else {
			msg = fmt.Sprintf("page %d", result.PageNum)
		}
		if result.Warnings > 0 {
			level = LogWarning
		}
	}

	return LogEntry{
		Time:    now,
		Message: msg,
		Level:   level,
	}
}
