-- CRAB migration 004: offline-capable authentication credential support.
--
-- Authority classification: durable supporting security state only.
-- These tables do not grant CRAB roles, incident access, compartments or outputs.

CREATE TABLE IF NOT EXISTS crab.totp_credentials (
    human_id text PRIMARY KEY CHECK (btrim(human_id) <> ''),
    secret_nonce bytea NOT NULL,
    secret_ciphertext bytea NOT NULL,
    totp_window integer NOT NULL DEFAULT 1 CHECK (totp_window >= 0 AND totp_window <= 2),
    updated_at timestamptz NOT NULL DEFAULT now()
);
COMMENT ON TABLE crab.totp_credentials IS
    'CRAB authority=durable-supporting; TOTP secret encrypted using deployment credential-wrap key';

CREATE TABLE IF NOT EXISTS crab.passkey_credentials (
    human_id text NOT NULL CHECK (btrim(human_id) <> ''),
    credential_id text NOT NULL CHECK (btrim(credential_id) <> ''),
    rp_id text NOT NULL CHECK (btrim(rp_id) <> ''),
    origin text NOT NULL CHECK (btrim(origin) <> ''),
    public_key_pem text NOT NULL CHECK (btrim(public_key_pem) <> ''),
    require_user_verification boolean NOT NULL DEFAULT true,
    sign_count bigint NOT NULL DEFAULT 0 CHECK (sign_count >= 0 AND sign_count <= 4294967295),
    PRIMARY KEY (human_id, credential_id)
);
COMMENT ON TABLE crab.passkey_credentials IS
    'CRAB authority=durable-supporting; public passkey verification material and counter';

CREATE TABLE IF NOT EXISTS crab.break_glass_credentials (
    keyholder_id text PRIMARY KEY CHECK (btrim(keyholder_id) <> ''),
    salt text NOT NULL CHECK (btrim(salt) <> ''),
    iterations integer NOT NULL CHECK (iterations >= 600000),
    key_hash text NOT NULL CHECK (btrim(key_hash) <> ''),
    active boolean NOT NULL DEFAULT true,
    updated_at timestamptz NOT NULL DEFAULT now()
);
COMMENT ON TABLE crab.break_glass_credentials IS
    'CRAB authority=durable-supporting; slow salted break-glass verification material; individually revocable';

INSERT INTO crab.schema_migrations(version) VALUES (4)
ON CONFLICT (version) DO NOTHING;
