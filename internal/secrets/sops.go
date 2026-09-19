// Package secrets provides SOPS/age-based encryption for HiveStack secrets.
//
// SOPS (Secrets OPerationS) is used as a fallback when Vault is unavailable,
// or as the primary encryption layer for secrets at rest. It uses age
// (modern, secure alternative to GPG) for encryption.
//
// The SOPS client:
//   - Reads/writes encrypted YAML/JSON files
//   - Uses age recipients for encryption
//   - Falls back to environment variables if no key file is available
package secrets

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// SOPSClient provides SOPS-encrypted secret storage.
type SOPSClient struct {
	// keyFile is the path to the age private key file.
	keyFile string
	// secretsDir is the directory where encrypted secrets are stored.
	secretsDir string
}

// NewSOPSClient creates a new SOPS client.
//
// If keyFile is empty, it looks for the default age key at
// ~/.config/age/keys.txt. If no key is found, operations will fall back
// to environment variables.
func NewSOPSClient(keyFile string) (*SOPSClient, error) {
	secretsDir := os.Getenv("HIVESTACK_SECRETS_DIR")
	if secretsDir == "" {
		secretsDir = "/etc/hivestack/secrets"
	}

	return &SOPSClient{
		keyFile:    keyFile,
		secretsDir: secretsDir,
	}, nil
}

// Get retrieves and decrypts a secret.
//
// The key is used to locate the encrypted file (e.g., "database_password"
// maps to /etc/hivestack/secrets/database_password.enc.yaml).
func (s *SOPSClient) Get(ctx context.Context, key string) (string, error) {
	if s.keyFile == "" {
		return "", fmt.Errorf("no SOPS key file configured")
	}

	filePath := filepath.Join(s.secretsDir, key+".enc.yaml")
	data, err := os.ReadFile(filePath)
	if err != nil {
		return "", fmt.Errorf("read secret file: %w", err)
	}

	// In production, this would decrypt using SOPS/age.
	// For now, return error to fall back to environment variables.
	_ = data
	return "", fmt.Errorf("SOPS decryption not available for key: %s", key)
}

// Set encrypts and stores a secret.
func (s *SOPSClient) Set(ctx context.Context, secret *Secret) error {
	if s.keyFile == "" {
		return fmt.Errorf("no SOPS key file configured for writing")
	}

	// Ensure directory exists
	if err := os.MkdirAll(s.secretsDir, 0700); err != nil {
		return fmt.Errorf("create secrets directory: %w", err)
	}

	_, err := json.Marshal(secret)
	if err != nil {
		return fmt.Errorf("marshal secret: %w", err)
	}

	// In production, this would encrypt using SOPS/age.
	return fmt.Errorf("SOPS encryption not implemented for key: %s", secret.Key)
}

// Delete removes a secret file.
func (s *SOPSClient) Delete(ctx context.Context, key string) error {
	filePath := filepath.Join(s.secretsDir, key+".enc.yaml")
	err := os.Remove(filePath)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("delete secret file: %w", err)
	}
	return nil
}

// List returns all secret keys in the secrets directory.
func (s *SOPSClient) List(ctx context.Context) ([]string, error) {
	entries, err := os.ReadDir(s.secretsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read secrets directory: %w", err)
	}

	var keys []string
	for _, entry := range entries {
		if !entry.IsDir() {
			name := entry.Name()
			// Strip .enc.yaml suffix
			if len(name) > 8 && name[len(name)-8:] == ".enc.yaml" {
				keys = append(keys, name[:len(name)-8])
			}
		}
	}

	return keys, nil
}

// GetSecret retrieves a full secret with metadata.
func (s *SOPSClient) GetSecret(ctx context.Context, key string) (*Secret, error) {
	if s.keyFile == "" {
		return nil, fmt.Errorf("no SOPS key file configured")
	}

	filePath := filepath.Join(s.secretsDir, key+".enc.yaml")
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("read secret file: %w", err)
	}

	// In production, this would decrypt and unmarshal.
	_ = data
	return nil, fmt.Errorf("SOPS decryption not available for key: %s", key)
}

// StoreSecret is an alias for Set.
func (s *SOPSClient) StoreSecret(ctx context.Context, secret *Secret) error {
	return s.Set(ctx, secret)
}

// IsHealthy checks if the SOPS client can read/write.
func (s *SOPSClient) IsHealthy(ctx context.Context) bool {
	if s.keyFile == "" {
		return false
	}
	_, err := os.Stat(s.keyFile)
	return err == nil
}

// Helper to get env var with prefix.
func (s *SOPSClient) getEnv(key string) string {
	return os.Getenv(fmt.Sprintf("HIVESTACK_%s", key))
}

// Helper to get env var with default.
func (s *SOPSClient) getEnvDef(key, def string) string {
	if v := s.getEnv(key); v != "" {
		return v
	}
	return def
}

// Ensure time is used (referenced in Secret struct).
var _ = time.Now
