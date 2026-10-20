-- Slice 8: governed Markdown/DOCX corpus and deterministic retrieval projections.
-- Immutable corpus bytes reuse crab.blobs/blob_chunks. Index text is rebuildable.

ALTER TABLE crab.blobs DROP CONSTRAINT IF EXISTS blobs_incident_id_check;
ALTER TABLE crab.blobs DROP CONSTRAINT IF EXISTS blobs_mode_check;
ALTER TABLE crab.blobs DROP CONSTRAINT IF EXISTS blobs_scope_check;
ALTER TABLE crab.blobs ADD CONSTRAINT blobs_scope_check CHECK (
    (mode = 'system' AND incident_id = '') OR
    (mode IN ('live','exercise') AND btrim(incident_id) <> '')
);

CREATE TABLE IF NOT EXISTS crab.corpus_proposals (
    source_id text PRIMARY KEY CHECK (btrim(source_id) <> ''),
    incident_id text NOT NULL DEFAULT '',
    mode text NOT NULL CHECK (mode IN ('system','live','exercise')),
    compartment text NOT NULL CHECK (btrim(compartment) <> ''),
    version text NOT NULL,
    maker text NOT NULL,
    payload bytea NOT NULL,
    proposed_at timestamptz NOT NULL,
    time_status text NOT NULL CHECK (time_status IN ('trusted','drifting','untrusted')),
    CONSTRAINT corpus_proposals_scope_check CHECK (
        (mode = 'system' AND incident_id = '') OR
        (mode IN ('live','exercise') AND btrim(incident_id) <> '')
    )
);
COMMENT ON TABLE crab.corpus_proposals IS
    'CRAB authority=rebuildable; exact pending governed corpus-source proposals';

CREATE TABLE IF NOT EXISTS crab.corpus_sources (
    source_id text PRIMARY KEY CHECK (btrim(source_id) <> ''),
    incident_id text NOT NULL DEFAULT '',
    mode text NOT NULL CHECK (mode IN ('system','live','exercise')),
    compartment text NOT NULL CHECK (btrim(compartment) <> ''),
    blob_id text NOT NULL REFERENCES crab.blobs(blob_id) ON DELETE RESTRICT,
    sha256 text NOT NULL CHECK (sha256 ~ '^[0-9a-f]{64}$'),
    size_bytes bigint NOT NULL CHECK (size_bytes BETWEEN 0 AND 16777216),
    original_filename text NOT NULL,
    declared_media_type text NOT NULL DEFAULT '',
    detected_media_type text NOT NULL,
    title text NOT NULL,
    format text NOT NULL CHECK (format IN ('markdown','docx')),
    maker text NOT NULL,
    confirmer text NOT NULL,
    confirmed_at timestamptz NOT NULL,
    CONSTRAINT corpus_sources_scope_check CHECK (
        (mode = 'system' AND incident_id = '') OR
        (mode IN ('live','exercise') AND btrim(incident_id) <> '')
    )
);
COMMENT ON TABLE crab.corpus_sources IS
    'CRAB authority=rebuildable; current confirmed corpus-source metadata';

CREATE TABLE IF NOT EXISTS crab.corpus_passages (
    source_id text NOT NULL REFERENCES crab.corpus_sources(source_id) ON DELETE CASCADE,
    ordinal integer NOT NULL CHECK (ordinal > 0),
    source_sha256 text NOT NULL CHECK (source_sha256 ~ '^[0-9a-f]{64}$'),
    locator text NOT NULL CHECK (btrim(locator) <> ''),
    passage text NOT NULL CHECK (btrim(passage) <> ''),
    terms text[] NOT NULL,
    PRIMARY KEY (source_id, ordinal)
);
COMMENT ON TABLE crab.corpus_passages IS
    'CRAB authority=rebuildable; deterministic extracted text and lexical terms';

CREATE INDEX IF NOT EXISTS corpus_sources_scope_idx
    ON crab.corpus_sources(incident_id, compartment, source_id);

INSERT INTO crab.schema_migrations(version) VALUES (12)
ON CONFLICT (version) DO NOTHING;
