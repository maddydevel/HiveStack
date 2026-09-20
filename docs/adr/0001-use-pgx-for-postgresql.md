# ADR-0001: Use pgx for PostgreSQL Connectivity

## Status

Accepted

## Context

HiveStack requires persistent storage for tenants, users, VMs, backups, audit logs, and compliance data. PostgreSQL was selected as the relational database (see ADR scope: PostgreSQL is the sole supported RDBMS). We need a Go driver/library to interact with PostgreSQL.

Options considered:

1. **`database/sql` + `lib/pq`** — The traditional `lib/pq` driver. Mature but in maintenance mode; lacks support for newer PostgreSQL features (e.g., `pgx`-native batching, `COPY` streaming, logical replication).

2. **`pgx` via `pgx/v5` with `database/sql` compatibility** — The `pgx` library offers a native driver plus a `database/sql`-compatible shim. Supports advanced features: array types, JSONB, `COPY` protocol, prepared statement caching, connection pooling (`pgpool` via `pgxpool`), listen/notify.

3. **`gORM` with PostgreSQL dialect** — ORM approach. Adds abstraction overhead; complex queries (CTEs, window functions) are harder to express; migration tooling is opinionated.

## Decision

Use **`jackc/pgx/v5`** as the primary PostgreSQL connectivity layer.

- The native `pgx` interface is used for new code to leverage batching, `COPY`, and `pgxpool` connection pooling.
- A `database/sql` compatibility shim is retained where existing tooling requires it (e.g., `sql-migrate`).

Connection pooling is handled by `pgxpool` (built into `pgx/v5`). The `internal/db` package wraps pool lifecycle (open, close, ping, migrate) and exposes a thin interface for repositories.

## Consequences

### Positive

- Access to PostgreSQL-specific features: `COPY` for bulk imports (audit logs), `pgxpool` for connection pooling without external dependencies.
- Type safety: `pgx` has first-class support for `uuid`, `jsonb`, `inet`, `timestamp with time zone`, and custom enums.
- Performance: The native protocol avoids the overhead of the `database/sql` abstraction layer.

### Negative

- Team must learn `pgx`-specific APIs (slightly different from `database/sql`).
- Cannot easily swap to another RDBMS in the future (not a current requirement).

## References

- `github.com/jackc/pgx/v5` — go.mod dependency.
- `internal/db/db.go` — Connection pool and migration entry point.
- `internal/db/schema.sql` — Initial schema.
