// Package ha provides Non-HANA High Availability for HiveStack.
//
// The scheduler selects optimal target hosts for VM restart during failover,
// considering resource capacity, NUMA topology, and anti-affinity rules.
package ha

import (
	"fmt"
	"sort"
	"sync"
)

// NUMANode represents a single NUMA domain on a host.
type NUMANode struct {
	ID          int   `json:"id"`
	CPUCores    []int `json:"cpu_cores"`
	MemoryBytes int64 `json:"memory_bytes"`
	FreeMemory  int64 `json:"free_memory"`
	PinnedCPUs  []int `json:"pinned_cpus"`
}

// Host describes a target host for VM placement decisions.
type Host struct {
	ID               string            `json:"id"`
	Name             string            `json:"name"`
	Hostname         string            `json:"hostname"`
	Status           string            `json:"status"`
	CPUCount         int               `json:"cpu_count"`
	MemoryTotalBytes int64             `json:"memory_total_bytes"`
	MemoryUsedBytes  int64             `json:"memory_used_bytes"`
	NUMANodeCount    int               `json:"numa_node_count"`
	NUMANodes        []NUMANode        `json:"numa_nodes,omitempty"`
	VMCount          int               `json:"vm_count"`
	Labels           map[string]string `json:"labels,omitempty"`
	MaintenanceMode  bool              `json:"maintenance_mode"`
	CurrentVMs       []string          `json:"current_vms"`
}

// AvailableMemory returns remaining memory for new VMs.
func (h *Host) AvailableMemory() int64 {
	avail := h.MemoryTotalBytes - h.MemoryUsedBytes
	if avail < 0 {
		return 0
	}
	return avail
}

// CanFit checks if the host has enough resources for the given requirements.
func (h *Host) CanFit(cpu int, memory int64) bool {
	if h.MaintenanceMode {
		return false
	}
	if h.Status != "online" && h.Status != "active" {
		return false
	}
	if h.AvailableMemory() < memory {
		return false
	}
	// Simplified CPU check: assume ~2x overcommit is acceptable
	availCPU := h.CPUCount * 2
	if cpu > availCPU {
		return false
	}
	return true
}

// VM describes a VM to be placed on a target host.
type VM struct {
	ID               string            `json:"id"`
	Name             string            `json:"name"`
	HostID           string            `json:"host_id"`
	CPUs             int               `json:"cpus"`
	MemoryBytes      int64             `json:"memory_bytes"`
	NUMAPolicy       *string           `json:"numa_policy,omitempty"`
	HugepagesEnabled bool              `json:"hugepages_enabled"`
	NUMAAffinity     *int              `json:"numa_affinity,omitempty"` // Preferred NUMA node
	AntiAffinity     []string          `json:"anti_affinity"`           // VM IDs that must not co-locate
	PreferredHost    *string           `json:"preferred_host"`
	Labels           map[string]string `json:"labels,omitempty"`
	Role             string            `json:"role"`
}

// Policy describes the scheduling policy for host selection.
type Policy struct {
	PolicyType          string  `json:"policy_type"` // "binpack", "spread", "numa-aware"
	PreferSameNUMANode  bool    `json:"prefer_same_numa_node"`
	MaxOvercommitRatio  float64 `json:"max_overcommit_ratio"`
	LoadBalanceWeight   float64 `json:"load_balance_weight"`  // 0-1, higher = prefer less loaded hosts
	NUMAAffinityWeight  float64 `json:"numa_affinity_weight"` // 0-1, higher = prefer NUMA-aligned
	AntiAffinityEnabled bool    `json:"anti_affinity_enabled"`
}

// DefaultPolicy returns a sensible default scheduling policy.
func DefaultPolicy() Policy {
	return Policy{
		PolicyType:          "spread",
		PreferSameNUMANode:  true,
		MaxOvercommitRatio:  2.0,
		LoadBalanceWeight:   0.7,
		NUMAAffinityWeight:  0.5,
		AntiAffinityEnabled: true,
	}
}

// Scheduler defines the interface for host selection during failover.
type Scheduler interface {
	// SelectTarget chooses the best host for a VM restart given available hosts.
	SelectTarget(vm VM, hosts []Host, policy Policy) (*Host, error)
	// SelectTargets chooses the best hosts for multiple VM restarts.
	SelectTargets(vms []VM, hosts []Host, policy Policy) (map[string]*Host, error)
	// ScoreHost returns a fitness score for placing vm on host.
	ScoreHost(vm VM, host Host, policy Policy) float64
}

// DefaultScheduler implements the Scheduler interface.
type DefaultScheduler struct {
	mu sync.Mutex
}

// NewDefaultScheduler creates a new scheduler.
func NewDefaultScheduler() *DefaultScheduler {
	return &DefaultScheduler{}
}

// SelectTarget chooses the best host for a single VM restart.
func (s *DefaultScheduler) SelectTarget(vm VM, hosts []Host, policy Policy) (*Host, error) {
	if len(hosts) == 0 {
		return nil, fmt.Errorf("no hosts available for scheduling")
	}

	// Filter eligible hosts
	eligible := s.filterHosts(vm, hosts, policy)
	if len(eligible) == 0 {
		return nil, fmt.Errorf("no eligible hosts found for VM %s (cpus=%d, memory=%d)",
			vm.Name, vm.CPUs, vm.MemoryBytes)
	}

	// Score and rank
	type scored struct {
		host  Host
		score float64
	}
	scoredHosts := make([]scored, 0, len(eligible))
	for _, h := range eligible {
		score := s.ScoreHost(vm, h, policy)
		scoredHosts = append(scoredHosts, scored{host: h, score: score})
	}

	// Sort by score descending
	sort.Slice(scoredHosts, func(i, j int) bool {
		return scoredHosts[i].score > scoredHosts[j].score
	})

	best := scoredHosts[0]
	return &best.host, nil
}

// SelectTargets assigns all VMs to hosts respecting capacity constraints.
func (s *DefaultScheduler) SelectTargets(vms []VM, hosts []Host, policy Policy) (map[string]*Host, error) {
	assignments := make(map[string]*Host)

	// Track mutable host state
	hostStates := make([]Host, len(hosts))
	copy(hostStates, hosts)

	// Sort VMs by priority (memory size descending for packing efficiency)
	sortedVMs := make([]VM, len(vms))
	copy(sortedVMs, vms)
	sort.SliceStable(sortedVMs, func(i, j int) bool {
		return sortedVMs[i].MemoryBytes > sortedVMs[j].MemoryBytes
	})

	for _, vm := range sortedVMs {
		target, err := s.SelectTarget(vm, hostStates, policy)
		if err != nil {
			return nil, fmt.Errorf("schedule VM %s: %w", vm.Name, err)
		}

		assignments[vm.ID] = target

		// Update host state to reflect the placement
		for i := range hostStates {
			if hostStates[i].ID == target.ID {
				hostStates[i].MemoryUsedBytes += vm.MemoryBytes
				hostStates[i].CPUCount -= vm.CPUs
				if hostStates[i].CPUCount < 0 {
					hostStates[i].CPUCount = 0
				}
				hostStates[i].VMCount++
				hostStates[i].CurrentVMs = append(hostStates[i].CurrentVMs, vm.ID)
				break
			}
		}
	}

	return assignments, nil
}

// ScoreHost returns a fitness score (higher = better) for placing vm on host.
func (s *DefaultScheduler) ScoreHost(vm VM, host Host, policy Policy) float64 {
	if !host.CanFit(vm.CPUs, vm.MemoryBytes) {
		return -1 // ineligible
	}

	score := 0.0

	// Resource availability score (prefer hosts with more headroom after placement)
	remainingMem := host.AvailableMemory() - vm.MemoryBytes
	memRatio := float64(remainingMem) / float64(host.MemoryTotalBytes)
	score += memRatio * 30.0 * policy.LoadBalanceWeight

	// Prefer hosts with fewer VMs (spread policy)
	if policy.PolicyType == "spread" {
		vmScore := float64(100-host.VMCount) * 0.5
		if vmScore < 0 {
			vmScore = 0
		}
		score += vmScore
	}

	// Prefer hosts with more free CPUs (binpack policy)
	if policy.PolicyType == "binpack" {
		cpuScore := float64(host.CPUCount) * 0.3
		score += cpuScore
	}

	// NUMA affinity bonus
	if vm.NUMAPolicy != nil && *vm.NUMAPolicy == "strict" && host.NUMANodeCount > 0 {
		score += 25.0 * policy.NUMAAffinityWeight
	}

	// Preferred host bonus
	if vm.PreferredHost != nil && *vm.PreferredHost == host.ID {
		score += 15.0
	}

	// Anti-affinity penalty: penalize hosts running VMs in the anti-affinity list
	if policy.AntiAffinityEnabled {
		for _, antiVM := range vm.AntiAffinity {
			for _, runningVM := range host.CurrentVMs {
				if antiVM == runningVM {
					score -= 100.0 // heavy penalty
				}
			}
		}
	}

	// Label affinity bonus
	if vm.Labels != nil && host.Labels != nil {
		for k, v := range vm.Labels {
			if hv, ok := host.Labels[k]; ok && hv == v {
				score += 5.0
			}
		}
	}

	return score
}

// filterHosts removes hosts that cannot accommodate the VM.
func (s *DefaultScheduler) filterHosts(vm VM, hosts []Host, policy Policy) []Host {
	var eligible []Host
	for _, h := range hosts {
		if !h.CanFit(vm.CPUs, vm.MemoryBytes) {
			continue
		}

		// Anti-affinity: skip hosts running conflicting VMs
		if policy.AntiAffinityEnabled {
			skip := false
			for _, antiVM := range vm.AntiAffinity {
				for _, runningVM := range h.CurrentVMs {
					if antiVM == runningVM {
						skip = true
						break
					}
				}
				if skip {
					break
				}
			}
			if skip {
				continue
			}
		}

		eligible = append(eligible, h)
	}
	return eligible
}

// SchedulingDecision captures the result of a scheduling operation.
type SchedulingDecision struct {
	VMID     string  `json:"vm_id"`
	VMName   string  `json:"vm_name"`
	HostID   string  `json:"host_id"`
	HostName string  `json:"host_name"`
	Score    float64 `json:"score"`
}

// ScheduleResult contains all decisions from a scheduling run.
type ScheduleResult struct {
	Decisions []SchedulingDecision `json:"decisions"`
	Success   bool                 `json:"success"`
	Error     string               `json:"error,omitempty"`
}
