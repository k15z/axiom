package provider

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestWithRetry_Success(t *testing.T) {
	orig := retryDelays
	retryDelays = []time.Duration{time.Millisecond}
	defer func() { retryDelays = orig }()

	calls := 0
	result, err := WithRetry(context.Background(), func() (string, error) {
		calls++
		return "ok", nil
	}, func(err error) bool { return true })

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "ok" {
		t.Errorf("result = %q, want %q", result, "ok")
	}
	if calls != 1 {
		t.Errorf("calls = %d, want 1", calls)
	}
}

func TestWithRetry_RetryOnRateLimit(t *testing.T) {
	orig := retryDelays
	retryDelays = []time.Duration{time.Millisecond, time.Millisecond}
	defer func() { retryDelays = orig }()

	calls := 0
	result, err := WithRetry(context.Background(), func() (string, error) {
		calls++
		if calls < 3 {
			return "", &rateLimitError{status: 429}
		}
		return "recovered", nil
	}, isRateLimitError)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "recovered" {
		t.Errorf("result = %q, want %q", result, "recovered")
	}
	if calls != 3 {
		t.Errorf("calls = %d, want 3", calls)
	}
}

func TestWithRetry_MaxAttemptsExhausted(t *testing.T) {
	orig := retryDelays
	retryDelays = []time.Duration{time.Millisecond}
	defer func() { retryDelays = orig }()

	calls := 0
	_, err := WithRetry(context.Background(), func() (string, error) {
		calls++
		return "", &rateLimitError{status: 429}
	}, isRateLimitError)

	if err == nil {
		t.Fatal("expected error after max attempts")
	}
	if !isRateLimitError(err) {
		t.Errorf("expected rateLimitError, got %v", err)
	}
	// With 1 delay, max attempts = 2
	if calls != 2 {
		t.Errorf("calls = %d, want 2", calls)
	}
}

func TestWithRetry_NonRetryableError(t *testing.T) {
	orig := retryDelays
	retryDelays = []time.Duration{time.Millisecond}
	defer func() { retryDelays = orig }()

	calls := 0
	_, err := WithRetry(context.Background(), func() (string, error) {
		calls++
		return "", errors.New("permanent error")
	}, isRateLimitError)

	if err == nil || err.Error() != "permanent error" {
		t.Errorf("expected 'permanent error', got %v", err)
	}
	if calls != 1 {
		t.Errorf("calls = %d, want 1 (should not retry non-retryable errors)", calls)
	}
}

func TestWithRetry_ContextCancelled(t *testing.T) {
	orig := retryDelays
	retryDelays = []time.Duration{time.Hour} // long delay to ensure context cancels first
	defer func() { retryDelays = orig }()

	ctx, cancel := context.WithCancel(context.Background())

	calls := 0
	go func() {
		time.Sleep(10 * time.Millisecond)
		cancel()
	}()

	_, err := WithRetry(ctx, func() (string, error) {
		calls++
		return "", &rateLimitError{status: 429}
	}, isRateLimitError)

	if !errors.Is(err, context.Canceled) {
		t.Errorf("expected context.Canceled, got %v", err)
	}
	if calls != 1 {
		t.Errorf("calls = %d, want 1", calls)
	}
}

func TestIsRateLimitError(t *testing.T) {
	if !isRateLimitError(&rateLimitError{status: 429}) {
		t.Error("expected true for rateLimitError")
	}
	if isRateLimitError(errors.New("other error")) {
		t.Error("expected false for non-rateLimitError")
	}
	if isRateLimitError(nil) {
		t.Error("expected false for nil")
	}
}

func TestRateLimitError_Error(t *testing.T) {
	e := &rateLimitError{status: 429}
	want := "rate limited (HTTP 429)"
	if e.Error() != want {
		t.Errorf("Error() = %q, want %q", e.Error(), want)
	}
}
