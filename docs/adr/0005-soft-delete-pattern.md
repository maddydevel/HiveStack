# ADR-0005: Soft delete for tenant-owned inventory

- **Status:** Accepted (schema implemented in migration `0007`; application adoption pending)
- **Date:** 2026-09-20 (recorded retroactively; decision made during the schema design)

## Context

Users, VMs, hosts, storage pools, and networks are referenced by other records:
compliance evidence, events, backups, snapshots, and HA history. Hard-deleting a
row either fails on a foreign key or, with `ON DELETE CASCADE`, silently removes
dependent history. In particular, `compliance_evidence.vm_id` cascades from
`vms`, so hard-deleting a VM would delete its audit trail (see
[ADR-0004](0004-use-hash-chain-for-compliance.md)).

Operators also delete things by mistake, and need a recovery window.

The alternatives considered were hard delete with a separate archive table, a
`status` column with a `deleted` value, and hard delete relying on backups.

## Decision

We will soft-delete the following tables by setting a nullable
`deleted_at TIMESTAMPTZ` column: `users`, `vms`, `hosts`, `storage_pools`, and
`networks` (migration `0007_soft_delete.sql`). A row with `deleted_at IS NULL` is
live.

Each table gets a partial index, for example
`idx_vms_not_deleted ON vms(tenant_id) WHERE deleted_at IS NULL`, so tenant-scoped
lookups of live rows stay cheap.

Rules for application code:

- Delete operations must `UPDATE ... SET deleted_at = now()` and must not issue
  `DELETE`.
- Every query that should hide deleted rows must add `deleted_at IS NULL`.
  Nothing enforces this automatically.
- Restoring is `SET deleted_at = NULL`. Purging (a real `DELETE`) is a separate,
  deliberate, retention-driven operation.

The down migration drops the columns. This makes soft-deleted rows visible again,
and the file says so.

## Consequences

- Dependent history (evidence, events, backups) survives deletion of its parent.
- Accidental deletes are recoverable until purged.
- Existing `UNIQUE (tenant_id, ...)` constraints still apply to soft-deleted rows,
  so a deleted user's email or a deleted VM's name cannot be reused until the row
  is purged. Converting them to partial unique indexes (`WHERE deleted_at IS NULL`)
  is a follow-up.
- Every read path carries an extra predicate, and forgetting it leaks deleted
  data. Repository helpers should centralize it.
- Data that must be erased (for example on a tenant's request) needs an explicit
  purge job. Soft delete alone is not erasure.
- **Current gap:** the schema is in place but the application does not use it yet.
  The `Delete*` methods in `internal/db/repositories.go` still issue hard
  `DELETE` statements and the read queries do not filter on `deleted_at`. Those
  queries also target singular table names (`vm`, `host`, `network`,
  `storage_pool`) that do not match the migrated plural names (`vms`, `hosts`,
  `networks`, `storage_pools`), so that mismatch must be fixed first.
- Follow-ups: switch repository deletes to `UPDATE`, add `deleted_at IS NULL` to
  reads, add partial unique indexes, and add a retention-based purge job.
