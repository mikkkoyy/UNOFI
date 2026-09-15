package device

import "time"

// Device represents a connected WiFi client.
type Device struct {
	ID         int64     `json:"id"`
	MAC        string    `json:"mac"`
	IP         string    `json:"ip"`
	Hostname   string    `json:"host"`
	Online     bool      `json:"online"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
	LastSeenAt time.Time `json:"last_seen_at"`
}

// DeviceRepository defines device persistence operations.
type DeviceRepository interface {
	Create(mac, ip, hostname string) (*Device, error)
	GetByID(id int64) (*Device, error)
	GetByMAC(mac string) (*Device, error)
	GetByIP(ip string) (*Device, error)
	Update(id int64, ip, hostname string) error
	UpdateLastSeen(id int64) error
	SetOnline(id int64, online bool) error
	List(limit, offset int) ([]*Device, error)
	ListOnline() ([]*Device, error)
	Count() (int64, error)
}

// DeviceStatus represents the current connection state of a device.
type DeviceStatus struct {
	Device    *Device `json:"device"`
	Connected bool    `json:"connected"`
	SessionID *int64  `json:"session_id,omitempty"`
}

// IsKnown returns true if the device has been registered.
func (d *Device) IsKnown() bool {
	return d.ID > 0
}

// IsActive returns true if the device was seen recently (within 5 minutes).
func (d *Device) IsActive() bool {
	if d.LastSeenAt.IsZero() {
		return false
	}
	return time.Since(d.LastSeenAt) < 5*time.Minute
}
