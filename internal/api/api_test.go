package api_test

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"miniedge/internal/api"
	"miniedge/internal/gateway"
	"miniedge/internal/health"
	"miniedge/internal/model"
	"miniedge/internal/observability"
	"miniedge/internal/proxy"
	"miniedge/internal/ratelimit"
	"miniedge/internal/router"
	"miniedge/internal/simulation"
)

// Test 1 — Empty Logs
func TestEmptyLogs(t *testing.T) {
	store := observability.NewStore(100)
	apiHandler := api.NewHandler(store)
	ts := httptest.NewServer(apiHandler)
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/api/logs")
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200 OK, got %d", resp.StatusCode)
	}
	if ct := resp.Header.Get("Content-Type"); ct != "application/json" {
		t.Errorf("expected Content-Type application/json, got %s", ct)
	}

	body, _ := io.ReadAll(resp.Body)
	var logResp api.LogsResponse
	if err := json.Unmarshal(body, &logResp); err != nil {
		t.Fatalf("failed to decode JSON: %v", err)
	}

	if logResp.Total != 0 {
		t.Errorf("expected total 0, got %d", logResp.Total)
	}
	if logResp.Requests == nil || len(logResp.Requests) != 0 {
		t.Errorf("expected empty non-nil requests slice, got %v", logResp.Requests)
	}
}

// Test 2 — Logs Response Mapping & Formatting
func TestLogsResponse(t *testing.T) {
	store := observability.NewStore(100)
	now := time.Now().Truncate(time.Second)

	store.Observe(model.RequestEvent{
		RequestID: "req_1",
		Timestamp: now,
		Method:    "GET",
		Path:      "/users/42",
		ServiceID: "users",
		Status:    200,
		Duration:  12450 * time.Microsecond, // 12.45 ms
		Error:     "",
	})

	apiHandler := api.NewHandler(store)
	ts := httptest.NewServer(apiHandler)
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/api/logs")
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	var logResp api.LogsResponse
	if err := json.NewDecoder(resp.Body).Decode(&logResp); err != nil {
		t.Fatalf("failed to decode JSON: %v", err)
	}

	if logResp.Total != 1 || len(logResp.Requests) != 1 {
		t.Fatalf("expected 1 log, got total=%d, len=%d", logResp.Total, len(logResp.Requests))
	}

	req := logResp.Requests[0]
	if req.RequestID != "req_1" || req.Method != "GET" || req.Path != "/users/42" || req.ServiceID != "users" || req.Status != 200 {
		t.Errorf("unexpected field values: %+v", req)
	}
	if req.DurationMs != 12.45 {
		t.Errorf("expected duration_ms 12.45, got %.2f", req.DurationMs)
	}
	if req.Timestamp != now.UTC().Format(time.RFC3339) {
		t.Errorf("expected timestamp %s, got %s", now.UTC().Format(time.RFC3339), req.Timestamp)
	}
}

// Test 3 — GET /api/simulations Default & POST Simulation Validation
func TestSimulationsAPI(t *testing.T) {
	services := []model.Service{
		{ID: "users", Name: "User Service"},
		{ID: "orders", Name: "Order Service"},
	}
	simStore := simulation.NewSimulationStore(services)
	obsStore := observability.NewStore(100)

	apiHandler := api.NewHandler(obsStore)
	apiHandler.SetSimulationStore(simStore)
	apiHandler.SetAPIKey("test-secret-key")

	ts := httptest.NewServer(apiHandler)
	defer ts.Close()

	postWithKey := func(targetURL string, body []byte, key string) (*http.Response, error) {
		req, err := http.NewRequest(http.MethodPost, targetURL, bytes.NewBuffer(body))
		if err != nil {
			return nil, err
		}
		req.Header.Set("Content-Type", "application/json")
		if key != "" {
			req.Header.Set("X-API-Key", key)
		}
		return http.DefaultClient.Do(req)
	}

	// 1. GET /api/simulations
	resp, err := http.Get(ts.URL + "/api/simulations")
	if err != nil {
		t.Fatalf("GET /api/simulations failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200 OK, got %d", resp.StatusCode)
	}

	var simResp api.SimulationsResponse
	if err := json.NewDecoder(resp.Body).Decode(&simResp); err != nil {
		t.Fatalf("failed decoding simulations JSON: %v", err)
	}

	if len(simResp.Services) != 2 {
		t.Fatalf("expected 2 services in simulation response, got %d", len(simResp.Services))
	}
	if simResp.Services["users"].Mode != "NORMAL" || simResp.Services["orders"].Mode != "NORMAL" {
		t.Errorf("expected NORMAL mode for services, got %+v", simResp.Services)
	}

	// 2. POST /api/simulations valid FAIL
	postBody, _ := json.Marshal(api.SetSimulationRequest{
		ServiceID: "users",
		Mode:      "FAIL",
		DelayMs:   0,
	})
	respPost, err := postWithKey(ts.URL+"/api/simulations", postBody, "test-secret-key")
	if err != nil {
		t.Fatalf("POST /api/simulations failed: %v", err)
	}
	defer respPost.Body.Close()

	if respPost.StatusCode != http.StatusOK {
		t.Errorf("expected 200 OK for POST FAIL, got %d", respPost.StatusCode)
	}

	var setResp api.SetSimulationResponse
	if err := json.NewDecoder(respPost.Body).Decode(&setResp); err != nil {
		t.Fatalf("failed decoding POST response: %v", err)
	}
	if setResp.Status != "ok" || setResp.Simulation.Mode != "FAIL" {
		t.Errorf("unexpected POST response: %+v", setResp)
	}

	// 3. POST /api/simulations invalid service ID
	postInvalidSvc, _ := json.Marshal(api.SetSimulationRequest{
		ServiceID: "nonexistent",
		Mode:      "FAIL",
	})
	respInvSvc, err := postWithKey(ts.URL+"/api/simulations", postInvalidSvc, "test-secret-key")
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer respInvSvc.Body.Close()
	if respInvSvc.StatusCode != http.StatusBadRequest {
		t.Errorf("expected 400 Bad Request for unknown service, got %d", respInvSvc.StatusCode)
	}

	// 4. POST /api/simulations invalid mode
	postInvalidMode, _ := json.Marshal(api.SetSimulationRequest{
		ServiceID: "users",
		Mode:      "INVALID_MODE",
	})
	respInvMode, err := postWithKey(ts.URL+"/api/simulations", postInvalidMode, "test-secret-key")
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer respInvMode.Body.Close()
	if respInvMode.StatusCode != http.StatusBadRequest {
		t.Errorf("expected 400 Bad Request for invalid mode, got %d", respInvMode.StatusCode)
	}

	// 5. POST /api/simulations invalid delay
	postInvalidDelay, _ := json.Marshal(api.SetSimulationRequest{
		ServiceID: "users",
		Mode:      "DELAY",
		DelayMs:   0, // DELAY requires delay > 0
	})
	respInvDelay, err := postWithKey(ts.URL+"/api/simulations", postInvalidDelay, "test-secret-key")
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer respInvDelay.Body.Close()
	if respInvDelay.StatusCode != http.StatusBadRequest {
		t.Errorf("expected 400 Bad Request for 0 delay in DELAY mode, got %d", respInvDelay.StatusCode)
	}

	// 6. Unsupported method on /api/simulations (PUT -> 405 Method Not Allowed)
	reqPut, _ := http.NewRequest(http.MethodPut, ts.URL+"/api/simulations", nil)
	respPut, err := http.DefaultClient.Do(reqPut)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer respPut.Body.Close()
	if respPut.StatusCode != http.StatusMethodNotAllowed {
		t.Errorf("expected 405 Method Not Allowed for PUT /api/simulations, got %d", respPut.StatusCode)
	}
	if allow := respPut.Header.Get("Allow"); allow != "GET, POST" {
		t.Errorf("expected Allow: GET, POST header, got '%s'", allow)
	}
}

// Test 4 — GET /api/ratelimits & POST /api/ratelimits Validation & Controls
func TestRateLimitsAPI(t *testing.T) {
	services := []model.Service{
		{ID: "users", Name: "User Service"},
		{ID: "orders", Name: "Order Service"},
	}
	rlStore := ratelimit.NewRateLimiterStore(services)
	obsStore := observability.NewStore(100)

	apiHandler := api.NewHandler(obsStore)
	apiHandler.SetRateLimiterStore(rlStore)
	apiHandler.SetAPIKey("test-secret-key")

	ts := httptest.NewServer(apiHandler)
	defer ts.Close()

	postWithKey := func(targetURL string, body []byte, key string) (*http.Response, error) {
		req, err := http.NewRequest(http.MethodPost, targetURL, bytes.NewBuffer(body))
		if err != nil {
			return nil, err
		}
		req.Header.Set("Content-Type", "application/json")
		if key != "" {
			req.Header.Set("X-API-Key", key)
		}
		return http.DefaultClient.Do(req)
	}

	// 1. GET /api/ratelimits
	resp, err := http.Get(ts.URL + "/api/ratelimits")
	if err != nil {
		t.Fatalf("GET /api/ratelimits failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200 OK, got %d", resp.StatusCode)
	}

	var rlResp api.RateLimitsResponse
	if err := json.NewDecoder(resp.Body).Decode(&rlResp); err != nil {
		t.Fatalf("failed decoding ratelimits JSON: %v", err)
	}
	if len(rlResp.Services) != 2 {
		t.Fatalf("expected 2 services in ratelimits response, got %d", len(rlResp.Services))
	}

	// 2. POST /api/ratelimits valid update
	postBody, _ := json.Marshal(api.SetRateLimitRequest{
		ServiceID:         "users",
		RequestsPerSecond: 50.0,
		Burst:             100,
		Enabled:           true,
	})
	respPost, err := postWithKey(ts.URL+"/api/ratelimits", postBody, "test-secret-key")
	if err != nil {
		t.Fatalf("POST /api/ratelimits failed: %v", err)
	}
	defer respPost.Body.Close()

	if respPost.StatusCode != http.StatusOK {
		t.Errorf("expected 200 OK for POST ratelimits, got %d", respPost.StatusCode)
	}

	var setResp api.SetRateLimitResponse
	_ = json.NewDecoder(respPost.Body).Decode(&setResp)
	if setResp.Status != "ok" || setResp.RateLimit.RequestsPerSecond != 50.0 || setResp.RateLimit.Burst != 100 {
		t.Errorf("unexpected POST response: %+v", setResp)
	}

	// 3. POST /api/ratelimits invalid service ID
	postInvalidSvc, _ := json.Marshal(api.SetRateLimitRequest{ServiceID: "unknown", RequestsPerSecond: 10, Burst: 20})
	respInvSvc, err := postWithKey(ts.URL+"/api/ratelimits", postInvalidSvc, "test-secret-key")
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer respInvSvc.Body.Close()
	if respInvSvc.StatusCode != http.StatusBadRequest {
		t.Errorf("expected 400 Bad Request for unknown service, got %d", respInvSvc.StatusCode)
	}

	// 4. POST /api/ratelimits invalid rate
	postInvalidRate, _ := json.Marshal(api.SetRateLimitRequest{ServiceID: "users", RequestsPerSecond: 0, Burst: 20})
	respInvRate, err := postWithKey(ts.URL+"/api/ratelimits", postInvalidRate, "test-secret-key")
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer respInvRate.Body.Close()
	if respInvRate.StatusCode != http.StatusBadRequest {
		t.Errorf("expected 400 Bad Request for 0 rate, got %d", respInvRate.StatusCode)
	}

	// 5. Unsupported method on /api/ratelimits (PUT -> 405 Method Not Allowed)
	reqPut, _ := http.NewRequest(http.MethodPut, ts.URL+"/api/ratelimits", nil)
	respPut, err := http.DefaultClient.Do(reqPut)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer respPut.Body.Close()
	if respPut.StatusCode != http.StatusMethodNotAllowed {
		t.Errorf("expected 405 Method Not Allowed for PUT /api/ratelimits, got %d", respPut.StatusCode)
	}
	if allow := respPut.Header.Get("Allow"); allow != "GET, POST" {
		t.Errorf("expected Allow: GET, POST header, got '%s'", allow)
	}
}

// Test 5 — Complete End-to-End Integration Flow (Rate Limiting, Gateway, Observability, Health Independence)
func TestFullEndToEndRateLimitingFlow(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("user profile"))
	}))
	defer upstream.Close()

	services := []model.Service{{ID: "users", Name: "User Service", Upstream: upstream.URL, HealthPath: "/health"}}
	routes := []model.Route{{Path: "/users/", ServiceID: "users"}}

	healthStore := health.NewHealthStore(services)
	checker := health.NewChecker(200*time.Millisecond, 50*time.Millisecond)
	worker := health.NewWorker(services, healthStore, checker, 100*time.Millisecond)
	worker.Start()
	defer worker.Stop()

	rlStore := ratelimit.NewRateLimiterStore(services)
	obsStore := observability.NewStore(100)

	apiHandler := api.NewHandler(obsStore, healthStore)
	apiHandler.SetRateLimiterStore(rlStore)
	apiHandler.SetAPIKey("test-secret-key")

	r := router.NewPrefixRouter(routes)
	reg := router.NewStaticServiceRegistry(services)
	px := proxy.NewServiceProxy(5 * time.Second)

	gwHandler := gateway.NewGatewayHandler(r, reg, px, obsStore, 5*time.Second)
	gwHandler.SetAPIHandler(apiHandler)
	gwHandler.SetRateLimiter(rlStore)

	gwServer := httptest.NewServer(gwHandler)
	defer gwServer.Close()

	// 1. Set Rate Limit via POST /api/ratelimits to 1.0 req/sec, burst 1
	postRl, _ := json.Marshal(api.SetRateLimitRequest{ServiceID: "users", RequestsPerSecond: 1.0, Burst: 1, Enabled: true})
	reqPost, _ := http.NewRequest(http.MethodPost, gwServer.URL+"/api/ratelimits", bytes.NewBuffer(postRl))
	reqPost.Header.Set("Content-Type", "application/json")
	reqPost.Header.Set("X-API-Key", "test-secret-key")

	respPost, err := http.DefaultClient.Do(reqPost)
	if err != nil || respPost.StatusCode != http.StatusOK {
		t.Fatalf("failed POST /api/ratelimits: %v, status=%v", err, respPost.StatusCode)
	}
	respPost.Body.Close()

	// 2. First request to /users/42 succeeds (200 OK)
	resp1, err := http.Get(gwServer.URL + "/users/42")
	if err != nil || resp1.StatusCode != http.StatusOK {
		t.Fatalf("failed 1st application request: %v, status=%v", err, resp1.StatusCode)
	}
	resp1.Body.Close()

	// 3. Second request to /users/42 gets throttled (429 Too Many Requests)
	resp2, err := http.Get(gwServer.URL + "/users/42")
	if err != nil {
		t.Fatalf("failed 2nd application request: %v", err)
	}
	defer resp2.Body.Close()
	if resp2.StatusCode != http.StatusTooManyRequests {
		t.Errorf("expected 429 Too Many Requests, got %d", resp2.StatusCode)
	}

	// 4. Verify /api/logs records 429 rate_limit_exceeded
	respLogs, err := http.Get(gwServer.URL + "/api/logs")
	if err != nil {
		t.Fatalf("failed GET /api/logs: %v", err)
	}
	defer respLogs.Body.Close()

	var logsResp api.LogsResponse
	_ = json.NewDecoder(respLogs.Body).Decode(&logsResp)
	if logsResp.Total != 2 || logsResp.Requests[0].Status != 429 || logsResp.Requests[0].Error != "rate_limit_exceeded" {
		t.Errorf("unexpected log entries: %+v", logsResp)
	}

	// 5. Verify /api/health remains independent (reports physical UP status)
	respHealth, err := http.Get(gwServer.URL + "/api/health")
	if err != nil {
		t.Fatalf("failed GET /api/health: %v", err)
	}
	defer respHealth.Body.Close()

	var healthResp api.HealthResponse
	_ = json.NewDecoder(respHealth.Body).Decode(&healthResp)
	u, exists := healthResp.Services["users"]
	if !exists || u.Status != "UP" {
		t.Errorf("expected health status UP independent of rate limiting, got %+v", u)
	}
}

// Test 6 — Admin API Key Authentication (Missing, Wrong, Correct)
func TestAdminAuthentication(t *testing.T) {
	services := []model.Service{{ID: "users"}}
	simStore := simulation.NewSimulationStore(services)
	obsStore := observability.NewStore(100)

	apiHandler := api.NewHandler(obsStore)
	apiHandler.SetSimulationStore(simStore)
	apiHandler.SetAPIKey("valid-admin-key")

	ts := httptest.NewServer(apiHandler)
	defer ts.Close()

	postBody, _ := json.Marshal(api.SetSimulationRequest{ServiceID: "users", Mode: "FAIL"})

	// 1. Missing API Key -> 401 Unauthorized
	reqMissing, _ := http.NewRequest(http.MethodPost, ts.URL+"/api/simulations", bytes.NewBuffer(postBody))
	reqMissing.Header.Set("Content-Type", "application/json")
	respMissing, err := http.DefaultClient.Do(reqMissing)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	respMissing.Body.Close()
	if respMissing.StatusCode != http.StatusUnauthorized {
		t.Errorf("expected 401 Unauthorized for missing API key, got %d", respMissing.StatusCode)
	}

	// 2. Wrong API Key -> 401 Unauthorized
	reqWrong, _ := http.NewRequest(http.MethodPost, ts.URL+"/api/simulations", bytes.NewBuffer(postBody))
	reqWrong.Header.Set("Content-Type", "application/json")
	reqWrong.Header.Set("X-API-Key", "wrong-admin-key")
	respWrong, err := http.DefaultClient.Do(reqWrong)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	respWrong.Body.Close()
	if respWrong.StatusCode != http.StatusUnauthorized {
		t.Errorf("expected 401 Unauthorized for wrong API key, got %d", respWrong.StatusCode)
	}

	// 3. Correct API Key -> 200 OK
	reqCorrect, _ := http.NewRequest(http.MethodPost, ts.URL+"/api/simulations", bytes.NewBuffer(postBody))
	reqCorrect.Header.Set("Content-Type", "application/json")
	reqCorrect.Header.Set("X-API-Key", "valid-admin-key")
	respCorrect, err := http.DefaultClient.Do(reqCorrect)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	respCorrect.Body.Close()
	if respCorrect.StatusCode != http.StatusOK {
		t.Errorf("expected 200 OK for correct API key, got %d", respCorrect.StatusCode)
	}
}

// Test 7 — CORS Response & OPTIONS Preflight
func TestAPICORS(t *testing.T) {
	obsStore := observability.NewStore(100)
	apiHandler := api.NewHandler(obsStore)
	apiHandler.SetAllowedOrigin("http://custom.origin.com")

	ts := httptest.NewServer(apiHandler)
	defer ts.Close()

	// 1. Normal GET request has CORS header
	respGet, err := http.Get(ts.URL + "/api/metrics")
	if err != nil {
		t.Fatalf("GET request failed: %v", err)
	}
	respGet.Body.Close()
	if origin := respGet.Header.Get("Access-Control-Allow-Origin"); origin != "http://custom.origin.com" {
		t.Errorf("expected Access-Control-Allow-Origin 'http://custom.origin.com', got '%s'", origin)
	}

	// 2. OPTIONS preflight request handling
	reqOpt, _ := http.NewRequest(http.MethodOptions, ts.URL+"/api/simulations", nil)
	respOpt, err := http.DefaultClient.Do(reqOpt)
	if err != nil {
		t.Fatalf("OPTIONS request failed: %v", err)
	}
	respOpt.Body.Close()
	if respOpt.StatusCode != http.StatusNoContent {
		t.Errorf("expected 204 No Content for OPTIONS, got %d", respOpt.StatusCode)
	}
	if allowMethods := respOpt.Header.Get("Access-Control-Allow-Methods"); allowMethods != "GET, POST, OPTIONS" {
		t.Errorf("expected Allow-Methods header, got '%s'", allowMethods)
	}
}

// Test 8 — Request Payload Size Limits
func TestAPIPayloadLimits(t *testing.T) {
	services := []model.Service{{ID: "users"}}
	simStore := simulation.NewSimulationStore(services)
	obsStore := observability.NewStore(100)

	apiHandler := api.NewHandler(obsStore)
	apiHandler.SetSimulationStore(simStore)
	apiHandler.SetAPIKey("valid-key")

	ts := httptest.NewServer(apiHandler)
	defer ts.Close()

	// 1. Oversized body > 4096 bytes -> 413 Payload Too Large
	largePadding := make([]byte, 5000)
	for i := range largePadding {
		largePadding[i] = 'a'
	}
	oversizedJSON, _ := json.Marshal(map[string]string{
		"service_id": "users",
		"mode":       "FAIL",
		"padding":    string(largePadding),
	})

	reqLarge, _ := http.NewRequest(http.MethodPost, ts.URL+"/api/simulations", bytes.NewBuffer(oversizedJSON))
	reqLarge.Header.Set("Content-Type", "application/json")
	reqLarge.Header.Set("X-API-Key", "valid-key")

	respLarge, err := http.DefaultClient.Do(reqLarge)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer respLarge.Body.Close()

	if respLarge.StatusCode != http.StatusRequestEntityTooLarge {
		t.Errorf("expected 413 Payload Too Large for oversized body, got %d", respLarge.StatusCode)
	}

	var jsonErr map[string]string
	_ = json.NewDecoder(respLarge.Body).Decode(&jsonErr)
	if jsonErr["error"] != "payload_too_large" {
		t.Errorf("expected error 'payload_too_large', got '%s'", jsonErr["error"])
	}

	// 2. Normal-sized valid POST succeeds -> 200 OK
	normalJSON, _ := json.Marshal(api.SetSimulationRequest{ServiceID: "users", Mode: "NORMAL"})
	reqNormal, _ := http.NewRequest(http.MethodPost, ts.URL+"/api/simulations", bytes.NewBuffer(normalJSON))
	reqNormal.Header.Set("Content-Type", "application/json")
	reqNormal.Header.Set("X-API-Key", "valid-key")

	respNormal, err := http.DefaultClient.Do(reqNormal)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	respNormal.Body.Close()

	if respNormal.StatusCode != http.StatusOK {
		t.Errorf("expected 200 OK for normal payload, got %d", respNormal.StatusCode)
	}
}
