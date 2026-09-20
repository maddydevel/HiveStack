-- Soft delete for key tables. Rows are marked with deleted_at instead of being
-- removed; a partial index over live rows keeps tenant-scoped "active" lookups
-- cheap. Queries that should hide deleted rows must add "deleted_at IS NULL".
--
-- Note: the UNIQUE (tenant_id, ...) constraints from earlier migrations still
-- apply to soft-deleted rows, so a deleted user's email or a deleted VM's
-- identifier/name cannot be reused until the row is purged.

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
