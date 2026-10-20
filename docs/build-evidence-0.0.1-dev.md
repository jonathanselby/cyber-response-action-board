# CRAB build evidence — 0.0.1-dev

**Date:** 20 October 2026  
**Scope:** first executable build checkpoint only

## Frozen sources used

- PRD V0.2 — product contract.
- EDD V0.4 FINAL — architecture/security baseline.
- CRAB Build Notes V0.1.
- CRAB Test and Acceptance Notes V0.1.
- CRAB UX and Print Build Notes V0.1.
- Garage GitHub Publication Learnings.

## Implemented checkpoint

This checkpoint deliberately implements only capability that can be completed without inventing a temporary persistence or security bypass:

1. one Go executable and deployment-config loader;
2. structured exit codes;
3. embedded offline web shell with no third-party web assets;
4. local-only HTTP binding until the HTTPS/authentication slice exists;
5. portable signed/hash-chained ledger verifier;
6. deterministic canonical record digest and Ed25519 verification lineage from `decision-ledger`;
7. source import-boundary test for presentation/data/security separation;
8. GitHub Actions workflow for module cleanliness, formatting, vet, race tests and Linux amd64/arm64 builds.

PostgreSQL durable state, authentication, authorisation and incident mutation are explicitly **not** represented by a temporary implementation. `crab readiness` therefore returns `ready: false`, and unbuilt control-plane commands refuse rather than pretending to work.

## Local gates run

The following were executed against the final source before packaging:

```text
go test ./...                                      PASS
go vet ./...                                       PASS
go test -race ...                                  PASS
GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build    PASS
GOOS=linux GOARCH=arm64 CGO_ENABLED=0 go build    PASS
```

Portable verifier smoke test:

```text
valid one-entry ledger                             PASS, exit 0
payload changed after signing                      DETECTED, exit 1
```

The tampered fixture produced independent maker-signature, confirmer-signature, hash and server-signature failures.

Web shell smoke test on loopback:

```text
GET /                                              200
Cache-Control                                      no-store
Content-Security-Policy                            present
X-Content-Type-Options                             nosniff
X-Frame-Options                                    DENY
external script/style/font dependency              none
```

## Acceptance coverage begun

This is not the full V0.2 acceptance run. It begins concrete coverage for:

- T-53 — historical ledger tamper detection;
- T-54 — standalone portable verification mechanics;
- T-68 — core web shell requires no externally hosted asset;
- T-110 — presentation tier cannot import PostgreSQL/data implementation;
- V0.4 record/governed-write separation invariant at the ledger envelope;
- V0.4 browser header / no-store baseline for the currently exposed local shell.

## Publication and CI status

The canonical repository is `jonathanselby/cyber-response-action-board`. The module path was corrected from the provisional local name `github.com/jonathanselby/crab` to the canonical repository path before source publication. After that rename, local tests, vet, race tests and amd64/arm64 builds passed using the installed Go 1.23.2 toolchain. The container could not download the pinned Go 1.27.2 toolchain because outbound access to `proxy.golang.org` was unavailable; the GitHub Actions run is therefore the authoritative pinned-toolchain gate. CI must not be claimed green until that run is inspected.

## Next frozen slice

Per `BUILD-NOTES.md`, the next implementation is the PostgreSQL ledger + projection kernel: connection/pool, schema-owner/runtime identities in the test setup, append transaction with tail lock, atomic projection update, full verification, integrity-failed read-only mode, rebuild and portable export.
