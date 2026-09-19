package ha

import (
	"testing"
	"time"
)

// TestPolicyValidation verifies that HAPolicy.Validate() works for all modes.
func TestPolicyValidation(t *testing.T) {
	tests := []struct {
		name    string
		policy  HAPolicy
		wantErr bool
	}{
		{
			name: "valid auto mode",
			policy: HAPolicy{
				VMID:         "vm-1",
				Mode:         HAModeAuto,
				Priority:     100,
				MaxRestarts:  3,
				RestartWindow: 5 * time.Minute,
			},
			wantErr: false,
		},
		{
			name: "valid manual mode",
			policy: HAPolicy{
				VMID:         "vm-2",
				Mode:         HAModeManual,
				Priority:     50,
				MaxRestarts:  0,
				RestartWindow: 5 * time.Minute,
			},
			wantErr: false,
		},
		{
			name: "valid never mode",
			policy: HAPolicy{
				VMID:         "vm-3",
				Mode:         HAModeNever,
				Priority:     0,
				MaxRestarts:  1,
				RestartWindow: time.Hour,
			},
			wantErr: false,
		},
		{
			name: "valid max-one mode",
			policy: HAPolicy{
				VMID:         "vm-4",
				Mode:         HAModeMaxOne,
				Priority:     200,
				MaxRestarts:  5,
				RestartWindow: 10 * time.Minute,
			},
			wantErr: false,
		},
		{
			name: "invalid mode",
			policy: HAPolicy{
				VMID:         "vm-5",
				Mode:         HAMode("invalid"),
				Priority:     100,
				MaxRestarts:  3,
				RestartWindow: 5 * time.Minute,
			},
			wantErr: true,
		},
		{
			name: "negative priority",
			policy: HAPolicy{
				VMID:         "vm-6",
				Mode:         HAModeAuto,
				Priority:     -1,
				MaxRestarts:  3,
				RestartWindow: 5 * time.Minute,
			},
			wantErr: true,
		},
		{
			name: "negative max_restarts",
			policy: HAPolicy{
				VMID:         "vm-7",
				Mode:         HAModeAuto,
				Priority:     100,
				MaxRestarts:  -1,
				RestartWindow: 5 * time.Minute,
			},
			wantErr: true,
		},
		{
			name: "zero restart_window defaults",
			policy: HAPolicy{
				VMID:         "vm-8",
				Mode:         HAModeAuto,
				Priority:     100,
				MaxRestarts:  3,
				RestartWindow: 0,
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.policy.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// TestShouldRestart verifies the restart decision logic.
func TestShouldRestart(t *testing.T) {
	tests := []struct {
		mode     HAMode
		expected bool
	}{
		{HAModeAuto, true},
		{HAModeMaxOne, true},
		{HAModeManual, false},
		{HAModeNever, false},
	}

	for _, tt := range tests {
		t.Run(string(tt.mode), func(t *testing.T) {
			p := HAPolicy{Mode: tt.mode}
			if got := p.ShouldRestart(); got != tt.expected {
				t.Errorf("ShouldRestart() for %s = %v, want %v", tt.mode, got, tt.expected)
			}
		})
	}
}

// TestGetAutoRestartVMs verifies priority ordering of auto-restart VMs.
func TestGetAutoRestartVMs(t *testing.T) {
	store := NewPolicyManager(nil)

	// Set up policies with different priorities
	policies := []HAPolicy{
		{VMID: "vm-low", Mode: HAModeAuto, Priority: 200},
		{VMID: "vm-high", Mode: HAModeAuto, Priority: 50},
		{VMID: "vm-mid", Mode: HAModeAuto, Priority: 100},
		{VMID: "vm-never", Mode: HAModeNever, Priority: 0},
		{VMID: "vm-manual", Mode: HAModeManual, Priority: 0},
	}

	for i := range policies {
		if err := store.SetPolicy(policies[i].VMID, policies[i], "test"); err != nil {
			t.Fatalf("SetPolicy failed: %v", err)
		}
	}

	allVMs := []string{"vm-low", "vm-high", "vm-mid", "vm-never", "vm-manual"}
	autoVMs := store.GetAutoRestartVMs(allVMs)

	// Should only include auto and max-one modes
	if len(autoVMs) != 3 {
		t.Fatalf("Expected 3 auto-restart VMs, got %d", len(autoVMs))
	}

	// Should be ordered by priority: vm-high(50), vm-mid(100), vm-low(200)
	expected := []string{"vm-high", "vm-mid", "vm-low"}
	for i, vm := range expected {
		if autoVMs[i] != vm {
			t.Errorf("Position %d: expected %s, got %s", i, vm, autoVMs[i])
		}
	}
}

// TestCanRestart verifies the rate limiting mechanism.
func TestCanRestart(t *testing.T) {
	store := NewPolicyManager(nil)

	// Set policy with max 3 restarts in 5 minute window
	policy := HAPolicy{
		VMID:          "vm-rate",
		Mode:          HAModeAuto,
		Priority:      100,
		MaxRestarts:   3,
		RestartWindow: 5 * time.Minute,
	}
	if err := store.SetPolicy("vm-rate", policy, "test"); err != nil {
		t.Fatalf("SetPolicy failed: %v", err)
	}

	// First 3 restarts should be allowed
	for i := 0; i < 3; i++ {
		if !store.CanRestart("vm-rate") {
			t.Errorf("Restart %d should be allowed", i+1)
		}
		store.RecordRestart("vm-rate")
	}

	// 4th restart should be blocked
	if store.CanRestart("vm-rate") {
		t.Error("4th restart should be blocked (rate limit exceeded)")
	}

	// After window expires, should be allowed again
	// (simulated by checking with a fresh policy with larger window)
	store2 := NewPolicyManager(nil)
	policy2 := HAPolicy{
		VMID:          "vm-rate2",
		Mode:          HAModeAuto,
		Priority:      100,
		MaxRestarts:   2,
		RestartWindow: 1 * time.Second,
	}
	store2.SetPolicy("vm-rate2", policy2, "test")

	// Exhaust restarts
	store2.RecordRestart("vm-rate2")
	store2.RecordRestart("vm-rate2")
	if store2.CanRestart("vm-rate2") {
		t.Error("Should be rate limited after 2 restarts")
	}

	// After window passes, should be allowed
	time.Sleep(2 * time.Second)
	if !store2.CanRestart("vm-rate2") {
		t.Error("Should be allowed after restart window expires")
	}
}

// TestDefaultHAPolicy verifies the default policy values.
func TestDefaultHAPolicy(t *testing.T) {
	p := DefaultHAPolicy("vm-default")

	if p.Mode != HAModeAuto {
		t.Errorf("Default mode should be auto, got %s", p.Mode)
	}
	if p.Priority != 100 {
		t.Errorf("Default priority should be 100, got %d", p.Priority)
	}
	if p.MaxRestarts != 3 {
		t.Errorf("Default max_restarts should be 3, got %d", p.MaxRestarts)
	}
	if p.RestartWindow != 5*time.Minute {
		t.Errorf("Default restart_window should be 5m, got %v", p.RestartWindow)
	}
}
