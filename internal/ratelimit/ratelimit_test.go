package ratelimit_test

import (
	"sync"
	"testing"
	"time"

	"miniedge/internal/model"
	"miniedge/internal/ratelimit"
)

func TestTokenBucketExhaustionAndRefill(t *testing.T) {
	// Refill rate 10/sec, burst 2
	bucket := ratelimit.NewTokenBucket(10.0, 2, true)

	// Consume 2 tokens
	if ok, _ := bucket.Allow(); !ok {
		t.Error("expected first request to be allowed")
	}
	if ok, _ := bucket.Allow(); !ok {
		t.Error("expected second request to be allowed")
	}

	// 3rd request should be rejected
	if ok, retryAfter := bucket.Allow(); ok {
		t.Error("expected 3rd request to be rejected due to exhaustion")
	} else if retryAfter < time.Second {
		t.Errorf("expected retryAfter >= 1s, got %v", retryAfter)
	}

	// Wait 120ms to refill 1+ token (10 tokens/sec * 0.12s = 1.2 tokens)
	time.Sleep(120 * time.Millisecond)

	if ok, _ := bucket.Allow(); !ok {
		t.Error("expected request to be allowed after refill")
	}
}

func TestTokenBucketDisabled(t *testing.T) {
	bucket := ratelimit.NewTokenBucket(10.0, 1, false)

	// Even with capacity 1, all requests should be allowed when disabled
	for i := 0; i < 5; i++ {
		if ok, _ := bucket.Allow(); !ok {
			t.Errorf("expected request %d to be allowed when disabled", i)
		}
	}
}

func TestRateLimiterStoreSetAndGetAll(t *testing.T) {
	services := []model.Service{{ID: "users"}}
	store := ratelimit.NewRateLimiterStore(services)

	// Default state
	states := store.GetAll()
	if states["users"].RequestsPerSecond != 10.0 || states["users"].Burst != 20 || !states["users"].Enabled {
		t.Errorf("unexpected default state: %+v", states["users"])
	}

	// Update configuration
	err := store.Set("users", 50.0, 100, true)
	if err != nil {
		t.Fatalf("unexpected error setting rate limit: %v", err)
	}

	states = store.GetAll()
	if states["users"].RequestsPerSecond != 50.0 || states["users"].Burst != 100 {
		t.Errorf("unexpected updated state: %+v", states["users"])
	}
}

func TestRateLimiterStoreValidation(t *testing.T) {
	services := []model.Service{{ID: "users"}}
	store := ratelimit.NewRateLimiterStore(services)

	// Unknown service
	if err := store.Set("unknown", 10.0, 20, true); err == nil {
		t.Error("expected error setting rate limit for unknown service")
	}

	// Invalid rate
	if err := store.Set("users", 0, 20, true); err == nil {
		t.Error("expected error for rate <= 0")
	}
	if err := store.Set("users", 20000, 20, true); err == nil {
		t.Error("expected error for rate > 10000")
	}

	// Invalid burst
	if err := store.Set("users", 10.0, 0, true); err == nil {
		t.Error("expected error for burst <= 0")
	}
	if err := store.Set("users", 10.0, 20000, true); err == nil {
		t.Error("expected error for burst > 10000")
	}
}

func TestRateLimiterStoreDetachedSnapshot(t *testing.T) {
	services := []model.Service{{ID: "users"}}
	store := ratelimit.NewRateLimiterStore(services)

	snapshot := store.GetAll()
	snapshot["users"] = ratelimit.RateLimitState{RequestsPerSecond: 999.0}

	fresh := store.GetAll()
	if fresh["users"].RequestsPerSecond == 999.0 {
		t.Error("snapshot mutation leaked into internal store state!")
	}
}

func TestRateLimiterStoreConcurrentReadsWrites(t *testing.T) {
	services := []model.Service{{ID: "users"}}
	store := ratelimit.NewRateLimiterStore(services)

	var wg sync.WaitGroup
	done := make(chan struct{})

	// Writer
	wg.Add(1)
	go func() {
		defer wg.Done()
		idx := 1
		for {
			select {
			case <-done:
				return
			default:
				_ = store.Set("users", float64(idx%50+1), idx%100+1, true)
				idx++
			}
		}
	}()

	// Readers
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-done:
					return
				default:
					_, _ = store.Allow("users")
					_ = store.GetAll()
				}
			}
		}()
	}

	time.Sleep(100 * time.Millisecond)
	close(done)
	wg.Wait()
}
