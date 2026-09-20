-- Reverts 0007_soft_delete.sql.
-- WARNING: dropping deleted_at makes soft-deleted rows visible again.

DROP INDEX IF EXISTS idx_networks_not_deleted;
ALTER TABLE networks DROP COLUMN IF EXISTS deleted_at;

DROP INDEX IF EXISTS idx_storage_pools_not_deleted;
ALTER TABLE storage_pools DROP COLUMN IF EXISTS deleted_at;

DROP INDEX IF EXISTS idx_hosts_not_deleted;
ALTER TABLE hosts DROP COLUMN IF EXISTS deleted_at;

DROP INDEX IF EXISTS idx_vms_not_deleted;
ALTER TABLE vms DROP COLUMN IF EXISTS deleted_at;

DROP INDEX IF EXISTS idx_users_not_deleted;
ALTER TABLE users DROP COLUMN IF EXISTS deleted_at;
