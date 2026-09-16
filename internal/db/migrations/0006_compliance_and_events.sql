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
