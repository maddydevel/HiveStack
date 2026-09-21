// Package ha provides Non-HANA High Availability for HiveStack.
//
// The orchestrator coordinates VM restarts after host failure: detect failure,
// fence the failed host, select target hosts, and restart VMs.
package ha

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"
)

// Orchestrator defines the interface for HA failover operations.
type Orchestrator interface {
	// HandleHostFailure performs the full failover sequence for a failed host.
	HandleHostFailure(ctx context.Context, nodeID string) error
	// RestartVMs restarts the given VMs on the specified target host.
	RestartVMs(ctx context.Context, vms []VM, target Host) error
	// GetActiveFailovers returns the list of currently executing failovers.
	GetActiveFailovers() []FailoverRecord
	// GetFailoverHistory returns past failover records.
	GetFailoverHistory(limit int) []FailoverRecord
}

// FailoverRecord tracks the progress of a failover operation.
type FailoverRecord struct {
	ID          string     `json:"id"`
	NodeID      string     `json:"node_id"`
	State       string     `json:"state"` // "detecting", "fencing", "scheduling", "restarting", "complete", "failed"
	VMCount     int        `json:"vm_count"`
	VMRestarted int        `json:"vm_restarted"`
	VMFailed    int        `json:"vm_failed"`
	StartedAt   time.Time  `json:"started_at"`
	CompletedAt *time.Time `json:"completed_at,omitempty"`
	Error       string     `json:"error,omitempty"`
}

// VMProvider looks up VM details from the data store.
type VMProvider interface {
	// GetVMsByHost returns all VMs assigned to a host.
	GetVMsByHost(ctx context.Context, hostID string) ([]VM, error)
	// GetVM returns a single VM by ID.
	GetVM(ctx context.Context, vmID string) (*VM, error)
	// UpdateVMHost updates the host assignment for a VM.
	UpdateVMHost(ctx context.Context, vmID, newHostID string) error
	// GetHAPolicy returns the HA policy for a VM.
	GetHAPolicy(ctx context.Context, vmID string) (*HAPolicy, error)
	// SetHAPolicy sets the HA policy for a VM.
	SetHAPolicy(ctx context.Context, vmID string, policy HAPolicy) error
	// ListHAPolicies returns all HA policies.
	ListHAPolicies(ctx context.Context) (map[string]HAPolicy, error)
}

// HostProvider looks up host details.
type HostProvider interface {
	// GetHost returns a host by ID.
	GetHost(ctx context.Context, hostID string) (*Host, error)
	// ListHosts returns all hosts matching the status filter (empty = all).
	ListHosts(ctx context.Context, status string) ([]Host, error)
}

// VMRestarter starts VMs on the target host.
type VMRestarter interface {
	// StartVM starts a VM on the target host.
	StartVM(ctx context.Context, vm VM, target Host) error
	// GetStartStatus returns the status of a VM start operation.
	GetStartStatus(ctx context.Context, vmID string) (string, error)
}

// EventPublisher emits lifecycle events.
type EventPublisher interface {
	Publish(ctx context.Context, eventType, severity, message, actorType, actorID, actorName,
		resourceType, resourceID, resourceName string) error
}

// DefaultOrchestrator implements Orchestrator.
type DefaultOrchestrator struct {
	mu              sync.Mutex
	fencer          Fencer
	scheduler       Scheduler
	vmProvider      VMProvider
	hostProvider    HostProvider
	vmRestarter     VMRestarter
	eventPublisher  EventPublisher
	activeFailovers map[string]*FailoverRecord
	history         []FailoverRecord
	maxHistory      int
	policy          Policy
}

// OrchestratorConfig holds dependencies for the orchestrator.
type OrchestratorConfig struct {
	Fencer         Fencer
	Scheduler      Scheduler
	VMProvider     VMProvider
	HostProvider   HostProvider
	VMRestarter    VMRestarter
	EventPublisher EventPublisher
	Policy         Policy
	MaxHistory     int
}

// NewOrchestrator creates a new orchestrator.
func NewOrchestrator(cfg OrchestratorConfig) (*DefaultOrchestrator, error) {
	if cfg.VMProvider == nil {
		return nil, fmt.Errorf("orchestrator: VMProvider is required")
	}
	if cfg.HostProvider == nil {
		return nil, fmt.Errorf("orchestrator: HostProvider is required")
	}
	if cfg.Scheduler == nil {
		cfg.Scheduler = NewDefaultScheduler()
	}
	if cfg.MaxHistory == 0 {
		cfg.MaxHistory = 100
	}
	if cfg.Policy.PolicyType == "" {
		cfg.Policy = DefaultPolicy()
	}

	return &DefaultOrchestrator{
		fencer:          cfg.Fencer,
		scheduler:       cfg.Scheduler,
		vmProvider:      cfg.VMProvider,
		hostProvider:    cfg.HostProvider,
		vmRestarter:     cfg.VMRestarter,
		eventPublisher:  cfg.EventPublisher,
		activeFailovers: make(map[string]*FailoverRecord),
		history:         make([]FailoverRecord, 0, cfg.MaxHistory),
		maxHistory:      cfg.MaxHistory,
		policy:          cfg.Policy,
	}, nil
}

// HandleHostFailure executes the full failover pipeline:
//  1. Verify host is truly failed
//  2. Fence the failed host to prevent split-brain
//  3. Identify affected VMs with auto-restart policy
//  4. Select target hosts via scheduler
//  5. Restart VMs on new hosts
func (o *DefaultOrchestrator) HandleHostFailure(ctx context.Context, nodeID string) error {
	o.mu.Lock()

	// Check if a failover is already in progress for this node
	if _, exists := o.activeFailovers[nodeID]; exists {
		o.mu.Unlock()
		return fmt.Errorf("failover already in progress for node %s", nodeID)
	}

	record := &FailoverRecord{
		ID:        fmt.Sprintf("failover-%s-%d", nodeID, time.Now().Unix()),
		NodeID:    nodeID,
		State:     "detecting",
		StartedAt: time.Now(),
	}
	o.activeFailovers[nodeID] = record
	o.mu.Unlock()

	log.Printf("[HA/Orchestrator] Starting failover for node %s (id=%s)", nodeID, record.ID)

	// Step 1: Get affected VMs
	record.State = "detecting"
	vms, err := o.vmProvider.GetVMsByHost(ctx, nodeID)
	if err != nil {
		o.failFailover(record, fmt.Sprintf("get VMs on failed host: %v", err))
		return fmt.Errorf("get VMs on failed host: %w", err)
	}

	if len(vms) == 0 {
		log.Printf("[HA/Orchestrator] No VMs found on failed host %s", nodeID)
		o.completeFailover(record, 0, 0, 0)
		return nil
	}

	// Filter VMs by HA policy: only auto-restart VMs
	autoVMs := make([]VM, 0, len(vms))
	for _, vm := range vms {
		policy, err := o.vmProvider.GetHAPolicy(ctx, vm.ID)
		if err != nil {
			log.Printf("[HA/Orchestrator] Warning: could not get HA policy for VM %s: %v", vm.ID, err)
			continue
		}
		if policy.Mode == HAModeAuto || policy.Mode == HAModeMaxOne {
			autoVMs = append(autoVMs, vm)
		}
	}

	record.VMCount = len(autoVMs)

	if len(autoVMs) == 0 {
		log.Printf("[HA/Orchestrator] No auto-restart VMs on failed host %s", nodeID)
		o.completeFailover(record, 0, 0, 0)
		return nil
	}

	// Publish event
	if o.eventPublisher != nil {
		o.eventPublisher.Publish(ctx, "ha_failover_started", "warning",
			fmt.Sprintf("Host failure detected on %s, initiating failover for %d VMs", nodeID, len(autoVMs)),
			"system", "ha-controller", "HA Controller", "host", nodeID, nodeID)
	}

	// Step 2: Fence the failed host
	record.State = "fencing"
	if o.fencer != nil {
		log.Printf("[HA/Orchestrator] Fencing failed host %s", nodeID)
		if err := o.fencer.Fence(ctx, nodeID); err != nil {
			log.Printf("[HA/Orchestrator] WARNING: Fencing failed for node %s: %v", nodeID, err)
			// Continue anyway — fencing failure is not fatal for restart
			// (VMs can still be restarted, but split-brain risk exists)
		}
	} else {
		log.Printf("[HA/Orchestrator] No fencer configured, skipping fence for node %s", nodeID)
	}

	// Step 3: Get available target hosts
	availableHosts, err := o.hostProvider.ListHosts(ctx, "online")
	if err != nil {
		o.failFailover(record, fmt.Sprintf("list available hosts: %v", err))
		return fmt.Errorf("list available hosts: %w", err)
	}

	// Filter out the failed host and maintenance mode hosts
	targets := make([]Host, 0, len(availableHosts))
	for _, h := range availableHosts {
		if h.ID == nodeID {
			continue
		}
		if h.MaintenanceMode {
			continue
		}
		targets = append(targets, h)
	}

	if len(targets) == 0 {
		o.failFailover(record, "no available target hosts for failover")
		return fmt.Errorf("no available target hosts for failover of node %s", nodeID)
	}

	// Step 4: Schedule VMs to target hosts
	record.State = "scheduling"
	log.Printf("[HA/Orchestrator] Scheduling %d VMs across %d target hosts", len(autoVMs), len(targets))
	assignments, err := o.scheduler.SelectTargets(autoVMs, targets, o.policy)
	if err != nil {
		o.failFailover(record, fmt.Sprintf("schedule VMs: %v", err))
		return fmt.Errorf("schedule VMs: %w", err)
	}

	// Step 5: Restart VMs
	record.State = "restarting"
	log.Printf("[HA/Orchestrator] Restarting %d VMs", len(assignments))

	restarted := 0
	failed := 0

	for _, vm := range autoVMs {
		target, ok := assignments[vm.ID]
		if !ok {
			log.Printf("[HA/Orchestrator] No target assigned for VM %s, skipping", vm.Name)
			failed++
			continue
		}

		if err := o.RestartVMs(ctx, []VM{vm}, *target); err != nil {
			log.Printf("[HA/Orchestrator] Failed to restart VM %s on host %s: %v",
				vm.Name, target.Name, err)
			failed++
		} else {
			restarted++
		}
	}

	o.completeFailover(record, len(autoVMs), restarted, failed)

	if o.eventPublisher != nil {
		severity := "info"
		if failed > 0 {
			severity = "warning"
		}
		o.eventPublisher.Publish(ctx, "ha_failover_complete", severity,
			fmt.Sprintf("Failover for node %s complete: %d/%d VMs restarted", nodeID, restarted, len(autoVMs)),
			"system", "ha-controller", "HA Controller", "host", nodeID, nodeID)
	}

	log.Printf("[HA/Orchestrator] Failover for node %s complete: %d/%d VMs restarted, %d failed",
		nodeID, restarted, len(autoVMs), failed)

	return nil
}

// RestartVMs starts the given VMs on the target host.
func (o *DefaultOrchestrator) RestartVMs(ctx context.Context, vms []VM, target Host) error {
	for _, vm := range vms {
		log.Printf("[HA/Orchestrator] Restarting VM %s on host %s", vm.Name, target.Name)

		// Update host assignment in the database
		if err := o.vmProvider.UpdateVMHost(ctx, vm.ID, target.ID); err != nil {
			return fmt.Errorf("update VM %s host to %s: %w", vm.Name, target.Name, err)
		}

		// If a VM restarter is configured, use it
		if o.vmRestarter != nil {
			if err := o.vmRestarter.StartVM(ctx, vm, target); err != nil {
				return fmt.Errorf("start VM %s on host %s: %w", vm.Name, target.Name, err)
			}
		} else {
			log.Printf("[HA/Orchestrator] No VM restarter configured, VM %s marked for restart on %s",
				vm.Name, target.Name)
		}
	}

	return nil
}

// GetActiveFailovers returns currently executing failovers.
func (o *DefaultOrchestrator) GetActiveFailovers() []FailoverRecord {
	o.mu.Lock()
	defer o.mu.Unlock()

	result := make([]FailoverRecord, 0, len(o.activeFailovers))
	for _, r := range o.activeFailovers {
		result = append(result, *r)
	}
	return result
}

// GetFailoverHistory returns past failovers.
func (o *DefaultOrchestrator) GetFailoverHistory(limit int) []FailoverRecord {
	o.mu.Lock()
	defer o.mu.Unlock()

	if limit <= 0 || limit > len(o.history) {
		limit = len(o.history)
	}
	start := len(o.history) - limit
	return append([]FailoverRecord{}, o.history[start:]...)
}

// Complete marks a failover as successfully completed.
func (o *DefaultOrchestrator) completeFailover(record *FailoverRecord, total, restarted, failed int) {
	o.mu.Lock()
	defer o.mu.Unlock()

	now := time.Now()
	record.State = "complete"
	record.VMCount = total
	record.VMRestarted = restarted
	record.VMFailed = failed
	record.CompletedAt = &now

	delete(o.activeFailovers, record.NodeID)
	o.addToHistory(*record)
}

// failFailover marks a failover as failed.
func (o *DefaultOrchestrator) failFailover(record *FailoverRecord, errMsg string) {
	o.mu.Lock()
	defer o.mu.Unlock()

	now := time.Now()
	record.State = "failed"
	record.Error = errMsg
	record.CompletedAt = &now

	delete(o.activeFailovers, record.NodeID)
	o.addToHistory(*record)

	if o.eventPublisher != nil {
		o.eventPublisher.Publish(context.Background(), "ha_failover_failed", "error",
			fmt.Sprintf("Failover for node %s failed: %s", record.NodeID, errMsg),
			"system", "ha-controller", "HA Controller", "host", record.NodeID, record.NodeID)
	}
}

func (o *DefaultOrchestrator) addToHistory(record FailoverRecord) {
	o.history = append(o.history, record)
	if len(o.history) > o.maxHistory {
		o.history = o.history[len(o.history)-o.maxHistory:]
	}
}
