package device

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/unofi/unofi/internal/database"
)

// SQLRepository implements DeviceRepository using SQLite.
type SQLRepository struct {
	db *database.DB
}

// NewSQLRepository creates a new SQL-backed device repository.
func NewSQLRepository(db *database.DB) *SQLRepository {
	return &SQLRepository{db: db}
}

// Create inserts a new device or updates an existing one by MAC.
func (r *SQLRepository) Create(mac, ip, hostname string) (*Device, error) {
	// Try upsert: insert or update on MAC conflict
	result, err := r.db.Exec(
		`INSERT INTO devices(mac_addr, ip_addr, hostname, created_at, updated_at, last_seen_at)
		 VALUES(?, ?, ?, ?, ?, ?)
		 ON CONFLICT(mac_addr) DO UPDATE SET
		   ip_addr = excluded.ip_addr,
		   hostname = excluded.hostname,
		   updated_at = excluded.updated_at,
		   last_seen_at = excluded.last_seen_at`,
		mac, ip, hostname, time.Now(), time.Now(), time.Now(),
	)
	if err != nil {
		return nil, fmt.Errorf("upsert device: %w", err)
	}

	id, _ := result.LastInsertId()
	// If it was an update, LastInsertId may be 0, look it up
	if id == 0 {
		return r.GetByMAC(mac)
	}

	return &Device{
		ID:         id,
		MAC:        mac,
		IP:         ip,
		Hostname:   hostname,
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
		LastSeenAt: time.Now(),
	}, nil
}

// GetByID retrieves a device by its ID.
func (r *SQLRepository) GetByID(id int64) (*Device, error) {
	row := r.db.QueryRow(
		`SELECT id, mac_addr, ip_addr, hostname, online, created_at, updated_at, last_seen_at
		 FROM devices WHERE id = ?`, id,
	)
	return scanDevice(row)
}

// GetByMAC retrieves a device by its MAC address.
func (r *SQLRepository) GetByMAC(mac string) (*Device, error) {
	row := r.db.QueryRow(
		`SELECT id, mac_addr, ip_addr, hostname, online, created_at, updated_at, last_seen_at
		 FROM devices WHERE mac_addr = ?`, mac,
	)
	return scanDevice(row)
}

// GetByIP retrieves a device by its IP address.
func (r *SQLRepository) GetByIP(ip string) (*Device, error) {
	row := r.db.QueryRow(
		`SELECT id, mac_addr, ip_addr, hostname, online, created_at, updated_at, last_seen_at
		 FROM devices WHERE ip_addr = ? ORDER BY last_seen_at DESC LIMIT 1`, ip,
	)
	return scanDevice(row)
}

// Update modifies a device's IP and hostname.
func (r *SQLRepository) Update(id int64, ip, hostname string) error {
	_, err := r.db.Exec(
		`UPDATE devices SET ip_addr = ?, hostname = ?, updated_at = ? WHERE id = ?`,
		ip, hostname, time.Now(), id,
	)
	return err
}

// UpdateLastSeen refreshes the device's last seen timestamp.
func (r *SQLRepository) UpdateLastSeen(id int64) error {
	_, err := r.db.Exec(
		`UPDATE devices SET last_seen_at = ?, updated_at = ? WHERE id = ?`,
		time.Now(), time.Now(), id,
	)
	return err
}

// SetOnline updates the device's online status.
func (r *SQLRepository) SetOnline(id int64, online bool) error {
	_, err := r.db.Exec(
		`UPDATE devices SET online = ?, updated_at = ? WHERE id = ?`,
		online, time.Now(), id,
	)
	return err
}

// List returns a paginated list of devices ordered by last seen.
func (r *SQLRepository) List(limit, offset int) ([]*Device, error) {
	rows, err := r.db.Query(
		`SELECT id, mac_addr, ip_addr, hostname, online, created_at, updated_at, last_seen_at
		 FROM devices ORDER BY last_seen_at DESC LIMIT ? OFFSET ?`,
		limit, offset,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var devices []*Device
	for rows.Next() {
		d, err := scanDevice(rows)
		if err != nil {
			return nil, err
		}
		devices = append(devices, d)
	}
	return devices, rows.Err()
}

// ListOnline returns all currently online devices.
func (r *SQLRepository) ListOnline() ([]*Device, error) {
	rows, err := r.db.Query(
		`SELECT id, mac_addr, ip_addr, hostname, online, created_at, updated_at, last_seen_at
		 FROM devices WHERE online = 1 ORDER BY last_seen_at DESC`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var devices []*Device
	for rows.Next() {
		d, err := scanDevice(rows)
		if err != nil {
			return nil, err
		}
		devices = append(devices, d)
	}
	return devices, rows.Err()
}

// Count returns the total number of devices.
func (r *SQLRepository) Count() (int64, error) {
	var count int64
	err := r.db.QueryRow(`SELECT COUNT(*) FROM devices`).Scan(&count)
	return count, err
}

func scanDevice(scanner interface {
	Scan(dest ...interface{}) error
}) (*Device, error) {
	var d Device
	var createdAt, updatedAt, lastSeenAt sql.NullTime
	err := scanner.Scan(
		&d.ID, &d.MAC, &d.IP, &d.Hostname, &d.Online,
		&createdAt, &updatedAt, &lastSeenAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("scan device: %w", err)
	}
	if createdAt.Valid {
		d.CreatedAt = createdAt.Time
	}
	if updatedAt.Valid {
		d.UpdatedAt = updatedAt.Time
	}
	if lastSeenAt.Valid {
		d.LastSeenAt = lastSeenAt.Time
	}
	return &d, nil
}
