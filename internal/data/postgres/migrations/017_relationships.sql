-- UAT Phase 2: related incidents (PRD 23, AC-84). Rebuildable projections.
-- A link never grants access and never merges history.

CREATE TABLE IF NOT EXISTS crab.incident_relationship_proposals (
    incident_id text NOT NULL CHECK (btrim(incident_id) <> ''),
    record_id text NOT NULL CHECK (btrim(record_id) <> ''),
    record_type text NOT NULL DEFAULT 'incident.relationship' CHECK (record_type = 'incident.relationship'),
    related_incident_id text NOT NULL,
    version text NOT NULL,
    maker text NOT NULL,
    payload jsonb NOT NULL,
    proposed_at timestamptz NOT NULL,
    mode text NOT NULL CHECK (mode IN ('live','exercise')),
    PRIMARY KEY (incident_id, record_id)
);
COMMENT ON TABLE crab.incident_relationship_proposals IS
    'CRAB authority=rebuildable; pending incident relationship proposals';

CREATE TABLE IF NOT EXISTS crab.incident_relationships (
    incident_id text NOT NULL,
    related_incident_id text NOT NULL,
    reason text NOT NULL,
    record_id text NOT NULL,
    maker text NOT NULL,
    confirmer text NOT NULL,
    confirmed_at timestamptz NOT NULL,
    PRIMARY KEY (incident_id, related_incident_id)
);
COMMENT ON TABLE crab.incident_relationships IS
    'CRAB authority=rebuildable; active confirmed relationships, stored in both directions; never an access grant';

INSERT INTO crab.schema_migrations(version) VALUES (17) ON CONFLICT (version) DO NOTHING;
