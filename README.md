# HiveStack

KVM-based virtualization appliance on SUSE SLES 15 SP7 — a VMware migration target with vCenter-like management, ESXi-like hypervisor nodes, and Proxmox-level flexibility.

## Architecture

- **HiveStack Manager**: Control plane — web UI, REST API, CLI (`hive`), auth, RBAC
- **HiveStack Node**: Compute node — KVM/QEMU, libvirt, live migration, HA

## Building

```bash
make build
```

## Documentation

See [docs/](docs/) for architecture, installation, and administration.
