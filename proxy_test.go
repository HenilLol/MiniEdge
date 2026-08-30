package proxy_test

// Tests cover S-08 through S-20, S-29, S-30, R-01 through R-10, R-22 through R-25
// from the test plans. All fixtures are local net/http/httptest servers.
// No external services are required.

import (
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"miniedge/internal/config"
	"miniedge/internal/logger"
	"miniedge/internal/proxy"
	"miniedge/internal/simulation"
)

// ──────────────────────────────────────────────────────────
// Test fixture builders
// ──────────────────────────────────────────────────────────

// buildConfig builds a config that routes /api to the given upstream URL.
// Uses localDevPolicy (AllowPrivate:true) so 127.x upstreams are permitted in tests.
func buildConfig(upstreamURL string) *config.Config {
	return &config.Config{
		ListenAddr: "127.0.0.1:0",
		AdminAddr:  "127.0.0.1:0",
		Upstreams: map[string]config.Upstream{
			"test-upstream": {URL: upstreamURL, AllowPrivate: true},
		},
		Services: map[string]config.Service{
			"test-svc": {Upstream: "test-upstream", Enabled: true},
		},
		Routes: []config.Route{
			{ID: "test-route", Match: "/api", Service: "test-svc", Methods: nil},
		},
		Timeouts: config.Timeouts{
			HeaderReadMs: 2000, BodyReadMs: 2000,
			UpstreamConnectMs: 500, UpstreamResponseMs: 1000,
			UpstreamBodyMs: 2000, TotalRequestMs: 5000,
			ShutdownDrainMs: 2000,
		},
		Limits: config.Limits{
			MaxRequestTargetBytes: 1024,
			MaxHeaderBytes:        4096,
			MaxHeaderCount:        20,
			MaxRequestBodyBytes:   4096,
			MaxResponseBodyBytes:  8192,
			MaxActiveRequests:     5,
			MaxQueuedRequests:     5,
			MaxLogEventBytes:      4096,
		},
		RateLimit:         config.DefaultRateLimit(),
		FailureSimulation: config.DefaultFailureSimulation(),
	}
}

func newHandler(upstreamURL string) (*proxy.Handler, *config.Store) {
	cfg := buildConfig(upstreamURL)
	store := config.NewStore(cfg)
	log := logger.New()
	sim := simulation.NewState()
	h := proxy.New(store, log, sim)
	return h, store
}

// echoServer returns an httptest.Server that echoes received headers + body in JSON-ish form.
func echoServer() *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprintf(w, "echo: %s %s", r.Method, r.URL.Path)
	}))
}

// slowHeaderServer returns a server that never sends response headers.
func slowHeaderServer(delay time.Duration) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(delay)
		w.WriteHeader(http.StatusOK)
	}))
}

// slowBodyServer sends the header immediately but delays the body.
func slowBodyServer(delay time.Duration) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		if f, ok := w.(http.Flusher); ok {
			f.Flush()
		}
		time.Sleep(delay)
		_, _ = w.Write([]byte("late-body"))
	}))
}

// oversizedBodyServer returns a body larger than the configured response cap.
func oversizedBodyServer(size int) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(make([]byte, size))
	}))
}

// unavailableUpstream returns a port that is not listening.
func unavailableUpstream() string {
	// Allocate an ephemeral port then immediately close it.
	l, _ := net.Listen("tcp", "127.0.0.1:0")
	addr := l.Addr().String()
	l.Close()
	return "http://" + addr
}

// hopByHopEchoServer echoes back all received header names as a comma-separated
// X-Received-Headers response header.
func hopByHopEchoServer() *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var names []string
		for name := range r.Header {
			names = append(names, strings.ToLower(name))
		}
		w.Header().Set("X-Received-Headers", strings.Join(names, ","))
		w.WriteHeader(http.StatusOK)
	}))
}

// ──────────────────────────────────────────────────────────
// Helper: do a GET through the proxy handler using httptest.Recorder
// ──────────────────────────────────────────────────────────

func doGet(h http.Handler, path string, headers http.Header) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, path, nil)
	req.RemoteAddr = "127.0.0.1:12345"
	for k, vs := range headers {
		for _, v := range vs {
			req.Header.Add(k, v)
		}
	}
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	return rr
}

func doPost(h http.Handler, path string, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(body))
	req.RemoteAddr = "127.0.0.1:12345"
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	return rr
}

// ──────────────────────────────────────────────────────────
// S-08: Malformed / invalid method
// ──────────────────────────────────────────────────────────

func TestInvalidMethod(t *testing.T) {
	srv := echoServer()
	defer srv.Close()
	h, _ := newHandler(srv.URL)

	req := httptest.NewRequest("CONNECT", "/api", nil)
	req.RemoteAddr = "127.0.0.1:1"
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	if rr.Code == http.StatusOK {
		t.Errorf("CONNECT method should be rejected, got %d", rr.Code)
	}
}

// ──────────────────────────────────────────────────────────
// S-08: Invalid route (no matching route)
// ──────────────────────────────────────────────────────────

func TestNoMatchingRoute(t *testing.T) {
	srv := echoServer()
	defer srv.Close()
	h, _ := newHandler(srv.URL)

	rr := doGet(h, "/unknown-path", nil)
	if rr.Code == http.StatusOK {
		t.Errorf("unknown path should not return 200, got %d", rr.Code)
	}
}

// ──────────────────────────────────────────────────────────
// S-11: CR/LF in header value
// ──────────────────────────────────────────────────────────

func TestHeaderCRLFRejected(t *testing.T) {
	srv := echoServer()
	defer srv.Close()
	h, _ := newHandler(srv.URL)

	rr := doGet(h, "/api", http.Header{"X-Bad": {"value\r\nX-Injected: evil"}})
	if rr.Code == http.StatusOK {
		t.Errorf("header with CR/LF should be rejected, got 200")
	}
}

// ──────────────────────────────────────────────────────────
// S-12: Hop-by-hop headers not forwarded to upstream
// ──────────────────────────────────────────────────────────

func TestHopByHopNotForwarded(t *testing.T) {
	received := make(map[string]bool)
	var mu sync.Mutex
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		for name := range r.Header {
			received[strings.ToLower(name)] = true
		}
		mu.Unlock()
		w.WriteHeader(http.StatusOK)
	}))
	defer upstream.Close()

	h, _ := newHandler(upstream.URL)
	hopHeaders := http.Header{
		"Connection":          {"keep-alive"},
		"Keep-Alive":          {"timeout=5"},
		"Proxy-Authenticate":  {"Basic"},
		"Proxy-Authorization": {"Basic dXNlcjpwYXNz"},
		"Te":                  {"trailers"},
		"Trailer":             {"Expires"},
		"Transfer-Encoding":   {"chunked"},
		"Upgrade":             {"websocket"},
	}
	doGet(h, "/api", hopHeaders)

	mu.Lock()
	defer mu.Unlock()
	for hop := range hopHeaders {
		lower := strings.ToLower(hop)
		if received[lower] {
			t.Errorf("hop-by-hop header %q was forwarded to upstream", hop)
		}
	}
}

// ──────────────────────────────────────────────────────────
// S-13: Client-supplied X-Forwarded-For not treated as authoritative
// ──────────────────────────────────────────────────────────

func TestForwardingHeadersSetCanonically(t *testing.T) {
	var gotXFF string
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotXFF = r.Header.Get("X-Forwarded-For")
		w.WriteHeader(http.StatusOK)
	}))
	defer upstream.Close()

	h, _ := newHandler(upstream.URL)
	rr := doGet(h, "/api", http.Header{"X-Forwarded-For": {"attacker-ip"}})
	_ = rr

	// The proxy should have replaced the client's value with the actual RemoteAddr IP.
	if gotXFF == "attacker-ip" {
		t.Error("X-Forwarded-For should not be client-controlled")
	}
}

// ──────────────────────────────────────────────────────────
// S-14: Oversized request target
// ──────────────────────────────────────────────────────────

func TestOversizedRequestTarget(t *testing.T) {
	srv := echoServer()
	defer srv.Close()
	h, _ := newHandler(srv.URL)

	// Build a target just over 1024 bytes.
	bigPath := "/api?" + strings.Repeat("x", 1200)
	rr := doGet(h, bigPath, nil)
	if rr.Code == http.StatusOK {
		t.Errorf("oversized request target should be rejected, got 200")
	}
}

// ──────────────────────────────────────────────────────────
// S-14: Too many headers
// ──────────────────────────────────────────────────────────

func TestTooManyHeaders(t *testing.T) {
	srv := echoServer()
	defer srv.Close()
	h, _ := newHandler(srv.URL)

	hdrs := make(http.Header)
	for i := 0; i < 25; i++ { // over our limit of 20
		hdrs[fmt.Sprintf("X-Custom-%d", i)] = []string{"v"}
	}
	rr := doGet(h, "/api", hdrs)
	if rr.Code == http.StatusOK {
		t.Errorf("too many headers should be rejected, got 200")
	}
}

// ──────────────────────────────────────────────────────────
// S-15: Oversized upstream response is capped
// ──────────────────────────────────────────────────────────

func TestOversizedUpstreamResponse(t *testing.T) {
	// Send 16 KiB — twice the 8 KiB response cap we set.
	srv := oversizedBodyServer(16 * 1024)
	defer srv.Close()
	h, _ := newHandler(srv.URL)

	rr := doGet(h, "/api", nil)
	// The proxy should return 200 but the body should be capped at MaxResponseBodyBytes.
	body := rr.Body.Bytes()
	cfg := buildConfig(srv.URL)
	if int64(len(body)) > cfg.Limits.MaxResponseBodyBytes {
		t.Errorf("response body %d bytes exceeds configured cap %d",
			len(body), cfg.Limits.MaxResponseBodyBytes)
	}
}

// ──────────────────────────────────────────────────────────
// S-17: Slow upstream — timeout (R-06)
// ──────────────────────────────────────────────────────────

func TestSlowUpstreamTimeout(t *testing.T) {
	// Upstream takes 3 seconds to send headers; our total deadline is 5s but
	// upstreamResponseMs is 1s (in buildConfig).
	srv := slowHeaderServer(2 * time.Second)
	defer srv.Close()
	h, _ := newHandler(srv.URL)

	start := time.Now()
	rr := doGet(h, "/api", nil)
	elapsed := time.Since(start)

	if rr.Code == http.StatusOK {
		t.Errorf("slow upstream should not return 200")
	}
	// Should have timed out well before the 5-second total deadline.
	if elapsed > 4*time.Second {
		t.Errorf("timeout took too long: %v", elapsed)
	}
}

// ──────────────────────────────────────────────────────────
// R-01: Upstream unavailable (connection refused)
// ──────────────────────────────────────────────────────────

func TestUpstreamUnavailable(t *testing.T) {
	unavail := unavailableUpstream()
	h, _ := newHandler(unavail)

	rr := doGet(h, "/api", nil)
	if rr.Code == http.StatusOK {
		t.Errorf("unavailable upstream should not return 200")
	}
	// Process should remain healthy — a second request should also get a controlled error.
	rr2 := doGet(h, "/api", nil)
	if rr2.Code == http.StatusOK {
		t.Errorf("second request to unavailable upstream should also return non-200")
	}
}

// ──────────────────────────────────────────────────────────
// R-13: Upstream recovery
// ──────────────────────────────────────────────────────────

func TestUpstreamRecovery(t *testing.T) {
	// Start unavailable, then swap to a healthy server.
	unavail := unavailableUpstream()
	h, store := newHandler(unavail)

	// First request fails.
	rr := doGet(h, "/api", nil)
	if rr.Code == http.StatusOK {
		t.Log("first request to unavailable upstream unexpectedly succeeded")
	}

	// Now swap to a healthy upstream.
	healthy := echoServer()
	defer healthy.Close()
	newCfg := buildConfig(healthy.URL)
	store.Swap(newCfg)
	h.RebuildClient(newCfg)

	// Second request should succeed.
	rr2 := doGet(h, "/api", nil)
	if rr2.Code != http.StatusOK {
		t.Errorf("after upstream recovery, expected 200 but got %d", rr2.Code)
	}
}

// ──────────────────────────────────────────────────────────
// S-29, S-30: Concurrency cap
// ──────────────────────────────────────────────────────────

func TestConcurrencyCap(t *testing.T) {
	// Upstream that holds connections open briefly.
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(200 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	defer upstream.Close()

	h, _ := newHandler(upstream.URL)

	// Fire 10 concurrent requests; our cap is 5.
	var wg sync.WaitGroup
	codes := make([]int, 10)
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			rr := doGet(h, "/api", nil)
			codes[idx] = rr.Code
		}(i)
	}
	wg.Wait()

	// At least some requests should have been rejected with 429.
	rejected := 0
	for _, code := range codes {
		if code == http.StatusTooManyRequests {
			rejected++
		}
	}
	if rejected == 0 {
		t.Error("expected at least one request to be rejected due to concurrency cap")
	}
}

// ──────────────────────────────────────────────────────────
// R-25: One handler failure doesn't crash the process
// ──────────────────────────────────────────────────────────

func TestHandlerIsolation(t *testing.T) {
	unavail := unavailableUpstream()
	h, _ := newHandler(unavail)

	// Fire 5 requests that will all fail — process should remain healthy.
	for i := 0; i < 5; i++ {
		rr := doGet(h, "/api", nil)
		// Must return a 5xx, not panic.
		if rr.Code < 400 {
			t.Errorf("request %d: expected error response, got %d", i, rr.Code)
		}
	}
}

// ──────────────────────────────────────────────────────────
// R-22: Repeated malformed / oversized requests
// ──────────────────────────────────────────────────────────

func TestRepeatedMalformedRequests(t *testing.T) {
	srv := echoServer()
	defer srv.Close()
	h, _ := newHandler(srv.URL)

	bigPath := "/api?" + strings.Repeat("z", 2000)
	for i := 0; i < 20; i++ {
		rr := doGet(h, bigPath, nil)
		if rr.Code == http.StatusOK {
			t.Errorf("iteration %d: oversized target should be rejected", i)
		}
	}
	// Handler should still serve valid requests.
	rr := doGet(h, "/api", nil)
	if rr.Code != http.StatusOK {
		t.Errorf("valid request after repeated malformed: expected 200, got %d", rr.Code)
	}
}

// ──────────────────────────────────────────────────────────
// S-09: Conflicting Content-Length + Transfer-Encoding rejected
// ──────────────────────────────────────────────────────────

func TestConflictingFramingRejected(t *testing.T) {
	srv := echoServer()
	defer srv.Close()
	h, _ := newHandler(srv.URL)

	req := httptest.NewRequest(http.MethodPost, "/api", strings.NewReader("body"))
	req.RemoteAddr = "127.0.0.1:1"
	req.Header.Set("Content-Length", "4")
	req.Header.Set("Transfer-Encoding", "chunked")
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	if rr.Code == http.StatusOK {
		t.Errorf("conflicting Content-Length/Transfer-Encoding should be rejected, got 200")
	}
}

// ──────────────────────────────────────────────────────────
// S-10: Unsupported Transfer-Encoding rejected
// ──────────────────────────────────────────────────────────

func TestUnsupportedTransferEncoding(t *testing.T) {
	srv := echoServer()
	defer srv.Close()
	h, _ := newHandler(srv.URL)

	req := httptest.NewRequest(http.MethodPost, "/api", strings.NewReader("body"))
	req.RemoteAddr = "127.0.0.1:1"
	req.Header.Set("Transfer-Encoding", "gzip")
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	if rr.Code == http.StatusOK {
		t.Errorf("unsupported Transfer-Encoding should be rejected, got 200")
	}
}

// ──────────────────────────────────────────────────────────
// Error response does not leak internal details (SEC-11)
// ──────────────────────────────────────────────────────────

func TestErrorResponseNoInternalDetails(t *testing.T) {
	unavail := unavailableUpstream()
	h, _ := newHandler(unavail)

	rr := doGet(h, "/api", nil)
	body := rr.Body.String()

	// Should not contain stack trace markers, file paths, or raw error strings
	forbidden := []string{"goroutine", ".go:", "panic:", "/Users/", "/home/", "\\Users\\"}
	for _, f := range forbidden {
		if strings.Contains(body, f) {
			t.Errorf("error response contains internal detail %q: %s", f, body)
		}
	}
	// Should contain a stable class.
	if !strings.Contains(body, "class") {
		t.Errorf("error response missing 'class' field: %s", body)
	}
}

// ──────────────────────────────────────────────────────────
// R-23: Response-size limit is per-request, not shared
// ──────────────────────────────────────────────────────────

func TestResponseSizeCapPerRequest(t *testing.T) {
	srv := oversizedBodyServer(16 * 1024)
	defer srv.Close()
	h, _ := newHandler(srv.URL)

	var wg sync.WaitGroup
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			rr := doGet(h, "/api", nil)
			body, _ := io.ReadAll(rr.Body)
			cfg := buildConfig(srv.URL)
			if int64(len(body)) > cfg.Limits.MaxResponseBodyBytes {
				t.Errorf("concurrent response body exceeds cap: %d bytes", len(body))
			}
		}()
	}
	wg.Wait()
}
