-- Slice 10: structured communications, cadence and immutable notification outcomes.

CREATE TABLE IF NOT EXISTS crab.communication_drafts (
    incident_id text NOT NULL CHECK (btrim(incident_id) <> ''),
    record_id text NOT NULL CHECK (btrim(record_id) <> ''),
    mode text NOT NULL CHECK (mode IN ('live','exercise')),
    compartment text NOT NULL CHECK (btrim(compartment) <> ''),
    kind text NOT NULL CHECK (btrim(kind) <> ''),
    subject text NOT NULL CHECK (btrim(subject) <> ''),
    body text NOT NULL CHECK (btrim(body) <> ''),
    audience jsonb NOT NULL,
    channels jsonb NOT NULL,
    source_refs jsonb NOT NULL,
    related_record_ids jsonb NOT NULL,
    cadence_id text NOT NULL DEFAULT '',
    sha256 text NOT NULL CHECK (length(sha256) = 64),
    maker text NOT NULL CHECK (btrim(maker) <> ''),
    recorded_at timestamptz NOT NULL,
    PRIMARY KEY (incident_id, record_id)
);
COMMENT ON TABLE crab.communication_drafts IS 'CRAB authority=rebuildable; immutable communication drafts/reports, not approved or issued state.';

CREATE TABLE IF NOT EXISTS crab.communication_proposals (
    incident_id text NOT NULL CHECK (btrim(incident_id) <> ''),
    record_id text NOT NULL CHECK (btrim(record_id) <> ''),
    record_type text NOT NULL CHECK (record_type IN ('incident.communication','incident.communication_cadence')),
    compartment text NOT NULL CHECK (btrim(compartment) <> ''),
    version text NOT NULL CHECK (btrim(version) <> ''),
    maker text NOT NULL CHECK (btrim(maker) <> ''),
    payload bytea NOT NULL,
    proposed_at timestamptz NOT NULL,
    mode text NOT NULL CHECK (mode IN ('live','exercise')),
    PRIMARY KEY (incident_id, record_type, record_id)
);
COMMENT ON TABLE crab.communication_proposals IS 'CRAB authority=rebuildable; exact pending communication/cadence proposals.';

CREATE TABLE IF NOT EXISTS crab.communications (
    incident_id text NOT NULL CHECK (btrim(incident_id) <> ''),
    record_id text NOT NULL CHECK (btrim(record_id) <> ''),
    mode text NOT NULL CHECK (mode IN ('live','exercise')),
    compartment text NOT NULL CHECK (btrim(compartment) <> ''),
    draft_record_id text NOT NULL CHECK (btrim(draft_record_id) <> ''),
    draft_sha256 text NOT NULL CHECK (length(draft_sha256) = 64),
    kind text NOT NULL CHECK (btrim(kind) <> ''),
    subject text NOT NULL CHECK (btrim(subject) <> ''),
    body text NOT NULL CHECK (btrim(body) <> ''),
    audience jsonb NOT NULL,
    channels jsonb NOT NULL,
    source_refs jsonb NOT NULL,
    related_record_ids jsonb NOT NULL,
    cadence_id text NOT NULL DEFAULT '',
    maker text NOT NULL CHECK (btrim(maker) <> ''),
    confirmer text NOT NULL CHECK (btrim(confirmer) <> ''),
    issued_at timestamptz NOT NULL,
    PRIMARY KEY (incident_id, record_id)
);
COMMENT ON TABLE crab.communications IS 'CRAB authority=rebuildable; confirmed/issued governed communications only.';

CREATE TABLE IF NOT EXISTS crab.communication_cadences (
    incident_id text NOT NULL CHECK (btrim(incident_id) <> ''),
    cadence_id text NOT NULL CHECK (btrim(cadence_id) <> ''),
    mode text NOT NULL CHECK (mode IN ('live','exercise')),
    compartment text NOT NULL CHECK (btrim(compartment) <> ''),
    name text NOT NULL CHECK (btrim(name) <> ''),
    interval_minutes integer NOT NULL CHECK (interval_minutes > 0),
    first_due_at timestamptz NOT NULL,
    owner_human_id text NOT NULL CHECK (btrim(owner_human_id) <> ''),
    audience jsonb NOT NULL,
    channels jsonb NOT NULL,
    maker text NOT NULL CHECK (btrim(maker) <> ''),
    confirmer text NOT NULL CHECK (btrim(confirmer) <> ''),
    confirmed_at timestamptz NOT NULL,
    last_generated_at timestamptz,
    next_due_at timestamptz NOT NULL,
    simulated boolean NOT NULL DEFAULT false,
    PRIMARY KEY (incident_id, cadence_id)
);
COMMENT ON TABLE crab.communication_cadences IS 'CRAB authority=rebuildable; confirmed situation-report cadence and derived next due time.';

CREATE TABLE IF NOT EXISTS crab.notification_outcomes (
    incident_id text NOT NULL CHECK (btrim(incident_id) <> ''),
    record_id text NOT NULL CHECK (btrim(record_id) <> ''),
    compartment text NOT NULL CHECK (btrim(compartment) <> ''),
    communication_id text NOT NULL CHECK (btrim(communication_id) <> ''),
    channel text NOT NULL CHECK (btrim(channel) <> ''),
    destination_id text NOT NULL DEFAULT '',
    attempted_at timestamptz NOT NULL,
    status text NOT NULL CHECK (status IN ('delivered','failed','suppressed')),
    detail text NOT NULL DEFAULT '',
    acknowledgement text NOT NULL DEFAULT '',
    duration_ms bigint NOT NULL CHECK (duration_ms >= 0),
    maker text NOT NULL CHECK (btrim(maker) <> ''),
    recorded_at timestamptz NOT NULL,
    PRIMARY KEY (incident_id, record_id)
);
CREATE INDEX IF NOT EXISTS notification_outcomes_communication_idx ON crab.notification_outcomes(incident_id, communication_id, attempted_at, record_id);
COMMENT ON TABLE crab.notification_outcomes IS 'CRAB authority=rebuildable; immutable ledger-derived notification attempt outcomes.';

CREATE TABLE IF NOT EXISTS crab.incoming_enquiries (
    incident_id text NOT NULL CHECK (btrim(incident_id) <> ''),
    record_id text NOT NULL CHECK (btrim(record_id) <> ''),
    compartment text NOT NULL CHECK (btrim(compartment) <> ''),
    source text NOT NULL CHECK (btrim(source) <> ''),
    contact text NOT NULL DEFAULT '',
    subject text NOT NULL CHECK (btrim(subject) <> ''),
    message text NOT NULL CHECK (btrim(message) <> ''),
    received_at timestamptz NOT NULL,
    assigned_to_human_id text NOT NULL CHECK (btrim(assigned_to_human_id) <> ''),
    related_record_ids jsonb NOT NULL,
    maker text NOT NULL CHECK (btrim(maker) <> ''),
    recorded_at timestamptz NOT NULL,
    PRIMARY KEY (incident_id, record_id)
);
COMMENT ON TABLE crab.incoming_enquiries IS 'CRAB authority=rebuildable; material incoming enquiries and assignment, not chat/helpdesk state.';

-- Reuse the existing deterministic timed wait-list. A communication cadence uses
-- clock_id='communication:<cadence-id>', lifecycle='firm' and its own wake kind.
ALTER TABLE crab.timed_wait_list DROP CONSTRAINT IF EXISTS timed_wait_list_wake_kind_check;
ALTER TABLE crab.timed_wait_list ADD CONSTRAINT timed_wait_list_wake_kind_check CHECK (wake_kind IN ('obligation_warning','obligation_deadline','communication_cadence'));

CREATE OR REPLACE FUNCTION crab.reject_communication_proposal_update() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
    RAISE EXCEPTION 'pending communication proposals are immutable until confirmed or rejected';
END;
$$;
DROP TRIGGER IF EXISTS communication_proposals_immutable_pending ON crab.communication_proposals;
CREATE TRIGGER communication_proposals_immutable_pending
    BEFORE UPDATE ON crab.communication_proposals
    FOR EACH ROW EXECUTE FUNCTION crab.reject_communication_proposal_update();

INSERT INTO crab.schema_migrations(version) VALUES (13) ON CONFLICT (version) DO NOTHING;
