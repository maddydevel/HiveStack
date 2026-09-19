# HiveStack Architecture Overview

## System Overview

HiveStack is a Non-HANA High Availability (HA) storage and migration platform built in Go. It provides:

- **High Availability**: Automatic VM failover on host failure with fencing, health monitoring, and orchestrated VM restart
- **VM Migration**: Import VMs from VMware vCenter or OVF/OVA packages into KVM-based infrastructure
- **Shared Storage**: Support for NFS, Ceph RBD, and iSCSI backends for VM state persistence

## Module Structure

```
github.com/example/ha-storage
├── internal/
│   ├── ha/           # HA subsystem (controller, orchestrator, fencer, health, scheduler, policy, storage)
│   ├── api/          # REST API handlers (server, migration service, OVF parser)
│   ├── vcenter/      # vCenter client (stub for govmomi integration)
│   ├── preflight/    # Pre-flight compatibility checks for VM migration
│   └── migration/    # Migration job tracking and execution
└── go.mod            # Go 1.23.6
```

## Core Components

### HA Subsystem (`internal/ha`)

| Component | File | Purpose |
|-----------|------|---------|
| Controller | `controller.go` | Main HA loop, health check ticker, failure detection |
| Orchestrator | `orchestrator.go` | Coordinates VM restarts after host failure |
| Fencer | `fencer.go` | Node power-off via IPMI, Redfish, or SSH |
| Health Processor | `health.go` | Heartbeat processing, state tracking (online/suspect/offline) |
| Scheduler | `scheduler.go` | Host selection for VM restart (binpack/spread/NUMA-aware) |
| Policy Manager | `policy.go` | Per-VM HA policy (auto/manual/never/max-one) |
| Storage Manager | `storage.go` | Shared storage backends (NFS, Ceph RBD, iSCSI) |

### API Layer (`internal/api`)

| Component | File | Purpose |
|-----------|------|---------|
| Server | `server.go` | HTTP server with migration/import endpoints |
| Migration Service | `migration.go` | Import job lifecycle management |
| OVF Parser | `ovf.go` | OVF/OVA XML parsing and VM extraction |

### Migration Pipeline (`internal/migration`)

| Component | File | Purpose |
|-----------|------|---------|
| Tracker | `tracker.go` | Job state management, progress tracking |
| Job Executor | `tracker.go` | Async job execution (discover → preflight → import → validate) |

### vCenter Integration (`internal/vcenter`)

| Component | File | Purpose |
|-----------|------|---------|
| Client | `client.go` | vCenter API client (stub for govmomi) |

### Pre-flight Checks (`internal/preflight`)

| Component | File | Purpose |
|-----------|------|---------|
| Checker | `checker.go` | Compatibility validation (CPU, memory, disk, OS, network) |

## Key Design Decisions

1. **Interface-Driven Design**: All major components use interfaces (Orchestrator, Fencer, Scheduler, VMProvider, HostProvider) for testability and pluggability
2. **Async Execution**: Failover and migration jobs run asynchronously with context-based cancellation
3. **Thread Safety**: All shared state protected by sync.Mutex/RWMutex
4. **Stub Implementations**: vCenter client uses stub data for development without live infrastructure
5. **Multi-Method Fencing**: Tries IPMI → Redfish → SSH in sequence with retries
6. **Configurable Policies**: HA behavior configurable per-VM and globally

## Technology Stack

- **Language**: Go 1.23.6
- **HTTP**: net/http with custom mux
- **XML**: encoding/xml for OVF parsing
- **Concurrency**: goroutines + channels + sync primitives
- **External Tools**: ipmitool, curl, ssh, mount.nfs, rbd, iscsiadm
