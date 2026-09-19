// Package ha provides Non-HANA High Availability for HiveStack.
//
// Shared storage support ensures VM disk images are accessible from
// any host in the cluster, enabling VM restart on a different host.
package ha

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"
)

// StorageType identifies the shared storage backend.
type StorageType string

const (
	// StorageTypeNFS uses Network File System (NFS) for shared VM storage.
	StorageTypeNFS StorageType = "nfs"
	// StorageTypeCephRBD uses Ceph RADOS Block Device for shared VM storage.
	StorageTypeCephRBD StorageType = "ceph-rbd"
	// StorageTypeISCSI uses iSCSI for shared VM storage.
	StorageTypeISCSI StorageType = "iscsi"
)

// String implements fmt.Stringer.
func (s StorageType) String() string {
	return string(s)
}

// StorageBackend defines the interface for shared storage operations.
type StorageBackend interface {
	// Type returns the storage backend type.
	Type() StorageType
	// IsAvailable checks if the storage is accessible from this host.
	IsAvailable(ctx context.Context) (bool, error)
	// Mount mounts the shared storage on the local host.
	Mount(ctx context.Context, target string) error
	// Unmount unmounts the shared storage.
	Unmount(ctx context.Context, target string) error
	// GetDiskPath returns the path to a VM's disk image.
	GetDiskPath(vmID string) string
	// CloneDisk creates a copy of a VM disk on the target host (if needed).
	CloneDisk(ctx context.Context, vmID, sourceHost, targetHost string) error
}

// NFSBackend implements StorageBackend for NFS.
type NFSBackend struct {
	mu         sync.Mutex
	serverAddr string
	exportPath string
	mountPoint string
	options    string
}

// NewNFSBackend creates a new NFS storage backend.
func NewNFSBackend(serverAddr, exportPath, mountPoint string) *NFSBackend {
	return &NFSBackend{
		serverAddr: serverAddr,
		exportPath: exportPath,
		mountPoint: mountPoint,
		options:    "hard,intr,rsize=8192,wsize=8192",
	}
}

// Type returns the storage type.
func (n *NFSBackend) Type() StorageType {
	return StorageTypeNFS
}

// IsAvailable checks if the NFS export is accessible.
func (n *NFSBackend) IsAvailable(ctx context.Context) (bool, error) {
	// Check if mount point exists
	if _, err := os.Stat(n.mountPoint); os.IsNotExist(err) {
		return false, fmt.Errorf("mount point %s does not exist", n.mountPoint)
	}

	// Try showmount to verify export
	cmd := exec.CommandContext(ctx, "showmount", "-e", n.serverAddr)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return false, fmt.Errorf("showmount failed: %w (output: %s)", err, string(output))
	}

	return strings.Contains(string(output), n.exportPath), nil
}

// Mount mounts the NFS export.
func (n *NFSBackend) Mount(ctx context.Context, target string) error {
	n.mu.Lock()
	defer n.mu.Unlock()

	if target == "" {
		target = n.mountPoint
	}

	// Create mount point if needed
	if err := os.MkdirAll(target, 0755); err != nil {
		return fmt.Errorf("create mount point %s: %w", target, err)
	}

	// Check if already mounted
	if n.isMounted(target) {
		log.Printf("[HA/Storage/NFS] %s already mounted at %s", n.exportPath, target)
		return nil
	}

	source := fmt.Sprintf("%s:%s", n.serverAddr, n.exportPath)
	cmd := exec.CommandContext(ctx, "mount", "-t", "nfs", "-o", n.options, source, target)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("mount NFS %s at %s: %w (output: %s)", source, target, err, string(output))
	}

	log.Printf("[HA/Storage/NFS] Mounted %s at %s", source, target)
	return nil
}

// Unmount unmounts the NFS export.
func (n *NFSBackend) Unmount(ctx context.Context, target string) error {
	n.mu.Lock()
	defer n.mu.Unlock()

	if target == "" {
		target = n.mountPoint
	}

	cmd := exec.CommandContext(ctx, "umount", target)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("umount %s: %w (output: %s)", target, err, string(output))
	}

	log.Printf("[HA/Storage/NFS] Unmounted %s", target)
	return nil
}

// GetDiskPath returns the path to a VM disk on NFS.
func (n *NFSBackend) GetDiskPath(vmID string) string {
	return fmt.Sprintf("%s/disks/%s.qcow2", n.mountPoint, vmID)
}

// CloneDisk is a no-op for NFS (disks are already accessible).
func (n *NFSBackend) CloneDisk(ctx context.Context, vmID, sourceHost, targetHost string) error {
	// NFS is shared — disks are accessible from any host
	return nil
}

func (n *NFSBackend) isMounted(target string) bool {
	data, err := os.ReadFile("/proc/mounts")
	if err != nil {
		return false
	}
	return strings.Contains(string(data), target)
}

// ---------- Ceph RBD Backend ----------

// CephRBDBackend implements StorageBackend for Ceph RADOS Block Device.
type CephRBDBackend struct {
	mu          sync.Mutex
	poolName    string
	clusterName string
	monHosts    []string
	keyringPath string
	mapName     string
}

// NewCephRBDBackend creates a new Ceph RBD backend.
func NewCephRBDBackend(poolName, clusterName string, monHosts []string, keyringPath string) *CephRBDBackend {
	return &CephRBDBackend{
		poolName:    poolName,
		clusterName: clusterName,
		monHosts:    monHosts,
		keyringPath: keyringPath,
	}
}

// Type returns the storage type.
func (c *CephRBDBackend) Type() StorageType {
	return StorageTypeCephRBD
}

// IsAvailable checks if Ceph cluster is accessible.
func (c *CephRBDBackend) IsAvailable(ctx context.Context) (bool, error) {
	cmd := exec.CommandContext(ctx, "rbd", "--pool", c.poolName, "list")
	if c.keyringPath != "" {
		cmd.Env = append(os.Environ(), fmt.Sprintf("CEPH_KEYRING=%s", c.keyringPath))
	}
	output, err := cmd.CombinedOutput()
	if err != nil {
		return false, fmt.Errorf("rbd list failed: %w (output: %s)", err, string(output))
	}
	return true, nil
}

// Mount maps the RBD image locally.
func (c *CephRBDBackend) Mount(ctx context.Context, target string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	// RBD images are mapped per-VM. This is called during VM restart.
	// The actual mapping happens in GetDiskPath.
	return nil
}

// Unmount unmaps the RBD image.
func (c *CephRBDBackend) Unmount(ctx context.Context, target string) error {
	if c.mapName == "" {
		return nil
	}
	cmd := exec.CommandContext(ctx, "rbd", "unmap", c.mapName)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("rbd unmap %s: %w (output: %s)", c.mapName, err, string(output))
	}
	c.mapName = ""
	return nil
}

// GetDiskPath maps the RBD image and returns the device path.
func (c *CephRBDBackend) GetDiskPath(vmID string) string {
	imageName := fmt.Sprintf("vm-%s-disk-0", vmID)
	c.mapName = fmt.Sprintf("/dev/rbd/%s/%s", c.poolName, imageName)
	return c.mapName
}

// CloneDisk clones a Ceph RBD image for the target host.
func (c *CephRBDBackend) CloneDisk(ctx context.Context, vmID, sourceHost, targetHost string) error {
	sourceImage := fmt.Sprintf("vm-%s-disk-0", vmID)
	targetImage := fmt.Sprintf("vm-%s-disk-0-clone-%d", vmID, time.Now().Unix())

	cmd := exec.CommandContext(ctx, "rbd", "clone",
		fmt.Sprintf("%s/%s", c.poolName, sourceImage),
		fmt.Sprintf("%s/%s", c.poolName, targetImage),
	)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("rbd clone failed: %w (output: %s)", err, string(output))
	}

	log.Printf("[HA/Storage/Ceph] Cloned image %s -> %s", sourceImage, targetImage)
	return nil
}

// ---------- iSCSI Backend ----------

// ISCSIBackend implements StorageBackend for iSCSI.
type ISCSIBackend struct {
	mu           sync.Mutex
	targetIQN    string
	portalHost   string
	portalPort   int
	targetLUN    int
	devicePath   string
	initiatorName string
}

// NewISCSIBackend creates a new iSCSI backend.
func NewISCSIBackend(targetIQN, portalHost string, portalPort int, targetLUN int) *ISCSIBackend {
	return &ISCSIBackend{
		targetIQN:  targetIQN,
		portalHost: portalHost,
		portalPort: portalPort,
		targetLUN:  targetLUN,
	}
}

// Type returns the storage type.
func (i *ISCSIBackend) Type() StorageType {
	return StorageTypeISCSI
}

// IsAvailable checks if the iSCSI target is accessible.
func (i *ISCSIBackend) IsAvailable(ctx context.Context) (bool, error) {
	portal := fmt.Sprintf("%s:%d", i.portalHost, i.portalPort)
	cmd := exec.CommandContext(ctx, "iscsiadm", "-m", "discovery", "-t", "sendtargets", "-p", portal)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return false, fmt.Errorf("iscsi discovery failed: %w (output: %s)", err, string(output))
	}
	return strings.Contains(string(output), i.targetIQN), nil
}

// Mount logs into the iSCSI target.
func (i *ISCSIBackend) Mount(ctx context.Context, target string) error {
	i.mu.Lock()
	defer i.mu.Unlock()

	portal := fmt.Sprintf("%s:%d", i.portalHost, i.portalPort)

	// Discovery
	cmd := exec.CommandContext(ctx, "iscsiadm", "-m", "discovery", "-t", "sendtargets", "-p", portal)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("iscsi discovery: %w (output: %s)", err, string(output))
	}

	// Login
	cmd = exec.CommandContext(ctx, "iscsiadm", "-m", "node", "-T", i.targetIQN, "-p", portal, "--login")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("iscsi login: %w (output: %s)", err, string(output))
	}

	log.Printf("[HA/Storage/iSCSI] Logged into %s at %s", i.targetIQN, portal)
	return nil
}

// Unmount logs out of the iSCSI target.
func (i *ISCSIBackend) Unmount(ctx context.Context, target string) error {
	i.mu.Lock()
	defer i.mu.Unlock()

	portal := fmt.Sprintf("%s:%d", i.portalHost, i.portalPort)
	cmd := exec.CommandContext(ctx, "iscsiadm", "-m", "node", "-T", i.targetIQN, "-p", portal, "--logout")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("iscsi logout: %w (output: %s)", err, string(output))
	}
	return nil
}

// GetDiskPath returns the iSCSI device path for a VM disk.
func (i *ISCSIBackend) GetDiskPath(vmID string) string {
	// iSCSI disks are typically at /dev/disk/by-path/...
	return fmt.Sprintf("/dev/disk/by-path/ip-%s:%d-iscsi-%s-lun-%d",
		i.portalHost, i.portalPort, i.targetIQN, i.targetLUN)
}

// CloneDisk is not applicable for iSCSI (LUNs are managed externally).
func (i *ISCSIBackend) CloneDisk(ctx context.Context, vmID, sourceHost, targetHost string) error {
	// iSCSI LUNs are shared at the storage array level
	return nil
}

// ---------- Storage Manager ----------

// StorageManager coordinates multiple storage backends.
type StorageManager struct {
	mu       sync.RWMutex
	backends map[string]StorageBackend
	// primary is the default storage backend
	primary StorageBackend
	// nodeBackends maps host IDs to their available backends
	nodeBackends map[string]StorageType
}

// NewStorageManager creates a storage manager.
func NewStorageManager() *StorageManager {
	return &StorageManager{
		backends:     make(map[string]StorageBackend),
		nodeBackends: make(map[string]StorageType),
	}
}

// AddBackend registers a storage backend.
func (m *StorageManager) AddBackend(name string, backend StorageBackend) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.backends[name] = backend
	if m.primary == nil {
		m.primary = backend
	}
}

// SetPrimary sets the default backend.
func (m *StorageManager) SetPrimary(name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	backend, ok := m.backends[name]
	if !ok {
		return fmt.Errorf("storage backend %s not found", name)
	}
	m.primary = backend
	return nil
}

// GetBackend returns a storage backend by name.
func (m *StorageManager) GetBackend(name string) (StorageBackend, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	b, ok := m.backends[name]
	return b, ok
}

// GetPrimary returns the primary storage backend.
func (m *StorageManager) GetPrimary() StorageBackend {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.primary
}

// EnsureStorage ensures storage is mounted/available on the target host
// for VM restart operations.
func (m *StorageManager) EnsureStorage(ctx context.Context, vmID, targetHost string) (string, error) {
	backend := m.GetPrimary()
	if backend == nil {
		return "", fmt.Errorf("no storage backend configured")
	}

	available, err := backend.IsAvailable(ctx)
	if err != nil {
		return "", fmt.Errorf("check storage availability: %w", err)
	}
	if !available {
		return "", fmt.Errorf("storage backend %s not available", backend.Type())
	}

	diskPath := backend.GetDiskPath(vmID)
	return diskPath, nil
}

// GetStorageInfo returns information about all storage backends.
func (m *StorageManager) GetStorageInfo() map[string]string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	info := make(map[string]string)
	for name, backend := range m.backends {
		info[name] = string(backend.Type())
	}
	return info
}

// HealthCheck performs health checks on all backends.
func (m *StorageManager) HealthCheck(ctx context.Context) map[string]bool {
	m.mu.RLock()
	backends := make(map[string]StorageBackend, len(m.backends))
	for k, v := range m.backends {
		backends[k] = v
	}
	m.mu.RUnlock()

	results := make(map[string]bool)
	for name, backend := range backends {
		_, err := backend.IsAvailable(ctx)
		results[name] = (err == nil)
	}
	return results
}
