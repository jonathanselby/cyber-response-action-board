# CRAB — Generated Engineering View

**Method:** Jonno's Garage `codebase-to-edd`  
**Repository:** `jonathanselby/cyber-response-action-board`  
**Pinned software commit:** `f4c9d59084723334b2808aeeb0deaf96dce26b11`  
**Purpose:** Slice 15 code-to-design conformance check against the frozen CRAB EDD v0.4.

## 1. Executive system description

**KNOWN** — CRAB is a single Go application with browser and CLI entry paths. The application composes domain/application services over PostgreSQL-backed adapters. PostgreSQL is the durable live store. Governed state is recorded through a signed append-only ledger and rebuilt into deterministic projections. The browser presentation layer contains no direct PostgreSQL or security-authority dependency.

**KNOWN** — the operational browser path uses local authentication, opaque server-side sessions, a scoped principal resolved from current access projections, and existing incident application services. The browser carries a session token and neutral view models; authorisation remains below presentation.

## 2. Scope and evidence basis

Evidence inspected for this reconstruction includes:

- `cmd/crab/main.go` and `cmd/crab/operator_runtime.go`;
- `internal/app/*` application services;
- `internal/domain/*` records, operating-picture and control types;
- `internal/data/postgres/*` ledger kernel, projections, stores, backup/restore and rebuild paths;
- `internal/security/authn`, `authz` and `ceremony`;
- `internal/presentation/web/*`;
- repository tests and `.github/workflows/test.yml`.

Generated documentation, repository history and the frozen EDD are used only for comparison, not as proof of runtime behaviour.

## 3. Repository/runtime map

```text
cmd/crab
  |
  +-- CLI: readiness / verify / rebuild / backup / restore
  |
  +-- serve
       |
       v
internal/presentation/web
       |
       | session token + neutral commands/views
       v
cmd/crab/operator_runtime
       |
       +--> security/authn --> PostgreSQL security store
       +--> data/postgres/AccessStore --> scoped authz principal
       +--> internal/app services
                 |
                 v
          data/postgres runtimes
                 |
                 v
          signed ledger + projections
                 |
                 v
              PostgreSQL
```

## 4. Component model

### Presentation

**KNOWN** — `internal/presentation/web` owns HTTP routing, forms, cookies, rendering, same-origin checks, response headers and embedded static assets. It does not make role/scope decisions or issue SQL.

### Composition

**KNOWN** — `cmd/crab` wires configuration, key custody, PostgreSQL, authentication/session services and application services. `operator_runtime.go` converts the session token into the current scoped principal before calling the application tier.

### Application/domain

**KNOWN** — `internal/app` contains semantic use-case services. `internal/domain` contains record payloads, operating-picture types, governance semantics and deterministic calculations. These packages are not coupled to concrete PostgreSQL implementation details.

### Security

**KNOWN** — local password authentication and opaque sessions are separate from authorisation. `AccessStore.Principal` constructs the current principal from role eligibility, incident membership, compartment membership, Custodian status and output permissions. Seniority alone does not create incident scope.

### Data and ledger

**KNOWN** — `internal/data/postgres` owns the concrete durable store, signed ledger kernel, projections, rebuild, immutable evidence/blob support, paper/print sources, recovery state, and native PostgreSQL backup/restore orchestration.

## 5. Representative runtime flows

### Browser read

```text
GET incident
 -> session cookie
 -> resolve server-side session
 -> build current scoped principal
 -> app.CommonOperatingPicture
 -> authorised projection reads
 -> neutral browser view
```

Failure behaviour: missing session redirects/refuses; unresolved or unauthorised scope does not widen access; integrity/configuration failures prevent operator runtime startup.

### Governed mutation

```text
POST action / declaration / decision
 -> same-origin + bounded form
 -> session resolution
 -> scoped principal
 -> application semantic method
 -> authz decision
 -> proposal/confirmation ledger append
 -> deterministic projection
 -> redirect/read refreshed state
```

Maker-checker semantics remain in the application/domain/data path rather than in HTML handlers.

### Bootstrap

```text
local first-run page
 -> two distinct humans + local credentials
 -> ceremony validation
 -> bootstrap records
 -> one atomic ledger batch
 -> AccessProjection
 -> bootstrap disabled once minimum authority exists
```

## 6. State and persistence

**KNOWN** — PostgreSQL is the single durable live state store. The ledger is authoritative for governed business/configuration state. Operational tables are projections. Authentication/session material is durable security-supporting state but is not reconstructed from the public ledger. Derived exports and paper files are not authoritative live stores.

**KNOWN** — backup uses PostgreSQL-native tooling with hashes/metadata. Restore verifies the backup, restores into a replacement database, verifies the ledger, rebuilds projections and verifies again.

## 7. Interfaces and dependencies

**KNOWN** — principal interfaces are the browser HTTP surface, CLI maintenance commands, PostgreSQL, filesystem key/config/export paths and optional external/model/notifier lanes implemented in their respective services.

**KNOWN** — browser assets are embedded/same-origin. Core browser rendering does not require an internet-hosted CDN, font or script.

## 8. Failure and recovery behaviour

**KNOWN** — ledger integrity failure gates mutation. Optional model failure does not become an authority or durable workflow dependency. Backup/restore and projection rebuild are explicit maintenance paths. Paper output provides a derived contingency path. The application uses deny-by-default authorisation and fail-closed alternate CLI control paths where an operator adapter is not implemented.

## 9. Security boundaries

**KNOWN** — authentication, role eligibility, incident assignment, compartment membership and action/output permission are distinct controls. Custodian status is not universal incident-content authority. Break-glass authentication does not widen principal scope. Presentation has no SQL/pgx/PostgreSQL import path.

**KNOWN** — browser mutations require an authenticated session and same-origin request. Session cookies are HttpOnly and SameSite Strict; TLS requests additionally mark them Secure. Browser responses use no-store, restrictive CSP, no-referrer, nosniff and frame denial headers.

## 10. Operational behaviour

**KNOWN** — one binary is built for Linux amd64 and arm64. Readiness, ledger verification, rebuild, backup and restore are explicit CLI operations. CI enforces module tidiness, formatting, vet, race-enabled unit/PostgreSQL integration tests, both architecture builds and core checkpoint acceptance.

## 11. Known / inferred / unknown

**KNOWN** — code structure matches the frozen three-tier intent: presentation -> composition/application -> data, with domain/application free of concrete PostgreSQL coupling.

**KNOWN** — PostgreSQL remains the only durable live store; no workflow engine, queue, vector database or second authoritative application store has appeared.

**KNOWN** — the operator web path reuses existing semantic services rather than recreating business logic in handlers.

**INFERRED** — the resulting mini-monolith remains suitable for the intended small independent host because no new runtime service was introduced in Slice 14. Resource proof requires the real Pi run.

**UNKNOWN** — actual Raspberry Pi 5 resource consumption until measured on the specified rig.

**UNKNOWN** — human judgement of visual polish until the rendered web and representative A4/Letter outputs are inspected.

## 12. Comparison with frozen EDD v0.4

| Frozen design constraint | Code-derived result | Finding |
|---|---|---|
| Single Go application | One `cmd/crab` application, amd64/arm64 builds | CONFORMS |
| Classic three-tier separation | Presentation has no PostgreSQL/security-authority dependency; composition/application/data remain separated | CONFORMS |
| PostgreSQL single durable live store | All durable operational state remains PostgreSQL-backed | CONFORMS |
| Immutable ledger + rebuildable projections | Kernel append/verify and projection rebuild remain central | CONFORMS |
| Scoped authority; Major/Custodian not universal | Principal is assembled from explicit scoped memberships/permissions | CONFORMS |
| Maker-checker governed changes | Existing propose/decide semantic path retained | CONFORMS |
| Offline-capable core/browser | Embedded same-origin web assets; local auth/session path | CONFORMS |
| Paper contingency | Existing print/paper service retained; operator page has print-specific rendering | CONFORMS |
| No unnecessary workflow/service machinery | No new service, queue, workflow engine or store introduced | CONFORMS |
| Recovery/backup/restore | Existing Slice 12/13 paths retained | CONFORMS |

**Code-to-design result:** no material architectural contradiction found in the pinned Slice 14 software commit. Remaining uncertainty is acceptance evidence, not missing architecture: Pi resource measurement and human visual inspection.

## 13. Open acceptance questions

1. Does the Pi 5 / 8 GB run remain inside the frozen resource budgets under the Bafflewick workload?
2. Does the rendered browser UI meet the required production-quality visual standard under stress-sized data?
3. Do representative A4 and Letter outputs pass human inspection for hierarchy, clipping and monochrome legibility?
