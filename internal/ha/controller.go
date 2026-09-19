// Package ha provides Non-HANA High Availability for HiveStack.
//
// The HA controller is the main loop that monitors host health,
// detects failures, and triggers the orchestrator for failover.
package ha

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"
)

// Controller manages the HA subsystem: runs the health check ticker,
// detects failures, and invokes the orchestrator.
type Controller struct {
	mu                sync.Mutex
	processor         *HeartbeatProcessor
	orchestrator      Orchestrator
	fencer            Fencer
	scheduler         Scheduler
	running           bool
	cancel            context.CancelFunc
	wg                sync.WaitGroup
	threshold         HealthThresholds
	failoverTimeout   time.Duration
	metricsCallback   func(metric string, value float64, labels map[string]string)
	eventCallback     func(eventType, severity, message string)
}

// ControllerConfig holds configuration for the HA controller.
type ControllerConfig struct {
	HeartbeatProcessor *HeartbeatProcessor
	Orchestrator       Orchestrator
	Fencer             Fencer
	Scheduler          Scheduler
	Threshold          HealthThresholds
	FailoverTimeout    time.Duration
	MetricsCallback    func(metric string, value float64, labels map[string]string)
	EventCallback      func(eventType, severity, message string)
}

// NewController creates a new HA controller.
func NewController(cfg ControllerConfig) (*Controller, error) {
	if cfg.Orchestrator == nil {
		return nil, fmt.Errorf("HA controller: orchestrator is required")
	}
	if err := cfg.Threshold.Validate(); err != nil {
		return nil, fmt.Errorf("HA controller: invalid thresholds: %w", err)
	}
	if cfg.FailoverTimeout <= 0 {
		cfg.FailoverTimeout = 5 * time.Minute
	}

	return &Controller{
		processor:       cfg.HeartbeatProcessor,
		orchestrator:    cfg.Orchestrator,
		fencer:          cfg.Fencer,
		scheduler:       cfg.Scheduler,
		threshold:       cfg.Threshold,
		failoverTimeout: cfg.FailoverTimeout,
		metricsCallback: cfg.MetricsCallback,
		eventCallback:   cfg.EventCallback,
	}, nil
}

// Start begins the HA controller's main loop.
func (c *Controller) Start(ctx context.Context) error {
	c.mu.Lock()
	if c.running {
		c.mu.Unlock()
		return fmt.Errorf("HA controller already running")
	}
	c.running = true
	ctx, cancel := context.WithCancel(ctx)
	c.cancel = cancel
	c.mu.Unlock()

	log.Println("[HA/Controller] Starting HA controller...")
	log.Printf("[HA/Controller] Thresholds: interval=%v, suspect=%d beats (%.0fs), offline=%d beats (%.0fs)",
		c.threshold.HeartbeatInterval,
		c.threshold.SuspectThreshold, c.threshold.HeartbeatInterval.Seconds()*float64(c.threshold.SuspectThreshold),
		c.threshold.OfflineThreshold, c.threshold.HeartbeatInterval.Seconds()*float64(c.threshold.OfflineThreshold))

	c.wg.Add(1)
	go c.mainLoop(ctx)

	return nil
}

// Stop gracefully shuts down the HA controller.
func (c *Controller) Stop() error {
	c.mu.Lock()
	if !c.running {
		c.mu.Unlock()
		return nil
	}
	c.running = false
	if c.cancel != nil {
		c.cancel()
	}
	c.mu.Unlock()

	c.wg.Wait()
	log.Println("[HA/Controller] HA controller stopped")
	return nil
}

// mainLoop runs the health check ticker and reconciliation.
func (c *Controller) mainLoop(ctx context.Context) {
	defer c.wg.Done()

	ticker := time.NewTicker(c.threshold.HeartbeatInterval)
	defer ticker.Stop()

	// Run initial check
	c.reconcile(ctx)

	for {
		select {
		case <-ctx.Done():
			log.Println("[HA/Controller] Context cancelled, shutting down")
			return
		case <-ticker.C:
			c.reconcile(ctx)
		}
	}
}

// reconcile checks all nodes for failures and triggers failover if needed.
func (c *Controller) reconcile(ctx context.Context) {
	now := time.Now()

	// Process heartbeat checks
	transitions := c.processor.CheckAll(now)

	for _, t := range transitions {
		c.handleTransition(ctx, t)
	}

	// Update metrics
	if c.metricsCallback != nil {
		online := c.processor.GetOnlineNodes()
		suspect := c.processor.GetSuspectNodes()
		offline := c.processor.GetOfflineNodes()
		c.metricsCallback("ha_nodes_online", float64(len(online)), nil)
		c.metricsCallback("ha_nodes_suspect", float64(len(suspect)), nil)
		c.metricsCallback("ha_nodes_offline", float64(len(offline)), nil)
	}
}

// handleTransition processes a health state transition.
func (c *Controller) handleTransition(ctx context.Context, t StateTransition) {
	log.Printf("[HA/Controller] State transition: node %s -> %s (was %s), affected VMs: %d",
		t.NodeID, t.NewState, t.OldState, len(t.VMs))

	switch t.NewState {
	case StateSuspect:
		if c.eventCallback != nil {
			c.eventCallback("ha_node_suspect", "warning",
				fmt.Sprintf("Node %s is suspect (missed heartbeats)", t.NodeID))
		}
	case StateOffline:
		if c.eventCallback != nil {
			c.eventCallback("ha_node_offline", "critical",
				fmt.Sprintf("Node %s is offline, initiating failover", t.NodeID))
		}
		// Trigger failover asynchronously
		go c.triggerFailover(ctx, t.NodeID)
	}
}

// triggerFailover starts the failover process for a failed node.
func (c *Controller) triggerFailover(ctx context.Context, nodeID string) {
	log.Printf("[HA/Controller] Triggering failover for node %s", nodeID)

	// Create a timeout context for the failover operation
	failoverCtx, cancel := context.WithTimeout(ctx, c.failoverTimeout)
	defer cancel()

	if err := c.orchestrator.HandleHostFailure(failoverCtx, nodeID); err != nil {
		log.Printf("[HA/Controller] Failover for node %s failed: %v", nodeID, err)
		if c.eventCallback != nil {
			c.eventCallback("ha_failover_failed", "error",
				fmt.Sprintf("Failover for node %s failed: %v", nodeID, err))
		}
	}
}

// RegisterNode adds a node to health monitoring.
func (c *Controller) RegisterNode(nodeID string) {
	if c.processor != nil {
		c.processor.RegisterNode(nodeID)
	}
}

// UnregisterNode removes a node from health monitoring.
func (c *Controller) UnregisterNode(nodeID string) {
	if c.processor != nil {
		c.processor.UnregisterNode(nodeID)
	}
}

// ProcessHeartbeat processes an incoming heartbeat.
func (c *Controller) ProcessHeartbeat(ctx context.Context, hb *Heartbeat) error {
	if c.processor == nil {
		return fmt.Errorf("heartbeat processor not configured")
	}
	return c.processor.ProcessHeartbeat(ctx, hb)
}

// GetNodeHealth returns the health status of a specific node.
func (c *Controller) GetNodeHealth(nodeID string) (*NodeHealth, bool) {
	if c.processor == nil {
		return nil, false
	}
	return c.processor.GetNodeHealth(nodeID)
}

// GetAllHealth returns health status of all tracked nodes.
func (c *Controller) GetAllHealth() map[string]*NodeHealth {
	if c.processor == nil {
		return nil
	}
	return c.processor.GetAllNodeHealth()
}

// IsRunning returns true if the controller is active.
func (c *Controller) IsRunning() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.running
}

// GetThresholds returns the current health thresholds.
func (c *Controller) GetThresholds() HealthThresholds {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.threshold
}

// SetThresholds updates the health thresholds at runtime.
func (c *Controller) SetThresholds(t HealthThresholds) error {
	if err := t.Validate(); err != nil {
		return fmt.Errorf("invalid thresholds: %w", err)
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.threshold = t
	log.Printf("[HA/Controller] Thresholds updated: interval=%v, suspect=%d, offline=%d",
		t.HeartbeatInterval, t.SuspectThreshold, t.OfflineThreshold)
	return nil
}

// TriggerFailover initiates a manual failover for a node (used by REST API).
func (c *Controller) TriggerFailover(ctx context.Context, nodeID string) error {
	log.Printf("[HA/Controller] Manual failover triggered for node %s", nodeID)
	go c.triggerFailover(ctx, nodeID)
	return nil
}

// GetStatus returns a summary of HA controller status.
func (c *Controller) GetStatus() map[string]interface{} {
	c.mu.Lock()
	defer c.mu.Unlock()

	status := map[string]interface{}{
		"running": c.running,
		"thresholds": c.threshold,
	}

	if c.processor != nil {
		status["online_nodes"] = len(c.processor.GetOnlineNodes())
		status["suspect_nodes"] = len(c.processor.GetSuspectNodes())
		status["offline_nodes"] = len(c.processor.GetOfflineNodes())
	}

	if c.orchestrator != nil {
		status["active_failovers"] = len(c.orchestrator.GetActiveFailovers())
	}

	return status
}
