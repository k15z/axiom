package provider

import (
	"context"
	"errors"
	"fmt"
	"math/rand/v2"
	"strings"
	"time"
)

var retryDelays = []time.Duration{5 * time.Second, 15 * time.Second, 30 * time.Second, 60 * time.Second}

// rateLimitError signals an HTTP 429 response so the retry loop can distinguish
// retryable rate limits from other errors.
type rateLimitError struct {
	status int
}

func (e *rateLimitError) Error() string {
	return fmt.Sprintf("rate limited (HTTP %d)", e.status)
}

// isRateLimitError returns true if err is a rateLimitError.
func isRateLimitError(err error) bool {
	var rle *rateLimitError
	return errors.As(err, &rle)
}

// IsRateLimitError reports whether err indicates a rate limit (HTTP 429).
// This checks for the structured rateLimitError type used by all providers,
// with a string-match fallback for errors from external SDKs.
func IsRateLimitError(err error) bool {
	if err == nil {
		return false
	}
	if isRateLimitError(err) {
		return true
	}
	// Fallback: some SDK errors (e.g. Anthropic streaming) may wrap
	// rate limit info in error strings rather than typed errors.
	s := strings.ToLower(err.Error())
	return strings.Contains(s, "429") || strings.Contains(s, "rate limit") || strings.Contains(s, "rate_limit")
}

// WithRetry calls fn up to len(retryDelays)+1 times, retrying when shouldRetry
// returns true. Between retries it waits for the corresponding delay, respecting
// ctx cancellation.
func WithRetry[T any](ctx context.Context, fn func() (T, error), shouldRetry func(error) bool) (T, error) {
	maxAttempts := len(retryDelays) + 1
	for attempt := 0; attempt < maxAttempts; attempt++ {
		result, err := fn()
		if err == nil {
			return result, nil
		}
		if !shouldRetry(err) || attempt == maxAttempts-1 {
			return result, err
		}
		// Add jitter (±25%) to avoid thundering herd with concurrent tests
		delay := retryDelays[attempt]
		jitter := time.Duration(rand.Int64N(int64(delay) / 2)) - delay/4
		delay += jitter

		select {
		case <-ctx.Done():
			var zero T
			return zero, ctx.Err()
		case <-time.After(delay):
		}
	}
	// unreachable
	var zero T
	return zero, nil
}
