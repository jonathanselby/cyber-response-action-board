-- CRAB migration 018: identity lifecycle (post-UAT remediation R-01).
--
-- Authority classification:
--   crab.identities              rebuildable projection of confirmed security.identity records.
--   crab.local_credentials (+)   durable supporting: one-time enrolment codes are stored as
--                                ordinary hashed credentials that must be replaced on first use.
--   crab.totp_credentials (+)    durable supporting: a TOTP secret is pending until confirmed.
--
-- No contact address is stored here or in the ledger.

CREATE TABLE IF NOT EXISTS crab.identities (
    human_id text PRIMARY KEY CHECK (btrim(human_id) <> ''),
    display_name text NOT NULL CHECK (btrim(display_name) <> ''),
    organisation text NOT NULL DEFAULT '',
    external boolean NOT NULL DEFAULT false,
    status text NOT NULL CHECK (status IN ('active','suspended','disabled')),
    account_expires_at timestamptz,
    reason text NOT NULL DEFAULT '',
    updated_at timestamptz NOT NULL
);
COMMENT ON TABLE crab.identities IS
    'CRAB authority=rebuildable; current identity and account state from confirmed security.identity records';

ALTER TABLE crab.local_credentials
    ADD COLUMN IF NOT EXISTS must_set_password boolean NOT NULL DEFAULT false,
    ADD COLUMN IF NOT EXISTS expires_at timestamptz;

ALTER TABLE crab.totp_credentials
    ADD COLUMN IF NOT EXISTS confirmed boolean NOT NULL DEFAULT true;

INSERT INTO crab.schema_migrations(version) VALUES (18)
ON CONFLICT (version) DO NOTHING;
