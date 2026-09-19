# HiveStack

## Architecture
- Go 1.26 hypervisor management platform for KVM/libvirt
- PostgreSQL via pgx/v5, gRPC for node-agent communication
- Manager control plane + REST API + Node agents

## Key Commands
- `go build ./...` — must exit 0
- `go vet ./...` — must exit 0
- `go test ./... -race` — run tests with race detector

## Code Standards
- Follow existing patterns (table-driven tests, interface-based design)
- No stub implementations (no `// stub` comments, no `log.Printf` instead of real logic)
- Always verify build/vet after changes before committing

## Project Structure
- `internal/manager/` — Manager control plane (manager.go, ha_service.go)
- `internal/api/` — REST API server (server.go, ha_handlers.go)
- `internal/ha/` — High Availability subsystem (health, policy, scheduler, orchestrator, controller, fencer)
- `internal/db/` — Database repositories
- `internal/node/` — Node agent gRPC server
- `internal/libvirt/` — Libvirt integration
- `internal/security/` — Security hardening (AppArmor, LUKS, middleware)
- `internal/secrets/` — Secrets management (SOPS, Vault)
- `internal/tls/` — Certificate management
- `internal/events/` — Event bus
- `internal/metrics/` — Prometheus metrics
- `internal/compliance/` — HANA compliance validation

## Dependencies
- pgx/v5 for PostgreSQL
- gRPC for node agent
- Prometheus client_golang
- cobra/viper for CLI
- golang-jwt for auth
