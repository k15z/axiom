package types

import (
	"time"

	"github.com/k15z/axiom/internal/discovery"
)

// Usage tracks token consumption and API call count for a test run.
type Usage struct {
	InputTokens  int
	OutputTokens int
	APICalls     int
}

// TestResult holds the outcome of a single test execution.
type TestResult struct {
	Test      discovery.Test
	Passed    bool
	Cached    bool
	Skipped   bool // true when skipped due to --bail
	Errored   bool // true when test failed due to infrastructure error (API failure, timeout)
	Flaky     bool // true when test passed only after retry
	Retries   int  // number of retries attempted
	Reasoning string
	Duration  time.Duration
	Usage     Usage
}

// TestStatus describes the cached state of a single test.
type TestStatus struct {
	Test   discovery.Test
	Status string // "pending" | "cached-pass" | "cached-fail" | "stale-pass" | "stale-fail"
}
