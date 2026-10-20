# CRAB — Cyber Response Action Board

CRAB is a single Go application for running a cyber incident from first report through active response, recovery, review and closure while remaining useful when normal corporate systems are unavailable.

> **The model proposes. One person makes. Another confirms. Nothing is erased.**

## Build status

**0.0.5-dev — Slice 4 actions, checklists and timeline.**

Implemented and CI-proven now:

- one `crab` executable with amd64/arm64 build gates;
- file-first deployment configuration and embedded offline web shell;
- PostgreSQL as the single durable live store with separate schema-owner/runtime identities;
- authoritative signed/hash-chained ledger, atomic projections, full verification, integrity-failed read-only mode, deterministic projection rebuild and portable JSONL export;
- Slice 2 local/password, TOTP, passkey and pinned-key OIDC authentication, isolate mode, hashed sessions, break-glass, OOB verification and failure limiting;
- one deny-by-default authorisation evaluator across role, incident, compartment, authoritative object scope and separately governed data exits;
- Slice 3 immediate `Record(...)` observations and `Propose(...) -> Decide(...)` governed state, with exact proposal-version maker-checker confirmation;
- rebuildable common-operating-picture projections for incident header, facts, unknowns, hypotheses, decisions, actions, operational roles and pending proposals;
- Slice 4 Action semantics with mandatory owner, status, priority and source; optional due time, notes and evidence; skipped Actions require a reason;
- Action overdue state derived from due time and current time rather than stored as a second source of truth;
- governed Action lifecycle updates reuse the same Action ID: the current projection advances while every proposal/confirmation remains in immutable ledger history;
- unresolved pending proposals are immutable and cannot be replaced by a later version before confirmation/rejection;
- data-driven checklist templates are strict validated configuration, not another database/service; activation expands them into ordinary governed Action proposals with runbook/template/step provenance and owner/due defaults;
- rebuildable incident timeline preserving occurrence, flagging, recording and named-human awareness times as distinct concepts, plus ledger time-trust status;
- dedicated semantic Action and timeline renderers showing the fields operators need without introducing generic CRUD screens or an unauthenticated incident route;
- explicit architecture test that Slice 4 introduces no workflow engine or Temporal-style dependency;
- facts still cannot mutate governed state; rejected proposals still never become effective; live/exercise scope and existing Slice 2 security boundaries remain intact;
- frozen PRD V0.2 / EDD V0.4 and companion build, test and UX/print notes retained beside source.

Not yet claimed complete:

- secure operator/presentation wiring for `crab setup`, browser login and authenticated incident routes; Slice 4 includes dedicated semantic renderers but does not expose them through an improvised unauthenticated path;
- Slice 5 obligations/clocks and the bounded timed-wait list;
- RLS spike, evidence blobs, retrieval, model assistance, communications/notifier implementation, print, backup/restore and later slices.

The checkpoint does not create a universal administrator, universal Major visibility, emergency read-in, per-object ACL estate, alternate store, workflow engine or generic CRUD incident model.

## Commands

```text
crab version
crab readiness
crab serve
crab verify --ledger <file> --keyring <file>
crab rebuild
crab export-ledger --output <file>
```

`crab verify` is the offline portable verifier. Database-backed commands require `CRAB_DATABASE_URL` and `CRAB_LEDGER_KEYRING_FILE`. `crab serve` remains loopback-only until the authenticated presentation path is wired. `crab setup` continues to fail closed until a secure operator input path is implemented; passwords, recovery proofs and signing private material are not accepted through ordinary command arguments.

## Write semantics

CRAB keeps the write model deliberately small:

```text
Record(...)                  observation/history: append now
Propose(...) -> Decide(...)  governed state: maker proposes, different eligible human confirms or rejects
```

A proposal is visible as pending but is not effective state. Confirmation must match the exact stored proposal version, maker, compartment and incident mode. An unresolved pending proposal cannot be overwritten. A rejected proposal is retained in ledger history but never appears as effective operating state.

An Action is not a workflow object. A confirmed Action is simply governed state projected from its ledger history. A later owner/status/due/evidence change is another proposal against the same Action ID and requires the same maker-checker path.

## Security boundary

CRAB's application decision remains deliberately small:

```text
ALLOW = authenticated principal
    AND role permits action
    AND active incident membership where required
    AND required compartment membership
    AND object resolves to that authorised scope
    AND requested output/action is separately permitted

otherwise DENY
```

Role determines what a person may do. Incident membership determines where. Compartment membership determines what information they may know. Custodian authority administers identity/access metadata but grants no incident-content visibility by itself. Break-glass changes authentication only.

## Database boundary

The schema-owner identity performs migrations. The ordinary runtime identity is not table owner, superuser or `BYPASSRLS`. Authoritative ledger rows cannot be updated/deleted by the runtime role. Credential/session state is durable supporting state; security and incident current state are rebuildable projections from ledger history.

## Governing documents

- `PRD.md` — product contract, V0.2.
- `EDD.md` — frozen architecture, V0.4.
- `BUILD-NOTES.md` — implementation sequencing and build discipline.
- `TEST-ACCEPTANCE-NOTES.md` — behavioural/security acceptance catalogue.
- `UX-PRINT-BUILD-NOTES.md` — web, accessibility and print quality requirements.

The implementation may simplify code. It may not silently weaken the product or security contract.

## Toolchain

CI is pinned to Go 1.27.2. The module baseline is Go 1.25 because the pinned PostgreSQL driver requires Go 1.25 or newer.
