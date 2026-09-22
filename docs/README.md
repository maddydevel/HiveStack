# HiveStack Documentation

HiveStack is a Go-based KVM/libvirt hypervisor management platform: a Manager
control plane (REST API + PostgreSQL), Node Agents on each hypervisor host
(gRPC, mTLS), and a CLI/Web UI on top. This index points to every document in
`docs/`, grouped by audience.

## Start here

| Audience | Document |
|----------|----------|
| Operators standing up or running HiveStack | **[ADMIN.md](ADMIN.md)** — Administrator Guide |
| People creating VMs, managing hosts, running backups | **[USER.md](USER.md)** — User Guide |
| Anyone debugging a failure | **[TROUBLESHOOTING.md](TROUBLESHOOTING.md)** — Troubleshooting Guide |
| Release engineers cutting a version | **[RELEASE.md](RELEASE.md)** — Release Management |

## Architecture & design

| Document | Contents |
|----------|----------|
| [ARCHITECTURE.md](ARCHITECTURE.md) | System overview: Manager, Node Agent, HA, migration |
| [architecture.md](architecture.md) | Shorter architecture tour with links into HLD/LLD/HA |
| [HLD.md](HLD.md) | High-level design — control plane, data flow |
| [LLD.md](LLD.md) | Low-level design — package structure, internal APIs |
| [HA.md](HA.md) | High Availability subsystem — health, fencing, failover, scheduler |
| [adr/](adr/) | Architecture Decision Records |

## API reference

| Document | Contents |
|----------|----------|
| [api.md](api.md) | REST API reference with request/response examples |
| [api-reference.md](api-reference.md) | Summary pointing at the OpenAPI contract (`api/openapi.yaml`) |

## Operations

| Document | Contents |
|----------|----------|
| [deployment.md](deployment.md) | Source/binary install, systemd units, config file reference |
| [ADMIN.md](ADMIN.md) | Full admin lifecycle: install, configure, secure, back up, monitor |
| [operations/](operations/) | SLO capacity planning, audit log retention, tracing |
| [RUNBOOKS.md](RUNBOOKS.md) | Step-by-step operational runbooks |
| [MAINTENANCE.md](MAINTENANCE.md) | Maintenance schedule and technical debt tracking |
| [INCIDENT_RESPONSE.md](INCIDENT_RESPONSE.md) | Incident response procedures |
| [TROUBLESHOOTING.md](TROUBLESHOOTING.md) | Symptom → cause → fix reference |

## Security & compliance

| Document | Contents |
|----------|----------|
| [SECURITY.md](SECURITY.md) | Auth, secrets, network security, fencing security notes |
| [THREAT_MODEL.md](THREAT_MODEL.md) | Threat model and scope |
| Compliance | HANA guardrail enforcement — see [USER.md](USER.md#compliance-validation-hana-guardrails) and `internal/compliance/` |

## Product & planning

| Document | Contents |
|----------|----------|
| [BRD.md](BRD.md) | Business Requirements Document |
| [SRS.md](SRS.md) | Software Requirements Specification |
| [RTM.md](RTM.md) | Requirements Traceability Matrix |
| [PROJECT_CHARTER.md](PROJECT_CHARTER.md) | Project charter |
| [FEASIBILITY_STUDY.md](FEASIBILITY_STUDY.md) | TELOS feasibility study |
| [RISK_REGISTER.md](RISK_REGISTER.md) | Risk register |
| [SLO_SLA.md](SLO_SLA.md) | Service level objectives and agreements |

## Release management

| Document | Contents |
|----------|----------|
| [RELEASE.md](RELEASE.md) | Versioning, tagging, changelog, artifacts, deployment strategies, rollback, support matrix |

## Contributing to docs

- All documentation lives under `docs/` in GitHub-flavored Markdown.
- Match the existing tone: concise, table-driven where possible, runnable
  code blocks over prose.
- New operational facts (ports, config keys, CLI flags, API routes) must be
  verified against the source (`internal/`, `cmd/`, `api/openapi.yaml`) —
  don't copy from memory or from another doc without checking it still
  matches the code.
- Cross-link instead of duplicating: if a fact already lives in
  [ARCHITECTURE.md](ARCHITECTURE.md), [HA.md](HA.md), [api.md](api.md), etc.,
  link to it rather than restating it in a new document.
- Keep [RELEASE.md](RELEASE.md)'s changelog conventions aligned with
  [Keep a Changelog](https://keepachangelog.com/en/1.1.0/) and
  [Semantic Versioning](https://semver.org/).
- When you add a new top-level document, add a row to this index in the
  right section.
- Diagrams are plain ASCII (see [HA.md](HA.md) for examples) — no binary
  image formats, so diffs stay reviewable.
