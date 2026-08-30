package gateway

import (
	"context"
	"encoding/json"
	"math"
	"net/http"
	"strconv"
	"strings"
	"time"

	"miniedge/internal/model"
	"miniedge/internal/proxy"
)

// RateLimiter evaluates whether a request to a service is allowed under capacity thresholds.
type RateLimiter interface {
	Allow(serviceID string) (allowed bool, retryAfter time.Duration)
}

// GatewayHandler coordinates request routing, service lookup, failure simulation, rate limiting, and proxy forwarding.
type GatewayHandler struct {
	router         model.Router
	registry       model.ServiceRegistry
	proxy          *proxy.ServiceProxy
	observer       model.RequestObserver
	simController  model.SimulationController
	rateLimiter    RateLimiter
	requestTimeout time.Duration
	apiHandler     http.Handler
}

// NewGatewayHandler constructs a GatewayHandler.
func NewGatewayHandler(
	router model.Router,
	registry model.ServiceRegistry,
	proxy *proxy.ServiceProxy,
	observer model.RequestObserver,
	requestTimeout time.Duration,
) *GatewayHandler {
	if requestTimeout <= 0 {
		requestTimeout = 10 * time.Second
	}
	return &GatewayHandler{
		router:         router,
		registry:       registry,
		proxy:          proxy,
		observer:       observer,
		requestTimeout: requestTimeout,
	}
}

// SetAPIHandler attaches an API HTTP handler for intercepting control paths (/api/...).
func (gh *GatewayHandler) SetAPIHandler(h http.Handler) {
	gh.apiHandler = h
}

// SetSimulationController attaches a SimulationController for controlled failure simulation.
func (gh *GatewayHandler) SetSimulationController(sc model.SimulationController) {
	gh.simController = sc
}

// SetRateLimiter attaches a RateLimiter for traffic throttling.
func (gh *GatewayHandler) SetRateLimiter(rl RateLimiter) {
	gh.rateLimiter = rl
}

func (gh *GatewayHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if (r.URL.Path == "/api" || strings.HasPrefix(r.URL.Path, "/api/")) && gh.apiHandler != nil {
		gh.apiHandler.ServeHTTP(w, r)
		return
	}
	reqID := GenerateRequestID()
	r.Header.Set("X-Request-ID", reqID)
	w.Header().Set("X-Request-ID", reqID)

	startTime := time.Now()
	sw := &statusResponseWriter{ResponseWriter: w, statusCode: http.StatusOK}

	var matchedServiceID string
	var errStr string

	defer func() {
		if gh.observer != nil {
			gh.observer.Observe(model.RequestEvent{
				RequestID: reqID,
				Timestamp: startTime,
				Method:    r.Method,
				Path:      r.URL.Path,
				ServiceID: matchedServiceID,
				Status:    sw.statusCode,
				Duration:  time.Since(startTime),
				Error:     errStr,
			})
		}
	}()

	route, found := gh.router.Match(r.URL.Path)
	if !found {
		sw.statusCode = http.StatusNotFound
		errStr = string(model.ErrCodeRouteNotFound)
		gh.writeJSONError(sw, http.StatusNotFound, model.ErrCodeRouteNotFound, "no route matches request path", reqID)
		return
	}

	matchedServiceID = route.ServiceID
	svc, found := gh.registry.Get(route.ServiceID)
	if !found {
		sw.statusCode = http.StatusBadGateway
		errStr = string(model.ErrCodeServiceUnavailable)
		gh.writeJSONError(sw, http.StatusBadGateway, model.ErrCodeServiceUnavailable, "configured service for route is missing", reqID)
		return
	}

	// M2.3 Controlled Failure Simulation Check
	if gh.simController != nil {
		if simState, ok := gh.simController.Get(svc.ID); ok {
			switch simState.Mode {
			case model.SimulationFail:
				sw.statusCode = http.StatusServiceUnavailable
				errStr = string(model.ErrCodeSimulationActive)
				gh.writeJSONError(sw, http.StatusServiceUnavailable, model.ErrCodeSimulationActive, "simulated service failure", reqID)
				return
			case model.SimulationDelay:
				if simState.Delay > 0 {
					timer := time.NewTimer(simState.Delay)
					select {
					case <-timer.C:
					case <-r.Context().Done():
						timer.Stop()
					}
				}
			}
		}
	}

	// M2.4 Rate Limiting Check
	if gh.rateLimiter != nil {
		if allowed, retryAfter := gh.rateLimiter.Allow(svc.ID); !allowed {
			sw.statusCode = http.StatusTooManyRequests
			errStr = string(model.ErrCodeRateLimitExceeded)
			retrySecs := int(math.Ceil(retryAfter.Seconds()))
			if retrySecs < 1 {
				retrySecs = 1
			}
			w.Header().Set("Retry-After", strconv.Itoa(retrySecs))
			gh.writeJSONError(sw, http.StatusTooManyRequests, model.ErrCodeRateLimitExceeded, "rate limit exceeded for service", reqID)
			return
		}
	}

	ctx, cancel := context.WithTimeout(r.Context(), gh.requestTimeout)
	defer cancel()

	gh.proxy.Forward(sw, r.WithContext(ctx), svc)
}

func (gh *GatewayHandler) writeJSONError(w http.ResponseWriter, status int, code model.ErrorCategory, message string, reqID string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{
		"error":      string(code),
		"message":    message,
		"request_id": reqID,
	})
}

type statusResponseWriter struct {
	http.ResponseWriter
	statusCode  int
	wroteHeader bool
}

func (s *statusResponseWriter) WriteHeader(code int) {
	if !s.wroteHeader {
		s.statusCode = code
		s.wroteHeader = true
		s.ResponseWriter.WriteHeader(code)
	}
}

func (s *statusResponseWriter) Write(b []byte) (int, error) {
	if !s.wroteHeader {
		s.wroteHeader = true
	}
	return s.ResponseWriter.Write(b)
}
