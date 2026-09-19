# HiveStack REST API Reference

**Version:** 0.1.0
**Base URL:** `https://localhost:8443/api/v1` (production) or `http://localhost:8080/api/v1` (development)

## Authentication

All endpoints (except `/health` and `/auth/login`) require a JWT Bearer token.

```
Authorization: Bearer <token>
```

Obtain a token via `POST /api/v1/auth/login`.

---

## Health

### GET /health

Health check endpoint. No authentication required.

**Response 200:**
```json
{
  "status": "ok",
  "version": "0.1.0",
  "database": "ok",
  "timestamp": "2026-01-01T00:00:00Z"
}
```

---

## Auth

### POST /auth/login

Authenticate and receive a JWT token.

**Request:**
```json
{
  "email": "admin@example.com",
  "password": "secret"
}
```

**Response 200:**
```json
{
  "user": { "id": "uuid", "name": "Admin", "email": "admin@example.com", "role": "admin" },
  "token": "eyJhbGciOi...",
  "expiresAt": "2026-01-01T00:00:00Z"
}
```

### POST /auth/logout

Log out (invalidate token).

**Response 200:** Logged out

### GET /auth/me

Get current authenticated user.

**Response 200:** User object

---

## Users

| Method | Path | Description |
|--------|------|-------------|
| GET | /users | List users (supports `role`, `limit`, `offset` query params) |
| POST | /users | Create user |
| GET | /users/{id} | Get user by ID |
| PUT | /users/{id} | Update user |
| DELETE | /users/{id} | Delete user |

**Create User Request:**
```json
{
  "name": "John Doe",
  "email": "john@example.com",
  "password": "securepassword",
  "role": "viewer"
}
```

Roles: `admin`, `operator`, `viewer`, `auditor`

---

## Datacenters

| Method | Path | Description |
|--------|------|-------------|
| GET | /datacenters | List datacenters |
| POST | /datacenters | Create datacenter |
| GET | /datacenters/{id} | Get datacenter |
| DELETE | /datacenters/{id} | Delete datacenter |

---

## Clusters

| Method | Path | Description |
|--------|------|-------------|
| GET | /clusters | List clusters (filter by `datacenterId`) |
| POST | /clusters | Create cluster |
| GET | /clusters/{id} | Get cluster |
| GET | /clusters/{id}/hosts | List hosts in cluster |

---

## Hosts

| Method | Path | Description |
|--------|------|-------------|
| GET | /hosts | List hosts (filter by `clusterId`, `status`) |
| POST | /hosts | Register host |
| GET | /hosts/{id} | Get host |
| PUT | /hosts/{id} | Update host |
| DELETE | /hosts/{id} | Remove host |
| GET | /hosts/{id}/status | Real-time host status |
| POST | /hosts/{id}/maintenance | Enter maintenance mode |
| DELETE | /hosts/{id}/maintenance | Exit maintenance mode |

**Register Host Request:**
```json
{
  "name": "node-01",
  "hostname": "node01.example.com",
  "ipAddress": "10.0.0.1",
  "clusterId": "uuid",
  "agentToken": "node-agent-auth-token"
}
```

---

## VMs

| Method | Path | Description |
|--------|------|-------------|
| GET | /vms | List VMs (filter by `hostId`, `clusterId`, `status`, `search`) |
| POST | /vms | Create VM |
| GET | /vms/{id} | Get VM |
| PUT | /vms/{id} | Update VM |
| DELETE | /vms/{id} | Delete VM |
| POST | /vms/{id}/start | Start VM |
| POST | /vms/{id}/stop | Stop VM |
| POST | /vms/{id}/restart | Restart VM |
| POST | /vms/{id}/migrate | Migrate VM |
| GET | /vms/{id}/snapshots | List snapshots |
| POST | /vms/{id}/snapshots | Create snapshot |
| DELETE | /vms/{id}/snapshots/{snapshotId} | Delete snapshot |
| GET | /vms/{id}/console | Get console URL (VNC/SPICE) |
| GET | /vms/{id}/stats | Get VM statistics |

**Create VM Request:**
```json
{
  "name": "my-vm",
  "description": "Web server",
  "cpus": 4,
  "cpuAllocation": "dedicated",
  "memory": 8589934592,
  "memoryBallooning": false,
  "disks": [
    {
      "name": "disk0",
      "size": 10737418240,
      "format": "qcow2",
      "storagePoolId": "uuid",
      "bus": "virtio"
    }
  ],
  "networkInterfaces": [
    {
      "name": "eth0",
      "networkId": "uuid",
      "model": "virtio"
    }
  ],
  "os": "ubuntu-22.04",
  "clusterId": "uuid",
  "hostId": "uuid"
}
```

**VM Statuses:** `running`, `stopped`, `paused`, `suspended`, `migrating`, `creating`

**Migrate VM Request:**
```json
{
  "targetHostId": "uuid",
  "live": true,
  "timeout": 300
}
```

---

## Storage Pools

| Method | Path | Description |
|--------|------|-------------|
| GET | /storage-pools | List storage pools |
| POST | /storage-pools | Create storage pool |
| GET | /storage-pools/{id} | Get storage pool |
| DELETE | /storage-pools/{id} | Delete storage pool |
| GET | /storage-pools/{id}/disks | List disks in pool |

**Create Storage Pool Request:**
```json
{
  "name": "pool-01",
  "type": "directory",
  "path": "/var/lib/hivestack/storage/pool-01"
}
```

Pool types: `directory`, `lvm`, `lvm-thin`, `zfs`, `nfs`, `iscsi`, `ceph-rbd`

---

## Disks

| Method | Path | Description |
|--------|------|-------------|
| GET | /disks/{diskId} | Get disk |
| PUT | /disks/{diskId} | Resize disk |

**Resize Disk Request:**
```json
{
  "size": 21474836480
}
```

---

## Networks

| Method | Path | Description |
|--------|------|-------------|
| GET | /networks | List networks |
| POST | /networks | Create network |
| GET | /networks/{id} | Get network |
| DELETE | /networks/{id} | Delete network |

**Create Network Request:**
```json
{
  "name": "prod-net",
  "type": "bridge",
  "bridgeName": "br0",
  "vlanId": 100,
  "subnet": "192.168.1.0/24",
  "gateway": "192.168.1.1",
  "dhcp": false,
  "dns": ["8.8.8.8", "8.8.4.4"]
}
```

Network types: `bridge`, `nat`, `macvlan`

---

## Backups

| Method | Path | Description |
|--------|------|-------------|
| GET | /backups | List backups (filter by `vmId`, `status`) |
| POST | /backups | Create backup |
| GET | /backups/{id} | Get backup |
| POST | /backups/{id}/restore | Restore from backup |
| POST | /backups/{id}/cancel | Cancel backup |

**Create Backup Request:**
```json
{
  "vmId": "uuid",
  "name": "daily-backup",
  "type": "full",
  "timeout": 3600
}
```

Backup types: `full`, `incremental`

---

## Migration

| Method | Path | Description |
|--------|------|-------------|
| POST | /migration/import/vcenter | Import from vCenter |
| POST | /migration/import/ovf | Import OVF/OVA |
| POST | /migration/import/vmx | Import VMX |
| GET | /migration/jobs/{jobId} | Get migration job status |
| POST | /migration/discovery | Discover VMware environment |

**vCenter Import Request:**
```json
{
  "vcenterUrl": "https://vcenter.example.com/sdk",
  "username": "administrator@vsphere.local",
  "password": "secret",
  "sslVerify": true,
  "mapping": {
    "networkMappings": { "VM Network": "prod-net" },
    "storageMappings": { "datastore1": "pool-01" }
  }
}
```

---

## Events

### GET /events

List events (audit log).

**Query Parameters:**
| Param | Type | Description |
|-------|------|-------------|
| since | datetime | Start time |
| until | datetime | End time |
| type | string | Event type filter |
| severity | string | `info`, `warning`, `error`, `critical` |
| limit | integer | Default 100 |
| offset | integer | Default 0 |

**Event Types:** `vm_created`, `vm_started`, `vm_stopped`, `vm_migrated`, `snapshot_created`, `host_registered`, `host_offline`, `backup_completed`, `backup_failed`, `user_login`, `user_logout`, `config_changed`

---

## Error Responses

All endpoints return consistent error format:

```json
{
  "error": "Description of the error",
  "code": "ERROR_CODE",
  "details": []
}
```

| Status | Meaning |
|--------|---------|
| 400 | Bad Request |
| 401 | Unauthorized (missing/invalid token) |
| 403 | Forbidden (insufficient permissions) |
| 404 | Not Found |
| 429 | Rate Limited |
| 500 | Internal Server Error |
