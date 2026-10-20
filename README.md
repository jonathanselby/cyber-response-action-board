# CRAB — Cyber Response Action Board

CRAB is a single Go application for running a cyber incident from first report through active response, recovery, review and closure while remaining useful when normal corporate systems are unavailable.

> **The model proposes. One person makes. Another confirms. Nothing is erased.**

## Build status

**0.0.8-dev — Slice 7 hostile evidence and immutable blobs.**

Implemented and CI-proven now:

- one `crab` executable with amd64/arm64 build gates;
- file-first deployment configuration and embedded offline web shell;
- PostgreSQL as the single durable live store with separate schema-owner/runtime identities;
- authoritative signed/hash-chained ledger, atomic projections, full verification, integrity-failed read-only mode, deterministic projection rebuild and portable JSONL export;
- Slice 2 authentication/security boundary with local credentials, TOTP, passkeys, pinned-key OIDC, isolate mode, break-glass, OOB verification and deny-by-default authorisation;
- Slice 3 immediate `Record(...)` observations and `Propose(...) -> Decide(...)` governed state with exact proposal-version maker-checker confirmation;
- Slice 4 governed Action lifecycle, configured checklist activation and rebuildable timeline without a workflow engine;
- Slice 5 organisation-defined deterministic obligation clocks and bounded rebuildable timed wait list without a legal rules engine or delivery queue;
- Slice 6 Sponsor/Controller -> Custodian access governance, expiring incident/compartment membership, metadata-only administration and selective-release scope; the PostgreSQL RLS spike is preserved but RLS is deliberately not retained;
- Slice 7 bounded hostile-byte streaming into ordinary PostgreSQL metadata/chunk rows with SHA-256 calculated while streaming;
- opaque random per-upload blob IDs and explicit **no cross-incident deduplication**, even when two uploads contain identical bytes;
- incomplete, disconnected or oversized uploads roll back and create no usable blob, evidence record or ledger entry;
- runtime can insert/read immutable blob rows but cannot update/delete blob metadata or chunks;
- an evidence record is immutable incident history and must exactly bind incident, mode, compartment, hash, size, filename and media metadata to the stored blob before it becomes usable evidence;
- evidence register/history records collector, source, collection/event times, location, current handler, related reference and later handling events without claiming certified forensic chain of custody;
- every detail/register/attachment access is re-authorised against the evidence record's stored incident/compartment scope; guessed IDs do not confer access;
- attachment download remains a separately governed output permission rather than following ordinary read permission;
- integrity verification re-hashes stored chunks before any attachment bytes are written to the response destination;
- safe-inline media is deliberately small: supported passive image/audio/video may be inline only when detected type is allowlisted and declared/detected types agree; HTML, SVG, PDF, active/ambiguous types and MIME mismatches fall back to download;
- safe attachment headers include `nosniff`, `no-store`, fixed download MIME for unsafe content and sanitised filenames;
- evidence projections rebuild from ledger history while durable blob bytes remain untouched;
- `CRAB_EVIDENCE_MAX_BYTES` remains the configurable product limit rather than a Pi-fixture ceiling;
- safe-inline spike evidence is retained in `docs/safe-inline-media-spike.md`;
- frozen PRD V0.2 / EDD V0.4 and companion build, test and UX/print notes remain beside source.

Not yet claimed complete:

- secure operator/presentation wiring for `crab setup`, browser login and authenticated operational routes; the Slice 7 capture application path accepts non-seekable browser/phone byte streams but no unauthenticated shortcut route is introduced;
- Slice 8 governed corpus/retrieval;
- model assistance, communications/notifier delivery, print, recovery/closure, backup/restore and final Bafflewick/Pi/visual acceptance.

The checkpoint does not create a universal administrator, universal Major visibility, emergency read-in, external portal, per-object ACL estate, RLS bypass role, object store, media-processing service, malware scanner, workflow engine, delivery queue or generic CRUD incident model.

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

Evidence capture metadata and evidence-handling events use `Record(...)`: they preserve what was received/handled without becoming a controlled decision. A blob becomes incident evidence only when an authorised evidence record binds that opaque blob to its incident and compartment. The byte store itself is durable supporting state and is not a second source of incident truth.

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

For evidence, storage identifiers are not authority. Evidence lookup resolves the authoritative stored scope first; attachment retrieval then requires the current principal to hold that scope and the separate download permission. Blob bytes are integrity-verified before output.

## Database boundary

The schema-owner identity performs migrations. The ordinary runtime identity is not table owner, superuser or `BYPASSRLS`. Authoritative ledger rows cannot be updated/deleted by the runtime role. Blob metadata/chunks are durable supporting state and are insert/read-only to runtime. Security, incident, obligation, access and evidence-register current state are rebuildable projections from ledger history.

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
