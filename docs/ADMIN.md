# HiveStack Administrator Guide

**Audience:** platform/infrastructure administrators installing, configuring, and
operating HiveStack.
**See also:** [deployment.md](deployment.md) for a leaner install walkthrough,
[RUNBOOKS.md](RUNBOOKS.md) for step-by-step operational procedures,
[HA.md](HA.md) for the High Availability subsystem, and
[SECURITY.md](SECURITY.md) for the security posture and open items.

## Table of Contents

- [System Requirements](#system-requirements)
- [Installation](#installation)
- [Configuration](#configuration)
- [Database Setup](#database-setup)
- [TLS Certificate Management](#tls-certificate-management)
- [User and RBAC Management](#user-and-rbac-management)
- [Backup and Restore](#backup-and-restore)
- [Monitoring and Alerting](#monitoring-and-alerting)
- [Security Hardening](#security-hardening)
- [Capacity Planning](#capacity-planning)

---

## System Requirements

### Manager node

| Resource | Minimum | Recommended |
|----------|---------|--------------|
| OS | SLES 15 SP7 (SAP HANA certified) or Ubuntu 24.04 (demo/eval) | SLES 15 SP7 |
| CPU | 2 vCPU | 4+ vCPU |
| RAM | 2 GB | 8 GB+ (scales with tenant/VM count) |
| Disk | 20 GB | 100 GB (DB + logs + TLS material) |
| Go toolchain (build from source only) | Go 1.26 | Go 1.26 |
| Database | PostgreSQL 14+ | PostgreSQL 15+ |
| Network | 1 GbE | 10 GbE if managing 20+ nodes |

### Compute (node agent) hosts

| Resource | Minimum | Recommended |
|----------|---------|--------------|
| OS | SLES 15 SP7 | SLES 15 SP7 |
| CPU | VT-x/AMD-v capable | Same, plus enough headroom for planned VM density |
| RAM | 4 GB + sum of guest RAM | 4 GB + sum of guest RAM, with hugepages reserved for HANA guests |
| Virtualization stack | KVM/QEMU/libvirt (`libvirtd` running) | Same |
| Storage backend | Local dir pool | NFS, Ceph RBD, or iSCSI for HA/shared storage (see [HA.md § Shared Storage Integration](HA.md#shared-storage-integration)) |

### Network ports

| Port | Component | Purpose |
|------|-----------|---------|
| 8080 | Manager | REST API (HTTP, dev/plaintext) |
| 8443 | Manager | REST API (HTTPS) + web UI |
| 9090 (source install) / 44567 (appliance default) | Manager ↔ Node | gRPC control channel (mTLS) — confirm the active value in your `manager.yaml`/`node.yaml`, the two install paths ship different defaults |
| 9091 | Manager | Prometheus metrics |
| 9092 | Node Agent | Prometheus metrics |
| 5432 | PostgreSQL | Database |

Open the gRPC port between the Manager and every node; it carries mTLS-authenticated
VM lifecycle, registration, and heartbeat traffic (see [HA.md](HA.md)).

---

## Installation

HiveStack ships three install paths. Pick one per environment.

### 1. Bootable ISO (bare metal / lab)

Built with `live-build` under `appliance/live-build/`. Produces a combined
BIOS+UEFI-bootable ISO with Manager, Node Agent, and PostgreSQL preinstalled.

```bash
cd appliance/live-build
sudo ./build-iso.sh
# Output: output/hivestack-appliance-<version>-<date>-amd64.iso
```

Boot the ISO on target hardware (or in QEMU/VirtualBox for testing), then
follow first-boot setup — it initializes PostgreSQL, generates TLS material,
writes `manager.yaml`/`node.yaml`, runs migrations, and starts services. See
[appliance/BUILD.md](../appliance/BUILD.md) for prerequisites and other
image formats (Packer QCOW2/OVA/VMDK, Helm chart). If the ISO fails to boot,
see [TROUBLESHOOTING.md § Installation Issues](TROUBLESHOOTING.md#installation-issues).

### 2. Docker / container (evaluation, quick start)

```bash
cd appliance/docker
docker compose -f docker-compose.appliance.yml up -d
# or, from appliance/: docker compose up -d   (runs postgres + manager + node)
```

Set `POSTGRES_PASSWORD` and `JWT_SECRET` via environment variables or an
`.env` file before starting — the compose defaults are placeholders and must
not be used in production. Access the API at `https://localhost:8443`.

### 3. Build and install from source (production, systemd)

```bash
git clone https://github.com/maddydevel/HiveStack.git
cd HiveStack
go build -o bin/hive ./cmd/hive
go build -o bin/hive-manager ./cmd/hive-manager
go build -o bin/hive-node ./node/
```

Then either:

- **One-command bootstrap** (recommended — generates TLS, config, schema, and
  admin user in one step):

  ```bash
  sudo hive-manager init \
    --config /etc/hivestack/manager.yaml \
    --tls-dir /etc/hivestack/tls \
    --admin-email admin@example.com \
    --admin-name "Administrator" \
    --tenant-name "Default"
  # If --admin-password is omitted, a random password is generated and
  # printed once — capture it immediately, it is not stored or shown again.
  ```

- **Manual install** — see [deployment.md](deployment.md) for the full
  binary placement, config, and systemd unit walkthrough.

Install the systemd units and start the services:

```bash
sudo cp pkg/systemd/hivestack-manager.service /etc/systemd/system/
sudo cp pkg/systemd/hivestack-node.service /etc/systemd/system/     # on each compute node
sudo systemctl daemon-reload
sudo systemctl enable --now hivestack-manager
sudo systemctl enable --now hivestack-node   # on each compute node
```

`hive-manager` also exposes `migrate` (apply pending SQL migrations),
`validate` (lint a config file before deploying it), and `version`. Run
`hive-manager <command> --help` for flags.

---

## Configuration

### Manager (`/etc/hivestack/manager.yaml`)

```yaml
host: "0.0.0.0"
port: 8080
tls_enabled: true
tls_cert_file: "/etc/hivestack/tls/server.crt"
tls_key_file: "/etc/hivestack/tls/server.key"
tls_ca_file: "/etc/hivestack/tls/ca.crt"
tls_dev_mode: false        # true only for local dev — skips cert verification

database:
  dsn: "postgres://hivestack:***@localhost:5432/hivestack?sslmode=require"

auth:
  jwt_secret: "change-me-to-a-random-secret"   # override via HIVESTACK_JWT_SECRET
  token_expiry: "24h"
```

`hive-manager validate --config /etc/hivestack/manager.yaml` checks the file
for structural and semantic errors before you restart the service.

### Node Agent (`/etc/hivestack/node.yaml`)

```yaml
node_id: "node-01"
manager_address: "hive-manager.example.com:9090"
grpc_address: "0.0.0.0:9090"
libvirt_uri: "qemu:///system"
heartbeat_interval: "30s"

tls:
  cert_file: "/etc/hivestack/tls/node.crt"
  key_file: "/etc/hivestack/tls/node.key"
  ca_file: "/etc/hivestack/tls/ca.crt"
```

### Environment variable overrides

| Variable | Overrides |
|----------|-----------|
| `HIVESTACK_DSN` | `database.dsn` |
| `HIVESTACK_JWT_SECRET` | `auth.jwt_secret` |
| `HIVE_STACK_LOG_LEVEL` | logging level (`--log-level` flag equivalent) |

Never commit real secrets to `manager.yaml`/`node.yaml` in version control —
use environment variables, [SOPS](../internal/secrets/sops.go) (age-encrypted
YAML/JSON, used when Vault is unavailable), or
[HashiCorp Vault](../internal/secrets/vault.go) (dynamic DB credentials,
KV v2, automatic token renewal) for anything sensitive.

---

## Database Setup

1. Install and start PostgreSQL 15+:

   ```bash
   sudo systemctl enable --now postgresql
   ```

2. Create the database and role:

   ```sql
   CREATE USER hivestack WITH PASSWORD 'your-secure-password';
   CREATE DATABASE hivestack OWNER hivestack;
   GRANT ALL PRIVILEGES ON DATABASE hivestack TO hivestack;
   ```

3. Apply migrations. `hive-manager init` does this automatically; to apply
   them independently (fresh install or after an upgrade):

   ```bash
   hive-manager migrate --dsn "$HIVESTACK_DSN"
   ```

   Migrations live in `internal/db/migrations/` (`0001` through `0008` as of
   this writing) and are tracked in a `schema_migrations` table, so each
   migration applies exactly once. They cover tenants/auth, inventory, VMs,
   storage/network, backups/snapshots, compliance/events, soft-delete, and
   hash-chain verification, in that order.

4. Confirm connectivity:

   ```bash
   psql -U hivestack -d hivestack -c "SELECT count(*) FROM hosts;"
   ```

HiveStack is multi-tenant at the schema level — every tenant-scoped table
carries a `tenant_id` foreign key, so a single PostgreSQL instance can safely
serve multiple tenants.

---

## TLS Certificate Management

All Manager↔Node gRPC traffic and (in production) the REST API run over TLS,
using a private CA generated by HiveStack.

### Generate certificates

`hive-manager init` generates a CA and a Manager server certificate
automatically. To (re)generate manually:

```bash
./scripts/gen-certs.sh
```

or by hand with openssl — see [deployment.md § TLS Certificates](deployment.md#tls-certificates)
for the full CA → Manager cert → Node cert sequence. Certificate generation
logic also lives in `internal/tls/` (`ca.go`, `manager.go`, `node.go`,
`grpc.go`) if you need programmatic issuance.

### Placement

| File | Manager | Node |
|------|---------|------|
| CA cert | `/etc/hivestack/tls/ca.crt` | `/etc/hivestack/tls/ca.crt` |
| Server/client cert | `/etc/hivestack/tls/server.crt` | `/etc/hivestack/tls/node.crt` |
| Key | `/etc/hivestack/tls/server.key` | `/etc/hivestack/tls/node.key` |

Keys must be `chmod 600`, owned by the service user.

### Rotation

Certificates carry a validity window (1 year by default in the sample
scripts). Rotate before expiry:

1. Generate new certs (with the existing CA, unless the CA itself is being
   rotated).
2. Distribute to Manager and all nodes.
3. Restart `hivestack-manager` then `hivestack-node` on each host (Manager
   first so nodes can immediately reconnect with the new material).
4. Verify: `openssl s_client -connect <manager>:<grpc-port> -CAfile ca.crt`.

See [RUNBOOKS.md § Rotate TLS Certificates](RUNBOOKS.md#rotate-tls-certificates)
for the full runbook, including a zero-downtime rolling sequence for
multi-node clusters. Certificate expiry is also a listed automatic-rollback
trigger during releases — see [RELEASE.md](RELEASE.md).

---

## User and RBAC Management

Authentication is JWT-based (`internal/auth`); passwords are hashed with
Argon2. Authorization is RBAC, enforced per-request against a
`(resource, action)` permission model (e.g. `vm.create`, `host.maintenance`).

### Built-in roles

| Role | Description |
|------|--------------|
| `admin` | Full access: VMs, hosts, storage, networks, backups, tenants, compliance, audit, user management |
| `operator` | Manage VMs, hosts, storage, networks, backups, and view/validate compliance — no tenant or user administration |
| `hana-operator` | `operator`-equivalent for VM lifecycle plus full compliance actions (view/validate/evidence/drift), but read-only on hosts/storage/network |
| `viewer` | Read-only across VMs, hosts, storage, networks, backups, compliance, events |
| `compliance-auditor` | Read-only, scoped to compliance evidence/drift and audit logs plus inventory visibility |

Built-in roles are defined in code (`internal/auth/rbac.go`) and require no
setup. Tenants can layer additional custom roles on top via the `roles`
table (`internal/db/migrations/0001_tenants_and_auth.sql`).

### Managing users

Via the REST API (see [api.md § Users](api.md#users)):

```bash
# Create a user (admin token required)
curl -k -X POST https://localhost:8443/api/v1/users \
  -H "Authorization: Bearer $TOKEN" -H "Content-Type: application/json" \
  -d '{"name":"Jane Operator","email":"jane@example.com","password":"...","role":"operator"}'

# List / get / update / delete
curl -k https://localhost:8443/api/v1/users -H "Authorization: Bearer $TOKEN"
curl -k -X PUT https://localhost:8443/api/v1/users/<id> -H "Authorization: Bearer $TOKEN" -d '{"role":"viewer"}'
curl -k -X DELETE https://localhost:8443/api/v1/users/<id> -H "Authorization: Bearer $TOKEN"
```

Or via the CLI once authenticated (`hive auth login --email ... --password ...`);
see [USER.md § CLI Reference](USER.md#cli-reference).

### Practices

- Create per-person accounts; avoid shared credentials — every state-changing
  API call is attributed to the caller in the audit trail.
- Grant the least-privileged built-in role that covers the job; use
  `viewer`/`compliance-auditor` for anyone who doesn't need to mutate state.
- Rotate `auth.jwt_secret` (and thus invalidate all outstanding tokens) if a
  secret leak is suspected; set it via `HIVESTACK_JWT_SECRET`, not the
  checked-in config file.
- Review `GET /api/v1/audit` (or `hive` audit tooling) periodically —
  see [operations/audit-retention.md](operations/audit-retention.md) for
  retention periods and the purge procedure.

---

## Backup and Restore

Back up four things: the database, configuration, TLS material, and VM
storage. See [RUNBOOKS.md § Backup and Restore](RUNBOOKS.md#backup-and-restore)
for the copy-pasteable commands; summarized:

```bash
# Database (logical backup)
pg_dump -U hivestack -Fc hivestack > /backup/hivestack-db-$(date +%Y%m%d).dump

# Configuration and TLS material
sudo tar czf /backup/hivestack-config-$(date +%Y%m%d).tar.gz /etc/hivestack/

# VM storage (per node — size/schedule depends on your storage backend)
sudo tar czf /backup/hivestack-vms-$(date +%Y%m%d).tar.gz /var/lib/hivestack/storage/
```

Restore in the reverse order — TLS/config, then database (`pg_restore
--clean`), then restart services — and validate with
`./scripts/health-check.sh --verbose` afterward.

For VM-level backups (as opposed to platform backups), use the `backup`
subsystem exposed via `hive backup create/list/restore/cancel` and
`POST /api/v1/backups` — see [USER.md § Backup and Restore Operations](USER.md#backup-and-restore-operations).
Follow the 3-2-1 rule: keep backups on storage independent of the primary
pool, and test restores regularly, not just on the day you need one.

---

## Monitoring and Alerting

### Metrics

Prometheus metrics are exposed by both components:

- Manager: `http://<manager>:9091/metrics`
- Node Agent: `http://<node>:9092/metrics`

### Key thresholds

| Metric | Warning | Critical |
|--------|---------|----------|
| API latency (p99) | > 200–500ms | > 2000ms |
| Manager/Node CPU | > 70–80% | > 90–95% |
| Database connections | > 80% of pool | > 95% of pool |
| Node heartbeat age | > 60s | > 180s |
| VM operation failure rate | > 5% | > 15% |
| Disk usage | > 75% | > 90% |

These map to the platform's published SLOs — see
[operations/slo-capacity.md](operations/slo-capacity.md) (API p99 < 500ms,
99.5% availability, RTO < 5 min, RPO < 15 min) and the automatic rollback
thresholds in [RELEASE.md](RELEASE.md).

### Logs

| Service | Location |
|---------|----------|
| Manager | `/var/log/hivestack/manager.log`, `journalctl -u hivestack-manager` |
| Node Agent | `/var/log/hivestack/node.log`, `journalctl -u hivestack-node` |
| PostgreSQL | `/var/log/postgresql/` |

Set `logging.level: debug` (or `--log-level debug`) temporarily for deeper
diagnostics; revert afterward, debug logging is verbose and can include
request-level detail.

### Health checks

```bash
curl -k https://localhost:8443/health
./scripts/health-check.sh --verbose
```

Wire alerting (Alertmanager, PagerDuty, etc.) off the Prometheus endpoints
and off `/health`; see [HA.md § Monitoring & Metrics](HA.md#monitoring--metrics)
for HA-specific failover events to alert on.

---

## Security Hardening

- **AppArmor** (`internal/security/apparmor.go`): generates per-service
  profiles (`hivestack-manager`, `hivestack-node`) restricting file access,
  network access, and dangerous capabilities. Recommended MAC system on
  SLES 15 SP7 — load the generated profiles with `apparmor_parser`.
- **LUKS** (`internal/security/luks.go`): encrypt VM storage volumes, backup
  storage, and swap at rest using LUKS via `cryptsetup`.
- **systemd hardening**: the shipped units already set `ProtectSystem=strict`,
  `ProtectHome=yes`, `NoNewPrivileges=yes`, `PrivateTmp=yes`, and a minimal
  `CapabilityBoundingSet` (`CAP_NET_ADMIN` for the Manager;
  `CAP_SYS_ADMIN CAP_NET_ADMIN` for the Node, required by KVM) — see
  [deployment.md § Systemd Services](deployment.md#systemd-services).
- **mTLS everywhere**: never disable `tls_enabled`/`tls_dev_mode: true`
  outside local development; the Manager↔Node channel and, where
  configured, the REST API both expect certificate-backed identities.
- **Secrets**: use Vault or SOPS (`internal/secrets/`) instead of plaintext
  values in config files; never commit `jwt_secret`, database passwords, or
  private keys.
- **API middleware** (`internal/security/middleware.go`): rate limiting,
  CORS, and security headers are available — enable and tune them for
  internet-facing deployments.
- **Fencing credentials**: IPMI/Redfish/SSH fencer credentials should not be
  passed as bare CLI args or with `-k`/`StrictHostKeyChecking=no` in
  production — see [SECURITY.md § Fencing Security](SECURITY.md#fencing-security)
  for the current gaps and required hardening.
- Run `govulncheck ./...` and `gosec ./...` regularly, and pin dependency
  versions — see [RELEASE.md § Pre-Release Checklist](RELEASE.md#release-checklist).
- Review [THREAT_MODEL.md](THREAT_MODEL.md) for the full threat model and
  scope, and [SECURITY.md](SECURITY.md) for outstanding production
  requirements before exposing HiveStack beyond a trusted network.

---

## Capacity Planning

Design limits and scaling triggers are tracked in
[operations/slo-capacity.md](operations/slo-capacity.md); headline numbers:

| Resource | Design limit |
|----------|---------------|
| VMs | 5,000 across all clusters |
| Clusters | 20 per Manager instance |
| Nodes | 50 per cluster |
| vCPUs per VM | 256 (KVM limit) |
| Memory per VM | 4 TB (KVM limit) |
| Concurrent users | 200 (tested) |
| API requests/sec | 1,000 |

**Scaling triggers:** add a node or migrate VMs when host CPU/memory is
sustained above 70%/80% for 5 minutes; expand a storage pool or archive VMs
at 85% storage usage; scale Manager API replicas when p99 latency exceeds
400ms sustained; grow the PostgreSQL connection pool when utilization
exceeds 80%.

For HANA workloads specifically, size hosts to dedicate whole NUMA nodes and
hugepage-backed memory per guest — see
[USER.md § Compliance Validation](USER.md#compliance-validation-hana-guardrails)
and `internal/compliance/`. HANA guardrails (NUMA alignment, hugepages,
dedicated vCPUs, no ballooning/swap) reduce effective consolidation ratio
compared to general-purpose VMs — plan headroom accordingly, not just raw
CPU/RAM totals.
