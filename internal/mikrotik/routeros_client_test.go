package mikrotik

import (
	"testing"
	"time"
)

func TestNewRouterOSClient(t *testing.T) {
	config := Config{
		Address:  "192.168.1.1",
		Port:     8728,
		Username: "admin",
		Password: "secret",
		Timeout:  10 * time.Second,
	}

	client := NewRouterOSClient(config)
	if client == nil {
		t.Fatal("NewRouterOSClient returned nil")
	}
	if client.IsConnected() {
		t.Error("new client should not be connected")
	}
	if client.config.Address != "192.168.1.1" {
		t.Errorf("expected address 192.168.1.1, got %s", client.config.Address)
	}
}

func TestRouterOSClientNotConnected(t *testing.T) {
	config := Config{
		Address:  "192.168.1.1",
		Port:     8728,
		Username: "admin",
		Password: "secret",
	}

	client := NewRouterOSClient(config)

	// Health should report not connected without trying to dial
	health := client.CheckHealth(nil)
	if health.Reachable {
		t.Error("health should report unreachable for new client")
	}
	if health.Error != "not connected" {
		t.Errorf("expected 'not connected' error, got %q", health.Error)
	}

	// IsConnected should return false
	if client.IsConnected() {
		t.Error("new client should not be connected")
	}
}

func TestRouterOSClientConnectionState(t *testing.T) {
	config := Config{
		Address:  "192.168.1.1",
		Port:     8728,
		Username: "admin",
		Password: "secret",
	}

	client := NewRouterOSClient(config)

	// Initially not connected
	if client.IsConnected() {
		t.Error("new client should not be connected")
	}

	// Health should report not connected
	health := client.CheckHealth(nil)
	if health.Reachable {
		t.Error("health should report unreachable for new client")
	}
	if health.Error != "not connected" {
		t.Errorf("expected 'not connected' error, got %q", health.Error)
	}
}

func TestParseMemory(t *testing.T) {
	tests := []struct {
		input    string
		expected int
	}{
		{"268435456", 268435456},
		{"134217728", 134217728},
		{"0", 0},
		{"invalid", 0},
		{"", 0},
	}

	for _, tt := range tests {
		got := parseMemory(tt.input)
		if got != tt.expected {
			t.Errorf("parseMemory(%q) = %d, want %d", tt.input, got, tt.expected)
		}
	}
}
