// Package simulation implements bounded, service-scoped, automatically-expiring
// failure simulation state (D-07, REL-10, SEC-08). It is disabled by default
// and requires explicit operator activation. All operations are concurrency-safe.
package simulation

import (
	"fmt"
	"sync"
	"time"
)

// Mode is an enumerated simulation behaviour.
type Mode string

const (
	ModeDelay Mode = "delay" // inject artificial latency
	ModeError Mode = "error" // return an error response immediately
)

// ActiveSim holds the parameters for one active simulation on a service.
type ActiveSim struct {
	ServiceID string
	Mode      Mode
	DelayMs   int64
	ExpiresAt time.Time
}

// IsExpired returns true if the simulation's deadline has passed.
func (s *ActiveSim) IsExpired() bool {
	return time.Now().After(s.ExpiresAt)
}

// State is the concurrency-safe simulation registry.
type State struct {
	mu   sync.Mutex
	sims map[string]*ActiveSim // keyed by service ID
}

// NewState creates an empty, disabled simulation state.
func NewState() *State {
	return &State{sims: make(map[string]*ActiveSim)}
}

// Activate installs a simulation for the given service, respecting all bounds.
//
// Parameters are validated against the caller-supplied maximums (from config)
// to prevent unbounded latency or duration (T-13, D-07).
func (st *State) Activate(
	serviceID string,
	mode Mode,
	delayMs int64,
	durationSec int64,
	maxDelayMs int64,
	maxDurationSec int64,
	allowedServices map[string]bool,
) error {
	if !allowedServices[serviceID] {
		return fmt.Errorf("simulation not allowed for service %q", serviceID)
	}
	if mode != ModeDelay && mode != ModeError {
		return fmt.Errorf("simulation mode %q is not supported", mode)
	}
	if delayMs < 0 || delayMs > maxDelayMs {
		return fmt.Errorf("simulation delayMs %d exceeds maximum %d", delayMs, maxDelayMs)
	}
	if durationSec <= 0 || durationSec > maxDurationSec {
		return fmt.Errorf("simulation durationSeconds %d must be 1–%d", durationSec, maxDurationSec)
	}

	sim := &ActiveSim{
		ServiceID: serviceID,
		Mode:      mode,
		DelayMs:   delayMs,
		ExpiresAt: time.Now().Add(time.Duration(durationSec) * time.Second),
	}

	st.mu.Lock()
	defer st.mu.Unlock()
	st.sims[serviceID] = sim
	return nil
}

// Deactivate removes any active simulation for the service.
func (st *State) Deactivate(serviceID string) {
	st.mu.Lock()
	defer st.mu.Unlock()
	delete(st.sims, serviceID)
}

// Get returns the active simulation for the service, or nil if none / expired.
// Expired entries are lazily removed.
func (st *State) Get(serviceID string) *ActiveSim {
	st.mu.Lock()
	defer st.mu.Unlock()
	sim, ok := st.sims[serviceID]
	if !ok {
		return nil
	}
	if sim.IsExpired() {
		delete(st.sims, serviceID)
		return nil
	}
	return sim
}

// Sweep removes all expired simulations. Call periodically or at shutdown.
func (st *State) Sweep() {
	st.mu.Lock()
	defer st.mu.Unlock()
	for id, sim := range st.sims {
		if sim.IsExpired() {
			delete(st.sims, id)
		}
	}
}

// Active returns a snapshot of currently active (non-expired) simulations.
func (st *State) Active() []ActiveSim {
	st.mu.Lock()
	defer st.mu.Unlock()
	var out []ActiveSim
	for id, sim := range st.sims {
		if !sim.IsExpired() {
			out = append(out, *sim)
		} else {
			delete(st.sims, id)
		}
	}
	return out
}
