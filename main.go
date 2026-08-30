// MiniEdge — local developer gateway and microservices command center.
//
// Zero third-party runtime dependencies: uses Go standard library only.
//
// Usage:
//
//	miniedge -config <path/to/config.json> [-admin-token <token>]
//
// The process exits with code 1 on invalid configuration or fatal startup error.
// It handles SIGTERM and SIGINT for graceful shutdown.
package main

import (
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"miniedge/internal/config"
	"miniedge/internal/logger"
	"miniedge/internal/server"
)

func main() {
	configPath := flag.String("config", "miniedge.json", "path to JSON configuration file")
	adminToken := flag.String("admin-token", "", "admin API token (empty = disabled for local dev)")
	flag.Parse()

	log := logger.New()

	// Load and validate configuration. Fail closed on any error (SEC-10, D-04).
	cfg, err := config.LoadFile(*configPath)
	if err != nil {
		log.Error("startup_config_invalid", logger.F("path", *configPath), logger.F("error", err.Error()))
		fmt.Fprintf(os.Stderr, "fatal: invalid configuration: %v\n", err)
		os.Exit(1)
	}

	cfgStore := config.NewStore(cfg)

	// Start the server.
	srv := server.New(cfgStore, log, *adminToken)
	errCh := srv.Start()

	// Wait for a fatal server error or a shutdown signal.
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGTERM, syscall.SIGINT)

	select {
	case err := <-errCh:
		log.Error("server_fatal", logger.F("error", err.Error()))
		fmt.Fprintf(os.Stderr, "fatal: %v\n", err)
		// Best-effort shutdown even on fatal error.
		srv.Shutdown(cfg.Timeouts.ShutdownDrainMs)
		os.Exit(1)
	case sig := <-sigCh:
		log.Info("shutdown_signal", logger.F("signal", sig.String()))
	}

	// Graceful shutdown (REL-06).
	srv.Shutdown(cfg.Timeouts.ShutdownDrainMs)
	os.Exit(0)
}
