package config

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

// Config holds all application configuration.
type Config struct {
	Server      ServerConfig      `mapstructure:"server"`
	Database    DatabaseConfig    `mapstructure:"database"`
	MikroTik    MikroTikConfig    `mapstructure:"mikrotik"`
	Auth        AuthConfig        `mapstructure:"auth"`
	Logging     LoggingConfig     `mapstructure:"logging"`
	Monitoring  MonitoringConfig  `mapstructure:"monitoring"`
	Discovery   DiscoveryConfig   `mapstructure:"discovery"`
	Enforcement EnforcementConfig `mapstructure:"enforcement"`
}

// ServerConfig holds HTTP server settings.
type ServerConfig struct {
	Host           string        `mapstructure:"host"`
	Port           int           `mapstructure:"port"`
	TLSCert        string        `mapstructure:"tls_cert"`
	TLSKey         string        `mapstructure:"tls_key"`
	ReadTimeout    time.Duration `mapstructure:"read_timeout"`
	WriteTimeout   time.Duration `mapstructure:"write_timeout"`
	IdleTimeout    time.Duration `mapstructure:"idle_timeout"`
	WebDir         string        `mapstructure:"web_dir"`
}

// DatabaseConfig holds database settings.
type DatabaseConfig struct {
	Path           string `mapstructure:"path"`
	BusyTimeout    int    `mapstructure:"busy_timeout_ms"`
	MaxOpenConns   int    `mapstructure:"max_open_conns"`
}

// MikroTikConfig holds MikroTik router connection settings.
type MikroTikConfig struct {
	Address  string        `mapstructure:"address"`
	Port     int           `mapstructure:"port"`
	Username string        `mapstructure:"username"`
	Password string        `mapstructure:"password"`
	UseTLS   bool          `mapstructure:"use_tls"`
	Timeout  time.Duration `mapstructure:"timeout"`
}

// AuthConfig holds authentication settings.
type AuthConfig struct {
	SessionTTL    time.Duration `mapstructure:"session_ttl"`
	TokenSecret   string        `mapstructure:"token_secret"`
}

// LoggingConfig holds logging settings.
type LoggingConfig struct {
	Level  string `mapstructure:"level"`
	Format string `mapstructure:"format"`
}

// MonitoringConfig holds health check settings.
type MonitoringConfig struct {
	Enabled bool   `mapstructure:"enabled"`
	Address string `mapstructure:"address"`
}

// DiscoveryConfig holds device discovery settings.
type DiscoveryConfig struct {
	Enabled  bool          `mapstructure:"enabled"`
	Interval time.Duration `mapstructure:"interval"`
}

// EnforcementConfig holds session enforcement settings.
type EnforcementConfig struct {
	Enabled  bool          `mapstructure:"enabled"`
	Interval time.Duration `mapstructure:"interval"`
}

// DefaultConfig returns a Config with sensible defaults.
func DefaultConfig() *Config {
	return &Config{
		Server: ServerConfig{
			Host:         "0.0.0.0",
			Port:         8080,
			ReadTimeout:  30 * time.Second,
			WriteTimeout: 60 * time.Second,
			IdleTimeout:  120 * time.Second,
			WebDir:       "./web/static",
		},
		Database: DatabaseConfig{
			Path:         "./data/unofi.db",
			BusyTimeout:  10000,
			MaxOpenConns: 1,
		},
		MikroTik: MikroTikConfig{
			Port:    8728,
			Timeout: 10 * time.Second,
		},
		Auth: AuthConfig{
			SessionTTL: 10 * time.Hour,
		},
		Logging: LoggingConfig{
			Level:  "info",
			Format: "text",
		},
		Monitoring: MonitoringConfig{
			Enabled: true,
			Address: ":6060",
		},
		Discovery: DiscoveryConfig{
			Enabled:  true,
			Interval: 5 * time.Second,
		},
		Enforcement: EnforcementConfig{
			Enabled:  true,
			Interval: 15 * time.Second,
		},
	}
}

// LoadFromEnv overrides config values from environment variables.
func (c *Config) LoadFromEnv() {
	if v := os.Getenv("UNOFI_SERVER_HOST"); v != "" {
		c.Server.Host = v
	}
	if v := os.Getenv("UNOFI_SERVER_PORT"); v != "" {
		if port, err := strconv.Atoi(v); err == nil {
			c.Server.Port = port
		}
	}
	if v := os.Getenv("UNOFI_DATABASE_PATH"); v != "" {
		c.Database.Path = v
	}
	if v := os.Getenv("UNOFI_MIKROTIK_ADDRESS"); v != "" {
		c.MikroTik.Address = v
	}
	if v := os.Getenv("UNOFI_MIKROTIK_PORT"); v != "" {
		if port, err := strconv.Atoi(v); err == nil {
			c.MikroTik.Port = port
		}
	}
	if v := os.Getenv("UNOFI_MIKROTIK_USERNAME"); v != "" {
		c.MikroTik.Username = v
	}
	if v := os.Getenv("UNOFI_MIKROTIK_PASSWORD"); v != "" {
		c.MikroTik.Password = v
	}
	if v := os.Getenv("UNOFI_MIKROTIK_TLS"); v == "1" || v == "true" {
		c.MikroTik.UseTLS = true
	}
	if v := os.Getenv("UNOFI_AUTH_SECRET"); v != "" {
		c.Auth.TokenSecret = v
	}
	if v := os.Getenv("UNOFI_LOG_LEVEL"); v != "" {
		c.Logging.Level = v
	}
	if v := os.Getenv("UNOFI_DISCOVERY_INTERVAL"); v != "" {
		if interval, err := time.ParseDuration(v); err == nil {
			c.Discovery.Interval = interval
		}
	}
	if v := os.Getenv("UNOFI_DISCOVERY_ENABLED"); v == "0" || v == "false" {
		c.Discovery.Enabled = false
	}
	if v := os.Getenv("UNOFI_ENFORCEMENT_INTERVAL"); v != "" {
		if interval, err := time.ParseDuration(v); err == nil {
			c.Enforcement.Interval = interval
		}
	}
	if v := os.Getenv("UNOFI_ENFORCEMENT_ENABLED"); v == "0" || v == "false" {
		c.Enforcement.Enabled = false
	}
}

// Validate checks that required configuration values are present.
func (c *Config) Validate() error {
	if c.Database.Path == "" {
		return fmt.Errorf("database path is required")
	}
	if c.MikroTik.Address != "" && c.MikroTik.Username == "" {
		return fmt.Errorf("mikrotik username is required when address is set")
	}
	if c.MikroTik.Address != "" && c.MikroTik.Password == "" {
		return fmt.Errorf("mikrotik password is required when address is set")
	}
	return nil
}

// BindAddress returns the full HTTP bind address.
func (s ServerConfig) BindAddress() string {
	return fmt.Sprintf("%s:%d", s.Host, s.Port)
}
