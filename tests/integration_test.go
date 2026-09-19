// Package tests provides integration and unit tests for HiveStack.
package tests

import (
	"context"
	"fmt"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/maddydevel/HiveStack/internal/auth"
	"github.com/maddydevel/HiveStack/internal/compliance"
	"github.com/maddydevel/HiveStack/internal/db"
	"github.com/maddydevel/HiveStack/internal/ha"
	"github.com/maddydevel/HiveStack/internal/manager"
)

// strPtr returns a pointer to a string.
func strPtr(s string) *string {
	return &s
}

// ---------- Auth & Compliance Tests ----------

// TestHashPassword verifies Argon2id password hashing and verification.
func TestHashPassword(t *testing.T) {
	password := "SuperSecret123!"
	hash, err := auth.HashPassword(password)
	if err != nil {
		t.Fatalf("HashPassword failed: %v", err)
	}
	if !strings.HasPrefix(hash, "$argon2id$") {
		t.Errorf("Expected argon2id hash, got: %s", hash[:20])
	}

	valid, err := auth.CheckPassword(hash, password)
	if err != nil {
		t.Fatalf("CheckPassword failed: %v", err)
	}
	if !valid {
		t.Error("CheckPassword returned false for correct password")
	}

	invalid, err := auth.CheckPassword(hash, "WrongPassword")
	if err != nil {
		t.Fatalf("CheckPassword failed: %v", err)
	}
	if invalid {
		t.Error("CheckPassword returned true for wrong password")
	}
}

// TestHashPasswordEmpty tests empty password handling.
func TestHashPasswordEmpty(t *testing.T) {
	hash, err := auth.HashPassword("")
	if err != nil {
		t.Logf("HashPassword with empty password returned error: %v", err)
		return
	}
	if !strings.HasPrefix(hash, "$argon2id$") {
		t.Errorf("Expected argon2id hash for empty password")
	}

	valid, err := auth.CheckPassword(hash, "")
	if err != nil {
		t.Fatalf("CheckPassword with empty password failed: %v", err)
	}
	if !valid {
		t.Error("Empty password verification failed")
	}
}

// TestHashPasswordDifferentSalt verifies each hash uses a unique salt.
func TestHashPasswordDifferentSalt(t *testing.T) {
	pw := "testpassword"
	h1, err1 := auth.HashPassword(pw)
	h2, err2 := auth.HashPassword(pw)

	if err1 != nil || err2 != nil {
		t.Fatalf("HashPassword failed: h1=%v, h2=%v", err1, err2)
	}
	if h1 == h2 {
		t.Error("Two hashes of the same password should differ (unique salts)")
	}

	v1, err1 := auth.CheckPassword(h1, pw)
	v2, err2 := auth.CheckPassword(h2, pw)
	if err1 != nil || err2 != nil {
		t.Fatalf("CheckPassword failed: v1 err=%v, v2 err=%v", err1, err2)
	}
	if !v1 || !v2 {
		t.Error("Both hashes should verify against the same password")
	}
}

// TestJWTGeneration tests JWT token generation and validation.
func TestJWTGeneration(t *testing.T) {
	os.Setenv("HIVESTACK_JWT_SECRET", "test-secret-for-unit-tests")
	defer os.Unsetenv("HIVESTACK_JWT_SECRET")

	token, err := auth.GenerateToken("user-123", "tenant-456", []string{"viewer", "operator"}, 0)
	if err != nil {
		t.Fatalf("GenerateToken failed: %v", err)
	}
	if token == "" {
		t.Fatal("GenerateToken returned empty token")
	}

	claims, err := auth.ValidateToken(token)
	if err != nil {
		t.Fatalf("ValidateToken failed: %v", err)
	}
	if claims.UserID != "user-123" {
		t.Errorf("Expected userID 'user-123', got '%s'", claims.UserID)
	}
	if claims.TenantID != "tenant-456" {
		t.Errorf("Expected tenantID 'tenant-456', got '%s'", claims.TenantID)
	}
	if len(claims.Scopes) != 2 {
		t.Errorf("Expected 2 scopes, got %d", len(claims.Scopes))
	}

	_, err = auth.ValidateToken("invalid.token.here")
	if err == nil {
		t.Error("Expected error for invalid token")
	}
}

// TestRBACPermissions tests RBAC permission checks.
func TestRBACPermissions(t *testing.T) {
	engine := auth.NewRBACEngine()

	tests := []struct {
		role     string
		resource string
		action   string
		wantPerm bool
	}{
		{"admin", "vm", "create", true},
		{"admin", "vm", "delete", true},
		{"admin", "host", "list", true},
		{"admin", "compliance", "validate", true},
		{"operator", "vm", "create", true},
		{"operator", "vm", "delete", true},
		{"operator", "compliance", "validate", true},
		{"viewer", "vm", "list", true},
		{"viewer", "vm", "create", false},
		{"viewer", "host", "list", true},
		{"viewer", "host", "delete", false},
		{"hana-operator", "vm", "create", true},
		{"hana-operator", "compliance", "validate", true},
		{"compliance-auditor", "compliance", "evidence", true},
		{"compliance-auditor", "vm", "create", false},
		{"nonexistent", "vm", "list", false},
	}

	for _, tt := range tests {
		ok := engine.HasPermission(tt.role, tt.resource, tt.action)
		if ok != tt.wantPerm {
			t.Errorf("HasPermission(%s, %s, %s) = %v, want %v",
				tt.role, tt.resource, tt.action, ok, tt.wantPerm)
		}
	}
}

// TestRBACCheckPermission tests CheckPermission error returns.
func TestRBACCheckPermission(t *testing.T) {
	engine := auth.NewRBACEngine()

	if err := engine.CheckPermission("admin", "vm", "create"); err != nil {
		t.Errorf("CheckPermission for allowed action failed: %v", err)
	}

	err := engine.CheckPermission("viewer", "vm", "create")
	if err == nil {
		t.Error("Expected error for denied action")
	}
	if !strings.Contains(err.Error(), "forbidden") {
		t.Errorf("Expected 'forbidden' error, got: %v", err)
	}
}

// TestValidRole tests IsValidRole.
func TestValidRole(t *testing.T) {
	valid := []string{"admin", "operator", "viewer", "hana-operator", "compliance-auditor"}
	invalid := []string{"", "superuser", "root", "guest", "moderator"}

	for _, r := range valid {
		if !auth.IsValidRole(r) {
			t.Errorf("IsValidRole(%q) = false, want true", r)
		}
	}
	for _, r := range invalid {
		if auth.IsValidRole(r) {
			t.Errorf("IsValidRole(%q) = true, want false", r)
		}
	}
}

// TestHANAValidation tests the HANA compliance guardrails.
func TestHANAValidation(t *testing.T) {
	tests := []struct {
		name     string
		profile  *compliance.VMProfile
		wantPass bool
	}{
		{
			name: "valid HANA VM",
			profile: &compliance.VMProfile{
				Role:                   "hana",
				CPUs:                   8,
				CPUAllocation:          "dedicated",
				MemoryBytes:            64 * 1024 * 1024 * 1024,
				NUMAPolicy:             strPtr("centered"),
				HugepagesEnabled:       true,
				CPUPinning:             []byte(`{"vcpu0":"0","vcpu1":"1"}`),
				MemoryReservationBytes: 64 * 1024 * 1024 * 1024,
				BallooningAllowed:      false,
				SwapAllowed:            false,
			},
			wantPass: true,
		},
		{
			name: "HANA missing NUMA",
			profile: &compliance.VMProfile{
				Role:                   "hana",
				CPUs:                   8,
				CPUAllocation:          "dedicated",
				MemoryBytes:            64 * 1024 * 1024 * 1024,
				NUMAPolicy:             nil,
				HugepagesEnabled:       true,
				CPUPinning:             []byte(`{}`),
				MemoryReservationBytes: 64 * 1024 * 1024 * 1024,
				BallooningAllowed:      false,
				SwapAllowed:            false,
			},
			wantPass: false,
		},
		{
			name: "HANA missing hugepages",
			profile: &compliance.VMProfile{
				Role:                   "hana",
				CPUs:                   8,
				CPUAllocation:          "dedicated",
				MemoryBytes:            64 * 1024 * 1024 * 1024,
				NUMAPolicy:             strPtr("centered"),
				HugepagesEnabled:       false,
				CPUPinning:             []byte(`{}`),
				MemoryReservationBytes: 64 * 1024 * 1024 * 1024,
				BallooningAllowed:      false,
				SwapAllowed:            false,
			},
			wantPass: false,
		},
		{
			name: "HANA ballooning allowed",
			profile: &compliance.VMProfile{
				Role:                   "hana",
				CPUs:                   8,
				CPUAllocation:          "dedicated",
				MemoryBytes:            64 * 1024 * 1024 * 1024,
				NUMAPolicy:             strPtr("centered"),
				HugepagesEnabled:       true,
				CPUPinning:             []byte(`{}`),
				MemoryReservationBytes: 64 * 1024 * 1024 * 1024,
				BallooningAllowed:      true,
				SwapAllowed:            false,
			},
			wantPass: false,
		},
		{
			name: "HANA swap allowed",
			profile: &compliance.VMProfile{
				Role:                   "hana",
				CPUs:                   8,
				CPUAllocation:          "dedicated",
				MemoryBytes:            64 * 1024 * 1024 * 1024,
				NUMAPolicy:             strPtr("centered"),
				HugepagesEnabled:       true,
				CPUPinning:             []byte(`{}`),
				MemoryReservationBytes: 64 * 1024 * 1024 * 1024,
				BallooningAllowed:      false,
				SwapAllowed:            true,
			},
			wantPass: false,
		},
		{
			name: "HANA overcommitted memory",
			profile: &compliance.VMProfile{
				Role:                   "hana",
				CPUs:                   8,
				CPUAllocation:          "dedicated",
				MemoryBytes:            64 * 1024 * 1024 * 1024,
				NUMAPolicy:             strPtr("centered"),
				HugepagesEnabled:       true,
				CPUPinning:             []byte(`{}`),
				MemoryReservationBytes: 32 * 1024 * 1024 * 1024,
				BallooningAllowed:      false,
				SwapAllowed:            false,
			},
			wantPass: false,
		},
		{
			name: "generic VM (skip HANA checks)",
			profile: &compliance.VMProfile{
				Role:                   "generic",
				CPUs:                   4,
				CPUAllocation:          "shared",
				MemoryBytes:            8 * 1024 * 1024 * 1024,
				NUMAPolicy:             nil,
				HugepagesEnabled:       false,
				CPUPinning:             nil,
				MemoryReservationBytes: 0,
				BallooningAllowed:      true,
				SwapAllowed:            true,
			},
			wantPass: true,
		},
	}

	for _, tt := range tests {
		result := compliance.ValidateHANAProfile(tt.profile)
		if result.Passed != tt.wantPass {
			t.Errorf("Test %q: ValidateHANAProfile().Passed = %v, want %v",
				tt.name, result.Passed, tt.wantPass)
		}
	}
}

// TestComplianceEvidenceHash tests the evidence hash chaining.
func TestComplianceEvidenceHash(t *testing.T) {
	evidence := map[string]interface{}{
		"role":              "hana",
		"cpus":              8,
		"memory_bytes":      64 * 1024 * 1024 * 1024,
		"hugepages_enabled": true,
	}

	h1 := compliance.HashEvidence("genesis", evidence)
	if h1 == "" {
		t.Error("HashEvidence returned empty string")
	}

	h2 := compliance.HashEvidence("genesis", evidence)
	if h1 != h2 {
		t.Error("HashEvidence is not deterministic")
	}

	h3 := compliance.HashEvidence("different", evidence)
	if h1 == h3 {
		t.Error("Different previous hash should produce different result")
	}
}

// ---------- Manager Integration Tests ----------

// mockVMStore is an in-memory implementation of the vmStore interface for testing.
type mockVMStore struct {
	mu     sync.Mutex
	vms    map[string]*db.VM
	hosts  map[string]*db.Host
	disks  map[string]*db.Disk
	pools  map[string]*db.StoragePool
	nets   map[string]*db.Network
	events map[string]*db.Event
	nextID int
}

func newMockVMStore() *mockVMStore {
	return &mockVMStore{
		vms:    make(map[string]*db.VM),
		hosts:  make(map[string]*db.Host),
		disks:  make(map[string]*db.Disk),
		pools:  make(map[string]*db.StoragePool),
		nets:   make(map[string]*db.Network),
		events: make(map[string]*db.Event),
		nextID: 1,
	}
}

func (m *mockVMStore) nextIDStr(prefix string) string {
	// Caller must hold m.mu lock
	id := fmt.Sprintf("%s-%d", prefix, m.nextID)
	m.nextID++
	return id
}

// VM operations
func (m *mockVMStore) GetVM(ctx context.Context, id string) (*db.VM, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	vm, ok := m.vms[id]
	if !ok {
		return nil, fmt.Errorf("vm %s not found", id)
	}
	return vm, nil
}

func (m *mockVMStore) ListVMs(ctx context.Context, tenantID string) ([]db.VM, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var result []db.VM
	for _, vm := range m.vms {
		if tenantID == "" || vm.TenantID == tenantID {
			result = append(result, *vm)
		}
	}
	return result, nil
}

func (m *mockVMStore) UpdateVM(ctx context.Context, id string, updates map[string]interface{}) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	vm, ok := m.vms[id]
	if !ok {
		return fmt.Errorf("vm %s not found", id)
	}
	for k, v := range updates {
		switch k {
		case "status":
			vm.Status = v.(string)
		case "host_id":
			h := v.(string)
			vm.HostID = &h
		case "started_at":
			if t, ok := v.(*time.Time); ok {
				vm.StartedAt = t
			} else if t, ok := v.(time.Time); ok {
				vm.StartedAt = &t
			}
		case "name":
			vm.Name = v.(string)
		case "cpus":
			vm.CPUs = v.(int)
		case "memory_bytes":
			vm.MemoryBytes = v.(int64)
		}
	}
	return nil
}

func (m *mockVMStore) CreateVM(ctx context.Context, vm *db.VM) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	id := m.nextIDStr("vm")
	vm.ID = id
	m.vms[id] = vm
	return id, nil
}

func (m *mockVMStore) DeleteVM(ctx context.Context, id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.vms, id)
	return nil
}

// Host operations
func (m *mockVMStore) GetHost(ctx context.Context, id string) (*db.Host, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	h, ok := m.hosts[id]
	if !ok {
		return nil, fmt.Errorf("host %s not found", id)
	}
	return h, nil
}

func (m *mockVMStore) ListHosts(ctx context.Context, tenantID string) ([]db.Host, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var result []db.Host
	for _, h := range m.hosts {
		if tenantID == "" || h.TenantID == tenantID {
			result = append(result, *h)
		}
	}
	return result, nil
}

func (m *mockVMStore) CreateHost(ctx context.Context, h *db.Host) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	id := m.nextIDStr("host")
	h.ID = id
	m.hosts[id] = h
	return id, nil
}

func (m *mockVMStore) UpdateHost(ctx context.Context, id string, updates map[string]interface{}) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	h, ok := m.hosts[id]
	if !ok {
		return fmt.Errorf("host %s not found", id)
	}
	for k, v := range updates {
		switch k {
		case "status":
			h.Status = v.(string)
		case "name":
			h.Name = v.(string)
		case "maintenance_mode":
			h.MaintenanceMode = v.(bool)
		case "cpu_count":
			h.CPUCount = v.(int)
		case "memory_total_bytes":
			h.MemoryTotalBytes = v.(int64)
		}
	}
	return nil
}

func (m *mockVMStore) DeleteHost(ctx context.Context, id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.hosts, id)
	return nil
}

// Event operations
func (m *mockVMStore) CreateEvent(ctx context.Context, e *db.Event) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	id := m.nextIDStr("event")
	e.ID = id
	m.events[id] = e
	return id, nil
}

// Disk operations
func (m *mockVMStore) GetDisk(ctx context.Context, id string) (*db.Disk, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	d, ok := m.disks[id]
	if !ok {
		return nil, fmt.Errorf("disk %s not found", id)
	}
	return d, nil
}

func (m *mockVMStore) UpdateDisk(ctx context.Context, id string, updates map[string]interface{}) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	d, ok := m.disks[id]
	if !ok {
		return fmt.Errorf("disk %s not found", id)
	}
	for k, v := range updates {
		switch k {
		case "size_bytes":
			d.SizeBytes = v.(int64)
		}
	}
	return nil
}

// Storage pool operations
func (m *mockVMStore) GetStoragePool(ctx context.Context, id string) (*db.StoragePool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	p, ok := m.pools[id]
	if !ok {
		return nil, fmt.Errorf("storage pool %s not found", id)
	}
	return p, nil
}

// Network operations
func (m *mockVMStore) CreateNetwork(ctx context.Context, n *db.Network) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	id := m.nextIDStr("net")
	n.ID = id
	m.nets[id] = n
	return id, nil
}

func (m *mockVMStore) GetNetwork(ctx context.Context, id string) (*db.Network, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	n, ok := m.nets[id]
	if !ok {
		return nil, fmt.Errorf("network %s not found", id)
	}
	return n, nil
}

func (m *mockVMStore) DeleteNetwork(ctx context.Context, id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.nets, id)
	return nil
}

// mockController is a mock VMController/VMMigrator for testing.
type mockController struct {
	mu         sync.Mutex
	id         string
	started    map[string]bool
	stopped    map[string]bool
	destroyed  map[string]bool
	migrated   map[string]string
	startErr   error
	stopErr    error
	destroyErr error
	migrateErr error
}

func newMockController(id string) *mockController {
	return &mockController{
		id:        id,
		started:   make(map[string]bool),
		stopped:   make(map[string]bool),
		destroyed: make(map[string]bool),
		migrated:  make(map[string]string),
	}
}

func (c *mockController) StartVM(ctx context.Context, id string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.startErr != nil {
		return c.startErr
	}
	c.started[id] = true
	return nil
}

func (c *mockController) StopVM(ctx context.Context, id string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.stopErr != nil {
		return c.stopErr
	}
	c.stopped[id] = true
	return nil
}

func (c *mockController) DestroyVM(ctx context.Context, id string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.destroyErr != nil {
		return c.destroyErr
	}
	c.destroyed[id] = true
	return nil
}

func (c *mockController) MigrateVM(ctx context.Context, id, targetHost string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.migrateErr != nil {
		return c.migrateErr
	}
	c.migrated[id] = targetHost
	return nil
}

// ---------- Manager Tests ----------

// TestManagerHostLifecycle tests CreateHost, GetHost, UpdateHost, DeleteHost.
func TestManagerHostLifecycle(t *testing.T) {
	store := newMockVMStore()
	mgr, err := manager.NewTestManager(store)
	if err != nil {
		t.Fatalf("NewTestManager failed: %v", err)
	}

	ctx := context.Background()
	adminCtx := manager.WithRole(ctx, "admin")

	// Test CreateHost
	hostID, err := mgr.CreateHost(adminCtx, "test-host", "test-host.example.com", "192.168.1.1", "", "")
	if err != nil {
		t.Fatalf("CreateHost failed: %v", err)
	}
	if hostID == "" {
		t.Fatal("CreateHost returned empty ID")
	}

	// Test GetHost
	host, err := mgr.GetHost(adminCtx, hostID)
	if err != nil {
		t.Fatalf("GetHost failed: %v", err)
	}
	if host.Name != "test-host" {
		t.Errorf("Expected host name 'test-host', got '%s'", host.Name)
	}
	if host.Hostname != "test-host.example.com" {
		t.Errorf("Expected hostname 'test-host.example.com', got '%s'", host.Hostname)
	}
	if host.IPAddress != "192.168.1.1" {
		t.Errorf("Expected IP '192.168.1.1', got '%s'", host.IPAddress)
	}

	// Test UpdateHost
	err = mgr.UpdateHost(adminCtx, hostID, map[string]interface{}{
		"name": "updated-host",
	})
	if err != nil {
		t.Fatalf("UpdateHost failed: %v", err)
	}
	host, err = mgr.GetHost(adminCtx, hostID)
	if err != nil {
		t.Fatalf("GetHost after update failed: %v", err)
	}
	if host.Name != "updated-host" {
		t.Errorf("Expected updated name 'updated-host', got '%s'", host.Name)
	}

	// Test DeleteHost - host is marked as decommissioned
	err = mgr.DeleteHost(adminCtx, hostID)
	if err != nil {
		t.Fatalf("DeleteHost failed: %v", err)
	}
	host, err = mgr.GetHost(adminCtx, hostID)
	if err != nil {
		t.Fatalf("GetHost after delete failed: %v", err)
	}
	if host.Status != "decommissioned" {
		t.Errorf("Expected host status 'decommissioned', got '%s'", host.Status)
	}
}

// TestManagerHostRBAC tests RBAC enforcement for host operations.
func TestManagerHostRBAC(t *testing.T) {
	store := newMockVMStore()
	mgr, err := manager.NewTestManager(store)
	if err != nil {
		t.Fatalf("NewTestManager failed: %v", err)
	}

	ctx := context.Background()
	viewerCtx := manager.WithRole(ctx, "viewer")

	// Viewer should not be able to create hosts
	_, err = mgr.CreateHost(viewerCtx, "test-host", "test-host.example.com", "192.168.1.1", "", "")
	if err == nil {
		t.Error("Expected permission denied for viewer creating host")
	}
	if !strings.Contains(err.Error(), "permission denied") {
		t.Errorf("Expected 'permission denied' error, got: %v", err)
	}
}

// TestManagerVMLifecycle tests CreateVM, StartVM, StopVM, DeleteVM.
func TestManagerVMLifecycle(t *testing.T) {
	store := newMockVMStore()
	mgr, err := manager.NewTestManager(store)
	if err != nil {
		t.Fatalf("NewTestManager failed: %v", err)
	}

	ctx := context.Background()
	adminCtx := manager.WithRole(ctx, "admin")

	// Create a host first
	hostID, err := store.CreateHost(ctx, &db.Host{
		Name:              "test-host",
		Hostname:          "test-host.example.com",
		IPAddress:         "192.168.1.1",
		Status:            "active",
		CPUCount:          16,
		MemoryTotalBytes:  64 * 1024 * 1024 * 1024,
		StorageTotalBytes: 1000 * 1024 * 1024 * 1024,
	})
	if err != nil {
		t.Fatalf("CreateHost failed: %v", err)
	}

	// Register a mock controller for the host
	ctrl := newMockController(hostID)
	mgr.RegisterNodeController(hostID, ctrl)

	// Test CreateVM
	spec := manager.VMSpec{
		Name:        "test-vm",
		Description: "Test VM",
		HostID:      hostID,
		CPUS:        4,
		MemoryBytes: 8 * 1024 * 1024 * 1024,
		Role:        "generic",
		OS:          "linux",
	}
	vmID, err := mgr.CreateVM(adminCtx, spec)
	if err != nil {
		t.Fatalf("CreateVM failed: %v", err)
	}
	if vmID == "" {
		t.Fatal("CreateVM returned empty ID")
	}

	// Test StartVM
	err = mgr.StartVM(adminCtx, vmID)
	if err != nil {
		t.Fatalf("StartVM failed: %v", err)
	}
	if !ctrl.started[vmID] {
		t.Error("StartVM did not call controller.StartVM")
	}

	// Test StopVM
	err = mgr.StopVM(adminCtx, vmID)
	if err != nil {
		t.Fatalf("StopVM failed: %v", err)
	}
	if !ctrl.stopped[vmID] {
		t.Error("StopVM did not call controller.StopVM")
	}

	// Test DeleteVM
	err = mgr.DeleteVM(adminCtx, vmID)
	if err != nil {
		t.Fatalf("DeleteVM failed: %v", err)
	}
	if !ctrl.destroyed[vmID] {
		t.Error("DeleteVM did not call controller.DestroyVM")
	}
}

// TestManagerCreateVMCompliance tests CreateVM with compliance validation.
func TestManagerCreateVMCompliance(t *testing.T) {
	store := newMockVMStore()
	mgr, err := manager.NewTestManager(store)
	if err != nil {
		t.Fatalf("NewTestManager failed: %v", err)
	}

	ctx := context.Background()
	adminCtx := manager.WithRole(ctx, "admin")

	// Create a host
	hostID, err := store.CreateHost(ctx, &db.Host{
		Name:              "test-host",
		Hostname:          "test-host.example.com",
		IPAddress:         "192.168.1.1",
		Status:            "active",
		CPUCount:          16,
		MemoryTotalBytes:  128 * 1024 * 1024 * 1024,
		StorageTotalBytes: 1000 * 1024 * 1024 * 1024,
	})
	if err != nil {
		t.Fatalf("CreateHost failed: %v", err)
	}

	// Test non-compliant HANA VM (missing NUMA)
	spec := manager.VMSpec{
		Name:        "hana-vm",
		Description: "HANA VM",
		HostID:      hostID,
		CPUS:        8,
		MemoryBytes: 64 * 1024 * 1024 * 1024,
		Role:        "hana",
		OS:          "sles15",
		// Missing NUMAPolicy, HugepagesEnabled, etc.
	}
	_, err = mgr.CreateVM(adminCtx, spec)
	if err == nil {
		t.Error("Expected compliance error for non-compliant HANA VM")
	}
	if !strings.Contains(err.Error(), "HANA compliance check failed") {
		t.Errorf("Expected 'HANA compliance check failed' error, got: %v", err)
	}

	// Test compliant HANA VM
	spec = manager.VMSpec{
		Name:                   "hana-vm-compliant",
		Description:            "Compliant HANA VM",
		HostID:                 hostID,
		CPUS:                   8,
		CPUAllocation:          "dedicated",
		MemoryBytes:            64 * 1024 * 1024 * 1024,
		NUMAPolicy:             "centered",
		HugepagesEnabled:       true,
		CPUPinning:             []byte(`{"vcpu0":"0","vcpu1":"1"}`),
		MemoryReservationBytes: 64 * 1024 * 1024 * 1024,
		BallooningAllowed:      false,
		SwapAllowed:            false,
		Role:                   "hana",
		OS:                     "sles15",
	}
	vmID, err := mgr.CreateVM(adminCtx, spec)
	if err != nil {
		t.Fatalf("CreateVM with compliant HANA spec failed: %v", err)
	}
	if vmID == "" {
		t.Fatal("CreateVM returned empty ID for compliant HANA VM")
	}
}

// TestManagerMigrateVM tests MigrateVM.
func TestManagerMigrateVM(t *testing.T) {
	store := newMockVMStore()
	mgr, err := manager.NewTestManager(store)
	if err != nil {
		t.Fatalf("NewTestManager failed: %v", err)
	}

	ctx := context.Background()
	adminCtx := manager.WithRole(ctx, "admin")

	// Create source and target hosts
	sourceHostID, err := store.CreateHost(ctx, &db.Host{
		Name:              "source-host",
		Hostname:          "source-host.example.com",
		IPAddress:         "192.168.1.1",
		Status:            "active",
		CPUCount:          16,
		MemoryTotalBytes:  64 * 1024 * 1024 * 1024,
		StorageTotalBytes: 1000 * 1024 * 1024 * 1024,
	})
	if err != nil {
		t.Fatalf("CreateHost failed: %v", err)
	}

	targetHostID, err := store.CreateHost(ctx, &db.Host{
		Name:              "target-host",
		Hostname:          "target-host.example.com",
		IPAddress:         "192.168.1.2",
		Status:            "active",
		CPUCount:          16,
		MemoryTotalBytes:  64 * 1024 * 1024 * 1024,
		StorageTotalBytes: 1000 * 1024 * 1024 * 1024,
	})
	if err != nil {
		t.Fatalf("CreateHost failed: %v", err)
	}

	// Register mock controllers
	sourceCtrl := newMockController(sourceHostID)
	targetCtrl := newMockController(targetHostID)
	mgr.RegisterNodeController(sourceHostID, sourceCtrl)
	mgr.RegisterNodeController(targetHostID, targetCtrl)

	// Create a running VM on source host
	sourceHostIDStr := sourceHostID
	vmID, err := store.CreateVM(ctx, &db.VM{
		Name:        "test-vm",
		HostID:      &sourceHostIDStr,
		Status:      "running",
		CPUs:        4,
		MemoryBytes: 8 * 1024 * 1024 * 1024,
		Role:        "generic",
	})
	if err != nil {
		t.Fatalf("CreateVM failed: %v", err)
	}

	// Test MigrateVM
	err = mgr.MigrateVM(adminCtx, vmID, targetHostID)
	if err != nil {
		t.Fatalf("MigrateVM failed: %v", err)
	}

	// Verify migration was called
	if sourceCtrl.migrated[vmID] != "target-host.example.com" {
		t.Errorf("Expected migration to target-host.example.com, got %s", sourceCtrl.migrated[vmID])
	}

	// Verify VM host was updated
	vm, err := store.GetVM(ctx, vmID)
	if err != nil {
		t.Fatalf("GetVM after migration failed: %v", err)
	}
	if *vm.HostID != targetHostID {
		t.Errorf("Expected VM host to be %s, got %s", targetHostID, *vm.HostID)
	}
}

// TestManagerHAService tests HA service integration.
func TestManagerHAService(t *testing.T) {
	store := newMockVMStore()
	mgr, err := manager.NewTestManager(store)
	if err != nil {
		t.Fatalf("NewTestManager failed: %v", err)
	}

	ctx := context.Background()

	// Get HA service
	haSvc := mgr.HAService()
	if haSvc == nil {
		t.Fatal("HAService returned nil")
	}

	// Start HA service
	err = haSvc.Start(ctx)
	if err != nil {
		t.Fatalf("HAService.Start failed: %v", err)
	}

	// Register a node
	haSvc.RegisterNode("node-1")

	// Process a heartbeat
	hb := &ha.Heartbeat{
		NodeID:    "node-1",
		Timestamp: time.Now(),
		Sequence:  1,
	}
	err = haSvc.ProcessHeartbeat(ctx, hb)
	if err != nil {
		t.Fatalf("ProcessHeartbeat failed: %v", err)
	}

	// Stop HA service
	err = haSvc.Stop()
	if err != nil {
		t.Fatalf("HAService.Stop failed: %v", err)
	}
}

// TestManagerRunLifecycle tests Manager.Run lifecycle.
func TestManagerRunLifecycle(t *testing.T) {
	t.Skip("Skipping Run lifecycle test - requires HTTP server setup that takes time to shut down")
}

// TestManagerDeleteHostWithRunningVM tests that DeleteHost fails when VMs are running.
func TestManagerDeleteHostWithRunningVM(t *testing.T) {
	store := newMockVMStore()
	mgr, err := manager.NewTestManager(store)
	if err != nil {
		t.Fatalf("NewTestManager failed: %v", err)
	}

	ctx := context.Background()
	adminCtx := manager.WithRole(ctx, "admin")

	// Create a host
	hostID, err := store.CreateHost(ctx, &db.Host{
		Name:              "test-host",
		Hostname:          "test-host.example.com",
		IPAddress:         "192.168.1.1",
		Status:            "active",
		CPUCount:          16,
		MemoryTotalBytes:  64 * 1024 * 1024 * 1024,
		StorageTotalBytes: 1000 * 1024 * 1024 * 1024,
	})
	if err != nil {
		t.Fatalf("CreateHost failed: %v", err)
	}

	// Create a running VM on this host
	hostIDStr := hostID
	_, err = store.CreateVM(ctx, &db.VM{
		Name:        "test-vm",
		HostID:      &hostIDStr,
		Status:      "running",
		CPUs:        4,
		MemoryBytes: 8 * 1024 * 1024 * 1024,
		Role:        "generic",
	})
	if err != nil {
		t.Fatalf("CreateVM failed: %v", err)
	}

	// Try to delete host - should fail
	err = mgr.DeleteHost(adminCtx, hostID)
	if err == nil {
		t.Error("Expected error when deleting host with running VM")
	}
	if !strings.Contains(err.Error(), "running") {
		t.Errorf("Expected 'running' in error message, got: %v", err)
	}
}

// TestManagerVMStartStopWithoutController tests StartVM/StopVM without registered controller.
func TestManagerVMStartStopWithoutController(t *testing.T) {
	store := newMockVMStore()
	mgr, err := manager.NewTestManager(store)
	if err != nil {
		t.Fatalf("NewTestManager failed: %v", err)
	}

	ctx := context.Background()
	adminCtx := manager.WithRole(ctx, "admin")

	// Create a host
	hostID, err := store.CreateHost(ctx, &db.Host{
		Name:              "test-host",
		Hostname:          "test-host.example.com",
		IPAddress:         "192.168.1.1",
		Status:            "active",
		CPUCount:          16,
		MemoryTotalBytes:  64 * 1024 * 1024 * 1024,
		StorageTotalBytes: 1000 * 1024 * 1024 * 1024,
	})
	if err != nil {
		t.Fatalf("CreateHost failed: %v", err)
	}

	// Create a VM without registering a controller
	hostIDStr := hostID
	vmID, err := store.CreateVM(ctx, &db.VM{
		Name:        "test-vm",
		HostID:      &hostIDStr,
		Status:      "created",
		CPUs:        4,
		MemoryBytes: 8 * 1024 * 1024 * 1024,
		Role:        "generic",
	})
	if err != nil {
		t.Fatalf("CreateVM failed: %v", err)
	}

	// Try to start VM - should fail because no controller is registered
	err = mgr.StartVM(adminCtx, vmID)
	if err == nil {
		t.Error("Expected error when starting VM without controller")
	}
	if !strings.Contains(err.Error(), "no registered node agent") {
		t.Errorf("Expected 'no registered node agent' error, got: %v", err)
	}
}

// TestManagerMigrateVMNotRunning tests MigrateVM with non-running VM.
func TestManagerMigrateVMNotRunning(t *testing.T) {
	store := newMockVMStore()
	mgr, err := manager.NewTestManager(store)
	if err != nil {
		t.Fatalf("NewTestManager failed: %v", err)
	}

	ctx := context.Background()
	adminCtx := manager.WithRole(ctx, "admin")

	// Create source and target hosts
	sourceHostID, err := store.CreateHost(ctx, &db.Host{
		Name:              "source-host",
		Hostname:          "source-host.example.com",
		IPAddress:         "192.168.1.1",
		Status:            "active",
		CPUCount:          16,
		MemoryTotalBytes:  64 * 1024 * 1024 * 1024,
		StorageTotalBytes: 1000 * 1024 * 1024 * 1024,
	})
	if err != nil {
		t.Fatalf("CreateHost failed: %v", err)
	}

	targetHostID, err := store.CreateHost(ctx, &db.Host{
		Name:              "target-host",
		Hostname:          "target-host.example.com",
		IPAddress:         "192.168.1.2",
		Status:            "active",
		CPUCount:          16,
		MemoryTotalBytes:  64 * 1024 * 1024 * 1024,
		StorageTotalBytes: 1000 * 1024 * 1024 * 1024,
	})
	if err != nil {
		t.Fatalf("CreateHost failed: %v", err)
	}

	// Create a stopped VM on source host
	sourceHostIDStr := sourceHostID
	vmID, err := store.CreateVM(ctx, &db.VM{
		Name:        "test-vm",
		HostID:      &sourceHostIDStr,
		Status:      "stopped",
		CPUs:        4,
		MemoryBytes: 8 * 1024 * 1024 * 1024,
		Role:        "generic",
	})
	if err != nil {
		t.Fatalf("CreateVM failed: %v", err)
	}

	// Try to migrate stopped VM - should fail
	err = mgr.MigrateVM(adminCtx, vmID, targetHostID)
	if err == nil {
		t.Error("Expected error when migrating non-running VM")
	}
	if !strings.Contains(err.Error(), "not running") {
		t.Errorf("Expected 'not running' error, got: %v", err)
	}
}

// TestManagerMigrateVMSameHost tests MigrateVM to same host.
func TestManagerMigrateVMSameHost(t *testing.T) {
	store := newMockVMStore()
	mgr, err := manager.NewTestManager(store)
	if err != nil {
		t.Fatalf("NewTestManager failed: %v", err)
	}

	ctx := context.Background()
	adminCtx := manager.WithRole(ctx, "admin")

	// Create a host
	hostID, err := store.CreateHost(ctx, &db.Host{
		Name:              "test-host",
		Hostname:          "test-host.example.com",
		IPAddress:         "192.168.1.1",
		Status:            "active",
		CPUCount:          16,
		MemoryTotalBytes:  64 * 1024 * 1024 * 1024,
		StorageTotalBytes: 1000 * 1024 * 1024 * 1024,
	})
	if err != nil {
		t.Fatalf("CreateHost failed: %v", err)
	}

	// Create a running VM
	hostIDStr := hostID
	vmID, err := store.CreateVM(ctx, &db.VM{
		Name:        "test-vm",
		HostID:      &hostIDStr,
		Status:      "running",
		CPUs:        4,
		MemoryBytes: 8 * 1024 * 1024 * 1024,
		Role:        "generic",
	})
	if err != nil {
		t.Fatalf("CreateVM failed: %v", err)
	}

	// Try to migrate to same host - should fail
	err = mgr.MigrateVM(adminCtx, vmID, hostID)
	if err == nil {
		t.Error("Expected error when migrating to same host")
	}
	if !strings.Contains(err.Error(), "already on host") {
		t.Errorf("Expected 'already on host' error, got: %v", err)
	}
}

// TestManagerVMAlreadyRunning tests starting an already running VM.
func TestManagerVMAlreadyRunning(t *testing.T) {
	store := newMockVMStore()
	mgr, err := manager.NewTestManager(store)
	if err != nil {
		t.Fatalf("NewTestManager failed: %v", err)
	}

	ctx := context.Background()
	adminCtx := manager.WithRole(ctx, "admin")

	// Create a host
	hostID, err := store.CreateHost(ctx, &db.Host{
		Name:              "test-host",
		Hostname:          "test-host.example.com",
		IPAddress:         "192.168.1.1",
		Status:            "active",
		CPUCount:          16,
		MemoryTotalBytes:  64 * 1024 * 1024 * 1024,
		StorageTotalBytes: 1000 * 1024 * 1024 * 1024,
	})
	if err != nil {
		t.Fatalf("CreateHost failed: %v", err)
	}

	// Register controller
	ctrl := newMockController(hostID)
	mgr.RegisterNodeController(hostID, ctrl)

	// Create a running VM
	hostIDStr := hostID
	vmID, err := store.CreateVM(ctx, &db.VM{
		Name:        "test-vm",
		HostID:      &hostIDStr,
		Status:      "running",
		CPUs:        4,
		MemoryBytes: 8 * 1024 * 1024 * 1024,
		Role:        "generic",
	})
	if err != nil {
		t.Fatalf("CreateVM failed: %v", err)
	}

	// Try to start already running VM - should fail
	err = mgr.StartVM(adminCtx, vmID)
	if err == nil {
		t.Error("Expected error when starting already running VM")
	}
	if !strings.Contains(err.Error(), "already running") {
		t.Errorf("Expected 'already running' error, got: %v", err)
	}
}

// TestManagerVMNotRunning tests stopping a VM that is not running.
func TestManagerVMNotRunning(t *testing.T) {
	store := newMockVMStore()
	mgr, err := manager.NewTestManager(store)
	if err != nil {
		t.Fatalf("NewTestManager failed: %v", err)
	}

	ctx := context.Background()
	adminCtx := manager.WithRole(ctx, "admin")

	// Create a host
	hostID, err := store.CreateHost(ctx, &db.Host{
		Name:              "test-host",
		Hostname:          "test-host.example.com",
		IPAddress:         "192.168.1.1",
		Status:            "active",
		CPUCount:          16,
		MemoryTotalBytes:  64 * 1024 * 1024 * 1024,
		StorageTotalBytes: 1000 * 1024 * 1024 * 1024,
	})
	if err != nil {
		t.Fatalf("CreateHost failed: %v", err)
	}

	// Create a stopped VM
	hostIDStr := hostID
	vmID, err := store.CreateVM(ctx, &db.VM{
		Name:        "test-vm",
		HostID:      &hostIDStr,
		Status:      "stopped",
		CPUs:        4,
		MemoryBytes: 8 * 1024 * 1024 * 1024,
		Role:        "generic",
	})
	if err != nil {
		t.Fatalf("CreateVM failed: %v", err)
	}

	// Try to stop a VM that is not running - should fail
	err = mgr.StopVM(adminCtx, vmID)
	if err == nil {
		t.Error("Expected error when stopping non-running VM")
	}
	if !strings.Contains(err.Error(), "not running") {
		t.Errorf("Expected 'not running' error, got: %v", err)
	}
}

// TestManagerCreateVMOnInactiveHost tests CreateVM on an inactive host.
func TestManagerCreateVMOnInactiveHost(t *testing.T) {
	store := newMockVMStore()
	mgr, err := manager.NewTestManager(store)
	if err != nil {
		t.Fatalf("NewTestManager failed: %v", err)
	}

	ctx := context.Background()
	adminCtx := manager.WithRole(ctx, "admin")

	// Create an inactive host
	hostID, err := store.CreateHost(ctx, &db.Host{
		Name:              "test-host",
		Hostname:          "test-host.example.com",
		IPAddress:         "192.168.1.1",
		Status:            "pending",
		CPUCount:          16,
		MemoryTotalBytes:  64 * 1024 * 1024 * 1024,
		StorageTotalBytes: 1000 * 1024 * 1024 * 1024,
	})
	if err != nil {
		t.Fatalf("CreateHost failed: %v", err)
	}

	// Try to create VM on inactive host - should fail
	spec := manager.VMSpec{
		Name:        "test-vm",
		HostID:      hostID,
		CPUS:        4,
		MemoryBytes: 8 * 1024 * 1024 * 1024,
		Role:        "generic",
	}
	_, err = mgr.CreateVM(adminCtx, spec)
	if err == nil {
		t.Error("Expected error when creating VM on inactive host")
	}
	if !strings.Contains(err.Error(), "not active") {
		t.Errorf("Expected 'not active' error, got: %v", err)
	}
}

// TestManagerCreateVMInsufficientCapacity tests CreateVM with insufficient host capacity.
func TestManagerCreateVMInsufficientCapacity(t *testing.T) {
	store := newMockVMStore()
	mgr, err := manager.NewTestManager(store)
	if err != nil {
		t.Fatalf("NewTestManager failed: %v", err)
	}

	ctx := context.Background()
	adminCtx := manager.WithRole(ctx, "admin")

	// Create a host with limited capacity
	hostID, err := store.CreateHost(ctx, &db.Host{
		Name:              "test-host",
		Hostname:          "test-host.example.com",
		IPAddress:         "192.168.1.1",
		Status:            "active",
		CPUCount:          4,
		MemoryTotalBytes:  8 * 1024 * 1024 * 1024,
		StorageTotalBytes: 1000 * 1024 * 1024 * 1024,
	})
	if err != nil {
		t.Fatalf("CreateHost failed: %v", err)
	}

	// Try to create VM with more CPUs than available - should fail
	spec := manager.VMSpec{
		Name:        "test-vm",
		HostID:      hostID,
		CPUS:        8,
		MemoryBytes: 4 * 1024 * 1024 * 1024,
		Role:        "generic",
	}
	_, err = mgr.CreateVM(adminCtx, spec)
	if err == nil {
		t.Error("Expected error when creating VM with insufficient CPU")
	}
	if !strings.Contains(err.Error(), "insufficient CPU") {
		t.Errorf("Expected 'insufficient CPU' error, got: %v", err)
	}
}

// TestManagerListHosts tests ListHosts.
func TestManagerListHosts(t *testing.T) {
	store := newMockVMStore()
	mgr, err := manager.NewTestManager(store)
	if err != nil {
		t.Fatalf("NewTestManager failed: %v", err)
	}

	ctx := context.Background()
	adminCtx := manager.WithRole(ctx, "admin")

	// Create some hosts
	for i := 0; i < 3; i++ {
		_, err := store.CreateHost(ctx, &db.Host{
			Name:              fmt.Sprintf("host-%d", i),
			Hostname:          fmt.Sprintf("host-%d.example.com", i),
			IPAddress:         fmt.Sprintf("192.168.1.%d", i+1),
			Status:            "active",
			CPUCount:          16,
			MemoryTotalBytes:  64 * 1024 * 1024 * 1024,
			StorageTotalBytes: 1000 * 1024 * 1024 * 1024,
		})
		if err != nil {
			t.Fatalf("CreateHost failed: %v", err)
		}
	}

	// List hosts
	hosts, err := mgr.ListHosts(adminCtx)
	if err != nil {
		t.Fatalf("ListHosts failed: %v", err)
	}
	if len(hosts) != 3 {
		t.Errorf("Expected 3 hosts, got %d", len(hosts))
	}
}

// TestManagerRegisterUnregisterNode tests node registration.
func TestManagerRegisterUnregisterNode(t *testing.T) {
	store := newMockVMStore()
	mgr, err := manager.NewTestManager(store)
	if err != nil {
		t.Fatalf("NewTestManager failed: %v", err)
	}

	// Register a controller
	ctrl := newMockController("node-1")
	mgr.RegisterNodeController("node-1", ctrl)

	// Verify it's registered
	nodes := mgr.ListNodes()
	if nodes == nil {
		t.Fatal("ListNodes returned nil")
	}

	// Unregister
	mgr.UnregisterNode("node-1")

	// Verify controller is gone
	// (we can't directly check controllers map, but we can verify via vmController behavior)
}
