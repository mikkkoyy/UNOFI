package session

import "time"

// Session represents an active data/time allocation for a device.
type Session struct {
	ID               int64     `json:"id"`
	DeviceID         int64     `json:"device_id"`
	VoucherID        *int64    `json:"voucher_id,omitempty"`
	PaymentID        *int64    `json:"payment_id,omitempty"`
	MBLimit          float64   `json:"mb_limit"`
	MBUsed           float64   `json:"mb_used"`
	TimeLimitSeconds int       `json:"time_limit_seconds"`
	TimeUsedSeconds  int       `json:"time_used_seconds"`
	ExpiresAt        time.Time `json:"expires_at,omitempty"`
	Status           string    `json:"status"`
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`
}

// SessionStatus represents possible session states.
const (
	SessionActive    = "active"
	SessionExpired   = "expired"
	SessionExhausted = "exhausted"
	SessionCancelled = "cancelled"
)

// SessionRepository defines session persistence operations.
type SessionRepository interface {
	Create(s *Session) (*Session, error)
	GetByID(id int64) (*Session, error)
	GetActiveByDevice(deviceID int64) (*Session, error)
	ListByDevice(deviceID int64, limit int) ([]*Session, error)
	UpdateUsage(id int64, mbUsed float64, timeUsed int) error
	UpdateStatus(id int64, status string) error
	ListExpired() ([]*Session, error)
	ListActive() ([]*Session, error)
}

// RemainingMB returns the remaining data allocation.
func (s *Session) RemainingMB() float64 {
	remaining := s.MBLimit - s.MBUsed
	if remaining < 0 {
		return 0
	}
	return remaining
}

// RemainingTime returns the remaining time allocation.
func (s *Session) RemainingTime() int {
	remaining := s.TimeLimitSeconds - s.TimeUsedSeconds
	if remaining < 0 {
		return 0
	}
	return remaining
}

// IsExpired checks if the session has exceeded its time limit.
func (s *Session) IsExpired() bool {
	if s.ExpiresAt.IsZero() {
		return false
	}
	return time.Now().After(s.ExpiresAt)
}

// IsExhausted checks if the session has used all its data/time.
func (s *Session) IsExhausted() bool {
	if s.MBLimit > 0 && s.MBUsed >= s.MBLimit {
		return true
	}
	if s.TimeLimitSeconds > 0 && s.TimeUsedSeconds >= s.TimeLimitSeconds {
		return true
	}
	return false
}

// IsValid checks if the session is still usable.
func (s *Session) IsValid() bool {
	return s.Status == SessionActive && !s.IsExpired() && !s.IsExhausted()
}
