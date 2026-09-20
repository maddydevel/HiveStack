// Package db provides repository access for HiveStack entities.
package db

import (
	"context"
	"fmt"
	"time"
)

// CreateTenant inserts a new tenant and returns its ID.
func (d *DB) CreateTenant(ctx context.Context, t *Tenant) (string, error) {
	tx, err := d.Begin(ctx)
	if err != nil {
		return "", err
	}
	defer tx.Rollback(ctx)

	id, err := NewRecord(ctx, tx, "tenant", map[string]interface{}{
		"name":   t.Name,
		"slug":   t.Slug,
		"status": t.Status,
		"config": t.Config,
	})
	if err != nil {
		return "", err
	}
	if err := tx.Commit(ctx); err != nil {
		return "", err
	}
	return id, nil
}

// GetTenant retrieves a tenant by ID.
func (d *DB) GetTenant(ctx context.Context, id string) (*Tenant, error) {
	row := d.QueryRowContext(ctx, `
        SELECT id, name, slug, status, config, created_at, updated_at
        FROM tenant WHERE id = $1
    `, id)
	var t Tenant
	err := row.Scan(&t.ID, &t.Name, &t.Slug, &t.Status, &t.Config, &t.CreatedAt, &t.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &t, nil
}

// ListTenants returns all tenants.
func (d *DB) ListTenants(ctx context.Context) ([]Tenant, error) {
	rows, err := d.QueryContext(ctx, `
        SELECT id, name, slug, status, config, created_at, updated_at
        FROM tenant ORDER BY name
    `)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var tenants []Tenant
	for rows.Next() {
		var t Tenant
		if err := rows.Scan(&t.ID, &t.Name, &t.Slug, &t.Status, &t.Config, &t.CreatedAt, &t.UpdatedAt); err != nil {
			return nil, err
		}
		tenants = append(tenants, t)
	}
	return tenants, rows.Err()
}

// CreateUser inserts a new user and returns its ID.
func (d *DB) CreateUser(ctx context.Context, u *User) (string, error) {
	tx, err := d.Begin(ctx)
	if err != nil {
		return "", err
	}
	defer tx.Rollback(ctx)

	id, err := NewRecord(ctx, tx, "users", map[string]interface{}{
		"tenant_id":     u.TenantID,
		"name":          u.Name,
		"email":         u.Email,
		"password_hash": u.PasswordHash,
		"role":          u.Role,
	})
	if err != nil {
		return "", err
	}
	if err := tx.Commit(ctx); err != nil {
		return "", err
	}
	return id, nil
}

// GetUser retrieves a user by ID.
func (d *DB) GetUser(ctx context.Context, id string) (*User, error) {
	row := d.QueryRowContext(ctx, `
        SELECT id, tenant_id, name, email, password_hash, role, created_at, updated_at
        FROM users WHERE id = $1
    `, id)
	var u User
	err := row.Scan(&u.ID, &u.TenantID, &u.Name, &u.Email, &u.PasswordHash, &u.Role, &u.CreatedAt, &u.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &u, nil
}

// GetUserInTenant retrieves a user by ID, scoped to a tenant. A user that
// belongs to a different tenant is reported as not found.
func (d *DB) GetUserInTenant(ctx context.Context, tenantID, id string) (*User, error) {
	row := d.QueryRowContext(ctx, `
        SELECT id, tenant_id, name, email, password_hash, role, created_at, updated_at
        FROM users WHERE tenant_id = $1 AND id = $2
    `, tenantID, id)
	var u User
	err := row.Scan(&u.ID, &u.TenantID, &u.Name, &u.Email, &u.PasswordHash, &u.Role, &u.CreatedAt, &u.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &u, nil
}

// GetUserByEmail retrieves a user by email within a tenant.
func (d *DB) GetUserByEmail(ctx context.Context, tenantID, email string) (*User, error) {
	row := d.QueryRowContext(ctx, `
        SELECT id, tenant_id, name, email, password_hash, role, created_at, updated_at
        FROM users WHERE tenant_id = $1 AND email = $2
    `, tenantID, email)
	var u User
	err := row.Scan(&u.ID, &u.TenantID, &u.Name, &u.Email, &u.PasswordHash, &u.Role, &u.CreatedAt, &u.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &u, nil
}

// ListUsers returns all users in a tenant.
func (d *DB) ListUsers(ctx context.Context, tenantID string) ([]User, error) {
	rows, err := d.QueryContext(ctx, `
        SELECT id, tenant_id, name, email, password_hash, role, created_at, updated_at
        FROM users WHERE tenant_id = $1 ORDER BY name
    `, tenantID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var users []User
	for rows.Next() {
		var u User
		if err := rows.Scan(&u.ID, &u.TenantID, &u.Name, &u.Email, &u.PasswordHash, &u.Role, &u.CreatedAt, &u.UpdatedAt); err != nil {
			return nil, err
		}
		users = append(users, u)
	}
	return users, rows.Err()
}

// UpdateUser updates a user's fields.
func (d *DB) UpdateUser(ctx context.Context, id string, updates map[string]interface{}) error {
	if len(updates) == 0 {
		return fmt.Errorf("no updates provided")
	}
	setClauses := make([]string, 0, len(updates))
	args := make([]interface{}, 0, len(updates))
	i := 1
	for col, val := range updates {
		setClauses = append(setClauses, fmt.Sprintf("%s = $%d", col, i))
		args = append(args, val)
		i++
	}
	args = append(args, id)
	query := fmt.Sprintf("UPDATE users SET %s WHERE id = $%d",
		joinStrings(setClauses, ", "), len(args))
	_, err := d.ExecContext(ctx, query, args...)
	return err
}

// DeleteUser removes a user by ID.
func (d *DB) DeleteUser(ctx context.Context, id string) error {
	_, err := d.ExecContext(ctx, "DELETE FROM users WHERE id = $1", id)
	return err
}

// CreateHost registers a new host and returns its ID.
func (d *DB) CreateHost(ctx context.Context, h *Host) (string, error) {
	tx, err := d.Begin(ctx)
	if err != nil {
		return "", err
	}
	defer tx.Rollback(ctx)

	id, err := NewRecord(ctx, tx, "host", map[string]interface{}{
		"tenant_id":           h.TenantID,
		"cluster_id":          h.ClusterID,
		"name":                h.Name,
		"hostname":            h.Hostname,
		"ip_address":          h.IPAddress,
		"status":              h.Status,
		"cpu_model":           h.CPUModel,
		"cpu_count":           h.CPUCount,
		"memory_total_bytes":  h.MemoryTotalBytes,
		"storage_total_bytes": h.StorageTotalBytes,
		"os":                  h.OS,
		"hypervisor":          h.Hypervisor,
		"agent_token_hash":    h.AgentTokenHash,
		"numa_node_count":     h.NUMANodeCount,
		"hugepages_total_kb":  h.HugepagesTotalKB,
		"maintenance_mode":    h.MaintenanceMode,
	})
	if err != nil {
		return "", err
	}
	if err := tx.Commit(ctx); err != nil {
		return "", err
	}
	return id, nil
}

// GetHost retrieves a host by ID.
func (d *DB) GetHost(ctx context.Context, id string) (*Host, error) {
	row := d.QueryRowContext(ctx, `
        SELECT id, tenant_id, cluster_id, name, hostname, ip_address, status,
               cpu_model, cpu_count, memory_total_bytes, storage_total_bytes,
               os, hypervisor, agent_token_hash, numa_node_count, hugepages_total_kb,
               maintenance_mode, joined_at, last_heartbeat, created_at, updated_at
        FROM host WHERE id = $1
    `, id)
	var h Host
	err := row.Scan(&h.ID, &h.TenantID, &h.ClusterID, &h.Name, &h.Hostname, &h.IPAddress, &h.Status,
		&h.CPUModel, &h.CPUCount, &h.MemoryTotalBytes, &h.StorageTotalBytes,
		&h.OS, &h.Hypervisor, &h.AgentTokenHash, &h.NUMANodeCount, &h.HugepagesTotalKB,
		&h.MaintenanceMode, &h.JoinedAt, &h.LastHeartbeat, &h.CreatedAt, &h.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &h, nil
}

// ListHosts returns all hosts in a tenant.
func (d *DB) ListHosts(ctx context.Context, tenantID string) ([]Host, error) {
	rows, err := d.QueryContext(ctx, `
        SELECT id, tenant_id, cluster_id, name, hostname, ip_address, status,
               cpu_model, cpu_count, memory_total_bytes, storage_total_bytes,
               os, hypervisor, agent_token_hash, numa_node_count, hugepages_total_kb,
               maintenance_mode, joined_at, last_heartbeat, created_at, updated_at
        FROM host WHERE tenant_id = $1 ORDER BY name
    `, tenantID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var hosts []Host
	for rows.Next() {
		var h Host
		if err := rows.Scan(&h.ID, &h.TenantID, &h.ClusterID, &h.Name, &h.Hostname, &h.IPAddress, &h.Status,
			&h.CPUModel, &h.CPUCount, &h.MemoryTotalBytes, &h.StorageTotalBytes,
			&h.OS, &h.Hypervisor, &h.AgentTokenHash, &h.NUMANodeCount, &h.HugepagesTotalKB,
			&h.MaintenanceMode, &h.JoinedAt, &h.LastHeartbeat, &h.CreatedAt, &h.UpdatedAt); err != nil {
			return nil, err
		}
		hosts = append(hosts, h)
	}
	return hosts, rows.Err()
}

// UpdateHost updates a host's fields.
func (d *DB) UpdateHost(ctx context.Context, id string, updates map[string]interface{}) error {
	if len(updates) == 0 {
		return fmt.Errorf("no updates provided")
	}
	setClauses := make([]string, 0, len(updates))
	args := make([]interface{}, 0, len(updates))
	i := 1
	for col, val := range updates {
		setClauses = append(setClauses, fmt.Sprintf("%s = $%d", col, i))
		args = append(args, val)
		i++
	}
	args = append(args, id)
	query := fmt.Sprintf("UPDATE host SET %s WHERE id = $%d",
		joinStrings(setClauses, ", "), len(args))
	_, err := d.ExecContext(ctx, query, args...)
	return err
}

// DeleteHost removes a host by ID.
func (d *DB) DeleteHost(ctx context.Context, id string) error {
	_, err := d.ExecContext(ctx, "DELETE FROM host WHERE id = $1", id)
	return err
}

// CreateVM inserts a new VM and returns its ID.
func (d *DB) CreateVM(ctx context.Context, vm *VM) (string, error) {
	tx, err := d.Begin(ctx)
	if err != nil {
		return "", err
	}
	defer tx.Rollback(ctx)

	id, err := NewRecord(ctx, tx, "vm", map[string]interface{}{
		"tenant_id":                vm.TenantID,
		"cluster_id":               vm.ClusterID,
		"host_id":                  vm.HostID,
		"identifier":               vm.Identifier,
		"name":                     vm.Name,
		"description":              vm.Description,
		"status":                   vm.Status,
		"role":                     string(vm.Role),
		"cpus":                     vm.CPUs,
		"cpu_allocation":           vm.CPUAllocation,
		"memory_bytes":             vm.MemoryBytes,
		"numa_policy":              vm.NUMAPolicy,
		"hugepages_enabled":        vm.HugepagesEnabled,
		"cpu_pinning":              vm.CPUPinning,
		"memory_reservation_bytes": vm.MemoryReservationBytes,
		"ballooning_allowed":       vm.BallooningAllowed,
		"swap_allowed":             vm.SwapAllowed,
		"os":                       vm.OS,
		"template_id":              vm.TemplateID,
	})
	if err != nil {
		return "", err
	}
	if err := tx.Commit(ctx); err != nil {
		return "", err
	}
	return id, nil
}

// GetVM retrieves a VM by ID.
func (d *DB) GetVM(ctx context.Context, id string) (*VM, error) {
	row := d.QueryRowContext(ctx, `
        SELECT id, tenant_id, cluster_id, host_id, identifier, name, description, status,
               role, cpus, cpu_allocation, memory_bytes,
               numa_policy, hugepages_enabled, cpu_pinning, memory_reservation_bytes,
               ballooning_allowed, swap_allowed,
               os, template_id, snapshot_count, created_at, started_at, updated_at
        FROM vm WHERE id = $1
    `, id)
	var vm VM
	err := row.Scan(&vm.ID, &vm.TenantID, &vm.ClusterID, &vm.HostID, &vm.Identifier, &vm.Name, &vm.Description, &vm.Status,
		&vm.Role, &vm.CPUs, &vm.CPUAllocation, &vm.MemoryBytes,
		&vm.NUMAPolicy, &vm.HugepagesEnabled, &vm.CPUPinning, &vm.MemoryReservationBytes,
		&vm.BallooningAllowed, &vm.SwapAllowed,
		&vm.OS, &vm.TemplateID, &vm.SnapshotCount, &vm.CreatedAt, &vm.StartedAt, &vm.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &vm, nil
}

// GetVMInTenant retrieves a VM by ID, scoped to a tenant. A VM that belongs
// to a different tenant is reported as not found.
func (d *DB) GetVMInTenant(ctx context.Context, tenantID, id string) (*VM, error) {
	row := d.QueryRowContext(ctx, `
        SELECT id, tenant_id, cluster_id, host_id, identifier, name, description, status,
               role, cpus, cpu_allocation, memory_bytes,
               numa_policy, hugepages_enabled, cpu_pinning, memory_reservation_bytes,
               ballooning_allowed, swap_allowed,
               os, template_id, snapshot_count, created_at, started_at, updated_at
        FROM vm WHERE tenant_id = $1 AND id = $2
    `, tenantID, id)
	var vm VM
	err := row.Scan(&vm.ID, &vm.TenantID, &vm.ClusterID, &vm.HostID, &vm.Identifier, &vm.Name, &vm.Description, &vm.Status,
		&vm.Role, &vm.CPUs, &vm.CPUAllocation, &vm.MemoryBytes,
		&vm.NUMAPolicy, &vm.HugepagesEnabled, &vm.CPUPinning, &vm.MemoryReservationBytes,
		&vm.BallooningAllowed, &vm.SwapAllowed,
		&vm.OS, &vm.TemplateID, &vm.SnapshotCount, &vm.CreatedAt, &vm.StartedAt, &vm.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &vm, nil
}

// ListVMs returns all VMs in a tenant.
func (d *DB) ListVMs(ctx context.Context, tenantID string) ([]VM, error) {
	rows, err := d.QueryContext(ctx, `
        SELECT id, tenant_id, cluster_id, host_id, identifier, name, description, status,
               role, cpus, cpu_allocation, memory_bytes,
               numa_policy, hugepages_enabled, cpu_pinning, memory_reservation_bytes,
               ballooning_allowed, swap_allowed,
               os, template_id, snapshot_count, created_at, started_at, updated_at
        FROM vm WHERE tenant_id = $1 ORDER BY name
    `, tenantID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var vms []VM
	for rows.Next() {
		var vm VM
		if err := rows.Scan(&vm.ID, &vm.TenantID, &vm.ClusterID, &vm.HostID, &vm.Identifier, &vm.Name, &vm.Description, &vm.Status,
			&vm.Role, &vm.CPUs, &vm.CPUAllocation, &vm.MemoryBytes,
			&vm.NUMAPolicy, &vm.HugepagesEnabled, &vm.CPUPinning, &vm.MemoryReservationBytes,
			&vm.BallooningAllowed, &vm.SwapAllowed,
			&vm.OS, &vm.TemplateID, &vm.SnapshotCount, &vm.CreatedAt, &vm.StartedAt, &vm.UpdatedAt); err != nil {
			return nil, err
		}
		vms = append(vms, vm)
	}
	return vms, rows.Err()
}

// UpdateVM updates a VM's fields.
func (d *DB) UpdateVM(ctx context.Context, id string, updates map[string]interface{}) error {
	if len(updates) == 0 {
		return fmt.Errorf("no updates provided")
	}
	setClauses := make([]string, 0, len(updates))
	args := make([]interface{}, 0, len(updates))
	i := 1
	for col, val := range updates {
		setClauses = append(setClauses, fmt.Sprintf("%s = $%d", col, i))
		args = append(args, val)
		i++
	}
	args = append(args, id)
	query := fmt.Sprintf("UPDATE vm SET %s WHERE id = $%d",
		joinStrings(setClauses, ", "), len(args))
	_, err := d.ExecContext(ctx, query, args...)
	return err
}

// DeleteVM removes a VM by ID.
func (d *DB) DeleteVM(ctx context.Context, id string) error {
	_, err := d.ExecContext(ctx, "DELETE FROM vm WHERE id = $1", id)
	return err
}

// CreateStoragePool inserts a new storage pool and returns its ID.
func (d *DB) CreateStoragePool(ctx context.Context, sp *StoragePool) (string, error) {
	tx, err := d.Begin(ctx)
	if err != nil {
		return "", err
	}
	defer tx.Rollback(ctx)

	id, err := NewRecord(ctx, tx, "storage_pool", map[string]interface{}{
		"tenant_id":            sp.TenantID,
		"datacenter_id":        sp.DatacenterID,
		"name":                 sp.Name,
		"type":                 sp.Type,
		"path":                 sp.Path,
		"status":               sp.Status,
		"total_capacity_bytes": sp.TotalCapacityBytes,
		"free_space_bytes":     sp.FreeSpaceBytes,
		"used_space_bytes":     sp.UsedSpaceBytes,
		"features":             sp.Features,
	})
	if err != nil {
		return "", err
	}
	if err := tx.Commit(ctx); err != nil {
		return "", err
	}
	return id, nil
}

// GetStoragePool retrieves a storage pool by ID.
func (d *DB) GetStoragePool(ctx context.Context, id string) (*StoragePool, error) {
	row := d.QueryRowContext(ctx, `
        SELECT id, tenant_id, datacenter_id, name, type, path, status,
               total_capacity_bytes, free_space_bytes, used_space_bytes,
               features, created_at, updated_at
        FROM storage_pool WHERE id = $1
    `, id)
	var sp StoragePool
	err := row.Scan(&sp.ID, &sp.TenantID, &sp.DatacenterID, &sp.Name, &sp.Type, &sp.Path, &sp.Status,
		&sp.TotalCapacityBytes, &sp.FreeSpaceBytes, &sp.UsedSpaceBytes,
		&sp.Features, &sp.CreatedAt, &sp.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &sp, nil
}

// ListStoragePools returns all storage pools in a tenant.
func (d *DB) ListStoragePools(ctx context.Context, tenantID string) ([]StoragePool, error) {
	rows, err := d.QueryContext(ctx, `
        SELECT id, tenant_id, datacenter_id, name, type, path, status,
               total_capacity_bytes, free_space_bytes, used_space_bytes,
               features, created_at, updated_at
        FROM storage_pool WHERE tenant_id = $1 ORDER BY name
    `, tenantID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var pools []StoragePool
	for rows.Next() {
		var sp StoragePool
		if err := rows.Scan(&sp.ID, &sp.TenantID, &sp.DatacenterID, &sp.Name, &sp.Type, &sp.Path, &sp.Status,
			&sp.TotalCapacityBytes, &sp.FreeSpaceBytes, &sp.UsedSpaceBytes,
			&sp.Features, &sp.CreatedAt, &sp.UpdatedAt); err != nil {
			return nil, err
		}
		pools = append(pools, sp)
	}
	return pools, rows.Err()
}

// CreateNetwork inserts a new network and returns its ID.
func (d *DB) CreateNetwork(ctx context.Context, n *Network) (string, error) {
	tx, err := d.Begin(ctx)
	if err != nil {
		return "", err
	}
	defer tx.Rollback(ctx)

	id, err := NewRecord(ctx, tx, "network", map[string]interface{}{
		"tenant_id":     n.TenantID,
		"datacenter_id": n.DatacenterID,
		"name":          n.Name,
		"description":   n.Description,
		"type":          n.Type,
		"bridge_name":   n.BridgeName,
		"vlan_id":       n.VLANID,
		"subnet":        n.Subnet,
		"gateway":       n.Gateway,
		"dhcp":          n.DHCP,
		"dns":           n.DNS,
		"status":        n.Status,
	})
	if err != nil {
		return "", err
	}
	if err := tx.Commit(ctx); err != nil {
		return "", err
	}
	return id, nil
}

// GetNetwork retrieves a network by ID.
func (d *DB) GetNetwork(ctx context.Context, id string) (*Network, error) {
	row := d.QueryRowContext(ctx, `
        SELECT id, tenant_id, datacenter_id, name, description, type, bridge_name,
               vlan_id, subnet, gateway, dhcp, dns, status, created_at, updated_at
        FROM network WHERE id = $1
    `, id)
	var n Network
	err := row.Scan(&n.ID, &n.TenantID, &n.DatacenterID, &n.Name, &n.Description, &n.Type, &n.BridgeName,
		&n.VLANID, &n.Subnet, &n.Gateway, &n.DHCP, &n.DNS, &n.Status, &n.CreatedAt, &n.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &n, nil
}

// ListNetworks returns all networks in a tenant.
func (d *DB) ListNetworks(ctx context.Context, tenantID string) ([]Network, error) {
	rows, err := d.QueryContext(ctx, `
        SELECT id, tenant_id, datacenter_id, name, description, type, bridge_name,
               vlan_id, subnet, gateway, dhcp, dns, status, created_at, updated_at
        FROM network WHERE tenant_id = $1 ORDER BY name
    `, tenantID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var networks []Network
	for rows.Next() {
		var n Network
		if err := rows.Scan(&n.ID, &n.TenantID, &n.DatacenterID, &n.Name, &n.Description, &n.Type, &n.BridgeName,
			&n.VLANID, &n.Subnet, &n.Gateway, &n.DHCP, &n.DNS, &n.Status, &n.CreatedAt, &n.UpdatedAt); err != nil {
			return nil, err
		}
		networks = append(networks, n)
	}
	return networks, rows.Err()
}

// DeleteNetwork removes a network by ID.
func (d *DB) DeleteNetwork(ctx context.Context, id string) error {
	_, err := d.ExecContext(ctx, "DELETE FROM network WHERE id = $1", id)
	return err
}

// CreateEvent inserts a new event record.
func (d *DB) CreateEvent(ctx context.Context, e *Event) (string, error) {
	tx, err := d.Begin(ctx)
	if err != nil {
		return "", err
	}
	defer tx.Rollback(ctx)

	id, err := NewRecord(ctx, tx, "event", map[string]interface{}{
		"tenant_id":     e.TenantID,
		"type":          e.Type,
		"severity":      e.Severity,
		"message":       e.Message,
		"actor_type":    e.ActorType,
		"actor_id":      e.ActorID,
		"actor_name":    e.ActorName,
		"resource_type": e.ResourceType,
		"resource_id":   e.ResourceID,
		"resource_name": e.ResourceName,
		"metadata":      e.Metadata,
	})
	if err != nil {
		return "", err
	}
	if err := tx.Commit(ctx); err != nil {
		return "", err
	}
	return id, nil
}

// ListEvents returns recent events in a tenant.
func (d *DB) ListEvents(ctx context.Context, tenantID string, limit int) ([]Event, error) {
	rows, err := d.QueryContext(ctx,
		"SELECT id, tenant_id, type, severity, message,"+
		" actor_type, actor_id, actor_name,"+
		" resource_type, resource_id, resource_name,"+
		" metadata, created_at"+
		" FROM event WHERE tenant_id = $1"+
		" ORDER BY created_at DESC LIMIT $2",
		tenantID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var events []Event
	for rows.Next() {
		var e Event
		if err := rows.Scan(&e.ID, &e.TenantID, &e.Type, &e.Severity, &e.Message,
			&e.ActorType, &e.ActorID, &e.ActorName,
			&e.ResourceType, &e.ResourceID, &e.ResourceName,
			&e.Metadata, &e.CreatedAt); err != nil {
			return nil, err
		}
		events = append(events, e)
	}
	return events, rows.Err()
}

// RecordEvent records a simple event with just a message.
func (d *DB) RecordEvent(ctx context.Context, tenantID, severity, message string,
	actorType, actorID, actorName, resourceType, resourceID, resourceName string) error {
	_, err := d.CreateEvent(ctx, &Event{
		TenantID:     tenantID,
		Type:         "info",
		Severity:     severity,
		Message:      message,
		ActorType:    actorType,
		ActorID:      actorID,
		ActorName:    actorName,
		ResourceType: resourceType,
		ResourceID:   resourceID,
		ResourceName: resourceName,
	})
	return err
}

// UpdateHostHeartbeat updates the last heartbeat timestamp for a host.
func (d *DB) UpdateHostHeartbeat(ctx context.Context, hostID string) error {
	_, err := d.ExecContext(ctx, `
        UPDATE host SET last_heartbeat = $1, updated_at = $1 WHERE id = $2
    `, time.Now(), hostID)
	return err
}

// CreateBackup inserts a new backup and returns its ID.
func (d *DB) CreateBackup(ctx context.Context, b *Backup) (string, error) {
	tx, err := d.Begin(ctx)
	if err != nil {
		return "", err
	}
	defer tx.Rollback(ctx)

	id, err := NewRecord(ctx, tx, "backup", map[string]interface{}{
		"tenant_id":    b.TenantID,
		"vm_id":        b.VMID,
		"name":         b.Name,
		"status":       b.Status,
		"type":         b.Type,
		"storage_path": b.StoragePath,
		"size_bytes":   b.SizeBytes,
		"progress":     b.Progress,
		"message":      b.Message,
	})
	if err != nil {
		return "", err
	}
	if err := tx.Commit(ctx); err != nil {
		return "", err
	}
	return id, nil
}

// GetBackup retrieves a backup by ID.
func (d *DB) GetBackup(ctx context.Context, id string) (*Backup, error) {
	row := d.QueryRowContext(ctx, `
        SELECT id, tenant_id, vm_id, name, status, type, storage_path,
               size_bytes, progress, message, schedule_id, started_at, completed_at,
               created_at, updated_at
        FROM backup WHERE id = $1
    `, id)
	var b Backup
	err := row.Scan(&b.ID, &b.TenantID, &b.VMID, &b.Name, &b.Status, &b.Type, &b.StoragePath,
		&b.SizeBytes, &b.Progress, &b.Message, &b.ScheduleID, &b.StartedAt, &b.CompletedAt,
		&b.CreatedAt, &b.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &b, nil
}

// ListBackups returns all backups for a tenant.
func (d *DB) ListBackups(ctx context.Context, tenantID string) ([]Backup, error) {
	rows, err := d.QueryContext(ctx, `
        SELECT id, tenant_id, vm_id, name, status, type, storage_path,
               size_bytes, progress, message, schedule_id, started_at, completed_at,
               created_at, updated_at
        FROM backup WHERE tenant_id = $1 ORDER BY created_at DESC
    `, tenantID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var backups []Backup
	for rows.Next() {
		var b Backup
		if err := rows.Scan(&b.ID, &b.TenantID, &b.VMID, &b.Name, &b.Status, &b.Type, &b.StoragePath,
			&b.SizeBytes, &b.Progress, &b.Message, &b.ScheduleID, &b.StartedAt, &b.CompletedAt,
			&b.CreatedAt, &b.UpdatedAt); err != nil {
			return nil, err
		}
		backups = append(backups, b)
	}
	return backups, rows.Err()
}

// UpdateBackup updates a backup's fields.
func (d *DB) UpdateBackup(ctx context.Context, id string, updates map[string]interface{}) error {
	if len(updates) == 0 {
		return fmt.Errorf("no updates provided")
	}
	setClauses := make([]string, 0, len(updates))
	args := make([]interface{}, 0, len(updates))
	i := 1
	for col, val := range updates {
		setClauses = append(setClauses, fmt.Sprintf("%s = $%d", col, i))
		args = append(args, val)
		i++
	}
	args = append(args, id)
	query := fmt.Sprintf("UPDATE backup SET %s WHERE id = $%d",
		joinStrings(setClauses, ", "), len(args))
	_, err := d.ExecContext(ctx, query, args...)
	return err
}

// DeleteBackup removes a backup by ID.
func (d *DB) DeleteBackup(ctx context.Context, id string) error {
	_, err := d.ExecContext(ctx, "DELETE FROM backup WHERE id = $1", id)
	return err
}

// CreateDatacenter inserts a new datacenter and returns its ID.
func (d *DB) CreateDatacenter(ctx context.Context, dc *Datacenter) (string, error) {
	tx, err := d.Begin(ctx)
	if err != nil {
		return "", err
	}
	defer tx.Rollback(ctx)

	id, err := NewRecord(ctx, tx, "datacenter", map[string]interface{}{
		"tenant_id":    dc.TenantID,
		"name":         dc.Name,
		"description":  dc.Description,
		"status":       dc.Status,
	})
	if err != nil {
		return "", err
	}
	if err := tx.Commit(ctx); err != nil {
		return "", err
	}
	return id, nil
}

// GetDatacenter retrieves a datacenter by ID.
func (d *DB) GetDatacenter(ctx context.Context, id string) (*Datacenter, error) {
	row := d.QueryRowContext(ctx, `
	    SELECT id, tenant_id, name, description, status, created_at, updated_at
	    FROM datacenter WHERE id = $1
	`, id)
	var dc Datacenter
	err := row.Scan(&dc.ID, &dc.TenantID, &dc.Name, &dc.Description, &dc.Status, &dc.CreatedAt, &dc.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &dc, nil
}

// ListDatacenters returns all datacenters for a tenant.
func (d *DB) ListDatacenters(ctx context.Context, tenantID string) ([]Datacenter, error) {
	rows, err := d.QueryContext(ctx, `
	    SELECT id, tenant_id, name, description, status, created_at, updated_at
	    FROM datacenter WHERE tenant_id = $1 ORDER BY name
	`, tenantID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var dcs []Datacenter
	for rows.Next() {
		var dc Datacenter
		if err := rows.Scan(&dc.ID, &dc.TenantID, &dc.Name, &dc.Description, &dc.Status, &dc.CreatedAt, &dc.UpdatedAt); err != nil {
			return nil, err
		}
		dcs = append(dcs, dc)
	}
	return dcs, rows.Err()
}

// DeleteDatacenter removes a datacenter by ID.
func (d *DB) DeleteDatacenter(ctx context.Context, id string) error {
	_, err := d.ExecContext(ctx, "DELETE FROM datacenter WHERE id = $1", id)
	return err
}

// GetDisk retrieves a disk by ID.
func (d *DB) GetDisk(ctx context.Context, id string) (*Disk, error) {
	row := d.QueryRowContext(ctx, `
        SELECT id, tenant_id, vm_id, storage_pool_id, name, size_bytes, format, path, bus, mounted, created_at, updated_at
        FROM disk WHERE id = $1
    `, id)
	var disk Disk
	err := row.Scan(&disk.ID, &disk.TenantID, &disk.VMID, &disk.StoragePoolID, &disk.Name, &disk.SizeBytes, &disk.Format, &disk.Path, &disk.Bus, &disk.Mounted, &disk.CreatedAt, &disk.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &disk, nil
}

// UpdateDisk updates a disk's fields.
func (d *DB) UpdateDisk(ctx context.Context, id string, updates map[string]interface{}) error {
	if len(updates) == 0 {
		return fmt.Errorf("no updates provided")
	}
	setClauses := make([]string, 0, len(updates))
	args := make([]interface{}, 0, len(updates))
	i := 1
	for col, val := range updates {
		setClauses = append(setClauses, fmt.Sprintf("%s = $%d", col, i))
		args = append(args, val)
		i++
	}
	args = append(args, id)
	query := fmt.Sprintf("UPDATE disk SET %s WHERE id = $%d",
		joinStrings(setClauses, ", "), len(args))
	_, err := d.ExecContext(ctx, query, args...)
	return err
}
