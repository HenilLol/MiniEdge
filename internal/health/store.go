package health

import (
	"sync"
	"time"

	"miniedge/internal/model"
)

// HealthStore maintains thread-safe, in-memory runtime health states for configured services.
// It implements model.HealthProvider.
type HealthStore struct {
	mu       sync.RWMutex
	services map[string]model.ServiceState
}

// NewHealthStore initializes a HealthStore with configured services in UNKNOWN state.
func NewHealthStore(services []model.Service) *HealthStore {
	store := &HealthStore{
		services: make(map[string]model.ServiceState, len(services)),
	}
	for _, svc := range services {
		store.services[svc.ID] = model.ServiceState{
			ServiceID:    svc.ID,
			Status:       model.HealthUnknown,
			LastChecked:  time.Time{},
			Latency:      0,
			FailureCount: 0,
		}
	}
	return store
}

// Get resolves the ServiceState for a given serviceID (implements model.HealthProvider).
func (hs *HealthStore) Get(serviceID string) (model.ServiceState, bool) {
	hs.mu.RLock()
	defer hs.mu.RUnlock()

	state, ok := hs.services[serviceID]
	return state, ok
}

// GetAll returns a detached, thread-safe snapshot map of all service health states.
func (hs *HealthStore) GetAll() map[string]model.ServiceState {
	hs.mu.RLock()
	defer hs.mu.RUnlock()

	snapshot := make(map[string]model.ServiceState, len(hs.services))
	for k, v := range hs.services {
		snapshot[k] = v
	}
	return snapshot
}

// Update records a new health check outcome for a service.
// Consecutive failure count resets to 0 on UP/SLOW, and increments on DOWN.
func (hs *HealthStore) Update(serviceID string, status model.HealthStatus, checkedAt time.Time, latency time.Duration) {
	hs.mu.Lock()
	defer hs.mu.Unlock()

	state, exists := hs.services[serviceID]
	if !exists {
		state = model.ServiceState{
			ServiceID: serviceID,
		}
	}

	state.Status = status
	state.LastChecked = checkedAt
	state.Latency = latency

	if status == model.HealthUp || status == model.HealthSlow {
		state.FailureCount = 0
	} else if status == model.HealthDown {
		state.FailureCount++
	}

	hs.services[serviceID] = state
}
