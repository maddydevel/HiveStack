# HiveStack Architecture

## Overview

HiveStack is a KVM-based virtualization management platform built on SUSE SLES 15 SP7, designed as a migration target for VMware vCenter/ESXi environments. It provides vCenter-like centralized management through a web UI and REST API, ESXi-like hypervisor nodes running KVM/QEMU, and Proxmox-level flexibility.

## System Components

### HiveStack Manager (Control Plane)
- **Web UI**: React 18 + TypeScript + Vite, dark-themed, professional look, responsive
- **REST API**: Go + chi router, JSON over HTTPS (TLS), JWT authentication, OpenAPI 3.0 documented
- **CLI (`hive`)**: Go + cobra, table/JSON/YAML output, API token auth, config file support
- **Core Services**: Auth, RBAC, inventory, VM lifecycle, storage, network, backup, migration (all in Go)
- **Database**: PostgreSQL, embedded initially (single-node Manager), external for HA later

### HiveStack Node (Compute Plane)
- **Base**: SUSE SLES 15 SP7 minimal install + KVM/QEMU/libvirt
- **Node Agent**: Go binary, systemd service, TLS-secured gRPC to Manager
- **Virtualization**: KVM hardware virtualization, QEMU for device emulation, libvirt for domain management
- **VM Types**: Full VMs (BIOS/UEFI, vCPUs, memory, disks, NICs), LXC containers (optional)
- **Storage**: Local (dir, LVM, ZFS), Shared (NFS, iSCSI, Ceph RBD)
- **Network**: Linux bridge, VLAN (802.1Q), NAT, OVS (optional), vmxnet3 emulation

### Component Interaction Diagram

```
User (UI/CLI) ──HTTPS──> Manager API ──gRPC/TLS──> Node Agent ──libvirt──> KVM/QEMU
                                               │
                                          PostgreSQL
```

### Manager-to-Node Communication Flow

```
1. Manager receives VM create request (UI/CLI/API)
2. Manager validates request (RBAC, resource availability)
3. Manager translates VM spec to libvirt XML
4. Manager sends command to Node Agent via gRPC
5. Node Agent executes on KVM (define VM, create disks, attach network)
6. Node Agent reports status back to Manager
7. Manager persists VM record in database
8. Manager returns response to user
```

### Live Migration Flow

```
Source Node ──QEMU migrate──> Manager ──validate+directed──> Dest Node
     │                            │                            │
     └──progress──>               │◄──progress───────────────┘
     │                            │
     └──completion──>            │
```

## Technology Choices

| Component | Technology | Rationale |
|-----------|------------|-----------|
| Manager backend | Go 1.23+ | Single binary, performant, great ecosystem |
| API framework | chi (go-chi/chi) | Lightweight, middleware support, idiomatic Go |
| CLI | cobra (spf13/cobra) | Standard Go CLI framework |
| Database | PostgreSQL | Mature, embedded option available |
| DB deployment | Embedded initially, external for HA | Simplicity first |
| Web UI | React 18 + TypeScript + Vite | Component-based, large ecosystem, vCenter-like UX |
| UI state | React Context + useReducer | Simple, no heavy deps initially |
| UI styling | Tailwind CSS | Rapid dev, consistent design |
| Node agent | Go | Same language as Manager, single binary |
| Node-to-Manager | gRPC (protobuf) | Typed contracts, efficient |
| Virtualization | KVM + QEMU + libvirt | Standard Linux virtualization |
| HA | Corosync + Pacemaker (production), custom heartbeat (initial) | Industry-standard |
| Monitoring | Prometheus + Grafana (bundled) | Standard observability |
| Logging | slog (Go stdlib) + file rotation | Structured logging |
| Build | Make + Go build | Simple, cross-compilation |
| Appliance | SLES 15 SP7 + OVA/ISO | SUSE LTS, enterprise support |

## Deployment Architecture

### Single-Node (Edge/Small)
```
┌─────────────────────────────────────┐
│  HiveStack Manager + Node (combined)│
│  SLES 15 SP7                        │
│  Manager (UI/API/CLI/DB) + Node     │
│  (Agent/KVM/VMs/Local Storage)      │
└─────────────────────────────────────┘
```
- Embedded PostgreSQL
- Local storage only
- Edge/lab/small deployments (<10 VMs)

### Multi-Node (Standard)
```
┌────────────────────────┐
│  Manager (dedicated)   │
│  UI/API/CLI/DB         │
└───────────┬────────────┘
            │ TLS-gRPC
     ┌──────┴──────┐
     ▼             ▼
┌──────────┐ ┌──────────┐
│  Node 1  │ │  Node 2  │
│  KVM+VMs  │ │  KVM+VMs  │
│  Local    │ │  Local    │
└──────────┘ └──────────┘
     └──────┬──────┘
          Shared Storage (NFS/iSCSI/Ceph)
```
- Dedicated Manager host
- Multiple Node hosts
- Shared storage for migration
- Most production deployments

### Large-Scale (Enterprise)
```
┌─────────────┐ ┌─────────────┐
│ Manager HA  │ │ Manager HA  │
│ (Active)    │ │ (Passive)   │
└──────┬──────┘ └──────┬──────┘
       │ shared DB       │
       ▼                 ▼
┌─────────────────────────────────────────┐
│         Cluster (N nodes)              │
│  ┌────────┐ ┌────────┐ ┌────────┐     │
│  │ Node 1 │ │ Node 2 │ │ Node N │     │
│  └────────┘ └────────┘ └────────┘     │
│       └──────────┬────────────────────┘
│            Shared Storage (Ceph preferred)
└─────────────────────────────────────────┘
```
- Active-passive Manager HA
- External PostgreSQL cluster
- Ceph storage
- Large production (100+ VMs)

## API Architecture

### Base URL
- Production: `https://<manager-host>:8443/api/v1`
- Development: `http://localhost:8080/api/v1`

### Authentication
- JWT tokens (initial)
- API tokens (scoped, expiring)
- SSO: LDAP/AD, OIDC, SAML 2.0 (future)

### Versioning
- URL path prefix: `/api/v1/`
- Backward-compatible additions within v1
- Breaking changes → new version (`/api/v2/`)

### Error Format
```json
{
  "error": "Human-readable message",
  "code": "MACHINE_READABLE_CODE",
  "details": []
}
```

### Key Endpoint Groups
- **Auth**: `/auth/login`, `/auth/logout`, `/auth/me`
- **Users**: `/users`, `/users/{id}` (CRUD + RBAC)
- **Datacenters**: `/datacenters`, `/datacenters/{id}`
- **Clusters**: `/clusters`, `/clusters/{id}`, `/clusters/{id}/hosts`
- **Hosts**: `/hosts`, `/hosts/{id}`, `/hosts/{id}/status`, `/hosts/{id}/maintenance`
- **VMs**: `/vms`, `/vms/{id}`, `/vms/{id}/start`, `/vms/{id}/stop`, `/vms/{id}/restart`, `/vms/{id}/migrate`, `/vms/{id}/snapshots`, `/vms/{id}/console`, `/vms/{id}/stats`
- **Storage**: `/storage-pools`, `/storage-pools/{id}`, `/storage-pools/{id}/disks`, `/disks/{diskId}`
- **Networks**: `/networks`, `/networks/{id}`
- **Backups**: `/backups`, `/backups/{id}`, `/backups/{id}/restore`, `/backups/{id}/cancel`
- **Migration**: `/migration/import/vcenter`, `/migration/import/ovf`, `/migration/import/vmx`, `/migration/jobs/{jobId}`, `/migration/discovery`
- **Events**: `/events`
- **Health**: `/health`

## Data Model (Initial Schema)

### Users & Auth
- `users`: id, name, email, password_hash, role, created_at, updated_at
- `sessions`: id, user_id, token, expires_at, created_at
- `api_tokens`: id, user_id, name, token_hash, scopes, expires_at, created_at

### RBAC
- `roles`: id, name, description, created_at
- `permissions`: id, name, resource, action
- `role_permissions`: role_id, permission_id

### Inventory
- `datacenters`: id, name, description, status, created_at, updated_at
- `clusters`: id, name, datacenter_id, status, host_count, vm_count, total_cpu, total_memory, created_at
- `hosts`: id, name, hostname, ip_address, cluster_id, status, cpu_model, cpu_count, cpu_usage, memory_total, memory_used, storage_total, storage_used, os, hypervisor, joined_at, last_heartbeat, maintenance_mode, created_at

### VMs
- `vms`: id, identifier, name, description, host_id, cluster_id, status, cpus, cpu_allocation, memory, memory_ballooning, os, template_id, snapshot_count, created_at, started_at, updated_at

### Storage
- `storage_pools`: id, name, type, datacenter_id, status, total_capacity, free_space, used_space, features, created_at
- `disks`: id, name, size, format, storage_pool_id, path, vm_id, mounted, created_at

### Networks
- `networks`: id, name, description, type, datacenter_id, bridge_name, vlan_id, subnet, gateway, dhcp, dns, status, created_at

### Backups
- `backups`: id, name, vm_id, vm_name, status, type, storage_path, size, progress, message, started_at, completed_at, schedule_id, created_at
- `backup_schedules`: id, name, vm_id, frequency, retention, storage_path, enabled, created_at

### Snapshots
- `snapshots`: id, name, description, vm_id, state, created_at, size

### Events
- `events`: id, type, severity, message, actor_type, actor_id, actor_name, resource_type, resource_id, resource_name, timestamp, metadata

## Storage Architecture

### Storage Backends (Priority Order)
1. **Directory (local)**: QCOW2 files on local filesystem — simplest, first
2. **LVM (local)**: LVM volumes — better performance, thin provisioning
3. **ZFS (local)**: Native ZFS — snapshots, clones, compression, encryption
4. **NFS (shared)**: Network filesystem — shared storage for live migration
5. **iSCSI (shared)**: Block storage — performance, shared
6. **Ceph RBD (shared)**: Distributed block storage — scale, redundancy

### Storage Abstraction
```
Storage Service (Manager)
  ├── Local Backend: dir, LVM, ZFS
  └── Shared Backend: NFS, iSCSI, Ceph
```

### VM Disk Management
- Formats: QCOW2 (default), raw, VMDK (imported)
- Provisioning: Thin (QCOW2), thick (raw/LVM)
- Operations: Create, resize (online), delete, attach/detach
- Snapshots: QEMU external snapshots, scheduled snapshots
- Backups: Full/incremental, scheduled, to multiple targets

## Network Architecture

### Virtual Networking
- **Bridge**: VM connected to Linux bridge, same L2 as host — simplest
- **VLAN**: Bridge with 802.1Q tagging — segmentation
- **NAT**: VMs behind NAT, isolated — default for testing
- **OVS**: Open vSwitch — advanced features
- **Macvlan/IPVLAN**: Direct connectivity on physical network

### VMware Network Compatibility
- **vmxnet3 emulation**: QEMU virtio + vmxnet3 model for Windows VMs
- **Port group mapping**: VMware port groups → HiveStack networks during migration
- **Network import**: Parse VMX network definitions

## Security Architecture

### TLS and Certificates
- Web UI: HTTPS with TLS 1.2+ (auto-generated on first boot)
- API: HTTPS with TLS 1.2+ (same certificate)
- Node-to-Manager: mTLS with per-node certificates
- Certificate rotation support, external CA optional

### Authentication
- Local users with Argon2id password hashing (initial)
- SSO: LDAP/AD, OIDC, SAML 2.0 (future)
- API tokens: Scoped, expiring tokens for API/CLI

### RBAC
- Built-in roles: admin, operator, viewer, auditor
- Custom roles: Defined by admin
- Permission model: Resource × Action matrix

### Storage Encryption
- Full-disk: LUKS on node OS disk (optional)
- ZFS datasets: Native ZFS encryption
- VM disks: Optional per-VM encryption (QEMU disk encryption)
- Key management: Local keystore initially, KMIP later

### VM Isolation
- KVM: Hardware-assisted isolation
- Seccomp: QEMU seccomp filters
- AppArmor/SELinux: MAC profiles for QEMU
- Capabilities: Minimal Linux capabilities for QEMU
- Network isolation: VLANs, separate virtual networks

## High Availability

### Node HA (VM-level)
- Detection: Node heartbeat (gRPC ping, periodic), health checks
- Action: Manager detects failure, restarts affected VMs on healthy nodes
- Policy: Per-VM HA enabled/disabled, per-pool HA policy
- Fencing: IPMI, watchdog, network fencing

### Manager HA (Future)
- Active-passive: Primary handles requests, passive stands by
- Database: External PostgreSQL cluster
- Not initial priority: Single Manager fine for most deployments

## VMware Migration Architecture

### Migration Tool Components
1. **Discovery Tool**: Scan vCenter/ESXi, inventory VMs
2. **VMX Parser**: Parse VMX files → HiveStack VM definition
3. **VMDK Importer**: Import VMDK disks → QCOW2/raw
4. **OVF/OVA Importer**: Parse OVF/OVA packages → Create VMs
5. **vCenter Inventory Importer**: Connect to vCenter API, import inventory
6. **Migration Planner**: Compatibility check, wave planning, capacity planning
7. **Guest Tools Adaptation**: VMware Tools → qemu-guest-agent migration

### Migration Flows
- **Online import**: VM running on VMware → export VMDK → import to HiveStack → start on HiveStack
- **Offline import**: VM powered off on VMware → export VMDK → import to HiveStack → start on HiveStack
- **Side-by-side**: Run VMware and HiveStack in parallel during migration
- **Cutover**: Planned downtime window with final sync

## Open Questions / To Be Decided

1. **Web UI library choices**: React Router, UI component library (MUI, Chakra, custom?), state management (Zustand?)
2. **Database ORM**: sqlc vs GORM vs raw SQL
3. **gRPC proto definitions**: Need to define .proto files for node-agent communication
4. **Manager HA timeline**: When to implement?
5. **Ceph integration**: Build in-house or use existing Ceph tools?
6. **Backup format**: Custom vs standard (tar + disks)?
7. **Windows guest support**: Which Windows versions initially?
8. **Licensing**: GPLv3, Apache 2.0, or dual-license?
9. **Node agent protocol**: gRPC confirmed, proto files pending
10. **CI/CD platform**: GitHub Actions confirmed (already set up)
