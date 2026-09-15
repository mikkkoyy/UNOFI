package config

import (
	"os"
	"testing"
	"time"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	if cfg.Server.Port != 8080 {
		t.Errorf("expected default port 8080, got %d", cfg.Server.Port)
	}
	if cfg.Database.Path != "./data/unofi.db" {
		t.Errorf("expected default db path, got %s", cfg.Database.Path)
	}
	if cfg.Auth.SessionTTL != 10*time.Hour {
		t.Errorf("expected 10h session TTL, got %v", cfg.Auth.SessionTTL)
	}
	if cfg.Logging.Level != "info" {
		t.Errorf("expected info log level, got %s", cfg.Logging.Level)
	}
}

func TestLoadFromEnv(t *testing.T) {
	cfg := DefaultConfig()

	// Set environment variables
	os.Setenv("UNOFI_SERVER_PORT", "9090")
	os.Setenv("UNOFI_DATABASE_PATH", "/tmp/test.db")
	os.Setenv("UNOFI_LOG_LEVEL", "debug")
	defer os.Unsetenv("UNOFI_SERVER_PORT")
	defer os.Unsetenv("UNOFI_DATABASE_PATH")
	defer os.Unsetenv("UNOFI_LOG_LEVEL")

	cfg.LoadFromEnv()

	if cfg.Server.Port != 9090 {
		t.Errorf("expected port 9090, got %d", cfg.Server.Port)
	}
	if cfg.Database.Path != "/tmp/test.db" {
		t.Errorf("expected /tmp/test.db, got %s", cfg.Database.Path)
	}
	if cfg.Logging.Level != "debug" {
		t.Errorf("expected debug log level, got %s", cfg.Logging.Level)
	}
}

func TestValidate(t *testing.T) {
	tests := []struct {
		name    string
		cfg     func() *Config
		wantErr bool
	}{
		{
			name: "valid config",
			cfg: func() *Config {
				c := DefaultConfig()
				return c
			},
			wantErr: false,
		},
		{
			name: "missing database path",
			cfg: func() *Config {
				c := DefaultConfig()
				c.Database.Path = ""
				return c
			},
			wantErr: true,
		},
		{
			name: "mikrotik without username",
			cfg: func() *Config {
				c := DefaultConfig()
				c.MikroTik.Address = "192.168.1.1"
				return c
			},
			wantErr: true,
		},
		{
			name: "mikrotik without password",
			cfg: func() *Config {
				c := DefaultConfig()
				c.MikroTik.Address = "192.168.1.1"
				c.MikroTik.Username = "admin"
				return c
			},
			wantErr: true,
		},
		{
			name: "valid mikrotik config",
			cfg: func() *Config {
				c := DefaultConfig()
				c.MikroTik.Address = "192.168.1.1"
				c.MikroTik.Username = "admin"
				c.MikroTik.Password = "secret"
				return c
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.cfg().Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestBindAddress(t *testing.T) {
	s := ServerConfig{Host: "0.0.0.0", Port: 8080}
	if got := s.BindAddress(); got != "0.0.0.0:8080" {
		t.Errorf("expected 0.0.0.0:8080, got %s", got)
	}
}
