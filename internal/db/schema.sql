-- HiveStack database schema (canonical reference).
--
-- This file is a consolidated, human-readable view of the full schema.
-- It is generated from, and must be kept in sync with, the numbered
-- migration files in internal/db/migrations/, which are the executable
-- source of truth applied by DB.Migrate().

-- ============================================================
-- 0001_tenants_and_auth.sql
-- ============================================================
-- Tenants, users, RBAC, and API tokens.
-- All tenant-scoped tables carry a NOT NULL tenant_id FK so a single Postgres
-- instance can safely serve multiple customers.

CREATE TABLE tenants (
    id          TEXT PRIMARY KEY,
    name        TEXT NOT NULL,
    slug        TEXT NOT NULL UNIQUE,
    status      TEXT NOT NULL DEFAULT 'active'
                CHECK (status IN ('active', 'suspended', 'archived')),
    config      JSONB NOT NULL DEFAULT '{}',
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE users (
    id             TEXT PRIMARY KEY,
    tenant_id      TEXT NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    name           TEXT NOT NULL,
    email          TEXT NOT NULL,
    password_hash  TEXT NOT NULL,
    role           TEXT NOT NULL DEFAULT 'viewer',
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (tenant_id, email)
);

CREATE INDEX idx_users_tenant_id ON users(tenant_id);

-- Custom, tenant-defined roles. Built-in roles (admin, operator, viewer,
-- hana-operator, compliance-auditor) are defined in code (internal/auth/rbac.go)
-- and do not require rows here; this table lets a tenant layer additional
-- custom roles/permissions on top of the built-ins.
CREATE TABLE roles (
    id          TEXT PRIMARY KEY,
    tenant_id   TEXT NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    name        TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (tenant_id, name)
);

CREATE INDEX idx_roles_tenant_id ON roles(tenant_id);

CREATE TABLE permissions (
    id        TEXT PRIMARY KEY,
    role_id   TEXT NOT NULL REFERENCES roles(id) ON DELETE CASCADE,
    resource  TEXT NOT NULL,
    action    TEXT NOT NULL,
    UNIQUE (role_id, resource, action)
);

CREATE INDEX idx_permissions_role_id ON permissions(role_id);

CREATE TABLE api_tokens (
    id            TEXT PRIMARY KEY,
    tenant_id     TEXT NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    user_id       TEXT REFERENCES users(id) ON DELETE CASCADE,
    name          TEXT NOT NULL,
    token_hash    TEXT NOT NULL UNIQUE,
    scopes        TEXT[] NOT NULL DEFAULT '{}',
    expires_at    TIMESTAMPTZ,
    last_used_at  TIMESTAMPTZ,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_api_tokens_tenant_id ON api_tokens(tenant_id);

-- ============================================================
-- 0002_inventory.sql
-- ============================================================
-- Datacenters, clusters, and hosts: the physical/logical inventory hierarchy.

CREATE TABLE datacenters (
    id          TEXT PRIMARY KEY,
    tenant_id   TEXT NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    name        TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    status      TEXT NOT NULL DEFAULT 'active'
                CHECK (status IN ('active', 'paused', 'archived')),
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (tenant_id, name)
);

CREATE INDEX idx_datacenters_tenant_id ON datacenters(tenant_id);

CREATE TABLE clusters (
    id             TEXT PRIMARY KEY,
    tenant_id      TEXT NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    datacenter_id  TEXT NOT NULL REFERENCES datacenters(id) ON DELETE CASCADE,
    name           TEXT NOT NULL,
    status         TEXT NOT NULL DEFAULT 'active'
                   CHECK (status IN ('active', 'paused')),
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (tenant_id, name)
);

CREATE INDEX idx_clusters_tenant_id ON clusters(tenant_id);
CREATE INDEX idx_clusters_datacenter_id ON clusters(datacenter_id);

CREATE TABLE hosts (
    id                   TEXT PRIMARY KEY,
    tenant_id            TEXT NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    cluster_id           TEXT REFERENCES clusters(id) ON DELETE SET NULL,
    name                 TEXT NOT NULL,
    hostname             TEXT NOT NULL,
    ip_address           TEXT NOT NULL DEFAULT '',
    status               TEXT NOT NULL DEFAULT 'offline'
                         CHECK (status IN ('online', 'offline', 'maintenance')),
    cpu_model            TEXT NOT NULL DEFAULT '',
    cpu_count            INTEGER NOT NULL DEFAULT 0,
    memory_total_bytes   BIGINT NOT NULL DEFAULT 0,
    storage_total_bytes  BIGINT NOT NULL DEFAULT 0,
    os                   TEXT NOT NULL DEFAULT '',
    hypervisor           TEXT NOT NULL DEFAULT 'kvm',
    agent_token_hash     TEXT NOT NULL DEFAULT '',
    numa_node_count      INTEGER NOT NULL DEFAULT 0,
    hugepages_total_kb   BIGINT NOT NULL DEFAULT 0,
    maintenance_mode     BOOLEAN NOT NULL DEFAULT false,
    joined_at            TIMESTAMPTZ,
    last_heartbeat       TIMESTAMPTZ,
    created_at           TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at           TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (tenant_id, name)
);

CREATE INDEX idx_hosts_tenant_id ON hosts(tenant_id);
CREATE INDEX idx_hosts_cluster_id ON hosts(cluster_id);

-- ============================================================
-- 0003_vms.sql
-- ============================================================
-- VMs. The "role" column distinguishes SAP HANA workloads from generic VMs;
-- HANA VMs carry additional guardrail columns enforced by internal/compliance
-- (NUMA pinning, hugepages, dedicated vCPU, no ballooning, no swap) per SAP's
-- HANA hardware/virtualization certification requirements.

CREATE TABLE vms (
    id                        TEXT PRIMARY KEY,
    tenant_id                 TEXT NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    cluster_id                TEXT REFERENCES clusters(id) ON DELETE SET NULL,
    host_id                   TEXT REFERENCES hosts(id) ON DELETE SET NULL,
    identifier                TEXT NOT NULL,
    name                      TEXT NOT NULL,
    description               TEXT NOT NULL DEFAULT '',
    status                    TEXT NOT NULL DEFAULT 'creating'
                              CHECK (status IN ('running', 'stopped', 'paused', 'suspended', 'migrating', 'creating', 'error')),

    -- Workload role. "hana" activates guardrail validation on create/update.
    role                      TEXT NOT NULL DEFAULT 'generic'
                              CHECK (role IN ('generic', 'hana')),

    cpus                      INTEGER NOT NULL,
    cpu_allocation            TEXT NOT NULL DEFAULT 'shared'
                              CHECK (cpu_allocation IN ('dedicated', 'shared')),
    memory_bytes              BIGINT NOT NULL,

    -- HANA guardrail columns. For role='generic' these are typically left at
    -- their permissive defaults; for role='hana' they must reflect a
    -- NUMA-pinned, hugepage-backed, non-overcommitted configuration.
    numa_policy               TEXT,                                  -- e.g. 'strict', 'preferred'; required for HANA
    hugepages_enabled         BOOLEAN NOT NULL DEFAULT false,
    cpu_pinning               JSONB,                                 -- e.g. {"vcpu0":"0","vcpu1":"1"} host CPU pin map
    memory_reservation_bytes  BIGINT NOT NULL DEFAULT 0,              -- hard memory reservation (no overcommit)
    ballooning_allowed        BOOLEAN NOT NULL DEFAULT true,
    swap_allowed              BOOLEAN NOT NULL DEFAULT true,

    os                        TEXT NOT NULL DEFAULT '',
    template_id               TEXT,
    snapshot_count            INTEGER NOT NULL DEFAULT 0,

    created_at                TIMESTAMPTZ NOT NULL DEFAULT now(),
    started_at                TIMESTAMPTZ,
    updated_at                TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (tenant_id, identifier)
);

CREATE INDEX idx_vms_tenant_id ON vms(tenant_id);
CREATE INDEX idx_vms_host_id ON vms(host_id);
CREATE INDEX idx_vms_cluster_id ON vms(cluster_id);
CREATE INDEX idx_vms_role ON vms(role);

-- ============================================================
-- 0004_storage_and_network.sql
-- ============================================================
-- Storage pools, disks, and networks.

CREATE TABLE storage_pools (
    id                   TEXT PRIMARY KEY,
    tenant_id            TEXT NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    datacenter_id        TEXT REFERENCES datacenters(id) ON DELETE SET NULL,
    name                 TEXT NOT NULL,
    type                 TEXT NOT NULL
                         CHECK (type IN ('directory', 'lvm', 'lvm-thin', 'zfs', 'nfs', 'iscsi', 'ceph-rbd')),
    path                 TEXT NOT NULL DEFAULT '',
    status               TEXT NOT NULL DEFAULT 'active'
                         CHECK (status IN ('active', 'inactive', 'error')),
    total_capacity_bytes BIGINT NOT NULL DEFAULT 0,
    free_space_bytes     BIGINT NOT NULL DEFAULT 0,
    used_space_bytes     BIGINT NOT NULL DEFAULT 0,
    features             TEXT[] NOT NULL DEFAULT '{}',
    created_at           TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at           TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (tenant_id, name)
);

CREATE INDEX idx_storage_pools_tenant_id ON storage_pools(tenant_id);

CREATE TABLE networks (
    id            TEXT PRIMARY KEY,
    tenant_id     TEXT NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    datacenter_id TEXT REFERENCES datacenters(id) ON DELETE SET NULL,
    name          TEXT NOT NULL,
    description   TEXT NOT NULL DEFAULT '',
    type          TEXT NOT NULL DEFAULT 'bridge'
                  CHECK (type IN ('bridge', 'nat', 'macvlan')),
    bridge_name   TEXT NOT NULL DEFAULT '',
    vlan_id       INTEGER,
    subnet        TEXT NOT NULL DEFAULT '',
    gateway       TEXT NOT NULL DEFAULT '',
    dhcp          BOOLEAN NOT NULL DEFAULT false,
    dns           TEXT[] NOT NULL DEFAULT '{}',
    status        TEXT NOT NULL DEFAULT 'active'
                  CHECK (status IN ('active', 'inactive')),
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (tenant_id, name)
);

CREATE INDEX idx_networks_tenant_id ON networks(tenant_id);

CREATE TABLE disks (
    id               TEXT PRIMARY KEY,
    tenant_id        TEXT NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    vm_id            TEXT REFERENCES vms(id) ON DELETE CASCADE,
    storage_pool_id  TEXT REFERENCES storage_pools(id) ON DELETE SET NULL,
    name             TEXT NOT NULL,
    size_bytes       BIGINT NOT NULL,
    format           TEXT NOT NULL DEFAULT 'qcow2'
                     CHECK (format IN ('qcow2', 'raw', 'vmdk')),
    path             TEXT NOT NULL DEFAULT '',
    bus              TEXT NOT NULL DEFAULT 'virtio'
                     CHECK (bus IN ('virtio', 'sata', 'ide')),
    mounted          BOOLEAN NOT NULL DEFAULT true,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at       TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_disks_tenant_id ON disks(tenant_id);
CREATE INDEX idx_disks_vm_id ON disks(vm_id);
CREATE INDEX idx_disks_storage_pool_id ON disks(storage_pool_id);

-- ============================================================
-- 0005_backups_and_snapshots.sql
-- ============================================================
-- Backups and snapshots.

CREATE TABLE backups (
    id            TEXT PRIMARY KEY,
    tenant_id     TEXT NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    vm_id         TEXT NOT NULL REFERENCES vms(id) ON DELETE CASCADE,
    name          TEXT NOT NULL,
    status        TEXT NOT NULL DEFAULT 'pending'
                  CHECK (status IN ('pending', 'running', 'completed', 'failed', 'cancelled')),
    type          TEXT NOT NULL DEFAULT 'full'
                  CHECK (type IN ('full', 'incremental')),
    storage_path  TEXT NOT NULL DEFAULT '',
    size_bytes    BIGINT NOT NULL DEFAULT 0,
    progress      INTEGER NOT NULL DEFAULT 0,
    message       TEXT NOT NULL DEFAULT '',
    schedule_id   TEXT,
    started_at    TIMESTAMPTZ,
    completed_at  TIMESTAMPTZ,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_backups_tenant_id ON backups(tenant_id);
CREATE INDEX idx_backups_vm_id ON backups(vm_id);

CREATE TABLE snapshots (
    id           TEXT PRIMARY KEY,
    tenant_id    TEXT NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    vm_id        TEXT NOT NULL REFERENCES vms(id) ON DELETE CASCADE,
    name         TEXT NOT NULL,
    description  TEXT NOT NULL DEFAULT '',
    state        TEXT NOT NULL DEFAULT 'active'
                 CHECK (state IN ('active', 'deleted')),
    size_bytes   BIGINT NOT NULL DEFAULT 0,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_snapshots_tenant_id ON snapshots(tenant_id);
CREATE INDEX idx_snapshots_vm_id ON snapshots(vm_id);

-- ============================================================
-- 0006_compliance_and_events.sql
-- ============================================================
-- Compliance evidence (hash-chained for audit integrity) and the tenant
-- event/audit log (append-only).
--
-- Hash chaining: each evidence row stores previous_hash (the hash of the
-- prior evidence row for the same VM, or a fixed genesis value for the
-- first row) and hash (sha256 over previous_hash + the row's own
-- canonicalized content, computed in internal/compliance before insert).
-- Because each hash commits to the previous one, the chain can be replayed
-- and verified end-to-end to prove no evidence row was altered or removed
-- after the fact — this is what makes the audit trail tamper-evident for
-- SAP HANA compliance reporting.

CREATE TABLE compliance_evidence (
    id             TEXT PRIMARY KEY,
    tenant_id      TEXT NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    vm_id          TEXT NOT NULL REFERENCES vms(id) ON DELETE CASCADE,
    check_type     TEXT NOT NULL,
    check_result   JSONB NOT NULL DEFAULT '{}',
    passed         BOOLEAN NOT NULL,
    evidence       JSONB NOT NULL DEFAULT '{}',
    previous_hash  TEXT NOT NULL,
    hash           TEXT NOT NULL UNIQUE,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_compliance_evidence_tenant_id ON compliance_evidence(tenant_id);
CREATE INDEX idx_compliance_evidence_vm_id ON compliance_evidence(vm_id, created_at);

-- Tenant-scoped audit log. Append-only: application code must never issue
-- UPDATE/DELETE against this table.
CREATE TABLE events (
    id             TEXT PRIMARY KEY,
    tenant_id      TEXT NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    type           TEXT NOT NULL,
    severity       TEXT NOT NULL DEFAULT 'info'
                   CHECK (severity IN ('info', 'warning', 'error', 'critical')),
    message        TEXT NOT NULL DEFAULT '',
    actor_type     TEXT NOT NULL DEFAULT 'system'
                   CHECK (actor_type IN ('user', 'agent', 'system')),
    actor_id       TEXT NOT NULL DEFAULT '',
    actor_name     TEXT NOT NULL DEFAULT '',
    resource_type  TEXT NOT NULL DEFAULT '',
    resource_id    TEXT NOT NULL DEFAULT '',
    resource_name  TEXT NOT NULL DEFAULT '',
    metadata       JSONB NOT NULL DEFAULT '{}',
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_events_tenant_id ON events(tenant_id, created_at DESC);
CREATE INDEX idx_events_type ON events(type);


-- ============================================================
-- 0007_soft_delete.sql
-- ============================================================
ALTER TABLE users ADD COLUMN deleted_at TIMESTAMPTZ;
CREATE INDEX idx_users_not_deleted ON users(tenant_id) WHERE deleted_at IS NULL;

ALTER TABLE vms ADD COLUMN deleted_at TIMESTAMPTZ;
CREATE INDEX idx_vms_not_deleted ON vms(tenant_id) WHERE deleted_at IS NULL;

ALTER TABLE hosts ADD COLUMN deleted_at TIMESTAMPTZ;
CREATE INDEX idx_hosts_not_deleted ON hosts(tenant_id) WHERE deleted_at IS NULL;

ALTER TABLE storage_pools ADD COLUMN deleted_at TIMESTAMPTZ;
CREATE INDEX idx_storage_pools_not_deleted ON storage_pools(tenant_id) WHERE deleted_at IS NULL;

ALTER TABLE networks ADD COLUMN deleted_at TIMESTAMPTZ;
CREATE INDEX idx_networks_not_deleted ON networks(tenant_id) WHERE deleted_at IS NULL;

-- ============================================================
-- 0008_chain_verification.sql
-- ============================================================
CREATE OR REPLACE FUNCTION verify_compliance_evidence_chain(
    p_vm_id     TEXT DEFAULT NULL,
    p_tenant_id TEXT DEFAULT NULL
)
RETURNS TABLE (
    vm_id                  TEXT,
    evidence_id            TEXT,
    evidence_created_at    TIMESTAMPTZ,
    expected_previous_hash TEXT,
    actual_previous_hash   TEXT
)
LANGUAGE sql STABLE AS $$
    SELECT l.vm_id, l.id, l.created_at, l.prior_hash, l.previous_hash
    FROM (
        SELECT ce.vm_id, ce.id, ce.created_at, ce.previous_hash,
               lag(ce.hash) OVER (
                   PARTITION BY ce.vm_id ORDER BY ce.created_at, ce.id
               ) AS prior_hash
        FROM compliance_evidence ce
        WHERE (p_vm_id IS NULL OR ce.vm_id = p_vm_id)
          AND (p_tenant_id IS NULL OR ce.tenant_id = p_tenant_id)
    ) l
    WHERE l.prior_hash IS NOT NULL
      AND l.previous_hash <> l.prior_hash
    ORDER BY l.vm_id, l.created_at, l.id;
$$;

-- Convenience wrapper: true when no broken links exist (an empty chain is valid).
CREATE OR REPLACE FUNCTION is_compliance_evidence_chain_valid(
    p_vm_id     TEXT DEFAULT NULL,
    p_tenant_id TEXT DEFAULT NULL
)
RETURNS BOOLEAN
LANGUAGE sql STABLE AS $$
    SELECT NOT EXISTS (
        SELECT 1 FROM verify_compliance_evidence_chain(p_vm_id, p_tenant_id)
    );
$$;
