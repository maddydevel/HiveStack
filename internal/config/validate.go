package config

import (
	"fmt"
	"strings"
)

// Validate checks that the server configuration is valid.
// Returns a list of validation errors, or nil if valid.
func (c *ServerConfig) Validate() []error {
	var errs []error

	if c.Host == "" {
		errs = append(errs, fmt.Errorf("host is required"))
	}

	if c.Port <= 0 || c.Port > 65535 {
		errs = append(errs, fmt.Errorf("port must be between 1 and 65535, got %d", c.Port))
	}

	if c.Database.DSN == "" {
		errs = append(errs, fmt.Errorf("database.dsn is required"))
	}

	if c.Auth.JWTSecret == "" || c.Auth.JWTSecret == "change-me-in-production" {
		errs = append(errs, fmt.Errorf("auth.jwt_secret must be set to a secure value"))
	}

	if c.Auth.TokenExpiry == "" {
		errs = append(errs, fmt.Errorf("auth.token_expiry is required"))
	}

	if c.TLSEnabled {
		if c.TLSCertFile == "" {
			errs = append(errs, fmt.Errorf("tls_cert_file is required when tls_enabled is true"))
		}
		if c.TLSKeyFile == "" {
			errs = append(errs, fmt.Errorf("tls_key_file is required when tls_enabled is true"))
		}
	}

	return errs
}

// ValidateNode checks that the node configuration is valid.
func (c *NodeConfig) Validate() []error {
	var errs []error

	if c.ManagerAddress == "" {
		errs = append(errs, fmt.Errorf("manager_address is required"))
	}

	if c.NodeID == "" {
		errs = append(errs, fmt.Errorf("node_id is required"))
	}

	if c.LibvirtURI == "" {
		errs = append(errs, fmt.Errorf("libvirt_uri is required"))
	}

	if c.HeartbeatInterval == "" {
		errs = append(errs, fmt.Errorf("heartbeat_interval is required"))
	}

	return errs
}

// FormatErrors formats a list of validation errors as a single string.
func FormatErrors(errs []error) string {
	if len(errs) == 0 {
		return ""
	}
	parts := make([]string, len(errs))
	for i, err := range errs {
		parts[i] = "  - " + err.Error()
	}
	return strings.Join(parts, "\n")
}
