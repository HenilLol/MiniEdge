package integration_test

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"miniedge/internal/api"
	"miniedge/internal/config"
	"miniedge/internal/gateway"
	"miniedge/internal/health"
	"miniedge/internal/model"
	"miniedge/internal/observability"
	"miniedge/internal/proxy"
	"miniedge/internal/ratelimit"
	"miniedge/internal/router"
	"miniedge/internal/simulation"
)

// TestE2ESuccessfulPipeline tests full end-to-end configuration loading, component wiring,
// request proxying, header propagation (X-Request-ID, X-Forwarded-Host), control API interaction,
// and clean worker/server shutdown.
func TestE2ESuccessfulPipeline(t *testing.T) {
	var receivedPath string
	var receivedQuery string
	var receivedHost string
	var receivedXForwardedHost string

	// 1. Upstream Backend Server
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedPath = r.URL.Path
		receivedQuery = r.URL.RawQuery
		receivedHost = r.Host
		receivedXForwardedHost = r.Header.Get("X-Forwarded-Host")

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ok","user_id":100}`))
	}))
	defer upstream.Close()

	// 2. Generate JSON Configuration File on Disk
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config.json")

	cfgData := model.Config{
		ListenAddr: "127.0.0.1:0", // dynamic test port
		Services: []model.Service{
			{
				ID:         "user-service",
				Name:       "User Backend Service",
				Upstream:   upstream.URL + "/v1",
				HealthPath: "/health",
			},
		},
		Routes: []model.Route{
			{
				Path:      "/users/",
				ServiceID: "user-service",
			},
		},
	}

	rawJSON, err := json.Marshal(cfgData)
	if err != nil {
		t.Fatalf("failed to marshal test config: %v", err)
	}
	if err := os.WriteFile(configPath, rawJSON, 0644); err != nil {
		t.Fatalf("failed to write test config file: %v", err)
	}

	// 3. Load & Validate Configuration from Disk File
	f, err := os.Open(configPath)
	if err != nil {
		t.Fatalf("failed to open config file: %v", err)
	}
	loadedCfg, err := config.LoadConfig(f)
	f.Close()
	if err != nil {
		t.Fatalf("failed to load config from file: %v", err)
	}

	// 4. Wire Components according to application architecture
	reg := router.NewStaticServiceRegistry(loadedCfg.Services)
	r := router.NewPrefixRouter(loadedCfg.Routes)
	obsStore := observability.NewStore(100)

	healthStore := health.NewHealthStore(loadedCfg.Services)
	healthChecker := health.NewChecker(200*time.Millisecond, 50*time.Millisecond)
	healthWorker := health.NewWorker(loadedCfg.Services, healthStore, healthChecker, 50*time.Millisecond)

	simStore := simulation.NewSimulationStore(loadedCfg.Services)
	rlStore := ratelimit.NewRateLimiterStore(loadedCfg.Services)

	px := proxy.NewServiceProxy(2 * time.Second)

	gwHandler := gateway.NewGatewayHandler(r, reg, px, obsStore, 2*time.Second)
	gwHandler.SetSimulationController(simStore)
	gwHandler.SetRateLimiter(rlStore)

	apiHandler := api.NewHandler(obsStore, healthStore)
	apiHandler.SetSimulationStore(simStore)
	apiHandler.SetRateLimiterStore(rlStore)
	apiHandler.SetAPIKey("integration-secret-key")
	apiHandler.SetAllowedOrigin("http://localhost:3000")
	gwHandler.SetAPIHandler(apiHandler)

	gwServer := httptest.NewServer(gwHandler)
	defer gwServer.Close()

	// 5. Start Background Health Worker
	healthWorker.Start()

	// 6. Test Proxied Request Execution
	clientReq, err := http.NewRequest(http.MethodGet, gwServer.URL+"/users/100?active=true", nil)
	if err != nil {
		t.Fatalf("failed to create client request: %v", err)
	}
	clientReq.Host = "my-client.example.com"

	resp, err := http.DefaultClient.Do(clientReq)
	if err != nil {
		t.Fatalf("gateway request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200 OK, got %d", resp.StatusCode)
	}

	reqID := resp.Header.Get("X-Request-ID")
	if reqID == "" || !strings.HasPrefix(reqID, "req_") {
		t.Errorf("expected valid X-Request-ID starting with 'req_', got '%s'", reqID)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("failed reading response body: %v", err)
	}
	if !strings.Contains(string(body), "user_id") {
		t.Errorf("unexpected body content: %s", string(body))
	}

	// Verify Upstream Received Request Details
	if receivedPath != "/v1/users/100" {
		t.Errorf("expected subpath joined upstream path '/v1/users/100', got '%s'", receivedPath)
	}
	if receivedQuery != "active=true" {
		t.Errorf("expected query 'active=true', got '%s'", receivedQuery)
	}
	if receivedXForwardedHost != "my-client.example.com" {
		t.Errorf("expected X-Forwarded-Host 'my-client.example.com', got '%s'", receivedXForwardedHost)
	}
	if receivedHost == "" {
		t.Error("expected non-empty upstream Host")
	}

	// 7. Test Control API Interaction (POST /api/simulations with API Key)
	postBody, _ := json.Marshal(api.SetSimulationRequest{
		ServiceID: "user-service",
		Mode:      "FAIL",
	})
	postReq, _ := http.NewRequest(http.MethodPost, gwServer.URL+"/api/simulations", bytes.NewBuffer(postBody))
	postReq.Header.Set("Content-Type", "application/json")
	postReq.Header.Set("X-API-Key", "integration-secret-key")

	postResp, err := http.DefaultClient.Do(postReq)
	if err != nil || postResp.StatusCode != http.StatusOK {
		t.Fatalf("control API request failed: %v, status=%v", err, postResp.StatusCode)
	}
	postResp.Body.Close()

	// Verify Simulation Mode Active (subsequent request returns 503)
	failResp, err := http.Get(gwServer.URL + "/users/100")
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	failResp.Body.Close()
	if failResp.StatusCode != http.StatusServiceUnavailable {
		t.Errorf("expected 503 Service Unavailable during FAIL simulation, got %d", failResp.StatusCode)
	}

	// 8. Verify Clean Worker & Server Shutdown
	shutdownDone := make(chan struct{})
	go func() {
		healthWorker.Stop()
		close(shutdownDone)
	}()

	select {
	case <-shutdownDone:
		// Clean shutdown completed
	case <-time.After(2 * time.Second):
		t.Fatal("health worker shutdown timed out or deadlocked")
	}
}

// TestE2EInvalidConfiguration verifies that malformed configuration files on disk
// fail parsing or validation cleanly without panic or silent bad state.
func TestE2EInvalidConfiguration(t *testing.T) {
	tempDir := t.TempDir()

	testCases := []struct {
		name        string
		configJSON  string
		expectedErr string
	}{
		{
			name:        "missing listen address",
			configJSON:  `{"listen_addr":"","services":[{"id":"s1","upstream":"http://127.0.0.1:8080"}],"routes":[{"path":"/s1/","service":"s1"}]}`,
			expectedErr: "missing listen address",
		},
		{
			name:        "empty service id",
			configJSON:  `{"listen_addr":"127.0.0.1:8080","services":[{"id":"","upstream":"http://127.0.0.1:8080"}],"routes":[]}`,
			expectedErr: "empty service ID",
		},
		{
			name:        "invalid upstream url scheme",
			configJSON:  `{"listen_addr":"127.0.0.1:8080","services":[{"id":"s1","upstream":"ftp://127.0.0.1:8080"}],"routes":[{"path":"/s1/","service":"s1"}]}`,
			expectedErr: "unsupported upstream URL scheme",
		},
		{
			name:        "route referencing unknown service",
			configJSON:  `{"listen_addr":"127.0.0.1:8080","services":[{"id":"s1","upstream":"http://127.0.0.1:8080"}],"routes":[{"path":"/s2/","service":"unknown"}]}`,
			expectedErr: "route referencing unknown service",
		},
		{
			name:        "invalid json syntax",
			configJSON:  `{"listen_addr": "127.0.0.1:8080", ...invalid}`,
			expectedErr: "failed to unmarshal JSON config",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(tempDir, tc.name+".json")
			if err := os.WriteFile(path, []byte(tc.configJSON), 0644); err != nil {
				t.Fatalf("failed to write test config file: %v", err)
			}

			f, err := os.Open(path)
			if err != nil {
				t.Fatalf("failed to open config file: %v", err)
			}
			defer f.Close()

			_, err = config.LoadConfig(f)
			if err == nil {
				t.Fatalf("expected configuration error containing '%s', got nil error", tc.expectedErr)
			}
			if !strings.Contains(err.Error(), tc.expectedErr) {
				t.Errorf("expected error to contain '%s', got '%v'", tc.expectedErr, err)
			}
		})
	}
}

// TestE2ECORSAndPreflight verifies OPTIONS preflight handling and CORS headers.
func TestE2ECORSAndPreflight(t *testing.T) {
	obsStore := observability.NewStore(100)
	apiHandler := api.NewHandler(obsStore)
	apiHandler.SetAllowedOrigin("http://frontend.example.com")

	r := router.NewPrefixRouter(nil)
	reg := router.NewStaticServiceRegistry(nil)
	px := proxy.NewServiceProxy(1 * time.Second)

	gwHandler := gateway.NewGatewayHandler(r, reg, px, obsStore, 1 * time.Second)
	gwHandler.SetAPIHandler(apiHandler)

	gwServer := httptest.NewServer(gwHandler)
	defer gwServer.Close()

	// 1. OPTIONS Preflight Request
	reqOpt, _ := http.NewRequest(http.MethodOptions, gwServer.URL+"/api/metrics", nil)
	reqOpt.Header.Set("Origin", "http://frontend.example.com")
	reqOpt.Header.Set("Access-Control-Request-Method", "GET")

	respOpt, err := http.DefaultClient.Do(reqOpt)
	if err != nil {
		t.Fatalf("OPTIONS request failed: %v", err)
	}
	defer respOpt.Body.Close()

	if respOpt.StatusCode != http.StatusNoContent {
		t.Errorf("expected 204 No Content for preflight OPTIONS, got %d", respOpt.StatusCode)
	}
	if origin := respOpt.Header.Get("Access-Control-Allow-Origin"); origin != "http://frontend.example.com" {
		t.Errorf("expected Access-Control-Allow-Origin 'http://frontend.example.com', got '%s'", origin)
	}

	// 2. GET Request CORS Headers
	respGet, err := http.Get(gwServer.URL + "/api/health")
	if err != nil {
		t.Fatalf("GET request failed: %v", err)
	}
	defer respGet.Body.Close()

	if origin := respGet.Header.Get("Access-Control-Allow-Origin"); origin != "http://frontend.example.com" {
		t.Errorf("expected Access-Control-Allow-Origin header on GET response, got '%s'", origin)
	}
}
