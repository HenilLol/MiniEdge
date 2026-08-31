package main

import (
	"context"
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"miniedge/internal/api"
	"miniedge/internal/config"
	"miniedge/internal/gateway"
	"miniedge/internal/health"
	"miniedge/internal/observability"
	"miniedge/internal/proxy"
	"miniedge/internal/ratelimit"
	"miniedge/internal/router"
	"miniedge/internal/simulation"
)

func main() {
	var configPath string
	flag.StringVar(&configPath, "config", "config.json", "Path to JSON configuration file")
	flag.Parse()

	f, err := os.Open(configPath)
	if err != nil && configPath == "config.json" {
		if fExample, errExample := os.Open("config.example.json"); errExample == nil {
			f = fExample
			configPath = "config.example.json"
			err = nil
		}
	}
	if err != nil {
		log.Fatalf("Failed to open configuration file '%s': %v", configPath, err)
	}
	defer f.Close()

	cfg, err := config.LoadConfig(f)
	if err != nil {
		log.Fatalf("Failed to load/validate configuration from '%s': %v", configPath, err)
	}
	log.Printf("Loaded configuration from '%s' (ListenAddr: %s, Services: %d, Routes: %d)",
		configPath, cfg.ListenAddr, len(cfg.Services), len(cfg.Routes))

	// 1. Router & Service Registry
	reg := router.NewStaticServiceRegistry(cfg.Services)
	r := router.NewPrefixRouter(cfg.Routes)

	// 2. Observability Store
	obsStore := observability.NewStore(observability.DefaultCapacity)

	// 3. Health Monitoring (Store, Checker, Worker)
	healthStore := health.NewHealthStore(cfg.Services)
	healthChecker := health.NewChecker(2*time.Second, 500*time.Millisecond)
	healthWorker := health.NewWorker(cfg.Services, healthStore, healthChecker, 5*time.Second)

	// 4. Failure Simulation & Rate Limiting Stores
	simStore := simulation.NewSimulationStore(cfg.Services)
	rlStore := ratelimit.NewRateLimiterStore(cfg.Services)

	// 5. Service Proxy
	px := proxy.NewServiceProxy(10 * time.Second)

	// 6. Gateway Handler Wiring
	gwHandler := gateway.NewGatewayHandler(r, reg, px, obsStore, 10*time.Second)
	gwHandler.SetSimulationController(simStore)
	gwHandler.SetRateLimiter(rlStore)

	// 7. Control API Handler Wiring
	apiHandler := api.NewHandler(obsStore, healthStore)
	apiHandler.SetSimulationStore(simStore)
	apiHandler.SetRateLimiterStore(rlStore)

	if apiKey := os.Getenv("MINIEDGE_API_KEY"); apiKey != "" {
		apiHandler.SetAPIKey(apiKey)
		log.Println("Configured administrative API key authentication for control endpoints")
	} else {
		log.Println("MINIEDGE_API_KEY is not set; administrative POST endpoints will reject requests")
	}

	if allowedOrigin := os.Getenv("MINIEDGE_ALLOWED_ORIGIN"); allowedOrigin != "" {
		apiHandler.SetAllowedOrigin(allowedOrigin)
		log.Printf("Configured CORS Access-Control-Allow-Origin: %s", allowedOrigin)
	}

	gwHandler.SetAPIHandler(apiHandler)

	listenAddr := cfg.ListenAddr
	if envPort := os.Getenv("PORT"); envPort != "" {
		listenAddr = "0.0.0.0:" + envPort
	}

	// 8. Server Construction
	server := gateway.NewServer(listenAddr, gwHandler)

	// 9. Start Health Worker
	healthWorker.Start()

	// 10. Start Server in background goroutine
	serverErr := make(chan error, 1)
	go func() {
		log.Printf("MiniEdge gateway server listening on %s", listenAddr)
		if err := server.Start(); err != nil {
			serverErr <- err
		}
	}()

	// 11. Listen for termination signals
	stopSignal := make(chan os.Signal, 1)
	signal.Notify(stopSignal, os.Interrupt, syscall.SIGTERM)

	select {
	case err := <-serverErr:
		log.Printf("Server failed to start: %v", err)
	case sig := <-stopSignal:
		log.Printf("Received signal (%s), initiating graceful shutdown...", sig)
	}

	// 12. Graceful Shutdown
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Printf("Error during server shutdown: %v", err)
	}

	healthWorker.Stop()
	log.Println("MiniEdge backend stopped cleanly.")
}
