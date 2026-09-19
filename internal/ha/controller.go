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

	"github.com/maddydevel/HiveStack/internal/metrics"
)

// Event types published by the controller.
const (
	EventNodeSuspect      = "ha_node_suspect"
	EventNodeOffline      = "ha_node_offline"
	EventFailoverStarted  = "ha_failover_started"
	EventFailoverComplete = "ha_failover_complete"
	EventFailoverFailed   = "ha_failover_failed"
)

// eventPublishTimeout bounds how long publishing a single event may take.
const eventPublishTimeout = 5 * time.Second

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
	eventPublisher    EventPublisher
	inFlight          map[string]struct{} // nodes with a failover currently running
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
	// EventPublisher, if set, receives every HA event the controller raises
	// (in addition to EventCallback).
	EventPublisher EventPublisher
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
		eventPublisher:  cfg.EventPublisher,
		inFlight:        make(map[string]struct{}),
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
	if c.processor == nil {
		return
	}
	transitions := c.processor.CheckAll(now)

	for _, t := range transitions {
		c.handleTransition(ctx, t)
	}

	// Update metrics
	online := len(c.processor.GetOnlineNodes())
	suspect := len(c.processor.GetSuspectNodes())
	offline := len(c.processor.GetOfflineNodes())
	metrics.UpdateHAHealthMetrics(online, suspect, offline)
	if c.metricsCallback != nil {
		c.metricsCallback("ha_nodes_online", float64(online), nil)
		c.metricsCallback("ha_nodes_suspect", float64(suspect), nil)
		c.metricsCallback("ha_nodes_offline", float64(offline), nil)
	}
}

// handleTransition processes a health state transition.
func (c *Controller) handleTransition(ctx context.Context, t StateTransition) {
	log.Printf("[HA/Controller] State transition: node %s -> %s (was %s), affected VMs: %d",
		t.NodeID, t.NewState, t.OldState, len(t.VMs))

	switch t.NewState {
	case StateSuspect:
		c.emit(ctx, EventNodeSuspect, "warning", t.NodeID,
			fmt.Sprintf("Node %s is suspect (missed heartbeats)", t.NodeID))
	case StateOffline:
		c.emit(ctx, EventNodeOffline, "critical", t.NodeID,
			fmt.Sprintf("Node %s is offline, initiating failover", t.NodeID))
		// Trigger failover asynchronously
		go c.triggerFailover(ctx, t.NodeID)
	}
}

// emit delivers an HA event to the EventCallback and the EventPublisher.
// Publishing runs detached from ctx's cancellation so that failover outcomes
// are still recorded when the failover context timed out or the controller is
// shutting down.
func (c *Controller) emit(ctx context.Context, eventType, severity, nodeID, message string) {
	if c.eventCallback != nil {
		c.eventCallback(eventType, severity, message)
	}
	if c.eventPublisher == nil {
		return
	}

	pubCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), eventPublishTimeout)
	defer cancel()
	if err := c.eventPublisher.Publish(pubCtx, eventType, severity, message,
		"system", "ha-controller", "HA Controller", "host", nodeID, nodeID); err != nil {
		log.Printf("[HA/Controller] Failed to publish %s event for node %s: %v", eventType, nodeID, err)
	}
}

// beginFailover marks a failover as running for nodeID. It returns false if
// one is already running.
func (c *Controller) beginFailover(nodeID string) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	if _, running := c.inFlight[nodeID]; running {
		return false
	}
	c.inFlight[nodeID] = struct{}{}
	return true
}

func (c *Controller) endFailover(nodeID string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.inFlight, nodeID)
}

// triggerFailover starts the failover process for a failed node.
func (c *Controller) triggerFailover(ctx context.Context, nodeID string) {
	if !c.beginFailover(nodeID) {
		log.Printf("[HA/Controller] Failover already in progress for node %s, skipping", nodeID)
		return
	}
	defer c.endFailover(nodeID)

	log.Printf("[HA/Controller] Triggering failover for node %s", nodeID)
	c.emit(ctx, EventFailoverStarted, "warning", nodeID,
		fmt.Sprintf("Failover started for node %s", nodeID))

	// Create a timeout context for the failover operation
	failoverCtx, cancel := context.WithTimeout(ctx, c.failoverTimeout)
	defer cancel()

	start := time.Now()
	if err := c.orchestrator.HandleHostFailure(failoverCtx, nodeID); err != nil {
		log.Printf("[HA/Controller] Failover for node %s failed: %v", nodeID, err)
		c.emit(ctx, EventFailoverFailed, "error", nodeID,
			fmt.Sprintf("Failover for node %s failed: %v", nodeID, err))
		return
	}
	metrics.RecordFailover(time.Since(start))

	severity, message := "info", fmt.Sprintf("Failover for node %s complete", nodeID)
	if rec, ok := c.findFailoverRecord(nodeID, start); ok {
		message = fmt.Sprintf("Failover for node %s complete: %d/%d VMs restarted",
			nodeID, rec.VMRestarted, rec.VMCount)
		if rec.VMFailed > 0 {
			severity = "warning"
		}
	}
	c.emit(ctx, EventFailoverComplete, severity, nodeID, message)
}

// findFailoverRecord returns the orchestrator's record of the failover for
// nodeID that started at or after since.
func (c *Controller) findFailoverRecord(nodeID string, since time.Time) (FailoverRecord, bool) {
	history := c.orchestrator.GetFailoverHistory(0)
	for i := len(history) - 1; i >= 0; i-- {
		if history[i].NodeID == nodeID && !history[i].StartedAt.Before(since) {
			return history[i], true
		}
	}
	return FailoverRecord{}, false
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
