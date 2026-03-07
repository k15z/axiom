package runner

import (
	"math/rand"
	"sync"
	"time"
)

// Throttle implements runner-level rate-limit backoff. When a test encounters
// a rate-limit error (429), it signals the throttle which causes subsequent
// test starts to pause with exponential backoff and jitter.
type Throttle struct {
	mu         sync.Mutex
	backoff    time.Duration
	lastHit    time.Time
	cooldown   time.Duration // how long after last hit before backoff resets
	initDelay  time.Duration // initial backoff after first signal
	maxBackoff time.Duration // cap on backoff duration
}

const (
	defaultInitialBackoff = 5 * time.Second
	defaultMaxBackoff     = 60 * time.Second
	defaultCooldown       = 2 * time.Minute // reset backoff after no rate limits for this long
)

// NewThrottle creates a Throttle with default settings.
func NewThrottle() *Throttle {
	return &Throttle{
		cooldown:   defaultCooldown,
		initDelay:  defaultInitialBackoff,
		maxBackoff: defaultMaxBackoff,
	}
}

// Signal records a rate-limit event and increases the backoff.
func (t *Throttle) Signal() {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.lastHit = time.Now()
	if t.backoff == 0 {
		t.backoff = t.initDelay
	} else {
		t.backoff *= 2
		if t.backoff > t.maxBackoff {
			t.backoff = t.maxBackoff
		}
	}
}

// Wait blocks for the current backoff duration (with jitter) if a recent
// rate-limit was signaled. Returns immediately if no backoff is active.
func (t *Throttle) Wait() {
	t.mu.Lock()
	if t.backoff == 0 {
		t.mu.Unlock()
		return
	}
	// If enough time has passed since the last rate limit, reset.
	if time.Since(t.lastHit) > t.cooldown {
		t.backoff = 0
		t.mu.Unlock()
		return
	}
	delay := t.jitter(t.backoff)
	t.mu.Unlock()

	time.Sleep(delay)
}

// jitter adds +/- 25% randomization to a duration.
func (t *Throttle) jitter(d time.Duration) time.Duration {
	// 75% to 125% of d
	factor := 0.75 + rand.Float64()*0.5
	return time.Duration(float64(d) * factor)
}
