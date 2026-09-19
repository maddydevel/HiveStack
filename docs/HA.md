# HiveStack Non-HANA High Availability (Phase 4)

## Overview

HiveStack's High Availability (HA) subsystem provides automatic detection of host
failures and restart of affected virtual machines on healthy cluster nodes.
This is designed for Non-HANA workloads where RTO (Recovery Time Objective) is
measured in minutes rather than seconds.

## Architecture

```
                    ┌─────────────────────┐
                    │    HA Controller     │
                    │  (health.go,         │
                    │   controller.go)     │
                    └──────────┬──────────┘
                               │ Heartbeat Ticker (30s)
                    ┌──────────▼──────────┐
                    │ Heartbeat Processor  │
                    │  (health.go)         │
                    └──────────┬──────────┘
                               │ State Transitions
              ┌────────────────┼────────────────┐
              │                │                │
    ┌─────────▼──────┐ ┌──────▼───────┐ ┌──────▼──────────┐
    │   Fencer       │ │  Scheduler   │ │  Orchestrator   │
    │ (fencer.go)    │ │(scheduler.go)│ │(orchestrator.go) │
    │ IPMI/Redfish/  │ │ NUMA-aware   │ │ VM restart      │
    │ SSH power off  │ │ placement    │ │ coordination    │
    └────────────────┘ └──────────────┘ └─────────────────┘
                                                  │
                                    ┌─────────────▼──────────┐
                                    │     Policy Manager      │
                                    │     (policy.go)         │
                                    │ Per-VM HA policy        │
                                    └────────────────────────┘
```

## Components

### 1. Health Monitor (`health.go`)

The heartbeat processor tracks the health state of each host node:

- **State Machine**: Online → Suspect → Offline
- **Heartbeat Interval**: 30 seconds (default, configurable)
- **Suspect Threshold**: 3 missed heartbeats (90 seconds)
- **Offline Threshold**: 5 missed heartbeats (150 seconds)

Each heartbeat carries:
- Node ID and timestamp
- List of running VM IDs
- Resource utilization (CPU, memory, storage, NUMA)

### 2. Fencer (`fencer.go`)

Prevents split-brain scenarios by ensuring failed nodes are truly offline
before VM restart begins. Three fencing methods are supported:

- **IPMI**: Uses `ipmitool` with IPMI v2.0+ chassis power commands
- **Redfish**: RESTful API for modern BMC power control
- **SSH**: Direct power-off command via SSH to the host

The `MultiFencer` tries methods in order, with retry logic and timeout handling.

### 3. Scheduler (`scheduler.go`)

Selects optimal target hosts for VM restart during failover. The scheduler
considers:

- **Resource Capacity**: CPU, memory availability
- **NUMA Topology**: Prefer hosts with NUMA alignment for VM requirements
- **Anti-Affinity**: Ensure related VMs are placed on different hosts
- **Policy**: Spread (load balance) or binpack (consolidate) strategies

### 4. Orchestrator (`orchestrator.go`)

Coordinates the full failover pipeline:

1. Detect host failure via health monitor
2. Fence the failed host (prevent split-brain)
3. Identify affected VMs with auto-restart policy
4. Schedule VMs to target hosts
5. Restart VMs on new hosts

### 5. Policy Manager (`policy.go`)

Defines per-VM HA behavior:

| Mode | Behavior |
|------|----------|
| `auto` | Automatically restart on healthy host (default) |
| `manual` | Require operator intervention |
| `never` | No automatic restart |
| `max-one` | At most one instance runs (anti-split-brain) |

Policies also control:
- **Priority**: Restart order (lower number = higher priority)
- **Anti-Affinity**: VMs that must not share a host
- **Rate Limiting**: Max restart attempts within a time window

### 6. Controller (`controller.go`)

The main HA loop:
- Runs heartbeat check ticker (default: 30s)
- Processes state transitions
- Triggers failover for offline nodes
- Updates Prometheus metrics
- Emits lifecycle events

### 7. Shared Storage (`storage.go`)

Supports three shared storage backends for VM disk accessibility:

| Backend | Protocol | Use Case |
|---------|----------|----------|
| **NFS** | v3/v4 | Simple, widely supported |
| **Ceph RBD** | RADOS | Enterprise, high performance |
| **iSCSI** | SCSI over IP | Legacy SAN integration |

All backends provide `Mount()`, `Unmount()`, `GetDiskPath()`, and `IsAvailable()`.

## Failover Process

```
Host Failure Detected
        │
        ▼
┌───────────────────┐
│ 1. Mark Host      │
│    State: Offline  │
└─────────┬─────────┘
          │
          ▼
┌───────────────────┐
│ 2. Fence Host     │  ← IPMI/Redfish/SSH power off
│    (prevent       │
│   split-brain)    │
└─────────┬─────────┘
          │
          ▼
┌───────────────────┐
│ 3. Identify VMs   │  ← Filter by HA policy
│    for Restart    │     (auto/max-one only)
└─────────┬─────────┘
          │
          ▼
┌───────────────────┐
│ 4. Select Target  │  ← NUMA-aware placement
│    Hosts          │     Respect anti-affinity
└─────────┬─────────┘
          │
          ▼
┌───────────────────┐
│ 5. Restart VMs    │  ← Update DB host assignment
│    on New Hosts   │     Start via node agent
└─────────┬─────────┘
          │
          ▼
    Failover Complete
    (Target: < 5 min)
```

## REST API

| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/api/v1/ha/status` | Overall HA status and metrics |
| POST | `/api/v1/ha/failover` | Trigger manual failover for a node |
| PUT | `/api/v1/vms/{id}/ha-policy` | Set HA policy for a VM |
| GET | `/api/v1/vms/{id}/ha-policy` | Get HA policy for a VM |
| GET | `/api/v1/ha/nodes` | List health of all monitored nodes |
| GET | `/api/v1/ha/history` | Failover history |

### Example: Get HA Status

```bash
curl -H "Authorization: Bearer $TOKEN" \
  http://manager:8080/api/v1/ha/status
```

```json
{
  "enabled": true,
  "controller": "running",
  "nodes_online": 3,
  "nodes_suspect": 0,
  "nodes_offline": 0,
  "active_failovers": 0,
  "thresholds": {
    "heartbeat_interval": "30s",
    "suspect_threshold": 3,
    "offline_threshold": 5
  }
}
```

### Example: Set VM HA Policy

```bash
curl -X PUT \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"mode": "auto", "priority": 50, "anti_affinity": ["vm-002"]}' \
  http://manager:8080/api/v1/vms/vm-001/ha-policy
```

### Example: Trigger Failover

```bash
curl -X POST \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"node_id": "node-003", "force": false}' \
  http://manager:8080/api/v1/ha/failover
```

## Configuration

```yaml
ha:
  enabled: true
  heartbeat_interval: 30s       # How often nodes must report health
  suspect_threshold: 3          # Missed heartbeats before suspect (90s)
  offline_threshold: 5          # Missed heartbeats before offline (150s)
  failover_timeout: 5m          # Max time for failover operation
  
  fencing:
    enabled: true
    methods:
      - type: ipmi
        host: "192.168.100.1"  # BMC IP
        port: 623
        username: "ADMIN"
        password: "ADMIN"
      - type: redfish
        host: "192.168.100.1"
        port: 443
        username: "root"
        password: "calvin"
      - type: ssh
        host: "node-001"
        username: "root"
        ssh_key_path: "/etc/hivestack/fence_key"
  
  storage:
    type: nfs                    # nfs, ceph-rbd, iscsi
    nfs:
      server: "nfs.hivestack.local"
      export: "/hivestack/vm-disks"
      mount_point: "/var/lib/hivestack/shared"
  
  scheduler:
    policy: spread               # spread or binpack
    anti_affinity_enabled: true
    numa_aware: true
```

## SLA Targets

| Metric | Target |
|--------|--------|
| Failure Detection | < 150 seconds |
| Fencing Completion | < 30 seconds |
| VM Restart Initiation | < 30 seconds |
| Total RTO | < 5 minutes |
| Concurrent Failovers | Up to 5 nodes |
| Max VMs per Failover | 50 VMs |

## Prometheus Metrics

| Metric | Type | Description |
|--------|------|-------------|
| `hivestack_ha_nodes_online` | Gauge | Number of online nodes |
| `hivestack_ha_nodes_suspect` | Gauge | Number of suspect nodes |
| `hivestack_ha_nodes_offline` | Gauge | Number of offline nodes |
| `hivestack_ha_active_failovers` | Gauge | Active failover operations |
| `hivestack_ha_failover_duration_seconds` | Histogram | Failover operation duration |
| `hivestack_ha_restart_total` | Counter | Total VM restarts by result |

## Deployment Notes

### Prerequisites

1. **Shared Storage**: All hosts must have access to shared VM disk storage
   (NFS, Ceph RBD, or iSCSI) for VM state reconstruction.

2. **Fencing**: At least one fencing method must be configured per host.
   IPMI is recommended for physical servers; Redfish for newer hardware.

3. **Network**: The HA controller communicates with node agents via gRPC.
   Firewall rules must allow heartbeat traffic.

4. **RBAC**: HA operations require appropriate permissions:
   - `ha:failover` for triggering failovers
   - `vm:update` for setting HA policies

### Enabling HA

```go
import "github.com/maddydevel/HiveStack/internal/ha"
import "github.com/maddydevel/HiveStack/internal/manager"

// Create HA service
haSvc, err := manager.NewHAService(mgr, ha.DefaultThresholds())
if err != nil {
    log.Fatal(err)
}

// Configure fencing
fenceCfg := ha.DefaultFenceConfig()
fenceCfg.Host = "192.168.100.1"
fenceCfg.Username = "ADMIN"
fenceCfg.Password = "ADMIN"
fencer, _ := ha.NewFencer(fenceCfg)
haSvc.SetFencer(fencer)

// Start
haSvc.Start(ctx)
```

## File Layout

```
internal/ha/
├── health.go       # Heartbeat processing and health state machine
├── fencer.go       # IPMI/Redfish/SSH fencing implementations
├── scheduler.go    # Host selection with NUMA and anti-affinity
├── orchestrator.go # VM restart coordination
├── policy.go       # Per-VM HA policy management
├── controller.go   # Main HA loop and failure detection
└── storage.go      # Shared storage backends (NFS/Ceph/iSCSI)

internal/api/
└── ha_handlers.go  # REST API handlers for HA operations

internal/manager/
└── ha_service.go   # HA service integration with Manager

docs/
└── HA.md           # This documentation
```

## See Also

- [HiveStack Architecture Overview](../README.md)
- [Node Agent Documentation](../internal/node/README.md)
- [Compliance Guardrails](../internal/compliance/README.md)
