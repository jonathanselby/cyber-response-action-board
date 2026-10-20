# CRAB build evidence — 0.0.9-dev

## Checkpoint

Slice 8 — governed corpus and deterministic retrieval.

Source checkpoint:

- substantive source commit: `5ec57c45009925b7ce50d72e153764c74332af85`
- formatting-only correction: `be36e55e3164a36cc95954f20499732dc3660f45`
- source validation run: GitHub Actions #106 / `39377040171` — green
- earlier run #105 / `39376912456` failed only the formatting contract for one alignment change in `internal/domain/retrieval.go`; no test, vet or architecture failure was reached before that gate stopped the run.

The promotion/final run is recorded after this document is committed.

## Built

- Governed Markdown and DOCX corpus admission using the existing maker-checker ledger semantics.
- Immutable original source bytes stored through the Slice 7 PostgreSQL blob primitive rather than a filesystem, object store or search service.
- A corpus-specific 16 MiB streaming bound, independent of the larger evidence product limit.
- Bounded DOCX ZIP handling: at most 512 entries, 64 MiB aggregate expanded metadata ceiling and a 16 MiB `word/document.xml` ceiling.
- Passive DOCX extraction from `word/document.xml` only. Macros and embedded active content are not executed or indexed.
- Deterministic Markdown line-range and DOCX paragraph locators.
- Rebuildable confirmed corpus-source and passage projections with stable source SHA-256 provenance.
- PostgreSQL scope filtering before candidate passages reach the lexical ranker.
- Deterministic lexical ranking with stable tie-breaks and bounded result count.
- Stable citation IDs bound to source ID, immutable source SHA-256 and deterministic locator.
- Explicit refusal when no authorised passage supports the query.
- No embedding model, vector database, local LLM, network search or second persistence engine.
- Closure of the frozen declaration invariant: a confirmed incident declaration derives the declaration maker's initial Sponsor, incident and `GENERAL` membership state from the declaration ledger record.

## Security and functional proof

The Slice 8 tests prove:

- Markdown and DOCX extraction is deterministic.
- Embedded DOCX macro bytes do not enter retrieval text and are never executed.
- ZIP entry limits fail closed.
- unsupported queries return refusal without citations.
- citation ordering and IDs are deterministic.
- a restricted-compartment passage with a stronger lexical match is excluded before ranking for a GENERAL-only responder.
- another incident's stronger lexical match is excluded before ranking.
- authorised restricted-compartment retrieval still works for a read-in responder.
- corpus projection rebuild produces the same citation while immutable source bytes remain untouched.
- the runtime role cannot update immutable corpus blob metadata.
- confirmed incident declaration establishes initial Sponsor and membership state and that derivation survives the combined access/incident rebuild.

The full source gate additionally passed `go mod tidy` contract, `gofmt`, `go vet`, `go test -race -count=1 ./...`, Linux amd64 build, Linux arm64 build and the existing core checkpoint acceptance suite.

## Deliberate boundary

Slice 8 does not add model drafting. The deterministic retrieval result is the governed input boundary for Slice 9. It also does not add an unauthenticated browser upload/search route; secure presentation composition remains separate from the application/domain capability.

No release or tag is created for this development checkpoint.
