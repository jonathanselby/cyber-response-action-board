-- Slice 4: richer Action projection, governed checklist templates and incident timeline.

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

CREATE TABLE IF NOT EXISTS crab.checklist_templates (
    record_id text PRIMARY KEY,
    version text NOT NULL,
    name text NOT NULL,
    source_ref text NOT NULL,
    payload jsonb NOT NULL,
    maker text NOT NULL,
    confirmer text NOT NULL,
    confirmed_at timestamptz NOT NULL
);
COMMENT ON TABLE crab.checklist_templates IS 'Rebuildable current projection of confirmed governed checklist templates.';

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

INSERT INTO crab.schema_migrations(version) VALUES (8) ON CONFLICT (version) DO NOTHING;
