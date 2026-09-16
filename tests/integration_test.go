// Package tests provides integration and unit tests for HiveStack.
package tests

import (
    "os"
    "strings"
    "testing"

    "github.com/maddydevel/HiveStack/internal/auth"
    "github.com/maddydevel/HiveStack/internal/compliance"
)

// TestHashPassword verifies Argon2id password hashing and verification.
func TestHashPassword(t *testing.T) {
    password := "SuperSecret123!"
    hash, err := auth.HashPassword(password)
    if err != nil {
        t.Fatalf("HashPassword failed: %v", err)
    }
    if !strings.HasPrefix(hash, "$argon2id$") {
        t.Errorf("Expected argon2id hash, got: %s", hash[:20])
    }

    valid, err := auth.CheckPassword(hash, password)
    if err != nil {
        t.Fatalf("CheckPassword failed: %v", err)
    }
    if !valid {
        t.Error("CheckPassword returned false for correct password")
    }

    invalid, err := auth.CheckPassword(hash, "WrongPassword")
    if err != nil {
        t.Fatalf("CheckPassword failed: %v", err)
    }
    if invalid {
        t.Error("CheckPassword returned true for wrong password")
    }
}

// TestHashPasswordEmpty tests empty password handling.
func TestHashPasswordEmpty(t *testing.T) {
    hash, err := auth.HashPassword("")
    if err != nil {
        t.Logf("HashPassword with empty password returned error: %v", err)
        return
    }
    if !strings.HasPrefix(hash, "$argon2id$") {
        t.Errorf("Expected argon2id hash for empty password")
    }

    valid, err := auth.CheckPassword(hash, "")
    if err != nil {
        t.Fatalf("CheckPassword with empty password failed: %v", err)
    }
    if !valid {
        t.Error("Empty password verification failed")
    }
}

// TestHashPasswordDifferentSalt verifies each hash uses a unique salt.
func TestHashPasswordDifferentSalt(t *testing.T) {
    pw := "testpassword"
    h1, err1 := auth.HashPassword(pw)
    h2, err2 := auth.HashPassword(pw)

    if err1 != nil || err2 != nil {
        t.Fatalf("HashPassword failed: h1=%v, h2=%v", err1, err2)
    }
    if h1 == h2 {
        t.Error("Two hashes of the same password should differ (unique salts)")
    }

    v1, err1 := auth.CheckPassword(h1, pw)
    v2, err2 := auth.CheckPassword(h2, pw)
    if err1 != nil || err2 != nil {
        t.Fatalf("CheckPassword failed: v1 err=%v, v2 err=%v", err1, err2)
    }
    if !v1 || !v2 {
        t.Error("Both hashes should verify against the same password")
    }
}

// TestJWTGeneration tests JWT token generation and validation.
func TestJWTGeneration(t *testing.T) {
    os.Setenv("HIVESTACK_JWT_SECRET", "test-secret-for-unit-tests")
    defer os.Unsetenv("HIVESTACK_JWT_SECRET")

    token, err := auth.GenerateToken("user-123", "tenant-456", []string{"viewer", "operator"}, 0)
    if err != nil {
        t.Fatalf("GenerateToken failed: %v", err)
    }
    if token == "" {
        t.Fatal("GenerateToken returned empty token")
    }

    claims, err := auth.ValidateToken(token)
    if err != nil {
        t.Fatalf("ValidateToken failed: %v", err)
    }
    if claims.UserID != "user-123" {
        t.Errorf("Expected userID 'user-123', got '%s'", claims.UserID)
    }
    if claims.TenantID != "tenant-456" {
        t.Errorf("Expected tenantID 'tenant-456', got '%s'", claims.TenantID)
    }
    if len(claims.Scopes) != 2 {
        t.Errorf("Expected 2 scopes, got %d", len(claims.Scopes))
    }

    _, err = auth.ValidateToken("invalid.token.here")
    if err == nil {
        t.Error("Expected error for invalid token")
    }
}

// TestRBACPermissions tests RBAC permission checks.
func TestRBACPermissions(t *testing.T) {
    engine := auth.NewRBACEngine()

    tests := []struct {
        role       string
        resource   string
        action     string
        wantPerm   bool
    }{
        {"admin", "vm", "create", true},
        {"admin", "vm", "delete", true},
        {"admin", "host", "list", true},
        {"admin", "compliance", "validate", true},
        {"operator", "vm", "create", true},
        {"operator", "vm", "delete", true},
        {"operator", "compliance", "validate", true},
        {"viewer", "vm", "list", true},
        {"viewer", "vm", "create", false},
        {"viewer", "host", "list", true},
        {"viewer", "host", "delete", false},
        {"hana-operator", "vm", "create", true},
        {"hana-operator", "compliance", "validate", true},
        {"compliance-auditor", "compliance", "evidence", true},
        {"compliance-auditor", "vm", "create", false},
        {"nonexistent", "vm", "list", false},
    }

    for _, tt := range tests {
        ok := engine.HasPermission(tt.role, tt.resource, tt.action)
        if ok != tt.wantPerm {
            t.Errorf("HasPermission(%s, %s, %s) = %v, want %v",
                tt.role, tt.resource, tt.action, ok, tt.wantPerm)
        }
    }
}

// TestRBACCheckPermission tests CheckPermission error returns.
func TestRBACCheckPermission(t *testing.T) {
    engine := auth.NewRBACEngine()

    if err := engine.CheckPermission("admin", "vm", "create"); err != nil {
        t.Errorf("CheckPermission for allowed action failed: %v", err)
    }

    err := engine.CheckPermission("viewer", "vm", "create")
    if err == nil {
        t.Error("Expected error for denied action")
    }
    if !strings.Contains(err.Error(), "forbidden") {
        t.Errorf("Expected 'forbidden' error, got: %v", err)
    }
}

// TestValidRole tests IsValidRole.
func TestValidRole(t *testing.T) {
    valid := []string{"admin", "operator", "viewer", "hana-operator", "compliance-auditor"}
    invalid := []string{"", "superuser", "root", "guest", "moderator"}

    for _, r := range valid {
        if !auth.IsValidRole(r) {
            t.Errorf("IsValidRole(%q) = false, want true", r)
        }
    }
    for _, r := range invalid {
        if auth.IsValidRole(r) {
            t.Errorf("IsValidRole(%q) = true, want false", r)
        }
    }
}

// TestHANAValidation tests the HANA compliance guardrails.
func TestHANAValidation(t *testing.T) {
    tests := []struct {
        name     string
        profile  *compliance.VMProfile
        wantPass bool
    }{
        {
            name: "valid HANA VM",
            profile: &compliance.VMProfile{
                Role:                "hana",
                CPUs:                8,
                CPUAllocation:       "dedicated",
                MemoryBytes:         64 * 1024 * 1024 * 1024,
                NUMAPolicy:          strPtr("centered"),
                HugepagesEnabled:    true,
                CPUPinning:          []byte(`{"vcpu0":"0","vcpu1":"1"}`),
                MemoryReservationBytes: 64 * 1024 * 1024 * 1024,
                BallooningAllowed:   false,
                SwapAllowed:         false,
            },
            wantPass: true,
        },
        {
            name: "HANA missing NUMA",
            profile: &compliance.VMProfile{
                Role:                "hana",
                CPUs:                8,
                CPUAllocation:       "dedicated",
                MemoryBytes:         64 * 1024 * 1024 * 1024,
                NUMAPolicy:          nil,
                HugepagesEnabled:    true,
                CPUPinning:          []byte(`{}`),
                MemoryReservationBytes: 64 * 1024 * 1024 * 1024,
                BallooningAllowed:   false,
                SwapAllowed:         false,
            },
            wantPass: false,
        },
        {
            name: "HANA missing hugepages",
            profile: &compliance.VMProfile{
                Role:                "hana",
                CPUs:                8,
                CPUAllocation:       "dedicated",
                MemoryBytes:         64 * 1024 * 1024 * 1024,
                NUMAPolicy:          strPtr("centered"),
                HugepagesEnabled:    false,
                CPUPinning:          []byte(`{}`),
                MemoryReservationBytes: 64 * 1024 * 1024 * 1024,
                BallooningAllowed:   false,
                SwapAllowed:         false,
            },
            wantPass: false,
        },
        {
            name: "HANA ballooning allowed",
            profile: &compliance.VMProfile{
                Role:                "hana",
                CPUs:                8,
                CPUAllocation:       "dedicated",
                MemoryBytes:         64 * 1024 * 1024 * 1024,
                NUMAPolicy:          strPtr("centered"),
                HugepagesEnabled:    true,
                CPUPinning:          []byte(`{}`),
                MemoryReservationBytes: 64 * 1024 * 1024 * 1024,
                BallooningAllowed:   true,
                SwapAllowed:         false,
            },
            wantPass: false,
        },
        {
            name: "HANA swap allowed",
            profile: &compliance.VMProfile{
                Role:                "hana",
                CPUs:                8,
                CPUAllocation:       "dedicated",
                MemoryBytes:         64 * 1024 * 1024 * 1024,
                NUMAPolicy:          strPtr("centered"),
                HugepagesEnabled:    true,
                CPUPinning:          []byte(`{}`),
                MemoryReservationBytes: 64 * 1024 * 1024 * 1024,
                BallooningAllowed:   false,
                SwapAllowed:         true,
            },
            wantPass: false,
        },
        {
            name: "HANA overcommitted memory",
            profile: &compliance.VMProfile{
                Role:                "hana",
                CPUs:                8,
                CPUAllocation:       "dedicated",
                MemoryBytes:         64 * 1024 * 1024 * 1024,
                NUMAPolicy:          strPtr("centered"),
                HugepagesEnabled:    true,
                CPUPinning:          []byte(`{}`),
                MemoryReservationBytes: 32 * 1024 * 1024 * 1024,
                BallooningAllowed:   false,
                SwapAllowed:         false,
            },
            wantPass: false,
        },
        {
            name: "generic VM (skip HANA checks)",
            profile: &compliance.VMProfile{
                Role:                "generic",
                CPUs:                4,
                CPUAllocation:       "shared",
                MemoryBytes:         8 * 1024 * 1024 * 1024,
                NUMAPolicy:          nil,
                HugepagesEnabled:    false,
                CPUPinning:          nil,
                MemoryReservationBytes: 0,
                BallooningAllowed:   true,
                SwapAllowed:         true,
            },
            wantPass: true,
        },
    }

    for _, tt := range tests {
        result := compliance.ValidateHANAProfile(tt.profile)
        if result.Passed != tt.wantPass {
            t.Errorf("Test %q: ValidateHANAProfile().Passed = %v, want %v",
                tt.name, result.Passed, tt.wantPass)
        }
    }
}

// TestComplianceEvidenceHash tests the evidence hash chaining.
func TestComplianceEvidenceHash(t *testing.T) {
    evidence := map[string]interface{}{
        "role":              "hana",
        "cpus":              8,
        "memory_bytes":      64 * 1024 * 1024 * 1024,
        "hugepages_enabled": true,
    }

    h1 := compliance.HashEvidence("genesis", evidence)
    if h1 == "" {
        t.Error("HashEvidence returned empty string")
    }

    h2 := compliance.HashEvidence("genesis", evidence)
    if h1 != h2 {
        t.Error("HashEvidence is not deterministic")
    }

    h3 := compliance.HashEvidence("different", evidence)
    if h1 == h3 {
        t.Error("Different previous hash should produce different result")
    }
}

// strPtr returns a pointer to a string.
func strPtr(s string) *string {
    return &s
}
