# HiveStack Service Level Objectives & Agreements

**Version:** 1.0.0
**Last Updated:** 2026-09-19
**Owner:** HiveStack Operations
**Status:** Draft — target framework for GA. HiveStack is currently v0.1.0
(pre-GA); these are the objectives the platform is engineered against and the
thresholds `docs/RUNBOOKS.md` and alerting are built around, not yet a live
production track record. Section 3 (SLA) is a template for customer-facing
commitments and must be reviewed by Sales/Legal before it is quoted externally.

## Table of Contents

- [1. Service Overview](#1-service-overview)
- [2. Service Level Objectives (SLOs)](#2-service-level-objectives-slos)
- [3. Service Level Agreement (SLA) Template](#3-service-level-agreement-sla-template)
- [4. Monitoring & Measurement](#4-monitoring--measurement)
- [5. Incident Severity Mapping](#5-incident-severity-mapping)
- [6. Review & Change Process](#6-review--change-process)

---

## 1. Service Overview

HiveStack is a self-hosted KVM/libvirt virtualization appliance for SLES 15
SP7: a Manager control plane (REST API + CLI + Web UI), a fleet of Node
Agents (gRPC), Postgres-backed state, an HA subsystem for non-HANA
workloads, and HANA-specific compliance guardrails. See
[`docs/ARCHITECTURE.md`](ARCHITECTURE.md) for the full component design.

### Service Components

| Component | Description | Criticality |
|-----------|-------------|-------------|
| Manager API | REST API (`internal/api`), 100+ endpoints, JWT + RBAC | Critical |
| Manager UI | React/TS web console, pure REST client | Critical |
| Node Agent | Per-host gRPC agent driving libvirt/QEMU | Critical |
| HA Controller | Host failure detection, fencing, VM restart (`internal/ha`) | Critical |
| Database | PostgreSQL via pgx/v5 — source of truth for all state | Critical |
| Event Bus | Pub/sub, 17 event types, DB-backed (`internal/events`) | High |
| Compliance Engine | HANA guardrail enforcement + evidence chain (`internal/compliance`) | High |
| Shared Storage (HA) | NFS, Ceph RBD, or iSCSI backing for failover (`internal/ha/storage.go`) | High |
| Monitoring | `/metrics` Prometheus endpoint (`internal/metrics`) | Medium |
| VMware Migration (VMX import) | VMX parser → HiveStack VM spec | Low — not yet GA (see README status table) |

---

## 2. Service Level Objectives (SLOs)

### 2.1 Availability SLOs

| Metric | Target | Measurement Period | Calculation |
|--------|--------|--------------------|--------------|
| Manager API uptime | 99.9% | Rolling 30 days | `GET /health` synthetic probe success rate |
| Manager UI uptime | 99.9% | Rolling 30 days | Same as Manager API (UI has no separate backend) |
| HA-enabled VM availability | 99.95% | Rolling 30 days | VM running-time / total time, including auto-restart |
| Node Agent connectivity | 99.5% per node | Rolling 30 days | gRPC heartbeat success rate |

**Error budget:** 99.9% monthly = ~43 minutes of allowed Manager downtime.

### 2.2 Latency / Performance SLOs

| Metric | Target | Source | Alert Threshold |
|--------|--------|--------|------------------|
| API request latency (p50) | < 100 ms | `hivestack_api_request_duration_seconds` | > 200 ms |
| API request latency (p99) | < 500 ms | `hivestack_api_request_duration_seconds` | > 1000 ms |
| VM create → running | < 30 s | Manager VM lifecycle | > 60 s |
| VM start → running | < 15 s | Manager VM lifecycle | > 30 s |
| `/health` and `/metrics` scrape | < 500 ms | Prometheus scrape duration | > 2 s |

### 2.3 High Availability SLOs

These mirror the targets already committed to in [`docs/HA.md`](HA.md#sla-targets)
for the non-HANA HA subsystem — restated here so operations tracks them as
top-level SLOs, not just an HA implementation detail:

| Metric | Target |
|--------|--------|
| Failure detection | < 150 seconds |
| Fencing completion | < 30 seconds |
| VM restart initiation | < 30 seconds |
| Total recovery time objective (RTO) | < 5 minutes |
| Concurrent failovers supported | Up to 5 nodes |
| Max VMs per failover batch | 50 VMs |

Recovery point objective (RPO): HA restart is process-level (no data loss for
running VMs on shared storage); it is not a backup/DR mechanism. Backup/restore
RPO is governed by backup schedule, not by this HA path.

### 2.4 Reliability SLOs

| Metric | Target | Measurement | Alert Threshold |
|--------|--------|-------------|------------------|
| Backup success rate | > 99.0% | Successful / total backup jobs | < 95% |
| Restore success rate | > 99.5% | Successful / total restore jobs | < 98% |
| Manager process availability | `hivestack_manager_up == 1` | Continuous | Any transition to 0 |
| Compliance evidence chain integrity | 100% | Hash-chain verification (`internal/compliance`) | Any break |

### 2.5 Out of scope for GA SLOs

- **VMware migration (VMX import):** marked low-priority / not targeted for
  first stable release (see `README.md` status table). No SLO is committed
  until it ships.
- **Live migration downtime:** the platform does not yet implement live
  (hot) migration between nodes; do not quote a migration-downtime SLO until
  that feature lands.

---

## 3. Service Level Agreement (SLA) Template

This section is the starting point for a customer-facing agreement once
HiveStack reaches GA. Numbers are illustrative and must be confirmed against
actual operating history before being offered externally.

### 3.1 Support Tiers

| Priority | Definition | Target Response | Target Resolution |
|----------|------------|------------------|--------------------|
| P1 — Critical | Manager unreachable, cluster-wide outage, data loss risk | 15 minutes | 4 hours |
| P2 — High | HA/failover broken, API returning 5xx broadly | 1 hour | 8 hours |
| P3 — Medium | Degraded performance, single-node impact | 4 hours | 24 hours |
| P4 — Low | Cosmetic issue, non-blocking question | 1 business day | Best effort |

### 3.2 Support Hours

- Standard coverage: Monday–Friday during customer's local business hours.
- After-hours: P1/P2 only, via on-call rotation (see `docs/RUNBOOKS.md`
  Emergency Procedures).

### 3.3 Uptime Commitment (Draft — pending commercial sign-off)

| Tier | Monthly Manager Uptime | Suggested Credit on Breach |
|------|------------------------|------------------------------|
| Standard | 99.5% | To be defined by Sales/Legal |
| Enterprise | 99.9% | To be defined by Sales/Legal |

### 3.4 Exclusions

- Scheduled maintenance windows (see `docs/MAINTENANCE.md`), announced in advance.
- Customer-caused outages (misconfigured fencing, exhausted shared storage,
  unsupported topology).
- Underlying infrastructure/hardware failures outside HiveStack's control
  (host hardware, network fabric, upstream cloud/datacenter).
- Features explicitly marked pre-GA in `README.md`.

---

## 4. Monitoring & Measurement

### 4.1 Metrics source

All SLOs above are computed from the Manager's Prometheus endpoint
(`GET /metrics`, implemented in `internal/api/server.go` `handleMetrics`,
metrics registered in `internal/metrics/metrics.go`). Key series:

| Metric | Type | Used for |
|--------|------|----------|
| `hivestack_manager_up` | Gauge | Manager availability SLO |
| `hivestack_api_requests_total` | Counter | API error-rate / traffic |
| `hivestack_api_request_duration_seconds` | Histogram | API latency SLOs (2.2) |
| `hivestack_database_connections_active` | Gauge | DB saturation, early-warning for latency SLOs |
| `hivestack_ha_nodes_online` / `_suspect` / `_offline` | Gauge | Node connectivity SLO (2.1) |
| `hivestack_ha_failover_total` | Counter | HA reliability |
| `hivestack_ha_failover_duration_seconds` | Histogram | HA RTO SLO (2.3) |
| `hivestack_ha_fencing_total` | Counter | Fencing completion SLO (2.3) |
| `hivestack_events_published_total` | Counter | Event bus health |
| `hivestack_hana_vm_compliance_status` | Gauge | Compliance evidence SLO (2.4) |

### 4.2 Dashboards

- HA subsystem: [`dashboards/hivestack-ha.json`](../dashboards/hivestack-ha.json)
  (node status, failover duration, fencing outcomes).
- Operations overview dashboard (API latency, uptime, backup/restore rates)
  is tracked as a separate deliverable — see EVO-178 — and should be linked
  here as `dashboards/hivestack-operations.json` once it lands.

### 4.3 Burn-rate alerting (target design)

| Alert | Condition | Severity | Action |
|-------|-----------|----------|--------|
| SLOBurnRateFast | 2% of monthly error budget consumed in 1 hour | Critical | Page on-call |
| SLOBurnRateMedium | 5% of monthly error budget consumed in 6 hours | High | Slack + email |
| SLOBurnRateSlow | 10% of monthly error budget consumed in 3 days | Medium | Email digest |

Alertmanager rules implementing this table are not yet checked in; this is
the spec to implement against when alerting rules are added.

### 4.4 Reporting

Once in production, a monthly SLO report should cover: actual vs. target per
metric in §2, remaining error budget, incident summary with links to
postmortems (see [`docs/INCIDENT_RESPONSE.md`](INCIDENT_RESPONSE.md)), and
recommended target adjustments.

---

## 5. Incident Severity Mapping

Severity definitions are owned by [`docs/INCIDENT_RESPONSE.md`](INCIDENT_RESPONSE.md);
this table maps them to which SLOs they burn:

| Severity | Example | SLOs affected |
|----------|---------|----------------|
| SEV1 | Manager unreachable, cluster-wide data loss | Availability (2.1), Manager uptime |
| SEV2 | HA failover not completing, API 5xx broadly | HA (2.3), Latency (2.2) |
| SEV3 | Degraded latency, single-node HA issue | Latency (2.2), Node connectivity (2.1) |
| SEV4 | Cosmetic/non-blocking | None (tracked, not budget-affecting) |

---

## 6. Review & Change Process

- **Quarterly:** review actual metrics (once production telemetry exists)
  against targets in §2; adjust with rationale recorded in this file's git
  history.
- **Post-incident:** any SEV1/SEV2 postmortem (per `docs/INCIDENT_RESPONSE.md`)
  must state whether it changes an SLO target or reveals a missing one.
- **Before GA:** §3 must be reviewed by Sales/Legal and the illustrative
  figures replaced with committed terms before this document is referenced
  in a customer contract.

*Last updated: 2026-09-19*
