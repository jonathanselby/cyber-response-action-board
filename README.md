# CRAB — Cyber Response Action Board

CRAB is a single Go application for running a cyber incident from first report through active response, recovery, review and closure while remaining useful when normal corporate systems are unavailable.

> **The model proposes. One person makes. Another confirms. Nothing is erased.**

## Build status

**0.0.7-dev — Slice 6 access governance and compartments.**

Implemented and CI-proven now:

- one `crab` executable with amd64/arm64 build gates;
- file-first deployment configuration and embedded offline web shell;
- PostgreSQL as the single durable live store with separate schema-owner/runtime identities;
- authoritative signed/hash-chained ledger, atomic projections, full verification, integrity-failed read-only mode, deterministic projection rebuild and portable JSONL export;
- Slice 2 local/password, TOTP, passkey and pinned-key OIDC authentication, isolate mode, hashed sessions, break-glass, OOB verification and failure limiting;
- one deny-by-default authorisation evaluator across role, incident, compartment, authoritative object scope and separately governed data exits;
- Slice 3 immediate `Record(...)` observations and `Propose(...) -> Decide(...)` governed state, with exact proposal-version maker-checker confirmation;
- rebuildable common-operating-picture projections for incident header, facts, unknowns, hypotheses, decisions, actions, active obligations, operational roles and pending proposals;
- Slice 4 governed Action lifecycle, data-driven checklist activation and rebuildable incident timeline without a workflow engine;
- Slice 5 organisation-defined deterministic obligation clocks and bounded rebuildable timed wait list without a legal rules engine or delivery queue;
- Slice 6 governed Incident Sponsor and Compartment Controller designations, kept separate from Major/Minor/Observer roles and from content visibility;
- Sponsor-proposed incident membership and Controller-proposed restricted-compartment read-in/read-out, independently confirmed by a Custodian through the ordinary maker-checker path;
- incident membership implies `GENERAL`; restricted compartments require explicit confirmed membership;
- effective-time, expiry and revocation semantics for incident and compartment access, evaluated again at read time so expired external responders lose access without deleting history;
- access-administration views expose identity/access metadata, designations, scopes and grant status without incident-content bodies;
- selective-release scope is limited to compartments the current principal already possesses; ordinary read still does not imply export/download/print permission;
- pending access proposals are immutable and exact-version matched before confirmation;
- the runtime database role remains non-owner, non-superuser and without `BYPASSRLS`;
- a real PostgreSQL RLS spike proved transaction-local incident/compartment policies can stop broad reads and do not leak across pooled-connection commit/rollback reuse;
- RLS is deliberately **not retained** in V0.4 because the same safe runtime role also performs deterministic projection writes/rebuilds: retaining RLS would require a second privileged projection role/path or bypass mechanism plus transaction-pinned multi-query reads, increasing privilege and recovery complexity for a defence-in-depth layer;
- the RLS decision and evidence are retained in `docs/rls-spike-decision.md` and automated integration tests;
- frozen PRD V0.2 / EDD V0.4 and companion build, test and UX/print notes retained beside source.

Not yet claimed complete:

- secure operator/presentation wiring for `crab setup`, browser login and authenticated operational routes;
- Slice 7 hostile evidence/blob storage and phone capture;
- retrieval, model assistance, communications/notifier delivery, print, recovery/closure, backup/restore and final Bafflewick/Pi/visual acceptance.

The checkpoint does not create a universal administrator, universal Major visibility, emergency read-in, separate external portal, per-object ACL estate, RLS bypass role, alternate store, workflow engine, delivery queue or generic CRUD incident model.

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

Access governance uses the same mechanism rather than introducing a separate workflow engine: mission authority proposes; Custodian independently verifies/confirms; the rebuildable access projection changes.

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

Role determines what a person may do. Incident membership determines where. Compartment membership determines what information they may know. Custodian authority administers identity/access metadata but grants no incident-content visibility by itself. Sponsor and Controller designations create mission authority to propose access changes, not content visibility. Break-glass changes authentication only.

## Database boundary

The schema-owner identity performs migrations. The ordinary runtime identity is not table owner, superuser or `BYPASSRLS`. Authoritative ledger rows cannot be updated/deleted by the runtime role. Credential/session state is durable supporting state; security, incident, obligation and timed-wait current state are rebuildable projections from ledger history.

RLS was tested, not hand-waved. The spike showed it can stop accidental broad reads with transaction-local scope, but retaining it would require an additional privileged projection/rebuild channel in the current one-runtime-role architecture. The frozen EDD says to remove RLS when recovery/privilege ambiguity outweighs the backstop; V0.4 therefore keeps application authorisation primary and records the rejected RLS design in `docs/rls-spike-decision.md`.

## Governing documents

- `PRD.md` — product contract, V0.2.
- `EDD.md` — frozen architecture, V0.4.
- `BUILD-NOTES.md` — implementation sequencing and build discipline.
- `TEST-ACCEPTANCE-NOTES.md` — behavioural/security acceptance catalogue.
- `UX-PRINT-BUILD-NOTES.md` — web, accessibility and print quality requirements.

The implementation may simplify code. It may not silently weaken the product or security contract.

## Toolchain

CI is pinned to Go 1.27.2. The module baseline is Go 1.25 because the pinned PostgreSQL driver requires Go 1.25 or newer.
