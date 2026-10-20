# CRAB build evidence — 0.0.3-dev

**Checkpoint:** Slice 2 security boundary  
**Promoted commit:** `38dd3da4d9a2ac8cdf2784101c7f4f9b9f415b62`  
**GitHub Actions run:** `39357680732` / run 33  
**Result:** green

## Governing baseline

This checkpoint remains governed by:

- `PRD.md` — CRAB PRD V0.2;
- `EDD.md` — CRAB EDD V0.4 FINAL;
- `BUILD-NOTES.md`;
- `TEST-ACCEPTANCE-NOTES.md`;
- `UX-PRINT-BUILD-NOTES.md`.

No governing document was changed while implementing Slice 2.

## Slice 1 retained proof

The Slice 1 PostgreSQL ledger/projection kernel remains green:

- separate schema-owner/runtime identities;
- authoritative signed/hash-chained ledger;
- advisory ledger-tail lock;
- atomic ledger + projection transaction;
- rollback when projection application fails;
- full verification before governed mutation;
- sticky integrity-failed read-only mode;
- deterministic projection rebuild;
- portable JSONL export and offline verification;
- PostgreSQL restore/replay equality;
- amd64 and arm64 builds.

## Slice 2 implemented proof

### Authentication and credential support

- local password authentication with minimum 600,000 PBKDF2 iterations;
- opaque random short-lived sessions; only SHA-256 token digests are stored;
- TOTP with minimum secret floor and bounded verification window;
- AES-GCM wrapping of persisted TOTP secrets using deployment-owned key material;
- P-256 passkey verification including RP ID, origin, user-presence, optional user-verification and counter checks;
- pinned-key RS256 OIDC verification with issuer/audience/nonce/time checks;
- isolate mode blocks federation before token-verifier execution;
- exactly two distinct valid offline keyholders required for break-glass authentication;
- break-glass verification material can be individually revoked;
- bounded authentication failure limiting;
- out-of-band verification retains the pre-registered callback route and ignores caller-supplied contact details.

### Authorisation boundary

One application authorisation evaluator now requires, as applicable:

- authenticated named human principal;
- role permission;
- incident membership;
- compartment membership;
- authoritative object-to-scope resolution;
- separately granted output/action permission.

Negative coverage proves:

- a Major has no universal incident visibility;
- `GENERAL` does not imply a restricted compartment;
- Custodian metadata authority does not grant incident-content visibility;
- ordinary read does not imply export/download/print/model/API/MCP/backup permissions;
- opaque identifier substitution cannot bypass authoritative scope resolution;
- an agent is bound to one named human and cannot swap or exceed that human principal.

### Governed/rebuildable security state

The ledger remains authoritative for current security/access state. Rebuildable projections cover:

- Major/Minor/Observer eligibility;
- Custodian designation;
- incident membership;
- compartment membership;
- output permissions;
- isolate mode.

`GENERAL` is derived from governed incident membership and cannot be independently granted. Removing incident membership removes the current compartment projection for that incident.

### Bootstrap, recovery and break-glass

- bootstrap requires two distinct named humans;
- at least two Custodians and two Major-eligible humans are required;
- local authentication is required for the bootstrap participants;
- exactly two separate offline break-glass keyholders are required;
- bootstrap creates no incident or compartment visibility;
- confirmed bootstrap/recovery records can be signed by the two named humans and enter the ordinary signed ledger chain;
- Custodian recovery verifies both offline proofs and can restore Custodian quorum only;
- Custodian recovery cannot create incident read-in, compartment read-in, role elevation or export authority;
- break-glass creates a time-limited session for an existing named human only;
- break-glass does not alter role, incident, compartment or output projections;
- an alert edge is attempted and the attempted/failed outcome is immutable audit material;
- alert failure does not cancel otherwise-valid emergency authentication.

## Implementation findings resolved during the slice

1. PostgreSQL projection replay originally attempted SQL while a streaming `Rows` result remained open. Replay was simplified to read one verified sequence at a time before projection application.
2. The original ledger validator incorrectly required an incident ID for every record. The frozen EDD permits system/setup records before incidents exist; the ledger now permits `mode=system` without an incident ID while retaining mandatory incident IDs for live/exercise records.
3. The first generated `go.sum` lacked pgx transitive/test checksums because the local toolchain could not run the pinned dependency graph. The exact CI `go mod tidy` result was applied.
4. PostgreSQL `window` was unsuitable as the TOTP column name; it was changed to `totp_window` without semantic change.
5. The unsupported-future-schema test was made relative to `currentMigrationVersion + 1`, removing brittle maintenance when legitimate migrations are added.
6. The break-glass unit test initially hashed the printable base64 bearer token instead of the raw token bytes used by the session service. The test was corrected; session semantics were unchanged.

## CI evidence

GitHub Actions run `39357680732` completed successfully on commit `38dd3da4d9a2ac8cdf2784101c7f4f9b9f415b62` with all gates green:

- PostgreSQL 17 service initialisation;
- module contract / `go mod tidy` cleanliness;
- `gofmt` contract;
- `go vet ./...`;
- `go test -race -count=1 ./...` including PostgreSQL and security integration tests;
- Linux amd64 build;
- Linux arm64 build;
- focused checkpoint acceptance covering ledger, PostgreSQL, authentication, authorisation, access projection, isolate mode, OOB, break-glass, bootstrap/recovery, web-offline headers, tier boundaries and readiness refusal.

## Deliberately not claimed

This checkpoint does **not** claim:

- operator-facing `crab setup` transport;
- browser login/session presentation wiring;
- incident/domain semantic write kernel (Slice 3);
- RLS retention decision (Slice 6 gate);
- evidence blobs, retrieval, model assistance, notifier implementation, print or backup/restore completion.

`crab setup` therefore continues to refuse rather than accepting passwords, recovery proofs or signing private material through an improvised command-line path.

No release tag is appropriate at this checkpoint.
