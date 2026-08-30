package health_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"miniedge/internal/health"
	"miniedge/internal/model"
)

// Test 1 — HealthStore Initial State & UNKNOWN
func TestHealthStoreInitialState(t *testing.T) {
	services := []model.Service{
		{ID: "users", Name: "User Service", Upstream: "http://127.0.0.1:3001", HealthPath: "/health"},
		{ID: "orders", Name: "Order Service", Upstream: "http://127.0.0.1:3002", HealthPath: "/health"},
	}

	store := health.NewHealthStore(services)

	u, ok := store.Get("users")
	if !ok || u.Status != model.HealthUnknown || u.FailureCount != 0 {
		t.Errorf("expected UNKNOWN status and 0 failure count, got %+v", u)
	}

	all := store.GetAll()
	if len(all) != 2 {
		t.Errorf("expected 2 services in GetAll, got %d", len(all))
	}
}

// Test 2 — Store Failure Counting & Resets
func TestHealthStoreFailureCounting(t *testing.T) {
	services := []model.Service{{ID: "users"}}
	store := health.NewHealthStore(services)
	now := time.Now()

	// 1. First DOWN -> FailureCount = 1
	store.Update("users", model.HealthDown, now, 10*time.Millisecond)
	u, _ := store.Get("users")
	if u.Status != model.HealthDown || u.FailureCount != 1 {
		t.Errorf("expected DOWN and FailureCount=1, got status=%s, count=%d", u.Status, u.FailureCount)
	}

	// 2. Second DOWN -> FailureCount = 2
	store.Update("users", model.HealthDown, now, 10*time.Millisecond)
	u, _ = store.Get("users")
	if u.Status != model.HealthDown || u.FailureCount != 2 {
		t.Errorf("expected DOWN and FailureCount=2, got status=%s, count=%d", u.Status, u.FailureCount)
	}

	// 3. Successful UP -> FailureCount resets to 0
	store.Update("users", model.HealthUp, now, 15*time.Millisecond)
	u, _ = store.Get("users")
	if u.Status != model.HealthUp || u.FailureCount != 0 {
		t.Errorf("expected UP and FailureCount=0, got status=%s, count=%d", u.Status, u.FailureCount)
	}

	// 4. Successful SLOW -> FailureCount remains 0
	store.Update("users", model.HealthSlow, now, 600*time.Millisecond)
	u, _ = store.Get("users")
	if u.Status != model.HealthSlow || u.FailureCount != 0 {
		t.Errorf("expected SLOW and FailureCount=0, got status=%s, count=%d", u.Status, u.FailureCount)
	}
}

// Test 3 — Checker Status Classifications (UP, SLOW, DOWN, Connection Refused, Timeout)
func TestCheckerStatusClassification(t *testing.T) {
	checker := health.NewChecker(200*time.Millisecond, 50*time.Millisecond)

	t.Run("200 fast -> UP", func(t *testing.T) {
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if ua := r.Header.Get("User-Agent"); ua != "MiniEdge-HealthChecker/1.0" {
				t.Errorf("unexpected User-Agent header: %s", ua)
			}
			w.WriteHeader(http.StatusOK)
		}))
		defer ts.Close()

		res := checker.Check(context.Background(), model.Service{ID: "s1", Upstream: ts.URL, HealthPath: "/health"})
		if res.Status != model.HealthUp {
			t.Errorf("expected UP, got %s", res.Status)
		}
	})

	t.Run("200 slow -> SLOW", func(t *testing.T) {
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			time.Sleep(80 * time.Millisecond)
			w.WriteHeader(http.StatusOK)
		}))
		defer ts.Close()

		res := checker.Check(context.Background(), model.Service{ID: "s1", Upstream: ts.URL, HealthPath: "/health"})
		if res.Status != model.HealthSlow {
			t.Errorf("expected SLOW, got %s", res.Status)
		}
	})

	t.Run("4xx/5xx -> DOWN", func(t *testing.T) {
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
		}))
		defer ts.Close()

		res := checker.Check(context.Background(), model.Service{ID: "s1", Upstream: ts.URL, HealthPath: "/health"})
		if res.Status != model.HealthDown {
			t.Errorf("expected DOWN for 500 status, got %s", res.Status)
		}
	})

	t.Run("connection refused -> DOWN", func(t *testing.T) {
		res := checker.Check(context.Background(), model.Service{ID: "s1", Upstream: "http://127.0.0.1:59999", HealthPath: "/health"})
		if res.Status != model.HealthDown {
			t.Errorf("expected DOWN for unused port, got %s", res.Status)
		}
	})

	t.Run("timeout -> DOWN", func(t *testing.T) {
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			time.Sleep(300 * time.Millisecond)
			w.WriteHeader(http.StatusOK)
		}))
		defer ts.Close()

		res := checker.Check(context.Background(), model.Service{ID: "s1", Upstream: ts.URL, HealthPath: "/health"})
		if res.Status != model.HealthDown {
			t.Errorf("expected DOWN for timeout, got %s", res.Status)
		}
	})
}

// Test 4 — URL Joining Helper
func TestBuildHealthURL(t *testing.T) {
	tests := []struct {
		upstream   string
		healthPath string
		expected   string
	}{
		{"http://127.0.0.1:3001", "/health", "http://127.0.0.1:3001/health"},
		{"http://127.0.0.1:3001/", "/health", "http://127.0.0.1:3001/health"},
		{"http://127.0.0.1:3001", "health", "http://127.0.0.1:3001/health"},
		{"http://127.0.0.1:3001/", "health", "http://127.0.0.1:3001/health"},
	}

	for _, tt := range tests {
		got, err := health.BuildHealthURL(tt.upstream, tt.healthPath)
		if err != nil {
			t.Errorf("unexpected error for %s + %s: %v", tt.upstream, tt.healthPath, err)
		}
		if got != tt.expected {
			t.Errorf("for %s + %s expected %s, got %s", tt.upstream, tt.healthPath, tt.expected, got)
		}
	}
}

// Test 5 — Worker Lifecycle, Immediate Probe, Periodic Ticks, and Error Isolation
func TestWorkerLifecycle(t *testing.T) {
	s1Calls := 0
	s1 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s1Calls++
		w.WriteHeader(http.StatusOK)
	}))
	defer s1.Close()

	// s2 points to closed port (fails)
	services := []model.Service{
		{ID: "svc1", Upstream: s1.URL, HealthPath: "/health"},
		{ID: "svc2", Upstream: "http://127.0.0.1:59999", HealthPath: "/health"},
	}

	store := health.NewHealthStore(services)
	checker := health.NewChecker(200*time.Millisecond, 50*time.Millisecond)

	worker := health.NewWorker(services, store, checker, 100*time.Millisecond)

	// Start worker -> triggers immediate check
	worker.Start()

	// Verify immediate check results
	st1, _ := store.Get("svc1")
	st2, _ := store.Get("svc2")

	if st1.Status != model.HealthUp {
		t.Errorf("expected immediate check UP for svc1, got %s", st1.Status)
	}
	if st2.Status != model.HealthDown || st2.FailureCount != 1 {
		t.Errorf("expected immediate check DOWN for svc2 (error isolation), got status=%s, failures=%d", st2.Status, st2.FailureCount)
	}

	// Wait for periodic ticker ticks
	time.Sleep(250 * time.Millisecond)

	if s1Calls < 2 {
		t.Errorf("expected periodic check ticks, got %d s1 calls", s1Calls)
	}

	// Stop worker cleanly without goroutine leak
	worker.Stop()
}

// Test 6 — Concurrent Store Reads & Writes
func TestConcurrentStoreReadsWrites(t *testing.T) {
	services := []model.Service{{ID: "users"}}
	store := health.NewHealthStore(services)

	var wg sync.WaitGroup
	done := make(chan struct{})

	// Writer
	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			select {
			case <-done:
				return
			default:
				store.Update("users", model.HealthUp, time.Now(), 10*time.Millisecond)
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
					_, _ = store.Get("users")
					_ = store.GetAll()
				}
			}
		}()
	}

	time.Sleep(100 * time.Millisecond)
	close(done)
	wg.Wait()
}
