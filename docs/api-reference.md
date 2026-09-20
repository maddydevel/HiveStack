# HiveStack API Reference

Summary of the REST API. The machine-readable contract is
[`api/openapi.yaml`](../api/openapi.yaml) (OpenAPI 3.0). Request and response
examples are in [api.md](api.md).

- **Base URL:** `https://<manager>:8443/api/v1` (TLS) or
  `http://<manager>:8080/api/v1` (development)
- **Format:** JSON request and response bodies
- **Version:** 0.1.0

## Authentication

Every endpoint except `/health`, `/auth/login`, and `/metrics` requires a JWT:

```
Authorization: Bearer <token>
```

Tokens are HS256-signed with the `HIVESTACK_JWT_SECRET` environment variable and
carry the user ID, tenant ID, and scopes. Handlers scope data by the tenant in
the token. A missing or invalid token returns `401`.

## Errors

Errors return a JSON body with a human-readable `error` and, where set, a
machine-readable `code`:

```json
{ "error": "unauthorized", "code": "UNAUTHORIZED" }
```

| Status | Meaning |
|--------|---------|
| 400 | Malformed body or failed validation |
| 401 | Missing, invalid, or expired token |
| 403 | Authenticated but not permitted (RBAC or another tenant's resource) |
| 404 | Resource not found |
| 409 | Conflict, such as a duplicate name |
| 500 | Internal error |

The codes map to the constants in [`internal/errors`](../internal/errors/errors.go).

## Endpoints

Paths below are relative to the base URL. `{id}` values are resource IDs.

### Auth and users

| Method | Path | Description |
|--------|------|-------------|
| POST | `/auth/login` | Log in, receive a JWT (no auth required) |
| POST | `/auth/logout` | Log out |
| GET | `/auth/me` | Current user, tenant, and scopes |
| GET, POST | `/users` | List (tenant-scoped) or create users |
| GET, PUT, DELETE | `/users/{id}` | Read, update, or delete a user |

### Inventory

| Method | Path | Description |
|--------|------|-------------|
| GET, POST | `/datacenters` | List or create datacenters |
| GET, DELETE | `/datacenters/{id}` | Read or delete a datacenter |
| GET, POST | `/clusters` | List or create clusters |
| GET | `/clusters/{id}` | Read a cluster |
| GET | `/clusters/{id}/hosts` | Hosts in a cluster |
| GET, POST | `/hosts` | List or register hosts |
| GET, PUT, DELETE | `/hosts/{id}` | Read, update, or delete a host |
| GET | `/hosts/{id}/status` | Host status and resources |
| POST, DELETE | `/hosts/{id}/maintenance` | Enter or exit maintenance mode |

### Virtual machines

| Method | Path | Description |
|--------|------|-------------|
| GET, POST | `/vms` | List or create VMs |
| GET, PUT, DELETE | `/vms/{id}` | Read, update, or delete a VM |
| POST | `/vms/{id}/start`, `/stop`, `/restart` | Power operations |
| POST | `/vms/{id}/migrate` | Live-migrate to another host |
| GET, POST | `/vms/{id}/snapshots` | List or create snapshots |
| DELETE | `/vms/{id}/snapshots/{sid}` | Delete a snapshot |
| GET | `/vms/{id}/console` | Console access |
| GET | `/vms/{id}/stats` | Resource statistics |

### Storage, networks, backups

| Method | Path | Description |
|--------|------|-------------|
| GET, POST | `/storage-pools` | List or create storage pools |
| GET, DELETE | `/storage-pools/{id}` | Read or delete a pool |
| GET | `/storage-pools/{id}/disks` | Disks in a pool |
| GET | `/disks/{diskId}` | Read a disk |
| PUT | `/disks/{diskId}/resize` | Resize a disk |
| GET, POST | `/networks` | List or create networks |
| GET, DELETE | `/networks/{id}` | Read or delete a network |
| GET, POST | `/backups` | List or create backups |
| GET | `/backups/{id}` | Read a backup |
| POST | `/backups/{id}/restore`, `/cancel` | Restore or cancel |

### High availability

| Method | Path | Description |
|--------|------|-------------|
| GET | `/ha/status` | HA controller status |
| GET | `/ha/nodes` | Node health as seen by HA |
| GET | `/ha/history` | Failover history |
| POST | `/ha/failover` | Trigger a manual failover (RBAC-checked) |
| GET, PUT | `/vms/{id}/ha-policy` | Read or set a VM's HA policy |

### Compliance and events

| Method | Path | Description |
|--------|------|-------------|
| GET | `/compliance/vms/{id}` | Run the HANA guardrail check for a VM |
| GET | `/compliance/evidence/{id}` | Hash-chained evidence for a VM |
| GET | `/compliance/drift` | Drift report for the tenant |
| GET | `/events` | Recent tenant events |

### Migration

| Method | Path | Description |
|--------|------|-------------|
| POST | `/migration/import/vcenter`, `/ovf`, `/vmx` | Start an import |
| POST | `/migration/import/vmx/precheck` | Pre-check a VMX import |
| POST | `/migration/discovery` | Discover source VMs |
| GET | `/migration/jobs` | List import jobs |
| GET | `/migration/jobs/{jobId}` | Read an import job |

### Operational

| Method | Path | Description |
|--------|------|-------------|
| GET | `/health` | Liveness; the server registers it at `/health`, and the spec lists it under the base URL |
| GET | `/metrics` | Prometheus metrics (unauthenticated) |

## Keeping this page in sync

Routes are registered in `internal/api/server.go`, `ha_handlers.go`, and
`migration_handlers.go`. The OpenAPI file is the source of truth for clients,
but it does not yet describe the HA, compliance, or `/migration/jobs` list
endpoints, and it names a few paths differently from the server (for example
`/vms/{id}/snapshots/{snapshotId}`). When you change a route, update
`api/openapi.yaml` and run `make validate-api`.
