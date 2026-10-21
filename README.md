# CRAB — Cyber Response Action Board

CRAB is a single Go application for running a cyber incident from first report through active response, recovery, review and closure while remaining useful when normal corporate systems are unavailable.

> **The model proposes. One person makes. Another confirms. Nothing is erased.**

## Build status

**0.0.14-dev — release-candidate checkpoint; automated acceptance complete.**

The frozen product contract is PRD V0.2 and EDD V0.4. The implementation may simplify mechanisms; it may not silently weaken product outcomes, security boundaries or degraded-operation behaviour.

CI-proven now:

- one `crab` executable with Linux amd64 and arm64 build gates;
- PostgreSQL as the single durable live store, with separate schema-owner/runtime identities;
- signed/hash-chained authoritative ledger, deterministic projections, full verification, rebuild and portable JSONL export;
- deny-by-default authentication and authorisation with local credentials, passkeys, TOTP, optional OIDC, isolate mode, break-glass and out-of-band verification;
- Major/Minor/Observer incident authority separated from Custodian, Sponsor and Compartment Controller governance;
- incident and compartment assignment enforced independently of role seniority;
- immediate immutable `Record(...)` observations and governed `Propose(...) -> Decide(...)` maker-checker state;
- incidents, facts, unknowns/hypotheses, decisions, actions, operational roles, checklists and timeline;
- organisation-defined obligation clocks and bounded rebuildable timed waits without a legal rules engine;
- immutable hostile-byte evidence storage in PostgreSQL with bounded streaming, SHA-256 verification, safe attachment handling and evidence history;
- governed Markdown/DOCX corpus ingestion and deterministic scope-filter-before-rank retrieval with stable citations;
- optional three-lane model assistance with local Ollama and managed Azure/Foundry, Bedrock and Vertex adapters; model output is always untrusted and never becomes governed state automatically;
- structured communications, cadence, incoming enquiries and thin local/webhook notification with exercise suppression before external I/O;
- pure-Go local PDF generation for Incident Brief, Emergency Response Pack, evidence register, timeline and review/closure material;
- A4/Letter, colour/monochrome, deterministic wrapping/page breaks and embedded font data with no browser/network renderer dependency;
- atomic last-good primary/secondary derived-copy handling;
- paper-era reconciliation that preserves original occurrence time separately from later digital transcription time;
- recovery coordination, stand-down, PIR, retention/disposition and closure, with closed incidents read-only;
- secure operator browser path for bootstrap, login, incident declaration, fact recording, action proposal and maker-checker decisions;
- local JSON API and authenticated MCP stdio adapter on the same operator runtime and application controls;
- authenticated CLI print using a private operator-session file;
- PostgreSQL-native backup, SHA-256 metadata, replacement-host restore, independent ledger verification and deterministic projection rebuild;
- readiness derived from actual key, backup/restore, export, storage, paper-pack, model and notification deployment evidence;
- local Custodian recovery that is refused while normal Custodian quorum is demonstrably usable and never grants incident-content access;
- non-loopback serving refused unless TLS certificate/key and an HTTPS public base URL are configured;
- file-first optional model and notification deployment composition, with local model endpoints constrained to loopback and managed endpoints constrained to HTTPS;
- explicit Bafflewick end-to-end regression acceptance with an 8/8 mandatory threshold;
- generated codebase-to-EDD comparison against frozen EDD V0.4 with no material architecture drift found;
- automated visual/accessibility preflight covering responsive/mobile contracts, keyboard focus, touch targets and local A4/Letter/monochrome print contracts.

The current candidate passes normal module/gofmt/`go vet`/race-enabled PostgreSQL CI, Linux amd64 and arm64 builds, the explicit core acceptance gate, Bafflewick 8/8 and the visual/accessibility preflight. See `docs/final-acceptance-progress.md`.

### Remaining release evidence

The product mechanisms and repository-automatable acceptance are complete. A stable release tag is intentionally withheld until two empirical gates are recorded:

1. Raspberry Pi 5 / 8 GB arm64 physical resource-budget run using `scripts/final-acceptance.sh`;
2. final human browser and print visual review using `docs/visual-acceptance.md`.

These are acceptance activities, not permission for another architectural rewrite.

## Commands

```text
crab version
crab readiness
crab verify --ledger <file> --keyring <file>
crab rebuild
crab export-ledger --output <file>
crab backup --output <file>
crab restore --input <file>
crab setup
crab serve
crab mcp --session-file <path>
crab print incident-brief --incident <id> --session-file <path>
crab print emergency-pack --organisation <name> --session-file <path>
crab recover --request-file <path>
```

Database-backed commands require the configured PostgreSQL and ledger-keyring paths. Operator mode additionally requires the server signing key. Sensitive session/recovery files must be private to the current user; CRAB refuses group/other-readable files.

`crab serve` may use loopback HTTP for local bootstrap/operation. A non-loopback listener requires TLS and an HTTPS public base URL.

`crab backup` verifies the authoritative ledger before creating a PostgreSQL custom-format backup. `crab restore` verifies backup integrity, restores through the maintenance connection, independently verifies the ledger, rebuilds projections and records restore-rehearsal evidence.

## Deployment configuration

Deployment configuration is file-first with environment overrides. Organisation business rules do not belong in deployment configuration; obligations, roles, access policy and incident data remain governed application state.

Important settings include:

```text
CRAB_CONFIG_FILE
CRAB_DATABASE_URL
CRAB_MAINTENANCE_DATABASE_URL
CRAB_LISTEN_ADDR
CRAB_PUBLIC_BASE_URL
CRAB_TLS_CERT_FILE
CRAB_TLS_KEY_FILE
CRAB_SESSION_KEY_FILE
CRAB_SERVER_SIGNING_KEY_FILE
CRAB_LEDGER_KEYRING_FILE
CRAB_CREDENTIAL_WRAP_KEY_FILE
CRAB_EXPORT_DIR
CRAB_SECONDARY_EXPORT_DIR
CRAB_PAGE_SIZE_DEFAULT
CRAB_EVIDENCE_MAX_BYTES
CRAB_OLLAMA_URL
CRAB_MODEL_CONFIG_FILE
CRAB_NOTIFICATION_CONFIG_FILE
```

Model and notification deployment files must be private local JSON files. Managed-provider authentication is referenced by environment-variable name from the private model deployment file; authentication material is not stored in repository configuration. A configured Ollama endpoint must be loopback-only. Managed provider endpoints must use HTTPS.

Notification channels are currently either local file sinks or HTTPS webhooks. CRAB derives whether a channel is external from its type; callers cannot relabel an external webhook as local. Exercise-mode external delivery is suppressed before adapter I/O.

## Write semantics

CRAB keeps the write model deliberately small:

```text
Record(...)                  observation/history: append now
Propose(...) -> Decide(...)  governed state: maker proposes, different eligible human confirms or rejects
```

Corrections are new records or versions. Governed history is not edited in place.

Model drafts are neither semantic. They are transient, untrusted assistance. A human still has to perform the ordinary CRAB record or maker-checker action before model-derived wording can affect incident state.

## Security boundary

The application decision is intentionally small:

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

For retrieval, ranking is not a security boundary: PostgreSQL first returns only authorised passages. For managed model use, retrieval authority is still not external-disclosure authority: sensitivity policy and separate `model_send` permission must pass before provider I/O. For notification and print, ordinary read access does not imply send/export authority.

## Durable-state boundary

PostgreSQL is the single durable live store. The ledger is authoritative for governed business/configuration state; operational current state is rebuildable. Immutable evidence/corpus blobs live in PostgreSQL as supporting durable content. Derived PDFs and exports are files, not a second live state store.

CRAB deliberately does **not** introduce a workflow engine, message bus, delivery/retry service, vector database, object store, chat/helpdesk system, print service, recovery orchestrator, records-management engine, autonomous agent framework or universal administrator.

PostgreSQL RLS was empirically tested. It was not retained in V0.4 because the extra privileged rebuild/projection channel required by the current one-runtime-role design made the system harder to reason about than the defence-in-depth gain justified. The application authorisation boundary remains primary and the rejected RLS spike is documented in `docs/rls-spike-decision.md`.

## Governing documents

- `PRD.md` — product contract, V0.2.
- `EDD.md` — frozen architecture, V0.4.
- `BUILD-NOTES.md` — implementation sequencing and build discipline.
- `TEST-ACCEPTANCE-NOTES.md` — behavioural/security acceptance catalogue.
- `UX-PRINT-BUILD-NOTES.md` — web, accessibility and print quality requirements.
- `docs/build-evidence-0.0.14-dev.md` — code-complete implementation checkpoint evidence.
- `docs/bafflewick-acceptance.md` — explicit operational regression gate.
- `docs/generated-codebase-edd.md` and `docs/edd-drift-review.md` — recovered implementation architecture and frozen-EDD comparison.
- `docs/visual-acceptance.md` — automated preflight plus human rendered-review contract.
- `docs/pi-final-acceptance.md` — one-command physical minimum-hardware acceptance.
- `docs/final-acceptance-progress.md` — current release-gate status.

## Toolchain

CI is pinned to Go 1.27.2. The module baseline is Go 1.25 because the pinned PostgreSQL driver requires Go 1.25 or newer.
