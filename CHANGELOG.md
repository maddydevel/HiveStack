# Changelog

All notable changes to HiveStack will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and HiveStack adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added

- **CLI**: GoVec-style CLI with table, JSON, and YAML output formats (`hive` command, 15+ subcommands)
- **Web UI**: Dark-themed management console — React 18 + TypeScript + Vite + Tailwind CSS (15 pages)
- **OpenAPI 3.0**: Auto-generated REST API specification published at `api/openapi.yaml`
- **YAML configuration**: Manager and node agent configuration via `/etc/hivestack/*.yaml` (viper)
- **Manager HA**: Eliminate single point of failure — active/standby controller replication (NFR-REL-005)
- **Job state persistence**: Migration and import job state survives manager restarts (NFR-REL-004)
- **VM scalability limits**: Enforce 128 VMs per node and 4,096 VMs per cluster (NFR-SCALE-002/003)
- **VMDK support**: Full VMDK format coverage (flat, sparse, thin, thick) with VMDK → QCOW2 conversion (NFR-COMPAT-003)
- **vCenter 7.0 integration**: Replace stub client with govmomi-based production vSphere 7.0 API client (NFR-COMPAT-001)

### Changed

- **Logging**: Migrate from `log.Printf` to structured JSON logging (slog) across all components (NFR-MAINT-002)
- **Heartbeat transport**: Replace plain HTTP heartbeat receiver with gRPC streaming (internal network)
- **API server**: HTTP → HTTPS enforcement (TLS 1.3) on all endpoints (NFR-SEC-001)
- **Authentication**: Add JWT bearer <REDACTED> validation middleware to all mutation endpoints (NFR-SEC-001)

### Fixed

- **IPMI credential exposure**: Move BMC credentials from CLI arguments (visible in `/proc`) to config files with `0600` permissions (addresses I-02)
- **Redfish TLS bypass**: Remove `-k` (insecure) flag from Redfish fencing calls; require valid TLS certificates
- **SSH host verification**: Replace `StrictHostKeyChecking=no` with known_hosts-based verification for SSH fencing
- **Heartbeat spoofing**: Sign all heartbeat payloads with HMAC-SHA256; reject unauthenticated heartbeats (addresses S-02, T-04)
- **Heartbeat flooding**: Add per-node rate limiting on heartbeat ingestion (addresses D-02)
- **Failover flapping**: Introduce cooldown periods between failover attempts to prevent VM thrashing

### Security

- **RBAC enforcement**: Apply role-based access control (admin/editor/viewer/operator) to all API endpoints (NFR-SEC-004)
- **mTLS for nodes**: Mutual TLS authentication for all node-agent ↔ manager gRPC channels (NFR-SEC-002)
- **Password hashing**: Argon2id password hashing for all user credentials (NFR-SEC-003)
- **Session management**: API session timeouts (24h default) and concurrent session limits (NFR-SEC-005)
- **Audit logging**: Structured, tamper-evident audit log for all state-changing operations with actor identity (NFR-SEC-006)
- **Secrets at rest**: Encrypt all credentials at rest; integrate HashiCorp Vault and SOPS for credential storage (NFR-SEC-007)
- **VM isolation hardening**: AppArmor/SELinux profiles, seccomp filters for QEMU processes (NFR-SEC-008)
- **Rate limiting**: Per-endpoint rate limiting on upload and authentication endpoints (addresses D-01)
- **Input validation**: Content-type and magic-byte validation for OVF/OVA uploads beyond file extension (addresses T-01, E-01)
- **Host authentication**: Certificate-based host admission control — reject unauthenticated node registrations (addresses S-05)
- **Fencing config integrity**: Signed fencing configuration to prevent tampering (addresses T-06)
- **Network segmentation**: Enforce management network isolation between control plane and data plane
- **Dependency scanning**: Add `govulncheck` gate to CI/CD pipeline (addresses supply chain risk)
- **Storage command auditing**: Log all storage backend commands for forensic analysis
- **Policy RBAC**: Fine-grained access control for HA policy modifications (P3 backlog)

---

## [v0.1.0] — 2026-09-19

### Added

#### Core Platform
- **Manager control plane**: Go 1.26 single-binary manager with REST API, scheduling, and orchestration
- **Node agent**: gRPC-based agent with 5 handlers — Register, Heartbeat, ExecuteCommand, GetStatus, ListVMs
- **Database layer**: PostgreSQL via pgx/v5, 12 tables, migration system, full CRUD repositories
- **Multi-tenancy**: Tenant-scoped entity isolation (`tenant_id`) across all resources

#### Authentication & Authorization
- **JWT authentication**: Tenant-scoped JWT tokens with configurable expiry
- **Argon2id password hashing**: Memory-hard password hashing for all user credentials
- **RBAC**: 5 built-in roles — admin, editor, viewer, operator, instance_admin — with permission-based access control
- **API middleware**: Auth middleware protecting all mutation endpoints

#### API & CLI
- **REST API**: 100+ endpoints using stdlib `ServeMux` — hosts, VMs, storage pools, networks, backups, events, compliance, snapshots, migrations
- **API client**: Go HTTP client with automatic auth injection
- **CLI**: Cobra-based CLI (`hive`) with 15+ commands and JSON output support
- **OpenAPI specification**: Machine-readable API schema at `api/openapi.yaml`

#### High Availability (Non-HANA)
- **Health monitoring**: 30-second heartbeat interval with configurable thresholds (3 missed = suspect, 5 missed = offline)
- **Failure detection**: Automatic host failure detection within 150 seconds
- **Fencing**: Multi-method fencing via IPMI v2.0, Redfish REST API, and SSH (sequential with 3 retries)
- **Orchestrated failover**: Full pipeline — detect → fence → schedule → restart (5-minute RTO)
- **Scheduling policies**: binpack, spread, and NUMA-aware placement with anti-affinity and preferred-host hints
- **Per-VM HA policies**: auto, manual, never, max-one restart modes with priority ordering and rate limiting

#### Storage
- **NFS backend**: Mount/unmount NFS v4 storage for VM images
- **Ceph RBD backend**: RADOS Block Device map/unmap
- **iSCSI backend**: Login/logout iSCSI targets
- **Storage manager**: Auto-detection of available backends via CLI tool discovery

#### VM Lifecycle
- **VM CRUD**: Create, read, update, delete VMs with HANA compliance guardrails
- **VM operations**: Start, stop, restart, migrate, snapshot, console access, statistics
- **HANA compliance**: Enforced guardrails — NUMA pinning, hugepages, dedicated vCPUs, no ballooning, no swap
- **Compliance evidence**: Hash-chained tamper-evident evidence storage with drift detection
- **VMX parser**: Convert VMware VMX configuration files to HiveStack VM definitions
- **OVF/OVA import**: Parse OVF XML descriptors and import VMs from vCenter inventory
- **Pre-flight checks**: CPU, memory, disk, guest OS, network bridge, VMware Tools validation

#### Migration
- **Migration tracker**: Async job execution with progress tracking (bytes copied, per-VM status)
- **Job state machine**: queued → discovering → preflight → importing → validating → completed/failed/cancelled
- **vCenter client**: API client for vCenter inventory discovery and VM retrieval (stub for govmomi)

#### Security Hardening
- **TLS 1.3**: Encrypted API and inter-service communication
- **mTLS gRPC**: Mutual TLS for node-agent channels
- **AppArmor**: Mandatory access control profiles for manager and node processes
- **LUKS**: Disk encryption support for sensitive VM data
- **Rate limiting**: Upload size limits (500MB OVF, 2GB OVA) and connection throttling

#### Observability
- **Prometheus metrics**: `/metrics` endpoint with host, VM, cluster, and HANA compliance metrics
- **Structured logging**: slog-compatible structured output with component prefixes
- **Event framework**: Pub/sub event bus with 17 event types, DB-backed persistence
- **Health checks**: `/api/v1/health` endpoint for load balancer integration

#### Deployment
- **Systemd units**: Manager and node agent service files with `ProtectSystem=strict`, `NoNewPrivileges=yes`, KVM capability bounding
- **KIWI appliance**: SUSE Linux Enterprise Server 15 SP7 appliance packaging with first-boot initialization
- **Web UI**: React 18 + TypeScript + Vite + Tailwind CSS management interface (303KB JS, 15 pages)
- **Dashboards**: Grafana dashboards for HA operations and cluster operations
- **Scripts**: Health check, canary deploy, and TLS certificate generation utilities

#### Testing & CI/CD
- **Integration tests**: API, compliance fuzz, and end-to-end integration test suites
- **Unit tests**: Table-driven tests with race detector across all packages
- **CI pipeline**: GitHub Actions — build, vet, test, lint, OpenAPI validation
- **Security scanning**: golangci-lint with gosec rules

---

[v0.1.0]: https://github.com/maddydevel/HiveStack/releases/tag/v0.1.0
[Unreleased]: https://github.com/maddydevel/HiveStack/compare/v0.1.0...HEAD
