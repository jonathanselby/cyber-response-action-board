-- Slice 5: governed obligation definitions, current clocks and timed wait list.

CREATE TABLE IF NOT EXISTS crab.obligation_definitions (
    definition_id text PRIMARY KEY CHECK (btrim(definition_id) <> ''),
    name text NOT NULL CHECK (btrim(name) <> ''),
    trigger_field text NOT NULL CHECK (btrim(trigger_field) <> ''),
    duration integer NOT NULL CHECK (duration >= 0),
    unit text NOT NULL CHECK (unit IN ('elapsed_hours','calendar_days','business_days')),
    calendar jsonb NOT NULL,
    applicability_condition text NOT NULL CHECK (btrim(applicability_condition) <> ''),
    recipient text NOT NULL CHECK (btrim(recipient) <> ''),
    owner_text text NOT NULL CHECK (btrim(owner_text) <> ''),
    authority_source text NOT NULL CHECK (btrim(authority_source) <> ''),
    warning_before_minutes jsonb NOT NULL DEFAULT '[]'::jsonb,
    maker text NOT NULL CHECK (btrim(maker) <> ''),
    confirmer text NOT NULL CHECK (btrim(confirmer) <> ''),
    confirmed_at timestamptz NOT NULL
);
COMMENT ON TABLE crab.obligation_definitions IS 'CRAB authority=rebuildable; confirmed organisation-defined obligation configuration.';

CREATE TABLE IF NOT EXISTS crab.obligation_proposals (
    scope_id text NOT NULL DEFAULT '',
    record_id text NOT NULL CHECK (btrim(record_id) <> ''),
    record_type text NOT NULL CHECK (record_type IN ('system.obligation_definition','incident.obligation_trigger','incident.obligation_change')),
    compartment text NOT NULL CHECK (btrim(compartment) <> ''),
    version text NOT NULL CHECK (btrim(version) <> ''),
    maker text NOT NULL CHECK (btrim(maker) <> ''),
    payload bytea NOT NULL,
    proposed_at timestamptz NOT NULL,
    mode text NOT NULL CHECK (mode IN ('system','live','exercise')),
    PRIMARY KEY (scope_id, record_type, record_id)
);
COMMENT ON TABLE crab.obligation_proposals IS 'CRAB authority=rebuildable; exact pending obligation proposals only.';

CREATE TABLE IF NOT EXISTS crab.obligation_clocks (
    incident_id text NOT NULL CHECK (btrim(incident_id) <> ''),
    clock_id text NOT NULL CHECK (btrim(clock_id) <> ''),
    compartment text NOT NULL CHECK (btrim(compartment) <> ''),
    definition_id text NOT NULL CHECK (btrim(definition_id) <> ''),
    name text NOT NULL CHECK (btrim(name) <> ''),
    lifecycle text NOT NULL CHECK (lifecycle IN ('provisional','firm')),
    status text NOT NULL CHECK (status IN ('active','waived')),
    trigger_at timestamptz NOT NULL,
    source_record_id text NOT NULL CHECK (btrim(source_record_id) <> ''),
    source_field text NOT NULL CHECK (btrim(source_field) <> ''),
    original_definition_id text NOT NULL CHECK (btrim(original_definition_id) <> ''),
    original_deadline timestamptz NOT NULL,
    deadline timestamptz NOT NULL,
    recipient text NOT NULL,
    owner_text text NOT NULL,
    authority_source text NOT NULL,
    last_change_kind text NOT NULL DEFAULT '',
    last_change_reason text NOT NULL DEFAULT '',
    time_status text NOT NULL,
    simulated boolean NOT NULL DEFAULT false,
    maker text NOT NULL,
    confirmer text NOT NULL DEFAULT '',
    confirmed_at timestamptz,
    PRIMARY KEY (incident_id, clock_id)
);
COMMENT ON TABLE crab.obligation_clocks IS 'CRAB authority=rebuildable; current provisional/firm clock state. Original deadline is immutable historical reference.';

CREATE TABLE IF NOT EXISTS crab.timed_wait_list (
    incident_id text NOT NULL,
    clock_id text NOT NULL,
    wake_at timestamptz NOT NULL,
    wake_kind text NOT NULL CHECK (wake_kind IN ('obligation_warning','obligation_deadline')),
    lifecycle text NOT NULL CHECK (lifecycle IN ('provisional','firm')),
    simulated boolean NOT NULL DEFAULT false,
    PRIMARY KEY (incident_id, clock_id, wake_at, wake_kind)
);
CREATE INDEX IF NOT EXISTS timed_wait_list_due_idx ON crab.timed_wait_list(wake_at, incident_id, clock_id);
COMMENT ON TABLE crab.timed_wait_list IS 'CRAB authority=rebuildable; deterministic future wake-ups only, not a workflow or delivery queue.';

CREATE OR REPLACE FUNCTION crab.reject_obligation_proposal_update() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
    RAISE EXCEPTION 'pending obligation proposals are immutable until confirmed or rejected';
END;
$$;
DROP TRIGGER IF EXISTS obligation_proposals_immutable_pending ON crab.obligation_proposals;
CREATE TRIGGER obligation_proposals_immutable_pending
    BEFORE UPDATE ON crab.obligation_proposals
    FOR EACH ROW EXECUTE FUNCTION crab.reject_obligation_proposal_update();

INSERT INTO crab.schema_migrations(version) VALUES (9) ON CONFLICT (version) DO NOTHING;
