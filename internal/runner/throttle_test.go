package runner

import (
	"context"
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	"github.com/k15z/axiom/internal/config"
	"github.com/k15z/axiom/internal/discovery"
	"github.com/k15z/axiom/internal/provider"
)

func TestThrottle_NoSignalNoDelay(t *testing.T) {
	th := NewThrottle()
	start := time.Now()
	th.Wait()
	if elapsed := time.Since(start); elapsed > 50*time.Millisecond {
		t.Errorf("Wait() took %v without signal, expected immediate return", elapsed)
	}
}

func TestThrottle_SignalSetsBackoff(t *testing.T) {
	th := NewThrottle()
	th.Signal()

	th.mu.Lock()
	backoff := th.backoff
	th.mu.Unlock()

	if backoff != defaultInitialBackoff {
		t.Errorf("backoff = %v, want %v after first signal", backoff, defaultInitialBackoff)
	}
}

func TestThrottle_ExponentialBackoff(t *testing.T) {
	th := NewThrottle()
	th.Signal() // 5s
	th.Signal() // 10s
	th.Signal() // 20s

	th.mu.Lock()
	backoff := th.backoff
	th.mu.Unlock()

	if backoff != 20*time.Second {
		t.Errorf("backoff = %v, want 20s after 3 signals", backoff)
	}
}

func TestThrottle_MaxBackoffCap(t *testing.T) {
	th := NewThrottle()
	// Signal enough times to exceed defaultMaxBackoff
	for i := 0; i < 10; i++ {
		th.Signal()
	}

	th.mu.Lock()
	backoff := th.backoff
	th.mu.Unlock()

	if backoff > defaultMaxBackoff {
		t.Errorf("backoff = %v, should be capped at %v", backoff, defaultMaxBackoff)
	}
}

func TestThrottle_DecayResetsBackoff(t *testing.T) {
	th := NewThrottle()
	th.cooldown = 10 * time.Millisecond // short cooldown for testing
	th.Signal()

	// Wait for cooldown to expire
	time.Sleep(20 * time.Millisecond)

	start := time.Now()
	th.Wait()
	if elapsed := time.Since(start); elapsed > 50*time.Millisecond {
		t.Errorf("Wait() took %v after cooldown, expected immediate return", elapsed)
	}

	// Verify backoff was actually reset
	th.mu.Lock()
	backoff := th.backoff
	th.mu.Unlock()
	if backoff != 0 {
		t.Errorf("backoff = %v, want 0 after cooldown", backoff)
	}
}

func TestIsRateLimitErr(t *testing.T) {
	tests := []struct {
		err  error
		want bool
	}{
		{nil, false},
		{fmt.Errorf("connection refused"), false},
		{fmt.Errorf("503 Service Unavailable"), false},
		{fmt.Errorf("API call failed: rate limited (HTTP 429)"), true},
		{fmt.Errorf("API call failed: 429 Too Many Requests"), true},
		{fmt.Errorf("rate_limit_error: too many requests"), true},
		{fmt.Errorf("Rate Limit exceeded"), true},
	}
	for _, tt := range tests {
		got := provider.IsRateLimitError(tt.err)
		if got != tt.want {
			t.Errorf("IsRateLimitError(%v) = %v, want %v", tt.err, got, tt.want)
		}
	}
}

func setupFastThrottle() func() {
	orig := newThrottle
	newThrottle = func() *Throttle {
		th := NewThrottle()
		th.initDelay = 10 * time.Millisecond
		th.maxBackoff = 50 * time.Millisecond
		th.cooldown = 50 * time.Millisecond
		return th
	}
	return func() { newThrottle = orig }
}

func TestRun_RateLimitSignalsThrottle(t *testing.T) {
	// First test returns 429, second test should still work
	var callCount int64
	mock := &mockRunnerProvider{response: func(condition string) (*provider.ChatResponse, error) {
		n := atomic.AddInt64(&callCount, 1)
		if n == 1 {
			return nil, fmt.Errorf("API call failed: rate limited (HTTP 429)")
		}
		return passResponse()
	}}
	cleanup := setupMock(mock)
	defer cleanup()
	cleanupThrottle := setupFastThrottle()
	defer cleanupThrottle()

	tests := []discovery.Test{makeTest("test_a"), makeTest("test_b")}
	results, err := Run(context.Background(), minimalConfig(t), tests, Options{Concurrency: 1})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// First test errored due to rate limit
	if !results[0].Errored {
		t.Error("expected first test to be errored (rate limit)")
	}
	// Second test should still pass
	if !results[1].Passed {
		t.Error("expected second test to pass after rate limit")
	}
}

func TestRun_RateLimitNotCached(t *testing.T) {
	mock := &mockRunnerProvider{response: func(_ string) (*provider.ChatResponse, error) {
		return nil, fmt.Errorf("API call failed: rate limited (HTTP 429)")
	}}
	cleanup := setupMock(mock)
	defer cleanup()
	cleanupThrottle := setupFastThrottle()
	defer cleanupThrottle()

	cfg := minimalConfig(t)
	cfg.Cache = config.CacheConfig{Enabled: true, Dir: cfg.Cache.Dir}

	results, err := Run(context.Background(), cfg, []discovery.Test{
		{Name: "test_a", Condition: "check", SourceFile: "test.yml", On: []string{"*.go"}},
	}, Options{Concurrency: 1})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !results[0].Errored {
		t.Error("expected Errored=true for rate-limited test")
	}
	// Errored tests should not be cached (existing behavior)
}
