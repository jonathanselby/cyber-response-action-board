-- Slice 4: richer Action projection and incident timeline.

ALTER TABLE crab.incident_actions
    ADD COLUMN IF NOT EXISTS status text NOT NULL DEFAULT 'not_started',
    ADD COLUMN IF NOT EXISTS priority text NOT NULL DEFAULT 'unspecified',
    ADD COLUMN IF NOT EXISTS due_at timestamptz,
    ADD COLUMN IF NOT EXISTS source_ref text NOT NULL DEFAULT 'legacy',
    ADD COLUMN IF NOT EXISTS notes text NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS evidence_refs jsonb NOT NULL DEFAULT '[]'::jsonb,
    ADD COLUMN IF NOT EXISTS skip_reason text NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS checklist_template_id text NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS checklist_step_id text NOT NULL DEFAULT '';

ALTER TABLE crab.incident_facts
    ADD COLUMN IF NOT EXISTS occurred_at timestamptz,
    ADD COLUMN IF NOT EXISTS flagged_at timestamptz,
    ADD COLUMN IF NOT EXISTS awareness jsonb NOT NULL DEFAULT '[]'::jsonb;

CREATE TABLE IF NOT EXISTS crab.incident_timeline (
    sequence bigint PRIMARY KEY,
    incident_id text NOT NULL,
    record_id text NOT NULL,
    record_type text NOT NULL,
    governance text NOT NULL,
    compartment text NOT NULL,
    summary text NOT NULL,
    occurred_at timestamptz,
    flagged_at timestamptz,
    recorded_at timestamptz NOT NULL,
    awareness jsonb NOT NULL DEFAULT '[]'::jsonb,
    time_status text NOT NULL
);
CREATE INDEX IF NOT EXISTS incident_timeline_incident_recorded_idx
    ON crab.incident_timeline(incident_id, recorded_at, sequence);
COMMENT ON TABLE crab.incident_timeline IS 'Rebuildable timeline derived from authoritative ledger history and explicit domain event times.';

CREATE OR REPLACE FUNCTION crab.reject_pending_proposal_update() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
    RAISE EXCEPTION 'pending proposals are immutable until confirmed or rejected';
END;
$$;
DROP TRIGGER IF EXISTS incident_proposals_immutable_pending ON crab.incident_proposals;
CREATE TRIGGER incident_proposals_immutable_pending
    BEFORE UPDATE ON crab.incident_proposals
    FOR EACH ROW EXECUTE FUNCTION crab.reject_pending_proposal_update();
COMMENT ON TRIGGER incident_proposals_immutable_pending ON crab.incident_proposals IS 'Prevents replacement of the exact proposal version awaiting maker-checker decision.';

INSERT INTO crab.schema_migrations(version) VALUES (8) ON CONFLICT (version) DO NOTHING;
