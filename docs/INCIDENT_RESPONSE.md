# HiveStack Incident Response Procedures

**Version:** 1.0  
**Effective Date:** 2026-09-19  
**Owner:** HiveStack Operations Team  
**Classification:** Internal Operations

---

## 1. Purpose

This document defines the incident response procedures for HiveStack, ensuring rapid detection, communication, resolution, and post-incident learning.

---

## 2. Incident Response Lifecycle

```
Detect → Triage → Mitigate → Resolve → Post-Incident Review
```

### Phase 1: Detection
- Automated monitoring alerts (Prometheus/Alertmanager)
- User reports (support tickets, Slack)
- On-call engineer observation

### Phase 2: Triage
- Assign severity (SEV1–SEV4)
- Identify affected components
- Determine scope (single VM, host, cluster, full service)
- Notify appropriate responders

### Phase 3: Mitigate
- Apply immediate workaround if possible
- Reduce blast radius (isolate affected components)
- Communicate status to stakeholders

### Phase 4: Resolve
- Implement permanent fix
- Verify resolution via monitoring
- Close incident

### Phase 5: Post-Incident Review
- Document timeline and root cause
- Identify action items
- Update runbooks and monitoring

---

## 3. On-Call Rotation

### 3.1 Rotation Schedule

| Week | Primary On-Call | Secondary (Escalation) |
|------|-----------------|------------------------|
| Week 1 | TATHENA (CTO) | HERMES (Lead Engineer) |
| Week 2 | HERMES (Lead Engineer) | ORACLE (AI Strategy) |
| Week 3 | ORACLE (AI Strategy) | ATHENA (CTO) |
| Week 4 | TATHENA (CTO) | HERMES (Lead Engineer) |

### 3.2 On-Call Responsibilities

**Primary:**
- Respond to alerts within SLA timeframes
- Acknowledge and triage incidents
- Escalate if unable to resolve within 30 minutes
- Document all actions in incident log

**Secondary (Escalation):**
- Available if primary is unreachable
- Takes over if primary requests assistance
- Provides domain expertise for complex issues

### 3.3 Contact Methods

1. **PagerDuty** (Primary) — automatic alert routing
2. **Slack** — #hivestack-incidents channel
3. **Email** — incidents@hivestack.internal
4. **Phone** — emergency escalation only

---

## 4. Incident Severity Procedures

### SEV1 — Critical (Complete Outage)

**Response Time:** 15 minutes  
**Resolution Target:** 4 hours

**Immediate Actions:**
1. On-call acknowledges alert
2. War room initiated (Zoom/Slack huddle)
3. Status page updated (status.hivestack.internal)
4. Stakeholder notification sent
5. Begin mitigation

**Escalation Path:**
```
On-Call (15 min) → Engineering Lead (30 min) → CTO (1 hour) → CEO (2 hours)
```

**Communication Cadence:**
- Every 30 minutes during active incident
- Immediate update on status change
- Final summary within 1 hour of resolution

### SEV2 — High (Major Feature Broken)

**Response Time:** 1 hour  
**Resolution Target:** 8 hours

**Immediate Actions:**
1. On-call acknowledges and assesses
2. Create incident ticket (EVO project)
3. Notify affected users if user-facing
4. Begin investigation

**Escalation Path:**
```
On-Call (1 hour) → Engineering Lead (2 hours) → CTO (4 hours)
```

### SEV3 — Medium (Degraded Performance)

**Response Time:** 4 hours  
**Resolution Target:** 24 hours

**Actions:**
1. Create incident ticket
2. Investigate during business hours
3. Apply fix in next maintenance window if non-urgent

### SEV4 — Low (Minor Issue)

**Response Time:** 1 business day  
**Resolution Target:** 72 hours

**Actions:**
1. Create backlog ticket
2. Address in regular sprint planning

---

## 5. Common Incident Runbooks

### 5.1 Manager Service Down

**Symptoms:** API returns 502/503, UI unreachable

**Diagnosis Steps:**
```bash
# Check Manager service status
sudo systemctl status hivestack-manager

# Check logs
sudo journalctl -u hivestack-manager --since "10 minutes ago"

# Check database connectivity
curl -s https://localhost:8443/api/health

# Check resource usage
df -h /var/lib/hivestack
free -m
```

**Mitigation:**
1. Restart Manager: `sudo systemctl restart hivestack-manager`
2. If restart fails, check database: `sudo systemctl status postgresql`
3. If database issue, failover to standby
4. If resource exhaustion, clear logs/temp: `sudo /opt/hivestack/scripts/cleanup-logs.sh`

### 5.2 Node Agent Unresponsive

**Symptoms:** Host shows "offline" in UI, VMs on host unreachable

**Diagnosis Steps:**
```bash
# SSH to affected node
ssh hivestack-node-<id>

# Check agent status
sudo systemctl status hivestack-node-agent

# Check heartbeat logs
sudo journalctl -u hivestack-node-agent --since "5 minutes ago"

# Check network connectivity to Manager
curl -k https://<manager>:8443/api/health

# Check resource usage
top -bn1 | head -20
```

**Mitigation:**
1. Restart agent: `sudo systemctl restart hivestack-node-agent`
2. If node is truly down, HA will restart VMs on other hosts
3. If fencing needed: `sudo hive node fence <node-id>`

### 5.3 VM Migration Failure

**Symptoms:** Migration job stuck/failed, VM inaccessible

**Diagnosis Steps:**
```bash
# Check migration job status
curl -s https://<manager>:8443/api/v1/migration/jobs/<job-id>

# Check source and target host connectivity
ping <target-host>

# Check storage accessibility on target
ssh <target-host> "ls -la /var/lib/hivestack/disks/"

# Check libvirt logs on source
ssh <source-host> "sudo journalctl -u libvirtd --since '10 minutes ago'"
```

**Mitigation:**
1. Cancel stuck job: `DELETE /api/v1/migration/jobs/<id>`
2. Verify VM is still running on source
3. Retry migration with adjusted parameters
4. If VM is corrupted, restore from snapshot

### 5.4 Storage Pool Full

**Symptoms:** VM creation fails, snapshot failures, disk I/O errors

**Diagnosis Steps:**
```bash
# Check pool usage
curl -s https://<manager>:8443/api/v1/storage/pools

# Check ZFS pool status
zpool list
zfs list -o name,used,avail,refer

# Find large consumers
zfs list -o name,used -s used | tail -20
```

**Mitigation:**
1. Delete old snapshots: `hive storage snapshot cleanup --older-than 7d`
2. Expand pool if possible: `zpool add <pool> <new-disk>`
3. Migrate VMs to another pool
4. Emergency: delete failed/temp disks

### 5.5 HA Failover Storm

**Symptoms:** Multiple VMs restarting, cluster instability

**Diagnosis Steps:**
```bash
# Check HA controller logs
sudo journalctl -u hivestack-ha --since "30 minutes ago"

# Check cluster health
curl -s https://<manager>:8443/api/v1/ha/status

# Check for network partition
corosync-cfgtool -s
```

**Mitigation:**
1. If network partition: resolve network issue first
2. If cascading failures: pause HA temporarily
   ```
   curl -X POST https://<manager>:8443/api/ha/pause
   ```
3. Stabilize one host at a time
4. Resume HA: `curl -X POST https://<manager>:8443/api/ha/resume`

---

## 6. Communication Templates

### Initial Notification (SEV1/SEV2)

```
Subject: [INCIDENT] <SEV> - <Brief Description>

Status: Investigating
Impact: <What is affected>
Started: <Timestamp>
Affected Components: <List>

We are investigating reports of <symptoms>. 
Updates will be provided every 30 minutes.

Incident Commander: <Name>
```

### Status Update

```
Subject: [UPDATE] <SEV> - <Brief Description>

Status: <Identified/Mitigating/Monitoring>
Current State: <What we know>
Actions Taken: <List>
Next Steps: <What's happening next>
ETA: <Estimated resolution time>
```

### Resolution Notice

```
Subject: [RESOLVED] <SEV> - <Brief Description>

Status: Resolved
Duration: <Start> to <End> (<Total time>)
Root Cause: <Brief description>
Resolution: <What was done>

A detailed post-incident review will be shared within 48 hours.
```

---

## 7. Post-Incident Review (PIR)

### Timeline
- **Within 24 hours:** Initial incident report filed
- **Within 48 hours:** Blameless post-mortem document
- **Within 1 week:** Action items created and assigned

### Post-Mortem Template

```markdown
# Post-Incident Review: <Incident Title>

**Date:** YYYY-MM-DD  
**Severity:** SEV1/2/3/4  
**Duration:** HH:MM  
**Author:** <Name>  

## Summary
Brief description of what happened.

## Impact
- Users affected: <number>
- Duration: <time>
- SLO impact: <error budget consumed>

## Timeline
| Time | Event |
|------|-------|
| HH:MM | Detection |
| HH:MM | Acknowledged |
| HH:MM | Mitigation started |
| HH:MM | Resolved |

## Root Cause
Detailed explanation of why the incident occurred.

## What Went Well
- ...

## What Went Wrong
- ...

## Action Items
| Action | Owner | Priority | Due Date |
|--------|-------|----------|----------|
| ... | ... | P1/P2 | YYYY-MM-DD |

## Lessons Learned
- ...
```

---

## 8. Incident Tracking

All incidents are tracked as issues in Paperclip:
- **Project:** HiveStack
- **Label:** `incident`
- **Template:** Use PIR template above

---

*Last updated: 2026-09-19*
