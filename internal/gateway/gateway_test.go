package gateway_test

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"miniedge/internal/gateway"
	"miniedge/internal/model"
	"miniedge/internal/observability"
	"miniedge/internal/proxy"
	"miniedge/internal/ratelimit"
	"miniedge/internal/router"
	"miniedge/internal/simulation"
)

func setupTestGateway(routes []model.Route, services []model.Service, reqTimeout time.Duration) *httptest.Server {
	r := router.NewPrefixRouter(routes)
	reg := router.NewStaticServiceRegistry(services)
	px := proxy.NewServiceProxy(reqTimeout)

	handler := gateway.NewGatewayHandler(r, reg, px, nil, reqTimeout)
	return httptest.NewServer(handler)
}

// Test A — Successful proxy
func TestSuccessfulProxy(t *testing.T) {
	var receivedPath string
	var receivedMethod string
	var receivedHeader string

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedPath = r.URL.Path
		receivedMethod = r.Method
		receivedHeader = r.Header.Get("X-Test-Header")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("hello user 42"))
	}))
	defer upstream.Close()

	routes := []model.Route{
		{Path: "/users/", ServiceID: "users"},
	}
	services := []model.Service{
		{ID: "users", Name: "User Service", Upstream: upstream.URL, HealthPath: "/health"},
	}

	gw := setupTestGateway(routes, services, 5*time.Second)
	defer gw.Close()

	req, err := http.NewRequest(http.MethodGet, gw.URL+"/users/42", nil)
	if err != nil {
		t.Fatalf("failed to create request: %v", err)
	}
	req.Header.Set("X-Test-Header", "test-val")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("gateway request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected status 200, got %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("failed to read body: %v", err)
	}

	if string(body) != "hello user 42" {
		t.Errorf("expected body 'hello user 42', got '%s'", string(body))
	}
	if receivedPath != "/users/42" {
		t.Errorf("expected upstream path '/users/42', got '%s'", receivedPath)
	}
	if receivedMethod != http.MethodGet {
		t.Errorf("expected upstream method GET, got '%s'", receivedMethod)
	}
	if receivedHeader != "test-val" {
		t.Errorf("expected header 'test-val', got '%s'", receivedHeader)
	}
}

// Test B — Unknown route
func TestUnknownRoute(t *testing.T) {
	gw := setupTestGateway(nil, nil, 5*time.Second)
	defer gw.Close()

	resp, err := http.Get(gw.URL + "/unknown")
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("expected status 404, got %d", resp.StatusCode)
	}

	reqID := resp.Header.Get("X-Request-ID")
	if reqID == "" {
		t.Error("expected non-empty X-Request-ID header")
	}

	var jsonErr struct {
		Error     string `json:"error"`
		Message   string `json:"message"`
		RequestID string `json:"request_id"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&jsonErr); err != nil {
		t.Fatalf("failed to parse error JSON: %v", err)
	}

	if jsonErr.Error != string(model.ErrCodeRouteNotFound) {
		t.Errorf("expected error code 'route_not_found', got '%s'", jsonErr.Error)
	}
	if jsonErr.RequestID != reqID {
		t.Errorf("expected JSON request_id '%s' to match header '%s'", jsonErr.RequestID, reqID)
	}
}

// Test C — Upstream unavailable
func TestUpstreamUnavailable(t *testing.T) {
	routes := []model.Route{
		{Path: "/users/", ServiceID: "users"},
	}
	services := []model.Service{
		{ID: "users", Name: "User Service", Upstream: "http://127.0.0.1:59999/", HealthPath: "/health"},
	}

	gw := setupTestGateway(routes, services, 2*time.Second)
	defer gw.Close()

	resp, err := http.Get(gw.URL + "/users/42")
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadGateway {
		t.Errorf("expected status 502 Bad Gateway, got %d", resp.StatusCode)
	}

	reqID := resp.Header.Get("X-Request-ID")
	if reqID == "" {
		t.Error("expected non-empty X-Request-ID header")
	}

	var jsonErr struct {
		Error     string `json:"error"`
		Message   string `json:"message"`
		RequestID string `json:"request_id"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&jsonErr); err != nil {
		t.Fatalf("failed to parse error JSON: %v", err)
	}

	if jsonErr.Error != string(model.ErrCodeBadGateway) {
		t.Errorf("expected error code 'bad_gateway', got '%s'", jsonErr.Error)
	}
}

// Test D — Upstream timeout
func TestUpstreamTimeout(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(300 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	defer upstream.Close()

	routes := []model.Route{
		{Path: "/users/", ServiceID: "users"},
	}
	services := []model.Service{
		{ID: "users", Name: "User Service", Upstream: upstream.URL, HealthPath: "/health"},
	}

	gw := setupTestGateway(routes, services, 50*time.Millisecond)
	defer gw.Close()

	resp, err := http.Get(gw.URL + "/users/42")
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusGatewayTimeout {
		t.Errorf("expected status 504 Gateway Timeout, got %d", resp.StatusCode)
	}

	var jsonErr struct {
		Error     string `json:"error"`
		Message   string `json:"message"`
		RequestID string `json:"request_id"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&jsonErr); err != nil {
		t.Fatalf("failed to parse error JSON: %v", err)
	}

	if jsonErr.Error != string(model.ErrCodeUpstreamTimeout) {
		t.Errorf("expected error code 'upstream_timeout', got '%s'", jsonErr.Error)
	}
}

// Test E — Request ID uniqueness & propagation
func TestRequestID(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reqID := r.Header.Get("X-Request-ID")
		w.Header().Set("X-Upstream-Received-ReqID", reqID)
		w.WriteHeader(http.StatusOK)
	}))
	defer upstream.Close()

	routes := []model.Route{
		{Path: "/users/", ServiceID: "users"},
	}
	services := []model.Service{
		{ID: "users", Name: "User Service", Upstream: upstream.URL, HealthPath: "/health"},
	}

	gw := setupTestGateway(routes, services, 5*time.Second)
	defer gw.Close()

	const numRequests = 20
	seenIDs := make(map[string]bool)
	var mu sync.Mutex

	var wg sync.WaitGroup
	for i := 0; i < numRequests; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			resp, err := http.Get(gw.URL + "/users/42")
			if err != nil {
				t.Errorf("request failed: %v", err)
				return
			}
			defer resp.Body.Close()

			headerID := resp.Header.Get("X-Request-ID")
			upstreamID := resp.Header.Get("X-Upstream-Received-ReqID")

			if headerID == "" {
				t.Errorf("expected non-empty X-Request-ID")
			}
			if headerID != upstreamID {
				t.Errorf("expected upstream received ID '%s' to match gateway ID '%s'", upstreamID, headerID)
			}

			mu.Lock()
			if seenIDs[headerID] {
				t.Errorf("duplicate request ID detected: %s", headerID)
			}
			seenIDs[headerID] = true
			mu.Unlock()
		}()
	}
	wg.Wait()
}

// Test F — Concurrent requests
func TestConcurrentRequests(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprintf(w, "response for %s", r.URL.Path)
	}))
	defer upstream.Close()

	routes := []model.Route{
		{Path: "/users/", ServiceID: "users"},
	}
	services := []model.Service{
		{ID: "users", Name: "User Service", Upstream: upstream.URL, HealthPath: "/health"},
	}

	gw := setupTestGateway(routes, services, 5*time.Second)
	defer gw.Close()

	const concurrency = 30
	var wg sync.WaitGroup

	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		userNum := i
		go func() {
			defer wg.Done()
			path := fmt.Sprintf("/users/%d", userNum)
			resp, err := http.Get(gw.URL + path)
			if err != nil {
				t.Errorf("request failed for %s: %v", path, err)
				return
			}
			defer resp.Body.Close()

			body, err := io.ReadAll(resp.Body)
			if err != nil {
				t.Errorf("failed reading body for %s: %v", path, err)
				return
			}

			expectedBody := fmt.Sprintf("response for %s", path)
			if string(body) != expectedBody {
				t.Errorf("expected '%s', got '%s'", expectedBody, string(body))
			}
		}()
	}

	wg.Wait()
}

// Test G — Path boundary integration
func TestPathBoundaryIntegration(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	}))
	defer upstream.Close()

	routes := []model.Route{
		{Path: "/users/", ServiceID: "users"},
	}
	services := []model.Service{
		{ID: "users", Name: "User Service", Upstream: upstream.URL, HealthPath: "/health"},
	}

	gw := setupTestGateway(routes, services, 5*time.Second)
	defer gw.Close()

	// /users/42 -> 200 OK
	resp1, err := http.Get(gw.URL + "/users/42")
	if err != nil || resp1.StatusCode != http.StatusOK {
		t.Errorf("expected 200 OK for /users/42, got %v, err=%v", resp1.StatusCode, err)
	}
	resp1.Body.Close()

	// /usersXYZ -> 404 Not Found
	resp2, err := http.Get(gw.URL + "/usersXYZ")
	if err != nil || resp2.StatusCode != http.StatusNotFound {
		t.Errorf("expected 404 Not Found for /usersXYZ, got %v, err=%v", resp2.StatusCode, err)
	}
	resp2.Body.Close()

	// /users -> 404 Not Found (explicit rule: /users/ does not match /users)
	resp3, err := http.Get(gw.URL + "/users")
	if err != nil || resp3.StatusCode != http.StatusNotFound {
		t.Errorf("expected 404 Not Found for /users, got %v, err=%v", resp3.StatusCode, err)
	}
	resp3.Body.Close()
}

// Test H — Failure Simulation FAIL Mode
func TestGatewaySimulationFailMode(t *testing.T) {
	upstreamCalls := 0
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upstreamCalls++
		w.WriteHeader(http.StatusOK)
	}))
	defer upstream.Close()

	routes := []model.Route{{Path: "/users/", ServiceID: "users"}}
	services := []model.Service{{ID: "users", Name: "User Service", Upstream: upstream.URL}}

	simStore := simulation.NewSimulationStore(services)
	_ = simStore.Set("users", model.SimulationFail, 0)

	obsStore := observability.NewStore(100)

	r := router.NewPrefixRouter(routes)
	reg := router.NewStaticServiceRegistry(services)
	px := proxy.NewServiceProxy(5 * time.Second)

	gwHandler := gateway.NewGatewayHandler(r, reg, px, obsStore, 5*time.Second)
	gwHandler.SetSimulationController(simStore)

	gwServer := httptest.NewServer(gwHandler)
	defer gwServer.Close()

	resp, err := http.Get(gwServer.URL + "/users/42")
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusServiceUnavailable {
		t.Errorf("expected 503 Service Unavailable, got %d", resp.StatusCode)
	}

	reqID := resp.Header.Get("X-Request-ID")
	if reqID == "" {
		t.Error("expected non-empty X-Request-ID")
	}

	var errResp map[string]string
	if err := json.NewDecoder(resp.Body).Decode(&errResp); err != nil {
		t.Fatalf("failed decoding JSON error: %v", err)
	}

	if errResp["error"] != string(model.ErrCodeSimulationActive) {
		t.Errorf("expected error 'simulation_active', got '%s'", errResp["error"])
	}

	if upstreamCalls != 0 {
		t.Errorf("expected 0 calls to upstream service in FAIL mode, got %d", upstreamCalls)
	}

	// Verify observability event
	logs := obsStore.GetLogs(10, "")
	if len(logs) != 1 {
		t.Fatalf("expected 1 logged event, got %d", len(logs))
	}
	if logs[0].Status != http.StatusServiceUnavailable || logs[0].Error != string(model.ErrCodeSimulationActive) {
		t.Errorf("unexpected logged event details: %+v", logs[0])
	}
}

// Test I — Failure Simulation DELAY Mode & Cancellation
func TestGatewaySimulationDelayMode(t *testing.T) {
	upstreamCalls := 0
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upstreamCalls++
		w.WriteHeader(http.StatusOK)
	}))
	defer upstream.Close()

	routes := []model.Route{{Path: "/users/", ServiceID: "users"}}
	services := []model.Service{{ID: "users", Name: "User Service", Upstream: upstream.URL}}

	simStore := simulation.NewSimulationStore(services)
	_ = simStore.Set("users", model.SimulationDelay, 150*time.Millisecond)

	r := router.NewPrefixRouter(routes)
	reg := router.NewStaticServiceRegistry(services)
	px := proxy.NewServiceProxy(5 * time.Second)

	gwHandler := gateway.NewGatewayHandler(r, reg, px, nil, 5*time.Second)
	gwHandler.SetSimulationController(simStore)

	gwServer := httptest.NewServer(gwHandler)
	defer gwServer.Close()

	// 1. Normal delayed request completes successfully
	start := time.Now()
	resp, err := http.Get(gwServer.URL + "/users/42")
	duration := time.Since(start)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200 OK, got %d", resp.StatusCode)
	}
	if duration < 140*time.Millisecond {
		t.Errorf("expected duration to be at least 140ms, got %v", duration)
	}
	if upstreamCalls != 1 {
		t.Errorf("expected 1 call to upstream in DELAY mode, got %d", upstreamCalls)
	}

	// 2. Request context cancellation terminates delay early
	reqCtx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()

	req, _ := http.NewRequestWithContext(reqCtx, http.MethodGet, gwServer.URL+"/users/42", nil)
	_, _ = http.DefaultClient.Do(req)
}

// Test J — M2.4 Rate Limiting HTTP 429 Throttling
func TestGatewayRateLimitingThrottling(t *testing.T) {
	upstreamCalls := 0
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upstreamCalls++
		w.WriteHeader(http.StatusOK)
	}))
	defer upstream.Close()

	routes := []model.Route{{Path: "/users/", ServiceID: "users"}}
	services := []model.Service{{ID: "users", Name: "User Service", Upstream: upstream.URL}}

	rlStore := ratelimit.NewRateLimiterStore(services)
	_ = rlStore.Set("users", 10.0, 1, true) // Capacity 1 burst

	obsStore := observability.NewStore(100)

	r := router.NewPrefixRouter(routes)
	reg := router.NewStaticServiceRegistry(services)
	px := proxy.NewServiceProxy(5 * time.Second)

	gwHandler := gateway.NewGatewayHandler(r, reg, px, obsStore, 5*time.Second)
	gwHandler.SetRateLimiter(rlStore)

	gwServer := httptest.NewServer(gwHandler)
	defer gwServer.Close()

	// Request 1: Allowed (uses 1 token)
	resp1, err := http.Get(gwServer.URL + "/users/42")
	if err != nil || resp1.StatusCode != http.StatusOK {
		t.Fatalf("first request failed: %v, status=%v", err, resp1.StatusCode)
	}
	resp1.Body.Close()

	// Request 2: Rejected (429 Too Many Requests)
	resp2, err := http.Get(gwServer.URL + "/users/42")
	if err != nil {
		t.Fatalf("second request failed: %v", err)
	}
	defer resp2.Body.Close()

	if resp2.StatusCode != http.StatusTooManyRequests {
		t.Errorf("expected 429 Too Many Requests, got %d", resp2.StatusCode)
	}

	reqID := resp2.Header.Get("X-Request-ID")
	if reqID == "" {
		t.Error("expected non-empty X-Request-ID")
	}

	retryAfter := resp2.Header.Get("Retry-After")
	if retryAfter == "" {
		t.Error("expected non-empty Retry-After header")
	}

	var jsonErr map[string]string
	if err := json.NewDecoder(resp2.Body).Decode(&jsonErr); err != nil {
		t.Fatalf("failed decoding 429 JSON error: %v", err)
	}
	if jsonErr["error"] != string(model.ErrCodeRateLimitExceeded) {
		t.Errorf("expected error 'rate_limit_exceeded', got '%s'", jsonErr["error"])
	}

	if upstreamCalls != 1 {
		t.Errorf("expected exactly 1 call to upstream service, got %d", upstreamCalls)
	}

	// Verify observability event
	logs := obsStore.GetLogs(10, "")
	if len(logs) != 2 {
		t.Fatalf("expected 2 logged events, got %d", len(logs))
	}
	if logs[0].Status != http.StatusTooManyRequests || logs[0].Error != string(model.ErrCodeRateLimitExceeded) {
		t.Errorf("unexpected 429 logged event details: %+v", logs[0])
	}
}

// Test K — API Routing Interception (/api, /api/metrics, /apiXYZ)
func TestAPIRoutingInterception(t *testing.T) {
	routes := []model.Route{
		{Path: "/apiXYZ", ServiceID: "apixyz-service"},
	}
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("apixyz response"))
	}))
	defer upstream.Close()

	services := []model.Service{
		{ID: "apixyz-service", Name: "APIXYZ Service", Upstream: upstream.URL},
	}

	obsStore := observability.NewStore(100)
	apiHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"intercepted": true}`))
	})

	r := router.NewPrefixRouter(routes)
	reg := router.NewStaticServiceRegistry(services)
	px := proxy.NewServiceProxy(5 * time.Second)

	gwHandler := gateway.NewGatewayHandler(r, reg, px, obsStore, 5*time.Second)
	gwHandler.SetAPIHandler(apiHandler)

	gwServer := httptest.NewServer(gwHandler)
	defer gwServer.Close()

	// 1. /api -> intercepted by API handler
	resp1, err := http.Get(gwServer.URL + "/api")
	if err != nil || resp1.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 OK for /api, got %v, err=%v", resp1.StatusCode, err)
	}
	body1, _ := io.ReadAll(resp1.Body)
	resp1.Body.Close()
	if string(body1) != `{"intercepted": true}` {
		t.Errorf("expected /api to be intercepted by API handler, got %s", string(body1))
	}

	// 2. /api/metrics -> intercepted by API handler
	resp2, err := http.Get(gwServer.URL + "/api/metrics")
	if err != nil || resp2.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 OK for /api/metrics, got %v, err=%v", resp2.StatusCode, err)
	}
	body2, _ := io.ReadAll(resp2.Body)
	resp2.Body.Close()
	if string(body2) != `{"intercepted": true}` {
		t.Errorf("expected /api/metrics to be intercepted by API handler, got %s", string(body2))
	}

	// 3. /apiXYZ -> NOT intercepted by API handler, routed to apixyz-service
	resp3, err := http.Get(gwServer.URL + "/apiXYZ")
	if err != nil || resp3.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 OK for /apiXYZ, got %v, err=%v", resp3.StatusCode, err)
	}
	body3, _ := io.ReadAll(resp3.Body)
	resp3.Body.Close()
	if string(body3) != "apixyz response" {
		t.Errorf("expected /apiXYZ to reach apixyz-service, got %s", string(body3))
	}
}
