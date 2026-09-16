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
