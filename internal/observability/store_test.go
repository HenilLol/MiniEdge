package observability_test

import (
	"sync"
	"testing"
	"time"

	"miniedge/internal/model"
	"miniedge/internal/observability"
)

// Test 1 — Empty Store
func TestEmptyStore(t *testing.T) {
	store := observability.NewStore(100)

	logs := store.GetLogs(10, "")
	if len(logs) != 0 {
		t.Fatalf("expected 0 logs from empty store, got %d", len(logs))
	}

	metrics := store.GetMetrics()
	if metrics.Global.TotalRequests != 0 || metrics.Global.SuccessfulRequests != 0 || metrics.Global.ErrorRequests != 0 {
		t.Errorf("expected zero global metrics, got %+v", metrics.Global)
	}
	if len(metrics.Services) != 0 {
		t.Errorf("expected empty services map, got %d entries", len(metrics.Services))
	}
}

// Test 2 — Observe Single Event
func TestObserveSingleEvent(t *testing.T) {
	store := observability.NewStore(100)
	now := time.Now()

	event := model.RequestEvent{
		RequestID: "req_1",
		Timestamp: now,
		Method:    "GET",
		Path:      "/users/42",
		ServiceID: "users",
		Status:    200,
		Duration:  10 * time.Millisecond,
	}
	store.Observe(event)

	logs := store.GetLogs(10, "")
	if len(logs) != 1 {
		t.Fatalf("expected 1 log, got %d", len(logs))
	}
	if logs[0].RequestID != "req_1" || logs[0].ServiceID != "users" || logs[0].Status != 200 {
		t.Errorf("unexpected log data: %+v", logs[0])
	}

	m := store.GetMetrics()
	if m.Global.TotalRequests != 1 || m.Global.SuccessfulRequests != 1 || m.Global.ErrorRequests != 0 {
		t.Errorf("unexpected global metrics: %+v", m.Global)
	}
	if m.Global.MinLatencyMs != 10.0 || m.Global.MaxLatencyMs != 10.0 || m.Global.AvgLatencyMs != 10.0 {
		t.Errorf("unexpected global latency metrics: min=%.2f, max=%.2f, avg=%.2f", m.Global.MinLatencyMs, m.Global.MaxLatencyMs, m.Global.AvgLatencyMs)
	}

	userSvc, exists := m.Services["users"]
	if !exists {
		t.Fatal("expected service metrics for 'users'")
	}
	if userSvc.TotalRequests != 1 || userSvc.SuccessfulRequests != 1 || userSvc.ErrorRequests != 0 {
		t.Errorf("unexpected user service metrics: %+v", userSvc)
	}
}

// Test 3 — Multiple Events
func TestMultipleEvents(t *testing.T) {
	store := observability.NewStore(100)

	store.Observe(model.RequestEvent{RequestID: "req_1", ServiceID: "users", Status: 200, Duration: 10 * time.Millisecond})
	store.Observe(model.RequestEvent{RequestID: "req_2", ServiceID: "orders", Status: 201, Duration: 20 * time.Millisecond})
	store.Observe(model.RequestEvent{RequestID: "req_3", ServiceID: "users", Status: 500, Duration: 30 * time.Millisecond})

	m := store.GetMetrics()
	if m.Global.TotalRequests != 3 {
		t.Errorf("expected total 3, got %d", m.Global.TotalRequests)
	}
	if m.Global.SuccessfulRequests != 2 {
		t.Errorf("expected successful 2, got %d", m.Global.SuccessfulRequests)
	}
	if m.Global.ErrorRequests != 1 {
		t.Errorf("expected error 1, got %d", m.Global.ErrorRequests)
	}

	if len(m.Services) != 2 {
		t.Errorf("expected 2 service entries, got %d", len(m.Services))
	}
}

// Test 4 — Error Classification
func TestErrorClassification(t *testing.T) {
	store := observability.NewStore(100)

	statuses := []int{200, 201, 301, 399, 400, 404, 500, 502, 599}
	for i, s := range statuses {
		store.Observe(model.RequestEvent{
			RequestID: string(rune('a' + i)),
			ServiceID: "svc",
			Status:    s,
			Duration:  5 * time.Millisecond,
		})
	}

	m := store.GetMetrics()
	if m.Global.TotalRequests != 9 {
		t.Errorf("expected 9 total requests, got %d", m.Global.TotalRequests)
	}
	if m.Global.SuccessfulRequests != 4 {
		t.Errorf("expected 4 successful (200, 201, 301, 399), got %d", m.Global.SuccessfulRequests)
	}
	if m.Global.ErrorRequests != 5 {
		t.Errorf("expected 5 errors (400, 404, 500, 502, 599), got %d", m.Global.ErrorRequests)
	}
}

// Test 5 — Status Code Counts
func TestStatusCodeCounts(t *testing.T) {
	store := observability.NewStore(100)

	store.Observe(model.RequestEvent{ServiceID: "users", Status: 200})
	store.Observe(model.RequestEvent{ServiceID: "users", Status: 200})
	store.Observe(model.RequestEvent{ServiceID: "users", Status: 404})
	store.Observe(model.RequestEvent{ServiceID: "users", Status: 502})

	m := store.GetMetrics()
	userSvc := m.Services["users"]

	if userSvc.StatusCodes[200] != 2 {
		t.Errorf("expected status 200 count = 2, got %d", userSvc.StatusCodes[200])
	}
	if userSvc.StatusCodes[404] != 1 {
		t.Errorf("expected status 404 count = 1, got %d", userSvc.StatusCodes[404])
	}
	if userSvc.StatusCodes[502] != 1 {
		t.Errorf("expected status 502 count = 1, got %d", userSvc.StatusCodes[502])
	}
}

// Test 6 — Average Latency
func TestAverageLatency(t *testing.T) {
	store := observability.NewStore(100)

	store.Observe(model.RequestEvent{ServiceID: "svc", Status: 200, Duration: 10 * time.Millisecond})
	store.Observe(model.RequestEvent{ServiceID: "svc", Status: 200, Duration: 20 * time.Millisecond})
	store.Observe(model.RequestEvent{ServiceID: "svc", Status: 200, Duration: 30 * time.Millisecond})

	m := store.GetMetrics()
	if m.Global.AvgLatencyMs != 20.0 {
		t.Errorf("expected avg latency 20.0 ms, got %.2f", m.Global.AvgLatencyMs)
	}
}

// Test 7 — Min/Max Latency
func TestMinMaxLatency(t *testing.T) {
	store := observability.NewStore(100)

	store.Observe(model.RequestEvent{ServiceID: "svc", Status: 200, Duration: 10 * time.Millisecond})
	store.Observe(model.RequestEvent{ServiceID: "svc", Status: 200, Duration: 20 * time.Millisecond})
	store.Observe(model.RequestEvent{ServiceID: "svc", Status: 200, Duration: 5 * time.Millisecond})

	m := store.GetMetrics()
	if m.Global.MinLatencyMs != 5.0 {
		t.Errorf("expected min latency 5.0 ms, got %.2f", m.Global.MinLatencyMs)
	}
	if m.Global.MaxLatencyMs != 20.0 {
		t.Errorf("expected max latency 20.0 ms, got %.2f", m.Global.MaxLatencyMs)
	}
}

// Test 8 — Ring Buffer Capacity
func TestRingBufferCapacity(t *testing.T) {
	store := observability.NewStore(3)

	events := []string{"A", "B", "C", "D", "E"}
	for _, id := range events {
		store.Observe(model.RequestEvent{RequestID: id, ServiceID: "svc", Status: 200})
	}

	logs := store.GetLogs(10, "")
	if len(logs) != 3 {
		t.Fatalf("expected exactly 3 logs retained, got %d", len(logs))
	}

	// Should return newest to oldest: E, D, C
	expected := []string{"E", "D", "C"}
	for i, exp := range expected {
		if logs[i].RequestID != exp {
			t.Errorf("at index %d expected %s, got %s", i, exp, logs[i].RequestID)
		}
	}
}

// Test 9 — Log Ordering (Newest First)
func TestLogOrdering(t *testing.T) {
	store := observability.NewStore(100)

	store.Observe(model.RequestEvent{RequestID: "req_1", ServiceID: "svc", Status: 200})
	store.Observe(model.RequestEvent{RequestID: "req_2", ServiceID: "svc", Status: 200})
	store.Observe(model.RequestEvent{RequestID: "req_3", ServiceID: "svc", Status: 200})

	logs := store.GetLogs(10, "")
	if len(logs) != 3 {
		t.Fatalf("expected 3 logs, got %d", len(logs))
	}
	if logs[0].RequestID != "req_3" || logs[1].RequestID != "req_2" || logs[2].RequestID != "req_1" {
		t.Errorf("unexpected ordering: %s, %s, %s", logs[0].RequestID, logs[1].RequestID, logs[2].RequestID)
	}
}

// Test 10 — Service Filter
func TestServiceFilter(t *testing.T) {
	store := observability.NewStore(100)

	store.Observe(model.RequestEvent{RequestID: "1", ServiceID: "users", Status: 200})
	store.Observe(model.RequestEvent{RequestID: "2", ServiceID: "orders", Status: 200})
	store.Observe(model.RequestEvent{RequestID: "3", ServiceID: "users", Status: 200})

	userLogs := store.GetLogs(10, "users")
	if len(userLogs) != 2 {
		t.Fatalf("expected 2 user logs, got %d", len(userLogs))
	}
	if userLogs[0].RequestID != "3" || userLogs[1].RequestID != "1" {
		t.Errorf("unexpected filtered user logs: %+v", userLogs)
	}

	orderLogs := store.GetLogs(10, "orders")
	if len(orderLogs) != 1 {
		t.Fatalf("expected 1 order log, got %d", len(orderLogs))
	}
	if orderLogs[0].RequestID != "2" {
		t.Errorf("unexpected filtered order log: %+v", orderLogs[0])
	}
}

// Test 11 — Limit
func TestLimit(t *testing.T) {
	store := observability.NewStore(100)

	for i := 1; i <= 5; i++ {
		store.Observe(model.RequestEvent{RequestID: string(rune('0' + i)), ServiceID: "svc", Status: 200})
	}

	t.Run("limit <= 0", func(t *testing.T) {
		logs := store.GetLogs(0, "")
		if len(logs) != 0 {
			t.Errorf("expected 0 logs for limit=0, got %d", len(logs))
		}
		logsNeg := store.GetLogs(-5, "")
		if len(logsNeg) != 0 {
			t.Errorf("expected 0 logs for limit=-5, got %d", len(logsNeg))
		}
	})

	t.Run("limit = 2", func(t *testing.T) {
		logs := store.GetLogs(2, "")
		if len(logs) != 2 {
			t.Fatalf("expected 2 logs, got %d", len(logs))
		}
	})

	t.Run("limit > available", func(t *testing.T) {
		logs := store.GetLogs(100, "")
		if len(logs) != 5 {
			t.Fatalf("expected 5 logs, got %d", len(logs))
		}
	})
}

// Test 12 — Detached Snapshot Logs
func TestDetachedSnapshotLogs(t *testing.T) {
	store := observability.NewStore(100)

	store.Observe(model.RequestEvent{RequestID: "original", ServiceID: "users", Status: 200})

	logs1 := store.GetLogs(10, "")
	logs1[0].RequestID = "MUTATED"
	logs1[0].Status = 500

	logs2 := store.GetLogs(10, "")
	if logs2[0].RequestID != "original" || logs2[0].Status != 200 {
		t.Errorf("store internal state was mutated via returned log slice! got %+v", logs2[0])
	}
}

// Test 13 — Detached Snapshot Metrics
func TestDetachedSnapshotMetrics(t *testing.T) {
	store := observability.NewStore(100)

	store.Observe(model.RequestEvent{ServiceID: "users", Status: 200})

	m1 := store.GetMetrics()
	m1.Global.TotalRequests = 999
	m1.Services["users"].StatusCodes[200] = 999

	m2 := store.GetMetrics()
	if m2.Global.TotalRequests != 1 {
		t.Errorf("global metrics mutated! expected 1, got %d", m2.Global.TotalRequests)
	}
	if m2.Services["users"].StatusCodes[200] != 1 {
		t.Errorf("service status codes mutated! expected 1, got %d", m2.Services["users"].StatusCodes[200])
	}
}

// Test 14 — Cumulative Metrics vs Ring Buffer
func TestCumulativeMetricsVsRingBuffer(t *testing.T) {
	store := observability.NewStore(3) // small capacity

	for i := 1; i <= 5; i++ {
		store.Observe(model.RequestEvent{RequestID: string(rune('0' + i)), ServiceID: "svc", Status: 200})
	}

	logs := store.GetLogs(10, "")
	if len(logs) != 3 {
		t.Fatalf("expected ring buffer to cap logs at 3, got %d", len(logs))
	}

	m := store.GetMetrics()
	if m.Global.TotalRequests != 5 {
		t.Fatalf("expected cumulative total requests to be 5, got %d", m.Global.TotalRequests)
	}
}

// Test 15 — Concurrent Observe
func TestConcurrentObserve(t *testing.T) {
	store := observability.NewStore(50)

	const numGoroutines = 20
	const requestsPerGoroutine = 50

	var wg sync.WaitGroup
	for g := 0; g < numGoroutines; g++ {
		wg.Add(1)
		go func(gID int) {
			defer wg.Done()
			for i := 0; i < requestsPerGoroutine; i++ {
				store.Observe(model.RequestEvent{
					RequestID: "concurrent_req",
					ServiceID: "users",
					Status:    200,
					Duration:  1 * time.Millisecond,
				})
			}
		}(g)
	}
	wg.Wait()

	m := store.GetMetrics()
	expectedTotal := int64(numGoroutines * requestsPerGoroutine)
	if m.Global.TotalRequests != expectedTotal {
		t.Fatalf("expected total requests %d, got %d", expectedTotal, m.Global.TotalRequests)
	}

	logs := store.GetLogs(100, "")
	if len(logs) != 50 {
		t.Fatalf("expected ring buffer capped at 50, got %d", len(logs))
	}
}

// Test 16 — Concurrent Reads and Writes
func TestConcurrentReadsWrites(t *testing.T) {
	store := observability.NewStore(50)

	var wg sync.WaitGroup
	done := make(chan struct{})

	// Writer goroutines
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-done:
					return
				default:
					store.Observe(model.RequestEvent{
						RequestID: "rw_req",
						ServiceID: "orders",
						Status:    200,
						Duration:  2 * time.Millisecond,
					})
				}
			}
		}()
	}

	// Reader goroutines
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-done:
					return
				default:
					_ = store.GetLogs(10, "orders")
					_ = store.GetMetrics()
				}
			}
		}()
	}

	time.Sleep(100 * time.Millisecond)
	close(done)
	wg.Wait()

	m := store.GetMetrics()
	if m.Global.TotalRequests == 0 {
		t.Error("expected non-zero total requests after concurrent read/write test")
	}
}
