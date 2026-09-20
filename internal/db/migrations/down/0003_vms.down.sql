-- Reverts 0003_vms.sql.

DROP INDEX IF EXISTS idx_vms_role;
DROP INDEX IF EXISTS idx_vms_cluster_id;
DROP INDEX IF EXISTS idx_vms_host_id;
DROP INDEX IF EXISTS idx_vms_tenant_id;
DROP TABLE IF EXISTS vms;
