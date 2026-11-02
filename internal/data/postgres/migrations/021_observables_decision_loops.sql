-- CRAB migration 021: observables (addendum A2) and decision-loop times for
-- the management summary (addendum A3).
--
-- Authority classification:
--   crab.observable_versions  rebuildable projection: every version of every
--                             incident.observable record. The current version
--                             is the highest version of an observable.
--   crab.decision_loops       rebuildable projection: proposal and decision time
--                             of each incident-scoped governed record.

CREATE TABLE IF NOT EXISTS crab.observable_versions (
    record_id text PRIMARY KEY CHECK (btrim(record_id) <> ''),
    observable_id text NOT NULL CHECK (btrim(observable_id) <> ''),
    version integer NOT NULL CHECK (version > 0),
    incident_id text NOT NULL CHECK (btrim(incident_id) <> ''),
    mode text NOT NULL CHECK (mode IN ('live','exercise')),
    compartment text NOT NULL CHECK (btrim(compartment) <> ''),
    type text NOT NULL CHECK (type IN ('ip','domain','url','hostname','email','hash')),
    value text NOT NULL CHECK (btrim(value) <> ''),
    status text NOT NULL CHECK (status IN ('suspected','confirmed','benign','blocked')),
    source text NOT NULL,
    first_seen timestamptz NOT NULL,
    time_confidence text NOT NULL,
    note text NOT NULL DEFAULT '',
    maker text NOT NULL,
    recorded_at timestamptz NOT NULL,
    UNIQUE (observable_id, version)
);
CREATE INDEX IF NOT EXISTS observable_versions_incident ON crab.observable_versions(incident_id, compartment);
CREATE INDEX IF NOT EXISTS observable_versions_value ON crab.observable_versions(type, value);
COMMENT ON TABLE crab.observable_versions IS
    'CRAB authority=rebuildable; every version of incident.observable records';

CREATE TABLE IF NOT EXISTS crab.decision_loops (
    record_type text NOT NULL CHECK (btrim(record_type) <> ''),
    record_id text NOT NULL CHECK (btrim(record_id) <> ''),
    incident_id text NOT NULL CHECK (btrim(incident_id) <> ''),
    mode text NOT NULL,
    compartment text NOT NULL,
    proposed_at timestamptz NOT NULL,
    decided_at timestamptz,
    outcome text,
    PRIMARY KEY (record_type, record_id)
);
CREATE INDEX IF NOT EXISTS decision_loops_decided ON crab.decision_loops(decided_at) WHERE outcome = 'confirmed';
COMMENT ON TABLE crab.decision_loops IS
    'CRAB authority=rebuildable; proposal and decision time of incident-scoped governed records';

-- Backfill from the ledger so an upgraded deployment has its history at once.
-- A later rebuild produces the same rows. Ledger times carry nanoseconds;
-- they are truncated to microseconds as the Go driver does, not rounded.
-- Migrations run on every start, so the backfill runs only before version 21
-- is recorded.
DO $backfill$
BEGIN
IF NOT EXISTS (SELECT 1 FROM crab.schema_migrations WHERE version = 21) THEN
INSERT INTO crab.decision_loops(record_type, record_id, incident_id, mode, compartment, proposed_at)
SELECT DISTINCT ON (e->'record'->>'record_type', e->'record'->>'record_id')
       e->'record'->>'record_type', e->'record'->>'record_id', e->'record'->>'incident_id', e->'record'->>'incident_mode',
       e->'record'->>'compartment', regexp_replace(e->'record'->>'timestamp', '(\.[0-9]{6})[0-9]+', '\1')::timestamptz
FROM (SELECT convert_from(entry_bytes, 'UTF8')::jsonb AS e, seq FROM crab.ledger_entries) l
WHERE e->'record'->>'write_semantic' = 'governed' AND e->'record'->>'status' = 'proposed'
  AND COALESCE(e->'record'->>'incident_id', '') <> ''
ORDER BY e->'record'->>'record_type', e->'record'->>'record_id', seq DESC
ON CONFLICT (record_type, record_id) DO NOTHING;

UPDATE crab.decision_loops d SET decided_at = x.decided_at, outcome = x.outcome
FROM (
    SELECT DISTINCT ON (e->'record'->>'record_type', e->'record'->>'record_id')
           e->'record'->>'record_type' AS record_type, e->'record'->>'record_id' AS record_id,
           regexp_replace(e->'record'->>'timestamp', '(\.[0-9]{6})[0-9]+', '\1')::timestamptz AS decided_at, e->'record'->>'status' AS outcome
    FROM (SELECT convert_from(entry_bytes, 'UTF8')::jsonb AS e, seq FROM crab.ledger_entries) l
    WHERE e->'record'->>'write_semantic' = 'governed' AND e->'record'->>'status' IN ('confirmed','rejected')
      AND COALESCE(e->'record'->>'incident_id', '') <> ''
    ORDER BY e->'record'->>'record_type', e->'record'->>'record_id', seq DESC
) x
WHERE d.record_type = x.record_type AND d.record_id = x.record_id;
END IF;
END
$backfill$;

INSERT INTO crab.schema_migrations(version) VALUES (21)
ON CONFLICT (version) DO NOTHING;
