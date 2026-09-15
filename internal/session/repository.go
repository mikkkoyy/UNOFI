package session

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/unofi/unofi/internal/database"
)

// SQLRepository implements SessionRepository using SQLite.
type SQLRepository struct {
	db *database.DB
}

// NewSQLRepository creates a new SQL-backed session repository.
func NewSQLRepository(db *database.DB) *SQLRepository {
	return &SQLRepository{db: db}
}

// Create inserts a new session.
func (r *SQLRepository) Create(s *Session) (*Session, error) {
	var expiresAt interface{}
	if !s.ExpiresAt.IsZero() {
		expiresAt = s.ExpiresAt
	}

	result, err := r.db.Exec(
		`INSERT INTO sessions(device_id, voucher_id, payment_id, mb_limit, mb_used,
			time_limit_seconds, time_used_seconds, expires_at, status, created_at, updated_at)
		 VALUES(?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		s.DeviceID, s.VoucherID, s.PaymentID, s.MBLimit, s.MBUsed,
		s.TimeLimitSeconds, s.TimeUsedSeconds, expiresAt, s.Status,
		time.Now(), time.Now(),
	)
	if err != nil {
		return nil, fmt.Errorf("create session: %w", err)
	}

	s.ID, _ = result.LastInsertId()
	return s, nil
}

// GetByID retrieves a session by ID.
func (r *SQLRepository) GetByID(id int64) (*Session, error) {
	row := r.db.QueryRow(
		`SELECT id, device_id, voucher_id, payment_id, mb_limit, mb_used,
		        time_limit_seconds, time_used_seconds, expires_at, status,
		        created_at, updated_at
		 FROM sessions WHERE id = ?`, id,
	)
	return scanSession(row)
}

// GetActiveByDevice retrieves the active session for a device.
func (r *SQLRepository) GetActiveByDevice(deviceID int64) (*Session, error) {
	row := r.db.QueryRow(
		`SELECT id, device_id, voucher_id, payment_id, mb_limit, mb_used,
		        time_limit_seconds, time_used_seconds, expires_at, status,
		        created_at, updated_at
		 FROM sessions WHERE device_id = ? AND status = ?
		 ORDER BY id DESC LIMIT 1`, deviceID, SessionActive,
	)
	return scanSession(row)
}

// ListByDevice returns sessions for a device, most recent first.
func (r *SQLRepository) ListByDevice(deviceID int64, limit int) ([]*Session, error) {
	rows, err := r.db.Query(
		`SELECT id, device_id, voucher_id, payment_id, mb_limit, mb_used,
		        time_limit_seconds, time_used_seconds, expires_at, status,
		        created_at, updated_at
		 FROM sessions WHERE device_id = ? ORDER BY created_at DESC LIMIT ?`,
		deviceID, limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return scanSessions(rows)
}

// UpdateUsage updates the session's data and time usage.
func (r *SQLRepository) UpdateUsage(id int64, mbUsed float64, timeUsed int) error {
	_, err := r.db.Exec(
		`UPDATE sessions SET mb_used = ?, time_used_seconds = ?, updated_at = ? WHERE id = ?`,
		mbUsed, timeUsed, time.Now(), id,
	)
	return err
}

// UpdateStatus updates the session's status.
func (r *SQLRepository) UpdateStatus(id int64, status string) error {
	_, err := r.db.Exec(
		`UPDATE sessions SET status = ?, updated_at = ? WHERE id = ?`,
		status, time.Now(), id,
	)
	return err
}

// ListExpired returns sessions that have passed their expiration time.
func (r *SQLRepository) ListExpired() ([]*Session, error) {
	rows, err := r.db.Query(
		`SELECT id, device_id, voucher_id, payment_id, mb_limit, mb_used,
		        time_limit_seconds, time_used_seconds, expires_at, status,
		        created_at, updated_at
		 FROM sessions WHERE status = ? AND expires_at IS NOT NULL AND expires_at < ?`,
		SessionActive, time.Now(),
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return scanSessions(rows)
}

// ListActive returns all active sessions.
func (r *SQLRepository) ListActive() ([]*Session, error) {
	rows, err := r.db.Query(
		`SELECT id, device_id, voucher_id, payment_id, mb_limit, mb_used,
		        time_limit_seconds, time_used_seconds, expires_at, status,
		        created_at, updated_at
		 FROM sessions WHERE status = ?`, SessionActive,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return scanSessions(rows)
}

func scanSession(scanner interface{ Scan(dest ...interface{}) error }) (*Session, error) {
	var s Session
	var voucherID, paymentID sql.NullInt64
	var expiresAt sql.NullTime
	var createdAt, updatedAt sql.NullTime

	err := scanner.Scan(
		&s.ID, &s.DeviceID, &voucherID, &paymentID,
		&s.MBLimit, &s.MBUsed, &s.TimeLimitSeconds, &s.TimeUsedSeconds,
		&expiresAt, &s.Status, &createdAt, &updatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("scan session: %w", err)
	}

	if voucherID.Valid {
		s.VoucherID = &voucherID.Int64
	}
	if paymentID.Valid {
		s.PaymentID = &paymentID.Int64
	}
	if expiresAt.Valid {
		s.ExpiresAt = expiresAt.Time
	}
	if createdAt.Valid {
		s.CreatedAt = createdAt.Time
	}
	if updatedAt.Valid {
		s.UpdatedAt = updatedAt.Time
	}

	return &s, nil
}

func scanSessions(rows *sql.Rows) ([]*Session, error) {
	var sessions []*Session
	for rows.Next() {
		s, err := scanSession(rows)
		if err != nil {
			return nil, err
		}
		sessions = append(sessions, s)
	}
	return sessions, rows.Err()
}
