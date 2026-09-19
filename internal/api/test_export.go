// Package api provides exported test helpers for integration tests.
//
// This file is NOT a _test.go file so that NewTestServer and the mock types
// are available to external test packages (e.g. package tests) via normal
// imports. The mock types here are prefixed with "test" to avoid colliding
// with the mocks defined in server_test.go.
package api

import (
	"context"
	"fmt"
	"net/http"

	"github.com/maddydevel/HiveStack/internal/auth"
	"github.com/maddydevel/HiveStack/internal/db"
)

// testDB implements dbInterface for integration testing.
type testDB struct {
	users       []db.User
	hosts       []db.Host
	vms         []db.VM
	pools       []db.StoragePool
	networks    []db.Network
	backups     []db.Backup
	events      []db.Event
	datacenters []db.Datacenter
	err         map[string]error
}

func (m *testDB) ListUsers(ctx context.Context, tenantID string) ([]db.User, error) {
	if e, ok := m.err["ListUsers"]; ok {
		return nil, e
	}
	return m.users, nil
}
func (m *testDB) CreateUser(ctx context.Context, u *db.User) (string, error) {
	if e, ok := m.err["CreateUser"]; ok {
		return "", e
	}
	u.ID = "test-user-id"
	m.users = append(m.users, *u)
	return u.ID, nil
}
func (m *testDB) GetUser(ctx context.Context, id string) (*db.User, error) {
	if e, ok := m.err["GetUser"]; ok {
		return nil, e
	}
	for i := range m.users {
		if m.users[i].ID == id {
			return &m.users[i], nil
		}
	}
	return nil, fmt.Errorf("user not found")
}
func (m *testDB) UpdateUser(ctx context.Context, id string, updates map[string]interface{}) error {
	if e, ok := m.err["UpdateUser"]; ok {
		return e
	}
	return nil
}
func (m *testDB) DeleteUser(ctx context.Context, id string) error {
	if e, ok := m.err["DeleteUser"]; ok {
		return e
	}
	for i, u := range m.users {
		if u.ID == id {
			m.users = append(m.users[:i], m.users[i+1:]...)
			return nil
		}
	}
	return fmt.Errorf("user not found")
}

func (m *testDB) ListHosts(ctx context.Context, tenantID string) ([]db.Host, error) {
	if e, ok := m.err["ListHosts"]; ok {
		return nil, e
	}
	return m.hosts, nil
}
func (m *testDB) CreateHost(ctx context.Context, h *db.Host) (string, error) {
	if e, ok := m.err["CreateHost"]; ok {
		return "", e
	}
	h.ID = "test-host-id"
	m.hosts = append(m.hosts, *h)
	return h.ID, nil
}
func (m *testDB) GetHost(ctx context.Context, id string) (*db.Host, error) {
	if e, ok := m.err["GetHost"]; ok {
		return nil, e
	}
	for i := range m.hosts {
		if m.hosts[i].ID == id {
			return &m.hosts[i], nil
		}
	}
	return nil, fmt.Errorf("host not found")
}
func (m *testDB) UpdateHost(ctx context.Context, id string, updates map[string]interface{}) error {
	if e, ok := m.err["UpdateHost"]; ok {
		return e
	}
	return nil
}
func (m *testDB) DeleteHost(ctx context.Context, id string) error {
	if e, ok := m.err["DeleteHost"]; ok {
		return e
	}
	return nil
}

func (m *testDB) ListVMs(ctx context.Context, tenantID string) ([]db.VM, error) {
	if e, ok := m.err["ListVMs"]; ok {
		return nil, e
	}
	return m.vms, nil
}
func (m *testDB) CreateVM(ctx context.Context, vm *db.VM) (string, error) {
	if e, ok := m.err["CreateVM"]; ok {
		return "", e
	}
	vm.ID = "test-vm-id"
	m.vms = append(m.vms, *vm)
	return vm.ID, nil
}
func (m *testDB) GetVM(ctx context.Context, id string) (*db.VM, error) {
	if e, ok := m.err["GetVM"]; ok {
		return nil, e
	}
	for i := range m.vms {
		if m.vms[i].ID == id {
			return &m.vms[i], nil
		}
	}
	return nil, fmt.Errorf("vm not found")
}
func (m *testDB) UpdateVM(ctx context.Context, id string, updates map[string]interface{}) error {
	if e, ok := m.err["UpdateVM"]; ok {
		return e
	}
	for i := range m.vms {
		if m.vms[i].ID == id {
			if name, ok := updates["name"].(string); ok {
				m.vms[i].Name = name
			}
			if desc, ok := updates["description"].(string); ok {
				m.vms[i].Description = desc
			}
			if status, ok := updates["status"].(string); ok {
				m.vms[i].Status = status
			}
			return nil
		}
	}
	return fmt.Errorf("vm not found")
}
func (m *testDB) DeleteVM(ctx context.Context, id string) error {
	if e, ok := m.err["DeleteVM"]; ok {
		return e
	}
	return nil
}

func (m *testDB) ListStoragePools(ctx context.Context, tenantID string) ([]db.StoragePool, error) {
	if e, ok := m.err["ListStoragePools"]; ok {
		return nil, e
	}
	return m.pools, nil
}
func (m *testDB) CreateStoragePool(ctx context.Context, sp *db.StoragePool) (string, error) {
	if e, ok := m.err["CreateStoragePool"]; ok {
		return "", e
	}
	m.pools = append(m.pools, *sp)
	return "test-pool-id", nil
}
func (m *testDB) GetStoragePool(ctx context.Context, id string) (*db.StoragePool, error) {
	if e, ok := m.err["GetStoragePool"]; ok {
		return nil, e
	}
	for i := range m.pools {
		if m.pools[i].ID == id {
			return &m.pools[i], nil
		}
	}
	return nil, fmt.Errorf("pool not found")
}

func (m *testDB) ListNetworks(ctx context.Context, tenantID string) ([]db.Network, error) {
	if e, ok := m.err["ListNetworks"]; ok {
		return nil, e
	}
	return m.networks, nil
}
func (m *testDB) CreateNetwork(ctx context.Context, n *db.Network) (string, error) {
	if e, ok := m.err["CreateNetwork"]; ok {
		return "", e
	}
	m.networks = append(m.networks, *n)
	return "test-net-id", nil
}
func (m *testDB) GetNetwork(ctx context.Context, id string) (*db.Network, error) {
	if e, ok := m.err["GetNetwork"]; ok {
		return nil, e
	}
	for i := range m.networks {
		if m.networks[i].ID == id {
			return &m.networks[i], nil
		}
	}
	return nil, fmt.Errorf("network not found")
}
func (m *testDB) DeleteNetwork(ctx context.Context, id string) error {
	if e, ok := m.err["DeleteNetwork"]; ok {
		return e
	}
	return nil
}

func (m *testDB) ListBackups(ctx context.Context, tenantID string) ([]db.Backup, error) {
	if e, ok := m.err["ListBackups"]; ok {
		return nil, e
	}
	return m.backups, nil
}
func (m *testDB) CreateBackup(ctx context.Context, b *db.Backup) (string, error) {
	if e, ok := m.err["CreateBackup"]; ok {
		return "", e
	}
	m.backups = append(m.backups, *b)
	return "test-backup-id", nil
}
func (m *testDB) GetBackup(ctx context.Context, id string) (*db.Backup, error) {
	if e, ok := m.err["GetBackup"]; ok {
		return nil, e
	}
	for i := range m.backups {
		if m.backups[i].ID == id {
			return &m.backups[i], nil
		}
	}
	return nil, fmt.Errorf("backup not found")
}
func (m *testDB) UpdateBackup(ctx context.Context, id string, updates map[string]interface{}) error {
	if e, ok := m.err["UpdateBackup"]; ok {
		return e
	}
	return nil
}

func (m *testDB) ListEvents(ctx context.Context, tenantID string, limit int) ([]db.Event, error) {
	if e, ok := m.err["ListEvents"]; ok {
		return nil, e
	}
	return m.events, nil
}

func (m *testDB) ListDatacenters(ctx context.Context, tenantID string) ([]db.Datacenter, error) {
	if e, ok := m.err["ListDatacenters"]; ok {
		return nil, e
	}
	return m.datacenters, nil
}
func (m *testDB) CreateDatacenter(ctx context.Context, dc *db.Datacenter) (string, error) {
	if e, ok := m.err["CreateDatacenter"]; ok {
		return "", e
	}
	m.datacenters = append(m.datacenters, *dc)
	return "test-dc-id", nil
}
func (m *testDB) GetDatacenter(ctx context.Context, id string) (*db.Datacenter, error) {
	if e, ok := m.err["GetDatacenter"]; ok {
		return nil, e
	}
	for i := range m.datacenters {
		if m.datacenters[i].ID == id {
			return &m.datacenters[i], nil
		}
	}
	return nil, fmt.Errorf("datacenter not found")
}
func (m *testDB) DeleteDatacenter(ctx context.Context, id string) error {
	if e, ok := m.err["DeleteDatacenter"]; ok {
		return e
	}
	return nil
}

// testVMHandler implements VMHandler for integration testing.
type testVMHandler struct{}

func (h *testVMHandler) MigrateVM(ctx context.Context, id, targetHostID string) error {
	return nil
}
func (h *testVMHandler) GetSnapshots(ctx context.Context, vmID string) ([]map[string]interface{}, error) {
	return []map[string]interface{}{}, nil
}
func (h *testVMHandler) CreateSnapshot(ctx context.Context, vmID, name string) (string, error) {
	return "snap-test", nil
}
func (h *testVMHandler) DeleteSnapshot(ctx context.Context, vmID, snapshotID string) error {
	return nil
}
func (h *testVMHandler) GetVMStats(ctx context.Context, vmID string) (map[string]interface{}, error) {
	return map[string]interface{}{
		"cpu_usage":    0,
		"memory_usage": 0,
		"disk_io":      0,
		"network_io":   0,
	}, nil
}

// testStorageHandler implements StorageHandler for integration testing.
type testStorageHandler struct{}

func (h *testStorageHandler) CreateStoragePool(ctx context.Context, id, name, poolType, path, hostID string) error {
	return nil
}
func (h *testStorageHandler) DeleteStoragePool(ctx context.Context, id, hostID string) error {
	return nil
}
func (h *testStorageHandler) ResizeDisk(ctx context.Context, diskID string, newSizeBytes int64) error {
	return nil
}

// testNetworkHandler implements NetworkHandler for integration testing.
type testNetworkHandler struct {
	networks map[string]db.Network
}

func (h *testNetworkHandler) CreateNetwork(ctx context.Context, n *db.Network) (string, error) {
	if h.networks == nil {
		h.networks = make(map[string]db.Network)
	}
	h.networks[n.ID] = *n
	return "test-net-id", nil
}
func (h *testNetworkHandler) DeleteNetwork(ctx context.Context, id string) error {
	delete(h.networks, id)
	return nil
}

// NewTestServer creates an APIServer with mock dependencies for integration
// testing. The returned server has all routes registered and is ready to use
// with httptest.NewServer via the Handler() method.
//
// The server is configured with:
//   - testDB (in-memory, no real database)
//   - testVMHandler (no-op lifecycle operations)
//   - testNetworkHandler (in-memory network tracking)
//   - testStorageHandler (no-op storage operations)
//   - A real migrationJobTracker
//   - JWT secret "test-secret" (set HIVESTACK_JWT_SECRET before generating tokens)
func NewTestServer() *APIServer {
	mdb := &testDB{}
	cfg := &Config{
		Server: ServerConfig{Host: "localhost", Port: 8080},
		Auth:   AuthConfig{JWTSecret: "test-secret"},
	}
	s := &APIServer{
		Config:         cfg,
		db:             mdb,
		rbac:           auth.NewRBACEngine(),
		mux:            new(http.ServeMux),
		vmHandler:      &testVMHandler{},
		networkHandler: &testNetworkHandler{},
		storageHandler: &testStorageHandler{},
	}
	s.registerRoutes()
	s.migrationJobTracker = newMigrationJobTracker()
	return s
}

// Handler returns the HTTP handler (mux) for the API server.
// This allows integration tests to wrap it with httptest.NewServer.
func (s *APIServer) Handler() http.Handler {
	return s.mux
}
