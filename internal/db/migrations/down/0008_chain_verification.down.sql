-- Reverts 0008_chain_verification.sql.

DROP FUNCTION IF EXISTS is_compliance_evidence_chain_valid(TEXT, TEXT);
DROP FUNCTION IF EXISTS verify_compliance_evidence_chain(TEXT, TEXT);
