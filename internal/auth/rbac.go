package auth

import (
    "fmt"
    "strings"
)

// Permission represents an action on a resource, e.g. "vm.create".
type Permission struct {
    Resource string
    Action   string
}

// String returns the permission in "resource.action" format.
func (p Permission) String() string {
    return strings.Join([]string{p.Resource, p.Action}, ".")
}

// Role represents a named collection of permissions.
type Role struct {
    Name        string
    Description string
    Permissions []Permission
}

// RBACEngine manages role-to-permission mappings and performs permission checks.
type RBACEngine struct {
    roles map[string]Role
}

// NewRBACEngine creates a new RBAC engine with built-in roles pre-loaded.
func NewRBACEngine() *RBACEngine {
    e := &RBACEngine{roles: make(map[string]Role)}
    e.register()
    return e
}

func (e *RBACEngine) register() {
    e.roles["admin"] = Role{
        Name: "admin", Description: "Full administrative access",
        Permissions: []Permission{
            {Resource: "vm", Action: "list"}, {Resource: "vm", Action: "create"},
            {Resource: "vm", Action: "get"}, {Resource: "vm", Action: "update"},
            {Resource: "vm", Action: "delete"}, {Resource: "vm", Action: "start"},
            {Resource: "vm", Action: "stop"}, {Resource: "vm", Action: "restart"},
            {Resource: "vm", Action: "migrate"}, {Resource: "vm", Action: "console"},
            {Resource: "vm", Action: "snapshot"}, {Resource: "vm", Action: "stats"},
            {Resource: "host", Action: "list"}, {Resource: "host", Action: "create"},
            {Resource: "host", Action: "get"}, {Resource: "host", Action: "update"},
            {Resource: "host", Action: "delete"}, {Resource: "host", Action: "status"},
            {Resource: "host", Action: "maintenance"},
            {Resource: "storage", Action: "list"}, {Resource: "storage", Action: "create"},
            {Resource: "storage", Action: "get"}, {Resource: "storage", Action: "delete"},
            {Resource: "network", Action: "list"}, {Resource: "network", Action: "create"},
            {Resource: "network", Action: "get"}, {Resource: "network", Action: "delete"},
            {Resource: "backup", Action: "list"}, {Resource: "backup", Action: "create"},
            {Resource: "backup", Action: "restore"}, {Resource: "backup", Action: "cancel"},
            {Resource: "tenant", Action: "get"}, {Resource: "tenant", Action: "update"},
            {Resource: "compliance", Action: "view"}, {Resource: "compliance", Action: "validate"},
            {Resource: "compliance", Action: "evidence"}, {Resource: "compliance", Action: "drift"},
            {Resource: "audit", Action: "list"},
            {Resource: "user", Action: "list"}, {Resource: "user", Action: "create"},
            {Resource: "user", Action: "get"}, {Resource: "user", Action: "update"},
            {Resource: "user", Action: "delete"}, {Resource: "event", Action: "list"},
        },
    }
    e.roles["operator"] = Role{
        Name: "operator", Description: "Can manage VMs, hosts, storage, networks, backups",
        Permissions: []Permission{
            {Resource: "vm", Action: "list"}, {Resource: "vm", Action: "create"},
            {Resource: "vm", Action: "get"}, {Resource: "vm", Action: "update"},
            {Resource: "vm", Action: "delete"}, {Resource: "vm", Action: "start"},
            {Resource: "vm", Action: "stop"}, {Resource: "vm", Action: "restart"},
            {Resource: "vm", Action: "migrate"}, {Resource: "vm", Action: "console"},
            {Resource: "vm", Action: "snapshot"}, {Resource: "vm", Action: "stats"},
            {Resource: "host", Action: "list"}, {Resource: "host", Action: "create"},
            {Resource: "host", Action: "get"}, {Resource: "host", Action: "update"},
            {Resource: "host", Action: "delete"}, {Resource: "host", Action: "status"},
            {Resource: "host", Action: "maintenance"},
            {Resource: "storage", Action: "list"}, {Resource: "storage", Action: "create"},
            {Resource: "storage", Action: "get"}, {Resource: "storage", Action: "delete"},
            {Resource: "network", Action: "list"}, {Resource: "network", Action: "create"},
            {Resource: "network", Action: "get"}, {Resource: "network", Action: "delete"},
            {Resource: "backup", Action: "list"}, {Resource: "backup", Action: "create"},
            {Resource: "backup", Action: "restore"}, {Resource: "backup", Action: "cancel"},
            {Resource: "compliance", Action: "view"}, {Resource: "compliance", Action: "validate"},
            {Resource: "compliance", Action: "evidence"}, {Resource: "compliance", Action: "drift"},
            {Resource: "event", Action: "list"},
        },
    }
    e.roles["viewer"] = Role{
        Name: "viewer", Description: "Read-only access to inventory and VMs",
        Permissions: []Permission{
            {Resource: "vm", Action: "list"}, {Resource: "vm", Action: "get"},
            {Resource: "vm", Action: "stats"},
            {Resource: "host", Action: "list"}, {Resource: "host", Action: "get"},
            {Resource: "host", Action: "status"},
            {Resource: "storage", Action: "list"}, {Resource: "storage", Action: "get"},
            {Resource: "network", Action: "list"}, {Resource: "network", Action: "get"},
            {Resource: "backup", Action: "list"},
            {Resource: "compliance", Action: "view"}, {Resource: "event", Action: "list"},
        },
    }
    e.roles["hana-operator"] = Role{
        Name: "hana-operator", Description: "Operator with HANA compliance capabilities",
        Permissions: []Permission{
            {Resource: "vm", Action: "list"}, {Resource: "vm", Action: "create"},
            {Resource: "vm", Action: "get"}, {Resource: "vm", Action: "update"},
            {Resource: "vm", Action: "delete"}, {Resource: "vm", Action: "start"},
            {Resource: "vm", Action: "stop"}, {Resource: "vm", Action: "restart"},
            {Resource: "vm", Action: "migrate"}, {Resource: "vm", Action: "console"},
            {Resource: "vm", Action: "snapshot"}, {Resource: "vm", Action: "stats"},
            {Resource: "host", Action: "list"}, {Resource: "host", Action: "get"},
            {Resource: "host", Action: "status"},
            {Resource: "storage", Action: "list"}, {Resource: "storage", Action: "get"},
            {Resource: "network", Action: "list"}, {Resource: "network", Action: "get"},
            {Resource: "backup", Action: "list"}, {Resource: "backup", Action: "create"},
            {Resource: "backup", Action: "restore"}, {Resource: "backup", Action: "cancel"},
            {Resource: "compliance", Action: "view"}, {Resource: "compliance", Action: "validate"},
            {Resource: "compliance", Action: "evidence"},
            {Resource: "compliance", Action: "drift"}, {Resource: "event", Action: "list"},
        },
    }
    e.roles["compliance-auditor"] = Role{
        Name: "compliance-auditor", Description: "Read-only access to compliance and audit",
        Permissions: []Permission{
            {Resource: "compliance", Action: "view"}, {Resource: "compliance", Action: "evidence"},
            {Resource: "compliance", Action: "drift"}, {Resource: "audit", Action: "list"},
            {Resource: "vm", Action: "list"}, {Resource: "vm", Action: "get"},
            {Resource: "vm", Action: "stats"},
            {Resource: "host", Action: "list"}, {Resource: "host", Action: "get"},
            {Resource: "host", Action: "status"},
            {Resource: "storage", Action: "list"}, {Resource: "storage", Action: "get"},
            {Resource: "network", Action: "list"}, {Resource: "network", Action: "get"},
            {Resource: "event", Action: "list"},
        },
    }
}

// HasPermission checks whether a given role has a specific permission.
func (e *RBACEngine) HasPermission(roleName, resource, action string) bool {
    role, ok := e.roles[roleName]
    if !ok {
        return false
    }
    for _, p := range role.Permissions {
        if p.Resource == resource && p.Action == action {
            return true
        }
    }
    return false
}

// CheckPermission validates that a role has a specific permission.
// Returns nil if allowed, or ErrForbidden if denied.
func (e *RBACEngine) CheckPermission(roleName, resource, action string) error {
    if e.HasPermission(roleName, resource, action) {
        return nil
    }
    return fmt.Errorf("forbidden: role '%s' cannot %s on %s", roleName, action, resource)
}

// IsValidRole returns true if the role name is a recognized built-in role.
func IsValidRole(name string) bool {
    switch name {
    case "admin", "operator", "viewer", "hana-operator", "compliance-auditor":
        return true
    default:
        return false
    }
}
