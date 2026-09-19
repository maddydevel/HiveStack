# HiveStack — TELOS Feasibility Study

**Document Version:** 1.0
**Date:** September 19, 2026
**Status:** Draft — Phase 1 Planning
**Classification:** Confidential
**Author:** HiveStack Product & Engineering Teams

---

## 1. Introduction

This document assesses the feasibility of the HiveStack project using the **TELOS** framework — evaluating **T**echnical, **E**conomic, **L**egal, **O**perational, and **S**cheduling dimensions. The goal is to identify showstoppers, validate assumptions, and confirm that the project is viable before committing significant resources.

**Project:** HiveStack (HivePlane + HarvestCMP) — Open-source Cloud Management Platform for SUSE Rancher Harvester HCI
**Assessment Date:** September 19, 2026
**Assessment Horizon:** Phase 1 (12 weeks), with outlook to full GA (38 weeks)

---

## 2. Technical Feasibility

### 2.1 Technology Maturity

| Technology | Maturity | Suitability | Risk |
|-----------|----------|-------------|------|
| **FastAPI** (Python) | Production-ready, v0.115+ | Excellent — auto OpenAPI, async, Pydantic v2 | Low |
| **React + Vite** (TypeScript) | Production-ready, React 19 / Vite 6 | Excellent — component ecosystem, shadcn/ui | Low |
| **PostgreSQL 16** | Production-ready, battle-tested | Excellent — proven at scale, great tooling | Low |
| **Kubernetes / KubeVirt** | Production-ready (Harvester built on it) | Required — core platform primitive | Low-Medium |
| **Keycloak 25+** | Production-ready | Excellent — OIDC/SAML, LDAP, mature | Low |
| **OPA + Kyverno** | Production-ready | Good — policy-as-code, active community | Low |
| **Argo Workflows** | Production-ready | Good — K8s-native workflows, well-documented | Low |
| **Celery + Redis** | Production-ready | Excellent — proven task queue pattern | Low |
| **Prometheus + Grafana** | Industry standard | Excellent — de facto observability stack | Low |
| **Helm** | Production-ready | Excellent — standard K8s packaging | Low |
| **Kepler (energy metrics)** | Emerging (v1.0+ releases) | Good — purpose-built for K8s energy telemetry | Medium |
| **MeiliSearch** | Production-ready (v1.8+) | Good — fast full-text search, Rust-based | Low |
| **kopf (K8s operator framework)** | Production-ready | Good — Python-native operator SDK | Low |
| **qemu-img** | Production-ready, decades old | Essential for VMDK→qcow2 conversion | Low |

### 2.2 Architecture Soundness

**Strengths:**
- Clean separation: Backend (FastAPI) → K8s API (Harvester) → KubeVirt → VMs
- Async-first design throughout (FastAPI, httpx, asyncpg, Celery)
- No direct hypervisor credential storage (outbound agent architecture for HivePlane)
- GitOps-compatible (Argo + Fleet + Gitea)
- CRD-based state management (Kubernetes-native)
- Multi-tenancy via namespace isolation + OPA enforcement

**Concerns:**
- **CRD translation complexity:** Mapping CMPVirtualMachine to KubeVirt VirtualMachine spec is non-trivial (different abstractions, edge cases around hot-plug, NUMA, PCI passthrough)
- **Prometheus label dependency:** Cost attribution assumes VMs have correct labels. Label drift in Harvester causes cost data gaps.
- **Operator reliability:** The CMP operator is a single point of truth. If it fails, drift goes undetected. Need operator HA + monitoring.
- **WebSocket scaling:** Real-time event streaming to browsers at scale (5000+ VMs) needs gateway pooling.

**Mitigations:**
- Version-aware Harvester client with adapter pattern for API drift
- Automated label enforcement + backfill job
- Operator runs as Deployment with 2 replicas + leader election
- Phase 2 adds distributed gateway pooling

### 2.3 Integration Complexity

| Integration | Complexity | Status |
|------------|-----------|--------|
| Harvester REST API | Medium | Well-documented, stable |
| KubeVirt CRD API | High | Complex schema, edge cases |
| Keycloak OIDC | Low | Standard protocol |
| Prometheus metrics | Low | Standard scraping |
| Argo Workflows | Medium | CRD-based, needs testing |
| Longhorn backups | Medium | API available, needs integration |
| Kube-OVN networking | Medium | Network policy APIs |
| SMTP/email | Low | Standard |
| Slack/Teams webhooks | Low | Standard |
| vCenter REST API (Phase 3) | High | VMware API complexity, version differences |

### 2.4 Technical Feasibility Verdict

✅ **FEASIBLE** — All core technologies are mature and production-ready. The primary technical risk (CRD translation) is manageable with proper testing patterns. The architecture is sound and follows cloud-native best practices.

**Technical Risk Score: 3/10** (Low-Medium)

---

## 3. Economic Feasibility

### 3.1 Cost Analysis

#### Phase 1 Costs (12 weeks)

| Category | Low Estimate | High Estimate |
|----------|-------------|---------------|
| Personnel (6.5 FTE × 12 weeks) | $156,000 | $234,000 |
| Infrastructure (dev/staging) | $600 | $1,200 |
| Tools & Licenses | $0 | $500 |
| Contingency (15%) | $23,490 | $35,355 |
| **Total Phase 1** | **$180,090** | **$271,055** |

#### Full GA Costs (38 weeks)

| Category | Estimate |
|----------|---------|
| Personnel (escalating team size) | $700,000 - $1,000,000 |
| Infrastructure | $5,000 - $10,000 |
| Community building (events, swag, hosting) | $20,000 - $50,000 |
| **Total GA** | **$725,000 - $1,060,000** |

### 3.2 Revenue Projections

#### Year 1 (GA + 6 months)

| Stream | Customers | ARR Each | Total ARR |
|--------|-----------|----------|-----------|
| Enterprise licenses | 5 | $25,000 | $125,000 |
| Professional services (migration) | 10 | $15,000 | $150,000 |
| Support subscriptions | 15 | $10,000 | $150,000 |
| **Total Year 1 Revenue** | | | **$425,000** |

#### Year 2

| Stream | Customers | ARR Each | Total ARR |
|--------|-----------|----------|-----------|
| Enterprise licenses | 20 | $25,000 | $500,000 |
| Professional services | 30 | $15,000 | $450,000 |
| Support subscriptions | 50 | $10,000 | $500,000 |
| **Total Year 2 Revenue** | | | **$1,450,000** |

### 3.3 Break-Even Analysis

| Metric | Value |
|--------|-------|
| Cumulative investment (GA + 12 months) | $1,200,000 |
| Annual revenue at break-even | $1,200,000 |
| Break-even point | **Month 28-32** (Year 2 H2 - Year 3 H1) |
| ROI at end of Year 2 | 21% |

### 3.4 Economic Feasibility Verdict

✅ **FEASIBLE** — Conservative revenue projections show break-even by Year 2 H2. The open-source Community edition drives adoption, converting to Enterprise via VMware migration toolchain and advanced features. Revenue model is validated by comparable open-source companies (GitHash, Vagrant, Terraform pre-BSL).

**Economic Risk Score: 4/10** (Medium)

---

## 4. Legal Feasibility

### 4.1 Licensing Strategy

| Component | License | Compatibility |
|-----------|---------|---------------|
| Core CMP (HarvestCMP) | Apache 2.0 | ✅ All deps compatible |
| Enterprise features | BSL (→ Apache 2.0 after 4yr) | ✅ Permissive, converts to open |
| Frontend (React) | MIT (React), Apache 2.0 (shadcn) | ✅ Compatible |
| OPA policies | Apache 2.0 | ✅ Compatible |
| Kyverno policies | Apache 2.0 | ✅ Compatible |
| Argo Workflows YAML | Apache 2.0 | ✅ Compatible |

### 4.2 License Compatibility Matrix

| Dependency | License | Compatible with Apache 2.0? |
|-----------|---------|----------------------------|
| FastAPI | MIT | ✅ |
| React | MIT | ✅ |
| PostgreSQL | PostgreSQL License (MIT-like) | ✅ |
| Keycloak | Apache 2.0 | ✅ |
| OPA | Apache 2.0 | ✅ |
| Kyverno | Apache 2.0 | ✅ |
| Argo Workflows | Apache 2.0 | ✅ |
| Celery | BSD 3-Clause | ✅ |
| Redis | BSD 3-Clause (RSAL for modules, but core is BSD) | ✅ |
| MeiliSearch | MIT | ✅ |
| kopf | MIT | ✅ |
| qemu-img | GPL v2+ | ⚠️ System tool, not linked — OK |
| Prometheus | Apache 2.0 | ✅ |
| Grafana | AGPL v3 | ⚠️ Embedded iframe, not modified — OK |

### 4.3 Intellectual Property

| Item | Status | Notes |
|------|--------|-------|
| Product name "HiveStack" | Needs trademark search | No known conflicts in IT infra space |
| Product name "HivePlane" | Needs trademark search | No known conflicts |
| Product name "HarvestCMP" | Proximity to "Harvester" (SUSE trademark) | Should rename to avoid confusion |
| Code originality | Clean room | All code original, no vendor code copied |
| VMware API usage | Public REST API | No proprietary SDK needed (avoid VDDK) |

### 4.4 Regulatory Compliance

| Regulation | Requirement | HiveStack Readiness |
|-----------|-------------|---------------------|
| **GDPR** (EU) | Data minimization, right to erasure, audit logs | PostgreSQL audit log immutable by design. Need data export/deletion API for PII. Phase 2. |
| **CSRD** (EU) | Carbon/environmental reporting | Kepler integration + carbon dashboard (Phase 1). ✅ Ready. |
| **SOC 2** | Security controls, audit trail | Audit logging in Phase 1. SOC 2 certification is a process, not a feature. Plan for Year 2. |
| **ISO 27001** | InfoSec management system | Organizational certification. Year 2+ goal. |

### 4.5 Legal Feasibility Verdict

✅ **FEASIBLE** — All dependencies are permissively licensed. No GPL contamination risk. BSL → Apache 2.0 model is proven (HashiCorp, Sentry, CockroachDB). One recommendation: **rename "HarvestCMP"** to avoid trademark proximity with SUSE's "Harvester" product.

**Legal Risk Score: 2/10** (Low)

---

## 5. Operational Feasibility

### 5.1 Deployment Model

| Aspect | Approach | Feasibility |
|--------|----------|-------------|
| **Target environment** | RKE2 cluster on Harvester VMs (self-hosted) | ✅ Standard deployment |
| **Packaging** | Helm chart + Docker Compose (dev) | ✅ Well-understood |
| **Upgrade** | Rolling deployment via Helm | ✅ Zero-downtime with PDB |
| **Scaling** | HPA for API/frontend, worker pool for Celery | ✅ K8s-native |
| **Backup** | PostgreSQL pg_dump + Longhorn snapshots | ✅ Automated |
| **Monitoring** | Prometheus + Grafana (self-hosted) | ✅ Built-in |

### 5.2 Operational Complexity

| Operation | Complexity | Automation |
|-----------|-----------|------------|
| CMP deployment | Low (Helm install) | ✅ CI/CD pipeline |
| Harvester cluster registration | Medium (kubeconfig, validation) | ✅ Automated in UI |
| CMP upgrade | Medium (DB migrations + rolling deploy) | ✅ Helm hooks + Alembic |
| Backup & DR | Medium | ✅ Longhorn snapshots + pg_dump |
| User onboarding | Low (SSO sync or manual) | ✅ Keycloak group sync |
| Incident response | Medium | Needs runbooks (Phase 2) |
| Certificate rotation | Low | ✅ cert-manager handles TLS |
| Secret rotation | Medium | Vault integration (Phase 2) |

### 5.3 Support Model

| Tier | What | Who | Channel |
|------|------|-----|---------|
| Self-service | Docs, FAQs, community | Users | MkDocs, Discord, GitHub Discussions |
| Community | Best-effort help | Community | GitHub Issues, Discord |
| Enterprise | SLA-backed support | HiveStack team | Email, phone, dedicated Slack |
| Professional Services | Migration, custom integration | HiveStack team | Statement of Work |

### 5.4 Operational Feasibility Verdict

✅ **FEASIBLE** — Standard cloud-native deployment model. All operational concerns have established K8s patterns. Primary gap is incident response runbooks (acceptable for Phase 1 MVP, must address before GA).

**Operational Risk Score: 3/10** (Low-Medium)

---

## 6. Scheduling Feasibility

### 6.1 Timeline Assessment

| Phase | Duration | Confidence |
|-------|----------|------------|
| Phase 0 (Foundation) | 3 weeks | High ✅ |
| Phase 1 (Self-Service MVP) | 8 weeks | Medium ⚠️ |
| Phase 2 (Governance & FinOps) | 10 weeks | Medium ⚠️ |
| Phase 3 (Intelligence & Migration) | 12 weeks | Low-Medium ⚠️ |

### 6.2 Schedule Risk Factors

| Factor | Impact | Mitigation |
|--------|--------|-----------|
| **Team ramp-up** (K8s/Harvester expertise) | 1-2 weeks delay | Start with experienced engineers; pair programming |
| **Harvester API instability** | 1-3 weeks delay | Version adapter pattern, e2e tests against target version |
| **Frontend complexity** (dynamic forms, real-time UI) | 1-2 weeks delay | shadcn/ui + react-hook-form accelerate; scope UI features carefully |
| **Integration testing** (multi-service) | 1-2 weeks delay | Invest in CI pipeline early; contract tests |
| **Scope creep** | Ongoing risk | Strict phase gates; "parking lot" for out-of-scope |

### 6.3 Critical Path

```
Harvester Cluster Access (W0)
    → FastAPI Scaffold (W0-1)
        → Harvester Client Library (W0-2)
            → Resource Abstraction API (W4-5)
                → CRD + Operator (W4-5)
                    → Service Catalog (W7-8)
                        → Self-Service UI (W8-9)
                            → Approval Workflow (W9-10)
                                → Cost Attribution (W10-11)
                                    → Phase 1 GA (W12)
```

The critical path runs through the backend API → CRD → catalog → cost attribution chain. The frontend can be partially parallelized (starts W5-6).

### 6.4 Scheduling Feasibility Verdict

⚠️ **FEASIBLE WITH RISK** — The 38-week timeline to GA is achievable but tight. Phase 1 (8 weeks for MVP) is the most aggressive phase and carries the highest risk of slippage. Recommend adding 2 weeks of buffer to Phase 1 (10 weeks total) to account for integration testing and unforeseen complexity.

**Schedule Risk Score: 5/10** (Medium)

---

## 7. Overall TELOS Summary

| Dimension | Score | Verdict | Key Risk |
|-----------|-------|---------|----------|
| **Technical** | 3/10 | ✅ Feasible | CRD translation complexity |
| **Economic** | 4/10 | ✅ Feasible | Revenue timing (break-even Year 2-3) |
| **Legal** | 2/10 | ✅ Feasible | Trademark proximity (rename HarvestCMP) |
| **Operational** | 3/10 | ✅ Feasible | Incident response runbooks needed |
| **Scheduling** | 5/10 | ⚠️ Feasible with risk | Phase 1 aggressive; add buffer |
| **OVERALL** | **3.4/10** | ✅ **FEASIBLE** | **Tight schedule is the primary concern** |

---

## 8. Recommendations

### 8.1 Go/No-Go Decision

**RECOMMENDATION: GO** — HiveStack is feasible across all TELOS dimensions. Proceed with Phase 1 execution.

### 8.2 Pre-Phase-1 Actions

| Action | Owner | Deadline |
|--------|-------|----------|
| Rename "HarvestCMP" to avoid SUSE trademark proximity | Product | Before Phase 1 kickoff |
| Add 2 weeks buffer to Phase 1 timeline (10 weeks total) | PM | Before Phase 1 kickoff |
| Secure Harvester development cluster access (3+ nodes) | Engineering Lead | Before W0 |
| Complete trademark search for "HiveStack" / "HivePlane" | Legal | Before public launch |
| Document incident response runbook template | DevOps | Before GA |

### 8.3 Phase 1 Success Criteria (Exit Gates)

| Criterion | Must Have | Nice to Have |
|-----------|-----------|--------------|
| Developer can provision VM via self-service | ✅ | |
| Cost data visible in showback dashboard | ✅ | |
| RBAC enforced at operation level | ✅ | |
| Audit log captures all writes | ✅ | |
| Zero cross-tenant data leakage | ✅ | |
| API response < 500ms at 1000 VMs | | ✅ |
| WebSocket real-time updates | | ✅ |

---

## 9. Appendices

### A. TELOS Scoring Methodology

Scores are on a 1-10 scale where:
- 1-3: Low risk, well-understood, proven
- 4-6: Medium risk, manageable with mitigations
- 7-9: High risk, significant concerns, may block project
- 10: Project killer, not feasible

### B. Assumptions

1. Harvester v1.7.x API remains stable through Phase 1
2. Team has or can acquire K8s/Harvester expertise
3. Development Harvester cluster is available
4. Open-source community interest exists
5. SUSE continues investment in Harvester/Rancher ecosystem
6. VMware migration market continues to grow

### C. References

- HarvestCMP PRD v1.0 (April 2026)
- HivePlane Feature Matrix (2026)
- HivePlane SUSE Summit Keynote (August 2026)
- Gartner IT Leader Sentiment Survey (January 2026)
- SUSE Rancher Harvester Documentation (v1.7)

---

*This TELOS Feasibility Study is a living document. It should be updated at each phase gate and whenever significant technical, market, or organizational changes occur.*
