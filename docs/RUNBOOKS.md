# HiveStack Operational Runbooks

**Version:** 1.0.0
**Last Updated:** 2026-09-19

## Table of Contents

- [Overview](#overview)
- [Common Procedures](#common-procedures)
  - [Deploy New Manager](#deploy-new-manager)
  - [Add Compute Node](#add-compute-node)
  - [Remove Compute Node](#remove-compute-node)
  - [Upgrade Manager](#upgrade-manager)
  - [Rotate TLS Certificates](#rotate-tls-certificates)
  - [Backup and Restore](#backup-and-restore)
  - [Database Migration](#database-migration)
- [Troubleshooting](#troubleshooting)
  - [Manager Won't Start](#manager-wont-start)
  - [Node Can't Connect to Manager](#node-cant-connect-to-manager)
  - [VM Operations Fail](#vm-operations-fail)
  - [Database Connection Issues](#database-connection-issues)
  - [Performance Degradation](#performance-degradation)
- [Monitoring & Alerting](#monitoring--alerting)
- [Emergency Procedures](#emergency-procedures)

---

## Overview

This runbook contains step-by-step procedures for common operational tasks on a HiveStack deployment. It is intended for operators and SRE teams managing production HiveStack environments.

### Prerequisites

- SSH access to Manager and Node hosts
- `sudo` privileges on all hosts
- `hive` CLI installed locally
- Access to PostgreSQL (for database tasks)

### Quick Reference

| Task | Command/Location |
|------|-----------------|
| Manager status | `sudo systemctl status hivestack-manager` |
| Node status | `sudo systemctl status hivestack-node` |
| Manager logs | `sudo journalctl -u hivestack-manager -f` |
| Node logs | `sudo journalctl -u hivestack-node -f` |
| Health check | `curl -k https://localhost:8443/api/v1/health` |
| CLI health | `hive health` |
| Database query | `psql -U hivestack -d hivestack -c "..."` |

---

## Common Procedures

### Deploy New Manager

**Scenario:** Deploying a HiveStack Manager on a new host.

**Steps:**

1. **Prepare the host:**
   ```bash
   # Install dependencies (SLES 15 SP7)
   sudo zypper install postgresql15-server go1.26

   # Create HiveStack directories
   sudo mkdir -p /etc/hivestack/tls /var/lib/hivestack /var/log/hivestack /run/hivestack
   ```

2. **Install binaries:**
   ```bash
   # Download latest release
   wget https://github.com/maddydevel/HiveStack/releases/latest/download/hivestack-v*-linux-amd64.tar.gz
   tar xzf hivestack-v*-linux-amd64.tar.gz

   # Install binaries
   sudo cp hive-manager /usr/bin/
   sudo cp hive /usr/bin/
   sudo chmod +x /usr/bin/hive-manager /usr/bin/hive
   ```

3. **Generate TLS certificates:**
   ```bash
   ./scripts/gen-certs.sh
   sudo cp ca.crt /etc/hivestack/tls/
   sudo cp manager.crt /etc/hivestack/tls/
   sudo cp manager.key /etc/hivestack/tls/
   sudo chmod 600 /etc/hivestack/tls/*.key
   ```

4. **Configure and start:**
   ```bash
   sudo cp configs/manager.yaml.example /etc/hivestack/manager.yaml
   sudo vim /etc/hivestack/manager.yaml  # Edit as needed

   sudo cp pkg/systemd/hivestack-manager.service /etc/systemd/system/
   sudo systemctl daemon-reload
   sudo systemctl enable --now hivestack-manager
   ```

5. **Verify:**
   ```bash
   sudo systemctl status hivestack-manager
   curl -k https://localhost:8443/api/v1/health
   hive health
   ```

---

### Add Compute Node

**Scenario:** Adding a new compute node to an existing HiveStack cluster.

**Steps:**

1. **Prepare the node host:**
   ```bash
   # Install KVM and dependencies
   sudo zypper install kvm qemu libvirt-daemon
   sudo systemctl enable --now libvirtd

   # Verify virtualization support
   egrep -c '(vmx|svm)' /proc/cpuinfo  # Should be > 0
   ```

2. **Install node agent:**
   ```bash
   # Download and extract
   wget https://github.com/maddydevel/HiveStack/releases/latest/download/hivestack-v*-linux-amd64.tar.gz
   tar xzf hivestack-v*-linux-amd64.tar.gz

   # Install binary
   sudo cp hive-node /usr/bin/
   sudo chmod +x /usr/bin/hive-node
   ```

3. **Generate node certificate:**
   ```bash
   # On Manager host, generate node cert
   ./scripts/gen-certs.sh node-<node-id>

   # Copy certs to node
   scp ca.crt node-<node-id>.crt node-<node-id>.key node-host:/etc/hivestack/tls/
   ```

4. **Configure and start:**
   ```bash
   sudo mkdir -p /etc/hivestack/tls /var/lib/hivestack /var/log/hivestack
   sudo chmod 600 /etc/hivestack/tls/node-*.key

   # Create config
   sudo tee /etc/hivestack/node.yaml <<EOF
   node_id: "node-03"
   manager_address: "manager.example.com:9090"
   libvirt_uri: "qemu:///system"
   grpc:
     port: 9090
     tls:
       ca_cert: "/etc/hivestack/tls/ca.crt"
       cert: "/etc/hivestack/tls/node-03.crt"
       key: "/etc/hivestack/tls/node-03.key"
   storage:
     base_path: "/var/lib/hivestack/storage"
   logging:
     level: "info"
   EOF

   # Install systemd service
   sudo cp pkg/systemd/hivestack-node.service /etc/systemd/system/
   sudo systemctl daemon-reload
   sudo systemctl enable --now hivestack-node
   ```

5. **Verify on Manager:**
   ```bash
   hive host list
   # Should show new node with "connected" status
   ```

---

### Remove Compute Node

**Scenario:** Decommissioning a compute node.

**Steps:**

1. **Drain the node (migrate VMs away):**
   ```bash
   # Set node to maintenance mode
   hive host maintenance enable node-03

   # Migrate all VMs to other nodes
   hive vm list --host node-03 --format json | \
     jq -r '.[].id' | \
     xargs -I{} hive vm migrate {} --target node-01
   ```

2. **Wait for migrations to complete:**
   ```bash
   # Check for remaining VMs on node
   hive vm list --host node-03
   ```

3. **Stop and remove node agent:**
   ```bash
   # On the node host
   sudo systemctl stop hivestack-node
   sudo systemctl disable hivestack-node
   sudo rm /etc/systemd/system/hivestack-node.service
   sudo rm /usr/bin/hive-node
   ```

4. **Remove node from Manager:**
   ```bash
   hive host delete node-03
   ```

5. **Revoke node certificate:**
   ```bash
   # On Manager host
   # Remove node cert from trust store if using a CA
   # For self-signed, no action needed (node can't connect anyway)
   ```

---

### Upgrade Manager

**Scenario:** Upgrading HiveStack Manager to a new version.

**Steps:**

1. **Pre-upgrade checks:**
   ```bash
   # Note current version
   hive version

   # Backup database
   pg_dump -U hivestack hivestack > /backup/hivestack-pre-upgrade-$(date +%Y%m%d).sql

   # Backup configuration
   sudo cp -r /etc/hivestack /etc/hivestack.backup.$(date +%Y%m%d)
   ```

2. **Download and install new version:**
   ```bash
   wget https://github.com/maddydevel/HiveStack/releases/download/vX.Y.Z/hivestack-vX.Y.Z-linux-amd64.tar.gz
   tar xzf hivestack-vX.Y.Z-linux-amd64.tar.gz

   # Stop manager
   sudo systemctl stop hivestack-manager

   # Install new binary
   sudo cp hive-manager /usr/bin/
   sudo cp hive /usr/bin/
   ```

3. **Run database migrations:**
   ```bash
   # Migrations run automatically on startup if configured
   # Or run manually:
   for f in internal/db/migrations/*.sql; do
     # Skip already applied migrations
     psql -U hivestack -d hivestack -f "$f"
   done
   ```

4. **Start and verify:**
   ```bash
   sudo systemctl start hivestack-manager
   sudo systemctl status hivestack-manager
   curl -k https://localhost:8443/api/v1/health
   hive version
   ```

5. **Post-upgrade verification:**
   - Verify all nodes reconnect
   - Check VM list and status
   - Verify API endpoints respond
   - Monitor logs for errors

---

### Rotate TLS Certificates

**Scenario:** Rotating TLS certificates before expiry.

**Steps:**

1. **Generate new certificates:**
   ```bash
   # Backup old certs
   sudo cp -r /etc/hivestack/tls /etc/hivestack/tls.old

   # Generate new CA and certs
   ./scripts/gen-certs.sh --force
   ```

2. **Distribute new Manager cert:**
   ```bash
   # On Manager host
   sudo cp ca.crt /etc/hivestack/tls/
   sudo cp manager.crt /etc/hivestack/tls/
   sudo cp manager.key /etc/hivestack/tls/
   sudo chmod 600 /etc/hivestack/tls/manager.key

   # Restart Manager
   sudo systemctl restart hivestack-manager
   ```

3. **Distribute new Node certs (one at a time):**
   ```bash
   # For each node:
   scp ca.crt node-<id>.crt node-<id>.key node-host:/etc/hivestack/tls/

   # On node host:
   sudo systemctl restart hive-node
   ```

4. **Verify all connections:**
   ```bash
   hive host list
   # All nodes should show "connected"
   ```

---

### Backup and Restore

**Scenario:** Backing up and restoring HiveStack data.

**Full Backup:**

```bash
# Backup database
pg_dump -U hivestack -Fc hivestack > /backup/hivestack-db-$(date +%Y%m%d).dump

# Backup configuration
sudo tar czf /backup/hivestack-config-$(date +%Y%m%d).tar.gz /etc/hivestack/

# Backup TLS keys
sudo tar czf /backup/hivestack-tls-$(date +%Y%m%d).tar.gz /etc/hivestack/tls/

# Backup VM data (on each node)
sudo tar czf /backup/hivestack-vms-$(date +%Y%m%d).tar.gz /var/lib/hivestack/storage/
```

**Restore:**

```bash
# Restore database
pg_restore -U hivestack -d hivestack --clean /backup/hivestack-db-XXXXXXXX.dump

# Restore configuration
sudo tar xzf /backup/hivestack-config-XXXXXXXX.tar.gz -C /

# Restore TLS
sudo tar xzf /backup/hivestack-tls-XXXXXXXX.tar.gz -C /
sudo chmod 600 /etc/hivestack/tls/*.key

# Restart services
sudo systemctl restart hivestack-manager
sudo systemctl restart hivestack-node
```

---

### Database Migration

**Scenario:** Applying or rolling back database migrations.

**Apply pending migrations:**

```bash
# Check current version
psql -U hivestack -d hivestack -c "SELECT * FROM schema_migrations ORDER BY version DESC LIMIT 5;"

# Apply all pending migrations
for f in internal/db/migrations/*.sql; do
  psql -U hivestack -d hivestack -v ON_ERROR_STOP=1 -f "$f" || exit 1
done
```

**Rollback a migration:**

```bash
# Apply specific rollback file (if exists)
psql -U hivestack -d hivestack -f internal/db/migrations/XXXX_rollback.sql

# Update schema_migrations table
psql -U hivestack -d hivestack -c "DELETE FROM schema_migrations WHERE version = 'XXXX';"
```

---

## Troubleshooting

### Manager Won't Start

**Symptom:** `sudo systemctl status hivestack-manager` shows failed state.

**Diagnostic steps:**

1. Check logs:
   ```bash
   sudo journalctl -u hivestack-manager --since "10 minutes ago"
   ```

2. Check for port conflicts:
   ```bash
   sudo ss -tlnp | grep -E '8080|8443|9090'
   ```

3. Check database connectivity:
   ```bash
   psql -U hivestack -d hivestack -h localhost -c "SELECT 1;"
   ```

4. Check TLS certificates:
   ```bash
   sudo ls -la /etc/hivestack/tls/
   openssl x509 -in /etc/hivestack/tls/manager.crt -noout -dates
   ```

**Common fixes:**

| Cause | Fix |
|-------|-----|
| Port already in use | `sudo fuser -k 8080/tcp` or change port in config |
| Database down | `sudo systemctl start postgresql` |
| Invalid TLS cert | Regenerate: `./scripts/gen-certs.sh` |
| Config syntax error | Validate YAML: `hive-manager --config /etc/hivestack/manager.yaml --dry-run` |

---

### Node Can't Connect to Manager

**Symptom:** Node logs show "connection refused" or "TLS handshake failed".

**Diagnostic steps:**

1. Check network connectivity:
   ```bash
   # From node host
   nc -zv manager-host 9090
   ```

2. Verify TLS certificates:
   ```bash
   openssl s_client -connect manager-host:9090 -CAfile /etc/hivestack/tls/ca.crt
   ```

3. Check Manager is listening:
   ```bash
   # On Manager host
   sudo ss -tlnp | grep 9090
   ```

**Common fixes:**

| Cause | Fix |
|-------|-----|
| Firewall blocking port 9090 | `sudo firewall-cmd --add-port=9090/tcp --permanent && sudo firewall-cmd --reload` |
| Wrong manager_address in node config | Update `/etc/hivestack/node.yaml` |
| Certificate CN mismatch | Regenerate node cert with correct hostname |
| Manager gRPC not running | `sudo systemctl restart hivestack-manager` |

---

### VM Operations Fail

**Symptom:** VM create/start/migrate fails with errors.

**Diagnostic steps:**

1. Check node status:
   ```bash
   hive host status node-01
   ```

2. Check libvirt on node:
   ```bash
   # On node host
   sudo systemctl status libvirtd
   virsh list --all
   virsh capabilities
   ```

3. Check storage:
   ```bash
   hive storage list
   df -h /var/lib/hivestack/storage
   ```

4. Check resource availability:
   ```bash
   hive host list --format json | jq '.[] | {name, cpu_usage, memory_used, memory_total}'
   ```

---

### Database Connection Issues

**Symptom:** Manager logs show "connection refused" or "too many connections".

**Diagnostic steps:**

1. Check PostgreSQL status:
   ```bash
   sudo systemctl status postgresql
   ```

2. Check active connections:
   ```sql
   SELECT count(*) FROM pg_stat_activity;
   SELECT max_connections FROM pg_settings WHERE name = 'max_connections';
   ```

3. Check connection string:
   ```bash
   # Test with psql
   psql "$HIVESTACK_DSN" -c "SELECT 1;"
   ```

---

### Performance Degradation

**Symptom:** Slow API response or high resource usage.

**Diagnostic steps:**

1. Check system resources:
   ```bash
   top -bn1 | head -20
   free -h
   df -h
   ```

2. Check Manager metrics:
   ```bash
   curl -k https://localhost:9091/metrics | grep -E '(process_cpu_seconds|go_goroutines)'
   ```

3. Check for long-running database queries:
   ```sql
   SELECT pid, now() - pg_stat_activity.query_start AS duration, query
   FROM pg_stat_activity
   WHERE state = 'active' AND now() - pg_stat_activity.query_start > interval '30 seconds';
   ```

---

## Monitoring & Alerting

### Key Metrics

| Metric | Warning Threshold | Critical Threshold |
|--------|------------------|-------------------|
| API response time | > 500ms | > 2s |
| Manager CPU usage | > 70% | > 90% |
| Node CPU usage | > 80% | > 95% |
| Database connections | > 80% of max | > 95% of max |
| VM operation failure rate | > 5% | > 15% |
| Node heartbeat age | > 60s | > 180s |

### Log Locations

| Service | Log Location |
|---------|-------------|
| Manager | `/var/log/hivestack/manager.log` |
| Node | `/var/log/hivestack/node.log` |
| Journal | `sudo journalctl -u hivestack-manager` |
| PostgreSQL | `/var/log/postgresql/` |

### Prometheus Metrics

HiveStack exposes Prometheus metrics at:

- Manager: `http://localhost:9091/metrics`
- Node: `http://localhost:9092/metrics`

---

## Emergency Procedures

### Complete Manager Failure

If the Manager host is completely unavailable:

1. **Promote standby Manager** (if HA configured):
   ```bash
   # On standby host
   sudo systemctl start hivestack-manager
   ```

2. **If no standby, deploy new Manager:**
   - Follow [Deploy New Manager](#deploy-new-manager)
   - Restore database from backup
   - Update node configs to point to new Manager address

3. **Verify all nodes reconnect:**
   ```bash
   hive host list
   ```

### Data Corruption

If database corruption is detected:

1. **Stop all writes:**
   ```bash
   sudo systemctl stop hivestack-manager
   ```

2. **Restore from backup:**
   ```bash
   pg_restore -U hivestack -d hivestack --clean /backup/hivestack-db-latest.dump
   ```

3. **Restart Manager:**
   ```bash
   sudo systemctl start hivestack-manager
   ```

4. **Verify data integrity:**
   ```bash
   hive host list
   hive vm list
   ```

### Security Breach

If a security breach is suspected:

1. **Isolate affected systems:**
   ```bash
   sudo iptables -A INPUT -j DROP  # Block all incoming
   sudo iptables -A OUTPUT -j DROP  # Block all outgoing
   ```

2. **Revoke all certificates:**
   ```bash
   # Revoke CA and regenerate
   ./scripts/gen-certs.sh --force --revoke-all
   ```

3. **Rotate all credentials:**
   - JWT secrets
   - Database passwords
   - API tokens

4. **Audit access logs:**
   ```bash
   sudo journalctl -u hivestack-manager | grep -i "auth\|login\|token"
   ```

5. **Rebuild affected systems** from known-good state.
