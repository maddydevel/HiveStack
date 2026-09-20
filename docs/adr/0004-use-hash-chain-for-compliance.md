# ADR-0004: Use hash-chained evidence for compliance records

- **Status:** Accepted (chain linking and link verification implemented; row-hash verification and external anchoring are open follow-ups)
- **Date:** 2026-09-20 (recorded retroactively; decision made during the compliance design)

## Context

HiveStack enforces SAP HANA guardrails on VMs (NUMA, hugepages, dedicated CPUs,
no ballooning, no swap, full memory reservation) and records every check as
evidence for audit. Auditors and hosting customers need to trust that a
historical check result was not edited, and that a failing result was not
quietly removed, after the fact.

Plain rows in a PostgreSQL table cannot provide that: anyone with write access
can change or delete them without a trace. The alternatives considered were an
append-only table protected only by permissions, an external immutable log
service, and cryptographically signed records.

## Decision

Each compliance check is stored as a row in `compliance_evidence` with two
extra columns:

- `previous_hash`: the `hash` of the prior evidence row for the same VM, or a
  caller-defined genesis value (an empty string in `GenerateEvidence`) for the
  first row.
- `hash`: a SHA-256 digest that commits to `previous_hash` and the row's own
  content. `hash` is `UNIQUE`.

Hashes are computed in `internal/compliance` before insert. `RecordEvidence`
uses `HashEvidence(previousHash, evidence)`. `GenerateEvidence` hashes the
previous hash together with the profile JSON, violation count, pass/fail flag,
timestamp, and checker. Chains are per VM and ordered by `created_at`, then `id`.

Verification is available in two equivalent forms, both of which check the
links between consecutive rows:

- `ComplianceStore.VerifyChain` in Go.
- `verify_compliance_evidence_chain(vm_id, tenant_id)` and
  `is_compliance_evidence_chain_valid(...)` in SQL (migration `0008`), so
  operators can audit directly in `psql`.

The chain is a tamper-evidence mechanism, not tamper-proofing. It makes silent
changes detectable; it does not prevent them.

## Consequences

- Deleting or reordering a row in the middle of a chain, or altering a row's
  `hash` or `previous_hash`, breaks a link and is detected.
- Verification is cheap and needs no key material, so any operator can run it.
- Known limits, which operators and auditors must understand:
  - Only links are checked. Row hashes are **not recomputed**, so editing
    `evidence` or `check_result` without touching `hash` is not detected. SQL
    cannot recompute them because the two writers hash different inputs and
    JSONB does not preserve the original JSON bytes.
  - The first row of a chain is not checked, and truncating the newest rows is
    not detected.
  - There is no secret in the hash, so someone with database write access can
    recompute and rewrite a chain from any point onward consistently.
  - The per-process mutex in `ComplianceStore` serializes writers within one
    manager only. Concurrent managers can fork a chain because nothing enforces
    one successor per `previous_hash`.
  - `compliance_evidence.vm_id` is `ON DELETE CASCADE`, so hard-deleting a VM
    deletes its evidence. See [ADR-0005](0005-soft-delete-pattern.md).
- Follow-ups to close these gaps:
  1. Canonicalize the hash input, use one hash function for all writers, and
     verify by recomputing row hashes.
  2. Periodically anchor the latest hash per tenant outside the database
     (signed, or written to the syslog/SIEM forwarder) so truncation and full
     rewrites are detectable.
  3. Add a unique constraint on `(vm_id, previous_hash)` to prevent forks, and
     revoke `UPDATE`/`DELETE` on the table from the application role.
  4. Make `GenerateEvidence` write the real `tenant_id`. It currently inserts
     an empty string, which the foreign key to `tenants(id)` rejects.
