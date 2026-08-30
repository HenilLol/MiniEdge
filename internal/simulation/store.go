package simulation

import (
	"errors"
	"fmt"
	"sync"
	"time"

	"miniedge/internal/model"
)

var (
	ErrUnknownService = errors.New("unknown service ID")
	ErrInvalidMode    = errors.New("mode must be NORMAL, FAIL, or DELAY")
	ErrInvalidDelay   = errors.New("delay must be between 0 and 30000ms")
)

// SimulationStore maintains thread-safe controlled failure simulation states for services.
// It implements model.SimulationController.
type SimulationStore struct {
	mu            sync.RWMutex
	services      map[string]model.SimulationState
	knownServices map[string]struct{}
}

// NewSimulationStore constructs a SimulationStore initializing all configured services to NORMAL mode with zero delay.
func NewSimulationStore(services []model.Service) *SimulationStore {
	store := &SimulationStore{
		services:      make(map[string]model.SimulationState, len(services)),
		knownServices: make(map[string]struct{}, len(services)),
	}
	for _, svc := range services {
		store.knownServices[svc.ID] = struct{}{}
		store.services[svc.ID] = model.SimulationState{
			ServiceID: svc.ID,
			Mode:      model.SimulationNormal,
			Delay:     0,
		}
	}
	return store
}

// Get resolves the SimulationState for a given serviceID (implements model.SimulationController).
func (ss *SimulationStore) Get(serviceID string) (model.SimulationState, bool) {
	ss.mu.RLock()
	defer ss.mu.RUnlock()

	state, ok := ss.services[serviceID]
	if !ok {
		if _, known := ss.knownServices[serviceID]; known {
			return model.SimulationState{
				ServiceID: serviceID,
				Mode:      model.SimulationNormal,
				Delay:     0,
			}, true
		}
		// Default to NORMAL mode for unlisted services if no static knownServices list
		if len(ss.knownServices) == 0 {
			return model.SimulationState{
				ServiceID: serviceID,
				Mode:      model.SimulationNormal,
				Delay:     0,
			}, true
		}
		return model.SimulationState{}, false
	}
	return state, true
}

// Set updates the simulation mode and delay duration for a service.
func (ss *SimulationStore) Set(serviceID string, mode model.SimulationMode, delay time.Duration) error {
	ss.mu.Lock()
	defer ss.mu.Unlock()

	if len(ss.knownServices) > 0 {
		if _, known := ss.knownServices[serviceID]; !known {
			return fmt.Errorf("%w: %s", ErrUnknownService, serviceID)
		}
	}

	switch mode {
	case model.SimulationNormal, model.SimulationFail, model.SimulationDelay:
	default:
		return fmt.Errorf("%w: %s", ErrInvalidMode, string(mode))
	}

	if delay < 0 || delay > 30*time.Second {
		return ErrInvalidDelay
	}

	if mode == model.SimulationDelay && delay <= 0 {
		return fmt.Errorf("%w: DELAY mode requires delay > 0", ErrInvalidDelay)
	}

	if mode != model.SimulationDelay {
		delay = 0
	}

	ss.services[serviceID] = model.SimulationState{
		ServiceID: serviceID,
		Mode:      mode,
		Delay:     delay,
	}

	return nil
}

// GetAll returns a detached, thread-safe snapshot map of all service simulation states.
func (ss *SimulationStore) GetAll() map[string]model.SimulationState {
	ss.mu.RLock()
	defer ss.mu.RUnlock()

	snapshot := make(map[string]model.SimulationState, len(ss.services))
	for k, v := range ss.services {
		snapshot[k] = v
	}
	return snapshot
}

// ResetAll resets all services to NORMAL mode with zero delay.
func (ss *SimulationStore) ResetAll() {
	ss.mu.Lock()
	defer ss.mu.Unlock()

	for k := range ss.services {
		ss.services[k] = model.SimulationState{
			ServiceID: k,
			Mode:      model.SimulationNormal,
			Delay:     0,
		}
	}
}
