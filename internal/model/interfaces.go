package model

// Router determines the matching route for a given request path using longest-prefix matching.
type Router interface {
	Match(path string) (Route, bool)
}

// ServiceRegistry resolves a ServiceID to a Service configuration.
type ServiceRegistry interface {
	Get(serviceID string) (Service, bool)
}

// RequestObserver receives completed request events.
type RequestObserver interface {
	Observe(event RequestEvent)
}

// HealthProvider exposes the current runtime health state of a service.
type HealthProvider interface {
	Get(serviceID string) (ServiceState, bool)
}

// SimulationController exposes the current simulation state for a service.
type SimulationController interface {
	Get(serviceID string) (SimulationState, bool)
}
