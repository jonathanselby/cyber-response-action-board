# CRAB — Cyber Response Action Board

CRAB is a single Go application for running a cyber incident from first report through active response, recovery, review and closure while remaining useful when normal corporate systems are unavailable.

> **The model proposes. One person makes. Another confirms. Nothing is erased.**

## Build status

**0.0.2-dev — Slice 1 ledger + projection kernel.**

Implemented now:

- one `crab` executable;
- file-first deployment configuration with environment overrides;
- embedded offline-only web shell with security headers;
- standalone portable signed/hash-chained ledger verification;
- bounded PostgreSQL pool using pinned `pgx`;
- migration harness with separate schema-owner and ordinary runtime roles;
- authoritative append-only `ledger_entries` storage;
- ledger-tail transaction lock and verified-tail check;
- atomic ledger append plus projection hook;
- full verification and sticky integrity-failed read-only gate;
- projection rebuild/replay path;
- portable JSONL ledger export;
- amd64/arm64, formatting, vet, race, integration and tier-boundary CI gates;
- frozen PRD V0.2 / EDD V0.4 and companion build, test and UX/print notes retained beside source.

Not yet claimed complete:

- authentication/authorisation and bootstrap ceremonies (Slice 2);
- incident/domain write semantics and operating picture (Slice 3);
- RLS spike, evidence blobs, retrieval, models, communications, print, backup/restore and later slices.

The checkpoint does not create alternate stores, fake identity, generic workflows or placeholder incident mutation paths.

## Commands

```text
crab version
crab readiness
crab serve
crab verify --ledger <file> --keyring <file>
crab rebuild
crab export-ledger --output <file>
```

`crab verify` is the offline portable verifier. Database-backed commands require `CRAB_DATABASE_URL` and `CRAB_LEDGER_KEYRING_FILE`. `crab serve` remains loopback-only until the security/authentication slice lands.

## Database boundary

Slice 1 deliberately separates:

- a schema-owner identity used for migrations; and
- a runtime identity that is not owner, superuser or `BYPASSRLS`.

The runtime role can read/append the authoritative ledger but cannot update or delete it. Projection changes occur inside the same transaction as an append; a projection failure rolls the ledger append back.

## Governing documents

- `PRD.md` — product contract, V0.2.
- `EDD.md` — frozen architecture, V0.4.
- `BUILD-NOTES.md` — implementation sequencing and build discipline.
- `TEST-ACCEPTANCE-NOTES.md` — behavioural/security acceptance catalogue.
- `UX-PRINT-BUILD-NOTES.md` — web, accessibility and print quality requirements.

The implementation may simplify code. It may not silently weaken the product or security contract.

## Toolchain

CI is pinned to Go 1.27.2. The module baseline is Go 1.25 because the pinned PostgreSQL driver requires Go 1.25 or newer.
