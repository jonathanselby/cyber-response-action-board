-- CRAB migration 005: governed installation security-state projection.
--
-- Authority classification: rebuildable projection only. The signed ledger is
-- authoritative. The table is deliberately key/value-small, not a generic
-- business-configuration bypass.

CREATE TABLE IF NOT EXISTS crab.security_state (
    state_key text PRIMARY KEY CHECK (state_key IN ('isolate_mode')),
    bool_value boolean NOT NULL
);
COMMENT ON TABLE crab.security_state IS
    'CRAB authority=rebuildable; small current installation-security state projected from confirmed ledger records';

INSERT INTO crab.schema_migrations(version) VALUES (5)
ON CONFLICT (version) DO NOTHING;
