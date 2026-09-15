package mikrotik

import (
	"context"
	"fmt"
	"time"
)

// ClientState represents the authorization state of a client on the MikroTik.
type ClientState string

const (
	StateAuthorized   ClientState = "authorized"
	StateUnauthorized ClientState = "unauthorized"
	StatePending      ClientState = "pending"
)

// Client represents a network client as seen by the MikroTik router.
type Client struct {
	ID           string      `json:"id"`
	MAC          string      `json:"mac"`
	IP           string      `json:"ip"`
	Hostname     string      `json:"hostname"`
	State        ClientState `json:"state"`
	BytesIn      int64       `json:"bytes_in"`
	BytesOut     int64       `json:"bytes_out"`
	Uptime       string      `json:"uptime"`
	Interface    string      `json:"interface"`
}

// BandwidthRule represents a queue or bandwidth limitation rule.
type BandwidthRule struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	TargetIP string `json:"target_ip"`
	MaxLimit string `json:"max_limit"`
	Enabled  bool   `json:"enabled"`
}

// SystemInfo represents MikroTik system information.
type SystemInfo struct {
	Identity     string `json:"identity"`
	Version      string `json:"version"`
	Model        string `json:"model"`
	Uptime       string `json:"uptime"`
	CPUUsage     int    `json:"cpu_usage"`
	MemoryUsage  int    `json:"memory_usage"`
	TotalMemory  int    `json:"total_memory"`
	FreeMemory   int    `json:"free_memory"`
}

// HealthStatus represents the health of the MikroTik connection.
type HealthStatus struct {
	Reachable bool          `json:"reachable"`
	Latency   time.Duration `json:"latency"`
	Error     string        `json:"error,omitempty"`
	LastCheck time.Time     `json:"last_check"`
}

// Client defines the interface for client management operations.
type ClientOps interface {
	// GetClients returns all hotspot clients.
	GetClients(ctx context.Context) ([]*Client, error)
	// GetClientByMAC returns a specific client by MAC address.
	GetClientByMAC(ctx context.Context, mac string) (*Client, error)
	// AuthorizeClient authorizes a client for internet access.
	AuthorizeClient(ctx context.Context, mac, ip string) error
	// DeauthorizeClient removes a client's authorization.
	DeauthorizeClient(ctx context.Context, mac string) error
	// DisconnectClient forcibly disconnects a client.
	DisconnectClient(ctx context.Context, mac string) error
	// GetClientUsage returns data usage for a client.
	GetClientUsage(ctx context.Context, mac string) (bytesIn, bytesOut int64, err error)
}

// Bandwidth defines the interface for bandwidth management operations.
type Bandwidth interface {
	// CreateRule creates a bandwidth limitation rule.
	CreateRule(ctx context.Context, rule *BandwidthRule) error
	// UpdateRule modifies an existing bandwidth rule.
	UpdateRule(ctx context.Context, rule *BandwidthRule) error
	// RemoveRule deletes a bandwidth rule.
	RemoveRule(ctx context.Context, ruleID string) error
	// GetRules returns all bandwidth rules.
	GetRules(ctx context.Context) ([]*BandwidthRule, error)
}

// System defines the interface for system information operations.
type System interface {
	// GetInfo returns system information.
	GetInfo(ctx context.Context) (*SystemInfo, error)
	// CheckHealth verifies connectivity to the router.
	CheckHealth(ctx context.Context) *HealthStatus
}

// Router combines all MikroTik operations into a single interface.
type Router interface {
	ClientOps
	Bandwidth
	System
}

// Config holds MikroTik connection configuration.
type Config struct {
	Address  string
	Port     int
	Username string
	Password string
	UseTLS   bool
	Timeout  time.Duration
}

// Validate checks that the MikroTik configuration is valid.
func (c Config) Validate() error {
	if c.Address == "" {
		return fmt.Errorf("mikrotik address is required")
	}
	if c.Port <= 0 || c.Port > 65535 {
		return fmt.Errorf("mikrotik port must be between 1 and 65535")
	}
	if c.Username == "" {
		return fmt.Errorf("mikrotik username is required")
	}
	if c.Password == "" {
		return fmt.Errorf("mikrotik password is required")
	}
	return nil
}

// AddressWithPort returns the full address with port.
func (c Config) AddressWithPort() string {
	return fmt.Sprintf("%s:%d", c.Address, c.Port)
}
