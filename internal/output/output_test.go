package output

import (
	"strings"
	"testing"
	"time"

	"github.com/k15z/axiom/internal/discovery"
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

func TestEstimateCost_Opus(t *testing.T) {
	// opus: $15.00/M input, $75.00/M output
	got := estimateCost("claude-opus-4-6", 1_000_000, 0)
	if got != 15.00 {
		t.Errorf("opus input cost = %v, want 15.00", got)
	}
	got = estimateCost("claude-opus-4-6", 0, 1_000_000)
	if got != 75.00 {
		t.Errorf("opus output cost = %v, want 75.00", got)
	}
}

func TestEstimateCost_MixedTokens(t *testing.T) {
	// haiku: 500K input ($0.50) + 200K output ($1.00) = $1.50
	got := estimateCost("claude-haiku-4-5-20251001", 500_000, 200_000)
	want := 1.50
	if got != want {
		t.Errorf("mixed tokens cost = %v, want %v", got, want)
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

func TestFirstLine_EmptyString(t *testing.T) {
	got := firstLine("")
	if got != "" {
		t.Errorf("got %q, want empty string", got)
	}
}

func TestFirstLine_OnlyWhitespace(t *testing.T) {
	got := firstLine("   \n\n  ")
	if got != "" {
		t.Errorf("got %q, want empty string", got)
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

func TestHasFailures_FlakyNotCounted(t *testing.T) {
	results := []types.TestResult{
		{Passed: true, Flaky: true},
	}
	if HasFailures(results) {
		t.Error("flaky (but passed) should not count as a failure")
	}
}

func TestHasFailures_AllCached(t *testing.T) {
	results := []types.TestResult{
		{Cached: true},
		{Cached: true},
	}
	if HasFailures(results) {
		t.Error("all-cached should return false")
	}
}

func TestHasFailures_MixedNonFailures(t *testing.T) {
	results := []types.TestResult{
		{Passed: true},
		{Cached: true},
		{Skipped: true},
		{Passed: true, Flaky: true},
	}
	if HasFailures(results) {
		t.Error("mix of passed/cached/skipped/flaky should return false")
	}
}

func TestHasFailures_ErroredNotCounted(t *testing.T) {
	results := []types.TestResult{
		{Passed: false, Errored: true},
	}
	if HasFailures(results) {
		t.Error("errored result should not count as a failure")
	}
}

// --- HasErrors ---

func TestHasErrors_Empty(t *testing.T) {
	if HasErrors(nil) {
		t.Error("empty slice should return false")
	}
}

func TestHasErrors_OneErrored(t *testing.T) {
	results := []types.TestResult{
		{Passed: true},
		{Errored: true},
	}
	if !HasErrors(results) {
		t.Error("one errored result should return true")
	}
}

func TestHasErrors_NoErrors(t *testing.T) {
	results := []types.TestResult{
		{Passed: true},
		{Passed: false},
	}
	if HasErrors(results) {
		t.Error("no errored results should return false")
	}
}

// --- HasFlaky ---

func TestHasFlaky_Empty(t *testing.T) {
	if HasFlaky(nil) {
		t.Error("empty slice should return false")
	}
}

func TestHasFlaky_OneFlaky(t *testing.T) {
	results := []types.TestResult{
		{Passed: true},
		{Passed: true, Flaky: true},
	}
	if !HasFlaky(results) {
		t.Error("one flaky result should return true")
	}
}

func TestHasFlaky_NoFlaky(t *testing.T) {
	results := []types.TestResult{
		{Passed: true},
		{Passed: false},
	}
	if HasFlaky(results) {
		t.Error("no flaky results should return false")
	}
}

// --- CISummary ---

func TestCISummary(t *testing.T) {
	cases := []struct {
		name    string
		passed  int
		failed  int
		errored int
		cached  int
		skipped int
		want    string
	}{
		{"all passed", 5, 0, 0, 0, 0, "5 passed"},
		{"mixed results", 3, 1, 0, 2, 0, "3 passed, 1 failed, 2 cached"},
		{"all cached", 0, 0, 0, 10, 0, "10 cached"},
		{"failures and skipped", 0, 2, 0, 0, 3, "2 failed, 3 skipped"},
		{"everything", 4, 1, 0, 3, 2, "4 passed, 1 failed, 3 cached, 2 skipped"},
		{"no tests", 0, 0, 0, 0, 0, "no tests ran"},
		{"only failed", 0, 1, 0, 0, 0, "1 failed"},
		{"only skipped", 0, 0, 0, 0, 5, "5 skipped"},
		{"passed and cached", 2, 0, 0, 3, 0, "2 passed, 3 cached"},
		{"single each", 1, 1, 0, 1, 1, "1 passed, 1 failed, 1 cached, 1 skipped"},
		{"with errors", 3, 1, 2, 0, 0, "3 passed, 1 failed, 2 errored"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := CISummary(tc.passed, tc.failed, tc.errored, tc.cached, tc.skipped)
			if got != tc.want {
				t.Errorf("CISummary(%d, %d, %d, %d, %d) = %q, want %q",
					tc.passed, tc.failed, tc.errored, tc.cached, tc.skipped, got, tc.want)
			}
		})
	}
}

// --- FormatGitHub ---

func makeResult(name, file string, passed, cached, skipped, flaky bool, reasoning string, dur time.Duration, usage types.Usage) types.TestResult {
	return types.TestResult{
		Test:      discovery.Test{Name: name, SourceFile: file},
		Passed:    passed,
		Cached:    cached,
		Skipped:   skipped,
		Flaky:     flaky,
		Reasoning: reasoning,
		Duration:  dur,
		Usage:     usage,
	}
}

func TestFormatGitHub_HTMLComment(t *testing.T) {
	results := []types.TestResult{
		makeResult("test1", "a.yml", true, false, false, false, "", 5*time.Second, types.Usage{}),
	}
	got := FormatGitHub(results, "claude-haiku-4-5-20251001")
	if !strings.HasPrefix(got, "<!-- axiom-summary -->") {
		t.Error("expected <!-- axiom-summary --> HTML comment at the top")
	}
}

func TestFormatGitHub_Header(t *testing.T) {
	results := []types.TestResult{
		makeResult("test1", "a.yml", true, false, false, false, "", 5*time.Second, types.Usage{}),
	}
	got := FormatGitHub(results, "claude-haiku-4-5-20251001")
	if !strings.Contains(got, "## Axiom Test Results") {
		t.Error("expected header")
	}
}

func TestFormatGitHub_TableRows(t *testing.T) {
	results := []types.TestResult{
		makeResult("test_pass", "security.yml", true, false, false, false, "looks good", 9200*time.Millisecond, types.Usage{APICalls: 3, InputTokens: 1000, OutputTokens: 500}),
		makeResult("test_fail", "concurrency.yml", false, false, false, false, "missing lock", 8400*time.Millisecond, types.Usage{APICalls: 2, InputTokens: 800, OutputTokens: 400}),
		makeResult("test_cached", "security.yml", true, true, false, false, "", 0, types.Usage{}),
		makeResult("test_skipped", "arch.yml", false, false, true, false, "", 0, types.Usage{}),
	}
	got := FormatGitHub(results, "claude-haiku-4-5-20251001")

	if !strings.Contains(got, "| test_pass | security.yml | :white_check_mark: Pass | 9.2s |") {
		t.Error("expected pass row")
	}
	if !strings.Contains(got, "| test_fail | concurrency.yml | :x: Fail | 8.4s |") {
		t.Error("expected fail row")
	}
	if !strings.Contains(got, "| test_cached | security.yml | :grey_question: Cached | - |") {
		t.Error("expected cached row")
	}
	if !strings.Contains(got, "| test_skipped | arch.yml | :fast_forward: Skipped | - |") {
		t.Error("expected skipped row")
	}
}

func TestFormatGitHub_FlakyIcon(t *testing.T) {
	results := []types.TestResult{
		{
			Test:      discovery.Test{Name: "test_flaky", SourceFile: "a.yml"},
			Passed:    true,
			Flaky:     true,
			Retries:   2,
			Reasoning: "passed on retry",
			Duration:  5 * time.Second,
		},
	}
	got := FormatGitHub(results, "claude-haiku-4-5-20251001")
	if !strings.Contains(got, ":warning: Flaky") {
		t.Error("expected flaky icon")
	}
	if !strings.Contains(got, "Flaky Tests") {
		t.Error("expected flaky tests section")
	}
	if !strings.Contains(got, "Passed on retry 2") {
		t.Error("expected retry count in flaky section")
	}
}

func TestFormatGitHub_ErroredIcon(t *testing.T) {
	results := []types.TestResult{
		{
			Test:      discovery.Test{Name: "test_api_down", SourceFile: "a.yml"},
			Errored:   true,
			Reasoning: "Agent error: API call failed: 503 Service Unavailable",
			Duration:  3 * time.Second,
		},
	}
	got := FormatGitHub(results, "claude-haiku-4-5-20251001")
	if !strings.Contains(got, ":boom: Error") {
		t.Error("expected errored icon")
	}
	if !strings.Contains(got, "<summary>Errors</summary>") {
		t.Error("expected Errors section")
	}
	if !strings.Contains(got, "### test_api_down") {
		t.Error("expected error heading")
	}
}

func TestFormatGitHub_SummaryLine(t *testing.T) {
	results := []types.TestResult{
		makeResult("t1", "a.yml", true, false, false, false, "", time.Second, types.Usage{}),
		makeResult("t2", "a.yml", false, false, false, false, "bad", time.Second, types.Usage{}),
		makeResult("t3", "a.yml", true, true, false, false, "", 0, types.Usage{}),
	}
	got := FormatGitHub(results, "claude-haiku-4-5-20251001")
	if !strings.Contains(got, "**1 passed**") {
		t.Error("expected passed count")
	}
	if !strings.Contains(got, "**1 failed**") {
		t.Error("expected failed count")
	}
	if !strings.Contains(got, "**1 cached**") {
		t.Error("expected cached count")
	}
}

func TestFormatGitHub_FailureDetails(t *testing.T) {
	results := []types.TestResult{
		makeResult("test_broken", "a.yml", false, false, false, false, "Route handler bypasses auth middleware", time.Second, types.Usage{}),
	}
	got := FormatGitHub(results, "claude-haiku-4-5-20251001")
	if !strings.Contains(got, "<details>") {
		t.Error("expected collapsible details section")
	}
	if !strings.Contains(got, "<summary>Failures</summary>") {
		t.Error("expected Failures summary")
	}
	if !strings.Contains(got, "### test_broken") {
		t.Error("expected failure heading")
	}
	if !strings.Contains(got, "> Route handler bypasses auth middleware") {
		t.Error("expected reasoning as blockquote")
	}
}

func TestFormatGitHub_MultilineReasoning(t *testing.T) {
	results := []types.TestResult{
		makeResult("test_multi", "a.yml", false, false, false, false, "Line 1\nLine 2\nLine 3", time.Second, types.Usage{}),
	}
	got := FormatGitHub(results, "claude-haiku-4-5-20251001")
	if !strings.Contains(got, "> Line 1\n> Line 2\n> Line 3") {
		t.Error("expected multiline blockquote reasoning")
	}
}

func TestFormatGitHub_CostSection(t *testing.T) {
	results := []types.TestResult{
		makeResult("t1", "a.yml", true, false, false, false, "", time.Second, types.Usage{APICalls: 5, InputTokens: 10000, OutputTokens: 2300}),
	}
	got := FormatGitHub(results, "claude-haiku-4-5-20251001")
	if !strings.Contains(got, "<summary>Cost</summary>") {
		t.Error("expected Cost section")
	}
	if !strings.Contains(got, "Model: claude-haiku-4-5-20251001") {
		t.Error("expected model name in cost section")
	}
	if !strings.Contains(got, "5 API calls") {
		t.Error("expected API call count")
	}
}

func TestFormatGitHub_NoCostWhenNoAPICalls(t *testing.T) {
	results := []types.TestResult{
		makeResult("t1", "a.yml", true, true, false, false, "", 0, types.Usage{}),
	}
	got := FormatGitHub(results, "claude-haiku-4-5-20251001")
	if strings.Contains(got, "Cost") {
		t.Error("should not show cost section when no API calls")
	}
}

func TestFormatGitHub_NoFailuresSection(t *testing.T) {
	results := []types.TestResult{
		makeResult("t1", "a.yml", true, false, false, false, "", time.Second, types.Usage{}),
	}
	got := FormatGitHub(results, "claude-haiku-4-5-20251001")
	if strings.Contains(got, "Failures") {
		t.Error("should not show failures section when all pass")
	}
}

func TestFormatGitHub_EmptyResults(t *testing.T) {
	got := FormatGitHub(nil, "claude-haiku-4-5-20251001")
	if !strings.Contains(got, "## Axiom Test Results") {
		t.Error("expected header even with empty results")
	}
	if strings.Contains(got, "Failures") {
		t.Error("should not show failures section for empty results")
	}
}
