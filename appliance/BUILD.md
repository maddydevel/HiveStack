# HiveStack Appliance Build Documentation

## Overview

This directory contains everything needed to build bootable HiveStack appliance images in multiple formats:

| Format | Use Case | Tool |
|--------|----------|------|
| **QCOW2/OVA/VMDK** | VMware, KVM, VirtualBox VMs | Packer |
| **ISO** | Bare-metal installation, USB boot | live-build |
| **Docker** | Container deployment, evaluation | Docker |
| **Helm Chart** | Kubernetes deployment | Helm |

## Quick Start

### Option 1: Docker (Easiest)

```bash
cd appliance/docker
docker compose -f docker-compose.appliance.yml up -d
# Access: https://localhost:8443
```

### Option 2: Build VM Image (Packer)

```bash
cd appliance/packer
packer init hivestack.pkr.hcl
packer build hivestack.pkr.hcl
# Output: output-hivestack/hivestack-1.0.0.qcow2
```

### Option 3: Build ISO (live-build)

```bash
cd appliance/live-build
sudo ./build-iso.sh
# Output: output/hivestack-appliance-1.0.0-YYYYMMDD-amd64.iso
```

## Prerequisites

### For Packer Builds

- Packer >= 1.9
- QEMU/KVM (`qemu-system-x86_64`, `qemu-img`)
- 20GB free disk space

**Install on Ubuntu:**
```bash
apt install qemu-system-x86 qemu-utils
packer version  # >= 1.9
```

### For live-build ISO

- Ubuntu 24.04 LTS build host
- 20GB free disk space

**Install dependencies:**
```bash
sudo apt install live-build squashfs-tools xorriso grub-efi-amd64-bin
```

### For Docker

- Docker >= 24.0
- Docker Compose >= 2.0
- 10GB free disk space

## Detailed Build Instructions

### Packer VM Images

Packer builds QCOW2 (native), VMDK (VMware), and OVA (universal) images from Ubuntu 24.04.

**What gets installed:**
- Ubuntu 24.04 LTS minimal
- KVM/QEMU/libvirt
- PostgreSQL 15
- HiveStack binaries
- systemd services (manager, node, first-boot)

**Build:**
```bash
cd appliance/packer
packer init hivestack.pkr.hcl
packer build hivestack.pkr.hcl
```

**Outputs:**
- `output-hivestack/hivestack-1.0.0.qcow2` — QEMU/KVM native
- `output-hivestack/hivestack-1.0.0.vmdk` — VMware ESXi/VirtualBox
- `output-hivestack/hivestack-1.0.0.ova` — Universal VM format

**Testing in QEMU:**
```bash
qemu-system-x86_64 -m 4G -drive file=output-hivestack/hivestack-1.0.0.qcow2,format=qcow2 -enable-kvm
```

**Convert to other formats:**
```bash
# QCOW2 -> VMDK
qemu-img convert -f qcow2 -O vmdk hivestack.qcow2 hivestack.vmdk

# QCOW2 -> VHD (Hyper-V)
qemu-img convert -f qcow2 -O vpc hivestack.qcow2 hivestack.vhd
```

### Live-Build ISO

Builds a bootable Ubuntu 24.04 ISO with HiveStack pre-installed. Supports both UEFI and BIOS boot.

**Build:**
```bash
cd appliance/live-build
sudo ./build-iso.sh
```

**Output:** `output/hivestack-appliance-1.0.0-YYYYMMDD-amd64.iso`

**Boot from ISO:**
```bash
# QEMU
qemu-system-x86_64 -m 4G -cdrom output/hivestack-appliance-1.0.0-20260921-amd64.iso -boot d

# Write to USB
sudo dd if=output/hivestack-appliance-1.0.0-20260921-amd64.iso of=/dev/sdX bs=4M status=progress
```

**First Boot:**
1. Boot from ISO/USB
2. Ubuntu installer runs (auto-configured)
3. System reboots
4. First-boot wizard initializes PostgreSQL, generates TLS certs, creates config
5. HiveStack services start automatically

### Docker Appliance

**Build and run:**
```bash
cd appliance/docker

# Build image
docker build -f Dockerfile.appliance -t hivestack:appliance .

# Run all-in-one
docker run -d --name hivestack \
  -p 8080:8080 -p 8443:8443 -p 44567:44567 \
  hivestack:appliance

# Run with Docker Compose (recommended)
docker compose -f docker-compose.appliance.yml up -d
```

**Access:**
- Web UI: https://localhost:8443
- API: https://localhost:8443/api/v1
- Health: http://localhost:8080/api/v1/health

### Helm Chart

```bash
cd appliance/helm
helm install hivestack . --namespace hivestack --create-namespace
```

## Post-Installation

### First Boot Setup

On first boot, the system automatically:
1. Initializes PostgreSQL
2. Generates TLS certificates (CA + server + client)
3. Creates HiveStack config files (`/etc/hivestack/manager.yaml`, `/etc/hivestack/node.yaml`)
4. Runs database migrations
5. Creates default admin user
6. Configures libvirt storage and networking
7. Sets up hugepages for HANA VMs
8. Starts all services

### Access

- **Web UI:** `https://<ip>:8443`
- **API:** `https://<ip>:8443/api/v1`
- **Health:** `http://<ip>:8080/api/v1/health`
- **Default admin:** `admin@localhost` (password in first-boot logs)

### Registering Nodes

1. Generate node certificates on manager:
   ```bash
   /opt/hivestack/bin/hive-manager node register <node-id>
   ```

2. Install on node host:
   ```bash
   scp /etc/hivestack/certs/{ca.crt,node-server.*} node-host:/etc/hivestack/certs/
   /opt/hivestack/bin/hive-node --config /etc/hivestack/node.yaml
   ```

## Troubleshooting

### ISO won't boot in UEFI mode
- Ensure `grub-efi-amd64` is installed in the build
- Check that the ISO was written in DD mode: `dd if=iso of=/dev/sdX bs=4M`

### QCOW2 image won't boot
- Verify QEMU supports KVM: `kvm-ok`
- Check image: `qemu-img info hivestack.qcow2`

### Services not starting
```bash
# Check logs
journalctl -u hivestack-manager -f
journalctl -u hivestack-node -f

# Verify PostgreSQL
pg_isready -h localhost -p 5432 -U hivestack

# Check TLS certs
ls -la /etc/hivestack/certs/
```

### Docker: Permission denied
The container needs `privileged: true` for KVM access:
```yaml
services:
  hivestack:
    privileged: true
    volumes:
      - /dev:/dev
```

## Release Process

### GitHub Actions

The `.github/workflows/release-appliance.yml` workflow:
1. Triggers on tag push (`v*`)
2. Builds all image formats (QCOW2, ISO, Docker)
3. Generates checksums
4. Creates GitHub Release with artifacts

### Manual Release

```bash
# Build all formats
cd appliance/packer && packer build hivestack.pkr.hcl
cd ../live-build && sudo ./build-iso.sh
cd ../docker && docker build -t hivestack:v1.0.0 -f Dockerfile.appliance .

# Create release
gh release create v1.0.0 \
  output-hivestack/*.qcow2 \
  output-hivestack/*.vmdk \
  output/*.iso \
  --title "HiveStack v1.0.0" \
  --notes "Release notes here"
```

## File Structure

```
appliance/
├── packer/              # Packer VM image builds
│   ├── hivestack.pkr.hcl
│   ├── http/            # Preseed/cloud-init
│   └── scripts/         # Provisioning scripts
├── live-build/          # Bootable ISO builds
│   ├── auto/config      # live-build config
│   ├── config/          # Hooks and includes
│   └── build-iso.sh
├── docker/              # Docker appliance
│   ├── Dockerfile.appliance
│   ├── entrypoint.sh
│   └── docker-compose.appliance.yml
├── helm/                # Kubernetes deployment
│   └── Chart.yaml
├── scripts/             # Shared scripts
│   ├── first-boot.sh
│   └── entrypoint.sh
├── systemd/             # Service files
│   ├── hivestack-manager.service
│   └── hivestack-node.service
├── config/              # Config templates
│   ├── manager.yaml.example
│   └── node.yaml.example
└── README.md
```

## Support

- Documentation: https://github.com/maddydevel/HiveStack/docs
- Issues: https://github.com/maddydevel/HiveStack/issues
- Security: security@hivestack.io
