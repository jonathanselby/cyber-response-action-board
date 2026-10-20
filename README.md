# CRAB — Cyber Response Action Board

CRAB is a single Go application for running a cyber incident from first report through active response, recovery, review and closure while remaining useful when normal corporate systems are unavailable.

> **The model proposes. One person makes. Another confirms. Nothing is erased.**

## Build status

**0.0.10-dev — Slice 9 model assistance.**

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
- Slice 8 governed Markdown/DOCX corpus admission, immutable source blobs, bounded passive extraction, scope filtering before deterministic lexical ranking, stable citations and explicit unsupported-query refusal;
- Slice 9 five small prompt-versioned model draft specifications: retrieval summary, situation report, board update, notification language and PIR summary;
- explicit `simple`, `standard` and `complex` model lanes: the caller classifies work and the harness routes exactly that lane; there is no hidden model selector or automatic fallback;
- local Ollama plus managed Azure OpenAI/Foundry, Amazon Bedrock and Google Vertex adapters using standard-library HTTP and no new direct runtime dependency;
- deterministic authorised retrieval always precedes model generation, so the model receives only the already-authorised source set;
- managed model calls require both the sensitivity policy and the separate `model_send` output permission before provider I/O;
- model traces carry request/spec/prompt version, selected provider/model, prompt SHA-256, stable citation IDs, duration and provider-reported token counts without routinely logging prompt bodies;
- model output is always explicitly untrusted; `validated` means only that deterministic structural checks passed and never means human approval or effective governed state;
- provider failures and invalid output retain authorised source provenance and fail visibly rather than coercing model text into incident truth;
- no automatic provider reroute/failover: failure of the configured lane remains a visible failure;
- bounded model capacity sheds new model generation first while deterministic retrieval and core incident work remain available;
- provider adapter tests exercise wire contracts locally and managed-provider error handling does not echo provider response bodies;
- frozen PRD V0.2 / EDD V0.4 and companion build, test and UX/print notes remain beside source.

Not yet claimed complete:

- secure operator/presentation wiring for `crab setup`, browser login and authenticated operational routes;
- deployment composition for live model credentials/provider configuration and operator-facing model routes; Slice 9 proves the application/provider boundary without pretending live cloud-account integration;
- communications/notifier delivery, print/paper, recovery/PIR/retention/closure, backup/restore and final Bafflewick/Pi/visual acceptance.

The checkpoint does not create a universal administrator, universal Major visibility, emergency read-in, external portal, per-object ACL estate, production RLS, object store, media-processing service, malware scanner, workflow engine, delivery queue, vector database, autonomous agent framework, automatic model classifier or generic CRUD incident model.

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

Model drafts use neither write semantic. They are transient, untrusted assistance. A human must still perform the ordinary CRAB Record or Propose/Decide action if any model-derived wording is to become incident history or governed state.

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

For managed model use, retrieval authorisation is not permission to disclose externally. The sensitivity rule and separate `model_send` permission must both pass before provider I/O. A local lane does not create governed authority either: all model output remains untrusted.

## Database boundary

The schema-owner identity performs migrations. The ordinary runtime identity is not table owner, superuser or `BYPASSRLS`. Authoritative ledger rows cannot be updated/deleted by the runtime role. Blob metadata/chunks are durable supporting state and are insert/read-only to runtime. Security, incident, obligation, access, evidence and corpus current state are rebuildable projections from ledger history.

Slice 9 adds no model database, model queue or prompt store. Model assistance consumes already-authorised retrieval results and returns transient drafts/traces; core incident operation does not depend on a provider being available.

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
