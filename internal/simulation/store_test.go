package simulation_test

import (
	"sync"
	"testing"
	"time"

	"miniedge/internal/model"
	"miniedge/internal/simulation"
)

func TestSimulationStoreDefault(t *testing.T) {
	services := []model.Service{
		{ID: "users", Name: "User Service"},
		{ID: "orders", Name: "Order Service"},
	}

	store := simulation.NewSimulationStore(services)

	u, ok := store.Get("users")
	if !ok || u.Mode != model.SimulationNormal || u.Delay != 0 {
		t.Errorf("expected NORMAL state for users, got %+v", u)
	}

	all := store.GetAll()
	if len(all) != 2 {
		t.Errorf("expected 2 services in GetAll, got %d", len(all))
	}
}

func TestSimulationStoreSetFailAndDelay(t *testing.T) {
	services := []model.Service{{ID: "users"}}
	store := simulation.NewSimulationStore(services)

	// Set FAIL mode
	err := store.Set("users", model.SimulationFail, 0)
	if err != nil {
		t.Fatalf("unexpected error setting FAIL mode: %v", err)
	}
	u, _ := store.Get("users")
	if u.Mode != model.SimulationFail || u.Delay != 0 {
		t.Errorf("expected FAIL mode, got %+v", u)
	}

	// Set DELAY mode
	err = store.Set("users", model.SimulationDelay, 500*time.Millisecond)
	if err != nil {
		t.Fatalf("unexpected error setting DELAY mode: %v", err)
	}
	u, _ = store.Get("users")
	if u.Mode != model.SimulationDelay || u.Delay != 500*time.Millisecond {
		t.Errorf("expected DELAY mode 500ms, got %+v", u)
	}

	// Reset to NORMAL
	store.ResetAll()
	u, _ = store.Get("users")
	if u.Mode != model.SimulationNormal || u.Delay != 0 {
		t.Errorf("expected NORMAL mode after reset, got %+v", u)
	}
}

func TestSimulationStoreUnknownServiceAndValidation(t *testing.T) {
	services := []model.Service{{ID: "users"}}
	store := simulation.NewSimulationStore(services)

	// Unknown service Set
	err := store.Set("nonexistent", model.SimulationFail, 0)
	if err == nil {
		t.Error("expected error for unknown service Set")
	}

	// Unknown service Get
	_, ok := store.Get("nonexistent")
	if ok {
		t.Error("expected ok=false for unknown service Get")
	}

	// Invalid mode
	err = store.Set("users", model.SimulationMode("INVALID"), 0)
	if err == nil {
		t.Error("expected error for invalid mode")
	}

	// Invalid delay (negative or excessive)
	err = store.Set("users", model.SimulationDelay, -10*time.Millisecond)
	if err == nil {
		t.Error("expected error for negative delay")
	}
	err = store.Set("users", model.SimulationDelay, 35*time.Second)
	if err == nil {
		t.Error("expected error for delay > 30s")
	}
	err = store.Set("users", model.SimulationDelay, 0)
	if err == nil {
		t.Error("expected error for DELAY mode with 0 delay")
	}
}

func TestSimulationStoreDetachedSnapshot(t *testing.T) {
	services := []model.Service{{ID: "users"}}
	store := simulation.NewSimulationStore(services)

	snapshot1 := store.GetAll()
	snapshot1["users"] = model.SimulationState{ServiceID: "users", Mode: model.SimulationFail}

	u, _ := store.Get("users")
	if u.Mode != model.SimulationNormal {
		t.Errorf("store internal state was mutated via snapshot! got %+v", u)
	}
}

func TestSimulationStoreConcurrentReadsWrites(t *testing.T) {
	services := []model.Service{{ID: "users"}}
	store := simulation.NewSimulationStore(services)

	var wg sync.WaitGroup
	done := make(chan struct{})

	// Writer
	wg.Add(1)
	go func() {
		defer wg.Done()
		modes := []model.SimulationMode{model.SimulationNormal, model.SimulationFail, model.SimulationDelay}
		idx := 0
		for {
			select {
			case <-done:
				return
			default:
				m := modes[idx%3]
				d := time.Duration(idx+1) * 10 * time.Millisecond
				_ = store.Set("users", m, d)
				idx++
			}
		}
	}()

	// Readers
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-done:
					return
				default:
					_, _ = store.Get("users")
					_ = store.GetAll()
				}
			}
		}()
	}

	time.Sleep(100 * time.Millisecond)
	close(done)
	wg.Wait()
}
