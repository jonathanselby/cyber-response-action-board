# CRAB — Build Notes

**Version:** 0.1  
**Date:** 20 October 2026  
**Companion to:** `CRAB-EDD-v0.4-FINAL.md`

These notes are deliberately outside the EDD. They are implementation guidance, sequencing, spikes and build discipline. They may change as code teaches us things without reopening the locked product/security architecture.

---

## 1. Build doctrine

1. Build one application vertically, not a collection of frameworks.
2. Keep the PRD and final EDD open during implementation.
3. Reuse Garage component semantics/code where useful; do not reproduce their repository boundaries as services.
4. Create a new abstraction only after at least two real call sites or a clear security/dependency boundary proves it.
5. Prefer boring Go and PostgreSQL.
6. Keep runtime AI optional.
7. Push security and failure testing into the first implementation of each path.
8. Make the primary UX purpose-built even when the backend is generic.
9. Do not build future physical deletion, semantic/vector search, media processing or vendor integrations speculatively.
10. The Raspberry Pi fixture is an engineering constraint/test, not the architecture target.

---

## 2. Suggested implementation sequence

Build in vertical slices that leave a runnable system after each meaningful step.

### Slice 0 — repository skeleton and build discipline

- Go module / pinned version.
- `cmd/crab` command skeleton.
- configuration loader;
- structured exit codes;
- CI for amd64/arm64 build;
- dependency/import checks enforcing tier boundaries;
- migration harness;
- test fixture/bootstrap database;
- build/version metadata;
- embedded static asset pipeline with no Node dependency.

Do not create domain packages merely to fill the tree.

### Slice 1 — ledger + projection kernel

Implement first because most other product features depend on it.

- PostgreSQL connection/pool;
- schema owner/runtime identities in local test setup;
- ledger canonicalisation/signature lineage from `decision-ledger`;
- append transaction + tail lock;
- projection interface;
- full verification;
- integrity-failed read-only mode;
- rebuild command;
- portable JSONL ledger export/verify.

Proof before moving on:

- append/replay equality;
- tamper detection;
- projection rollback with ledger append failure;
- restore/replay reproduces logical state.

### Slice 2 — authentication + security boundary

Use lineage from `authentication`, `authorisation-dual-control` and `out-of-band-verification`.

Build:

- local identity/credential support;
- session boundary;
- Major/Minor/Observer eligibility;
- Custodian metadata authority;
- application `Authorise(principal, action, objectScope)`;
- incident and compartment membership projections;
- object-to-scope binding;
- output/action matrix;
- rate limits;
- break-glass authentication;
- isolate mode;
- OOB verification;
- bootstrap ceremony;
- Custodian recovery ceremony.

Security negative tests should arrive with the code, not after UI completion.

### Slice 3 — incident + semantic write kernel

Implement the two write semantics explicitly:

```text
Record(...)
Propose(...) -> Decide(...)
```

Add only the semantic schemas needed by the first vertical incident flow:

- incident declaration;
- fact;
- unknown/hypothesis;
- decision;
- action;
- operational role assignment.

Build the first common operating picture from projections.

This slice proves that generic internals can still produce semantic forms/views.

### Slice 4 — actions/checklists + timeline

- dedicated Action UX;
- owner/status/priority/due time;
- overdue derivation;
- checklist template activation -> ordinary Actions;
- skip-with-reason;
- timeline projection with occurrence/flagged/recorded/awareness semantics.

Do not build a workflow engine.

### Slice 5 — obligations + timed wait list

- obligation configuration schemas;
- deterministic clock lineage from `obligation-clock`;
- provisional/firm deadline semantics;
- amendment/extension/waiver/replacement history;
- timed wait-list rebuild;
- upcoming/overdue presentation.

Keep legal applicability human/configuration-owned.

### Slice 6 — access governance + compartments

- incident Sponsor;
- Compartment Controller;
- incident membership grants/revocations/expiry;
- GENERAL implicit after governed incident membership;
- restricted compartment read-in/read-out;
- access administration views using metadata only;
- external responder expiry/revocation;
- selective release UI.

Run the RLS spike here because the projections/blobs and connection-pool pattern now exist.

### Slice 7 — hostile blobs + evidence

Refactor the `immutable-blob-store` primitive into PostgreSQL.

Implement:

- bounded streaming;
- SHA-256;
- opaque per-upload blob ID;
- blob metadata/chunks;
- atomic/staged commit semantics;
- no cross-incident dedupe;
- exact-byte retrieval;
- current re-authorisation;
- safe download headers;
- evidence register;
- phone-browser capture.

Then run the safe-inline media spike.

### Slice 8 — retrieval + corpus

- governed Markdown/DOCX ingestion;
- immutable source content;
- deterministic lexical index/rank;
- incident/compartment filter before ranking;
- stable citations;
- refusal when unsupported.

No vector DB.

### Slice 9 — model assistance

Use the three-lane harness only after the deterministic retrieval path is complete.

- static CRAB draft specs;
- provider/lane policy;
- sensitivity gate before network call;
- provenance/latency/token traces;
- untrusted draft output;
- no automatic provider reroute;
- queue shedding.

Evaluation remains primarily CI evidence rather than a production service.

### Slice 10 — communications + notifier

- semantic communication types;
- draft -> propose -> confirm -> issue;
- situation-report cadence;
- local sink;
- generic webhook;
- delivered/failed/suppressed immutable outcomes;
- exercise suppression before external I/O;
- incoming enquiry capture/assignment.

No chat subsystem. No retry service.

### Slice 11 — print/paper

Run the PDF-renderer spike before committing deeply to renderer-specific code.

Implement:

- Incident Brief;
- Emergency Response Pack;
- evidence register;
- timeline;
- PIR/closure outputs;
- blank contingency forms;
- A4/Letter;
- monochrome;
- last-good atomic primary and optional secondary copies;
- source ledger/config version markings.

Print composition belongs in the application layer; typography/layout belongs in the presentation renderer.

### Slice 12 — recovery / PIR / retention / closure

- recovery coordination;
- derived operational phases;
- stand-down decision;
- PIR generation + human lessons + confirmation;
- closure prerequisites;
- privileged/shareable closure;
- closed read-only enforcement;
- retention/hold/disposition-eligibility metadata;
- no physical deletion.

### Slice 13 — readiness / backup / restore

- readiness page/CLI;
- PostgreSQL-native backup orchestration;
- hashes/result metadata;
- replacement-host restore;
- projection/retrieval/wait-list rebuild;
- key-custody failure modes;
- full independent ledger verify.

### Slice 14 — Bafflewick + Pi + visual acceptance

Only now call the product feature-complete.

Run:

- full Bafflewick scenario;
- attack/negative tests;
- outage/degraded-operation tests;
- Pi resource run;
- accessibility pass;
- web visual review;
- print visual review;
- generated EDD via `codebase-to-edd` and comparison against hand-written final EDD.

---

## 3. Package discipline

Start with the broad tier/security groups from the final EDD.

Do not pre-create every V0.3 package.

A new package earns its place if at least one is true:

- it prevents an illegal dependency;
- it isolates a security-sensitive area for review;
- it encapsulates a genuinely reusable pure function;
- it has enough coherent code that a package improves comprehension;
- it isolates an optional external adapter.

A package does **not** earn its place merely because a noun exists in the PRD.

Examples:

- obligation clock likely earns a small pure package/function set;
- every Item subtype does not need a package;
- handover does not need a package/state machine;
- related incidents do not need a graph subsystem;
- retention does not need a records-management service.

---

## 4. Application API shape

Keep application commands/queries semantic even when internals share primitives.

Good:

```text
DeclareIncident(...)
RecordFirstReport(...)
RecordFact(...)
ProposeSeverityChange(...)
CreateAction(...)
ConfirmAction(...)
AddEvidence(...)
IssueSituationReport(...)
GrantIncidentAccess(...)
ReadIntoCompartment(...)
StandDown(...)
ConfirmPIR(...)
CloseIncident(...)
```

Avoid making every adapter call a public generic primitive such as:

```text
CreateItem(kind, payload)
```

The generic primitive may exist below the application semantic boundary.

This makes it difficult for web/API/MCP callers to bypass domain intent.

---

## 5. Record versus governed change implementation

Internally define the invariant once.

### `Record`

Must:

- authenticate;
- authorise;
- validate semantic payload;
- bind incident/compartment;
- assign provenance/times;
- append immutable ledger record;
- update allowed observational projections.

Must not:

- alter effective controlled state merely because the payload says it should;
- issue an approved external communication;
- change severity/access/obligation/configuration;
- make model output authoritative.

### `Propose`

Must:

- authenticate/authorise maker;
- validate semantic payload;
- capture exact version;
- append proposed ledger entry;
- optionally derive provisional outputs such as clocks;
- expose for confirmation.

### `Decide`

Must:

- authorise confirmer;
- ensure proposal version has not changed;
- enforce different-human rule unless an explicitly permitted controlled self-approval applies;
- append confirmed/rejected decision;
- atomically update projections;
- schedule only deterministic follow-on work.

---

## 6. Authorisation implementation notes

A good internal shape is:

```text
Authorise(principal, action, ScopedObject) -> Allow/Deny
```

`ScopedObject` should provide or resolve:

- object ID/type;
- incident ID if any;
- compartment;
- live/exercise/system mode;
- other small control metadata required for the action.

Do not let call sites supply arbitrary incident/compartment claims that are not verified against authoritative metadata.

Repository methods for sensitive incident data should be scope-explicit; avoid unscoped convenience functions that make cross-incident mistakes easy.

Examples:

```text
GetEvidence(scope, evidenceID)
ListActions(scope, filter)
SearchCorpus(scope, query)
```

rather than unrestricted `GetByID` functions exposed throughout the domain.

---

## 7. Action permission matrix

Keep the action vocabulary small and explicit. A likely starting set:

```text
view
record
propose
confirm
administer_access
view_attachment
inline_media
download_attachment
print
bulk_export
model_send
notify_external
backup
restore
```

Do not create a new permission for every screen button.

Domain rules still determine whether a role may perform an action on a semantic object.

The matrix is governed configuration/code policy, not presentation logic.

---

## 8. Bootstrap and recovery implementation notes

Bootstrap must be testable in automation and usable by a small organisation.

Suggested ceremony:

1. `crab setup` creates pending setup and local temporary bootstrap session on the host.
2. Human A enrols a local credential and enters minimum setup including both initial people/Custodian designations.
3. Human B independently enrols/authenticates and confirms the exact pending setup digest.
4. Application signs/ledgers the confirmed setup and disables the one-time bootstrap path.
5. Repeat execution refuses unless explicit installation-recovery conditions are satisfied.

Custodian recovery:

- local operator command only;
- unavailable while normal Custodian quorum is demonstrably usable unless explicit policy allows recovery testing in exercise/setup mode;
- requires both offline recovery proofs and two named humans;
- may only restore/replace Custodian administrative authority;
- always ledgered and readiness-visible;
- never grants incident/compartment membership.

Red-team this implementation heavily because bootstrap is where elegant security models often acquire an accidental root account.

---

## 9. RLS spike plan

Purpose: determine whether PostgreSQL RLS cheaply catches application query mistakes.

Use the real pgx pooling/transaction pattern.

Test at least:

- user A / Incident A then user B / Incident B reuse same pooled connection;
- transaction-local scope always clears on commit/rollback;
- deliberate missing `WHERE incident_id` still cannot cross scope if RLS retained;
- deliberate wrong compartment cannot cross scope;
- migration owner cannot be used by runtime;
- runtime role is not owner/superuser/BYPASSRLS;
- projection rebuild has an explicit privileged operational path that cannot leak into normal runtime;
- backup/restore remains documented;
- no maker-checker/action policy migrates into SQL.

Retain RLS only if the result is simpler to reason about than it is to recover from.

---

## 10. PostgreSQL blob notes

Do not default to PostgreSQL Large Objects without evidence.

Start with chunk rows because they keep transaction/backup behaviour ordinary and visible.

Things to measure:

- 1 MiB / 4 MiB / 8 MiB chunk candidates;
- pgx streaming behaviour;
- transaction duration for configured fixture sizes;
- backup/restore impact;
- evidence download throughput;
- memory usage on Pi;
- deletion is irrelevant in V0.4, so do not optimise for blob garbage collection.

Hash while streaming.

Do not store the entire evidence file in memory.

Do not deduplicate across incidents.

---

## 11. Safe-inline media spike

Goal: improve responder usability without broadening hostile-content attack surface.

Start from download-only and earn additions.

For each candidate type test:

- server-side identification strategy;
- fixed safe response MIME;
- browser behaviour with `nosniff`;
- content disposition;
- CSP interaction;
- polyglot/mislabelled fixtures;
- malformed/corrupt fixtures;
- cross-origin behaviour;
- no script execution;
- no automatic upload parsing beyond bounded safe metadata needed for presentation.

Likely passive image/audio/video formats may earn inline support. SVG/HTML and active office/executable formats should not.

Do not introduce thumbnail/transcoding infrastructure merely to make inline display work.

---

## 12. Retrieval implementation notes

Keep source ingestion deterministic and bounded.

For DOCX:

- treat ZIP/XML as data;
- impose zip-entry/expanded-size limits;
- never execute macros/embedded active content;
- extract only required text/provenance;
- keep original governed source blob.

Ranking can stay simple. Good retrieval quality should come primarily from:

- clean governed corpus;
- sensible chunking/heading context;
- filter-before-rank;
- exact/keyword weighting;
- stable citations;
- refusal threshold.

Do not add embeddings because keyword quality needs tuning.

---

## 13. Model build notes

Reuse the three-lane model harness contract.

CRAB-specific drafting specs should be small static definitions containing:

- purpose;
- prompt version;
- allowed lane(s);
- input sensitivity rule;
- output schema/type;
- provenance requirements;
- deterministic acceptance checks where possible.

Examples may include:

- draft situation report;
- summarise authorised retrieved passages;
- draft board update;
- draft notification language;
- draft PIR summary.

No runtime workflow graph is needed.

If model output fails validation, keep sources and return visible failure/untrusted text; never silently coerce it into governed state.

---

## 14. Queue/wait-list notes

Keep both bounded.

Priority queue likely classes:

1. ledger/signature/integrity control;
2. authentication/authorisation/maker-checker;
3. board/clocks/notifier;
4. timeline/evidence/action projections;
5. print/export/backup;
6. retrieval;
7. model generation.

Tune after measurement, but model work remains first to shed.

Timed wait-list rows/entries are rebuildable. They should point to authoritative/configured source state rather than becoming another source of truth.

Do not add a hidden retry queue for notifications.

---

## 15. Web implementation notes

Use server-rendered pages first.

Small JavaScript is appropriate for:

- progressive form interactions;
- file capture/progress;
- confirmation dialogs;
- keyboard helpers;
- targeted dynamic refresh where it materially reduces cognitive load.

Do not create a client-side state store duplicating server/domain state.

Keep current incident and compartment context prominent in every operational surface.

Do not use generic Item labels in operator-facing pages.

---

## 16. PDF renderer spike

The library must prove:

- arm64 build;
- deterministic local rendering;
- embedded fonts/assets without network;
- tables and wrapping;
- page-break control;
- headers/footers/page numbers;
- A4/Letter;
- monochrome fixtures;
- reasonable memory/CPU on Pi;
- no headless browser dependency.

If `gopdf` fails, replace it. Do not weaken the paper requirement to protect the library choice.

---

## 17. Configuration discipline

Keep three different concepts separate:

**Deployment configuration**

- DB URL;
- listen address;
- export paths;
- secrets/key paths;
- provider endpoints;
- resource ceilings.

**Governed organisation configuration**

- contacts;
- roles;
- obligations;
- calendars;
- impact tiers;
- checklists;
- compartments;
- holding statements;
- notifier recipient definitions.

**Incident state**

- actual live/exercise response data.

Do not let deployment YAML become an unaudited shortcut for changing business configuration.

---

## 18. Migration discipline

Every migration should state:

- affected table/class;
- authority category: authoritative / supporting / rebuildable;
- backwards/forwards compatibility expectation;
- replay impact;
- backup/restore impact;
- whether portable verifier format changes;
- whether migration can run with CRAB live or requires maintenance mode.

Avoid migrations that rewrite historical ledger payloads. Prefer versioned readers/migration projections where feasible.

---

## 19. Failure-path implementation notes

Write the failure path with the happy path.

Examples:

- database write fails after payload validation;
- projection application fails in transaction;
- confirmer loses access between proposal and confirmation;
- access expires while browser page remains open;
- blob upload disconnects halfway;
- notifier timeout;
- model provider hangs;
- PDF export disk fills;
- clock calendar invalid;
- time becomes untrusted;
- restore lacks signing private key;
- corpus DOCX is malformed/zip bomb;
- browser requests an unauthorised blob ID directly.

The user should see an explicit governed/operational outcome rather than a misleading success state.

---

## 20. Micro-component discipline

No new CRAB micro-components are planned.

`immutable-blob-store` is already extracted and complete as a reference primitive.

During build, extract nothing else merely because code looks reusable.

After CRAB is working, another substantial Garage application may prove an implementation independently reusable. Extract then.

Reusable architecture is not automatically a reusable repository.

---

## 21. Build evidence to retain in Git

Keep enough evidence that a reviewer can see design discipline rather than only source code.

At minimum:

- PRD V0.2;
- EDD V0.4 final;
- these build notes;
- test/acceptance notes;
- UX/print build notes;
- migrations;
- architecture ASCII diagram;
- threat/security regression test outputs;
- Bafflewick acceptance record;
- Pi benchmark record;
- screenshots of key screens;
- A4/Letter colour/monochrome PDF fixtures;
- backup/restore rehearsal record;
- independent ledger verification fixture;
- generated EDD from `codebase-to-edd`;
- hand-written/generated EDD comparison;
- tagged/released build artefacts when complete.

---

## 22. Build stop signs

Stop and return to the final EDD before introducing any of these:

- another durable live store;
- another scheduler;
- another approval path;
- a workflow engine;
- per-object ACL graph;
- service-to-service internal HTTP;
- Redis/message bus;
- semantic/vector search stack;
- new vendor SDK estate;
- automatic evidence processing;
- root/DBA encryption layer;
- physical destruction;
- native mobile app;
- generic UI replacing semantic workflows.

Any one may someday be justified. None is justified by the current PRD.
