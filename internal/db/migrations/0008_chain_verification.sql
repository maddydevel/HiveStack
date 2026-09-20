-- Hash-chain verification for compliance_evidence (see 0006).
--
-- verify_compliance_evidence_chain returns one row per broken link: a row
-- whose previous_hash does not equal the hash of the preceding evidence row
-- for the same VM (ordered by created_at, then id). An empty result means the
-- chain is intact. The first row of each VM's chain is not checked because
-- its previous_hash is a caller-defined genesis value.
--
-- Only the links are verified, not the row hashes themselves: the writers in
-- internal/compliance hash different inputs (and the original JSON bytes are
-- not preserved by JSONB), so hashes cannot be recomputed in SQL. This
-- mirrors ComplianceStore.VerifyChain. Pass NULL to check all VMs / tenants.

CREATE OR REPLACE FUNCTION verify_compliance_evidence_chain(
    p_vm_id     TEXT DEFAULT NULL,
    p_tenant_id TEXT DEFAULT NULL
)
RETURNS TABLE (
    vm_id                  TEXT,
    evidence_id            TEXT,
    evidence_created_at    TIMESTAMPTZ,
    expected_previous_hash TEXT,
    actual_previous_hash   TEXT
)
LANGUAGE sql STABLE AS $$
    SELECT l.vm_id, l.id, l.created_at, l.prior_hash, l.previous_hash
    FROM (
        SELECT ce.vm_id, ce.id, ce.created_at, ce.previous_hash,
               lag(ce.hash) OVER (
                   PARTITION BY ce.vm_id ORDER BY ce.created_at, ce.id
               ) AS prior_hash
        FROM compliance_evidence ce
        WHERE (p_vm_id IS NULL OR ce.vm_id = p_vm_id)
          AND (p_tenant_id IS NULL OR ce.tenant_id = p_tenant_id)
    ) l
    WHERE l.prior_hash IS NOT NULL
      AND l.previous_hash <> l.prior_hash
    ORDER BY l.vm_id, l.created_at, l.id;
$$;

-- Convenience wrapper: true when no broken links exist (an empty chain is valid).
CREATE OR REPLACE FUNCTION is_compliance_evidence_chain_valid(
    p_vm_id     TEXT DEFAULT NULL,
    p_tenant_id TEXT DEFAULT NULL
)
RETURNS BOOLEAN
LANGUAGE sql STABLE AS $$
    SELECT NOT EXISTS (
        SELECT 1 FROM verify_compliance_evidence_chain(p_vm_id, p_tenant_id)
    );
$$;
