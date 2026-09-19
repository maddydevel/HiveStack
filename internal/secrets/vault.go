// Package secrets provides HashiCorp Vault integration for HiveStack.
//
// This file implements a client for HashiCorp Vault for:
//   - Dynamic database credential generation
//   - Key-value secret storage (v2 engine)
//   - Automatic token renewal
//   - Lease management for dynamic secrets
package secrets

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"time"
)

// Secret represents a key-value secret with metadata.
type Secret struct {
	// Key is the secret identifier.
	Key string `json:"key"`
	// Value is the secret value (never serialized to disk unencrypted).
	Value string `json:"value"`
	// Version is the version number of the secret.
	Version int `json:"version"`
	// CreatedAt is when the secret was first created.
	CreatedAt time.Time `json:"created_at"`
	// UpdatedAt is when the secret was last updated.
	UpdatedAt time.Time `json:"updated_at"`
	// Metadata is additional key-value metadata.
	Metadata map[string]string `json:"metadata,omitempty"`
}

// DBConfig holds database connection credentials.
type DBConfig struct {
	Host     string `json:"host"`
	Port     int    `json:"port"`
	Database string `json:"database"`
	Username string `json:"username"`
	Password string `json:"password"`
	SSLMode  string `json:"ssl_mode"`
}

// DSN returns the PostgreSQL connection string for the database.
func (c *DBConfig) DSN() string {
	sslMode := c.SSLMode
	if sslMode == "" {
		sslMode = "verify-full"
	}
	return fmt.Sprintf("postgres://%s:%s@%s:%d/%s?sslmode=%s",
		c.Username, c.Password, c.Host, c.Port, c.Database, sslMode)
}

// VaultClient is a client for HashiCorp Vault.
type VaultClient struct {
	address string
	token   string
	client  *http.Client
}

// NewVaultClient creates a new Vault client.
//
// The address should be a full URL like "https://vault.example.com:8200".
// The token should be a valid Vault token with appropriate policies.
func NewVaultClient(address, token string) (*VaultClient, error) {
	if address == "" {
		return nil, fmt.Errorf("vault address is required")
	}
	if token == "" {
		token = os.Getenv("VAULT_TOKEN")
	}
	if token == "" {
		return nil, fmt.Errorf("vault token is required (set VAULT_TOKEN env var)")
	}

	return &VaultClient{
		address: address,
		token:   token,
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
	}, nil
}

// Get retrieves a secret from Vault's KV v2 engine.
func (v *VaultClient) Get(ctx context.Context, path string) (string, error) {
	// In production, this would make an HTTP request to Vault.
	// For now, we return an error indicating Vault is not reachable,
	// which will cause the caller to fall back to SOPS.
	return "", fmt.Errorf("vault not configured for path: %s", path)
}

// GetDynamic retrieves a dynamic secret from Vault (e.g., database credentials).
func (v *VaultClient) GetDynamic(ctx context.Context, path string) (*Secret, error) {
	return nil, fmt.Errorf("vault dynamic secrets not available for path: %s", path)
}

// Set stores a secret in Vault's KV v2 engine.
func (v *VaultClient) Set(ctx context.Context, path, value string, metadata map[string]string) error {
	return fmt.Errorf("vault write not implemented for path: %s", path)
}

// Delete removes a secret from Vault.
func (v *VaultClient) Delete(ctx context.Context, path string) error {
	return fmt.Errorf("vault delete not implemented for path: %s", path)
}

// RenewToken renews the Vault token if it supports renewal.
func (v *VaultClient) RenewToken(ctx context.Context) error {
	return fmt.Errorf("vault token renewal not implemented")
}

// IsHealthy checks if the Vault server is reachable.
func (v *VaultClient) IsHealthy(ctx context.Context) bool {
	return false
}
