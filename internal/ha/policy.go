// Package ha provides Non-HANA High Availability for HiveStack.
//
// HA policies define per-VM behavior during host failures.
package ha

import (
	"fmt"
	"sync"
	"time"
)

// HAMode defines the failover behavior for a VM.
type HAMode string

const (
	// HAModeAuto automatically restarts the VM on a healthy host when its current host fails.
	HAModeAuto HAMode = "auto"
	// HAModeManual requires operator intervention to restart the VM after host failure.
	HAModeManual HAMode = "manual"
	// HAModeNever never automatically restarts the VM (best-effort, no guarantees).
	HAModeNever HAMode = "never"
	// HAModeMaxOne ensures at most one instance runs (prevents split-brain even without fencing).
	HAModeMaxOne HAMode = "max-one"
)

// String implements fmt.Stringer.
func (m HAMode) String() string {
	return string(m)
}

// HAPolicy defines the HA behavior for a single VM.
type HAPolicy struct {
	VMID         string   `json:"vm_id"`
	Mode         HAMode   `json:"mode"`          // auto, manual, never, max-one
	Priority     int      `json:"priority"`      // 0=highest, higher=lower priority
	AntiAffinity []string `json:"anti_affinity"` // VM IDs that must not share a host
	MaxRestarts  int      `json:"max_restarts"`  // Max restart attempts within window
	RestartWindow time.Duration `json:"restart_window"` // Time window for restart counting
	LastModified time.Time `json:"last_modified"`
}

// DefaultHAPolicy returns the default HA policy (auto-restart, priority 100).
func DefaultHAPolicy(vmID string) HAPolicy {
	return HAPolicy{
		VMID:          vmID,
		Mode:          HAModeAuto,
		Priority:      100,
		AntiAffinity:  []string{},
		MaxRestarts:   3,
		RestartWindow: 5 * time.Minute,
		LastModified:  time.Now(),
	}
}

// Validate checks the HA policy for correctness.
func (p *HAPolicy) Validate() error {
	switch p.Mode {
	case HAModeAuto, HAModeManual, HAModeNever, HAModeMaxOne:
		// valid
	default:
		return fmt.Errorf("invalid HA mode: %s", p.Mode)
	}
	if p.Priority < 0 {
		return fmt.Errorf("priority must be non-negative")
	}
	if p.MaxRestarts < 0 {
		return fmt.Errorf("max_restarts must be non-negative")
	}
	if p.RestartWindow <= 0 {
		p.RestartWindow = 5 * time.Minute
	}
	return nil
}

// ShouldRestart returns true if the VM should be restarted based on policy.
func (p *HAPolicy) ShouldRestart() bool {
	switch p.Mode {
	case HAModeAuto, HAModeMaxOne:
		return true
	default:
		return false
	}
}

// PolicyManager manages HA policies and restart tracking.
type PolicyManager struct {
	mu       sync.RWMutex
	policies map[string]HAPolicy
	// restartCounts tracks restart timestamps per VM for rate limiting
	restartCounts map[string][]time.Time
	// historyStore optionally persists policy changes
	historyStore PolicyHistoryStore
}

// PolicyHistoryStore persists policy change history.
type PolicyHistoryStore interface {
	RecordPolicyChange(vmID string, oldMode, newMode HAMode, actor string) error
}

// NewPolicyManager creates a new HA policy manager.
func NewPolicyManager(store PolicyHistoryStore) *PolicyManager {
	return &PolicyManager{
		policies:      make(map[string]HAPolicy),
		restartCounts: make(map[string][]time.Time),
		historyStore:  store,
	}
}

// GetPolicy returns the HA policy for a VM.
func (m *PolicyManager) GetPolicy(vmID string) (HAPolicy, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	p, ok := m.policies[vmID]
	return p, ok
}

// SetPolicy sets the HA policy for a VM.
func (m *PolicyManager) SetPolicy(vmID string, policy HAPolicy, actor string) error {
	if err := policy.Validate(); err != nil {
		return fmt.Errorf("invalid HA policy: %w", err)
	}

	m.mu.Lock()
	oldPolicy, hadOld := m.policies[vmID]
	policy.VMID = vmID
	policy.LastModified = time.Now()
	m.policies[vmID] = policy
	m.mu.Unlock()

	if hadOld && m.historyStore != nil {
		m.historyStore.RecordPolicyChange(vmID, oldPolicy.Mode, policy.Mode, actor)
	}

	return nil
}

// DeletePolicy removes the HA policy for a VM.
func (m *PolicyManager) DeletePolicy(vmID string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.policies, vmID)
	delete(m.restartCounts, vmID)
}

// ListPolicies returns all HA policies.
func (m *PolicyManager) ListPolicies() map[string]HAPolicy {
	m.mu.RLock()
	defer m.mu.RUnlock()
	result := make(map[string]HAPolicy, len(m.policies))
	for k, v := range m.policies {
		result[k] = v
	}
	return result
}

// GetAutoRestartVMs returns VMs that should be auto-restarted, ordered by priority.
func (m *PolicyManager) GetAutoRestartVMs(vmIDs []string) []string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	type vmPriority struct {
		id       string
		priority int
	}

	var autoVMs []vmPriority
	for _, id := range vmIDs {
		if p, ok := m.policies[id]; ok && p.ShouldRestart() {
			autoVMs = append(autoVMs, vmPriority{id: id, priority: p.Priority})
		}
	}

	// Sort by priority (lower number = higher priority, restarted first)
	for i := 0; i < len(autoVMs)-1; i++ {
		for j := i + 1; j < len(autoVMs); j++ {
			if autoVMs[j].priority < autoVMs[i].priority {
				autoVMs[i], autoVMs[j] = autoVMs[j], autoVMs[i]
			}
		}
	}

	result := make([]string, len(autoVMs))
	for i, v := range autoVMs {
		result[i] = v.id
	}
	return result
}

// RecordRestart records a restart event for rate limiting.
func (m *PolicyManager) RecordRestart(vmID string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.restartCounts[vmID] = append(m.restartCounts[vmID], time.Now())
}

// CanRestart checks if the VM is allowed to restart (rate limiting).
func (m *PolicyManager) CanRestart(vmID string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()

	policy, ok := m.policies[vmID]
	if !ok {
		return true // No policy = default = allowed
	}

	if policy.MaxRestarts <= 0 {
		return true // No limit
	}

	// Clean old entries outside the window
	now := time.Now()
	cutoff := now.Add(-policy.RestartWindow)
	valid := make([]time.Time, 0, len(m.restartCounts[vmID]))
	for _, t := range m.restartCounts[vmID] {
		if t.After(cutoff) {
			valid = append(valid, t)
		}
	}
	m.restartCounts[vmID] = valid

	return len(valid) < policy.MaxRestarts
}

// GetAntiAffinityGroups returns anti-affinity groupings for a set of VMs.
func (m *PolicyManager) GetAntiAffinityGroups(vmIDs []string) map[string][]string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	groups := make(map[string][]string)
	for _, id := range vmIDs {
		if p, ok := m.policies[id]; ok {
			groups[id] = p.AntiAffinity
		}
	}
	return groups
}

// LoadDefaults sets default policies for a set of VMs.
func (m *PolicyManager) LoadDefaults(vmIDs []string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, id := range vmIDs {
		if _, exists := m.policies[id]; !exists {
			m.policies[id] = DefaultHAPolicy(id)
		}
	}
}
