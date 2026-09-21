// Package libvirt provides a mock implementation of the Libvirt wrapper for
// testing and development environments without real KVM/libvirt.
//
// All methods return success without touching actual hardware — the mock
// maintains in-memory state to simulate domain lifecycle.
package libvirt

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// MockLibvirt is a mock implementation that satisfies the same interface as
// Libvirt without requiring a real libvirt daemon.
type MockLibvirt struct {
	mu        sync.Mutex
	uri       string
	connected bool
	domains   map[string]*mockDomain
	hostInfo  HostInfo
	nextVMID  int
}

// mockDomain represents a simulated VM domain.
type mockDomain struct {
	ID     string
	Name   string
	XML    string
	CPUs   int
	Memory uint64
	State  string // "running", "shut off", "paused"
	HostID string
}

// NewMockLibvirt creates a new mock libvirt instance.
func NewMockLibvirt(uri string) (*MockLibvirt, error) {
	return &MockLibvirt{
		uri: uri,
		hostInfo: HostInfo{
			Hostname: "mock-host",
			CPU:      CPUInfo{Count: 8, Model: "x86_64"},
			Memory:   MemoryInfo{Total: 34359738368, Free: 17179869184},
			Disk:     DiskInfo{Total: 2199023255552, Free: 1099511627776},
		},
		domains: make(map[string]*mockDomain),
	}, nil
}

// Connect establishes a mock connection.
func (m *MockLibvirt) Connect() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.connected = true
	return nil
}

// Close closes the mock connection.
func (m *MockLibvirt) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.connected = false
	return nil
}

// GetHostInfo returns mock host information.
func (m *MockLibvirt) GetHostInfo(ctx context.Context) (*HostInfo, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if !m.connected {
		return nil, fmt.Errorf("not connected")
	}
	info := m.hostInfo
	return &info, nil
}

// CreateDomain creates a mock domain from XML specification.
func (m *MockLibvirt) CreateDomain(ctx context.Context, name, xml string, cpus int, memory uint64) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if !m.connected {
		return fmt.Errorf("not connected")
	}
	m.nextVMID++
	id := fmt.Sprintf("vm-%d", m.nextVMID)
	m.domains[id] = &mockDomain{
		ID:     id,
		Name:   name,
		XML:    xml,
		CPUs:   cpus,
		Memory: memory,
		State:  "shut off",
	}
	return nil
}

// StartDomain starts a mock domain.
func (m *MockLibvirt) StartDomain(ctx context.Context, id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if !m.connected {
		return fmt.Errorf("not connected")
	}
	dom, ok := m.domains[id]
	if !ok {
		return fmt.Errorf("domain %s not found", id)
	}
	if dom.State == "running" {
		return fmt.Errorf("domain %s already running", id)
	}
	dom.State = "running"
	return nil
}

// StopDomain stops a mock domain gracefully.
func (m *MockLibvirt) StopDomain(ctx context.Context, id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if !m.connected {
		return fmt.Errorf("not connected")
	}
	dom, ok := m.domains[id]
	if !ok {
		return fmt.Errorf("domain %s not found", id)
	}
	dom.State = "shut off"
	return nil
}

// DestroyDomain forcefully destroys a mock domain.
func (m *MockLibvirt) DestroyDomain(ctx context.Context, id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if !m.connected {
		return fmt.Errorf("not connected")
	}
	dom, ok := m.domains[id]
	if !ok {
		return fmt.Errorf("domain %s not found", id)
	}
	dom.State = "shut off"
	return nil
}

// GetDomainXML returns the XML definition for a mock domain.
func (m *MockLibvirt) GetDomainXML(ctx context.Context, id string) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if !m.connected {
		return "", fmt.Errorf("not connected")
	}
	dom, ok := m.domains[id]
	if !ok {
		return "", fmt.Errorf("domain %s not found", id)
	}
	return dom.XML, nil
}

// ListDomains returns all mock domains.
func (m *MockLibvirt) ListDomains(ctx context.Context) ([]VMInfo, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if !m.connected {
		return nil, fmt.Errorf("not connected")
	}
	vms := make([]VMInfo, 0, len(m.domains))
	for _, dom := range m.domains {
		vms = append(vms, VMInfo{
			ID:     dom.ID,
			Name:   dom.Name,
			CPUs:   dom.CPUs,
			Memory: dom.Memory,
			State:  dom.State,
		})
	}
	return vms, nil
}

// DefineVM wraps CreateDomain with simpler signature.
func (m *MockLibvirt) DefineVM(ctx context.Context, xml string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if !m.connected {
		return fmt.Errorf("not connected")
	}
	// Parse name from XML — simplified: just use timestamp
	m.nextVMID++
	id := fmt.Sprintf("vm-%d", m.nextVMID)
	m.domains[id] = &mockDomain{
		ID:    id,
		Name:  fmt.Sprintf("vm-%d", m.nextVMID),
		XML:   xml,
		State: "shut off",
	}
	return nil
}

// StartVM starts a domain by ID.
func (m *MockLibvirt) StartVM(ctx context.Context, id string) error {
	return m.StartDomain(ctx, id)
}

// StopVM stops a domain by ID.
func (m *MockLibvirt) StopVM(ctx context.Context, id string) error {
	return m.StopDomain(ctx, id)
}

// DestroyVM destroys a domain by ID.
func (m *MockLibvirt) DestroyVM(ctx context.Context, id string) error {
	return m.DestroyDomain(ctx, id)
}

// ListVMs returns all mock VMs.
func (m *MockLibvirt) ListVMs(ctx context.Context) ([]VMInfo, error) {
	return m.ListDomains(ctx)
}

// GetVMStats returns mock statistics for a domain.
func (m *MockLibvirt) GetVMStats(ctx context.Context, id string) (*VMStats, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if !m.connected {
		return nil, fmt.Errorf("not connected")
	}
	if _, ok := m.domains[id]; !ok {
		return nil, fmt.Errorf("domain %s not found", id)
	}
	return &VMStats{
		CPUUsage:       25.5,
		MemoryUsage:    4294967296,
		MemoryTotal:    8589934592,
		DiskReadBytes:  1048576,
		DiskWriteBytes: 524288,
		DiskIO:         1572864,
		NetRxBytes:     2097152,
		NetTxBytes:     1048576,
		NetworkIO:      3145728,
	}, nil
}

// IsConnected returns connection status.
func (m *MockLibvirt) IsConnected() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.connected
}

// GetURI returns the connection URI.
func (m *MockLibvirt) GetURI() string {
	return m.uri
}

// GetDomainCount returns the number of defined domains.
func (m *MockLibvirt) GetDomainCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.domains)
}

// GetRunningDomainCount returns domains currently in "running" state.
func (m *MockLibvirt) GetRunningDomainCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	count := 0
	for _, dom := range m.domains {
		if dom.State == "running" {
			count++
		}
	}
	return count
}

// CreateStoragePool creates a mock storage pool.
func (m *MockLibvirt) CreateStoragePool(ctx context.Context, id, name, poolType, path string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if !m.connected {
		return fmt.Errorf("not connected")
	}
	return nil
}

// DeleteStoragePool deletes a mock storage pool.
func (m *MockLibvirt) DeleteStoragePool(ctx context.Context, id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if !m.connected {
		return fmt.Errorf("not connected")
	}
	return nil
}

// CreateVolume creates a mock storage volume.
func (m *MockLibvirt) CreateVolume(ctx context.Context, pool, name string, format string, size uint64) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if !m.connected {
		return fmt.Errorf("not connected")
	}
	return nil
}

// CreateNetwork creates a mock virtual network.
func (m *MockLibvirt) CreateNetwork(ctx context.Context, name, bridge string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if !m.connected {
		return fmt.Errorf("not connected")
	}
	return nil
}

// WaitForState simulates waiting for a domain to reach a specific state.
// Returns immediately since mock state changes are synchronous.
func (m *MockLibvirt) WaitForState(ctx context.Context, id, desiredState string, timeout time.Duration) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if !m.connected {
		return fmt.Errorf("not connected")
	}
	dom, ok := m.domains[id]
	if !ok {
		return fmt.Errorf("domain %s not found", id)
	}
	if dom.State != desiredState {
		return fmt.Errorf("domain %s is in state %s, expected %s", id, dom.State, desiredState)
	}
	return nil
}
