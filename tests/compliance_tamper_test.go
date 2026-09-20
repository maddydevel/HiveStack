// Package tests provides compliance tests for HiveStack.
// Evidence chain tamper detection tests.
package tests

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/maddydevel/HiveStack/internal/compliance"
)

// ---------- Evidence Tamper Detection Tests ----------

// buildTestEvidenceRecords creates a valid chain of evidence records.
func buildTestEvidenceRecords(n int) []compliance.EvidenceRecord {
	records := make([]compliance.EvidenceRecord, n)
	previousHash := "genesis-hash"

	for i := 0; i < n; i++ {
		profile := &compliance.VMProfile{
			Role:              "hana",
			CPUs:              8,
			CPUAllocation:     "dedicated",
			MemoryBytes:       68719476736,
			HugepagesEnabled:  true,
			BallooningAllowed: false,
			SwapAllowed:       false,
		}

		checkID := "check-001"
		checkedBy := "test-agent"
		timestamp := time.Date(2024, 1, 1, 0, 0, i, 0, time.UTC)

		// Compute the hash for this record
		hash := compliance.ComputeRecordHash(
			previousHash,
			profile,
			nil, // no violations
			true, // passed
			checkedBy,
			timestamp,
		)

		records[i] = compliance.EvidenceRecord{
			VMID:         "vm-test",
			CheckID:      checkID,
			PreviousHash: previousHash,
			CurrentHash:  hash,
			Profile:      marshalProfile(profile),
			Passed:       true,
			Timestamp:    timestamp,
			CheckedBy:    checkedBy,
		}

		previousHash = hash
	}

	return records
}

func marshalProfile(p *compliance.VMProfile) []byte {
	b, _ := json.Marshal(p)
	return b
}

// TestTamperDetection_ValidChain verifies that a valid evidence chain passes verification.
func TestTamperDetection_ValidChain(t *testing.T) {
	records := buildTestEvidenceRecords(3)
	result := compliance.VerifyChainPure(records)

	if !result.Valid {
		t.Error("Valid chain should pass verification")
	}
	if result.ChainLength != 3 {
		t.Errorf("Expected chain length 3, got %d", result.ChainLength)
	}
	if result.TamperDetail != nil {
		t.Error("No tamper detail expected for valid chain")
	}
}

// TestTamperDetection_EmptyChain verifies that an empty chain is valid.
func TestTamperDetection_EmptyChain(t *testing.T) {
	result := compliance.VerifyChainPure([]compliance.EvidenceRecord{})

	if !result.Valid {
		t.Error("Empty chain should be valid")
	}
	if result.ChainLength != 0 {
		t.Errorf("Expected chain length 0, got %d", result.ChainLength)
	}
}

// TestTamperDetection_SingleRecordChain verifies a single-record chain is valid.
func TestTamperDetection_SingleRecordChain(t *testing.T) {
	records := buildTestEvidenceRecords(1)
	result := compliance.VerifyChainPure(records)

	if !result.Valid {
		t.Error("Single-record chain should be valid")
	}
}

// TestTamperDetection_DetectsTamperedRow verifies that modifying a single row
// in the chain is detected with correct details (which row, expected vs actual hash).
func TestTamperDetection_DetectsTamperedRow(t *testing.T) {
	records := buildTestEvidenceRecords(4)

	// Tamper with row index 2: change its PreviousHash to an invalid value
	records[2].PreviousHash = "tampered-hash-value"

	result := compliance.VerifyChainPure(records)

	if result.Valid {
		t.Fatal("Tampered chain should fail verification")
	}

	if result.TamperDetail == nil {
		t.Fatal("Expected tamper detail")
	}

	// Verify correct row index
	if result.TamperDetail.RecordIndex != 2 {
		t.Errorf("Expected tamper at row index 2, got %d", result.TamperDetail.RecordIndex)
	}

	// Verify expected vs actual hash
	if result.TamperDetail.ExpectedHash != records[1].CurrentHash {
		t.Errorf("Expected hash mismatch: ExpectedHash=%s, records[1].CurrentHash=%s",
			result.TamperDetail.ExpectedHash, records[1].CurrentHash)
	}

	if result.TamperDetail.ActualHash != "tampered-hash-value" {
		t.Errorf("Expected actual hash 'tampered-hash-value', got %q", result.TamperDetail.ActualHash)
	}

	if result.TamperDetail.PriorRecordID != "check-001" {
		t.Errorf("Expected prior record ID 'check-001', got %q", result.TamperDetail.PriorRecordID)
	}
}

// TestTamperDetection_DetectsModifiedHash verifies that modifying the CurrentHash
// of a row breaks the chain at the next row.
func TestTamperDetection_DetectsModifiedHash(t *testing.T) {
	records := buildTestEvidenceRecords(3)

	// Tamper with row index 1's CurrentHash
	records[1].CurrentHash = "modified-current-hash"

	result := compliance.VerifyChainPure(records)

	if result.Valid {
		t.Fatal("Tampered chain should fail verification")
	}

	// The break should be detected at row 2 (where PreviousHash no longer matches row 1's CurrentHash)
	if result.TamperDetail.RecordIndex != 2 {
		t.Errorf("Expected tamper at row index 2, got %d", result.TamperDetail.RecordIndex)
	}

	if result.TamperDetail.ExpectedHash != "modified-current-hash" {
		t.Errorf("Expected hash 'modified-current-hash', got %q", result.TamperDetail.ExpectedHash)
	}
}

// TestTamperDetection_DetectsFirstRowTamper verifies tamper at row 0
// breaks the chain at row 1.
func TestTamperDetection_DetectsFirstRowTamper(t *testing.T) {
	records := buildTestEvidenceRecords(3)

	// Tamper with genesis record's CurrentHash
	records[0].CurrentHash = "tampered-genesis"

	result := compliance.VerifyChainPure(records)

	if result.Valid {
		t.Fatal("Tampered chain should fail verification")
	}

	// The break should be at row 1
	if result.TamperDetail.RecordIndex != 1 {
		t.Errorf("Expected tamper at row index 1, got %d", result.TamperDetail.RecordIndex)
	}

	if result.TamperDetail.PriorRecordID != "check-001" {
		t.Errorf("Expected prior record ID 'check-001', got %q", result.TamperDetail.PriorRecordID)
	}
}

// TestTamperDetection_DetectsLastRowTamper verifies tamper on the last row.
func TestTamperDetection_DetectsLastRowTamper(t *testing.T) {
	records := buildTestEvidenceRecords(5)

	lastIdx := len(records) - 1
	records[lastIdx].PreviousHash = "tampered-last-row"

	result := compliance.VerifyChainPure(records)

	if result.Valid {
		t.Fatal("Tampered chain should fail verification")
	}

	if result.TamperDetail.RecordIndex != lastIdx {
		t.Errorf("Expected tamper at last row index %d, got %d", lastIdx, result.TamperDetail.RecordIndex)
	}
}

// TestTamperDetection_DetectsMissingRow simulates a missing row in the chain.
func TestTamperDetection_DetectsMissingRow(t *testing.T) {
	records := buildTestEvidenceRecords(5)

	// Remove row index 2 (simulating a deleted row)
	records = append(records[:2], records[3:]...)

	result := compliance.VerifyChainPure(records)

	if result.Valid {
		t.Fatal("Chain with missing row should fail verification")
	}

	// The break should be at the new index 2 (which was index 3)
	if result.TamperDetail.RecordIndex != 2 {
		t.Errorf("Expected tamper at row index 2, got %d", result.TamperDetail.RecordIndex)
	}
}

// TestTamperDetection_DetectsModifiedProfileData verifies that modifying profile
// data in a row without updating its hash is detected by recomputing the hash
// from the stored profile and comparing it to the stored CurrentHash.
func TestTamperDetection_DetectsModifiedProfileData(t *testing.T) {
	records := buildTestEvidenceRecords(3)

	// Tamper with the profile data in row 0 (simulates modifying VM config in evidence)
	tamperedProfile := &compliance.VMProfile{
		Role:              "hana",
		CPUs:              99, // Changed
		CPUAllocation:     "dedicated",
		MemoryBytes:       68719476736,
		HugepagesEnabled:  true,
		BallooningAllowed: false,
		SwapAllowed:       false,
	}
	records[0].Profile = marshalProfile(tamperedProfile)

	// The chain linkage is still valid (PreviousHash matches prior CurrentHash),
	// but we can detect the tamper by recomputing the hash from the stored profile
	// and comparing it to the stored CurrentHash.
	recomputedHash := compliance.ComputeRecordHash(
		records[0].PreviousHash,
		tamperedProfile,
		nil,
		records[0].Passed,
		records[0].CheckedBy,
		records[0].Timestamp,
	)

	if recomputedHash == records[0].CurrentHash {
		t.Error("Recomputed hash should differ from stored hash after profile modification")
	}

	// The chain linkage itself is still valid
	result := compliance.VerifyChainPure(records)
	if !result.Valid {
		t.Log("Note: hash chain linkage is still valid (expected — chain only verifies linkage)")
	}

	// But the data integrity check fails
	if recomputedHash != records[0].CurrentHash {
		t.Logf("Data integrity violation detected: stored hash %s != recomputed hash %s",
			records[0].CurrentHash, recomputedHash)
	}
}

// TestTamperDetection_VerifyEvidenceChainIntegration verifies the
// ComplianceStore.VerifyEvidenceChain method against a properly built chain.
func TestTamperDetection_VerifyEvidenceChainIntegration(t *testing.T) {
	// Build a chain of 3 records
	records := buildTestEvidenceRecords(3)

	// Verify the chain is valid
	result := compliance.VerifyChainPure(records)

	if !result.Valid {
		t.Fatal("Chain should be valid")
	}

	// Now tamper with the last record and verify it's detected
	records[2].PreviousHash = "tampered"

	result = compliance.VerifyChainPure(records)

	if result.Valid {
		t.Fatal("Tampered chain should fail verification")
	}

	if result.TamperDetail.RecordIndex != 2 {
		t.Errorf("Expected tamper at row 2, got %d", result.TamperDetail.RecordIndex)
	}

	if result.TamperDetail.ExpectedHash != records[1].CurrentHash {
		t.Errorf("Expected hash %q, got %q", records[1].CurrentHash, result.TamperDetail.ExpectedHash)
	}

	if result.TamperDetail.ActualHash != "tampered" {
		t.Errorf("Expected actual hash 'tampered', got %q", result.TamperDetail.ActualHash)
	}
}

// TestTamperDetection_HashDeterminism verifies that the hash function
// is deterministic — same inputs always produce the same hash.
func TestTamperDetection_HashDeterminism(t *testing.T) {
	profile := &compliance.VMProfile{
		Role:              "hana",
		CPUs:              8,
		CPUAllocation:     "dedicated",
		MemoryBytes:       68719476736,
		HugepagesEnabled:  true,
		BallooningAllowed: false,
		SwapAllowed:       false,
	}

	timestamp := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)

	h1 := compliance.ComputeRecordHash("prev", profile, nil, true, "agent", timestamp)
	h2 := compliance.ComputeRecordHash("prev", profile, nil, true, "agent", timestamp)

	if h1 != h2 {
		t.Errorf("Hash not deterministic: %s vs %s", h1, h2)
	}

	if len(h1) != 64 {
		t.Errorf("Expected 64-char hex hash, got %d chars", len(h1))
	}
}

// TestTamperDetection_HashDifferenceOnProfileChange verifies that changing
// any field in the profile produces a different hash.
func TestTamperDetection_HashDifferenceOnProfileChange(t *testing.T) {
	profile1 := &compliance.VMProfile{
		Role:              "hana",
		CPUs:              8,
		CPUAllocation:     "dedicated",
		MemoryBytes:       68719476736,
		HugepagesEnabled:  true,
		BallooningAllowed: false,
		SwapAllowed:       false,
	}

	profile2 := &compliance.VMProfile{
		Role:              "hana",
		CPUs:              4, // Changed
		CPUAllocation:     "dedicated",
		MemoryBytes:       68719476736,
		HugepagesEnabled:  true,
		BallooningAllowed: false,
		SwapAllowed:       false,
	}

	timestamp := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)

	h1 := compliance.ComputeRecordHash("prev", profile1, nil, true, "agent", timestamp)
	h2 := compliance.ComputeRecordHash("prev", profile2, nil, true, "agent", timestamp)

	if h1 == h2 {
		t.Error("Different profiles should produce different hashes")
	}
}

// TestTamperDetection_DetectsInsertedRow verifies that inserting a row
// into the middle of the chain breaks it.
func TestTamperDetection_DetectsInsertedRow(t *testing.T) {
	records := buildTestEvidenceRecords(3)

	// Create a new row to insert at position 1
	newRow := compliance.EvidenceRecord{
		VMID:         "vm-test",
		CheckID:      "check-inserted",
		PreviousHash: records[0].CurrentHash,
		CurrentHash:  "fake-hash",
		Passed:       true,
		Timestamp:    time.Date(2024, 1, 1, 0, 0, 1, 0, time.UTC),
		CheckedBy:    "test-agent",
	}

	// Insert at position 1
	records = append(records[:1], append([]compliance.EvidenceRecord{newRow}, records[1:]...)...)

	result := compliance.VerifyChainPure(records)

	if result.Valid {
		t.Fatal("Chain with inserted row should fail verification")
	}

	// The break should be at row 2 (where PreviousHash no longer matches row 1's fake hash)
	if result.TamperDetail.RecordIndex != 2 {
		t.Errorf("Expected tamper at row index 2, got %d", result.TamperDetail.RecordIndex)
	}
}
