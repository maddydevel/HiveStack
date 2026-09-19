# HiveStack

KVM-based virtualization appliance on SUSE SLES 15 SP7 — a VMware vCenter/ESXi migration target with vCenter-like management, ESXi-like hypervisor nodes, and multi-tenant hosting platform capabilities.

HiveStack is designed for:
- **Enterprise IT departments** running SAP or non-SAP workloads on-premises
- **VMware shops migrating off VMware** who also happen to run SAP HANA
- **SAP HANA hosting providers / MSPs** building a hosting platform

## Status

**v0.1.0 — Core platform complete, 8,697 lines Go across 18 packages.**

Build: `go build ./...` ✅ | Vet: `go vet ./...` ✅

| Component | Status | Description |
|-----------|--------|-------------|
| Auth (JWT + Argon2id) | ✅ | Tenant-scoped JWT auth, Argon2id password hashing |
| RBAC (5 roles) | ✅ | admin/editor/viewer/operator, permission-based access |
| DB layer | ✅ | pgxpool, 12 tables, migrations, full CRUD repos |
| API Server | ✅ | 100+ REST endpoints, stdlib ServeMux, auth middleware |
| CLI (15+ commands) | ✅ | cobra CLI with table/JSON/YAML output |
| Node Agent gRPC | ✅ | 5 gRPC handlers: Register/Heartbeat/ExecuteCommand/GetStatus/ListVMs |
| Manager service | ✅ | Full CRUD for hosts and VMs, HANA compliance enforcement |
| Event framework | ✅ | Pub/sub with 17 event types, DB-backed |
| Compliance evidence | ✅ | Hash-chained evidence, drift detection, chain verification |
| HANA guardrails | ✅ | NUMA, hugepages, dedicated CPUs, no ballooning, no swap |
| VMX parser | ✅ | VMware VMX → HiveStack VM config converter |
| API client | ✅ | Go HTTP client with auth, CRUD for all resources |
| Systemd units | ✅ | Manager + Node Agent service files with security hardening |
| Prometheus metrics | ✅ | /metrics endpoint, host/VM/cluster/HANA metrics |
| Structured logging | ✅ | log.Printf/log.Println (slog-compatible structured output) |
| KIWI appliance | ✅ | SLES 15 SP7 appliance packaging with first-boot initialization |
| Web UI | ✅ | React 18 + TypeScript + Vite + Tailwind (303KB JS, 15 pages) |
| HA (non-HANA) | 🔲 | Host failure detection + VM restart (planned) |
| Security hardening | ✅ | TLS 1.3, mTLS gRPC, Vault/SOPS secrets, AppArmor, LUKS, rate limiting |
| VMware migration | 🔲 | Low priority, skip in first stable version |
| SLOs & capacity | 🔲 | Definition + enforcement (planned) |

## Quick Start

### Prerequisites

- Go 1.26+
- PostgreSQL (or use the embedded database for development)
- SUSE SLES 15 SP7 (for appliance builds)

### Build

```bash
go build -o bin/hive-manager ./cmd/hive-manager
go build -o bin/hive ./cmd/hive
go build -o bin/hive-node ./node
```

### Run CLI (development)

```bash
./bin/hive host list
./bin/hive vm list
./bin/hive vm create my-vm --cpus 4 --memory 8GiB
./bin/hive vm start my-vm
./bin/hive vm stop my-vm
./bin/hive backup create --vm my-vm --name daily-snap
./bin/hive compliance check my-vm
./bin/hive compliance evidence my-vm
./bin/hive events list --limit 50
```

Run `./bin/hive --help` for full command reference.

## Architecture

### Components

- **HiveStack Manager**: Control plane — REST API, CLI, auth, RBAC, PostgreSQL database, event framework, compliance enforcement
- **HiveStack Node**: Compute node — KVM/QEMU/libvirt, node agent (gRPC), SLES 15 SP7

### Technology Stack

| Component | Technology |
|-----------|------------|
| Manager backend | Go 1.26 |
| CLI | Go, cobra |
| Web UI (planned) | React 18 + TypeScript + Vite + Tailwind CSS |
| Database | PostgreSQL (pgx/v5 driver) |
| Virtualization | KVM + QEMU + libvirt |
| Node agent | Go, gRPC |
| HA (HANA VMs) | SAP HANA System Replication + Pacemaker (customer-managed) |
| HA (non-HANA VMs) | Host failure detection + automatic VM restart (planned) |
| Monitoring | Prometheus + Grafana, structured logging (slog) |
| Packaging (planned) | KIWI appliance description for SLES 15 SP7 |

### Directory Structure

```
cmd/hive/                # CLI main entry point
cmd/hive-manager/        # Manager server entry point
internal/
  api/                   # REST API server (stdlib ServeMux, 100+ endpoints)
  auth/                   # Authentication (JWT + Argon2id) and RBAC
  cli/                    # CLI commands and implementations (15+ commands)
  compliance/             # SAP HANA VM guardrails enforcement
  config/                 # Configuration loading (viper)
  db/                     # Database layer (pgxpool, migrations, CRUD repos)
  events/                 # Pub/sub event framework
  libvirt/                # KVM/QEMU/libvirt wrapper
  manager/                # Manager core services (CRUD, orchestration)
  node/                   # Node agent (gRPC server)
  metrics/                # Prometheus metrics
pkg/
  api/                    # HTTP API client
api/                      # OpenAPI specification
migration/               # VMware VMX parser/converter
tests/                    # Integration and unit tests
pkg/systemd/              # Systemd service files
docs/                     # Documentation
```

## API

See [api/openapi.yaml](api/openapi.yaml) for the full REST API specification (OpenAPI 3.0).

### Key Endpoints

```
POST   /api/v1/auth/login              - Authenticate, get JWT
POST   /api/v1/auth/logout             - Log out
GET    /api/v1/auth/me                 - Current user info
GET    /api/v1/users                   - List users (tenant-scoped)
POST   /api/v1/users                   - Create user
GET/PUT/DELETE /api/v1/users/{id}     - User CRUD

GET    /api/v1/hosts                   - List hosts
POST   /api/v1/hosts                   - Register host
GET/PUT/DELETE /api/v1/hosts/{id}     - Host CRUD
GET    /api/v1/hosts/{id}/status       - Host status + resources
POST   /api/v1/hosts/{id}/maintenance  - Enter maintenance mode
DELETE /api/v1/hosts/{id}/maintenance  - Exit maintenance mode

GET    /api/v1/vms                     - List VMs
POST   /api/v1/vms                     - Create VM (HANA guardrails enforced)
GET/PUT/DELETE /api/v1/vms/{id}       - VM CRUD
POST   /api/v1/vms/{id}/start          - Start VM
POST   /api/v1/vms/{id}/stop           - Stop VM
POST   /api/v1/vms/{id}/restart        - Restart VM
POST   /api/v1/vms/{id}/migrate        - Migrate VM to another host
GET    /api/v1/vms/{id}/snapshots      - List snapshots
POST   /api/v1/vms/{id}/snapshots      - Create snapshot
DELETE /api/v1/vms/{id}/snapshots/{sid} - Delete snapshot
GET    /api/v1/vms/{id}/console        - Console URL
GET    /api/v1/vms/{id}/stats          - VM statistics

GET    /api/v1/storage-pools           - List storage pools
POST   /api/v1/storage-pools           - Create storage pool
GET/PUT/DELETE /api/v1/storage-pools/{id} - Storage pool CRUD

GET    /api/v1/networks                - List networks
POST   /api/v1/networks                - Create network
GET/PUT/DELETE /api/v1/networks/{id}  - Network CRUD

GET    /api/v1/backups                 - List backups
POST   /api/v1/backups                 - Create backup
GET/PUT/DELETE /api/v1/backups/{id}   - Backup CRUD
POST   /api/v1/backups/{id}/restore    - Restore backup
POST   /api/v1/backups/{id}/cancel     - Cancel backup

GET    /api/v1/events                  - List events (audit log)
GET    /api/v1/compliance/vms/{id}     - Compliance check for VM
GET    /api/v1/compliance/evidence/{id} - Compliance evidence chain
GET    /api/v1/compliance/drift        - Compliance drift report
GET    /api/v1/health                  - Health check
GET    /api/v1/metrics                 - Prometheus metrics
```

## SAP HANA Compliance

HiveStack enforces VM-level guardrails for SAP HANA VMs to maintain certified status (host runs certified SLES 15 SP7 + KVM — no re-certification needed). Guest OS compatibility is the customer's responsibility.

### Guardrails Enforced

- **NUMA alignment**: vCPUs pinned to a single NUMA node
- **Hugepages**: hugepages enabled and sized appropriately
- **Dedicated vCPUs**: no CPU overcommit; dedicated allocation
- **No ballooning**: memory ballooning disabled
- **No swap**: swap disabled inside the VM

### Dual Enforcement

1. **Manager side**: API rejects non-compliant HANA VM specs at creation/update
2. **Node Agent side**: libvirt XML generation refuses non-compliant configs

### Compliance Evidence

Hash-chained evidence stored in `compliance_evidence` table — tamper-evident audit trail. Drift detection compares running VM config against last compliant config and generates alerts.

## Multi-Tenancy

Every entity is tenant-scoped via `tenant_id`. RBAC is scoped per tenant. Network isolation and storage quotas are designed into the architecture (implementation in progress).

## Deployment

### Systemd Services

```bash
# Manager
cp pkg/systemd/hivestack-manager.service /etc/systemd/system/
systemctl daemon-reload
systemctl enable --now hivestack-manager

# Node Agent (on each compute node)
cp pkg/systemd/hivestack-node.service /etc/systemd/system/
systemctl daemon-reload
systemctl enable --now hivestack-node
```

Both service files include security hardening: ProtectSystem=strict, NoNewPrivileges=yes, CapabilityBoundingSet for KVM, LimitNOFILE=65536.

### Configuration

```yaml
# /etc/hivestack/manager.yaml
server:
  host: "0.0.0.0"
  port: 8080
database:
  dsn: "postgres://hivestack:***@localhost:5432/hivestack?sslmode=disable"
auth:
  jwt_secret: "your-secret-here"
  token_expiry: "24h"
```

## Documentation

- [Architecture](docs/ARCHITECTURE.md) — System architecture, components, data flows
- [API Reference](api/openapi.yaml) — Full REST API specification (OpenAPI 3.0)

## Development

### Prerequisites

- Go 1.26+
- Node.js 18+ (for web UI development)

### Build

```bash
go build ./...       # Build all packages
go vet ./...         # Run go vet
go test ./... -v     # Run tests
```

### CI/CD

GitHub Actions workflow at `.github/workflows/ci.yml` — build, test, lint, OpenAPI validation.

## License

TBD — likely GPLv3 or Apache 2.0 for open source, with commercial options.

Copyright (c) 2026 Maddy AI Consultancy