# CRAB build evidence — 0.0.8-dev

## Checkpoint

**Slice 7 — hostile evidence and immutable blobs**

Substantive source head before promotion: `0577ff60d1bc5ac66796a2c05ddd3dbf8e3ed9d5`.

Substantive CI run: **#102 / `39374366672`**.

Result: module contract, gofmt, vet, full `go test -race -count=1 ./...`, linux/amd64 build, linux/arm64 build and existing focused checkpoint all passed.

Promotion head: `c14a44ffdd451d3c73f985b0b7131cf75c40aabd`.

Promotion CI run: **#103 / `39374638490`**. The expanded focused gate explicitly includes the Slice 7 evidence, corruption, safe-inline and no-deduplication tests.

## Built

- PostgreSQL `crab.blobs` metadata plus bounded 64 KiB `crab.blob_chunks` durable-supporting rows.
- SHA-256 calculated while the untrusted input stream is stored; whole files are not buffered in application memory.
- opaque random per-upload blob identifiers rather than content-addressed identifiers;
- no cross-incident deduplication;
- configured maximum-byte refusal and transaction rollback on oversized/interrupted streams;
- runtime insert/read-only privileges on durable blob rows; no runtime update/delete;
- immutable incident evidence records that exactly bind blob metadata to incident/mode/compartment;
- evidence register and immutable handling-history projections;
- current-handler derivation from handling history;
- exact-byte integrity verification and verified retrieval;
- evidence detail/register/history scope enforcement and separate attachment-download authorisation;
- fail-closed response policy for active/ambiguous/mislabelled media;
- `nosniff`, `no-store`, safe attachment MIME and sanitised response filenames;
- projection rebuild of evidence metadata/history without rebuilding or rewriting blob bytes;
- readiness and version checkpoint `0.0.8-dev`;
- safe-inline decision record in `docs/safe-inline-media-spike.md`.

## Security/failure proof

The Slice 7 tests prove:

- byte-for-byte retrieval matches the stored upload;
- SHA-256 and size are stable and checked;
- ordinary read does not imply attachment download;
- a guessed evidence ID from another incident does not bypass authoritative scope;
- identical bytes in separate incidents receive separate blob IDs;
- runtime cannot modify/delete committed blob rows;
- interrupted upload and oversized upload leave no blob/chunk/evidence/ledger residue;
- privileged chunk tampering is detected before any corrupt bytes are emitted;
- HTML disguised as an image is download-only;
- SVG/PDF and MIME mismatch are download-only;
- filename/path/control-character tricks cannot escape the attachment header policy;
- LEGAL evidence does not appear in a GENERAL register;
- evidence register/handling history rebuild from ledger while the durable blob remains intact.

## Boundary retained

CRAB stores hostile bytes; it does not execute, transcode or analyse them. This checkpoint does not add malware scanning, thumbnails, media transformation, object storage or certified forensic chain-of-custody claims.

The evidence capture application path accepts an `io.Reader`, so a phone/browser multipart file stream can be passed without whole-file buffering. The authenticated browser route/session composition remains later presentation work; Slice 7 does not create an unauthenticated evidence-upload shortcut.

No release or tag was created.
