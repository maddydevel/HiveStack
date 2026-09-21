# HiveStack Appliance

Bootable appliance images for HiveStack — the KVM hypervisor management platform.

## Build Formats

| Format | Directory | Use |
|--------|-----------|-----|
| Packer (QCOW2/VMDK/OVA) | `packer/` | VM deployment (VMware, KVM, VirtualBox) |
| Live-build ISO | `live-build/` | Bare-metal installation, USB boot |
| Docker | `docker/` | Container deployment, evaluation |
| Helm Chart | `helm/` | Kubernetes deployment |

## Quick Start

```bash
# Docker (easiest)
cd docker && docker compose -f docker-compose.appliance.yml up -d

# Packer (VM images)
cd packer && packer build hivestack.pkr.hcl

# Live-build (bootable ISO)
cd live-build && sudo ./build-iso.sh
```

See [BUILD.md](BUILD.md) for detailed instructions.

## Architecture

- **Base:** Ubuntu 24.04 LTS
- **Virtualization:** KVM/QEMU/libvirt
- **Database:** PostgreSQL 15
- **Services:** Manager (REST API + gRPC), Node Agent
- **Security:** mTLS, JWT, RBAC

## First Boot

The appliance automatically:
1. Initializes PostgreSQL with HiveStack schema
2. Generates TLS certificates (CA, server, client)
3. Creates manager/node configuration
4. Runs database migrations
5. Configures libvirt storage and networking
6. Sets up hugepages for HANA VMs
7. Starts all services

Access: `https://<ip>:8443` (Web UI), `https://<ip>:8080/api/v1/health` (Health)

## Ports

| Port | Service |
|------|---------|
| 8080 | HTTP API |
| 8443 | HTTPS API + gRPC |
| 44567 | Manager gRPC |
| 5432 | PostgreSQL |
| 9090 | Cockpit Web UI |
| 9100 | Prometheus Metrics |

## Post-Installation Security

- [ ] Change default database password
- [ ] Change JWT secret (32+ random chars)
- [ ] Regenerate TLS certificates
- [ ] Set root password
- [ ] Configure firewall
- [ ] Enable automatic security updates

## License

Apache 2.0 — Copyright 2026 Maddy AI Consultancy

## Support

- GitHub: https://github.com/maddydevel/HiveStack
- Issues: https://github.com/maddydevel/HiveStack/issues
- Security: security@hivestack.io

## Project Structure

```
appliance/
├── Dockerfile                    # Multi-stage container build
├── docker-compose.yaml           # Full stack orchestration
├── build-appliance.sh            # KIWI ISO build script
├── config.xml                    # KIWI image description
├── config.sh                     # KIWI chroot customization
├── images.sh                     # KIWI image cleanup
├── config/
│   ├── manager.yaml.example      # Manager config template
│   └── node.yaml.example         # Node config template
├── systemd/
│   ├── hivestack-manager.service # Manager systemd unit
│   └── hivestack-node.service    # Node systemd unit
├── scripts/
│   ├── first-boot.sh             # First-boot initialization
│   ├── build-container.sh        # Container image build
│   ├── entrypoint.sh             # Container entrypoint
│   └── hivestack-first-boot.service
├── overlay/                      # Files overlaid into ISO
│   ├── etc/systemd/system/
│   └── tmp/HiveStack/
├── images/                       # KIWI image config
└── profiles/                     # KIWI build profiles
```

## Quick Start

### Option 1: Container Deployment (Recommended)

```bash
# Build and run with Docker Compose
cd /tmp/HiveStack/appliance
docker compose up -d

# Or build manually
./scripts/build-container.sh all latest

# Run manager
docker run -d --name hivestack-manager \
  -p 8080:8080 -p 8443:8443 \
  -v hivestack-data:/var/lib/hivestack \
  hivestack:latest manager

# Run node agent
docker run -d --name hivestack-node \
  --privileged \
  -p 9090:9090 \
  -v /dev/kvm:/dev/kvm \
  hivestack:latest node
```

### Option 2: Bare Metal / VM Appliance

```bash
# Build the ISO
cd /tmp/HiveStack/appliance
sudo ./build-appliance.sh

# Write to USB (replace sdX with your device)
sudo cp /tmp/hivestack-build/*.iso /dev/sdX

# Boot from USB and install
```

## Building the Container Image

### Prerequisites
- Docker or Podman installed
- Go 1.26+ (for building binaries)
- At least 4GB free disk space

### Build Commands

```bash
cd /tmp/HiveStack/appliance

# Build all components (default)
./scripts/build-container.sh all latest

# Build specific component
./scripts/build-container.sh manager v1.0.0
./scripts/build-container.sh node v1.0.0

# Build Go binaries only
./scripts/build-container.sh binaries

# Build with custom registry
REGISTRY=ghcr.io/maddydevel ./scripts/build-container.sh all v1.0.0
```

### Container Image Details

The Dockerfile uses multi-stage builds:
1. **Builder stage**: Compiles Go binaries with `CGO_ENABLED=0` for static linking
2. **Runtime stage**: Based on Rocky Linux 9 minimal with required runtime dependencies

**Ports exposed:**
- `8080` - HiveStack Manager REST API
- `8443` - HiveStack Manager gRPC
- `9090` - HiveStack Node Agent gRPC

**Volumes:**
- `/var/lib/hivestack` - Persistent data
- `/etc/hivestack` - Configuration and certificates
- `/var/log/hivestack` - Log files

## Docker Compose Deployment

The `docker-compose.yaml` provides a complete stack:

```yaml
services:
  postgres:     # PostgreSQL 15 database
  manager:      # HiveStack Manager (REST API + gRPC)
  node:         # HiveStack Node Agent (libvirt/KVM)
  node-exporter: # Prometheus metrics
```

### Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `POSTGRES_PASSWORD` | `hivestack-secure-password-change-me` | Database password |
| `JWT_SECRET` | `change-this-in-production-use-strong-random-secret` | JWT signing secret |
| `NODE_ID` | (auto) | Node identifier |
| `MANAGER_ADDRESS` | `manager:8443` | Manager gRPC address |

### Scaling Nodes

```bash
docker compose up -d --scale node=3
```

## Systemd Services

For bare-metal or VM deployments, install the systemd service files:

```bash
# Copy service files
sudo cp systemd/hivestack-manager.service /etc/systemd/system/
sudo cp systemd/hivestack-node.service /etc/systemd/system/

# Reload systemd
sudo systemctl daemon-reload

# Enable and start services
sudo systemctl enable --now hivestack-manager
sudo systemctl enable --now hivestack-node
```

### Service Details

**hivestack-manager.service:**
- Runs as `hivestack:hivestack`
- Depends on `network.target` and `postgresql.service`
- Security hardening: `ProtectSystem=strict`, `NoNewPrivileges=yes`, etc.
- Resource limits: `LimitNOFILE=65536`, `LimitMEMLOCK=infinity`

**hivestack-node.service:**
- Runs as `hivestack-node:hivestack-node`
- Depends on `network.target` and `libvirtd.service`
- Requires `CAP_SYS_ADMIN` and `CAP_SYS_RESOURCE` for KVM management
- `PrivateDevices=false` (needs `/dev/kvm` access)

## Configuration

### Manager Configuration (`/etc/hivestack/manager.yaml`)

```yaml
server:
  host: "0.0.0.0"
  port: 8080
  tls_enabled: true
  tls_cert_file: "/etc/hivestack/certs/manager.crt"
  tls_key_file: "/etc/hivestack/certs/manager.key"

grpc:
  host: "0.0.0.0"
  port: 8443
  tls_enabled: true
  tls_cert_file: "/etc/hivestack/certs/manager.crt"
  tls_key_file: "/etc/hivestack/certs/manager.key"
  tls_ca_file: "/etc/hivestack/certs/ca.crt"

database:
  dsn: "postgres://hivestack:***@localhost:5432/hivestack?sslmode=disable"

auth:
  jwt_secret: "change-this-in-production-use-strong-random-secret-min-32-chars"
  token_expiry: "24h"

logging:
  level: "info"
  format: "json"
  output: "/var/log/hivestack/manager.log"
```

### Node Configuration (`/etc/hivestack/node.yaml`)

```yaml
manager_address: "hivestack-manager:8443"
grpc_address: "0.0.0.0:9090"
node_id: ""
heartbeat_interval: "30s"
libvirt_uri: "qemu:///system"

tls:
  cert_file: "/etc/hivestack/certs/node-server.crt"
  key_file: "/etc/hivestack/certs/node-server.key"
  ca_file: "/etc/hivestack/certs/ca.crt"

logging:
  level: "info"
  format: "json"
  output: "/var/log/hivestack/node.log"
```

## First Boot Process

On first boot, the appliance automatically:

1. **Initializes PostgreSQL** - Creates `hivestack` database and user
2. **Generates TLS certificates** - CA, Manager server, Node client/server certs for mTLS (using ECDSA P-256)
3. **Creates default configuration** - `/etc/hivestack/manager.yaml` and `node.yaml`
4. **Installs systemd services** - `hivestack-manager.service`, `hivestack-node.service`
5. **Configures libvirt** - Creates default storage pool and network
6. **Reserves hugepages** - 2GB (1024 x 2MB) for HANA VMs
7. **Starts all services** - PostgreSQL, libvirtd, Manager, Node Agent, Cockpit, Prometheus
8. **Verifies health** - Checks API endpoint responds

### Environment Variables for First Boot

| Variable | Default | Description |
|----------|---------|-------------|
| `HIVESTACK_DB_PASSWORD` | `hivestack-secure-password-change-me` | Database password |
| `HIVESTACK_JWT_SECRET` | `change-this-in-production...` | JWT signing secret |
| `HIVESTACK_MANAGER_HOST` | `hivestack-manager` | Manager hostname for certs |
| `HIVESTACK_HUGEPAGES` | `1024` | Number of 2MB hugepages |

## Building the ISO Appliance

### Build Host Requirements
- SUSE SLES 15 SP7 (or openSUSE Leap 15.5+)
- kiwi-ng installed: `zypper install kiwi-ng python3-kiwi`
- Root access (for chroot operations)
- At least 20GB free disk space
- Internet access for package repositories

### Register SLES (if using SLES)
```bash
SUSEConnect -r <registration-code> -e <email>
```

### Build Commands
```bash
cd /tmp/HiveStack/appliance
sudo ./build-appliance.sh
```

### Build Output
- `hivestack-appliance-<version>-<date>-x86_64.iso` - Bootable installation ISO
- `hivestack-appliance-<version>-<date>-x86_64.iso.sha256` - SHA256 checksum

## Appliance Components

### Base System
- SLES 15 SP7 minimal installation
- Kernel-default with KVM support
- systemd, NetworkManager, chrony

### Virtualization Stack
- qemu-kvm, qemu-tools
- libvirt-daemon, libvirt-client, libvirt-daemon-driver-qemu
- virt-install, virt-manager-common
- libguestfs-tools, guestfsd
- OVMF/EDK2 for UEFI VMs
- openvswitch for advanced networking

### Database
- postgresql15-server, postgresql15-contrib

### Management & Monitoring
- cockpit, cockpit-machines, cockpit-podman, cockpit-storaged
- prometheus-node-exporter
- podman, buildah, skopeo, crictl

### Security
- apparmor with HiveStack profiles
- auditd
- firewalld with HiveStack zone

### HiveStack Specific
- Manager service (REST API + gRPC)
- Node Agent (gRPC server for libvirt)
- CLI tools
- First-boot initialization script

## Post-Installation Configuration

### Required Changes (Security)
```bash
# 1. Change database password
sudo -u postgres psql -c "ALTER USER hivestack WITH PASSWORD 'your-secure-password';"

# 2. Update JWT secret
# Edit /etc/hivestack/manager.yaml
# jwt_secret: "your-strong-random-secret-min-32-chars"

# 3. Update manager.yaml with new DB password
# dsn: "postgres://hivestack:***@localhost:5432/hivestack?sslmode=disable"

# 4. Restart services
systemctl restart hivestack-manager hivestack-node
```

### Network Configuration
```bash
# Configure static IP (if not using DHCP)
nmcli con mod "Wired connection 1" \
    ipv4.method manual \
    ipv4.addresses 192.168.1.100/24 \
    ipv4.gateway 192.168.1.1 \
    ipv4.dns "8.8.8.8,1.1.1.1" \
    connection.autoconnect yes
nmcli con up "Wired connection 1"
```

### Access Points
| Service | URL | Default Credentials |
|---------|-----|---------------------|
| HiveStack Manager API | https://<ip>:8080 | (JWT token) |
| HiveStack Manager gRPC | <ip>:8443 | mTLS cert |
| Node Agent gRPC | <ip>:9090 | mTLS cert |
| Cockpit Web UI | https://<ip>:9090 | root / (no password - set one) |
| Prometheus Metrics | http://<ip>:9100/metrics | - |

## Deploying HiveStack Binaries

### Option 1: Build on Appliance
```bash
cd /tmp/HiveStack
export PATH=$PATH:/usr/local/go/bin
go build -o /opt/hivestack/bin/hive-manager ./cmd/hive-manager
go build -o /opt/hivestack/bin/hive-node ./node
go build -o /opt/hivestack/bin/hive ./cmd/hive
systemctl restart hivestack-manager hivestack-node
```

### Option 2: Copy from Build Host
```bash
# On build host
scp bin/hive-manager bin/hive-node bin/hive root@<appliance-ip>:/opt/hivestack/bin/
ssh root@<appliance-ip> "systemctl restart hivestack-manager hivestack-node"
```

### Option 3: Build RPM Package (Recommended for Production)
```bash
# Create RPM spec and build
# Then install on appliance:
rpm -ivh hivestack-*.rpm
```

## Adding Compute Nodes

1. Install the same ISO on additional servers
2. On each node, configure `/etc/hivestack/node.yaml` with Manager address
3. On Manager, register node via API:
   ```bash
   curl -k -X POST https://<manager>:8080/api/v1/hosts \
     -H "Authorization: Bearer ***" \
     -H "Content-Type: application/json" \
     -d '{"name":"node-01","address":"<node-ip>","grpc_port":9090}'
   ```

## Updating the Appliance

### Rebuild from Source
```bash
cd /tmp/HiveStack/appliance
sudo ./build-appliance.sh
```

### In-Place Updates
```bash
# System updates
zypper patch

# HiveStack updates
# 1. Build new binaries
# 2. Replace /opt/hivestack/bin/*
# 3. Run migrations: hive-manager migrate up
# 4. Restart services
```

### Container Updates
```bash
cd /tmp/HiveStack/appliance
./scripts/build-container.sh all v1.1.0
docker compose up -d --build
```

## Troubleshooting

### Build Fails
- Check `/var/log/kiwi/` for build logs
- Ensure all repositories are accessible
- Verify disk space: `df -h /tmp`

### First Boot Fails
```bash
# Check logs
journalctl -u hivestack-first-boot -f
cat /var/log/hivestack/first-boot.log

# Re-run manually
/etc/hivestack/.first-boot && /tmp/HiveStack/appliance/scripts/first-boot.sh
```

### Services Won't Start
```bash
# Check status
systemctl status hivestack-manager hivestack-node

# Check logs
journalctl -u hivestack-manager -f
journalctl -u hivestack-node -f

# Verify config
/opt/hivestack/bin/hive-manager --config /etc/hivestack/manager.yaml --validate
```

### Container Issues
```bash
# Check container logs
docker logs hivestack-manager
docker logs hivestack-node

# Inspect container
docker inspect hivestack-manager

# Check health
docker ps --filter "name=hivestack"
```

### libvirt Issues
```bash
# Check libvirtd
systemctl status libvirtd
virsh -c qemu:///system list

# Check KVM
ls -la /dev/kvm
kvm-ok
```

## Security Checklist

- [ ] Change default database password
- [ ] Change JWT secret (32+ random chars)
- [ ] Regenerate TLS certificates with proper CA
- [ ] Set root password: `passwd root`
- [ ] Configure firewall for your network
- [ ] Enable automatic security updates: `systemctl enable transactional-update.timer`
- [ ] Configure auditd rules for compliance
- [ ] Set up log forwarding to SIEM

## Support

- GitHub: https://github.com/maddydevel/HiveStack
- Documentation: https://github.com/maddydevel/HiveStack/docs
- Issues: https://github.com/maddydevel/HiveStack/issues

## License

TBD — likely GPLv3 or Apache 2.0 for open source, with commercial options.

Copyright (c) 2026 Maddy AI Consultancy
