package api

import (
	"crypto/subtle"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"miniedge/internal/model"
	"miniedge/internal/observability"
	"miniedge/internal/ratelimit"
	"miniedge/internal/simulation"
)

// RequestLogDTO represents the API response DTO for a single request log.
type RequestLogDTO struct {
	RequestID  string  `json:"request_id"`
	Timestamp  string  `json:"timestamp"`
	Method     string  `json:"method"`
	Path       string  `json:"path"`
	ServiceID  string  `json:"service_id"`
	Status     int     `json:"status"`
	DurationMs float64 `json:"duration_ms"`
	Error      string  `json:"error"`
}

// LogsResponse represents the API response schema for GET /api/logs.
type LogsResponse struct {
	Requests []RequestLogDTO `json:"requests"`
	Total    int             `json:"total"`
}

// GlobalMetricsDTO represents global metrics in GET /api/metrics.
type GlobalMetricsDTO struct {
	TotalRequests      int64   `json:"total_requests"`
	SuccessfulRequests int64   `json:"successful_requests"`
	ErrorRequests      int64   `json:"error_requests"`
	AvgLatencyMs       float64 `json:"avg_latency_ms"`
	MinLatencyMs       float64 `json:"min_latency_ms"`
	MaxLatencyMs       float64 `json:"max_latency_ms"`
}

// ServiceMetricsDTO represents per-service metrics in GET /api/metrics.
type ServiceMetricsDTO struct {
	ServiceID          string           `json:"service_id"`
	TotalRequests      int64            `json:"total_requests"`
	SuccessfulRequests int64            `json:"successful_requests"`
	ErrorRequests      int64            `json:"error_requests"`
	AvgLatencyMs       float64          `json:"avg_latency_ms"`
	StatusCodes        map[string]int64 `json:"status_codes"`
}

// MetricsResponse represents the API response schema for GET /api/metrics.
type MetricsResponse struct {
	Global   GlobalMetricsDTO             `json:"global"`
	Services map[string]ServiceMetricsDTO `json:"services"`
}

// ServiceStateDTO represents service health state in GET /api/health.
type ServiceStateDTO struct {
	ServiceID    string  `json:"service_id"`
	Status       string  `json:"status"`
	LastChecked  string  `json:"last_checked"`
	LatencyMs    float64 `json:"latency_ms"`
	FailureCount int     `json:"failure_count"`
}

// HealthResponse represents the API response schema for GET /api/health.
type HealthResponse struct {
	Services map[string]ServiceStateDTO `json:"services"`
}

// SimulationStateDTO represents simulation configuration for a service.
type SimulationStateDTO struct {
	ServiceID string `json:"service_id"`
	Mode      string `json:"mode"`
	DelayMs   int64  `json:"delay_ms"`
}

// SimulationsResponse represents the response schema for GET /api/simulations.
type SimulationsResponse struct {
	Services map[string]SimulationStateDTO `json:"services"`
}

// SetSimulationRequest represents the request payload for POST /api/simulations.
type SetSimulationRequest struct {
	ServiceID string `json:"service_id"`
	Mode      string `json:"mode"`
	DelayMs   int64  `json:"delay_ms"`
}

// SetSimulationResponse represents the success payload for POST /api/simulations.
type SetSimulationResponse struct {
	Status     string             `json:"status"`
	Simulation SimulationStateDTO `json:"simulation"`
}

// RateLimitDTO represents rate limit settings for a service.
type RateLimitDTO struct {
	ServiceID         string  `json:"service_id"`
	RequestsPerSecond float64 `json:"requests_per_second"`
	Burst             int     `json:"burst"`
	Enabled           bool    `json:"enabled"`
}

// RateLimitsResponse represents the response schema for GET /api/ratelimits.
type RateLimitsResponse struct {
	Services map[string]RateLimitDTO `json:"services"`
}

// SetRateLimitRequest represents the request payload for POST /api/ratelimits.
type SetRateLimitRequest struct {
	ServiceID         string  `json:"service_id"`
	RequestsPerSecond float64 `json:"requests_per_second"`
	Burst             int     `json:"burst"`
	Enabled           bool    `json:"enabled"`
}

// SetRateLimitResponse represents the success payload for POST /api/ratelimits.
type SetRateLimitResponse struct {
	Status    string       `json:"status"`
	RateLimit RateLimitDTO `json:"ratelimit"`
}

// Handler handles control REST endpoints (/api/logs, /api/metrics, /api/health, /api/simulations, /api/ratelimits).
type Handler struct {
	store         *observability.ObservabilityStore
	healthStore   model.HealthProvider
	simStore      *simulation.SimulationStore
	rlStore       *ratelimit.RateLimiterStore
	apiKey        string
	allowedOrigin string
}

// NewHandler creates a new API Handler instance around ObservabilityStore and optional HealthProvider.
func NewHandler(store *observability.ObservabilityStore, healthProvider ...model.HealthProvider) *Handler {
	h := &Handler{
		store:         store,
		allowedOrigin: "http://localhost:3000",
	}
	if len(healthProvider) > 0 {
		h.healthStore = healthProvider[0]
	}
	return h
}

// SetAPIKey sets the administrative API key required for POST mutation requests.
func (h *Handler) SetAPIKey(key string) {
	h.apiKey = key
}

// SetAllowedOrigin configures the CORS Access-Control-Allow-Origin header value.
func (h *Handler) SetAllowedOrigin(origin string) {
	if origin != "" {
		h.allowedOrigin = origin
	}
}

// SetHealthProvider sets or updates the HealthProvider dependency.
func (h *Handler) SetHealthProvider(hp model.HealthProvider) {
	h.healthStore = hp
}

// SetSimulationStore sets or updates the SimulationStore dependency.
func (h *Handler) SetSimulationStore(ss *simulation.SimulationStore) {
	h.simStore = ss
}

// SetRateLimiterStore sets or updates the RateLimiterStore dependency.
func (h *Handler) SetRateLimiterStore(rl *ratelimit.RateLimiterStore) {
	h.rlStore = rl
}

func (h *Handler) authenticate(r *http.Request) bool {
	if h.apiKey == "" {
		return false
	}
	clientKey := r.Header.Get("X-API-Key")
	if clientKey == "" {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(clientKey), []byte(h.apiKey)) == 1
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	origin := h.allowedOrigin
	if origin == "" {
		origin = "http://localhost:3000"
	}
	w.Header().Set("Access-Control-Allow-Origin", origin)
	w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type, X-API-Key, X-Request-ID")
	w.Header().Set("Access-Control-Max-Age", "86400")

	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	path := r.URL.Path

	switch path {
	case "/api/logs":
		h.handleLogs(w, r)
	case "/api/metrics":
		h.handleMetrics(w, r)
	case "/api/health":
		h.handleHealth(w, r)
	case "/api/simulations":
		h.handleSimulations(w, r)
	case "/api/ratelimits":
		h.handleRateLimits(w, r)
	default:
		writeJSONError(w, http.StatusNotFound, string(model.ErrCodeRouteNotFound), "API endpoint not found")
	}
}

func (h *Handler) handleLogs(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", "GET")
		writeJSONError(w, http.StatusMethodNotAllowed, "method_not_allowed", "only GET method is supported")
		return
	}

	limitStr := r.URL.Query().Get("limit")
	serviceID := r.URL.Query().Get("service")

	limit := 50
	if limitStr != "" {
		val, err := strconv.Atoi(limitStr)
		if err != nil || val <= 0 || val > 100 {
			writeJSONError(w, http.StatusBadRequest, "invalid_limit", "limit must be an integer between 1 and 100")
			return
		}
		limit = val
	}

	rawEvents := h.store.GetLogs(limit, serviceID)
	dtos := make([]RequestLogDTO, 0, len(rawEvents))
	for _, evt := range rawEvents {
		dtos = append(dtos, RequestLogDTO{
			RequestID:  evt.RequestID,
			Timestamp:  evt.Timestamp.UTC().Format(time.RFC3339),
			Method:     evt.Method,
			Path:       evt.Path,
			ServiceID:  evt.ServiceID,
			Status:     evt.Status,
			DurationMs: float64(evt.Duration.Nanoseconds()) / 1e6,
			Error:      evt.Error,
		})
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(LogsResponse{
		Requests: dtos,
		Total:    len(dtos),
	})
}

func (h *Handler) handleMetrics(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", "GET")
		writeJSONError(w, http.StatusMethodNotAllowed, "method_not_allowed", "only GET method is supported")
		return
	}

	snapshot := h.store.GetMetrics()

	serviceDTOs := make(map[string]ServiceMetricsDTO, len(snapshot.Services))
	for svcID, sm := range snapshot.Services {
		codesMap := make(map[string]int64, len(sm.StatusCodes))
		for code, count := range sm.StatusCodes {
			codesMap[strconv.Itoa(code)] = count
		}

		serviceDTOs[svcID] = ServiceMetricsDTO{
			ServiceID:          sm.ServiceID,
			TotalRequests:      sm.TotalRequests,
			SuccessfulRequests: sm.SuccessfulRequests,
			ErrorRequests:      sm.ErrorRequests,
			AvgLatencyMs:       sm.AvgLatencyMs,
			StatusCodes:        codesMap,
		}
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(MetricsResponse{
		Global: GlobalMetricsDTO{
			TotalRequests:      snapshot.Global.TotalRequests,
			SuccessfulRequests: snapshot.Global.SuccessfulRequests,
			ErrorRequests:      snapshot.Global.ErrorRequests,
			AvgLatencyMs:       snapshot.Global.AvgLatencyMs,
			MinLatencyMs:       snapshot.Global.MinLatencyMs,
			MaxLatencyMs:       snapshot.Global.MaxLatencyMs,
		},
		Services: serviceDTOs,
	})
}

func (h *Handler) handleHealth(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", "GET")
		writeJSONError(w, http.StatusMethodNotAllowed, "method_not_allowed", "only GET method is supported")
		return
	}

	servicesMap := make(map[string]ServiceStateDTO)

	if h.healthStore != nil {
		type allGetter interface {
			GetAll() map[string]model.ServiceState
		}

		if getter, ok := h.healthStore.(allGetter); ok {
			rawMap := getter.GetAll()
			for svcID, state := range rawMap {
				lastChecked := ""
				if !state.LastChecked.IsZero() {
					lastChecked = state.LastChecked.UTC().Format(time.RFC3339)
				}
				servicesMap[svcID] = ServiceStateDTO{
					ServiceID:    state.ServiceID,
					Status:       string(state.Status),
					LastChecked:  lastChecked,
					LatencyMs:    float64(state.Latency.Nanoseconds()) / 1e6,
					FailureCount: state.FailureCount,
				}
			}
		}
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(HealthResponse{
		Services: servicesMap,
	})
}

func (h *Handler) handleSimulations(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		h.handleGetSimulations(w, r)
	case http.MethodPost:
		h.handlePostSimulations(w, r)
	default:
		w.Header().Set("Allow", "GET, POST")
		writeJSONError(w, http.StatusMethodNotAllowed, "method_not_allowed", "only GET and POST methods are supported")
	}
}

func (h *Handler) handleGetSimulations(w http.ResponseWriter, r *http.Request) {
	servicesMap := make(map[string]SimulationStateDTO)

	if h.simStore != nil {
		rawMap := h.simStore.GetAll()
		for svcID, state := range rawMap {
			servicesMap[svcID] = SimulationStateDTO{
				ServiceID: state.ServiceID,
				Mode:      string(state.Mode),
				DelayMs:   state.Delay.Milliseconds(),
			}
		}
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(SimulationsResponse{
		Services: servicesMap,
	})
}

func (h *Handler) handlePostSimulations(w http.ResponseWriter, r *http.Request) {
	if !h.authenticate(r) {
		writeJSONError(w, http.StatusUnauthorized, "unauthorized", "missing or invalid API key")
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 4096)

	var req SetSimulationRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		var maxBytesErr *http.MaxBytesError
		if errors.As(err, &maxBytesErr) || strings.Contains(err.Error(), "request body too large") {
			writeJSONError(w, http.StatusRequestEntityTooLarge, "payload_too_large", "request payload exceeds maximum allowed size")
			return
		}
		writeJSONError(w, http.StatusBadRequest, "invalid_json", "malformed JSON request body")
		return
	}

	if req.ServiceID == "" {
		writeJSONError(w, http.StatusBadRequest, "invalid_service", "service_id must not be empty")
		return
	}

	mode := model.SimulationMode(req.Mode)
	switch mode {
	case model.SimulationNormal, model.SimulationFail, model.SimulationDelay:
	default:
		writeJSONError(w, http.StatusBadRequest, "invalid_mode", "mode must be NORMAL, FAIL, or DELAY")
		return
	}

	if req.DelayMs < 0 || req.DelayMs > 30000 {
		writeJSONError(w, http.StatusBadRequest, "invalid_delay", "delay_ms must be between 0 and 30000")
		return
	}

	if mode == model.SimulationDelay && req.DelayMs <= 0 {
		writeJSONError(w, http.StatusBadRequest, "invalid_delay", "delay_ms must be greater than 0 for DELAY mode")
		return
	}

	if h.simStore == nil {
		writeJSONError(w, http.StatusInternalServerError, string(model.ErrCodeInternalError), "simulation store not configured")
		return
	}

	delayDuration := time.Duration(req.DelayMs) * time.Millisecond
	if err := h.simStore.Set(req.ServiceID, mode, delayDuration); err != nil {
		if err == simulation.ErrUnknownService || err.Error() != "" {
			writeJSONError(w, http.StatusBadRequest, "invalid_service", err.Error())
			return
		}
		writeJSONError(w, http.StatusBadRequest, "invalid_simulation", err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(SetSimulationResponse{
		Status: "ok",
		Simulation: SimulationStateDTO{
			ServiceID: req.ServiceID,
			Mode:      string(mode),
			DelayMs:   req.DelayMs,
		},
	})
}

func (h *Handler) handleRateLimits(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		h.handleGetRateLimits(w, r)
	case http.MethodPost:
		h.handlePostRateLimits(w, r)
	default:
		w.Header().Set("Allow", "GET, POST")
		writeJSONError(w, http.StatusMethodNotAllowed, "method_not_allowed", "only GET and POST methods are supported")
	}
}

func (h *Handler) handleGetRateLimits(w http.ResponseWriter, r *http.Request) {
	servicesMap := make(map[string]RateLimitDTO)

	if h.rlStore != nil {
		rawMap := h.rlStore.GetAll()
		for svcID, state := range rawMap {
			servicesMap[svcID] = RateLimitDTO{
				ServiceID:         state.ServiceID,
				RequestsPerSecond: state.RequestsPerSecond,
				Burst:             state.Burst,
				Enabled:           state.Enabled,
			}
		}
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(RateLimitsResponse{
		Services: servicesMap,
	})
}

func (h *Handler) handlePostRateLimits(w http.ResponseWriter, r *http.Request) {
	if !h.authenticate(r) {
		writeJSONError(w, http.StatusUnauthorized, "unauthorized", "missing or invalid API key")
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 4096)

	var req SetRateLimitRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		var maxBytesErr *http.MaxBytesError
		if errors.As(err, &maxBytesErr) || strings.Contains(err.Error(), "request body too large") {
			writeJSONError(w, http.StatusRequestEntityTooLarge, "payload_too_large", "request payload exceeds maximum allowed size")
			return
		}
		writeJSONError(w, http.StatusBadRequest, "invalid_json", "malformed JSON request body")
		return
	}

	if req.ServiceID == "" {
		writeJSONError(w, http.StatusBadRequest, "invalid_service", "service_id must not be empty")
		return
	}

	if req.RequestsPerSecond <= 0 || req.RequestsPerSecond > 10000 {
		writeJSONError(w, http.StatusBadRequest, "invalid_rate", "requests_per_second must be between 0.1 and 10000")
		return
	}

	if req.Burst <= 0 || req.Burst > 10000 {
		writeJSONError(w, http.StatusBadRequest, "invalid_burst", "burst must be between 1 and 10000")
		return
	}

	if h.rlStore == nil {
		writeJSONError(w, http.StatusInternalServerError, string(model.ErrCodeInternalError), "rate limiter store not configured")
		return
	}

	if err := h.rlStore.Set(req.ServiceID, req.RequestsPerSecond, req.Burst, req.Enabled); err != nil {
		if err == ratelimit.ErrUnknownService || err.Error() != "" {
			writeJSONError(w, http.StatusBadRequest, "invalid_service", err.Error())
			return
		}
		writeJSONError(w, http.StatusBadRequest, "invalid_ratelimit", err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(SetRateLimitResponse{
		Status: "ok",
		RateLimit: RateLimitDTO{
			ServiceID:         req.ServiceID,
			RequestsPerSecond: req.RequestsPerSecond,
			Burst:             req.Burst,
			Enabled:           req.Enabled,
		},
	})
}

func writeJSONError(w http.ResponseWriter, status int, errCode string, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{
		"error":   errCode,
		"message": message,
	})
}
