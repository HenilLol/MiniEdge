package router

import (
	"miniedge/internal/model"
)

// StaticServiceRegistry implements model.ServiceRegistry using an in-memory map.
type StaticServiceRegistry struct {
	services map[string]model.Service
}

// NewStaticServiceRegistry constructs a StaticServiceRegistry from configured services.
func NewStaticServiceRegistry(services []model.Service) *StaticServiceRegistry {
	svcMap := make(map[string]model.Service, len(services))
	for _, s := range services {
		svcMap[s.ID] = s
	}
	return &StaticServiceRegistry{
		services: svcMap,
	}
}

// Get resolves a ServiceID to its configured Service definition.
func (s *StaticServiceRegistry) Get(serviceID string) (model.Service, bool) {
	svc, ok := s.services[serviceID]
	return svc, ok
}
