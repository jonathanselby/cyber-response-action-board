-- CRAB migration 001: Slice 1 ledger kernel.
--
-- Authority classification:
--   crab.schema_migrations  supporting  - operational schema metadata.
--   crab.ledger_entries     authoritative - signed/hash-chained audit history.
--
-- Compatibility: additive initial schema.
-- Replay impact: establishes the authoritative source replayed by later projections.
-- Backup/restore impact: ledger_entries must be restored exactly.
-- Portable verifier format: unchanged; entry_bytes are portable JSONL records without the newline.
-- Live migration: initial build only; later migrations must state their own maintenance requirement.

CREATE SCHEMA IF NOT EXISTS crab;

CREATE TABLE IF NOT EXISTS crab.schema_migrations (
    version integer PRIMARY KEY,
    applied_at timestamptz NOT NULL DEFAULT now()
);
COMMENT ON TABLE crab.schema_migrations IS
    'CRAB authority=supporting; migration metadata; not business state';

CREATE TABLE IF NOT EXISTS crab.ledger_entries (
    seq bigint PRIMARY KEY CHECK (seq > 0),
    prev_hash text NOT NULL,
    hash text NOT NULL UNIQUE,
    entry_bytes bytea NOT NULL,
    committed_at timestamptz NOT NULL DEFAULT now(),
    CHECK (octet_length(entry_bytes) > 0 AND octet_length(entry_bytes) <= 4194304),
    CHECK ((seq = 1 AND prev_hash = '') OR (seq > 1 AND length(prev_hash) = 64)),
    CHECK (length(hash) = 64)
);
COMMENT ON TABLE crab.ledger_entries IS
    'CRAB authority=authoritative; signed append-only ledger; never repaired by editing history';

INSERT INTO crab.schema_migrations(version) VALUES (1)
ON CONFLICT (version) DO NOTHING;
