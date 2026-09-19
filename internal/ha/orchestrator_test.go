package ha

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"
)

// ---------- Mock implementations ----------

type mockVMProvider struct {
	mu              sync.Mutex
	vmsByHost       map[string][]VM
	vmHosts         map[string]string
	policies        map[string]HAPolicy
	getVMsByHostFn  func(ctx context.Context, hostID string) ([]VM, error)
	getVMFn         func(ctx context.Context, vmID string) (*VM, error)
	updateVMHostFn  func(ctx context.Context, vmID, newHostID string) error
	getHAPolicyFn   func(ctx context.Context, vmID string) (*HAPolicy, error)
	setHAPolicyFn   func(ctx context.Context, vmID string, policy HAPolicy) error
	listHAPoliciesFn func(ctx context.Context) (map[string]HAPolicy, error)
}

func newMockVMProvider() *mockVMProvider {
	return &mockVMProvider{
		vmsByHost: make(map[string][]VM),
		vmHosts:   make(map[string]string),
		policies:  make(map[string]HAPolicy),
	}
}

func (m *mockVMProvider) GetVMsByHost(ctx context.Context, hostID string) ([]VM, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.getVMsByHostFn != nil {
		return m.getVMsByHostFn(ctx, hostID)
	}
	vms := m.vmsByHost[hostID]
	result := make([]VM, len(vms))
	copy(result, vms)
	return result, nil
}

func (m *mockVMProvider) GetVM(ctx context.Context, vmID string) (*VM, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.getVMFn != nil {
		return m.getVMFn(ctx, vmID)
	}
	for _, vms := range m.vmsByHost {
		for _, vm := range vms {
			if vm.ID == vmID {
				v := vm
				return &v, nil
			}
		}
	}
	return nil, nil
}

func (m *mockVMProvider) UpdateVMHost(ctx context.Context, vmID, newHostID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.updateVMHostFn != nil {
		return m.updateVMHostFn(ctx, vmID, newHostID)
	}
	m.vmHosts[vmID] = newHostID
	for hostID, vms := range m.vmsByHost {
		for i, vm := range vms {
			if vm.ID == vmID {
				m.vmsByHost[hostID][i].HostID = newHostID
			}
		}
	}
	return nil
}

func (m *mockVMProvider) GetHAPolicy(ctx context.Context, vmID string) (*HAPolicy, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.getHAPolicyFn != nil {
		return m.getHAPolicyFn(ctx, vmID)
	}
	p, ok := m.policies[vmID]
	if !ok {
		defaultP := DefaultHAPolicy(vmID)
		return &defaultP, nil
	}
	return &p, nil
}

func (m *mockVMProvider) SetHAPolicy(ctx context.Context, vmID string, policy HAPolicy) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.setHAPolicyFn != nil {
		return m.setHAPolicyFn(ctx, vmID, policy)
	}
	m.policies[vmID] = policy
	return nil
}

func (m *mockVMProvider) ListHAPolicies(ctx context.Context) (map[string]HAPolicy, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.listHAPoliciesFn != nil {
		return m.listHAPoliciesFn(ctx)
	}
	result := make(map[string]HAPolicy, len(m.policies))
	for k, v := range m.policies {
		result[k] = v
	}
	return result, nil
}

type mockHostProvider struct {
	mu          sync.Mutex
	hosts       map[string]Host
	listHostsFn func(ctx context.Context, status string) ([]Host, error)
	getHostFn   func(ctx context.Context, hostID string) (*Host, error)
}

func newMockHostProvider() *mockHostProvider {
	return &mockHostProvider{
		hosts: make(map[string]Host),
	}
}

func (m *mockHostProvider) GetHost(ctx context.Context, hostID string) (*Host, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.getHostFn != nil {
		return m.getHostFn(ctx, hostID)
	}
	h, ok := m.hosts[hostID]
	if !ok {
		return nil, nil
	}
	return &h, nil
}

func (m *mockHostProvider) ListHosts(ctx context.Context, status string) ([]Host, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.listHostsFn != nil {
		return m.listHostsFn(ctx, status)
	}
	var result []Host
	for _, h := range m.hosts {
		if status == "" || h.Status == status {
			result = append(result, h)
		}
	}
	return result, nil
}

type mockVMRestarter struct {
	mu          sync.Mutex
	startedVMs  []string
	startVMFn   func(ctx context.Context, vm VM, target Host) error
	getStatusFn func(ctx context.Context, vmID string) (string, error)
}

func newMockVMRestarter() *mockVMRestarter {
	return &mockVMRestarter{
		startedVMs: make([]string, 0),
	}
}

func (m *mockVMRestarter) StartVM(ctx context.Context, vm VM, target Host) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.startVMFn != nil {
		return m.startVMFn(ctx, vm, target)
	}
	m.startedVMs = append(m.startedVMs, vm.ID)
	return nil
}

func (m *mockVMRestarter) GetStartStatus(ctx context.Context, vmID string) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.getStatusFn != nil {
		return m.getStatusFn(ctx, vmID)
	}
	return "started", nil
}

type mockEventPublisher struct {
	mu     sync.Mutex
	events []PublishEvent
}

type PublishEvent struct {
	EventType    string
	Severity     string
	Message      string
	ActorType    string
	ActorID      string
	ActorName    string
	ResourceType string
	ResourceID   string
	ResourceName string
}

func newMockEventPublisher() *mockEventPublisher {
	return &mockEventPublisher{
		events: make([]PublishEvent, 0),
	}
}

func (m *mockEventPublisher) Publish(ctx context.Context, eventType, severity, message, actorType, actorID, actorName, resourceType, resourceID, resourceName string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.events = append(m.events, PublishEvent{
		EventType:    eventType,
		Severity:     severity,
		Message:      message,
		ActorType:    actorType,
		ActorID:      actorID,
		ActorName:    actorName,
		ResourceType: resourceType,
		ResourceID:   resourceID,
		ResourceName: resourceName,
	})
	return nil
}

type orchMockFencer struct {
	mu      sync.Mutex
	fenced  []string
	fenceFn func(ctx context.Context, nodeID string) error
}

func newOrchMockFencer() *orchMockFencer {
	return &orchMockFencer{
		fenced: make([]string, 0),
	}
}

func (m *orchMockFencer) Fence(ctx context.Context, nodeID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.fenceFn != nil {
		return m.fenceFn(ctx, nodeID)
	}
	m.fenced = append(m.fenced, nodeID)
	return nil
}

func (m *orchMockFencer) GetPowerState(ctx context.Context, nodeID string) (string, error) {
	return "off", nil
}

func (m *orchMockFencer) GetMethod() FenceMethod {
	return FenceMethodIPMI
}

// ---------- Test helpers ----------

func makeHost(id string, status string, maintenance bool) Host {
	return Host{
		ID:               id,
		Name:             id,
		Status:           status,
		CPUCount:         32,
		MemoryTotalBytes: 64 * 1024 * 1024 * 1024,
		MemoryUsedBytes:  8 * 1024 * 1024 * 1024,
		VMCount:          0,
		Labels:           map[string]string{"env": "prod"},
		MaintenanceMode:  maintenance,
		CurrentVMs:       []string{},
	}
}

func makeVM(id, hostID string, cpus int, mem int64) VM {
	return VM{
		ID:          id,
		Name:        id,
		HostID:      hostID,
		CPUs:        cpus,
		MemoryBytes: mem,
		Labels:      map[string]string{"env": "prod"},
	}
}

// ---------- NewOrchestrator validation tests ----------

func TestNewOrchestrator_Validation_VMProviderRequired(t *testing.T) {
	cfg := OrchestratorConfig{
		HostProvider: newMockHostProvider(),
	}
	_, err := NewOrchestrator(cfg)
	if err == nil {
		t.Error("Expected error when VMProvider is nil")
	}
}

func TestNewOrchestrator_Validation_HostProviderRequired(t *testing.T) {
	cfg := OrchestratorConfig{
		VMProvider: newMockVMProvider(),
	}
	_, err := NewOrchestrator(cfg)
	if err == nil {
		t.Error("Expected error when HostProvider is nil")
	}
}

func TestNewOrchestrator_Defaults(t *testing.T) {
	vmProvider := newMockVMProvider()
	hostProvider := newMockHostProvider()

	cfg := OrchestratorConfig{
		VMProvider:  vmProvider,
		HostProvider: hostProvider,
	}
	o, err := NewOrchestrator(cfg)
	if err != nil {
		t.Fatalf("NewOrchestrator failed: %v", err)
	}
	if o.scheduler == nil {
		t.Error("Expected default scheduler to be set")
	}
	if o.maxHistory != 100 {
		t.Errorf("Expected default maxHistory=100, got %d", o.maxHistory)
	}
	if o.policy.PolicyType == "" {
		t.Error("Expected default policy to be set")
	}
}

func TestNewOrchestrator_CustomValues(t *testing.T) {
	vmProvider := newMockVMProvider()
	hostProvider := newMockHostProvider()
	scheduler := NewDefaultScheduler()
	fencer := newOrchMockFencer()
	restarter := newMockVMRestarter()
	publisher := newMockEventPublisher()
	policy := Policy{PolicyType: "binpack"}

	cfg := OrchestratorConfig{
		VMProvider:     vmProvider,
		HostProvider:   hostProvider,
		Scheduler:      scheduler,
		Fencer:         fencer,
		VMRestarter:    restarter,
		EventPublisher: publisher,
		Policy:         policy,
		MaxHistory:     50,
	}
	o, err := NewOrchestrator(cfg)
	if err != nil {
		t.Fatalf("NewOrchestrator failed: %v", err)
	}
	if o.scheduler != scheduler {
		t.Error("Expected custom scheduler to be preserved")
	}
	if o.maxHistory != 50 {
		t.Errorf("Expected maxHistory=50, got %d", o.maxHistory)
	}
	if o.fencer != fencer {
		t.Error("Expected custom fencer to be preserved")
	}
	if o.vmRestarter != restarter {
		t.Error("Expected custom vmRestarter to be preserved")
	}
	if o.eventPublisher != publisher {
		t.Error("Expected custom eventPublisher to be preserved")
	}
	if o.policy.PolicyType != "binpack" {
		t.Errorf("Expected policy type 'binpack', got '%s'", o.policy.PolicyType)
	}
}

// ---------- HandleHostFailure tests ----------

func TestHandleHostFailure_NoVMs(t *testing.T) {
	vmProvider := newMockVMProvider()
	hostProvider := newMockHostProvider()
	publisher := newMockEventPublisher()

	cfg := OrchestratorConfig{
		VMProvider:     vmProvider,
		HostProvider:   hostProvider,
		EventPublisher: publisher,
		Fencer:         newOrchMockFencer(),
	}
	o, err := NewOrchestrator(cfg)
	if err != nil {
		t.Fatalf("NewOrchestrator failed: %v", err)
	}

	ctx := context.Background()
	err = o.HandleHostFailure(ctx, "host-1")
	if err != nil {
		t.Errorf("Expected no error with no VMs, got: %v", err)
	}

	// Should complete with no active failovers
	if len(o.GetActiveFailovers()) != 0 {
		t.Errorf("Expected 0 active failovers, got %d", len(o.GetActiveFailovers()))
	}

	// Should have history entry
	history := o.GetFailoverHistory(10)
	if len(history) != 1 {
		t.Fatalf("Expected 1 history record, got %d", len(history))
	}
	if history[0].State != "complete" {
		t.Errorf("Expected state 'complete', got '%s'", history[0].State)
	}
	if history[0].VMCount != 0 {
		t.Errorf("Expected VMCount=0, got %d", history[0].VMCount)
	}
}

func TestHandleHostFailure_NoAutoRestartVMs(t *testing.T) {
	vmProvider := newMockVMProvider()
	hostProvider := newMockHostProvider()

	// Add VMs with manual policy (no auto-restart)
	vm1 := makeVM("vm-1", "host-1", 4, 8*1024*1024*1024)
	vmProvider.vmsByHost["host-1"] = []VM{vm1}
	vmProvider.policies["vm-1"] = HAPolicy{VMID: "vm-1", Mode: HAModeManual, Priority: 100}

	cfg := OrchestratorConfig{
		VMProvider:   vmProvider,
		HostProvider: hostProvider,
		Fencer:       newOrchMockFencer(),
	}
	o, err := NewOrchestrator(cfg)
	if err != nil {
		t.Fatalf("NewOrchestrator failed: %v", err)
	}

	ctx := context.Background()
	err = o.HandleHostFailure(ctx, "host-1")
	if err != nil {
		t.Errorf("Expected no error with no auto-restart VMs, got: %v", err)
	}

	history := o.GetFailoverHistory(10)
	if len(history) != 1 {
		t.Fatalf("Expected 1 history record, got %d", len(history))
	}
	if history[0].State != "complete" {
		t.Errorf("Expected state 'complete', got '%s'", history[0].State)
	}
	if history[0].VMCount != 0 {
		t.Errorf("Expected VMCount=0, got %d", history[0].VMCount)
	}
}

func TestHandleHostFailure_WithAutoRestartVMs(t *testing.T) {
	vmProvider := newMockVMProvider()
	hostProvider := newMockHostProvider()
	restarter := newMockVMRestarter()
	fencer := newOrchMockFencer()
	publisher := newMockEventPublisher()

	// Add VMs with auto policy
	vm1 := makeVM("vm-1", "host-1", 4, 8*1024*1024*1024)
	vm2 := makeVM("vm-2", "host-1", 2, 4*1024*1024*1024)
	vmProvider.vmsByHost["host-1"] = []VM{vm1, vm2}

	// Add target hosts
	hostProvider.hosts["host-2"] = makeHost("host-2", "online", false)
	hostProvider.hosts["host-3"] = makeHost("host-3", "online", false)

	cfg := OrchestratorConfig{
		VMProvider:     vmProvider,
		HostProvider:   hostProvider,
		VMRestarter:    restarter,
		Fencer:         fencer,
		EventPublisher: publisher,
	}
	o, err := NewOrchestrator(cfg)
	if err != nil {
		t.Fatalf("NewOrchestrator failed: %v", err)
	}

	ctx := context.Background()
	err = o.HandleHostFailure(ctx, "host-1")
	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}

	// Both VMs should have been started
	restarter.mu.Lock()
	startedCount := len(restarter.startedVMs)
	restarter.mu.Unlock()
	if startedCount != 2 {
		t.Errorf("Expected 2 VMs restarted, got %d", startedCount)
	}

	// Fencer should have been called
	fencer.mu.Lock()
	fencedCount := len(fencer.fenced)
	fencer.mu.Unlock()
	if fencedCount != 1 {
		t.Errorf("Expected 1 fence operation, got %d", fencedCount)
	}

	// Events should be published
	publisher.mu.Lock()
	eventCount := len(publisher.events)
	publisher.mu.Unlock()
	if eventCount < 2 {
		t.Errorf("Expected at least 2 events, got %d", eventCount)
	}

	// History should show complete
	history := o.GetFailoverHistory(10)
	if len(history) != 1 {
		t.Fatalf("Expected 1 history record, got %d", len(history))
	}
	if history[0].State != "complete" {
		t.Errorf("Expected state 'complete', got '%s'", history[0].State)
	}
	if history[0].VMCount != 2 {
		t.Errorf("Expected VMCount=2, got %d", history[0].VMCount)
	}
	if history[0].VMRestarted != 2 {
		t.Errorf("Expected VMRestarted=2, got %d", history[0].VMRestarted)
	}
}

func TestHandleHostFailure_NoAvailableTargets(t *testing.T) {
	vmProvider := newMockVMProvider()
	hostProvider := newMockHostProvider()
	publisher := newMockEventPublisher()

	// Add VM with auto policy
	vm1 := makeVM("vm-1", "host-1", 4, 8*1024*1024*1024)
	vmProvider.vmsByHost["host-1"] = []VM{vm1}

	// No other hosts available
	hostProvider.hosts["host-1"] = makeHost("host-1", "online", false)

	cfg := OrchestratorConfig{
		VMProvider:     vmProvider,
		HostProvider:   hostProvider,
		Fencer:         newOrchMockFencer(),
		EventPublisher: publisher,
	}
	o, err := NewOrchestrator(cfg)
	if err != nil {
		t.Fatalf("NewOrchestrator failed: %v", err)
	}

	ctx := context.Background()
	err = o.HandleHostFailure(ctx, "host-1")
	if err == nil {
		t.Error("Expected error when no target hosts available")
	}

	history := o.GetFailoverHistory(10)
	if len(history) != 1 {
		t.Fatalf("Expected 1 history record, got %d", len(history))
	}
	if history[0].State != "failed" {
		t.Errorf("Expected state 'failed', got '%s'", history[0].State)
	}
	if history[0].Error == "" {
		t.Error("Expected error message in failed failover")
	}
}

func TestHandleHostFailure_MaintenanceHostsExcluded(t *testing.T) {
	vmProvider := newMockVMProvider()
	hostProvider := newMockHostProvider()

	// Add VM with auto policy
	vm1 := makeVM("vm-1", "host-1", 4, 8*1024*1024*1024)
	vmProvider.vmsByHost["host-1"] = []VM{vm1}

	// Only maintenance mode hosts available
	hostProvider.hosts["host-1"] = makeHost("host-1", "online", false)
	hostProvider.hosts["host-2"] = makeHost("host-2", "online", true) // maintenance

	cfg := OrchestratorConfig{
		VMProvider:   vmProvider,
		HostProvider: hostProvider,
		Fencer:       newOrchMockFencer(),
	}
	o, err := NewOrchestrator(cfg)
	if err != nil {
		t.Fatalf("NewOrchestrator failed: %v", err)
	}

	ctx := context.Background()
	err = o.HandleHostFailure(ctx, "host-1")
	if err == nil {
		t.Error("Expected error when only maintenance hosts available")
	}
}

func TestHandleHostFailure_TargetCapacityExceeded(t *testing.T) {
	vmProvider := newMockVMProvider()
	hostProvider := newMockHostProvider()

	// Add many VMs requiring lots of memory
	var vms []VM
	for i := 0; i < 10; i++ {
		vm := makeVM(fmt.Sprintf("vm-%d", i), "host-1", 16, 16*1024*1024*1024)
		vms = append(vms, vm)
	}
	vmProvider.vmsByHost["host-1"] = vms

	// Target host with limited memory
	targetHost := Host{
		ID:               "host-2",
		Name:             "host-2",
		Status:           "online",
		CPUCount:         32,
		MemoryTotalBytes: 32 * 1024 * 1024 * 1024,
		MemoryUsedBytes:  16 * 1024 * 1024 * 1024, // Only 16GB free
	}
	hostProvider.hosts["host-2"] = targetHost

	cfg := OrchestratorConfig{
		VMProvider:   vmProvider,
		HostProvider: hostProvider,
		Fencer:       newOrchMockFencer(),
	}
	o, err := NewOrchestrator(cfg)
	if err != nil {
		t.Fatalf("NewOrchestrator failed: %v", err)
	}

	ctx := context.Background()
	err = o.HandleHostFailure(ctx, "host-1")
	// Should succeed (scheduler handles partial placement)
	if err != nil {
		t.Logf("Note: HandleHostFailure returned error (expected with tight capacity): %v", err)
	}

	// Check history
	history := o.GetFailoverHistory(10)
	if len(history) != 1 {
		t.Fatalf("Expected 1 history record, got %d", len(history))
	}
}

func TestHandleHostFailure_AlreadyInProgress(t *testing.T) {
	vmProvider := newMockVMProvider()
	hostProvider := newMockHostProvider()

	// Add VM
	vm1 := makeVM("vm-1", "host-1", 4, 8*1024*1024*1024)
	vmProvider.vmsByHost["host-1"] = []VM{vm1}
	hostProvider.hosts["host-2"] = makeHost("host-2", "online", false)

	// Make VM lookup slow so we can trigger concurrent access
	vmProvider.getHAPolicyFn = func(ctx context.Context, vmID string) (*HAPolicy, error) {
		time.Sleep(200 * time.Millisecond)
		defaultP := DefaultHAPolicy(vmID)
		return &defaultP, nil
	}

	cfg := OrchestratorConfig{
		VMProvider:   vmProvider,
		HostProvider: hostProvider,
		Fencer:       newOrchMockFencer(),
	}
	o, err := NewOrchestrator(cfg)
	if err != nil {
		t.Fatalf("NewOrchestrator failed: %v", err)
	}

	ctx := context.Background()

	// First call should start processing
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		o.HandleHostFailure(ctx, "host-1")
	}()

	// Give time for first call to register
	time.Sleep(50 * time.Millisecond)

	// Second call should fail with "already in progress"
	err = o.HandleHostFailure(ctx, "host-1")
	if err == nil {
		t.Error("Expected error when failover already in progress")
	}

	wg.Wait()
}

func TestHandleHostFailure_GetVMsError(t *testing.T) {
	vmProvider := newMockVMProvider()
	hostProvider := newMockHostProvider()
	publisher := newMockEventPublisher()

	vmProvider.getVMsByHostFn = func(ctx context.Context, hostID string) ([]VM, error) {
		return nil, fmt.Errorf("database connection failed")
	}

	cfg := OrchestratorConfig{
		VMProvider:     vmProvider,
		HostProvider:   hostProvider,
		Fencer:         newOrchMockFencer(),
		EventPublisher: publisher,
	}
	o, err := NewOrchestrator(cfg)
	if err != nil {
		t.Fatalf("NewOrchestrator failed: %v", err)
	}

	ctx := context.Background()
	err = o.HandleHostFailure(ctx, "host-1")
	if err == nil {
		t.Error("Expected error when GetVMsByHost fails")
	}

	history := o.GetFailoverHistory(10)
	if len(history) != 1 {
		t.Fatalf("Expected 1 history record, got %d", len(history))
	}
	if history[0].State != "failed" {
		t.Errorf("Expected state 'failed', got '%s'", history[0].State)
	}
}

func TestHandleHostFailure_FencerCalled(t *testing.T) {
	vmProvider := newMockVMProvider()
	hostProvider := newMockHostProvider()
	fencer := newOrchMockFencer()

	// Add VM with auto policy
	vm1 := makeVM("vm-1", "host-1", 4, 8*1024*1024*1024)
	vmProvider.vmsByHost["host-1"] = []VM{vm1}
	hostProvider.hosts["host-2"] = makeHost("host-2", "online", false)

	cfg := OrchestratorConfig{
		VMProvider:   vmProvider,
		HostProvider: hostProvider,
		Fencer:       fencer,
	}
	o, err := NewOrchestrator(cfg)
	if err != nil {
		t.Fatalf("NewOrchestrator failed: %v", err)
	}

	ctx := context.Background()
	o.HandleHostFailure(ctx, "host-1")

	fencer.mu.Lock()
	defer fencer.mu.Unlock()
	if len(fencer.fenced) != 1 || fencer.fenced[0] != "host-1" {
		t.Errorf("Expected fencer to be called for host-1, got %v", fencer.fenced)
	}
}

func TestHandleHostFailure_NoFencerConfigured(t *testing.T) {
	vmProvider := newMockVMProvider()
	hostProvider := newMockHostProvider()

	// Add VM with auto policy
	vm1 := makeVM("vm-1", "host-1", 4, 8*1024*1024*1024)
	vmProvider.vmsByHost["host-1"] = []VM{vm1}
	hostProvider.hosts["host-2"] = makeHost("host-2", "online", false)

	cfg := OrchestratorConfig{
		VMProvider:   vmProvider,
		HostProvider: hostProvider,
		// No fencer configured
	}
	o, err := NewOrchestrator(cfg)
	if err != nil {
		t.Fatalf("NewOrchestrator failed: %v", err)
	}

	ctx := context.Background()
	err = o.HandleHostFailure(ctx, "host-1")
	if err != nil {
		t.Errorf("Expected no error without fencer, got: %v", err)
	}
}

func TestHandleHostFailure_SchedulerError(t *testing.T) {
	vmProvider := newMockVMProvider()
	hostProvider := newMockHostProvider()

	// Add VM with auto policy
	vm1 := makeVM("vm-1", "host-1", 1000, 999*1024*1024*1024) // Impossible requirements
	vmProvider.vmsByHost["host-1"] = []VM{vm1}

	// Target host that can't fit the VM
	hostProvider.hosts["host-2"] = makeHost("host-2", "online", false)

	cfg := OrchestratorConfig{
		VMProvider:   vmProvider,
		HostProvider: hostProvider,
		Fencer:       newOrchMockFencer(),
	}
	o, err := NewOrchestrator(cfg)
	if err != nil {
		t.Fatalf("NewOrchestrator failed: %v", err)
	}

	ctx := context.Background()
	err = o.HandleHostFailure(ctx, "host-1")
	if err == nil {
		t.Error("Expected error when scheduler can't find target")
	}

	history := o.GetFailoverHistory(10)
	if len(history) != 1 {
		t.Fatalf("Expected 1 history record, got %d", len(history))
	}
	if history[0].State != "failed" {
		t.Errorf("Expected state 'failed', got '%s'", history[0].State)
	}
}

func TestHandleHostFailure_SomeVMsFail(t *testing.T) {
	vmProvider := newMockVMProvider()
	hostProvider := newMockHostProvider()
	restarter := newMockVMRestarter()

	// Add VMs with auto policy
	vm1 := makeVM("vm-1", "host-1", 4, 8*1024*1024*1024)
	vm2 := makeVM("vm-2", "host-1", 4, 8*1024*1024*1024)
	vmProvider.vmsByHost["host-1"] = []VM{vm1, vm2}

	hostProvider.hosts["host-2"] = makeHost("host-2", "online", false)

	// Make vm-2 fail to start
	restarter.startVMFn = func(ctx context.Context, vm VM, target Host) error {
		if vm.ID == "vm-2" {
			return fmt.Errorf("start failed")
		}
		return nil
	}

	cfg := OrchestratorConfig{
		VMProvider:  vmProvider,
		HostProvider: hostProvider,
		VMRestarter: restarter,
		Fencer:      newOrchMockFencer(),
	}
	o, err := NewOrchestrator(cfg)
	if err != nil {
		t.Fatalf("NewOrchestrator failed: %v", err)
	}

	ctx := context.Background()
	err = o.HandleHostFailure(ctx, "host-1")
	if err != nil {
		t.Errorf("Expected no error (partial failure is ok), got: %v", err)
	}

	history := o.GetFailoverHistory(10)
	if len(history) != 1 {
		t.Fatalf("Expected 1 history record, got %d", len(history))
	}
	if history[0].VMRestarted != 1 {
		t.Errorf("Expected VMRestarted=1, got %d", history[0].VMRestarted)
	}
	if history[0].VMFailed != 1 {
		t.Errorf("Expected VMFailed=1, got %d", history[0].VMFailed)
	}
}

func TestHandleHostFailure_HAModeMaxOne(t *testing.T) {
	vmProvider := newMockVMProvider()
	hostProvider := newMockHostProvider()
	restarter := newMockVMRestarter()

	// Add VM with max-one policy
	vm1 := makeVM("vm-1", "host-1", 4, 8*1024*1024*1024)
	vmProvider.vmsByHost["host-1"] = []VM{vm1}
	vmProvider.policies["vm-1"] = HAPolicy{VMID: "vm-1", Mode: HAModeMaxOne, Priority: 100}

	hostProvider.hosts["host-2"] = makeHost("host-2", "online", false)

	cfg := OrchestratorConfig{
		VMProvider:  vmProvider,
		HostProvider: hostProvider,
		VMRestarter: restarter,
		Fencer:      newOrchMockFencer(),
	}
	o, err := NewOrchestrator(cfg)
	if err != nil {
		t.Fatalf("NewOrchestrator failed: %v", err)
	}

	ctx := context.Background()
	err = o.HandleHostFailure(ctx, "host-1")
	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}

	restarter.mu.Lock()
	startedCount := len(restarter.startedVMs)
	restarter.mu.Unlock()
	if startedCount != 1 {
		t.Errorf("Expected 1 VM restarted (max-one mode), got %d", startedCount)
	}
}

func TestHandleHostFailure_GetHAPolicyError(t *testing.T) {
	vmProvider := newMockVMProvider()
	hostProvider := newMockHostProvider()

	// Add VMs with auto policy
	vm1 := makeVM("vm-1", "host-1", 4, 8*1024*1024*1024)
	vm2 := makeVM("vm-2", "host-1", 4, 8*1024*1024*1024)
	vmProvider.vmsByHost["host-1"] = []VM{vm1, vm2}

	// Make GetHAPolicy fail for vm-1
	vmProvider.getHAPolicyFn = func(ctx context.Context, vmID string) (*HAPolicy, error) {
		if vmID == "vm-1" {
			return nil, fmt.Errorf("policy lookup failed")
		}
		defaultP := DefaultHAPolicy(vmID)
		return &defaultP, nil
	}

	hostProvider.hosts["host-2"] = makeHost("host-2", "online", false)

	cfg := OrchestratorConfig{
		VMProvider:  vmProvider,
		HostProvider: hostProvider,
		Fencer:      newOrchMockFencer(),
	}
	o, err := NewOrchestrator(cfg)
	if err != nil {
		t.Fatalf("NewOrchestrator failed: %v", err)
	}

	ctx := context.Background()
	err = o.HandleHostFailure(ctx, "host-1")
	if err != nil {
		t.Errorf("Expected no error (VM with policy error skipped), got: %v", err)
	}

	history := o.GetFailoverHistory(10)
	if len(history) != 1 {
		t.Fatalf("Expected 1 history record, got %d", len(history))
	}
	if history[0].VMCount != 1 {
		t.Errorf("Expected VMCount=1 (one VM skipped due to policy error), got %d", history[0].VMCount)
	}
}

// ---------- RestartVMs tests ----------

func TestRestartVMs_Success(t *testing.T) {
	vmProvider := newMockVMProvider()
	hostProvider := newMockHostProvider()
	restarter := newMockVMRestarter()

	cfg := OrchestratorConfig{
		VMProvider:  vmProvider,
		HostProvider: hostProvider,
		VMRestarter: restarter,
	}
	o, err := NewOrchestrator(cfg)
	if err != nil {
		t.Fatalf("NewOrchestrator failed: %v", err)
	}

	vms := []VM{
		makeVM("vm-1", "host-1", 4, 8*1024*1024*1024),
		makeVM("vm-2", "host-1", 4, 8*1024*1024*1024),
	}
	target := makeHost("host-2", "online", false)

	ctx := context.Background()
	err = o.RestartVMs(ctx, vms, target)
	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}

	restarter.mu.Lock()
	startedCount := len(restarter.startedVMs)
	restarter.mu.Unlock()
	if startedCount != 2 {
		t.Errorf("Expected 2 VMs started, got %d", startedCount)
	}

	// Verify host assignments were updated
	for _, vm := range vms {
		if host, _ := vmProvider.GetVM(ctx, vm.ID); host != nil {
			if host.HostID != "host-2" {
				t.Errorf("Expected VM %s host updated to host-2, got %s", vm.ID, host.HostID)
			}
		}
	}
}

func TestRestartVMs_UpdateHostError(t *testing.T) {
	vmProvider := newMockVMProvider()
	hostProvider := newMockHostProvider()

	vmProvider.updateVMHostFn = func(ctx context.Context, vmID, newHostID string) error {
		return fmt.Errorf("update failed")
	}

	cfg := OrchestratorConfig{
		VMProvider:  vmProvider,
		HostProvider: hostProvider,
	}
	o, err := NewOrchestrator(cfg)
	if err != nil {
		t.Fatalf("NewOrchestrator failed: %v", err)
	}

	vms := []VM{makeVM("vm-1", "host-1", 4, 8*1024*1024*1024)}
	target := makeHost("host-2", "online", false)

	ctx := context.Background()
	err = o.RestartVMs(ctx, vms, target)
	if err == nil {
		t.Error("Expected error when UpdateVMHost fails")
	}
}

func TestRestartVMs_StartVMError(t *testing.T) {
	vmProvider := newMockVMProvider()
	hostProvider := newMockHostProvider()
	restarter := newMockVMRestarter()

	restarter.startVMFn = func(ctx context.Context, vm VM, target Host) error {
		return fmt.Errorf("start failed")
	}

	cfg := OrchestratorConfig{
		VMProvider:  vmProvider,
		HostProvider: hostProvider,
		VMRestarter: restarter,
	}
	o, err := NewOrchestrator(cfg)
	if err != nil {
		t.Fatalf("NewOrchestrator failed: %v", err)
	}

	vms := []VM{makeVM("vm-1", "host-1", 4, 8*1024*1024*1024)}
	target := makeHost("host-2", "online", false)

	ctx := context.Background()
	err = o.RestartVMs(ctx, vms, target)
	if err == nil {
		t.Error("Expected error when StartVM fails")
	}
}

func TestRestartVMs_NoRestarterConfigured(t *testing.T) {
	vmProvider := newMockVMProvider()
	hostProvider := newMockHostProvider()

	cfg := OrchestratorConfig{
		VMProvider:  vmProvider,
		HostProvider: hostProvider,
		// No VMRestarter configured
	}
	o, err := NewOrchestrator(cfg)
	if err != nil {
		t.Fatalf("NewOrchestrator failed: %v", err)
	}

	vms := []VM{makeVM("vm-1", "host-1", 4, 8*1024*1024*1024)}
	target := makeHost("host-2", "online", false)

	ctx := context.Background()
	err = o.RestartVMs(ctx, vms, target)
	if err != nil {
		t.Errorf("Expected no error without restarter, got: %v", err)
	}
}

func TestRestartVMs_EmptyVMList(t *testing.T) {
	vmProvider := newMockVMProvider()
	hostProvider := newMockHostProvider()

	cfg := OrchestratorConfig{
		VMProvider:  vmProvider,
		HostProvider: hostProvider,
	}
	o, err := NewOrchestrator(cfg)
	if err != nil {
		t.Fatalf("NewOrchestrator failed: %v", err)
	}

	ctx := context.Background()
	target := makeHost("host-2", "online", false)
	err = o.RestartVMs(ctx, []VM{}, target)
	if err != nil {
		t.Errorf("Expected no error with empty VM list, got: %v", err)
	}
}

// ---------- GetActiveFailovers tests ----------

func TestGetActiveFailovers_Empty(t *testing.T) {
	vmProvider := newMockVMProvider()
	hostProvider := newMockHostProvider()

	cfg := OrchestratorConfig{
		VMProvider:  vmProvider,
		HostProvider: hostProvider,
	}
	o, err := NewOrchestrator(cfg)
	if err != nil {
		t.Fatalf("NewOrchestrator failed: %v", err)
	}

	active := o.GetActiveFailovers()
	if len(active) != 0 {
		t.Errorf("Expected 0 active failovers, got %d", len(active))
	}
}

// ---------- GetFailoverHistory tests ----------

func TestGetFailoverHistory_Empty(t *testing.T) {
	vmProvider := newMockVMProvider()
	hostProvider := newMockHostProvider()

	cfg := OrchestratorConfig{
		VMProvider:  vmProvider,
		HostProvider: hostProvider,
	}
	o, err := NewOrchestrator(cfg)
	if err != nil {
		t.Fatalf("NewOrchestrator failed: %v", err)
	}

	history := o.GetFailoverHistory(10)
	if len(history) != 0 {
		t.Errorf("Expected 0 history records, got %d", len(history))
	}
}

func TestGetFailoverHistory_Limit(t *testing.T) {
	vmProvider := newMockVMProvider()
	hostProvider := newMockHostProvider()

	cfg := OrchestratorConfig{
		VMProvider:  vmProvider,
		HostProvider: hostProvider,
	}
	o, err := NewOrchestrator(cfg)
	if err != nil {
		t.Fatalf("NewOrchestrator failed: %v", err)
	}

	// Manually add history records (using a synchronous approach to avoid races)
	for i := 0; i < 5; i++ {
		o.mu.Lock()
		o.history = append(o.history, FailoverRecord{
			ID:     fmt.Sprintf("failover-%d", i),
			NodeID: fmt.Sprintf("host-%d", i),
			State:  "complete",
		})
		o.mu.Unlock()
	}

	// Request with limit 3
	history := o.GetFailoverHistory(3)
	if len(history) != 3 {
		t.Errorf("Expected 3 history records, got %d", len(history))
	}

	// Should be the last 3
	if history[0].ID != "failover-2" {
		t.Errorf("Expected first record to be failover-2, got %s", history[0].ID)
	}

	// Request with limit larger than history
	history = o.GetFailoverHistory(100)
	if len(history) != 5 {
		t.Errorf("Expected 5 history records, got %d", len(history))
	}

	// Request with limit 0 (should return all)
	history = o.GetFailoverHistory(0)
	if len(history) != 5 {
		t.Errorf("Expected 5 history records, got %d", len(history))
	}

	// Request with negative limit (should return all)
	history = o.GetFailoverHistory(-1)
	if len(history) != 5 {
		t.Errorf("Expected 5 history records, got %d", len(history))
	}
}

func TestGetFailoverHistory_MaxHistory(t *testing.T) {
	vmProvider := newMockVMProvider()
	hostProvider := newMockHostProvider()

	// Each HandleHostFailure with no VMs creates a history entry
	// Use unique host names to avoid "already in progress" errors
	for i := 0; i < 5; i++ {
		hostProvider.hosts[fmt.Sprintf("test-host-%d", i)] = makeHost(fmt.Sprintf("test-host-%d", i), "online", false)
	}

	cfg := OrchestratorConfig{
		VMProvider:  vmProvider,
		HostProvider: hostProvider,
		MaxHistory:  3,
	}
	o, err := NewOrchestrator(cfg)
	if err != nil {
		t.Fatalf("NewOrchestrator failed: %v", err)
	}

	// Perform 5 failovers (with no VMs) - each creates a history entry
	ctx := context.Background()
	for i := 0; i < 5; i++ {
		err := o.HandleHostFailure(ctx, fmt.Sprintf("test-host-%d", i))
		if err != nil {
			t.Fatalf("HandleHostFailure failed: %v", err)
		}
	}

	// Direct check of internal history
	o.mu.Lock()
	internalLen := len(o.history)
	o.mu.Unlock()
	if internalLen != 3 {
		t.Errorf("Expected internal history capped at 3, got %d", internalLen)
	}

	history := o.GetFailoverHistory(10)
	if len(history) != 3 {
		t.Errorf("Expected 3 history records (capped by MaxHistory), got %d", len(history))
	}
}

// ---------- Concurrent failover handling tests ----------

func TestConcurrentFailover_DifferentNodes(t *testing.T) {
	vmProvider := newMockVMProvider()
	hostProvider := newMockHostProvider()
	restarter := newMockVMRestarter()

	// Add VMs on different hosts
	vm1 := makeVM("vm-1", "host-1", 4, 8*1024*1024*1024)
	vm2 := makeVM("vm-2", "host-2", 4, 8*1024*1024*1024)
	vmProvider.vmsByHost["host-1"] = []VM{vm1}
	vmProvider.vmsByHost["host-2"] = []VM{vm2}

	hostProvider.hosts["host-3"] = makeHost("host-3", "online", false)
	hostProvider.hosts["host-4"] = makeHost("host-4", "online", false)

	cfg := OrchestratorConfig{
		VMProvider:  vmProvider,
		HostProvider: hostProvider,
		VMRestarter: restarter,
		Fencer:      newOrchMockFencer(),
	}
	o, err := NewOrchestrator(cfg)
	if err != nil {
		t.Fatalf("NewOrchestrator failed: %v", err)
	}

	ctx := context.Background()
	var wg sync.WaitGroup
	var err1, err2 error

	wg.Add(2)
	go func() {
		defer wg.Done()
		err1 = o.HandleHostFailure(ctx, "host-1")
	}()
	go func() {
		defer wg.Done()
		err2 = o.HandleHostFailure(ctx, "host-2")
	}()
	wg.Wait()

	if err1 != nil {
		t.Errorf("First failover returned error: %v", err1)
	}
	if err2 != nil {
		t.Errorf("Second failover returned error: %v", err2)
	}

	history := o.GetFailoverHistory(10)
	if len(history) != 2 {
		t.Errorf("Expected 2 history records, got %d", len(history))
	}
}

func TestConcurrentFailover_SameNode(t *testing.T) {
	vmProvider := newMockVMProvider()
	hostProvider := newMockHostProvider()

	// Add VM
	vm1 := makeVM("vm-1", "host-1", 4, 8*1024*1024*1024)
	vmProvider.vmsByHost["host-1"] = []VM{vm1}
	hostProvider.hosts["host-2"] = makeHost("host-2", "online", false)

	// Make processing slow
	vmProvider.getHAPolicyFn = func(ctx context.Context, vmID string) (*HAPolicy, error) {
		time.Sleep(200 * time.Millisecond)
		defaultP := DefaultHAPolicy(vmID)
		return &defaultP, nil
	}

	cfg := OrchestratorConfig{
		VMProvider:  vmProvider,
		HostProvider: hostProvider,
		Fencer:      newOrchMockFencer(),
	}
	o, err := NewOrchestrator(cfg)
	if err != nil {
		t.Fatalf("NewOrchestrator failed: %v", err)
	}

	ctx := context.Background()
	var wg sync.WaitGroup
	var err1, err2 error

	wg.Add(2)
	go func() {
		defer wg.Done()
		err1 = o.HandleHostFailure(ctx, "host-1")
	}()
	// Small delay to ensure first goroutine registers first
	time.Sleep(10 * time.Millisecond)
	go func() {
		defer wg.Done()
		err2 = o.HandleHostFailure(ctx, "host-1")
	}()
	wg.Wait()

	// One should succeed, one should fail with "already in progress"
	if err1 != nil && err2 != nil {
		t.Error("Expected one of the failovers to succeed")
	}
	if err1 == nil && err2 == nil {
		t.Error("Expected one failover to fail with 'already in progress'")
	}
}

// ---------- Integration-like tests ----------

func TestHandleHostFailure_FullLifecycle(t *testing.T) {
	vmProvider := newMockVMProvider()
	hostProvider := newMockHostProvider()
	restarter := newMockVMRestarter()
	fencer := newOrchMockFencer()
	publisher := newMockEventPublisher()

	// Set up VMs with mixed policies
	vm1 := makeVM("vm-auto", "host-1", 4, 8*1024*1024*1024)
	vm2 := makeVM("vm-manual", "host-1", 2, 4*1024*1024*1024)
	vm3 := makeVM("vm-maxone", "host-1", 4, 8*1024*1024*1024)
	vm4 := makeVM("vm-never", "host-1", 2, 4*1024*1024*1024)
	vmProvider.vmsByHost["host-1"] = []VM{vm1, vm2, vm3, vm4}

	vmProvider.policies["vm-auto"] = HAPolicy{VMID: "vm-auto", Mode: HAModeAuto, Priority: 100}
	vmProvider.policies["vm-manual"] = HAPolicy{VMID: "vm-manual", Mode: HAModeManual, Priority: 50}
	vmProvider.policies["vm-maxone"] = HAPolicy{VMID: "vm-maxone", Mode: HAModeMaxOne, Priority: 75}
	vmProvider.policies["vm-never"] = HAPolicy{VMID: "vm-never", Mode: HAModeNever, Priority: 0}

	hostProvider.hosts["host-2"] = makeHost("host-2", "online", false)

	cfg := OrchestratorConfig{
		VMProvider:     vmProvider,
		HostProvider:   hostProvider,
		VMRestarter:    restarter,
		Fencer:         fencer,
		EventPublisher: publisher,
	}
	o, err := NewOrchestrator(cfg)
	if err != nil {
		t.Fatalf("NewOrchestrator failed: %v", err)
	}

	ctx := context.Background()
	err = o.HandleHostFailure(ctx, "host-1")
	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}

	// Verify only auto and max-one VMs were restarted (vm-auto and vm-maxone)
	restarter.mu.Lock()
	startedCount := len(restarter.startedVMs)
	restarter.mu.Unlock()
	if startedCount != 2 {
		t.Errorf("Expected 2 VMs restarted (auto + max-one), got %d", startedCount)
	}

	// Verify history
	history := o.GetFailoverHistory(10)
	if len(history) != 1 {
		t.Fatalf("Expected 1 history record, got %d", len(history))
	}
	if history[0].VMCount != 2 {
		t.Errorf("Expected VMCount=2 (auto + max-one), got %d", history[0].VMCount)
	}
	if history[0].VMRestarted != 2 {
		t.Errorf("Expected VMRestarted=2, got %d", history[0].VMRestarted)
	}
	if history[0].State != "complete" {
		t.Errorf("Expected state 'complete', got '%s'", history[0].State)
	}

	// Verify events
	publisher.mu.Lock()
	eventCount := len(publisher.events)
	publisher.mu.Unlock()
	if eventCount < 2 {
		t.Errorf("Expected at least 2 events, got %d", eventCount)
	}

	// Verify no active failovers
	if len(o.GetActiveFailovers()) != 0 {
		t.Error("Expected no active failovers after completion")
	}
}

func TestHandleHostFailure_NoEventPublisher(t *testing.T) {
	vmProvider := newMockVMProvider()
	hostProvider := newMockHostProvider()

	// Add VM with auto policy
	vm1 := makeVM("vm-1", "host-1", 4, 8*1024*1024*1024)
	vmProvider.vmsByHost["host-1"] = []VM{vm1}

	hostProvider.hosts["host-2"] = makeHost("host-2", "online", false)

	cfg := OrchestratorConfig{
		VMProvider:   vmProvider,
		HostProvider: hostProvider,
		Fencer:       newOrchMockFencer(),
		// No event publisher
	}
	o, err := NewOrchestrator(cfg)
	if err != nil {
		t.Fatalf("NewOrchestrator failed: %v", err)
	}

	ctx := context.Background()
	err = o.HandleHostFailure(ctx, "host-1")
	if err != nil {
		t.Errorf("Expected no error without event publisher, got: %v", err)
	}
}
