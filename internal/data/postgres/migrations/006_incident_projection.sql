-- CRAB migration 006: first incident common-operating-picture projections.
--
-- Authority classification: every table below is rebuildable from the signed
-- ledger. Pending proposals are visible work awaiting decision, not effective
-- state. The ledger remains the sole authority.

CREATE TABLE IF NOT EXISTS crab.incidents (
    incident_id text PRIMARY KEY CHECK (btrim(incident_id) <> ''),
    mode text NOT NULL CHECK (mode IN ('live', 'exercise')),
    title text NOT NULL CHECK (btrim(title) <> ''),
    summary text NOT NULL CHECK (btrim(summary) <> ''),
    severity text NOT NULL CHECK (btrim(severity) <> ''),
    declaration_record_id text NOT NULL CHECK (btrim(declaration_record_id) <> ''),
    declared_at timestamptz NOT NULL
);
COMMENT ON TABLE crab.incidents IS
    'CRAB authority=rebuildable; effective confirmed incident declaration/current operating-picture header';

CREATE TABLE IF NOT EXISTS crab.incident_facts (
    incident_id text NOT NULL CHECK (btrim(incident_id) <> ''),
    record_id text NOT NULL CHECK (btrim(record_id) <> ''),
    compartment text NOT NULL CHECK (btrim(compartment) <> ''),
    statement text NOT NULL CHECK (btrim(statement) <> ''),
    source text NOT NULL DEFAULT '',
    maker text NOT NULL CHECK (btrim(maker) <> ''),
    recorded_at timestamptz NOT NULL,
    PRIMARY KEY (incident_id, record_id)
);
COMMENT ON TABLE crab.incident_facts IS
    'CRAB authority=rebuildable; observations only; fact text cannot alter controlled incident state';

CREATE TABLE IF NOT EXISTS crab.incident_uncertainties (
    incident_id text NOT NULL CHECK (btrim(incident_id) <> ''),
    record_id text NOT NULL CHECK (btrim(record_id) <> ''),
    compartment text NOT NULL CHECK (btrim(compartment) <> ''),
    kind text NOT NULL CHECK (kind IN ('unknown', 'hypothesis')),
    statement text NOT NULL CHECK (btrim(statement) <> ''),
    maker text NOT NULL CHECK (btrim(maker) <> ''),
    recorded_at timestamptz NOT NULL,
    PRIMARY KEY (incident_id, record_id)
);
COMMENT ON TABLE crab.incident_uncertainties IS
    'CRAB authority=rebuildable; unknowns and hypotheses remain explicitly distinct from confirmed facts';

CREATE TABLE IF NOT EXISTS crab.incident_decisions (
    incident_id text NOT NULL CHECK (btrim(incident_id) <> ''),
    record_id text NOT NULL CHECK (btrim(record_id) <> ''),
    compartment text NOT NULL CHECK (btrim(compartment) <> ''),
    decision_text text NOT NULL CHECK (btrim(decision_text) <> ''),
    rationale text NOT NULL DEFAULT '',
    maker text NOT NULL CHECK (btrim(maker) <> ''),
    confirmer text NOT NULL CHECK (btrim(confirmer) <> ''),
    confirmed_at timestamptz NOT NULL,
    PRIMARY KEY (incident_id, record_id)
);

CREATE TABLE IF NOT EXISTS crab.incident_actions (
    incident_id text NOT NULL CHECK (btrim(incident_id) <> ''),
    record_id text NOT NULL CHECK (btrim(record_id) <> ''),
    compartment text NOT NULL CHECK (btrim(compartment) <> ''),
    title text NOT NULL CHECK (btrim(title) <> ''),
    owner_human_id text NOT NULL DEFAULT '',
    maker text NOT NULL CHECK (btrim(maker) <> ''),
    confirmer text NOT NULL CHECK (btrim(confirmer) <> ''),
    confirmed_at timestamptz NOT NULL,
    PRIMARY KEY (incident_id, record_id)
);

CREATE TABLE IF NOT EXISTS crab.incident_operational_roles (
    incident_id text NOT NULL CHECK (btrim(incident_id) <> ''),
    record_id text NOT NULL CHECK (btrim(record_id) <> ''),
    compartment text NOT NULL CHECK (btrim(compartment) <> ''),
    role_name text NOT NULL CHECK (btrim(role_name) <> ''),
    human_id text NOT NULL CHECK (btrim(human_id) <> ''),
    deputy boolean NOT NULL DEFAULT false,
    maker text NOT NULL CHECK (btrim(maker) <> ''),
    confirmer text NOT NULL CHECK (btrim(confirmer) <> ''),
    confirmed_at timestamptz NOT NULL,
    PRIMARY KEY (incident_id, record_id)
);
COMMENT ON TABLE crab.incident_operational_roles IS
    'CRAB authority=rebuildable; organisation role labels only; does not alter Major/Minor/Observer authorisation role';

CREATE TABLE IF NOT EXISTS crab.incident_proposals (
    incident_id text NOT NULL CHECK (btrim(incident_id) <> ''),
    record_id text NOT NULL CHECK (btrim(record_id) <> ''),
    record_type text NOT NULL CHECK (btrim(record_type) <> ''),
    compartment text NOT NULL CHECK (btrim(compartment) <> ''),
    version text NOT NULL CHECK (btrim(version) <> ''),
    maker text NOT NULL CHECK (btrim(maker) <> ''),
    payload bytea NOT NULL,
    proposed_at timestamptz NOT NULL,
    PRIMARY KEY (incident_id, record_type, record_id)
);
COMMENT ON TABLE crab.incident_proposals IS
    'CRAB authority=rebuildable; pending governed proposals only; never effective controlled state';

INSERT INTO crab.schema_migrations(version) VALUES (6)
ON CONFLICT (version) DO NOTHING;
