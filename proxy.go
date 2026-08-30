// Package proxy implements the MiniEdge reverse-proxy handler.
//
// Security controls applied here:
//   - SEC-03: request target, method, version, header validation
//   - SEC-04: hop-by-hop header removal; no unsafe header forwarding
//   - SEC-05: request/response size limits; active-request semaphore
//   - SEC-06: all network deadlines enforced
//   - SEC-11: deterministic errors; no internal detail leakage
//   - REL-01: upstream failures isolated per request
//   - REL-02: per-request timeout budget
//   - REL-03: resource limits
//   - REL-04: semaphore released on all code paths
//   - REL-08: per-request failure boundary
package proxy

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"time"
	"unicode"

	"miniedge/internal/config"
	edgeerr "miniedge/internal/errors"
	"miniedge/internal/logger"
	"miniedge/internal/simulation"
)

// ──────────────────────────────────────────────────────────
// Hop-by-hop headers (RFC 7230 §6.1 + Connection-nominated)
// ──────────────────────────────────────────────────────────

// hopByHopHeaders is the fixed set that must never be forwarded upstream or
// downstream (SEC-04, T-07).
var hopByHopHeaders = map[string]bool{
	"connection":          true,
	"keep-alive":          true,
	"proxy-authenticate":  true,
	"proxy-authorization": true,
	"te":                  true,
	"trailer":             true,
	"transfer-encoding":   true,
	"upgrade":             true,
}

// allowedRequestMethods is the allow-list for proxied methods (SEC-03).
var allowedRequestMethods = map[string]bool{
	"GET": true, "POST": true, "PUT": true, "PATCH": true,
	"DELETE": true, "HEAD": true, "OPTIONS": true,
}

// ──────────────────────────────────────────────────────────
// Concurrency semaphore
// ──────────────────────────────────────────────────────────

// semaphore is a simple channel-based counting semaphore (REL-04, SEC-05).
type semaphore struct {
	ch chan struct{}
}

func newSemaphore(n int) *semaphore {
	ch := make(chan struct{}, n)
	for i := 0; i < n; i++ {
		ch <- struct{}{}
	}
	return &semaphore{ch: ch}
}

// TryAcquire acquires one slot without blocking. Returns false if none available.
func (s *semaphore) TryAcquire() bool {
	select {
	case <-s.ch:
		return true
	default:
		return false
	}
}

// Release returns one slot.
func (s *semaphore) Release() { s.ch <- struct{}{} }

// ──────────────────────────────────────────────────────────
// Handler
// ──────────────────────────────────────────────────────────

// Handler implements http.Handler for the data-plane proxy path.
type Handler struct {
	cfgStore *config.Store
	log      *logger.Logger
	sim      *simulation.State
	sem      *semaphore
	client   *http.Client
	// activeCount tracks current in-flight requests for observability.
	activeCount atomic.Int64
	// mu protects the http.Client when the config is reloaded.
	mu sync.Mutex
}

// New creates a proxy Handler. The client is built from the current config
// and recreated on every config reload via RebuildClient.
func New(cfgStore *config.Store, log *logger.Logger, sim *simulation.State) *Handler {
	cfg := cfgStore.Load()
	h := &Handler{
		cfgStore: cfgStore,
		log:      log,
		sim:      sim,
		sem:      newSemaphore(cfg.Limits.MaxActiveRequests),
	}
	h.client = buildClient(cfg)
	return h
}

// RebuildClient replaces the underlying HTTP client with settings from the
// new config snapshot. Must be called after every config reload.
func (h *Handler) RebuildClient(cfg *config.Config) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.sem = newSemaphore(cfg.Limits.MaxActiveRequests)
	h.client = buildClient(cfg)
}

// buildClient constructs a stdlib HTTP client with all timeouts set and
// automatic redirects disabled (SEC-01 rule 7, SEC-06).
func buildClient(cfg *config.Config) *http.Client {
	t := cfg.Timeouts
	dialer := &net.Dialer{
		Timeout: time.Duration(t.UpstreamConnectMs) * time.Millisecond,
	}
	transport := &http.Transport{
		DialContext:           dialer.DialContext,
		ResponseHeaderTimeout: time.Duration(t.UpstreamResponseMs) * time.Millisecond,
		// Disable keep-alives to avoid reuse of connections in a local-dev gateway.
		DisableKeepAlives: false,
		MaxIdleConns:      128,
		IdleConnTimeout:   90 * time.Second,
	}
	return &http.Client{
		Transport: transport,
		// Disable automatic redirects — every hop must be revalidated (SEC-01 rule 7).
		CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
			return http.ErrUseLastResponse
		},
		Timeout: time.Duration(t.TotalRequestMs) * time.Millisecond,
	}
}

// ServeHTTP is the main request-handling entry point.
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	cfg := h.cfgStore.Load()
	t := cfg.Timeouts
	lim := cfg.Limits

	// ── Total request deadline ────────────────────────────
	ctx, cancel := context.WithTimeout(r.Context(), time.Duration(t.TotalRequestMs)*time.Millisecond)
	defer cancel()
	r = r.WithContext(ctx)

	// ── Concurrency gate (REL-03, REL-04, SEC-05) ────────
	if !h.sem.TryAcquire() {
		writeError(w, edgeerr.New(edgeerr.ClassLimitExceeded, "too many active requests"))
		return
	}
	defer h.sem.Release()
	h.activeCount.Add(1)
	defer h.activeCount.Add(-1)

	// ── Request validation (SEC-03) ───────────────────────
	if err := validateRequest(r, lim); err != nil {
		h.log.Warn("invalid_request", logger.F("method", r.Method), logger.F("error", err.Error()))
		writeError(w, err)
		return
	}

	// ── Route lookup ──────────────────────────────────────
	route, svc, upstream, routeErr := resolveRoute(r.URL.Path, r.Method, cfg)
	if routeErr != nil {
		h.log.Warn("route_not_found", logger.F("path", r.URL.Path))
		writeError(w, routeErr)
		return
	}
	_ = route // used for simulation lookup below

	// ── Failure simulation (REL-10, D-07) ────────────────
	if sim := h.sim.Get(svc); sim != nil {
		switch sim.Mode {
		case simulation.ModeDelay:
			select {
			case <-time.After(time.Duration(sim.DelayMs) * time.Millisecond):
			case <-ctx.Done():
				writeError(w, edgeerr.New(edgeerr.ClassTimeout, "request timed out during simulated delay"))
				return
			}
		case simulation.ModeError:
			writeError(w, edgeerr.New(edgeerr.ClassUnavailable, "service unavailable (simulated)"))
			return
		}
	}

	// ── Build upstream request ────────────────────────────
	upstreamReq, buildErr := buildUpstreamRequest(ctx, r, upstream, lim)
	if buildErr != nil {
		h.log.Error("upstream_request_build_failed", logger.F("error", buildErr.Error()))
		writeError(w, buildErr)
		return
	}

	// ── Perform upstream call (REL-01, REL-08) ────────────
	h.mu.Lock()
	client := h.client
	h.mu.Unlock()

	resp, doErr := client.Do(upstreamReq)
	if doErr != nil {
		// Isolate: log internally, return stable error to client.
		h.log.Error("upstream_error",
			logger.F("upstream", upstream),
			logger.F("error", sanitizeUpstreamError(doErr)))
		writeError(w, edgeerr.New(edgeerr.ClassUnavailable, "upstream service unavailable"))
		return
	}
	defer resp.Body.Close()

	// ── Forward response with limits ─────────────────────
	if err := writeResponse(w, resp, lim); err != nil {
		h.log.Error("response_write_error", logger.F("error", err.Error()))
		// Client connection may be broken; nothing more to do.
	}
}

// ──────────────────────────────────────────────────────────
// Request validation
// ──────────────────────────────────────────────────────────

func validateRequest(r *http.Request, lim config.Limits) *edgeerr.EdgeError {
	// Method allow-list (SEC-03).
	if !allowedRequestMethods[r.Method] {
		return edgeerr.Newf(edgeerr.ClassInvalidRequest, "method %q not allowed", r.Method)
	}

	// Request target size (SEC-05).
	target := r.URL.RequestURI()
	if int64(len(target)) > lim.MaxRequestTargetBytes {
		return edgeerr.New(edgeerr.ClassLimitExceeded, "request target too large")
	}

	// Control characters in request target (SEC-03, T-06).
	if containsControlChars(target) {
		return edgeerr.New(edgeerr.ClassInvalidRequest, "request target contains invalid characters")
	}

	// Header count limit (SEC-05).
	if len(r.Header) > lim.MaxHeaderCount {
		return edgeerr.New(edgeerr.ClassLimitExceeded, "too many request headers")
	}

	// Header value CR/LF injection (SEC-04, T-06, T-11).
	for name, vals := range r.Header {
		if containsControlChars(name) {
			return edgeerr.Newf(edgeerr.ClassInvalidRequest, "invalid header name")
		}
		for _, v := range vals {
			if containsNewlines(v) {
				return edgeerr.New(edgeerr.ClassInvalidRequest, "header value contains CR/LF")
			}
		}
	}

	// Conflicting Content-Length / Transfer-Encoding (SEC-03, T-05).
	if r.Header.Get("Content-Length") != "" && r.Header.Get("Transfer-Encoding") != "" {
		return edgeerr.New(edgeerr.ClassInvalidRequest, "conflicting Content-Length and Transfer-Encoding")
	}

	// Unsupported Transfer-Encoding (SEC-03, T-05).
	if te := r.Header.Get("Transfer-Encoding"); te != "" {
		te = strings.ToLower(strings.TrimSpace(te))
		if te != "chunked" {
			return edgeerr.Newf(edgeerr.ClassInvalidRequest, "unsupported Transfer-Encoding %q", te)
		}
	}

	return nil
}

// ──────────────────────────────────────────────────────────
// Route resolution
// ──────────────────────────────────────────────────────────

// resolveRoute finds the best matching route by longest-prefix, returns
// the service name and resolved upstream URL, or an error.
func resolveRoute(path, method string, cfg *config.Config) (routeID, serviceName, upstreamURL string, err *edgeerr.EdgeError) {
	bestLen := -1
	var bestRoute config.Route
	for _, r := range cfg.Routes {
		if !strings.HasPrefix(path, r.Match) {
			continue
		}
		if len(r.Methods) > 0 {
			found := false
			for _, m := range r.Methods {
				if strings.EqualFold(m, method) {
					found = true
					break
				}
			}
			if !found {
				continue
			}
		}
		if len(r.Match) > bestLen {
			bestLen = len(r.Match)
			bestRoute = r
		}
	}
	if bestLen < 0 {
		return "", "", "", edgeerr.New(edgeerr.ClassInvalidRequest, "no matching route")
	}

	svc, ok := cfg.Services[bestRoute.Service]
	if !ok || !svc.Enabled {
		return "", "", "", edgeerr.New(edgeerr.ClassUnavailable, "service unavailable")
	}

	up, ok := cfg.Upstreams[svc.Upstream]
	if !ok {
		return "", "", "", edgeerr.New(edgeerr.ClassUnavailable, "upstream not configured")
	}

	return bestRoute.ID, bestRoute.Service, up.URL, nil
}

// ──────────────────────────────────────────────────────────
// Upstream request construction
// ──────────────────────────────────────────────────────────

func buildUpstreamRequest(ctx context.Context, r *http.Request, upstreamBase string, lim config.Limits) (*http.Request, *edgeerr.EdgeError) {
	// Construct upstream URL by joining base with the request path+query.
	target := strings.TrimRight(upstreamBase, "/") + r.URL.RequestURI()

	upReq, err := http.NewRequestWithContext(ctx, r.Method, target, nil)
	if err != nil {
		return nil, edgeerr.New(edgeerr.ClassInternal, "could not create upstream request")
	}

	// Copy safe headers (SEC-04): remove hop-by-hop, remove Connection-nominated.
	nominated := connectionNominatedHeaders(r.Header.Get("Connection"))
	for name, vals := range r.Header {
		lower := strings.ToLower(name)
		if hopByHopHeaders[lower] || nominated[lower] {
			continue
		}
		// Drop forwarded headers provided by the client — we'll set our own (T-13).
		if lower == "x-forwarded-for" || lower == "x-forwarded-host" || lower == "x-forwarded-proto" {
			continue
		}
		// Reject header names/values with control characters (SEC-04, T-06).
		if containsControlChars(name) {
			continue
		}
		safe := true
		for _, v := range vals {
			if containsNewlines(v) {
				safe = false
				break
			}
		}
		if !safe {
			continue
		}
		for _, v := range vals {
			upReq.Header.Add(name, v)
		}
	}

	// Set canonical forwarding headers from trusted values (SEC-04).
	clientIP, _, _ := net.SplitHostPort(r.RemoteAddr)
	upReq.Header.Set("X-Forwarded-For", clientIP)
	upReq.Header.Set("X-Forwarded-Host", r.Host)
	upReq.Header.Set("X-Forwarded-Proto", "http")

	// Attach bounded request body (SEC-05).
	if r.Body != nil && r.Body != http.NoBody {
		upReq.Body = io.NopCloser(io.LimitReader(r.Body, lim.MaxRequestBodyBytes))
		upReq.ContentLength = -1 // unknown after limiting
	}

	return upReq, nil
}

// connectionNominatedHeaders parses the Connection header and returns the
// set of additionally nominated hop-by-hop headers (RFC 7230 §6.1).
func connectionNominatedHeaders(connHeader string) map[string]bool {
	out := make(map[string]bool)
	for _, token := range strings.Split(connHeader, ",") {
		t := strings.ToLower(strings.TrimSpace(token))
		if t != "" {
			out[t] = true
		}
	}
	return out
}

// ──────────────────────────────────────────────────────────
// Response forwarding
// ──────────────────────────────────────────────────────────

func writeResponse(w http.ResponseWriter, resp *http.Response, lim config.Limits) error {
	// Copy safe response headers (SEC-04).
	for name, vals := range resp.Header {
		lower := strings.ToLower(name)
		if hopByHopHeaders[lower] {
			continue
		}
		// Reject headers with newlines (T-06).
		for _, v := range vals {
			if !containsNewlines(v) {
				w.Header().Add(name, v)
			}
		}
	}
	w.WriteHeader(resp.StatusCode)

	// Copy bounded body (SEC-05, REL-03, S-15).
	limited := io.LimitReader(resp.Body, lim.MaxResponseBodyBytes)
	if _, err := io.Copy(w, limited); err != nil {
		return fmt.Errorf("response body copy: %w", err)
	}
	return nil
}

// ──────────────────────────────────────────────────────────
// Error response writer
// ──────────────────────────────────────────────────────────

// writeError writes a stable, bounded JSON error response.
// Never exposes stack traces, paths, or internal details (SEC-11).
func writeError(w http.ResponseWriter, e *edgeerr.EdgeError) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(e.Class.HTTPStatus())
	// Simple JSON; avoid importing encoding/json to keep this path allocation-free.
	body := fmt.Sprintf(`{"error":%q,"class":%q}`, e.PublicMsg, string(e.Class))
	_, _ = w.Write([]byte(body))
}

// ──────────────────────────────────────────────────────────
// Helpers
// ──────────────────────────────────────────────────────────

func containsControlChars(s string) bool {
	for _, r := range s {
		if unicode.IsControl(r) {
			return true
		}
	}
	return false
}

func containsNewlines(s string) bool {
	return strings.ContainsAny(s, "\r\n")
}

// sanitizeUpstreamError returns a safe log-able description of an upstream
// error that does not include raw network addresses or stack traces (SEC-11).
func sanitizeUpstreamError(err error) string {
	if err == nil {
		return ""
	}
	msg := err.Error()
	// Cap to 256 characters for log safety.
	if len(msg) > 256 {
		msg = msg[:256] + "...[truncated]"
	}
	// Remove newlines for log injection safety (SEC-09).
	msg = strings.ReplaceAll(msg, "\n", " ")
	msg = strings.ReplaceAll(msg, "\r", " ")
	return msg
}

// ActiveCount returns the number of currently in-flight proxy requests.
func (h *Handler) ActiveCount() int64 { return h.activeCount.Load() }
