-- CRAB migration 007: preserve proposal incident mode in the pending projection.
--
-- The table remains rebuildable. Existing pre-007 pending rows, if any in a
-- development database, are intentionally left NULL and become usable again
-- after the normal projection rebuild rather than guessing live/exercise mode.

ALTER TABLE crab.incident_proposals
    ADD COLUMN IF NOT EXISTS mode text CHECK (mode IN ('live', 'exercise'));

COMMENT ON COLUMN crab.incident_proposals.mode IS
    'Rebuildable copy of the authoritative proposal incident mode; never inferred during confirmation';

INSERT INTO crab.schema_migrations(version) VALUES (7)
ON CONFLICT (version) DO NOTHING;
