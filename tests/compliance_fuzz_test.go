// Package tests provides integration and unit tests for HiveStack.
package tests

import (
	"encoding/json"
	"math/rand"
	"strings"
	"testing"

	"github.com/maddydevel/HiveStack/internal/compliance"
)

// randomString generates a random string of length n from the given alphabet.
func randomString(n int, alphabet string) string {
	if n <= 0 {
		return ""
	}
	var sb strings.Builder
	for i := 0; i < n; i++ {
		sb.WriteByte(alphabet[rand.Intn(len(alphabet))])
	}
	return sb.String()
}

// randomRole returns a random role string, sometimes "hana" (case variations).
func randomRole() string {
	roles := []string{"hana", "HANA", "Hana", "generic", "worker", "storage", "network", "edge", "compute", "app"}
	return roles[rand.Intn(len(roles))]
}

// randomNUMAPolicy returns a random NUMA policy string.
func randomNUMAPolicy() *string {
	policies := []string{"centered", "bind", "interleave", "preferred", "", "strict", "numa"}
	p := policies[rand.Intn(len(policies))]
	return &p
}

// generateRandomVMProfile creates a randomized VMProfile.
func generateRandomVMProfile() *compliance.VMProfile {
	profile := &compliance.VMProfile{
		Role:                   randomRole(),
		CPUs:                   rand.Intn(256), // 0-255
		CPUAllocation:          randomString(10, "abcdefghijklmnopqrstuvwxyz"),
		MemoryBytes:            int64(rand.Int63n(1 << 40)), // up to 1TiB
		NUMAPolicy:             randomNUMAPolicy(),
		HugepagesEnabled:       rand.Intn(2) == 1,
		CPUPinning:             json.RawMessage(`{}`),
		MemoryReservationBytes: int64(rand.Int63n(1 << 40)),
		BallooningAllowed:      rand.Intn(2) == 1,
		SwapAllowed:            rand.Intn(2) == 1,
		OS:                     randomString(20, "abcdefghijklmnopqrstuvwxyz -"),
		HasHugepagesConfig:     rand.Intn(2) == 1,
		NUMANodeCount:          rand.Intn(16),
		HugepagesTotalKB:       int64(rand.Int63n(1 << 30)),
		HostCPUCount:           rand.Intn(512),
		HostMemoryBytes:        int64(rand.Int63n(1 << 42)),
	}
	// Sometimes make CPUPinning empty
	if rand.Intn(3) == 0 {
		profile.CPUPinning = []byte{}
	}
	// Sometimes make NUMAPolicy nil
	if rand.Intn(3) == 0 {
		profile.NUMAPolicy = nil
	}
	return profile
}

// TestFuzzValidateHANAProfile_RandomProfiles fuzz tests with random profiles.
// Ensures ValidateHANAProfile never panics on arbitrary input.
func TestFuzzValidateHANAProfile_RandomProfiles(t *testing.T) {
	iterations := 1000
	for i := 0; i < iterations; i++ {
		profile := generateRandomVMProfile()
		// Must not panic
		result := compliance.ValidateHANAProfile(profile)

		// Invariant: non-HANA roles always pass
		if !strings.EqualFold(profile.Role, "hana") {
			if !result.Passed {
				t.Errorf("non-HANA role %q should pass, got %d violations", profile.Role, len(result.Violations))
			}
			if result.Evidence["skip_reason"] != "not a HANA VM" {
				t.Errorf("expected skip_reason for non-HANA role %q", profile.Role)
			}
		}

		// Invariant: if role is hana (case-insensitive), check violations are consistent
		if strings.EqualFold(profile.Role, "hana") {
			// If result passed, there should be no violations
			if result.Passed && len(result.Violations) > 0 {
				t.Errorf("passed=true but has %d violations", len(result.Violations))
			}
			// If result failed, there must be at least one violation
			if !result.Passed && len(result.Violations) == 0 {
				t.Errorf("passed=false but no violations reported")
			}
			// All violations must have non-empty rule and severity
			for _, v := range result.Violations {
				if v.Rule == "" {
					t.Errorf("violation has empty Rule: %+v", v)
				}
				if v.Severity != "error" && v.Severity != "warning" {
					t.Errorf("unexpected severity %q for rule %q", v.Severity, v.Rule)
				}
			}
		}

		// Evidence must always contain these keys
		for _, key := range []string{"role", "cpus", "memory_bytes", "hugepages_enabled", "ballooning_allowed", "swap_allowed"} {
			if _, ok := result.Evidence[key]; !ok {
				t.Errorf("evidence missing key %q", key)
			}
		}
	}
}

// TestFuzzValidateHANAProfile_EdgeCases tests specific edge cases.
func TestFuzzValidateHANAProfile_EdgeCases(t *testing.T) {
	tests := []struct {
		name    string
		profile *compliance.VMProfile
	}{
		{
			name: "zero CPUs",
			profile: &compliance.VMProfile{
				Role:                   "hana",
				CPUs:                   0,
				CPUAllocation:          "dedicated",
				MemoryBytes:            68719476736,
				NUMAPolicy:             strPtr("centered"),
				HugepagesEnabled:       true,
				CPUPinning:             []byte(`{}`),
				MemoryReservationBytes: 68719476736,
				BallooningAllowed:      false,
				SwapAllowed:            false,
			},
		},
		{
			name: "zero memory",
			profile: &compliance.VMProfile{
				Role:                   "hana",
				CPUs:                   4,
				CPUAllocation:          "dedicated",
				MemoryBytes:            0,
				NUMAPolicy:             strPtr("centered"),
				HugepagesEnabled:       true,
				CPUPinning:             []byte(`{}`),
				MemoryReservationBytes: 0,
				BallooningAllowed:      false,
				SwapAllowed:            false,
			},
		},
		{
			name: "nil NUMAPolicy",
			profile: &compliance.VMProfile{
				Role:                   "hana",
				CPUs:                   4,
				CPUAllocation:          "dedicated",
				MemoryBytes:            68719476736,
				NUMAPolicy:             nil,
				HugepagesEnabled:       true,
				CPUPinning:             []byte(`{}`),
				MemoryReservationBytes: 68719476736,
				BallooningAllowed:      false,
				SwapAllowed:            false,
			},
		},
		{
			name: "empty NUMAPolicy string",
			profile: &compliance.VMProfile{
				Role:                   "hana",
				CPUs:                   4,
				CPUAllocation:          "dedicated",
				MemoryBytes:            68719476736,
				NUMAPolicy:             strPtr(""),
				HugepagesEnabled:       true,
				CPUPinning:             []byte(`{}`),
				MemoryReservationBytes: 68719476736,
				BallooningAllowed:      false,
				SwapAllowed:            false,
			},
		},
		{
			name: "max boundary CPUs (256)",
			profile: &compliance.VMProfile{
				Role:                   "hana",
				CPUs:                   256,
				CPUAllocation:          "dedicated",
				MemoryBytes:            68719476736,
				NUMAPolicy:             strPtr("bind"),
				HugepagesEnabled:       true,
				CPUPinning:             []byte(`{}`),
				MemoryReservationBytes: 68719476736,
				BallooningAllowed:      false,
				SwapAllowed:            false,
			},
		},
		{
			name: "negative memory",
			profile: &compliance.VMProfile{
				Role:                   "hana",
				CPUs:                   4,
				CPUAllocation:          "dedicated",
				MemoryBytes:            -1,
				NUMAPolicy:             strPtr("centered"),
				HugepagesEnabled:       true,
				CPUPinning:             []byte(`{}`),
				MemoryReservationBytes: -1,
				BallooningAllowed:      false,
				SwapAllowed:            false,
			},
		},
		{
			name: "all violations",
			profile: &compliance.VMProfile{
				Role:                   "hana",
				CPUs:                   0,
				CPUAllocation:          "shared",
				MemoryBytes:            0,
				NUMAPolicy:             nil,
				HugepagesEnabled:       false,
				CPUPinning:             []byte{},
				MemoryReservationBytes: 1,
				BallooningAllowed:      true,
				SwapAllowed:            true,
			},
		},
		{
			name: "CPU allocation mismatch with reservation",
			profile: &compliance.VMProfile{
				Role:                   "hana",
				CPUs:                   8,
				CPUAllocation:          "dedicated",
				MemoryBytes:            68719476736,
				NUMAPolicy:             strPtr("centered"),
				HugepagesEnabled:       true,
				CPUPinning:             []byte(`{"vcpu0":"0"}`),
				MemoryReservationBytes: 34359738368, // mismatch
				BallooningAllowed:      false,
				SwapAllowed:            false,
			},
		},
		{
			name: "role HANA uppercase",
			profile: &compliance.VMProfile{
				Role:                   "HANA",
				CPUs:                   4,
				CPUAllocation:          "dedicated",
				MemoryBytes:            68719476736,
				NUMAPolicy:             strPtr("centered"),
				HugepagesEnabled:       true,
				CPUPinning:             []byte(`{}`),
				MemoryReservationBytes: 68719476736,
				BallooningAllowed:      false,
				SwapAllowed:            false,
			},
		},
		{
			name: "role hana mixed case",
			profile: &compliance.VMProfile{
				Role:                   "Hana",
				CPUs:                   4,
				CPUAllocation:          "dedicated",
				MemoryBytes:            68719476736,
				NUMAPolicy:             strPtr("centered"),
				HugepagesEnabled:       true,
				CPUPinning:             []byte(`{}`),
				MemoryReservationBytes: 68719476736,
				BallooningAllowed:      false,
				SwapAllowed:            false,
			},
		},
		{
			name: "large memory boundary (1TiB)",
			profile: &compliance.VMProfile{
				Role:                   "hana",
				CPUs:                   64,
				CPUAllocation:          "dedicated",
				MemoryBytes:            1 << 40, // 1TiB
				NUMAPolicy:             strPtr("bind"),
				HugepagesEnabled:       true,
				CPUPinning:             []byte(`{"vcpu0":"0"}`),
				MemoryReservationBytes: 1 << 40,
				BallooningAllowed:      false,
				SwapAllowed:            false,
			},
		},
		{
			name: "zero CPUs with zero memory",
			profile: &compliance.VMProfile{
				Role:                   "hana",
				CPUs:                   0,
				CPUAllocation:          "dedicated",
				MemoryBytes:            0,
				NUMAPolicy:             strPtr("centered"),
				HugepagesEnabled:       true,
				CPUPinning:             []byte(`{}`),
				MemoryReservationBytes: 0,
				BallooningAllowed:      false,
				SwapAllowed:            false,
			},
		},
		{
			name: "empty CPU pinning (null bytes)",
			profile: &compliance.VMProfile{
				Role:                   "hana",
				CPUs:                   4,
				CPUAllocation:          "dedicated",
				MemoryBytes:            68719476736,
				NUMAPolicy:             strPtr("centered"),
				HugepagesEnabled:       true,
				CPUPinning:             nil,
				MemoryReservationBytes: 68719476736,
				BallooningAllowed:      false,
				SwapAllowed:            false,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Must not panic
			result := compliance.ValidateHANAProfile(tt.profile)

			// Basic invariants
			if result.CheckType != "hana_guardrails" {
				t.Errorf("CheckType = %q", result.CheckType)
			}

			// Non-HANA roles always pass
			if !strings.EqualFold(tt.profile.Role, "hana") {
				if !result.Passed {
					t.Errorf("non-HANA role should pass")
				}
				return
			}

			// HANA role: if passed, no violations; if failed, at least one
			if result.Passed && len(result.Violations) > 0 {
				t.Errorf("passed but has violations: %d", len(result.Violations))
			}
			if !result.Passed && len(result.Violations) == 0 {
				t.Errorf("failed but no violations")
			}
		})
	}
}

// TestFuzzValidateHANAProfile_RulesInvariant tests that specific rules are always triggered
// when conditions are met, across many random profiles.
func TestFuzzValidateHANAProfile_RulesInvariant(t *testing.T) {
	// Test: nil/empty NUMA policy always triggers numa_pinned violation
	for i := 0; i < 100; i++ {
		profile := &compliance.VMProfile{
			Role:                   "hana",
			CPUs:                   rand.Intn(128) + 1,
			CPUAllocation:          "dedicated",
			MemoryBytes:            int64(rand.Int63n(1<<40)) + 1,
			NUMAPolicy:             nil,
			HugepagesEnabled:       true,
			CPUPinning:             []byte(`{}`),
			MemoryReservationBytes: 0, // will set below
			BallooningAllowed:      false,
			SwapAllowed:            false,
		}
		profile.MemoryReservationBytes = profile.MemoryBytes

		result := compliance.ValidateHANAProfile(profile)
		found := false
		for _, v := range result.Violations {
			if v.Rule == "numa_pinned" {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("iteration %d: nil NUMAPolicy should trigger numa_pinned, got %d violations", i, len(result.Violations))
		}
	}

	// Test: invalid NUMA policy value triggers numa_policy_value
	invalidPolicies := []string{"interleave", "preferred", "strict", "foobar", "123"}
	for _, policy := range invalidPolicies {
		profile := &compliance.VMProfile{
			Role:                   "hana",
			CPUs:                   4,
			CPUAllocation:          "dedicated",
			MemoryBytes:            68719476736,
			NUMAPolicy:             &policy,
			HugepagesEnabled:       true,
			CPUPinning:             []byte(`{}`),
			MemoryReservationBytes: 68719476736,
			BallooningAllowed:      false,
			SwapAllowed:            false,
		}
		result := compliance.ValidateHANAProfile(profile)
		found := false
		for _, v := range result.Violations {
			if v.Rule == "numa_policy_value" {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("policy %q should trigger numa_policy_value violation", policy)
		}
	}

	// Test: hugepages disabled always triggers
	for i := 0; i < 50; i++ {
		profile := &compliance.VMProfile{
			Role:                   "hana",
			CPUs:                   rand.Intn(128) + 1,
			CPUAllocation:          "dedicated",
			MemoryBytes:            int64(rand.Int63n(1<<40)) + 1,
			NUMAPolicy:             strPtr("centered"),
			HugepagesEnabled:       false,
			CPUPinning:             []byte(`{}`),
			MemoryReservationBytes: 0,
			BallooningAllowed:      false,
			SwapAllowed:            false,
		}
		profile.MemoryReservationBytes = profile.MemoryBytes

		result := compliance.ValidateHANAProfile(profile)
		found := false
		for _, v := range result.Violations {
			if v.Rule == "hugepages_enabled" {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("iteration %d: hugepages disabled should trigger violation", i)
		}
	}

	// Test: ballooning enabled always triggers
	for i := 0; i < 50; i++ {
		profile := &compliance.VMProfile{
			Role:                   "hana",
			CPUs:                   rand.Intn(128) + 1,
			CPUAllocation:          "dedicated",
			MemoryBytes:            int64(rand.Int63n(1<<40)) + 1,
			NUMAPolicy:             strPtr("centered"),
			HugepagesEnabled:       true,
			CPUPinning:             []byte(`{}`),
			MemoryReservationBytes: 0,
			BallooningAllowed:      true,
			SwapAllowed:            false,
		}
		profile.MemoryReservationBytes = profile.MemoryBytes

		result := compliance.ValidateHANAProfile(profile)
		found := false
		for _, v := range result.Violations {
			if v.Rule == "no_ballooning" {
				found = true
				if v.Correctable {
					t.Error("ballooning violation should not be correctable")
				}
				break
			}
		}
		if !found {
			t.Errorf("iteration %d: ballooning enabled should trigger violation", i)
		}
	}

	// Test: swap enabled always triggers
	for i := 0; i < 50; i++ {
		profile := &compliance.VMProfile{
			Role:                   "hana",
			CPUs:                   rand.Intn(128) + 1,
			CPUAllocation:          "dedicated",
			MemoryBytes:            int64(rand.Int63n(1<<40)) + 1,
			NUMAPolicy:             strPtr("centered"),
			HugepagesEnabled:       true,
			CPUPinning:             []byte(`{}`),
			MemoryReservationBytes: 0,
			BallooningAllowed:      false,
			SwapAllowed:            true,
		}
		profile.MemoryReservationBytes = profile.MemoryBytes

		result := compliance.ValidateHANAProfile(profile)
		found := false
		for _, v := range result.Violations {
			if v.Rule == "no_swap" {
				found = true
				if v.Correctable {
					t.Error("swap violation should not be correctable")
				}
				break
			}
		}
		if !found {
			t.Errorf("iteration %d: swap enabled should trigger violation", i)
		}
	}

	// Test: empty CPU pinning always triggers
	for i := 0; i < 50; i++ {
		profile := &compliance.VMProfile{
			Role:                   "hana",
			CPUs:                   rand.Intn(128) + 1,
			CPUAllocation:          "dedicated",
			MemoryBytes:            int64(rand.Int63n(1<<40)) + 1,
			NUMAPolicy:             strPtr("centered"),
			HugepagesEnabled:       true,
			CPUPinning:             []byte{},
			MemoryReservationBytes: 0,
			BallooningAllowed:      false,
			SwapAllowed:            false,
		}
		profile.MemoryReservationBytes = profile.MemoryBytes

		result := compliance.ValidateHANAProfile(profile)
		found := false
		for _, v := range result.Violations {
			if v.Rule == "cpu_pinning" {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("iteration %d: empty CPU pinning should trigger violation", i)
		}
	}

	// Test: memory reservation mismatch always triggers
	for i := 0; i < 50; i++ {
		memSize := int64(rand.Int63n(1<<40)) + 1
		profile := &compliance.VMProfile{
			Role:                   "hana",
			CPUs:                   rand.Intn(128) + 1,
			CPUAllocation:          "dedicated",
			MemoryBytes:            memSize,
			NUMAPolicy:             strPtr("centered"),
			HugepagesEnabled:       true,
			CPUPinning:             []byte(`{}`),
			MemoryReservationBytes: memSize + 1, // mismatch
			BallooningAllowed:      false,
			SwapAllowed:            false,
		}

		result := compliance.ValidateHANAProfile(profile)
		found := false
		for _, v := range result.Violations {
			if v.Rule == "memory_reservation" {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("iteration %d: memory reservation mismatch should trigger violation", i)
		}
	}

	// Test: non-dedicated CPU allocation always triggers
	nonDedicated := []string{"shared", "overcommit", "burst", "elastic", ""}
	for _, alloc := range nonDedicated {
		profile := &compliance.VMProfile{
			Role:                   "hana",
			CPUs:                   4,
			CPUAllocation:          alloc,
			MemoryBytes:            68719476736,
			NUMAPolicy:             strPtr("centered"),
			HugepagesEnabled:       true,
			CPUPinning:             []byte(`{}`),
			MemoryReservationBytes: 68719476736,
			BallooningAllowed:      false,
			SwapAllowed:            false,
		}

		result := compliance.ValidateHANAProfile(profile)
		found := false
		for _, v := range result.Violations {
			if v.Rule == "dedicated_cpu" {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("cpu allocation %q should trigger dedicated_cpu violation", alloc)
		}
	}
}

// TestFuzzValidateHANAProfile_CompliantRandom generates random compliant profiles
// and verifies they pass validation.
func TestFuzzValidateHANAProfile_CompliantRandom(t *testing.T) {
	for i := 0; i < 200; i++ {
		memBytes := int64(rand.Int63n(1<<40)) + 1
		cpus := rand.Intn(128) + 1
		numaPolicy := "centered"
		if rand.Intn(2) == 0 {
			numaPolicy = "bind"
		}

		profile := &compliance.VMProfile{
			Role:                   "hana",
			CPUs:                   cpus,
			CPUAllocation:          "dedicated",
			MemoryBytes:            memBytes,
			NUMAPolicy:             &numaPolicy,
			HugepagesEnabled:       true,
			CPUPinning:             json.RawMessage(`{"vcpu0":"0"}`),
			MemoryReservationBytes: memBytes,
			BallooningAllowed:      false,
			SwapAllowed:            false,
		}

		result := compliance.ValidateHANAProfile(profile)
		if !result.Passed {
			t.Errorf("iteration %d: compliant profile should pass, got %d violations: %+v",
				i, len(result.Violations), result.Violations)
		}
	}
}
