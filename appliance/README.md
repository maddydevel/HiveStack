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

## Ports

| Port | Service |
|------|---------|
| 8080 | HTTP API |
| 8443 | HTTPS API + gRPC |
| 44567 | Manager gRPC |
| 5432 | PostgreSQL |

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

## Configuration

### Manager (`/etc/hivestack/manager.yaml`)

```yaml
dsn: "postgres://hivestack:***@localhost:5432/hivestack?sslmode=disable"
jwt_secret: "change-me-use-random-32-chars"
tls_cert_file: "/etc/hivestack/certs/manager.crt"
tls_key_file: "/etc/hivestack/certs/manager.key"
tls_ca_file: "/etc/hivestack/certs/ca.crt"
listen_address: ":8080"
listen_tls_address: ":8443"
grpc_port: 44567
log_level: "info"
log_format: "json"
```

### Node (`/etc/hivestack/node.yaml`)

```yaml
node_id: "node-001"
manager_address: "localhost:44567"
tls_cert_file: "/etc/hivestack/certs/node-server.crt"
tls_key_file: "/etc/hivestack/certs/node-server.key"
tls_ca_file: "/etc/hivestack/certs/ca.crt"
heartbeat_interval: 30
log_level: "info"
log_format: "json"
```

## Post-Installation Security

- [ ] Change default database password
- [ ] Change JWT secret (32+ random chars)
- [ ] Regenerate TLS certificates
- [ ] Set root password
- [ ] Configure firewall
- [ ] Enable automatic security updates

## Systemd Services

```bash
# Enable and start
sudo systemctl enable --now hivestack-manager
sudo systemctl enable --now hivestack-node

# Check status
systemctl status hivestack-manager hivestack-node

# View logs
journalctl -u hivestack-manager -f
journalctl -u hivestack-node -f
```

## Adding Compute Nodes

1. Install the same appliance on additional servers
2. Configure `/etc/hivestack/node.yaml` with Manager address
3. Register node via API:

```bash
curl -k -X POST https://<manager>:8080/api/v1/hosts \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"name":"node-01","address":"<node-ip>","grpc_port":44567}'
```

## Updating

```bash
# Build new binaries
go build -o /opt/hivestack/bin/hive-manager ./cmd/hive-manager
go build -o /opt/hivestack/bin/hive-node ./node

# Run migrations
/opt/hivestack/bin/hive-manager migrate up

# Restart
systemctl restart hivestack-manager hivestack-node
```

## Troubleshooting

```bash
# Services won't start
systemctl status hivestack-manager hivestack-node
journalctl -u hivestack-manager -f

# Check PostgreSQL
pg_isready -h localhost -p 5432 -U hivestack

# Check TLS certs
ls -la /etc/hivestack/certs/

# Check libvirt
systemctl status libvirtd
virsh list --all

# Docker
docker compose -f appliance/docker/docker-compose.appliance.yml logs
```

## Project Structure

```
appliance/
├── packer/              # Packer VM image builds
│   ├── hivestack.pkr.hcl
│   ├── http/            # Preseed/cloud-init
│   └── scripts/         # Provisioning scripts
├── live-build/          # Bootable ISO builds
│   ├── auto/config
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
├── BUILD.md             # Detailed build docs
└── README.md            # This file
```

## License

Apache 2.0 — Copyright 2026 Maddy AI Consultancy

## Support

- GitHub: https://github.com/maddydevel/HiveStack
- Issues: https://github.com/maddydevel/HiveStack/issues
- Security: security@hivestack.io
