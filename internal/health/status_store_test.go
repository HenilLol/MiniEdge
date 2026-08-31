package health_test

import (
	"sync"
	"testing"
	"time"

	"miniedge/internal/health"
)

// ──────────────────────────────────────────────────────────
// Register
// ──────────────────────────────────────────────────────────

func TestRegister_Success(t *testing.T) {
	s := health.NewStatusStore()
	if err := s.Register("svc-a"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	h, ok := s.Get("svc-a")
	if !ok {
		t.Fatal("expected service to be registered")
	}
	if h.Status != health.StatusUnknown {
		t.Errorf("expected UNKNOWN status, got %s", h.Status)
	}
}

func TestRegister_EmptyName(t *testing.T) {
	s := health.NewStatusStore()
	if err := s.Register(""); err == nil {
		t.Error("expected error for empty name")
	}
}

func TestRegister_Duplicate(t *testing.T) {
	s := health.NewStatusStore()
	_ = s.Register("svc-a")
	if err := s.Register("svc-a"); err == nil {
		t.Error("expected error for duplicate registration")
	}
}

// ──────────────────────────────────────────────────────────
// Deregister
// ──────────────────────────────────────────────────────────

func TestDeregister_Success(t *testing.T) {
	s := health.NewStatusStore()
	_ = s.Register("svc-a")
	if err := s.Deregister("svc-a"); err != nil {
		t.Fatalf("unexpected deregister error: %v", err)
	}
	if _, ok := s.Get("svc-a"); ok {
		t.Error("expected service to be gone after deregister")
	}
}

func TestDeregister_Missing(t *testing.T) {
	s := health.NewStatusStore()
	if err := s.Deregister("nonexistent"); err == nil {
		t.Error("expected error deregistering unknown service")
	}
}

// ──────────────────────────────────────────────────────────
// Update
// ──────────────────────────────────────────────────────────

func TestUpdate_Success(t *testing.T) {
	s := health.NewStatusStore()
	_ = s.Register("svc-a")
	h := health.ServiceHealth{
		Name:           "svc-a",
		Status:         health.StatusUp,
		LatencyMs:      42,
		LastObservedAt: time.Now(),
	}
	if err := s.Update(h); err != nil {
		t.Fatalf("unexpected update error: %v", err)
	}
	got, _ := s.Get("svc-a")
	if got.Status != health.StatusUp {
		t.Errorf("expected UP, got %s", got.Status)
	}
	if got.LatencyMs != 42 {
		t.Errorf("expected latency 42, got %d", got.LatencyMs)
	}
}

func TestUpdate_MissingService(t *testing.T) {
	s := health.NewStatusStore()
	err := s.Update(health.ServiceHealth{Name: "unknown"})
	if err == nil {
		t.Error("expected error updating unregistered service")
	}
}

// ──────────────────────────────────────────────────────────
// Snapshot isolation
// ──────────────────────────────────────────────────────────

func TestSnapshot_IsolatedCopy(t *testing.T) {
	s := health.NewStatusStore()
	_ = s.Register("svc-a")

	snap1 := s.Snapshot()
	// Mutate the store after the snapshot.
	_ = s.Update(health.ServiceHealth{
		Name:      "svc-a",
		Status:    health.StatusUp,
		LatencyMs: 99,
	})
	snap2 := s.Snapshot()

	// snap1 should still show UNKNOWN.
	if snap1["svc-a"].Status != health.StatusUnknown {
		t.Errorf("snap1 status should be UNKNOWN, got %s", snap1["svc-a"].Status)
	}
	// snap2 should show UP.
	if snap2["svc-a"].Status != health.StatusUp {
		t.Errorf("snap2 status should be UP, got %s", snap2["svc-a"].Status)
	}
}

func TestSnapshot_MutatingReturnDoesNotAffectStore(t *testing.T) {
	s := health.NewStatusStore()
	_ = s.Register("svc-a")
	snap := s.Snapshot()
	// Mutate the snapshot entry.
	entry := snap["svc-a"]
	entry.Status = health.StatusDown
	snap["svc-a"] = entry

	// The store should be unchanged.
	stored, _ := s.Get("svc-a")
	if stored.Status != health.StatusUnknown {
		t.Errorf("store should still be UNKNOWN, got %s", stored.Status)
	}
}

// ──────────────────────────────────────────────────────────
// Multiple service independence
// ──────────────────────────────────────────────────────────

func TestMultipleServicesIndependent(t *testing.T) {
	s := health.NewStatusStore()
	_ = s.Register("a")
	_ = s.Register("b")
	_ = s.Update(health.ServiceHealth{Name: "a", Status: health.StatusUp})
	_ = s.Update(health.ServiceHealth{Name: "b", Status: health.StatusDown})

	a, _ := s.Get("a")
	b, _ := s.Get("b")
	if a.Status != health.StatusUp {
		t.Errorf("a should be UP, got %s", a.Status)
	}
	if b.Status != health.StatusDown {
		t.Errorf("b should be DOWN, got %s", b.Status)
	}
}

// ──────────────────────────────────────────────────────────
// Concurrent operations (no race detector needed to catch obvious bugs)
// ──────────────────────────────────────────────────────────

func TestConcurrentOperations(t *testing.T) {
	s := health.NewStatusStore()
	_ = s.Register("svc")

	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(3)
		go func() {
			defer wg.Done()
			_ = s.Update(health.ServiceHealth{Name: "svc", Status: health.StatusUp})
		}()
		go func() {
			defer wg.Done()
			_, _ = s.Get("svc")
		}()
		go func() {
			defer wg.Done()
			_ = s.Snapshot()
		}()
	}
	wg.Wait()
}
