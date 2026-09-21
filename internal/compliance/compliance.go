// Package compliance enforces SAP HANA VM guardrails for HiveStack.
//
// The host runs certified SLES 15 SP7 + KVM — no re-certification is needed.
// Guest OS compatibility is the customer's responsibility (migrated from VMware
// or fresh installed). This package enforces VM-level guardrails so that HANA
// VMs stay within the certified envelope:
//
//   - NUMA alignment: vCPUs pinned to a single NUMA node
//   - Hugepages: hugepages enabled and sized appropriately
//   - Dedicated vCPUs: no CPU overcommit; dedicated allocation
//   - No ballooning: memory ballooning disabled
//   - No swap: swap disabled inside the VM
//
// Dual enforcement:
//  1. Manager side (this package): API rejects non-compliant HANA VM specs
//  2. Node Agent side: libvirt XML generation refuses non-compliant configs
//
// Compliance evidence is hash-chained and stored in the compliance_evidence table
// for audit purposes.
package compliance

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/maddydevel/HiveStack/internal/db"
)

// VMProfile holds the VM configuration to validate.
type VMProfile struct {
	Role                   string  `json:"role"`
	CPUs                   int     `json:"cpus"`
	CPUAllocation          string  `json:"cpu_allocation"`
	MemoryBytes            int64   `json:"memory_bytes"`
	NUMAPolicy             *string `json:"numa_policy"`
	HugepagesEnabled       bool    `json:"hugepages_enabled"`
	CPUPinning             []byte  `json:"cpu_pinning"` // JSON
	MemoryReservationBytes int64   `json:"memory_reservation_bytes"`
	BallooningAllowed      bool    `json:"ballooning_allowed"`
	SwapAllowed            bool    `json:"swap_allowed"`
	OS                     string  `json:"os"`
	HasHugepagesConfig     bool    `json:"has_hugepages_config"`
	NUMANodeCount          int     `json:"numa_node_count"`
	HugepagesTotalKB       int64   `json:"hugepages_total_kb"`
	HostCPUCount           int     `json:"host_cpu_count"`
	HostMemoryBytes        int64   `json:"host_memory_bytes"`
}

// ValidationResult represents the outcome of a compliance check.
type ValidationResult struct {
	VMID       string                 `json:"vm_id"`
	CheckType  string                 `json:"check_type"`
	Passed     bool                   `json:"passed"`
	Violations []Violation            `json:"violations"`
	Evidence   map[string]interface{} `json:"evidence"`
	CheckedAt  string                 `json:"checked_at"`
}

// Violation describes a single guardrail breach.
type Violation struct {
	Rule        string `json:"rule"`
	Field       string `json:"field"`
	Expected    string `json:"expected"`
	Actual      string `json:"actual"`
	Severity    string `json:"severity"` // "error" or "warning"
	Correctable bool   `json:"correctable"`
}

// ValidateHANAProfile validates a VM profile against HANA guardrails.
// Returns a ValidationResult; if Passed is false, the VM must not be created
// or started until violations are resolved.
func ValidateHANAProfile(v *VMProfile) ValidationResult {
	result := ValidationResult{
		VMID:       "",
		CheckType:  "hana_guardrails",
		Passed:     true,
		Violations: []Violation{},
		Evidence: map[string]interface{}{
			"role":               v.Role,
			"cpus":               v.CPUs,
			"memory_bytes":       v.MemoryBytes,
			"hugepages_enabled":  v.HugepagesEnabled,
			"ballooning_allowed": v.BallooningAllowed,
			"swap_allowed":       v.SwapAllowed,
			"numa_policy": func() string {
				if v.NUMAPolicy != nil {
					return *v.NUMAPolicy
				}
				return ""
			}(),
		},
		CheckedAt: "",
	}

	role := strings.ToLower(v.Role)
	if role != "hana" {
		// Not a HANA VM — skip guardrail checks
		result.Evidence["skip_reason"] = "not a HANA VM"
		return result
	}

	// 1. NUMA: vCPUs must be pinned to a single NUMA node
	if v.NUMAPolicy == nil || *v.NUMAPolicy == "" {
		result.Passed = false
		result.Violations = append(result.Violations, Violation{
			Rule:        "numa_pinned",
			Field:       "numa_policy",
			Expected:    "centered or bind with single NUMA node",
			Actual:      "unset",
			Severity:    "error",
			Correctable: true,
		})
	} else if *v.NUMAPolicy != "centered" && *v.NUMAPolicy != "bind" {
		result.Passed = false
		result.Violations = append(result.Violations, Violation{
			Rule:        "numa_policy_value",
			Field:       "numa_policy",
			Expected:    "centered or bind",
			Actual:      *v.NUMAPolicy,
			Severity:    "error",
			Correctable: true,
		})
	}

	// 2. Hugepages must be enabled
	if !v.HugepagesEnabled {
		result.Passed = false
		result.Violations = append(result.Violations, Violation{
			Rule:        "hugepages_enabled",
			Field:       "hugepages_enabled",
			Expected:    "true",
			Actual:      "false",
			Severity:    "error",
			Correctable: true,
		})
	}

	// 3. Dedicated vCPUs: CPU allocation must be "dedicated" (no overcommit)
	if v.CPUAllocation != "dedicated" {
		result.Passed = false
		result.Violations = append(result.Violations, Violation{
			Rule:        "dedicated_cpu",
			Field:       "cpu_allocation",
			Expected:    "dedicated",
			Actual:      v.CPUAllocation,
			Severity:    "error",
			Correctable: true,
		})
	}

	// 4. No ballooning
	if v.BallooningAllowed {
		result.Passed = false
		result.Violations = append(result.Violations, Violation{
			Rule:        "no_ballooning",
			Field:       "ballooning_allowed",
			Expected:    "false",
			Actual:      "true",
			Severity:    "error",
			Correctable: false, // ballooning is a host-level setting
		})
	}

	// 5. No swap
	if v.SwapAllowed {
		result.Passed = false
		result.Violations = append(result.Violations, Violation{
			Rule:        "no_swap",
			Field:       "swap_allowed",
			Expected:    "false",
			Actual:      "true",
			Severity:    "error",
			Correctable: false, // swap is inside the guest OS
		})
	}

	// 6. CPU pinning must be non-empty for HANA
	if len(v.CPUPinning) == 0 {
		result.Passed = false
		result.Violations = append(result.Violations, Violation{
			Rule:        "cpu_pinning",
			Field:       "cpu_pinning",
			Expected:    "non-empty vCPU-to-host-CPU mapping",
			Actual:      "empty",
			Severity:    "error",
			Correctable: true,
		})
	}

	// 7. Memory reservation must equal requested memory (no overcommit)
	if v.MemoryReservationBytes != v.MemoryBytes {
		result.Passed = false
		result.Violations = append(result.Violations, Violation{
			Rule:        "memory_reservation",
			Field:       "memory_reservation_bytes",
			Expected:    fmt.Sprintf("%d bytes (equal to memory)", v.MemoryBytes),
			Actual:      fmt.Sprintf("%d bytes", v.MemoryReservationBytes),
			Severity:    "error",
			Correctable: true,
		})
	}

	return result
}

// ComputeHash computes a SHA-256 hash of arbitrary data.
func ComputeHash(data []byte) string {
	h := sha256.New()
	h.Write(data)
	return hex.EncodeToString(h.Sum(nil))
}

// HashEvidence computes a SHA-256 hash of the evidence for chain integrity.
// Kept for backward compatibility with existing callers.
func HashEvidence(previousHash string, evidence map[string]interface{}) string {
	data, _ := json.Marshal(evidence)
	h := sha256.New()
	h.Write([]byte(previousHash))
	h.Write(data)
	return hex.EncodeToString(h.Sum(nil))
}

// ChainHash combines a previous hash with a record hash to produce a chained hash.
func ChainHash(prevHash, recordHash string) string {
	h := sha256.New()
	h.Write([]byte(prevHash))
	h.Write([]byte(recordHash))
	return hex.EncodeToString(h.Sum(nil))
}

// EvidenceRecord represents a single compliance evidence entry with hash-chain integrity.
type EvidenceRecord struct {
	VMID         string          `json:"vm_id"`
	CheckID      string          `json:"check_id"`
	PreviousHash string          `json:"previous_hash"`
	CurrentHash  string          `json:"current_hash"`
	Profile      json.RawMessage `json:"profile"` // VMProfile as JSON
	Violations   []Violation     `json:"violations"`
	Passed       bool            `json:"passed"`
	Timestamp    time.Time       `json:"timestamp"`
	CheckedBy    string          `json:"checked_by"`
}

// EvidenceChain represents a chronological chain of evidence records for a VM.
type EvidenceChain struct {
	VMID    string            `json:"vm_id"`
	Records []*EvidenceRecord `json:"records"`
}

// ComplianceStore handles storing and retrieving compliance evidence.
type ComplianceStore struct {
	db *db.DB
	mu sync.Mutex
}

// NewComplianceStore creates a new compliance evidence store.
func NewComplianceStore(database *db.DB) *ComplianceStore {
	return &ComplianceStore{db: database}
}

// RecordEvidence stores a compliance check result with hash-chain integrity.
func (s *ComplianceStore) RecordEvidence(ctx context.Context, tenantID, vmID string,
	checkType string, result ValidationResult, previousHash string) error {
	evidenceData, _ := json.Marshal(result.Evidence)
	checkData, _ := json.Marshal(result)

	hash := HashEvidence(previousHash, result.Evidence)

	_, err := s.db.ExecContext(ctx, `
        INSERT INTO compliance_evidence (tenant_id, vm_id, check_type, check_result, passed, evidence, previous_hash, hash)
        VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
    `, tenantID, vmID, checkType, checkData, result.Passed, evidenceData, previousHash, hash)
	return err
}

// GetEvidence retrieves hash-chained compliance evidence for a VM.
func (s *ComplianceStore) GetEvidence(ctx context.Context, tenantID, vmID string) ([]db.ComplianceEvidence, error) {
	rows, err := s.db.QueryContext(ctx, `
        SELECT id, tenant_id, vm_id, check_type, check_result, passed, evidence, previous_hash, hash, created_at
        FROM compliance_evidence
        WHERE tenant_id = $1 AND vm_id = $2
        ORDER BY created_at ASC
    `, tenantID, vmID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []db.ComplianceEvidence
	for rows.Next() {
		var e db.ComplianceEvidence
		err := rows.Scan(&e.ID, &e.TenantID, &e.VMID, &e.CheckType, &e.CheckResult,
			&e.Passed, &e.Evidence, &e.PreviousHash, &e.Hash, &e.CreatedAt)
		if err != nil {
			return nil, err
		}
		results = append(results, e)
	}
	return results, rows.Err()
}

// ValidateAndRecord checks a HANA VM profile and records the evidence.
func (s *ComplianceStore) ValidateAndRecord(ctx context.Context, tenantID, vmID string,
	profile *VMProfile) (ValidationResult, error) {
	result := ValidateHANAProfile(profile)
	result.VMID = vmID
	result.CheckedAt = ""

	// Get the latest evidence hash for this VM to chain from
	evidence, _ := s.GetEvidence(ctx, tenantID, vmID)
	previousHash := ""
	if len(evidence) > 0 {
		previousHash = evidence[len(evidence)-1].Hash
	}

	if err := s.RecordEvidence(ctx, tenantID, vmID, "hana_guardrails", result, previousHash); err != nil {
		return result, fmt.Errorf("record compliance evidence: %w", err)
	}

	return result, nil
}

// DriftReport represents compliance drift across all HANA VMs in a tenant.
type DriftReport struct {
	TenantID     string    `json:"tenant_id"`
	TotalVMs     int       `json:"total_vms"`
	Compliant    int       `json:"compliant"`
	NonCompliant int       `json:"non_compliant"`
	VMs          []DriftVM `json:"vms"`
	GeneratedAt  string    `json:"generated_at"`
}

// DriftVM represents a single VM's drift status.
type DriftVM struct {
	VMID       string      `json:"vm_id"`
	Name       string      `json:"name"`
	Status     string      `json:"status"`
	LastCheck  string      `json:"last_check"`
	Compliant  bool        `json:"compliant"`
	Violations []Violation `json:"violations"`
}

// GenerateDriftReport generates a compliance drift report for all HANA VMs
// in a tenant.
func (s *ComplianceStore) GenerateDriftReport(ctx context.Context, tenantID string) (DriftReport, error) {
	// Get all HANA VMs
	rows, err := s.db.QueryContext(ctx, `
        SELECT id, name, status, role, cpus, cpu_allocation, memory_bytes,
               numa_policy, hugepages_enabled, cpu_pinning, memory_reservation_bytes,
               ballooning_allowed, swap_allowed
        FROM vm
        WHERE tenant_id = $1 AND role = 'hana'
    `, tenantID)
	if err != nil {
		return DriftReport{}, err
	}
	defer rows.Close()

	report := DriftReport{
		TenantID:     tenantID,
		TotalVMs:     0,
		Compliant:    0,
		NonCompliant: 0,
		VMs:          []DriftVM{},
		GeneratedAt:  "",
	}

	for rows.Next() {
		var vmID, name, status, role, cpuAlloc, numaPolicy string
		var cpus int
		var memoryBytes, memoryReservationBytes int64
		var hugepagesEnabled, ballooningAllowed, swapAllowed bool
		var cpuPinning []byte

		if err := rows.Scan(&vmID, &name, &status, &role, &cpus, &cpuAlloc, &memoryBytes,
			&numaPolicy, &hugepagesEnabled, &cpuPinning, &memoryReservationBytes,
			&ballooningAllowed, &swapAllowed); err != nil {
			return DriftReport{}, err
		}

		profile := &VMProfile{
			Role:                   role,
			CPUs:                   cpus,
			CPUAllocation:          cpuAlloc,
			MemoryBytes:            memoryBytes,
			NUMAPolicy:             nil,
			HugepagesEnabled:       hugepagesEnabled,
			CPUPinning:             cpuPinning,
			MemoryReservationBytes: memoryReservationBytes,
			BallooningAllowed:      ballooningAllowed,
			SwapAllowed:            swapAllowed,
		}
		if numaPolicy != "" {
			profile.NUMAPolicy = &numaPolicy
		}

		result := ValidateHANAProfile(profile)
		report.TotalVMs++
		if result.Passed {
			report.Compliant++
		} else {
			report.NonCompliant++
		}

		report.VMs = append(report.VMs, DriftVM{
			VMID:       vmID,
			Name:       name,
			Status:     status,
			LastCheck:  "",
			Compliant:  result.Passed,
			Violations: result.Violations,
		})
	}

	return report, rows.Err()
}

// GenerateEvidence validates a VM profile, creates an evidence record, and stores it
// in the compliance_evidence table with hash-chain integrity.
func (s *ComplianceStore) GenerateEvidence(ctx context.Context, vmID string, profile *VMProfile) (*EvidenceRecord, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	result := ValidateHANAProfile(profile)
	result.VMID = vmID
	result.CheckedAt = time.Now().UTC().Format(time.RFC3339)

	// Get the latest evidence hash for this VM to chain from
	existing, err := s.GetEvidence(ctx, "", vmID)
	if err != nil {
		return nil, fmt.Errorf("get existing evidence: %w", err)
	}

	previousHash := ""
	if len(existing) > 0 {
		previousHash = existing[len(existing)-1].Hash
	}

	// Build the evidence record
	profileJSON, err := json.Marshal(profile)
	if err != nil {
		return nil, fmt.Errorf("marshal profile: %w", err)
	}

	checkID := fmt.Sprintf("check-%s-%d", vmID, time.Now().UnixNano())

	record := &EvidenceRecord{
		VMID:         vmID,
		CheckID:      checkID,
		PreviousHash: previousHash,
		Profile:      profileJSON,
		Violations:   result.Violations,
		Passed:       result.Passed,
		Timestamp:    time.Now().UTC(),
		CheckedBy:    "hana-guardrails",
	}

	// Compute current hash: hash of (previousHash + profileJSON + violations + passed + timestamp + checkedBy)
	recordData := fmt.Sprintf("%s%s%d%d%s%s",
		previousHash,
		string(profileJSON),
		len(result.Violations),
		boolToInt(result.Passed),
		record.Timestamp.Format(time.RFC3339),
		record.CheckedBy,
	)
	record.CurrentHash = ComputeHash([]byte(recordData))

	// Store in DB via compliance_evidence table
	checkResultJSON, _ := json.Marshal(result)
	evidenceJSON, _ := json.Marshal(map[string]interface{}{
		"profile":    profileJSON,
		"violations": result.Violations,
		"check_id":   checkID,
		"checked_by": record.CheckedBy,
	})

	_, err = s.db.ExecContext(ctx, `
        INSERT INTO compliance_evidence (tenant_id, vm_id, check_type, check_result, passed, evidence, previous_hash, hash, created_at)
        VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
    `, "", vmID, "hana_guardrails", checkResultJSON, result.Passed, evidenceJSON, previousHash, record.CurrentHash, record.Timestamp)
	if err != nil {
		return nil, fmt.Errorf("store compliance evidence: %w", err)
	}

	return record, nil
}

// GetEvidenceChain returns the evidence chain as an EvidenceChain object.
func (s *ComplianceStore) GetEvidenceChain(ctx context.Context, vmID string) (*EvidenceChain, error) {
	evidence, err := s.GetEvidence(ctx, "", vmID)
	if err != nil {
		return nil, err
	}

	chain := &EvidenceChain{
		VMID:    vmID,
		Records: make([]*EvidenceRecord, 0, len(evidence)),
	}

	for _, e := range evidence {
		var profile json.RawMessage
		var violations []Violation
		var checkID string
		var checkedBy string

		if len(e.Evidence) > 0 {
			var evidenceMap map[string]interface{}
			if err := json.Unmarshal(e.Evidence, &evidenceMap); err == nil {
				if p, ok := evidenceMap["profile"]; ok {
					if raw, ok := p.(json.RawMessage); ok {
						profile = raw
					} else {
						b, _ := json.Marshal(p)
						profile = b
					}
				}
				if v, ok := evidenceMap["violations"]; ok {
					if arr, ok := v.([]interface{}); ok {
						for _, item := range arr {
							if m, ok := item.(map[string]interface{}); ok {
								violations = append(violations, Violation{
									Rule:        fmt.Sprint(m["rule"]),
									Field:       fmt.Sprint(m["field"]),
									Expected:    fmt.Sprint(m["expected"]),
									Actual:      fmt.Sprint(m["actual"]),
									Severity:    fmt.Sprint(m["severity"]),
									Correctable: m["correctable"] == true,
								})
							}
						}
					}
				}
				if cid, ok := evidenceMap["check_id"]; ok {
					checkID = fmt.Sprint(cid)
				}
				if cb, ok := evidenceMap["checked_by"]; ok {
					checkedBy = fmt.Sprint(cb)
				}
			}
		}

		record := &EvidenceRecord{
			VMID:         e.VMID,
			CheckID:      checkID,
			PreviousHash: e.PreviousHash,
			CurrentHash:  e.Hash,
			Profile:      profile,
			Violations:   violations,
			Passed:       e.Passed,
			Timestamp:    e.CreatedAt,
			CheckedBy:    checkedBy,
		}
		chain.Records = append(chain.Records, record)
	}

	return chain, nil
}

// CheckDrift compares a current profile against the last stored compliant profile.
// If drift is detected, a ComplianceDriftDetected event is published.
func (s *ComplianceStore) CheckDrift(ctx context.Context, vmID string, currentProfile *VMProfile) (*ValidationResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Get the latest evidence for this VM
	evidence, err := s.GetEvidence(ctx, "", vmID)
	if err != nil {
		return nil, fmt.Errorf("get evidence for drift check: %w", err)
	}

	result := ValidateHANAProfile(currentProfile)
	result.VMID = vmID
	result.CheckedAt = time.Now().UTC().Format(time.RFC3339)

	// If no previous evidence, treat as new baseline — no drift
	if len(evidence) == 0 {
		result.Evidence = map[string]interface{}{
			"drift_detected": false,
			"reason":         "no previous evidence found — new baseline",
		}
		return &result, nil
	}

	// Find the last passed evidence
	var lastPassed *db.ComplianceEvidence
	for i := len(evidence) - 1; i >= 0; i-- {
		if evidence[i].Passed {
			lastPassed = &evidence[i]
			break
		}
	}

	// If no passed evidence exists, compare against the most recent
	compareTarget := lastPassed
	if compareTarget == nil {
		compareTarget = &evidence[len(evidence)-1]
	}

	// Parse the stored profile from evidence
	var storedProfile VMProfile
	if len(compareTarget.Evidence) > 0 {
		var evidenceMap map[string]interface{}
		if err := json.Unmarshal(compareTarget.Evidence, &evidenceMap); err == nil {
			if p, ok := evidenceMap["profile"]; ok {
				if raw, ok := p.(json.RawMessage); ok {
					json.Unmarshal(raw, &storedProfile)
				} else {
					b, _ := json.Marshal(p)
					json.Unmarshal(b, &storedProfile)
				}
			}
		}
	}

	// Compare current vs stored profile
	driftDetected := false
	var driftDetails []map[string]interface{}

	if currentProfile.CPUs != storedProfile.CPUs {
		driftDetected = true
		driftDetails = append(driftDetails, map[string]interface{}{
			"field":    "cpus",
			"expected": storedProfile.CPUs,
			"actual":   currentProfile.CPUs,
			"severity": "warning",
		})
	}
	if currentProfile.MemoryBytes != storedProfile.MemoryBytes {
		driftDetected = true
		driftDetails = append(driftDetails, map[string]interface{}{
			"field":    "memory_bytes",
			"expected": storedProfile.MemoryBytes,
			"actual":   currentProfile.MemoryBytes,
			"severity": "warning",
		})
	}
	if currentProfile.CPUAllocation != storedProfile.CPUAllocation {
		driftDetected = true
		driftDetails = append(driftDetails, map[string]interface{}{
			"field":    "cpu_allocation",
			"expected": storedProfile.CPUAllocation,
			"actual":   currentProfile.CPUAllocation,
			"severity": "error",
		})
	}
	if currentProfile.HugepagesEnabled != storedProfile.HugepagesEnabled {
		driftDetected = true
		driftDetails = append(driftDetails, map[string]interface{}{
			"field":    "hugepages_enabled",
			"expected": storedProfile.HugepagesEnabled,
			"actual":   currentProfile.HugepagesEnabled,
			"severity": "error",
		})
	}
	if currentProfile.BallooningAllowed != storedProfile.BallooningAllowed {
		driftDetected = true
		driftDetails = append(driftDetails, map[string]interface{}{
			"field":    "ballooning_allowed",
			"expected": storedProfile.BallooningAllowed,
			"actual":   currentProfile.BallooningAllowed,
			"severity": "error",
		})
	}
	if currentProfile.SwapAllowed != storedProfile.SwapAllowed {
		driftDetected = true
		driftDetails = append(driftDetails, map[string]interface{}{
			"field":    "swap_allowed",
			"expected": storedProfile.SwapAllowed,
			"actual":   currentProfile.SwapAllowed,
			"severity": "error",
		})
	}

	if driftDetected {
		result.Evidence = map[string]interface{}{
			"drift_detected": true,
			"drift_details":  driftDetails,
			"baseline_check": compareTarget.ID,
		}

		// Publish ComplianceDriftDetected event
		driftEventJSON, _ := json.Marshal(map[string]interface{}{
			"vm_id":         vmID,
			"drift_details": driftDetails,
			"baseline_hash": compareTarget.Hash,
		})
		_, err = s.db.CreateEvent(ctx, &db.Event{
			TenantID:     "",
			Type:         "compliance_drift_detected",
			Severity:     "warning",
			Message:      fmt.Sprintf("Compliance drift detected for VM %s", vmID),
			ActorType:    "compliance",
			ActorID:      "hana-guardrails",
			ActorName:    "HANA Guardrails",
			ResourceType: "vm",
			ResourceID:   vmID,
			ResourceName: vmID,
			Metadata:     driftEventJSON,
		})
		if err != nil {
			return &result, fmt.Errorf("publish drift event: %w", err)
		}
	} else {
		result.Evidence = map[string]interface{}{
			"drift_detected": false,
			"reason":         "profile matches last compliant baseline",
		}
	}

	return &result, nil
}

// VerifyChain verifies the hash chain integrity for a VM's evidence records.
// Returns true if the chain is valid (each record's CurrentHash matches the next
// record's PreviousHash), false otherwise.
func (s *ComplianceStore) VerifyChain(ctx context.Context, vmID string) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	evidence, err := s.GetEvidence(ctx, "", vmID)
	if err != nil {
		return false, fmt.Errorf("get evidence for chain verification: %w", err)
	}

	if len(evidence) == 0 {
		return true, nil // empty chain is valid
	}

	for i := 1; i < len(evidence); i++ {
		prev := evidence[i-1]
		curr := evidence[i]

		// The next record's PreviousHash should equal the previous record's Hash
		if curr.PreviousHash != prev.Hash {
			return false, nil
		}
	}

	return true, nil
}

// TamperDetail describes a detected break in the evidence hash chain.
type TamperDetail struct {
	RecordIndex   int    `json:"record_index"`    // index in chain where break occurs
	RecordID      string `json:"record_id"`       // the evidence row ID with bad previous_hash
	ExpectedHash  string `json:"expected_hash"`   // what previous_hash should be (prior row's hash)
	ActualHash    string `json:"actual_hash"`     // the corrupted previous_hash value
	PriorRecordID string `json:"prior_record_id"` // the ID of the preceding record
}

// EvidenceChainVerificationResult holds the outcome of VerifyEvidenceChain.
type EvidenceChainVerificationResult struct {
	VMID         string        `json:"vm_id"`
	Valid        bool          `json:"valid"`
	ChainLength  int           `json:"chain_length"`
	TamperDetail *TamperDetail `json:"tamper_detail,omitempty"`
}

// VerifyEvidenceChain performs detailed hash-chain verification for a VM's
// compliance evidence. Unlike VerifyChain (which returns a bool), this function
// returns detailed tamper information including which record index is broken,
// the expected hash vs the actual (tampered) hash, and the IDs involved.
func (s *ComplianceStore) VerifyEvidenceChain(ctx context.Context, vmID string) (*EvidenceChainVerificationResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	evidence, err := s.GetEvidence(ctx, "", vmID)
	if err != nil {
		return nil, fmt.Errorf("get evidence for chain verification: %w", err)
	}

	result := &EvidenceChainVerificationResult{
		VMID:        vmID,
		Valid:       true,
		ChainLength: len(evidence),
	}

	if len(evidence) == 0 {
		return result, nil
	}

	for i := 1; i < len(evidence); i++ {
		prev := evidence[i-1]
		curr := evidence[i]

		if curr.PreviousHash != prev.Hash {
			result.Valid = false
			result.TamperDetail = &TamperDetail{
				RecordIndex:   i,
				RecordID:      curr.ID,
				ExpectedHash:  prev.Hash,
				ActualHash:    curr.PreviousHash,
				PriorRecordID: prev.ID,
			}
			return result, nil
		}
	}

	return result, nil
}

// InsertRawEvidence inserts a compliance evidence record with explicit hashes.
// This is intended for testing scenarios (e.g., simulating tampered records)
// where the caller needs full control over previous_hash and hash values.
func (s *ComplianceStore) InsertRawEvidence(ctx context.Context, tenantID, vmID, checkType string,
	passed bool, evidence []byte, previousHash, hash string, createdAt time.Time) error {
	_, err := s.db.ExecContext(ctx, `
        INSERT INTO compliance_evidence (tenant_id, vm_id, check_type, check_result, passed, evidence, previous_hash, hash, created_at)
        VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
    `, tenantID, vmID, checkType, []byte("{}"), passed, evidence, previousHash, hash, createdAt)
	return err
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

// DriftCheckResult holds the result of a pure drift check.
type DriftCheckResult struct {
	DriftDetected bool          `json:"drift_detected"`
	DriftDetails  []DriftDetail `json:"drift_details,omitempty"`
	BaselineCheck string        `json:"baseline_check,omitempty"`
	Reason        string        `json:"reason,omitempty"`
}

// DriftDetail describes a single field that drifted.
type DriftDetail struct {
	Field    string      `json:"field"`
	Expected interface{} `json:"expected"`
	Actual   interface{} `json:"actual"`
	Severity string      `json:"severity"`
}

// CheckDriftPure compares a current profile against a stored baseline profile
// and returns whether drift was detected. This is a pure function that does
// not require a database — it operates on the profiles directly.
func CheckDriftPure(baseline *VMProfile, current *VMProfile) DriftCheckResult {
	result := DriftCheckResult{
		DriftDetected: false,
		DriftDetails:  []DriftDetail{},
	}

	if baseline == nil {
		result.Reason = "no baseline profile — new baseline"
		return result
	}

	if current == nil {
		result.DriftDetected = true
		result.DriftDetails = append(result.DriftDetails, DriftDetail{
			Field:    "profile",
			Expected: "non-nil",
			Actual:   "nil",
			Severity: "error",
		})
		return result
	}

	// Compare fields
	if current.CPUs != baseline.CPUs {
		result.DriftDetected = true
		result.DriftDetails = append(result.DriftDetails, DriftDetail{
			Field:    "cpus",
			Expected: baseline.CPUs,
			Actual:   current.CPUs,
			Severity: "warning",
		})
	}
	if current.MemoryBytes != baseline.MemoryBytes {
		result.DriftDetected = true
		result.DriftDetails = append(result.DriftDetails, DriftDetail{
			Field:    "memory_bytes",
			Expected: baseline.MemoryBytes,
			Actual:   current.MemoryBytes,
			Severity: "warning",
		})
	}
	if current.CPUAllocation != baseline.CPUAllocation {
		result.DriftDetected = true
		result.DriftDetails = append(result.DriftDetails, DriftDetail{
			Field:    "cpu_allocation",
			Expected: baseline.CPUAllocation,
			Actual:   current.CPUAllocation,
			Severity: "error",
		})
	}
	if current.HugepagesEnabled != baseline.HugepagesEnabled {
		result.DriftDetected = true
		result.DriftDetails = append(result.DriftDetails, DriftDetail{
			Field:    "hugepages_enabled",
			Expected: baseline.HugepagesEnabled,
			Actual:   current.HugepagesEnabled,
			Severity: "error",
		})
	}
	if current.BallooningAllowed != baseline.BallooningAllowed {
		result.DriftDetected = true
		result.DriftDetails = append(result.DriftDetails, DriftDetail{
			Field:    "ballooning_allowed",
			Expected: baseline.BallooningAllowed,
			Actual:   current.BallooningAllowed,
			Severity: "error",
		})
	}
	if current.SwapAllowed != baseline.SwapAllowed {
		result.DriftDetected = true
		result.DriftDetails = append(result.DriftDetails, DriftDetail{
			Field:    "swap_allowed",
			Expected: baseline.SwapAllowed,
			Actual:   current.SwapAllowed,
			Severity: "error",
		})
	}

	if !result.DriftDetected {
		result.Reason = "profile matches baseline"
	}

	return result
}

// VerifyChainPure verifies the hash chain integrity for a slice of evidence
// records represented as simple structs. Returns detailed tamper information.
func VerifyChainPure(records []EvidenceRecord) *EvidenceChainVerificationResult {
	result := &EvidenceChainVerificationResult{
		Valid:       true,
		ChainLength: len(records),
	}

	if len(records) == 0 {
		return result
	}

	for i := 1; i < len(records); i++ {
		prev := records[i-1]
		curr := records[i]

		if curr.PreviousHash != prev.CurrentHash {
			result.Valid = false
			result.TamperDetail = &TamperDetail{
				RecordIndex:   i,
				RecordID:      curr.CheckID,
				ExpectedHash:  prev.CurrentHash,
				ActualHash:    curr.PreviousHash,
				PriorRecordID: prev.CheckID,
			}
			return result
		}
	}

	return result
}

// ComputeRecordHash computes the hash for an evidence record given the
// previous hash. This is the same logic used in GenerateEvidence.
func ComputeRecordHash(previousHash string, profile *VMProfile, violations []Violation, passed bool, checkedBy string, timestamp time.Time) string {
	profileJSON, _ := json.Marshal(profile)
	recordData := fmt.Sprintf("%s%s%d%d%s%s",
		previousHash,
		string(profileJSON),
		len(violations),
		boolToInt(passed),
		timestamp.Format(time.RFC3339),
		checkedBy,
	)
	return ComputeHash([]byte(recordData))
}
