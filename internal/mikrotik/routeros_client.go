package mikrotik

import (
	"context"
	"crypto/tls"
	"fmt"
	"strings"
	"sync"
	"time"

	routeros "github.com/go-routeros/routeros/v3"
)

// RouterOSClient is a real implementation of the Router interface using the RouterOS API.
type RouterOSClient struct {
	mu        sync.Mutex
	client    *routeros.Client
	config    Config
	connected bool
}

// NewRouterOSClient creates a new RouterOS client with the given configuration.
func NewRouterOSClient(config Config) *RouterOSClient {
	return &RouterOSClient{
		config: config,
	}
}

// Connect establishes a connection to the MikroTik router.
func (r *RouterOSClient) Connect() error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.connected && r.client != nil {
		return nil
	}

	addr := r.config.AddressWithPort()

	var client *routeros.Client
	var err error

	if r.config.UseTLS {
		client, err = routeros.DialTLS(addr, r.config.Username, r.config.Password, &tls.Config{InsecureSkipVerify: true})
	} else {
		client, err = routeros.Dial(addr, r.config.Username, r.config.Password)
	}

	if err != nil {
		return fmt.Errorf("connect to mikrotik %s: %w", r.config.Address, err)
	}

	r.client = client
	r.connected = true
	return nil
}

// Close disconnects from the MikroTik router.
func (r *RouterOSClient) Close() error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.client != nil {
		if err := r.client.Close(); err != nil {
			return fmt.Errorf("close mikrotik connection: %w", err)
		}
	}

	r.client = nil
	r.connected = false
	return nil
}

// ensureConnected checks if the client is connected and reconnects if necessary.
func (r *RouterOSClient) ensureConnected() error {
	if r.connected && r.client != nil {
		return nil
	}
	return r.Connect()
}

// IsConnected returns the current connection state.
func (r *RouterOSClient) IsConnected() bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.connected
}

// runCommand executes a RouterOS command and returns the reply.
func (r *RouterOSClient) runCommand(ctx context.Context, sentence string, args ...string) (*routeros.Reply, error) {
	r.mu.Lock()
	client := r.client
	r.mu.Unlock()

	if client == nil {
		return nil, fmt.Errorf("not connected to mikrotik")
	}

	fullArgs := append([]string{sentence}, args...)
	return client.RunContext(ctx, fullArgs...)
}

// ==================== Client Operations ====================

// GetClients returns all active HotSpot clients.
func (r *RouterOSClient) GetClients(ctx context.Context) ([]*Client, error) {
	if err := r.ensureConnected(); err != nil {
		return nil, err
	}

	reply, err := r.runCommand(ctx, "/ip/hotspot/active/print")
	if err != nil {
		return nil, fmt.Errorf("get hotspot active users: %w", err)
	}

	var clients []*Client
	for _, re := range reply.Re {
		c := &Client{
			ID:       re.Map[".id"],
			MAC:      strings.ToLower(re.Map["mac-address"]),
			IP:       re.Map["address"],
			Hostname: re.Map["host-name"],
			State:    StateAuthorized,
			Uptime:   re.Map["uptime"],
		}
		if c.Hostname == "" {
			c.Hostname = "-NA-"
		}
		clients = append(clients, c)
	}

	// Also get HotSpot hosts (unauthenticated but known)
	hostReply, err := r.runCommand(ctx, "/ip/hotspot/host/print")
	if err == nil {
		existingIPs := make(map[string]bool)
		for _, c := range clients {
			existingIPs[c.IP] = true
		}
		for _, re := range hostReply.Re {
			ip := re.Map["address"]
			if ip == "" || existingIPs[ip] {
				continue
			}
			c := &Client{
				ID:       re.Map[".id"],
				MAC:      strings.ToLower(re.Map["mac-address"]),
				IP:       ip,
				Hostname: re.Map["to-address"],
				State:    StateUnauthorized,
			}
			if c.Hostname == "" {
				c.Hostname = "-NA-"
			}
			clients = append(clients, c)
		}
	}

	return clients, nil
}

// GetClientByMAC returns a specific client by MAC address.
func (r *RouterOSClient) GetClientByMAC(ctx context.Context, mac string) (*Client, error) {
	if err := r.ensureConnected(); err != nil {
		return nil, err
	}

	mac = strings.ToLower(mac)

	// Search in active HotSpot users
	reply, err := r.runCommand(ctx, "/ip/hotspot/active/print", "?"+"mac-address="+mac)
	if err != nil {
		return nil, fmt.Errorf("get hotspot active by mac: %w", err)
	}

	for _, re := range reply.Re {
		c := &Client{
			ID:       re.Map[".id"],
			MAC:      strings.ToLower(re.Map["mac-address"]),
			IP:       re.Map["address"],
			Hostname: re.Map["host-name"],
			State:    StateAuthorized,
			Uptime:   re.Map["uptime"],
		}
		if c.Hostname == "" {
			c.Hostname = "-NA-"
		}
		return c, nil
	}

	// Search in HotSpot hosts
	hostReply, err := r.runCommand(ctx, "/ip/hotspot/host/print", "?"+"mac-address="+mac)
	if err != nil {
		return nil, fmt.Errorf("get hotspot host by mac: %w", err)
	}

	for _, re := range hostReply.Re {
		c := &Client{
			ID:       re.Map[".id"],
			MAC:      strings.ToLower(re.Map["mac-address"]),
			IP:       re.Map["address"],
			Hostname: re.Map["to-address"],
			State:    StateUnauthorized,
		}
		if c.Hostname == "" {
			c.Hostname = "-NA-"
		}
		return c, nil
	}

	return nil, nil
}

// AuthorizeClient authorizes a client for internet access via HotSpot login.
func (r *RouterOSClient) AuthorizeClient(ctx context.Context, mac, ip string) error {
	if err := r.ensureConnected(); err != nil {
		return err
	}

	mac = strings.ToLower(mac)

	// Use HotSpot login API to authorize the client
	_, err := r.runCommand(ctx,
		"/ip/hotspot/active/login",
		"=mac-address="+mac,
		"=ip="+ip,
		"=user=unofi",
	)
	if err != nil {
		return fmt.Errorf("authorize client mac=%s ip=%s: %w", mac, ip, err)
	}

	return nil
}

// DeauthorizeClient removes a client's authorization.
func (r *RouterOSClient) DeauthorizeClient(ctx context.Context, mac string) error {
	if err := r.ensureConnected(); err != nil {
		return err
	}

	mac = strings.ToLower(mac)

	reply, err := r.runCommand(ctx, "/ip/hotspot/active/print", "?"+"mac-address="+mac)
	if err != nil {
		return fmt.Errorf("find active user by mac: %w", err)
	}

	for _, re := range reply.Re {
		id := re.Map[".id"]
		if id == "" {
			continue
		}
		_, err := r.runCommand(ctx, "/ip/hotspot/active/remove", "=.id="+id)
		if err != nil {
			return fmt.Errorf("deauthorize client mac=%s: %w", mac, err)
		}
	}

	return nil
}

// DisconnectClient forcibly disconnects a client.
func (r *RouterOSClient) DisconnectClient(ctx context.Context, mac string) error {
	if err := r.ensureConnected(); err != nil {
		return err
	}

	mac = strings.ToLower(mac)

	// Remove from active users
	reply, err := r.runCommand(ctx, "/ip/hotspot/active/print", "?"+"mac-address="+mac)
	if err != nil {
		return fmt.Errorf("find active user by mac: %w", err)
	}

	for _, re := range reply.Re {
		id := re.Map[".id"]
		if id == "" {
			continue
		}
		_, err := r.runCommand(ctx, "/ip/hotspot/active/remove", "=.id="+id)
		if err != nil {
			return fmt.Errorf("disconnect client mac=%s: %w", mac, err)
		}
	}

	// Also remove from host table
	hostReply, err := r.runCommand(ctx, "/ip/hotspot/host/print", "?"+"mac-address="+mac)
	if err != nil {
		return fmt.Errorf("find host by mac: %w", err)
	}

	for _, re := range hostReply.Re {
		id := re.Map[".id"]
		if id == "" {
			continue
		}
		_, err := r.runCommand(ctx, "/ip/hotspot/host/remove", "=.id="+id)
		if err != nil {
			return fmt.Errorf("remove host mac=%s: %w", mac, err)
		}
	}

	return nil
}

// GetClientUsage returns data usage for a client.
func (r *RouterOSClient) GetClientUsage(ctx context.Context, mac string) (int64, int64, error) {
	if err := r.ensureConnected(); err != nil {
		return 0, 0, err
	}

	mac = strings.ToLower(mac)

	reply, err := r.runCommand(ctx, "/ip/hotspot/active/print", "?"+"mac-address="+mac)
	if err != nil {
		return 0, 0, fmt.Errorf("get client usage by mac: %w", err)
	}

	for _, re := range reply.Re {
		var bytesIn, bytesIn64, bytesOut, bytesOut64 int64
		fmt.Sscanf(re.Map["bytes-in"], "%d", &bytesIn)
		fmt.Sscanf(re.Map["bytes-in64"], "%d", &bytesIn64)
		fmt.Sscanf(re.Map["bytes-out"], "%d", &bytesOut)
		fmt.Sscanf(re.Map["bytes-out64"], "%d", &bytesOut64)

		if bytesIn64 > 0 {
			bytesIn = bytesIn64
		}
		if bytesOut64 > 0 {
			bytesOut = bytesOut64
		}

		return bytesIn, bytesOut, nil
	}

	return 0, 0, fmt.Errorf("client not found: %s", mac)
}

// ==================== Bandwidth Operations ====================

// CreateRule creates a simple queue bandwidth limitation rule.
func (r *RouterOSClient) CreateRule(ctx context.Context, rule *BandwidthRule) error {
	if err := r.ensureConnected(); err != nil {
		return err
	}

	_, err := r.runCommand(ctx,
		"/queue/simple/add",
		"=name="+rule.Name,
		"=target="+rule.TargetIP+"/32",
		"=max-limit="+rule.MaxLimit,
	)
	if err != nil {
		return fmt.Errorf("create bandwidth rule: %w", err)
	}

	return nil
}

// UpdateRule modifies an existing simple queue rule.
func (r *RouterOSClient) UpdateRule(ctx context.Context, rule *BandwidthRule) error {
	if err := r.ensureConnected(); err != nil {
		return err
	}

	_, err := r.runCommand(ctx,
		"/queue/simple/set",
		"=.id="+rule.ID,
		"=name="+rule.Name,
		"=target="+rule.TargetIP+"/32",
		"=max-limit="+rule.MaxLimit,
	)
	if err != nil {
		return fmt.Errorf("update bandwidth rule: %w", err)
	}

	return nil
}

// RemoveRule deletes a simple queue rule.
func (r *RouterOSClient) RemoveRule(ctx context.Context, ruleID string) error {
	if err := r.ensureConnected(); err != nil {
		return err
	}

	_, err := r.runCommand(ctx, "/queue/simple/remove", "=.id="+ruleID)
	if err != nil {
		return fmt.Errorf("remove bandwidth rule: %w", err)
	}

	return nil
}

// GetRules returns all simple queue rules.
func (r *RouterOSClient) GetRules(ctx context.Context) ([]*BandwidthRule, error) {
	if err := r.ensureConnected(); err != nil {
		return nil, err
	}

	reply, err := r.runCommand(ctx, "/queue/simple/print")
	if err != nil {
		return nil, fmt.Errorf("get bandwidth rules: %w", err)
	}

	var rules []*BandwidthRule
	for _, re := range reply.Re {
		enabled := re.Map["disabled"] != "true"
		rule := &BandwidthRule{
			ID:       re.Map[".id"],
			Name:     re.Map["name"],
			TargetIP: re.Map["target"],
			MaxLimit: re.Map["max-limit"],
			Enabled:  enabled,
		}
		rules = append(rules, rule)
	}

	return rules, nil
}

// ==================== System Operations ====================

// GetInfo returns system information from the router.
func (r *RouterOSClient) GetInfo(ctx context.Context) (*SystemInfo, error) {
	if err := r.ensureConnected(); err != nil {
		return nil, err
	}

	reply, err := r.runCommand(ctx, "/system/resource/print")
	if err != nil {
		return nil, fmt.Errorf("get system resource: %w", err)
	}

	if len(reply.Re) == 0 {
		return nil, fmt.Errorf("no system resource data returned")
	}

	re := reply.Re[0]
	info := &SystemInfo{
		Version:     re.Map["version"],
		Model:       re.Map["board-name"],
		Uptime:      re.Map["uptime"],
		TotalMemory: parseMemory(re.Map["total-memory"]),
		FreeMemory:  parseMemory(re.Map["free-memory"]),
	}

	if info.TotalMemory > 0 && info.FreeMemory > 0 {
		info.MemoryUsage = ((info.TotalMemory - info.FreeMemory) * 100) / info.TotalMemory
	}

	// Get CPU load
	fmt.Sscanf(re.Map["cpu-load"], "%d", &info.CPUUsage)

	// Get identity
	identityReply, err := r.runCommand(ctx, "/system/identity/print")
	if err == nil && len(identityReply.Re) > 0 {
		info.Identity = identityReply.Re[0].Map["name"]
	}

	return info, nil
}

// CheckHealth verifies connectivity to the router.
func (r *RouterOSClient) CheckHealth(ctx context.Context) *HealthStatus {
	status := &HealthStatus{
		LastCheck: time.Now(),
	}

	r.mu.Lock()
	connected := r.connected
	r.mu.Unlock()

	if !connected {
		status.Reachable = false
		status.Error = "not connected"
		return status
	}

	start := time.Now()
	_, err := r.runCommand(ctx, "/system/resource/print")
	latency := time.Since(start)

	status.Latency = latency
	if err != nil {
		status.Reachable = false
		status.Error = err.Error()
		r.mu.Lock()
		r.connected = false
		r.mu.Unlock()
	} else {
		status.Reachable = true
	}

	return status
}

// ==================== Helper Functions ====================

// parseMemory parses a memory string (e.g., "268435456") to bytes.
func parseMemory(s string) int {
	var mem int
	fmt.Sscanf(s, "%d", &mem)
	return mem
}
