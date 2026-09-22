# HiveStack Troubleshooting Guide

## Table of Contents

1. [Installation Issues](#installation-issues)
2. [Service Startup Failures](#service-startup-failures)
3. [Database Issues](#database-issues)
4. [Authentication Problems](#authentication-problems)
5. [VM Lifecycle Failures](#vm-lifecycle-failures)
6. [Network Connectivity Issues](#network-connectivity-issues)
7. [Storage Backend Failures](#storage-backend-failures)
8. [HA Failover Issues](#ha-failover-issues)
9. [Performance Degradation](#performance-degradation)
10. [Log Locations and Analysis](#log-locations-and-analysis)
11. [Common Error Messages](#common-error-messages)
12. [Debug Mode and Diagnostics](#debug-mode-and-diagnostics)

---

## Installation Issues

### ISO Won't Boot

**Symptom**: "Not a bootable disk" or system ignores ISO

**Solutions**:
1. Verify ISO integrity: `sha256sum hivestack-demo.iso`
2. In QEMU: Use `-boot d` flag to boot from CD
3. For USB: Use `dd` correctly: `sudo dd if=hivestack-demo.iso of=/dev/sdX bs=4M status=progress`
4. Ensure BIOS/UEFI boot order prioritizes CD/USB
5. Try disabling Secure Boot in BIOS

### Docker Container Won't Start

**Symptom**: Container exits immediately

**Solutions**:
1. Check logs: `docker logs hivestack-manager`
2. Verify PostgreSQL is running: `docker compose ps`
3. Check port conflicts: `ss -tlnp | grep 8080`
4. Ensure environment variables are set:
   ```bash
   HIVESTACK_DSN="postgres://hivestack:hivestack@postgres:5432/hivestack"
   HIVESTACK_JWT_SECRET="your-secret-here"
   ```

### Source Build Failures

**Symptom**: `go build ./...` fails

**Solutions**:
1. Verify Go version: `go version` (requires 1.23+)
2. Download dependencies: `go mod download`
3. Clean module cache: `go clean -modcache && go mod download`
4. Check for CGO issues: `CGO_ENABLED=0 go build ./...`

---

## Service Startup Failures

### Manager Won't Start

**Symptom**: `systemctl status hivestack-manager` shows failed

**Solutions**:
1. Check logs: `journalctl -u hivestack-manager -f`
2. Verify config file exists: `/etc/hivestack/manager.yaml`
3. Test config: `hive-manager --config /etc/hivestack/manager.yaml --validate`
4. Check database connection: `pg_isready -h localhost -p 5432 -U hivestack`
5. Verify JWT secret is set

### Node Agent Won't Start

**Symptom**: `systemctl status hivestack-node` shows failed

**Solutions**:
1. Check logs: `journalctl -u hivestack-node -f`
2. Verify libvirtd: `systemctl status libvirtd`
3. Check KVM: `ls -la /dev/kvm`
4. Test libvirt connection: `virsh -c qemu:///system list`
5. Verify TLS certificates exist

---

## Database Issues

### Connection Refused

**Symptom**: "connection refused" on port 5432

**Solutions**:
1. Start PostgreSQL: `systemctl start postgresql`
2. Check PostgreSQL is listening: `ss -tlnp | grep 5432`
3. Verify pg_hba.conf allows local connections
4. Check PostgreSQL logs: `journalctl -u postgresql`

### Migration Failures

**Symptom**: "migration failed" or database errors

**Solutions**:
1. Check migration status: `SELECT * FROM schema_migrations;`
2. Run migrations manually:
   ```bash
   for f in internal/db/migrations/*.sql; do
     psql -h localhost -U hivestack -d hivestack -f "$f"
   done
   ```
3. For down migrations: `internal/db/migrations/down/*.sql`

---

## Authentication Problems

### Invalid Token

**Symptom**: "unauthorized" errors

**Solutions**:
1. Check token expiry (default 15 minutes for access tokens)
2. Refresh token: `hive auth refresh`
3. Regenerate token: `hive config set-token <new-token>`
4. Verify JWT secret matches between manager and API calls

### Permission Denied

**Symptom**: "forbidden" errors

**Solutions**:
1. Check user role: `hive user me`
2. Verify RBAC permissions for the resource
3. Admin users can check roles: `hive user list`

---

## VM Lifecycle Failures

### VM Create Fails

**Symptom**: VM creation returns error

**Solutions**:
1. Check host capacity: `hive host status <host-id>`
2. Verify HANA compliance (if role=hana): `hive compliance validate`
3. Check storage pool has space
4. Verify network bridge exists

### VM Start Fails

**Symptom**: VM stuck in "starting" or "error" state

**Solutions**:
1. Check libvirt logs: `journalctl -u libvirtd`
2. Verify KVM acceleration: `kvm-ok`
3. Check QEMU logs: `/var/log/libvirt/qemu/<vm-name>.log`
4. Verify VM XML: `virsh dumpxml <vm-name>`

### Migration Fails

**Symptom**: VM migration hangs or fails

**Solutions**:
1. Verify source and target hosts are online
2. Check network connectivity between hosts
3. Verify shared storage is accessible
4. Check migration port (TCP 49152-49215)

---

## Network Connectivity Issues

### Node Can't Reach Manager

**Symptom**: Node heartbeat fails

**Solutions**:
1. Test connectivity: `curl -k https://<manager-ip>:8443/api/v1/health`
2. Check firewall rules (ports 8080, 8443, 44567)
3. Verify DNS resolution
4. Check TLS certificates are valid

### VM Network Issues

**Symptom**: VM has no network connectivity

**Solutions**:
1. Check bridge: `brctl show` or `ip link show`
2. Verify libvirt network: `virsh net-list --all`
3. Start default network: `virsh net-start default`
4. Check iptables rules: `iptables -L -n`

---

## Storage Backend Failures

### NFS Mount Fails

**Symptom**: "mount.nfs: Connection timed out"

**Solutions**:
1. Test NFS: `showmount -e <nfs-server>`
2. Check NFS service: `systemctl status nfs-client.target`
3. Verify exports on server: `cat /etc/exports`
4. Check firewall (nfs, rpc-bind, mountd)

### Ceph RBD Fails

**Symptom**: "rbd: failed to map"

**Solutions**:
1. Check Ceph health: `ceph -s`
2. Verify keyring: `ceph auth get-or-create client.hivestack`
3. Test RBD: `rbd map <pool>/<image>`

---

## HA Failover Issues

### Failover Doesn't Trigger

**Symptom**: Host marked offline but VMs not restarted

**Solutions**:
1. Check HA policy: `hive host ha-policy <host-id>`
2. Verify target hosts have capacity
3. Check fence configuration
4. Review HA logs: `journalctl -u hivestack-manager | grep ha`

### Split Brain

**Symptom**: Multiple hosts think they're primary

**Solutions**:
1. Check network between HA nodes
2. Verify clock synchronization (chrony)
3. Review fencing configuration
4. Manual intervention: `hive ha fence <host-id>`

---

## Performance Degradation

### Slow VM Operations

**Solutions**:
1. Check host CPU/memory: `top`, `free -h`
2. Monitor I/O: `iostat -x 1`
3. Check PostgreSQL: `SELECT * FROM pg_stat_activity;`
4. Review Prometheus metrics: `http://<ip>:8080/metrics`

### Database Slowdown

**Solutions**:
1. Check slow queries: `SELECT * FROM pg_stat_statements ORDER BY mean_time DESC LIMIT 10;`
2. Vacuum database: `VACUUM ANALYZE;`
3. Check connection pool settings
4. Monitor disk I/O on PostgreSQL storage

---

## Log Locations and Analysis

### Log Files

| Service | Log Location |
|---------|-------------|
| Manager | `journalctl -u hivestack-manager` |
| Node Agent | `journalctl -u hivestack-node` |
| libvirtd | `journalctl -u libvirtd` |
| PostgreSQL | `journalctl -u postgresql` |
| QEMU | `/var/log/libvirt/qemu/*.log` |
| System | `journalctl -xe` |

### Enable Debug Mode

```bash
# Manager
hive-manager --config /etc/hivestack/manager.yaml --log-level debug

# Node agent
hive-node --config /etc/hivestack/node.yaml --log-level debug

# libvirtd
echo 'log_filters="3:remote 4:event"' >> /etc/libvirt/libvirtd.conf
echo 'log_outputs="3:file:/var/log/libvirt/libvirtd.log"' >> /etc/libvirt/libvirtd.conf
systemctl restart libvirtd
```

---

## Common Error Messages

| Error | Cause | Solution |
|-------|-------|----------|
| `unauthorized` | Invalid/expired token | Refresh or regenerate token |
| `forbidden` | Insufficient permissions | Check RBAC role |
| `connection refused` | Service not running | Start the service |
| `migration failed` | Network/storage issue | Check connectivity |
| `compliance violation` | HANA guardrail breach | Fix VM configuration |
| `fence failed` | IPMI/SSH unreachable | Check BMC connectivity |
| `pool exhausted` | No storage space | Free up space or add storage |

---

## Debug Mode and Diagnostics

### Health Check

```bash
curl -k https://<ip>:8443/api/v1/health
```

Expected response: `{"status":"ok"}`

### Cluster Status

```bash
hive host list
hive vm list
hive events list --severity error
```

### Generate Diagnostics Bundle

```bash
#!/bin/bash
# Create diagnostics bundle
BUNDLE="hivestack-diagnostics-$(date +%Y%m%d-%H%M%S).tar.gz"
tar -czf "$BUNDLE"   /etc/hivestack/   /var/log/hivestack/   <(journalctl -u hivestack-manager --since "1 hour ago")   <(journalctl -u hivestack-node --since "1 hour ago")   <(virsh list --all)   <(virsh nodeinfo)
echo "Diagnostics bundle: $BUNDLE"
```

### Reset Admin Password

```bash
sudo -u postgres psql -d hivestack -c   "UPDATE users SET password_hash = '$(echo -n 'newpassword' | argon2 $(openssl rand -hex 8) -e)' WHERE email = 'admin@localhost';"
```

---

## Getting Help

- GitHub Issues: https://github.com/maddydevel/HiveStack/issues
- Documentation: https://github.com/maddydevel/HiveStack/docs
- Security: security@hivestack.io
