# Changelog

## Unreleased — 0.0.2-dev

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

Authentication/authorisation, incident semantics, RLS, evidence, retrieval and later slices remain out of scope for this checkpoint.

No release tag should be created until the complete repository is built and acceptance gates are green.

## 0.0.1-dev

First executable build checkpoint: shell, configuration, offline web surface, portable signed-ledger verification and CI baseline.
