package router_test

import (
	"testing"

	"miniedge/internal/model"
	"miniedge/internal/router"
)

func TestPrefixRouter(t *testing.T) {
	routes := []model.Route{
		{Path: "/users/", ServiceID: "user-service"},
		{Path: "/users/admin/", ServiceID: "admin-service"},
		{Path: "/orders", ServiceID: "order-service"},
		{Path: "/events/v1/", ServiceID: "event-service"},
	}

	r := router.NewPrefixRouter(routes)

	t.Run("exact prefix match", func(t *testing.T) {
		matched, found := r.Match("/users/")
		if !found {
			t.Fatal("expected match for /users/, got not found")
		}
		if matched.ServiceID != "user-service" {
			t.Errorf("expected user-service, got %s", matched.ServiceID)
		}
	})

	t.Run("normal prefix match", func(t *testing.T) {
		matched, found := r.Match("/users/42")
		if !found {
			t.Fatal("expected match for /users/42, got not found")
		}
		if matched.ServiceID != "user-service" {
			t.Errorf("expected user-service, got %s", matched.ServiceID)
		}

		matched, found = r.Match("/users/profile/settings")
		if !found {
			t.Fatal("expected match for /users/profile/settings, got not found")
		}
		if matched.ServiceID != "user-service" {
			t.Errorf("expected user-service, got %s", matched.ServiceID)
		}
	})

	t.Run("longest prefix match - more specific route wins", func(t *testing.T) {
		matched, found := r.Match("/users/admin/profile")
		if !found {
			t.Fatal("expected match for /users/admin/profile, got not found")
		}
		if matched.ServiceID != "admin-service" {
			t.Errorf("expected admin-service (longest prefix /users/admin/), got %s", matched.ServiceID)
		}
	})

	t.Run("no match", func(t *testing.T) {
		_, found := r.Match("/products/10")
		if found {
			t.Fatal("expected no match for /products/10")
		}
	})

	t.Run("word boundary check - /users/ does not match /usersXYZ", func(t *testing.T) {
		_, found := r.Match("/usersXYZ")
		if found {
			t.Fatal("expected /users/ NOT to match /usersXYZ")
		}
	})

	t.Run("explicit rule - /users/ does not automatically match /users", func(t *testing.T) {
		_, found := r.Match("/users")
		if found {
			t.Fatal("expected /users/ NOT to match /users")
		}
	})

	t.Run("route without trailing slash - /orders matches /orders and /orders/123, but not /ordersXYZ", func(t *testing.T) {
		matched, found := r.Match("/orders")
		if !found || matched.ServiceID != "order-service" {
			t.Errorf("expected /orders to match order-service, got %v, found=%v", matched, found)
		}

		matched, found = r.Match("/orders/123")
		if !found || matched.ServiceID != "order-service" {
			t.Errorf("expected /orders/123 to match order-service, got %v, found=%v", matched, found)
		}

		_, found = r.Match("/ordersXYZ")
		if found {
			t.Error("expected /orders NOT to match /ordersXYZ")
		}
	})

	t.Run("overlapping routes and deterministic behavior", func(t *testing.T) {
		overlapping := []model.Route{
			{Path: "/api/v1/", ServiceID: "v1-service"},
			{Path: "/api/v1/auth/", ServiceID: "auth-service"},
			{Path: "/api/", ServiceID: "general-api-service"},
		}
		rOverlapping := router.NewPrefixRouter(overlapping)

		// /api/v1/auth/login -> should match auth-service (/api/v1/auth/)
		m, ok := rOverlapping.Match("/api/v1/auth/login")
		if !ok || m.ServiceID != "auth-service" {
			t.Errorf("expected auth-service, got %v, ok=%v", m, ok)
		}

		// /api/v1/users -> should match v1-service (/api/v1/)
		m, ok = rOverlapping.Match("/api/v1/users")
		if !ok || m.ServiceID != "v1-service" {
			t.Errorf("expected v1-service, got %v, ok=%v", m, ok)
		}

		// /api/health -> should match general-api-service (/api/)
		m, ok = rOverlapping.Match("/api/health")
		if !ok || m.ServiceID != "general-api-service" {
			t.Errorf("expected general-api-service, got %v, ok=%v", m, ok)
		}
	})
}
