-- Slice 6: scoped access-governance projections and expiring grants.
-- Authority classification: all tables/columns below are rebuildable projections.

ALTER TABLE crab.incident_memberships
    ADD COLUMN IF NOT EXISTS effective_at timestamptz,
    ADD COLUMN IF NOT EXISTS expires_at timestamptz,
    ADD COLUMN IF NOT EXISTS reason text NOT NULL DEFAULT '';

ALTER TABLE crab.compartment_memberships
    ADD COLUMN IF NOT EXISTS effective_at timestamptz,
    ADD COLUMN IF NOT EXISTS expires_at timestamptz,
    ADD COLUMN IF NOT EXISTS reason text NOT NULL DEFAULT '';

CREATE TABLE IF NOT EXISTS crab.incident_sponsors (
    human_id text NOT NULL CHECK (btrim(human_id) <> ''),
    incident_id text NOT NULL CHECK (btrim(incident_id) <> ''),
    PRIMARY KEY (human_id, incident_id)
);
COMMENT ON TABLE crab.incident_sponsors IS
    'CRAB authority=rebuildable; current scoped Incident Sponsor designations; designation does not itself grant content access';

CREATE TABLE IF NOT EXISTS crab.compartment_controllers (
    human_id text NOT NULL CHECK (btrim(human_id) <> ''),
    incident_id text NOT NULL CHECK (btrim(incident_id) <> ''),
    compartment text NOT NULL CHECK (btrim(compartment) <> '' AND compartment <> 'GENERAL'),
    PRIMARY KEY (human_id, incident_id, compartment)
);
COMMENT ON TABLE crab.compartment_controllers IS
    'CRAB authority=rebuildable; current scoped Compartment Controller designations; designation does not itself grant content access';

CREATE TABLE IF NOT EXISTS crab.access_proposals (
    scope_id text NOT NULL DEFAULT '',
    record_id text NOT NULL CHECK (btrim(record_id) <> ''),
    record_type text NOT NULL CHECK (btrim(record_type) <> ''),
    compartment text NOT NULL CHECK (btrim(compartment) <> ''),
    version text NOT NULL CHECK (btrim(version) <> ''),
    maker text NOT NULL CHECK (btrim(maker) <> ''),
    payload bytea NOT NULL,
    proposed_at timestamptz NOT NULL,
    mode text NOT NULL CHECK (mode IN ('system','live','exercise')),
    time_status text NOT NULL CHECK (time_status IN ('trusted','drifting','untrusted')),
    PRIMARY KEY (scope_id, record_type, record_id)
);
COMMENT ON TABLE crab.access_proposals IS
    'CRAB authority=rebuildable; exact pending access-governance proposals only';

CREATE OR REPLACE FUNCTION crab.reject_access_proposal_update() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
    RAISE EXCEPTION 'pending access proposals are immutable until confirmed or rejected';
END;
$$;
DROP TRIGGER IF EXISTS access_proposals_immutable_pending ON crab.access_proposals;
CREATE TRIGGER access_proposals_immutable_pending
    BEFORE UPDATE ON crab.access_proposals
    FOR EACH ROW EXECUTE FUNCTION crab.reject_access_proposal_update();

INSERT INTO crab.schema_migrations(version) VALUES (10)
ON CONFLICT (version) DO NOTHING;
