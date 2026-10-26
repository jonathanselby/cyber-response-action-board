# CRAB — generated codebase EDD

**Analysis method:** `jonathanselby/codebase-to-edd`  
**Repository:** `jonathanselby/cyber-response-action-board`  
**Immutable implementation ref analysed:** `77adcde68a271683d4c94706bac89955530d84ea`  
**Branch context:** `main`  
**Licence:** MIT  
**Purpose:** independent implementation description for comparison with frozen `EDD.md` V0.4

This document describes the running system evidenced by code and tests at the pinned commit. It does not replace the frozen design EDD. Findings are classified as **KNOWN**, **INFERRED** or **UNKNOWN** where material.

---

## 1. Executive system description

**KNOWN** — CRAB is one Go application for cyber-incident operation from intake through response, recovery, review and closure. Work enters through the browser, local JSON API, CLI, MCP stdio and print/recovery commands. The executable composition root is `cmd/crab`.

**KNOWN** — PostgreSQL is the only durable live application data store. It contains the authoritative signed/hash-chained ledger, immutable evidence/corpus bytes, authentication/security supporting records and rebuildable projections. Filesystem outputs are deployment material, backups or derived artefacts rather than a second live business-state store.

**KNOWN** — governed mutation uses a small command model: immediate immutable `Record(...)` observations and `Propose(...) -> Decide(...)` maker-checker state. Application services authorise before domain records are created; the PostgreSQL kernel then appends the authoritative ledger entry and applies projections in one transaction.

**KNOWN** — model assistance and external notification are optional edges. Model output is untrusted and cannot become governed state without ordinary human action. External delivery is thin and policy remains inside CRAB. Local PDF output is generated without a browser/network renderer.

**KNOWN** — the runtime is designed to fail closed around integrity, scope, outputs and hostile content while retaining degraded paths such as local authentication, deterministic retrieval, paper artefacts, backup/restore and ledger verification.

The principal qualification is operational rather than architectural: physical Raspberry Pi resource performance and final human browser/print visual acceptance remain release evidence outside repository-only analysis.

## 2. Scope and evidence basis

### Included

- `cmd/crab` executable composition and command entry points;
- browser/API/MCP/print operator paths;
- application/domain/security boundaries;
- PostgreSQL ledger, stores, projections, blobs and recovery mechanics;
- optional model and notifier adapters;
- local PDF/derived artefact handling;
- configuration, CI, architecture tests and Bafflewick acceptance.

### Excluded

- uncommitted deployment configuration and secrets;
- actual production topology;
- physical Raspberry Pi measurements;
- subjective browser/print visual quality not provable from source;
- behaviour of third-party managed services beyond CRAB's adapter contracts.

### Representative evidence

- `cmd/crab/main.go`, `operator_runtime.go`, `interfaces.go`, `drop2_*.go`;
- `internal/presentation/web/*`, `internal/presentation/mcp/*`, `internal/presentation/pdf/*`;
- `internal/app/*`, `internal/domain/*`, `internal/security/*`;
- `internal/data/postgres/kernel.go`, stores/runtimes/projections, backup/restore code and migrations;
- `internal/ledger/*`;
- `internal/adapters/models/*`, `notifier/*`, `artifacts/*`;
- `internal/config/config.go`;
- `internal/architecture/imports_test.go`;
- `.github/workflows/test.yml`, `.github/workflows/bafflewick.yml`;
- integration, negative-security, rebuild and degraded-operation tests.

## 3. Repository map

| Area | Runtime responsibility | Evidence |
|---|---|---|
| `cmd/crab` | Single executable, configuration/composition root, CLI, operator runtime, backup/restore, readiness, API/MCP/print wiring | `main.go`, `operator_runtime.go`, `interfaces.go` |
| `internal/presentation` | HTTP/API/MCP/PDF syntax and rendering; minimum view/input models | `web`, `mcp`, `pdf` |
| `internal/app` | Use-case orchestration, authorisation calls, controlled state transitions, retrieval/model/print/recovery composition | service files and tests |
| `internal/domain` | Semantic records, validation, deterministic calculations and view types | domain files/tests |
| `internal/security` | Authentication, authorisation, ceremonies and related policy primitives | `authn`, `authz`, `access`, `ceremony` |
| `internal/data/postgres` | SQL persistence, migrations, ledger transaction kernel, projections, immutable blobs, backup/restore support | `kernel.go`, stores/projections/tests |
| `internal/ledger` | Canonical signed/hash-chained ledger format and portable verification | ledger package/tests |
| `internal/adapters` | Optional managed/local models, notifier edges and derived artefact storage | adapter packages/tests |
| `scripts` / `.github` | Build, final acceptance, Pi harness and Bafflewick regression gates | scripts/workflows |

## 4. Component model

| Component | Purpose | Inputs | Outputs | State / dependencies | Confidence |
|---|---|---|---|---|---|
| CRAB composition root | Build one runtime from deployment configuration | files/env/CLI | configured services and interfaces | PostgreSQL, keys, optional adapters | KNOWN |
| Web/API presentation | Parse authenticated requests and render authorised models | HTTP | HTML/JSON | runtime interfaces only | KNOWN |
| MCP presentation | Authenticated local stdio interface | MCP messages + session | MCP responses | operator runtime | KNOWN |
| Application services | Enforce use-case control and invoke domain/persistence contracts | principal + semantic inputs | records/views/errors | interfaces to stores/runtimes | KNOWN |
| Authorisation evaluator | Single deny-by-default role/scope/output decision | principal, action, authoritative scope | allow/deny + reason | no SQL | KNOWN |
| Domain | Validate semantic records and deterministic calculations | typed inputs | typed immutable/governed data | no PostgreSQL | KNOWN |
| PostgreSQL kernel | Integrity gate and atomic ledger/projection transaction | ledger record + signer + projections | authoritative entry/current projection | PostgreSQL | KNOWN |
| Projections/stores | Rebuildable query state and supporting persistent records | verified ledger entries / queries | current views | PostgreSQL | KNOWN |
| Blob store | Bounded immutable hostile-byte persistence | stream + metadata | opaque blob/hash/exact bytes | PostgreSQL | KNOWN |
| Retrieval | Scope-filtered deterministic lexical retrieval | authorised query | cited passages/refusal | governed corpus projections | KNOWN |
| Model service/adapters | Optional untrusted drafting | authorised cited context + selected lane | untrusted draft/trace/error | local or managed provider | KNOWN |
| Notifier | Deliver already-authorised communication on approved channels | issued communication | immutable outcomes | local file or HTTPS webhook | KNOWN |
| Print service/renderer | Compose authorised print models and local PDFs | principal + current state | PDFs + last-good derived copy | gopdf + local filesystem | KNOWN |
| Backup/restore | PostgreSQL-native portable operational recovery | DB URL + file | custom dump/restore + metadata | `pg_dump`, `pg_restore` | KNOWN |

## 5. Runtime flows

### Flow A — governed operator mutation

1. HTTP route receives authenticated operator input through `internal/presentation/web`.
2. The presentation package passes syntax-level values to the runtime interface; it does not import PostgreSQL or security policy.
3. `cmd/crab/operator_runtime.go` resolves the opaque session to a projected principal and calls an application service.
4. The application service invokes the shared authorisation boundary using role, incident, compartment and separate output/action authority where relevant.
5. The domain layer validates the semantic payload and constructs either an immediate record or a governed proposal/decision.
6. A PostgreSQL runtime calls `Kernel.Append`.
7. The kernel verifies its current integrity state/tail, takes the ledger lock, inserts the signed authoritative entry and applies projections in the same transaction.
8. Commit updates the verified tail; any projection/commit failure rolls the mutation back.
9. Query paths return authorised, purpose-built views.

Failure branches include expired/invalid session, denied scope/action, invalid semantic input, exact-version mismatch, self-confirmation refusal, ledger integrity failure, unexpected tail and PostgreSQL failure.

### Flow B — hostile evidence capture and later use

1. An authenticated incident-scoped request reaches the evidence operator surface.
2. The application authorises the principal for the incident/compartment and record action.
3. Evidence bytes are streamed into PostgreSQL under configured bounds while SHA-256 is calculated; each upload receives an opaque blob identity rather than global content-addressed identity.
4. Evidence metadata is bound to the incident/scope and authoritative ledger/projection path.
5. Register/history reads are scope-filtered.
6. Attachment access re-resolves current authority rather than trusting the opaque ID as authority.
7. Bytes are integrity-checked before emission; safe-inline policy is server-controlled and active/ambiguous/mismatched content falls back to safe attachment handling.

Failures such as oversize/partial upload, scope substitution or hash corruption fail closed and do not silently create usable evidence.

### Flow C — degraded operation and recovery

1. Optional model failure returns a visible model error while deterministic authorised sources remain available; capacity saturation sheds new model work before core work.
2. Corporate identity can be isolated while valid local authentication remains independent.
3. Generated incident/emergency PDFs are derived files and remain readable if PostgreSQL later disappears; paper-era events are reconciled as new immutable records with original occurrence time distinct from transcription time.
4. Backup verifies the authoritative ledger before a PostgreSQL custom-format dump is committed with SHA-256 metadata.
5. Restore verifies the backup before `pg_restore`, performs independent ledger verification and deterministic projection rebuild through the explicit recovery path.
6. Recovery, stand-down, PIR and closure use ordinary governed application semantics; closed incident mutation is refused while permitted system operations remain possible.

## 6. State and persistence

| Store / state | Content | Authority | Lifecycle | Confidence |
|---|---|---|---|---|
| PostgreSQL `ledger_entries` | Exact authoritative signed/hash-chained entries | authoritative | append-only | KNOWN |
| PostgreSQL projections | incidents, actions, obligations, access, communications, recovery, retrieval indexes, timelines | derived | rebuildable from verified ledger where designed | KNOWN |
| PostgreSQL immutable blobs | evidence/corpus source bytes and blob metadata/chunks | supporting durable content | immutable application boundary | KNOWN |
| PostgreSQL security support | credentials, sessions and related authentication-support records | supporting runtime state | controlled by security stores | KNOWN |
| Deployment key/config files | signing/keyring/wrap keys and deployment adapter settings | deployment authority, not business state | operator-managed private files | KNOWN |
| PDF/export files | Incident Brief, Emergency Pack and other derived artefacts | derived | atomic last-good copies | KNOWN |
| PostgreSQL backup file + metadata | recovery copy of live store | backup, not live authority | operator-managed | KNOWN |
| In-process model capacity/session composition | transient execution control | transient | process lifetime | KNOWN |

No second live business database, object store, vector store, queue or workflow database is evidenced.

## 7. Interfaces

| Interface | Direction | Contract | Evidence |
|---|---|---|---|
| Browser HTTP/HTTPS | inbound | embedded server-rendered forms/views, authenticated sessions, security headers | web package / `runServe` |
| Local JSON API | inbound | selected operator functions on the same runtime | web API routes |
| MCP stdio | inbound/local | authenticated operator session file + MCP messages | `runMCP`, MCP presentation |
| CLI | inbound/local | readiness, verify, rebuild, export, backup, restore, setup, serve, print, recover | `cmd/crab/main.go` |
| PostgreSQL | internal/external process | pgx SQL and PostgreSQL-native backup/restore | data package |
| Model provider HTTP | outbound optional | configured lane/provider contracts; managed sensitivity gate precedes I/O | model service/adapters |
| Notifier | outbound optional | local file sink or HTTPS webhook for approved issued communication | notifier adapter |
| Filesystem | deployment/derived | private key/config/session files; atomic derived output/backup files | config/interfaces/artifacts/backup |

## 8. External dependencies

The Go module has three direct runtime libraries: `pgx/v5`, `gopdf`, and `golang.org/x/image`. PostgreSQL is the essential external runtime service. `pg_dump`/`pg_restore` are required only for backup/restore commands. Managed model providers and HTTPS webhook endpoints are optional and their failure does not become core-state authority.

The browser UI embeds static assets and does not require externally hosted frontend assets for core operation.

## 9. Configuration

`internal/config/config.go` is file-first through `CRAB_CONFIG_FILE`, with deployment environment overrides. Material settings include database/maintenance URLs; listen/public/TLS values; session/signing/keyring/wrap-key files; export directories; page size; evidence byte limit; Ollama URL; model configuration file; notification configuration file.

Non-loopback serving is refused unless TLS certificate/key and an HTTPS public base URL are configured. Page size is constrained to A4/LETTER and evidence limits must be positive. Business rules are deliberately not accepted as deployment configuration.

## 10. Failure and recovery behaviour

| Failure | Behaviour | Recovery | Confidence |
|---|---|---|---|
| Ledger verification failure | sticky integrity-failed mutation refusal | explicit verified rebuild; read/export paths remain separately controlled | KNOWN |
| Unexpected ledger tail | mutation refused and integrity failure made sticky | independent verify/rebuild | KNOWN |
| PostgreSQL unavailable | live DB-backed operation unavailable; existing derived paper artefacts remain readable | backup/restore/replacement host + paper reconciliation | KNOWN |
| Corporate federation unavailable/isolate mode | federation refused; local auth path remains available if configured | local authentication / later normalisation | KNOWN |
| Model provider unavailable | visible failure; authorised sources retained; no silent reroute | operator-selected/configured provider later restored | KNOWN |
| Model capacity saturated | new model work shed before core deterministic work | capacity becomes available | KNOWN |
| Notification channel fails | immutable failure outcome; later sequential channels may still run | human/operator follow-up; no retry service | KNOWN |
| Evidence corruption | hash mismatch / unsafe emission refusal | preserve history; obtain valid bytes through governed process | KNOWN |
| Backup corruption | restore refused before PostgreSQL restore | use valid backup | KNOWN |
| Closed incident mutation | refused | none unless governed product semantics explicitly permit relevant system operation | KNOWN |

## 11. Security boundaries

**KNOWN** — authentication and authorisation are separate. `authz.Authorise` is deny-by-default and evaluates authentication, role, incident membership, compartment membership and separate output/action permission. Custodian system-metadata administration is separately scoped and does not grant incident content.

**KNOWN** — opaque object IDs are not accepted as authority; object scope is resolved before authorisation where applicable. Break-glass changes authentication, not incident/compartment authorisation. Agents bind to a named human and inherit no greater authority.

**KNOWN** — governed maker-checker decisions reload stored pending proposals and exact versions rather than accepting caller-supplied effective bodies. Presentation packages are statically prevented from importing PostgreSQL or security policy, and application/security/domain packages are prevented from importing concrete PostgreSQL.

**KNOWN** — managed model disclosure and outputs such as print/export/download/notify are separately authorised. Exercise-mode external notification is suppressed before external I/O.

**KNOWN** — the deployment trusts Linux root and unrestricted PostgreSQL administration for confidentiality; ledger verification is integrity evidence, not encryption against that trusted computing base.

## 12. Operational behaviour

- One compiled Go executable is built for Linux amd64 and arm64 in CI.
- HTTP server has explicit read/write/idle timeouts and graceful signal-driven shutdown.
- `/healthz` exposes a small runtime/version/time health response; detailed operational confidence is provided through readiness logic.
- CI enforces module tidiness, gofmt, vet, race-enabled full tests, both architecture builds and explicit core acceptance.
- `internal/architecture/imports_test.go` protects tier import boundaries.
- Bafflewick adds an explicit 9/9 end-to-end regression gate over existing production-path tests.
- Projection replay is deliberately bounded one verified sequence at a time rather than loading an unbounded ledger into memory.
- Physical Pi performance remains to be measured rather than inferred from CI.

## 13. Known / inferred / unknown

### Known

- one Go application/composition root;
- PostgreSQL single durable live store;
- authoritative signed/hash-chained ledger with atomic projection updates;
- deny-by-default scoped authorisation;
- immediate Record and governed Propose/Decide semantics;
- immutable blobs, deterministic retrieval, optional models/notifiers and local print;
- backup/restore/rebuild and paper reconciliation paths;
- no workflow engine, message bus, vector database, object store or chat subsystem;
- tier boundaries are executable CI constraints.

### Inferred

- `cmd/crab` is intentionally the allowed composition root that may know concrete data adapters while presentation/application/domain tiers remain isolated; this is consistent with the frozen package shape and import tests.
- filesystem deployment key/config material is outside the phrase "single durable live store" because it is deployment custody, not live business state.

### Unknown

- measured Raspberry Pi 5 resource usage under the final Bafflewick load;
- final human judgement of browser/print visual quality and accessibility on target devices;
- organisation-specific production PKI/key-custody operating procedures;
- real managed-provider/webhook service availability beyond tested adapter contracts.

## 14. Open questions

No material architecture question remains unresolved by the repository. The remaining release questions are empirical:

1. Does the final build meet the frozen Pi CPU/RAM/latency/disk budgets on the physical target?
2. Does final browser and print rendering pass human visual/accessibility review on target devices and paper?

## 15. Architecture diagrams

### System/context

```text
Responders / executives / advisers / authorised agents
                         |
          +--------------+---------------+
          |              |               |
       Browser        CLI/MCP          Print
          |              |               |
          +--------------v---------------+
                         |
                  ONE crab binary
                         |
       +-----------------+------------------+
       | application/domain/security         |
       | authz + Record + Propose -> Decide  |
       +-----------------+------------------+
                         |
                  PostgreSQL
       ledger | blobs | security | projections
                         |
             +-----------+-----------+
             |                       |
      derived files            optional edges
    PDF/export/backup       models / notifications
```

### Governed mutation flow

```text
HTTP / API / MCP / CLI
        |
        v
presentation syntax boundary
        |
        v
composition runtime -> authenticated principal
        |
        v
application service -> authz.Authorise
        |
        v
domain semantic validation
        |
        v
PostgreSQL runtime
        |
        v
Kernel.Append
  | ledger tail/integrity gate
  | signed ledger INSERT
  | projection Apply
  +---- one DB transaction ----+
        |
      COMMIT
        |
        v
authorised purpose-built view
```

---

## Implementation conclusion

At `77adcde68a271683d4c94706bac89955530d84ea`, CRAB's implemented architecture is materially the same machine described by frozen EDD V0.4. Differences are implementation detail and additional completed presentation/composition coverage, not a replacement architecture. See `docs/edd-drift-review.md` for the explicit comparison.
