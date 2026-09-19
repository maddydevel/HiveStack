package compliance

import (
    "crypto/sha256"
    "encoding/hex"
    "encoding/json"
    "testing"
    "time"
)

func TestValidateHANAProfile_GenERIC_skips(t *testing.T) {
    profile := &VMProfile{
        Role:                   "generic",
        CPUs:                   4,
        CPUAllocation:          "shared",
        MemoryBytes:            8589934592,
        NUMAPolicy:             nil,
        HugepagesEnabled:       false,
        CPUPinning:             nil,
        MemoryReservationBytes: 4294967296,
        BallooningAllowed:      true,
        SwapAllowed:            true,
    }
    result := ValidateHANAProfile(profile)
    if !result.Passed {
        t.Fatal("generic VM should pass (skip reason)")
    }
    if result.Evidence["skip_reason"] != "not a HANA VM" {
        t.Errorf("skip_reason = %v", result.Evidence["skip_reason"])
    }
}

func TestValidateHANAProfile_BallooningFails(t *testing.T) {
    profile := &VMProfile{
        Role:                   "hana",
        CPUs:                   4,
        CPUAllocation:          "dedicated",
        MemoryBytes:            68719476736,
        NUMAPolicy:             func() *string { s := "centered"; return &s }(),
        HugepagesEnabled:       true,
        CPUPinning:             []byte(`{"vcpu0":"0","vcpu1":"1","vcpu2":"2","vcpu3":"3"}`),
        MemoryReservationBytes: 68719476736,
        BallooningAllowed:      true,  // violation
        SwapAllowed:            false,
    }
    result := ValidateHANAProfile(profile)
    if result.Passed {
        t.Fatal("expected failure due to ballooning")
    }
    found := false
    for _, v := range result.Violations {
        if v.Rule == "no_ballooning" {
            found = true
            if v.Severity != "error" {
                t.Errorf("expected error severity, got %s", v.Severity)
            }
            if v.Correctable {
                t.Error("ballooning should not be correctable")
            }
        }
    }
    if !found {
        t.Error("missing no_ballooning violation")
    }
}

func TestValidateHANAProfile_SwapFails(t *testing.T) {
	profile := &VMProfile{
		Role:                   "hana",
		CPUs:                   4,
		CPUAllocation:          "dedicated",
		MemoryBytes:            68719476736,
		NUMAPolicy:             func() *string { s := "centered"; return &s }(),
		HugepagesEnabled:       true,
		CPUPinning:             []byte(`{"vcpu0":"0"}`),
		MemoryReservationBytes: 68719476736,
		BallooningAllowed:      false,
		SwapAllowed:            true, // violation
	}
	result := ValidateHANAProfile(profile)
	if result.Passed {
		t.Fatal("expected failure due to swap")
	}
	for _, v := range result.Violations {
		if v.Rule == "no_swap" {
			if v.Correctable {
				t.Error("swap should not be correctable (guest OS setting)")
			}
		}
	}
}

func TestValidateHANAProfile_NumaPolicyFails(t *testing.T) {
    profile := &VMProfile{
        Role:                   "hana",
        CPUs:                   4,
        CPUAllocation:          "dedicated",
        MemoryBytes:            68719476736,
        NUMAPolicy:             nil,  // unset — violation
        HugepagesEnabled:       true,
        CPUPinning:             []byte(`{"vcpu0":"0"}`),
        MemoryReservationBytes: 68719476736,
        BallooningAllowed:      false,
        SwapAllowed:            false,
    }
    result := ValidateHANAProfile(profile)
    if result.Passed {
        t.Fatal("expected failure due to missing NUMA policy")
    }
    for _, v := range result.Violations {
        if v.Rule == "numa_pinned" {
            if v.Actual != "unset" {
                t.Errorf("actual = %q", v.Actual)
            }
        }
    }
}

func TestValidateHANAProfile_NumaPolicyValueFails(t *testing.T) {
    profile := &VMProfile{
        Role:                   "hana",
        CPUs:                   4,
        CPUAllocation:          "dedicated",
        MemoryBytes:            68719476736,
        NUMAPolicy:             func() *string { s := "interleave"; return &s }(),  // invalid value
        HugepagesEnabled:       true,
        CPUPinning:             []byte(`{"vcpu0":"0"}`),
        MemoryReservationBytes: 68719476736,
        BallooningAllowed:      false,
        SwapAllowed:            false,
    }
    result := ValidateHANAProfile(profile)
    if result.Passed {
        t.Fatal("expected failure due to invalid NUMA policy")
    }
    for _, v := range result.Violations {
        if v.Rule == "numa_policy_value" {
            if v.Actual != "interleave" {
                t.Errorf("actual = %q", v.Actual)
            }
        }
    }
}

func TestValidateHANAProfile_HugepagesDisabledFails(t *testing.T) {
    profile := &VMProfile{
        Role:                   "hana",
        CPUs:                   4,
        CPUAllocation:          "dedicated",
        MemoryBytes:            68719476736,
        NUMAPolicy:             func() *string { s := "centered"; return &s }(),
        HugepagesEnabled:       false,  // violation
        CPUPinning:             []byte(`{"vcpu0":"0"}`),
        MemoryReservationBytes: 68719476736,
        BallooningAllowed:      false,
        SwapAllowed:            false,
    }
    result := ValidateHANAProfile(profile)
    if result.Passed {
        t.Fatal("expected failure due to hugepages disabled")
    }
}

func TestValidateHANAProfile_DedicatedCPUFails(t *testing.T) {
	profile := &VMProfile{
		Role:                   "hana",
		CPUs:                   4,
		CPUAllocation:          "shared", // violation
		MemoryBytes:            68719476736,
		NUMAPolicy:             func() *string { s := "centered"; return &s }(),
		HugepagesEnabled:       true,
		CPUPinning:             []byte(`{"vcpu0":"0"}`),
		MemoryReservationBytes: 68719476736,
		BallooningAllowed:      false,
		SwapAllowed:            false,
	}
	result := ValidateHANAProfile(profile)
	if result.Passed {
		t.Fatal("expected failure due to non-dedicated CPU")
	}
	for _, v := range result.Violations {
		if v.Rule == "dedicated_cpu" {
			if v.Actual != "shared" {
				t.Errorf("actual = %q", v.Actual)
			}
			if !v.Correctable {
				t.Error("CPU allocation should be correctable")
			}
		}
	}
}

func TestValidateHANAProfile_CPPinningEmptyFails(t *testing.T) {
    profile := &VMProfile{
        Role:                   "hana",
        CPUs:                   4,
        CPUAllocation:          "dedicated",
        MemoryBytes:            68719476736,
        NUMAPolicy:             func() *string { s := "centered"; return &s }(),
        HugepagesEnabled:       true,
        CPUPinning:             []byte{},  // empty — violation
        MemoryReservationBytes: 68719476736,
        BallooningAllowed:      false,
        SwapAllowed:            false,
    }
    result := ValidateHANAProfile(profile)
    if result.Passed {
        t.Fatal("expected failure due to empty CPU pinning")
    }
    for _, v := range result.Violations {
        if v.Rule == "cpu_pinning" {
            if v.Actual != "empty" {
                t.Errorf("actual = %q", v.Actual)
            }
        }
    }
}

func TestValidateHANAProfile_MemoryReservationMismatchFails(t *testing.T) {
	profile := &VMProfile{
		Role:                   "hana",
		CPUs:                   4,
		CPUAllocation:          "dedicated",
		MemoryBytes:            68719476736,
		NUMAPolicy:             func() *string { s := "centered"; return &s }(),
		HugepagesEnabled:       true,
		CPUPinning:             []byte(`{"vcpu0":"0"}`),
		MemoryReservationBytes: 34359738368, // half — violation
		BallooningAllowed:      false,
		SwapAllowed:            false,
	}
	result := ValidateHANAProfile(profile)
	if result.Passed {
		t.Fatal("expected failure due to memory reservation mismatch")
	}
	for _, v := range result.Violations {
		if v.Rule == "memory_reservation" {
			if !v.Correctable {
				t.Error("memory reservation should be correctable")
			}
		}
	}
}

func TestValidateHANAProfile_CompliantProfile(t *testing.T) {
    profile := &VMProfile{
        Role:                   "hana",
        CPUs:                   4,
        CPUAllocation:          "dedicated",
        MemoryBytes:            68719476736,
        NUMAPolicy:             func() *string { s := "centered"; return &s }(),
        HugepagesEnabled:       true,
        CPUPinning:             []byte(`{"vcpu0":"0","vcpu1":"1","vcpu2":"2","vcpu3":"3"}`),
        MemoryReservationBytes: 68719476736,
        BallooningAllowed:      false,
        SwapAllowed:            false,
    }
    result := ValidateHANAProfile(profile)
    if !result.Passed {
        t.Fatalf("expected compliant profile, got violations: %d", len(result.Violations))
    }
    if len(result.Violations) != 0 {
        t.Errorf("expected no violations, got %d", len(result.Violations))
    }
}

func TestValidateHANAProfile_HANA_with_bind_numa(t *testing.T) {
    profile := &VMProfile{
        Role:                   "HANA",  // case-insensitive
        CPUs:                   8,
        CPUAllocation:          "dedicated",
        MemoryBytes:            137438953472,
        NUMAPolicy:             func() *string { s := "bind"; return &s }(),
        HugepagesEnabled:       true,
        CPUPinning:             []byte(`{"vcpu0":"0","vcpu1":"1","vcpu2":"2","vcpu3":"3","vcpu4":"4","vcpu5":"5","vcpu6":"6","vcpu7":"7"}`),
        MemoryReservationBytes: 137438953472,
        BallooningAllowed:      false,
        SwapAllowed:            false,
    }
    result := ValidateHANAProfile(profile)
    if !result.Passed {
        t.Fatalf("HANA with bind NUMA should be compliant, got %d violations", len(result.Violations))
    }
}

func TestComputeHash_Deterministic(t *testing.T) {
    data := []byte("test-data")
    h1 := ComputeHash(data)
    h2 := ComputeHash(data)
    if h1 != h2 {
        t.Errorf("hash not deterministic: %s vs %s", h1, h2)
    }
    if len(h1) != 64 {
        t.Errorf("expected 64-char hex hash, got %d", len(h1))
    }
}

func TestComputeHash_DifferentData(t *testing.T) {
    h1 := ComputeHash([]byte("data1"))
    h2 := ComputeHash([]byte("data2"))
    if h1 == h2 {
        t.Fatal("different data should produce different hashes")
    }
}

func TestComputeHash_VerifyKnown(t *testing.T) {
    // SHA-256 of "test-data" — verify with known value
    data := []byte("test-data")
    hash := ComputeHash(data)
    // Compute expected
    h := sha256.New()
    h.Write(data)
    expected := hex.EncodeToString(h.Sum(nil))
    if hash != expected {
        t.Errorf("hash mismatch: got %s, expected %s", hash, expected)
    }
}

func TestHashEvidence_ChainIntegrity(t *testing.T) {
    evidence := map[string]interface{}{
        "vm_id":     "vm-1",
        "role":      "hana",
        "cpus":      4,
    }
    prevHash := "genesis-hash-abcdef"
    hash1 := HashEvidence(prevHash, evidence)
    if len(hash1) != 64 {
        t.Fatalf("hash length = %d", len(hash1))
    }
    // Hash with different previous should be different
    hash2 := HashEvidence("different-prev", evidence)
    if hash1 == hash2 {
        t.Fatal("different previous hash should produce different result")
    }
}

func TestChainHash(t *testing.T) {
    h1 := ChainHash("abc", "def")
    h2 := ChainHash("abc", "def")
    if h1 != h2 {
        t.Fatal("ChainHash not deterministic")
    }
    h3 := ChainHash("xyz", "def")
    if h1 == h3 {
        t.Fatal("different input should produce different hash")
    }
    if len(h1) != 64 {
        t.Errorf("expected 64-char hash, got %d", len(h1))
    }
}

func TestVMProfile_JSONSerialization(t *testing.T) {
    profile := &VMProfile{
        Role:                   "hana",
        CPUs:                   8,
        CPUAllocation:          "dedicated",
        MemoryBytes:            137438953472,
        NUMAPolicy:             func() *string { s := "centered"; return &s }(),
        HugepagesEnabled:       true,
        CPUPinning:             []byte(`{"vcpu0":"0"}`),
        MemoryReservationBytes: 137438953472,
        BallooningAllowed:      false,
        SwapAllowed:            false,
        OS:                     "SUSE Linux Enterprise Server 15 SP7",
        HasHugepagesConfig:     true,
        NUMANodeCount:          1,
        HugepagesTotalKB:       2048000,
        HostCPUCount:           32,
        HostMemoryBytes:        137438953472,
    }
    data, err := json.Marshal(profile)
    if err != nil {
        t.Fatalf("marshal: %v", err)
    }
    var decoded VMProfile
    if err := json.Unmarshal(data, &decoded); err != nil {
        t.Fatalf("unmarshal: %v", err)
    }
    if decoded.Role != profile.Role {
        t.Errorf("Role = %q", decoded.Role)
    }
    if decoded.CPUs != profile.CPUs {
        t.Errorf("CPUs = %d", decoded.CPUs)
    }
    // NUMAPolicy is a pointer — verify it roundtrips
    if decoded.NUMAPolicy == nil {
        t.Error("NUMAPolicy should not be nil after roundtrip")
    } else if *decoded.NUMAPolicy != "centered" {
        t.Errorf("NUMAPolicy = %q", *decoded.NUMAPolicy)
    }
}

func TestValidationResult_JSON(t *testing.T) {
    result := ValidationResult{
        VMID:      "vm-1",
        CheckType: "hana_guardrails",
        Passed:    false,
        Violations: []Violation{
            {Rule: "numa_pinned", Field: "numa_policy", Expected: "centered", Actual: "unset", Severity: "error", Correctable: true},
        },
        Evidence: map[string]interface{}{
            "role": "hana",
            "cpus": 4,
        },
        CheckedAt: time.Now().UTC().Format(time.RFC3339),
    }
    data, err := json.Marshal(result)
    if err != nil {
        t.Fatalf("marshal: %v", err)
    }
    var decoded ValidationResult
    if err := json.Unmarshal(data, &decoded); err != nil {
        t.Fatalf("unmarshal: %v", err)
    }
    if decoded.VMID != "vm-1" {
        t.Errorf("VMID = %q", decoded.VMID)
    }
    if decoded.Passed {
        t.Error("expected Passed=false")
    }
    if len(decoded.Violations) != 1 {
        t.Errorf("expected 1 violation, got %d", len(decoded.Violations))
    }
    if decoded.Violations[0].Rule != "numa_pinned" {
        t.Errorf("violation rule = %q", decoded.Violations[0].Rule)
    }
}

func TestViolation_StringFields(t *testing.T) {
    v := Violation{
        Rule:       "test_rule",
        Field:      "test_field",
        Expected:   "expected_val",
        Actual:     "actual_val",
        Severity:   "error",
        Correctable: true,
    }
    if v.Rule != "test_rule" {
        t.Errorf("Rule = %q", v.Rule)
    }
    if v.Severity != "error" {
        t.Errorf("Severity = %q", v.Severity)
    }
    if !v.Correctable {
        t.Error("Correctable should be true")
    }
}
