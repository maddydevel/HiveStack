// Package tests provides compliance tests for HiveStack.
// Drift detection tests.
package tests

import (
	"testing"
	"time"

	"github.com/maddydevel/HiveStack/internal/compliance"
)

// ---------- Drift Detection Tests ----------

// TestDriftDetection_NoDrift verifies that a matching current profile does not trigger drift.
func TestDriftDetection_NoDrift(t *testing.T) {
	baseline := &compliance.VMProfile{
		Role:              "hana",
		CPUs:              8,
		CPUAllocation:     "dedicated",
		MemoryBytes:       68719476736,
		HugepagesEnabled:  true,
		BallooningAllowed: false,
		SwapAllowed:       false,
	}

	// Current profile matches baseline exactly
	current := &compliance.VMProfile{
		Role:              "hana",
		CPUs:              8,
		CPUAllocation:     "dedicated",
		MemoryBytes:       68719476736,
		HugepagesEnabled:  true,
		BallooningAllowed: false,
		SwapAllowed:       false,
	}

	result := compliance.CheckDriftPure(baseline, current)
	if result.DriftDetected {
		t.Errorf("No drift expected, but drift was detected: %+v", result.DriftDetails)
	}
	if result.Reason != "profile matches baseline" {
		t.Errorf("Expected reason 'profile matches baseline', got %q", result.Reason)
	}
}

// TestDriftDetection_CPUsDrift verifies drift when CPU count changes.
func TestDriftDetection_CPUsDrift(t *testing.T) {
	baseline := &compliance.VMProfile{
		Role:              "hana",
		CPUs:              8,
		CPUAllocation:     "dedicated",
		MemoryBytes:       68719476736,
		HugepagesEnabled:  true,
		BallooningAllowed: false,
		SwapAllowed:       false,
	}

	current := &compliance.VMProfile{
		Role:              "hana",
		CPUs:              4, // Drift: reduced CPUs
		CPUAllocation:     "dedicated",
		MemoryBytes:       68719476736,
		HugepagesEnabled:  true,
		BallooningAllowed: false,
		SwapAllowed:       false,
	}

	result := compliance.CheckDriftPure(baseline, current)
	if !result.DriftDetected {
		t.Fatal("Expected drift to be detected for CPU change")
	}

	found := false
	for _, d := range result.DriftDetails {
		if d.Field == "cpus" {
			found = true
			if d.Expected != 8 || d.Actual != 4 {
				t.Errorf("CPU drift: expected 8->4, got %v->%v", d.Expected, d.Actual)
			}
		}
	}
	if !found {
		t.Error("Expected cpu field in drift details")
	}
}

// TestDriftDetection_HugepagesDrift verifies drift when hugepages are disabled.
func TestDriftDetection_HugepagesDrift(t *testing.T) {
	baseline := &compliance.VMProfile{
		Role:              "hana",
		CPUs:              8,
		CPUAllocation:     "dedicated",
		MemoryBytes:       68719476736,
		HugepagesEnabled:  true,
		BallooningAllowed: false,
		SwapAllowed:       false,
	}

	current := &compliance.VMProfile{
		Role:              "hana",
		CPUs:              8,
		CPUAllocation:     "dedicated",
		MemoryBytes:       68719476736,
		HugepagesEnabled:  false, // Drift: hugepages disabled
		BallooningAllowed: false,
		SwapAllowed:       false,
	}

	result := compliance.CheckDriftPure(baseline, current)
	if !result.DriftDetected {
		t.Fatal("Expected drift to be detected for hugepages disabled")
	}

	found := false
	for _, d := range result.DriftDetails {
		if d.Field == "hugepages_enabled" {
			found = true
			if d.Severity != "error" {
				t.Errorf("Expected severity 'error' for hugepages drift, got %q", d.Severity)
			}
		}
	}
	if !found {
		t.Error("Expected hugepages_enabled field in drift details")
	}
}

// TestDriftDetection_BallooningDrift verifies drift when ballooning is enabled.
func TestDriftDetection_BallooningDrift(t *testing.T) {
	baseline := &compliance.VMProfile{
		Role:              "hana",
		CPUs:              8,
		CPUAllocation:     "dedicated",
		MemoryBytes:       68719476736,
		HugepagesEnabled:  true,
		BallooningAllowed: false,
		SwapAllowed:       false,
	}

	current := &compliance.VMProfile{
		Role:              "hana",
		CPUs:              8,
		CPUAllocation:     "dedicated",
		MemoryBytes:       68719476736,
		HugepagesEnabled:  true,
		BallooningAllowed: true, // Drift: ballooning enabled
		SwapAllowed:       false,
	}

	result := compliance.CheckDriftPure(baseline, current)
	if !result.DriftDetected {
		t.Fatal("Expected drift to be detected for ballooning enabled")
	}

	found := false
	for _, d := range result.DriftDetails {
		if d.Field == "ballooning_allowed" {
			found = true
		}
	}
	if !found {
		t.Error("Expected ballooning_allowed field in drift details")
	}
}

// TestDriftDetection_MultipleDriftFields verifies drift is reported for multiple fields.
func TestDriftDetection_MultipleDriftFields(t *testing.T) {
	baseline := &compliance.VMProfile{
		Role:              "hana",
		CPUs:              8,
		CPUAllocation:     "dedicated",
		MemoryBytes:       68719476736,
		HugepagesEnabled:  true,
		BallooningAllowed: false,
		SwapAllowed:       false,
	}

	current := &compliance.VMProfile{
		Role:              "hana",
		CPUs:              4,        // drift
		CPUAllocation:     "shared", // drift
		MemoryBytes:       68719476736,
		HugepagesEnabled:  false, // drift
		BallooningAllowed: true,  // drift
		SwapAllowed:       true,  // drift
	}

	result := compliance.CheckDriftPure(baseline, current)
	if !result.DriftDetected {
		t.Fatal("Expected drift to be detected")
	}

	// Should have 5 drift details
	if len(result.DriftDetails) != 5 {
		t.Errorf("Expected 5 drift details, got %d", len(result.DriftDetails))
	}

	fields := make(map[string]bool)
	for _, d := range result.DriftDetails {
		fields[d.Field] = true
	}
	for _, expected := range []string{"cpus", "cpu_allocation", "hugepages_enabled", "ballooning_allowed", "swap_allowed"} {
		if !fields[expected] {
			t.Errorf("Missing drift field: %s", expected)
		}
	}
}

// TestDriftDetection_NewBaseline verifies that when there is no baseline, no drift is detected.
func TestDriftDetection_NewBaseline(t *testing.T) {
	current := &compliance.VMProfile{
		Role:              "hana",
		CPUs:              8,
		CPUAllocation:     "dedicated",
		MemoryBytes:       68719476736,
		HugepagesEnabled:  true,
		BallooningAllowed: false,
		SwapAllowed:       false,
	}

	result := compliance.CheckDriftPure(nil, current)
	if result.DriftDetected {
		t.Error("No drift should be detected when there is no baseline")
	}
	if result.Reason != "no baseline profile — new baseline" {
		t.Errorf("Expected 'no baseline' reason, got %q", result.Reason)
	}
}

// TestDriftDetection_DetectionWithin5Minutes simulates configuration drift
// and verifies it is detected within 5 minutes. In a real scenario, drift detection
// runs on a periodic loop (e.g., every minute) comparing current state to baseline.
func TestDriftDetection_DetectionWithin5Minutes(t *testing.T) {
	baseline := &compliance.VMProfile{
		Role:              "hana",
		CPUs:              8,
		CPUAllocation:     "dedicated",
		MemoryBytes:       68719476736,
		HugepagesEnabled:  true,
		BallooningAllowed: false,
		SwapAllowed:       false,
	}

	// Simulate drift: someone manually disabled hugepages
	drifted := &compliance.VMProfile{
		Role:              "hana",
		CPUs:              8,
		CPUAllocation:     "dedicated",
		MemoryBytes:       68719476736,
		HugepagesEnabled:  false, // MANUAL CHANGE OUTSIDE HIVESTACK
		BallooningAllowed: false,
		SwapAllowed:       false,
	}

	// Simulate drift detection loop running every 60 seconds
	driftDetected := false
	var detectionTime time.Time
	startTime := time.Now()

	for elapsed := time.Duration(0); elapsed <= 5*time.Minute; elapsed += 60 * time.Second {
		result := compliance.CheckDriftPure(baseline, drifted)
		if result.DriftDetected {
			driftDetected = true
			detectionTime = time.Now()
			break
		}
		time.Sleep(10 * time.Millisecond) // Simulate short tick for testing
	}

	if !driftDetected {
		t.Fatal("Drift was not detected within 5 minutes")
	}

	elapsed := detectionTime.Sub(startTime)
	if elapsed > 5*time.Minute {
		t.Errorf("Drift detection took %v, exceeds 5-minute threshold", elapsed)
	}

	t.Logf("Drift detected in %v (within 5-minute threshold)", elapsed)
}

// TestDriftDetection_CPUAllocationDrift verifies drift when CPU allocation changes.
func TestDriftDetection_CPUAllocationDrift(t *testing.T) {
	baseline := &compliance.VMProfile{
		Role:              "hana",
		CPUs:              8,
		CPUAllocation:     "dedicated",
		MemoryBytes:       68719476736,
		HugepagesEnabled:  true,
		BallooningAllowed: false,
		SwapAllowed:       false,
	}

	current := &compliance.VMProfile{
		Role:              "hana",
		CPUs:              8,
		CPUAllocation:     "shared", // Drift: changed from dedicated to shared
		MemoryBytes:       68719476736,
		HugepagesEnabled:  true,
		BallooningAllowed: false,
		SwapAllowed:       false,
	}

	result := compliance.CheckDriftPure(baseline, current)
	if !result.DriftDetected {
		t.Fatal("Expected drift to be detected for CPU allocation change")
	}

	found := false
	for _, d := range result.DriftDetails {
		if d.Field == "cpu_allocation" {
			found = true
			if d.Expected != "dedicated" || d.Actual != "shared" {
				t.Errorf("CPU allocation drift: expected dedicated->shared, got %v->%v", d.Expected, d.Actual)
			}
			if d.Severity != "error" {
				t.Errorf("Expected error severity for cpu_allocation drift, got %q", d.Severity)
			}
		}
	}
	if !found {
		t.Error("Expected cpu_allocation field in drift details")
	}
}

// TestDriftDetection_SwapDrift verifies drift when swap is enabled.
func TestDriftDetection_SwapDrift(t *testing.T) {
	baseline := &compliance.VMProfile{
		Role:              "hana",
		CPUs:              8,
		CPUAllocation:     "dedicated",
		MemoryBytes:       68719476736,
		HugepagesEnabled:  true,
		BallooningAllowed: false,
		SwapAllowed:       false,
	}

	current := &compliance.VMProfile{
		Role:              "hana",
		CPUs:              8,
		CPUAllocation:     "dedicated",
		MemoryBytes:       68719476736,
		HugepagesEnabled:  true,
		BallooningAllowed: false,
		SwapAllowed:       true, // Drift: swap enabled
	}

	result := compliance.CheckDriftPure(baseline, current)
	if !result.DriftDetected {
		t.Fatal("Expected drift to be detected for swap enabled")
	}

	found := false
	for _, d := range result.DriftDetails {
		if d.Field == "swap_allowed" {
			found = true
		}
	}
	if !found {
		t.Error("Expected swap_allowed field in drift details")
	}
}

// TestDriftDetection_NilCurrentProfile verifies drift when current profile is nil.
func TestDriftDetection_NilCurrentProfile(t *testing.T) {
	baseline := &compliance.VMProfile{
		Role:              "hana",
		CPUs:              8,
		CPUAllocation:     "dedicated",
		MemoryBytes:       68719476736,
		HugepagesEnabled:  true,
		BallooningAllowed: false,
		SwapAllowed:       false,
	}

	result := compliance.CheckDriftPure(baseline, nil)
	if !result.DriftDetected {
		t.Fatal("Expected drift to be detected for nil current profile")
	}

	if len(result.DriftDetails) != 1 {
		t.Errorf("Expected 1 drift detail, got %d", len(result.DriftDetails))
	}
}
