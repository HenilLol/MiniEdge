package ratelimit

import (
	"errors"
	"fmt"
	"sync"
	"time"

	"miniedge/internal/model"
)

var (
	ErrUnknownService = errors.New("unknown service ID")
	ErrInvalidRate    = errors.New("requests_per_second must be between 0.1 and 10000")
	ErrInvalidBurst   = errors.New("burst must be between 1 and 10000")
)

// RateLimitState represents rate limit settings for a service in API responses.
type RateLimitState struct {
	ServiceID         string  `json:"service_id"`
	RequestsPerSecond float64 `json:"requests_per_second"`
	Burst             int     `json:"burst"`
	Enabled           bool    `json:"enabled"`
}

// RateLimiterStore maintains thread-safe per-service token buckets.
type RateLimiterStore struct {
	mu            sync.RWMutex
	buckets       map[string]*TokenBucket
	knownServices map[string]struct{}
}

// NewRateLimiterStore constructs a RateLimiterStore initializing buckets with default settings (10.0 req/sec, burst 20, enabled).
func NewRateLimiterStore(services []model.Service) *RateLimiterStore {
	store := &RateLimiterStore{
		buckets:       make(map[string]*TokenBucket, len(services)),
		knownServices: make(map[string]struct{}, len(services)),
	}
	for _, svc := range services {
		store.knownServices[svc.ID] = struct{}{}
		store.buckets[svc.ID] = NewTokenBucket(10.0, 20, true)
	}
	return store
}

// Allow checks if a request is allowed for the given serviceID.
func (s *RateLimiterStore) Allow(serviceID string) (bool, time.Duration) {
	s.mu.RLock()
	bucket, ok := s.buckets[serviceID]
	s.mu.RUnlock()

	if !ok {
		s.mu.Lock()
		bucket, ok = s.buckets[serviceID]
		if !ok {
			bucket = NewTokenBucket(10.0, 20, true)
			s.buckets[serviceID] = bucket
		}
		s.mu.Unlock()
	}

	return bucket.Allow()
}

// Set updates rate limit parameters for a service.
func (s *RateLimiterStore) Set(serviceID string, reqPerSec float64, burst int, enabled bool) error {
	if reqPerSec <= 0 || reqPerSec > 10000 {
		return ErrInvalidRate
	}
	if burst <= 0 || burst > 10000 {
		return ErrInvalidBurst
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if len(s.knownServices) > 0 {
		if _, known := s.knownServices[serviceID]; !known {
			return fmt.Errorf("%w: %s", ErrUnknownService, serviceID)
		}
	}

	bucket, ok := s.buckets[serviceID]
	if !ok {
		bucket = NewTokenBucket(reqPerSec, burst, enabled)
		s.buckets[serviceID] = bucket
	} else {
		bucket.SetConfig(reqPerSec, burst, enabled)
	}

	return nil
}

// GetAll returns a detached, thread-safe snapshot of all service rate limit states.
func (s *RateLimiterStore) GetAll() map[string]RateLimitState {
	s.mu.RLock()
	defer s.mu.RUnlock()

	snapshot := make(map[string]RateLimitState, len(s.buckets))
	for svcID, bucket := range s.buckets {
		rate, burst, enabled := bucket.GetConfig()
		snapshot[svcID] = RateLimitState{
			ServiceID:         svcID,
			RequestsPerSecond: rate,
			Burst:             burst,
			Enabled:           enabled,
		}
	}
	return snapshot
}
