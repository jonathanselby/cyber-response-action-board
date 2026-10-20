# CRAB build evidence — 0.0.13-dev

## Slice

Slice 12 — recovery, PIR, retention and closure.

Frozen PRD V0.2 and EDD V0.4 were not changed.

## Source checkpoint

The completed Slice 12 code reached GitHub Actions run **#170 / 39393839761** fully green with:

- module contract;
- gofmt contract;
- `go vet ./...`;
- full `go test -race -count=1 ./...` including PostgreSQL integration tests;
- Linux amd64 build;
- Linux arm64 build;
- explicit core checkpoint acceptance.

The immediately preceding run exposed only a gofmt difference after the final compartment-security correction; no behavioural or architectural change was needed.

## Implemented boundary

### Recovery coordination

CRAB records recovery work with a named human owner, restoration priority, verification state, residual risk and optional handoff reference. It coordinates recovery; it does not orchestrate remediation.

Operational phase is derived from recovery evidence and confirmed milestones. No second mutable incident state machine was introduced.

### Stand-down and PIR

Stand-down is a governed maker-checker milestone using the existing `Propose(...) -> Decide(...)` path.

PIR drafting is generated from the existing authorised common operating picture and incident history. Human lessons remain explicit human input, and the PIR must be confirmed before closure.

### Retention and disposition

Retention/disposition records retention class, hold state/reason, optional disposition eligibility and an external handoff reference. Slice 12 deliberately adds no physical deletion engine or records-management subsystem.

### Closure

Closure is refused unless:

- stand-down is confirmed;
- PIR is confirmed;
- retention/disposition is confirmed;
- every recovery item is either verified or handed off.

Confirmed closure makes incident-scoped writes read-only. The enforcement converges at the authoritative ledger append boundary so other feature paths cannot bypass closure by using a different application service.

### Privileged/shareable closure material

The final security correction prevents privileged closure text being carried in a `GENERAL` closure record. Shareable-only closure uses `GENERAL`; any privileged closure material must identify and use a matching restricted compartment. State reads remove restricted closure material when the requester is not read into that compartment.

### Replay and recovery

Recovery updates and milestones are rebuildable projections over the immutable ledger. The integration test closes an incident, rejects post-closure writes, rebuilds projections and confirms the closed phase and recovery handoff survive replay.

The `crab rebuild` command was also corrected to register the full deterministic projection set rather than invoking rebuild with no projections.

## Acceptance evidence

Slice 12 acceptance covers:

- derived recovery phase without a second state machine;
- closure prerequisites;
- retention hold-reason validation without deletion behaviour;
- PIR drafting from existing incident state;
- maker-checker stand-down/PIR/retention/closure;
- exact pending-proposal bytes through PostgreSQL;
- compartment restrictions on recovery milestones and closure material;
- shareable closure material not containing privileged text;
- cross-feature post-closure write refusal;
- deterministic recovery/closure projection rebuild;
- full Go race-enabled unit/PostgreSQL integration suite;
- amd64 and arm64 builds.

## Explicitly not claimed

Slice 12 does not add remediation orchestration, a workflow engine, records-management subsystem, physical destruction, a second datastore, universal administrator or universal Major visibility.

Backup/restore and final Bafflewick/Pi/visual acceptance remain later work.

No release or tag should be created for this development checkpoint.
