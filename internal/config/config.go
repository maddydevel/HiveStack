// Package config provides configuration loading for HiveStack components.
package config

import (
	"fmt"
	"os"

	"github.com/spf13/viper"
)

// ServerConfig holds the API server co...[truncated]
type ServerConfig struct {
	Host        string `mapstructure:"host"`
	Port        int    `mapstructure:"port"`
	TLSEnabled  bool   `mapstructure:"tls_enabled"`
	TLSCertFile string `mapstructure:"tls_cert_file"`
	TLSKeyFile  string `mapstructure:"tls_key_file"`
	// TLSCAFile is the CA bundle used to verify client certificates (optional).
	TLSCAFile string `mapstructure:"tls_ca_file"`
	// TLSDevMode generates a self-signed certificate instead of reading the
	// cert and key files. Development only; ignored unless TLSEnabled.
	TLSDevMode bool           `mapstructure:"tls_dev_mode"`
	Database   DatabaseConfig `mapstructure:"database"`
	Auth       AuthConfig     `mapstructure:"auth"`
}

// DatabaseConfig holds the database configuration.
type DatabaseConfig struct {
	DSN string `mapstructure:"dsn"`
}

// AuthConfig holds the authentication configuration.
type AuthConfig struct {
	JWTSecret   string `mapstructure:"jwt_secret"`
	TokenExpiry string `mapstructure:"token_expiry"`
}

// LoadNodeConfig loads node agent configuration from a YAML file.
func LoadNodeConfig(path string) (*NodeConfig, error) {
	v := viper.New()
	v.SetConfigFile(path)
	v.SetConfigType("yaml")
	v.SetDefault("manager_address", "hivestack-manager:8443")
	v.SetDefault("node_id", "")
	v.SetDefault("heartbeat_interval", "30s")
	v.SetDefault("libvirt_uri", "qemu:///system")
	if err := v.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); ok {
			return DefaultNodeConfig(), nil
		}
		return nil, fmt.Errorf("read node config: %w", err)
	}
	var cfg NodeConfig
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("unmarshal node config: %w", err)
	}
	if cfg.NodeID == "" {
		cfg.NodeID = fmt.Sprintf("node-%s", shortHostname())
	}
	return &cfg, nil
}

// DefaultNodeConfig returns sensible defaults for a node agent.
func DefaultNodeConfig() *NodeConfig {
	return &NodeConfig{
		ManagerAddress:    "hivestack-manager:8443",
		NodeID:            fmt.Sprintf("node-%s", shortHostname()),
		HeartbeatInterval: "30s",
		LibvirtURI:        "qemu:///system",
	}
}

// LoadServerConfig loads API server configuration from a YAML file.
func LoadServerConfig(path string) (*ServerConfig, error) {
	v := viper.New()
	v.SetConfigFile(path)
	v.SetConfigType("yaml")
	v.SetDefault("host", "0.0.0.0")
	v.SetDefault("port", 8080)
	v.SetDefault("tls_enabled", false)
	v.SetDefault("tls_dev_mode", false)
	v.SetDefault("database.dsn", "postgres://hivestack:hivestack@localhost:5432/hivestack?sslmode=disable")
	v.SetDefault("auth.jwt_secret", "change-me-in-production")
	v.SetDefault("auth.token_expiry", "24h")
	if err := v.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); ok {
			return DefaultServerConfig(), nil
		}
		return nil, fmt.Errorf("read server config: %w", err)
	}
	var cfg ServerConfig
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("unmarshal server config: %w", err)
	}
	return &cfg, nil
}

// DefaultServerConfig returns sensible defaults for the API server.
func DefaultServerConfig() *ServerConfig {
	return &ServerConfig{
		Host: "0.0.0.0",
		Port: 8080,
		Database: DatabaseConfig{
			DSN: "postgres://hivestack:hivestack@localhost:5432/hivestack?sslmode=disable",
		},
		Auth: AuthConfig{
			JWTSecret:   "change-me-in-production",
			TokenExpiry: "24h",
		},
	}
}

// shortHostname returns a short hostname for node identification.
func shortHostname() string {
	hn, err := os.Hostname()
	if err != nil {
		return "unknown"
	}
	if len(hn) > 8 {
		hn = hn[:8]
	}
	return hn
}
