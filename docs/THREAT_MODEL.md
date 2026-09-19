# HiveStack Threat Model

## Scope

This threat model applies to the HiveStack control plane, including:
- REST API server
- HA controller and orchestrator
- Migration pipeline (OVF upload, vCenter import)
- Storage backends (NFS, Ceph RBD, iSCSI)
- Fencing agents (IPMI, Redfish, SSH)

## STRIDE Analysis

### Spoofing (Identity)

| ID | Threat | Component | Impact | Likelihood | Mitigation |
|----|--------|-----------|--------|------------|------------|
| S-01 | Attacker impersonates API client | API Server | Unauthorized migration/failover operations | Medium | mTLS, JWT authentication (planned) |
| S-02 | Attacker forges heartbeat messages | HA Controller | False health state transitions, unnecessary failovers | Medium | Heartbeat authentication, network segmentation |
| S-03 | Attacker spoofs vCenter identity | vCenter Client | Connect to malicious vCenter, exfiltrate credentials | Low | TLS certificate verification, pinned certificates |
| S-04 | Attacker spoofs BMC/IPMI identity | Fencer | Power off legitimate nodes (DoS) | Medium | IPMI LAN isolation, encrypted IPMI sessions |
| S-05 | Malicious node registers as healthy host | HA Controller | VMs scheduled on compromised host | Medium | Host authentication, admission control (planned) |

### Tampering (Data Integrity)

| ID | Threat | Component | Impact | Likelihood | Mitigation |
|----|--------|-----------|--------|------------|------------|
| T-01 | Attacker modifies OVF file in transit | API Server / Network | Malicious VM imported, code execution | Low | TLS 1.3, file hash verification |
| T-02 | Attacker modifies migration job state | Tracker | Race condition, duplicate VM starts | Low | Thread-safe state machine, atomic operations |
| T-03 | Attacker tampers with HA policies | PolicyManager | Unauthorized failover behavior | Low | Policy validation, audit logging |
| T-04 | Attacker modifies heartbeat payload | Network / HA Controller | False health reporting | Low | Input validation (NodeID, Timestamp required) |
| T-05 | Attacker modifies storage backend commands | Storage Manager | Mount malicious filesystem | Low | Command whitelisting, sudo restrictions |
| T-06 | Attacker tampers with fencing configuration | Fencer | Disable fencing, enable split-brain | Medium | Configuration integrity checks, RBAC |

### Repudiation (Non-repudiation)

| ID | Threat | Component | Impact | Likelihood | Mitigation |
|----|--------|-----------|--------|------------|------------|
| R-01 | Operator denies triggering migration | API Server | Cannot audit who started migration | Medium | Structured audit logging with actor identity |
| R-02 | Operator denies modifying HA policy | PolicyManager | Cannot trace policy change | Medium | PolicyHistoryStore records actor |
| R-03 | System denies fence operation result | Fencer | Cannot prove node was fenced | Low | FenceResult logged with timestamp and method |
| R-04 | VM restart action not recorded | Orchestrator | Cannot troubleshoot failover | Low | FailoverRecord with full lifecycle |

### Information Disclosure (Confidentiality)

| ID | Threat | Component | Impact | Likelihood | Mitigation |
|----|--------|-----------|--------|------------|------------|
| I-01 | vCenter credentials exposed in logs | API Server / vCenter Client | Credential theft | Low | Password field marked `json:"-"`, not logged |
| I-02 | IPMI credentials exposed in process list | Fencer (IPMIFencer) | Credential theft via /proc | Medium | Use config files instead of CLI args (planned) |
| I-03 | API responses leak internal data | API Server | Information gathering | Low | Error messages sanitized, no stack traces |
| I-04 | OVF file contents exposed | API Server | VM configuration disclosure | Low | OVF processed in memory, not written to disk |
| I-05 | Heartbeat data reveals infrastructure topology | HA Controller | Reconnaissance for attack | Low | API endpoints require authentication (planned) |
| I-06 | Failover history exposes infrastructure details | Orchestrator | Infrastructure mapping | Low | History buffer in-memory only, access controlled |

### Denial of Service (Availability)

| ID | Threat | Component | Impact | Likelihood | Mitigation |
|----|--------|-----------|--------|------------|------------|
| D-01 | Attacker floods API with large OVF uploads | API Server | Resource exhaustion (disk, memory) | High | Multipart form size limits (500MB OVF, 2GB OVA), timeout |
| D-02 | Attacker spams heartbeat requests | HA Controller | CPU exhaustion from heartbeat processing | Medium | Heartbeat rate limiting (planned) |
| D-03 | Attacker triggers repeated failovers | Orchestrator | Service disruption, VM thrashing | Low | Failover timeout, active failover dedup, rate limiting |
| D-04 | Attacker creates excessive migration jobs | Tracker | Memory exhaustion from job tracking | Medium | Job limit, cleanup of completed jobs |
| D-05 | Network partition causes false failover detection | HA Controller | Unnecessary VM restarts, split-brain | Low | Fencing verification, suspect state before offline |
| D-06 | Slow storage backend blocks failover | Storage Manager | Failover timeout exceeded | Low | Storage operations use context with timeout |
| D-07 | vCenter unavailability blocks migrations | Migration Service | Migration backlog | High | Async job execution, retry with backoff (planned) |

### Elevation of Privilege

| ID | Threat | Component | Impact | Likelihood | Mitigation |
|----|--------|-----------|--------|------------|------------|
| E-01 | Attacker exploits OVF parser XML bomb | OVF Parser | CPU/memory exhaustion | Low | Standard xml.Unmarshal (Go resists XXE by default) |
| E-02 | Attacker uses SSH fencing to execute arbitrary commands | Fencer (SSHFencer) | Code execution on BMC | Low | Command whitelisted to `sudo poweroff -f` |
| E-03 | Attacker manipulates scheduler policy | Scheduler | VMs placed on attacker-controlled host | Low | Policy validation, host authentication (planned) |
| E-04 | Attacker exploits preflight checker | Preflight | Bypass compatibility checks | Low | Checker is read-only, no side effects |
| E-05 | Attacker uses storage backend to mount arbitrary paths | Storage Manager | Access to host filesystem | Low | Mount paths validated, no user-controlled paths |

## Attack Surface Analysis

### Network Attack Surface

| Entry Point | Protocol | Exposure | Risk |
|-------------|----------|----------|------|
| API Server (HTTP) | HTTP (planned: HTTPS) | Internal network | High |
| Heartbeat Receiver | TBD (planned: gRPC/HTTP) | Internal network | Medium |
| IPMI Interface | IPMI v2.0+ (lanplus) | Management network | Medium |
| Redfish API | HTTPS | Management network | Medium |
| SSH Fencing | SSH | Management network | Low |
| vCenter Connection | SOAP/HTTPS | Data center network | Medium |
| NFS Mount | NFS v4 | Storage network | Low |
| Ceph RBD | Ceph protocol | Storage network | Low |
| iSCSI | iSCSI | Storage network | Low |

### API Attack Surface

| Endpoint | Auth | Input Validation | Risk |
|----------|------|------------------|------|
| `/api/v1/migration/import/ovf` | None (planned: JWT) | File extension, size limit | High |
| `/api/v1/migration/import/ova` | None (planned: JWT) | File extension, size limit | High |
| `/api/v1/migration/vcenter/discover` | None (planned: JWT) | Host required | High |
| `/api/v1/migration/vcenter/import` | None (planned: JWT) | Host, VM IDs required | High |
| `/api/v1/migration/jobs/:id` | None (planned: JWT) | Job ID format | Medium |
| `/api/v1/migration/vcenter/jobs/:id` | None (planned: JWT) | Job ID format | Medium |

## Trust Boundaries

```
┌─────────────────────────────────────────────────────────────────────────────────┐
│                              Untrusted Zone                                      │
│                                                                                  │
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐  ┌─────────────────────┐ │
│  │ External    │  │ Operator    │  │ API Clients │  │ Heartbeat Senders   │ │
│  │ Internet    │  │ Workstation │  │             │  │ (Host Agents)       │ │
│  └─────────────┘  └─────────────┘  └─────────────┘  └─────────────────────┘ │
└─────────────────────────────────────────────────────────────────────────────────┘
          │                 │                 │                     │
          └─────────────────┴─────────────────┴─────────────────────┘
                                        │
                              ┌─────────┴─────────┐
                              │  API Gateway      │
                              │  (Auth + TLS)     │
                              └─────────┬─────────┘
                                        │
┌─────────────────────────────────────────────────────────────────────────────────┐
│                           Trusted Zone (Control Plane)                           │
│                                                                                  │
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐  ┌─────────────────────┐ │
│  │ API Server  │  │ HA Controller│  │ Migration   │  │ Job Executor        │ │
│  │             │  │             │  │ Tracker     │  │                     │ │
│  └─────────────┘  └─────────────┘  └─────────────┘  └─────────────────────┘ │
│                                                                                  │
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐                            │
│  │ Orchestrator│  │ Scheduler   │  │ Preflight   │                            │
│  │             │  │             │  │ Checker     │                            │
│  └─────────────┘  └─────────────┘  └─────────────┘                            │
└─────────────────────────────────────────────────────────────────────────────────┘
          │                 │                     │
          └─────────────────┴─────────────────────┘
                              │
                    ┌─────────┴─────────┐
                    │  Trusted Zone     │
                    │  (Management Net) │
                    └─────────┬─────────┘
                              │
┌─────────────────────────────────────────────────────────────────────────────────┐
│                           Infrastructure Zone                                     │
│                                                                                  │
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐  ┌─────────────────────┐ │
│  │ vCenter     │  │ BMC/IPMI    │  │ ESXi Hosts  │  │ Storage Targets     │ │
│  │ Server      │  │             │  │             │  │                     │ │
│  └─────────────┘  └─────────────┘  └─────────────┘  └─────────────────────┘ │
└─────────────────────────────────────────────────────────────────────────────────┘
```

## Risk Matrix

```
                    │                  Likelihood
                    │   Low    Medium   High
        ────────────┼──────────────────────────
        High        │  S-03    S-01     D-01
                    │  E-05    S-04     D-07
                    │  I-01    S-05     T-06
        Impact      │  I-02    D-02
                    │  R-03    D-03
                    │          T-04
        ────────────┼──────────────────────────
        Medium      │  E-04    T-05     D-04
                    │  I-06    T-03
                    │  R-04    E-02
                    │          I-05
        ────────────┼──────────────────────────
        Low         │  T-01    R-01
                    │  D-06    R-02
                    │  E-03    D-05
                    │  I-03    E-01
                    │  I-04    T-02
```

## Mitigation Recommendations (Prioritized)

### P0 - Critical (Immediate)

1. **API Authentication**: Implement JWT or mTLS for all API endpoints (addresses S-01, D-01)
2. **TLS Enforcement**: Require HTTPS for all API traffic (addresses T-01, I-01)
3. **Input Validation**: Add content-type validation for OVF uploads beyond file extension (addresses T-01, E-01)
4. **Rate Limiting**: Add rate limiting on upload endpoints (addresses D-01)

### P1 - High (Next Sprint)

5. **Audit Logging**: Structured audit logs for all state-changing operations (addresses R-01, R-02)
6. **IPMI Credential Protection**: Move from CLI args to config files with restricted permissions (addresses I-02)
7. **Heartbeat Authentication**: Sign heartbeat messages with HMAC (addresses S-02, T-04)
8. **Job Quotas**: Limit concurrent migration jobs per user/tenant (addresses D-04, D-07)

### P2 - Medium (Planned)

9. **Host Authentication**: Certificate-based host admission (addresses S-05, E-03)
10. **Fencing Configuration Integrity**: Signed fencing configs (addresses T-06)
11. **Network Segmentation**: Enforce management network isolation (addresses S-04)
12. **Secret Management**: Integrate with Vault for credential storage (addresses I-01, I-02)

### P3 - Low (Backlog)

13. **Policy RBAC**: Fine-grained access control for HA policy modifications
14. **Failover Rate Limiting**: Prevent failover flapping with cooldown periods
15. **Storage Command Auditing**: Log all storage backend commands
16. **Session Management**: API session timeouts and rotation

## Compliance Mapping

| Requirement | Status | Notes |
|-------------|--------|-------|
| Authentication | ❌ Not implemented | Planned for v1.0 |
| Authorization (RBAC) | ❌ Not implemented | Planned for v1.1 |
| Audit Logging | ⚠️ Partial | Structured logs, no SIEM integration |
| Encryption in Transit | ⚠️ Partial | API lacks TLS, Redfish uses -k |
| Encryption at Rest | ⚠️ Partial | Credentials not encrypted at rest |
| Input Validation | ✅ Implemented | Size limits, extension checks |
| Error Handling | ✅ Implemented | Graceful degradation |
| Thread Safety | ✅ Implemented | Mutex protection throughout |
| Resource Limits | ⚠️ Partial | Upload limits, no connection limits |
| Dependency Scanning | ❌ Not implemented | Add go vulncheck to CI |
