package metrics_test

import (
	"sync"
	"testing"

	"miniedge/internal/metrics"
)

func TestGlobal_Counters(t *testing.T) {
	r := metrics.New()
	r.RecordRequest("svc", 10, false)
	r.RecordRequest("svc", 20, true)

	g := r.GlobalSnapshot()
	if g.TotalRequests != 2 {
		t.Errorf("expected 2 total requests, got %d", g.TotalRequests)
	}
	if g.TotalErrors != 1 {
		t.Errorf("expected 1 error, got %d", g.TotalErrors)
	}
	if g.TotalLatencyMs != 30 {
		t.Errorf("expected 30ms total latency, got %d", g.TotalLatencyMs)
	}
	if g.AverageLatencyMs != 15 {
		t.Errorf("expected 15ms average latency, got %d", g.AverageLatencyMs)
	}
}

func TestGlobal_RateLimited(t *testing.T) {
	r := metrics.New()
	r.RecordRateLimited()
	r.RecordRateLimited()
	g := r.GlobalSnapshot()
	if g.RateLimited != 2 {
		t.Errorf("expected 2 rate-limited, got %d", g.RateLimited)
	}
}

func TestPerService_Counters(t *testing.T) {
	r := metrics.New()
	r.RecordRequest("alpha", 10, false)
	r.RecordRequest("alpha", 30, true)
	r.RecordRequest("beta", 5, false)

	alpha, ok := r.ServiceSnapshot("alpha")
	if !ok {
		t.Fatal("expected alpha snapshot")
	}
	if alpha.Requests != 2 {
		t.Errorf("alpha: expected 2 requests, got %d", alpha.Requests)
	}
	if alpha.Errors != 1 {
		t.Errorf("alpha: expected 1 error, got %d", alpha.Errors)
	}
	if alpha.TotalLatencyMs != 40 {
		t.Errorf("alpha: expected 40ms total, got %d", alpha.TotalLatencyMs)
	}
	if alpha.AverageLatencyMs != 20 {
		t.Errorf("alpha: expected 20ms avg, got %d", alpha.AverageLatencyMs)
	}
	if alpha.LastLatencyMs != 30 {
		t.Errorf("alpha: expected last latency 30, got %d", alpha.LastLatencyMs)
	}

	beta, ok := r.ServiceSnapshot("beta")
	if !ok {
		t.Fatal("expected beta snapshot")
	}
	if beta.Requests != 1 {
		t.Errorf("beta: expected 1 request, got %d", beta.Requests)
	}
}

func TestServiceSnapshot_Missing(t *testing.T) {
	r := metrics.New()
	_, ok := r.ServiceSnapshot("nonexistent")
	if ok {
		t.Error("expected false for unknown service")
	}
}

func TestServiceSnapshots_All(t *testing.T) {
	r := metrics.New()
	r.RecordRequest("a", 1, false)
	r.RecordRequest("b", 2, false)
	snaps := r.ServiceSnapshots()
	if len(snaps) != 2 {
		t.Errorf("expected 2 service snapshots, got %d", len(snaps))
	}
}

func TestSnapshotIsolation(t *testing.T) {
	r := metrics.New()
	r.RecordRequest("svc", 10, false)
	g1 := r.GlobalSnapshot()
	r.RecordRequest("svc", 10, false)
	g2 := r.GlobalSnapshot()

	if g1.TotalRequests != 1 {
		t.Errorf("g1 should have 1 request, got %d", g1.TotalRequests)
	}
	if g2.TotalRequests != 2 {
		t.Errorf("g2 should have 2 requests, got %d", g2.TotalRequests)
	}
}

func TestConcurrentUpdates(t *testing.T) {
	r := metrics.New()
	var wg sync.WaitGroup
	for i := 0; i < 200; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			svc := "svc-a"
			if n%2 == 0 {
				svc = "svc-b"
			}
			r.RecordRequest(svc, int64(n), n%3 == 0)
		}(i)
	}
	wg.Wait()

	g := r.GlobalSnapshot()
	if g.TotalRequests != 200 {
		t.Errorf("expected 200 total requests after concurrent updates, got %d", g.TotalRequests)
	}
}
