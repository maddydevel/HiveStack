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
