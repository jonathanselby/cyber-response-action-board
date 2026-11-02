-- CRAB migration 020: machine first-report intake (contract addendum A4).
--
-- Authority classification:
--   crab.intake_keys          rebuildable projection of confirmed security.intake_key records.
--   crab.intake_key_secrets   durable supporting: SHA-256 hashes of intake key secrets. A
--                             secret is never stored in clear or in the ledger. After a
--                             rotation the previous secret works until valid_until (<= 24 h).
--
-- A first report submitted with an external_ref is unique per intake key, so a
-- retried submission can never be committed twice.

CREATE TABLE IF NOT EXISTS crab.intake_keys (
    key_name text PRIMARY KEY CHECK (btrim(key_name) <> ''),
    accountable_human text NOT NULL CHECK (btrim(accountable_human) <> ''),
    status text NOT NULL CHECK (status IN ('active','revoked')),
    expires_at timestamptz NOT NULL,
    issued_at timestamptz NOT NULL,
    rotated_at timestamptz,
    updated_at timestamptz NOT NULL
);
COMMENT ON TABLE crab.intake_keys IS
    'CRAB authority=rebuildable; intake key state from confirmed security.intake_key records';

CREATE TABLE IF NOT EXISTS crab.intake_key_secrets (
    secret_hash bytea PRIMARY KEY CHECK (length(secret_hash) = 32),
    key_name text NOT NULL CHECK (btrim(key_name) <> ''),
    valid_until timestamptz,
    created_at timestamptz NOT NULL
);
CREATE INDEX IF NOT EXISTS intake_key_secrets_key ON crab.intake_key_secrets(key_name);
COMMENT ON TABLE crab.intake_key_secrets IS
    'CRAB authority=supporting; hashed intake key secrets, never in the ledger';

CREATE UNIQUE INDEX IF NOT EXISTS first_reports_intake_ref
    ON crab.first_reports ((payload->>'intake_key'), (payload->>'external_ref'))
    WHERE payload ? 'external_ref';

INSERT INTO crab.schema_migrations(version) VALUES (20)
ON CONFLICT (version) DO NOTHING;
