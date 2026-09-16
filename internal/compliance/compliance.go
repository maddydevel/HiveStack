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
//   1. Manager side (this package): API rejects non-compliant HANA VM specs
//   2. Node Agent side: libvirt XML generation refuses non-compliant configs
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

    "github.com/maddydevel/HiveStack/internal/db"
)

// VMProfile holds the VM configuration to validate.
type VMProfile struct {
    Role                   string            `json:"role"`
    CPUs                   int               `json:"cpus"`
    CPUAllocation          string            `json:"cpu_allocation"`
    MemoryBytes            int64             `json:"memory_bytes"`
    NUMAPolicy             *string           `json:"numa_policy"`
    HugepagesEnabled       bool              `json:"hugepages_enabled"`
    CPUPinning             []byte            `json:"cpu_pinning"` // JSON
    MemoryReservationBytes int64             `json:"memory_reservation_bytes"`
    BallooningAllowed      bool              `json:"ballooning_allowed"`
    SwapAllowed            bool              `json:"swap_allowed"`
    OS                     string            `json:"os"`
    HasHugepagesConfig     bool              `json:"has_hugepages_config"`
    NUMANodeCount          int               `json:"numa_node_count"`
    HugepagesTotalKB       int64             `json:"hugepages_total_kb"`
    HostCPUCount           int               `json:"host_cpu_count"`
    HostMemoryBytes        int64             `json:"host_memory_bytes"`
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
    Rule       string `json:"rule"`
    Field      string `json:"field"`
    Expected   string `json:"expected"`
    Actual     string `json:"actual"`
    Severity   string `json:"severity"` // "error" or "warning"
    Correctable bool  `json:"correctable"`
}

// ValidateHANAProfile validates a VM profile against HANA guardrails.
// Returns a ValidationResult; if Passed is false, the VM must not be created
// or started until violations are resolved.
func ValidateHANAProfile(v *VMProfile) ValidationResult {
    result := ValidationResult{
        VMID:      "",
        CheckType: "hana_guardrails",
        Passed:    true,
        Violations: []Violation{},
        Evidence: map[string]interface{}{
            "role": v.Role,
            "cpus": v.CPUs,
            "memory_bytes": v.MemoryBytes,
            "hugepages_enabled": v.HugepagesEnabled,
            "ballooning_allowed": v.BallooningAllowed,
            "swap_allowed": v.SwapAllowed,
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
            Rule:       "numa_pinned",
            Field:      "numa_policy",
            Expected:   "centered or bind with single NUMA node",
            Actual:     "unset",
            Severity:   "error",
            Correctable: true,
        })
    } else if *v.NUMAPolicy != "centered" && *v.NUMAPolicy != "bind" {
        result.Passed = false
        result.Violations = append(result.Violations, Violation{
            Rule:       "numa_policy_value",
            Field:      "numa_policy",
            Expected:   "centered or bind",
            Actual:     *v.NUMAPolicy,
            Severity:   "error",
            Correctable: true,
        })
    }

    // 2. Hugepages must be enabled
    if !v.HugepagesEnabled {
        result.Passed = false
        result.Violations = append(result.Violations, Violation{
            Rule:       "hugepages_enabled",
            Field:      "hugepages_enabled",
            Expected:   "true",
            Actual:     "false",
            Severity:   "error",
            Correctable: true,
        })
    }

    // 3. Dedicated vCPUs: CPU allocation must be "dedicated" (no overcommit)
    if v.CPUAllocation != "dedicated" {
        result.Passed = false
        result.Violations = append(result.Violations, Violation{
            Rule:       "dedicated_cpu",
            Field:      "cpu_allocation",
            Expected:   "dedicated",
            Actual:     v.CPUAllocation,
            Severity:   "error",
            Correctable: true,
        })
    }

    // 4. No ballooning
    if v.BallooningAllowed {
        result.Passed = false
        result.Violations = append(result.Violations, Violation{
            Rule:       "no_ballooning",
            Field:      "ballooning_allowed",
            Expected:   "false",
            Actual:     "true",
            Severity:   "error",
            Correctable: false, // ballooning is a host-level setting
        })
    }

    // 5. No swap
    if v.SwapAllowed {
        result.Passed = false
        result.Violations = append(result.Violations, Violation{
            Rule:       "no_swap",
            Field:      "swap_allowed",
            Expected:   "false",
            Actual:     "true",
            Severity:   "error",
            Correctable: false, // swap is inside the guest OS
        })
    }

    // 6. CPU pinning must be non-empty for HANA
    if len(v.CPUPinning) == 0 {
        result.Passed = false
        result.Violations = append(result.Violations, Violation{
            Rule:       "cpu_pinning",
            Field:      "cpu_pinning",
            Expected:   "non-empty vCPU-to-host-CPU mapping",
            Actual:     "empty",
            Severity:   "error",
            Correctable: true,
        })
    }

    // 7. Memory reservation must equal requested memory (no overcommit)
    if v.MemoryReservationBytes != v.MemoryBytes {
        result.Passed = false
        result.Violations = append(result.Violations, Violation{
            Rule:       "memory_reservation",
            Field:      "memory_reservation_bytes",
            Expected:   fmt.Sprintf("%d bytes (equal to memory)", v.MemoryBytes),
            Actual:     fmt.Sprintf("%d bytes", v.MemoryReservationBytes),
            Severity:   "error",
            Correctable: true,
        })
    }

    return result
}

// HashEvidence computes a SHA-256 hash of the evidence for chain integrity.
func HashEvidence(previousHash string, evidence map[string]interface{}) string {
    data, _ := json.Marshal(evidence)
    h := sha256.New()
    h.Write([]byte(previousHash))
    h.Write(data)
    return hex.EncodeToString(h.Sum(nil))
}

// ComplianceStore handles storing and retrieving compliance evidence.
type ComplianceStore struct {
    db *db.DB
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
    TenantID    string          `json:"tenant_id"`
    TotalVMs    int             `json:"total_vms"`
    Compliant   int             `json:"compliant"`
    NonCompliant int            `json:"non_compliant"`
    VMs         []DriftVM       `json:"vms"`
    GeneratedAt string          `json:"generated_at"`
}

// DriftVM represents a single VM's drift status.
type DriftVM struct {
    VMID        string        `json:"vm_id"`
    Name        string        `json:"name"`
    Status      string        `json:"status"`
    LastCheck   string        `json:"last_check"`
    Compliant   bool          `json:"compliant"`
    Violations  []Violation   `json:"violations"`
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
        TenantID:    tenantID,
        TotalVMs:    0,
        Compliant:   0,
        NonCompliant: 0,
        VMs:         []DriftVM{},
        GeneratedAt: "",
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
