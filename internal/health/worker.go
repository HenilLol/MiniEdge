package health

import (
	"context"
	"sync"
	"time"

	"miniedge/internal/model"
)

// Worker runs periodic background health checks for configured services.
type Worker struct {
	services []model.Service
	store    *HealthStore
	checker  *Checker
	interval time.Duration

	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

// NewWorker initializes a Worker instance with given services, store, checker, and tick interval.
func NewWorker(services []model.Service, store *HealthStore, checker *Checker, interval time.Duration) *Worker {
	if interval <= 0 {
		interval = 5 * time.Second
	}
	ctx, cancel := context.WithCancel(context.Background())
	return &Worker{
		services: services,
		store:    store,
		checker:  checker,
		interval: interval,
		ctx:      ctx,
		cancel:   cancel,
	}
}

// Start executes an immediate initial health check for all services and begins the periodic ticker goroutine.
func (w *Worker) Start() {
	if len(w.services) == 0 {
		return
	}

	// Immediate initial health checks
	w.checkAllServices()

	// Launch single background worker loop
	w.wg.Add(1)
	go func() {
		defer w.wg.Done()
		ticker := time.NewTicker(w.interval)
		defer ticker.Stop()

		for {
			select {
			case <-w.ctx.Done():
				return
			case <-ticker.C:
				w.checkAllServices()
			}
		}
	}()
}

// checkAllServices probes each service independently with per-service isolation.
func (w *Worker) checkAllServices() {
	for _, svc := range w.services {
		if w.ctx.Err() != nil {
			return
		}
		res := w.checker.Check(w.ctx, svc)
		w.store.Update(res.ServiceID, res.Status, res.CheckedAt, res.Latency)
	}
}

// Stop stops the background ticker loop and waits for the worker goroutine to exit.
func (w *Worker) Stop() {
	w.cancel()
	w.wg.Wait()
}
