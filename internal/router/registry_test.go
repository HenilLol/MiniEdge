package router_test

import (
	"testing"

	"miniedge/internal/model"
	"miniedge/internal/router"
)

func TestStaticServiceRegistry(t *testing.T) {
	services := []model.Service{
		{
			ID:         "users",
			Name:       "User Service",
			Upstream:   "http://127.0.0.1:3001/",
			HealthPath: "/health",
		},
		{
			ID:         "orders",
			Name:       "Order Service",
			Upstream:   "http://127.0.0.1:3002/",
			HealthPath: "/health",
		},
	}

	reg := router.NewStaticServiceRegistry(services)

	t.Run("existing service lookup", func(t *testing.T) {
		svc, ok := reg.Get("users")
		if !ok {
			t.Fatal("expected service 'users' to be found")
		}
		if svc.Name != "User Service" {
			t.Errorf("expected Name 'User Service', got '%s'", svc.Name)
		}
		if svc.Upstream != "http://127.0.0.1:3001/" {
			t.Errorf("expected Upstream 'http://127.0.0.1:3001/', got '%s'", svc.Upstream)
		}
	})

	t.Run("missing service lookup", func(t *testing.T) {
		_, ok := reg.Get("nonexistent")
		if ok {
			t.Fatal("expected service 'nonexistent' to NOT be found")
		}
	})
}
