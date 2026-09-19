# HiveStack — Software Requirements Specification (SRS)

| Field | Value |
|---|---|
| **Project** | HiveStack |
| **Version** | 0.1.0 (Phase 2 — Requirements Engineering) |
| **Date** | 2026-09-19 |
| **Author** | SDLC Automation (Phase 2) |
| **Status** | Draft |

---

## 1. Introduction

### 1.1 Purpose

This Software Requirements Specification (SRS) defines the functional and non-functional requirements for **HiveStack**, a KVM-based virtualization appliance built on SUSE Linux Enterprise Server 15 SP7. HiveStack is designed to be a VMware vCenter/ESXi migration target with vCenter-like management capabilities, ESXi-like hypervisor nodes, and Proxmox-level flexibility.

### 1.2 Scope

HiveStack provides:
- A **Manager** (control plane) with REST API, web UI, and CLI for infrastructure management.
- **Nodes** (compute) running KVM/QEMU with libvirt integration.
- **VMware Migration Toolkit** for importing VMs from vCenter/ESXi environments.
- **High Availability (HA)** with automatic failover, fencing, and VM restart.
- **Storage subsystem** supporting NFS, Ceph RBD, and iSCSI backends.
- **Virtual networking** with bridge and VLAN support.

### 1.3 Definitions and Acronyms

| Term | Definition |
|---|---|
| **HA** | High Availability |
| **OVF** | Open Virtualization Format |
| **OVA** | Open Virtualization Appliance (tar archive of OVF + disks) |
| **VMDK** | Virtual Machine Disk (VMware format) |
| **QCOW2** | QEMU Copy-On-Write v2 disk format |
| **BMC** | Baseboard Management Controller |
| **IPMI** | Intelligent Platform Management Interface |
| **RBD** | RADOS Block Device (Ceph) |
| **IQN** | iSCSI Qualified Name |
| **NUMA** | Non-Uniform Memory Access |
| **RBAC** | Role-Based Access Control |
| **SLES** | SUSE Linux Enterprise Server |

### 1.4 References

- DMTF OVF Specification 1.1
- VMware vSphere API 7.0
- SUSE SLES 15 SP7 Documentation
- libvirt Domain XML Format
- QEMU Documentation

---

## 2. System Overview

### 2.1 Architecture

HiveStack follows a manager-node architecture:

```
┌─────────────────────────────────────────────┐
│              HiveStack Manager               │
│  ┌─────────┐ ┌─────────┐ ┌──────────────┐  │
│  │ REST API│ │ Web UI  │ │ CLI (hive)   │  │
│  └────┬────┘ └────┬────┘ └──────┬───────┘  │
│       └───────────┼─────────────┘            │
│  ┌────────────────┴────────────────────┐     │
│  │         Core Services               │     │
│  │  - Auth/RBAC  - Migration Engine    │     │
│  │  - HA Controller  - Scheduler       │     │
│  │  - Storage Manager  - Fencer        │     │
│  └────────────────────────────────────┘     │
└──────────────────┬──────────────────────────┘
                   │ TLS
    ┌──────────────┼──────────────┐
    ▼              ▼              ▼
┌────────┐   ┌────────┐   ┌────────┐
│ Node 1 │   │ Node 2 │   │ Node N │
│ KVM    │   │ KVM    │   │ KVM    │
│ Agent  │   │ Agent  │   │ Agent  │
└────────┘   └────────┘   └────────┘
```

### 2.2 Technology Stack

| Layer | Technology |
|---|---|
| Base OS | SUSE SLES 15 SP7 |
| Hypervisor | KVM/QEMU + libvirt |
| Manager API | Go (HTTP/REST) |
| Database | PostgreSQL (embedded or external) |
| Web UI | React (planned) |
| CLI | Go (cobra-style) |
| Monitoring | Prometheus + Grafana |
| HA | Custom heartbeat + corosync (planned) |
| Storage | NFS, Ceph RBD, iSCSI, ZFS, LVM |

---

## 3. Functional Requirements

### 3.1 API Server (FR-API)

| ID | Requirement | Priority | Source |
|---|---|---|---|
| FR-API-001 | The system SHALL provide a REST API server listening on a configurable address. | High | `api/server.go` |
| FR-API-002 | The API SHALL support OVF file import via multipart upload (max 500MB). | High | `api/server.go:handleMigrationImportOVF` |
| FR-API-003 | The API SHALL support OVA file import via multipart upload (max 2GB). | High | `api/server.go:handleMigrationImportOVA` |
| FR-API-004 | The API SHALL list all import jobs (GET /api/v1/migration/jobs). | High | `api/server.go:handleMigrationJobs` |
| FR-API-005 | The API SHALL return details for a specific import job by ID. | High | `api/server.go:handleMigrationJobDetail` |
| FR-API-006 | The API SHALL support cancelling an import job (PUT action=cancel). | High | `api/server.go:handleMigrationJobDetail` |
| FR-API-007 | The API SHALL support starting an import job (PUT action=start). | High | `api/server.go:handleMigrationJobDetail` |
| FR-API-008 | The API SHALL delete an import job (DELETE). | Medium | `api/server.go:handleMigrationJobDetail` |
| FR-API-009 | The API SHALL perform vCenter inventory discovery (POST /api/v1/migration/vcenter/discover). | High | `api/server.go:handleMigrationDiscover` |
| FR-API-010 | The API SHALL import VMs from vCenter by ID list (POST /api/v1/migration/vcenter/import). | High | `api/server.go:handleMigrationImportvCenter` |
| FR-API-011 | The API SHALL list vCenter migration jobs (GET /api/v1/migration/vcenter/jobs). | High | `api/server.go:handleMigrationVCJobs` |
| FR-API-012 | The API SHALL return vCenter job details and support cancellation. | High | `api/server.go:handleMigrationVCJobDetail` |
| FR-API-013 | The API SHALL return real-time progress for vCenter jobs. | High | `api/server.go:handleMigrationVCJobProgress` |
| FR-API-014 | The API SHALL return 503 when vCenter client is not configured. | Medium | `api/server.go` |
| FR-API-015 | The API SHALL support graceful shutdown via context cancellation. | High | `api/server.go:Shutdown` |
| FR-API-016 | The API SHALL return JSON error responses with appropriate HTTP status codes. | High | `api/server.go:writeError` |

### 3.2 Migration Service (FR-MIG)

| ID | Requirement | Priority | Source |
|---|---|---|---|
| FR-MIG-001 | The system SHALL create import jobs with unique IDs and track their state. | High | `api/migration.go:CreateImportJob` |
| FR-MIG-002 | Import jobs SHALL progress through states: pending → preflight → parsing → importing → completed/failed/cancelled. | High | `api/migration.go:ImportJobState` |
| FR-MIG-003 | The system SHALL update job state and progress (0-100%). | High | `api/migration.go:UpdateJobState` |
| FR-MIG-004 | The system SHALL cancel jobs and mark them as cancelled. | High | `api/migration.go:CancelImportJob` |
| FR-MIG-005 | The system SHALL delete jobs from tracking. | Medium | `api/migration.go:DeleteImportJob` |
| FR-MIG-006 | The system SHALL run pre-flight compatibility checks before import. | High | `api/migration.go:RunPreflightChecks` |
| FR-MIG-007 | Pre-flight checks SHALL validate target host existence. | High | `api/migration.go:RunPreflightChecks` |
| FR-MIG-008 | Pre-flight checks SHALL verify OVF contains at least one VM. | High | `api/migration.go:RunPreflightChecks` |
| FR-MIG-009 | Pre-flight checks SHALL warn if memory exceeds host capacity. | Medium | `api/migration.go:RunPreflightChecks` |
| FR-MIG-010 | Pre-flight checks SHALL warn if CPU count exceeds host capacity. | Medium | `api/migration.go:RunPreflightChecks` |
| FR-MIG-011 | Pre-flight checks SHALL warn on unknown disk formats. | Low | `api/migration.go:RunPreflightChecks` |
| FR-MIG-012 | Pre-flight checks SHALL warn on unknown virtual system types. | Low | `api/migration.go:RunPreflightChecks` |
| FR-MIG-013 | The system SHALL support key=value labels on import jobs. | Low | `api/migration.go:parseLabels` |

### 3.3 OVF Parser (FR-OVF)

| ID | Requirement | Priority | Source |
|---|---|---|---|
| FR-OVF-001 | The system SHALL parse OVF XML descriptors (Envelope, VirtualSystem, VirtualSystemCollection). | High | `api/ovf.go:ParseOVF` |
| FR-OVF-002 | The system SHALL extract VM configurations (name, ID, CPUs, memory, OS, disks, networks). | High | `api/ovf.go:extractVM` |
| FR-OVF-003 | The system SHALL parse memory allocation units (byte * 2^N format). | High | `api/ovf.go:parseMemoryUnit` |
| FR-OVF-004 | The system SHALL extract disk information (ID, file reference, capacity, format). | High | `api/ovf.go:ParseOVF` |
| FR-OVF-005 | The system SHALL extract network names from NetworkSection. | Medium | `api/ovf.go:ParseOVF` |
| FR-OVF-006 | The system SHALL extract product metadata (vendor, product, version). | Low | `api/ovf.go:extractVM` |
| FR-OVF-007 | The system SHALL set default values: 1 CPU, 52MB memory if not specified. | Medium | `api/ovf.go:extractVM` |
| FR-OVF-008 | The system SHALL reject OVF files with no virtual systems. | High | `api/ovf.go:ParseOVF` |

### 3.4 HA Controller (FR-HA-CTL)

| ID | Requirement | Priority | Source |
|---|---|---|---|
| FR-HA-CTL-001 | The system SHALL run a health check ticker at configurable intervals. | High | `ha/controller.go:mainLoop` |
| FR-HA-CTL-002 | The system SHALL detect node state transitions (online → suspect → offline). | High | `ha/controller.go:reconcile` |
| FR-HA-CTL-003 | The system SHALL trigger automatic failover when a node goes offline. | High | `ha/controller.go:handleTransition` |
| FR-HA-CTL-004 | The system SHALL support manual failover triggering via API. | High | `ha/controller.go:TriggerFailover` |
| FR-HA-CTL-005 | The system SHALL register and unregister nodes for health monitoring. | High | `ha/controller.go:RegisterNode` |
| FR-HA-CTL-006 | The system SHALL process incoming heartbeats from nodes. | High | `ha/controller.go:ProcessHeartbeat` |
| FR-HA-CTL-007 | The system SHALL expose node health status (single node and all nodes). | High | `ha/controller.go:GetNodeHealth` |
| FR-HA-CTL-008 | The system SHALL update health thresholds at runtime. | Medium | `ha/controller.go:SetThresholds` |
| FR-HA-CTL-009 | The system SHALL emit metrics (online/suspect/offline node counts). | Medium | `ha/controller.go:reconcile` |
| FR-HA-CTL-010 | The system SHALL emit events for state transitions and failover. | Medium | `ha/controller.go:handleTransition` |
| FR-HA-CTL-011 | The system SHALL support graceful start and stop. | High | `ha/controller.go:Start/Stop` |

### 3.5 HA Orchestrator (FR-HA-ORC)

| ID | Requirement | Priority | Source |
|---|---|---|---|
| FR-HA-ORC-001 | The system SHALL execute a full failover pipeline: detect → fence → schedule → restart. | High | `ha/orchestrator.go:HandleHostFailure` |
| FR-HA-ORC-002 | The system SHALL fence the failed host before restarting VMs. | High | `ha/orchestrator.go:HandleHostFailure` |
| FR-HA-ORC-003 | The system SHALL identify VMs with auto-restart HA policy on the failed host. | High | `ha/orchestrator.go:HandleHostFailure` |
| FR-HA-ORC-004 | The system SHALL select target hosts via the scheduler. | High | `ha/orchestrator.go:HandleHostFailure` |
| FR-HA-ORC-005 | The system SHALL restart VMs on selected target hosts. | High | `ha/orchestrator.go:RestartVMs` |
| FR-HA-ORC-006 | The system SHALL track active failovers and failover history. | High | `ha/orchestrator.go` |
| FR-HA-ORC-007 | The system SHALL prevent concurrent failovers for the same node. | High | `ha/orchestrator.go:HandleHostFailure` |
| FR-HA-ORC-008 | The system SHALL publish failover lifecycle events. | Medium | `ha/orchestrator.go` |
| FR-HA-ORC-009 | The system SHALL use a configurable failover timeout (default 5 minutes). | High | `ha/controller.go:NewController` |

### 3.6 HA Health (FR-HA-HLT)

| ID | Requirement | Priority | Source |
|---|---|---|---|
| FR-HA-HLT-001 | The system SHALL process heartbeats with node ID, timestamp, VM list, and resource data. | High | `ha/health.go:ProcessHeartbeat` |
| FR-HA-HLT-002 | The system SHALL track per-node health state (online, suspect, offline). | High | `ha/health.go:NodeHealth` |
| FR-HA-HLT-003 | The system SHALL count missed heartbeats and transition states based on thresholds. | High | `ha/health.go:MissHeartbeat` |
| FR-HA-HLT-004 | Default thresholds SHALL be: 30s interval, 3 missed = suspect (90s), 5 missed = offline (150s). | High | `ha/health.go:DefaultThresholds` |
| FR-HA-HLT-005 | The system SHALL validate heartbeat fields (node_id, timestamp required). | High | `ha/health.go:Heartbeat.Validate` |
| FR-HA-HLT-006 | The system SHALL recover nodes to online state upon receiving a heartbeat. | High | `ha/health.go:UpdateHeartbeat` |
| FR-HA-HLT-007 | The system SHALL provide thread-safe access to node health data. | High | `ha/health.go` |

### 3.7 HA Fencer (FR-HA-FEN)

| ID | Requirement | Priority | Source |
|---|---|---|---|
| FR-HA-FEN-001 | The system SHALL support IPMI v2.0 fencing via ipmitool. | High | `ha/fencer.go:IPMIFencer` |
| FR-HA-FEN-002 | The system SHALL support Redfish REST API fencing. | High | `ha/fencer.go:RedfishFencer` |
| FR-HA-FEN-003 | The system SHALL support SSH-based fencing (poweroff command). | Medium | `ha/fencer.go:SSHFencer` |
| FR-HA-FEN-004 | The system SHALL try multiple fencing methods in order (MultiFencer). | High | `ha/fencer.go:MultiFencer.Fence` |
| FR-HA-FEN-005 | The system SHALL retry fencing up to 3 attempts per method with configurable intervals. | High | `ha/fencer.go:MultiFencer.Fence` |
| FR-HA-FEN-006 | The system SHALL check power state via each fencing method. | High | `ha/fencer.go:GetPowerState` |
| FR-HA-FEN-007 | The system SHALL record fencing results for audit. | Medium | `ha/fencer.go:FenceResult` |
| FR-HA-FEN-008 | The system SHALL validate fence configuration (required fields per method). | High | `ha/fencer.go:FenceConfig.Validate` |

### 3.8 HA Scheduler (FR-HA-SCH)

| ID | Requirement | Priority | Source |
|---|---|---|---|
| FR-HA-SCH-001 | The system SHALL select optimal target hosts for VM restart during failover. | High | `ha/scheduler.go:SelectTarget` |
| FR-HA-SCH-002 | The system SHALL support multiple scheduling policies: binpack, spread, numa-aware. | High | `ha/scheduler.go:Policy` |
| FR-HA-SCH-003 | The system SHALL score hosts based on resource availability, NUMA affinity, and anti-affinity. | High | `ha/scheduler.go:ScoreHost` |
| FR-HA-SCH-004 | The system SHALL filter out hosts in maintenance mode or offline status. | High | `ha/scheduler.go:Host.CanFit` |
| FR-HA-SCH-005 | The system SHALL enforce anti-affinity rules (VMs that must not co-locate). | High | `ha/scheduler.go:ScoreHost` |
| FR-HA-SCH-006 | The system SHALL support NUMA-aware placement with strict policy preference. | Medium | `ha/scheduler.go:ScoreHost` |
| FR-HA-SCH-007 | The system SHALL support preferred host hints. | Low | `ha/scheduler.go:ScoreHost` |
| FR-HA-SCH-008 | The system SHALL support label-based affinity scoring. | Low | `ha/scheduler.go:ScoreHost` |
| FR-HA-SCH-009 | The system SHALL sort VMs by memory size (descending) for efficient packing. | Medium | `ha/scheduler.go:SelectTargets` |
| FR-HA-SCH-010 | The system SHALL track mutable host state during multi-VM scheduling. | High | `ha/scheduler.go:SelectTargets` |

### 3.9 HA Policy (FR-HA-POL)

| ID | Requirement | Priority | Source |
|---|---|---|---|
| FR-HA-POL-001 | The system SHALL support per-VM HA modes: auto, manual, never, max-one. | High | `ha/policy.go:HAMode` |
| FR-HA-POL-002 | The system SHALL restart VMs with auto or max-one mode on host failure. | High | `ha/policy.go:HAPolicy.ShouldRestart` |
| FR-HA-POL-003 | The system SHALL support priority-based restart ordering (0 = highest). | High | `ha/policy.go` |
| FR-HA-POL-004 | The system SHALL enforce anti-affinity groups per VM. | High | `ha/policy.go:HAPolicy` |
| FR-HA-POL-005 | The system SHALL rate-limit restarts (max restarts within a time window). | High | `ha/policy.go:CanRestart` |
| FR-HA-POL-006 | The system SHALL track restart history per VM. | Medium | `ha/policy.go:RecordRestart` |
| FR-HA-POL-007 | The system SHALL validate HA policy configuration. | High | `ha/policy.go:HAPolicy.Validate` |
| FR-HA-POL-008 | The system SHALL support policy change history (optional persistence). | Low | `ha/policy.go:PolicyHistoryStore` |
| FR-HA-POL-009 | Default HA policy SHALL be: auto-restart, priority 100, max 3 restarts per 5 minutes. | High | `ha/policy.go:DefaultHAPolicy` |

### 3.10 HA Storage (FR-HA-STO)

| ID | Requirement | Priority | Source |
|---|---|---|---|
| FR-HA-STO-001 | The system SHALL support NFS storage backend (mount/unmount). | High | `ha/storage.go:NFSBackend` |
| FR-HA-STO-002 | The system SHALL support Ceph RBD storage backend (map/unmap). | High | `ha/storage.go:CephRBDBackend` |
| FR-HA-STO-003 | The system SHALL support iSCSI storage backend (login/logout). | High | `ha/storage.go:ISCSIBackend` |
| FR-HA-STO-004 | The system SHALL detect available storage backends based on CLI tools in PATH. | High | `ha/storage.go:IsAvailable` |
| FR-HA-STO-005 | The system SHALL initialize storage backends (create mount points, etc.). | High | `ha/storage.go:StorageManager.Init` |
| FR-HA-STO-006 | The system SHALL provide a common StorageBackend interface for all backends. | High | `ha/storage.go:StorageBackend` |
| FR-HA-STO-007 | The system SHALL support command injection via CommandRunner interface (testability). | Medium | `ha/storage.go:CommandRunner` |

### 3.11 Migration Tracker (FR-MIG-TRK)

| ID | Requirement | Priority | Source |
|---|---|---|---|
| FR-MIG-TRK-001 | The system SHALL create vCenter migration jobs with unique IDs. | High | `migration/tracker.go:CreateJob` |
| FR-MIG-TRK-002 | Jobs SHALL progress through states: queued → discovering → preflight → importing → validating → completed/failed/cancelled. | High | `migration/tracker.go:JobState` |
| FR-MIG-TRK-003 | The system SHALL track per-VM import status (pending, copying, converting, booting, done, failed). | High | `migration/tracker.go:VMImportStatus` |
| FR-MIG-TRK-004 | The system SHALL track bytes copied and total bytes for progress calculation. | High | `migration/tracker.go` |
| FR-MIG-TRK-005 | The system SHALL support job cancellation (cannot cancel terminal states). | High | `migration/tracker.go:CancelJob` |
| FR-MIG-TRK-006 | The system SHALL provide progress snapshots for reporting. | High | `migration/tracker.go:GetProgress` |
| FR-MIG-TRK-007 | The system SHALL execute jobs asynchronously via JobExecutor. | High | `migration/tracker.go:JobExecutor` |
| FR-MIG-TRK-008 | The system SHALL validate requested VMs exist in vCenter inventory before import. | High | `migration/tracker.go:JobExecutor.execute` |
| FR-MIG-TRK-009 | The system SHALL run pre-flight checks during job execution. | High | `migration/tracker.go:JobExecutor.execute` |
| FR-MIG-TRK-010 | The system SHALL count completed and failed VMs per job. | Medium | `migration/tracker.go:Job` |

### 3.12 vCenter Client (FR-VC)

| ID | Requirement | Priority | Source |
|---|---|---|---|
| FR-VC-001 | The system SHALL connect to vCenter with host, port, username, password, and insecure flag. | High | `vcenter/client.go:NewClient` |
| FR-VC-002 | The system SHALL manage vCenter sessions with expiry tracking. | High | `vcenter/client.go:Client` |
| FR-VC-003 | The system SHALL discover vCenter inventory (datacenters, hosts, VMs). | High | `vcenter/client.go:Discover` |
| FR-VC-004 | The system SHALL retrieve a single VM by ID. | High | `vcenter/client.go:GetVM` |
| FR-VC-005 | The system SHALL retrieve guest OS information for running VMs. | Medium | `vcenter/client.go:GetGuestInfo` |
| FR-VC-006 | The system SHALL validate connection state before operations. | High | `vcenter/client.go:IsConnected` |
| FR-VC-007 | The system SHALL support graceful disconnect. | High | `vcenter/client.go:Disconnect` |

### 3.13 Preflight Checker (FR-PRE)

| ID | Requirement | Priority | Source |
|---|---|---|---|
| FR-PRE-001 | The system SHALL check CPU compatibility (count within KVM limits). | High | `preflight/checker.go:checkCPU` |
| FR-PRE-002 | The system SHALL check memory configuration (within limits, hugepage alignment). | High | `preflight/checker.go:checkMemory` |
| FR-PRE-003 | The system SHALL check disk size compatibility. | High | `preflight/checker.go:checkDisk` |
| FR-PRE-004 | The system SHALL check guest OS support (exact and partial match). | High | `preflight/checker.go:checkGuestOS` |
| FR-PRE-005 | The system SHALL verify all VM networks have KVM bridge mappings. | High | `preflight/checker.go:checkNetworks` |
| FR-PRE-006 | The system SHALL check VMware Tools status. | Medium | `preflight/checker.go:checkVMwareTools` |
| FR-PRE-007 | The system SHALL report VM power state for migration planning. | Medium | `preflight/checker.go:checkPowerState` |
| FR-PRE-008 | The system SHALL report disk provisioning type. | Low | `preflight/checker.go:checkProvisioningType` |
| FR-PRE-009 | The system SHALL determine overall pass/fail based on critical check failures. | High | `preflight/checker.go:CheckVM` |
| FR-PRE-010 | The system SHALL support configurable limits (max CPUs, memory, disk). | High | `preflight/checker.go:Checker` |
| FR-PRE-011 | The system SHALL support configurable network mappings. | High | `preflight/checker.go:Checker` |
| FR-PRE-012 | The system SHALL support configurable supported guest OS list. | High | `preflight/checker.go:Checker` |

---

## 4. Non-Functional Requirements

### 4.1 Performance (NFR-PERF)

| ID | Requirement | Target |
|---|---|---|
| NFR-PERF-001 | API response time for job listing | < 100ms |
| NFR-PERF-002 | Heartbeat processing latency | < 10ms per node |
| NFR-PERF-003 | Failover detection time (offline threshold) | ≤ 150 seconds (5 missed beats × 30s) |
| NFR-PERF-004 | VM restart SLA after host failure | ≤ 5 minutes |
| NFR-PERF-005 | OVF parsing time (100MB file) | < 5 seconds |
| NFR-PERF-006 | Concurrent migration jobs supported | ≥ 10 |
| NFR-PERF-007 | Maximum OVA upload size | 2 GB |

### 4.2 Reliability (NFR-REL)

| ID | Requirement | Target |
|---|---|---|
| NFR-REL-001 | Manager service availability | 99.9% |
| NFR-REL-002 | Graceful degradation when vCenter unavailable | Return 503, continue other operations |
| NFR-REL-003 | Fencing failure handling | Log warning, continue with restart (split-brain risk noted) |
| NFR-REL-004 | Job state persistence across restarts | Required (planned) |
| NFR-REL-005 | No single point of failure for Manager | Manager HA (planned) |

### 4.3 Security (NFR-SEC)

| ID | Requirement | Target |
|---|---|---|
| NFR-SEC-001 | TLS encryption for all API communications | Required |
| NFR-SEC-002 | Mutual TLS for node-to-manager communication | Required |
| NFR-SEC-003 | Password hashing (Argon2/bcrypt) | Required |
| NFR-SEC-004 | RBAC enforcement on all API endpoints | Required |
| NFR-SEC-005 | Session timeout and concurrent session limits | Required |
| NFR-SEC-006 | Audit logging for all operations | Required |
| NFR-SEC-007 | Secrets management for credentials | Required |
| NFR-SEC-008 | VM isolation (KVM, seccomp, AppArmor/SELinux) | Required |

### 4.4 Scalability (NFR-SCALE)

| ID | Requirement | Target |
|---|---|---|
| NFR-SCALE-001 | Maximum nodes per cluster | 64 (planned) |
| NFR-SCALE-002 | Maximum VMs per node | 128 (planned) |
| NFR-SCALE-003 | Maximum VMs per cluster | 4096 (planned) |
| NFR-SCALE-004 | Heartbeat processing per ticker | O(n) where n = nodes |

### 4.5 Maintainability (NFR-MAINT)

| ID | Requirement | Target |
|---|---|---|
| NFR-MAINT-001 | Unit test coverage | > 80% for core components |
| NFR-MAINT-002 | Structured logging (JSON) | All components |
| NFR-MAINT-003 | Configuration via YAML file | Required |
| NFR-MAINT-004 | Graceful shutdown with context cancellation | All long-running services |

### 4.6 Usability (NFR-USA)

| ID | Requirement | Target |
|---|---|---|
| NFR-USA-001 | CLI modeled after govc / pvesm / qm | Familiar to admins |
| NFR-USA-002 | Output formats: table, JSON, YAML | All CLI commands |
| NFR-USA-003 | Web UI dark theme | Professional, VMware-like |
| NFR-USA-004 | API documentation (OpenAPI 3.0) | Auto-generated |

### 4.7 Compatibility (NFR-COMPAT)

| ID | Requirement | Target |
|---|---|---|
| NFR-COMPAT-001 | VMware vCenter API compatibility | vSphere 7.0 |
| NFR-COMPAT-002 | OVF specification compliance | DMTF OVF 1.1 |
| NFR-COMPAT-003 | VMDK format support | All formats (flat, sparse, thin, thick) |
| NFR-COMPAT-004 | Guest OS support | SLES 12/15, RHEL 8/9, Ubuntu, Windows Server 2019/2022 |
| NFR-COMPAT-005 | Virtual hardware types | vmx-07 through vmx-21, xen, kvm, qemu |

---

## 5. Constraints

| ID | Constraint |
|---|---|
| CON-001 | Must run on SUSE SLES 15 SP7 as the base operating system. |
| CON-002 | Must use KVM/QEMU as the hypervisor (no Xen or Hyper-V). |
| CON-003 | Manager must be implementable as a single binary for appliance packaging. |
| CON-004 | Migration from VMware must support offline and planned live migration. |
| CON-005 | Fencing must support IPMI, Redfish, and SSH methods. |

---

## 6. Assumptions and Dependencies

| ID | Assumption/Dependency |
|---|---|
| DEP-001 | govmomi library will be used for production vCenter API communication. |
| DEP-002 | ipmitool, rbd, iscsiadm CLI tools are available on the host for respective backends. |
| DEP-003 | Network infrastructure supports VLAN tagging for virtual networking. |
| DEP-004 | Shared storage (NFS, Ceph, iSCSI) is available for HA VM state reconstruction. |
| DEP-005 | BMC/IPMI hardware is available for production fencing operations. |

---

## 7. Traceability

See `docs/RTM.md` for the Requirements Traceability Matrix linking these requirements to source code.
