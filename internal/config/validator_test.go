package config_test

import (
	"strings"
	"testing"

	"miniedge/internal/config"
	"miniedge/internal/model"
)

func TestValidateConfig(t *testing.T) {
	validConfig := func() *model.Config {
		return &model.Config{
			ListenAddr: "127.0.0.1:8080",
			Services: []model.Service{
				{
					ID:         "users",
					Name:       "User Service",
					Upstream:   "http://127.0.0.1:3001/",
					HealthPath: "/health",
				},
				{
					ID:         "orders",
					Name:       "Order Service",
					Upstream:   "https://127.0.0.1:3002/",
					HealthPath: "/health",
				},
			},
			Routes: []model.Route{
				{
					Path:      "/users/",
					ServiceID: "users",
				},
				{
					Path:      "/orders/",
					ServiceID: "orders",
				},
			},
		}
	}

	t.Run("valid configuration", func(t *testing.T) {
		cfg := validConfig()
		if err := config.ValidateConfig(cfg); err != nil {
			t.Fatalf("expected valid config to pass validation, got: %v", err)
		}
	})

	t.Run("missing listen address", func(t *testing.T) {
		cfg := validConfig()
		cfg.ListenAddr = ""
		err := config.ValidateConfig(cfg)
		if err == nil || !strings.Contains(err.Error(), "missing listen address") {
			t.Fatalf("expected missing listen address error, got: %v", err)
		}
	})

	t.Run("empty service ID", func(t *testing.T) {
		cfg := validConfig()
		cfg.Services[0].ID = "   "
		err := config.ValidateConfig(cfg)
		if err == nil || !strings.Contains(err.Error(), "empty service ID") {
			t.Fatalf("expected empty service ID error, got: %v", err)
		}
	})

	t.Run("duplicate service ID", func(t *testing.T) {
		cfg := validConfig()
		cfg.Services[1].ID = "users"
		err := config.ValidateConfig(cfg)
		if err == nil || !strings.Contains(err.Error(), "duplicate service ID") {
			t.Fatalf("expected duplicate service ID error, got: %v", err)
		}
	})

	t.Run("invalid upstream URL", func(t *testing.T) {
		cfg := validConfig()
		cfg.Services[0].Upstream = "http://"
		err := config.ValidateConfig(cfg)
		if err == nil || !strings.Contains(err.Error(), "invalid upstream URL") {
			t.Fatalf("expected invalid upstream URL error, got: %v", err)
		}
	})

	t.Run("unsupported upstream scheme", func(t *testing.T) {
		cfg := validConfig()
		cfg.Services[0].Upstream = "ftp://127.0.0.1:3001/"
		err := config.ValidateConfig(cfg)
		if err == nil || !strings.Contains(err.Error(), "unsupported upstream URL scheme") {
			t.Fatalf("expected unsupported upstream scheme error, got: %v", err)
		}
	})

	t.Run("empty route path", func(t *testing.T) {
		cfg := validConfig()
		cfg.Routes[0].Path = ""
		err := config.ValidateConfig(cfg)
		if err == nil || !strings.Contains(err.Error(), "empty route path") {
			t.Fatalf("expected empty route path error, got: %v", err)
		}
	})

	t.Run("invalid route path", func(t *testing.T) {
		cfg := validConfig()
		cfg.Routes[0].Path = "users/"
		err := config.ValidateConfig(cfg)
		if err == nil || !strings.Contains(err.Error(), "invalid route path") {
			t.Fatalf("expected invalid route path error, got: %v", err)
		}
	})

	t.Run("duplicate route", func(t *testing.T) {
		cfg := validConfig()
		cfg.Routes[1].Path = "/users/"
		err := config.ValidateConfig(cfg)
		if err == nil || !strings.Contains(err.Error(), "duplicate route path") {
			t.Fatalf("expected duplicate route path error, got: %v", err)
		}
	})

	t.Run("route referencing unknown service", func(t *testing.T) {
		cfg := validConfig()
		cfg.Routes[0].ServiceID = "nonexistent"
		err := config.ValidateConfig(cfg)
		if err == nil || !strings.Contains(err.Error(), "route referencing unknown service") {
			t.Fatalf("expected unknown service reference error, got: %v", err)
		}
	})

	t.Run("ParseConfig JSON success", func(t *testing.T) {
		jsonStr := `{
			"listen_addr": "127.0.0.1:8080",
			"services": [
				{
					"id": "users",
					"name": "User Service",
					"upstream": "http://127.0.0.1:3001/",
					"health_path": "/health"
				}
			],
			"routes": [
				{
					"path": "/users/",
					"service": "users"
				}
			]
		}`
		cfg, err := config.ParseConfig([]byte(jsonStr))
		if err != nil {
			t.Fatalf("expected successful JSON config parsing, got error: %v", err)
		}
		if cfg.ListenAddr != "127.0.0.1:8080" {
			t.Errorf("unexpected listen_addr: %s", cfg.ListenAddr)
		}
	})
}
