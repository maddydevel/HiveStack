# HiveStack — Project Charter

**Document Version:** 1.0
**Date:** September 19, 2026
**Status:** Draft — Phase 1 Planning & Feasibility
**Classification:** Confidential
**Author:** HiveStack Product & Engineering Teams

---

## 1. Executive Summary

HiveStack (comprising **HivePlane** and **HarvestCMP**) is an open-source Cloud Management Platform (CMP) purpose-built for enterprise SUSE Rancher Harvester HCI environments. It delivers the missing business-level abstraction layer above Harvester's Kubernetes-native infrastructure — enabling self-service provisioning, FinOps cost attribution, policy governance, GreenOps sustainability tracking, and automated Day-2 operations.

The VMware pricing crisis (Broadcom acquisition) has created a once-in-a-decade market opportunity. **76% of IT leaders** no longer trust their incumbent virtualization vendor. HiveStack positions itself as the **free, open-source migration destination** from VMware, turning a massive exodus into long-term platform adoption.

---

## 2. Business Case

### 2.1 Problem Statement

Organizations migrating from VMware to SUSE Rancher Harvester face critical gaps:

| Gap | Impact |
|-----|--------|
| No business-level API abstraction | Teams must hand-write KubeVirt YAML; high barrier to entry |
| Zero cost attribution or FinOps | Finance cannot track per-project/team spend; budget overruns |
| Namespace-scoped RBAC only | Cannot restrict "create" vs "delete" per resource type; audit risk |
| No self-service portal | IT remains bottleneck for every VM request; slow time-to-value |
| No Day-2 automation | Manual patching, decommissioning, rightsizing — operational toil |
| No VMware migration tooling | Migration planning and execution remain manual, error-prone |
| No sustainability tracking | CSRD and upcoming regulations demand carbon reporting |

### 2.2 Solution

HiveStack provides:

- **Resource Abstraction API** — Business-level intent translated to KubeVirt/Harvester calls
- **Self-Service Portal** — Service catalog with approval workflows, cost estimates, and one-click provisioning
- **FinOps Engine** — Real-time cost attribution, showback/chargeback, budget enforcement, idle detection
- **Policy Engine** — OPA + Kyverno governance, quota enforcement, compliance scoring
- **GreenOps** — Kepler energy telemetry → CO2 equivalent reporting
- **VMware Migration Toolchain** — Discovery, assessment, wave planning, VMDK→qcow2 conversion, cutover
- **Workflow Automation** — Argo Workflows for Day-2 operations (provision, decommission, patch, migrate)

### 2.3 Market Opportunity

| Factor | Data Point |
|--------|-----------|
| VMware distrust | 76% of IT leaders (Gartner, Jan 2026) |
| SAP HANA on SLES KVM | Certified enterprise workload; growing adoption |
| VMware → open-source migration | Massive, accelerating wave driven by Broadcom pricing |
| Sustainability regulation | CSRD (EU) and successors require carbon reporting by 2026-2027 |
| Target segments | Enterprise IT, SAP hosting providers, MSPs, regulated industries |

### 2.4 Revenue Model

| Tier | License | Features | Target |
|------|---------|----------|--------|
| **Community** | Apache 2.0 | Core CMP, self-service, FinOps, policy, GitOps | Adoption driver, VMware migration funnel |
| **Enterprise** | BSL → Apache 2.0 (4yr) | VMware migration toolchain, AI modules, enterprise SAML/SSO, Rancher UI extension | Revenue from enterprises with 500+ VMs |
| **Managed Service** | Subscription | Multi-tenant SaaS, white-label for MSPs | Recurring revenue from hosting providers |

---

## 3. Goals & Success Metrics

### 3.1 Strategic Goals

| # | Goal | Success Metric |
|---|------|---------------|
| G1 | Become the de facto open-source CMP for Harvester | 1000+ GitHub stars, 50+ contributors by GA |
| G2 | Capture VMware migration market | 100+ organizations using migration toolchain in Year 1 |
| G3 | Achieve product-market fit for self-service | NPS > 40 from developer personas |
| G4 | Build sustainable open-source community | Monthly release cadence, active Discord/Slack |
| G5 | Establish enterprise revenue stream | 20+ Enterprise customers by end of Year 2 |

### 3.2 Phase 1 MVP Goals (Weeks 1-12)

| Goal | Metric |
|------|--------|
| Developer self-service VM provisioning | < 5 min from request to running VM |
| Cost visibility | Showback dashboard with < 15 min data lag |
| RBAC enforcement | Operation-level permissions working across all APIs |
| Audit compliance | Immutable audit log capturing 100% of write operations |
| Multi-tenancy | Zero cross-tenant data leakage (verified by automated tests) |

---

## 4. Scope

### 4.1 In Scope (Phase 1 MVP)

- Monorepo scaffold (FastAPI backend, React frontend, Helm charts, OPA policies, Argo workflows)
- Harvester API async client library
- Keycloak OIDC authentication
- CMPVirtualMachine CRD + Kubernetes operator
- Size class, OS profile, network profile, storage profile systems
- VM lifecycle states API (pending → provisioning → running → ... → deleted)
- Label & tag enforcement
- Service catalog backend + UI (browse, search, filter)
- Dynamic request forms (JSON Schema → react-hook-form)
- Multi-tier approval workflow (email + in-app notifications)
- Cost attribution engine (Prometheus scraper, hourly cost calc, showback API)
- FinOps dashboard UI (Grafana-embedded)
- My Resources dashboard
- VM console access (virtctl proxy)
- Resource expiry & renewal flow
- Audit log viewer
- Team & RBAC system (7 built-in roles, OPA middleware)
- Namespace-to-team sync
- Cluster registry + global inventory
- Cluster health dashboard

### 4.2 Out of Scope (Phase 1)

- VMware migration toolchain (Phase 3)
- AI/ML intelligence features (Phase 3)
- Custom workflow builder UI (Phase 3)
- Rancher UI extension (Phase 3)
- Reserved capacity pricing (Phase 3)
- VM→Container advisor (Phase 3)
- Blueprints / multi-VM stacks (Phase 2)
- Batch operations API (Phase 2)
- ServiceNow CMDB sync (Phase 2)
- Terraform provider (Phase 2)
- Cross-cluster policy federation (Phase 2)

---

## 5. Timeline & Milestones

### 5.1 High-Level Roadmap

| Phase | Name | Duration | Target Completion |
|-------|------|----------|-------------------|
| **Phase 0** | Foundation & Infrastructure | Weeks 1-3 | October 10, 2026 |
| **Phase 1** | Core Self-Service MVP | Weeks 4-12 | December 5, 2026 |
| **Phase 2** | Governance, Automation & FinOps | Weeks 13-24 | March 6, 2027 |
| **Phase 3** | Intelligence, VMware Migration & Advanced | Weeks 25-38 | June 12, 2027 |

### 5.2 Phase 1 Detailed Timeline

| Week | Dates | Deliverables | Features |
|------|-------|-------------|----------|
| W1-3 | Sep 19 - Oct 10 | Foundation complete, CI green, dev env working | F0.1-F0.6 |
| W4-5 | Oct 11 - Oct 24 | Resource Abstraction API + VM CRUD | F1.1-F1.5, F1.8, F1.9 |
| W5-6 | Oct 18 - Oct 31 | VM List + Detail UI, admin dashboard | F7.1, F7.2, F7.6 |
| W6-7 | Oct 25 - Nov 7 | Team & RBAC system | F3.1-F3.6, F5.8 |
| W7-8 | Nov 1 - Nov 14 | Service catalog backend | F2.1-F2.5 |
| W8-9 | Nov 8 - Nov 21 | Self-service portal UI | F2.1, F2.3 |
| W9-10 | Nov 15 - Nov 28 | Approval workflow + notifications | F2.4, F2.11, F11.1 |
| W10-11 | Nov 22 - Dec 5 | Cost Attribution Engine v1 | F4.1-F4.3, F4.6 |
| W11-12 | Nov 29 - Dec 12 | FinOps dashboard, My Resources, console, expiry, audit | F2.6-F2.8, F4.3, F5.10 |

### 5.3 Key Milestones

| Milestone | Date | Gate Criteria |
|-----------|------|--------------|
| M0: Foundation Complete | Oct 10 | CI green, dev env one-command, auth working |
| M1: API Complete | Oct 24 | All P1 backend APIs tested, OpenAPI docs generated |
| M2: UI Complete | Nov 21 | Self-service portal usable end-to-end |
| M3: Cost Live | Dec 5 | Showback dashboard showing real cost data |
| **M4: Phase 1 GA** | **Dec 12** | **MVP feature-complete, deployed to staging** |

---

## 6. Budget & Resources

### 6.1 Team Composition (Phase 1)

| Role | Count | Allocation |
|------|-------|-----------|
| Engineering Lead / Architect | 1 | 100% |
| Backend Engineer (FastAPI/K8s) | 2 | 100% |
| Frontend Engineer (React) | 2 | 100% |
| DevOps / Platform Engineer | 1 | 50% |
| Product Manager | 1 | 50% |
| QA Engineer | 1 | 50% |
| **Total** | **8** | **6.5 FTE** |

### 6.2 Infrastructure Costs (Phase 1)

| Item | Monthly Cost | Notes |
|------|-------------|-------|
| Development Harvester cluster (3 nodes) | $0 | Self-hosted on existing hardware |
| CI/CD (Gitea + Woodpecker) | $0 | Self-hosted |
| Staging environment | $0 | Runs on Harvester |
| Cloud resources (testing) | ~$200 | Occasional cloud VMs for compatibility testing |
| **Total Monthly** | **~$200** | |

### 6.3 Total Phase 1 Budget Estimate

| Category | Estimate |
|----------|---------|
| Personnel (12 weeks × 6.5 FTE) | $156,000 - $234,000 |
| Infrastructure | $600 |
| Tools & Licenses | $0 (all open-source) |
| **Total** | **$157,000 - $235,000** |

---

## 7. Governance & Decision-Making

### 7.1 Decision Authority

| Decision Type | Authority |
|--------------|----------|
| Feature scope changes | Product Manager + Engineering Lead |
| Architecture decisions | Engineering Lead (with team input) |
| Release go/no-go | Product Manager + Engineering Lead |
| Budget reallocation | Executive Sponsor |
| Community policy | Open-source maintainers (post-launch) |

### 7.2 Communication Cadence

| Meeting | Frequency | Attendees |
|---------|-----------|----------|
| Daily standup | Daily | Engineering team |
| Sprint planning | Bi-weekly | Full team |
| Stakeholder update | Monthly | PM + Exec Sponsor |
| Community update | Monthly (post-launch) | Public |

---

## 8. Assumptions & Constraints

### 8.1 Assumptions

1. Harvester API remains stable (v1.7.x) through Phase 1
2. SUSE continues investment in Harvester/Rancher platform
3. Keycloak OIDC integration pattern is well-understood by team
4. Prometheus metrics from Harvester are sufficient for cost attribution
5. Team has access to a Harvester cluster for development and testing
6. Open-source community interest exists for a Harvester-focused CMP

### 8.2 Constraints

1. Phase 1 must deliver standalone value (no dependency on Phase 2/3)
2. All dependencies must be Apache 2.0-compatible (no GPL contamination)
3. VMware VDDK cannot be used in open-source build (licensing)
4. Must support air-gapped / on-premises deployments (no cloud dependency)
5. API must remain backward-compatible within a phase

---

## 9. Dependencies

| Dependency | Type | Risk if Unavailable |
|-----------|------|---------------------|
| SUSE Rancher Harvester | Platform | Cannot build product without it |
| KubeVirt | Technology | Core VM management primitive |
| Keycloak | Technology | Auth would need replacement (Authelia, Ory) |
| Prometheus | Technology | Cost attribution would need alternative metrics source |
| Argo Workflows | Technology | Would need alternative workflow engine (Temporal, Cadence) |
| Longhorn | Technology | Backup/DR features would need different storage backend |

---

## 10. Approval

| Role | Name | Signature | Date |
|------|------|-----------|------|
| Executive Sponsor | | | |
| Product Manager | | | |
| Engineering Lead | | | |
| Finance Approver | | | |

---

*This Project Charter is a living document. It will be reviewed and updated at the end of each phase gate. All changes require approval from the Executive Sponsor and Engineering Lead.*
