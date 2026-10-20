# CRAB — Cyber Response Action Board

CRAB is a single Go application for running a cyber incident from first report through active response, recovery, review and closure while remaining useful when normal corporate systems are unavailable.

> **The model proposes. One person makes. Another confirms. Nothing is erased.**

## Build status

**0.0.9-dev — Slice 8 governed corpus and deterministic retrieval.**

Implemented and CI-proven now:

- one `crab` executable with amd64/arm64 build gates;
- file-first deployment configuration and embedded offline web shell;
- PostgreSQL as the single durable live store with separate schema-owner/runtime identities;
- authoritative signed/hash-chained ledger, atomic projections, full verification, integrity-failed read-only mode, deterministic projection rebuild and portable JSONL export;
- Slice 2 authentication/security boundary with local credentials, TOTP, passkeys, pinned-key OIDC, isolate mode, break-glass, OOB verification and deny-by-default authorisation;
- Slice 3 immediate `Record(...)` observations and `Propose(...) -> Decide(...)` governed state with exact proposal-version maker-checker confirmation;
- Slice 4 governed Action lifecycle, configured checklist activation and rebuildable timeline without a workflow engine;
- Slice 5 organisation-defined deterministic obligation clocks and bounded rebuildable timed wait list without a legal rules engine or delivery queue;
- Slice 6 Sponsor/Controller -> Custodian access governance, expiring incident/compartment membership, metadata-only administration and selective-release scope; PostgreSQL RLS was tested and deliberately not retained;
- Slice 7 bounded hostile-byte streaming into immutable PostgreSQL blob rows, exact SHA-256 verification, evidence register/handling history and fail-closed attachment presentation;
- Slice 8 governed Markdown/DOCX corpus sources backed by the same immutable blob primitive, with a hard 16 MiB source limit and bounded passive ZIP/XML handling for DOCX;
- original corpus bytes remain immutable durable supporting state while confirmed source metadata and extracted passages are rebuildable projections;
- only `word/document.xml` text is extracted from DOCX; macros and embedded active content are neither executed nor indexed;
- incident/compartment scope is filtered in PostgreSQL **before** candidate passages reach the lexical ranker;
- deterministic lexical retrieval requires no embedding model, vector database, local LLM or network connection;
- citations are stable across rebuilds because they bind source ID, immutable source SHA-256 and deterministic source locator;
- unsupported queries return an explicit refusal rather than invented support;
- tests deliberately make restricted and other-incident passages stronger lexical matches and prove they never enter the authorised ranking set;
- a confirmed incident declaration now establishes the declaration maker's initial Sponsor, incident-membership and `GENERAL` membership state from the declaration ledger record itself, closing the frozen Slice 6 declaration invariant;
- frozen PRD V0.2 / EDD V0.4 and companion build, test and UX/print notes remain beside source.

Not yet claimed complete:

- secure operator/presentation wiring for `crab setup`, browser login and authenticated operational routes; source/evidence application paths exist but no unauthenticated shortcut is introduced;
- Slice 9 model assistance;
- communications/notifier delivery, print/paper, recovery/PIR/retention/closure, backup/restore and final Bafflewick/Pi/visual acceptance.

The checkpoint does not create a universal administrator, universal Major visibility, emergency read-in, external portal, per-object ACL estate, production RLS, object store, media-processing service, malware scanner, workflow engine, delivery queue, vector database or generic CRUD incident model.

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

Evidence capture metadata and evidence-handling events use `Record(...)`: they preserve what was received/handled without becoming a controlled decision. A blob becomes incident evidence only when an authorised evidence record binds that opaque blob to its incident and compartment.

Corpus admission uses governed `Propose(...) -> Decide(...)`. Uploading bytes alone does not make a document retrievable. A confirmed corpus source must exactly bind the immutable blob metadata before deterministic extraction/index projection is effective.

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

For retrieval, ranking is not a security boundary. PostgreSQL first returns only organisation-wide or incident-specific passages within the requester's authorised compartment set; the deterministic ranker can only rank that already-authorised set.

## Database boundary

The schema-owner identity performs migrations. The ordinary runtime identity is not table owner, superuser or `BYPASSRLS`. Authoritative ledger rows cannot be updated/deleted by the runtime role. Blob metadata/chunks are durable supporting state and are insert/read-only to runtime. Security, incident, obligation, access, evidence and corpus current state are rebuildable projections from ledger history.

RLS was tested, not hand-waved. The Slice 6 spike showed it can stop accidental broad reads with transaction-local scope, but retaining it would require an additional privileged projection/rebuild channel in the current one-runtime-role architecture. V0.4 therefore keeps application authorisation primary and records the rejected RLS design in `docs/rls-spike-decision.md`.

## Governing documents

- `PRD.md` — product contract, V0.2.
- `EDD.md` — frozen architecture, V0.4.
- `BUILD-NOTES.md` — implementation sequencing and build discipline.
- `TEST-ACCEPTANCE-NOTES.md` — behavioural/security acceptance catalogue.
- `UX-PRINT-BUILD-NOTES.md` — web, accessibility and print quality requirements.

The implementation may simplify code. It may not silently weaken the product or security contract.

## Toolchain

CI is pinned to Go 1.27.2. The module baseline is Go 1.25 because the pinned PostgreSQL driver requires Go 1.25 or newer.
