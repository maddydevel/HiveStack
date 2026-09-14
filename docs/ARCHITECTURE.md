# HiveStack Architecture

## Overview

HiveStack is a KVM-based virtualization appliance on SUSE SLES 15 SP7.

## Components

### HiveStack Manager
- Web UI (React/Vue, TBD)
- REST API (Go, gorilla/mux)
- CLI (`hive`, Go, cobra)
- Auth + RBAC
- PostgreSQL database

### HiveStack Node
- SLES 15 SP7 + KVM/QEMU/libvirt
- Node agent (Go) communicating with Manager
- Corosync/Pacemaker for HA (optional)
- Storage: local (dir, LVM, ZFS), shared (NFS, iSCSI, Ceph)

## API

See [api/openapi.yaml](api/openapi.yaml).

## Build

```bash
make build
```
