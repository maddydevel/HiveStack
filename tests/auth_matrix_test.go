// Package tests provides RBAC matrix tests for HiveStack.
//
// This file tests the complete permission matrix for all 5 built-in roles
// against key operations: vm.create, vm.delete, vm.list, host.create, backup.delete.
package tests

import (
	"testing"

	"github.com/maddydevel/HiveStack/internal/auth"
)

// TestRBACMatrix verifies the complete permission matrix for all built-in roles.
// The matrix tests 5 roles × key operations to ensure correct access control.
func TestRBACMatrix(t *testing.T) {
	engine := auth.NewRBACEngine()

	// Define the complete permission matrix
	// Format: role -> resource.action -> expected result
	matrix := []struct {
		role     string
		resource string
		action   string
		expected bool
	}{
		// ─── Admin: full access to everything ─────────────────────────────────
		{"admin", "vm", "create", true},
		{"admin", "vm", "delete", true},
		{"admin", "vm", "list", true},
		{"admin", "vm", "get", true},
		{"admin", "vm", "update", true},
		{"admin", "vm", "start", true},
		{"admin", "vm", "stop", true},
		{"admin", "vm", "restart", true},
		{"admin", "vm", "migrate", true},
		{"admin", "vm", "console", true},
		{"admin", "vm", "snapshot", true},
		{"admin", "vm", "stats", true},
		{"admin", "host", "create", true},
		{"admin", "host", "delete", true},
		{"admin", "host", "list", true},
		{"admin", "host", "get", true},
		{"admin", "host", "update", true},
		{"admin", "host", "status", true},
		{"admin", "host", "maintenance", true},
		{"admin", "backup", "delete", true},
		{"admin", "backup", "create", true},
		{"admin", "backup", "list", true},
		{"admin", "backup", "restore", true},
		{"admin", "backup", "cancel", true},
		{"admin", "storage", "create", true},
		{"admin", "storage", "delete", true},
		{"admin", "network", "create", true},
		{"admin", "network", "delete", true},
		{"admin", "user", "create", true},
		{"admin", "user", "delete", true},
		{"admin", "user", "list", true},
		{"admin", "audit", "list", true},
		{"admin", "compliance", "view", true},
		{"admin", "compliance", "validate", true},
		{"admin", "event", "list", true},

		// ─── Operator: VM/host/storage/network/backup management, no user/audit ─
		{"operator", "vm", "create", true},
		{"operator", "vm", "delete", true},
		{"operator", "vm", "list", true},
		{"operator", "vm", "get", true},
		{"operator", "vm", "update", true},
		{"operator", "vm", "start", true},
		{"operator", "vm", "stop", true},
		{"operator", "vm", "restart", true},
		{"operator", "vm", "migrate", true},
		{"operator", "vm", "console", true},
		{"operator", "vm", "snapshot", true},
		{"operator", "vm", "stats", true},
		{"operator", "host", "create", true},
		{"operator", "host", "delete", true},
		{"operator", "host", "list", true},
		{"operator", "host", "get", true},
		{"operator", "host", "update", true},
		{"operator", "host", "status", true},
		{"operator", "host", "maintenance", true},
		{"operator", "backup", "delete", true},
		{"operator", "backup", "create", true},
		{"operator", "backup", "list", true},
		{"operator", "backup", "restore", true},
		{"operator", "backup", "cancel", true},
		{"operator", "storage", "create", true},
		{"operator", "storage", "delete", true},
		{"operator", "network", "create", true},
		{"operator", "network", "delete", true},
		{"operator", "user", "create", false}, // operator cannot manage users
		{"operator", "user", "delete", false},
		{"operator", "user", "list", false},
		{"operator", "audit", "list", false}, // operator cannot view audit
		{"operator", "compliance", "view", true},
		{"operator", "compliance", "validate", true},
		{"operator", "event", "list", true},

		// ─── Viewer: read-only access ─────────────────────────────────────────
		{"viewer", "vm", "create", false},
		{"viewer", "vm", "delete", false},
		{"viewer", "vm", "list", true},
		{"viewer", "vm", "get", true},
		{"viewer", "vm", "update", false},
		{"viewer", "vm", "start", false},
		{"viewer", "vm", "stop", false},
		{"viewer", "vm", "restart", false},
		{"viewer", "vm", "migrate", false},
		{"viewer", "vm", "console", false},
		{"viewer", "vm", "snapshot", false},
		{"viewer", "vm", "stats", true},
		{"viewer", "host", "create", false},
		{"viewer", "host", "delete", false},
		{"viewer", "host", "list", true},
		{"viewer", "host", "get", true},
		{"viewer", "host", "update", false},
		{"viewer", "host", "status", true},
		{"viewer", "host", "maintenance", false},
		{"viewer", "backup", "delete", false},
		{"viewer", "backup", "create", false},
		{"viewer", "backup", "list", true},
		{"viewer", "backup", "restore", false},
		{"viewer", "backup", "cancel", false},
		{"viewer", "storage", "create", false},
		{"viewer", "storage", "delete", false},
		{"viewer", "storage", "list", true},
		{"viewer", "storage", "get", true},
		{"viewer", "network", "create", false},
		{"viewer", "network", "delete", false},
		{"viewer", "network", "list", true},
		{"viewer", "network", "get", true},
		{"viewer", "user", "create", false},
		{"viewer", "user", "delete", false},
		{"viewer", "user", "list", false},
		{"viewer", "audit", "list", false},
		{"viewer", "compliance", "view", true},
		{"viewer", "compliance", "validate", false},
		{"viewer", "event", "list", true},

		// ─── HANA Operator: operator minus host.create ────────────────────────
		{"hana-operator", "vm", "create", true},
		{"hana-operator", "vm", "delete", true},
		{"hana-operator", "vm", "list", true},
		{"hana-operator", "vm", "get", true},
		{"hana-operator", "vm", "update", true},
		{"hana-operator", "vm", "start", true},
		{"hana-operator", "vm", "stop", true},
		{"hana-operator", "vm", "restart", true},
		{"hana-operator", "vm", "migrate", true},
		{"hana-operator", "vm", "console", true},
		{"hana-operator", "vm", "snapshot", true},
		{"hana-operator", "vm", "stats", true},
		{"hana-operator", "host", "create", false}, // hana-operator cannot create hosts
		{"hana-operator", "host", "delete", false},
		{"hana-operator", "host", "list", true},
		{"hana-operator", "host", "get", true},
		{"hana-operator", "host", "update", false},
		{"hana-operator", "host", "status", true},
		{"hana-operator", "host", "maintenance", false},
		{"hana-operator", "backup", "delete", true},
		{"hana-operator", "backup", "create", true},
		{"hana-operator", "backup", "list", true},
		{"hana-operator", "backup", "restore", true},
		{"hana-operator", "backup", "cancel", true},
		{"hana-operator", "storage", "create", false},
		{"hana-operator", "storage", "delete", false},
		{"hana-operator", "storage", "list", true},
		{"hana-operator", "storage", "get", true},
		{"hana-operator", "network", "create", false},
		{"hana-operator", "network", "delete", false},
		{"hana-operator", "network", "list", true},
		{"hana-operator", "network", "get", true},
		{"hana-operator", "user", "create", false},
		{"hana-operator", "user", "delete", false},
		{"hana-operator", "user", "list", false},
		{"hana-operator", "audit", "list", false},
		{"hana-operator", "compliance", "view", true},
		{"hana-operator", "compliance", "validate", true},
		{"hana-operator", "event", "list", true},

		// ─── Compliance Auditor: read-only compliance and audit ───────────────
		{"compliance-auditor", "vm", "create", false},
		{"compliance-auditor", "vm", "delete", false},
		{"compliance-auditor", "vm", "list", true},
		{"compliance-auditor", "vm", "get", true},
		{"compliance-auditor", "vm", "update", false},
		{"compliance-auditor", "vm", "start", false},
		{"compliance-auditor", "vm", "stop", false},
		{"compliance-auditor", "vm", "stats", true},
		{"compliance-auditor", "host", "create", false},
		{"compliance-auditor", "host", "delete", false},
		{"compliance-auditor", "host", "list", true},
		{"compliance-auditor", "host", "get", true},
		{"compliance-auditor", "host", "status", true},
		{"compliance-auditor", "backup", "delete", false},
		{"compliance-auditor", "backup", "create", false},
		{"compliance-auditor", "backup", "list", false},
		{"compliance-auditor", "storage", "create", false},
		{"compliance-auditor", "storage", "list", true},
		{"compliance-auditor", "storage", "get", true},
		{"compliance-auditor", "network", "create", false},
		{"compliance-auditor", "network", "list", true},
		{"compliance-auditor", "network", "get", true},
		{"compliance-auditor", "user", "create", false},
		{"compliance-auditor", "user", "delete", false},
		{"compliance-auditor", "user", "list", false},
		{"compliance-auditor", "audit", "list", true},
		{"compliance-auditor", "compliance", "view", true},
		{"compliance-auditor", "compliance", "evidence", true},
		{"compliance-auditor", "compliance", "drift", true},
		{"compliance-auditor", "compliance", "validate", false}, // auditor views but doesn't validate
		{"compliance-auditor", "event", "list", true},
	}

	for _, tt := range matrix {
		name := tt.role + "." + tt.resource + "." + tt.action
		t.Run(name, func(t *testing.T) {
			got := engine.HasPermission(tt.role, tt.resource, tt.action)
			if got != tt.expected {
				t.Errorf("HasPermission(%q, %q, %q) = %v, want %v",
					tt.role, tt.resource, tt.action, got, tt.expected)
			}
		})
	}
}

// TestRBACMatrix_CheckPermission verifies that CheckPermission returns
// the correct error for allowed and denied operations.
func TestRBACMatrix_CheckPermission(t *testing.T) {
	engine := auth.NewRBACEngine()

	tests := []struct {
		name      string
		role      string
		resource  string
		action    string
		expectErr bool
	}{
		// Admin can do everything
		{"admin_vm_create", "admin", "vm", "create", false},
		{"admin_vm_delete", "admin", "vm", "delete", false},
		{"admin_host_create", "admin", "host", "create", false},
		{"admin_backup_delete", "admin", "backup", "delete", false},

		// Operator can manage VMs and backups
		{"operator_vm_create", "operator", "vm", "create", false},
		{"operator_vm_delete", "operator", "vm", "delete", false},
		{"operator_host_create", "operator", "host", "create", false},
		{"operator_backup_delete", "operator", "backup", "delete", false},
		{"operator_user_create", "operator", "user", "create", true},
		{"operator_audit_list", "operator", "audit", "list", true},

		// Viewer is read-only
		{"viewer_vm_list", "viewer", "vm", "list", false},
		{"viewer_vm_create", "viewer", "vm", "create", true},
		{"viewer_vm_delete", "viewer", "vm", "delete", true},
		{"viewer_host_create", "viewer", "host", "create", true},
		{"viewer_backup_delete", "viewer", "backup", "delete", true},

		// HANA operator cannot create hosts
		{"hana_vm_create", "hana-operator", "vm", "create", false},
		{"hana_vm_delete", "hana-operator", "vm", "delete", false},
		{"hana_host_create", "hana-operator", "host", "create", true},
		{"hana_backup_delete", "hana-operator", "backup", "delete", false},

		// Compliance auditor is read-only
		{"auditor_vm_list", "compliance-auditor", "vm", "list", false},
		{"auditor_vm_create", "compliance-auditor", "vm", "create", true},
		{"auditor_vm_delete", "compliance-auditor", "vm", "delete", true},
		{"auditor_host_create", "compliance-auditor", "host", "create", true},
		{"auditor_backup_delete", "compliance-auditor", "backup", "delete", true},
		{"auditor_audit_list", "compliance-auditor", "audit", "list", false},
		{"auditor_compliance_view", "compliance-auditor", "compliance", "view", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := engine.CheckPermission(tt.role, tt.resource, tt.action)
			if tt.expectErr && err == nil {
				t.Errorf("CheckPermission(%q, %q, %q) expected error, got nil",
					tt.role, tt.resource, tt.action)
			}
			if !tt.expectErr && err != nil {
				t.Errorf("CheckPermission(%q, %q, %q) unexpected error: %v",
					tt.role, tt.resource, tt.action, err)
			}
		})
	}
}

// TestRBACMatrix_UnknownRoles verifies that unknown roles have no permissions.
func TestRBACMatrix_UnknownRoles(t *testing.T) {
	engine := auth.NewRBACEngine()

	unknownRoles := []string{"superadmin", "root", "", "ADMIN", "Admin", "viewer "}
	operations := []struct {
		resource string
		action   string
	}{
		{"vm", "create"},
		{"vm", "delete"},
		{"vm", "list"},
		{"host", "create"},
		{"backup", "delete"},
	}

	for _, role := range unknownRoles {
		for _, op := range operations {
			name := "unknown_" + role + "." + op.resource + "." + op.action
			t.Run(name, func(t *testing.T) {
				if engine.HasPermission(role, op.resource, op.action) {
					t.Errorf("unknown role %q should not have %s.%s permission",
						role, op.resource, op.action)
				}
			})
		}
	}
}

// TestRBACMatrix_KeyOperations tests the 5 key operations specifically mentioned
// in the requirements: vm.create, vm.delete, vm.list, host.create, backup.delete
func TestRBACMatrix_KeyOperations(t *testing.T) {
	engine := auth.NewRBACEngine()

	keyOps := []struct {
		resource string
		action   string
	}{
		{"vm", "create"},
		{"vm", "delete"},
		{"vm", "list"},
		{"host", "create"},
		{"backup", "delete"},
	}

	// Expected results for each role × key operation
	expected := map[string]map[string]bool{
		"admin": {
			"vm.create":     true,
			"vm.delete":     true,
			"vm.list":       true,
			"host.create":   true,
			"backup.delete": true,
		},
		"operator": {
			"vm.create":     true,
			"vm.delete":     true,
			"vm.list":       true,
			"host.create":   true,
			"backup.delete": true,
		},
		"viewer": {
			"vm.create":     false,
			"vm.delete":     false,
			"vm.list":       true,
			"host.create":   false,
			"backup.delete": false,
		},
		"hana-operator": {
			"vm.create":     true,
			"vm.delete":     true,
			"vm.list":       true,
			"host.create":   false,
			"backup.delete": true,
		},
		"compliance-auditor": {
			"vm.create":     false,
			"vm.delete":     false,
			"vm.list":       true,
			"host.create":   false,
			"backup.delete": false,
		},
	}

	for role, ops := range expected {
		for opKey, want := range ops {
			t.Run(role+"_"+opKey, func(t *testing.T) {
				// Find the resource/action for this key
				var resource, action string
				for _, k := range keyOps {
					if k.resource+"."+k.action == opKey {
						resource = k.resource
						action = k.action
						break
					}
				}
				got := engine.HasPermission(role, resource, action)
				if got != want {
					t.Errorf("HasPermission(%q, %q, %q) = %v, want %v",
						role, resource, action, got, want)
				}
			})
		}
	}
}

// TestRBACMatrix_RoleHierarchy verifies the permission hierarchy:
// admin > operator > viewer
// admin > hana-operator > viewer
func TestRBACMatrix_RoleHierarchy(t *testing.T) {
	engine := auth.NewRBACEngine()

	// All permissions that viewer has, operator and admin should also have
	viewerPerms := []struct {
		resource string
		action   string
	}{
		{"vm", "list"}, {"vm", "get"}, {"vm", "stats"},
		{"host", "list"}, {"host", "get"}, {"host", "status"},
		{"storage", "list"}, {"storage", "get"},
		{"network", "list"}, {"network", "get"},
		{"backup", "list"},
		{"compliance", "view"},
		{"event", "list"},
	}

	for _, p := range viewerPerms {
		t.Run("operator_has_viewer_"+p.resource+"_"+p.action, func(t *testing.T) {
			if !engine.HasPermission("operator", p.resource, p.action) {
				t.Errorf("operator should have %s.%s (inherited from viewer)", p.resource, p.action)
			}
		})
		t.Run("admin_has_viewer_"+p.resource+"_"+p.action, func(t *testing.T) {
			if !engine.HasPermission("admin", p.resource, p.action) {
				t.Errorf("admin should have %s.%s (inherited from viewer)", p.resource, p.action)
			}
		})
	}

	// Admin has permissions that operator doesn't (user management, audit)
	adminOnlyPerms := []struct {
		resource string
		action   string
	}{
		{"user", "list"}, {"user", "create"}, {"user", "delete"},
		{"audit", "list"},
	}

	for _, p := range adminOnlyPerms {
		t.Run("admin_only_"+p.resource+"_"+p.action, func(t *testing.T) {
			if !engine.HasPermission("admin", p.resource, p.action) {
				t.Errorf("admin should have %s.%s", p.resource, p.action)
			}
			if engine.HasPermission("operator", p.resource, p.action) {
				t.Errorf("operator should NOT have %s.%s", p.resource, p.action)
			}
		})
	}
}

// TestRBACMatrix_IsValidRole verifies role name validation.
func TestRBACMatrix_IsValidRole(t *testing.T) {
	validRoles := []string{"admin", "operator", "viewer", "hana-operator", "compliance-auditor"}
	invalidRoles := []string{"superadmin", "root", "", "ADMIN", "Admin", "viewer ", " admin"}

	for _, role := range validRoles {
		t.Run("valid_"+role, func(t *testing.T) {
			if !auth.IsValidRole(role) {
				t.Errorf("IsValidRole(%q) = false, want true", role)
			}
		})
	}

	for _, role := range invalidRoles {
		t.Run("invalid_"+role, func(t *testing.T) {
			if auth.IsValidRole(role) {
				t.Errorf("IsValidRole(%q) = true, want false", role)
			}
		})
	}
}
