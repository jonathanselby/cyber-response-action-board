# Cyber Response Action Board (CRAB) — Engineering Design Document

**Version:** 0.4  
**Status:** FINAL DESIGN — READY TO BUILD  
**Design date:** 20 October 2026  
**Product baseline:** `CRAB-PRD-v0.2.md`  
**Supersedes:** `CRAB-EDD-v0.3.md`

> **Garage rule:** Never remove a mission requirement to make the implementation elegant. Find the smallest set of mechanisms that delivers the entire mission requirement.

This document is the final pre-build engineering baseline for CRAB. PRD V0.2 remains the product contract. This EDD locks the minimum architecture, security floor and functional boundaries needed to implement all 88 PRD acceptance criteria without turning CRAB into a service estate, generic workflow product or engineering prototype.

The central design conclusion is simple:

> **CRAB is a large product built from a small machine.**

The machine is one Go application, one PostgreSQL durable store, one ledger, one authorisation decision, two write semantics, deterministic projections, two schedulers, one hostile-byte boundary, deterministic retrieval, optional model assistance and local professional print.

---

## 1. Mission

CRAB helps a team run a cyber incident from first report through active response, recovery, stand-down, Post-Incident Review (PIR), retention hand-off and administrative closure while remaining useful when internet access, corporate identity, normal collaboration systems, model services, the corporate network or the live CRAB database are unavailable.

The governing operating rule remains:

> **The model proposes. One person makes. Another confirms. Nothing is erased.**

CRAB is designed for non-specialists operating under pressure. Engineering simplicity must improve reliability and legibility; it must not reduce product capability, security, accessibility, visual quality or paper resilience.

---

## 2. Product boundary

CRAB is an incident-management and decision-support product. It is not:

- a SIEM or security-monitoring platform;
- a SOAR or technical remediation engine;
- an endpoint-control system;
- a malware-analysis or forensic-acquisition platform;
- a general helpdesk, project-management or problem-management suite;
- a general-purpose chat product;
- a graphical workflow/orchestration engine;
- an integration marketplace;
- a document-management suite;
- a multi-tenant SaaS control plane;
- a universal DLP/DRM or employee-surveillance product;
- a system that determines legal applicability, privilege or reportability;
- a system that claims confidentiality against a malicious Linux root user or unrestricted PostgreSQL administrator.

CRAB may coordinate technical remediation, legal work, insurer activity, suppliers, communications and recovery. It does not become the system that performs those activities.

---

## 3. Architecture in one page

CRAB uses a classic three-tier architecture inside one executable.

```text
RESPONDERS / EXECUTIVES / ADVISERS / AUTHORISED AGENTS
                         |
                         v
+-----------------------------------------------------------+
| PRESENTATION                                              |
|                                                           |
| Web | JSON API | CLI | MCP | local PDF renderer          |
|                                                           |
| Parse syntax, collect input, render authorised models.    |
| NO business/security policy. NO SQL.                      |
+----------------------------+------------------------------+
                             |
                             v
+-----------------------------------------------------------+
| APPLICATION / DOMAIN                                      |
|                                                           |
| Authentication                                            |
| Authorisation                                             |
| Record(...)                                               |
| Propose(...) -> Decide(...)                               |
| Incident / actions / obligations / access / evidence      |
| Retrieval / communications / readiness / print models     |
| Deterministic projections                                 |
| Bounded work queue | timed wait list                      |
| Optional model and notifier policy                        |
+----------------------------+------------------------------+
                             |
                             v
+-----------------------------------------------------------+
| DATA                                                      |
|                       PostgreSQL                          |
|                                                           |
| Signed append-only ledger                                 |
| Immutable blobs                                           |
| Credential/security support data                          |
| Rebuildable projections                                   |
+-----------------------------------------------------------+

DERIVED, NEVER LIVE AUTHORITY
  Incident Brief
  Emergency Response Pack
  other PDFs / exports
  database backups
  portable ledger export

OPTIONAL OUTBOUND EDGES
  local or managed model providers
  configured notification channels
```

There is:

- one operated application binary;
- one durable live store;
- one authoritative governed-state ledger;
- one authorisation model;
- one maker-checker implementation;
- two write semantics;
- two schedulers;
- no message bus;
- no Redis;
- no vector database;
- no service mesh;
- no internal HTTP between CRAB components;
- no workflow service;
- no external frontend application;
- no second CRAB database.

---

## 4. Hard engineering invariants

The following are locked.

### 4.1 One application

CRAB builds as one Go application for `linux/amd64` and `linux/arm64`.

Runtime components are internal code boundaries, not separately operated services.

### 4.2 Three tiers remain distinct

Presentation, application/domain and data responsibilities may be simplified within a tier but not collapsed across tiers.

Presentation:

- contains no business rules;
- contains no security-policy decisions;
- contains no CRAB persistence SQL;
- renders only authorised view/print models.

Application/domain:

- owns business meaning;
- owns authentication/authorisation orchestration;
- owns state transitions and governance;
- owns authorised query/view composition;
- depends on store interfaces, not PostgreSQL implementation details.

Data:

- is the only tier that issues CRAB persistence SQL;
- owns migrations and database transactions;
- does not invent business policy.

### 4.3 PostgreSQL is the single durable live store

PostgreSQL contains:

- the authoritative ledger;
- immutable evidence/corpus blobs;
- credential and security-supporting records;
- rebuildable projections and index metadata;
- optional short-lived session state.

Filesystem PDFs, backups and portable exports are sensitive derived copies, not second live authorities.

### 4.4 One authoritative history

Governed organisational and incident state is established through the append-only signed ledger.

Operational tables are projections and can be destroyed and rebuilt.

No hidden second state store may become necessary to explain the business state.

### 4.5 Generic mechanisms must not create a generic product

Internal reuse must never leak into operator language or screen design.

The user sees:

- Actions as actions;
- Decisions as decisions;
- Facts as facts;
- Unknowns as unknowns;
- Evidence as evidence;
- Obligations as clocks/obligations;
- Communications as communications;
- Access as access;
- Handover as handover.

The implementation may share envelopes, governance and projection machinery underneath them.

A generic `New Item -> choose type -> generic form` user experience is explicitly rejected.

---

## 5. Code boundary

The exact number of Go packages is not a design goal. Boundaries exist only where they protect dependency direction, security review or genuinely different concerns.

A suitable initial shape is:

```text
cmd/crab

internal/presentation/
    web
    api
    cli
    mcp
    pdf

internal/app/
    command/query services
    authorised view/print models
    transaction orchestration

internal/security/
    authn
    authz
    access governance
    isolate / break-glass
    out-of-band verification

internal/domain/
    incident
    records/governed changes
    actions/checklists
    obligations/time
    evidence metadata
    retrieval
    communications
    projections
    readiness

internal/adapters/
    models
    notifier
    local/export edges

internal/data/postgres/
    SQL
    migrations
    repositories
    transactions
```

Subpackages are created only when code size, dependency isolation or auditability proves the need.

Compile/import tests enforce:

```text
presentation  X----> PostgreSQL
presentation  X----> security/business decisions

domain       X----> pgx / CRAB SQL

app/domain/security ----> store interfaces ----> data/postgres
```

`internal/security` remains explicit because the access model deserves disproportionate auditability.

---

## 6. The two write semantics

V0.3 used one Item lifecycle but still described many domain concepts as if they might need distinct mechanics. V0.4 makes the real simplification explicit.

CRAB has two write semantics.

### 6.1 `Record(...)`

`Record` preserves something that happened, was received, was observed or was produced without making it authoritative organisational state.

Examples:

- first report received;
- responder progress/comment;
- incoming stakeholder enquiry;
- evidence capture metadata;
- notification delivery outcome;
- agent/model finding recorded as untrusted output;
- paper transcription provenance;
- observed handling event.

A record is:

- authenticated;
- authorised;
- schema-validated;
- incident/compartment scoped where applicable;
- signed/provenanced as configured;
- immutable once written;
- ledgered;
- visible through appropriate projections.

A `Record` may not silently alter effective governed state.

For example:

```text
record: "Responder believes severity should be Critical"
               |
               X  does not change effective severity
               |
               v
Major proposes severity = Critical
               |
               v
separate confirmer
               |
               v
effective severity changes
```

### 6.2 `Propose(...) -> Decide(...)`

Any change to effective organisational, security or incident state uses the single governed-change path.

```text
                  +------------+
                  |  proposed  |
                  +------+-----+
                         |
            +------------+-------------+
            |                          |
            v                          v
      +-----------+              +-----------+
      | confirmed |              | rejected  |
      +-----------+              +-----------+

amendment = new proposed version superseding an earlier confirmed version
```

Examples include:

- incident declaration;
- severity change;
- confirmed fact/assessment where it becomes authoritative;
- decision;
- action creation or material governed change;
- obligation/configuration change;
- privilege/compartment classification change;
- incident membership;
- compartment read-in/read-out;
- operational role assignment;
- isolate-mode change;
- issued governed communication;
- stand-down;
- PIR confirmation;
- closure;
- retention/hold/disposition decision.

There is one maker-checker implementation.

The normal rule is that the maker cannot confirm the same proposed version.

Any permitted single-person window remains explicitly governed, time-bounded, visible and flagged. It does not silently apply to ordinary access grants.

---

## 7. Common record envelope and semantic contracts

The common envelope contains only universal provenance/control metadata.

```text
RecordEnvelope
  record_id
  incident_id?              # absent for system/setup scope where appropriate
  kind
  subtype?
  schema_version
  record_version
  supersedes_version?
  mode                      # live | exercise | system
  governance                # recorded | proposed | confirmed | rejected
  author_human_id
  actor_agent_id?
  compartment
  created_at
  recorded_at
  time_status               # trusted | drifting | untrusted
  source_ref?
  payload
  maker_signature?
  confirmer_human_id?
  confirmer_signature?
  self_approval?
  self_approval_reason?
  ledger_chain_fields
```

Domain-specific dates, owners, deadlines, audiences, evidence properties and awareness times live in versioned payload schemas rather than bloating the universal envelope.

CRAB retains explicit semantic contracts for at least:

- incident declaration/assessment;
- fact;
- unknown/question;
- hypothesis/assumption;
- decision;
- action;
- checklist template/step provenance;
- obligation;
- evidence record;
- communication / situation report;
- incoming enquiry;
- operational role/deputy assignment;
- incident membership/access change;
- compartment membership/access change;
- Sponsor/Controller/Custodian designation change;
- retention/hold/disposition metadata;
- related-incident relationship;
- recovery/stand-down/PIR/closure records.

These contracts may reuse the same persistence and governance mechanics but retain their own validation, view models, forms and product language.

---

## 8. Incident state and operational phase

The persisted incident lifecycle stays deliberately small:

```text
live ----------------------> closed
```

CRAB does not create a second mutable incident-phase state machine.

Operational phase is a first-class deterministic projection from confirmed records:

```text
no confirmed stand-down                         -> ACTIVE RESPONSE
recovery work dominant / configured conditions -> RECOVERY
confirmed stand-down                            -> STOOD DOWN / REVIEW
confirmed PIR + closure prerequisites met       -> CLOSURE READY
confirmed incident-close decision               -> CLOSED
```

The exact labels may be refined during UX build, but the functional distinctions remain explicit on screen, in handover and in printed material.

Stand-down does not make the incident read-only.

Closure does.

---

## 9. Security floor — non-negotiable

No later Garage simplification may reduce the controls in this section without reopening the security design.

### 9.1 One authorisation decision

All web, API, CLI and MCP reads/writes converge on the same application authorisation boundary.

```text
ALLOW =
    authenticated principal
    AND role permits requested action
    AND active incident membership where required
    AND required compartment membership
    AND requested object belongs to that authorised scope
    AND requested output/action is separately permitted

otherwise DENY
```

`object` is not a second arbitrary ACL system. It means the application resolves the object's authoritative incident and compartment and re-evaluates access before returning or changing it.

Possession or guessing of an ID never confers authority.

### 9.2 Roles, incident membership and compartments

CRAB retains three incident-participation roles:

- Major;
- Minor;
- Observer.

Role determines what a person may do.

Incident membership determines where they may do it.

Compartment membership determines what information they may know.

No role creates universal visibility.

A governed incident-membership grant includes the ordinary `GENERAL` incident working compartment.

Restricted compartments require explicit read-in.

Being authenticated, a Major, a Custodian or merely present in the system does not itself grant `GENERAL` incident access.

### 9.3 No arbitrary per-object ACL estate

Selective release uses incident/compartment classification and action permissions rather than a second per-object ACL graph.

An individual report, communication or attachment may be placed into a restricted compartment smaller than the incident as a whole.

The UI may make this a simple visibility/classification choice; the underlying control remains the compartment model.

### 9.4 Data exits are separate actions

Every route by which CRAB can disclose information is independently authorised.

```text
screen / browser view
search / retrieval result
attachment view/download
PDF / print
API / MCP response
bulk export / ledger export
managed model provider
notification / external communication
backup / removable media
```

Ordinary read permission does not automatically imply:

- attachment download;
- bulk export;
- print;
- managed-model send;
- notification send;
- backup creation/removal.

### 9.5 Hidden means hidden

List, search, count and retrieval queries apply incident/compartment scope before pagination, ranking or model prompting.

Unauthorised incidents/compartments do not appear through:

- lists;
- counts;
- search results;
- retrieval citations;
- direct-object requests;
- generated output.

Error behaviour should avoid becoming an existence oracle where practical.

### 9.6 Custodian, Sponsor and Compartment Controller

These concepts remain because they separate different kinds of authority, but they use one generic governed access-change mechanism.

**Custodian**

- administers identity and access metadata;
- verifies eligibility/identity for access changes;
- has no automatic incident-content access.

**Incident Sponsor**

- is scoped to one incident;
- answers whether a person needs mission access to that incident;
- proposes incident membership grant/revocation.

**Compartment Controller**

- is scoped to one compartment within one incident;
- answers whether a person needs knowledge of that compartment;
- proposes read-in/read-out.

Normal effective change:

```text
mission authority proposes
        |
        v
Custodian independently verifies/confirms
        |
        v
access projection changes
```

Maker and confirmer are different named humans.

### 9.7 Bootstrap

Fresh installation begins with no incident-content super-user.

`crab setup` establishes a pending system configuration containing at least:

- two distinct named human identities;
- at least two Custodian designations;
- at least two humans eligible to act as incident Majors;
- local authentication sufficient to survive corporate-identity loss;
- the two separately held offline break-glass credentials.

One named human prepares the setup. A second distinct named human independently authenticates and confirms it.

No incident exists at bootstrap, so bootstrap creates no incident-content visibility.

Incident declaration establishes initial incident membership and Sponsor designation through the normal governed path.

### 9.8 Custodian recovery

Loss of a usable Custodian quorum must not require a permanent universal administrator.

A separate local installation-recovery command may restore Custodian quorum only when normal Custodian governance is unavailable. It requires:

- access to the host/operator recovery path;
- both independently held offline recovery proofs used by the break-glass keyholders;
- two distinct named humans;
- an immutable recovery record;
- explicit reason;
- no incident read-in, compartment read-in, role elevation or export authority.

This is an installation authority-recovery mechanism, not ordinary break-glass and not emergency incident read-in.

### 9.9 Break-glass

Break-glass restores authentication only.

Two distinct valid offline keyholders are required.

Successful break-glass authentication leaves existing:

- role;
- incident memberships;
- compartment memberships;
- action/export permissions

unchanged.

There is no emergency read-in feature in V0.4.

If urgent mission access is needed, the normal visible Sponsor/Controller + Custodian governance path is used.

### 9.10 Root and DBA boundary

Linux root and unrestricted PostgreSQL DBA are part of the deployment trusted computing base in this version.

Tamper-evident ledger verification can detect many integrity changes. It does not create confidentiality against an administrator who can read plaintext storage.

Application-layer payload encryption against root/DBA is not added to V0.4 because it would introduce key-custody and recovery complexity that materially affects CRAB's offline/restore mission. A higher-assurance edition may revisit it with a separately designed key model.

### 9.11 PostgreSQL RLS defence-in-depth gate

Application authorisation is always primary.

A mandatory implementation spike tests simple PostgreSQL Row-Level Security on incident/compartment-scoped projections and blob references as independent defence against an accidental over-broad application query.

RLS is retained only if all of the following hold:

- policies remain simple incident/compartment filters;
- transaction-local security context cannot leak between pooled connections;
- runtime role is not table owner/superuser and has no `BYPASSRLS`;
- restore/rebuild/backup operations remain clear;
- negative tests prove both application denial and database denial;
- the control does not duplicate maker-checker or action/business policy.

If the spike introduces greater ambiguity or recovery risk than protection, it is removed and the reason/test evidence retained.

This decision gate cannot weaken the application authorisation invariant.

### 9.12 Browser and protocol security

The implementation retains at least:

- HTTPS for browser/API operation;
- secure `HttpOnly`/`SameSite` cookies where cookies are used;
- CSRF protection for browser mutations;
- Content Security Policy;
- no externally hosted scripts/styles/fonts needed for core operation;
- output encoding by templates;
- no trusted raw HTML from corpus/model content;
- authentication and break-glass rate limiting;
- WebAuthn origin/relying-party validation;
- safe credential hashing/work factors;
- TOTP secret wrapping/encryption using deployment-owned key material where practical;
- no secrets in normal logs, portable public ledger export or command arguments;
- `Cache-Control: no-store` for appropriate sensitive responses;
- no incident content persisted in browser `localStorage`;
- routine logs containing identifiers/outcomes rather than incident bodies/evidence/model prompts by default.

---

## 10. Durable state

CRAB separates authority from efficient current-state representation.

### 10.1 Authoritative

`ledger_entries`

The signed/hash-chained ledger is authoritative for governed business/configuration history and immutable operational records that are part of the audit history.

### 10.2 Durable supporting state

Examples:

- immutable blobs;
- credential records;
- key/public verification records;
- optional short-lived sessions.

These are required durable state but are not reconstructed solely from the portable public business ledger.

### 10.3 Rebuildable

Examples:

- incident projection;
- current semantic-record projections;
- actions;
- clocks;
- operational roles;
- evidence register;
- timeline;
- approvals;
- incident memberships;
- compartment memberships;
- Sponsor/Controller/Custodian current state;
- retrieval index metadata;
- timed-wait-list inputs;
- derived readiness metadata where reconstructable.

Every migration/table must be classified as authoritative, durable supporting or rebuildable.

Unknown authority is not permitted.

---

## 11. Ledger transaction and replay

### 11.1 Append transaction

Before governed mutation is enabled at startup, CRAB validates schema and performs full ledger verification.

Each committed append:

1. obtains the ledger-tail lock;
2. confirms expected previous chain hash;
3. canonicalises and signs the entry;
4. inserts the ledger entry;
5. updates affected projections in the same database transaction;
6. commits atomically.

If projection application fails, the ledger append rolls back with it.

Full verification is also run for explicit verify/readiness operations, backup, portable export, closure and restore/rebuild.

Ledger verification failure places CRAB into **integrity-failed read-only mode**:

- authorised read/export of existing material may continue;
- governed mutation is refused;
- recovery requires a verified history/restore decision.

### 11.2 Replay

Projection code is deterministic and versioned.

```text
verify ledger
    |
truncate rebuildable projections
    |
replay in ledger order
    |
recompute projections / retrieval metadata / wait-list inputs
    |
compare logical state/digest
    |
ready
```

Historical payloads carry schema version so replay never silently interprets old history using incompatible current assumptions.

---

## 12. Immutable blob boundary

`jonathanselby/immutable-blob-store` is a completed Garage reference component and governs the primitive semantics.

CRAB refactors that primitive into PostgreSQL rather than operating it as a separate service.

### 12.1 Blob contract

CRAB must:

- accept bounded untrusted bytes;
- preserve exact original bytes;
- calculate SHA-256 while streaming;
- refuse writes beyond configured limits before authoritative commit;
- expose an opaque per-upload blob identifier;
- preserve filename, size, declared/detected-safe metadata where available;
- verify integrity for integrity-sensitive retrieval/export;
- never execute uploaded bytes;
- re-authorise every view/download request;
- default active/ambiguous content to safe download.

### 12.2 Storage shape

The implementation may use ordinary PostgreSQL rows plus bounded chunks rather than PostgreSQL Large Object semantics.

Conceptually:

```text
blob
  blob_id
  sha256
  size
  created_at
  integrity metadata

blob_chunk
  blob_id
  sequence
  bytes
```

Chunk size is an implementation constant selected by measurement.

The upload is staged inside a bounded transaction or equivalent staging mechanism. No authoritative evidence reference becomes visible until the complete byte stream, size and hash are known and committed.

Failed/oversized uploads leave no usable partial authoritative blob.

### 12.3 No cross-incident content deduplication

CRAB does not deduplicate evidence across incidents merely because hashes match.

SHA-256 is integrity evidence, not a global storage identity.

This avoids reference-counting, retention and cross-incident coupling between otherwise separate incidents.

### 12.4 Safe presentation

The product should not become unnecessarily download-only.

A deliberately small server-controlled allow-list may render genuinely passive supported image/audio/video formats inline when this can be done without weakening the hostile-content boundary.

Rules:

- client MIME/extension is not trusted;
- server selects response content type;
- `X-Content-Type-Options: nosniff`;
- active/ambiguous types are download-only;
- HTML, SVG, executable content, macro-active documents and unknown formats are never served as trusted active inline content;
- no automatic OCR, media analysis, transcription, transcoding or thumbnail pipeline is required;
- every response is re-authorised at request time.

The exact allow-list is proven during build. Download-only is the safe fallback for any uncertain type.

---

## 13. Core product capability mapping

The full PRD remains in scope. The simplification works by composing capabilities from the same mechanisms rather than creating a subsystem for each noun.

### 13.1 Incident declaration and common operating picture

A Major may propose a live incident or exercise declaration; a separate eligible human confirms it under normal maker-checker.

The primary incident operating picture exposes, according to authorisation:

- incident identity and live/exercise mode;
- current operational phase;
- current situation;
- severity/impact and affected/suspected scope;
- known facts;
- unknowns/questions;
- hypotheses/assumptions distinctly labelled;
- operational role holders and deputies;
- open critical actions and owners;
- active obligation clocks;
- pending approvals/decisions;
- recent confirmed decisions;
- communications cadence/commitments;
- time trust;
- isolate/break-glass status;
- model availability;
- paper-pack freshness;
- relevant readiness warnings.

The common operating picture is a derived authorised view, not another stored state model.

### 13.2 Contacts, assets and operational authority

Organisation-defined data includes:

- contacts and alternate contact routes;
- internal/external responders;
- assets/services and impact tiers;
- insurers/regulators/other recipients;
- operational incident roles and deputies;
- delegated decision authorities.

Application permissions remain separate from organisational authority to shut down a service, spend money, notify externally or take another consequential action.

### 13.3 Actions and response checklists

Actions retain dedicated product semantics:

- owner;
- status;
- priority;
- due time where applicable;
- source decision/checklist/runbook;
- notes/evidence references.

Overdue state is derived.

Checklist templates create ordinary Action records with source/template provenance.

Checklist steps may be not started, complete or skipped with reason.

There is no workflow graph, branching language or orchestration engine.

### 13.4 Facts, timeline, uncertainty and awareness

Facts, unknowns and hypotheses remain visibly different semantic types.

No model or rule silently promotes a hypothesis into a confirmed fact.

The timeline is a derived projection from ledger history and domain event times. It can include:

- facts/reports;
- decisions;
- actions;
- obligations;
- evidence;
- operational-role changes;
- communications/notification outcomes;
- recovery activity;
- paper reconciliation;
- closure events.

Where relevant, CRAB distinguishes:

- occurred time;
- flagged time;
- recorded time;
- recipient/role awareness time;
- time trust/confidence.

### 13.5 Severity and evolving scope

CRAB preserves changing human assessments of:

- affected/suspected assets/services;
- people/customer/data populations;
- physical/cloud locations;
- suppliers/third parties;
- impact/confidence;
- severity.

Previous confirmed assessments remain in history.

Severity escalation/de-escalation is governed and records reason/evidence.

### 13.6 Obligations and clocks

Obligation configuration remains organisation-defined data.

CRAB does not decide legal applicability/reportability.

Deterministic obligation calculation supports configured:

- trigger field/event;
- duration;
- elapsed/calendar/business-day unit;
- timezone;
- business calendar;
- adjustment rule;
- applicability condition;
- recipient;
- owner/responsible role.

A proposed trigger may create a provisional clock; confirmation makes the resulting deadline firm where appropriate.

Amendment, extension, waiver or replacement preserves original rule/deadline, authority/source, reason and resulting status/deadline.

The clock remains a pure deterministic function plus projections/wake-ups; it is not a legal reasoning engine.

### 13.7 Retrieval and optional AI

V0.4 ingests Markdown and DOCX governed runbook/retrieval material.

Deterministic retrieval:

1. authorises principal and scope;
2. filters incident/compartment before ranking;
3. performs deterministic lexical/keyword ranking;
4. returns stable citations;
5. refuses when the authorised corpus does not support the question.

No embedding model, vector database or network is required.

Optional model drafting occurs only after retrieval and content-policy checks.

Model output remains untrusted draft/proposal text.

No model:

- approves;
- confirms;
- changes effective state;
- calculates authoritative legal deadlines;
- bypasses managed-lane sensitivity policy.

AI workflow behaviour is expressed as static CRAB-owned prompt/output/provenance contracts, not a runtime workflow engine.

### 13.8 Structured communications

CRAB supports semantically rich communication functions for:

- regulator notifications;
- notices to individuals;
- executive/board updates;
- staff communications;
- situation reports;
- holding statements;
- closure/PIR material;
- material incoming enquiries.

Drafting may be human or model-assisted.

**Issuing** a governed communication uses `Propose -> Decide` before it is treated as approved/issued.

Notification outcomes use immutable `Record` entries such as `delivered`, `failed` or `suppressed`.

CRAB does not become general chat or helpdesk software.

### 13.9 Evidence register and rich evidence

Evidence records retain appropriate metadata such as:

- evidence ID;
- description;
- source/collector;
- collection/occurrence/recording times;
- timezone/time trust;
- original filename/media metadata where available;
- byte size/hash;
- handler/custodian and handling events;
- related fact/decision/action;
- incident/compartment;
- immutable blob reference.

CRAB records incident-management evidence and handling history but does not claim forensic acquisition, malware analysis or certified chain of custody.

### 13.10 First-report intake and phone capture

First report exists before incident declaration.

It may record source/contact, received time/confidence, description, suspected scope/impact and optional evidence.

A human may later:

- associate it with an existing incident;
- use it to propose a new incident;
- retain it as non-incident input;
- reject it as an incident trigger without erasing the source record.

Phone-browser capture supports device/browser file selection and capture for photos, audio, video and supported files without requiring a native application.

No classifier silently turns intake into a live incident.

### 13.11 External participants and temporary access

External counsel, forensic responders, insurers, suppliers, cloud/service providers, communications advisers, regulators and other authorised people use the same identity, incident-scope and compartment model as internal people.

Access may have:

- effective time;
- expiry;
- revocation;
- preserved history/provenance after access ends.

No separate external portal is required unless later evidence proves one necessary.

### 13.12 Shift handover

Handover is primarily a derived authorised operating view containing at least:

- current situation/impact;
- operational roles/deputies;
- critical actions;
- pending decisions/approvals;
- active obligations/deadlines;
- unknowns/questions;
- significant recent changes;
- next objectives;
- communications commitments.

An optional immutable handover report may be recorded.

There is no handover database or state machine.

### 13.13 Surge, resource and logistics requests

These use dedicated Action/Decision/Report semantics with subtype/category, owner, priority, status and authorising decision.

CRAB coordinates need and authority. Fulfilment remains outside CRAB.

### 13.14 Related incidents

A governed relationship may link two incidents without:

- merging histories;
- granting access;
- inheriting compartments;
- revealing restricted details to a user without access to both sides.

### 13.15 Exercises

Exercises use the same screens, controls, checklists, print system and operating model as live incidents.

They are visibly labelled `EXERCISE` on screen/report/print/export.

They:

- do not create real external obligations;
- suppress external notifications before I/O;
- may use simulated clocks/injects;
- cannot be converted into live incidents.

### 13.16 Recovery, stand-down, PIR and closure

CRAB coordinates recovery actions, owners, restoration priorities, verification and residual risks without executing technical remediation.

A confirmed stand-down ends active emergency response but does not close the incident.

PIR is derived as far as possible from the ledger plus human-entered lessons and is then reviewed/confirmed.

Closure is refused until configured closure prerequisites are satisfied, including at least confirmed PIR and disposition of required unresolved long-term work.

Confirmed closure makes the incident read-only and produces controlled immutable closure material, including separate privileged and shareable outputs.

### 13.17 Retention, legal hold and disposition boundary

Closed incidents may carry governed:

- retention policy metadata;
- retention period/date;
- legal/records hold;
- hold reason/authority;
- disposition eligibility;
- disposition/handoff decision.

An active hold blocks eligibility.

V0.4 deliberately does **not** implement physical destructive record deletion.

Ordinary edit/delete can never masquerade as records disposition.

A future physical-destruction design would have to reconcile ledger integrity, immutable blobs, backups, signing material and organisation/legal requirements before implementation.

This is a product boundary, not missing functionality: the PRD requires governed eligibility and an authorised/auditable disposition boundary, not unreviewed physical erasure in the first implementation.

---

## 14. Paper is first-class

Digital and paper operation form one operating model.

### 14.1 Live Incident Brief

The Incident Brief is generated locally, normally targets one to two pages and is regenerated on material change and configured schedule.

It includes at least the authorised current:

- incident/generation identity;
- situation and impact;
- operational roles/deputies;
- key decisions;
- priority actions/owners;
- active obligations/deadlines;
- next communications deadline;
- key contacts;
- time trust;
- source ledger/configuration version.

### 14.2 Emergency Response Pack

The Emergency Response Pack is pre-generated and useful when CRAB, PostgreSQL, normal communications or responder devices are unavailable.

Approved configuration may include:

- incident-response plan/runbooks;
- checklist material;
- role/deputy cards;
- contacts and alternative communications;
- delegated authorities/escalations;
- obligation reference material;
- holding statements/scripts;
- critical reference data;
- blank incident, decision, action, communication and evidence forms.

Standing contingency packs minimise privileged content. Privileged/legal print is separate, conspicuously marked and custodied.

### 14.3 Paper reconciliation

After digital restoration, CRAB supports controlled transcription preserving:

- what happened;
- original occurred/observed time where known;
- paper-recorded time where known;
- digital transcription time;
- transcriber;
- source/paper reference;
- time confidence/trust;
- normal governance for effective state.

CRAB never falsifies history by pretending a later transcription was digitally entered at the earlier event time.

### 14.4 Print quality

Operational PDF output is purpose-designed, not browser print.

It supports:

- professional typography;
- spacing/hierarchy;
- deliberate pagination;
- tables;
- headers/footers;
- source/status/classification markings;
- colour and monochrome;
- A4 and US Letter.

Product quality is not simplified away.

---

## 15. Presentation and UX architecture

The preferred implementation is deliberately small:

- Go `net/http`;
- `html/template`;
- embedded local CSS/SVG/assets;
- small focused JavaScript where interaction genuinely benefits;
- no Node/SPA build chain required;
- local Go PDF renderer after build spike.

This is an implementation simplification, not an interaction-design constraint.

### 15.1 Semantic presentation rule

Generic internal storage is never exposed as generic forms or generic tables when the operational concepts differ.

Each important operation receives purpose-built presentation and language.

Examples:

- `Add Action` rather than `New Item`;
- `Record Fact`;
- `Record Unknown`;
- `Add Evidence`;
- `Update Scope`;
- `Change Severity`;
- `Draft Situation Report`;
- `Request Access`;
- `Read into LEGAL`;
- `Stand Down`;
- `Start PIR`.

### 15.2 Scope clarity

The current incident and relevant compartment context remain persistently clear without exposing unauthorised names/details.

Operational role and application permission are visually distinct concepts.

### 15.3 Accessibility

Core flows must be keyboard-operable.

Status cannot depend on colour alone.

Typography, contrast, touch targets and responsive phone-capture paths are acceptance requirements.

### 15.4 Web data minimisation

The presentation tier receives minimum authorised view models rather than serialising raw domain/database structures.

No third-party analytics/telemetry or remote assets are required for core operation.

---

## 16. Interfaces and one control plane

Supported interfaces:

- HTTPS web UI;
- local versioned JSON API;
- CLI;
- MCP stdio adapter.

All reach the same application command/query services.

MCP maps protocol calls directly into in-process application services; it does not call CRAB over self-HTTP and does not create separate security, approval or business logic.

CLI remains primarily operational/recovery/verification tooling rather than an alternative human incident-management UX.

---

## 17. Two schedulers only

CRAB keeps two schedulers because they solve materially different problems.

### 17.1 Timed wait list

Holds deterministic future wake-ups such as:

- obligation warnings;
- situation-report cadence;
- pack regeneration cadence;
- other explicitly scheduled application events.

It is rebuildable from authoritative state/configuration.

### 17.2 Bounded priority work queue

Executes asynchronous/background work with explicit priority and shedding.

Priority ordering preserves core control work ahead of optional work.

Model work is shed first under pressure.

No third scheduler, retry service, message bus or workflow runtime may be introduced merely for convenience.

Notification failure is recorded; it does not create a hidden guaranteed-delivery queue.

---

## 18. Notification

CRAB owns recipients, authorised content, exercise suppression and provenance.

The notifier owns only deterministic delivery attempts through configured adapters such as local sink or generic HTTP webhook.

Each attempt records:

- channel;
- destination identifier where appropriate;
- attempt time;
- `delivered | failed | suppressed` outcome;
- response/acknowledgement metadata where operationally meaningful.

One channel failure does not silently suppress later configured attempts.

Exercise-mode suppression occurs before external I/O.

---

## 19. Readiness

A single readiness page/command answers: **will CRAB work when it is needed?**

Checks include at least:

- PostgreSQL reachability;
- schema compatibility;
- ledger verification state;
- backup age/last result;
- latest restore/rebuild rehearsal;
- time trust;
- local authentication configuration;
- federation/isolate configuration;
- break-glass configuration;
- certificate/hostname configuration;
- Emergency Response Pack freshness;
- primary/secondary export path status/headroom;
- corpus availability;
- notifier configuration/last explicit test;
- model-lane availability as optional status;
- storage/disk headroom.

Each result is PASS/WARN/FAIL with human-readable reason.

Readiness does not silently send external notifications.

---

## 20. Backup, restore and independent verification

CRAB uses PostgreSQL-native backup tooling rather than inventing a database backup format.

`crab backup`:

- runs readiness/preconditions;
- verifies the ledger;
- invokes PostgreSQL backup tooling without shell interpolation;
- hashes the result;
- records result metadata;
- preserves the last good backup if the new operation fails.

`crab restore`:

- restores the supported backup;
- applies explicit compatible migrations where required;
- verifies ledger history;
- rebuilds projections;
- rebuilds retrieval metadata;
- rebuilds timed-wait-list inputs;
- runs readiness.

Database backup is a bulk exfiltration surface and uses deployment custody controls separate from normal incident/Custodian permissions.

Portable ledger export remains independently verifiable without the database, network or running CRAB process.

Signing private material is never included in a portable public verification export.

---

## 21. CLI contract

One executable provides the operational surface:

```text
crab serve
crab setup
crab mcp
crab readiness [--probe-external]
crab backup --output <path>
crab restore --input <path>
crab rebuild
crab export-ledger --output <file>
crab verify --ledger <file> --keyring <file>
crab print incident-brief --incident <id> [--page-size A4|LETTER]
crab print emergency-pack [--page-size A4|LETTER]
crab version
```

A narrowly scoped Custodian-recovery subcommand may be added under `crab setup`/`crab recover` if needed to implement the locked recovery ceremony.

Additional commands may expose existing application capabilities but may not create a second control path.

---

## 22. Configuration

Non-secret deployment settings are file-first. Secrets use environment/file injection or workload identity rather than normal command arguments.

Representative deployment settings include:

```text
CRAB_CONFIG_FILE
CRAB_DATABASE_URL
CRAB_LISTEN_ADDR
CRAB_PUBLIC_BASE_URL
CRAB_SESSION_KEY_FILE
CRAB_SERVER_SIGNING_KEY_FILE
CRAB_CREDENTIAL_WRAP_KEY_FILE
CRAB_EXPORT_DIR
CRAB_SECONDARY_EXPORT_DIR
CRAB_PAGE_SIZE_DEFAULT
CRAB_EVIDENCE_MAX_BYTES
CRAB_OLLAMA_URL
```

Organisation business configuration such as obligations, contacts, operational roles, calendars, impact tiers, checklists, compartments and holding statements is governed application data, not an administrator-edited configuration-file bypass.

---

## 23. Logging and trace

Operational logs are diagnostic, not the authoritative audit history.

Logs may include:

- request/correlation ID;
- authenticated principal ID where appropriate;
- adapter/command;
- incident opaque ID where appropriate;
- result class;
- committed ledger sequence/hash;
- latency;
- model provider/model/trace metadata without sensitive prompt content by default;
- notifier outcome without secret;
- print artefact ID/hash;
- backup/readiness result.

Logs do not contain secrets and should not routinely copy incident bodies, evidence content or model prompts.

The ledger preserves governed provenance even if operational logs rotate or disappear.

---

## 24. Failure and degraded operation

CRAB explicitly states useful behaviour when dependencies disappear.

| Dependency/failure | Required behaviour |
|---|---|
| internet unavailable | full local incident management, local auth, ledger, clocks, evidence, checklists, print, local retrieval and local notification sink remain |
| corporate IdP unavailable | local authentication/passkey/TOTP and break-glass remain; isolate may disable federation |
| corporate network compromised | rehearsed independent deployment/network pattern is used |
| managed AI unavailable | deterministic retrieval/manual drafting remain |
| local AI unavailable | all deterministic product functions remain |
| all AI unavailable | complete incident-management control path remains |
| external notification unavailable | in-app/local records and visible failures remain |
| trusted time unavailable | operation continues with explicit `time unverified` markings; no silent deadline certainty |
| PostgreSQL unavailable | digital live read/write stops rather than inventing a second authority; last-good paper material remains |
| CRAB host unavailable | pre-generated paper/secondary material plus replacement-host restore procedure |
| PDF renderer unavailable | last-good generated artefacts remain; web operation continues |
| export target full/unavailable | generation fails visibly without destroying last good copy |
| ledger verification failure | integrity-failed read-only mode; mutation refused |
| projection mismatch | rebuild from ledger; never repair by editing history |
| unsupported schema | unsafe mutation refused until explicit migration |
| corrupt evidence blob | refuse to represent bytes as unchanged evidence; show integrity failure and recover source/backup |
| model queue full | model job refused/shed; core remains |
| signing private key missing after restore | history verification possible with public material; governed writes disabled until key custody recovery |

Failure is visible. CRAB does not hide failure behind endless retry machinery.

---

## 25. Resource and performance constraints

The Raspberry Pi 5 8 GB ARM64 platform remains a minimum-hardware acceptance rig, not the production target.

The final build retains explicit resource budgets from V0.3 as test targets until measured and refined through implementation evidence.

Guiding limits remain:

- CRAB process designed for low hundreds of MiB under normal operation;
- bounded queue sizes;
- bounded upload memory through streaming/chunking;
- no model memory counted as required core capability;
- ordinary local pages should feel immediate under normal load;
- core work continues when optional model work is shed.

Exact measured acceptance values and exceptions belong in the build/test evidence rather than being invented in the EDD before code exists.

---

## 26. Direct dependencies

Target direct non-standard dependencies remain deliberately small:

1. PostgreSQL driver/pool, expected `github.com/jackc/pgx/v5`, pinned;
2. local PDF renderer, proposed `github.com/signintech/gopdf`, pinned only after the ARM64/offline/typography spike passes.

Authentication/model component lineage may bring code patterns or necessary cryptographic/provider support into the build, but any additional direct runtime dependency requires a short explicit design note showing why standard library or existing selected dependency is insufficient.

The target is not dependency zero. The target is dependencies that earn their operational cost.

---

## 27. Component lineage

Existing Garage components remain reference implementations and reusable engineering lineage, not deployment boundaries:

- `three-lane-model-harness`;
- `enterprise-ai-workbench`;
- `data-dictionary-first`;
- `governed-retrieval`;
- `evaluation-service`;
- `governed-mcp`;
- `decision-ledger`;
- `authentication`;
- `authorisation-dual-control`;
- `out-of-band-verification`;
- `obligation-clock`;
- `notifier`;
- `immutable-blob-store`.

CRAB may refactor code internally to fit its one-application / PostgreSQL architecture.

No component becomes a separately operated service merely because it has its own Garage repository.

No new micro-component repository is justified by the final CRAB design.

A later generic version of `authorisation-dual-control` may absorb proven scoped grant expiry/revocation primitives, but CRAB implementation does not wait for that extraction.

---

## 28. Build-time proof gates that do not reopen architecture

The architecture is final. A small number of implementation choices still require proof because empirical build evidence is better than speculation.

### 28.1 RLS gate

Prove whether simple incident/compartment RLS provides cheap independent defence without pool/recovery ambiguity. Keep or drop according to the locked criteria in section 9.11.

### 28.2 PDF renderer gate

Prove the selected Go PDF library on `amd64` and `arm64` for:

- offline build/runtime;
- typography;
- tables;
- pagination;
- A4/Letter;
- colour/monochrome;
- required resource budget.

If it fails, replace the renderer library. Do not weaken the print contract.

### 28.3 Safe-inline media gate

Select the smallest safe format allow-list proven by browser/security fixtures. Anything uncertain remains download-only.

### 28.4 Blob chunk sizing

Measure PostgreSQL chunk size/transaction behaviour under realistic evidence files and Pi fixture limits. Change the chunk constant, not the architecture.

### 28.5 Package splitting

Begin with the small tier-oriented package structure. Split within a tier only when a dependency/auditability/code-size reason appears.

These are implementation proofs, not invitations to add systems.

---

## 29. Acceptance and test architecture

PRD V0.2 contains 88 acceptance criteria. All remain binding.

The V0.3 design established 150 named pre-code tests. Those tests remain the starting regression set. Mechanisms may merge, but the behaviour/attack/failure coverage may not disappear merely because a former mechanism has been removed.

The parallel build test notes retain the detailed catalogue and add tests for the final simplifications, including:

- `Record` cannot mutate governed effective state;
- issued communication cannot bypass maker-checker;
- GENERAL is available only after governed incident membership;
- restricted compartments always require explicit read-in;
- identifier substitution cannot bypass authoritative object binding;
- failed/oversized blob ingest leaves no usable partial authoritative content;
- derived operational phase remains correct without a second state machine;
- generic backend machinery does not collapse the required semantic UX;
- safe-inline media policy fails closed;
- RLS is either demonstrably safe/useful or demonstrably removed with replacement negative coverage.

### 29.1 Security regression rule

> **No simplification is accepted if it deletes a negative security test. Mechanisms may merge; the attack test survives.**

### 29.2 Functional regression rule

> **No simplification is accepted merely because the capability can be represented in generic storage. The operational flow, language, legibility and acceptance outcome must remain at least as good.**

### 29.3 Bafflewick acceptance

The end-to-end acceptance scenario must exercise at least:

- first-report intake;
- phone evidence capture;
- incident declaration;
- evolving scope/unknowns/hypotheses/severity;
- Major/Minor/Observer scoped access;
- counsel-only restricted compartment;
- external forensic responder with expiry/revocation;
- actions/checklists;
- obligations/clocks;
- supplier outage/resource request;
- insurer/other external communication;
- structured issued communications and failure outcomes;
- shift change/handover;
- safe rich evidence handling;
- model loss;
- internet/corporate identity loss;
- database-down paper operation;
- replacement-host restore/rebuild;
- paper reconciliation;
- recovery/stand-down;
- PIR;
- retention/hold metadata;
- closure;
- standalone ledger verification.

If this scenario needs another runtime subsystem, the first response is to question the subsystem rather than assume the requirement is wrong.

---

## 30. Product-quality acceptance

CRAB is not complete merely because the domain logic passes tests.

The build must retain acceptance evidence for:

- incident board/common operating picture;
- approval view;
- access administration;
- readiness view;
- phone capture;
- evidence register/media presentation;
- handover;
- recovery/PIR/closure;
- Incident Brief colour and monochrome;
- Emergency Response Pack;
- evidence register print;
- timelines/PIR/closure material;
- blank contingency forms;
- A4 and Letter fixtures;
- keyboard-only core workflows;
- status comprehension without colour;
- current incident/compartment clarity.

Visual defects are defects, not post-MVP polish.

---

## 31. Locked removals

The following are deliberately absent unless a future product requirement proves otherwise:

- universal Major visibility;
- omniscient application administrator;
- break-glass authorisation elevation;
- emergency read-in;
- arbitrary per-object ACL estate;
- permission filtering performed only by presentation;
- SQL access outside the data tier;
- additional authoritative stores;
- vector/embedding stack by default;
- workflow/orchestration engine;
- general chat subsystem;
- automatic model-provider failover;
- service mesh/microservice deployment;
- Redis/message bus;
- Node/SPA build chain as a dependency for the product;
- self-HTTP inside the monolith;
- hidden guaranteed-delivery/retry queue;
- user scoring/behavioural monitoring;
- automatic evidence parsing/analysis/transcoding/OCR/speech-to-text;
- native mobile application for V0.4;
- physical records destruction;
- application-layer payload encryption against root/DBA in the base design.

Each removal has a clear mission boundary or an existing simpler mechanism that already provides the required capability.

---

## 32. Final architecture decision

CRAB V0.4 is locked to the following minimum machine:

```text
ONE GO APPLICATION
        |
        +-- presentation tier
        |
        +-- one application/domain command-query boundary
        |      |
        |      +-- one authorisation evaluator
        |      +-- Record(...)
        |      +-- Propose(...) -> Decide(...)
        |      +-- semantic domain contracts
        |      +-- deterministic projections
        |      +-- deterministic obligation calculator
        |      +-- deterministic retrieval
        |      +-- optional model drafting
        |      +-- notifier edge
        |      +-- bounded priority queue
        |      +-- timed wait list
        |
        +-- PostgreSQL data tier
               |
               +-- signed ledger
               +-- immutable blobs
               +-- security-support state
               +-- rebuildable projections

DERIVED / CUSTODIED
  professional PDFs
  paper packs
  backups
  portable ledger exports
```

The design is intentionally asymmetric:

- **small implementation surface**;
- **rich operational semantics**;
- **strong security boundaries**;
- **high product-quality bar**.

The architecture has now survived three simplification passes, a security-regression pass and a functional/product-regression pass.

Further simplification during build is encouraged only where it preserves the locked invariants, security floor, 88 PRD acceptance criteria, negative-security coverage and product-quality requirements in this document.

**Status: READY TO BUILD.**
