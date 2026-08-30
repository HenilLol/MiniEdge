package gateway

import (
	"context"
	"errors"
	"net/http"
	"time"
)

// Server wraps http.Server with conservative production timeouts and graceful shutdown support.
type Server struct {
	httpServer *http.Server
}

// NewServer constructs a Server listening on listenAddr with the provided handler.
func NewServer(listenAddr string, handler http.Handler) *Server {
	return &Server{
		httpServer: &http.Server{
			Addr:              listenAddr,
			Handler:           handler,
			ReadHeaderTimeout: 5 * time.Second,
			ReadTimeout:       30 * time.Second,
			WriteTimeout:      30 * time.Second,
			IdleTimeout:       60 * time.Second,
		},
	}
}

// Start begins listening for incoming HTTP connections.
// It returns nil if the server was gracefully closed via Shutdown.
func (s *Server) Start() error {
	err := s.httpServer.ListenAndServe()
	if errors.Is(err, http.ErrServerClosed) {
		return nil
	}
	return err
}

// Shutdown gracefully stops the server, waiting for active requests to finish
// or until the context deadline is reached.
func (s *Server) Shutdown(ctx context.Context) error {
	return s.httpServer.Shutdown(ctx)
}
