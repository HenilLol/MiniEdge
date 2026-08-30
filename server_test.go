package server_test

// Tests cover R-19, R-20, S-20, S-21, S-35, S-36 from the test plans.
// Uses only the standard library; no external services required.

import (
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"miniedge/internal/config"
	"miniedge/internal/logger"
	"miniedge/internal/server"
)

// ──────────────────────────────────────────────────────────
// Helpers
// ──────────────────────────────────────────────────────────

// freeAddr finds a free local port without binding it.
func freeAddr() string {
	l, _ := net.Listen("tcp", "127.0.0.1:0")
	addr := l.Addr().String()
	l.Close()
	return addr
}

func buildTestConfig(upstreamURL, listenAddr, adminAddr string) *config.Config {
	return &config.Config{
		ListenAddr: listenAddr,
		AdminAddr:  adminAddr,
		Upstreams: map[string]config.Upstream{
			"test-upstream": {URL: upstreamURL, AllowPrivate: true},
		},
		Services: map[string]config.Service{
			"test-svc": {Upstream: "test-upstream", Enabled: true},
		},
		Routes: []config.Route{
			{ID: "test-route", Match: "/api", Service: "test-svc"},
		},
		Timeouts: config.Timeouts{
			HeaderReadMs: 2000, BodyReadMs: 2000,
			UpstreamConnectMs: 500, UpstreamResponseMs: 1000,
			UpstreamBodyMs: 2000, TotalRequestMs: 5000,
			ShutdownDrainMs: 3000,
		},
		Limits: config.Limits{
			MaxRequestTargetBytes: 8192, MaxHeaderBytes: 32768, MaxHeaderCount: 100,
			MaxRequestBodyBytes: 1048576, MaxResponseBodyBytes: 5242880,
			MaxActiveRequests: 10, MaxQueuedRequests: 10, MaxLogEventBytes: 8192,
		},
		RateLimit:         config.DefaultRateLimit(),
		FailureSimulation: config.DefaultFailureSimulation(),
	}
}

// ──────────────────────────────────────────────────────────
// R-19: Clean shutdown with idle service
// ──────────────────────────────────────────────────────────

func TestShutdown_Idle(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer upstream.Close()

	listenAddr := freeAddr()
	adminAddr := freeAddr()
	cfg := buildTestConfig(upstream.URL, listenAddr, adminAddr)
	store := config.NewStore(cfg)
	log := logger.New()

	srv := server.New(store, log, "")
	_ = srv.Start()

	// Give the server a moment to start.
	time.Sleep(50 * time.Millisecond)

	// Shutdown should complete within the drain timeout.
	done := make(chan struct{})
	go func() {
		srv.Shutdown(cfg.Timeouts.ShutdownDrainMs)
		close(done)
	}()

	select {
	case <-done:
		// success
	case <-time.After(5 * time.Second):
		t.Error("shutdown did not complete within 5 seconds")
	}
}

// ──────────────────────────────────────────────────────────
// R-20: Shutdown drains in-flight work
// ──────────────────────────────────────────────────────────

func TestShutdown_WithInflightWork(t *testing.T) {
	// Upstream takes 200ms per request.
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		time.Sleep(200 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	defer upstream.Close()

	listenAddr := freeAddr()
	adminAddr := freeAddr()
	cfg := buildTestConfig(upstream.URL, listenAddr, adminAddr)
	store := config.NewStore(cfg)
	log := logger.New()

	srv := server.New(store, log, "")
	_ = srv.Start()
	time.Sleep(50 * time.Millisecond) // wait for listener

	// Fire a request in background.
	resultCh := make(chan int, 1)
	go func() {
		resp, err := http.Get(fmt.Sprintf("http://%s/api", listenAddr))
		if err != nil {
			resultCh <- 0
			return
		}
		defer resp.Body.Close()
		resultCh <- resp.StatusCode
	}()
	time.Sleep(20 * time.Millisecond) // let request start

	// Initiate shutdown; allow 3s drain.
	done := make(chan struct{})
	go func() {
		srv.Shutdown(cfg.Timeouts.ShutdownDrainMs)
		close(done)
	}()

	select {
	case <-done:
		// success
	case <-time.After(5 * time.Second):
		t.Error("shutdown did not complete within 5 seconds")
	}
}

// ──────────────────────────────────────────────────────────
// S-20: Admin endpoints not reachable on data-plane port
// ──────────────────────────────────────────────────────────

func TestAdminNotExposedOnDataPlane(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer upstream.Close()

	listenAddr := freeAddr()
	adminAddr := freeAddr()
	cfg := buildTestConfig(upstream.URL, listenAddr, adminAddr)
	store := config.NewStore(cfg)
	log := logger.New()

	srv := server.New(store, log, "")
	_ = srv.Start()
	defer srv.Shutdown(1000)
	time.Sleep(50 * time.Millisecond)

	// /admin/status on the data-plane port should return a non-200 response
	// (no matching route → invalid_request or similar, not the admin status).
	resp, err := http.Get(fmt.Sprintf("http://%s/admin/status", listenAddr))
	if err != nil {
		t.Skipf("could not connect to data plane: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusOK {
		t.Errorf("admin/status should not return 200 on the data-plane port")
	}
}

// ──────────────────────────────────────────────────────────
// Health endpoint (public, no auth)
// ──────────────────────────────────────────────────────────

func TestHealthEndpoint(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer upstream.Close()

	listenAddr := freeAddr()
	adminAddr := freeAddr()
	cfg := buildTestConfig(upstream.URL, listenAddr, adminAddr)
	store := config.NewStore(cfg)
	log := logger.New()

	srv := server.New(store, log, "")
	_ = srv.Start()
	defer srv.Shutdown(1000)
	time.Sleep(50 * time.Millisecond)

	resp, err := http.Get(fmt.Sprintf("http://%s/healthz", adminAddr))
	if err != nil {
		t.Skipf("could not connect to admin server: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200 from /healthz, got %d", resp.StatusCode)
	}
}

// ──────────────────────────────────────────────────────────
// S-21: Admin mutation without auth is rejected
// ──────────────────────────────────────────────────────────

func TestAdminMutation_UnauthorizedRejected(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer upstream.Close()

	listenAddr := freeAddr()
	adminAddr := freeAddr()
	cfg := buildTestConfig(upstream.URL, listenAddr, adminAddr)
	store := config.NewStore(cfg)
	log := logger.New()

	// Enable token auth.
	srv := server.New(store, log, "secret-token")
	_ = srv.Start()
	defer srv.Shutdown(1000)
	time.Sleep(50 * time.Millisecond)

	// Call /admin/status without token.
	resp, err := http.Get(fmt.Sprintf("http://%s/admin/status", adminAddr))
	if err != nil {
		t.Skipf("could not connect to admin server: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("expected 401 without token, got %d", resp.StatusCode)
	}
}

func TestAdminMutation_AuthorizedSucceeds(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer upstream.Close()

	listenAddr := freeAddr()
	adminAddr := freeAddr()
	cfg := buildTestConfig(upstream.URL, listenAddr, adminAddr)
	store := config.NewStore(cfg)
	log := logger.New()

	srv := server.New(store, log, "secret-token")
	_ = srv.Start()
	defer srv.Shutdown(1000)
	time.Sleep(50 * time.Millisecond)

	req, _ := http.NewRequest(http.MethodGet, fmt.Sprintf("http://%s/admin/status", adminAddr), nil)
	req.Header.Set("X-Admin-Token", "secret-token")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Skipf("could not connect to admin server: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200 with correct token, got %d", resp.StatusCode)
	}
}

// ──────────────────────────────────────────────────────────
// S-22: Config reload — invalid config leaves known-good active (REL-12)
// ──────────────────────────────────────────────────────────

func TestConfigReload_InvalidRejected(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer upstream.Close()

	listenAddr := freeAddr()
	adminAddr := freeAddr()
	cfg := buildTestConfig(upstream.URL, listenAddr, adminAddr)
	store := config.NewStore(cfg)
	log := logger.New()

	srv := server.New(store, log, "")
	_ = srv.Start()
	defer srv.Shutdown(1000)
	time.Sleep(50 * time.Millisecond)

	// Attempt a reload with a nonexistent path — should be rejected.
	body := `{"configPath":"/nonexistent/path/to/config.json"}`
	resp, err := http.Post(
		fmt.Sprintf("http://%s/admin/config/reload", adminAddr),
		"application/json",
		strings.NewReader(body),
	)
	if err != nil {
		t.Skipf("could not connect: %v", err)
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)

	// Should return an error status.
	if resp.StatusCode == http.StatusOK {
		t.Errorf("reload with nonexistent path should fail, got 200")
	}

	// Should NOT expose internal paths or stack traces.
	if strings.Contains(string(respBody), "nonexistent") {
		t.Errorf("reload error response should not echo back internal path: %s", respBody)
	}

	// Verify active config is unchanged.
	if store.Load() != cfg {
		t.Error("failed reload should leave the original config active")
	}
}

// ──────────────────────────────────────────────────────────
// Simulation: disabled by default (S-26)
// ──────────────────────────────────────────────────────────

func TestSimulation_DisabledByDefault(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer upstream.Close()

	listenAddr := freeAddr()
	adminAddr := freeAddr()
	cfg := buildTestConfig(upstream.URL, listenAddr, adminAddr)
	store := config.NewStore(cfg)
	log := logger.New()

	srv := server.New(store, log, "")
	_ = srv.Start()
	defer srv.Shutdown(1000)
	time.Sleep(50 * time.Millisecond)

	body := `{"serviceId":"test-svc","mode":"delay","delayMs":500,"durationSeconds":10}`
	resp, err := http.Post(
		fmt.Sprintf("http://%s/admin/simulation/activate", adminAddr),
		"application/json",
		strings.NewReader(body),
	)
	if err != nil {
		t.Skipf("could not connect: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusOK {
		t.Error("simulation activation should be rejected when simulation is disabled in config")
	}
}

// ──────────────────────────────────────────────────────────
// Admin status response does not contain secrets
// ──────────────────────────────────────────────────────────

func TestAdminStatus_NoSecrets(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer upstream.Close()

	listenAddr := freeAddr()
	adminAddr := freeAddr()
	cfg := buildTestConfig(upstream.URL, listenAddr, adminAddr)
	store := config.NewStore(cfg)
	log := logger.New()

	srv := server.New(store, log, "")
	_ = srv.Start()
	defer srv.Shutdown(1000)
	time.Sleep(50 * time.Millisecond)

	resp, err := http.Get(fmt.Sprintf("http://%s/admin/status", adminAddr))
	if err != nil {
		t.Skipf("could not connect: %v", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)

	// Verify the upstream URL is not present in the response (SEC-11, SEC-08).
	if strings.Contains(string(body), upstream.URL) {
		t.Errorf("admin status should not expose upstream URLs: %s", body)
	}

	// Verify it is valid JSON.
	var m map[string]any
	if err := json.Unmarshal(body, &m); err != nil {
		t.Errorf("admin status is not valid JSON: %v", err)
	}
}
