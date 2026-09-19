# HiveStack — Risk Register

**Document Version:** 1.0
**Date:** September 19, 2026
**Status:** Draft — Phase 1 Planning
**Classification:** Confidential
**Author:** HiveStack Product & Engineering Teams

---

## 1. Overview

This Risk Register identifies and assesses all known risks to the HiveStack project (Phase 1 through GA). Risks are scored on **Likelihood** (1-5) and **Impact** (1-5) to produce a **Severity Score** (1-25). Risks scoring ≥12 are considered **Critical** and require immediate mitigation plans. Risks scoring 8-11 are **High** and require active monitoring. Risks scoring ≤7 are **Medium** and are tracked.

### Risk Scoring Matrix

| Score | Severity | Action |
|-------|----------|--------|
| 20-25 | 🔴 Critical | Immediate mitigation; escalate to Executive Sponsor |
| 12-19 | 🟠 High | Active mitigation plan; weekly monitoring |
| 8-11 | 🟡 Medium | Mitigation plan; bi-weekly monitoring |
| 1-7 | 🟢 Low | Monitor; address if escalates |

### Likelihood Scale

| Score | Label | Meaning |
|-------|-------|---------|
| 1 | Rare | < 10% probability |
| 2 | Unlikely | 10-25% probability |
| 3 | Possible | 25-50% probability |
| 4 | Likely | 50-75% probability |
| 5 | Almost Certain | > 75% probability |

### Impact Scale

| Score | Label | Meaning |
|-------|-------|---------|
| 1 | Negligible | No noticeable impact on timeline, budget, or quality |
| 2 | Minor | < 2 week delay, < 5% budget overrun |
| 3 | Moderate | 2-4 week delay, 5-15% budget overrun |
| 4 | Major | 1-2 month delay, 15-30% budget overrun |
| 5 | Catastrophic | Project cancellation or > 30% budget overrun |

---

## 2. Technical Risks

### T01: Harvester/KubeVirt API Breaking Changes

| Field | Value |
|-------|-------|
| **Category** | Technical — Dependency |
| **Severity** | **16** (🟠 High) |
| **Likelihood** | 3 (Possible) |
| **Impact** | 4 (Major) |
| **Phase** | All |

**Description:** SUSE releases a new Harvester version that changes KubeVirt CRD schemas, deprecates endpoints, or modifies API behavior. This breaks the Harvester client library, CRD translation, and potentially the operator.

**Indicators:**
- Harvester release notes mention breaking API changes
- New CRD version (v1alpha2 → v1beta1) in release
- Deprecation warnings in API responses

**Mitigation:**
1. Implement version-aware Harvester client with adapter pattern
2. Pin Harvester version per CMP release; document compatibility matrix
3. Run e2e tests against each Harvester version in CI matrix
4. Subscribe to Harvester release announcements and beta programs
5. Maintain abstraction layer that isolates Harvester-specific code

**Contingency:**
- If breaking change detected, pause new feature work for 1-2 week compatibility sprint
- Fall back to previous Harvester version if critical

**Owner:** Engineering Lead
**Status:** Open

---

### T02: CRD Translation Complexity Leads to Brittle Abstraction

| Field | Value |
|-------|-------|
| **Category** | Technical — Architecture |
| **Severity** | **15** (🟠 High) |
| **Likelihood** | 3 (Possible) |
| **Impact** | 5 (Catastrophic) |
| **Phase** | 1-2 |

**Description:** Mapping CMPVirtualMachine CRD to KubeVirt VirtualMachine spec involves edge cases (NUMA pinning, PCI passthrough, hot-plug, CPU pinning) that are difficult to abstract cleanly. Brittle translation leads to data loss, VM corruption, or operator panics.

**Indicators:**
- Increasing bug reports around VM creation/edge cases
- Operator crash loops on specific VM configurations
- Test coverage gaps in CRD translation layer

**Mitigation:**
1. Comprehensive unit tests for every CRD translation path (target: 90%+ coverage)
2. Property-based testing for CRD generation (hypothesis library)
3. Explicit compatibility matrix documenting supported features
4. Schema validation at both CMP and KubeVirt layers
5. Graceful degradation: unsupported features return clear error, not crash

**Contingency:**
- Disable problematic features via feature flags while fixes are developed
- Escalate to KubeVirt community for guidance on edge cases

**Owner:** Backend Engineering Lead
**Status:** Open

---

### T03: Operator Failure Leads to Undetected Drift

| Field | Value |
|-------|-------|
| **Category** | Technical — Reliability |
| **Severity** | **12** (🟠 High) |
| **Likelihood** | 3 (Possible) |
| **Impact** | 4 (Major) |
| **Phase** | 1+ |

**Description:** The CMP operator (watches CMPVirtualMachine CRDs, reconciles against Harvester API) crashes or becomes unresponsive. Drift between declared and actual VM state goes undetected, leading to configuration inconsistencies.

**Indicators:**
- Operator pod restarts in Kubernetes events
- Drift detection latency exceeds 5 minutes
- Operator memory/cpu usage increasing over time

**Mitigation:**
1. Operator runs as Deployment with 2 replicas + leader election
2. Liveness/readiness probes with Kubernetes auto-restart
3. Operator health metrics exported to Prometheus
4. Alert on operator downtime > 2 minutes
5. Periodic full reconciliation job (hourly) as safety net

**Contingency:**
- Manual reconciliation script for emergency use
- Fallback to direct Harvester UI if operator down > 1 hour

**Owner:** Platform Engineer
**Status:** Open

---

### T04: Prometheus Metrics Insufficient for Cost Attribution

| Field | Value |
|-------|-------|
| **Category** | Technical — Data Quality |
| **Severity** | **12** (🟠 High) |
| **Likelihood** | 3 (Possible) |
| **Impact** | 4 (Major) |
| **Phase** | 1-2 |

**Description:** Harvester's Prometheus metrics lack required labels (cost_center, team, env) or granularity for accurate per-VM cost attribution. Cost data becomes estimated or inaccurate, undermining FinOps value proposition.

**Indicators:**
- Cost data gaps in dashboard (VMs showing $0 or null)
- Increasing "estimated" vs "actual" cost ratio
- User complaints about cost report accuracy

**Mitigation:**
1. CMP operator enriches VMs with required labels at creation
2. Backfill job retroactively labels existing VMs
3. Prometheus recording rules to derive missing labels from metadata
4. Cost data marked "estimated" when labels missing
5. Document known limitations in Phase 1

**Contingency:**
- Fall back to average-cost allocation if per-VM metrics unavailable
- Provide manual cost override capability for finance team

**Owner:** Backend Engineer (FinOps)
**Status:** Open

---

### T05: WebSocket Scaling Issues at Large VM Counts

| Field | Value |
|-------|-------|
| **Category** | Technical — Performance |
| **Severity** | **10** (🟡 Medium) |
| **Likelihood** | 2 (Unlikely) |
| **Impact** | 5 (Catastrophic) |
| **Phase** | 2-3 |

**Description:** Real-time event streaming to browsers via WebSockets degrades when managing 5000+ VMs. Connection overhead, event fan-out, and gateway memory usage cause latency or disconnections.

**Indicators:**
- WebSocket latency > 1s at 1000+ concurrent connections
- Gateway pod memory usage exceeding limits
- Browser console WebSocket errors

**Mitigation:**
1. Phase 1: Acceptable at < 1000 VMs (current scope)
2. Phase 2: Distributed gateway pool with connection sharding
3. Event aggregation (batch updates for high-frequency events)
4. Backoff: polling fallback when WebSocket unavailable

**Contingency:**
- Disable real-time updates; fall back to manual refresh
- Scale gateway replicas horizontally

**Owner:** Engineering Lead
**Status:** Open

---

### T06: VDDK Licensing Blocks VMDK Access in Open-Source Build

| Field | Value |
|-------|-------|
| **Category** | Legal — Licensing |
| **Severity** | **15** (🟠 High) |
| **Likelihood** | 5 (Almost Certain) |
| **Impact** | 3 (Moderate) |
| **Phase** | 3 |

**Description:** VMware's VDDK SDK is proprietary and cannot be distributed in Apache 2.0 builds. Without VDDK, VMDK access for migration is limited to slower HTTP/NBD paths, impacting migration toolchain usability.

**Indicators:**
- VDDK license terms prohibit redistribution
- Community asks why VDDK isn't included

**Mitigation:**
1. Use publicly accessible VMware APIs (vSphere REST API, VADP/NBD)
2. VMDK conversion via qemu-img with NBDKit (no VDDK dependency)
3. Document VDDK as optional "bring your own license" plugin for enterprises
4. Partner with VMware for open-source licensing exception (long shot)

**Contingency:**
- If VDDK truly required, offer proprietary Enterprise migration plugin
- Accept slower NBD-based migration as default

**Owner:** Engineering Lead + Legal
**Status:** Open

---

## 3. Business Risks

### B01: SUSE Discontinues or Deprioritizes Harvester

| Field | Value |
|-------|-------|
| **Category** | Business — Strategic |
| **Severity** | **20** (🔴 Critical) |
| **Likelihood** | 1 (Rare) |
| **Impact** | 5 (Catastrophic) |
| **Phase** | All |

**Description:** SUSE reduces investment in Harvester or discontinues the product. Without the target platform, HiveStack loses its primary value proposition and market.

**Indicators:**
- SUSE earnings calls mention Harvester deprioritization
- Harvester release cadence slows significantly
- Key Harvester maintainers leave SUSE

**Mitigation:**
1. Monitor SUSE public communications quarterly
2. Maintain awareness of alternative platforms (oVirt, Proxmox) for future pivot
3. Build abstraction layer that could theoretically support other KVM platforms
4. Diversify: HivePlane (agent-based) already has standalone value beyond Harvester

**Contingency:**
- Pivot HivePlane to manage standalone KVM+SLES hosts (already designed for this)
- Position as multi-HCI CMP (add Proxmox/oVirt support)

**Owner:** Executive Sponsor
**Status:** Open

---

### B02: Insufficient Open-Source Community Adoption

| Field | Value |
|-------|-------|
| **Category** | Business — Market |
| **Severity** | **16** (🟠 High) |
| **Likelihood** | 3 (Possible) |
| **Impact** | 4 (Major) |
| **Phase** | 2+ |

**Description:** The open-source community does not adopt HiveStack at sufficient scale. Low GitHub stars, few contributors, and limited word-of-mouth reduce the Enterprise edition sales funnel.

**Indicators:**
- < 100 GitHub stars after 6 months
- < 5 external contributors
- Low community engagement (Discord, issues, PRs)

**Mitigation:**
1. Ship Community edition from Day 1 (not "eventually open-source")
2. Target VMware migration as community growth driver (pain point = urgency)
3. Invest in documentation, tutorials, and "getting started" guides
4. Attend conferences (SUSE Summit, KubeCon, VMworld replacement events)
5. Engage with SUSE community channels for co-marketing
6. Offer free migration assessments (lead generation)

**Contingency:**
- Pivot to purely Enterprise (closed-source) if community traction fails
- Reduce team size; focus on direct Enterprise sales only

**Owner:** Product Manager
**Status:** Open

---

### B03: Enterprise Revenue Timing Slower Than Projected

| Field | Value |
|-------|-------|
| **Category** | Business — Financial |
| **Severity** | **15** (🟠 High) |
| **Likelihood** | 3 (Possible) |
| **Impact** | 3 (Moderate) |
| **Phase** | 2-3 |

**Description:** Enterprise customers take longer to evaluate, pilot, and purchase. Sales cycles stretch beyond projections, delaying break-even and straining cash flow.

**Indicators:**
- Pipeline conversion rate < 10%
- Average sales cycle > 90 days
- Pilot customers don't convert

**Mitigation:**
1. Offer free 30-day pilot with migration assessment (reduces friction)
2. Build professional services revenue stream (faster to close than licenses)
3. Target VMware-motivated enterprises (urgent need = shorter cycle)
4. Price competitively vs. commercial alternatives (Morpheus, CloudBolt)
5. Consider BSL licensing with free tier for small deployments (< 50 VMs)

**Contingency:**
- Extend runway via additional funding or reduce burn rate
- Focus on services revenue while building license pipeline

**Owner:** Product Manager + Executive Sponsor
**Status:** Open

---

### B04: Competitive Entry from Established CMP Vendors

| Field | Value |
|-------|-------|
| **Category** | Business — Market |
| **Severity** | **12** (🟠 High) |
| **Likelihood** | 2 (Unlikely) |
| **Impact** | 4 (Major) |
| **Phase** | 2-3 |

**Description:** Established CMP vendors (Morpheus, CloudBolt, Flexera) add Harvester support, leveraging existing enterprise relationships and feature breadth to outcompete HiveStack.

**Indicators:**
- Competitor press releases mentioning Harvester
- Competitor GitHub activity targeting Harvester APIs

**Mitigation:**
1. Differentiate via deep Harvester integration (native CRD, KubeVirt awareness)
2. Leverage open-source advantage (trust, transparency, no vendor lock-in)
3. Focus on VMware migration niche (competitors are multi-cloud, not migration-focused)
4. Build community moat (contributors, ecosystem, plugins)

**Contingency:**
- Pivot to Enterprise-only features competitors can't easily replicate
- Offer migration/integration services to differentiate

**Owner:** Product Manager
**Status:** Open

---

### B05: VMware Resolves Pricing Crisis, Slowing Migration Wave

| Field | Value |
|-------|-------|
| **Category** | Business — Market |
| **Severity** | **15** (🟠 High) |
| **Likelihood** | 2 (Unlikely) |
| **Impact** | 5 (Catastrophic) |
| **Phase** | 2-3 |

**Description:** Broadcom adjusts VMware pricing or licensing to reduce churn. The migration wave slows, reducing the addressable market for HiveStack's migration toolchain.

**Indicators:**
- Broadcom announces VMware pricing changes
- VMware churn rates stabilize or decline
- Industry analysts revise migration forecasts downward

**Mitigation:**
1. Build value beyond cost avoidance (governance, automation, sustainability)
2. Position HiveStack as "run better on Harvester" not just "escape VMware"
3. Target organizations that have already committed to migrate (irreversible decisions)
4. Diversify messaging: FinOps, GreenOps, self-service are universal needs

**Contingency:**
- Reduce Phase 3 scope (migration toolchain) if market shifts
- Focus on Phase 2 features (governance, automation) that have broader appeal

**Owner:** Executive Sponsor
**Status:** Open

---

## 4. Project Risks

### P01: Team Velocity Too Slow for 38-Week Plan

| Field | Value |
|-------|-------|
| **Category** | Project — Schedule |
| **Severity** | **15** (🟠 High) |
| **Likelihood** | 3 (Possible) |
| **Impact** | 3 (Moderate) |
| **Phase** | All |

**Description:** The 6.5 FTE team cannot deliver all planned features within 38 weeks. Integration complexity, learning curves, and unforeseen issues cause schedule slippage.

**Indicators:**
- Sprint velocity below plan for 2+ consecutive sprints
- Feature completion rate < 70%
- Increasing bug backlog

**Mitigation:**
1. Phase-based plan allows scope trimming (defer Phase 2-3 features if needed)
2. Use AI-assisted coding (Claude Code, Cursor) for 50-70% faster CRUD scaffolding
3. Each phase delivers standalone value (no "half-built" product)
4. Bi-weekly retrospectives to identify and address blockers early
5. 2-week buffer added to Phase 1 (10 weeks total)

**Contingency:**
- Extend timeline by 4-8 weeks if core features are close
- Reduce Phase 1 scope to bare minimum (VM CRUD + cost only) if severely behind
- Hire contractor for specific skill gaps (frontend, K8s operator)

**Owner:** Engineering Lead + PM
**Status:** Open

---

### P02: Key Person Dependency

| Field | Value |
|-------|-------|
| **Category** | Project — Personnel |
| **Severity** | **16** (🟠 High) |
| **Likelihood** | 3 (Possible) |
| **Impact** | 4 (Major) |
| **Phase** | All |

**Description:** One or two key engineers (K8s operator expert, Harvester specialist) leave the project, causing significant knowledge loss and delays.

**Indicators:**
- Single person is only expert in critical area
- Bus factor = 1 for operator, CRD translation, or Harvester client
- Key engineer shows signs of burnout

**Mitigation:**
1. Pair programming on critical components
2. Document architecture decisions (ADRs) for all major design choices
3. Cross-train team members on Harvester/K8s specifics
4. Competitive compensation and equity to retain talent
5. Contingency budget for contractor backup

**Contingency:**
- Hire contractor with specific expertise
- Reduce scope to match remaining team capabilities

**Owner:** Engineering Lead
**Status:** Open

---

### P03: Scope Creep from Stakeholder Requests

| Field | Value |
|-------|-------|
| **Category** | Project — Scope |
| **Severity** | **12** (🟠 High) |
| **Likelihood** | 4 (Likely) |
| **Impact** | 3 (Moderate) |
| **Phase** | All |

**Description:** Stakeholders (executives, customers, partners) request additional features outside the current phase scope, diverting resources and delaying core deliverables.

**Indicators:**
- "Can you also..." requests in sprint reviews
- Increasing backlog size without corresponding scope approval
- Feature requests not in PRD or phase plan

**Mitigation:**
1. Strict phase gate process (no new features mid-phase without formal change request)
2. "Parking lot" for out-of-scope ideas (transparent, visible to all)
3. Phase gates require Executive Sponsor approval for scope changes
4. Educate stakeholders on cost of scope creep (timeline vs. features tradeoff)

**Contingency:**
- Formal change request process with timeline impact analysis
- Escalate to Executive Sponsor for prioritization decisions

**Owner:** Product Manager
**Status:** Open

---

### P04: Integration Testing Gaps Cause Production Issues

| Field | Value |
|-------|-------|
| **Category** | Project — Quality |
| **Severity** | **16** (🟠 High) |
| **Likelihood** | 3 (Possible) |
| **Impact** | 4 (Major) |
| **Phase** | 1+ |

**Description:** Insufficient integration testing leads to bugs in production: VMs fail to provision, cost data is wrong, RBAC allows unauthorized access, or audit logs miss events.

**Indicators:**
- Unit tests pass but e2e tests fail intermittently
- Production incidents in staging-like environments
- Security review finds authorization gaps

**Mitigation:**
1. Invest in CI pipeline early (Phase 0) with e2e test framework
2. Contract tests between CMP API and Harvester client
3. Load testing at each phase gate (k6)
4. Security-focused code review for all auth/RBAC changes
5. Staging environment mirrors production for pre-release validation

**Contingency:**
- Rollback deployment if critical bug found in production
- Hotfix branch process for emergency patches
- Feature flags to disable problematic features without full rollback

**Owner:** QA Engineer + Engineering Lead
**Status:** Open

---

## 5. Legal & Compliance Risks

### L01: Trademark Conflict with "HarvestCMP" Name

| Field | Value |
|-------|-------|
| **Category** | Legal — IP |
| **Severity** | **9** (🟡 Medium) |
| **Likelihood** | 3 (Possible) |
| **Impact** | 3 (Moderate) |
| **Phase** | Pre-launch |

**Description:** SUSE asserts trademark rights over "Harvester" and objects to the "HarvestCMP" product name as confusingly similar. Forced rebranding after launch is costly and damages early momentum.

**Indicators:**
- SUSE legal team contacts about name similarity
- Trademark search reveals SUSE has broad "Harvester" claims

**Mitigation:**
1. **Recommended: Rename to "HiveStack CMP" or "HiveCMP" before launch**
2. Conduct comprehensive trademark search in key markets (US, EU, India)
3. Register "HiveStack" and "HivePlane" trademarks proactively
4. Consult with IP attorney before public launch

**Contingency:**
- If challenged, rebrand with 2-4 week sprint (new GitHub org, docs, website)
- Maintain redirects from old name for 12 months

**Owner:** Executive Sponsor + Legal
**Status:** Open — Decision Needed

---

### L02: GDPR Compliance Gaps in Audit Log

| Field | Value |
|-------|-------|
| **Category** | Legal — Compliance |
| **Severity** | **12** (🟠 High) |
| **Likelihood** | 2 (Unlikely) |
| **Impact** | 5 (Catastrophic) |
| **Phase** | 2+ |

**Description:** The immutable audit log contains PII (user emails, IP addresses) that GDPR requires to be deletable upon request. "No-delete" constraint conflicts with "right to erasure."

**Indicators:**
- GDPR audit identifies audit log as non-compliant
- User requests data deletion but audit log retains their PII

**Mitigation:**
1. Phase 1: Document audit log as "legitimate interest" with data retention policy
2. Phase 2: Implement anonymization option for audit records older than 13 months
3. Implement data export API for GDPR data portability requests
4. Consult with GDPR compliance expert during Phase 2 planning

**Contingency:**
- Emergency patch to support selective anonymization
- Restrict EU deployments until resolved

**Owner:** Product Manager + Legal
**Status:** Open

---

## 6. Risk Heat Map

```
                                    IMPACT
                    1        2        3        4        5
              ┌────────┬────────┬────────┬────────┬────────┐
         5    │        │        │        │        │ T06     │
              │        │        │        │        │ B05    │
    L         ├────────┼────────┼────────┼────────┼────────┤
    I    4    │        │        │ P03    │ T01    │        │
    K         │        │        │        │ B01    │        │
    E         ├────────┼────────┼────────┼────────┼────────┤
    L    3    │        │        │ T03    │ T02    │ T05    │
    I         │        │        │ T04    │ P01    │        │
    H         │        │        │ L01    │ P02    │        │
    O         │        │        │ B03    │ P04    │        │
    O         ├────────┼────────┼────────┼────────┼────────┤
    D    2    │        │        │        │ B04    │        │
              │        │        │        │ L02    │        │
              ├────────┼────────┼────────┼────────┼────────┤
         1    │        │        │        │ B01    │        │
              └────────┴────────┴────────┴────────┴────────┘

LEGEND:
  T = Technical    B = Business    P = Project    L = Legal
```

---

## 7. Risk Ownership Summary

| Risk | Severity | Owner | Due Date |
|------|----------|-------|----------|
| B01: SUSE Discontinues Harvester | 20 (🔴) | Exec Sponsor | Quarterly review |
| T01: Harvester API Breaking Changes | 16 (🟠) | Eng Lead | Phase 0 |
| P02: Key Person Dependency | 16 (🟠) | Eng Lead | Ongoing |
| P04: Integration Testing Gaps | 16 (🟠) | QA + Eng Lead | Phase 0 |
| T02: CRD Translation Complexity | 15 (🟠) | Backend Lead | Phase 1 |
| P01: Team Velocity Too Slow | 15 (🟠) | Eng Lead + PM | Ongoing |
| B03: Enterprise Revenue Timing | 15 (🟠) | PM + Exec Sponsor | Phase 2 |
| B05: VMware Pricing Reversal | 15 (🟠) | Exec Sponsor | Quarterly review |
| T06: VDDK Licensing | 15 (🟠) | Eng Lead + Legal | Phase 3 |
| T03: Operator Failure | 12 (🟠) | Platform Eng | Phase 1 |
| T04: Prometheus Metrics Insufficient | 12 (🟠) | Backend (FinOps) | Phase 1 |
| B04: Competitive CMP Entry | 12 (🟠) | PM | Phase 2 |
| P03: Scope Creep | 12 (🟠) | PM | Ongoing |
| L02: GDPR Audit Log | 12 (🟠) | PM + Legal | Phase 2 |
| T05: WebSocket Scaling | 10 (🟡) | Eng Lead | Phase 2 |
| L01: Trademark Conflict (HarvestCMP) | 9 (🟡) | Exec Sponsor + Legal | Pre-launch |

---

## 8. Risk Review Cadence

| Review Type | Frequency | Attendees | Output |
|-------------|-----------|-----------|--------|
| Sprint risk check | Bi-weekly | Eng team | Updated risk status |
| Phase gate review | Per phase | Full team + Sponsor | Go/No-Go decision |
| Monthly executive review | Monthly | Exec Sponsor + Leads | Strategic risk assessment |
| Quarterly deep review | Quarterly | All stakeholders | Full register update |

---

## 9. Appendices

### A. Risks Accepted (No Active Mitigation)

| Risk | Reason Accepted |
|------|-----------------|
| B05: VMware pricing reversal | Market forces outside our control; diversify value prop instead |
| B04: Competitive entry | Focus on execution speed; open-source moat |

### B. Risks Transferred

| Risk | Transfer Mechanism |
|------|-------------------|
| Infrastructure outage | Cloud provider SLA (if using cloud) |
| Key person departure | Retention bonuses, equity vesting |

### C. References

- HiveStack Project Charter (docs/PROJECT_CHARTER.md)
- HiveStack TELOS Feasibility Study (docs/FEASIBILITY_STUDY.md)
- HarvestCMP PRD v1.0 (Documents/SUSE/Harvester CMP/)
- HivePlane Feature Matrix (Downloads/)

---

*This Risk Register is a living document. It will be reviewed at each sprint retrospective, phase gate, and monthly executive review. All Critical and High risks require active mitigation plans with assigned owners and due dates.*
