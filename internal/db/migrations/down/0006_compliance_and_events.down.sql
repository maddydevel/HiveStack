-- Reverts 0006_compliance_and_events.sql.
-- WARNING: compliance_evidence and events are audit tables. Dropping them
-- destroys the tamper-evident hash chain and the audit log irrecoverably.

DROP INDEX IF EXISTS idx_events_type;
DROP INDEX IF EXISTS idx_events_tenant_id;
DROP TABLE IF EXISTS events;

DROP INDEX IF EXISTS idx_compliance_evidence_vm_id;
DROP INDEX IF EXISTS idx_compliance_evidence_tenant_id;
DROP TABLE IF EXISTS compliance_evidence;
