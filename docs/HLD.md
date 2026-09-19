# HiveStack High-Level Design (HLD)

## System Architecture

HiveStack is a distributed system providing High Availability and VM Migration for virtualized infrastructure. It operates as a control plane that monitors hosts, detects failures, orchestrates failovers, and manages VM migrations from VMware vCenter to KVM.

```
┌─────────────────────────────────────────────────────────────────────────────────┐
│                              HiveStack Control Plane                             │
│                                                                                  │
│  ┌─────────────────────────────────────────────────────────────────────────────┐ │
│  │                            API Gateway (HTTP Server)                         │ │
│  │  ┌───────────┐  ┌───────────┐  ┌───────────┐  ┌──────────────────────────┐│ │
│  │  │ OVF Import│  │ vCenter   │  │ HA Control│  │ Migration Job Management ││ │
│  │  │ Endpoints │  │ Discovery │  │ Endpoints │  │ Endpoints                ││ │
│  │  └───────────┘  └───────────┘  └───────────┘  └──────────────────────────┘│ │
│  └─────────────────────────────────────────────────────────────────────────────┘ │
│                                       │                                          │
│  ┌────────────────────────────────────┼────────────────────────────────────────┐ │
│  │                          Core Services                                      │ │
│  │                                    │                                        │ │
│  │  ┌─────────────────────┐  ┌───────┴───────────────┐  ┌──────────────────┐ │ │
│  │  │  Migration Service  │  │   HA Controller        │  │  Preflight       │ │ │
│  │  │                     │  │                        │  │  Checker         │ │ │
│  │  │  - Job Creation     │  │  - Health Monitor      │  │                  │ │ │
│  │  │  - OVF Parsing      │  │  - Failure Detection   │  │  - CPU Check     │ │ │
│  │  │  - Progress Track   │  │  - Failover Trigger    │  │  - Memory Check  │ │ │
│  │  └─────────┬───────────┘  └───────────┬────────────┘  │  - Disk Check    │ │ │
│  │            │                          │                │  - Network Check │ │ │
│  │            │                          │                └──────────────────┘ │
│  │  ┌─────────┴───────────┐  ┌───────────┴────────────┐                        │ │
│  │  │  Job Executor       │  │   Orchestrator          │                        │ │
│  │  │                     │  │                        │                        │ │
│  │  │  - Async Execution  │  │  - Fencing Coord       │                        │ │
│  │  │  - State Machine    │  │  - Scheduler Coord     │                        │ │
│  │  │  - Progress Updates │  │  - VM Restart Coord    │                        │ │
│  │  └─────────┬───────────┘  └───────────┬────────────┘                        │ │
│  │            │                          │                                      │ │
│  │  ┌─────────┴──────────────────────────┴───────────────────────────────────┐  │ │
│  │  │                    External System Adapters                             │  │ │
│  │  │  ┌────────────────┐  ┌────────────────┐  ┌──────────────────────────┐│  │ │
│  │  │  │ vCenter Client │  │ Fencing Agents │  │ Storage Backends         ││  │ │
│  │  │  │ (govmomi)      │  │ (IPMI/Redfish/ │  │ (NFS/Ceph/iSCSI)         ││  │ │
│  │  │  │                │  │  SSH)          │  │                          ││  │ │
│  │  │  └────────────────┘  └────────────────┘  └──────────────────────────┘│  │ │
│  │  └────────────────────────────────────────────────────────────────────────┘  │ │
│  └─────────────────────────────────────────────────────────────────────────────┘ │
└─────────────────────────────────────────────────────────────────────────────────┘
```

## Data Flow

### VM Migration Data Flow (vCenter Import)

```
┌──────────┐     ┌──────────────┐     ┌───────────────┐     ┌──────────────┐
│ Operator │────►│ API Server   │────►│ vCenter Client│────►│ VMware       │
│          │     │ /vcenter/    │     │ (govmomi)     │     │ vCenter      │
│          │     │ import       │     │               │     │ Server       │
└──────────┘     └──────┬───────┘     └───────────────┘     └──────────────┘
                        │
                        ▼
               ┌───────────────┐
               │   Preflight   │
               │   Checker     │
               │               │
               │ - CPU compat  │
               │ - Memory      │
               │ - Disk format │
               │ - Guest OS    │
               │ - Network map │
               └───────┬───────┘
                       │
                       ▼
               ┌───────────────┐     ┌──────────────┐
               │ Job Executor  │────►│  Storage     │
               │               │     │  Backend     │
               │ 1. Discover   │     │  (NFS/Ceph/  │
               │ 2. Preflight  │     │   iSCSI)     │
               │ 3. Import     │     └──────────────┘
               │ 4. Convert    │
               │ 5. Validate   │
               └───────────────┘
```

### HA Failover Data Flow

```
┌──────────┐     ┌────────────────┐     ┌─────────────────┐
│ Host     │────►│ HA Controller  │────►│ Health          │
│ Agents   │     │                │     │ Processor       │
│          │     │ - Heartbeat    │     │                 │
│          │     │   ticker       │     │ - State machine │
│          │     │ - Reconcile    │     │ - Threshold     │
└──────────┘     └───────┬────────┘     │   evaluation    │
                         │              └─────────────────┘
                         ▼
               ┌─────────────────┐
               │  Orchestrator   │
               │                 │
               │ 1. Detect       │
               │ 2. Fence        │────►┌────────────────┐
               │ 3. Schedule     │     │ Fencing Agent  │
               │ 4. Restart      │     │ (IPMI/Redfish/ │
               └────────┬────────┘     │  SSH)          │
                        │              └────────────────┘
                        ▼
               ┌─────────────────┐     ┌────────────────┐
               │   Scheduler     │     │  VM Restarter  │
               │                 │     │                │
               │ - Score hosts   │     │ - Start VM     │
               │ - Anti-affinity │     │ - Update host  │
               │ - NUMA aware    │     │   assignment   │
               └─────────────────┘     └────────────────┘
```

### OVF Upload Data Flow

```
┌──────────┐     ┌────────────────┐     ┌─────────────────┐
│ Operator │────►│ API Server     │────►│ OVF Parser      │
│          │     │ /import/ovf    │     │                 │
│ (upload) │     │                │     │ - XML parse     │
└──────────┘     └───────┬────────┘     │ - VM extract    │
                         │              │ - Disk extract  │
                         ▼              └────────┬────────┘
               ┌─────────────────┐              │
               │ Migration       │◄─────────────┘
               │ Service         │
               │                 │
               │ - Create job    │
               │ - Run preflight │
               │ - Return status │
               └────────┬────────┘
                        │
                        ▼
               ┌─────────────────┐
               │ Async Import    │
               │                 │
               │ 1. Parse        │
               │ 2. Import       │
               │ 3. Complete     │
               └─────────────────┘
```

## Deployment Topology

```
┌─────────────────────────────────────────────────────────────────┐
│                    Production Deployment                         │
│                                                                  │
│  ┌──────────────────┐  ┌──────────────────┐  ┌───────────────┐ │
│  │ HiveStack Node 1 │  │ HiveStack Node 2 │  │ HiveStack N   │ │
│  │ (Primary)        │  │ (Secondary)      │  │               │ │
│  │                  │  │                  │  │               │ │
│  │ - HA Controller  │  │ - HA Controller  │  │ - HA Ctrl     │ │
│  │ - API Server     │  │ - API Server     │  │ - API Server  │ │
│  │ - Job Executor   │  │ - Job Executor   │  │ - Job Exec    │ │
│  └────────┬─────────┘  └────────┬─────────┘  └───────┬───────┘ │
│           │                     │                     │         │
│           └─────────────────────┼─────────────────────┘         │
│                                 │                                │
│                    ┌────────────┴────────────┐                  │
│                    │    Shared Storage       │                  │
│                    │    (NFS/Ceph/iSCSI)     │                  │
│                    │                        │                  │
│                    │ - VM disk images       │                  │
│                    │ - Migration job state  │                  │
│                    │ - HA policy store      │                  │
│                    └────────────────────────┘                  │
│                                                                  │
│  ┌───────────────────────────────────────────────────────────┐  │
│  │                   Managed Hosts                           │  │
│  │  ┌─────────┐  ┌─────────┐  ┌─────────┐  ┌─────────┐   │  │
│  │  │ Host 1  │  │ Host 2  │  │ Host 3  │  │ Host N  │   │  │
│  │  │         │  │         │  │         │  │         │   │  │
│  │  │ Agent   │  │ Agent   │  │ Agent   │  │ Agent   │   │  │
│  │  │ VMs     │  │ VMs     │  │ VMs     │  │ VMs     │   │  │
│  │  │ BMC     │  │ BMC     │  │ BMC     │  │ BMC     │   │  │
│  │  └─────────┘  └─────────┘  └─────────┘  └─────────┘   │  │
│  └───────────────────────────────────────────────────────────┘  │
└─────────────────────────────────────────────────────────────────┘
```

## External System Dependencies

| System | Purpose | Protocol | Notes |
|--------|---------|----------|-------|
| VMware vCenter | VM discovery & migration | SOAP (govmomi) | Stub for development |
| IPMI/BMC | Node power management | IPMI v2.0+ | Via ipmitool |
| Redfish API | Node power management | HTTPS REST | Via curl |
| SSH | Node power management, storage | SSH | Via ssh command |
| NFS Server | Shared storage | NFS v4 | Via mount.nfs |
| Ceph Cluster | Shared storage | RBD | Via rbd CLI |
| iSCSI Target | Shared storage | iSCSI | Via iscsiadm |

## API Surface

### REST Endpoints

| Method | Path | Description | Request Body | Response |
|--------|------|-------------|--------------|----------|
| POST | `/api/v1/migration/import/ovf` | Upload OVF for import | Multipart form (ovf file) | ImportJob |
| POST | `/api/v1/migration/import/ova` | Upload OVA for import | Multipart form (ova file) | ImportJob |
| GET | `/api/v1/migration/jobs` | List OVF import jobs | - | []ImportJob |
| GET | `/api/v1/migration/jobs/:id` | Get job details | - | ImportJob |
| PUT | `/api/v1/migration/jobs/:id` | Start/cancel job | {action: "start"\|"cancel"} | ImportJob |
| DELETE | `/api/v1/migration/jobs/:id` | Delete job | - | 204 |
| POST | `/api/v1/migration/vcenter/discover` | Discover vCenter inventory | {host, port, username, password, insecure} | DiscoveryResult |
| POST | `/api/v1/migration/vcenter/import` | Import VMs from vCenter | {host, port, username, password, insecure, vm_ids} | Job |
| GET | `/api/v1/migration/vcenter/jobs` | List vCenter jobs | - | {jobs, total, active} |
| GET | `/api/v1/migration/vcenter/jobs/:id` | Get job details | - | Job |
| DELETE | `/api/v1/migration/vcenter/jobs/:id` | Cancel job | - | {status, job_id} |
| GET | `/api/v1/migration/vcenter/jobs/progress/:id` | Get job progress | - | ProgressUpdate |

## Disaster Recovery

### RPO/RTO Targets

| Metric | Target | Notes |
|--------|--------|-------|
| RPO (Recovery Point Objective) | 0 (zero data loss) | Shared storage preserves VM state |
| RTO (Recovery Time Objective) | < 5 minutes | From host failure to VM running on new node |

### Failure Scenarios

| Scenario | Detection | Recovery | SLA |
|----------|-----------|----------|-----|
| Host failure (single node) | Heartbeat timeout (150s) | Automatic failover to healthy host | < 5 minutes |
| Network partition | Heartbeat timeout | Fencing + failover | < 5 minutes |
| HiveStack node failure | N/A (stateless control plane) | Restart on secondary node | < 2 minutes |
| Storage backend failure | Mount failure | Alert operator, queue restarts | Manual intervention |
| vCenter unavailable | API error | Queue migration jobs | Retry with backoff |

### Data Persistence

| Data | Storage | Replication |
|------|---------|-------------|
| VM Disk Images | Shared storage (NFS/Ceph/iSCSI) | Storage-level replication |
| Migration Job State | In-memory (with shared storage backup) | Job tracker recovery on restart |
| HA Policies | Shared storage / database | Synchronous replication |
| Failover History | In-memory ring buffer (100 entries) | Event log shipped to external system |

### Backup Strategy

1. **VM Disk Images**: Rely on storage backend snapshots (Ceph RBD snapshots, NFS snapshots)
2. **HiveStack Configuration**: Version-controlled configuration files
3. **Migration Job State**: Persist to shared storage for recovery after restart
4. **HA Policy Data**: Database replication across HiveStack nodes

## Capacity Planning

### Sizing Guidelines

| Resource | Minimum | Recommended | Maximum |
|----------|---------|-------------|---------|
| HiveStack Node CPU | 2 cores | 4 cores | 8 cores |
| HiveStack Node RAM | 4 GB | 8 GB | 16 GB |
| Network Bandwidth | 1 Gbps | 10 Gbps | 25 Gbps |
| Migration throughput | 100 MB/s | 500 MB/s | 1 GB/s |

### Scaling Characteristics

- **Horizontal**: Multiple HiveStack nodes can run concurrently with leader election for HA controller
- **Vertical**: More CPU/RAM allows concurrent migration of more VMs
- **Storage**: Ceph scales linearly with OSD count
- **Network**: Migration throughput limited by slowest link (source → target)
