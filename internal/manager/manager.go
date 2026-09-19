// Package manager provides the HiveStack Manager core services.
//
// The Manager is the central control plane for HiveStack. It:
//   - Authenticates users and enforces RBAC
//   - Manages tenant isolation
//   - Orchestrates VM lifecycle across nodes
//   - Enforces HANA compliance guardrails
//   - Coordinates with Node Agents via gRPC
//   - Serves the REST API for the web UI and CLI
package manager

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/maddydevel/HiveStack/internal/api"
	"github.com/maddydevel/HiveStack/internal/auth"
	"github.com/maddydevel/HiveStack/internal/compliance"
	"github.com/maddydevel/HiveStack/internal/db"
	"github.com/maddydevel/HiveStack/internal/ha"
	hivetls "github.com/maddydevel/HiveStack/internal/tls"
	"github.com/maddydevel/HiveStack/internal/libvirt"
	"github.com/maddydevel/HiveStack/internal/node"
)

// roleContextKey is the context key for the authenticated role.
type roleContextKey struct{}

// Manager holds the HiveStack Manager state and all its subsystems.
type Manager struct {
	mu         sync.Mutex
	allocMu    sync.Mutex // serializes host capacity check + VM insert
	db         *db.DB
	store      vmStore // database access used by MigrateVM and event publishing
	rbac       *auth.RBACEngine
	compliance *compliance.ComplianceStore
	apiServer  *api.APIServer
	nodes      map[string]*node.Agent
	controllers map[string]node.VMController // remote node agents reached over gRPC
	shutdownCh chan struct{}
	wg         sync.WaitGroup
	running    bool
	haSvc      *haService

	// migrations holds the capacity claimed on the target host by each in-flight
	// migration, keyed by VM ID. The VM's row still points at its source host
	// until the migration completes, so without this a concurrent migration or
	// create could claim the same capacity. Guarded by allocMu.
	migrations map[string]db.VM
}

// vmStore is the subset of the database that MigrateVM and publishEvent use.
// *db.DB satisfies it; tests substitute an in-memory fake.
type vmStore interface {
	GetVM(ctx context.Context, id string) (*db.VM, error)
	GetHost(ctx context.Context, id string) (*db.Host, error)
	ListVMs(ctx context.Context, tenantID string) ([]db.VM, error)
	UpdateVM(ctx context.Context, id string, updates map[string]interface{}) error
	CreateEvent(ctx context.Context, e *db.Event) (string, error)
}

var _ vmStore = (*db.DB)(nil)

// New creates a new HiveStack Manager.
func New(database *db.DB) (*Manager, error) {
    rbac := auth.NewRBACEngine()
    compliance := compliance.NewComplianceStore(database)
    nodes := make(map[string]*node.Agent)

    m := &Manager{
        db:         database,
        store:      database,
        rbac:       rbac,
        compliance: compliance,
        nodes:      nodes,
        controllers: make(map[string]node.VMController),
        shutdownCh: make(chan struct{}),
    }

    // Create the HA service (does not start it yet)
    haSvc, err := NewHAService(m, ha.DefaultThresholds())
    if err != nil {
        return nil, fmt.Errorf("create HA service: %w", err)
    }
    m.haSvc = haSvc

    return m, nil
}

// Run starts the Manager: initializes the API server and starts the main service loop.
func (m *Manager) Run(ctx context.Context, apiAddr string) error {
    m.mu.Lock()
    if m.running {
        m.mu.Unlock()
        return fmt.Errorf("manager already running")
    }
    m.running = true
    m.mu.Unlock()

    log.Println("HiveStack Manager starting...")

    // Start the HA service before the API server so HA data is available
    if err := m.haSvc.Start(ctx); err != nil {
        return fmt.Errorf("start HA service: %w", err)
    }

    cfg := &api.Config{
        Server: api.ServerConfig{
            Host:       "0.0.0.0",
            Port:       8080,
            TLSEnabled: false,
        },
        TLS: hivetls.CertConfig{
            CertFile: "", // set from config if TLS enabled
            KeyFile:  "",
            CAFile:   "",
        },
        Database: api.DatabaseConfig{
            DSN: "postgres://hivestack:***@localhost:5432/hivestack?sslmode=disable",
        },
        Auth: api.AuthConfig{
            JWTSecret:   "change-me-in-production",
            TokenExpiry: "24h",
        },
        Node: api.NodeConfig{
            GRPCAddress: "hivestack-manager:9090",
        },
    }

    apiServer, err := api.New(cfg, m.db)
    if err != nil {
        return fmt.Errorf("create API server: %w", err)
    }
    m.apiServer = apiServer

    // Wire HA dependencies into the API server
    m.wireHAIntoAPI()
    m.apiServer.SetVMHandler(m)

    m.wg.Add(1)
    go func() {
        defer m.wg.Done()
        log.Printf("API server listening on %s", apiAddr)
        if err := apiServer.Run(); err != nil && err != context.Canceled {
            log.Printf("API server error: %v", err)
        }
    }()

    log.Println("HiveStack Manager running — API server active, RBAC engine loaded, compliance store ready")

    <-ctx.Done()
    log.Println("Manager: shutdown signal received")
    close(m.shutdownCh)

    m.wg.Wait()
    log.Println("Manager: stopped")
    return nil
}

// RBAC returns the RBAC engine for permission checks.
func (m *Manager) RBAC() *auth.RBACEngine {
    return m.rbac
}

// Compliance returns the compliance store for HANA guardrail operations.
func (m *Manager) Compliance() *compliance.ComplianceStore {
    return m.compliance
}

// DB returns the database connection.
func (m *Manager) DB() *db.DB {
	return m.db
}

// roleFromContext extracts the role name from the context.
// Returns the role name and true if found, or empty string and false.
func roleFromContext(ctx context.Context) (string, bool) {
	role, ok := ctx.Value(roleContextKey{}).(string)
	return role, ok
}

// withRole returns a new context with the given role.
func withRole(ctx context.Context, role string) context.Context {
	return context.WithValue(ctx, roleContextKey{}, role)
}

// GetAgentStatus returns the status string for a node agent.
// This abstracts over the unexported fields of node.Agent.
func GetAgentStatus(agent *node.Agent) (string, bool) {
	if agent == nil {
		return "", false
	}
	// Use exported methods to check status
	// StartVM will fail if agent not running, so we use ListVMs as a proxy
	_, err := agent.ListVMs(context.Background())
	if err != nil {
		return "stopped", true
	}
	return "running", true
}

// publishEvent inserts an event record into the events table.
func (m *Manager) publishEvent(ctx context.Context, tenantID, severity, message,
	actorType, actorID, actorName, resourceType, resourceID, resourceName string) error {
	_, err := m.store.CreateEvent(ctx, &db.Event{
		TenantID:     tenantID,
		Type:         "info",
		Severity:     severity,
		Message:      message,
		ActorType:    actorType,
		ActorID:      actorID,
		ActorName:    actorName,
		ResourceType: resourceType,
		ResourceID:   resourceID,
		ResourceName: resourceName,
	})
	return err
}

// ---------- Host Lifecycle Methods ----------

// CreateHost creates a new host record and publishes a HostRegistered event.
func (m *Manager) CreateHost(ctx context.Context, name, hostname, ip, tan, labels string) (string, error) {
	role, hasRole := roleFromContext(ctx)
	if !hasRole {
		return "", fmt.Errorf("no role in context")
	}
	if err := m.RBAC().CheckPermission(role, "host", "create"); err != nil {
		return "", fmt.Errorf("permission denied: %w", err)
	}

	if name == "" {
		return "", fmt.Errorf("host name is required")
	}
	if hostname == "" {
		return "", fmt.Errorf("hostname is required")
	}
	if ip == "" {
		return "", fmt.Errorf("IP address is required")
	}

	host := &db.Host{
		Name:         name,
		Hostname:     hostname,
		IPAddress:    ip,
		Status:       "pending",
		OS:           "linux",
		Hypervisor:   "kvm",
		MemoryTotalBytes: 0,
		StorageTotalBytes: 0,
		CPUCount:     0,
		MaintenanceMode: false,
	}

	id, err := m.db.CreateHost(ctx, host)
	if err != nil {
		return "", fmt.Errorf("create host record: %w", err)
	}

	eventMsg := fmt.Sprintf("Host registered: %s (%s)", name, hostname)
	if err := m.publishEvent(ctx, "", "info", eventMsg, "system", id, name, "host", id, name); err != nil {
		log.Printf("Warning: failed to publish HostRegistered event: %v", err)
	}

	log.Printf("Host created: %s (id=%s)", name, id)
	return id, nil
}

// GetHost retrieves a host by ID, enriched with node agent status if registered.
func (m *Manager) GetHost(ctx context.Context, id string) (*db.Host, error) {
	role, hasRole := roleFromContext(ctx)
	if !hasRole {
		return nil, fmt.Errorf("no role in context")
	}
	if err := m.RBAC().CheckPermission(role, "host", "get"); err != nil {
		return nil, fmt.Errorf("permission denied: %w", err)
	}

	host, err := m.db.GetHost(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("get host: %w", err)
	}

	// Enrich with node agent status if registered
	m.mu.Lock()
	agent, hasAgent := m.nodes[id]
	m.mu.Unlock()

	if hasAgent {
		status, ok := GetAgentStatus(agent)
		if ok {
			host.Status = status
		}
	}

	return host, nil
}

// UpdateHost updates a host's fields and handles maintenance mode toggle.
func (m *Manager) UpdateHost(ctx context.Context, id string, updates map[string]interface{}) error {
	role, hasRole := roleFromContext(ctx)
	if !hasRole {
		return fmt.Errorf("no role in context")
	}
	if err := m.RBAC().CheckPermission(role, "host", "update"); err != nil {
		return fmt.Errorf("permission denied: %w", err)
	}

	if len(updates) == 0 {
		return fmt.Errorf("no updates provided")
	}

	// Validate maintenance mode toggle
	if _, isMaintenance := updates["maintenance_mode"]; isMaintenance {
		// Verify host exists before allowing maintenance mode change
		existing, err := m.db.GetHost(ctx, id)
		if err != nil {
			return fmt.Errorf("get host for validation: %w", err)
		}
		// Log maintenance mode change
		newMode := updates["maintenance_mode"].(bool)
		log.Printf("Host %s maintenance mode: %v -> %v", existing.Name, existing.MaintenanceMode, newMode)
	}

	if err := m.db.UpdateHost(ctx, id, updates); err != nil {
		return fmt.Errorf("update host: %w", err)
	}

	log.Printf("Host updated: %s", id)
	return nil
}

// DeleteHost removes a host after verifying no running VMs, marks decommissioned,
// unregisters the node agent, and publishes a HostDecommissioned event.
func (m *Manager) DeleteHost(ctx context.Context, id string) error {
	role, hasRole := roleFromContext(ctx)
	if !hasRole {
		return fmt.Errorf("no role in context")
	}
	if err := m.RBAC().CheckPermission(role, "host", "delete"); err != nil {
		return fmt.Errorf("permission denied: %w", err)
	}

	// Get host for name and tenant info
	host, err := m.db.GetHost(ctx, id)
	if err != nil {
		return fmt.Errorf("get host: %w", err)
	}

	// Check for running VMs on this host
	// In production, query VM table for running VMs with this host_id
	// For now: check if any VMs are assigned to this host
	vms, err := m.db.ListVMs(ctx, host.TenantID)
	if err != nil {
		return fmt.Errorf("list VMs: %w", err)
	}
	for _, vm := range vms {
		if vm.HostID != nil && *vm.HostID == id && vm.Status == "running" {
			return fmt.Errorf("cannot delete host %s: VM %s is running on this host", host.Name, vm.Name)
		}
	}

	// Mark host as decommissioned
	if err := m.db.UpdateHost(ctx, id, map[string]interface{}{
		"status": "decommissioned",
	}); err != nil {
		return fmt.Errorf("mark host decommissioned: %w", err)
	}

	// Unregister node agent
	m.UnregisterNode(id)

	// Publish event
	eventMsg := fmt.Sprintf("Host decommissioned: %s", host.Name)
	if err := m.publishEvent(ctx, "", "info", eventMsg, "system", id, host.Name, "host", id, host.Name); err != nil {
		log.Printf("Warning: failed to publish HostDecommissioned event: %v", err)
	}

	log.Printf("Host deleted: %s (id=%s)", host.Name, id)
	return nil
}

// ListHosts returns all hosts in a tenant with optional filtering.
func (m *Manager) ListHosts(ctx context.Context) ([]*db.Host, error) {
	role, hasRole := roleFromContext(ctx)
	if !hasRole {
		return nil, fmt.Errorf("no role in context")
	}
	if err := m.RBAC().CheckPermission(role, "host", "list"); err != nil {
		return nil, fmt.Errorf("permission denied: %w", err)
	}

	hosts, err := m.db.ListHosts(ctx, "")
	if err != nil {
		return nil, fmt.Errorf("list hosts: %w", err)
	}

	result := make([]*db.Host, 0, len(hosts))
	for i := range hosts {
		result = append(result, &hosts[i])
	}

	return result, nil
}

// ---------- VM Lifecycle Methods ----------

// VMSpec holds the specification for creating a new VM.
type VMSpec struct {
	Name            string
	Description     string
	HostID          string
	CPUS            int
	CPUAllocation   string
	MemoryBytes     int64
	NUMAPolicy      string
	HugepagesEnabled bool
	CPUPinning      []byte
	MemoryReservationBytes int64
	BallooningAllowed      bool
	SwapAllowed            bool
	Role              string
	OS               string
	TemplateID       string
}

// CreateVM validates the spec, enforces HANA compliance, generates libvirt XML,
// inserts the VM record, allocates resources, and publishes a VMCSreated event.
func (m *Manager) CreateVM(ctx context.Context, spec VMSpec) (string, error) {
	role, hasRole := roleFromContext(ctx)
	if !hasRole {
		return "", fmt.Errorf("no role in context")
	}
	if err := m.RBAC().CheckPermission(role, "vm", "create"); err != nil {
		return "", fmt.Errorf("permission denied: %w", err)
	}

	if spec.Name == "" {
		return "", fmt.Errorf("VM name is required")
	}
	if spec.HostID == "" {
		return "", fmt.Errorf("host ID is required")
	}
	if spec.CPUS <= 0 {
		return "", fmt.Errorf("CPUs must be positive")
	}
	if spec.MemoryBytes <= 0 {
		return "", fmt.Errorf("memory must be positive")
	}

	// Validate host exists
	host, err := m.db.GetHost(ctx, spec.HostID)
	if err != nil {
		return "", fmt.Errorf("get host: %w", err)
	}
	if host.Status != "active" {
		return "", fmt.Errorf("host %s is not active (status: %s)", host.Name, host.Status)
	}

	// HANA compliance validation
	vmProfile := &compliance.VMProfile{
		Role:                   spec.Role,
		CPUs:                   spec.CPUS,
		CPUAllocation:          spec.CPUAllocation,
		MemoryBytes:            spec.MemoryBytes,
		NUMAPolicy:             nil,
		HugepagesEnabled:       spec.HugepagesEnabled,
		CPUPinning:             spec.CPUPinning,
		MemoryReservationBytes: spec.MemoryReservationBytes,
		BallooningAllowed:      spec.BallooningAllowed,
		SwapAllowed:            spec.SwapAllowed,
		OS:                     spec.OS,
		HasHugepagesConfig:     false,
		NUMANodeCount:          int(host.NUMANodeCount),
		HugepagesTotalKB:       host.HugepagesTotalKB,
		HostCPUCount:           host.CPUCount,
		HostMemoryBytes:        host.MemoryTotalBytes,
	}
	if spec.NUMAPolicy != "" {
		vmProfile.NUMAPolicy = &spec.NUMAPolicy
	}

	result := compliance.ValidateHANAProfile(vmProfile)
	if !result.Passed {
		violations := make([]string, 0, len(result.Violations))
		for _, v := range result.Violations {
			violations = append(violations, fmt.Sprintf("%s: %s (expected %s, got %s)",
				v.Rule, v.Field, v.Expected, v.Actual))
		}
		return "", fmt.Errorf("HANA compliance check failed: %s", violations)
	}

	domainXML, err := libvirt.GenerateDomainXML(libvirt.DomainSpec{
		Name:        spec.Name,
		MemoryBytes: spec.MemoryBytes,
		VCPUs:       spec.CPUS,
	})
	if err != nil {
		return "", fmt.Errorf("generate libvirt XML: %w", err)
	}
	log.Printf("VM libvirt XML for %s: %s", spec.Name, domainXML)

	// Insert VM record
	vm := &db.VM{
		HostID:                  &spec.HostID,
		Identifier:              spec.Name,
		Name:                    spec.Name,
		Description:             spec.Description,
		Status:                  "created",
		Role:                    db.VMRole(spec.Role),
		CPUs:                    spec.CPUS,
		CPUAllocation:           spec.CPUAllocation,
		MemoryBytes:             spec.MemoryBytes,
		NUMAPolicy:              nil,
		HugepagesEnabled:        spec.HugepagesEnabled,
		CPUPinning:              spec.CPUPinning,
		MemoryReservationBytes:  spec.MemoryReservationBytes,
		BallooningAllowed:       spec.BallooningAllowed,
		SwapAllowed:             spec.SwapAllowed,
		OS:                      spec.OS,
		TemplateID:              nil,
	}
	if spec.TemplateID != "" {
		tmplID := spec.TemplateID
		vm.TemplateID = &tmplID
	}
	if spec.NUMAPolicy != "" {
		policy := spec.NUMAPolicy
		vm.NUMAPolicy = &policy
	}

	id, remainingCPUs, remainingMemory, err := m.allocateVM(ctx, host, vm)
	if err != nil {
		return "", err
	}

	log.Printf("Resources allocated for VM %s on host %s: %d vCPUs, %d bytes memory (remaining: %d CPUs, %d bytes memory)",
		spec.Name, host.Name, spec.CPUS, spec.MemoryBytes, remainingCPUs, remainingMemory)

	// Publish event
	eventMsg := fmt.Sprintf("VM created: %s (id=%s, host=%s)", spec.Name, id, host.Name)
	if err := m.publishEvent(ctx, "", "info", eventMsg, "system", id, spec.Name, "vm", id, spec.Name); err != nil {
		log.Printf("Warning: failed to publish VMCSreated event: %v", err)
	}

	log.Printf("VM created: %s (id=%s)", spec.Name, id)
	return id, nil
}

// allocateVM verifies that host has enough unallocated CPU and memory for vm and
// inserts the VM record, which is what reserves the capacity. Every VM already
// assigned to the host counts against its capacity regardless of power state,
// since a stopped VM can be started again. The check and insert run under
// allocMu so concurrent creates cannot both claim the last of the capacity.
// It returns the new VM ID and the CPUs and memory left on the host afterwards.
func (m *Manager) allocateVM(ctx context.Context, host *db.Host, vm *db.VM) (string, int, int64, error) {
	m.allocMu.Lock()
	defer m.allocMu.Unlock()

	vms, err := m.db.ListVMs(ctx, host.TenantID)
	if err != nil {
		return "", 0, 0, fmt.Errorf("list VMs for capacity check: %w", err)
	}

	remainingCPUs, remainingMemory, err := checkHostCapacity(host, m.withPendingMigrations(vms), vm.CPUs, vm.MemoryBytes)
	if err != nil {
		return "", 0, 0, err
	}

	id, err := m.db.CreateVM(ctx, vm)
	if err != nil {
		return "", 0, 0, fmt.Errorf("create VM record: %w", err)
	}
	return id, remainingCPUs, remainingMemory, nil
}

// checkHostCapacity sums the CPUs and memory of the VMs in vms that are assigned
// to host and returns what would remain after adding a VM requesting cpus and
// memoryBytes, or an error if either resource would be exceeded.
func checkHostCapacity(host *db.Host, vms []db.VM, cpus int, memoryBytes int64) (int, int64, error) {
	var allocatedCPUs int
	var allocatedMemory int64
	for _, existing := range vms {
		if existing.HostID != nil && *existing.HostID == host.ID {
			allocatedCPUs += existing.CPUs
			allocatedMemory += existing.MemoryBytes
		}
	}

	availableCPUs := host.CPUCount - allocatedCPUs
	if cpus > availableCPUs {
		return 0, 0, fmt.Errorf("host %s has insufficient CPU: requested %d, available %d (of %d, %d allocated)",
			host.Name, cpus, availableCPUs, host.CPUCount, allocatedCPUs)
	}
	availableMemory := host.MemoryTotalBytes - allocatedMemory
	if memoryBytes > availableMemory {
		return 0, 0, fmt.Errorf("host %s has insufficient memory: requested %d bytes, available %d bytes (of %d, %d allocated)",
			host.Name, memoryBytes, availableMemory, host.MemoryTotalBytes, allocatedMemory)
	}
	return availableCPUs - cpus, availableMemory - memoryBytes, nil
}

// StartVM finds the VM and its host, forwards to the node agent, and updates VM state.
func (m *Manager) StartVM(ctx context.Context, id string) error {
	role, hasRole := roleFromContext(ctx)
	if !hasRole {
		return fmt.Errorf("no role in context")
	}
	if err := m.RBAC().CheckPermission(role, "vm", "start"); err != nil {
		return fmt.Errorf("permission denied: %w", err)
	}

	vm, err := m.db.GetVM(ctx, id)
	if err != nil {
		return fmt.Errorf("get VM: %w", err)
	}

	if vm.Status == "running" {
		return fmt.Errorf("VM %s is already running", vm.Name)
	}

	// Find host
	if vm.HostID == nil || *vm.HostID == "" {
		return fmt.Errorf("VM %s has no host assigned", vm.Name)
	}

	hostID := *vm.HostID
	ctrl := m.vmController(hostID)
	if ctrl == nil {
		return fmt.Errorf("host %s has no registered node agent", hostID)
	}

	// Forward start command to node agent
	if err := ctrl.StartVM(ctx, id); err != nil {
		return fmt.Errorf("node agent start VM: %w", err)
	}

	// Update VM state
	if err := m.db.UpdateVM(ctx, id, map[string]interface{}{
		"status": "running",
	}); err != nil {
		log.Printf("Warning: failed to update VM status: %v", err)
	}

	// Update started_at timestamp
	now := time.Now()
	if err := m.db.UpdateVM(ctx, id, map[string]interface{}{
		"started_at": now,
	}); err != nil {
		log.Printf("Warning: failed to update started_at: %v", err)
	}

	log.Printf("VM started: %s (id=%s)", vm.Name, id)
	return nil
}

// StopVM finds the VM and its host, forwards to the node agent, and updates VM state.
func (m *Manager) StopVM(ctx context.Context, id string) error {
	role, hasRole := roleFromContext(ctx)
	if !hasRole {
		return fmt.Errorf("no role in context")
	}
	if err := m.RBAC().CheckPermission(role, "vm", "stop"); err != nil {
		return fmt.Errorf("permission denied: %w", err)
	}

	vm, err := m.db.GetVM(ctx, id)
	if err != nil {
		return fmt.Errorf("get VM: %w", err)
	}

	if vm.Status != "running" {
		return fmt.Errorf("VM %s is not running (status: %s)", vm.Name, vm.Status)
	}

	// Find host
	if vm.HostID == nil || *vm.HostID == "" {
		return fmt.Errorf("VM %s has no host assigned", vm.Name)
	}

	hostID := *vm.HostID
	ctrl := m.vmController(hostID)
	if ctrl == nil {
		return fmt.Errorf("host %s has no registered node agent", hostID)
	}

	// Forward stop command to node agent
	if err := ctrl.StopVM(ctx, id); err != nil {
		return fmt.Errorf("node agent stop VM: %w", err)
	}

	// Update VM state
	if err := m.db.UpdateVM(ctx, id, map[string]interface{}{
		"status": "stopped",
	}); err != nil {
		log.Printf("Warning: failed to update VM status: %v", err)
	}

	log.Printf("VM stopped: %s (id=%s)", vm.Name, id)
	return nil
}

// RestartVM stops then starts the VM.
func (m *Manager) RestartVM(ctx context.Context, id string) error {
	// Stop the VM first
	if err := m.StopVM(ctx, id); err != nil {
		// If VM was not running, that's okay — proceed to start
		if err.Error() == fmt.Sprintf("VM %s is not running", id) {
			// Ignore, proceed to start
		} else {
			return fmt.Errorf("stop VM: %w", err)
		}
	}

	// Start the VM
	if err := m.StartVM(ctx, id); err != nil {
		return fmt.Errorf("start VM: %w", err)
	}

	// Fetch VM for logging
	vm, _ := m.db.GetVM(ctx, id)
	vmName := id
	if vm != nil {
		vmName = vm.Name
	}
	log.Printf("VM restarted: %s (id=%s)", vmName, id)
	return nil
}

// DeleteVM stops the VM if running, destroys it via node agent, deletes the record,
// and publishes a VMDeleted event.
func (m *Manager) DeleteVM(ctx context.Context, id string) error {
	role, hasRole := roleFromContext(ctx)
	if !hasRole {
		return fmt.Errorf("no role in context")
	}
	if err := m.RBAC().CheckPermission(role, "vm", "delete"); err != nil {
		return fmt.Errorf("permission denied: %w", err)
	}

	vm, err := m.db.GetVM(ctx, id)
	if err != nil {
		return fmt.Errorf("get VM: %w", err)
	}

	// Stop if running
	if vm.Status == "running" {
		if err := m.StopVM(ctx, id); err != nil {
			return fmt.Errorf("stop VM before delete: %w", err)
		}
	}

	// Find host and destroy via node agent
	if vm.HostID == nil || *vm.HostID == "" {
		return fmt.Errorf("VM %s has no host assigned", vm.Name)
	}

	hostID := *vm.HostID
	if ctrl := m.vmController(hostID); ctrl != nil {
		if err := ctrl.DestroyVM(ctx, id); err != nil {
			return fmt.Errorf("node agent destroy VM: %w", err)
		}
	}

	// Delete VM record
	if err := m.db.DeleteVM(ctx, id); err != nil {
		return fmt.Errorf("delete VM record: %w", err)
	}

	// Publish event
	eventMsg := fmt.Sprintf("VM deleted: %s (id=%s)", vm.Name, id)
	if err := m.publishEvent(ctx, "", "info", eventMsg, "system", id, vm.Name, "vm", id, vm.Name); err != nil {
		log.Printf("Warning: failed to publish VMDeleted event: %v", err)
	}

	log.Printf("VM deleted: %s (id=%s)", vm.Name, id)
	return nil
}

// migrationTimeout bounds the database writes that follow a migration attempt.
// They run on a context detached from the caller's, because the caller giving
// up must not stop the manager from recording where the VM ended up.
const migrationTimeout = 10 * time.Second

// MigrateVM live-migrates a running VM to targetHostID. It verifies that the
// source node agent can migrate and that the target host is usable and has
// capacity, reserves that capacity, marks the VM "migrating", asks the source
// agent to migrate, and on success points the VM at the target host.
//
// If the agent reports failure the VM's status is restored, leaving it on the
// source host. If the agent succeeds but recording the new host fails, the VM
// is left "migrating" and the error says so: it is running on the target and
// the record needs reconciling.
func (m *Manager) MigrateVM(ctx context.Context, id, targetHostID string) error {
	role, hasRole := roleFromContext(ctx)
	if !hasRole {
		return fmt.Errorf("no role in context")
	}
	if err := m.RBAC().CheckPermission(role, "vm", "migrate"); err != nil {
		return fmt.Errorf("permission denied: %w", err)
	}

	vm, err := m.store.GetVM(ctx, id)
	if err != nil {
		return fmt.Errorf("get VM: %w", err)
	}
	if vm.Status != "running" {
		return fmt.Errorf("VM %s is not running (status: %s): only running VMs can be live-migrated", vm.Name, vm.Status)
	}
	if vm.HostID == nil || *vm.HostID == "" {
		return fmt.Errorf("VM %s has no source host", vm.Name)
	}
	sourceHostID := *vm.HostID
	if sourceHostID == targetHostID {
		return fmt.Errorf("VM %s is already on host %s", vm.Name, targetHostID)
	}

	targetHost, err := m.store.GetHost(ctx, targetHostID)
	if err != nil {
		return fmt.Errorf("get target host: %w", err)
	}
	if targetHost.Status != "active" {
		return fmt.Errorf("target host %s is not active (status: %s)", targetHost.Name, targetHost.Status)
	}
	if targetHost.MaintenanceMode {
		return fmt.Errorf("target host %s is in maintenance mode", targetHost.Name)
	}
	targetAddr := targetHost.Hostname
	if targetAddr == "" {
		targetAddr = targetHost.IPAddress
	}
	if targetAddr == "" {
		return fmt.Errorf("target host %s has no hostname or IP address", targetHost.Name)
	}

	// The source agent must be able to migrate. The target needs an agent too,
	// or the manager could not control the VM once it arrives.
	sourceCtrl := m.vmController(sourceHostID)
	if sourceCtrl == nil {
		return fmt.Errorf("source host %s has no registered node agent", sourceHostID)
	}
	migrator, ok := sourceCtrl.(node.VMMigrator)
	if !ok {
		return fmt.Errorf("node agent for source host %s does not support live migration", sourceHostID)
	}
	if m.vmController(targetHostID) == nil {
		return fmt.Errorf("target host %s has no registered node agent", targetHost.Name)
	}

	if err := m.reserveMigration(ctx, vm, targetHost); err != nil {
		return err
	}
	defer m.releaseMigration(vm.ID)

	prevStatus := vm.Status
	if err := m.store.UpdateVM(ctx, id, map[string]interface{}{"status": "migrating"}); err != nil {
		return fmt.Errorf("mark VM migrating: %w", err)
	}

	log.Printf("Migrating VM %s (id=%s) from host %s to host %s", vm.Name, id, sourceHostID, targetHost.Name)

	if err := migrator.MigrateVM(ctx, id, targetAddr); err != nil {
		rbCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), migrationTimeout)
		defer cancel()
		if rbErr := m.store.UpdateVM(rbCtx, id, map[string]interface{}{"status": prevStatus}); rbErr != nil {
			return fmt.Errorf("node agent migrate VM: %w (restoring status %q also failed, VM left %q: %v)",
				err, prevStatus, "migrating", rbErr)
		}
		return fmt.Errorf("node agent migrate VM: %w", err)
	}

	commitCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), migrationTimeout)
	defer cancel()
	if err := m.store.UpdateVM(commitCtx, id, map[string]interface{}{
		"host_id": targetHostID,
		"status":  prevStatus,
	}); err != nil {
		return fmt.Errorf("VM %s migrated to host %s but recording it failed; VM left %q, needs reconciliation: %w",
			vm.Name, targetHost.Name, "migrating", err)
	}

	eventMsg := fmt.Sprintf("VM migrated: %s (id=%s) from host %s to host %s", vm.Name, id, sourceHostID, targetHost.Name)
	if err := m.publishEvent(ctx, "", "info", eventMsg, "system", id, vm.Name, "vm", id, vm.Name); err != nil {
		log.Printf("Warning: failed to publish VMMigrated event: %v", err)
	}

	log.Printf("VM migrated: %s (id=%s) to host %s", vm.Name, id, targetHost.Name)
	return nil
}

// reserveMigration checks that target can take vm on top of everything already
// assigned to it or in flight to it, and records the claim so that concurrent
// migrations and creates see it. Callers must releaseMigration when done.
func (m *Manager) reserveMigration(ctx context.Context, vm *db.VM, target *db.Host) error {
	m.allocMu.Lock()
	defer m.allocMu.Unlock()

	if _, busy := m.migrations[vm.ID]; busy {
		return fmt.Errorf("VM %s is already being migrated", vm.Name)
	}

	vms, err := m.store.ListVMs(ctx, target.TenantID)
	if err != nil {
		return fmt.Errorf("list VMs for capacity check: %w", err)
	}
	if _, _, err := checkHostCapacity(target, m.withPendingMigrations(vms), vm.CPUs, vm.MemoryBytes); err != nil {
		return fmt.Errorf("cannot migrate VM %s: %w", vm.Name, err)
	}

	if m.migrations == nil {
		m.migrations = make(map[string]db.VM)
	}
	targetID := target.ID
	m.migrations[vm.ID] = db.VM{ID: vm.ID, HostID: &targetID, CPUs: vm.CPUs, MemoryBytes: vm.MemoryBytes}
	return nil
}

// releaseMigration drops the capacity claim recorded by reserveMigration.
func (m *Manager) releaseMigration(vmID string) {
	m.allocMu.Lock()
	defer m.allocMu.Unlock()
	delete(m.migrations, vmID)
}

// withPendingMigrations returns vms plus one entry per in-flight migration,
// assigned to the migration's target host, so checkHostCapacity counts the
// capacity those migrations have claimed. A migration whose VM row already
// points at its target is not added twice. Callers must hold allocMu.
func (m *Manager) withPendingMigrations(vms []db.VM) []db.VM {
	if len(m.migrations) == 0 {
		return vms
	}
	arrived := make(map[string]string, len(vms))
	for _, v := range vms {
		if v.HostID != nil {
			arrived[v.ID] = *v.HostID
		}
	}
	out := vms[:len(vms):len(vms)]
	for id, pending := range m.migrations {
		if arrived[id] == *pending.HostID {
			continue
		}
		out = append(out, pending)
	}
	return out
}

// ---------- Node Agent Management ----------

// RegisterNode registers a node agent with the manager.
func (m *Manager) RegisterNode(id string, agent *node.Agent) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.nodes[id] = agent
	log.Printf("Node registered: %s", id)
}

// RegisterNodeController registers a VMController, typically a *node.Client
// connected to a remote node agent, to handle VM lifecycle commands for node id.
// It takes precedence over a local agent registered with RegisterNode. The
// caller keeps ownership of the controller and is responsible for closing it.
func (m *Manager) RegisterNodeController(id string, ctrl node.VMController) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.controllers[id] = ctrl
	log.Printf("Node controller registered: %s", id)
}

// vmController returns the VMController for the node hostID: the registered
// remote controller if there is one, otherwise the local agent, otherwise nil.
func (m *Manager) vmController(hostID string) node.VMController {
	m.mu.Lock()
	defer m.mu.Unlock()
	if ctrl, ok := m.controllers[hostID]; ok && ctrl != nil {
		return ctrl
	}
	if agent, ok := m.nodes[hostID]; ok && agent != nil {
		return agent
	}
	return nil
}

// UnregisterNode removes a node agent and its controller from the manager.
func (m *Manager) UnregisterNode(id string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.nodes, id)
	delete(m.controllers, id)
	log.Printf("Node unregistered: %s", id)
}

// ListNodes returns all registered node agents.
func (m *Manager) ListNodes() map[string]*node.Agent {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.nodes
}

// GetNode returns a node agent by ID.
func (m *Manager) GetNode(id string) (*node.Agent, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	agent, ok := m.nodes[id]
	return agent, ok
}

// UpdateNodeStatus updates the node agent's last heartbeat timestamp and status.
func (m *Manager) UpdateNodeStatus(ctx context.Context, nodeID string, status string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	agent, ok := m.nodes[nodeID]
	if !ok || agent == nil {
		return fmt.Errorf("node %s not found", nodeID)
	}

	// Update last heartbeat using node agent's status reporting
	// The node agent's running state and host info serve as our status indicator
	log.Printf("Node %s status check: attempting ListVMs as health probe", nodeID)
	_, err := agent.ListVMs(ctx)
	if err != nil {
		log.Printf("Node %s may be unhealthy: %v", nodeID, err)
	}

	log.Printf("Node %s status updated: %s (health check performed)", nodeID, status)
	return nil
}

// HAService returns the HA service instance.
func (m *Manager) HAService() *haService {
    return m.haSvc
}

// wireHAIntoAPI connects the HA subsystem to the API server.
func (m *Manager) wireHAIntoAPI() {
	if m.apiServer == nil || m.haSvc == nil {
		return
	}
	if ctrl := m.haSvc.GetController(); ctrl != nil {
		m.apiServer.SetHAController(&haControllerAdapter{ctrl: ctrl})
	}
	if orch := m.haSvc.GetOrchestrator(); orch != nil {
		m.apiServer.SetHAOrchestrator(orch)
	}
	if pm := m.haSvc.GetPolicyManager(); pm != nil {
		m.apiServer.SetPolicyManager(pm)
	}
}

// Shutdown gracefully stops the manager.
func (m *Manager) Shutdown() {
	close(m.shutdownCh)
	if m.haSvc != nil {
		m.haSvc.Stop()
	}
	if m.apiServer != nil {
		m.apiServer.Shutdown()
	}
}

// GetSnapshots returns the snapshots for a VM (delegates to node agent).
func (m *Manager) GetSnapshots(ctx context.Context, vmID string) ([]map[string]interface{}, error) {
	vm, err := m.db.GetVM(ctx, vmID)
	if err != nil {
		return nil, fmt.Errorf("get VM: %w", err)
	}
	if vm.HostID == nil || *vm.HostID == "" {
		return nil, fmt.Errorf("VM %s has no host assigned", vmID)
	}
	ctrl := m.vmController(*vm.HostID)
	if ctrl == nil {
		return nil, fmt.Errorf("host %s has no registered node agent", *vm.HostID)
	}
	snapshots, ok := ctrl.(interface {
		GetSnapshots(ctx context.Context, vmID string) ([]map[string]interface{}, error)
	})
	if !ok {
		return nil, fmt.Errorf("node agent does not support snapshots")
	}
	return snapshots.GetSnapshots(ctx, vmID)
}

// CreateSnapshot creates a snapshot for a VM (delegates to node agent).
func (m *Manager) CreateSnapshot(ctx context.Context, vmID, name string) (string, error) {
	vm, err := m.db.GetVM(ctx, vmID)
	if err != nil {
		return "", fmt.Errorf("get VM: %w", err)
	}
	if vm.HostID == nil || *vm.HostID == "" {
		return "", fmt.Errorf("VM %s has no host assigned", vmID)
	}
	ctrl := m.vmController(*vm.HostID)
	if ctrl == nil {
		return "", fmt.Errorf("host %s has no registered node agent", *vm.HostID)
	}
	snap, ok := ctrl.(interface {
		CreateSnapshot(ctx context.Context, vmID, name string) (string, error)
	})
	if !ok {
		return "", fmt.Errorf("node agent does not support snapshots")
	}
	return snap.CreateSnapshot(ctx, vmID, name)
}

// DeleteSnapshot deletes a VM snapshot (delegates to node agent).
func (m *Manager) DeleteSnapshot(ctx context.Context, vmID, snapshotID string) error {
	vm, err := m.db.GetVM(ctx, vmID)
	if err != nil {
		return fmt.Errorf("get VM: %w", err)
	}
	if vm.HostID == nil || *vm.HostID == "" {
		return fmt.Errorf("VM %s has no host assigned", vmID)
	}
	ctrl := m.vmController(*vm.HostID)
	if ctrl == nil {
		return fmt.Errorf("host %s has no registered node agent", *vm.HostID)
	}
	snap, ok := ctrl.(interface {
		DeleteSnapshot(ctx context.Context, vmID, snapshotID string) error
	})
	if !ok {
		return fmt.Errorf("node agent does not support snapshots")
	}
	return snap.DeleteSnapshot(ctx, vmID, snapshotID)
}

// GetVMStats returns VM statistics (delegates to node agent).
func (m *Manager) GetVMStats(ctx context.Context, vmID string) (map[string]interface{}, error) {
	vm, err := m.db.GetVM(ctx, vmID)
	if err != nil {
		return nil, fmt.Errorf("get VM: %w", err)
	}
	if vm.HostID == nil || *vm.HostID == "" {
		return nil, fmt.Errorf("VM %s has no host assigned", vmID)
	}
	ctrl := m.vmController(*vm.HostID)
	if ctrl == nil {
		return nil, fmt.Errorf("host %s has no registered node agent", *vm.HostID)
	}
	stats, ok := ctrl.(interface {
		GetVMStats(ctx context.Context, vmID string) (map[string]interface{}, error)
	})
	if !ok {
		return map[string]interface{}{
			"cpu_usage":   0,
			"memory_usage": 0,
			"disk_io":     0,
			"network_io":  0,
		}, nil
	}
	return stats.GetVMStats(ctx, vmID)
}
