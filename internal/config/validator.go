package config

import (
	"errors"
	"fmt"
	"net/url"
	"strings"

	"miniedge/internal/model"
)

// ValidateConfig validates startup configuration against MiniEdge contract rules.
func ValidateConfig(cfg *model.Config) error {
	if cfg == nil {
		return errors.New("config cannot be nil")
	}

	if strings.TrimSpace(cfg.ListenAddr) == "" {
		return errors.New("missing listen address")
	}

	serviceIDs := make(map[string]struct{}, len(cfg.Services))
	for _, svc := range cfg.Services {
		id := strings.TrimSpace(svc.ID)
		if id == "" {
			return errors.New("empty service ID")
		}
		if _, exists := serviceIDs[id]; exists {
			return fmt.Errorf("duplicate service ID: %s", id)
		}
		serviceIDs[id] = struct{}{}

		if strings.TrimSpace(svc.Upstream) == "" {
			return fmt.Errorf("empty upstream URL for service: %s", id)
		}

		u, err := url.Parse(svc.Upstream)
		if err != nil || u.Host == "" {
			return fmt.Errorf("invalid upstream URL for service '%s': %s", id, svc.Upstream)
		}

		scheme := strings.ToLower(u.Scheme)
		if scheme != "http" && scheme != "https" {
			return fmt.Errorf("unsupported upstream URL scheme '%s' for service '%s': must be http or https", scheme, id)
		}
	}

	routePaths := make(map[string]struct{}, len(cfg.Routes))
	for _, r := range cfg.Routes {
		path := strings.TrimSpace(r.Path)
		if path == "" {
			return errors.New("empty route path")
		}
		if !strings.HasPrefix(path, "/") {
			return fmt.Errorf("invalid route path: %s", path)
		}
		if _, exists := routePaths[path]; exists {
			return fmt.Errorf("duplicate route path: %s", path)
		}
		routePaths[path] = struct{}{}

		svcID := strings.TrimSpace(r.ServiceID)
		if _, exists := serviceIDs[svcID]; !exists {
			return fmt.Errorf("route referencing unknown service: %s", svcID)
		}
	}

	return nil
}
