# CRAB — Cyber Response Action Board

CRAB is a single Go application for running a cyber incident from first report through active response, recovery, review and closure while remaining useful when normal corporate systems are unavailable.

> **The model proposes. One person makes. Another confirms. Nothing is erased.**

## Build status

**0.0.1-dev — first executable vertical slice.**

Implemented now:

- one `crab` executable;
- file-first deployment configuration with environment overrides;
- `crab version`;
- `crab readiness` for the implemented local runtime surface;
- `crab serve` with an embedded, offline-only CRAB shell and security headers;
- standalone signed/hash-chained portable ledger verification via `crab verify`;
- deterministic ledger canonicalisation/signature lineage from the Garage `decision-ledger` component;
- amd64/arm64 CI build gates, gofmt, vet, race tests and tier-boundary checks;
- frozen PRD V0.2 / EDD V0.4 and companion build, test and UX/print notes retained beside source.

Not yet claimed complete:

- PostgreSQL durable store and replay transaction;
- authentication/authorisation and bootstrap ceremonies;
- incident/domain mutations;
- evidence blobs, retrieval, models, communications, print, backup/restore and later slices.

Those are built in the frozen sequence in `BUILD-NOTES.md`. The current checkpoint does not simulate them with alternate stores or bypasses.

## Run

```text
crab version
crab readiness
crab serve
crab verify --ledger <file> --keyring <file>
```

`crab serve` defaults to `127.0.0.1:8080`. Production HTTPS termination is not claimed by this checkpoint; the browser surface is a local build shell only until the security/authentication slice lands.

## Governing documents

- `PRD.md` — product contract, V0.2.
- `EDD.md` — frozen architecture, V0.4.
- `BUILD-NOTES.md` — implementation sequencing and build discipline.
- `TEST-ACCEPTANCE-NOTES.md` — behavioural/security acceptance catalogue.
- `UX-PRINT-BUILD-NOTES.md` — web, accessibility and print quality requirements.

The implementation may simplify code. It may not silently weaken the product or security contract.

## Toolchain

The checkpoint CI toolchain is pinned to Go 1.27.2. This source was also exercised locally under Go 1.23.2 before the toolchain pin was added; the GitHub workflow is the publication gate once the repository exists.
