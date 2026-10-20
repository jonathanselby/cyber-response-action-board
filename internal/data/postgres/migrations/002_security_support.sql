-- CRAB migration 002: local authentication and session supporting state.
--
-- Authority classification:
--   crab.local_credentials  durable supporting - local credential verification material.
--   crab.sessions           durable supporting - short-lived hashed authentication sessions.
--
-- Neither table is authoritative incident/business state and neither grants role,
-- incident membership, compartment membership or output authority.
-- Credential lifecycle remains subject to the application governance boundary.
-- Session bearer tokens are never stored: only their SHA-256 digests are persisted.

CREATE TABLE IF NOT EXISTS crab.local_credentials (
    human_id text PRIMARY KEY CHECK (btrim(human_id) <> ''),
    salt text NOT NULL CHECK (salt <> ''),
    iterations integer NOT NULL CHECK (iterations >= 600000),
    password_hash text NOT NULL CHECK (password_hash <> ''),
    updated_at timestamptz NOT NULL DEFAULT now()
);
COMMENT ON TABLE crab.local_credentials IS
    'CRAB authority=durable-supporting; local password verification material; no authorisation state';

CREATE TABLE IF NOT EXISTS crab.sessions (
    token_digest bytea PRIMARY KEY CHECK (octet_length(token_digest) = 32),
    human_id text NOT NULL CHECK (btrim(human_id) <> ''),
    auth_method text NOT NULL CHECK (btrim(auth_method) <> ''),
    break_glass boolean NOT NULL DEFAULT false,
    created_at timestamptz NOT NULL,
    expires_at timestamptz NOT NULL,
    CHECK (expires_at > created_at)
);
COMMENT ON TABLE crab.sessions IS
    'CRAB authority=durable-supporting; short-lived authentication session; bearer token itself is never stored';

CREATE INDEX IF NOT EXISTS sessions_expires_at_idx ON crab.sessions(expires_at);

INSERT INTO crab.schema_migrations(version) VALUES (2)
ON CONFLICT (version) DO NOTHING;
