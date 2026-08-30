// Package server wires all MiniEdge components together and manages the HTTP
// server lifecycle including graceful shutdown (REL-06, SEC-06).
//
// Two listeners are created:
//   - data-plane listener at cfg.ListenAddr (public)
//   - admin listener at cfg.AdminAddr (local-only by default)
//
// This separation implements SEC-08: admin operations are not reachable through
// the public data-plane path.
package server

import (
	"context"
	"net/http"
	"time"

	"miniedge/internal/admin"
	"miniedge/internal/config"
	"miniedge/internal/logger"
	"miniedge/internal/proxy"
	"miniedge/internal/ratelimit"
	"miniedge/internal/simulation"
)

// Server holds both HTTP servers and all shared state.
type Server struct {
	dataPlane *http.Server
	adminSrv  *http.Server
	log       *logger.Logger
	limiter   *ratelimit.Limiter
	sim       *simulation.State
}

// New creates and wires the full MiniEdge server from a validated config.
func New(cfgStore *config.Store, log *logger.Logger, adminToken string) *Server {
	cfg := cfgStore.Load()

	sim := simulation.NewState()

	// Build the proxy handler.
	proxyH := proxy.New(cfgStore, log, sim)

	// Build the rate limiter (P1 but always constructed; disabled in config skips enforcement).
	rl := cfg.RateLimit
	limiter := ratelimit.New(rl.Requests, rl.WindowSeconds, rl.MaxKeys)

	// Data-plane mux: routes all requests through rate-limit middleware → proxy.
	dataMux := http.NewServeMux()
	dataMux.Handle("/", rateLimitMiddleware(proxyH, cfgStore, limiter, log))

	// Admin mux: all admin endpoints.
	adminH := admin.New(cfgStore, proxyH, sim, log, adminToken)
	adminMux := http.NewServeMux()
	adminH.RegisterRoutes(adminMux)

	t := cfg.Timeouts
	dataServer := &http.Server{
		Addr:              cfg.ListenAddr,
		Handler:           dataMux,
		ReadHeaderTimeout: time.Duration(t.HeaderReadMs) * time.Millisecond,
		ReadTimeout:       time.Duration(t.BodyReadMs) * time.Millisecond,
		WriteTimeout:      time.Duration(t.TotalRequestMs) * time.Millisecond,
		// MaxHeaderBytes covers the stdlib limit; our own enforcement is in proxy.
		MaxHeaderBytes: int(cfg.Limits.MaxHeaderBytes),
	}
	adminServer := &http.Server{
		Addr:              cfg.AdminAddr,
		Handler:           adminMux,
		ReadHeaderTimeout: time.Duration(t.HeaderReadMs) * time.Millisecond,
		ReadTimeout:       time.Duration(t.BodyReadMs) * time.Millisecond,
		WriteTimeout:      time.Duration(t.TotalRequestMs) * time.Millisecond,
	}

	return &Server{
		dataPlane: dataServer,
		adminSrv:  adminServer,
		log:       log,
		limiter:   limiter,
		sim:       sim,
	}
}

// Start starts both listeners in background goroutines and returns immediately.
// Errors are sent to the returned channel.
func (s *Server) Start() <-chan error {
	errCh := make(chan error, 2)
	go func() {
		s.log.Info("data_plane_listening", logger.F("addr", s.dataPlane.Addr))
		if err := s.dataPlane.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errCh <- err
		}
	}()
	go func() {
		s.log.Info("admin_listening", logger.F("addr", s.adminSrv.Addr))
		if err := s.adminSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errCh <- err
		}
	}()

	// Background sweep goroutines for rate-limit and simulation tables.
	go s.sweepLoop()

	return errCh
}

// Shutdown performs a bounded, idempotent graceful shutdown (REL-06, SEC-06).
// It stops accepting new requests, drains in-flight work to the deadline, then
// closes resources.
func (s *Server) Shutdown(drainMs int64) {
	s.log.Info("shutdown_initiated")
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(drainMs)*time.Millisecond)
	defer cancel()

	// Stop both listeners concurrently.
	done := make(chan struct{}, 2)
	shutdownOne := func(srv *http.Server) {
		if err := srv.Shutdown(ctx); err != nil {
			s.log.Error("shutdown_error", logger.F("addr", srv.Addr), logger.F("error", err.Error()))
		}
		done <- struct{}{}
	}
	go shutdownOne(s.dataPlane)
	go shutdownOne(s.adminSrv)

	// Wait for both to finish (or deadline).
	<-done
	<-done

	// Sweep any remaining simulations so they don't persist after restart.
	s.sim.Sweep()
	s.log.Info("shutdown_complete")
}

// sweepLoop periodically sweeps the rate-limit and simulation tables to
// reclaim memory from expired entries (REL-05, T-14).
func (s *Server) sweepLoop() {
	// Sweep roughly every 60 seconds — a reasonable cadence for a local-dev service.
	ticker := time.NewTicker(60 * time.Second)
	defer ticker.Stop()
	for range ticker.C {
		s.limiter.Sweep()
		s.sim.Sweep()
	}
}

// ──────────────────────────────────────────────────────────
// Rate-limit middleware
// ──────────────────────────────────────────────────────────

// rateLimitMiddleware wraps the proxy handler with in-memory rate limiting
// (REL-09, D-06). When rate limiting is disabled in config, requests pass through.
func rateLimitMiddleware(
	next http.Handler,
	cfgStore *config.Store,
	limiter *ratelimit.Limiter,
	log *logger.Logger,
) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cfg := cfgStore.Load()
		if cfg.RateLimit.Enabled {
			// Key: client IP only (no path/body to prevent cardinality explosion).
			clientIP, _, err := splitHostPort(r.RemoteAddr)
			if err != nil {
				clientIP = r.RemoteAddr
			}
			if !limiter.Allow(clientIP) {
				log.Warn("rate_limited", logger.F("ip", clientIP))
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusTooManyRequests)
				_, _ = w.Write([]byte(`{"error":"rate limit exceeded","class":"rate_limited"}`))
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}

// splitHostPort is a thin wrapper around net.SplitHostPort that avoids
// importing net in a hot path — just string splitting for IP extraction.
func splitHostPort(hostport string) (host, port string, err error) {
	// Try bracketed IPv6: [::1]:1234
	if len(hostport) > 0 && hostport[0] == '[' {
		i := lastIndex(hostport, ']')
		if i < 0 {
			return "", "", errInvalidAddr
		}
		host = hostport[1:i]
		if i+1 < len(hostport) && hostport[i+1] == ':' {
			port = hostport[i+2:]
		}
		return host, port, nil
	}
	// IPv4 or hostname: host:port
	i := lastIndex(hostport, ':')
	if i < 0 {
		return hostport, "", nil
	}
	return hostport[:i], hostport[i+1:], nil
}

type addrError string

func (e addrError) Error() string { return string(e) }

const errInvalidAddr addrError = "invalid address"

func lastIndex(s, sub string) int {
	for i := len(s) - len(sub); i >= 0; i-- {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
