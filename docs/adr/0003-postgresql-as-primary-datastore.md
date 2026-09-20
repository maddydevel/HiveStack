# ADR-0003: PostgreSQL as primary datastore

- **Status:** Accepted
- **Date:** 2026-09-20 (recorded retroactively; decision made during initial design)

## Context

The manager needs durable storage for cluster state: nodes, virtual machines,
storage and network definitions, HA policies, users, and audit records. This
state is relational, is updated transactionally (for example, VM placement and
HA state transitions must not be applied partially), and must be queryable for
the REST API and reporting. The manager itself may be made highly available, so
the datastore must support replication and standard backup and restore
tooling.

The alternatives considered were an embedded store such as SQLite or BoltDB,
etcd, and a document database.

## Decision

We will use PostgreSQL as the primary datastore for the manager. Access goes
through the repositories in `internal/db/` using the `pgx/v5` driver. Schema
changes are applied through versioned SQL migrations embedded in the binary
(`internal/db/migrations/`).

## Consequences

- We get ACID transactions, foreign keys, and rich querying for relational
  cluster state.
- PostgreSQL has mature replication, backup, and operational tooling that
  operators already know.
- `pgx/v5` provides a fast native driver with connection pooling.
- Deployments must provision and operate a PostgreSQL instance, which is more
  operational overhead than an embedded database.
- The manager depends on database availability, so the database's own HA
  becomes part of the overall availability story.
- Schema changes must go through migrations and stay backward compatible
  during rolling upgrades.
