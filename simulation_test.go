package simulation_test

// Tests cover S-26, S-27, S-28, R-16, R-17, R-18 from the test plans.

import (
	"sync"
	"testing"
	"time"

	"miniedge/internal/simulation"
)

var allowedMap = map[string]bool{"catalog": true, "orders": true}

// ──────────────────────────────────────────────────────────
// Activation validation (S-26, S-27)
// ──────────────────────────────────────────────────────────

func TestActivate_DisabledByDefault(t *testing.T) {
	// Simulating S-26: simulation must be enabled in config before Activate is called.
	// The State itself doesn't know about the config flag; that's enforced in admin.go.
	// Here we verify that Activate with an unknown service is rejected.
	s := simulation.NewState()
	err := s.Activate("unknown-svc", simulation.ModeDelay, 100, 10, 2000, 60, allowedMap)
	if err == nil {
		t.Error("Activate for unlisted service should be rejected")
	}
}

func TestActivate_InvalidMode(t *testing.T) {
	s := simulation.NewState()
	err := s.Activate("catalog", "crash", 100, 10, 2000, 60, allowedMap)
	if err == nil {
		t.Error("Activate with invalid mode should be rejected")
	}
}

func TestActivate_ExceedsMaxDelay(t *testing.T) {
	s := simulation.NewState()
	err := s.Activate("catalog", simulation.ModeDelay, 3000, 10, 2000, 60, allowedMap)
	if err == nil {
		t.Error("Activate with delay > maxDelayMs should be rejected")
	}
}

func TestActivate_NegativeDelay(t *testing.T) {
	s := simulation.NewState()
	err := s.Activate("catalog", simulation.ModeDelay, -1, 10, 2000, 60, allowedMap)
	if err == nil {
		t.Error("Activate with negative delay should be rejected")
	}
}

func TestActivate_ExceedsMaxDuration(t *testing.T) {
	s := simulation.NewState()
	err := s.Activate("catalog", simulation.ModeDelay, 100, 999, 2000, 60, allowedMap)
	if err == nil {
		t.Error("Activate with duration > maxDurationSec should be rejected")
	}
}

func TestActivate_ZeroDuration(t *testing.T) {
	s := simulation.NewState()
	err := s.Activate("catalog", simulation.ModeDelay, 100, 0, 2000, 60, allowedMap)
	if err == nil {
		t.Error("Activate with zero duration should be rejected")
	}
}

// ──────────────────────────────────────────────────────────
// Successful activation and Get (R-16)
// ──────────────────────────────────────────────────────────

func TestActivate_Success(t *testing.T) {
	s := simulation.NewState()
	err := s.Activate("catalog", simulation.ModeDelay, 500, 5, 2000, 60, allowedMap)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	sim := s.Get("catalog")
	if sim == nil {
		t.Fatal("expected active simulation, got nil")
	}
	if sim.Mode != simulation.ModeDelay {
		t.Errorf("expected mode delay, got %s", sim.Mode)
	}
	if sim.DelayMs != 500 {
		t.Errorf("expected delayMs 500, got %d", sim.DelayMs)
	}
}

func TestGet_NoSimulation(t *testing.T) {
	s := simulation.NewState()
	if sim := s.Get("catalog"); sim != nil {
		t.Error("expected nil for service with no simulation")
	}
}

// ──────────────────────────────────────────────────────────
// Deactivation / restore (S-28)
// ──────────────────────────────────────────────────────────

func TestDeactivate(t *testing.T) {
	s := simulation.NewState()
	_ = s.Activate("catalog", simulation.ModeDelay, 100, 5, 2000, 60, allowedMap)
	s.Deactivate("catalog")
	if s.Get("catalog") != nil {
		t.Error("expected nil after deactivation")
	}
}

// ──────────────────────────────────────────────────────────
// Automatic expiry (S-28, R-17)
// ──────────────────────────────────────────────────────────

func TestExpiry(t *testing.T) {
	s := simulation.NewState()
	// Use 1-second duration.
	_ = s.Activate("catalog", simulation.ModeDelay, 100, 1, 2000, 60, allowedMap)
	if s.Get("catalog") == nil {
		t.Fatal("simulation should be active before expiry")
	}
	time.Sleep(1100 * time.Millisecond)
	if s.Get("catalog") != nil {
		t.Error("simulation should have expired")
	}
}

// ──────────────────────────────────────────────────────────
// Sweep (REL-05)
// ──────────────────────────────────────────────────────────

func TestSweep(t *testing.T) {
	s := simulation.NewState()
	_ = s.Activate("catalog", simulation.ModeDelay, 100, 1, 2000, 60, allowedMap)
	time.Sleep(1100 * time.Millisecond)
	s.Sweep()
	active := s.Active()
	if len(active) != 0 {
		t.Errorf("expected 0 active simulations after sweep, got %d", len(active))
	}
}

// ──────────────────────────────────────────────────────────
// Concurrent access (REL-04)
// ──────────────────────────────────────────────────────────

func TestConcurrentAccess(t *testing.T) {
	s := simulation.NewState()
	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = s.Activate("catalog", simulation.ModeDelay, 100, 5, 2000, 60, allowedMap)
			_ = s.Get("catalog")
			s.Deactivate("catalog")
		}()
	}
	wg.Wait()
	// No panic = pass
}

// ──────────────────────────────────────────────────────────
// Error mode (R-16)
// ──────────────────────────────────────────────────────────

func TestErrorMode(t *testing.T) {
	s := simulation.NewState()
	_ = s.Activate("orders", simulation.ModeError, 0, 5, 2000, 60, allowedMap)
	sim := s.Get("orders")
	if sim == nil {
		t.Fatal("expected active simulation")
	}
	if sim.Mode != simulation.ModeError {
		t.Errorf("expected error mode, got %s", sim.Mode)
	}
}
