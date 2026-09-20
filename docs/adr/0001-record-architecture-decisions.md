# ADR-0001: Record architecture decisions

- **Status:** Accepted
- **Date:** 2026-09-20

## Context

HiveStack is a KVM/libvirt hypervisor management platform made up of a
manager control plane, a REST API, and node agents. Decisions about its
structure, dependencies, and protocols shape the system for a long time, and
the reasoning behind them is easily lost as contributors change. We need a
lightweight way to record what was decided and why.

## Decision

We will record architecturally significant decisions as Architecture Decision
Records (ADRs), following the format described by Michael Nygard in
["Documenting Architecture Decisions"](https://cognitect.com/blog/2011/11/15/documenting-architecture-decisions).

- ADRs live in `docs/adr/` and are named `NNNN-short-title.md`, numbered
  sequentially starting at `0001`.
- Each ADR has a title, status, context, decision, and consequences.
- Status is one of: `Proposed`, `Accepted`, `Deprecated`, or
  `Superseded by ADR-NNNN`.
- ADRs are immutable once accepted. If a decision changes, write a new ADR
  that supersedes the old one and update the old one's status.
- ADRs are proposed and reviewed through pull requests like any other change.

### Template

Copy the following into a new `docs/adr/NNNN-short-title.md`:

```markdown
# ADR-NNNN: Title

- **Status:** Proposed | Accepted | Deprecated | Superseded by ADR-NNNN
- **Date:** YYYY-MM-DD

## Context

The forces at play: technical, organizational, and project constraints. State
them neutrally.

## Decision

The response to those forces, in full sentences, active voice ("We will ...").

## Consequences

What becomes easier or harder as a result, including positive, negative, and
neutral outcomes.
```

## Consequences

- New contributors can learn why the system is shaped as it is without
  relying on tribal knowledge.
- Decisions get reviewed before they are implemented.
- Writing an ADR adds a small amount of overhead to significant changes.
- Decisions made before this ADR was adopted are recorded retroactively
  (see ADR-0002 and ADR-0003).

## Index

| ADR | Title | Status |
|-----|-------|--------|
| [0001](0001-record-architecture-decisions.md) | Record architecture decisions | Accepted |
| [0002](0002-use-grpc-for-node-agent-communication.md) | Use gRPC for node-agent communication | Accepted |
| [0003](0003-postgresql-as-primary-datastore.md) | PostgreSQL as primary datastore | Accepted |
