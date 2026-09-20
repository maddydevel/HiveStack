# Contributing to HiveStack

Thank you for your interest in contributing to HiveStack! This document provides guidelines and instructions for contributing to the project.

## Code of Conduct

This project adheres to a standard code of conduct. By participating, you are expected to uphold this code:

- Be respectful and inclusive in all interactions
- Welcome newcomers and help them get started
- Focus on constructive feedback and collaboration
- Respect differing viewpoints and experiences

Harassment, trolling, and other exclusionary behavior will not be tolerated.

---

## Development Setup

### Prerequisites

| Tool | Minimum Version | Purpose |
|------|----------------|---------|
| Go | 1.26+ | Backend, CLI, Node Agent |
| Node.js | 18+ | Web UI development |
| PostgreSQL | 15+ | Database |
| Make | any | Build automation |

### Go Setup

1. Install Go 1.26 or later from [golang.org](https://golang.org/dl/)
2. Verify installation:
   ```bash
   go version
   ```
3. Clone the repository:
   ```bash
   git clone https://github.com/maddydevel/HiveStack.git
   cd HiveStack
   ```
4. Download dependencies:
   ```bash
   go mod download
   go mod verify
   ```

### Node.js Setup

1. Install Node.js 18+ (LTS recommended)
2. Install web dependencies:
   ```bash
   cd web
   npm install
   ```

### PostgreSQL Setup

1. Install PostgreSQL 15+
2. Create the development database:
   ```sql
   CREATE DATABASE hivestack;
   CREATE USER hivestack WITH PASSWORD 'hivestack';
   GRANT ALL PRIVILEGES ON DATABASE hivestack TO hivestack;
   ```
3. Set the connection string:
   ```bash
   export HIVESTACK_DSN="postgres://hivestack:hivestack@localhost:5432/hivestack?sslmode=disable"
   ```
4. Run migrations:
   ```bash
   for f in internal/db/migrations/*.sql; do
     psql -h localhost -U hivestack -d hivestack -f "$f"
   done
   ```

### Running the Full Stack

**Terminal 1 — Manager API:**
```bash
go run ./cmd/hive-manager
```

**Terminal 2 — Web UI:**
```bash
cd web
npm run dev
```

**Terminal 3 — Node Agent (optional, for HA features):**
```bash
go run ./node
```

---

## How to Contribute

### Fork and Branch Workflow

1. **Fork** the repository on GitHub
2. **Clone** your fork locally:
   ```bash
   git clone https://github.com/YOUR_USERNAME/HiveStack.git
   cd HiveStack
   ```
3. **Add upstream remote**:
   ```bash
   git remote add upstream https://github.com/maddydevel/HiveStack.git
   ```
4. **Create a feature branch** from `main`:
   ```bash
   git checkout main
   git pull upstream main
   git checkout -b feature/your-feature-name
   ```
5. **Make your changes** and commit (see Commit Message Convention below)
6. **Push** to your fork:
   ```bash
   git push origin feature/your-feature-name
   ```
7. **Open a Pull Request** against the `main` branch

### Branch Naming

| Prefix | Use Case |
|--------|----------|
| `feature/` | New features or enhancements |
| `bugfix/` | Bug fixes |
| `docs/` | Documentation changes |
| `refactor/` | Code refactoring |
| `test/` | Test additions or fixes |
| `chore/` | Build, CI, or tooling changes |

---

## Coding Standards

### Go Standards

All Go code must pass the following checks before submission:

#### Formatting
```bash
gofmt -l .          # List unformatted files
gofmt -w .          # Auto-format all files
```

#### Linting
```bash
golangci-lint run   # Run the same linter as CI
```

#### Vetting
```bash
go vet ./...        # Static analysis
```

#### Testing
```bash
go test ./... -v -race -coverprofile=coverage.out
go tool cover -func=coverage.out
```

#### Go Conventions

- **Table-driven tests**: Use table-driven tests for functions with multiple test cases:
  ```go
  func TestValidateVM(t *testing.T) {
      tests := []struct {
          name    string
          input   VMConfig
          wantErr bool
      }{
          {"valid config", VMConfig{CPUs: 4}, false},
          {"zero CPUs", VMConfig{CPUs: 0}, true},
          {"negative memory", VMConfig{Memory: -1}, true},
      }
      for _, tt := range tests {
          t.Run(tt.name, func(t *testing.T) {
              err := ValidateVM(tt.input)
              if (err != nil) != tt.wantErr {
                  t.Errorf("ValidateVM() error = %v, wantErr %v", err, tt.wantErr)
              }
          })
      }
  }
  ```

- **Error wrapping**: Use `fmt.Errorf("context: %w", err)` for error propagation
- **Interface-driven design**: Define interfaces for testability; accept interfaces, return structs
- **Context propagation**: Pass `context.Context` as the first parameter to functions that perform I/O
- **Thread safety**: Protect shared state with `sync.Mutex` or `sync.RWMutex`
- **Package naming**: Use short, lowercase, single-word package names; avoid underscores and mixed caps

### React / TypeScript Standards

#### Type Safety
- Always define explicit types for props, state, and function parameters
- Use `interface` for object shapes, `type` for unions and primitives
- Avoid `any`; use `unknown` when the type is truly dynamic

#### Component Structure
```tsx
// Functional components with explicit return type
interface HostListProps {
  hosts: Host[];
  onSelect: (id: string) => void;
}

export function HostList({ hosts, onSelect }: HostListProps): JSX.Element {
  // ...
}
```

#### Styling
- Use **Tailwind CSS** utility classes for all styling
- Use `clsx()` for conditional class composition
- Use the `cn()` utility (tailwind-merge + clsx) for merged class logic
- Follow the existing design system tokens in `index.css`

#### State Management
- Use React hooks (`useState`, `useEffect`, `useCallback`, `useMemo`) for local state
- Use `@tanstack/react-query` for server state and caching
- Keep state as close to where it's used as possible

#### File Organization
```
web/src/
├── api/           # API client and types
├── components/    # Reusable UI components
├── hooks/         # Custom React hooks
├── pages/         # Route-level page components
└── App.tsx        # Root application component
```

#### Naming Conventions
- Components: `PascalCase` (e.g., `HostList.tsx`)
- Hooks: `use` prefix (e.g., `useAuth.ts`)
- Utilities: `camelCase` (e.g., `formatBytes.ts`)
- Types/Interfaces: `PascalCase` with descriptive names

---

## Commit Message Convention

We follow the [Conventional Commits](https://www.conventionalcommits.org/) specification:

```
<type>(<optional scope>): <description>

[optional body]

[optional footer(s)]
```

### Types

| Type | Description |
|------|-------------|
| `feat` | New feature |
| `fix` | Bug fix |
| `docs` | Documentation only changes |
| `style` | Code style changes (formatting, no logic change) |
| `refactor` | Code refactoring (no feature change) |
| `perf` | Performance improvement |
| `test` | Adding or fixing tests |
| `build` | Build system or dependency changes |
| `ci` | CI/CD configuration changes |
| `chore` | Other changes (tooling, cleanup) |
| `revert` | Revert a previous commit |

### Scopes

Common scopes: `api`, `cli`, `web`, `db`, `auth`, `ha`, `compliance`, `node`, `manager`, `docs`, `ci`

### Examples

```
feat(api): add VM snapshot rollback endpoint

fix(cli): resolve panic when listing empty host table

docs(architecture): update HA subsystem diagram

test(compliance): add NUMA alignment validation tests

refactor(db): extract connection pool into separate package

ci(lint): add gosec security scanning job
```

### Rules

- Use imperative mood: "add" not "added" or "adds"
- Limit the first line to 72 characters
- Reference issues and PRs in the footer: `Closes #123`, `Refs #456`
- Separate subject from body with a blank line

---

## Pull Request Process

### Before Submitting

1. **All CI checks must pass**:
   - Build compiles without errors
   - Unit tests pass with ≥60% coverage
   - Integration tests pass
   - `golangci-lint` reports no issues
   - `gofmt` reports no unformatted files
   - Security scanning (govulncheck, gosec, Nancy) passes
   - OpenAPI spec validates successfully

2. **Run checks locally**:
   ```bash
   go build ./...
   go vet ./...
   gofmt -l .
   go test ./... -race -cover
   cd web && npm run build && npm run lint
   ```

3. **Update documentation** if your change affects:
   - API endpoints → update `api/openapi.yaml`
   - Architecture → update `docs/ARCHITECTURE.md`
   - Configuration → update relevant docs
   - New features → update `README.md`

### PR Requirements

- [ ] Branch is up to date with `main`
- [ ] All CI checks pass
- [ ] Code review approved by at least one maintainer
- [ ] No merge conflicts
- [ ] Commit messages follow conventional commits
- [ ] Tests added for new functionality
- [ ] Documentation updated as needed

### Review Process

1. Open a PR with a clear title and description
2. Fill out the PR template (if available)
3. Link related issues: `Closes #123`
4. Request review from maintainers
5. Address review feedback with additional commits
6. Maintainers will merge once approved and CI passes

### PR Title Format

Follow conventional commits for PR titles:
```
feat(ha): add VM failover orchestration
fix(api): correct pagination offset in host list
docs: add deployment guide for SLES 15 SP7
```

---

## Architecture Overview

HiveStack is a monorepo containing a Go backend (API, CLI, Node Agent) and a React web UI.

```
HiveStack/
├── cmd/                        # Application entry points
│   ├── hive/                   # CLI (cobra-based, 15+ commands)
│   └── hive-manager/           # Manager server (REST API + orchestration)
├── internal/                   # Private application code
│   ├── api/                    # REST API server (stdlib ServeMux, 100+ endpoints)
│   ├── auth/                   # JWT authentication + Argon2id + RBAC
│   ├── cli/                    # CLI command implementations
│   ├── compliance/             # SAP HANA VM guardrails enforcement
│   ├── config/                 # Configuration loading (viper)
│   ├── db/                     # Database layer (pgxpool, migrations, CRUD repos)
│   ├── events/                 # Pub/sub event framework (17 event types)
│   ├── ha/                     # High Availability subsystem
│   ├── libvirt/                # KVM/QEMU/libvirt wrapper
│   ├── manager/                # Core orchestration services
│   ├── metrics/                # Prometheus metrics
│   ├── migration/              # VMware VMX parser/converter
│   ├── node/                   # Node agent (gRPC server)
│   ├── preflight/              # VM migration compatibility checks
│   ├── secrets/                # Vault/SOPS secrets management
│   ├── security/               # TLS, AppArmor, rate limiting
│   └── tls/                    # TLS certificate management
├── pkg/                        # Public library code
│   ├── api/                    # HTTP API client
│   └── systemd/                # Systemd service files
├── web/                        # React web UI
│   ├── src/
│   │   ├── api/                # API client and TypeScript types
│   │   ├── components/         # Reusable UI components
│   │   ├── hooks/              # Custom React hooks
│   │   ├── pages/              # Route-level pages (15 pages)
│   │   ├── App.tsx             # Root application
│   │   └── main.tsx            # Entry point
│   ├── package.json            # Node.js dependencies
│   └── vite.config.ts          # Vite build configuration
├── api/                        # API specifications
│   └── openapi.yaml            # OpenAPI 3.0 spec
├── node/                       # Node Agent entry point
├── migration/                  # VMware migration tools
├── appliance/                  # SLES 15 SP7 KIWI appliance
├── dashboards/                 # Grafana dashboards
├── scripts/                    # Deployment and utility scripts
├── tests/                      # Integration and compliance tests
├── docs/                       # Project documentation
├── .github/workflows/          # CI/CD pipelines
├── go.mod                      # Go module definition
├── Makefile                    # Build targets
└── LICENSE                     # Apache 2.0
```

### Key Components

| Component | Technology | Purpose |
|-----------|------------|---------|
| Manager | Go 1.26, stdlib ServeMux | Control plane: REST API, auth, RBAC, orchestration |
| CLI | Go, cobra | Administrative CLI with table/JSON/YAML output |
| Node Agent | Go, gRPC | Compute node agent: libvirt, health, VM operations |
| Web UI | React 18, TypeScript, Vite, Tailwind | Management dashboard (15 pages) |
| Database | PostgreSQL 15, pgx/v5 | Persistent storage with connection pooling |
| Virtualization | KVM, QEMU, libvirt | VM lifecycle management |
| HA | Go, custom orchestration | Host failure detection + VM restart |
| Monitoring | Prometheus, Grafana | Metrics collection and visualization |

### Data Flow

```
┌─────────────┐     HTTP/REST      ┌─────────────────┐     SQL      ┌────────────┐
│   Web UI    │ ◄──────────────► │  Manager API    │ ◄─────────► │ PostgreSQL │
│  (React)    │                   │  (Go/stdlib)    │              │            │
└─────────────┘                   └────────┬────────┘              └────────────┘
                                           │
                                    gRPC/mTLS
                                           │
                                    ┌──────▼──────┐
                                    │ Node Agent  │
                                    │ (libvirt)   │
                                    └─────────────┘
```

---

## Getting Help

- **Documentation**: See `docs/` directory for architecture, API, and deployment guides
- **Issues**: Open a GitHub issue for bugs or feature requests
- **Discussions**: Use GitHub Discussions for questions and ideas

---

## License

By contributing to HiveStack, you agree that your contributions will be licensed under the Apache 2.0 License.
