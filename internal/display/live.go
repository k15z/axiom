// Package output provides terminal rendering utilities.
package display

// LiveDisplay manages an in-place updating status panel written to stderr.
// Each running test occupies one line, updated in-place via ANSI cursor control.
// Falls back to plain newlines when stderr is not a TTY (e.g. CI).

import (
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/fatih/color"
	"github.com/mattn/go-isatty"
)

var tty = isatty.IsTerminal(os.Stderr.Fd())

var spinnerFrames = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

type slotState struct {
	name     string
	status   string
	frame    int
	done     bool
	passed   bool
	cached   bool
	skipped  bool
	errored  bool
	duration time.Duration
}

// LiveDisplay coordinates live rendering of running tests to stderr.
type LiveDisplay struct {
	mu        sync.Mutex
	slots     []*slotState
	byName    map[string]int // name → slot index
	ticker    *time.Ticker
	stopCh    chan struct{}
	lines     int // lines currently on screen (for cursor-up)
	total     int // total number of tests to run
	completed int // number of tests finished so far
}

// NewLiveDisplay creates and starts a LiveDisplay for total tests.
func NewLiveDisplay(total int) *LiveDisplay {
	d := &LiveDisplay{
		byName: make(map[string]int),
		stopCh: make(chan struct{}),
		total:  total,
	}
	if tty {
		d.ticker = time.NewTicker(80 * time.Millisecond)
		go d.spinLoop()
	}
	return d
}

// StartTest registers a new test as running and renders it to the display.
func (d *LiveDisplay) StartTest(name string) {
	d.mu.Lock()
	defer d.mu.Unlock()
	idx := len(d.slots)
	d.byName[name] = idx
	d.slots = append(d.slots, &slotState{name: name, status: "starting…"})
	d.render()
}

// Update sets the status message for a running test.
func (d *LiveDisplay) Update(name, status string) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if idx, ok := d.byName[name]; ok {
		d.slots[idx].status = status
		// In non-TTY (CI), only print tool calls — skip streaming text to avoid flooding logs.
		if !tty && !isTextDelta(status) {
			color.New(color.FgHiBlack).Fprintf(os.Stderr, "  [%d/%d] [%s] %s\n", d.completed, d.total, name, status)
		}
	}
}

// isTextDelta returns true for streaming text updates (prefixed with "✎ "),
// which arrive frequently and would flood CI logs.
func isTextDelta(status string) bool {
	return len(status) >= 4 && status[:4] == "✎ "
}

// FinishTest marks a test as complete and updates its final state in the display.
func (d *LiveDisplay) FinishTest(name string, passed, cached, skipped, errored bool, dur time.Duration) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if idx, ok := d.byName[name]; ok {
		s := d.slots[idx]
		s.done, s.passed, s.cached, s.skipped, s.errored, s.duration = true, passed, cached, skipped, errored, dur
		d.completed++
	}
	if !tty {
		d.printCIProgress(name, passed, cached, skipped, errored, dur)
	}
	d.render()
}

// printCIProgress prints a one-line progress summary for non-TTY environments.
func (d *LiveDisplay) printCIProgress(name string, passed, cached, skipped, errored bool, dur time.Duration) {
	var marker string
	switch {
	case cached:
		marker = "○"
	case skipped:
		marker = "○"
	case errored:
		marker = "!"
	case passed:
		marker = "✓"
	default:
		marker = "✗"
	}

	var detail string
	switch {
	case cached:
		detail = "(cached)"
	case skipped:
		detail = "(skipped)"
	case errored:
		detail = fmt.Sprintf("(error, %.1fs)", dur.Seconds())
	default:
		detail = fmt.Sprintf("(%.1fs)", dur.Seconds())
	}

	fmt.Fprintf(os.Stderr, "  [%d/%d] %s %s %s\n", d.completed, d.total, marker, name, detail)
}

// Close stops the spinner and renders the final state of all slots.
func (d *LiveDisplay) Close() {
	if d.ticker != nil {
		d.ticker.Stop()
		close(d.stopCh)
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	d.render()
	if tty && d.lines > 0 {
		fmt.Fprintf(os.Stderr, "\n")
	}
}

func (d *LiveDisplay) spinLoop() {
	for {
		select {
		case <-d.stopCh:
			return
		case <-d.ticker.C:
			d.mu.Lock()
			for _, s := range d.slots {
				if !s.done {
					s.frame = (s.frame + 1) % len(spinnerFrames)
				}
			}
			d.render()
			d.mu.Unlock()
		}
	}
}

func (d *LiveDisplay) render() {
	if !tty || len(d.slots) == 0 {
		return
	}

	// Move cursor back up over previously rendered lines
	if d.lines > 0 {
		fmt.Fprintf(os.Stderr, "\033[%dA", d.lines)
	}

	for _, s := range d.slots {
		// \033[2K clears the entire current line before writing
		fmt.Fprintf(os.Stderr, "\033[2K%s\n", d.slotLine(s))
	}

	d.lines = len(d.slots)
}

func (d *LiveDisplay) slotLine(s *slotState) string {
	if !s.done {
		prefix := fmt.Sprintf("[%d/%d] ", d.completed, d.total)
		nameWidth := 38 - len(prefix)
		if nameWidth < 20 {
			nameWidth = 20
		}
		spinner := color.New(color.FgCyan).Sprint(spinnerFrames[s.frame])
		status := truncateStatus(s.status, 52)
		return fmt.Sprintf("  %s%s %s  %s",
			color.New(color.FgHiBlack).Sprint(prefix),
			spinner,
			color.New(color.FgHiBlack).Sprintf("%-*s", nameWidth, s.name),
			color.New(color.FgHiBlack).Sprint(status),
		)
	}

	nameCol := fmt.Sprintf("%-40s", s.name)
	switch {
	case s.cached:
		return color.New(color.FgHiBlack).Sprintf("  ○ %s(cached)", nameCol)
	case s.skipped:
		return color.New(color.FgHiBlack).Sprintf("  ○ %s(skipped)", nameCol)
	case s.errored:
		return color.New(color.FgRed).Sprintf("  ! %s", nameCol) +
			color.New(color.FgHiBlack).Sprintf("(error, %.1fs)", s.duration.Seconds())
	case s.passed:
		return color.New(color.FgGreen).Sprintf("  ✓ %s", nameCol) +
			color.New(color.FgHiBlack).Sprintf("(%.1fs)", s.duration.Seconds())
	default:
		return color.New(color.FgRed).Sprintf("  ✗ %s", nameCol) +
			color.New(color.FgHiBlack).Sprintf("(%.1fs)", s.duration.Seconds())
	}
}

func truncateStatus(s string, max int) string {
	// Strip visible length (ignore ANSI) — status comes from us so no ANSI yet
	if len(s) > max {
		return s[:max-1] + "…"
	}
	return s
}
