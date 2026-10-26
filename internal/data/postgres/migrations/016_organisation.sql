-- UAT Phase 2: governed organisation configuration (severity scale, emergency
-- configuration, checklist templates, single-person windows). Rebuildable
-- projections of SYSTEM ledger records; the ledger stays authoritative.

CREATE TABLE IF NOT EXISTS crab.organisation_proposals (
    record_id text PRIMARY KEY CHECK (btrim(record_id) <> ''),
    record_type text NOT NULL CHECK (btrim(record_type) <> ''),
    version text NOT NULL,
    maker text NOT NULL CHECK (btrim(maker) <> ''),
    payload jsonb NOT NULL,
    proposed_at timestamptz NOT NULL
);
COMMENT ON TABLE crab.organisation_proposals IS
    'CRAB authority=rebuildable; pending organisation configuration proposals';

CREATE TABLE IF NOT EXISTS crab.organisation_records (
    record_type text NOT NULL CHECK (btrim(record_type) <> ''),
    record_key text NOT NULL CHECK (btrim(record_key) <> ''),
    record_id text NOT NULL,
    version text NOT NULL,
    payload jsonb NOT NULL,
    maker text NOT NULL,
    confirmer text NOT NULL,
    confirmed_at timestamptz NOT NULL,
    time_status text NOT NULL,
    self_approval boolean NOT NULL DEFAULT false,
    PRIMARY KEY (record_type, record_key)
);
COMMENT ON TABLE crab.organisation_records IS
    'CRAB authority=rebuildable; current confirmed organisation configuration, one row per type and key';

INSERT INTO crab.schema_migrations(version) VALUES (16) ON CONFLICT (version) DO NOTHING;
