package ratelimit

import (
	"math"
	"sync"
	"time"
)

// TokenBucket implements a thread-safe token bucket algorithm for rate limiting.
type TokenBucket struct {
	mu         sync.Mutex
	capacity   float64
	refillRate float64 // Tokens per second
	tokens     float64
	lastRefill time.Time
	enabled    bool
}

// NewTokenBucket creates a new TokenBucket with specified refill rate, burst capacity, and enabled state.
func NewTokenBucket(refillRate float64, burst int, enabled bool) *TokenBucket {
	if refillRate <= 0 {
		refillRate = 10.0
	}
	if burst <= 0 {
		burst = 20
	}
	capFloat := float64(burst)
	return &TokenBucket{
		capacity:   capFloat,
		refillRate: refillRate,
		tokens:     capFloat,
		lastRefill: time.Now(),
		enabled:    enabled,
	}
}

// Allow checks if a token is available. If allowed, consumes 1 token and returns true.
// If disallowed, returns false and the estimated retryAfter duration.
func (tb *TokenBucket) Allow() (bool, time.Duration) {
	tb.mu.Lock()
	defer tb.mu.Unlock()

	if !tb.enabled {
		return true, 0
	}

	now := time.Now()
	elapsed := now.Sub(tb.lastRefill).Seconds()
	if elapsed > 0 {
		tb.tokens = math.Min(tb.capacity, tb.tokens+elapsed*tb.refillRate)
		tb.lastRefill = now
	}

	if tb.tokens >= 1.0 {
		tb.tokens -= 1.0
		return true, 0
	}

	needed := 1.0 - tb.tokens
	retrySeconds := needed / tb.refillRate
	retryAfter := time.Duration(math.Ceil(retrySeconds)) * time.Second
	if retryAfter < time.Second {
		retryAfter = time.Second
	}

	return false, retryAfter
}

// GetConfig returns the current rate limit settings for the bucket.
func (tb *TokenBucket) GetConfig() (reqPerSec float64, burst int, enabled bool) {
	tb.mu.Lock()
	defer tb.mu.Unlock()
	return tb.refillRate, int(tb.capacity), tb.enabled
}

// SetConfig updates the rate limit settings for the bucket.
func (tb *TokenBucket) SetConfig(reqPerSec float64, burst int, enabled bool) {
	tb.mu.Lock()
	defer tb.mu.Unlock()

	if reqPerSec <= 0 {
		reqPerSec = 10.0
	}
	if burst <= 0 {
		burst = 20
	}

	tb.refillRate = reqPerSec
	tb.capacity = float64(burst)
	tb.enabled = enabled
	if tb.tokens > tb.capacity {
		tb.tokens = tb.capacity
	}
}
