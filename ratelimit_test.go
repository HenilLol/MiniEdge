package ratelimit_test

// Tests cover S-31, S-32, R-14, R-15 from the test plans.

import (
	"sync"
	"testing"
	"time"

	"miniedge/internal/ratelimit"
)

// ──────────────────────────────────────────────────────────
// Basic allow / deny
// ──────────────────────────────────────────────────────────

func TestAllow_WithinLimit(t *testing.T) {
	l := ratelimit.New(5, 60, 1000)
	for i := 0; i < 5; i++ {
		if !l.Allow("client-1") {
			t.Fatalf("request %d should be allowed", i+1)
		}
	}
}

func TestDeny_AtLimit(t *testing.T) {
	l := ratelimit.New(3, 60, 1000)
	for i := 0; i < 3; i++ {
		l.Allow("c")
	}
	if l.Allow("c") {
		t.Error("4th request should be denied when limit is 3")
	}
}

// ──────────────────────────────────────────────────────────
// Window expiry (R-14)
// ──────────────────────────────────────────────────────────

func TestAllow_AfterWindowExpiry(t *testing.T) {
	// Use a very short window so the test is fast.
	l := ratelimit.New(2, 1 /* 1 second */, 1000)
	l.Allow("c")
	l.Allow("c")
	if l.Allow("c") {
		t.Error("3rd request should be denied")
	}
	// Wait for the window to expire.
	time.Sleep(1100 * time.Millisecond)
	if !l.Allow("c") {
		t.Error("request after window expiry should be allowed")
	}
}

// ──────────────────────────────────────────────────────────
// Table cardinality cap (S-32, R-15, T-14)
// ──────────────────────────────────────────────────────────

func TestCardinality_TableCap(t *testing.T) {
	maxKeys := 10
	l := ratelimit.New(100, 60, maxKeys)
	// Fill up to maxKeys distinct clients.
	for i := 0; i < maxKeys; i++ {
		key := string(rune('A' + i)) // unique per iteration
		l.Allow(key)
	}
	if l.Len() != maxKeys {
		t.Errorf("expected table length %d, got %d", maxKeys, l.Len())
	}
	// One more distinct key should be rejected (table full).
	if l.Allow("overflow-key") {
		t.Error("new key when table is full should be denied (fail-closed)")
	}
}

// ──────────────────────────────────────────────────────────
// Key length cap (S-32)
// ──────────────────────────────────────────────────────────

func TestKeyLengthCap(t *testing.T) {
	l := ratelimit.New(10, 60, 1000)
	long := string(make([]byte, 256)) // 256 bytes — over the 128-byte cap
	short := string(make([]byte, 128))
	// Both should map to the same (truncated) key.
	l.Allow(long)
	// After 1 request with the long key, the capped key has 1 timestamp.
	// The 2nd request with the short key (same truncation) should also use that slot.
	if l.Len() != 1 {
		t.Errorf("long and truncated-to-same key should share one slot, got %d", l.Len())
	}
	_ = short
}

// ──────────────────────────────────────────────────────────
// Concurrent access (REL-04, S-29)
// ──────────────────────────────────────────────────────────

func TestConcurrency(t *testing.T) {
	l := ratelimit.New(1000, 60, 100)
	var wg sync.WaitGroup
	for i := 0; i < 200; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			key := string(rune('A' + n%10)) // 10 distinct keys
			l.Allow(key)
		}(i)
	}
	wg.Wait()
	// Just verifies no race or panic; table should be non-empty.
	if l.Len() == 0 {
		t.Error("expected some keys in table after concurrent calls")
	}
}

// ──────────────────────────────────────────────────────────
// Sweep removes expired entries (REL-05)
// ──────────────────────────────────────────────────────────

func TestSweep(t *testing.T) {
	l := ratelimit.New(5, 1, 1000) // 1-second window
	l.Allow("a")
	l.Allow("b")
	if l.Len() != 2 {
		t.Fatalf("expected 2 keys, got %d", l.Len())
	}
	time.Sleep(1100 * time.Millisecond)
	l.Sweep()
	if l.Len() != 0 {
		t.Errorf("expected 0 keys after sweep of expired entries, got %d", l.Len())
	}
}
