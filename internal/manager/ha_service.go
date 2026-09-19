// Package manager provides HiveStack Manager HA service integration.
//
// This file integrates the HA subsystem (health checks, fencing, failover orchestration)
// with the Manager core: wiring events, metrics, and heartbeat processing from node agents.
package manager

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/maddydevel/HiveStack/internal/db"
	"github.com/maddydevel/HiveStack/internal/ha"
)

// haService implements ha.VMProvider and ha.HostProvider, bridging the HA subsystem
// with the Manager's database and node registry.
type haService struct {
	mu sync.Mutex

	db           *db.DB
	manager      *Manager

	// HA subsystems
	controller    *ha.Controller
	haController  haControllerAdapter
	haOrchestrator ha.Orchestrator
	policyManager *ha.PolicyManager

	// Configuration
	thresholds   ha.HealthThresholds
	running      bool
	cancel       context.CancelFunc

	// Dependencies for orchestrator
	fencer    ha.Fencer
	scheduler ha.Scheduler
	storage   *ha.StorageManager
}

// haControllerAdapter wraps ha.Controller to satisfy the interface expected by the API.
type haControllerAdapter struct {
	ctrl *ha.Controller
}

func (a *haControllerAdapter) GetStatus() map[string]interface{} {
	if a.ctrl == nil {
		return map[string]interface{}{}
	}
	return a.ctrl.GetStatus()
}

func (a *haControllerAdapter) GetAllHealth() map[string]*ha.NodeHealth {
	if a.ctrl == nil {
		return nil
	}
	return a.ctrl.GetAllHealth()
}

func (a *haControllerAdapter) TriggerFailover(ctx context.Context, nodeID string) error {
	if a.ctrl == nil {
		return fmt.Errorf("HA controller not running")
	}
	return a.ctrl.TriggerFailover(ctx, nodeID)
}

// vmRestartAdapter implements ha.VMRestarter.
type vmRestartAdapter struct {
	manager *Manager
}

func (r *vmRestartAdapter) StartVM(ctx context.Context, vm ha.VM, target ha.Host) error {
	// Forward to the Manager's VM start flow
	log.Printf("[HA-Service] Forwarding VM %s start to node agent on host %s", vm.Name, target.ID)
	agent, ok := r.manager.GetNode(target.ID)
	if !ok || agent == nil {
		return fmt.Errorf("node agent for host %s not available", target.ID)
	}
	// Use the agent's StartVM method via context
	return agent.StartVM(ctx, vm.ID)
}

func (r *vmRestartAdapter) GetStartStatus(ctx context.Context, vmID string) (string, error) {
	dbVM, err := r.manager.DB().GetVM(ctx, vmID)
	if err != nil {
		return "", err
	}
	return dbVM.Status, nil
}

// eventPublisherAdapter implements ha.EventPublisher.
type eventPublisherAdapter struct {
	manager *Manager
}

func (e *eventPublisherAdapter) Publish(ctx context.Context, eventType, severity, message, actorType, actorID, actorName,
	resourceType, resourceID, resourceName string) error {
	return e.manager.publishEvent(ctx, "", severity, message,
		actorType, actorID, actorName, resourceType, resourceID, resourceName)
}

// NewHAService creates and initializes the HA service for the Manager.
func NewHAService(manager *Manager, thresholds ha.HealthThresholds) (*haService, error) {
	if err := thresholds.Validate(); err != nil {
		return nil, fmt.Errorf("invalid HA thresholds: %w", err)
	}

	svc := &haService{
		db:         manager.DB(),
		manager:    manager,
		thresholds: thresholds,
		scheduler:  ha.NewDefaultScheduler(),
		storage:    ha.NewStorageManager(),
	}

	// Create the policy manager (nil history store for now)
	svc.policyManager = ha.NewPolicyManager(nil)

	return svc, nil
}

// SetFencer sets the fencing backend.
func (s *haService) SetFencer(f ha.Fencer) {
	s.fencer = f
}

// SetScheduler sets a custom scheduler.
func (s *haService) SetScheduler(sched ha.Scheduler) {
	s.scheduler = sched
}

// GetController returns the HA controller.
func (s *haService) GetController() *ha.Controller {
	return s.controller
}

// GetOrchestrator returns the HA orchestrator.
func (s *haService) GetOrchestrator() ha.Orchestrator {
	return s.haOrchestrator
}

// GetPolicyManager returns the policy manager.
func (s *haService) GetPolicyManager() *ha.PolicyManager {
	return s.policyManager
}

// Start initializes and starts the HA controller.
func (s *haService) Start(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.running {
		return fmt.Errorf("HA service already running")
	}

	// Create heartbeat processor with state transition callback
	processor := ha.NewHeartbeatProcessor(s.thresholds, func(nodeID string, oldState, newState ha.HealthState, vms []string) {
		s.onStateTransition(ctx, nodeID, oldState, newState, vms)
	})

	// Create orchestrator config
	orchConfig := ha.OrchestratorConfig{
		Fencer:         s.fencer,
		Scheduler:      s.scheduler,
		VMProvider:     s,
		HostProvider:   s,
		VMRestarter:    &vmRestartAdapter{manager: s.manager},
		EventPublisher: &eventPublisherAdapter{manager: s.manager},
		Policy:         ha.DefaultPolicy(),
	}

	orchestrator, err := ha.NewOrchestrator(orchConfig)
	if err != nil {
		return fmt.Errorf("create HA orchestrator: %w", err)
	}
	s.haOrchestrator = orchestrator

	// Create controller config
	ctrlConfig := ha.ControllerConfig{
		HeartbeatProcessor: processor,
		Orchestrator:       orchestrator,
		Fencer:             s.fencer,
		Scheduler:          s.scheduler,
		Threshold:          s.thresholds,
		FailoverTimeout:    5 * time.Minute,
		MetricsCallback:    s.onMetrics,
		EventCallback:      s.onEvent,
	}

	controller, err := ha.NewController(ctrlConfig)
	if err != nil {
		return fmt.Errorf("create HA controller: %w", err)
	}
	s.controller = controller
	s.haController = haControllerAdapter{ctrl: controller}

	// Start the controller
	if err := controller.Start(ctx); err != nil {
		return fmt.Errorf("start HA controller: %w", err)
	}

	s.running = true

	// Register all currently known nodes
	s.registerExistingNodes()

	log.Println("[HA-Service] HA service started successfully")
	return nil
}

// Stop gracefully stops the HA service.
func (s *haService) Stop() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.running {
		return nil
	}

	if s.controller != nil {
		if err := s.controller.Stop(); err != nil {
			log.Printf("[HA-Service] Error stopping controller: %v", err)
		}
	}

	if s.cancel != nil {
		s.cancel()
	}

	s.running = false
	log.Println("[HA-Service] HA service stopped")
	return nil
}

// ProcessHeartbeat processes an incoming heartbeat from a node agent.
func (s *haService) ProcessHeartbeat(ctx context.Context, hb *ha.Heartbeat) error {
	if s.controller == nil {
		return fmt.Errorf("HA controller not running")
	}
	return s.controller.ProcessHeartbeat(ctx, hb)
}

// RegisterNode adds a node to HA monitoring.
func (s *haService) RegisterNode(nodeID string) {
	if s.controller != nil {
		s.controller.RegisterNode(nodeID)
	}
}

// UnregisterNode removes a node from HA monitoring.
func (s *haService) UnregisterNode(nodeID string) {
	if s.controller != nil {
		s.controller.UnregisterNode(nodeID)
	}
}

// registerExistingNodes populates HA tracking with currently registered nodes.
func (s *haService) registerExistingNodes() {
	s.manager.mu.Lock()
	for id := range s.manager.nodes {
		s.controller.RegisterNode(id)
	}
	s.manager.mu.Unlock()

	// Also register hosts from the database
	hosts, err := s.manager.ListHosts(context.Background())
	if err != nil {
		log.Printf("[HA-Service] Warning: failed to list existing hosts: %v", err)
		return
	}
	for _, h := range hosts {
		s.controller.RegisterNode(h.ID)
	}
}

// onStateTransition is called when a node's health state changes.
func (s *haService) onStateTransition(ctx context.Context, nodeID string, oldState, newState ha.HealthState, vms []string) {
	log.Printf("[HA-Service] State transition for node %s: %s -> %s (affected VMs: %d)",
		nodeID, oldState, newState, len(vms))

	switch newState {
	case ha.StateSuspect:
		s.manager.publishEvent(ctx, "", "warning",
			fmt.Sprintf("Node %s is suspect (missed heartbeats)", nodeID),
			"system", "ha-service", "HA Service", "host", nodeID, nodeID)

	case ha.StateOffline:
		s.manager.publishEvent(ctx, "", "critical",
			fmt.Sprintf("Node %s is offline, initiating failover for %d VMs", nodeID, len(vms)),
			"system", "ha-service", "HA Service", "host", nodeID, nodeID)
	}
}

// onMetrics handles HA metrics updates.
func (s *haService) onMetrics(metric string, value float64, labels map[string]string) {
	// In production, forward to Prometheus or metrics backend
	log.Printf("[HA-Service] Metric: %s = %v (labels: %v)", metric, value, labels)
}

// onEvent handles HA events.
func (s *haService) onEvent(eventType, severity, message string) {
	log.Printf("[HA-Service] Event [%s/%s]: %s", eventType, severity, message)
}

// ---------- ha.VMProvider interface ----------

// GetVMsByHost returns all VMs assigned to a host.
func (s *haService) GetVMsByHost(ctx context.Context, hostID string) ([]ha.VM, error) {
	dbVMs, err := s.db.ListVMs(ctx, "")
	if err != nil {
		return nil, err
	}

	var vms []ha.VM
	for i := range dbVMs {
		if dbVMs[i].HostID != nil && *dbVMs[i].HostID == hostID {
			vm := dbVMs[i]
			hvm := ha.VM{
				ID:               vm.ID,
				Name:             vm.Name,
				HostID:           *vm.HostID,
				CPUs:             vm.CPUs,
				MemoryBytes:      vm.MemoryBytes,
				NUMAPolicy:       vm.NUMAPolicy,
				HugepagesEnabled: vm.HugepagesEnabled,
				Role:             string(vm.Role),
			}
			// Apply HA policy settings if they exist
			if policy, ok := s.policyManager.GetPolicy(vm.ID); ok {
				hvm.AntiAffinity = policy.AntiAffinity
			}
			vms = append(vms, hvm)
		}
	}
	return vms, nil
}

// GetVM returns a single VM by ID.
func (s *haService) GetVM(ctx context.Context, vmID string) (*ha.VM, error) {
	dbVM, err := s.db.GetVM(ctx, vmID)
	if err != nil {
		return nil, err
	}
	hvm := &ha.VM{
		ID:               dbVM.ID,
		Name:             dbVM.Name,
		CPUs:             dbVM.CPUs,
		MemoryBytes:      dbVM.MemoryBytes,
		NUMAPolicy:       dbVM.NUMAPolicy,
		HugepagesEnabled: dbVM.HugepagesEnabled,
		Role:             string(dbVM.Role),
	}
	if dbVM.HostID != nil {
		hvm.HostID = *dbVM.HostID
	}
	return hvm, nil
}

// UpdateVMHost updates the host assignment for a VM.
func (s *haService) UpdateVMHost(ctx context.Context, vmID, newHostID string) error {
	return s.db.UpdateVM(ctx, vmID, map[string]interface{}{"host_id": newHostID})
}

// GetHAPolicy returns the HA policy for a VM.
func (s *haService) GetHAPolicy(ctx context.Context, vmID string) (*ha.HAPolicy, error) {
	policy, ok := s.policyManager.GetPolicy(vmID)
	if !ok {
		defaultPolicy := ha.DefaultHAPolicy(vmID)
		return &defaultPolicy, nil
	}
	return &policy, nil
}

// SetHAPolicy sets the HA policy for a VM.
func (s *haService) SetHAPolicy(ctx context.Context, vmID string, policy ha.HAPolicy) error {
	return s.policyManager.SetPolicy(vmID, policy, "manager")
}

// ListHAPolicies returns all HA policies.
func (s *haService) ListHAPolicies(ctx context.Context) (map[string]ha.HAPolicy, error) {
	return s.policyManager.ListPolicies(), nil
}

// ---------- ha.HostProvider interface ----------

// GetHost returns a host by ID.
func (s *haService) GetHost(ctx context.Context, hostID string) (*ha.Host, error) {
	dbHost, err := s.db.GetHost(ctx, hostID)
	if err != nil {
		return nil, err
	}
	h := &ha.Host{
		ID:               dbHost.ID,
		Name:             dbHost.Name,
		Hostname:         dbHost.Hostname,
		Status:           dbHost.Status,
		CPUCount:         dbHost.CPUCount,
		MemoryTotalBytes: dbHost.MemoryTotalBytes,
		NUMANodeCount:    dbHost.NUMANodeCount,
		MaintenanceMode:  dbHost.MaintenanceMode,
	}
	return h, nil
}

// ListHosts returns all hosts matching the status filter.
func (s *haService) ListHosts(ctx context.Context, status string) ([]ha.Host, error) {
	dbHosts, err := s.db.ListHosts(ctx, "")
	if err != nil {
		return nil, err
	}

	var hosts []ha.Host
	for i := range dbHosts {
		if status != "" && dbHosts[i].Status != status {
			continue
		}
		hosts = append(hosts, ha.Host{
			ID:               dbHosts[i].ID,
			Name:             dbHosts[i].Name,
			Hostname:         dbHosts[i].Hostname,
			Status:           dbHosts[i].Status,
			CPUCount:         dbHosts[i].CPUCount,
			MemoryTotalBytes: dbHosts[i].MemoryTotalBytes,
			NUMANodeCount:    dbHosts[i].NUMANodeCount,
			MaintenanceMode:  dbHosts[i].MaintenanceMode,
		})
	}
	return hosts, nil
}

// TriggerManualFailover triggers a manual failover for a node (used by API).
func (s *haService) TriggerManualFailover(ctx context.Context, nodeID string) error {
	if s.controller == nil {
		return fmt.Errorf("HA controller not running")
	}
	return s.controller.TriggerFailover(ctx, nodeID)
}

// GetStatus returns the current HA service status.
func (s *haService) GetStatus() map[string]interface{} {
	if s.controller == nil {
		return map[string]interface{}{
			"running": false,
		}
	}
	return s.controller.GetStatus()
}

// Ensure haService implements the required interfaces.
var _ ha.VMProvider = (*haService)(nil)
var _ ha.HostProvider = (*haService)(nil)
