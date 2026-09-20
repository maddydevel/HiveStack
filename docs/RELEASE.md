# HiveStack Release Process

**Version:** 1.0.0
**Last Updated:** 2026-09-20

## Table of Contents

- [Overview](#overview)
- [Versioning](#versioning)
- [Version Bump Procedure](#version-bump-procedure)
- [Changelog Update](#changelog-update)
- [Git Tag Signing](#git-tag-signing)
- [SBOM Generation](#sbom-generation)
- [Release Checklist](#release-checklist)
- [Canary Deployment](#canary-deployment)
- [Rollback Criteria](#rollback-criteria)
- [Post-Release](#post-release)

---

## Overview

HiveStack follows a structured release process to ensure quality, stability, and traceability. All releases are automated through GitHub Actions with signed artifacts and comprehensive audit trails.

### Release Cadence

| Release Type | Frequency | Description |
|--------------|-----------|-------------|
| **Major** | As needed | Breaking changes, major features |
| **Minor** | Monthly | New features, improvements |
| **Patch** | As needed | Bug fixes, security patches |
| **Hotfix** | Emergency | Critical production fixes |

---

## Versioning

HiveStack uses [Semantic Versioning](https://semver.org/): `MAJOR.MINOR.PATCH`

- **MAJOR**: Incompatible API changes
- **MINOR**: Backward-compatible functionality additions
- **PATCH**: Backward-compatible bug fixes

Pre-release versions use suffixes: `v1.0.0-alpha.1`, `v1.0.0-beta.1`, `v1.0.0-rc.1`

### Helm Chart Versioning

The Helm chart version in `appliance/helm/Chart.yaml` tracks both the chart version (`version`) and the application version (`appVersion`):

```yaml
apiVersion: v2
name: hivestack
description: HiveStack - VM Management Platform
type: application
version: 1.0.0        # Chart version (SemVer)
appVersion: "1.0.0"   # Application version (matches git tag)
```

**Rules:**
- `appVersion` MUST match the git tag version (without `v` prefix)
- `version` follows chart-specific SemVer (bumped on chart-only changes)
- Both versions MUST be updated in lockstep for application releases

---

## Version Bump Procedure

### 1. Update Source Version References

```bash
# Update version in build configuration
export NEW_VERSION="1.2.0"

# If version is hardcoded in source:
sed -i "s/version = \".*\"/version = \"${NEW_VERSION}\"/" cmd/version.go
```

### 2. Update Helm Chart

```bash
# Update appliance/helm/Chart.yaml
yq e -i '.version = "${NEW_VERSION}"' appliance/helm/Chart.yaml
yq e -i '.appVersion = "${NEW_VERSION}"' appliance/helm/Chart.yaml
```

### 3. Verify Build

```bash
go build ./...
go vet ./...
go test ./... -count=1
```

---

## Changelog Update

Follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/) format.

### Update Steps

1. Move entries from `[Unreleased]` to new version section:

```markdown
## [v1.2.0] — 2026-09-20

### Added
- New feature X

### Changed
- Modified behavior Y

### Fixed
- Bug Z

### Security
- CVE-2026-XXXX patch
```

2. Add compare link at bottom:

```markdown
[v1.2.0]: https://github.com/maddydevel/HiveStack/compare/v1.1.0...v1.2.0
[Unreleased]: https://github.com/maddydevel/HiveStack/compare/v1.2.0...HEAD
```

3. Commit: `git commit -m "chore: update changelog for v${NEW_VERSION}"`

---

## Git Tag Signing

All release tags MUST be signed with GPG.

### Prerequisites

```bash
# Verify GPG key is configured
git config --global user.signingkey YOUR_KEY_ID
git config --global commit.gpgsign true
git config --global tag.gpgsign true
```

### Create Signed Tag

```bash
# Create annotated, signed tag
git tag -s v${NEW_VERSION} -m "Release v${NEW_VERSION}"

# Verify signature
git tag -v v${NEW_VERSION}

# Push tag
git push origin v${NEW_VERSION}
```

### Automated Signing (CI/CD)

Tags created through GitHub Actions are signed with [cosign](https://sigstore.dev/) keyless signing:

```bash
# Keyless signing (CI)
cosign sign-blob \
  --output-signature hivestack-${NEW_VERSION}.sig \
  --output-certificate hivestack-${NEW_VERSION}.cert \
  dist/hivestack-${NEW_VERSION}-linux-amd64.tar.gz
```

---

## SBOM Generation

Software Bill of Materials (SBOM) is generated automatically for each release using Syft (SPDX + CycloneDX formats).

### Manual Generation

```bash
# Install Syft
brew install syft

# Generate SPDX JSON SBOM
syft dir:. -o spdx-json > sbom-spdx.json

# Generate CycloneDX JSON SBOM
syft dir:. -o cyclonedx-json > sbom-cyclonedx.json

# Generate SBOM for container image
syft ghcr.io/maddydevel/hivestack:${NEW_VERSION} -o spdx-json > sbom-image-spdx.json
```

### SBOM Attestation

SBOM is attached to container images using cosign:

```bash
# Attach SBOM to container image
cosign attach sbom \
  --sbom sbom-spdx.json \
  --type spdx \
  ghcr.io/maddydevel/hivestack:${NEW_VERSION}
```

### Output Formats

| Format | File | Purpose |
|--------|------|---------|
| SPDX JSON | `sbom-spdx.json` | Open standard, tool interoperability |
| CycloneDX JSON | `sbom-cyclonedx.json` | Security-focused, vulnerability mapping |
| Syft JSON | `sbom-syft.json` | Full detail, Syft-specific |

---

## Release Checklist

### Pre-Release (1 week before)

- [ ] All planned features merged to `main`
- [ ] All tests passing (`go test ./... -race`)
- [ ] `go vet ./...` clean
- [ ] Security scan clean (govulncheck, gosec, trivy)
- [ ] CHANGELOG.md updated with all changes since last release
- [ ] Version bumped in all relevant files
- [ ] Helm Chart.yaml `version` and `appVersion` updated
- [ ] Database migrations reviewed and tested (if applicable)
- [ ] Backward compatibility verified
- [ ] Performance benchmarks run

### Release Day

- [ ] Final CI pass on `main`
- [ ] Create signed git tag: `git tag -s vX.Y.Z -m "Release vX.Y.Z"`
- [ ] Push tag: `git push origin vX.Y.Z`
- [ ] GitHub Actions workflow triggered successfully
- [ ] Container images published and signed
- [ ] Binary assets attached to GitHub Release
- [ ] SBOM attached to release
- [ ] Release notes published
- [ ] Canary deployment initiated

### Post-Release

- [ ] Monitor error rates for 24 hours
- [ ] Update deployment in staging
- [ ] Announce release
- [ ] Close related issues/milestones

---

## Canary Deployment

### Process

1. **Deploy to canary node (25% of cluster)**
   ```bash
   ./scripts/canary-deploy.sh --image ghcr.io/maddydevel/hivestack:v${NEW_VERSION} \
     --version ${NEW_VERSION} \
     --percentage 25
   ```

2. **Observe for 30 minutes**
   - Monitor error rates (< 0.1% threshold)
   - Monitor API latency (p99 < 500ms)
   - Monitor resource usage (CPU < 80%, Memory < 85%)
   - Check logs for unexpected errors

3. **Full rollout** (if canary passes)
   ```bash
   ./scripts/canary-deploy.sh --image ghcr.io/maddydevel/hivestack:v${NEW_VERSION} \
     --version ${NEW_VERSION} \
     --percentage 100
   ```

4. **Post-deployment verification**
   ```bash
   ./scripts/health-check.sh --verbose
   ```

### Monitoring Metrics

| Metric | Warning | Critical |
|--------|---------|----------|
| Error Rate | > 0.1% | > 1% |
| API Latency (p99) | > 200ms | > 500ms |
| CPU Usage | > 70% | > 90% |
| Memory Usage | > 80% | > 95% |
| Disk Usage | > 75% | > 90% |
| VM Error Count | > 0 | > 5 |

---

## Rollback Criteria

### Automatic Rollback Triggers

The following conditions will trigger automatic rollback:

| Condition | Threshold | Duration | Action |
|-----------|-----------|----------|--------|
| Health check failure | HTTP != 200 | > 5 minutes | Immediate rollback |
| Error rate spike | > 5% | > 10 minutes | Immediate rollback |
| API latency spike | p99 > 2000ms | > 15 minutes | Immediate rollback |
| Service crash loop | 3 restarts | < 5 minutes | Immediate rollback |
| Data corruption | Any | Any | Immediate rollback |
| Certificate expiry | Any | Any | Immediate rollback |

### Manual Rollback Decision

Evaluate rollback when:

- Feature causes customer-reported issues
- Performance regression > 20% from baseline
- Critical bug discovered post-release
- Security vulnerability in released version
- Migration failure rate > 5%

### Rollback Execution

```bash
# Quick rollback to previous version
./scripts/rollback.sh --force

# Rollback to specific version
./scripts/rollback.sh v1.1.0 --force

# Rollback without health check (for partial recovery)
./scripts/rollback.sh --no-health-check --force
```

### Rollback Verification

After rollback:
1. Health checks pass within 5 minutes
2. Error rates return to baseline
3. No data loss confirmed
4. Previous functionality restored
5. Incident documented in post-mortem

---

## Post-Release Verification

### Immediate (within 1 hour)

```bash
# Verify all services healthy
./scripts/health-check.sh --verbose

# Check systemd service status
sudo systemctl status hivestack-manager
sudo systemctl status hivestack-node

# Verify version
hive version
```

### 24-Hour Monitoring

- Monitor dashboards for anomalies
- Check alert channels for false positives
- Review error logs for new patterns
- Confirm backup success

### Post-Mortem

For any issues discovered post-release:

1. Document the issue and impact
2. Identify root cause
3. Apply preventive measures
4. Update runbooks if needed
5. Communicate resolution to stakeholders
