# Changelog

## Unreleased — 0.0.3-dev

Slice 2 security boundary.

- Added local password authentication, opaque hashed sessions and PostgreSQL supporting-state persistence.
- Added TOTP with AES-GCM secret wrapping, passkey verification and pinned-key RS256 OIDC verification without adding another direct runtime dependency.
- Added isolate-mode federation refusal before verifier work.
- Added exactly-two-keyholder break-glass authentication with individually revocable verification material, bounded failure limiting and immutable alert-attempt outcomes.
- Added pre-registered-route out-of-band verification that ignores caller-supplied contact details.
- Added the single deny-by-default authorisation boundary across role, incident, compartment, authoritative object scope and separate output/action permissions.
- Added rebuildable role eligibility, Custodian, incident membership, compartment membership, output-permission and isolate-mode projections.
- Made `GENERAL` derive from governed incident membership; restricted compartments require explicit confirmed read-in.
- Added opaque-ID scope re-resolution so identifier substitution cannot confer access.
- Added agent-to-named-human binding with no authority above the named human.
- Added two-human bootstrap and Custodian-recovery ceremony validation with no incident visibility or emergency read-in.
- Added explicit human signing for governed ceremony records and proved they form a valid portable ledger chain.
- Added break-glass audit and alert-edge attempt recording while preserving emergency authentication when notification fails.
- Corrected the ledger envelope to support frozen-design `system` records with no incident ID while keeping live/exercise incident IDs mandatory.
- Kept the RLS decision gate deferred to Slice 6 as designed.

Operator-facing `crab setup`/login transport is not faked in this checkpoint: secret-safe presentation/input wiring remains to be built. No release tag should be created yet.

## 0.0.2-dev

Slice 1 ledger + projection kernel.

- Added bounded `pgx` PostgreSQL pool as the first direct runtime dependency.
- Added explicit owner/runtime database-role separation and migration harness.
- Added authoritative `crab.ledger_entries` storage with exact portable entry bytes.
- Added advisory-lock ledger append with cached verified-tail check.
- Added same-transaction projection hook and rollback on projection failure.
- Added full database ledger verification and sticky integrity-failed read-only mode.
- Added verified projection rebuild and portable JSONL export commands.
- Added PostgreSQL integration tests for ownership, append/replay equality, rollback, tamper detection and restore/replay.
- Canonicalised ledger payload JSON before authoritative entry storage.

## 0.0.1-dev

First executable build checkpoint: shell, configuration, offline web surface, portable signed-ledger verification and CI baseline.
