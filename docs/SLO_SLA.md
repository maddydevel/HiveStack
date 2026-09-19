# HiveStack Service Level Objectives & Agreements

**Version:** 1.0  
**Effective Date:** 2026-09-19  
**Owner:** HiveStack Operations Team  
**Review Cycle:** Quarterly

---

## 1. Service Overview

HiveStack is a KVM-based virtualization appliance providing VMware migration capabilities, VM lifecycle management, high availability, and multi-tenant hosting on SUSE SLES 15 SP7.

### Service Components

| Component | Description | Criticality |
|-----------|-------------|-------------|
| Manager API | REST API for all management operations | Critical |
| Manager UI | Web-based management console | Critical |
| Node Agent | Per-host agent for VM operations | Critical |
| HA Controller | Host failure detection and VM restart | Critical |
| Migration Service | VMware import and conversion | High |
| Storage Backend | ZFS, LVM, NFS, iSCSI, Ceph RBD | High |
| Network Layer | Bridges, OVS, VLANs | High |
| Monitoring | Prometheus metrics, alerting | Medium |
| Backup System | VM backup and restore | High |

---

## 2. Service Level Objectives (SLOs)

### 2.1 Availability SLOs

| Metric | Target | Measurement Period | Calculation |
|--------|--------|-------------------|-------------|
| Manager API Uptime | 99.9% | Monthly | (Total time - Downtime) / Total time |
| Manager UI Uptime | 99.9% | Monthly | (Total time - Downtime) / Total time |
| VM Availability (HA-enabled) | 99.95% | Monthly | Per-VM uptime with auto-restart |
| Migration Service Availability | 99.0% | Monthly | Service responsive / Total time |

**Error Budget:** 0.1% monthly downtime = ~43 minutes

### 2.2 Performance SLOs

| Metric | Target | Measurement | Threshold |
|--------|--------|-------------|-----------|
| API Response Time (p50) | < 100ms | Per-request latency | Alert at > 200ms |
| API Response Time (p99) | < 500ms | Per-request latency | Alert at > 1000ms |
| VM Create Operation | < 30 seconds | Time to VM running | Alert at > 60s |
| VM Start Operation | < 15 seconds | Time to VM running | Alert at > 30s |
| Live Migration Downtime | < 1 second | Network interruption | Alert at > 5s |
| VM Failover (HA) | < 5 minutes | Detection to restart | Alert at > 10s |
| Migration Import Rate | > 50 MB/s | Disk throughput | Alert at < 20 MB/s |

### 2.3 Reliability SLOs

| Metric | Target | Measurement | Threshold |
|--------|--------|-------------|-----------|
| Backup Success Rate | > 99.0% | Successful / Total backups | Alert at < 95% |
| Restore Success Rate | > 99.5% | Successful / Total restores | Alert at < 98% |
| Data Durability | 99.999999999% (11 nines) | Annual | ZFS checksums + replication |
| Snapshot Success Rate | > 99.0% | Successful / Total snapshots | Alert at < 95% |

### 2.4 Support SLOs

| Priority | Response Time | Resolution Target | Escalation |
|----------|--------------|-------------------|------------|
| P1 - Critical (Production down) | 15 minutes | 4 hours | 1 hour |
| P2 - High (Major feature broken) | 1 hour | 8 hours | 4 hours |
| P3 - Medium (Partial impact) | 4 hours | 24 hours | 12 hours |
| P4 - Low (Cosmetic/Question) | 1 business day | 72 hours | N/A |

---

## 3. Service Level Agreements (SLAs)

### 3.1 Standard Support Hours

- **Business Hours:** Monday–Friday, 09:00–18:00 IST (UTC+05:30)
- **After-Hours:** P1/P2 only, via on-call rotation
- **Holiday Coverage:** On-call for P1 only

### 3.2 Uptime Guarantee (Enterprise)

| Tier | Monthly Uptime | Credit on Breach |
|------|---------------|------------------|
| Standard | 99.5% | 10% monthly fee credit |
| Premium | 99.9% | 15% monthly fee credit |
| Enterprise | 99.95% | 25% monthly fee credit |

### 3.3 Exclusions

SLA credits do not apply to:
- Scheduled maintenance windows (announced 72h in advance)
- Force majeure events
- Customer-caused outages (misconfiguration, resource exhaustion)
- Third-party service failures outside HiveStack control
- Beta/pre-release features

---

## 4. Monitoring & Measurement

### 4.1 SLO Monitoring

All SLOs are tracked via Prometheus + Grafana dashboards:

- **Dashboard:** `dashboards/hivestack-operations.json`
- **Metrics Endpoint:** `https://<manager>/metrics`
- **Alertmanager:** Configured for SLO burn rate alerts

### 4.2 Burn Rate Alerts

| Alert Name | Condition | Severity | Action |
|------------|-----------|----------|--------|
| SLOBurnRateFast | 2% budget consumed in 1 hour | Critical | Page on-call |
| SLOBurnRateMedium | 5% budget consumed in 6 hours | High | Slack + email |
| SLOBurnRateSlow | 10% budget consumed in 3 days | Medium | Email notification |

### 4.3 Monthly SLO Report

Generated automatically on the 1st of each month:
- Actual vs target for each SLO
- Error budget remaining
- Incident summary
- Top contributing factors
- Recommendations

---

## 5. Incident Classification

| Severity | Description | Examples |
|----------|-------------|----------|
| SEV1 | Complete service outage | All VMs down, Manager unreachable, data loss |
| SEV2 | Major feature unavailable | HA not working, migration failing, API 500s |
| SEV3 | Degraded performance | Slow API, partial VM issues, monitoring gaps |
| SEV4 | Minor issues | Single VM problem, cosmetic UI bugs |

---

## 6. Review & Updates

- **Quarterly Review:** SLO targets assessed against actual performance
- **Post-Incident Review:** SLO impact documented, targets adjusted if needed
- **Annual Review:** Full SLA review with stakeholders

---

*Last updated: 2026-09-19*
