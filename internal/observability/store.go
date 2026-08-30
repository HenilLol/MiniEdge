package observability

import (
	"sync"
	"time"

	"miniedge/internal/model"
)

// GlobalMetrics holds global cumulative performance and count metrics.
type GlobalMetrics struct {
	TotalRequests      int64   `json:"total_requests"`
	SuccessfulRequests int64   `json:"successful_requests"`
	ErrorRequests      int64   `json:"error_requests"`
	AvgLatencyMs       float64 `json:"avg_latency_ms"`
	MinLatencyMs       float64 `json:"min_latency_ms"`
	MaxLatencyMs       float64 `json:"max_latency_ms"`
}

// ServiceMetrics holds cumulative metrics for a specific backend service.
type ServiceMetrics struct {
	ServiceID          string        `json:"service_id"`
	TotalRequests      int64         `json:"total_requests"`
	SuccessfulRequests int64         `json:"successful_requests"`
	ErrorRequests      int64         `json:"error_requests"`
	AvgLatencyMs       float64       `json:"avg_latency_ms"`
	StatusCodes        map[int]int64 `json:"status_codes"`
}

// MetricsSnapshot represents a detached, thread-safe point-in-time metrics snapshot.
type MetricsSnapshot struct {
	Global   GlobalMetrics             `json:"global"`
	Services map[string]ServiceMetrics `json:"services"`
}

type serviceAccumulator struct {
	serviceID          string
	totalRequests      int64
	successfulRequests int64
	errorRequests      int64
	totalDuration      time.Duration
	statusCodes        map[int]int64
}

// ObservabilityStore maintains a bounded ring buffer of recent request logs
// and cumulative global/per-service metrics. It satisfies model.RequestObserver.
type ObservabilityStore struct {
	mu sync.RWMutex

	// Bounded Ring Buffer
	capacity int
	events   []model.RequestEvent
	head     int // next insertion index
	count    int // current count of stored items (0 <= count <= capacity)

	// Global Cumulative Metrics
	totalRequests      int64
	successfulRequests int64
	errorRequests      int64
	totalDuration      time.Duration
	minDuration        time.Duration
	maxDuration        time.Duration

	// Per-Service Cumulative Metrics
	services map[string]*serviceAccumulator
}

// DefaultCapacity is the default ring buffer capacity (100 events).
const DefaultCapacity = 100

// NewStore initializes an ObservabilityStore with the given ring buffer capacity.
// If capacity <= 0, DefaultCapacity (100) is used.
func NewStore(capacity int) *ObservabilityStore {
	if capacity <= 0 {
		capacity = DefaultCapacity
	}
	return &ObservabilityStore{
		capacity: capacity,
		events:   make([]model.RequestEvent, capacity),
		services: make(map[string]*serviceAccumulator),
	}
}

// ensureInitialized lazily initializes zero-value stores for safety.
func (s *ObservabilityStore) ensureInitialized() {
	if s.capacity == 0 {
		s.capacity = DefaultCapacity
		s.events = make([]model.RequestEvent, s.capacity)
	}
	if s.services == nil {
		s.services = make(map[string]*serviceAccumulator)
	}
}

// Observe records a completed RequestEvent into the bounded ring buffer and updates cumulative metrics.
func (s *ObservabilityStore) Observe(event model.RequestEvent) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.ensureInitialized()

	// 1. Store in ring buffer
	s.events[s.head] = event
	s.head = (s.head + 1) % s.capacity
	if s.count < s.capacity {
		s.count++
	}

	// 2. Update Global Cumulative Metrics
	s.totalRequests++
	s.totalDuration += event.Duration

	if isSuccessStatus(event.Status) {
		s.successfulRequests++
	} else if isErrorStatus(event.Status) {
		s.errorRequests++
	}

	if s.totalRequests == 1 {
		s.minDuration = event.Duration
		s.maxDuration = event.Duration
	} else {
		if event.Duration < s.minDuration {
			s.minDuration = event.Duration
		}
		if event.Duration > s.maxDuration {
			s.maxDuration = event.Duration
		}
	}

	// 3. Update Per-Service Cumulative Metrics
	if event.ServiceID != "" {
		svcAcc, exists := s.services[event.ServiceID]
		if !exists {
			svcAcc = &serviceAccumulator{
				serviceID:   event.ServiceID,
				statusCodes: make(map[int]int64),
			}
			s.services[event.ServiceID] = svcAcc
		}
		svcAcc.totalRequests++
		svcAcc.totalDuration += event.Duration
		if isSuccessStatus(event.Status) {
			svcAcc.successfulRequests++
		} else if isErrorStatus(event.Status) {
			svcAcc.errorRequests++
		}
		svcAcc.statusCodes[event.Status]++
	}
}

// GetLogs retrieves up to `limit` recent request logs matching optional `serviceID`, ordered newest-first.
// It returns a detached slice copy.
func (s *ObservabilityStore) GetLogs(limit int, serviceID string) []model.RequestEvent {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if limit <= 0 || s.count == 0 {
		return []model.RequestEvent{}
	}

	result := make([]model.RequestEvent, 0, min(limit, s.count))

	// Iterate backwards from newest to oldest in the ring buffer
	for i := 0; i < s.count; i++ {
		idx := (s.head - 1 - i + s.capacity*2) % s.capacity
		evt := s.events[idx]

		if serviceID == "" || evt.ServiceID == serviceID {
			result = append(result, evt)
			if len(result) == limit {
				break
			}
		}
	}

	return result
}

// GetMetrics generates a thread-safe, detached snapshot of global and per-service cumulative metrics.
func (s *ObservabilityStore) GetMetrics() MetricsSnapshot {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var global GlobalMetrics
	if s.totalRequests > 0 {
		global = GlobalMetrics{
			TotalRequests:      s.totalRequests,
			SuccessfulRequests: s.successfulRequests,
			ErrorRequests:      s.errorRequests,
			AvgLatencyMs:       durationToMs(s.totalDuration / time.Duration(s.totalRequests)),
			MinLatencyMs:       durationToMs(s.minDuration),
			MaxLatencyMs:       durationToMs(s.maxDuration),
		}
	}

	services := make(map[string]ServiceMetrics, len(s.services))
	for svcID, acc := range s.services {
		statusCodesCopy := make(map[int]int64, len(acc.statusCodes))
		for code, count := range acc.statusCodes {
			statusCodesCopy[code] = count
		}

		var avgMs float64
		if acc.totalRequests > 0 {
			avgMs = durationToMs(acc.totalDuration / time.Duration(acc.totalRequests))
		}

		services[svcID] = ServiceMetrics{
			ServiceID:          svcID,
			TotalRequests:      acc.totalRequests,
			SuccessfulRequests: acc.successfulRequests,
			ErrorRequests:      acc.errorRequests,
			AvgLatencyMs:       avgMs,
			StatusCodes:        statusCodesCopy,
		}
	}

	return MetricsSnapshot{
		Global:   global,
		Services: services,
	}
}

func isSuccessStatus(status int) bool {
	return status >= 200 && status <= 399
}

func isErrorStatus(status int) bool {
	return status >= 400 && status <= 599
}

func durationToMs(d time.Duration) float64 {
	return float64(d.Nanoseconds()) / 1e6
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
