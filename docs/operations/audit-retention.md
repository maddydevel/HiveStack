# HiveStack — Audit Log Retention Policy

## Retention Periods

| Table | Retention | Rationale |
|-------|-----------|-----------|
| `events` (audit log) | 13 months | GDPR Article 5(1)(e) — storage limitation |
| `compliance_evidence` | Immutable | SAP HANA certification requirement |
| `api_tokens` | Until expiry + 30 days | Security — prevent token reuse |

## Archival Strategy

### Phase 1: Events Table (13 months)
```sql
-- Archive events older than 13 months to cold storage
CREATE TABLE events_archive (LIKE events INCLUDING ALL);

-- Monthly archive job (via pg_cron or external scheduler)
INSERT INTO events_archive
SELECT * FROM events
WHERE created_at < NOW() - INTERVAL '13 months';

DELETE FROM events
WHERE created_at < NOW() - INTERVAL '13 months';
```

### Phase 2: Compliance Evidence (Immutable)
- Never deleted — hash chain integrity depends on complete history
- Archived to S3/NFS after 2 years for cost optimization
- Original retained in PostgreSQL for chain verification

### Phase 3: API Tokens
- Tokens auto-deleted 30 days after expiry
- Token hashes retained for audit trail (cannot be reversed to plaintext)

## Purge Procedure

1. **Verify archive integrity**: `SELECT count(*) FROM events_archive WHERE created_at < NOW() - INTERVAL '13 months'`
2. **Backup to S3**: `pg_dump --table=events | gzip | aws s3 cp - s3://hivestack-archive/events_YYYYMMDD.sql.gz`
3. **Delete from production**: `DELETE FROM events WHERE created_at < NOW() - INTERVAL '13 months'`
4. **Verify**: `SELECT min(created_at) FROM events` should be within 13 months

## GDPR Compliance

- **Right to Erasure**: Events containing PII (user emails, IP addresses) can be anonymized upon verified request
- **Anonymization**: Replace `actor_id` with hash, `actor_name` with `[REDACTED]`
- **Evidence Preservation**: Compliance evidence is exempt — legitimate interest for audit compliance
