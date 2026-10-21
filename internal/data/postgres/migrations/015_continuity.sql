-- PRD V0.2 continuity: first-report intake, evolving scope/severity, handover and resource coordination.

CREATE TABLE IF NOT EXISTS crab.first_reports (
    report_id text PRIMARY KEY CHECK (btrim(report_id) <> ''),
    payload jsonb NOT NULL,
    maker text NOT NULL CHECK (btrim(maker) <> ''),
    recorded_at timestamptz NOT NULL
);
CREATE TABLE IF NOT EXISTS crab.first_report_dispositions (
    record_id text PRIMARY KEY CHECK (btrim(record_id) <> ''),
    report_id text NOT NULL REFERENCES crab.first_reports(report_id) ON DELETE RESTRICT,
    payload jsonb NOT NULL,
    maker text NOT NULL CHECK (btrim(maker) <> ''),
    recorded_at timestamptz NOT NULL
);
CREATE INDEX IF NOT EXISTS first_report_dispositions_report_idx ON crab.first_report_dispositions(report_id,recorded_at DESC);
CREATE TABLE IF NOT EXISTS crab.continuity_proposals (
    incident_id text NOT NULL, record_id text NOT NULL, record_type text NOT NULL, compartment text NOT NULL,
    version text NOT NULL, maker text NOT NULL, payload jsonb NOT NULL, proposed_at timestamptz NOT NULL,
    mode text NOT NULL CHECK (mode IN ('live','exercise')), PRIMARY KEY (incident_id,record_type,record_id)
);
CREATE TABLE IF NOT EXISTS crab.incident_scope_assessments (
    record_id text PRIMARY KEY, incident_id text NOT NULL, compartment text NOT NULL, payload jsonb NOT NULL,
    maker text NOT NULL, recorded_at timestamptz NOT NULL
);
CREATE INDEX IF NOT EXISTS incident_scope_assessments_scope_idx ON crab.incident_scope_assessments(incident_id,compartment,recorded_at DESC);
CREATE TABLE IF NOT EXISTS crab.incident_severity_changes (
    record_id text PRIMARY KEY, incident_id text NOT NULL, compartment text NOT NULL, payload jsonb NOT NULL,
    maker text NOT NULL, confirmer text NOT NULL, confirmed_at timestamptz NOT NULL
);
CREATE INDEX IF NOT EXISTS incident_severity_changes_scope_idx ON crab.incident_severity_changes(incident_id,compartment,confirmed_at DESC);
CREATE TABLE IF NOT EXISTS crab.incident_handovers (
    record_id text PRIMARY KEY, incident_id text NOT NULL, compartment text NOT NULL, payload jsonb NOT NULL,
    maker text NOT NULL, recorded_at timestamptz NOT NULL
);
CREATE INDEX IF NOT EXISTS incident_handovers_scope_idx ON crab.incident_handovers(incident_id,compartment,recorded_at DESC);
CREATE TABLE IF NOT EXISTS crab.incident_resource_requests (
    record_id text PRIMARY KEY, incident_id text NOT NULL, compartment text NOT NULL, payload jsonb NOT NULL,
    maker text NOT NULL, recorded_at timestamptz NOT NULL
);
CREATE INDEX IF NOT EXISTS incident_resource_requests_scope_idx ON crab.incident_resource_requests(incident_id,compartment,recorded_at DESC);
ALTER TABLE crab.blobs DROP CONSTRAINT IF EXISTS blobs_incident_id_check;
ALTER TABLE crab.blobs DROP CONSTRAINT IF EXISTS blobs_mode_check;
ALTER TABLE crab.blobs ADD CONSTRAINT blobs_incident_id_check CHECK ((mode IN ('live','exercise') AND btrim(incident_id) <> '') OR (mode = 'system' AND incident_id = ''));
ALTER TABLE crab.blobs ADD CONSTRAINT blobs_mode_check CHECK (mode IN ('live','exercise','system'));
INSERT INTO crab.schema_migrations(version) VALUES (15) ON CONFLICT (version) DO NOTHING;
