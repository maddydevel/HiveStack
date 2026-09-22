-- Reverts 0001_tenants_and_auth.sql. Tables are dropped in reverse
-- dependency order (children before parents).

DROP INDEX IF EXISTS idx_api_tokens_tenant_id;
DROP TABLE IF EXISTS api_tokens;

DROP INDEX IF EXISTS idx_permissions_role_id;
DROP TABLE IF EXISTS permissions;

DROP INDEX IF EXISTS idx_roles_tenant_id;
DROP TABLE IF EXISTS roles;

DROP INDEX IF EXISTS idx_users_tenant_id;
DROP TABLE IF EXISTS users;

DROP TABLE IF EXISTS tenants;
