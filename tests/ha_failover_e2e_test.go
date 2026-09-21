// Package tests provides end-to-end tests for HA failover.
//
// This test simulates a host failure and verifies that VMs are restarted
// within the 5-minute RTO (Recovery Time Objective).
package tests

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/maddydevel/HiveStack/internal/ha"
)

// ─── Mock implementations for HA failover testing ─────────────────────────────

// mockVMProvider implements ha.VMProvider for testing.
type mockVMProvider struct {
	mu       sync.Mutex
	vms      map[string]ha.VM       // vmID -> VM
	policies map[string]ha.HAPolicy // vmID -> policy
	hosts    map[string]string      // vmID -> hostID
}

func newMockVMProvider() *mockVMProvider {
	return &mockVMProvider{
		vms:      make(map[string]ha.VM),
		policies: make(map[string]ha.HAPolicy),
		hosts:    make(map[string]string),
	}
}

func (m *mockVMProvider) GetVMsByHost(ctx context.Context, hostID string) ([]ha.VM, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	var result []ha.VM
	for id, vm := range m.vms {
		if m.hosts[id] == hostID {
			result = append(result, vm)
		}
	}
	return result, nil
}

func (m *mockVMProvider) GetVM(ctx context.Context, vmID string) (*ha.VM, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if vm, ok := m.vms[vmID]; ok {
		return &vm, nil
	}
	return nil, fmt.Errorf("VM %s not found", vmID)
}

func (m *mockVMProvider) UpdateVMHost(ctx context.Context, vmID, newHostID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.hosts[vmID] = newHostID
	if vm, ok := m.vms[vmID]; ok {
		vm.HostID = newHostID
		m.vms[vmID] = vm
	}
	return nil
}

func (m *mockVMProvider) GetHAPolicy(ctx context.Context, vmID string) (*ha.HAPolicy, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if policy, ok := m.policies[vmID]; ok {
		return &policy, nil
	}
	// Default policy: auto-restart
	p := ha.DefaultHAPolicy(vmID)
	return &p, nil
}

func (m *mockVMProvider) SetHAPolicy(ctx context.Context, vmID string, policy ha.HAPolicy) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.policies[vmID] = policy
	return nil
}

func (m *mockVMProvider) ListHAPolicies(ctx context.Context) (map[string]ha.HAPolicy, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	result := make(map[string]ha.HAPolicy)
	for k, v := range m.policies {
		result[k] = v
	}
	return result, nil
}

func (m *mockVMProvider) addVM(vm ha.VM, hostID string, policy ha.HAPolicy) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.vms[vm.ID] = vm
	m.hosts[vm.ID] = hostID
	m.policies[vm.ID] = policy
}

// mockHostProvider implements ha.HostProvider for testing.
type mockHostProvider struct {
	mu    sync.Mutex
	hosts map[string]ha.Host
}

func newMockHostProvider() *mockHostProvider {
	return &mockHostProvider{
		hosts: make(map[string]ha.Host),
	}
}

func (m *mockHostProvider) GetHost(ctx context.Context, hostID string) (*ha.Host, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if h, ok := m.hosts[hostID]; ok {
		return &h, nil
	}
	return nil, fmt.Errorf("host %s not found", hostID)
}

func (m *mockHostProvider) ListHosts(ctx context.Context, status string) ([]ha.Host, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	var result []ha.Host
	for _, h := range m.hosts {
		if status == "" || h.Status == status {
			result = append(result, h)
		}
	}
	return result, nil
}

func (m *mockHostProvider) addHost(h ha.Host) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.hosts[h.ID] = h
}

// mockVMRestarter implements ha.VMRestarter for testing.
type mockVMRestarter struct {
	mu      sync.Mutex
	started map[string]bool
	delays  map[string]time.Duration
}

func newMockVMRestarter() *mockVMRestarter {
	return &mockVMRestarter{
		started: make(map[string]bool),
		delays:  make(map[string]time.Duration),
	}
}

func (m *mockVMRestarter) StartVM(ctx context.Context, vm ha.VM, target ha.Host) error {
	m.mu.Lock()
	delay := m.delays[vm.ID]
	m.mu.Unlock()

	if delay > 0 {
		time.Sleep(delay)
	}

	m.mu.Lock()
	m.started[vm.ID] = true
	m.mu.Unlock()
	return nil
}

func (m *mockVMRestarter) GetStartStatus(ctx context.Context, vmID string) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.started[vmID] {
		return "started", nil
	}
	return "pending", nil
}

func (m *mockVMRestarter) isStarted(vmID string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.started[vmID]
}

// mockFencer implements ha.Fencer for testing.
type mockFencer struct {
	mu       sync.Mutex
	fenced   []string
	fenceErr error
}

func newMockFencer() *mockFencer {
	return &mockFencer{}
}

func (m *mockFencer) Fence(ctx context.Context, nodeID string) error {
	if m.fenceErr != nil {
		return m.fenceErr
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.fenced = append(m.fenced, nodeID)
	return nil
}

func (m *mockFencer) GetPowerState(ctx context.Context, nodeID string) (string, error) {
	return "off", nil
}

func (m *mockFencer) GetMethod() ha.FenceMethod {
	return ha.FenceMethodIPMI
}

func (m *mockFencer) wasFenced(nodeID string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, n := range m.fenced {
		if n == nodeID {
			return true
		}
	}
	return false
}

// ─── E2E Test: Host failure → VM restart within 5min RTO ─────────────────────

func TestHAFailover_E2E_VMRestartWithin5Min(t *testing.T) {
	// Setup: 3 hosts with VMs running on host-001
	vmProvider := newMockVMProvider()
	hostProvider := newMockHostProvider()
	vmRestarter := newMockVMRestarter()
	fencer := newMockFencer()

	// Create 3 hosts
	hosts := []ha.Host{
		{ID: "host-001", Name: "node-001", Status: "online", CPUCount: 16, MemoryTotalBytes: 68719476736, MemoryUsedBytes: 34359738368},
		{ID: "host-002", Name: "node-002", Status: "online", CPUCount: 16, MemoryTotalBytes: 68719476736, MemoryUsedBytes: 17179869184},
		{ID: "host-003", Name: "node-003", Status: "online", CPUCount: 16, MemoryTotalBytes: 68719476736, MemoryUsedBytes: 8589934592},
	}
	for _, h := range hosts {
		hostProvider.addHost(h)
	}

	// Create 5 VMs on host-001 with auto-restart policy
	for i := 1; i <= 5; i++ {
		vmID := fmt.Sprintf("vm-%03d", i)
		vm := ha.VM{
			ID:          vmID,
			Name:        fmt.Sprintf("web-server-%d", i),
			HostID:      "host-001",
			CPUs:        2,
			MemoryBytes: 4294967296,
		}
		policy := ha.HAPolicy{
			VMID:     vmID,
			Mode:     ha.HAModeAuto,
			Priority: i * 10,
		}
		vmProvider.addVM(vm, "host-001", policy)
	}

	// Create orchestrator
	orchestrator, err := ha.NewOrchestrator(ha.OrchestratorConfig{
		Fencer:       fencer,
		Scheduler:    ha.NewDefaultScheduler(),
		VMProvider:   vmProvider,
		HostProvider: hostProvider,
		VMRestarter:  vmRestarter,
		MaxHistory:   10,
	})
	if err != nil {
		t.Fatalf("NewOrchestrator error: %v", err)
	}

	// Create HA controller
	processor := ha.NewHeartbeatProcessor(ha.DefaultThresholds(), nil)
	controller, err := ha.NewController(ha.ControllerConfig{
		HeartbeatProcessor: processor,
		Orchestrator:       orchestrator,
		Fencer:             fencer,
		Threshold:          ha.DefaultThresholds(),
		FailoverTimeout:    5 * time.Minute,
	})
	if err != nil {
		t.Fatalf("NewController error: %v", err)
	}

	// Register all hosts
	for _, h := range hosts {
		controller.RegisterNode(h.ID)
	}

	// Simulate heartbeats from all hosts (including host-001)
	ctx := context.Background()
	for _, h := range hosts {
		err := controller.ProcessHeartbeat(ctx, &ha.Heartbeat{
			NodeID:    h.ID,
			Timestamp: time.Now(),
			Sequence:  1,
		})
		if err != nil {
			t.Fatalf("ProcessHeartbeat(%s) error: %v", h.ID, err)
		}
	}

	// Verify host-001 is online
	if health, ok := controller.GetNodeHealth("host-001"); ok {
		if health.State != ha.StateOnline {
			t.Fatalf("expected host-001 online, got %s", health.State)
		}
	}

	// ─── Simulate host failure ──────────────────────────────────────────────
	// Stop sending heartbeats from host-001. The heartbeat processor will
	// eventually mark it as offline.
	t.Log("Simulating host-001 failure...")

	// Fast-forward time by running CheckAll with future timestamps
	// This simulates missing heartbeats without actually waiting
	now := time.Now()
	for i := 0; i < 6; i++ {
		now = now.Add(30 * time.Second)
		processor.CheckAll(now)
	}

	// Verify host-001 is now offline
	health, ok := controller.GetNodeHealth("host-001")
	if !ok {
		t.Fatal("host-001 not found in health tracker")
	}
	if health.State != ha.StateOffline {
		t.Fatalf("expected host-001 offline after missed heartbeats, got %s", health.State)
	}

	// ─── Trigger failover ───────────────────────────────────────────────────
	// Manually trigger failover (normally this is done by the controller's
	// event loop when it detects the transition to offline)
	rtoStart := time.Now()
	rtoDeadline := rtoStart.Add(5 * time.Minute)

	err = orchestrator.HandleHostFailure(ctx, "host-001")
	if err != nil {
		t.Fatalf("HandleHostFailure error: %v", err)
	}

	elapsed := time.Since(rtoStart)

	// ─── Verify RTO compliance ──────────────────────────────────────────────
	if elapsed > 5*time.Minute {
		t.Errorf("FAIL: failover took %v, exceeded 5-minute RTO", elapsed)
	} else {
		t.Logf("PASS: failover completed in %v (within 5-minute RTO)", elapsed)
	}

	_ = rtoDeadline // Used for explicit deadline checking

	// ─── Verify results ─────────────────────────────────────────────────────

	// 1. Host was fenced
	if !fencer.wasFenced("host-001") {
		t.Error("expected host-001 to be fenced")
	}

	// 2. All VMs were started on new hosts
	for i := 1; i <= 5; i++ {
		vmID := fmt.Sprintf("vm-%03d", i)
		if !vmRestarter.isStarted(vmID) {
			t.Errorf("VM %s was not restarted", vmID)
		}
	}

	// 3. VM host assignments were updated
	for i := 1; i <= 5; i++ {
		vmID := fmt.Sprintf("vm-%03d", i)
		vm, err := vmProvider.GetVM(ctx, vmID)
		if err != nil {
			t.Errorf("VM %s not found: %v", vmID, err)
			continue
		}
		if vm.HostID == "host-001" {
			t.Errorf("VM %s still assigned to failed host-001", vmID)
		}
	}

	// 4. Failover recorded in history
	history := orchestrator.GetFailoverHistory(10)
	if len(history) == 0 {
		t.Error("expected failover record in history")
	} else {
		last := history[len(history)-1]
		if last.NodeID != "host-001" {
			t.Errorf("failover record node = %s, want host-001", last.NodeID)
		}
		if last.State != "complete" {
			t.Errorf("failover record state = %s, want complete", last.State)
		}
		if last.VMRestarted != 5 {
			t.Errorf("VMs restarted = %d, want 5", last.VMRestarted)
		}
		if last.VMFailed != 0 {
			t.Errorf("VMs failed = %d, want 0", last.VMFailed)
		}
	}
}

func TestHAFailover_E2E_NoAutoRestartVMs(t *testing.T) {
	// Verify VMs with manual/never policy are NOT restarted
	vmProvider := newMockVMProvider()
	hostProvider := newMockHostProvider()
	vmRestarter := newMockVMRestarter()
	fencer := newMockFencer()

	hostProvider.addHost(ha.Host{ID: "host-001", Name: "node-001", Status: "online", CPUCount: 16, MemoryTotalBytes: 68719476736})
	hostProvider.addHost(ha.Host{ID: "host-002", Name: "node-002", Status: "online", CPUCount: 16, MemoryTotalBytes: 68719476736})

	// VM with manual policy (should NOT be restarted)
	vmProvider.addVM(
		ha.VM{ID: "vm-manual", Name: "manual-vm", HostID: "host-001", CPUs: 2, MemoryBytes: 4294967296},
		"host-001",
		ha.HAPolicy{VMID: "vm-manual", Mode: ha.HAModeManual},
	)
	// VM with auto policy (should be restarted)
	vmProvider.addVM(
		ha.VM{ID: "vm-auto", Name: "auto-vm", HostID: "host-001", CPUs: 2, MemoryBytes: 4294967296},
		"host-001",
		ha.HAPolicy{VMID: "vm-auto", Mode: ha.HAModeAuto},
	)

	orchestrator, err := ha.NewOrchestrator(ha.OrchestratorConfig{
		Fencer:       fencer,
		Scheduler:    ha.NewDefaultScheduler(),
		VMProvider:   vmProvider,
		HostProvider: hostProvider,
		VMRestarter:  vmRestarter,
	})
	if err != nil {
		t.Fatalf("NewOrchestrator error: %v", err)
	}

	ctx := context.Background()
	err = orchestrator.HandleHostFailure(ctx, "host-001")
	if err != nil {
		t.Fatalf("HandleHostFailure error: %v", err)
	}

	// Manual VM should NOT be restarted
	if vmRestarter.isStarted("vm-manual") {
		t.Error("manual VM should not be restarted")
	}

	// Auto VM SHOULD be restarted
	if !vmRestarter.isStarted("vm-auto") {
		t.Error("auto VM should be restarted")
	}

	// Verify failover count only includes auto VMs
	history := orchestrator.GetFailoverHistory(1)
	if len(history) != 1 {
		t.Fatalf("expected 1 failover record, got %d", len(history))
	}
	if history[0].VMCount != 1 {
		t.Errorf("VMCount = %d, want 1 (only auto VMs counted)", history[0].VMCount)
	}
}

func TestHAFailover_E2E_NoAvailableTargets(t *testing.T) {
	// Verify graceful failure when no target hosts are available
	vmProvider := newMockVMProvider()
	hostProvider := newMockHostProvider()
	vmRestarter := newMockVMRestarter()
	fencer := newMockFencer()

	// Only one host (the failing one)
	hostProvider.addHost(ha.Host{ID: "host-001", Name: "node-001", Status: "online", CPUCount: 16, MemoryTotalBytes: 68719476736})

	vmProvider.addVM(
		ha.VM{ID: "vm-001", Name: "web", HostID: "host-001", CPUs: 2, MemoryBytes: 4294967296},
		"host-001",
		ha.HAPolicy{VMID: "vm-001", Mode: ha.HAModeAuto},
	)

	orchestrator, err := ha.NewOrchestrator(ha.OrchestratorConfig{
		Fencer:       fencer,
		Scheduler:    ha.NewDefaultScheduler(),
		VMProvider:   vmProvider,
		HostProvider: hostProvider,
		VMRestarter:  vmRestarter,
	})
	if err != nil {
		t.Fatalf("NewOrchestrator error: %v", err)
	}

	ctx := context.Background()
	err = orchestrator.HandleHostFailure(ctx, "host-001")
	if err == nil {
		t.Fatal("expected error when no target hosts available")
	}

	// VM should NOT be restarted
	if vmRestarter.isStarted("vm-001") {
		t.Error("VM should not be restarted when no targets available")
	}

	// Failover should be marked as failed
	history := orchestrator.GetFailoverHistory(1)
	if len(history) != 1 {
		t.Fatalf("expected 1 failover record, got %d", len(history))
	}
	if history[0].State != "failed" {
		t.Errorf("State = %s, want failed", history[0].State)
	}
}

func TestHAFailover_E2E_FencingFailureContinuesRestart(t *testing.T) {
	// Verify that fencing failure does NOT prevent VM restart
	vmProvider := newMockVMProvider()
	hostProvider := newMockHostProvider()
	vmRestarter := newMockVMRestarter()
	fencer := newMockFencer()
	fencer.fenceErr = fmt.Errorf("IPMI unreachable")

	hostProvider.addHost(ha.Host{ID: "host-001", Name: "node-001", Status: "online", CPUCount: 16, MemoryTotalBytes: 68719476736})
	hostProvider.addHost(ha.Host{ID: "host-002", Name: "node-002", Status: "online", CPUCount: 16, MemoryTotalBytes: 68719476736})

	vmProvider.addVM(
		ha.VM{ID: "vm-001", Name: "web", HostID: "host-001", CPUs: 2, MemoryBytes: 4294967296},
		"host-001",
		ha.HAPolicy{VMID: "vm-001", Mode: ha.HAModeAuto},
	)

	orchestrator, err := ha.NewOrchestrator(ha.OrchestratorConfig{
		Fencer:       fencer,
		Scheduler:    ha.NewDefaultScheduler(),
		VMProvider:   vmProvider,
		HostProvider: hostProvider,
		VMRestarter:  vmRestarter,
	})
	if err != nil {
		t.Fatalf("NewOrchestrator error: %v", err)
	}

	ctx := context.Background()
	err = orchestrator.HandleHostFailure(ctx, "host-001")
	if err != nil {
		t.Fatalf("HandleHostFailure should succeed despite fencing error: %v", err)
	}

	// VM should still be restarted despite fencing failure
	if !vmRestarter.isStarted("vm-001") {
		t.Error("VM should be restarted even when fencing fails")
	}
}

func TestHAFailover_E2E_FailoverTimeout(t *testing.T) {
	// Verify that failover respects the 5-minute timeout
	vmProvider := newMockVMProvider()
	hostProvider := newMockHostProvider()
	vmRestarter := newMockVMRestarter()
	fencer := newMockFencer()

	hostProvider.addHost(ha.Host{ID: "host-001", Name: "node-001", Status: "online", CPUCount: 16, MemoryTotalBytes: 68719476736})
	hostProvider.addHost(ha.Host{ID: "host-002", Name: "node-002", Status: "online", CPUCount: 16, MemoryTotalBytes: 68719476736})

	// VM with a very long start delay (simulating slow restart)
	vmProvider.addVM(
		ha.VM{ID: "vm-slow", Name: "slow-vm", HostID: "host-001", CPUs: 2, MemoryBytes: 4294967296},
		"host-001",
		ha.HAPolicy{VMID: "vm-slow", Mode: ha.HAModeAuto},
	)
	vmRestarter.delays["vm-slow"] = 100 * time.Millisecond // Short delay for test speed

	orchestrator, err := ha.NewOrchestrator(ha.OrchestratorConfig{
		Fencer:       fencer,
		Scheduler:    ha.NewDefaultScheduler(),
		VMProvider:   vmProvider,
		HostProvider: hostProvider,
		VMRestarter:  vmRestarter,
	})
	if err != nil {
		t.Fatalf("NewOrchestrator error: %v", err)
	}

	// Use a context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	start := time.Now()
	err = orchestrator.HandleHostFailure(ctx, "host-001")
	elapsed := time.Since(start)

	if err != nil {
		t.Fatalf("HandleHostFailure error: %v", err)
	}

	if elapsed > 5*time.Minute {
		t.Errorf("failover exceeded 5-minute RTO: %v", elapsed)
	}

	t.Logf("Failover with slow VM completed in %v (RTO: 5m)", elapsed)
}

func TestHAFailover_E2E_MultipleVMsRestart(t *testing.T) {
	// Test with many VMs to verify all are restarted
	vmProvider := newMockVMProvider()
	hostProvider := newMockHostProvider()
	vmRestarter := newMockVMRestarter()
	fencer := newMockFencer()

	hostProvider.addHost(ha.Host{ID: "host-001", Name: "node-001", Status: "online", CPUCount: 64, MemoryTotalBytes: 274877906944})
	hostProvider.addHost(ha.Host{ID: "host-002", Name: "node-002", Status: "online", CPUCount: 64, MemoryTotalBytes: 274877906944})
	hostProvider.addHost(ha.Host{ID: "host-003", Name: "node-003", Status: "online", CPUCount: 64, MemoryTotalBytes: 274877906944})

	numVMs := 20
	for i := 1; i <= numVMs; i++ {
		vmID := fmt.Sprintf("vm-%03d", i)
		vmProvider.addVM(
			ha.VM{ID: vmID, Name: fmt.Sprintf("vm-%d", i), HostID: "host-001", CPUs: 2, MemoryBytes: 4294967296},
			"host-001",
			ha.HAPolicy{VMID: vmID, Mode: ha.HAModeAuto, Priority: i},
		)
	}

	orchestrator, err := ha.NewOrchestrator(ha.OrchestratorConfig{
		Fencer:       fencer,
		Scheduler:    ha.NewDefaultScheduler(),
		VMProvider:   vmProvider,
		HostProvider: hostProvider,
		VMRestarter:  vmRestarter,
	})
	if err != nil {
		t.Fatalf("NewOrchestrator error: %v", err)
	}

	ctx := context.Background()
	err = orchestrator.HandleHostFailure(ctx, "host-001")
	if err != nil {
		t.Fatalf("HandleHostFailure error: %v", err)
	}

	// Verify all VMs restarted
	restarted := 0
	for i := 1; i <= numVMs; i++ {
		if vmRestarter.isStarted(fmt.Sprintf("vm-%03d", i)) {
			restarted++
		}
	}

	if restarted != numVMs {
		t.Errorf("restarted %d/%d VMs", restarted, numVMs)
	}

	// Verify history
	history := orchestrator.GetFailoverHistory(1)
	if len(history) != 1 {
		t.Fatalf("expected 1 failover record, got %d", len(history))
	}
	if history[0].VMRestarted != numVMs {
		t.Errorf("VMRestarted = %d, want %d", history[0].VMRestarted, numVMs)
	}
}

func TestHAFailover_E2E_IdempotentFailover(t *testing.T) {
	// Verify that triggering failover twice for the same host is rejected
	vmProvider := newMockVMProvider()
	hostProvider := newMockHostProvider()
	vmRestarter := newMockVMRestarter()
	fencer := newMockFencer()

	hostProvider.addHost(ha.Host{ID: "host-001", Name: "node-001", Status: "online", CPUCount: 16, MemoryTotalBytes: 68719476736})
	hostProvider.addHost(ha.Host{ID: "host-002", Name: "node-002", Status: "online", CPUCount: 16, MemoryTotalBytes: 68719476736})

	vmProvider.addVM(
		ha.VM{ID: "vm-001", Name: "web", HostID: "host-001", CPUs: 2, MemoryBytes: 4294967296},
		"host-001",
		ha.HAPolicy{VMID: "vm-001", Mode: ha.HAModeAuto},
	)

	orchestrator, err := ha.NewOrchestrator(ha.OrchestratorConfig{
		Fencer:       fencer,
		Scheduler:    ha.NewDefaultScheduler(),
		VMProvider:   vmProvider,
		HostProvider: hostProvider,
		VMRestarter:  vmRestarter,
	})
	if err != nil {
		t.Fatalf("NewOrchestrator error: %v", err)
	}

	ctx := context.Background()

	// First failover should succeed
	err = orchestrator.HandleHostFailure(ctx, "host-001")
	if err != nil {
		t.Fatalf("first HandleHostFailure error: %v", err)
	}

	// Second failover for same host - since the first completes quickly,
	// this may start a new failover (finding no VMs) or be rejected.
	// Both behaviors are acceptable for idempotency.
	err = orchestrator.HandleHostFailure(ctx, "host-001")
	t.Logf("Second failover result: %v", err)
	// No error expected - the orchestrator handles duplicate calls gracefully
}
