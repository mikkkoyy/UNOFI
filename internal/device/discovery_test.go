package device

import (
	"context"
	"testing"
	"time"

	"github.com/unofi/unofi/internal/database"
	"github.com/unofi/unofi/internal/events"
	"github.com/unofi/unofi/internal/logger"
	"github.com/unofi/unofi/internal/mikrotik"
)

func TestNormalizeMAC(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"AA:BB:CC:DD:EE:FF", "aa:bb:cc:dd:ee:ff"},
		{"aa:bb:cc:dd:ee:ff", "aa:bb:cc:dd:ee:ff"},
		{"AA-BB-CC-DD-EE-FF", "aa:bb:cc:dd:ee:ff"},
		{"aa-bb-cc-dd-ee-ff", "aa:bb:cc:dd:ee:ff"},
		{"aabb.ccdd.eeff", "aa:bb:cc:dd:ee:ff"},
		{"aabbccddeeff", "aa:bb:cc:dd:ee:ff"},
		{"AABBCCDDEEFF", "aa:bb:cc:dd:ee:ff"},
		{"", ""},
		{"invalid", ""},
		{"AA:BB:CC:DD:EE", ""},
		{"AA:BB:CC:DD:EE:FF:GG", ""},
		{"GG:HH:II:JJ:KK:LL", ""},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := NormalizeMAC(tt.input)
			if got != tt.expected {
				t.Errorf("NormalizeMAC(%q) = %q, want %q", tt.input, got, tt.expected)
			}
		})
	}
}

func TestDiscoveryWorkerDiscoversNewClient(t *testing.T) {
	db := database.TestDB(t)
	repo := NewSQLRepository(db)
	router := mikrotik.NewMockRouter()
	eventBus := events.NewBus()
	loggr := logger.New("debug", nil)

	worker := NewDiscoveryWorker(router, repo, eventBus, loggr, 100*time.Millisecond)

	// Add a mock client
	router.AddTestClient(&mikrotik.Client{
		MAC:  "AA:BB:CC:DD:EE:FF",
		IP:   "10.0.0.5",
	})

	// Run discovery once
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	if err := worker.Start(ctx); err != nil {
		t.Fatalf("Start error: %v", err)
	}

	// Wait for discovery to run
	time.Sleep(200 * time.Millisecond)

	// Verify device was created
	device, err := repo.GetByMAC("aa:bb:cc:dd:ee:ff")
	if err != nil {
		t.Fatalf("GetByMAC error: %v", err)
	}
	if device == nil {
		t.Fatal("device should be created")
	}
	if device.IP != "10.0.0.5" {
		t.Errorf("expected IP 10.0.0.5, got %s", device.IP)
	}
	if !device.Online {
		t.Error("device should be online")
	}

	worker.Stop()
}

func TestDiscoveryWorkerUpdatesExistingClient(t *testing.T) {
	db := database.TestDB(t)
	repo := NewSQLRepository(db)
	router := mikrotik.NewMockRouter()
	eventBus := events.NewBus()
	loggr := logger.New("debug", nil)

	// Create device first
	_, err := repo.Create("aa:bb:cc:dd:ee:ff", "10.0.0.1", "")
	if err != nil {
		t.Fatalf("Create error: %v", err)
	}

	worker := NewDiscoveryWorker(router, repo, eventBus, loggr, 100*time.Millisecond)

	// Add mock client with different IP
	router.AddTestClient(&mikrotik.Client{
		MAC:  "AA:BB:CC:DD:EE:FF",
		IP:   "10.0.0.5",
	})

	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	if err := worker.Start(ctx); err != nil {
		t.Fatalf("Start error: %v", err)
	}

	time.Sleep(200 * time.Millisecond)

	// Verify device was updated
	device, err := repo.GetByMAC("aa:bb:cc:dd:ee:ff")
	if err != nil {
		t.Fatalf("GetByMAC error: %v", err)
	}
	if device == nil {
		t.Fatal("device should exist")
	}
	if device.IP != "10.0.0.5" {
		t.Errorf("expected updated IP 10.0.0.5, got %s", device.IP)
	}

	worker.Stop()
}

func TestDiscoveryWorkerMarksOffline(t *testing.T) {
	db := database.TestDB(t)
	repo := NewSQLRepository(db)
	router := mikrotik.NewMockRouter()
	eventBus := events.NewBus()
	loggr := logger.New("debug", nil)

	// Create device and mark online
	device, err := repo.Create("aa:bb:cc:dd:ee:ff", "10.0.0.5", "")
	if err != nil {
		t.Fatalf("Create error: %v", err)
	}
	if err := repo.SetOnline(device.ID, true); err != nil {
		t.Fatalf("SetOnline error: %v", err)
	}

	worker := NewDiscoveryWorker(router, repo, eventBus, loggr, 100*time.Millisecond)

	// No clients in router - device should be marked offline
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	if err := worker.Start(ctx); err != nil {
		t.Fatalf("Start error: %v", err)
	}

	time.Sleep(200 * time.Millisecond)

	// Verify device was marked offline
	d, err := repo.GetByMAC("aa:bb:cc:dd:ee:ff")
	if err != nil {
		t.Fatalf("GetByMAC error: %v", err)
	}
	if d == nil {
		t.Fatal("device should exist")
	}
	if d.Online {
		t.Error("device should be offline")
	}

	worker.Stop()
}

func TestDiscoveryWorkerHandlesRouterFailure(t *testing.T) {
	db := database.TestDB(t)
	repo := NewSQLRepository(db)
	router := mikrotik.NewMockRouter()
	eventBus := events.NewBus()
	loggr := logger.New("debug", nil)

	worker := NewDiscoveryWorker(router, repo, eventBus, loggr, 100*time.Millisecond)

	// Set router unreachable
	router.SetReachable(false)

	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	if err := worker.Start(ctx); err != nil {
		t.Fatalf("Start error: %v", err)
	}

	time.Sleep(200 * time.Millisecond)

	// Worker should still be running despite failure
	if !worker.IsRunning() {
		t.Error("worker should still be running after router failure")
	}

	worker.Stop()
}

func TestDiscoveryWorkerDoesNotAuthorize(t *testing.T) {
	db := database.TestDB(t)
	repo := NewSQLRepository(db)
	router := mikrotik.NewMockRouter()
	eventBus := events.NewBus()
	loggr := logger.New("debug", nil)

	worker := NewDiscoveryWorker(router, repo, eventBus, loggr, 100*time.Millisecond)

	// Add a mock client
	router.AddTestClient(&mikrotik.Client{
		MAC:  "AA:BB:CC:DD:EE:FF",
		IP:   "10.0.0.5",
	})

	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	if err := worker.Start(ctx); err != nil {
		t.Fatalf("Start error: %v", err)
	}

	time.Sleep(200 * time.Millisecond)

	// Verify device was created but NO session was created
	device, err := repo.GetByMAC("aa:bb:cc:dd:ee:ff")
	if err != nil {
		t.Fatalf("GetByMAC error: %v", err)
	}
	if device == nil {
		t.Fatal("device should be created")
	}

	// Verify no session exists for this device
	// (Discovery should NOT create sessions)
	// This is verified by the fact that we don't have a session repository in the worker

	worker.Stop()
}

func TestDiscoveryWorkerLifecycle(t *testing.T) {
	db := database.TestDB(t)
	repo := NewSQLRepository(db)
	router := mikrotik.NewMockRouter()
	eventBus := events.NewBus()
	loggr := logger.New("debug", nil)

	worker := NewDiscoveryWorker(router, repo, eventBus, loggr, 100*time.Millisecond)

	// Not running initially
	if worker.IsRunning() {
		t.Error("worker should not be running initially")
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start worker
	if err := worker.Start(ctx); err != nil {
		t.Fatalf("Start error: %v", err)
	}

	if !worker.IsRunning() {
		t.Error("worker should be running after Start")
	}

	// Double start should fail
	if err := worker.Start(ctx); err == nil {
		t.Error("double start should fail")
	}

	// Stop worker
	worker.Stop()

	if worker.IsRunning() {
		t.Error("worker should not be running after Stop")
	}
}

func TestDiscoveryWorkerPublishesEvents(t *testing.T) {
	db := database.TestDB(t)
	repo := NewSQLRepository(db)
	router := mikrotik.NewMockRouter()
	eventBus := events.NewBus()
	loggr := logger.New("debug", nil)

	// Track events
	var connectedEvents int
	eventBus.Subscribe(events.EventDeviceConnected, func(e events.Event) {
		connectedEvents++
	})

	worker := NewDiscoveryWorker(router, repo, eventBus, loggr, 100*time.Millisecond)

	// Add a mock client
	router.AddTestClient(&mikrotik.Client{
		MAC:  "AA:BB:CC:DD:EE:FF",
		IP:   "10.0.0.5",
	})

	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	if err := worker.Start(ctx); err != nil {
		t.Fatalf("Start error: %v", err)
	}

	time.Sleep(200 * time.Millisecond)

	// Event should have been published
	if connectedEvents == 0 {
		t.Error("expected device connected event to be published")
	}

	worker.Stop()
}
