# HiveStack

KVM-based virtualization appliance on SUSE SLES 15 SP7 — a VMware vCenter/ESXi migration target with vCenter-like management, ESXi-like hypervisor nodes, and Proxmox-level flexibility.

## Quick Start

### Prerequisites
- Go 1.23+
- Kubernetes (optional, for containerized deployment)
- SUSE SLES 15 SP7 (for appliance builds)

### Build

```bash
make build
```

This produces:
- `bin/hive` — CLI tool
- `bin/hive-node` — Node agent binary

### Run CLI (development)

```bash
./bin/hive --server http://localhost:8080 vm list
./bin/hive --server http://localhost:8080 host list
```

## Architecture

### Components

- **HiveStack Manager**: Control plane — web UI, REST API, CLI (`hive`), auth, RBAC, PostgreSQL database
- **HiveStack Node**: Compute node — KVM/QEMU/libvirt, node agent (gRPC), SLES 15 SP7

### Technology Stack

| Component | Technology |
|-----------|------------|
| Manager backend | Go 1.23, chi router |
| CLI | Go, cobra |
| Web UI | React 18 + TypeScript + Vite |
| Database | PostgreSQL (embedded initially) |
| Virtualization | KVM + QEMU + libvirt |
| Node agent | Go, gRPC |
| HA | Corosync/Pacemaker |
| Monitoring | Prometheus + Grafana |

### Directory Structure

```
cmd/hive/          # CLI main entry point
internal/          # Private application packages
  cli/             # CLI commands and implementations
  api/             # REST API server
  auth/            # Authentication and RBAC
  db/              # Database layer
  config/          # Configuration handling
  node/            # Node agent (also in top-level node/)
  migration/       # VMware migration tools
pkg/               # Public packages (API client, etc.)
api/               # OpenAPI specification
web/               # Web UI (React + TypeScript)
node/              # Node agent (top-level for separate builds)
migration/         # VMware migration tools
docs/              # Documentation
tests/             # Integration and E2E tests
scripts/           # Build and utility scripts
```

## API

See [api/openapi.yaml](api/openapi.yaml) for the full REST API specification.

## CLI

```bash
hive vm list                          # List all VMs
hive vm create my-vm --cpus 4 --mem 8G   # Create a VM
hive vm start my-vm                    # Start a VM
hive vm stop my-vm                     # Stop a VM
hive vm migrate my-vm --target node-2  # Migrate a VM
hive vm snapshot my-vm --name snap1    # Create snapshot
hive vm console my-vm                  # Open console
hive host list                         # List hosts
hive storage list                      # List storage pools
hive network list                      # List networks
hive backup create --vm my-vm          # Create backup
hive backup list                       # List backups
hive migrate import ovf file.ova       # Import from OVA
hive migrate import vmx file.vmx       # Import from VMX
hive migrate discover --vcenter https://vcenter --username admin  # Discover vCenter
```

Run `hive --help` for full command reference.

## Documentation

- [Architecture](docs/ARCHITECTURE.md) — System architecture, components, data flows, technology choices
- [API Reference](api/openapi.yaml) — Full REST API specification (OpenAPI 3.0)

## Development

### Prerequisites
- Go 1.23+
- Node.js 18+ (for web UI development)
- golangci-lint (for linting)

### Build

```bash
make build    # Build CLI and Node agent
make test     # Run tests
make lint     # Run linters
```

### Testing

```bash
go test ./... -v -race -cover
```

## Contributing

1. Fork the repository
2. Create a feature branch
3. Make changes
4. Run `make test` and `make lint`
5. Submit a pull request

## License

TBD — likely GPLv3 or Apache 2.0 for open source, with commercial options.

Copyright (c) 2026 Maddy AI Consultancy
