package ha

import (
	"testing"
)

// TestSelectTarget verifies that the scheduler picks the correct host
// based on resource availability and spread policy.
func TestSelectTarget(t *testing.T) {
	scheduler := NewDefaultScheduler()
	policy := DefaultPolicy()

	vm := VM{
		ID:          "vm-test",
		Name:        "test-vm",
		HostID:      "host-a",
		CPUs:        4,
		MemoryBytes: 8 * 1024 * 1024 * 1024, // 8GB
	}

	hosts := []Host{
		{
			ID:               "host-a",
			Name:             "host-a",
			Status:           "online",
			CPUCount:         16,
			MemoryTotalBytes: 32 * 1024 * 1024 * 1024,
			MemoryUsedBytes:  28 * 1024 * 1024 * 1024, // Almost full
		},
		{
			ID:               "host-b",
			Name:             "host-b",
			Status:           "online",
			CPUCount:         16,
			MemoryTotalBytes: 32 * 1024 * 1024 * 1024,
			MemoryUsedBytes:  8 * 1024 * 1024 * 1024, // Plenty of space
		},
		{
			ID:               "host-c",
			Name:             "host-c",
			Status:           "offline",
			CPUCount:         32,
			MemoryTotalBytes: 64 * 1024 * 1024 * 1024,
			MemoryUsedBytes:  4 * 1024 * 1024 * 1024,
		},
	}

	target, err := scheduler.SelectTarget(vm, hosts, policy)
	if err != nil {
		t.Fatalf("SelectTarget failed: %v", err)
	}

	if target.ID != "host-b" {
		t.Errorf("Expected scheduler to pick host-b (more resources), got %s", target.ID)
	}
}

// TestAntiAffinity verifies that VMs avoid hosts running anti-affinity VMs.
func TestAntiAffinity(t *testing.T) {
	scheduler := NewDefaultScheduler()
	policy := DefaultPolicy()
	policy.AntiAffinityEnabled = true

	vm := VM{
		ID:           "vm-1",
		Name:         "vm-1",
		HostID:       "host-a",
		CPUs:         2,
		MemoryBytes:  4 * 1024 * 1024 * 1024,
		AntiAffinity: []string{"vm-2"},
	}

	hosts := []Host{
		{
			ID:               "host-a",
			Name:             "host-a",
			Status:           "online",
			CPUCount:         16,
			MemoryTotalBytes: 32 * 1024 * 1024 * 1024,
			MemoryUsedBytes:  16 * 1024 * 1024 * 1024,
		},
		{
			ID:               "host-b",
			Name:             "host-b",
			Status:           "online",
			CPUCount:         16,
			MemoryTotalBytes: 32 * 1024 * 1024 * 1024,
			MemoryUsedBytes:  16 * 1024 * 1024 * 1024,
			CurrentVMs:       []string{"vm-2"}, // Hosts anti-affinity VM
		},
		{
			ID:               "host-c",
			Name:             "host-c",
			Status:           "online",
			CPUCount:         16,
			MemoryTotalBytes: 32 * 1024 * 1024 * 1024,
			MemoryUsedBytes:  16 * 1024 * 1024 * 1024,
		},
	}

	target, err := scheduler.SelectTarget(vm, hosts, policy)
	if err != nil {
		t.Fatalf("SelectTarget failed: %v", err)
	}

	// Should not pick host-b since it runs vm-2 (anti-affinity)
	if target.ID == "host-b" {
		t.Error("Scheduler should avoid host-b due to anti-affinity rule")
	}

	// Verify it picks host-a or host-c (host-a has more memory headroom)
	if target.ID != "host-a" && target.ID != "host-c" {
		t.Errorf("Unexpected target host: %s", target.ID)
	}
}

// TestCanFit verifies resource filtering works.
func TestCanFit(t *testing.T) {
	tests := []struct {
		name     string
		host     Host
		vmCPU    int
		vmMem    int64
		expected bool
	}{
		{
			name: "host has plenty of resources",
			host: Host{
				ID:               "h1",
				Status:           "online",
				CPUCount:         16,
				MemoryTotalBytes: 32 * 1024 * 1024 * 1024,
				MemoryUsedBytes:  8 * 1024 * 1024 * 1024,
			},
			vmCPU:    4,
			vmMem:    8 * 1024 * 1024 * 1024,
			expected: true,
		},
		{
			name: "host in maintenance mode",
			host: Host{
				ID:               "h2",
				Status:           "online",
				CPUCount:         16,
				MemoryTotalBytes: 32 * 1024 * 1024 * 1024,
				MemoryUsedBytes:  8 * 1024 * 1024 * 1024,
				MaintenanceMode:  true,
			},
			vmCPU:    4,
			vmMem:    8 * 1024 * 1024 * 1024,
			expected: false,
		},
		{
			name: "host offline",
			host: Host{
				ID:               "h3",
				Status:           "offline",
				CPUCount:         16,
				MemoryTotalBytes: 32 * 1024 * 1024 * 1024,
				MemoryUsedBytes:  8 * 1024 * 1024 * 1024,
			},
			vmCPU:    4,
			vmMem:    8 * 1024 * 1024 * 1024,
			expected: false,
		},
		{
			name: "not enough memory",
			host: Host{
				ID:               "h4",
				Status:           "online",
				CPUCount:         16,
				MemoryTotalBytes: 16 * 1024 * 1024 * 1024,
				MemoryUsedBytes:  14 * 1024 * 1024 * 1024, // Only 2GB free
			},
			vmCPU:    4,
			vmMem:    8 * 1024 * 1024 * 1024,
			expected: false,
		},
		{
			name: "too many CPUs requested",
			host: Host{
				ID:               "h5",
				Status:           "online",
				CPUCount:         4,
				MemoryTotalBytes: 32 * 1024 * 1024 * 1024,
				MemoryUsedBytes:  8 * 1024 * 1024 * 1024,
			},
			vmCPU:    16,
			vmMem:    8 * 1024 * 1024 * 1024,
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.host.CanFit(tt.vmCPU, tt.vmMem)
			if result != tt.expected {
				t.Errorf("CanFit() = %v, expected %v", result, tt.expected)
			}
		})
	}
}

// TestScoreHost verifies the scoring logic for host selection.
func TestScoreHost(t *testing.T) {
	scheduler := NewDefaultScheduler()
	policy := DefaultPolicy() // spread with LoadBalanceWeight 0.7

	vm := VM{
		ID:          "vm-score",
		Name:        "test-vm",
		CPUs:        4,
		MemoryBytes: 8 * 1024 * 1024 * 1024,
	}

	hosts := []Host{
		{
			ID:               "host-empty",
			Name:             "host-empty",
			Status:           "online",
			CPUCount:         16,
			MemoryTotalBytes: 32 * 1024 * 1024 * 1024,
			MemoryUsedBytes:  4 * 1024 * 1024 * 1024,
			VMCount:          0,
		},
		{
			ID:               "host-loaded",
			Name:             "host-loaded",
			Status:           "online",
			CPUCount:         16,
			MemoryTotalBytes: 32 * 1024 * 1024 * 1024,
			MemoryUsedBytes:  24 * 1024 * 1024 * 1024,
			VMCount:          10,
		},
	}

	scoreEmpty := scheduler.ScoreHost(vm, hosts[0], policy)
	scoreLoaded := scheduler.ScoreHost(vm, hosts[1], policy)

	// Host with more available memory should score higher (spread policy)
	if scoreEmpty <= scoreLoaded {
		t.Errorf("Empty host (score=%f) should score higher than loaded host (score=%f)", scoreEmpty, scoreLoaded)
	}

	// Verify that ineligible host returns -1
	badHost := Host{
		ID:               "bad",
		Status:           "offline",
		CPUCount:         2,
		MemoryTotalBytes: 4 * 1024 * 1024 * 1024,
		MemoryUsedBytes:  2 * 1024 * 1024 * 1024,
	}
	scoreBad := scheduler.ScoreHost(vm, badHost, policy)
	if scoreBad != -1 {
		t.Errorf("Ineligible host should score -1, got %f", scoreBad)
	}

	// Test NUMA affinity bonus
	vmWithNUMA := VM{
		ID:          "vm-numa",
		Name:        "numa-vm",
		CPUs:        4,
		MemoryBytes: 8 * 1024 * 1024 * 1024,
	}
	numaPolicy := "strict"
	vmWithNUMA.NUMAPolicy = &numaPolicy

	hostNoNUMA := Host{
		ID:               "host-no-numa",
		Name:             "host-no-numa",
		Status:           "online",
		CPUCount:         16,
		MemoryTotalBytes: 32 * 1024 * 1024 * 1024,
		MemoryUsedBytes:  4 * 1024 * 1024 * 1024,
		NUMANodeCount:    0,
	}

	hostWithNUMA := Host{
		ID:               "host-with-numa",
		Name:             "host-with-numa",
		Status:           "online",
		CPUCount:         16,
		MemoryTotalBytes: 32 * 1024 * 1024 * 1024,
		MemoryUsedBytes:  4 * 1024 * 1024 * 1024,
		NUMANodeCount:    2,
	}

	scoreNoNUMA := scheduler.ScoreHost(vmWithNUMA, hostNoNUMA, policy)
	scoreWithNUMA := scheduler.ScoreHost(vmWithNUMA, hostWithNUMA, policy)

	if scoreWithNUMA <= scoreNoNUMA {
		t.Errorf("NUMA host (score=%f) should score higher than non-NUMA host (score=%f) for strict NUMA policy", scoreWithNUMA, scoreNoNUMA)
	}
}
