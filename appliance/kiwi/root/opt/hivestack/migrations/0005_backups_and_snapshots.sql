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
