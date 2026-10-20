-- CRAB migration 003: governed access current-state projections.
--
-- Authority classification: all tables below are rebuildable projections.
-- The signed ledger remains the authority. There is no per-object ACL estate.

CREATE TABLE IF NOT EXISTS crab.role_eligibility (
    human_id text PRIMARY KEY CHECK (btrim(human_id) <> ''),
    role text NOT NULL CHECK (role IN ('Major', 'Minor', 'Observer'))
);
COMMENT ON TABLE crab.role_eligibility IS
    'CRAB authority=rebuildable; current Major/Minor/Observer eligibility from confirmed ledger history';

CREATE TABLE IF NOT EXISTS crab.custodian_designations (
    human_id text PRIMARY KEY CHECK (btrim(human_id) <> '')
);
COMMENT ON TABLE crab.custodian_designations IS
    'CRAB authority=rebuildable; current Custodian metadata authority; no incident-content access implied';

CREATE TABLE IF NOT EXISTS crab.incident_memberships (
    human_id text NOT NULL CHECK (btrim(human_id) <> ''),
    incident_id text NOT NULL CHECK (btrim(incident_id) <> ''),
    PRIMARY KEY (human_id, incident_id)
);
COMMENT ON TABLE crab.incident_memberships IS
    'CRAB authority=rebuildable; current governed incident memberships';

CREATE TABLE IF NOT EXISTS crab.compartment_memberships (
    human_id text NOT NULL CHECK (btrim(human_id) <> ''),
    incident_id text NOT NULL CHECK (btrim(incident_id) <> ''),
    compartment text NOT NULL CHECK (btrim(compartment) <> ''),
    PRIMARY KEY (human_id, incident_id, compartment)
);
COMMENT ON TABLE crab.compartment_memberships IS
    'CRAB authority=rebuildable; GENERAL derives from incident membership; restricted compartments require explicit confirmed read-in';

CREATE TABLE IF NOT EXISTS crab.output_permissions (
    human_id text NOT NULL CHECK (btrim(human_id) <> ''),
    action text NOT NULL CHECK (btrim(action) <> ''),
    PRIMARY KEY (human_id, action)
);
COMMENT ON TABLE crab.output_permissions IS
    'CRAB authority=rebuildable; current separately governed data-exit/action permissions';

INSERT INTO crab.schema_migrations(version) VALUES (3)
ON CONFLICT (version) DO NOTHING;
