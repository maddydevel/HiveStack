# HiveStack Security Guide

## Authentication & Authorization

### vCenter Credentials
- vCenter credentials are passed via API request body (JSON)
- **Current state**: Credentials transmitted in plaintext in request body
- **Production requirement**: Use TLS 1.3 for all API endpoints
- **Production requirement**: Integrate with enterprise secret management (HashiCorp Vault, Kubernetes Secrets)

### API Security
- REST API currently has no authentication middleware
- **Production requirement**: Add JWT or mTLS authentication
- **Production requirement**: Implement RBAC for migration and HA operations

## Data Protection

### Secrets in Transit
- All API endpoints should require HTTPS
- vCenter `Insecure` flag controls TLS verification (default: false for production)
- Redfish fencing uses `-k` (insecure) in current implementation — replace with proper TLS in production

### Secrets at Rest
- IPMI passwords stored in FenceConfig structs (memory only)
- vCenter passwords marked `json:"-"` to prevent serialization
- **Production requirement**: Encrypt all credentials at rest

## Network Security

### API Endpoints
| Endpoint | Method | Auth Required |
|----------|--------|---------------|
| `/api/v1/migration/import/ovf` | POST | Yes |
| `/api/v1/migration/import/ova` | POST | Yes |
| `/api/v1/migration/jobs` | GET | Yes |
| `/api/v1/migration/jobs/:id` | GET/PUT/DELETE | Yes |
| `/api/v1/migration/vcenter/discover` | POST | Yes |
| `/api/v1/migration/vcenter/import` | POST | Yes |
| `/api/v1/migration/vcenter/jobs` | GET | Yes |
| `/api/v1/migration/vcenter/jobs/:id` | GET/DELETE | Yes |
| `/api/v1/migration/vcenter/jobs/progress/:id` | GET | Yes |

### Input Validation
- OVF file upload: validates file extension (.ovf, .ova)
- OVF size limits: 500MB for OVF, 2GB for OVA
- JSON request bodies validated for required fields
- **Production requirement**: Add file content-type validation (not just extension)
- **Production requirement**: Add rate limiting on upload endpoints

## Fencing Security

### IPMI
- Uses IPMI v2.0+ (lanplus interface)
- Credentials passed as command-line arguments to ipmitool
- **Security note**: Command-line arguments visible in /proc — consider using environment variables or config files

### Redfish
- Uses HTTPS with basic auth
- Current implementation uses `-k` flag (skip TLS verification) — **must be removed in production**
- **Production requirement**: Use TLS client certificates for Redfish authentication

### SSH
- Uses BatchMode=yes and StrictHostKeyChecking=no
- **Security note**: StrictHostKeyChecking=no is vulnerable to MITM — use known_hosts in production
- SSH key-based authentication supported via SSHKeyPath

## Supply Chain Security

### Dependencies
- Minimal external dependencies (stdlib-first approach)
- Production vCenter integration will add `github.com/vmware/govmomi`
- **Production requirement**: Pin all dependency versions, use Go checksum database
- **Production requirement**: Run `govulncheck` in CI/CD pipeline

## Audit & Logging

### Current Logging
- All operations logged with component prefix (e.g., `[HA/Controller]`, `[Migration]`)
- Failover events published via EventPublisher interface
- Policy changes recorded via PolicyHistoryStore interface

### Production Requirements
- Structured logging (JSON) for SIEM integration
- Audit log for all state-changing operations
- Log retention policy (minimum 90 days)
