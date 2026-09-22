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
