package ha

import (
	"context"
	"testing"
	"time"
)

// TestHeartbeatProcessing verifies that a heartbeat is correctly processed
// and the node state remains online.
func TestHeartbeatProcessing(t *testing.T) {
	thresholds := DefaultThresholds()
	cb := func(nodeID string, oldState, newState HealthState, vms []string) {}
	processor := NewHeartbeatProcessor(thresholds, cb)

	nodeID := "test-node-1"
	processor.RegisterNode(nodeID)

	hb := &Heartbeat{
		NodeID:    nodeID,
		Timestamp: time.Now(),
		VMs:       []string{"vm-1", "vm-2"},
		Sequence:  1,
		Resources: HostResources{
			CPUCount:         8,
			MemoryTotalBytes: 16 * 1024 * 1024 * 1024,
			MemoryUsedBytes:  4 * 1024 * 1024 * 1024,
		},
	}

	ctx := context.Background()
	if err := processor.ProcessHeartbeat(ctx, hb); err != nil {
		t.Fatalf("ProcessHeartbeat failed: %v", err)
	}

	node, ok := processor.GetNodeHealth(nodeID)
	if !ok {
		t.Fatalf("Node %s not found in health tracker", nodeID)
	}

	if node.GetState() != StateOnline {
		t.Errorf("Expected state %s, got %s", StateOnline, node.GetState())
	}

	if node.LastSequence != 1 {
		t.Errorf("Expected sequence 1, got %d", node.LastSequence)
	}

	if len(node.VMs) != 2 {
		t.Errorf("Expected 2 VMs, got %d", len(node.VMs))
	}
}

// TestStateTransition verifies that missed heartbeats cause proper
// state transitions: online -> suspect -> offline.
func TestStateTransition(t *testing.T) {
	thresholds := HealthThresholds{
		HeartbeatInterval: 1 * time.Second,
		SuspectThreshold:  3,
		OfflineThreshold:  5,
	}

	processor := NewHeartbeatProcessor(thresholds, nil)

	nodeID := "test-node-transition"
	processor.RegisterNode(nodeID)

	// Initial heartbeat to set online state
	hb := &Heartbeat{
		NodeID:    nodeID,
		Timestamp: time.Now(),
		Sequence:  1,
	}
	ctx := context.Background()
	if err := processor.ProcessHeartbeat(ctx, hb); err != nil {
		t.Fatalf("Initial heartbeat failed: %v", err)
	}

	// Simulate missed heartbeats by calling CheckAll multiple times
	// with incremental time advances (one heartbeat interval each)
	now := time.Now()

	// After 3 missed beats, should transition to suspect
	for i := 1; i <= 3; i++ {
		future := now.Add(time.Duration(i+1) * thresholds.HeartbeatInterval)
		processor.CheckAll(future)
	}

	node, ok := processor.GetNodeHealth(nodeID)
	if !ok {
		t.Fatalf("Node %s not found", nodeID)
	}
	if node.GetState() != StateSuspect {
		t.Errorf("Expected suspect state after 3 missed beats, got %s", node.GetState())
	}

	// After 5 total missed beats, should transition to offline
	for i := 4; i <= 5; i++ {
		future := now.Add(time.Duration(i+1) * thresholds.HeartbeatInterval)
		processor.CheckAll(future)
	}

	node, ok = processor.GetNodeHealth(nodeID)
	if !ok {
		t.Fatalf("Node %s not found", nodeID)
	}
	if node.GetState() != StateOffline {
		t.Errorf("Expected offline state after 5 missed beats, got %s", node.GetState())
	}

	// Verify that a recovery heartbeat brings it back online
	hb2 := &Heartbeat{
		NodeID:    nodeID,
		Timestamp: time.Now().Add(10 * time.Second),
		Sequence:  2,
	}
	if err := processor.ProcessHeartbeat(ctx, hb2); err != nil {
		t.Fatalf("Recovery heartbeat failed: %v", err)
	}

	node, ok = processor.GetNodeHealth(nodeID)
	if !ok {
		t.Fatalf("Node %s not found", nodeID)
	}
	if node.GetState() != StateOnline {
		t.Errorf("Expected online state after recovery, got %s", node.GetState())
	}
	if node.GetMissedCount() != 0 {
		t.Errorf("Expected 0 missed beats after recovery, got %d", node.GetMissedCount())
	}
}

// TestThresholdsValidation verifies that threshold validation works correctly.
func TestThresholdsValidation(t *testing.T) {
	// Default thresholds should validate
	defaults := DefaultThresholds()
	if err := defaults.Validate(); err != nil {
		t.Errorf("Default thresholds should validate: %v", err)
	}

	// Invalid: zero interval
	invalid1 := HealthThresholds{HeartbeatInterval: 0, SuspectThreshold: 3, OfflineThreshold: 5}
	if err := invalid1.Validate(); err == nil {
		t.Error("Expected error for zero heartbeat interval")
	}

	// Invalid: zero suspect threshold
	invalid2 := HealthThresholds{HeartbeatInterval: 30 * time.Second, SuspectThreshold: 0, OfflineThreshold: 5}
	if err := invalid2.Validate(); err == nil {
		t.Error("Expected error for zero suspect threshold")
	}

	// Invalid: offline <= suspect
	invalid3 := HealthThresholds{HeartbeatInterval: 30 * time.Second, SuspectThreshold: 5, OfflineThreshold: 3}
	if err := invalid3.Validate(); err == nil {
		t.Error("Expected error for offline <= suspect threshold")
	}

	// Valid custom thresholds
	valid := HealthThresholds{HeartbeatInterval: 10 * time.Second, SuspectThreshold: 2, OfflineThreshold: 4}
	if err := valid.Validate(); err != nil {
		t.Errorf("Valid thresholds should not error: %v", err)
	}
}

// TestGetAllNodeHealth verifies that health tracking works for multiple nodes.
func TestGetAllNodeHealth(t *testing.T) {
	thresholds := DefaultThresholds()
	cb := func(nodeID string, oldState, newState HealthState, vms []string) {}
	processor := NewHeartbeatProcessor(thresholds, cb)

	nodes := []string{"node-a", "node-b", "node-c"}
	for _, id := range nodes {
		processor.RegisterNode(id)
		hb := &Heartbeat{
			NodeID:    id,
			Timestamp: time.Now(),
			Sequence:  1,
		}
		if err := processor.ProcessHeartbeat(context.Background(), hb); err != nil {
			t.Fatalf("Heartbeat for %s failed: %v", id, err)
		}
	}

	allHealth := processor.GetAllNodeHealth()
	if len(allHealth) != 3 {
		t.Fatalf("Expected 3 nodes, got %d", len(allHealth))
	}

	for _, id := range nodes {
		node, ok := allHealth[id]
		if !ok {
			t.Errorf("Node %s not in GetAllNodeHealth result", id)
			continue
		}
		if node.GetState() != StateOnline {
			t.Errorf("Node %s expected online, got %s", id, node.GetState())
		}
	}

	online := processor.GetOnlineNodes()
	if len(online) != 3 {
		t.Errorf("Expected 3 online nodes, got %d", len(online))
	}

	// Verify HealthState string conversion
	if StateOnline.String() != "online" {
		t.Errorf("Expected 'online', got '%s'", StateOnline.String())
	}
	if StateSuspect.String() != "suspect" {
		t.Errorf("Expected 'suspect', got '%s'", StateSuspect.String())
	}
	if StateOffline.String() != "offline" {
		t.Errorf("Expected 'offline', got '%s'", StateOffline.String())
	}

	// Test HealthStateFrom
	if HealthStateFrom("online") != StateOnline {
		t.Error("HealthStateFrom('online') should return StateOnline")
	}
	if HealthStateFrom("suspect") != StateSuspect {
		t.Error("HealthStateFrom('suspect') should return StateSuspect")
	}
	if HealthStateFrom("offline") != StateOffline {
		t.Error("HealthStateFrom('offline') should return StateOffline")
	}
	if HealthStateFrom("invalid") != StateOffline {
		t.Error("HealthStateFrom('invalid') should default to StateOffline")
	}
}
