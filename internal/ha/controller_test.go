package ha

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"
)

// mockOrchestrator is a test double for the Orchestrator interface.
type mockOrchestrator struct {
	mu              sync.Mutex
	failoverCalled  bool
	failoverNodeID  string
	failoverErr     error
	failoverDelay   time.Duration
	activeFailovers []FailoverRecord
	history         []FailoverRecord
}

func newMockOrchestrator() *mockOrchestrator {
	return &mockOrchestrator{
		activeFailovers: make([]FailoverRecord, 0),
		history:         make([]FailoverRecord, 0),
	}
}

func (m *mockOrchestrator) HandleHostFailure(ctx context.Context, nodeID string) error {
	m.mu.Lock()
	m.failoverCalled = true
	m.failoverNodeID = nodeID
	m.mu.Unlock()

	if m.failoverDelay > 0 {
		select {
		case <-time.After(m.failoverDelay):
		case <-ctx.Done():
			return ctx.Err()
		}
	}

	m.mu.Lock()
	defer m.mu.Unlock()
	if m.failoverErr != nil {
		return m.failoverErr
	}
	// Record the failover
	m.activeFailovers = append(m.activeFailovers, FailoverRecord{
		NodeID:    nodeID,
		State:     "complete",
		StartedAt: time.Now(),
	})
	return nil
}

func (m *mockOrchestrator) RestartVMs(ctx context.Context, vms []VM, target Host) error {
	return nil
}

func (m *mockOrchestrator) GetActiveFailovers() []FailoverRecord {
	m.mu.Lock()
	defer m.mu.Unlock()
	return append([]FailoverRecord{}, m.activeFailovers...)
}

func (m *mockOrchestrator) GetFailoverHistory(limit int) []FailoverRecord {
	m.mu.Lock()
	defer m.mu.Unlock()
	if limit <= 0 || limit > len(m.history) {
		limit = len(m.history)
	}
	return append([]FailoverRecord{}, m.history[limit:]...)
}

// mockFencer is a test double for the Fencer interface.
type mockFencer struct {
	fenceCalled bool
	fenceErr    error
}

func (m *mockFencer) Fence(ctx context.Context, nodeID string) error {
	m.fenceCalled = true
	return m.fenceErr
}

func (m *mockFencer) GetPowerState(ctx context.Context, nodeID string) (string, error) {
	return "off", nil
}

func (m *mockFencer) GetMethod() FenceMethod {
	return FenceMethodIPMI
}

// mockScheduler is a test double for the Scheduler interface.
type mockScheduler struct {
	selectTargetCalled bool
	selectTargetErr    error
}

func (m *mockScheduler) SelectTarget(vm VM, hosts []Host, policy Policy) (*Host, error) {
	m.selectTargetCalled = true
	if m.selectTargetErr != nil {
		return nil, m.selectTargetErr
	}
	if len(hosts) == 0 {
		return nil, errors.New("no hosts available")
	}
	return &hosts[0], nil
}

func (m *mockScheduler) SelectTargets(vms []VM, hosts []Host, policy Policy) (map[string]*Host, error) {
	m.selectTargetCalled = true
	if m.selectTargetErr != nil {
		return nil, m.selectTargetErr
	}
	assignments := make(map[string]*Host)
	for i := range vms {
		if len(hosts) > 0 {
			assignments[vms[i].ID] = &hosts[0]
		}
	}
	return assignments, nil
}

func (m *mockScheduler) ScoreHost(vm VM, host Host, policy Policy) float64 {
	return 0.0
}

// --- Test: NewController ---

func TestNewController(t *testing.T) {
	tests := []struct {
		name        string
		cfg         ControllerConfig
		expectErr   bool
		errContains string
	}{
		{
			name:        "nil orchestrator returns error",
			cfg:         ControllerConfig{},
			expectErr:   true,
			errContains: "orchestrator is required",
		},
		{
			name: "invalid thresholds returns error",
			cfg: ControllerConfig{
				Orchestrator:    newMockOrchestrator(),
				Threshold:       HealthThresholds{HeartbeatInterval: 0, SuspectThreshold: 3, OfflineThreshold: 5},
				FailoverTimeout: 5 * time.Minute,
			},
			expectErr:   true,
			errContains: "invalid thresholds",
		},
		{
			name: "offline <= suspect threshold returns error",
			cfg: ControllerConfig{
				Orchestrator:    newMockOrchestrator(),
				Threshold:       HealthThresholds{HeartbeatInterval: 30 * time.Second, SuspectThreshold: 5, OfflineThreshold: 3},
				FailoverTimeout: 5 * time.Minute,
			},
			expectErr:   true,
			errContains: "invalid thresholds",
		},
		{
			name: "valid config succeeds",
			cfg: ControllerConfig{
				Orchestrator:    newMockOrchestrator(),
				Threshold:       DefaultThresholds(),
				FailoverTimeout: 5 * time.Minute,
			},
			expectErr: false,
		},
		{
			name: "zero failover timeout gets default",
			cfg: ControllerConfig{
				Orchestrator:    newMockOrchestrator(),
				Threshold:       DefaultThresholds(),
				FailoverTimeout: 0,
			},
			expectErr: false,
		},
		{
			name: "nil processor is allowed",
			cfg: ControllerConfig{
				Orchestrator:       newMockOrchestrator(),
				Threshold:          DefaultThresholds(),
				FailoverTimeout:    5 * time.Minute,
				HeartbeatProcessor: nil,
			},
			expectErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl, err := NewController(tt.cfg)
			if tt.expectErr {
				if err == nil {
					t.Fatalf("expected error containing %q, got nil", tt.errContains)
				}
				if tt.errContains != "" && !contains(err.Error(), tt.errContains) {
					t.Errorf("expected error containing %q, got %q", tt.errContains, err.Error())
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if ctrl == nil {
				t.Fatal("expected non-nil controller")
			}
			if ctrl.orchestrator == nil {
				t.Error("expected orchestrator to be set")
			}
			// Check default failover timeout was applied
			if tt.cfg.FailoverTimeout == 0 {
				if ctrl.failoverTimeout != 5*time.Minute {
					t.Errorf("expected default failover timeout 5m, got %v", ctrl.failoverTimeout)
				}
			}
		})
	}
}

// --- Test: Start/Stop lifecycle ---

func TestStartStop(t *testing.T) {
	t.Run("start then stop succeeds", func(t *testing.T) {
		ctrl, err := NewController(ControllerConfig{
			Orchestrator:    newMockOrchestrator(),
			Threshold:       HealthThresholds{HeartbeatInterval: 100 * time.Millisecond, SuspectThreshold: 3, OfflineThreshold: 5},
			FailoverTimeout: 5 * time.Minute,
		})
		if err != nil {
			t.Fatalf("NewController: %v", err)
		}

		ctx := context.Background()
		if err := ctrl.Start(ctx); err != nil {
			t.Fatalf("Start failed: %v", err)
		}

		if !ctrl.IsRunning() {
			t.Error("expected controller to be running")
		}

		// Let the ticker fire a few times
		time.Sleep(250 * time.Millisecond)

		if err := ctrl.Stop(); err != nil {
			t.Fatalf("Stop failed: %v", err)
		}

		if ctrl.IsRunning() {
			t.Error("expected controller to be stopped")
		}
	})

	t.Run("double start returns error", func(t *testing.T) {
		ctrl, err := NewController(ControllerConfig{
			Orchestrator:    newMockOrchestrator(),
			Threshold:       DefaultThresholds(),
			FailoverTimeout: 5 * time.Minute,
		})
		if err != nil {
			t.Fatalf("NewController: %v", err)
		}

		ctx := context.Background()
		if err := ctrl.Start(ctx); err != nil {
			t.Fatalf("First Start failed: %v", err)
		}
		defer ctrl.Stop()

		if err := ctrl.Start(ctx); err == nil {
			t.Error("expected error on double start")
		}
	})

	t.Run("stop when not running is no-op", func(t *testing.T) {
		ctrl, err := NewController(ControllerConfig{
			Orchestrator:    newMockOrchestrator(),
			Threshold:       DefaultThresholds(),
			FailoverTimeout: 5 * time.Minute,
		})
		if err != nil {
			t.Fatalf("NewController: %v", err)
		}

		if err := ctrl.Stop(); err != nil {
			t.Errorf("Stop on non-running controller should not error: %v", err)
		}
	})

	t.Run("context cancellation stops loop", func(t *testing.T) {
		ctrl, err := NewController(ControllerConfig{
			Orchestrator:    newMockOrchestrator(),
			Threshold:       HealthThresholds{HeartbeatInterval: 100 * time.Millisecond, SuspectThreshold: 3, OfflineThreshold: 5},
			FailoverTimeout: 5 * time.Minute,
		})
		if err != nil {
			t.Fatalf("NewController: %v", err)
		}

		ctx, cancel := context.WithCancel(context.Background())
		if err := ctrl.Start(ctx); err != nil {
			t.Fatalf("Start failed: %v", err)
		}

		if !ctrl.IsRunning() {
			t.Error("expected controller to be running")
		}

		cancel()
		// The mainLoop should exit, but Stop should still be called to wait
		time.Sleep(50 * time.Millisecond)
		if err := ctrl.Stop(); err != nil {
			t.Errorf("Stop after context cancel: %v", err)
		}
	})
}

// --- Test: RegisterNode / UnregisterNode ---

func TestRegisterUnregisterNode(t *testing.T) {
	processor := NewHeartbeatProcessor(DefaultThresholds(), nil)

	tests := []struct {
		name       string
		processor  *HeartbeatProcessor
		nodeID     string
		op         string // "register" or "unregister"
		wantExists bool   // after operation, does the node exist?
	}{
		{
			name:       "register node with processor",
			processor:  processor,
			nodeID:     "node-1",
			op:         "register",
			wantExists: true,
		},
		{
			name:       "unregister node with processor",
			processor:  processor,
			nodeID:     "node-1",
			op:         "unregister",
			wantExists: false,
		},
		{
			name:       "register without processor is safe",
			processor:  nil,
			nodeID:     "node-2",
			op:         "register",
			wantExists: false,
		},
		{
			name:       "unregister without processor is safe",
			processor:  nil,
			nodeID:     "node-2",
			op:         "unregister",
			wantExists: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl, err := NewController(ControllerConfig{
				Orchestrator:       newMockOrchestrator(),
				Threshold:          DefaultThresholds(),
				FailoverTimeout:    5 * time.Minute,
				HeartbeatProcessor: tt.processor,
			})
			if err != nil {
				t.Fatalf("NewController: %v", err)
			}

			switch tt.op {
			case "register":
				ctrl.RegisterNode(tt.nodeID)
			case "unregister":
				ctrl.UnregisterNode(tt.nodeID)
			}

			if tt.processor != nil {
				_, exists := tt.processor.GetNodeHealth(tt.nodeID)
				if exists != tt.wantExists {
					t.Errorf("after %s: expected node exists=%v, got %v", tt.op, tt.wantExists, exists)
				}
			}
		})
	}
}

// --- Test: ProcessHeartbeat ---

func TestProcessHeartbeat(t *testing.T) {
	tests := []struct {
		name        string
		processor   *HeartbeatProcessor
		hb          *Heartbeat
		expectErr   bool
		errContains string
	}{
		{
			name:      "nil processor returns error",
			processor: nil,
			hb: &Heartbeat{
				NodeID:    "node-1",
				Timestamp: time.Now(),
				Sequence:  1,
			},
			expectErr:   true,
			errContains: "heartbeat processor not configured",
		},
		{
			name:      "valid heartbeat succeeds",
			processor: NewHeartbeatProcessor(DefaultThresholds(), nil),
			hb: &Heartbeat{
				NodeID:    "node-1",
				Timestamp: time.Now(),
				Sequence:  1,
			},
			expectErr: false,
		},
		{
			name:      "invalid heartbeat (missing node_id) returns error",
			processor: NewHeartbeatProcessor(DefaultThresholds(), nil),
			hb: &Heartbeat{
				Timestamp: time.Now(),
				Sequence:  1,
			},
			expectErr:   true,
			errContains: "node_id is required",
		},
		{
			name:      "invalid heartbeat (zero timestamp) returns error",
			processor: NewHeartbeatProcessor(DefaultThresholds(), nil),
			hb: &Heartbeat{
				NodeID:   "node-1",
				Sequence: 1,
			},
			expectErr:   true,
			errContains: "timestamp is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl, err := NewController(ControllerConfig{
				Orchestrator:       newMockOrchestrator(),
				Threshold:          DefaultThresholds(),
				FailoverTimeout:    5 * time.Minute,
				HeartbeatProcessor: tt.processor,
			})
			if err != nil {
				t.Fatalf("NewController: %v", err)
			}

			err = ctrl.ProcessHeartbeat(context.Background(), tt.hb)
			if tt.expectErr {
				if err == nil {
					t.Fatalf("expected error containing %q, got nil", tt.errContains)
				}
				if tt.errContains != "" && !contains(err.Error(), tt.errContains) {
					t.Errorf("expected error containing %q, got %q", tt.errContains, err.Error())
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

// --- Test: GetNodeHealth ---

func TestGetNodeHealth(t *testing.T) {
	processor := NewHeartbeatProcessor(DefaultThresholds(), nil)
	processor.RegisterNode("known-node")
	hb := &Heartbeat{
		NodeID:    "known-node",
		Timestamp: time.Now(),
		Sequence:  1,
	}
	if err := processor.ProcessHeartbeat(context.Background(), hb); err != nil {
		t.Fatalf("ProcessHeartbeat: %v", err)
	}

	tests := []struct {
		name      string
		processor *HeartbeatProcessor
		nodeID    string
		wantOK    bool
		wantState HealthState
	}{
		{
			name:      "nil processor returns false",
			processor: nil,
			nodeID:    "any-node",
			wantOK:    false,
		},
		{
			name:      "known node returns health",
			processor: processor,
			nodeID:    "known-node",
			wantOK:    true,
			wantState: StateOnline,
		},
		{
			name:      "unknown node returns false",
			processor: processor,
			nodeID:    "unknown-node",
			wantOK:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl, err := NewController(ControllerConfig{
				Orchestrator:       newMockOrchestrator(),
				Threshold:          DefaultThresholds(),
				FailoverTimeout:    5 * time.Minute,
				HeartbeatProcessor: tt.processor,
			})
			if err != nil {
				t.Fatalf("NewController: %v", err)
			}

			health, ok := ctrl.GetNodeHealth(tt.nodeID)
			if ok != tt.wantOK {
				t.Errorf("GetNodeHealth(%s) ok=%v, want %v", tt.nodeID, ok, tt.wantOK)
			}
			if ok && health.GetState() != tt.wantState {
				t.Errorf("GetNodeHealth(%s) state=%v, want %v", tt.nodeID, health.GetState(), tt.wantState)
			}
		})
	}
}

// --- Test: GetAllHealth ---

func TestGetAllHealth(t *testing.T) {
	tests := []struct {
		name      string
		processor *HeartbeatProcessor
		nodeCount int
	}{
		{
			name:      "nil processor returns nil",
			processor: nil,
			nodeCount: 0,
		},
		{
			name:      "empty processor returns empty map",
			processor: NewHeartbeatProcessor(DefaultThresholds(), nil),
			nodeCount: 0,
		},
		{
			name: "multiple nodes returns all",
			processor: func() *HeartbeatProcessor {
				p := NewHeartbeatProcessor(DefaultThresholds(), nil)
				for i := 0; i < 3; i++ {
					p.RegisterNode("node-" + string(rune('a'+i)))
				}
				return p
			}(),
			nodeCount: 3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl, err := NewController(ControllerConfig{
				Orchestrator:       newMockOrchestrator(),
				Threshold:          DefaultThresholds(),
				FailoverTimeout:    5 * time.Minute,
				HeartbeatProcessor: tt.processor,
			})
			if err != nil {
				t.Fatalf("NewController: %v", err)
			}

			allHealth := ctrl.GetAllHealth()
			if tt.processor == nil {
				if allHealth != nil {
					t.Errorf("expected nil for nil processor, got %v", allHealth)
				}
				return
			}
			if len(allHealth) != tt.nodeCount {
				t.Errorf("expected %d nodes, got %d", tt.nodeCount, len(allHealth))
			}
		})
	}
}

// --- Test: GetStatus ---

func TestGetStatus(t *testing.T) {
	tests := []struct {
		name            string
		processor       *HeartbeatProcessor
		running         bool
		expectNodes     bool
		expectFailovers bool
	}{
		{
			name:    "not running, nil processor",
			running: false,
		},
		{
			name: "running with nodes",
			processor: func() *HeartbeatProcessor {
				p := NewHeartbeatProcessor(DefaultThresholds(), nil)
				p.RegisterNode("node-1")
				return p
			}(),
			running:     true,
			expectNodes: true,
		},
		{
			name:            "running with orchestrator",
			running:         true,
			expectFailovers: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			orch := newMockOrchestrator()
			ctrl, err := NewController(ControllerConfig{
				Orchestrator:       orch,
				Threshold:          DefaultThresholds(),
				FailoverTimeout:    5 * time.Minute,
				HeartbeatProcessor: tt.processor,
			})
			if err != nil {
				t.Fatalf("NewController: %v", err)
			}

			if tt.running {
				ctx := context.Background()
				if err := ctrl.Start(ctx); err != nil {
					t.Fatalf("Start: %v", err)
				}
				defer ctrl.Stop()
			}

			status := ctrl.GetStatus()

			if status["running"] != tt.running {
				t.Errorf("expected running=%v, got %v", tt.running, status["running"])
			}

			if tt.expectNodes {
				if _, ok := status["online_nodes"]; !ok {
					t.Error("expected online_nodes in status")
				}
				if _, ok := status["suspect_nodes"]; !ok {
					t.Error("expected suspect_nodes in status")
				}
				if _, ok := status["offline_nodes"]; !ok {
					t.Error("expected offline_nodes in status")
				}
			}

			if tt.expectFailovers {
				if _, ok := status["active_failovers"]; !ok {
					t.Error("expected active_failovers in status")
				}
			}

			if _, ok := status["thresholds"]; !ok {
				t.Error("expected thresholds in status")
			}
		})
	}
}

// --- Test: SetThresholds ---

func TestSetThresholds(t *testing.T) {
	tests := []struct {
		name        string
		thresholds  HealthThresholds
		expectErr   bool
		errContains string
	}{
		{
			name:       "valid thresholds succeed",
			thresholds: HealthThresholds{HeartbeatInterval: 10 * time.Second, SuspectThreshold: 2, OfflineThreshold: 4},
			expectErr:  false,
		},
		{
			name:        "invalid thresholds rejected",
			thresholds:  HealthThresholds{HeartbeatInterval: 0, SuspectThreshold: 3, OfflineThreshold: 5},
			expectErr:   true,
			errContains: "invalid thresholds",
		},
		{
			name:        "offline <= suspect rejected",
			thresholds:  HealthThresholds{HeartbeatInterval: 30 * time.Second, SuspectThreshold: 5, OfflineThreshold: 3},
			expectErr:   true,
			errContains: "invalid thresholds",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl, err := NewController(ControllerConfig{
				Orchestrator:    newMockOrchestrator(),
				Threshold:       DefaultThresholds(),
				FailoverTimeout: 5 * time.Minute,
			})
			if err != nil {
				t.Fatalf("NewController: %v", err)
			}

			err = ctrl.SetThresholds(tt.thresholds)
			if tt.expectErr {
				if err == nil {
					t.Fatalf("expected error containing %q, got nil", tt.errContains)
				}
				if tt.errContains != "" && !contains(err.Error(), tt.errContains) {
					t.Errorf("expected error containing %q, got %q", tt.errContains, err.Error())
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			// Verify the threshold was actually updated
			got := ctrl.GetThresholds()
			if got.HeartbeatInterval != tt.thresholds.HeartbeatInterval {
				t.Errorf("threshold not updated: expected interval %v, got %v", tt.thresholds.HeartbeatInterval, got.HeartbeatInterval)
			}
		})
	}
}

// --- Test: GetThresholds ---

func TestGetThresholds(t *testing.T) {
	customThresholds := HealthThresholds{
		HeartbeatInterval: 15 * time.Second,
		SuspectThreshold:  2,
		OfflineThreshold:  6,
	}

	ctrl, err := NewController(ControllerConfig{
		Orchestrator:    newMockOrchestrator(),
		Threshold:       customThresholds,
		FailoverTimeout: 5 * time.Minute,
	})
	if err != nil {
		t.Fatalf("NewController: %v", err)
	}

	got := ctrl.GetThresholds()
	if got != customThresholds {
		t.Errorf("GetThresholds: expected %+v, got %+v", customThresholds, got)
	}
}

// --- Test: TriggerFailover ---

func TestTriggerFailover(t *testing.T) {
	t.Run("manual failover calls orchestrator", func(t *testing.T) {
		orch := newMockOrchestrator()
		ctrl, err := NewController(ControllerConfig{
			Orchestrator:    orch,
			Threshold:       DefaultThresholds(),
			FailoverTimeout: 5 * time.Minute,
		})
		if err != nil {
			t.Fatalf("NewController: %v", err)
		}

		ctx := context.Background()
		if err := ctrl.TriggerFailover(ctx, "failed-node"); err != nil {
			t.Fatalf("TriggerFailover: %v", err)
		}

		// The failover is async, so wait for it
		time.Sleep(100 * time.Millisecond)

		orch.mu.Lock()
		called := orch.failoverCalled
		nodeID := orch.failoverNodeID
		orch.mu.Unlock()

		if !called {
			t.Error("expected orchestrator.HandleHostFailure to be called")
		}
		if nodeID != "failed-node" {
			t.Errorf("expected nodeID 'failed-node', got %q", nodeID)
		}
	})

	t.Run("trigger failover on context cancel", func(t *testing.T) {
		orch := newMockOrchestrator()
		orch.failoverDelay = 5 * time.Second // Will block until context cancelled

		ctrl, err := NewController(ControllerConfig{
			Orchestrator:    orch,
			Threshold:       DefaultThresholds(),
			FailoverTimeout: 100 * time.Millisecond,
		})
		if err != nil {
			t.Fatalf("NewController: %v", err)
		}

		ctx := context.Background()
		if err := ctrl.TriggerFailover(ctx, "slow-node"); err != nil {
			t.Fatalf("TriggerFailover: %v", err)
		}

		// Let the failover timeout fire
		time.Sleep(200 * time.Millisecond)

		// The failover should have been cancelled by the timeout context
		// We can't directly check the error from the goroutine, but we can verify
		// it didn't hang forever
	})
}

// --- Test: IsRunning ---

func TestIsRunning(t *testing.T) {
	ctrl, err := NewController(ControllerConfig{
		Orchestrator:    newMockOrchestrator(),
		Threshold:       DefaultThresholds(),
		FailoverTimeout: 5 * time.Minute,
	})
	if err != nil {
		t.Fatalf("NewController: %v", err)
	}

	if ctrl.IsRunning() {
		t.Error("expected controller to not be running initially")
	}

	ctx := context.Background()
	if err := ctrl.Start(ctx); err != nil {
		t.Fatalf("Start: %v", err)
	}

	if !ctrl.IsRunning() {
		t.Error("expected controller to be running after Start")
	}

	if err := ctrl.Stop(); err != nil {
		t.Fatalf("Stop: %v", err)
	}

	if ctrl.IsRunning() {
		t.Error("expected controller to not be running after Stop")
	}
}

// --- Test: mainLoop reconciliation triggers failover on offline transition ---

func TestMainLoopTriggersFailover(t *testing.T) {
	processor := NewHeartbeatProcessor(HealthThresholds{
		HeartbeatInterval: 50 * time.Millisecond,
		SuspectThreshold:  1,
		OfflineThreshold:  2,
	}, nil)
	processor.RegisterNode("failing-node")

	// Send initial heartbeat to set the node online
	hb := &Heartbeat{
		NodeID:    "failing-node",
		Timestamp: time.Now(),
		Sequence:  1,
	}
	if err := processor.ProcessHeartbeat(context.Background(), hb); err != nil {
		t.Fatalf("ProcessHeartbeat: %v", err)
	}

	orch := newMockOrchestrator()
	ctrl, err := NewController(ControllerConfig{
		Orchestrator:       orch,
		Threshold:          HealthThresholds{HeartbeatInterval: 50 * time.Millisecond, SuspectThreshold: 1, OfflineThreshold: 2},
		FailoverTimeout:    5 * time.Minute,
		HeartbeatProcessor: processor,
	})
	if err != nil {
		t.Fatalf("NewController: %v", err)
	}

	ctx := context.Background()
	if err := ctrl.Start(ctx); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer ctrl.Stop()

	// Wait for the ticker to fire enough times to transition the node to offline
	// (2 missed beats at 50ms interval = ~100ms+, give it 500ms)
	time.Sleep(500 * time.Millisecond)

	orch.mu.Lock()
	called := orch.failoverCalled
	orch.mu.Unlock()

	if !called {
		t.Error("expected failover to be triggered for offline node")
	}
}

// --- Test: handleTransition fires events ---

func TestHandleTransitionEvents(t *testing.T) {
	var mu sync.Mutex
	var events []string

	eventCallback := func(eventType, severity, message string) {
		mu.Lock()
		events = append(events, eventType)
		mu.Unlock()
	}

	orch := newMockOrchestrator()
	ctrl, err := NewController(ControllerConfig{
		Orchestrator:    orch,
		Threshold:       DefaultThresholds(),
		FailoverTimeout: 5 * time.Minute,
		EventCallback:   eventCallback,
	})
	if err != nil {
		t.Fatalf("NewController: %v", err)
	}

	tests := []struct {
		name        string
		transition  StateTransition
		expectEvent string
	}{
		{
			name: "suspect transition fires warning event",
			transition: StateTransition{
				NodeID:   "node-1",
				OldState: StateOnline,
				NewState: StateSuspect,
			},
			expectEvent: "ha_node_suspect",
		},
		{
			name: "offline transition fires critical event",
			transition: StateTransition{
				NodeID:   "node-2",
				OldState: StateSuspect,
				NewState: StateOffline,
			},
			expectEvent: "ha_node_offline",
		},
		{
			name: "online transition fires no event",
			transition: StateTransition{
				NodeID:   "node-3",
				OldState: StateSuspect,
				NewState: StateOnline,
			},
			expectEvent: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mu.Lock()
			events = nil
			mu.Unlock()

			ctrl.handleTransition(context.Background(), tt.transition)

			mu.Lock()
			defer mu.Unlock()

			if tt.expectEvent == "" {
				if len(events) != 0 {
					t.Errorf("expected no events, got %v", events)
				}
				return
			}

			found := false
			for _, e := range events {
				if e == tt.expectEvent {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("expected event %q, got events: %v", tt.expectEvent, events)
			}
		})
	}
}

// --- Test: metrics callback fires during reconciliation ---

func TestReconcileMetricsCallback(t *testing.T) {
	var mu sync.Mutex
	var metrics []string

	metricsCallback := func(metric string, value float64, labels map[string]string) {
		mu.Lock()
		metrics = append(metrics, metric)
		mu.Unlock()
	}

	processor := NewHeartbeatProcessor(DefaultThresholds(), nil)
	processor.RegisterNode("node-1")
	processor.RegisterNode("node-2")

	orch := newMockOrchestrator()
	ctrl, err := NewController(ControllerConfig{
		Orchestrator:       orch,
		Threshold:          DefaultThresholds(),
		FailoverTimeout:    5 * time.Minute,
		HeartbeatProcessor: processor,
		MetricsCallback:    metricsCallback,
	})
	if err != nil {
		t.Fatalf("NewController: %v", err)
	}

	ctrl.reconcile(context.Background())

	mu.Lock()
	defer mu.Unlock()

	expectedMetrics := map[string]bool{
		"ha_nodes_online":  false,
		"ha_nodes_suspect": false,
		"ha_nodes_offline": false,
	}

	for _, m := range metrics {
		if _, ok := expectedMetrics[m]; ok {
			expectedMetrics[m] = true
		}
	}

	for metric, found := range expectedMetrics {
		if !found {
			t.Errorf("expected metric %q to be emitted", metric)
		}
	}
}

// --- Test: metrics callback is nil-safe ---

func TestReconcileNilMetricsCallback(t *testing.T) {
	processor := NewHeartbeatProcessor(DefaultThresholds(), nil)
	processor.RegisterNode("node-1")

	orch := newMockOrchestrator()
	ctrl, err := NewController(ControllerConfig{
		Orchestrator:       orch,
		Threshold:          DefaultThresholds(),
		FailoverTimeout:    5 * time.Minute,
		HeartbeatProcessor: processor,
		MetricsCallback:    nil, // explicitly nil
	})
	if err != nil {
		t.Fatalf("NewController: %v", err)
	}

	// Should not panic
	ctrl.reconcile(context.Background())
}

// --- Test: concurrent Start/Stop safety ---

func TestConcurrentStartStop(t *testing.T) {
	ctrl, err := NewController(ControllerConfig{
		Orchestrator:    newMockOrchestrator(),
		Threshold:       HealthThresholds{HeartbeatInterval: 50 * time.Millisecond, SuspectThreshold: 3, OfflineThreshold: 5},
		FailoverTimeout: 5 * time.Minute,
	})
	if err != nil {
		t.Fatalf("NewController: %v", err)
	}

	ctx := context.Background()
	var wg sync.WaitGroup

	// Start multiple goroutines that try to start/stop the controller
	for i := 0; i < 5; i++ {
		wg.Add(2)
		go func() {
			defer wg.Done()
			ctrl.Start(ctx)
		}()
		go func() {
			defer wg.Done()
			time.Sleep(10 * time.Millisecond)
			ctrl.Stop()
		}()
	}

	wg.Wait()

	// Final stop to ensure cleanup
	ctrl.Stop()
}

// --- Test: event callback is nil-safe in handleTransition ---

func TestHandleTransitionNilEventCallback(t *testing.T) {
	orch := newMockOrchestrator()
	ctrl, err := NewController(ControllerConfig{
		Orchestrator:    orch,
		Threshold:       DefaultThresholds(),
		FailoverTimeout: 5 * time.Minute,
		EventCallback:   nil,
	})
	if err != nil {
		t.Fatalf("NewController: %v", err)
	}

	// Should not panic
	ctrl.handleTransition(context.Background(), StateTransition{
		NodeID:   "node-1",
		OldState: StateOnline,
		NewState: StateSuspect,
	})
}

// Helper function

func contains(s, substr string) bool {
	return len(s) >= len(substr) && searchString(s, substr)
}

func searchString(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
