package mikrotik

import (
	"context"
	"testing"
)

func TestMockRouterAuthorizeClient(t *testing.T) {
	router := NewMockRouter()
	ctx := context.Background()

	err := router.AuthorizeClient(ctx, "AA:BB:CC:DD:EE:FF", "10.0.0.5")
	if err != nil {
		t.Fatalf("AuthorizeClient error: %v", err)
	}

	client, err := router.GetClientByMAC(ctx, "AA:BB:CC:DD:EE:FF")
	if err != nil {
		t.Fatalf("GetClientByMAC error: %v", err)
	}
	if client == nil {
		t.Fatal("client should be found")
	}
	if client.State != StateAuthorized {
		t.Errorf("expected state %s, got %s", StateAuthorized, client.State)
	}
	if client.IP != "10.0.0.5" {
		t.Errorf("expected IP 10.0.0.5, got %s", client.IP)
	}
}

func TestMockRouterDeauthorizeClient(t *testing.T) {
	router := NewMockRouter()
	ctx := context.Background()

	_ = router.AuthorizeClient(ctx, "AA:BB:CC:DD:EE:FF", "10.0.0.5")
	err := router.DeauthorizeClient(ctx, "AA:BB:CC:DD:EE:FF")
	if err != nil {
		t.Fatalf("DeauthorizeClient error: %v", err)
	}

	client, _ := router.GetClientByMAC(ctx, "AA:BB:CC:DD:EE:FF")
	if client.State != StateUnauthorized {
		t.Errorf("expected state %s, got %s", StateUnauthorized, client.State)
	}
}

func TestMockRouterDisconnectClient(t *testing.T) {
	router := NewMockRouter()
	ctx := context.Background()

	_ = router.AuthorizeClient(ctx, "AA:BB:CC:DD:EE:FF", "10.0.0.5")
	err := router.DisconnectClient(ctx, "AA:BB:CC:DD:EE:FF")
	if err != nil {
		t.Fatalf("DisconnectClient error: %v", err)
	}

	client, _ := router.GetClientByMAC(ctx, "AA:BB:CC:DD:EE:FF")
	if client != nil {
		t.Error("client should be nil after disconnect")
	}
}

func TestMockRouterGetClients(t *testing.T) {
	router := NewMockRouter()
	ctx := context.Background()

	_ = router.AuthorizeClient(ctx, "AA:BB:CC:DD:EE:FF", "10.0.0.5")
	_ = router.AuthorizeClient(ctx, "11:22:33:44:55:66", "10.0.0.6")

	clients, err := router.GetClients(ctx)
	if err != nil {
		t.Fatalf("GetClients error: %v", err)
	}
	if len(clients) != 2 {
		t.Errorf("expected 2 clients, got %d", len(clients))
	}
}

func TestMockRouterBandwidthRules(t *testing.T) {
	router := NewMockRouter()
	ctx := context.Background()

	rule := &BandwidthRule{
		Name:     "limit-10.0.0.5",
		TargetIP: "10.0.0.5",
		MaxLimit: "10M",
		Enabled:  true,
	}

	err := router.CreateRule(ctx, rule)
	if err != nil {
		t.Fatalf("CreateRule error: %v", err)
	}

	rules, err := router.GetRules(ctx)
	if err != nil {
		t.Fatalf("GetRules error: %v", err)
	}
	if len(rules) != 1 {
		t.Errorf("expected 1 rule, got %d", len(rules))
	}

	// Update rule
	rule.MaxLimit = "20M"
	err = router.UpdateRule(ctx, rule)
	if err != nil {
		t.Fatalf("UpdateRule error: %v", err)
	}

	// Remove rule
	err = router.RemoveRule(ctx, rule.ID)
	if err != nil {
		t.Fatalf("RemoveRule error: %v", err)
	}

	rules, _ = router.GetRules(ctx)
	if len(rules) != 0 {
		t.Errorf("expected 0 rules after removal, got %d", len(rules))
	}
}

func TestMockRouterGetInfo(t *testing.T) {
	router := NewMockRouter()
	ctx := context.Background()

	info, err := router.GetInfo(ctx)
	if err != nil {
		t.Fatalf("GetInfo error: %v", err)
	}
	if info.Identity != "MockRouter" {
		t.Errorf("expected MockRouter, got %s", info.Identity)
	}
}

func TestMockRouterCheckHealth(t *testing.T) {
	router := NewMockRouter()
	ctx := context.Background()

	health := router.CheckHealth(ctx)
	if !health.Reachable {
		t.Error("mock router should be reachable by default")
	}

	// Set unreachable
	router.SetReachable(false)
	health = router.CheckHealth(ctx)
	if health.Reachable {
		t.Error("mock router should be unreachable after SetReachable(false)")
	}

	// Operations should fail when unreachable
	err := router.AuthorizeClient(ctx, "AA:BB:CC:DD:EE:FF", "10.0.0.5")
	if err == nil {
		t.Error("expected error when router is unreachable")
	}
}

func TestMockRouterGetClientUsage(t *testing.T) {
	router := NewMockRouter()
	ctx := context.Background()

	_ = router.AuthorizeClient(ctx, "AA:BB:CC:DD:EE:FF", "10.0.0.5")

	in, out, err := router.GetClientUsage(ctx, "AA:BB:CC:DD:EE:FF")
	if err != nil {
		t.Fatalf("GetClientUsage error: %v", err)
	}
	if in != 0 || out != 0 {
		t.Errorf("expected 0,0 usage initially, got %d,%d", in, out)
	}
}

func TestConfigValidate(t *testing.T) {
	tests := []struct {
		name    string
		config  Config
		wantErr bool
	}{
		{
			name:    "empty config",
			config:  Config{},
			wantErr: true,
		},
		{
			name:    "missing port",
			config:  Config{Address: "192.168.1.1", Username: "admin", Password: "secret"},
			wantErr: true,
		},
		{
			name:    "missing username",
			config:  Config{Address: "192.168.1.1", Port: 8728, Password: "secret"},
			wantErr: true,
		},
		{
			name:    "missing password",
			config:  Config{Address: "192.168.1.1", Port: 8728, Username: "admin"},
			wantErr: true,
		},
		{
			name:    "valid config",
			config:  Config{Address: "192.168.1.1", Port: 8728, Username: "admin", Password: "secret"},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
