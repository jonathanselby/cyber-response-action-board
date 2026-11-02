-- CRAB migration 019: governed incident reopening (contract addendum A1).
--
-- Authority classification: the new columns are part of the rebuildable
-- recovery-state projection. A confirmed incident.reopening supersedes the
-- current closure and stand-down; the superseded closure is kept here for
-- display and in the ledger for history.

ALTER TABLE crab.incident_recovery_state
    ADD COLUMN IF NOT EXISTS reopen_record_id text,
    ADD COLUMN IF NOT EXISTS reopened_at timestamptz,
    ADD COLUMN IF NOT EXISTS reopen_payload jsonb,
    ADD COLUMN IF NOT EXISTS superseded_closure_record_id text,
    ADD COLUMN IF NOT EXISTS superseded_closed_at timestamptz,
    ADD COLUMN IF NOT EXISTS superseded_closure_payload jsonb;

-- The closed-incident gate admits exactly one record type: the governed
-- reopening (proposal and decision). Everything else stays read-only.
CREATE OR REPLACE FUNCTION crab.reject_closed_incident_ledger_append() RETURNS trigger
LANGUAGE plpgsql AS $$
DECLARE
    envelope jsonb;
    scoped_incident text;
BEGIN
    envelope := convert_from(NEW.entry_bytes, 'UTF8')::jsonb;
    scoped_incident := envelope #>> '{record,incident_id}';
    IF scoped_incident IS NOT NULL AND scoped_incident <> ''
       AND COALESCE(envelope #>> '{record,record_type}', '') <> 'incident.reopening'
       AND EXISTS (
        SELECT 1 FROM crab.incident_recovery_state
        WHERE incident_id = scoped_incident AND closed_at IS NOT NULL
    ) THEN
        RAISE EXCEPTION 'incident is closed and read-only';
    END IF;
    RETURN NEW;
END;
$$;

INSERT INTO crab.schema_migrations(version) VALUES (19)
ON CONFLICT (version) DO NOTHING;
