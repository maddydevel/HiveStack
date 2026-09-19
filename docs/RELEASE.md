# HiveStack Release Process

**Version:** 1.0.0
**Last Updated:** 2026-09-19

## Table of Contents

- [Overview](#overview)
- [Versioning](#versioning)
- [Release Branches](#release-branches)
- [Release Checklist](#release-checklist)
- [Release Procedure](#release-procedure)
- [Hotfix Process](#hotfix-process)
- [Rollback Procedure](#rollback-procedure)
- [Post-Release](#post-release)

---

## Overview

HiveStack follows a structured release process to ensure quality, stability, and traceability. All releases are automated through GitHub Actions.

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

### Version Bump Procedure

1. Update version in source code (if hardcoded)
2. Update `CHANGELOG.md`
3. Create a git tag: `git tag -a v1.0.0 -m "Release v1.0.0"`
4. Push tag: `git push origin v1.0.0`
5. GitHub Actions automatically builds and publishes the release

---

## Release Branches

### Main Branch (`main`)
- Always deployable
- Protected branch
- Requires PR review and CI pass
- Tags on `main` trigger production releases

### Develop Branch (`develop`)
- Integration branch for features
- Nightly builds from this branch
- Merged to `main` for release

### Release Branches (`release/vX.Y`)
- Created from `develop` when preparing a release
- Only bug fixes allowed (no new features)
- Merged to `main` and back to `develop`

### Hotfix Branches (`hotfix/vX.Y.Z`)
- Created from `main` tag
- Critical fixes only
- Merged to `main` and `develop`

---

## Release Checklist

### Pre-Release (1 week before)

- [ ] All planned features merged to `develop`
- [ ] All tests passing on `develop`
- [ ] Security scan clean (govulncheck, gosec, trivy)
- [ ] Documentation updated (API docs, deployment guide, runbooks)
- [ ] CHANGELOG.md updated with all changes since last release
- [ ] Database migrations reviewed and tested
- [ ] Backward compatibility verified (API, config, database)
- [ ] Performance benchmarks run and documented
- [ ] Release notes drafted

### Release Day

- [ ] Create release branch from `develop`: `release/vX.Y.Z`
- [ ] Final testing on release branch
- [ ] Merge release branch to `main`
- [ ] Tag the release: `vX.Y.Z`
- [ ] Verify GitHub Actions release workflow completes
- [ ] Verify container images published to GHCR
- [ ] Verify binary assets attached to GitHub Release
- [ ] Test installation from release artifacts
- [ ] Update documentation site (if applicable)

### Post-Release

- [ ] Merge release branch back to `develop`
- [ ] Announce release (Discord, mailing list, blog)
- [ ] Update deployment in staging environment
- [ ] Monitor error rates and metrics for 24 hours
- [ ] Close related issues and milestones

---

## Release Procedure

### Automated Release (Recommended)

1. **Prepare the release:**
   ```bash
   git checkout develop
   git pull origin develop
   
   # Update CHANGELOG.md
   # Update version references if any
   
   git add CHANGELOG.md
   git commit -m "chore: prepare release vX.Y.Z"
   ```

2. **Create and push tag:**
   ```bash
   git checkout main
   git merge develop
   git tag -a vX.Y.Z -m "Release vX.Y.Z"
   git push origin main --tags
   ```

3. **GitHub Actions automatically:**
   - Runs all tests
   - Builds binaries for all platforms
   - Builds and pushes container images
   - Creates GitHub Release with artifacts
   - Generates changelog from commits

### Manual Release (Emergency)

If GitHub Actions is unavailable:

```bash
# Build locally
make build VERSION=X.Y.Z

# Build container image
docker build -t hivestack:vX.Y.Z \
  --build-arg VERSION=X.Y.Z \
  --build-arg BUILD_DATE=$(date -u +%Y-%m-%dT%H:%M:%SZ) \
  --build-arg GIT_COMMIT=$(git rev-parse HEAD) \
  -f appliance/Dockerfile .

# Push to registry
docker tag hivestack:vX.Y.Z ghcr.io/maddydevel/hivestack:vX.Y.Z
docker push ghcr.io/maddydevel/hivestack:vX.Y.Z

# Create GitHub Release
gh release create vX.Y.Z \
  --title "HiveStack vX.Y.Z" \
  --notes-file RELEASE_NOTES.md \
  dist/*.tar.gz
```

---

## Hotfix Process

For critical production issues requiring immediate fix:

1. **Create hotfix branch from main tag:**
   ```bash
   git checkout -b hotfix/vX.Y.Z+1 vX.Y.Z
   ```

2. **Apply the fix:**
   ```bash
   # Make minimal changes to fix the issue
   git commit -m "fix: [description of fix]"
   ```

3. **Test and release:**
   ```bash
   go test ./...
   git tag -a vX.Y.Z+1 -m "Hotfix vX.Y.Z+1"
   git push origin hotfix/vX.Y.Z+1 --tags
   ```

4. **Merge back:**
   ```bash
   git checkout main
   git merge hotfix/vX.Y.Z+1
   git checkout develop
   git merge hotfix/vX.Y.Z+1
   git branch -d hotfix/vX.Y.Z+1
   ```

---

## Rollback Procedure

If a release causes issues in production:

### Container Rollback

```bash
# Rollback to previous version
docker pull ghcr.io/maddydevel/hivestack:vX.Y.Z-1
docker stop hivestack-manager
docker run -d --name hivestack-manager \
  -p 8080:8080 -p 8443:8443 \
  ghcr.io/maddydevel/hivestack:vX.Y.Z-1 manager
```

### Binary Rollback

```bash
# Download previous release
wget https://github.com/maddydevel/HiveStack/releases/download/vX.Y.Z-1/hivestack-vX.Y.Z-1-linux-amd64.tar.gz

# Extract and install
tar xzf hivestack-vX.Y.Z-1-linux-amd64.tar.gz
sudo cp hive-manager /usr/bin/
sudo systemctl restart hivestack-manager
```

### Database Rollback

If database migrations were applied:

```bash
# Check current migration version
psql -U hivestack -d hivestack -c "SELECT * FROM schema_migrations ORDER BY version DESC LIMIT 5;"

# Rollback specific migration (if down migration exists)
psql -U hivestack -d hivestack -f internal/db/migrations/XXXX_rollback.sql
```

---

## Post-Release Verification

After deploying a release, verify:

1. **Health checks pass:**
   ```bash
   curl -k https://localhost:8443/api/v1/health
   ```

2. **All services running:**
   ```bash
   sudo systemctl status hivestack-manager
   sudo systemctl status hivestack-node
   ```

3. **No errors in logs:**
   ```bash
   sudo journalctl -u hivestack-manager --since "5 minutes ago" | grep -i error
   ```

4. **Metrics normal:**
   - CPU/Memory usage within expected range
   - API response times normal
   - No spike in error rates

5. **Key workflows functional:**
   - User login
   - VM list/create
   - Node status reporting
