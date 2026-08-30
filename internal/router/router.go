package router

import (
	"sort"
	"strings"

	"miniedge/internal/model"
)

// PrefixRouter implements model.Router using longest-prefix matching.
type PrefixRouter struct {
	routes []model.Route
}

// NewPrefixRouter constructs a PrefixRouter from a list of configured routes.
// It pre-sorts routes by path length descending so that longest-prefix matches
// are consistently evaluated first.
func NewPrefixRouter(routes []model.Route) *PrefixRouter {
	compiled := make([]model.Route, len(routes))
	copy(compiled, routes)

	// Sort routes primarily by path length descending to enforce longest-prefix matching,
	// and secondarily by path string to guarantee deterministic order.
	sort.SliceStable(compiled, func(i, j int) bool {
		if len(compiled[i].Path) != len(compiled[j].Path) {
			return len(compiled[i].Path) > len(compiled[j].Path)
		}
		return compiled[i].Path < compiled[j].Path
	})

	return &PrefixRouter{
		routes: compiled,
	}
}

// Match finds the longest-prefix route matching the provided request path.
func (r *PrefixRouter) Match(path string) (model.Route, bool) {
	for _, route := range r.routes {
		if matchPath(path, route.Path) {
			return route, true
		}
	}
	return model.Route{}, false
}

// matchPath checks if reqPath satisfies the path boundary rules for routePath.
func matchPath(reqPath, routePath string) bool {
	if reqPath == routePath {
		return true
	}
	if !strings.HasPrefix(reqPath, routePath) {
		return false
	}
	// reqPath starts with routePath, but reqPath != routePath
	if strings.HasSuffix(routePath, "/") {
		return true
	}
	// routePath does not end with '/' (e.g. "/users")
	// Must enforce path boundary so "/usersXYZ" does not match "/users"
	return reqPath[len(routePath)] == '/'
}
