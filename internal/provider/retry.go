package provider

import (
	"context"
	"errors"
	"fmt"
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
		select {
		case <-ctx.Done():
			var zero T
			return zero, ctx.Err()
		case <-time.After(retryDelays[attempt]):
		}
	}
	// unreachable
	var zero T
	return zero, nil
}
