# CRAB build evidence — 0.0.12-dev

## Slice

Slice 11 — print and paper resilience.

Frozen product/design documents were not changed.

## Source checkpoint

The completed Slice 11 source and acceptance set reached `main` before promotion. GitHub Actions run **#141 / 39388672094** completed green with:

- module contract;
- gofmt contract;
- `go vet ./...`;
- full `go test -race -count=1 ./...` including PostgreSQL integration tests;
- Linux amd64 build;
- Linux arm64 build;
- existing explicit core checkpoint acceptance.

Earlier substantive checkpoints #139 and #140 were also fully green after adding paper-reconciliation/PostgreSQL proof and the hardened pagination/wrapping renderer respectively.

## Implemented boundary

### Authorised composition

Print is a separate output action. The application service checks `ActionPrint` before reading incident content for print composition. Incident and compartment scope remain effective, including restricted-compartment read-in.

The application composes typed print documents; the PDF package performs typography/layout only and contains no SQL or authorisation policy.

### Operational print set

Implemented local rendering for:

- Incident Brief;
- Emergency Response Pack;
- evidence register;
- incident timeline;
- review/closure material;
- five blank contingency forms: incident, decision, action, communication and evidence.

The normal Incident Brief acceptance fixture renders within the frozen 1–2 page target. The implementation does not refuse a genuine larger incident merely because current authorised content needs more pages.

### Renderer spike

The gopdf renderer passed the frozen spike:

- pure Go;
- no headless browser;
- no runtime network dependency;
- embedded Go Regular TTF;
- A4 and US Letter;
- colour and monochrome;
- measured wrapping with hard-token splitting;
- controlled page breaks;
- page/source footers;
- visible exercise marking on every page;
- deterministic PDF bytes for fixed inputs;
- amd64 and arm64 CI builds.

Decision details are in `docs/pdf-renderer-spike-decision.md`.

### Provenance

Operational PDFs carry a CRAB-generated source mark containing current ledger sequence/hash, configuration version, generation time and time-trust state. The Emergency Response Pack overwrites caller-supplied source metadata with the source returned by CRAB.

### Last-good derived copies

The derived-file adapter writes a same-directory temporary file, fsyncs, closes and atomically renames it over the target. A failed primary refresh preserves the previous primary file. An optional secondary target is refreshed after primary success; secondary failure returns a visible warning and preserves the previous secondary copy.

Generated PDFs remain ordinary files and can be read without PostgreSQL.

### Paper reconciliation

Paper reconciliation adds no table or paper database. A paper event is an ordinary immutable ledger `Record(...)` with:

- paper occurrence time in the payload/timeline occurrence field;
- actual later CRAB transcription time as the ledger/digital recorded time;
- named transcriber as maker;
- explicit trusted/drifting/untrusted time confidence.

`PaperProjection` contributes that record to the existing incident timeline. Combined ledger rebuild restores the event with both times unchanged; the digital record is never backdated.

## Acceptance evidence

Slice 11 tests cover the frozen AC-37–42 and print-security intent in AC-66, including:

- separate print permission and restricted-compartment denial before incident-content read;
- current-state Incident Brief composition and source mark;
- caller provenance replacement for Emergency Response Pack;
- A4 and Letter rendering;
- monochrome and exercise-safe meaning;
- deterministic renderer output;
- long opaque references wrapping inside printable width;
- primary/secondary atomic last-good behaviour;
- secondary-unavailable warning while primary succeeds;
- renderer failure causing no derived-copy write;
- generated-file readability without database access;
- paper occurrence time kept separate from digital transcription time;
- projection rebuild preserving paper event/time trust;
- current ledger position used for print source marking.

## Explicitly not claimed

`crab print` remains fail-closed because the current CLI has no authenticated operator/session path that can provide a real principal and separate print permission. Slice 11 does not introduce an unauthenticated shortcut. Operator-facing print wiring remains a later presentation/composition concern.

No print database, print queue, print daemon, headless browser or secondary presentation architecture was introduced.

No release or tag should be created for this development checkpoint.
