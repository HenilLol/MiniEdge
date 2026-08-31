// Package health implements the MiniEdge service health subsystem.
//
// It provides:
//   - StatusStore: thread-safe storage for per-service health state
//   - HealthChecker: periodic active probes + passive failure integration
//
// Security/reliability properties:
//   - All shared state protected by sync.RWMutex (concurrent reads, exclusive writes)
//   - Snapshots are value copies — callers cannot mutate internal state
//   - Constructor validates all targets before mutating store; rolls back on error
//   - Health-check latency is kept strictly separate from request latency
package health

import (
	"fmt"
	"sync"
	"time"
)

// ──────────────────────────────────────────────────────────
// Health state types
// ──────────────────────────────────────────────────────────

// Status is the health state of a service.
type Status string

const (
	StatusUnknown Status = "UNKNOWN" // registered but not yet probed
	StatusUp      Status = "UP"
	StatusDown    Status = "DOWN"
)

// ServiceHealth is the full health record for one service.
// All fields are value types so snapshots are safe without deep copying.
type ServiceHealth struct {
	Name                string
	Status              Status
	LastObservedAt      time.Time
	LatencyMs           int64 // last health-probe round-trip in ms
	ConsecutiveFailures int
	LastError           string // empty when UP
}

// ──────────────────────────────────────────────────────────
// StatusStore
// ──────────────────────────────────────────────────────────

// StatusStore is a thread-safe registry of service health records.
// It supports concurrent reads via RWMutex and provides snapshot isolation.
type StatusStore struct {
	mu      sync.RWMutex
	records map[string]*ServiceHealth
}

// NewStatusStore creates an empty store.
func NewStatusStore() *StatusStore {
	return &StatusStore{records: make(map[string]*ServiceHealth)}
}

// Register adds a new service to the store with StatusUnknown.
// Returns an error if the name is already registered.
func (s *StatusStore) Register(name string) error {
	if name == "" {
		return fmt.Errorf("service name must not be empty")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, exists := s.records[name]; exists {
		return fmt.Errorf("service %q is already registered", name)
	}
	s.records[name] = &ServiceHealth{
		Name:   name,
		Status: StatusUnknown,
	}
	return nil
}

// Deregister removes a service from the store.
// Returns an error if the service does not exist.
func (s *StatusStore) Deregister(name string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, exists := s.records[name]; !exists {
		return fmt.Errorf("service %q is not registered", name)
	}
	delete(s.records, name)
	return nil
}

// Update replaces the health record for a named service.
// Returns an error if the service is not registered.
func (s *StatusStore) Update(h ServiceHealth) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, exists := s.records[h.Name]; !exists {
		return fmt.Errorf("service %q is not registered", h.Name)
	}
	cp := h // store a copy
	s.records[h.Name] = &cp
	return nil
}

// Get returns a copy of the health record for the named service.
// Returns (zero, false) if not registered.
func (s *StatusStore) Get(name string) (ServiceHealth, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	r, ok := s.records[name]
	if !ok {
		return ServiceHealth{}, false
	}
	return *r, true // value copy — caller cannot mutate store
}

// Snapshot returns a copy of all current health records.
// The returned map is safe for the caller to read and iterate without locking.
func (s *StatusStore) Snapshot() map[string]ServiceHealth {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make(map[string]ServiceHealth, len(s.records))
	for k, v := range s.records {
		out[k] = *v // value copy
	}
	return out
}
