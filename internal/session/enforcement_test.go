package session

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/unofi/unofi/internal/database"
	"github.com/unofi/unofi/internal/device"
	"github.com/unofi/unofi/internal/events"
	"github.com/unofi/unofi/internal/logger"
	"github.com/unofi/unofi/internal/mikrotik"
)

// mockDeviceRepo implements session.DeviceGetter for testing.
type mockDeviceRepo struct {
	devices map[int64]*device.Device
}

func newMockDeviceRepo() *mockDeviceRepo {
	return &mockDeviceRepo{
		devices: make(map[int64]*device.Device),
	}
}

func (r *mockDeviceRepo) GetByID(id int64) (*device.Device, error) {
	return r.devices[id], nil
}

func (r *mockDeviceRepo) GetByMAC(mac string) (*device.Device, error) {
	for _, d := range r.devices {
		if d.MAC == mac {
			return d, nil
		}
	}
	return nil, nil
}

func (r *mockDeviceRepo) addDevice(d *device.Device) {
	r.devices[d.ID] = d
}

func TestEnforcementWorkerLifecycle(t *testing.T) {
	db := database.TestDB(t)
	sessionRepo := NewSQLRepository(db)
	router := mikrotik.NewMockRouter()
	deviceRepo := newMockDeviceRepo()
	eventBus := events.NewBus()
	loggr := logger.New("debug", nil)

	worker := NewEnforcementWorker(router, sessionRepo, deviceRepo, nil, eventBus, loggr, 100*time.Millisecond)

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

func TestEnforcementWorkerValidSession(t *testing.T) {
	db := database.TestDB(t)
	sessionRepo := NewSQLRepository(db)
	router := mikrotik.NewMockRouter()
	deviceRepo := newMockDeviceRepo()
	eventBus := events.NewBus()
	loggr := logger.New("debug", nil)

	// Create device
	deviceID, err := db.Exec(`INSERT INTO devices(mac_addr, ip_addr, hostname) VALUES(?, ?, ?)`,
		"aa:bb:cc:dd:ee:ff", "10.0.0.5", "test-device")
	if err != nil {
		t.Fatalf("insert device: %v", err)
	}
	devID, _ := deviceID.LastInsertId()

	deviceRepo.addDevice(&device.Device{
		ID:       devID,
		MAC:      "aa:bb:cc:dd:ee:ff",
		IP:       "10.0.0.5",
		Hostname: "test-device",
	})

	// Create active session with data remaining
	session, err := sessionRepo.Create(&Session{
		DeviceID:  devID,
		MBLimit:   100,
		MBUsed:    50,
		Status:    SessionActive,
		ExpiresAt: time.Now().Add(1 * time.Hour),
	})
	if err != nil {
		t.Fatalf("create session: %v", err)
	}

	// Add client to mock router
	router.AddTestClient(&mikrotik.Client{
		MAC:  "aa:bb:cc:dd:ee:ff",
		IP:   "10.0.0.5",
	})

	worker := NewEnforcementWorker(router, sessionRepo, deviceRepo, nil, eventBus, loggr, 100*time.Millisecond)

	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	if err := worker.Start(ctx); err != nil {
		t.Fatalf("Start error: %v", err)
	}

	time.Sleep(200 * time.Millisecond)

	// Verify session is still active
	sess, err := sessionRepo.GetByID(session.ID)
	if err != nil {
		t.Fatalf("get session: %v", err)
	}
	if sess == nil {
		t.Fatal("session should exist")
	}
	if sess.Status != SessionActive {
		t.Errorf("expected status %s, got %s", SessionActive, sess.Status)
	}

	worker.Stop()
}

func TestEnforcementWorkerExpiredSession(t *testing.T) {
	db := database.TestDB(t)
	sessionRepo := NewSQLRepository(db)
	router := mikrotik.NewMockRouter()
	deviceRepo := newMockDeviceRepo()
	eventBus := events.NewBus()
	loggr := logger.New("debug", nil)

	// Create device
	deviceID, err := db.Exec(`INSERT INTO devices(mac_addr, ip_addr, hostname) VALUES(?, ?, ?)`,
		"aa:bb:cc:dd:ee:ff", "10.0.0.5", "test-device")
	if err != nil {
		t.Fatalf("insert device: %v", err)
	}
	devID, _ := deviceID.LastInsertId()

	deviceRepo.addDevice(&device.Device{
		ID:       devID,
		MAC:      "aa:bb:cc:dd:ee:ff",
		IP:       "10.0.0.5",
		Hostname: "test-device",
	})

	// Create expired session
	session, err := sessionRepo.Create(&Session{
		DeviceID:  devID,
		MBLimit:   100,
		MBUsed:    50,
		Status:    SessionActive,
		ExpiresAt: time.Now().Add(-1 * time.Hour), // expired
	})
	if err != nil {
		t.Fatalf("create session: %v", err)
	}

	worker := NewEnforcementWorker(router, sessionRepo, deviceRepo, nil, eventBus, loggr, 100*time.Millisecond)

	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	if err := worker.Start(ctx); err != nil {
		t.Fatalf("Start error: %v", err)
	}

	time.Sleep(200 * time.Millisecond)

	// Verify session is now expired
	sess, err := sessionRepo.GetByID(session.ID)
	if err != nil {
		t.Fatalf("get session: %v", err)
	}
	if sess == nil {
		t.Fatal("session should exist")
	}
	if sess.Status != SessionExpired {
		t.Errorf("expected status %s, got %s", SessionExpired, sess.Status)
	}

	worker.Stop()
}

func TestEnforcementWorkerDataExhausted(t *testing.T) {
	db := database.TestDB(t)
	sessionRepo := NewSQLRepository(db)
	router := mikrotik.NewMockRouter()
	deviceRepo := newMockDeviceRepo()
	eventBus := events.NewBus()
	loggr := logger.New("debug", nil)

	// Create device
	deviceID, err := db.Exec(`INSERT INTO devices(mac_addr, ip_addr, hostname) VALUES(?, ?, ?)`,
		"aa:bb:cc:dd:ee:ff", "10.0.0.5", "test-device")
	if err != nil {
		t.Fatalf("insert device: %v", err)
	}
	devID, _ := deviceID.LastInsertId()

	deviceRepo.addDevice(&device.Device{
		ID:       devID,
		MAC:      "aa:bb:cc:dd:ee:ff",
		IP:       "10.0.0.5",
		Hostname: "test-device",
	})

	// Create session that's exhausted (MBUsed >= MBLimit)
	session, err := sessionRepo.Create(&Session{
		DeviceID:  devID,
		MBLimit:   100,
		MBUsed:    100, // exhausted
		Status:    SessionActive,
		ExpiresAt: time.Now().Add(1 * time.Hour),
	})
	if err != nil {
		t.Fatalf("create session: %v", err)
	}

	worker := NewEnforcementWorker(router, sessionRepo, deviceRepo, nil, eventBus, loggr, 100*time.Millisecond)

	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	if err := worker.Start(ctx); err != nil {
		t.Fatalf("Start error: %v", err)
	}

	time.Sleep(200 * time.Millisecond)

	// Verify session is now exhausted
	sess, err := sessionRepo.GetByID(session.ID)
	if err != nil {
		t.Fatalf("get session: %v", err)
	}
	if sess == nil {
		t.Fatal("session should exist")
	}
	if sess.Status != SessionExhausted {
		t.Errorf("expected status %s, got %s", SessionExhausted, sess.Status)
	}

	worker.Stop()
}

func TestEnforcementWorkerCancelledSession(t *testing.T) {
	db := database.TestDB(t)
	sessionRepo := NewSQLRepository(db)
	router := mikrotik.NewMockRouter()
	deviceRepo := newMockDeviceRepo()
	eventBus := events.NewBus()
	loggr := logger.New("debug", nil)

	// Create device
	deviceID, err := db.Exec(`INSERT INTO devices(mac_addr, ip_addr, hostname) VALUES(?, ?, ?)`,
		"aa:bb:cc:dd:ee:ff", "10.0.0.5", "test-device")
	if err != nil {
		t.Fatalf("insert device: %v", err)
	}
	devID, _ := deviceID.LastInsertId()

	deviceRepo.addDevice(&device.Device{
		ID:       devID,
		MAC:      "aa:bb:cc:dd:ee:ff",
		IP:       "10.0.0.5",
		Hostname: "test-device",
	})

	// Create cancelled session
	session, err := sessionRepo.Create(&Session{
		DeviceID:  devID,
		MBLimit:   100,
		MBUsed:    50,
		Status:    SessionCancelled,
		ExpiresAt: time.Now().Add(1 * time.Hour),
	})
	if err != nil {
		t.Fatalf("create session: %v", err)
	}

	worker := NewEnforcementWorker(router, sessionRepo, deviceRepo, nil, eventBus, loggr, 100*time.Millisecond)

	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	if err := worker.Start(ctx); err != nil {
		t.Fatalf("Start error: %v", err)
	}

	time.Sleep(200 * time.Millisecond)

	// Verify session status is still cancelled
	sess, err := sessionRepo.GetByID(session.ID)
	if err != nil {
		t.Fatalf("get session: %v", err)
	}
	if sess == nil {
		t.Fatal("session should exist")
	}
	if sess.Status != SessionCancelled {
		t.Errorf("expected status %s, got %s", SessionCancelled, sess.Status)
	}

	worker.Stop()
}

func TestEnforcementWorkerUsageSynchronization(t *testing.T) {
	db := database.TestDB(t)
	sessionRepo := NewSQLRepository(db)
	router := mikrotik.NewMockRouter()
	deviceRepo := newMockDeviceRepo()
	eventBus := events.NewBus()
	loggr := logger.New("debug", nil)

	// Create device
	deviceID, err := db.Exec(`INSERT INTO devices(mac_addr, ip_addr, hostname) VALUES(?, ?, ?)`,
		"aa:bb:cc:dd:ee:ff", "10.0.0.5", "test-device")
	if err != nil {
		t.Fatalf("insert device: %v", err)
	}
	devID, _ := deviceID.LastInsertId()

	deviceRepo.addDevice(&device.Device{
		ID:       devID,
		MAC:      "aa:bb:cc:dd:ee:ff",
		IP:       "10.0.0.5",
		Hostname: "test-device",
	})

	// Create active session
	session, err := sessionRepo.Create(&Session{
		DeviceID:  devID,
		MBLimit:   100,
		MBUsed:    0,
		Status:    SessionActive,
		ExpiresAt: time.Now().Add(1 * time.Hour),
	})
	if err != nil {
		t.Fatalf("create session: %v", err)
	}

	// Add client to mock router with usage
	router.AddTestClient(&mikrotik.Client{
		MAC:      "aa:bb:cc:dd:ee:ff",
		IP:       "10.0.0.5",
		BytesIn:  1024 * 1024 * 10, // 10 MB
		BytesOut: 1024 * 1024 * 5,  // 5 MB
	})

	worker := NewEnforcementWorker(router, sessionRepo, deviceRepo, nil, eventBus, loggr, 100*time.Millisecond)

	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	if err := worker.Start(ctx); err != nil {
		t.Fatalf("Start error: %v", err)
	}

	time.Sleep(200 * time.Millisecond)

	// Verify usage was synchronized
	sess, err := sessionRepo.GetByID(session.ID)
	if err != nil {
		t.Fatalf("get session: %v", err)
	}
	if sess == nil {
		t.Fatal("session should exist")
	}

	// Usage should be approximately 15 MB (10 + 5)
	if sess.MBUsed < 14 || sess.MBUsed > 16 {
		t.Errorf("expected MBUsed ~15, got %f", sess.MBUsed)
	}

	worker.Stop()
}

func TestEnforcementWorkerRouterFailure(t *testing.T) {
	db := database.TestDB(t)
	sessionRepo := NewSQLRepository(db)
	router := mikrotik.NewMockRouter()
	deviceRepo := newMockDeviceRepo()
	eventBus := events.NewBus()
	loggr := logger.New("debug", nil)

	// Create device
	deviceID, err := db.Exec(`INSERT INTO devices(mac_addr, ip_addr, hostname) VALUES(?, ?, ?)`,
		"aa:bb:cc:dd:ee:ff", "10.0.0.5", "test-device")
	if err != nil {
		t.Fatalf("insert device: %v", err)
	}
	devID, _ := deviceID.LastInsertId()

	deviceRepo.addDevice(&device.Device{
		ID:       devID,
		MAC:      "aa:bb:cc:dd:ee:ff",
		IP:       "10.0.0.5",
		Hostname: "test-device",
	})

	// Create active session
	session, err := sessionRepo.Create(&Session{
		DeviceID:  devID,
		MBLimit:   100,
		MBUsed:    50,
		Status:    SessionActive,
		ExpiresAt: time.Now().Add(1 * time.Hour),
	})
	if err != nil {
		t.Fatalf("create session: %v", err)
	}

	// Set router unreachable
	router.SetReachable(false)

	worker := NewEnforcementWorker(router, sessionRepo, deviceRepo, nil, eventBus, loggr, 100*time.Millisecond)

	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	if err := worker.Start(ctx); err != nil {
		t.Fatalf("Start error: %v", err)
	}

	time.Sleep(200 * time.Millisecond)

	// Worker should still be running despite router failure
	if !worker.IsRunning() {
		t.Error("worker should still be running after router failure")
	}

	// Session should NOT be falsely expired
	sess, err := sessionRepo.GetByID(session.ID)
	if err != nil {
		t.Fatalf("get session: %v", err)
	}
	if sess == nil {
		t.Fatal("session should exist")
	}
	if sess.Status != SessionActive {
		t.Errorf("session should still be active, got %s", sess.Status)
	}

	worker.Stop()
}

func TestEnforcementWorkerAuthorizationFailure(t *testing.T) {
	db := database.TestDB(t)
	sessionRepo := NewSQLRepository(db)
	router := mikrotik.NewMockRouter()
	deviceRepo := newMockDeviceRepo()
	eventBus := events.NewBus()
	loggr := logger.New("debug", nil)

	// Create device
	deviceID, err := db.Exec(`INSERT INTO devices(mac_addr, ip_addr, hostname) VALUES(?, ?, ?)`,
		"aa:bb:cc:dd:ee:ff", "10.0.0.5", "test-device")
	if err != nil {
		t.Fatalf("insert device: %v", err)
	}
	devID, _ := deviceID.LastInsertId()

	deviceRepo.addDevice(&device.Device{
		ID:       devID,
		MAC:      "aa:bb:cc:dd:ee:ff",
		IP:       "10.0.0.5",
		Hostname: "test-device",
	})

	// Create valid session
	session, err := sessionRepo.Create(&Session{
		DeviceID:  devID,
		MBLimit:   100,
		MBUsed:    50,
		Status:    SessionActive,
		ExpiresAt: time.Now().Add(1 * time.Hour),
	})
	if err != nil {
		t.Fatalf("create session: %v", err)
	}

	// Set router unreachable (authorization will fail)
	router.SetReachable(false)

	worker := NewEnforcementWorker(router, sessionRepo, deviceRepo, nil, eventBus, loggr, 100*time.Millisecond)

	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	if err := worker.Start(ctx); err != nil {
		t.Fatalf("Start error: %v", err)
	}

	time.Sleep(200 * time.Millisecond)

	// Worker should still be running
	if !worker.IsRunning() {
		t.Error("worker should still be running after authorization failure")
	}

	// Session should still be active (authorization failure doesn't change DB state)
	sess, err := sessionRepo.GetByID(session.ID)
	if err != nil {
		t.Fatalf("get session: %v", err)
	}
	if sess == nil {
		t.Fatal("session should exist")
	}
	if sess.Status != SessionActive {
		t.Errorf("session should still be active, got %s", sess.Status)
	}

	worker.Stop()
}

func TestEnforcementWorkerMultipleSessions(t *testing.T) {
	db := database.TestDB(t)
	sessionRepo := NewSQLRepository(db)
	router := mikrotik.NewMockRouter()
	deviceRepo := newMockDeviceRepo()
	eventBus := events.NewBus()
	loggr := logger.New("debug", nil)

	// Create two devices
	deviceID1, _ := db.Exec(`INSERT INTO devices(mac_addr, ip_addr, hostname) VALUES(?, ?, ?)`,
		"aa:bb:cc:dd:ee:01", "10.0.0.1", "device-1")
	devID1, _ := deviceID1.LastInsertId()
	deviceRepo.addDevice(&device.Device{ID: devID1, MAC: "aa:bb:cc:dd:ee:01", IP: "10.0.0.1"})

	deviceID2, _ := db.Exec(`INSERT INTO devices(mac_addr, ip_addr, hostname) VALUES(?, ?, ?)`,
		"aa:bb:cc:dd:ee:02", "10.0.0.2", "device-2")
	devID2, _ := deviceID2.LastInsertId()
	deviceRepo.addDevice(&device.Device{ID: devID2, MAC: "aa:bb:cc:dd:ee:02", IP: "10.0.0.2"})

	// Create valid session for device 1
	_, err := sessionRepo.Create(&Session{
		DeviceID:  devID1,
		MBLimit:   100,
		MBUsed:    50,
		Status:    SessionActive,
		ExpiresAt: time.Now().Add(1 * time.Hour),
	})
	if err != nil {
		t.Fatalf("create session 1: %v", err)
	}

	// Create expired session for device 2
	_, err = sessionRepo.Create(&Session{
		DeviceID:  devID2,
		MBLimit:   100,
		MBUsed:    50,
		Status:    SessionActive,
		ExpiresAt: time.Now().Add(-1 * time.Hour), // expired
	})
	if err != nil {
		t.Fatalf("create session 2: %v", err)
	}

	worker := NewEnforcementWorker(router, sessionRepo, deviceRepo, nil, eventBus, loggr, 100*time.Millisecond)

	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	if err := worker.Start(ctx); err != nil {
		t.Fatalf("Start error: %v", err)
	}

	time.Sleep(200 * time.Millisecond)

	// Verify session 1 is still active
	sessions1, _ := sessionRepo.ListByDevice(devID1, 10)
	if len(sessions1) != 1 {
		t.Fatalf("expected 1 session for device 1, got %d", len(sessions1))
	}
	if sessions1[0].Status != SessionActive {
		t.Errorf("session 1 should be active, got %s", sessions1[0].Status)
	}

	// Verify session 2 is now expired
	sessions2, _ := sessionRepo.ListByDevice(devID2, 10)
	if len(sessions2) != 1 {
		t.Fatalf("expected 1 session for device 2, got %d", len(sessions2))
	}
	if sessions2[0].Status != SessionExpired {
		t.Errorf("session 2 should be expired, got %s", sessions2[0].Status)
	}

	worker.Stop()
}

func TestEnforcementWorkerNoDevice(t *testing.T) {
	db := database.TestDB(t)
	sessionRepo := NewSQLRepository(db)
	router := mikrotik.NewMockRouter()
	deviceRepo := newMockDeviceRepo()
	eventBus := events.NewBus()
	loggr := logger.New("debug", nil)

	// Create session without a device
	session, err := sessionRepo.Create(&Session{
		DeviceID:  999, // non-existent device
		MBLimit:   100,
		MBUsed:    50,
		Status:    SessionActive,
		ExpiresAt: time.Now().Add(1 * time.Hour),
	})
	if err != nil {
		t.Fatalf("create session: %v", err)
	}

	worker := NewEnforcementWorker(router, sessionRepo, deviceRepo, nil, eventBus, loggr, 100*time.Millisecond)

	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	if err := worker.Start(ctx); err != nil {
		t.Fatalf("Start error: %v", err)
	}

	time.Sleep(200 * time.Millisecond)

	// Session should still be active (no device found, so we skip)
	sess, err := sessionRepo.GetByID(session.ID)
	if err != nil {
		t.Fatalf("get session: %v", err)
	}
	if sess == nil {
		t.Fatal("session should exist")
	}
	if sess.Status != SessionActive {
		t.Errorf("session should still be active, got %s", sess.Status)
	}

	worker.Stop()
}

func TestEnforcementWorkerRepeatedCycles(t *testing.T) {
	db := database.TestDB(t)
	sessionRepo := NewSQLRepository(db)
	router := mikrotik.NewMockRouter()
	deviceRepo := newMockDeviceRepo()
	eventBus := events.NewBus()
	loggr := logger.New("debug", nil)

	// Create device
	deviceID, err := db.Exec(`INSERT INTO devices(mac_addr, ip_addr, hostname) VALUES(?, ?, ?)`,
		"aa:bb:cc:dd:ee:ff", "10.0.0.5", "test-device")
	if err != nil {
		t.Fatalf("insert device: %v", err)
	}
	devID, _ := deviceID.LastInsertId()

	deviceRepo.addDevice(&device.Device{
		ID:       devID,
		MAC:      "aa:bb:cc:dd:ee:ff",
		IP:       "10.0.0.5",
		Hostname: "test-device",
	})

	// Create active session
	_, err = sessionRepo.Create(&Session{
		DeviceID:  devID,
		MBLimit:   100,
		MBUsed:    50,
		Status:    SessionActive,
		ExpiresAt: time.Now().Add(1 * time.Hour),
	})
	if err != nil {
		t.Fatalf("create session: %v", err)
	}

	// Add client to mock router
	router.AddTestClient(&mikrotik.Client{
		MAC:  "aa:bb:cc:dd:ee:ff",
		IP:   "10.0.0.5",
	})

	worker := NewEnforcementWorker(router, sessionRepo, deviceRepo, nil, eventBus, loggr, 100*time.Millisecond)

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	if err := worker.Start(ctx); err != nil {
		t.Fatalf("Start error: %v", err)
	}

	// Let it run for multiple cycles
	time.Sleep(500 * time.Millisecond)

	// Verify no duplicate sessions were created
	sessions, err := sessionRepo.ListActive()
	if err != nil {
		t.Fatalf("list active sessions: %v", err)
	}
	if len(sessions) != 1 {
		t.Errorf("expected 1 active session, got %d", len(sessions))
	}

	worker.Stop()
}

func TestEnforcementWorkerPublishesEvents(t *testing.T) {
	db := database.TestDB(t)
	sessionRepo := NewSQLRepository(db)
	router := mikrotik.NewMockRouter()
	deviceRepo := newMockDeviceRepo()
	eventBus := events.NewBus()
	loggr := logger.New("debug", nil)

	// Track events
	var authorizedEvents int
	var deauthorizedEvents int
	eventBus.Subscribe(events.EventClientAuthorized, func(e events.Event) {
		authorizedEvents++
	})
	eventBus.Subscribe(events.EventClientDeauthorized, func(e events.Event) {
		deauthorizedEvents++
	})

	// Create device
	deviceID, err := db.Exec(`INSERT INTO devices(mac_addr, ip_addr, hostname) VALUES(?, ?, ?)`,
		"aa:bb:cc:dd:ee:ff", "10.0.0.5", "test-device")
	if err != nil {
		t.Fatalf("insert device: %v", err)
	}
	devID, _ := deviceID.LastInsertId()

	deviceRepo.addDevice(&device.Device{
		ID:       devID,
		MAC:      "aa:bb:cc:dd:ee:ff",
		IP:       "10.0.0.5",
		Hostname: "test-device",
	})

	// Create expired session (should trigger deauthorization event)
	_, err = sessionRepo.Create(&Session{
		DeviceID:  devID,
		MBLimit:   100,
		MBUsed:    50,
		Status:    SessionActive,
		ExpiresAt: time.Now().Add(-1 * time.Hour),
	})
	if err != nil {
		t.Fatalf("create session: %v", err)
	}

	worker := NewEnforcementWorker(router, sessionRepo, deviceRepo, nil, eventBus, loggr, 100*time.Millisecond)

	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	if err := worker.Start(ctx); err != nil {
		t.Fatalf("Start error: %v", err)
	}

	time.Sleep(200 * time.Millisecond)

	// Deauthorization event should have been published
	if deauthorizedEvents == 0 {
		t.Error("expected deauthorization event to be published")
	}

	worker.Stop()
}

// mockErrorRouter wraps MockRouter and returns errors for specific operations.
type mockErrorRouter struct {
	*mikrotik.MockRouter
	getUsageErr error
}

func (m *mockErrorRouter) GetClientUsage(ctx context.Context, mac string) (int64, int64, error) {
	if m.getUsageErr != nil {
		return 0, 0, m.getUsageErr
	}
	return m.MockRouter.GetClientUsage(ctx, mac)
}

func TestEnforcementWorkerUsageError(t *testing.T) {
	db := database.TestDB(t)
	sessionRepo := NewSQLRepository(db)
	router := &mockErrorRouter{
		MockRouter:  mikrotik.NewMockRouter(),
		getUsageErr: errors.New("router error"),
	}
	deviceRepo := newMockDeviceRepo()
	eventBus := events.NewBus()
	loggr := logger.New("debug", nil)

	// Create device
	deviceID, err := db.Exec(`INSERT INTO devices(mac_addr, ip_addr, hostname) VALUES(?, ?, ?)`,
		"aa:bb:cc:dd:ee:ff", "10.0.0.5", "test-device")
	if err != nil {
		t.Fatalf("insert device: %v", err)
	}
	devID, _ := deviceID.LastInsertId()

	deviceRepo.addDevice(&device.Device{
		ID:       devID,
		MAC:      "aa:bb:cc:dd:ee:ff",
		IP:       "10.0.0.5",
		Hostname: "test-device",
	})

	// Create active session
	session, err := sessionRepo.Create(&Session{
		DeviceID:  devID,
		MBLimit:   100,
		MBUsed:    50,
		Status:    SessionActive,
		ExpiresAt: time.Now().Add(1 * time.Hour),
	})
	if err != nil {
		t.Fatalf("create session: %v", err)
	}

	worker := NewEnforcementWorker(router, sessionRepo, deviceRepo, nil, eventBus, loggr, 100*time.Millisecond)

	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	if err := worker.Start(ctx); err != nil {
		t.Fatalf("Start error: %v", err)
	}

	time.Sleep(200 * time.Millisecond)

	// Worker should still be running
	if !worker.IsRunning() {
		t.Error("worker should still be running after usage error")
	}

	// Session should still be active (usage error doesn't change DB state)
	sess, err := sessionRepo.GetByID(session.ID)
	if err != nil {
		t.Fatalf("get session: %v", err)
	}
	if sess == nil {
		t.Fatal("session should exist")
	}
	if sess.Status != SessionActive {
		t.Errorf("session should still be active, got %s", sess.Status)
	}

	// Usage should not have changed
	if sess.MBUsed != 50 {
		t.Errorf("usage should not have changed, got %f", sess.MBUsed)
	}

	worker.Stop()
}
