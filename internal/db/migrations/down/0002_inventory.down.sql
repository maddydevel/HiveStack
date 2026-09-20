-- Reverts 0002_inventory.sql.

DROP INDEX IF EXISTS idx_hosts_cluster_id;
DROP INDEX IF EXISTS idx_hosts_tenant_id;
DROP TABLE IF EXISTS hosts;

DROP INDEX IF EXISTS idx_clusters_datacenter_id;
DROP INDEX IF EXISTS idx_clusters_tenant_id;
DROP TABLE IF EXISTS clusters;

DROP INDEX IF EXISTS idx_datacenters_tenant_id;
DROP TABLE IF EXISTS datacenters;
