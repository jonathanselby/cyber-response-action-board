# CRAB — frozen EDD V0.4 drift review

**Implementation analysed:** `77adcde68a271683d4c94706bac89955530d84ea`  
**Frozen design:** `EDD.md` V0.4  
**Method:** `jonathanselby/codebase-to-edd` evidence hierarchy  
**Result:** **PASS — no material architectural drift detected**

This is a comparison, not a redesign exercise. The test is whether the completed code silently weakened or replaced the frozen architecture, security boundaries or degraded-operation model.

## Material invariants

| Frozen V0.4 invariant | Implementation evidence | Result |
|---|---|---|
| One Go application | one `cmd/crab` executable; browser/API/MCP/CLI/print composed into the same application | PASS |
| PostgreSQL is the single durable live store | ledger, supporting blobs/security and projections are PostgreSQL; files are deployment material, backups or derived output | PASS |
| Presentation contains no SQL/security decisions | `internal/architecture/imports_test.go` rejects data/pgx/security imports from presentation | PASS |
| Application/domain/security do not depend on concrete PostgreSQL | same static import gate plus service interfaces | PASS |
| One deny-by-default authorisation boundary | `internal/security/authz.Authorise` plus authoritative scope resolution and separate output permissions | PASS |
| Two write semantics only | immediate `Record(...)` and governed `Propose(...) -> Decide(...)`; tests reject controlled state through immediate records | PASS |
| Ledger authoritative; current state rebuildable | `Kernel.Append` atomically inserts ledger + projections; `Kernel.Rebuild` verifies then replays | PASS |
| Integrity failure fails closed | full verification on kernel open, sticky integrity-failed mode, unexpected-tail refusal | PASS |
| Hostile bytes remain bounded and immutable | PostgreSQL blob store, per-upload IDs, SHA-256, scoped current re-authorisation and safe presentation | PASS |
| Deterministic retrieval before model assistance | scope filtering and lexical retrieval precede optional model use; unsupported retrieval refuses | PASS |
| Model is optional/untrusted/non-authoritative | provider failure retains sources, no silent reroute, human governed write still required | PASS |
| Notifications are thin, not a delivery platform | local/webhook adapters, immutable outcomes, no retry service/queue; exercise suppression before external I/O | PASS |
| Local professional print without browser renderer | pure-Go PDF package and atomic derived-copy store | PASS |
| Paper remains a degraded-operation path, not second live state | PDFs survive DB loss; paper events reconcile as later immutable ledger records | PASS |
| RLS retained only if simple enough | real pooled-connection spike was run and production RLS rejected under the frozen conditional gate because it required a second privileged projection path | PASS |
| No workflow engine, vector DB, object store, chat system or message bus | module graph, source tree and architecture tests show none introduced | PASS |
| amd64/arm64 and constrained-hardware intent preserved | both architectures build in every normal CI run; physical Pi measurement remains explicit release evidence | PASS |

## Drop 2 review

The five Drop 2 commits added operator-facing access/readiness, evidence, obligation, communication/enquiry and integrated incident-control surfaces. They did **not** introduce a new persistence layer, scheduler, workflow engine, security model or separate application. The web package still talks through runtime interfaces; application services retain policy; existing PostgreSQL runtimes/stores remain the persistence implementations.

Result: **presentation/composition completion, not architecture drift.**

## Bafflewick acceptance review

Commit `77adcde68a271683d4c94706bac89955530d84ea` adds the named Bafflewick regression gate as CI/test composition only. It binds existing production-path tests into eight mandatory operational phases and does not add a runtime subsystem or product state.

Result: **acceptance evidence, not architecture drift.**

## Documentation drift found

One material descriptive file was stale:

- `architecture.txt` still described the 0.0.2-dev Slice 1 checkpoint and claimed authentication, incident semantics and later slices were unimplemented.

That statement contradicted the current code and README but did not affect runtime behaviour. This acceptance component updates `architecture.txt` to describe the completed implementation while retaining the frozen V0.4 architectural diagram.

`README.md` at the analysed ref still lists Bafflewick among remaining release evidence because the Bafflewick gate was only just added. Release-state wording should be consolidated after all final acceptance components complete rather than churned after each gate.

## Security drift result

No weakening was found in the design minimums:

- Major is not universal incident authority;
- Custodian does not obtain incident content by role;
- Sponsor/Controller governance remains maker-checker;
- break-glass changes authentication rather than authorisation;
- object IDs do not confer scope;
- restricted compartments remain explicit;
- managed model disclosure and output actions remain separately controlled;
- governed proposal decisions are exact-version and reload stored proposal state;
- root/DBA trust boundary remains explicit rather than falsely encrypted around.

## Simplicity drift result

The implementation remains a mini-monolith. New capabilities mostly reuse the ledger kernel, projections, timed-wait pattern, application services and presentation interfaces. Direct runtime dependencies remain `pgx`, `gopdf` and `x/image`. The completed product has grown in surface area without growing a corresponding estate of infrastructure services.

## Accepted implementation qualifications

These are not drift, but they materially describe the final implementation:

1. `cmd/crab` is the composition root and therefore legitimately imports concrete PostgreSQL adapters while the presentation/application/domain/security tiers remain isolated.
2. `pg_dump` and `pg_restore` are operational dependencies for backup/restore commands; they do not become application state stores.
3. deployment keys/config/session files are filesystem custody material, not a second live business database.
4. derived PDF/export and backup files are deliberately outside the live-store statement.
5. physical Pi performance and human visual quality remain empirical acceptance evidence and cannot be inferred from source CI.

## Final decision

**PASS. Do not reopen EDD V0.4.**

The code has not drifted into a different architecture. Correct stale documentation, retain the frozen EDD as the design contract, and continue only with remaining empirical release acceptance.
