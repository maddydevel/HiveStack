// Package ha provides Non-HANA High Availability for HiveStack.
//
// The HA subsystem implements:
//   - Heartbeat processing and health state tracking (online/suspect/offline)
//   - Configurable failure detection thresholds
//   - Fencing (IPMI, Redfish, SSH power control) to prevent split-brain
//   - Host selection for VM restart based on capacity, NUMA, and anti-affinity
//   - VM restart orchestration on host failure
//   - Per-VM HA policy (auto-restart, manual, never)
//   - Shared storage integration (NFS, Ceph RBD, iSCSI) for VM state reconstruction
//   - REST API for HA operations
//
// Defaults:
//   - Heartbeat interval: 30s (configurable)
//   - Suspect threshold: 3 missed heartbeats (90s)
//   - Offline threshold: 5 missed heartbeats (150s)
//   - Auto-restart SLA: VM restarted on healthy host within 5 minutes
package ha

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"
)

// HealthState represents the current health status of a host node.
type HealthState int

const (
	// StateOnline indicates the host is healthy and responding to heartbeats.
	StateOnline HealthState = iota
	// StateSuspect indicates the host has missed heartbeats and may be failing.
	StateSuspect
	// StateOffline indicates the host has exceeded the offline threshold and is considered failed.
	StateOffline
)

// String implements fmt.Stringer for HealthState.
func (h HealthState) String() string {
	switch h {
	case StateOnline:
		return "online"
	case StateSuspect:
		return "suspect"
	case StateOffline:
		return "offline"
	default:
		return "unknown"
	}
}

// HealthStateFrom converts a string to a HealthState.
func HealthStateFrom(s string) HealthState {
	switch s {
	case "online":
		return StateOnline
	case "suspect":
		return StateSuspect
	case "offline":
		return StateOffline
	default:
		return StateOffline
	}
}

// HostResources describes the available resources on a host.
type HostResources struct {
	CPUCount          int     `json:"cpu_count"`
	MemoryTotalBytes  int64   `json:"memory_total_bytes"`
	MemoryUsedBytes   int64   `json:"memory_used_bytes"`
	StorageTotalBytes int64   `json:"storage_total_bytes"`
	StorageUsedBytes  int64   `json:"storage_used_bytes"`
	NUMANodeCount     int     `json:"numa_node_count"`
	CPUUsagePercent   float64 `json:"cpu_usage_percent"`
}

// AvailableMemory returns the remaining memory available for new VMs.
func (r *HostResources) AvailableMemory() int64 {
	avail := r.MemoryTotalBytes - r.MemoryUsedBytes
	if avail < 0 {
		return 0
	}
	return avail
}

// AvailableCPU returns the remaining CPU capacity (simplified).
func (r *HostResources) AvailableCPU() int {
	// Estimate based on usage percentage
	avail := int(float64(r.CPUCount) * (100.0 - r.CPUUsagePercent) / 100.0)
	if avail < 0 {
		avail = 0
	}
	return avail
}

// Heartbeat represents a health signal sent by a node agent.
type Heartbeat struct {
	NodeID    string        `json:"node_id"`
	Timestamp time.Time     `json:"timestamp"`
	VMs       []string      `json:"vms"`       // List of VM IDs running on this host
	Resources HostResources `json:"resources"` // Current resource utilization
	Sequence  uint64        `json:"sequence"`  // Monotonically increasing sequence number
}

// Validate checks the heartbeat for validity.
func (h *Heartbeat) Validate() error {
	if h.NodeID == "" {
		return fmt.Errorf("heartbeat: node_id is required")
	}
	if h.Timestamp.IsZero() {
		return fmt.Errorf("heartbeat: timestamp is required")
	}
	return nil
}

// NodeHealth tracks the health state and heartbeat history for a single host.
type NodeHealth struct {
	NodeID           string      `json:"node_id"`
	State            HealthState `json:"state"`
	LastHeartbeat    time.Time   `json:"last_heartbeat"`
	MissedHeartbeats int         `json:"missed_beats"`
	FirstMissedAt    *time.Time  `json:"first_missed_at,omitempty"`
	LastSequence     uint64      `json:"last_sequence"`
	VMs              []string    `json:"vms"`
	Resources        HostResources `json:"resources"`
	mu               sync.RWMutex
}

// UpdateHeartbeat processes an incoming heartbeat for this node.
func (n *NodeHealth) UpdateHeartbeat(hb *Heartbeat) {
	n.mu.Lock()
	defer n.mu.Unlock()

	n.LastHeartbeat = hb.Timestamp
	n.MissedHeartbeats = 0
	n.FirstMissedAt = nil
	n.LastSequence = hb.Sequence
	n.Resources = hb.Resources
	n.VMs = hb.VMs

	if n.State != StateOnline {
		log.Printf("[HA] Node %s recovered from %s to online", n.NodeID, n.State)
		n.State = StateOnline
	}
}

// MissHeartbeat increments the missed heartbeat counter and updates state.
func (n *NodeHealth) MissHeartbeat(now time.Time, suspectThreshold, offlineThreshold int) {
	n.mu.Lock()
	defer n.mu.Unlock()

	n.MissedHeartbeats++
	if n.FirstMissedAt == nil {
		n.FirstMissedAt = &now
	}

	switch {
	case n.MissedHeartbeats >= offlineThreshold:
		if n.State != StateOffline {
			log.Printf("[HA] Node %s transitioned to offline (missed %d heartbeats)", n.NodeID, n.MissedHeartbeats)
			n.State = StateOffline
		}
	case n.MissedHeartbeats >= suspectThreshold:
		if n.State == StateOnline {
			log.Printf("[HA] Node %s transitioned to suspect (missed %d heartbeats)", n.NodeID, n.MissedHeartbeats)
			n.State = StateSuspect
		}
	}
}

// GetState returns the current health state (thread-safe).
func (n *NodeHealth) GetState() HealthState {
	n.mu.RLock()
	defer n.mu.RUnlock()
	return n.State
}

// GetMissedCount returns the count of missed heartbeats (thread-safe).
func (n *NodeHealth) GetMissedCount() int {
	n.mu.RLock()
	defer n.mu.RUnlock()
	return n.MissedHeartbeats
}

// HealthThresholds defines configurable parameters for failure detection.
type HealthThresholds struct {
	HeartbeatInterval  time.Duration `json:"heartbeat_interval"`
	SuspectThreshold   int           `json:"suspect_threshold"`  // missed heartbeats before suspect
	OfflineThreshold   int           `json:"offline_threshold"`  // missed heartbeats before offline
}

// DefaultThresholds returns the default health thresholds.
func DefaultThresholds() HealthThresholds {
	return HealthThresholds{
		HeartbeatInterval: 30 * time.Second,
		SuspectThreshold:  3, // 90 seconds
		OfflineThreshold:  5, // 150 seconds
	}
}

// Validate checks that thresholds are consistent.
func (t *HealthThresholds) Validate() error {
	if t.HeartbeatInterval <= 0 {
		return fmt.Errorf("heartbeat_interval must be positive")
	}
	if t.SuspectThreshold <= 0 {
		return fmt.Errorf("suspect_threshold must be positive")
	}
	if t.OfflineThreshold <= t.SuspectThreshold {
		return fmt.Errorf("offline_threshold (%d) must be greater than suspect_threshold (%d)",
			t.OfflineThreshold, t.SuspectThreshold)
	}
	return nil
}

// StateTransitionCallback is invoked when a node transitions to a new health state.
type StateTransitionCallback func(nodeID string, oldState, newState HealthState, vms []string)

// HeartbeatProcessor processes heartbeats and manages per-node health state.
type HeartbeatProcessor struct {
	mu        sync.RWMutex
	nodes     map[string]*NodeHealth
	threshold HealthThresholds
	callback  StateTransitionCallback
}

// NewHeartbeatProcessor creates a heartbeat processor with the given thresholds.
func NewHeartbeatProcessor(threshold HealthThresholds, cb StateTransitionCallback) *HeartbeatProcessor {
	return &HeartbeatProcessor{
		nodes:     make(map[string]*NodeHealth),
		threshold: threshold,
		callback:  cb,
	}
}

// RegisterNode adds a node to the health tracking system.
func (p *HeartbeatProcessor) RegisterNode(nodeID string) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if _, exists := p.nodes[nodeID]; !exists {
		log.Printf("[HA] Registering node %s for health monitoring", nodeID)
		p.nodes[nodeID] = &NodeHealth{
			NodeID: nodeID,
			State:  StateOnline,
		}
	}
}

// UnregisterNode removes a node from health tracking.
func (p *HeartbeatProcessor) UnregisterNode(nodeID string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	delete(p.nodes, nodeID)
}

// ProcessHeartbeat handles an incoming heartbeat, updating the node's state.
func (p *HeartbeatProcessor) ProcessHeartbeat(ctx context.Context, hb *Heartbeat) error {
	if err := hb.Validate(); err != nil {
		return fmt.Errorf("invalid heartbeat: %w", err)
	}

	p.mu.Lock()
	node, exists := p.nodes[hb.NodeID]
	if !exists {
		node = &NodeHealth{
			NodeID: hb.NodeID,
			State:  StateOnline,
		}
		p.nodes[hb.NodeID] = node
	}
	oldState := node.GetState()
	p.mu.Unlock()

	node.UpdateHeartbeat(hb)

	if oldState != StateOnline && p.callback != nil {
		p.callback(hb.NodeID, oldState, StateOnline, nil)
	}

	return nil
}

// CheckAll evaluates all registered nodes for missed heartbeats.
// Called by the controller's ticker.
func (p *HeartbeatProcessor) CheckAll(now time.Time) []StateTransition {
	p.mu.RLock()
	defer p.mu.RUnlock()

	var transitions []StateTransition

	for _, node := range p.nodes {
		node.mu.RLock()
		lastHB := node.LastHeartbeat
		currentState := node.State
		nodeID := node.NodeID
		node.mu.RUnlock()

		// Skip nodes that have never sent a heartbeat
		if lastHB.IsZero() {
			continue
		}

		elapsed := now.Sub(lastHB)
		expectedBeats := int(elapsed / p.threshold.HeartbeatInterval)

		if expectedBeats > 0 {
			node.mu.Lock()
			oldState := currentState
			node.MissHeartbeat(now, p.threshold.SuspectThreshold, p.threshold.OfflineThreshold)
			newState := node.State
			vms := append([]string{}, node.VMs...)
			node.mu.Unlock()

			if oldState != newState {
				transitions = append(transitions, StateTransition{
					NodeID:    nodeID,
					OldState:  oldState,
					NewState:  newState,
					VMs:       vms,
					Timestamp: now,
				})
				if p.callback != nil {
					p.callback(nodeID, oldState, newState, vms)
				}
			}
		}
	}

	return transitions
}

// StateTransition records a change in node health state.
type StateTransition struct {
	NodeID    string      `json:"node_id"`
	OldState  HealthState `json:"old_state"`
	NewState  HealthState `json:"new_state"`
	VMs       []string    `json:"vms"`
	Timestamp time.Time   `json:"timestamp"`
}

// GetNodeHealth returns the current health for a node.
func (p *HeartbeatProcessor) GetNodeHealth(nodeID string) (*NodeHealth, bool) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	node, ok := p.nodes[nodeID]
	return node, ok
}

// GetAllNodeHealth returns the health status of all tracked nodes.
func (p *HeartbeatProcessor) GetAllNodeHealth() map[string]*NodeHealth {
	p.mu.RLock()
	defer p.mu.RUnlock()

	result := make(map[string]*NodeHealth, len(p.nodes))
	for id, node := range p.nodes {
		result[id] = node
	}
	return result
}

// GetOfflineNodes returns nodes that are in StateOffline.
func (p *HeartbeatProcessor) GetOfflineNodes() []string {
	p.mu.RLock()
	defer p.mu.RUnlock()

	var offline []string
	for id, node := range p.nodes {
		if node.GetState() == StateOffline {
			offline = append(offline, id)
		}
	}
	return offline
}

// GetSuspectNodes returns nodes that are in StateSuspect.
func (p *HeartbeatProcessor) GetSuspectNodes() []string {
	p.mu.RLock()
	defer p.mu.RUnlock()

	var suspect []string
	for id, node := range p.nodes {
		if node.GetState() == StateSuspect {
			suspect = append(suspect, id)
		}
	}
	return suspect
}

// GetOnlineNodes returns nodes that are in StateOnline.
func (p *HeartbeatProcessor) GetOnlineNodes() []string {
	p.mu.RLock()
	defer p.mu.RUnlock()

	var online []string
	for id, node := range p.nodes {
		if node.GetState() == StateOnline {
			online = append(online, id)
		}
	}
	return online
}
