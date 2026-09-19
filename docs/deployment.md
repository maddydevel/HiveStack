# HiveStack Deployment Guide

**Version:** 0.1.0

## Table of Contents

- [Architecture Overview](#architecture-overview)
- [Prerequisites](#prerequisites)
- [Building](#building)
- [Database Setup](#database-setup)
- [TLS Certificates](#tls-certificates)
- [Manager Deployment](#manager-deployment)
- [Node Agent Deployment](#node-agent-deployment)
- [Configuration](#configuration)
- [Systemd Services](#systemd-services)
- [Verification](#verification)
- [Troubleshooting](#troubleshooting)

---

## Architecture Overview

HiveStack has two main components:

| Component | Binary | Description |
|-----------|--------|-------------|
| **Manager** | `hive-manager` | Control plane — REST API, CLI, auth, RBAC, PostgreSQL, event framework |
| **Node Agent** | `hive-node` | Compute node — KVM/QEMU/libvirt, gRPC server, VM lifecycle execution |

The Manager communicates with Node Agents over TLS-secured gRPC (default port 9090). Users interact via the REST API (default port 8080/8443) or CLI.

```
┌─────────────┐     REST API      ┌─────────────┐     gRPC (mTLS)     ┌─────────────┐
│   CLI / UI   │ ───────────────► │   Manager    │ ──────────────────► │  Node Agent  │
│  (port 8080) │                  │  (port 8080) │                     │  (port 9090) │
└─────────────┘                  └─────────────┘                     └─────────────┘
                                         │
                                         │ SQL
                                         ▼
                                  ┌─────────────┐
                                  │  PostgreSQL  │
                                  └─────────────┘
```

---

## Prerequisites

### Manager Node
- Linux (SUSE SLES 15 SP7 recommended)
- Go 1.26+ (for building from source)
- PostgreSQL 14+
- 2+ GB RAM

### Compute Nodes
- SUSE SLES 15 SP7
- KVM/QEMU/libvirt installed and configured
- CPU with virtualization extensions (VT-x/AMD-v)
- 4+ GB RAM (depends on VM workload)

### Install Dependencies (SLES 15 SP7)

```bash
# Manager
sudo zypper install postgresql14-server go1.26

# Compute nodes
sudo zypper install kvm qemu libvirt-daemon
sudo systemctl enable --now libvirtd
```

---

## Building

```bash
git clone https://github.com/maddydevel/HiveStack.git
cd HiveStack

# Build all binaries
make build

# Or build individually:
go build -o bin/hive ./cmd/hive
go build -o bin/hive-manager ./cmd/hive-manager
go build -o bin/hive-node ./node/
```

Output:
- `bin/hive` — CLI client
- `bin/hive-manager` — Manager service
- `bin/hive-node` — Node Agent service

---

## Database Setup

1. **Start PostgreSQL:**
   ```bash
   sudo systemctl enable --now postgresql
   ```

2. **Create database and user:**
   ```sql
   CREATE USER hivestack WITH PASSWORD 'your-secure-password';
   CREATE DATABASE hivestack OWNER hivestack;
   GRANT ALL PRIVILEGES ON DATABASE hivestack TO hivestack;
   ```

3. **Run migrations:**
   ```bash
   # Migrations are in internal/db/migrations/
   # Apply in order: 0001 through 0006
   for f in internal/db/migrations/*.sql; do
     psql -U hivestack -d hivestack -f "$f"
   done
   ```

4. **Set the DSN environment variable:**
   ```bash
   export HIVESTACK_DSN="postgres://hivestack:your-secure-password@localhost:5432/hivestack?sslmode=require"
   ```

---

## TLS Certificates

HiveStack uses mTLS for gRPC communication between Manager and Node Agents.

### Generate Certificates

```bash
# Run the included certificate generation script
./scripts/gen-certs.sh

# Or manually with openssl:
# 1. Create CA
openssl genrsa -out ca.key 4096
openssl req -new -x509 -days 365 -key ca.key -out ca.crt -subj "/CN=HiveStack CA"

# 2. Create Manager certificate
openssl genrsa -out manager.key 2048
openssl req -new -key manager.key -out manager.csr -subj "/CN=hive-manager"
openssl x509 -req -days 365 -in manager.csr -CA ca.crt -CAkey ca.key -CAcreateserial -out manager.crt

# 3. Create Node certificate
openssl genrsa -out node.key 2048
openssl req -new -key node.key -out node.csr -subj "/CN=hive-node"
openssl x509 -req -days 365 -in node.csr -CA ca.crt -CAkey ca.key -CAcreateserial -out node.crt
```

### Certificate Placement

| File | Manager Path | Node Path |
|------|-------------|-----------|
| CA cert | `/etc/hivestack/tls/ca.crt` | `/etc/hivestack/tls/ca.crt` |
| Server cert | `/etc/hivestack/tls/manager.crt` | `/etc/hivestack/tls/node.crt` |
| Server key | `/etc/hivestack/tls/manager.key` | `/etc/hivestack/tls/node.key` |

---

## Manager Deployment

1. **Install binary:**
   ```bash
   sudo cp bin/hive-manager /usr/bin/
   sudo chmod 755 /usr/bin/hive-manager
   ```

2. **Create directories:**
   ```bash
   sudo mkdir -p /etc/hivestack/tls
   sudo mkdir -p /var/lib/hivestack
   sudo mkdir -p /var/log/hivestack
   sudo mkdir -p /run/hivestack
   ```

3. **Install configuration:**
   ```bash
   sudo cp configs/manager.yaml /etc/hivestack/manager.yaml
   sudo chmod 600 /etc/hivestack/manager.yaml
   ```

4. **Install TLS certificates** (see [TLS Certificates](#tls-certificates))

5. **Enable and start service:**
   ```bash
   sudo cp pkg/systemd/hivestack-manager.service /etc/systemd/system/
   sudo systemctl daemon-reload
   sudo systemctl enable --now hivestack-manager
   ```

---

## Node Agent Deployment

On each compute node:

1. **Install binary:**
   ```bash
   sudo cp bin/hive-node /usr/bin/
   sudo chmod 755 /usr/bin/hive-node
   ```

2. **Create directories:**
   ```bash
   sudo mkdir -p /etc/hivestack/tls
   sudo mkdir -p /var/lib/hivestack
   sudo mkdir -p /var/log/hivestack
   sudo mkdir -p /run/hivestack
   ```

3. **Install configuration:**
   ```bash
   sudo cp configs/node.yaml /etc/hivestack/node.yaml
   sudo chmod 600 /etc/hivestack/node.yaml
   ```

4. **Install TLS certificates** (node cert + CA cert)

5. **Enable and start service:**
   ```bash
   sudo cp pkg/systemd/hivestack-node.service /etc/systemd/system/
   sudo systemctl daemon-reload
   sudo systemctl enable --now hivestack-node
   ```

---

## Configuration

### Manager Configuration (`/etc/hivestack/manager.yaml`)

```yaml
server:
  host: "0.0.0.0"
  port: 8080
  tls:
    enabled: true
    cert: "/etc/hivestack/tls/manager.crt"
    key: "/etc/hivestack/tls/manager.key"

database:
  dsn: "postgres://hivestack:***@localhost:5432/hivestack?sslmode=require"
  max_connections: 25

auth:
  jwt_secret: "change-me-to-a-random-secret"
  token_expiry: "24h"
  argon2_memory: 65536
  argon2_iterations: 3
  argon2_parallelism: 4

grpc:
  port: 9090
  tls:
    ca_cert: "/etc/hivestack/tls/ca.crt"
    cert: "/etc/hivestack/tls/manager.crt"
    key: "/etc/hivestack/tls/manager.key"

logging:
  level: "info"
  format: "json"

metrics:
  enabled: true
  port: 9091
```

### Node Agent Configuration (`/etc/hivestack/node.yaml`)

```yaml
node_id: "node-01"
manager_address: "hive-manager.example.com:9090"
libvirt_uri: "qemu:///system"

grpc:
  port: 9090
  tls:
    ca_cert: "/etc/hivestack/tls/ca.crt"
    cert: "/etc/hivestack/tls/node.crt"
    key: "/etc/hivestack/tls/node.key"

storage:
  base_path: "/var/lib/hivestack/storage"

logging:
  level: "info"
```

### Environment Variables

| Variable | Description | Default |
|----------|-------------|---------|
| `HIVESTACK_DSN` | PostgreSQL connection string | (from config) |
| `HIVE_STACK_LOG_LEVEL` | Logging level | `info` |
| `HIVESTACK_JWT_SECRET` | JWT signing secret | (from config) |

---

## Systemd Services

### Manager Service (`hivestack-manager.service`)

```ini
[Unit]
Description=HiveStack Manager Service
After=network.target postgresql.service
Wants=network.target

[Service]
Type=simple
ExecStart=/usr/bin/hive-manager --config /etc/hivestack/manager.yaml
Restart=always
RestartSec=5
Environment=HIVE_STACK_LOG_LEVEL=info

# Security hardening
ProtectSystem=strict
ProtectHome=yes
NoNewPrivileges=yes
ReadWritePaths=/var/lib/hivestack /var/log/hivestack /run/hivestack
PrivateTmp=yes

# Capability bounding
CapabilityBoundingSet=CAP_NET_ADMIN

# Resource limits
LimitNOFILE=65536
MemoryMax=2G

[Install]
WantedBy=multi-user.target
```

### Node Agent Service (`hivestack-node.service`)

```ini
[Unit]
Description=HiveStack Node Agent Service
After=network.target libvirtd.service
Wants=network.target

[Service]
Type=simple
ExecStart=/usr/bin/hive-node --config /etc/hivestack/node.yaml
Restart=always
RestartSec=5
Environment=HIVE_STACK_LOG_LEVEL=info

# Security hardening
ProtectSystem=strict
ProtectHome=yes
NoNewPrivileges=yes
ReadWritePaths=/var/lib/hivestack /var/log/hivestack /run/hivestack
PrivateTmp=yes

# Capability bounding (KVM requires SYS_ADMIN)
CapabilityBoundingSet=CAP_SYS_ADMIN CAP_NET_ADMIN

# Resource limits
LimitNOFILE=65536

[Install]
WantedBy=multi-user.target
```

---

## Verification

### Check Manager Status

```bash
# Service status
sudo systemctl status hivestack-manager

# Health check
curl -k https://localhost:8443/health

# View logs
sudo journalctl -u hivestack-manager -f
```

### Check Node Agent Status

```bash
# Service status
sudo systemctl status hivestack-node

# View logs
sudo journalctl -u hivestack-node -f
```

### Test API

```bash
# Login
curl -k -X POST https://localhost:8443/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{"email":"admin@example.com","password":"secret"}'

# List hosts (use token from login)
curl -k https://localhost:8443/api/v1/hosts \
  -H "Authorization: Bearer <token>"
```

### Test CLI

```bash
./bin/hive host list
./bin/hive vm list
./bin/hive health
```

---

## Troubleshooting

### Manager won't start

| Symptom | Cause | Fix |
|---------|-------|-----|
| `connection refused` to database | PostgreSQL not running | `sudo systemctl start postgresql` |
| `permission denied` on TLS certs | Wrong file permissions | `sudo chmod 600 /etc/hivestack/tls/*.key` |
| `bind: address already in use` | Port 8080 occupied | Change port in config or kill conflicting process |

### Node Agent won't connect to Manager

| Symptom | Cause | Fix |
|---------|-------|-----|
| `TLS handshake failed` | Certificate mismatch | Regenerate certs with correct CN |
| `connection refused` | Manager gRPC not reachable | Check firewall rules for port 9090 |
| `libvirt connection failed` | libvirtd not running | `sudo systemctl start libvirtd` |

### VM Operations Fail

| Symptom | Cause | Fix |
|---------|-------|-----|
| `insufficient resources` | Node lacks CPU/memory | Check host capacity |
| `storage pool not found` | Pool not configured | Create storage pool via API |
| `HANA compliance violation` | Non-compliant VM spec | Enable hugepages, dedicated CPUs, no ballooning |

### Useful Commands

```bash
# Check gRPC connectivity
openssl s_client -connect manager:9090 -CAfile /etc/hivestack/tls/ca.crt

# View all HiveStack logs
sudo journalctl -u hivestack-manager -u hivestack-node -f

# Check database connectivity
psql -U hivestack -d hivestack -c "SELECT count(*) FROM hosts;"

# Verify libvirt on node
virsh list --all
virsh capabilities
```
