// Package metrics provides in-process request metrics for MiniEdge.
																													//
																													// Design:
																													//   - Global counters use sync/atomic for lock-free increment.
																													//   - Per-service counters use a mutex-protected map (cardinality is bounded
																													//     by the number of configured services — not unbounded user input).
																													//   - Snapshots return immutable value copies; callers cannot mutate internal state.
																													//   - Health-check latency is NOT tracked here (kept in the health package).
																													//
																													// This is intentionally small. No Prometheus client, no histograms, no percentiles.
																													package metrics

																													import (
																														"sync"
																														"sync/atomic"
																													)

																													// ──────────────────────────────────────────────────────────
																													// Global metrics
																													// ──────────────────────────────────────────────────────────

																													// GlobalSnapshot is an immutable copy of the global counters.
																													type GlobalSnapshot struct {
																														TotalRequests    int64
																														TotalErrors      int64
																														TotalLatencyMs   int64
																														AverageLatencyMs int64
																														RateLimited      int64
																													}

																													// ──────────────────────────────────────────────────────────
																													// Per-service metrics
																													// ──────────────────────────────────────────────────────────

																													// ServiceSnapshot is an immutable copy of one service's counters.
																													type ServiceSnapshot struct {
																														Name             string
																														Requests         int64
																														Errors           int64
																														TotalLatencyMs   int64
																														AverageLatencyMs int64
																														LastLatencyMs    int64
																													}

																													// serviceRecord holds the mutable counters for one service.
																													// Protected by the Recorder's mutex.
																													type serviceRecord struct {
																														requests       int64
																														errors         int64
																														totalLatencyMs int64
																														lastLatencyMs  int64
																													}

																													// ──────────────────────────────────────────────────────────
																													// Recorder
																													// ──────────────────────────────────────────────────────────

																													// Recorder is the central metrics collector.
																													type Recorder struct {
																														// Global atomics — no mutex needed for simple increment/read.
																														totalRequests  atomic.Int64
																														totalErrors    atomic.Int64
																														totalLatencyMs atomic.Int64
																														rateLimited    atomic.Int64

																														// Per-service records — mutex-protected map.
																														mu       sync.Mutex
																														services map[string]*serviceRecord
																													}

																													// New creates a Recorder with an empty service map.
																													func New() *Recorder {
																														return &Recorder{services: make(map[string]*serviceRecord)}
																													}

																													// RecordRequest records one completed request for the named service.
																													//   - latencyMs: total request round-trip (client → edge → upstream → client).
																													//   - isError: true if the upstream returned an error or non-2xx was forwarded as error.
																													func (r *Recorder) RecordRequest(service string, latencyMs int64, isError bool) {
																														r.totalRequests.Add(1)
																														r.totalLatencyMs.Add(latencyMs)
																														if isError {
																															r.totalErrors.Add(1)
																														}

																														r.mu.Lock()
																														rec, ok := r.services[service]
																														if !ok {
																															rec = &serviceRecord{}
																															r.services[service] = rec
																														}
																														rec.requests++
																														rec.totalLatencyMs += latencyMs
																														rec.lastLatencyMs = latencyMs
																														if isError {
																															rec.errors++
																														}
																														r.mu.Unlock()
																													}

																													// RecordRateLimited increments the global rate-limited counter.
																													func (r *Recorder) RecordRateLimited() {
																														r.rateLimited.Add(1)
																													}

																													// GlobalSnapshot returns an immutable copy of the global counters.
																													func (r *Recorder) GlobalSnapshot() GlobalSnapshot {
																														total := r.totalRequests.Load()
																														lat := r.totalLatencyMs.Load()
																														var avg int64
																														if total > 0 {
																															avg = lat / total
																														}
																														return GlobalSnapshot{
																															TotalRequests:    total,
																															TotalErrors:      r.totalErrors.Load(),
																															TotalLatencyMs:   lat,
																															AverageLatencyMs: avg,
																															RateLimited:      r.rateLimited.Load(),
																														}
																													}

																													// ServiceSnapshots returns a slice of immutable per-service snapshots.
																													// The order is not guaranteed.
																													func (r *Recorder) ServiceSnapshots() []ServiceSnapshot {
																														r.mu.Lock()
																														defer r.mu.Unlock()

																														out := make([]ServiceSnapshot, 0, len(r.services))
																														for name, rec := range r.services {
																															var avg int64
																															if rec.requests > 0 {
																																avg = rec.totalLatencyMs / rec.requests
																															}
																															out = append(out, ServiceSnapshot{
																																Name:             name,
																																Requests:         rec.requests,
																																Errors:           rec.errors,
																																TotalLatencyMs:   rec.totalLatencyMs,
																																AverageLatencyMs: avg,
																																LastLatencyMs:    rec.lastLatencyMs,
																															})
																														}
																														return out
																													}

																													// ServiceSnapshot returns one service's snapshot. Returns (zero, false) if not found.
																													func (r *Recorder) ServiceSnapshot(name string) (ServiceSnapshot, bool) {
																														r.mu.Lock()
																														defer r.mu.Unlock()
																														rec, ok := r.services[name]
																														if !ok {
																															return ServiceSnapshot{}, false
																														}
																														var avg int64
																														if rec.requests > 0 {
																															avg = rec.totalLatencyMs / rec.requests
																														}
																														return ServiceSnapshot{
																															Name:             name,
																															Requests:         rec.requests,
																															Errors:           rec.errors,
																															TotalLatencyMs:   rec.totalLatencyMs,
																															AverageLatencyMs: avg,
																															LastLatencyMs:    rec.lastLatencyMs,
																														}, true
																													}
