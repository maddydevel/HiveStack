-- Tenants, users, RBAC, and API tokens.
-- All tenant-scoped tables carry a NOT NULL tenant_id FK so a single Postgres
-- instance can safely serve multiple customers.

CREATE TABLE tenants (
    id          TEXT PRIMARY KEY,
    name        TEXT NOT NULL,
    slug        TEXT NOT NULL UNIQUE,
    status      TEXT NOT NULL DEFAULT 'active'
                CHECK (status IN ('active', 'suspended', 'archived')),
    config      JSONB NOT NULL DEFAULT '{}',
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE users (
    id             TEXT PRIMARY KEY,
    tenant_id      TEXT NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    name           TEXT NOT NULL,
    email          TEXT NOT NULL,
    password_hash  TEXT NOT NULL,
    role           TEXT NOT NULL DEFAULT 'viewer',
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (tenant_id, email)
);

CREATE INDEX idx_users_tenant_id ON users(tenant_id);

-- Custom, tenant-defined roles. Built-in roles (admin, operator, viewer,
-- hana-operator, compliance-auditor) are defined in code (internal/auth/rbac.go)
-- and do not require rows here; this table lets a tenant layer additional
-- custom roles/permissions on top of the built-ins.
CREATE TABLE roles (
    id          TEXT PRIMARY KEY,
    tenant_id   TEXT NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    name        TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (tenant_id, name)
);

CREATE INDEX idx_roles_tenant_id ON roles(tenant_id);

CREATE TABLE permissions (
    id        TEXT PRIMARY KEY,
    role_id   TEXT NOT NULL REFERENCES roles(id) ON DELETE CASCADE,
    resource  TEXT NOT NULL,
    action    TEXT NOT NULL,
    UNIQUE (role_id, resource, action)
);

CREATE INDEX idx_permissions_role_id ON permissions(role_id);

CREATE TABLE api_tokens (
    id            TEXT PRIMARY KEY,
    tenant_id     TEXT NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    user_id       TEXT REFERENCES users(id) ON DELETE CASCADE,
    name          TEXT NOT NULL,
    token_hash    TEXT NOT NULL UNIQUE,
    scopes        TEXT[] NOT NULL DEFAULT '{}',
    expires_at    TIMESTAMPTZ,
    last_used_at  TIMESTAMPTZ,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_api_tokens_tenant_id ON api_tokens(tenant_id);
