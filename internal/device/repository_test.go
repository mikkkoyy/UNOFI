package device

import (
	"testing"
	"time"

	"github.com/unofi/unofi/internal/database"
)

func TestSQLRepositoryCreate(t *testing.T) {
	db := database.TestDB(t)
	repo := NewSQLRepository(db)

	d, err := repo.Create("AA:BB:CC:DD:EE:FF", "10.0.0.5", "test-device")
	if err != nil {
		t.Fatalf("Create error: %v", err)
	}
	if d.ID == 0 {
		t.Error("device ID should not be 0")
	}
	if d.MAC != "AA:BB:CC:DD:EE:FF" {
		t.Errorf("expected MAC AA:BB:CC:DD:EE:FF, got %s", d.MAC)
	}
	if d.IP != "10.0.0.5" {
		t.Errorf("expected IP 10.0.0.5, got %s", d.IP)
	}
}

func TestSQLRepositoryGetByMAC(t *testing.T) {
	db := database.TestDB(t)
	repo := NewSQLRepository(db)

	_, err := repo.Create("AA:BB:CC:DD:EE:FF", "10.0.0.5", "test-device")
	if err != nil {
		t.Fatalf("Create error: %v", err)
	}

	d, err := repo.GetByMAC("AA:BB:CC:DD:EE:FF")
	if err != nil {
		t.Fatalf("GetByMAC error: %v", err)
	}
	if d == nil {
		t.Fatal("device should be found")
	}
	if d.MAC != "AA:BB:CC:DD:EE:FF" {
		t.Errorf("expected MAC AA:BB:CC:DD:EE:FF, got %s", d.MAC)
	}
}

func TestSQLRepositoryGetByMACNotFound(t *testing.T) {
	db := database.TestDB(t)
	repo := NewSQLRepository(db)

	d, err := repo.GetByMAC("FF:FF:FF:FF:FF:FF")
	if err != nil {
		t.Fatalf("GetByMAC error: %v", err)
	}
	if d != nil {
		t.Error("device should not be found")
	}
}

func TestSQLRepositoryUpsert(t *testing.T) {
	db := database.TestDB(t)
	repo := NewSQLRepository(db)

	// Create device
	_, err := repo.Create("AA:BB:CC:DD:EE:FF", "10.0.0.5", "test-device")
	if err != nil {
		t.Fatalf("Create error: %v", err)
	}

	// Upsert with new IP
	d, err := repo.Create("AA:BB:CC:DD:EE:FF", "10.0.0.6", "test-device-2")
	if err != nil {
		t.Fatalf("Upsert error: %v", err)
	}

	// Should have updated IP
	got, _ := repo.GetByMAC("AA:BB:CC:DD:EE:FF")
	if got.IP != "10.0.0.6" {
		t.Errorf("expected updated IP 10.0.0.6, got %s", got.IP)
	}
	if got.Hostname != "test-device-2" {
		t.Errorf("expected updated hostname, got %s", got.Hostname)
	}
	_ = d
}

func TestSQLRepositoryList(t *testing.T) {
	db := database.TestDB(t)
	repo := NewSQLRepository(db)

	// Create multiple devices
	for i := 0; i < 5; i++ {
		_, err := repo.Create(
			string(rune('A'+i))+":BB:CC:DD:EE:FF",
			"10.0.0."+string(rune('1'+i)),
			"device-"+string(rune('a'+i)),
		)
		if err != nil {
			t.Fatalf("Create error: %v", err)
		}
	}

	devices, err := repo.List(10, 0)
	if err != nil {
		t.Fatalf("List error: %v", err)
	}
	if len(devices) != 5 {
		t.Errorf("expected 5 devices, got %d", len(devices))
	}
}

func TestSQLRepositoryCount(t *testing.T) {
	db := database.TestDB(t)
	repo := NewSQLRepository(db)

	count, _ := repo.Count()
	if count != 0 {
		t.Errorf("expected 0 devices initially, got %d", count)
	}

	_, _ = repo.Create("AA:BB:CC:DD:EE:FF", "10.0.0.5", "test-device")

	count, _ = repo.Count()
	if count != 1 {
		t.Errorf("expected 1 device, got %d", count)
	}
}

func TestDeviceIsActive(t *testing.T) {
	d := &Device{}
	if d.IsActive() {
		t.Error("device with zero time should not be active")
	}

	d2 := &Device{LastSeenAt: time.Now()}
	if !d2.IsActive() {
		t.Error("device with recent time should be active")
	}
}
