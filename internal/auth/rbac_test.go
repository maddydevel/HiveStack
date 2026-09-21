package auth

import (
	"testing"
)

func TestNewRBACEngine_PreloadRoles(t *testing.T) {
	engine := NewRBACEngine()
	if engine == nil {
		t.Fatal("expected non-nil engine")
	}

	// Verify all 5 built-in roles exist
	for _, roleName := range []string{"admin", "operator", "viewer", "hana-operator", "compliance-auditor"} {
		if !engine.HasPermission(roleName, "vm", "list") && roleName != "compliance-auditor" {
			// compliance-auditor has vm.list too
		}
		role, ok := engine.roles[roleName]
		if !ok {
			t.Fatalf("role %q not found", roleName)
		}
		if role.Name != roleName {
			t.Errorf("role.Name = %q, want %q", role.Name, roleName)
		}
		if role.Description == "" {
			t.Errorf("role %q has empty description", roleName)
		}
	}
}

func TestHasPermission_Admin(t *testing.T) {
	engine := NewRBACEngine()
	tests := []struct {
		resource string
		action   string
		want     bool
	}{
		{"vm", "list", true},
		{"vm", "create", true},
		{"vm", "delete", true},
		{"vm", "start", true},
		{"vm", "stop", true},
		{"vm", "migrate", true},
		{"host", "list", true},
		{"host", "create", true},
		{"host", "delete", true},
		{"storage", "list", true},
		{"storage", "create", true},
		{"storage", "delete", true},
		{"network", "list", true},
		{"network", "create", true},
		{"network", "delete", true},
		{"backup", "list", true},
		{"backup", "create", true},
		{"backup", "restore", true},
		{"backup", "cancel", true},
		{"compliance", "view", true},
		{"compliance", "validate", true},
		{"compliance", "evidence", true},
		{"compliance", "drift", true},
		{"audit", "list", true},
		{"user", "list", true},
		{"user", "create", true},
		{"user", "delete", true},
		{"event", "list", true},
	}
	for _, tt := range tests {
		t.Run(tt.resource+"."+tt.action, func(t *testing.T) {
			got := engine.HasPermission("admin", tt.resource, tt.action)
			if got != tt.want {
				t.Errorf("HasPermission(admin, %s, %s) = %v, want %v", tt.resource, tt.action, got, tt.want)
			}
		})
	}
}

func TestHasPermission_Operator(t *testing.T) {
	engine := NewRBACEngine()
	// Operator should have most permissions but NOT user management or audit
	tests := []struct {
		resource string
		action   string
		want     bool
	}{
		{"vm", "list", true},
		{"vm", "create", true},
		{"vm", "delete", true},
		{"vm", "migrate", true},
		{"host", "list", true},
		{"host", "create", true},
		{"storage", "list", true},
		{"storage", "create", true},
		{"storage", "delete", true},
		{"network", "list", true},
		{"network", "create", true},
		{"network", "delete", true},
		{"backup", "list", true},
		{"backup", "create", true},
		{"backup", "restore", true},
		{"backup", "cancel", true},
		{"compliance", "view", true},
		{"compliance", "validate", true},
		{"compliance", "evidence", true},
		{"compliance", "drift", true},
		{"user", "list", false}, // operator cannot manage users
		{"user", "create", false},
		{"user", "delete", false},
		{"audit", "list", false}, // operator cannot view audit
		{"event", "list", true},
	}
	for _, tt := range tests {
		t.Run(tt.resource+"."+tt.action, func(t *testing.T) {
			got := engine.HasPermission("operator", tt.resource, tt.action)
			if got != tt.want {
				t.Errorf("HasPermission(operator, %s, %s) = %v, want %v", tt.resource, tt.action, got, tt.want)
			}
		})
	}
}

func TestHasPermission_Viewer(t *testing.T) {
	engine := NewRBACEngine()
	tests := []struct {
		resource string
		action   string
		want     bool
	}{
		// Read-only
		{"vm", "list", true},
		{"vm", "get", true},
		{"vm", "stats", true},
		{"vm", "create", false},
		{"vm", "delete", false},
		{"vm", "start", false},
		{"host", "list", true},
		{"host", "get", true},
		{"host", "status", true},
		{"host", "create", false},
		{"storage", "list", true},
		{"storage", "get", true},
		{"storage", "create", false},
		{"network", "list", true},
		{"network", "get", true},
		{"network", "create", false},
		{"backup", "list", true},
		{"backup", "create", false},
		{"compliance", "view", true},
		{"event", "list", true},
		// Not allowed
		{"vm", "update", false},
		{"user", "list", false},
		{"audit", "list", false},
	}
	for _, tt := range tests {
		t.Run(tt.resource+"."+tt.action, func(t *testing.T) {
			got := engine.HasPermission("viewer", tt.resource, tt.action)
			if got != tt.want {
				t.Errorf("HasPermission(viewer, %s, %s) = %v, want %v", tt.resource, tt.action, got, tt.want)
			}
		})
	}
}

func TestHasPermission_HanaOperator(t *testing.T) {
	engine := NewRBACEngine()
	tests := []struct {
		resource string
		action   string
		want     bool
	}{
		{"vm", "list", true},
		{"vm", "create", true},
		{"vm", "delete", true},
		{"vm", "migrate", true},
		{"host", "list", true},
		{"host", "get", true},
		{"host", "status", true},
		{"host", "create", false}, // hana-operator cannot create hosts
		{"storage", "list", true},
		{"storage", "get", true},
		{"storage", "create", false},
		{"network", "list", true},
		{"network", "get", true},
		{"backup", "list", true},
		{"backup", "create", true},
		{"backup", "restore", true},
		{"compliance", "view", true},
		{"compliance", "validate", true},
		{"compliance", "evidence", true},
		{"compliance", "drift", true},
		{"event", "list", true},
		// Not allowed
		{"user", "list", false},
		{"audit", "list", false},
	}
	for _, tt := range tests {
		t.Run(tt.resource+"."+tt.action, func(t *testing.T) {
			got := engine.HasPermission("hana-operator", tt.resource, tt.action)
			if got != tt.want {
				t.Errorf("HasPermission(hana-operator, %s, %s) = %v, want %v", tt.resource, tt.action, got, tt.want)
			}
		})
	}
}

func TestHasPermission_ComplianceAuditor(t *testing.T) {
	engine := NewRBACEngine()
	tests := []struct {
		resource string
		action   string
		want     bool
	}{
		{"compliance", "view", true},
		{"compliance", "evidence", true},
		{"compliance", "drift", true},
		{"audit", "list", true},
		{"vm", "list", true},
		{"vm", "get", true},
		{"vm", "stats", true},
		{"vm", "create", false},
		{"vm", "delete", false},
		{"vm", "start", false},
		{"host", "list", true},
		{"host", "get", true},
		{"host", "status", true},
		{"storage", "list", true},
		{"storage", "get", true},
		{"network", "list", true},
		{"network", "get", true},
		{"event", "list", true},
		// Not allowed
		{"vm", "update", false},
		{"backup", "create", false},
		{"compliance", "validate", false}, // auditor views but doesn't validate
		{"user", "list", false},
	}
	for _, tt := range tests {
		t.Run(tt.resource+"."+tt.action, func(t *testing.T) {
			got := engine.HasPermission("compliance-auditor", tt.resource, tt.action)
			if got != tt.want {
				t.Errorf("HasPermission(compliance-auditor, %s, %s) = %v, want %v", tt.resource, tt.action, got, tt.want)
			}
		})
	}
}

func TestHasPermission_UnknownRole(t *testing.T) {
	engine := NewRBACEngine()
	if engine.HasPermission("nonexistent", "vm", "list") {
		t.Fatal("expected false for unknown role")
	}
}

func TestHasPermission_UnknownResource(t *testing.T) {
	engine := NewRBACEngine()
	if engine.HasPermission("admin", "nonexistent", "list") {
		t.Fatal("expected false for unknown resource")
	}
}

func TestHasPermission_UnknownAction(t *testing.T) {
	engine := NewRBACEngine()
	if engine.HasPermission("admin", "vm", "nonexistent") {
		t.Fatal("expected false for unknown action")
	}
}

func TestCheckPermission_Allowed(t *testing.T) {
	engine := NewRBACEngine()
	err := engine.CheckPermission("admin", "vm", "create")
	if err != nil {
		t.Fatalf("expected nil, got %v", err)
	}
}

func TestCheckPermission_Denied(t *testing.T) {
	engine := NewRBACEngine()
	err := engine.CheckPermission("viewer", "vm", "create")
	if err == nil {
		t.Fatal("expected forbidden error")
	}
	if err.Error() != "forbidden: role 'viewer' cannot create on vm" {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestIsValidRole(t *testing.T) {
	tests := []struct {
		name string
		want bool
	}{
		{"admin", true},
		{"operator", true},
		{"viewer", true},
		{"hana-operator", true},
		{"compliance-auditor", true},
		{"superadmin", false},
		{"", false},
		{"ADMIN", false}, // case-sensitive
		{" Viewer", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsValidRole(tt.name)
			if got != tt.want {
				t.Errorf("IsValidRole(%q) = %v, want %v", tt.name, got, tt.want)
			}
		})
	}
}

func TestPermission_String(t *testing.T) {
	p := Permission{Resource: "vm", Action: "create"}
	if p.String() != "vm.create" {
		t.Errorf("String() = %q, want vm.create", p.String())
	}
	p2 := Permission{Resource: "user", Action: "delete"}
	if p2.String() != "user.delete" {
		t.Errorf("String() = %q", p2.String())
	}
}

func TestRBACEngine_RolesHaveDescriptions(t *testing.T) {
	engine := NewRBACEngine()
	for name, role := range engine.roles {
		t.Run(name, func(t *testing.T) {
			if role.Description == "" {
				t.Errorf("role %q has empty description", name)
			}
			if len(role.Permissions) == 0 {
				t.Errorf("role %q has no permissions", name)
			}
		})
	}
}

func TestRBACEngine_AdminHasMostPermissions(t *testing.T) {
	engine := NewRBACEngine()
	adminPerms := make(map[string]bool)
	for _, p := range engine.roles["admin"].Permissions {
		adminPerms[p.String()] = true
	}
	// Admin should be a superset of viewer
	viewerPerms := make(map[string]bool)
	for _, p := range engine.roles["viewer"].Permissions {
		viewerPerms[p.String()] = true
	}
	for perm := range viewerPerms {
		if !adminPerms[perm] {
			t.Errorf("admin missing viewer permission: %s", perm)
		}
	}
}
