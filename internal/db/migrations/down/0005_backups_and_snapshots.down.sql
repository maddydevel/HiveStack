-- Reverts 0005_backups_and_snapshots.sql.

DROP INDEX IF EXISTS idx_snapshots_vm_id;
DROP INDEX IF EXISTS idx_snapshots_tenant_id;
DROP TABLE IF EXISTS snapshots;

DROP INDEX IF EXISTS idx_backups_vm_id;
DROP INDEX IF EXISTS idx_backups_tenant_id;
DROP TABLE IF EXISTS backups;
