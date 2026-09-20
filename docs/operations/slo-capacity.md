# HiveStack — SLOs & Capacity Planning

## Service Level Objectives (SLOs)

| Metric | Target | Measurement | Window |
|--------|--------|-------------|--------|
| API Response Time (p99) | < 500ms | Histogram from Prometheus | 30 days |
| Availability | 99.5% | up{job="hivestack-manager"} | 30 days |
| Recovery Time Objective (RTO) | < 5 minutes | Host failover duration | Incident |
| Recovery Point Objective (RPO) | < 15 minutes | Cost attribution data lag | Incident |
| Error Rate | < 0.1% | 5xx responses / total | 24 hours |
| VM Provision Time | < 5 minutes | POST /vms to running state | Per request |

## Error Budget

Based on 99.5% availability:

| Time Window | Allowed Downtime | Error Budget |
|-------------|------------------|--------------|
| 30 days | 1 hour 12 minutes | 0.5% |
| 7 days | 18 minutes 8 seconds | 0.5% |

## Capacity Planning

### Current Design Limits

| Resource | Limit | Notes |
|----------|-------|-------|
| VMs | 5,000 | Across all clusters |
| Clusters | 20 | Per Manager instance |
| Nodes | 50 | Per cluster |
| vCPUs per VM | 256 | KVM limit |
| Memory per VM | 4 TB | KVM limit |
| Concurrent Users | 200 | Tested load |
| API Requests/sec | 1,000 | With Redis caching |

### Scaling Triggers

| Metric | Threshold | Action |
|--------|-----------|--------|
| CPU > 70% | Sustained 5min | Add node or migrate VMs |
| Memory > 80% | Sustained 5min | Add node or rightsizing |
| Storage > 85% | Alert | Expand pool or archive VMs |
| API p99 > 400ms | Sustained 5min | Scale Manager replicas |
| DB connections > 80% | Alert | Increase pool size |

### Resource Planning Formula

```
Required Nodes = ceil(Total VMs × Avg vCPUs per VM / Target vCPUs per Node) + HA Buffer

Where:
- Target vCPUs per Node = Physical Cores × 0.8 (20% headroom)
- HA Buffer = 1 node per cluster (N+1)
```

### Example: 1000 VM Deployment

- Average VM: 4 vCPUs, 16 GB RAM
- Node: 64 cores, 512 GB RAM
- Target vCPUs per Node = 64 × 0.8 = 51
- Required Nodes = ceil(1000 × 4 / 51) + 1 = 79 + 1 = 80 nodes
- Clusters: 80 / 50 = 2 clusters
