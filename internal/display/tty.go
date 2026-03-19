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

	// Push log entry
	d.logBuffer.Push(logEntryFromResult(result, now))

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

	d.stopStatusTicker()
	d.status = StatusLine{}
	d.clearLines()
	RenderHeader(d.out, d.header, d.colors)
	for _, key := range d.phaseOrder {
		ps := d.phases[key]
		row := d.buildFinishRow(key, ps)
		fmt.Fprintln(d.out, RenderProgressLine(row, d.colors))
		if weLine := RenderWarnErrorLine(ps.warnings, ps.failed, d.colors); weLine != "" {
			fmt.Fprintln(d.out, weLine)
		}
	}
	d.currentLines = 0
}

func (d *TTYDisplay) render() {
	d.clearLines()

	lines := RenderHeader(d.out, d.header, d.colors)
	statusRendered := false
	for _, key := range d.phaseOrder {
		ps := d.phases[key]
		row := d.buildLiveRow(key, ps)
		fmt.Fprintln(d.out, RenderProgressLine(row, d.colors))
		lines++

		// Status line under the active (non-finished) phase
		if !statusRendered && !ps.finished && d.status.Text != "" {
			elapsed := d.now().Sub(d.status.StartedAt)
			fmt.Fprintln(d.out, FormatStatusLine(d.status.Text, elapsed, d.status.Countdown, d.colors))
			lines++
			statusRendered = true
		}

		if weLine := RenderWarnErrorLine(ps.warnings, ps.failed, d.colors); weLine != "" {
			fmt.Fprintln(d.out, weLine)
			lines++
		}
	}

	// Log tail
	entries := d.logBuffer.Entries()
	if len(entries) > 0 {
		fmt.Fprintln(d.out)
		fmt.Fprintln(d.out, "  \u2500\u2500\u2500 recent \u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500")
		lines += 2
		for _, e := range entries {
			fmt.Fprintf(d.out, "  %s\n", FormatLogEntry(e, 80, d.colors))
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

	if result.Err != nil {
		level = LogError
		msg = fmt.Sprintf("page %d \u2014 %v", result.PageNum, result.Err)
	} else {
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
