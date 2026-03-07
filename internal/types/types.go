package types

import (
	"time"

	"github.com/k15z/axiom/internal/discovery"
)

type Usage struct {
	InputTokens  int
	OutputTokens int
	APICalls     int
}

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
