# HiveStack User Guide

## Table of Contents

1. [Getting Started](#getting-started)
2. [Dashboard Overview](#dashboard-overview)
3. [Managing Virtual Machines](#managing-virtual-machines)
4. [Managing Hosts](#managing-hosts)
5. [Storage Management](#storage-management)
6. [Network Management](#network-management)
7. [Backup and Restore](#backup-and-restore)
8. [Compliance Validation](#compliance-validation)
9. [CLI Reference](#cli-reference)
10. [API Reference](#api-reference)

---

## Getting Started

### First Login

1. Access the Web UI: `https://<manager-ip>:8443`
2. Default admin credentials: `admin@localhost` / (see first-boot output)
3. **Immediately change the default password**
4. Navigate to Administration → Users to create additional accounts

### Access Points

| Endpoint | Purpose |
|----------|---------|
| `https://<ip>:8443` | Web UI + REST API |
| `http://<ip>:8080` | Health checks, metrics |
| Port `44567` | gRPC (node agents) |

### Roles

| Role | Permissions |
|------|-------------|
| `admin` | Full access |
| `editor` | Create/edit resources, no user management |
| `viewer` | Read-only access |
| `operator` | VM operations (start/stop/migrate) |
| `instance_admin` | Per-tenant admin |

---

## Dashboard Overview

The dashboard provides:

- **Cluster Overview**: Host count, VM count, resource utilization
- **Health Status**: Service status, heartbeat monitoring
- **Compliance**: HANA guardrail compliance status
- **Events**: Recent cluster events and alerts

---

## Managing Virtual Machines

### Create a VM

```bash
# Via CLI
hive vm create --name web-server-01 --cpus 4 --memory 8192   --disk 50G --os ubuntu22.04 --network default

# Via API
curl -k -X POST https://<ip>:8443/api/v1/vms   -H "Authorization: Bearer $TOKEN"   -H "Content-Type: application/json"   -d '{"name":"web-server-01","cpus":4,"memory_bytes":8589934592,"disk_bytes":53687091200}'
```

### VM Operations

```bash
hive vm start <vm-id>
hive vm stop <vm-id>
hive vm restart <vm-id>
hive vm migrate <vm-id> --target-host <host-id>
hive vm snapshot <vm-id> --name pre-update
hive vm restore <vm-id> --snapshot <snapshot-id>
hive vm delete <vm-id>
```

### HANA VM Requirements

SAP HANA VMs have additional guardrails enforced:

- NUMA pinning required
- Hugepages enabled
- Dedicated vCPU allocation (no overcommit)
- Memory reservation equals requested memory
- No ballooning
- No swap

### Snapshots

```bash
# Create snapshot
hive vm snapshot <vm-id> --name before-upgrade

# List snapshots
hive vm snapshots <vm-id>

# Restore
hive vm restore <vm-id> --snapshot <snapshot-id>
```

---

## Managing Hosts

### Register a Host

1. Install the HiveStack Node Agent on the host
2. Configure `/etc/hivestack/node.yaml` with Manager address
3. Start the node service: `systemctl start hivestack-node`
4. Register via Manager API:

```bash
curl -k -X POST https://<ip>:8443/api/v1/hosts   -H "Authorization: Bearer $TOKEN"   -d '{"name":"node-01","address":"192.168.1.101","grpc_port":44567}'
```

### Host Status

```bash
hive host list
hive host status <host-id>
hive host maintenance <host-id> --enable
hive host maintenance <host-id> --disable
```

---

## Storage Management

### Storage Backends

| Backend | Protocol | Use Case |
|---------|----------|----------|
| NFS | v4 | General purpose |
| Ceph RBD | RADOS | High performance |
| iSCSI | iSCSI | Block storage |

### Create Storage Pool

```bash
hive storage create --name nfs-pool --type nfs   --server 192.168.1.50 --path /export/hivestack
```

---

## Network Management

### Create Network

```bash
hive network create --name prod-net --bridge br-prod   --vlan 100 --subnet 10.0.100.0/24
```

---

## Backup and Restore

### VM Backup

```bash
hive backup create --vm <vm-id> --name daily-backup
hive backup restore --vm <vm-id> --backup <backup-id>
```

### Database Backup

```bash
pg_dump -h localhost -U hivestack hivestack > backup.sql
psql -h localhost -U hivestack hivestack < backup.sql
```

---

## Compliance Validation

### HANA Guardrails

```bash
# Validate a VM profile
hive compliance validate --vm <vm-id>

# Generate drift report
hive compliance drift --tenant <tenant-id>
```

---

## CLI Reference

### Global Flags

| Flag | Description |
|------|-------------|
| `--config` | Config file path |
| `--output` | Output format (table, json, yaml) |
| `--server` | Manager API URL |
| `--token` | Auth token |

### Commands

```bash
hive config set-token <token>
hive user me
hive vm list
hive vm get <id>
hive host list
hive storage list
hive network list
hive events list
hive compliance validate
```

---

## API Reference

### Authentication

```bash
# Login
curl -k -X POST https://<ip>:8443/api/v1/auth/login   -d '{"email":"admin@localhost","password":"changeme"}'

# Use token
curl -k -H "Authorization: Bearer $TOKEN" https://<ip>:8443/api/v1/vms
```

### Endpoints

| Method | Path | Description |
|--------|------|-------------|
| POST | `/api/v1/auth/login` | Login |
| GET | `/api/v1/vms` | List VMs |
| POST | `/api/v1/vms` | Create VM |
| GET | `/api/v1/vms/{id}` | Get VM |
| PUT | `/api/v1/vms/{id}` | Update VM |
| DELETE | `/api/v1/vms/{id}` | Delete VM |
| POST | `/api/v1/vms/{id}/start` | Start VM |
| POST | `/api/v1/vms/{id}/stop` | Stop VM |
| GET | `/api/v1/hosts` | List hosts |
| POST | `/api/v1/hosts` | Register host |
| GET | `/api/v1/storage` | List storage pools |
| GET | `/api/v1/networks` | List networks |
| GET | `/api/v1/health` | Health check |
| GET | `/api/v1/events` | List events |

### Example: Full VM Lifecycle

```bash
# 1. Login
TOKEN=$(curl -sk -X POST https://<ip>:8443/api/v1/auth/login   -d '{"email":"admin@localhost","password":"changeme"}' | jq -r .token)

# 2. Create VM
VM_ID=$(curl -sk -X POST https://<ip>:8443/api/v1/vms   -H "Authorization: Bearer $TOKEN"   -d '{"name":"test-vm","cpus":2,"memory_bytes":4294967296}' | jq -r .id)

# 3. Start VM
curl -sk -X POST https://<ip>:8443/api/v1/vms/$VM_ID/start   -H "Authorization: Bearer $TOKEN"

# 4. Check status
curl -sk https://<ip>:8443/api/v1/vms/$VM_ID   -H "Authorization: Bearer $TOKEN" | jq .status

# 5. Stop VM
curl -sk -X POST https://<ip>:8443/api/v1/vms/$VM_ID/stop   -H "Authorization: Bearer $TOKEN"
```

---

## Next Steps

- [Administrator Guide](ADMIN.md) — Installation and configuration
- [Troubleshooting](TROUBLESHOOTING.md) — Common issues and solutions
- [API Documentation](api-reference.md) — Full API specification
