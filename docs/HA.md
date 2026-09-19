# HiveStack High Availability (HA)

## Overview

HiveStack provides Non-HANA High Availability for virtualized infrastructure. When a host fails, the HA subsystem automatically detects the failure, fences the failed node, and restarts affected VMs on healthy hosts.

## HA Defaults

| Parameter | Default Value | Description |
|-----------|---------------|-------------|
| Heartbeat Interval | 30 seconds | How often hosts send health signals |
| Suspect Threshold | 3 missed beats (90s) | Mark host as suspect after missing 3 heartbeats |
| Offline Threshold | 5 missed beats (150s) | Mark host as failed after missing 5 heartbeats |
| Failover Timeout | 5 minutes | Maximum time for a failover operation |
| Auto-Restart SLA | 5 minutes | VM restarted on healthy host within 5 minutes |

## Health States

```
┌──────────┐   missed heartbeats   ┌──────────┐   missed heartbeats   ┌──────────┐
│  ONLINE  │ ────────────────────► │  SUSPECT │ ────────────────────► │ OFFLINE  │
│          │ ────────────────────► │          │                       │          │
└──────────┘   heartbeat received  └──────────┘                       └──────────┘
     ▲                                                                   │
     └───────────────────────────────────────────────────────────────────┘
                              heartbeat received (recovery)
```

### State Transitions

| From | To | Condition |
|------|-----|-----------|
| Online | Suspect | Missed heartbeats ≥ SuspectThreshold (3) |
| Suspect | Offline | Missed heartbeats ≥ OfflineThreshold (5) |
| Suspect | Online | Heartbeat received before Offline threshold |
| Offline | Online | Heartbeat received (node recovered) |

## Failover Pipeline

```
Host Failure Detected
        │
        ▼
┌─────────────────┐
│ 1. Detect Failure│ ◄── HeartbeatProcessor.CheckAll()
└────────┬────────┘
         │
         ▼
┌─────────────────┐
│ 2. Fence Node   │ ◄── MultiFencer tries IPMI → Redfish → SSH
└────────┬────────┘
         │
         ▼
┌─────────────────┐
│ 3. Get Affected │ ◄── VMProvider.GetVMsByHost()
│    VMs          │
└────────┬────────┘
         │
         ▼
┌─────────────────┐
│ 4. Filter by HA │ ◄── Only HAModeAuto and HAModeMaxOne VMs
│    Policy       │
└────────┬────────┘
         │
         ▼
┌─────────────────┐
│ 5. Select Target│ ◄── Scheduler.SelectTargets()
│    Hosts        │     (binpack/spread/NUMA-aware)
└────────┬────────┘
         │
         ▼
┌─────────────────┐
│ 6. Restart VMs  │ ◄── VMRestarter.StartVM()
└────────┬────────┘
         │
         ▼
    ┌─────────┐
    │ Complete │
    └─────────┘
```

## Fencing Methods

### IPMI (Intelligent Platform Management Interface)
- **Method**: `ipmitool chassis power off`
- **Interface**: IPMI v2.0+ (lanplus)
- **Default Port**: 623
- **Retry**: 3 attempts with 10s interval

### Redfish (RESTful API)
- **Method**: POST to `/redfish/v1/Systems/1/Actions/ComputerSystem.Reset`
- **Payload**: `{"ResetType": "ForceOff"}`
- **Auth**: Basic auth (username:password)
- **Retry**: 3 attempts with 10s interval

### SSH
- **Method**: `ssh user@host "sudo poweroff -f"`
- **Auth**: SSH key or password
- **Retry**: 3 attempts with 10s interval

### Multi-Fencer
- Tries all configured methods in order
- Returns on first success
- Records all attempts in results log

## HA Policies (Per-VM)

| Mode | Behavior |
|------|----------|
| `auto` | Automatically restart VM on healthy host |
| `manual` | Require operator intervention |
| `never` | Do not restart (best-effort) |
| `max-one` | Restart but ensure at most one instance runs |

### Policy Attributes
- **Priority**: Lower number = higher priority (restarted first)
- **Max Restarts**: Rate limiting within time window
- **Anti-Affinity**: VM IDs that must not share a host

## Scheduler Policies

| Policy Type | Description |
|-------------|-------------|
| `binpack` | Pack VMs onto fewest hosts |
| `spread` | Distribute VMs across many hosts (default) |
| `numa-aware` | Prefer NUMA-aligned placement |

### Scoring Factors
- Resource availability (memory headroom)
- VM count (spread policy)
- NUMA affinity bonus
- Anti-affinity penalty (-100)
- Label affinity bonus (+5)

## Shared Storage Integration

### Supported Backends

| Backend | CLI Tool | Capabilities |
|---------|----------|--------------|
| NFS | mount.nfs | Mount/unmount NFS exports |
| Ceph RBD | rbd | Map/unmap RADOS block devices |
| iSCSI | iscsiadm | Login/logout iSCSI targets |

### Storage Manager
- Coordinates multiple backends
- `AvailableBackends()` filters by CLI tool availability
- Used during failover to attach shared storage to target hosts

## Configuration

```go
// HA Controller Configuration
cfg := ha.ControllerConfig{
    HeartbeatProcessor: processor,
    Orchestrator:       orchestrator,
    Fencer:             fencer,
    Scheduler:          scheduler,
    Threshold: ha.HealthThresholds{
        HeartbeatInterval: 30 * time.Second,
        SuspectThreshold:  3,
        OfflineThreshold:  5,
    },
    FailoverTimeout: 5 * time.Minute,
}
```

## Monitoring & Metrics

| Metric | Type | Description |
|--------|------|-------------|
| `ha_nodes_online` | Gauge | Number of online nodes |
| `ha_nodes_suspect` | Gauge | Number of suspect nodes |
| `ha_nodes_offline` | Gauge | Number of offline nodes |

### Events
- `ha_node_suspect` (warning) — Node missed heartbeats
- `ha_node_offline` (critical) — Node declared offline
- `ha_failover_started` (warning) — Failover initiated
- `ha_failover_complete` (info/warning) — Failover finished
- `ha_failover_failed` (error) — Failover failed
