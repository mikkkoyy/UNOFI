package mikrotik

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// MockRouter is a mock implementation of the Router interface for development and testing.
type MockRouter struct {
	mu         sync.RWMutex
	clients    map[string]*Client
	rules      map[string]*BandwidthRule
	reachable  bool
	latency    time.Duration
}

// NewMockRouter creates a new MockRouter.
func NewMockRouter() *MockRouter {
	return &MockRouter{
		clients:   make(map[string]*Client),
		rules:     make(map[string]*BandwidthRule),
		reachable: true,
		latency:   1 * time.Millisecond,
	}
}

// GetClients returns all hotspot clients.
func (m *MockRouter) GetClients(ctx context.Context) ([]*Client, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if !m.reachable {
		return nil, fmt.Errorf("mikrotik unreachable")
	}
	clients := make([]*Client, 0, len(m.clients))
	for _, c := range m.clients {
		clients = append(clients, c)
	}
	return clients, nil
}

// GetClientByMAC returns a specific client by MAC address.
func (m *MockRouter) GetClientByMAC(ctx context.Context, mac string) (*Client, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if !m.reachable {
		return nil, fmt.Errorf("mikrotik unreachable")
	}
	if c, ok := m.clients[mac]; ok {
		return c, nil
	}
	return nil, nil
}

// AuthorizeClient authorizes a client for internet access.
func (m *MockRouter) AuthorizeClient(ctx context.Context, mac, ip string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if !m.reachable {
		return fmt.Errorf("mikrotik unreachable")
	}
	if c, ok := m.clients[mac]; ok {
		c.State = StateAuthorized
		c.IP = ip
	} else {
		m.clients[mac] = &Client{
			ID:    fmt.Sprintf("*%d", len(m.clients)+1),
			MAC:   mac,
			IP:    ip,
			State: StateAuthorized,
		}
	}
	return nil
}

// DeauthorizeClient removes a client's authorization.
func (m *MockRouter) DeauthorizeClient(ctx context.Context, mac string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if !m.reachable {
		return fmt.Errorf("mikrotik unreachable")
	}
	if c, ok := m.clients[mac]; ok {
		c.State = StateUnauthorized
	}
	return nil
}

// DisconnectClient forcibly disconnects a client.
func (m *MockRouter) DisconnectClient(ctx context.Context, mac string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if !m.reachable {
		return fmt.Errorf("mikrotik unreachable")
	}
	delete(m.clients, mac)
	return nil
}

// GetClientUsage returns data usage for a client.
func (m *MockRouter) GetClientUsage(ctx context.Context, mac string) (int64, int64, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if !m.reachable {
		return 0, 0, fmt.Errorf("mikrotik unreachable")
	}
	if c, ok := m.clients[mac]; ok {
		return c.BytesIn, c.BytesOut, nil
	}
	return 0, 0, fmt.Errorf("client not found: %s", mac)
}

// CreateRule creates a bandwidth limitation rule.
func (m *MockRouter) CreateRule(ctx context.Context, rule *BandwidthRule) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if !m.reachable {
		return fmt.Errorf("mikrotik unreachable")
	}
	if rule.ID == "" {
		rule.ID = fmt.Sprintf("*%d", len(m.rules)+1)
	}
	m.rules[rule.ID] = rule
	return nil
}

// UpdateRule modifies an existing bandwidth rule.
func (m *MockRouter) UpdateRule(ctx context.Context, rule *BandwidthRule) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if !m.reachable {
		return fmt.Errorf("mikrotik unreachable")
	}
	if _, ok := m.rules[rule.ID]; !ok {
		return fmt.Errorf("rule not found: %s", rule.ID)
	}
	m.rules[rule.ID] = rule
	return nil
}

// RemoveRule deletes a bandwidth rule.
func (m *MockRouter) RemoveRule(ctx context.Context, ruleID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if !m.reachable {
		return fmt.Errorf("mikrotik unreachable")
	}
	delete(m.rules, ruleID)
	return nil
}

// GetRules returns all bandwidth rules.
func (m *MockRouter) GetRules(ctx context.Context) ([]*BandwidthRule, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if !m.reachable {
		return nil, fmt.Errorf("mikrotik unreachable")
	}
	rules := make([]*BandwidthRule, 0, len(m.rules))
	for _, r := range m.rules {
		rules = append(rules, r)
	}
	return rules, nil
}

// GetInfo returns system information.
func (m *MockRouter) GetInfo(ctx context.Context) (*SystemInfo, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if !m.reachable {
		return nil, fmt.Errorf("mikrotik unreachable")
	}
	return &SystemInfo{
		Identity:    "MockRouter",
		Version:     "7.1",
		Model:       "CCR1009",
		Uptime:      "1d 2h 3m",
		CPUUsage:    5,
		MemoryUsage: 30,
		TotalMemory: 1024,
		FreeMemory:  716,
	}, nil
}

// CheckHealth verifies connectivity to the router.
func (m *MockRouter) CheckHealth(ctx context.Context) *HealthStatus {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return &HealthStatus{
		Reachable: m.reachable,
		Latency:   m.latency,
		LastCheck: time.Now(),
	}
}

// SetReachable sets the mock router's reachability for testing.
func (m *MockRouter) SetReachable(reachable bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.reachable = reachable
}

// AddTestClient adds a client to the mock router for testing.
func (m *MockRouter) AddTestClient(client *Client) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.clients[client.MAC] = client
}
