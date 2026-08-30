// Package ratelimit implements a simple, bounded, in-memory, per-process
// sliding-window rate limiter using only the standard library (D-06, REL-09).
//
// Keys are caller-supplied strings (e.g. client IP). The table is bounded by
// maxKeys; when full, new keys are rejected rather than evicting existing entries
// (fail-closed). Expired entries are lazily swept during Allow and explicitly
// during Sweep.
package ratelimit

import (
	"sync"
	"time"
)

const (
	maxKeyBytes = 128 // per-key length cap (SEC-05)
)

// entry tracks the timestamps of requests within the current window.
type entry struct {
	timestamps []time.Time
}

// Limiter is a concurrency-safe sliding-window rate limiter.
type Limiter struct {
	mu      sync.Mutex
	table   map[string]*entry
	limit   int           // max requests per window
	window  time.Duration // window duration
	maxKeys int           // maximum number of keys stored
}

// New creates a Limiter with the given configuration.
// limit: max allowed requests per window; must be > 0.
// windowSeconds: window duration; must be > 0.
// maxKeys: maximum tracked keys; must be > 0.
func New(limit, windowSeconds, maxKeys int) *Limiter {
	if limit <= 0 {
		limit = 60
	}
	if windowSeconds <= 0 {
		windowSeconds = 60
	}
	if maxKeys <= 0 {
		maxKeys = 10_000
	}
	return &Limiter{
		table:   make(map[string]*entry),
		limit:   limit,
		window:  time.Duration(windowSeconds) * time.Second,
		maxKeys: maxKeys,
	}
}

// Allow returns true if the key is within the rate limit, false otherwise.
// The key is capped to maxKeyBytes to prevent table pollution (SEC-05, T-14).
func (l *Limiter) Allow(key string) bool {
	if len(key) > maxKeyBytes {
		key = key[:maxKeyBytes]
	}
	now := time.Now()
	cutoff := now.Add(-l.window)

	l.mu.Lock()
	defer l.mu.Unlock()

	e, exists := l.table[key]
	if !exists {
		// New key: check table cardinality before inserting (T-14).
		if len(l.table) >= l.maxKeys {
			// Table full — reject new key fail-closed.
			return false
		}
		e = &entry{}
		l.table[key] = e
	}

	// Evict timestamps outside the window (lazy sweep).
	valid := e.timestamps[:0]
	for _, ts := range e.timestamps {
		if ts.After(cutoff) {
			valid = append(valid, ts)
		}
	}
	e.timestamps = valid

	if len(e.timestamps) >= l.limit {
		return false
	}
	e.timestamps = append(e.timestamps, now)
	return true
}

// Sweep removes all expired entries from the table, bounding memory use.
// Call periodically (e.g. every window duration) from a background goroutine.
func (l *Limiter) Sweep() {
	cutoff := time.Now().Add(-l.window)
	l.mu.Lock()
	defer l.mu.Unlock()
	for key, e := range l.table {
		valid := e.timestamps[:0]
		for _, ts := range e.timestamps {
			if ts.After(cutoff) {
				valid = append(valid, ts)
			}
		}
		if len(valid) == 0 {
			delete(l.table, key)
		} else {
			e.timestamps = valid
		}
	}
}

// Len returns the current number of tracked keys (for observability/testing).
func (l *Limiter) Len() int {
	l.mu.Lock()
	defer l.mu.Unlock()
	return len(l.table)
}
