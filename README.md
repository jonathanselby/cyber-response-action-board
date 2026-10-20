# CRAB — Cyber Response Action Board

CRAB is a single Go application for running a cyber incident from first report through active response, recovery, review and closure while remaining useful when normal corporate systems are unavailable.

> **The model proposes. One person makes. Another confirms. Nothing is erased.**

## Build status

**0.0.13-dev — Slice 12 recovery, PIR, retention and closure.**

Implemented and CI-proven now:

- one `crab` executable with amd64/arm64 build gates;
- file-first deployment configuration and embedded offline web shell;
- PostgreSQL as the single durable live store with separate schema-owner/runtime identities;
- authoritative signed/hash-chained ledger, atomic projections, full verification, integrity-failed read-only mode, deterministic full projection rebuild and portable JSONL export;
- Slice 2 authentication/security boundary with local credentials, TOTP, passkeys, pinned-key OIDC, isolate mode, break-glass, OOB verification and deny-by-default authorisation;
- Slice 3 immediate `Record(...)` observations and `Propose(...) -> Decide(...)` governed state with exact proposal-version maker-checker confirmation;
- Slice 4 governed Action lifecycle, configured checklist activation and rebuildable timeline without a workflow engine;
- Slice 5 organisation-defined deterministic obligation clocks and bounded rebuildable timed wait list without a legal rules engine or delivery queue;
- Slice 6 Sponsor/Controller -> Custodian access governance, expiring incident/compartment membership, metadata-only administration and selective-release scope; PostgreSQL RLS was tested and deliberately not retained;
- Slice 7 bounded hostile-byte streaming into immutable PostgreSQL blob rows, exact SHA-256 verification, evidence register/handling history and fail-closed attachment presentation;
- Slice 8 governed Markdown/DOCX corpus admission, immutable source blobs, bounded passive extraction, scope filtering before deterministic lexical ranking, stable citations and explicit unsupported-query refusal;
- Slice 9 five small prompt-versioned model draft specifications with explicit three-lane routing, local/managed provider adapters, sensitivity/model-send gates and untrusted traced output;
- Slice 10 structured communications, governed issue, situation-report cadence, incoming-enquiry assignment and thin local/webhook notification with immutable delivery outcomes;
- Slice 11 application-level print composition protected by the existing separate `print` output permission; ordinary read access does not imply print/export authority;
- local pure-Go PDF rendering for Incident Brief, Emergency Response Pack, evidence register, timeline and review/closure material;
- A4 and US Letter layouts, colour and monochrome modes, deterministic width-measured wrapping, bounded page breaks, page/source footers and embedded Go font data with no runtime font/network/browser dependency;
- live Incident Brief composition from the current authorised operating picture, including situation/impact, roles, decisions, actions, obligations, next communications deadline, contacts and source/time provenance;
- Emergency Response Pack rendering from approved material plus five blank contingency forms: incident, decision, action, communication and evidence;
- all generated operational PDFs are source-marked from CRAB's current ledger sequence/hash, configuration version, generation time and time-trust state; caller-supplied provenance is not trusted;
- exercise output is visibly marked in text on every page so meaning is not colour-dependent;
- primary derived copies use same-directory temp/write/fsync/rename semantics; failed refresh leaves the previous last-good copy untouched;
- optional secondary copies refresh independently after primary success; secondary failure is surfaced without destroying either the fresh primary or the last-good secondary;
- already-generated derived files are ordinary local PDFs and remain readable without PostgreSQL;
- paper-era events reconcile through the existing immutable ledger and timeline rather than a new paper database: original paper occurrence time remains separate from the later named-human transcription time and digital history cannot be backdated;
- Slice 12 recovery coordination with named human owners, restoration priority, verification/residual-risk/handoff metadata and no remediation orchestrator;
- operational phase derived from recovery evidence rather than a second mutable state machine;
- maker-checker stand-down, PIR, retention/disposition and closure milestones using the existing governed-write model;
- PIR draft generation from the existing authorised operating picture and incident history;
- closure prerequisites requiring stand-down, confirmed PIR, retention disposition and all recovery work either verified or handed off;
- confirmed closure makes the incident read-only across feature boundaries through the ledger convergence point;
- privileged closure material cannot be stored in `GENERAL`; shareable-only closure uses `GENERAL`, while restricted closure material must carry a matching restricted compartment;
- recovery/closure projections replay deterministically from the immutable ledger;
- frozen PRD V0.2 / EDD V0.4 and companion build, test and UX/print notes remain beside source.

Not yet claimed complete:

- secure operator/presentation wiring for `crab setup`, browser login and authenticated operational routes;
- deployment composition for live model credentials and notification channel configuration;
- authenticated operator/session wiring for the `crab print` command; it intentionally remains fail-closed rather than bypassing the print authorisation boundary;
- backup/restore and final Bafflewick/Pi/visual acceptance.

The checkpoint does not create a universal administrator, universal Major visibility, emergency read-in, external portal, per-object ACL estate, production RLS, object store, media-processing service, malware scanner, workflow engine, delivery queue, retry service, vector database, autonomous agent framework, automatic model classifier, chat/helpdesk platform, print service, print database, headless-browser renderer, records-management subsystem, recovery orchestrator or generic CRUD incident model.

## Commands

```text
crab version
crab readiness
crab serve
crab verify --ledger <file> --keyring <file>
crab rebuild
crab export-ledger --output <file>
```

`crab verify` is the offline portable verifier. Database-backed commands require `CRAB_DATABASE_URL` and `CRAB_LEDGER_KEYRING_FILE`. `crab serve` remains loopback-only until the authenticated presentation path is wired. `crab setup` continues to fail closed until a secure operator input path is implemented; passwords, recovery proofs and signing private material are not accepted through ordinary command arguments. `crab print` likewise remains fail-closed until the authenticated operator/session path can supply a real principal; the implemented print service itself already enforces incident/compartment scope and separate print permission. `crab rebuild` verifies the authoritative ledger and deterministically rebuilds the full registered projection set.

## Write semantics

CRAB keeps the write model deliberately small:

```text
Record(...)                  observation/history: append now
Propose(...) -> Decide(...)  governed state: maker proposes, different eligible human confirms or rejects
```

Evidence capture metadata and evidence-handling events use `Record(...)`: they preserve what was received/handled without becoming a controlled decision. A blob becomes incident evidence only when an authorised evidence record binds that opaque blob to its incident and compartment.

Corpus admission uses governed `Propose(...) -> Decide(...)`. Uploading bytes alone does not make a document retrievable. A confirmed corpus source must exactly bind the immutable blob metadata before deterministic extraction/index projection is effective.

Model drafts use neither write semantic. They are transient, untrusted assistance. A human must still perform the ordinary CRAB Record or Propose/Decide action if any model-derived wording is to become incident history or governed state.

Communication drafts, incoming enquiries and delivery outcomes use `Record(...)`: they preserve what was drafted, received or attempted. **Issuing a communication and changing its cadence use `Propose(...) -> Decide(...)`.** An unconfirmed draft cannot be notified.

Paper reconciliation uses `Record(...)`. The paper occurrence time is preserved in the transcription payload and timeline occurrence field; the authoritative ledger timestamp is the actual later CRAB transcription time. CRAB never rewrites the digital record as though it existed during the outage.

Recovery updates use `Record(...)` for operational coordination. Stand-down, PIR, retention/disposition and closure use `Propose(...) -> Decide(...)`. Closure is refused until prerequisites are satisfied and confirmed closure makes incident-scoped writes read-only.

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

For notification, ordinary read or communication drafting is not permission to send. The communication must first exist as confirmed governed state and `notify` remains a separate output action. Exercise suppression occurs before external adapter I/O.

For print, ordinary read does not imply a derived-output entitlement. `ActionPrint` is checked before incident content is read for print composition, and restricted-compartment output requires current read-in to that compartment.

For closure, `GENERAL` cannot carry privileged closure text. Shareable closure material remains separate from restricted-compartment material, and access remains governed by the ordinary incident/compartment boundary.

## Database boundary

The schema-owner identity performs migrations. The ordinary runtime identity is not table owner, superuser or `BYPASSRLS`. Authoritative ledger rows cannot be updated/deleted by the runtime role. Blob metadata/chunks are durable supporting state and are insert/read-only to runtime. Security, incident, obligation, access, evidence, corpus, communications, print/paper and recovery current state are rebuildable projections from ledger history.

Slice 9 adds no model database, model queue or prompt store. Slice 10 adds no message bus, delivery queue, retry service or third scheduler. Slice 11 adds no print database, print queue or paper-state store: PDFs are derived files and paper transcriptions replay into the ordinary incident timeline from the authoritative ledger. Slice 12 adds no recovery workflow engine, records-management subsystem or deletion engine.

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
