package session

import (
	"testing"
	"time"

	"github.com/unofi/unofi/internal/database"
)

func TestSQLRepositoryCreate(t *testing.T) {
	db := database.TestDB(t)
	repo := NewSQLRepository(db)

	s := &Session{
		DeviceID:         1,
		MBLimit:          100,
		MBUsed:           0,
		TimeLimitSeconds: 3600,
		TimeUsedSeconds:  0,
		Status:           SessionActive,
	}

	created, err := repo.Create(s)
	if err != nil {
		t.Fatalf("Create error: %v", err)
	}
	if created.ID == 0 {
		t.Error("session ID should not be 0")
	}
	if created.Status != SessionActive {
		t.Errorf("expected status %s, got %s", SessionActive, created.Status)
	}
}

func TestSQLRepositoryGetByID(t *testing.T) {
	db := database.TestDB(t)
	repo := NewSQLRepository(db)

	s := &Session{
		DeviceID: 1,
		MBLimit:  100,
		Status:   SessionActive,
	}
	created, _ := repo.Create(s)

	got, err := repo.GetByID(created.ID)
	if err != nil {
		t.Fatalf("GetByID error: %v", err)
	}
	if got == nil {
		t.Fatal("session should be found")
	}
	if got.MBLimit != 100 {
		t.Errorf("expected MBLimit 100, got %f", got.MBLimit)
	}
}

func TestSQLRepositoryGetActiveByDevice(t *testing.T) {
	db := database.TestDB(t)
	repo := NewSQLRepository(db)

	// Create two sessions for same device
	_, _ = repo.Create(&Session{DeviceID: 1, MBLimit: 50, Status: SessionActive})
	_, _ = repo.Create(&Session{DeviceID: 1, MBLimit: 100, Status: SessionActive})

	active, err := repo.GetActiveByDevice(1)
	if err != nil {
		t.Fatalf("GetActiveByDevice error: %v", err)
	}
	if active == nil {
		t.Fatal("active session should be found")
	}
	// Should return the most recent one
	if active.MBLimit != 100 {
		t.Errorf("expected most recent MBLimit 100, got %f", active.MBLimit)
	}
}

func TestSQLRepositoryUpdateUsage(t *testing.T) {
	db := database.TestDB(t)
	repo := NewSQLRepository(db)

	s, _ := repo.Create(&Session{DeviceID: 1, MBLimit: 100, Status: SessionActive})

	err := repo.UpdateUsage(s.ID, 25.5, 600)
	if err != nil {
		t.Fatalf("UpdateUsage error: %v", err)
	}

	updated, _ := repo.GetByID(s.ID)
	if updated.MBUsed != 25.5 {
		t.Errorf("expected MBUsed 25.5, got %f", updated.MBUsed)
	}
	if updated.TimeUsedSeconds != 600 {
		t.Errorf("expected TimeUsedSeconds 600, got %d", updated.TimeUsedSeconds)
	}
}

func TestSQLRepositoryUpdateStatus(t *testing.T) {
	db := database.TestDB(t)
	repo := NewSQLRepository(db)

	s, _ := repo.Create(&Session{DeviceID: 1, MBLimit: 100, Status: SessionActive})

	err := repo.UpdateStatus(s.ID, SessionExpired)
	if err != nil {
		t.Fatalf("UpdateStatus error: %v", err)
	}

	updated, _ := repo.GetByID(s.ID)
	if updated.Status != SessionExpired {
		t.Errorf("expected status %s, got %s", SessionExpired, updated.Status)
	}
}

func TestSQLRepositoryListExpired(t *testing.T) {
	db := database.TestDB(t)
	repo := NewSQLRepository(db)

	// Create expired session
	expiresInPast := time.Now().Add(-1 * time.Hour)
	_, _ = repo.Create(&Session{
		DeviceID:  1,
		MBLimit:   100,
		Status:    SessionActive,
		ExpiresAt: expiresInPast,
	})

	// Create future session
	expiresInFuture := time.Now().Add(1 * time.Hour)
	_, _ = repo.Create(&Session{
		DeviceID:  2,
		MBLimit:   100,
		Status:    SessionActive,
		ExpiresAt: expiresInFuture,
	})

	expired, err := repo.ListExpired()
	if err != nil {
		t.Fatalf("ListExpired error: %v", err)
	}
	if len(expired) != 1 {
		t.Errorf("expected 1 expired session, got %d", len(expired))
	}
}

func TestSessionIsValid(t *testing.T) {
	tests := []struct {
		name    string
		session Session
		valid   bool
	}{
		{
			name:    "valid active session",
			session: Session{Status: SessionActive, MBLimit: 100, MBUsed: 50},
			valid:   true,
		},
		{
			name:    "exhausted data",
			session: Session{Status: SessionActive, MBLimit: 100, MBUsed: 100},
			valid:   false,
		},
		{
			name:    "expired status",
			session: Session{Status: SessionExpired, MBLimit: 100, MBUsed: 50},
			valid:   false,
		},
		{
			name:    "exhausted time",
			session: Session{Status: SessionActive, TimeLimitSeconds: 3600, TimeUsedSeconds: 3600},
			valid:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.session.IsValid(); got != tt.valid {
				t.Errorf("IsValid() = %v, want %v", got, tt.valid)
			}
		})
	}
}

func TestSessionRemainingMB(t *testing.T) {
	s := Session{MBLimit: 100, MBUsed: 30}
	if got := s.RemainingMB(); got != 70 {
		t.Errorf("expected 70, got %f", got)
	}

	s2 := Session{MBLimit: 100, MBUsed: 150}
	if got := s2.RemainingMB(); got != 0 {
		t.Errorf("expected 0, got %f", got)
	}
}
