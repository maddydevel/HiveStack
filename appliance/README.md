# HiveStack Appliance Build Documentation

## Overview

This directory contains the KIWI appliance description for building a SUSE SLES 15 SP7 based ISO image with HiveStack pre-installed.

## Prerequisites

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

## Building the Appliance

### Quick Build
```bash
cd /tmp/HiveStack/appliance
sudo ./build-appliance.sh
```

### Manual Build Steps
```bash
# 1. Install kiwi-ng
sudo zypper install kiwi-ng python3-kiwi

# 2. Build the appliance
cd /tmp/HiveStack/appliance
sudo kiwi-ng --type iso \
    --description . \
    --target-dir /tmp/hivestack-build \
    --profile Standard \
    --allow-existing-root \
    build

# 3. Find the ISO
ls -la /tmp/hivestack-build/*.iso
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

## First Boot Process

On first boot, the appliance automatically:

1. **Initializes PostgreSQL** - Creates `hivestack` database and user
2. **Generates TLS certificates** - CA, Manager server, Node client/server certs for mTLS
3. **Creates default configuration** - `/etc/hivestack/manager.yaml` and `node.yaml`
4. **Installs systemd services** - `hivestack-manager.service`, `hivestack-node.service`
5. **Configures libvirt** - Creates default storage pool and network
6. **Reserves hugepages** - 2GB (1024 x 2MB) for HANA VMs
7. **Starts all services** - PostgreSQL, libvirtd, Manager, Node Agent, Cockpit, Prometheus
8. **Verifies health** - Checks API endpoint responds

## Post-Installation Configuration

### Required Changes (Security)
```bash
# 1. Change database password
sudo -u postgres psql -c "ALTER USER hivestack WITH PASSWORD 'your-secure-password';"

# 2. Update JWT secret
# Edit /etc/hivestack/manager.yaml
# jwt_secret: "your-strong-random-secret-min-32-chars"

# 3. Update manager.yaml with new DB password
# dsn: "postgres://hivestack:your-secure-password@localhost:5432/hivestack?sslmode=disable"

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

The appliance includes placeholder scripts. To deploy actual binaries:

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
     -H "Authorization: Bearer <token>" \
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