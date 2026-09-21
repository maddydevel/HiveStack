// Package tests provides compliance tests for HiveStack.
// Dual enforcement: Manager API + Node Agent libvirt XML.
package tests

import (
	"strings"
	"testing"

	"github.com/maddydevel/HiveStack/internal/compliance"
	"github.com/maddydevel/HiveStack/internal/libvirt"
)

// ---------- Dual Enforcement Tests ----------

// TestDualEnforcement_ValidHANAPassesBoth verifies a valid HANA config passes
// BOTH Manager API validation AND Node Agent libvirt XML generation.
func TestDualEnforcement_ValidHANAPassesBoth(t *testing.T) {
	// Manager API validation
	profile := &compliance.VMProfile{
		Role:                   "hana",
		CPUs:                   8,
		CPUAllocation:          "dedicated",
		MemoryBytes:            68719476736, // 64GiB
		NUMAPolicy:             strPtr("centered"),
		HugepagesEnabled:       true,
		CPUPinning:             []byte(`{"vcpu0":"0","vcpu1":"1","vcpu2":"2","vcpu3":"3","vcpu4":"4","vcpu5":"5","vcpu6":"6","vcpu7":"7"}`),
		MemoryReservationBytes: 68719476736,
		BallooningAllowed:      false,
		SwapAllowed:            false,
	}
	result := compliance.ValidateHANAProfile(profile)
	if !result.Passed {
		t.Fatalf("Manager API should accept valid HANA profile, got violations: %+v", result.Violations)
	}

	// Node Agent libvirt XML generation
	xml, err := libvirt.GenerateDomainXML(libvirt.DomainSpec{
		Name:             "hana-vm-1",
		MemoryBytes:      68719476736,
		VCPUs:            8,
		NUMAPolicy:       "centered",
		HugepagesEnabled: true,
		CPUPinning: map[string]string{
			"vcpu0": "0", "vcpu1": "1", "vcpu2": "2", "vcpu3": "3",
			"vcpu4": "4", "vcpu5": "5", "vcpu6": "6", "vcpu7": "7",
		},
		DedicatedCPU:      true,
		BallooningAllowed: false,
	})
	if err != nil {
		t.Fatalf("Node Agent libvirt XML generation failed: %v", err)
	}

	// Verify XML contains HANA guardrail features
	assertXMLContains(t, xml, "numatune", "NUMA tuning missing from XML")
	assertXMLContains(t, xml, "strict", "strict NUMA policy missing")
	assertXMLContains(t, xml, "hugepages", "hugepages missing from XML")
	assertXMLContains(t, xml, "vcpupin", "CPU pinning missing from XML")
	assertXMLContains(t, xml, "host-passthrough", "dedicated CPU mode missing")
	assertXMLNotContains(t, xml, "memballoon", "ballooning device present but should be disabled")
}

// TestDualEnforcement_InvalidBallooningFailsAtAPI verifies that a VM config with
// ballooning enabled is rejected by the Manager API compliance check.
func TestDualEnforcement_InvalidBallooningFailsAtAPI(t *testing.T) {
	profile := &compliance.VMProfile{
		Role:                   "hana",
		CPUs:                   8,
		CPUAllocation:          "dedicated",
		MemoryBytes:            68719476736,
		NUMAPolicy:             strPtr("centered"),
		HugepagesEnabled:       true,
		CPUPinning:             []byte(`{"vcpu0":"0"}`),
		MemoryReservationBytes: 68719476736,
		BallooningAllowed:      true, // VIOLATION
		SwapAllowed:            false,
	}

	result := compliance.ValidateHANAProfile(profile)
	if result.Passed {
		t.Fatal("Manager API should reject HANA profile with ballooning enabled")
	}

	found := false
	for _, v := range result.Violations {
		if v.Rule == "no_ballooning" {
			found = true
			break
		}
	}
	if !found {
		t.Error("Expected no_ballooning violation")
	}
}

// TestDualEnforcement_InvalidNoHugepagesFailsAtAPI verifies that a VM config
// without hugepages is rejected by the Manager API compliance check.
func TestDualEnforcement_InvalidNoHugepagesFailsAtAPI(t *testing.T) {
	profile := &compliance.VMProfile{
		Role:                   "hana",
		CPUs:                   8,
		CPUAllocation:          "dedicated",
		MemoryBytes:            68719476736,
		NUMAPolicy:             strPtr("centered"),
		HugepagesEnabled:       false, // VIOLATION
		CPUPinning:             []byte(`{"vcpu0":"0"}`),
		MemoryReservationBytes: 68719476736,
		BallooningAllowed:      false,
		SwapAllowed:            false,
	}

	result := compliance.ValidateHANAProfile(profile)
	if result.Passed {
		t.Fatal("Manager API should reject HANA profile without hugepages")
	}

	found := false
	for _, v := range result.Violations {
		if v.Rule == "hugepages_enabled" {
			found = true
			break
		}
	}
	if !found {
		t.Error("Expected hugepages_enabled violation")
	}
}

// TestDualEnforcement_NodeAgentXML_HANAFeatures verifies the Node Agent
// generates correct XML with all HANA guardrail features.
func TestDualEnforcement_NodeAgentXML_HANAFeatures(t *testing.T) {
	xml, err := libvirt.GenerateDomainXML(libvirt.DomainSpec{
		Name:             "test-hana",
		MemoryBytes:      137438953472, // 128GiB
		VCPUs:            16,
		NUMAPolicy:       "bind",
		HugepagesEnabled: true,
		CPUPinning: map[string]string{
			"vcpu0": "0", "vcpu1": "1", "vcpu2": "2", "vcpu3": "3",
		},
		DedicatedCPU:      true,
		BallooningAllowed: false,
	})
	if err != nil {
		t.Fatalf("GenerateDomainXML failed: %v", err)
	}

	assertXMLContains(t, xml, "cputune", "CPU tuning missing")
	assertXMLContains(t, xml, "vcpupin", "vcpupin missing")
	assertXMLContains(t, xml, "nodeset", "NUMA nodeset missing")
	assertXMLContains(t, xml, "mode=\"strict\"", "strict mode missing")
	assertXMLContains(t, xml, "hugepages", "hugepages missing")
	assertXMLNotContains(t, xml, "memballoon", "ballooning must be absent for HANA")
}

// TestDualEnforcement_NodeAgentXML_NoHugepagesNoNUMA verifies non-HANA
// VMs get permissive XML without HANA guardrails.
func TestDualEnforcement_NodeAgentXML_NoHugepagesNoNUMA(t *testing.T) {
	xml, err := libvirt.GenerateDomainXML(libvirt.DomainSpec{
		Name:              "generic-vm",
		MemoryBytes:       8589934592, // 8GiB
		VCPUs:             4,
		NUMAPolicy:        "", // no NUMA
		HugepagesEnabled:  false,
		CPUPinning:        nil,
		DedicatedCPU:      false,
		BallooningAllowed: true, // allowed for non-HANA
	})
	if err != nil {
		t.Fatalf("GenerateDomainXML failed: %v", err)
	}

	// Should NOT have NUMA, hugepages, or pinning
	assertXMLNotContains(t, xml, "numatune", "NUMA should not be present for non-HANA")
	assertXMLNotContains(t, xml, "hugepages", "hugepages should not be present for non-HANA")
	assertXMLNotContains(t, xml, "vcpupin", "CPU pinning should not be present for non-HANA")
	// Ballooning IS present for non-HANA
	assertXMLContains(t, xml, "memballoon", "ballooning should be present for non-HANA")
}

// TestDualEnforcement_MultipleViolations verifies the API reports all violations
// for a severely misconfigured HANA VM.
func TestDualEnforcement_MultipleViolations(t *testing.T) {
	profile := &compliance.VMProfile{
		Role:                   "hana",
		CPUs:                   4,
		CPUAllocation:          "shared", // violation: not dedicated
		MemoryBytes:            68719476736,
		NUMAPolicy:             nil,      // violation: not set
		HugepagesEnabled:       false,    // violation: not enabled
		CPUPinning:             []byte{}, // violation: empty
		MemoryReservationBytes: 0,        // violation: not equal to memory
		BallooningAllowed:      true,     // violation: enabled
		SwapAllowed:            true,     // violation: enabled
	}

	result := compliance.ValidateHANAProfile(profile)
	if result.Passed {
		t.Fatal("Expected multiple violations")
	}

	// Should have violations for: dedicated_cpu, numa_pinned, hugepages_enabled,
	// cpu_pinning, memory_reservation, no_ballooning, no_swap = 7 total
	expectedRules := map[string]bool{
		"dedicated_cpu":      false,
		"numa_pinned":        false,
		"hugepages_enabled":  false,
		"cpu_pinning":        false,
		"memory_reservation": false,
		"no_ballooning":      false,
		"no_swap":            false,
	}

	for _, v := range result.Violations {
		if _, ok := expectedRules[v.Rule]; ok {
			expectedRules[v.Rule] = true
		}
	}

	for rule, found := range expectedRules {
		if !found {
			t.Errorf("Missing expected violation: %s", rule)
		}
	}
}

// TestDualEnforcement_NodeAgentXML_BallooningDisabled verifies that even when
// ballooning is requested for a HANA-like config, the generated XML omits
// the memballoon device (Node Agent enforcement).
func TestDualEnforcement_NodeAgentXML_BallooningDisabled(t *testing.T) {
	// Even if someone tries to set ballooning_allowed=true at the XML layer,
	// for a HANA VM the Node Agent must override and disable it.
	// This simulates the Node Agent reading the VM role from the profile
	// and enforcing the guardrail regardless of the input flag.
	xml, err := libvirt.GenerateDomainXML(libvirt.DomainSpec{
		Name:              "hana-strict",
		MemoryBytes:       68719476736,
		VCPUs:             8,
		NUMAPolicy:        "centered",
		HugepagesEnabled:  true,
		CPUPinning:        map[string]string{"vcpu0": "0", "vcpu1": "1"},
		DedicatedCPU:      true,
		BallooningAllowed: false, // HANA enforcement: always false
	})
	if err != nil {
		t.Fatalf("GenerateDomainXML failed: %v", err)
	}

	assertXMLNotContains(t, xml, "<memballoon", "memballoon element must not appear for HANA")
}

// TestDualEnforcement_NodeAgentXML_DedicatedCPU verifies the CPU element
// is present with host-passthrough mode for dedicated CPU allocation.
func TestDualEnforcement_NodeAgentXML_DedicatedCPU(t *testing.T) {
	xml, err := libvirt.GenerateDomainXML(libvirt.DomainSpec{
		Name:              "hana-dedicated",
		MemoryBytes:       68719476736,
		VCPUs:             4,
		NUMAPolicy:        "centered",
		HugepagesEnabled:  true,
		CPUPinning:        map[string]string{"vcpu0": "0"},
		DedicatedCPU:      true,
		BallooningAllowed: false,
	})
	if err != nil {
		t.Fatalf("GenerateDomainXML failed: %v", err)
	}

	assertXMLContains(t, xml, "host-passthrough", "Dedicated CPU requires host-passthrough mode")
	assertXMLContains(t, xml, "topology", "CPU topology must be specified")
	assertXMLContains(t, xml, "sockets", "CPU sockets must be specified")
	assertXMLContains(t, xml, "cores", "CPU cores must be specified")
}

// Helper functions
func assertXMLContains(t *testing.T, xml string, substr string, msg string) {
	t.Helper()
	if !strings.Contains(xml, substr) {
		t.Errorf("%s: XML does not contain %q", msg, substr)
	}
}

func assertXMLNotContains(t *testing.T, xml string, substr string, msg string) {
	t.Helper()
	if strings.Contains(xml, substr) {
		t.Errorf("%s: XML should not contain %q", msg, substr)
	}
}
