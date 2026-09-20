# HiveStack Architecture

HiveStack is a KVM/libvirt virtualization management platform: a **Manager**
control plane plus a **Node agent** on every hypervisor host. This page is the
short tour. For depth, see [HLD](HLD.md), [LLD](LLD.md), [HA](HA.md), and the
[ADRs](adr/).

## Components

```
                 ┌──────────────────────────────────────────────┐
  hive CLI ─────▶│                 HiveStack Manager            │
  Web UI   ─────▶│  REST API ─▶ auth/RBAC ─▶ manager services   │
  Prometheus ───▶│  (internal/api)  (internal/auth)             │
                 │        │                    │                │
                 │  HA controller        compliance + events    │
                 │  (internal/ha)   (internal/compliance,events)│
                 └────────┬───────────────────┬─────────────────┘
                          │ gRPC + mTLS       │ pgx/v5
                          ▼                   ▼
              ┌───────────────────┐    ┌──────────────┐
              │   Node agent(s)   │    │  PostgreSQL  │
              │ internal/node     │    └──────────────┘
              │ libvirt / KVM     │
              └───────────────────┘
```

| Component | Package | Responsibility |
|-----------|---------|----------------|
| REST API | `internal/api` | HTTP routing (stdlib `ServeMux`), request handling, JWT auth per route |
| Auth | `internal/auth` | JWT issue/validate, Argon2id password hashing, RBAC engine |
| Manager services | `internal/manager` | Host and VM lifecycle, HANA compliance enforcement |
| Database | `internal/db` | pgx pool, embedded SQL migrations, repositories |
| HA | `internal/ha` | Health tracking, failure detection, fencing, scheduling, VM restart |
| Compliance | `internal/compliance` | HANA guardrail validation, hash-chained evidence, drift detection |
| Events | `internal/events` | Pub/sub event bus, persisted to the `events` table |
| Node agent | `internal/node`, `node/` | gRPC server: `Register`, `Heartbeat`, `ExecuteCommand`, `GetStatus`, `ListVMs` |
| Libvirt | `internal/libvirt` | KVM/QEMU wrapper used by the node agent |
| TLS | `internal/tls` | CA and certificate generation, TLS 1.3 server/client configs |
| Security | `internal/security` | Security headers, rate limiting, audit log, AppArmor profiles, LUKS |
| Secrets | `internal/secrets` | SOPS and Vault integrations |
| Metrics | `internal/metrics` | Prometheus metrics served at `/metrics` |
| Migration | `internal/migration`, `migration/` | VMware VMX/OVF/vCenter import |
| Clients | `cmd/hive`, `pkg/api`, `web/` | CLI, Go API client, React web UI |

Entry points: `cmd/hive-manager` (`run`, `init`, `migrate`, `validate`,
`version`), `cmd/hive` (CLI), and `node/` (node agent).

## Key interactions

**API request.** Client sends `Authorization: Bearer <jwt>`. `auth.RequireAuth`
validates the token and stores the claims (user, tenant, scopes) in the request
context. The handler scopes reads by the tenant in the claims and calls the
database or manager service.

**Node lifecycle.** A node agent calls `Register`, then sends `Heartbeat` every
30 seconds. The manager commands VM operations through `ExecuteCommand` and reads
state through `GetStatus` and `ListVMs`. All manager-node traffic is gRPC over
mutual TLS ([ADR-0002](adr/0002-use-grpc-for-node-agent-communication.md)).

**HA failover.** The HA controller marks a host suspect and then offline when
heartbeats stop. The fencer powers the host off (IPMI, Redfish, or SSH) so its
VMs cannot run twice. The scheduler picks target hosts (binpack, spread, or
NUMA-aware) and the orchestrator restarts each VM according to its HA policy.
Target RTO is 5 minutes.

**Compliance evidence.** A guardrail check for a HANA VM produces a result that
is stored as a row chained to the previous row by SHA-256
([ADR-0004](adr/0004-use-hash-chain-for-compliance.md)). Drift detection
compares the current profile to the last compliant baseline and publishes an
event.

## Data model

PostgreSQL is the only datastore
([ADR-0003](adr/0003-postgresql-as-primary-datastore.md)). Migrations live in
`internal/db/migrations/`, are embedded in the binary, and are applied in order
by `hive-manager migrate` (tracked in `schema_migrations`).

- Tenancy: `tenants` own users, hosts, VMs, storage, networks, backups, evidence,
  and events. Every tenant-owned table carries `tenant_id`.
- Inventory: datacenters → clusters → hosts → VMs, with disks, storage pools,
  networks, snapshots, and backups.
- Audit: `events` (append-only log) and `compliance_evidence` (hash-chained).
- Deletion: soft delete on key tables ([ADR-0005](adr/0005-soft-delete-pattern.md)).

## Network surfaces

| Port | Protocol | Purpose |
|------|----------|---------|
| 8080 | HTTP | REST API (development, no TLS) |
| 8443 | HTTPS / gRPC over TLS 1.3 | REST API with TLS; node agent connections |
| 5432 | PostgreSQL | Manager to database only |

## Implementation status

Some components described above are implemented but not yet wired into the
running manager. Check before relying on them:

- `hive-manager run` currently connects to the database and waits. It does not
  yet start the API server, and it skips migrations (run `hive-manager migrate`
  first).
- `internal/security` (security headers, rate limiting, audit logger) is not
  referenced by the API server.
- RBAC is enforced on the HA failover and VM update handlers only; other routes
  require a valid JWT but do not check the role.

The [threat model](threat-model.md) tracks these as open items.
