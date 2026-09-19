# HiveStack Security Guide

This document describes the security architecture and hardening measures
implemented in HiveStack, designed to run on SLES 15 SP7.

## Table of Contents

1. [Security Architecture Overview](#security-architecture-overview)
2. [TLS and Mutual Authentication](#tls-and-mutual-authentication)
3. [Secret Management](#secret-management)
4. [AppArmor Mandatory Access Control](#apparmor-mandatory-access-control)
5. [Audit Logging](#audit-logging)
6. [LUKS Disk Encryption](#luks-disk-encryption)
7. [REST API Security](#rest-api-security)
8. [RBAC and Authorization](#rbac-and-authorization)
9. [Deployment Checklist](#deployment-checklist)
10. [Compliance](#compliance)

---

## Security Architecture Overview

HiveStack implements defense-in-depth with the following layers:

```
┌─────────────────────────────────────────────────────────────────┐
│ External Network (Untrusted)                                     │
│  - TLS 1.3 only                                                  │
│  - Rate limiting (100 req/s per IP)                              │
│  - CORS headers                                                  │
├─────────────────────────────────────────────────────────────────┤
│ REST API Server                                                  │
│  - JWT authentication (Argon2id hashed passwords)               │
│  - RBAC authorization                                           │
│  - Security headers (HSTS, X-Frame-Options, CSP)               │
├─────────────────────────────────────────────────────────────────┤
│ gRPC (mTLS between Manager and Nodes)                           │
│  - Mutual TLS with client certificates                          │
│  - Certificate rotation                                          │
├─────────────────────────────────────────────────────────────────┤
│ AppArmor MAC (per-service profiles)                             │
│  - File access restrictions                                      │
│  - Network access restrictions                                   │
│  - Capability bounding                                           │
├─────────────────────────────────────────────────────────────────┤
│ LUKS Disk Encryption                                            │
│  - VM storage encryption                                         │
│  - Backup storage encryption                                     │
│  - Swap encryption                                               │
├─────────────────────────────────────────────────────────────────┤
│ Audit Logging                                                    │
│  - Structured JSON events                                        │
│  - SIEM integration                                              │
└─────────────────────────────────────────────────────────────────┘
```

---

## TLS and Mutual Authentication

### Certificate Authority (CA)

HiveStack uses a private CA for internal service communication:

```bash
# Generate CA
./scripts/gen-certs.sh --ca-dir /etc/hivestack/tls

# Or manually:
openssl req -x509 -new -nodes -newkey ec -pkeyopt ec_paramgen_curve:prime256v1 \
    -keyout ca.key -out ca.crt -days 3650 \
    -subj "/O=HiveStack/CN=HiveStack Root CA"
```

### Server Certificate (Manager)

```bash
# Generate server cert signed by CA
openssl req -new -nodes -newkey ec -pkeyopt ec_paramgen_curve:prime256v1 \
    -keyout server.key -out server.csr \
    -subj "/O=HiveStack/CN=manager.hivestack.local"

openssl x509 -req -in server.csr -CA ca.crt -CAkey ca.key \
    -CAcreateserial -out server.crt -days 365 \
    -extfile <(printf "subjectAltName=DNS:localhost,DNS:manager.hivestack.local,IP:127.0.0.1")
```

### Client Certificate (Node Agent)

Each Node Agent gets a unique client certificate:

```bash
# Generate client cert for node-01
./scripts/gen-certs.sh --node-id node-01 --domain hivestack.local
```

### TLS Configuration

| Setting | Value |
|---------|-------|
| Min Version | TLS 1.3 |
| Max Version | TLS 1.3 |
| Cipher Suites | TLS_AES_256_GCM_SHA384, TLS_AES_128_GCM_SHA256, TLS_CHACHA20_POLY1305_SHA256 |
| Curve Preferences | X25519, P-256, P-384 |
| Client Auth | RequireAndVerifyClientCert (gRPC) |
| Session Tickets | Disabled (Node Agent) |

### Certificate Rotation

Certificates should be rotated before expiry (default: 1 year). Set up a cron job:

```bash
# Check certificate expiry
openssl x509 -in /etc/hivestack/tls/server.crt -noout -dates

# Rotate when < 30 days to expiry
```

---

## Secret Management

### Storage Hierarchy

HiveStack uses a layered approach to secret management:

1. **HashiCorp Vault** (preferred for dynamic secrets)
2. **SOPS/age** (fallback for encrypted secrets at rest)
3. **Environment variables** (development only)

### HashiCorp Vault Setup

```bash
# Enable KV v2 secrets engine
vault secrets enable -path=hivestack kv-v2

# Enable database secrets engine for dynamic credentials
vault secrets enable database

# Configure PostgreSQL connection
vault write database/config/hivestack \
    plugin_name=postgresql-database-plugin \
    connection_url="postgresql://{{username}}:{{password}}@localhost:5432/hivestack" \
    allowed_roles="hivestack" \
    username="vaultadmin" \
    password="..."

# Create role with dynamic credentials
vault write database/roles/hivestack \
    db_name=hivestack \
    default_ttl="1h" \
    max_ttl="24h" \
    creation_statements="CREATE ROLE \"{{name}}\" WITH LOGIN PASSWORD '{{password}}' VALID UNTIL '{{expiration}}';"
```

### SOPS/age Setup

```bash
# Generate age key
age-keygen -o /etc/hivestack/sops-key.txt

# Encrypt a secret
sops --encrypt --age $(cat /etc/hivestack/sops-key.txt | grep -oP 'public key: \K.*') \
    --encrypted-regex '^(data|stringData)$' \
    secrets.yaml > secrets.enc.yaml

# Decrypt at runtime (automated by HiveStack)
sops --decrypt /etc/hivestack/secrets/database_password.enc.yaml
```

### Dynamic Database Credentials

When Vault is configured, HiveStack requests temporary PostgreSQL credentials:

```go
// Dynamic credentials - automatically rotated by Vault
dbConfig, err := secretManager.GetDBCredentials(ctx)
// Username: v-token-hivestack-abc123 (expires in 1 hour)
// Password: auto-generated, rotated by Vault
```

---

## AppArmor Mandatory Access Control

SLES 15 SP7 includes AppArmor for mandatory access control. HiveStack generates
restrictive profiles for both the Manager and Node Agent services.

### Manager Profile

The Manager profile restricts:
- File access: `/etc/hivestack/**` (read), `/var/lib/hivestack/**` (read-write)
- Network: Only TCP streams on localhost for PostgreSQL
- Capabilities: `net_bind_service` only (no `sys_admin`, `sys_ptrace`)
- Deny: Write to `/usr/**`, `/boot/**`, read `/etc/shadow`

### Node Agent Profile

The Node Agent profile restricts:
- File access: `/etc/hivestack/**`, `/var/lib/hivestack/**`, VM images
- Network: Stream connect to Manager gRPC port
- Libvirt: Read-write access to `/var/run/libvirt/libvirt-sock`
- Capabilities: `net_bind_service`, `chown`, `fowner` (for VM disk ownership)
- Deny: Write to `/usr/**`, `/boot/**`

### Installation

```bash
# Generate and install profiles
sudo hive-manager --generate-apparmor-profiles

# Or manually:
sudo apparmor_parser -r /etc/apparmor.d/hivestack-manager
sudo apparmor_parser -r /etc/apparmor.d/hivestack-node

# Check status
sudo aa-status
```

### Verification

```bash
# Confirm profiles are loaded
sudo apparmor_status | grep hivestack

# Check for denials
sudo journalctl -k | grep apparmor | grep DENIED
```

---

## Audit Logging

HiveStack generates structured audit logs for security-relevant events.

### Event Categories

| Category | Description | Severity |
|----------|-------------|----------|
| auth | Login/logout, token refresh | info - warning |
| authorization | RBAC denials, privilege escalation | warning - error |
| data_access | VM creation, storage access | info |
| system | Service start/stop, config changes | info - warning |
| security | Cert expiry, encryption operations | warning - critical |

### Audit Event Format

```json
{
  "id": "audit-1234567890-12345",
  "timestamp": "2026-09-19T10:30:00Z",
  "type": "auth",
  "severity": "info",
  "actor": "user-abc123",
  "action": "login",
  "resource": "session",
  "result": "success",
  "client_ip": "10.0.1.50",
  "metadata": {
    "method": "password",
    "mfa": "totp"
  }
}
```

### Log Forwarding

Configure forwarding to your SIEM:

```yaml
audit:
  log_dir: /var/log/hivestack/audit
  forwarder:
    enabled: true
    network: tcp
    address: "siem.example.com:514"
    format: syslog
```

### Log Rotation

```bash
# Logrotate configuration
# /etc/logrotate.d/hivestack-audit
/var/log/hivestack/audit/audit.log {
    daily
    rotate 90
    compress
    delaycompress
    missingok
    notifempty
    create 0640 hivestack hivestack
}
```

---

## LUKS Disk Encryption

VM storage and backups are encrypted at rest using LUKS2.

### Default Encryption Settings

| Setting | Value | Notes |
|---------|-------|-------|
| Cipher | aes-xts-plain64 | Standard for disk encryption |
| Key Size | 512 bits | 256-bit XTS (uses two keys) |
| Hash | sha256 | For key derivation |
| LUKS Version | 2 | Latest with metadata backup |
| Filesystem | XFS | Default for VM storage |

### Encrypted Volume Types

1. **VM Storage**: `/var/lib/hivestack/images/` (mapped device)
2. **Backup Storage**: `/var/lib/hivestack/backups/` (mapped device)
3. **Swap**: Encrypted with random key on boot (no hibernation support)

### Creating an Encrypted Volume

```bash
# Generate a key file
openssl rand -hex 64 > /etc/hivestack/luks-keys/vm-data.key
chmod 400 /etc/hivestack/luks-keys/vm-data.key

# Format with LUKS
cryptsetup luksFormat --type luks2 \
    --cipher aes-xts-plain64 \
    --key-size 512 \
    --hash sha256 \
    --key-file /etc/hivestack/luks-keys/vm-data.key \
    --batch-mode \
    /dev/sdb

# Open (unlock) the volume
cryptsetup open --type luks2 \
    --key-file /etc/hivestack/luks-keys/vm-data.key \
    /dev/sdb hivestack-vm-data

# Create filesystem
mkfs.xfs /dev/mapper/hivestack-vm-data

# Mount
mount /dev/mapper/hivestack-vm-data /var/lib/hivestack/images
```

### Verification

```bash
# Check LUKS status
cryptsetup status hivestack-vm-data

# Check if device is LUKS
cryptsetup isLuks /dev/sdb && echo "LUKS: yes" || echo "LUKS: no"

# List active mappings
ls /dev/mapper/
```

---

## REST API Security

### Security Headers

All responses include security headers:

| Header | Value | Purpose |
|--------|-------|---------|
| Strict-Transport-Security | max-age=31536000; includeSubDomains; preload | HSTS |
| X-Content-Type-Options | nosniff | Prevent MIME sniffing |
| X-Frame-Options | DENY | Clickjacking protection |
| X-XSS-Protection | 1; mode=block | XSS filter |
| Referrer-Policy | strict-origin-when-cross-origin | Privacy |
| Content-Security-Policy | default-src 'self' | XSS mitigation |
| Cache-Control | no-store | No sensitive data caching |
| Pragma | no-cache | Backward compatibility |

### Rate Limiting

| Setting | Value | Description |
|---------|-------|-------------|
| Rate | 100 req/s | Per IP address |
| Burst | 200 req | Maximum burst size |
| Response | 423 Too Many Requests | With Retry-After header |

### CORS Configuration

By default, CORS is disabled (same-origin only). To enable:

```yaml
api:
  cors:
    enabled: true
    allowed_origins:
      - "https://hivestack.example.com"
    allowed_methods: ["GET", "POST", "PUT", "DELETE"]
    allowed_headers: ["Authorization", "Content-Type"]
    allow_credentials: true
    max_age: 300
```

### Request Limits

- Maximum body size: 10 MB (configurable)
- Request timeout: 30 seconds (configurable)

---

## RBAC and Authorization

HiveStack uses Role-Based Access Control (RBAC) with predefined roles:

| Role | Description | Permissions |
|------|-------------|-------------|
| admin | Full system access | * |
| operator | VM lifecycle management | vm:*, host:read |
| viewer | Read-only access | vm:read, host:read, storage:read |

### Policy Enforcement

```go
// Check if user can perform action
if !rbac.CheckPermission(user, "vm", "create") {
    http.Error(w, "Forbidden", http.StatusForbidden)
    return
}
```

---

## Deployment Checklist

### Pre-Deployment

- [ ] Generate CA certificate
- [ ] Generate server certificate for Manager
- [ ] Generate client certificate for each Node Agent
- [ ] Configure Vault or SOPS for secret management
- [ ] Enable AppArmor on all hosts
- [ ] Configure LUKS encryption for storage volumes

### Per-Node Setup

```bash
# 1. Install HiveStack
sudo rpm -i hivestack-node-*.rpm

# 2. Install certificates
sudo mkdir -p /etc/hivestack/tls
sudo cp ca.crt client.crt client.key /etc/hivestack/tls/
sudo chmod 600 /etc/hivestack/tls/client.key
sudo chmod 644 /etc/hivestack/tls/client.crt /etc/hivestack/tls/ca.crt

# 3. Setup LUKS (if not already done)
sudo cryptsetup open /dev/sdb hivestack-vm-data \
    --key-file /etc/hivestack/luks-keys/vm-data.key
sudo mount /dev/mapper/hivestack-vm-data /var/lib/hivestack/images

# 4. Enable AppArmor
sudo apparmor_parser -r /etc/apparmor.d/hivestack-node

# 5. Start service
sudo systemctl enable --now hivestack-node
```

### Post-Deployment Verification

- [ ] Verify TLS: `openssl s_client -connect manager:8443 -tls1_3`
- [ ] Verify mTLS: Attempt connection without client cert (should fail)
- [ ] Verify AppArmor: `sudo aa-status | grep hivestack`
- [ ] Verify LUKS: `cryptsetup status hivestack-vm-data`
- [ ] Verify audit logs: `tail /var/log/hivestack/audit/audit.log`
- [ ] Verify rate limiting: `curl -w "%{http_code}" -o /dev/null https://manager:8443/api/v1/health`

---

## Compliance

### Standards

HiveStack security is designed to meet:

- **SOC 2 Type II**: Audit logging, access controls, encryption
- **ISO 27001**: Information security management
- **GDPR**: Data protection at rest and in transit
- **PCI DSS**: If configured with appropriate network segmentation

### Security Contacts

Report security issues to: security@hivestack.example.com

### Vulnerability Disclosure

See [VULNERABILITY_DISCLOSURE.md](./VULNERABILITY_DISCLOSURE.md) for our
responsible disclosure policy.
