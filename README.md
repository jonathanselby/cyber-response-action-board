# CRAB — Cyber Response Action Board

CRAB is a single Go application for running a cyber incident from first report through active response, recovery, review and closure while remaining useful when normal corporate systems are unavailable.

> **The model proposes. One person makes. Another confirms. Nothing is erased.**

## Build status

**0.0.4-dev — Slice 3 incident + semantic write kernel.**

Implemented and CI-proven now:

- one `crab` executable with amd64/arm64 build gates;
- file-first deployment configuration and embedded offline web shell;
- PostgreSQL as the single durable live store with separate schema-owner/runtime identities;
- authoritative signed/hash-chained ledger, atomic projections, full verification, integrity-failed read-only mode, deterministic rebuild and portable JSONL export;
- Slice 2 local/password, TOTP, passkey and pinned-key OIDC authentication, isolate mode, hashed sessions, break-glass, OOB verification and failure limiting;
- one deny-by-default authorisation evaluator across role, incident, compartment, authoritative object scope and separately governed data exits;
- rebuildable role/Custodian/incident/compartment/output/isolate security state, agent-to-human binding, bootstrap and Custodian recovery;
- explicit Slice 3 semantic write paths: immediate `Record(...)` for observations and `Propose(...) -> Decide(...)` for governed state;
- dedicated incident declaration, fact, unknown/hypothesis, decision, action and operational-role schemas rather than generic Item CRUD;
- exact proposal-version capture and maker-checker confirmation against the stored proposal rather than caller-supplied state;
- rebuildable PostgreSQL projections for the first common operating picture: incident header, facts, unknowns, hypotheses, decisions, actions, operational roles and pending proposals;
- strict separation of pending proposals from effective state; rejected proposals never become effective;
- facts cannot mutate governed state: an observation saying “severity should change to Critical” leaves effective severity unchanged, including after full ledger rebuild;
- custom organisation role labels are preserved without changing Major/Minor/Observer authorisation roles;
- live/exercise mode is carried by the authoritative proposal path and cannot be swapped during confirmation;
- application-service role boundaries: Major/Minor may record observations, Major alone may propose/confirm governed changes, Observer remains read-only, and existing-incident writes require current incident + compartment authority;
- new incident declaration follows the frozen rule that a Major may propose before membership exists, with a different Major confirming it;
- end-to-end CI proof across application service → domain semantics → ledger → PostgreSQL projection → authorised common-operating-picture read;
- frozen PRD V0.2 / EDD V0.4 and companion build, test and UX/print notes retained beside source.

Not yet claimed complete:

- secure operator/presentation wiring for `crab setup`, browser login and the semantic incident UI; current Slice 3 is the application/data kernel, not the finished front end;
- Slice 4 action lifecycle, checklists and timeline behaviour beyond the first confirmed Action projection;
- obligations/clocks, RLS spike, evidence blobs, retrieval, model assistance, communications/notifier implementation, print, backup/restore and later slices.

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

A proposal is visible as pending but is not effective state. Confirmation must match the exact stored proposal version, maker, compartment and incident mode. A rejected proposal is retained in ledger history but never appears as effective operating state.

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
