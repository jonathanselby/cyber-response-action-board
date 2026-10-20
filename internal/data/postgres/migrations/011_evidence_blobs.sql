-- Slice 7: immutable hostile-byte storage and rebuildable evidence register.
-- crab.blobs/blob_chunks are durable supporting state, not ledger projections.
-- evidence_register/evidence_handling are rebuildable from ledger history.

CREATE TABLE IF NOT EXISTS crab.blobs (
    blob_id text PRIMARY KEY CHECK (btrim(blob_id) <> ''),
    incident_id text NOT NULL CHECK (btrim(incident_id) <> ''),
    mode text NOT NULL CHECK (mode IN ('live','exercise')),
    compartment text NOT NULL CHECK (btrim(compartment) <> ''),
    sha256 text NOT NULL CHECK (sha256 ~ '^[0-9a-f]{64}$'),
    size_bytes bigint NOT NULL CHECK (size_bytes >= 0),
    original_filename text NOT NULL CHECK (btrim(original_filename) <> ''),
    declared_media_type text NOT NULL DEFAULT '',
    detected_media_type text NOT NULL CHECK (btrim(detected_media_type) <> ''),
    created_at timestamptz NOT NULL
);
COMMENT ON TABLE crab.blobs IS
    'CRAB authority=durable-supporting; immutable opaque per-upload blobs; no cross-incident deduplication';

CREATE TABLE IF NOT EXISTS crab.blob_chunks (
    blob_id text NOT NULL,
    chunk_no integer NOT NULL CHECK (chunk_no >= 0),
    bytes bytea NOT NULL CHECK (octet_length(bytes) BETWEEN 1 AND 65536),
    PRIMARY KEY (blob_id, chunk_no),
    CONSTRAINT blob_chunks_blob_fk FOREIGN KEY (blob_id) REFERENCES crab.blobs(blob_id)
        ON DELETE RESTRICT DEFERRABLE INITIALLY DEFERRED
);
COMMENT ON TABLE crab.blob_chunks IS
    'CRAB authority=durable-supporting; exact untrusted bytes in bounded 64KiB chunks';

CREATE TABLE IF NOT EXISTS crab.evidence_register (
    evidence_id text PRIMARY KEY CHECK (btrim(evidence_id) <> ''),
    incident_id text NOT NULL CHECK (btrim(incident_id) <> ''),
    mode text NOT NULL CHECK (mode IN ('live','exercise')),
    compartment text NOT NULL CHECK (btrim(compartment) <> ''),
    blob_id text NOT NULL REFERENCES crab.blobs(blob_id) ON DELETE RESTRICT,
    sha256 text NOT NULL CHECK (sha256 ~ '^[0-9a-f]{64}$'),
    size_bytes bigint NOT NULL CHECK (size_bytes >= 0),
    original_filename text NOT NULL,
    declared_media_type text NOT NULL DEFAULT '',
    detected_media_type text NOT NULL,
    description text NOT NULL DEFAULT '',
    collector text NOT NULL,
    source text NOT NULL,
    collected_at timestamptz,
    occurred_at timestamptz,
    source_location text NOT NULL DEFAULT '',
    current_handler text NOT NULL,
    related_ref text NOT NULL DEFAULT '',
    recorded_at timestamptz NOT NULL,
    time_status text NOT NULL CHECK (time_status IN ('trusted','drifting','untrusted'))
);
COMMENT ON TABLE crab.evidence_register IS
    'CRAB authority=rebuildable; current evidence register projected from immutable ledger history';

CREATE TABLE IF NOT EXISTS crab.evidence_handling (
    record_id text PRIMARY KEY CHECK (btrim(record_id) <> ''),
    evidence_id text NOT NULL REFERENCES crab.evidence_register(evidence_id) ON DELETE CASCADE,
    incident_id text NOT NULL,
    mode text NOT NULL CHECK (mode IN ('live','exercise')),
    compartment text NOT NULL,
    action text NOT NULL,
    handler text NOT NULL,
    location text NOT NULL DEFAULT '',
    note text NOT NULL DEFAULT '',
    occurred_at timestamptz,
    recorded_at timestamptz NOT NULL,
    time_status text NOT NULL CHECK (time_status IN ('trusted','drifting','untrusted'))
);
COMMENT ON TABLE crab.evidence_handling IS
    'CRAB authority=rebuildable; immutable evidence handling history without forensic-chain-of-custody claim';

INSERT INTO crab.schema_migrations(version) VALUES (11)
ON CONFLICT (version) DO NOTHING;
