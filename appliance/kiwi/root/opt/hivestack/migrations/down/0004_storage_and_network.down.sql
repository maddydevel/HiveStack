-- Reverts 0004_storage_and_network.sql. disks references storage_pools, so it
-- is dropped first.

DROP INDEX IF EXISTS idx_disks_storage_pool_id;
DROP INDEX IF EXISTS idx_disks_vm_id;
DROP INDEX IF EXISTS idx_disks_tenant_id;
DROP TABLE IF EXISTS disks;

DROP INDEX IF EXISTS idx_networks_tenant_id;
DROP TABLE IF EXISTS networks;

DROP INDEX IF EXISTS idx_storage_pools_tenant_id;
DROP TABLE IF EXISTS storage_pools;
