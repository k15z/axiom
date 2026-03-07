package output

import (
	"strings"
	"testing"

	"github.com/k15z/axiom/internal/types"
)

// --- estimateCost ---

func TestEstimateCost_Haiku(t *testing.T) {
	// haiku: $1.00/M input, $5.00/M output
	got := estimateCost("claude-haiku-4-5-20251001", 1_000_000, 0)
	if got != 1.00 {
		t.Errorf("haiku input cost = %v, want 1.00", got)
	}
	got = estimateCost("claude-haiku-4-5-20251001", 0, 1_000_000)
	if got != 5.00 {
		t.Errorf("haiku output cost = %v, want 5.00", got)
	}
}

func TestEstimateCost_Sonnet(t *testing.T) {
	// sonnet: $3.00/M input, $15.00/M output
	got := estimateCost("claude-sonnet-4-6", 1_000_000, 0)
	if got != 3.00 {
		t.Errorf("sonnet input cost = %v, want 3.00", got)
	}
	got = estimateCost("claude-sonnet-4-6", 0, 1_000_000)
	if got != 15.00 {
		t.Errorf("sonnet output cost = %v, want 15.00", got)
	}
}

func TestEstimateCost_UnknownModel(t *testing.T) {
	// unknown model falls back to haiku pricing
	got := estimateCost("gpt-unknown", 1_000_000, 0)
	if got != 1.00 {
		t.Errorf("unknown model cost = %v, want 1.00 (haiku fallback)", got)
	}
}

func TestEstimateCost_ZeroTokens(t *testing.T) {
	got := estimateCost("claude-haiku-4-5-20251001", 0, 0)
	if got != 0.0 {
		t.Errorf("zero tokens cost = %v, want 0.0", got)
	}
}

// --- formatTokens ---

func TestFormatTokens(t *testing.T) {
	cases := []struct {
		n    int
		want string
	}{
		{0, "0"},
		{999, "999"},
		{1000, "1.0K"},
		{1500, "1.5K"},
		{999_999, "1000.0K"},
		{1_000_000, "1.0M"},
		{2_500_000, "2.5M"},
	}
	for _, tc := range cases {
		got := formatTokens(tc.n)
		if got != tc.want {
			t.Errorf("formatTokens(%d) = %q, want %q", tc.n, got, tc.want)
		}
	}
}

// --- firstLine ---

func TestFirstLine_SingleLine(t *testing.T) {
	got := firstLine("hello world")
	if got != "hello world" {
		t.Errorf("got %q, want %q", got, "hello world")
	}
}

func TestFirstLine_MultiLine(t *testing.T) {
	got := firstLine("first\nsecond\nthird")
	if got != "first" {
		t.Errorf("got %q, want %q", got, "first")
	}
}

func TestFirstLine_Whitespace(t *testing.T) {
	got := firstLine("  trimmed  ")
	if got != "trimmed" {
		t.Errorf("got %q, want %q", got, "trimmed")
	}
}

func TestFirstLine_TruncatesLongLine(t *testing.T) {
	long := strings.Repeat("x", 101)
	got := firstLine(long)
	if len(got) != 100 {
		t.Errorf("expected length 100, got %d", len(got))
	}
	if !strings.HasSuffix(got, "...") {
		t.Errorf("expected '...' suffix, got %q", got[97:])
	}
}

func TestFirstLine_ExactlyHundredChars(t *testing.T) {
	exact := strings.Repeat("a", 100)
	got := firstLine(exact)
	if got != exact {
		t.Errorf("100-char string should not be truncated")
	}
}

// --- HasFailures ---

func TestHasFailures_Empty(t *testing.T) {
	if HasFailures(nil) {
		t.Error("empty slice should return false")
	}
}

func TestHasFailures_AllPassed(t *testing.T) {
	results := []types.TestResult{
		{Passed: true},
		{Passed: true},
	}
	if HasFailures(results) {
		t.Error("all-passed should return false")
	}
}

func TestHasFailures_OneFailed(t *testing.T) {
	results := []types.TestResult{
		{Passed: true},
		{Passed: false, Cached: false, Skipped: false},
	}
	if !HasFailures(results) {
		t.Error("one real failure should return true")
	}
}

func TestHasFailures_FailedButCached(t *testing.T) {
	results := []types.TestResult{
		{Passed: false, Cached: true},
	}
	if HasFailures(results) {
		t.Error("cached failure should not count as a failure")
	}
}

func TestHasFailures_FailedButSkipped(t *testing.T) {
	results := []types.TestResult{
		{Passed: false, Skipped: true},
	}
	if HasFailures(results) {
		t.Error("skipped result should not count as a failure")
	}
}
