package display

import (
	"bytes"
	"os"
	"strings"
	"testing"
	"time"
)

func TestPrintCIProgress(t *testing.T) {
	tests := []struct {
		name      string
		testName  string
		passed    bool
		cached    bool
		skipped   bool
		errored   bool
		dur       time.Duration
		completed int
		total     int
		wantParts []string
	}{
		{
			name:      "passed test",
			testName:  "test_auth",
			passed:    true,
			dur:       2500 * time.Millisecond,
			completed: 3,
			total:     10,
			wantParts: []string{"[3/10]", "✓", "test_auth", "2.5s"},
		},
		{
			name:      "failed test",
			testName:  "test_security",
			passed:    false,
			dur:       1200 * time.Millisecond,
			completed: 5,
			total:     10,
			wantParts: []string{"[5/10]", "✗", "test_security", "1.2s"},
		},
		{
			name:      "cached test",
			testName:  "test_cached",
			cached:    true,
			completed: 1,
			total:     5,
			wantParts: []string{"[1/5]", "○", "test_cached", "(cached)"},
		},
		{
			name:      "skipped test",
			testName:  "test_skipped",
			skipped:   true,
			completed: 2,
			total:     5,
			wantParts: []string{"[2/5]", "○", "test_skipped", "(skipped)"},
		},
		{
			name:      "errored test",
			testName:  "test_errored",
			errored:   true,
			dur:       3 * time.Second,
			completed: 4,
			total:     10,
			wantParts: []string{"[4/10]", "!", "test_errored", "(error, 3.0s)"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := &LiveDisplay{
				total:     tt.total,
				completed: tt.completed,
			}

			// Capture stderr output
			r, w, _ := os.Pipe()
			oldStderr := os.Stderr
			os.Stderr = w

			d.printCIProgress(tt.testName, tt.passed, tt.cached, tt.skipped, tt.errored, tt.dur)

			w.Close()
			os.Stderr = oldStderr

			var buf bytes.Buffer
			buf.ReadFrom(r)
			got := buf.String()

			for _, part := range tt.wantParts {
				if !strings.Contains(got, part) {
					t.Errorf("output %q missing expected substring %q", got, part)
				}
			}
		})
	}
}

func TestLiveDisplayNonTTY(t *testing.T) {
	// Force non-TTY mode
	origTTY := tty
	tty = false
	defer func() { tty = origTTY }()

	// Capture stderr
	r, w, _ := os.Pipe()
	oldStderr := os.Stderr
	os.Stderr = w

	d := NewLiveDisplay(3)
	d.StartTest("test_a")
	d.FinishTest("test_a", true, false, false, false, 1*time.Second)
	d.StartTest("test_b")
	d.FinishTest("test_b", false, true, false, false, 0)
	d.StartTest("test_c")
	d.FinishTest("test_c", false, false, false, false, 2*time.Second)
	d.Close()

	w.Close()
	os.Stderr = oldStderr

	var buf bytes.Buffer
	buf.ReadFrom(r)
	output := buf.String()

	// Verify progress lines were printed
	if !strings.Contains(output, "[1/3]") {
		t.Errorf("missing [1/3] progress, got: %s", output)
	}
	if !strings.Contains(output, "[2/3]") {
		t.Errorf("missing [2/3] progress, got: %s", output)
	}
	if !strings.Contains(output, "[3/3]") {
		t.Errorf("missing [3/3] progress, got: %s", output)
	}
}

func TestNonTTYFiltersTextDeltas(t *testing.T) {
	origTTY := tty
	tty = false
	defer func() { tty = origTTY }()

	r, w, _ := os.Pipe()
	oldStderr := os.Stderr
	os.Stderr = w

	d := NewLiveDisplay(1)
	d.StartTest("test_a")
	// Streaming text deltas should be suppressed in CI
	d.Update("test_a", "✎ The agent is exploring the codebase")
	d.Update("test_a", "✎ I found the file at src/main.go")
	// Tool calls should still print
	d.Update("test_a", "→ read_file src/main.go")
	d.Update("test_a", "thinking (turn 2/30)")
	d.FinishTest("test_a", true, false, false, false, 1*time.Second)
	d.Close()

	w.Close()
	os.Stderr = oldStderr

	var buf bytes.Buffer
	buf.ReadFrom(r)
	output := buf.String()

	// Text deltas should NOT appear
	if strings.Contains(output, "exploring the codebase") {
		t.Error("text delta should be suppressed in non-TTY mode")
	}
	if strings.Contains(output, "I found the file") {
		t.Error("text delta should be suppressed in non-TTY mode")
	}
	// Tool calls and thinking SHOULD appear
	if !strings.Contains(output, "read_file") {
		t.Errorf("tool call should appear in non-TTY output, got: %s", output)
	}
	if !strings.Contains(output, "thinking") {
		t.Errorf("thinking status should appear in non-TTY output, got: %s", output)
	}
}

func TestIsTextDelta(t *testing.T) {
	tests := []struct {
		status string
		want   bool
	}{
		{"✎ some streaming text", true},
		{"✎ ", true},
		{"→ read_file foo.go", false},
		{"thinking (turn 1/30)", false},
		{"starting…", false},
		{"retrying (1/3)…", false},
		{"", false},
	}
	for _, tt := range tests {
		if got := isTextDelta(tt.status); got != tt.want {
			t.Errorf("isTextDelta(%q) = %v, want %v", tt.status, got, tt.want)
		}
	}
}
