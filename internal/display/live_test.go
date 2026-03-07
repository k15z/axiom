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

			d.printCIProgress(tt.testName, tt.passed, tt.cached, tt.skipped, tt.dur)

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
	d.FinishTest("test_a", true, false, false, 1*time.Second)
	d.StartTest("test_b")
	d.FinishTest("test_b", false, true, false, 0)
	d.StartTest("test_c")
	d.FinishTest("test_c", false, false, false, 2*time.Second)
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
