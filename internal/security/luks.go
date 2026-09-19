// Package security provides LUKS disk encryption configuration for HiveStack.
//
// LUKS (Linux Unified Key Setup) is the standard for disk encryption on Linux.
// On SLES 15 SP7, it's used to encrypt:
//   - VM storage volumes (data at rest)
//   - Backup storage
//   - Swap partitions
//
// This file provides configuration and management utilities for LUKS-encrypted
// volumes. The actual cryptsetup operations are performed by the system's
// cryptsetup tool.
package security

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"context"
)

// LUKSConfig holds LUKS encryption configuration for a volume.
type LUKSConfig struct {
	// Device is the block device to encrypt (e.g., /dev/sdb).
	Device string `json:"device"`
	// Name is the mapper name (e.g., "hivestack-vm-data").
	Name string `json:"name"`
	// Cipher is the encryption cipher (default: aes-xts-plain64).
	Cipher string `json:"cipher"`
	// KeySize is the key size in bits (default: 512 for XTS).
	KeySize int `json:"key_size"`
	// Hash is the hash for the key derivation (default: sha256).
	Hash string `json:"hash"`
	// KeyFile is the path to the key file (if not using passphrase).
	KeyFile string `json:"key_file,omitempty"`
	// MountPoint is where to open/mount the volume.
	MountPoint string `json:"mount_point"`
	// Filesystem is the filesystem type (default: xfs).
	Filesystem string `json:"filesystem"`
}

// DefaultLUKSConfig returns a LUKSConfig with secure defaults.
func DefaultLUKSConfig(device, name, mountPoint string) *LUKSConfig {
	return &LUKSConfig{
		Device:     device,
		Name:       name,
		Cipher:     "aes-xts-plain64",
		KeySize:    512,
		Hash:       "sha256",
		MountPoint: mountPoint,
		Filesystem: "xfs",
	}
}

// Manager manages LUKS-encrypted volumes.
type Manager struct {
	// keyStore is where encryption keys are stored.
	keyStore string
}

// NewManager creates a new LUKS manager.
func NewManager(keyStore string) *Manager {
	if keyStore == "" {
		keyStore = "/etc/hivestack/luks-keys"
	}
	return &Manager{keyStore: keyStore}
}

// CreateVolume creates a new LUKS-encrypted volume.
//
// Steps:
//  1. Generate a random key file
//  2. Format the device with LUKS
//  3. Open (unlock) the LUKS volume
//  4. Create a filesystem
//  5. Mount the volume
func (m *Manager) CreateVolume(ctx context.Context, config *LUKSConfig) error {
	// Generate key file
	keyFile, err := m.generateKeyFile(config.Name)
	if err != nil {
		return fmt.Errorf("generate key file: %w", err)
	}
	config.KeyFile = keyFile

	// Format with LUKS
	if err := m.luksFormat(ctx, config); err != nil {
		return fmt.Errorf("luks format: %w", err)
	}

	// Open the volume
	if err := m.luksOpen(ctx, config); err != nil {
		return fmt.Errorf("luks open: %w", err)
	}

	// Create filesystem
	if err := m.createFilesystem(ctx, config); err != nil {
		return fmt.Errorf("create filesystem: %w", err)
	}

	// Mount
	if err := m.mount(ctx, config); err != nil {
		return fmt.Errorf("mount: %w", err)
	}

	return nil
}

// OpenVolume unlocks and mounts an existing LUKS volume.
func (m *Manager) OpenVolume(ctx context.Context, config *LUKSConfig) error {
	if err := m.luksOpen(ctx, config); err != nil {
		return fmt.Errorf("luks open: %w", err)
	}
	return m.mount(ctx, config)
}

// CloseVolume unmounts and locks a LUKS volume.
func (m *Manager) CloseVolume(ctx context.Context, config *LUKSConfig) error {
	// Unmount
	exec.CommandContext(ctx, "umount", config.MountPoint).Run()
	return m.luksClose(ctx, config)
}

// generateKeyFile creates a random key file for LUKS encryption.
func (m *Manager) generateKeyFile(name string) (string, error) {
	if err := os.MkdirAll(m.keyStore, 0700); err != nil {
		return "", fmt.Errorf("create key store: %w", err)
	}

	// Generate 64 bytes (512 bits) of random key material
	key := make([]byte, 64)
	if _, err := rand.Read(key); err != nil {
		return "", fmt.Errorf("generate key: %w", err)
	}

	keyFile := filepath.Join(m.keyStore, name+".key")
	// Write as hex-encoded for cryptsetup compatibility
	hexKey := hex.EncodeToString(key)
	if err := os.WriteFile(keyFile, []byte(hexKey), 0400); err != nil {
		return "", fmt.Errorf("write key file: %w", err)
	}

	return keyFile, nil
}

// luksFormat formats a device with LUKS encryption.
func (m *Manager) luksFormat(ctx context.Context, config *LUKSConfig) error {
	args := []string{
		"luksFormat",
		"--type", "luks2",
		"--cipher", config.Cipher,
		"--key-size", fmt.Sprintf("%d", config.KeySize),
		"--hash", config.Hash,
		"--key-file", config.KeyFile,
		"--batch-mode",
		config.Device,
	}
	cmd := exec.CommandContext(ctx, "cryptsetup", args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("luksFormat %s: %w: %s", config.Device, err, strings.TrimSpace(string(out)))
	}
	return nil
}

// luksOpen unlocks a LUKS volume.
func (m *Manager) luksOpen(ctx context.Context, config *LUKSConfig) error {
	args := []string{
		"open",
		"--type", "luks2",
		"--key-file", config.KeyFile,
		config.Device,
		config.Name,
	}
	cmd := exec.CommandContext(ctx, "cryptsetup", args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("luksOpen %s: %w: %s", config.Device, err, strings.TrimSpace(string(out)))
	}
	return nil
}

// luksClose locks a LUKS volume.
func (m *Manager) luksClose(ctx context.Context, config *LUKSConfig) error {
	cmd := exec.CommandContext(ctx, "cryptsetup", "close", config.Name)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("luksClose %s: %w: %s", config.Name, err, strings.TrimSpace(string(out)))
	}
	return nil
}

// createFilesystem creates a filesystem on the mapped device.
func (m *Manager) createFilesystem(ctx context.Context, config *LUKSConfig) error {
	fs := config.Filesystem
	if fs == "" {
		fs = "xfs"
	}
	device := "/dev/mapper/" + config.Name
	cmd := exec.CommandContext(ctx, "mkfs."+fs, device)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("mkfs.%s %s: %w: %s", fs, device, err, strings.TrimSpace(string(out)))
	}
	return nil
}

// mount mounts the mapped device.
func (m *Manager) mount(ctx context.Context, config *LUKSConfig) error {
	if err := os.MkdirAll(config.MountPoint, 0755); err != nil {
		return fmt.Errorf("create mount point: %w", err)
	}
	device := "/dev/mapper/" + config.Name
	cmd := exec.CommandContext(ctx, "mount", device, config.MountPoint)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("mount %s %s: %w: %s", device, config.MountPoint, err, strings.TrimSpace(string(out)))
	}
	return nil
}

// IsLUKS checks if a device is already formatted with LUKS.
func IsLUKS(device string) bool {
	cmd := exec.Command("cryptsetup", "isLuks", device)
	return cmd.Run() == nil
}

// IsActive checks if a LUKS volume is currently open.
func IsActive(name string) bool {
	_, err := os.Stat("/dev/mapper/" + name)
	return err == nil
}
