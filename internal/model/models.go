package model

import (
	"time"
)

// Config represents startup configuration loaded from JSON.
type Config struct {
	ListenAddr string    `json:"listen_addr"`
	Services   []Service `json:"services"`
	Routes     []Route   `json:"routes"`
}

// Service represents a configured backend service.
type Service struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	Upstream   string `json:"upstream"`
	HealthPath string `json:"health_path"`
}

// Route represents a path prefix to service ID mapping.
type Route struct {
	Path      string `json:"path"`
	ServiceID string `json:"service"`
}

// RequestEvent represents the observable result of a completed request.
type RequestEvent struct {
	RequestID string        `json:"request_id"`
	Timestamp time.Time     `json:"timestamp"`
	Method    string        `json:"method"`
	Path      string        `json:"path"`
	ServiceID string        `json:"service_id"`
	Status    int           `json:"status"`
	Duration  time.Duration `json:"duration"`
	Error     string        `json:"error,omitempty"`
}

// HealthStatus represents the conceptual health state of a service.
type HealthStatus string

const (
	HealthUnknown HealthStatus = "UNKNOWN"
	HealthUp      HealthStatus = "UP"
	HealthDown    HealthStatus = "DOWN"
	HealthSlow    HealthStatus = "SLOW"
)

// ServiceState represents runtime health information.
type ServiceState struct {
	ServiceID    string        `json:"service_id"`
	Status       HealthStatus  `json:"status"`
	LastChecked  time.Time     `json:"last_checked"`
	Latency      time.Duration `json:"latency"`
	FailureCount int           `json:"failure_count"`
}

// SimulationMode represents controlled failure simulation modes.
type SimulationMode string

const (
	SimulationNormal SimulationMode = "NORMAL"
	SimulationFail   SimulationMode = "FAIL"
	SimulationDelay  SimulationMode = "DELAY"
)

// SimulationState represents controlled failure simulation configuration/state.
type SimulationState struct {
	ServiceID string         `json:"service_id"`
	Mode      SimulationMode `json:"mode"`
	Delay     time.Duration  `json:"delay"`
}
