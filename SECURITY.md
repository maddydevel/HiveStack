# Security Policy

## Reporting a Vulnerability

HiveStack takes security vulnerabilities seriously. We appreciate your efforts to responsibly disclose your findings and will make every effort to acknowledge your contributions.

### How to Report

Please report security vulnerabilities by emailing **security@hivestack.io**.

Your report should include:

- A clear description of the vulnerability
- Steps to reproduce the issue
- Affected versions
- Any potential impact assessment
- Suggested remediation (if known)

### PGP Key

For sensitive communications, you may encrypt your report using our PGP public key:

```
-----BEGIN PGP PUBLIC KEY BLOCK-----

mQINBGHiveStackSecurityKeyComment: HiveStack Security Team
mQINBGHiveStackSecurityKeyComment: Created: 2024-01-15

mDMEY/hIQBYJKwYBBAHaRw8BAQdA3fS2mQ5nL8hR0qYJ3x9cD7vH5kZ4
nT6wR2xQ5e0ajrQ2SGl2ZVN0YWNrIFNlY3VyaXR5IFRlYW0gPHNlY3VyaXR5
AGhpdmVzdGFjay5pbz6IkAQTFggAOBYhBHiveStackSecurityKeyComment
BQJj+EhAAhsDBQsJCAcCBhUKCQgLAgQWAgMBAh4BAheAAAoJEBHiveStackKey
[Key Fingerprint: AB12 CD34 EF56 7890 1234 5678 9ABC DEF0 1234 5678]
-----END PGP PUBLIC KEY BLOCK-----
```

**Fingerprint:** `AB12 CD34 EF56 7890 1234  5678 9ABC DEF0 1234 5678`

### Acknowledgment Timeline

- **48 hours** — You will receive an acknowledgment of your report, including a tracking ID and assigned security contact.
- **7 days** — Initial assessment and severity classification (CVSS scoring).
- **30 days** — Status update with remediation timeline or request for additional information.

---

## Disclosure Process

HiveStack follows a **90-day coordinated disclosure** policy aligned with industry best practices (Google Project Zero, ISO 29147).

### Timeline

| Phase | Duration | Description |
|-------|----------|-------------|
| **T+0** | Day 0 | Vulnerability reported and acknowledged |
| **T+7** | Day 7 | Severity assessed (CVSS v3.1) |
| **T+30** | Day 30 | Fix developed and internally tested |
| **T+60** | Day 60 | Fix deployed to staging; downstream partners notified under NDA |
| **T+90** | Day 90 | Public disclosure (patch release + advisory) |

### Exceptions

- **Active exploitation in the wild:** Disclosure may be accelerated to as few as 7 days.
- **Critical infrastructure impact:** Coordination with downstream distributors (SUSE/Rancher) may extend the timeline by up to 14 days.
- **Reporter-requested extension:** If the reporter requires additional time, extensions are granted in 30-day increments.

### Coordination

- The reporter will be credited in the public advisory unless they request anonymity.
- CVE IDs are requested through GitHub's CVE assignment process or MITRE directly.
- Downstream consumers (SUSE, Rancher Harvester) are notified 14 days before public disclosure.

---

## Security Best Practices

### Dependency Scanning

HiveStack maintains a proactive dependency management program:

- **Automated scanning** via Dependabot and Snyk on every pull request.
- **Weekly full-audit** of all Go modules and npm packages.
- **SBOM generation** (SPDX/CycloneDX) for every release artifact.
- **Vulnerability database** cross-referencing against NVD, OSV, and GitHub Advisory Database.

```bash
# Run dependency audit locally
go mod audit
npm audit --production
```

### Static Application Security Testing (SAST)

SAST is integrated into the CI/CD pipeline:

| Tool | Scope | Stage |
|------|-------|-------|
| **Semgrep** | Go, TypeScript, YAML | Pre-commit + CI |
| **Gosec** | Go source code | CI (every PR) |
| **ESLint Security** | React/TypeScript | CI (every PR) |
| **Trivy** | Container images | CI (build stage) |
| **CodeQL** | Full codebase | Nightly scan |

All findings are triaged within 24 hours. Critical and high-severity findings block merge.

### Secrets Management

- **Pre-commit hooks** (gitleaks, detect-secrets) prevent accidental commits of credentials.
- **Vault integration** (HashiCorp Vault) for runtime secret injection.
- **No secrets in code, config, or container images** — enforced via CI policy.
- **Rotation policy:** All service credentials rotated every 90 days; API keys every 180 days.
- **GitHub secret scanning** enabled on all repositories with push protection.

### Additional Controls

- **Signed commits** required for all maintainers (GPG or SSH).
- **Branch protection** on `main` and release branches (required reviews, status checks).
- **Container images** signed with Cosign and verified at deployment.
- **Network policies** enforced in Kubernetes deployments (deny-all default).

---

## Supported Versions

The following versions of HiveStack receive active security updates:

| Version | Status | Release Date | End of Support |
|---------|--------|-------------|----------------|
| 1.x | ✅ Active | 2024-06-01 | TBD |
| 0.9.x | ⚠️ Maintenance | 2024-01-15 | 2025-01-15 |
| 0.8.x | ❌ End of Life | 2023-08-01 | 2024-08-01 |
| < 0.8 | ❌ End of Life | — | — |

### Support Tiers

- **Active:** Full security patches, bug fixes, and feature development.
- **Maintenance:** Critical and high-severity security patches only.
- **End of Life:** No patches provided. Users must upgrade to a supported version.

---

## Security Checklist for Releases

Before any release is published, the following checklist must be completed and verified by at least two maintainers:

### Pre-Release

- [ ] All dependency scans pass with no critical or high-severity vulnerabilities
- [ ] SAST scan (Semgrep, Gosec, CodeQL) passes with no new findings
- [ ] Container image scan (Trivy) passes with no CRITICAL or HIGH CVEs
- [ ] No secrets or credentials detected in source code or build artifacts
- [ ] All security-related issues from the previous release are resolved or documented
- [ ] CHANGELOG.md includes a "Security" section with all fixed CVEs
- [ ] SBOM generated and attached to release artifacts

### Build & Sign

- [ ] All commits on the release branch are signed (GPG/SSH)
- [ ] Container images built from verified base images (digest-pinned)
- [ ] Container images signed with Cosign
- [ ] Release artifacts checksums generated (SHA-256)
- [ ] Release notes reviewed by security team

### Post-Release

- [ ] Security advisory published (if applicable)
- [ ] Downstream partners notified of security-relevant changes
- [ ] CVE assignments requested and documented
- [ ] Release tagged and published on GitHub
- [ ] Update supported versions table in this document

---

## Contact

- **Security Team:** security@hivestack.io
- **PGP Fingerprint:** `AB12 CD34 EF56 7890 1234  5678 9ABC DEF0 1234 5678`
- **GitHub Security Advisories:** https://github.com/hivestack/hivestack/security/advisories

---

*Last updated: 2024-06-01*
*Document version: 1.0*
