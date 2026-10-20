-- Slice 12: recovery coordination, stand-down, PIR, retention and closure.
-- Operational phase remains derived in Go; there is deliberately no mutable phase state machine.

CREATE TABLE IF NOT EXISTS crab.incident_recovery_updates (
    incident_id text NOT NULL,
    item_id text NOT NULL,
    record_id text NOT NULL,
    compartment text NOT NULL,
    payload jsonb NOT NULL,
    maker text NOT NULL,
    recorded_at timestamptz NOT NULL,
    PRIMARY KEY (incident_id, item_id)
);
CREATE INDEX IF NOT EXISTS incident_recovery_updates_incident_idx
    ON crab.incident_recovery_updates(incident_id, recorded_at, item_id);
COMMENT ON TABLE crab.incident_recovery_updates IS 'Rebuildable current recovery coordination view; immutable history remains in the ledger.';

CREATE TABLE IF NOT EXISTS crab.incident_recovery_proposals (
    incident_id text NOT NULL,
    record_id text NOT NULL,
    record_type text NOT NULL,
    compartment text NOT NULL,
    version text NOT NULL,
    maker text NOT NULL,
    payload bytea NOT NULL,
    proposed_at timestamptz NOT NULL,
    mode text NOT NULL CHECK (mode IN ('live','exercise')),
    PRIMARY KEY (incident_id, record_type, record_id)
);
COMMENT ON TABLE crab.incident_recovery_proposals IS 'Exact pending governed recovery proposal bytes awaiting maker-checker decision.';

CREATE TABLE IF NOT EXISTS crab.incident_recovery_state (
    incident_id text PRIMARY KEY,
    stand_down_record_id text,
    stand_down_at timestamptz,
    stand_down_payload jsonb,
    pir_record_id text,
    pir_at timestamptz,
    pir_payload jsonb,
    retention_record_id text,
    retention_at timestamptz,
    retention_payload jsonb,
    closure_record_id text,
    closed_at timestamptz,
    closure_payload jsonb,
    CHECK ((stand_down_record_id IS NULL) = (stand_down_at IS NULL)),
    CHECK ((pir_record_id IS NULL) = (pir_at IS NULL)),
    CHECK ((retention_record_id IS NULL) = (retention_at IS NULL)),
    CHECK ((closure_record_id IS NULL) = (closed_at IS NULL))
);
COMMENT ON TABLE crab.incident_recovery_state IS 'Rebuildable recovery/closure milestones. Operational phase is derived, never stored as a second state machine.';

CREATE OR REPLACE FUNCTION crab.reject_recovery_proposal_update() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
    RAISE EXCEPTION 'pending recovery proposals are immutable until confirmed or rejected';
END;
$$;
DROP TRIGGER IF EXISTS incident_recovery_proposals_immutable_pending ON crab.incident_recovery_proposals;
CREATE TRIGGER incident_recovery_proposals_immutable_pending
    BEFORE UPDATE ON crab.incident_recovery_proposals
    FOR EACH ROW EXECUTE FUNCTION crab.reject_recovery_proposal_update();

-- Closure is a hard cross-feature write gate. The ledger insert is the single
-- convergence point for every CRAB mutation, so enforce read-only here as
-- defence in depth rather than duplicating the rule in each feature runtime.
CREATE OR REPLACE FUNCTION crab.reject_closed_incident_ledger_append() RETURNS trigger
LANGUAGE plpgsql AS $$
DECLARE
    envelope jsonb;
    scoped_incident text;
BEGIN
    envelope := convert_from(NEW.entry_bytes, 'UTF8')::jsonb;
    scoped_incident := envelope #>> '{record,incident_id}';
    IF scoped_incident IS NOT NULL AND scoped_incident <> '' AND EXISTS (
        SELECT 1 FROM crab.incident_recovery_state
        WHERE incident_id = scoped_incident AND closed_at IS NOT NULL
    ) THEN
        RAISE EXCEPTION 'incident is closed and read-only';
    END IF;
    RETURN NEW;
END;
$$;
DROP TRIGGER IF EXISTS ledger_reject_closed_incident ON crab.ledger_entries;
CREATE TRIGGER ledger_reject_closed_incident
    BEFORE INSERT ON crab.ledger_entries
    FOR EACH ROW EXECUTE FUNCTION crab.reject_closed_incident_ledger_append();
COMMENT ON TRIGGER ledger_reject_closed_incident ON crab.ledger_entries IS 'Hard read-only gate after confirmed incident closure; system-scope records remain unaffected.';

INSERT INTO crab.schema_migrations(version) VALUES (14) ON CONFLICT (version) DO NOTHING;
