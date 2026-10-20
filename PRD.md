# Cyber Response Action Board (CRAB) — Product Requirements Document

**Version:** 0.2  
**Status:** DESIGN — REOPENED FOR REQUIREMENTS COMPLETION  
**Reopened:** 20 October 2026  
**Supersedes:** `CRAB-PRD-v0.1.md`

> V0.1 was deliberately baselined before engineering iteration. Subsequent red-team work identified genuine product requirements that had been under-specified rather than implementation details: scoped incident access, access administration without universal visibility, rich/hostile evidence media, phone-first capture, first-report intake, evolving scope/impact, external participation, long-running incident continuity, controlled disclosure, retention and related incident handling. V0.2 reopens the product contract to capture those requirements before further Garage simplification. The EDD may simplify mechanisms but may not remove these outcomes.

## Mission

> **Help a team run a cyber incident from first report to closure, keeping decisions, deadlines, evidence and communications intact, even when the network, corporate identity or normal digital working environment is unavailable.**

## Problem

A serious cyber incident is an operating crisis as well as a technical event. The organisation may have lost access to its normal identity provider, collaboration tools, shared drives, corporate network or even the systems on which its incident response plan is stored.

The people responding still have to answer, under pressure:

- What do we know, and how certain are we?
- Who is currently in charge of each part of the response?
- What do we do next?
- Who owns each action, and when is it due?
- Which legal, regulatory, contractual or internal clocks may have started?
- What did we decide, who confirmed it, and what evidence supported it?
- Who has been told what, and when?
- Which information is privileged or otherwise restricted?
- What happens if CRAB itself, its database or its host becomes unavailable?
- How do we reconstruct the response afterwards without relying on memory, chat history or email?

Incident obligations differ by organisation, jurisdiction, sector and circumstance. They may run from awareness, determination, classification, payment, assessment or another user-defined event. CRAB therefore cannot encode a universal legal answer. It must let an organisation define its own obligations and decision authorities while keeping the execution of those rules visible and auditable.

The system must remain useful to non-specialists working under stress. It must not depend on a large integration estate, a specialist automation team, a managed artificial intelligence service or the continued availability of the systems affected by the incident.

## Product objective

Provide a single Go application that runs a cyber incident out-of-band from the organisation's normal working environment. CRAB receives and preserves first reports, provides a common operating picture, tracks evolving scope and impact, guides responders from the organisation's own material, records facts, unknowns, decisions and rich evidence, assigns and tracks actions, tracks user-defined obligations, coordinates internal and external participants, supports structured communications and handover, separates restricted work, preserves a signed tamper-evident decision history, produces professional paper contingency material, and carries the incident through stand-down, post-incident review, retention hand-off and administrative closure.

The same application runs exercises so that responders rehearse on the system and paper material they will use for a real incident.

> **The model proposes. One person makes. Another confirms. Nothing is erased.**

CRAB uses artificial intelligence only where it reduces operator workload without becoming an authority. Deterministic rules, explicit state and human control remain on the critical path.

## Product principles

1. **Incident response, not problem management.** CRAB manages the active response through stand-down, review and closure. Long-term root-cause remediation and post-closure programmes belong elsewhere.
2. **Requirements are not removed to simplify engineering.** CRAB delivers the complete incident-management requirement with the smallest defensible set of mechanisms.
3. **Humans remain accountable.** Models and agents may retrieve, analyse and propose. They do not independently decide, approve or execute changes.
4. **Global by configuration.** Jurisdictions, regulators, operational role titles, deadlines, calendars, contacts and escalation rules are data, not hard-coded product logic.
5. **Digital and paper are one operating model.** CRAB assumes that digital operation may be interrupted and supports controlled paper contingency and later reconciliation.
6. **Graceful degradation is designed.** Loss of internet, corporate identity, a model lane or another optional dependency reduces capability visibly rather than silently breaking the core response.
7. **Product quality matters.** The web user interface and all printed output must be visually polished, coherent and immediately readable under pressure. “Engineering prototype” quality is not acceptable.
8. **Authority does not imply knowledge.** Role seniority never creates universal visibility. Access is constrained by incident assignment, compartment and permitted action.
9. **Evidence is preserved before it is interpreted.** CRAB may store potentially hostile files and rich media as evidence without executing, analysing or silently transforming them.
10. **Access is temporary where the mission is temporary.** External and exceptional access can expire or be revoked without erasing the access history.
11. **Long incidents must survive people changing.** Shift handover, deputies, current unknowns, pending decisions and next deadlines are first-class continuity requirements.

## Users and authority

CRAB has three incident-participation roles. Role determines **what** a person may do; incident assignment determines **where** they may do it; compartment membership determines **what information** they may know. No role creates universal incident visibility.

| Application role | Typical users | V0.2 authority |
|---|---|---|
| **Major** | Incident leads, decision-makers and authorised controllers | Within assigned incidents and compartments: declare where permitted; propose governed changes; confirm or reject another person's proposal; amend by proposing a new version; use approved single-person exceptions; perform incident governance; close incidents where authorised |
| **Minor** | Assigned responders, external specialists, advisers and staff | Within assigned incidents and compartments: read authorised material; comment; report progress; supply supporting material and evidence; perform assigned actions |
| **Observer** | Assigned stakeholders | Read authorised incident material only; no mutation unless separately granted a different role |

CRAB also separates incident-content authority from system administration:

- a **Custodian** administers identities, role eligibility and access metadata without automatically receiving incident-content access;
- an **Incident Sponsor** is a scoped designation that proposes who needs access to one incident;
- a **Compartment Controller** is a scoped designation that proposes read-in/read-out for one restricted compartment;
- Custodian, Sponsor and Controller are authorities/designations, not additional universal content roles.

Every access decision is deny-by-default and evaluates authenticated identity, role, incident assignment, compartment and action/object permission. Search, lists and dashboards must not disclose incidents or compartments outside that scope.

Break-glass restores authentication only; it does not widen incident or compartment authorisation.

An agent never acts as an independent principal. It acts for one named human, is structurally distinguishable from that human, and cannot exceed that human's assignment, role or compartment access.

### Operational incident roles

Organisations define their own operational incident roles, role cards and deputies. Examples might include incident commander, technical lead, communications lead, legal liaison or business recovery lead, but CRAB does not hard-code any title.

Operational role assignment:

- is independent of Major, Minor and Observer permissions;
- records the person, role, deputy if applicable and effective time;
- is visible in the common operating picture and paper material;
- changes through the same governed change process as other incident state;
- preserves the complete handover history.

## Operating environment

- **Production hosts:** any independent Linux host selected by the organisation, including an independent cloud server, mini PC, laptop or server on a network chosen for the response.
- **Independence:** CRAB must not depend on the organisation's corporate network, corporate identity provider or collaboration platform. Independence is achieved through documented deployment patterns and identity separation; CRAB does not require a particular physical form factor.
- **Primary interface:** web browser over Hypertext Transfer Protocol Secure (HTTPS), usable from normal desktop/laptop browsers and modern phone browsers for field capture and urgent operation.
- **Offline local operation:** core incident functions must work with no internet connection. A local deployment may use a local certificate authority established and rehearsed before an incident. Publicly addressed hosts use normal public certificates.
- **Identity:** federation through OpenID Connect (OIDC) is optional. Local accounts, passkeys, time-based one-time passwords (TOTP) and break-glass remain available without the corporate identity provider. Isolate mode disables federation completely.
- **Time:** network time is used when trustworthy. Every ledger entry records time status as trusted, drifting or untrusted. Manual time correction is exceptional and governed.
- **Data sensitivity:** incident facts, personal information, runbooks, communications and evidence are sensitive by default. Managed artificial intelligence lanes may receive only content explicitly permitted by organisation policy. Uploaded evidence may be actively malicious and must be preserved as untrusted bytes rather than executed content.
- **Managed AI lanes:** optional Amazon Bedrock, Microsoft Foundry / Azure OpenAI and Google Vertex AI lanes through the model harness. A local lane may use Ollama.
- **Geography:** global. No jurisdiction, regulator, legal threshold, deadline, holiday calendar or operational role title is hard-coded.
- **Operators:** non-specialists may use CRAB during high stress. Core operation must not depend on heroic memory, specialist platform knowledge or hidden recovery procedures.
- **Minimum-hardware test:** Raspberry Pi 5 with 8 GB RAM, `linux/arm64`, is a deliberate engineering acceptance rig, not the production target. V0.2 attempts and records the complete minimum-hardware test against EDD budgets.
- **Paper contingency:** approved emergency material must remain available when normal digital operation is unavailable.

## Durable state and authority

PostgreSQL is CRAB's single durable live store.

Within PostgreSQL:

- the append-only ledger is authoritative for governed business and configuration state;
- operational tables and views are rebuildable projections of that ledger;
- immutable uploaded content such as runbooks, attachments and evidence files is stored in the same durable store and referenced by governed records;
- authentication credential material and other security-supporting records are kept in the same durable store but are not reconstructed from the public ledger;
- portable ledger exports are for independent verification, audit and recovery evidence and are not a second live state store.

CRAB itself must not introduce another authoritative database, object store, vector database, queue or workflow store.

Derived paper and export files may exist outside PostgreSQL by design. They are clearly marked as derived copies, carry source/version information, and never become the authoritative live state.

## Component lineage

V0.2 uses the following existing Jonno's Garage components as reference implementations and code lineage. They are refactored into the CRAB monolith; they are not deployed as twelve services.

- `three-lane-model-harness` V0.2.0;
- `enterprise-ai-workbench` V0.1.0;
- `data-dictionary-first` V0.2.0;
- `governed-retrieval` V0.1.0;
- `evaluation-service` V0.1.0;
- `governed-mcp` V0.1.0;
- `decision-ledger` V0.1.0;
- `authentication` V0.1.0;
- `authorisation-dual-control` V0.1.0;
- `out-of-band-verification` V0.1.0;
- `obligation-clock` V0.1.0;
- `notifier` V0.1.0.

The existing twelve component lineages remain the starting point. V0.2 does not require a new component merely because it adds product requirements. The EDD must apply the normal removal/extraction test. One candidate is safe immutable binary-content storage for arbitrary potentially hostile evidence; it becomes a new micro-component only if the capability proves independently reusable and cleaner than keeping the small mechanism inside CRAB.

## Control model

### Governed changes

A governed state change follows one maker-checker path regardless of record kind.

1. A person proposes a new record or a new version of an existing record.
2. A different authorised person confirms or rejects that version.
3. An amendment is another proposed version; history is never overwritten.
4. A confirmed version becomes effective according to its effective time.
5. The ledger records the proposal, decision, signatures, provenance and time trust.

Comments, situation messages and agent findings are reports rather than governed state changes. They are confirmed on write, remain immutable, and may be plotted by a Major user into a governed proposed fact or decision.

### Provisional obligation clocks

If a proposed fact matches the trigger for a configured obligation, CRAB starts a clearly labelled provisional clock from the applicable proposed trigger time. Confirmation makes the clock firm. Rejection removes its effect while preserving the history.

### Single-person windows

An organisation may configure specific single-person windows. Inside an active window, a Major may self-confirm an urgent proposal only where durable policy permits it and a reason is supplied.

A self-confirmation:

- is visibly flagged;
- carries the same person's signature in maker and confirmer positions;
- appears on the single-approval dashboard;
- is never silently treated as ordinary dual control.

Outside an approved single-person window, the proposal waits for a different authorised person.

### No deletion during an incident

Nothing governed is deleted or edited in place during a live incident. Corrections are new versions. When an incident is administratively closed, its governed records and reports become read-only.

### Setup approval

Initial setup is prepared by one Major and confirmed by another. Until confirmed, the system is visibly in `setup pending approval`. Subsequent setup changes use the same process.

### Human in the middle

Models and agents may retrieve material, draft content and propose actions. A named human remains the maker. A different human confirms except for an explicitly permitted single-person exception.

### Signatures and provenance

Every confirmed governed change records:

- maker identity and signature where available;
- confirmer identity and signature where available;
- server signature and ledger-chain binding;
- named human principal for agent-originated work;
- explicit reason where a human-device signature is unavailable;
- time status;
- incident mode and privilege compartment.

### Legal privilege and compartments

Every record, report, draft, attachment and evidence item carries a privilege/compartment status. A privileged item records the stated legal direction and purpose supplied by the organisation.

CRAB:

- enforces access to the configured compartment;
- filters privileged material before retrieval or model prompting;
- requires maker-checker to change an item's privilege status;
- produces separate privileged and shareable closure material;
- does not decide whether legal privilege actually applies.

### Awareness and time

A fact may record:

- when the event occurred, if known;
- when it was first flagged;
- when it was recorded;
- when specified people or operational roles became aware;
- the trust status of the recorded time.

Obligation clocks use the trigger field named by the organisation's obligation definition. CRAB does not replace contested or uncertain times with a single false certainty.

### Exercises

An exercise is segregated from a live incident. It uses the same screens, controls, checklists, paper material and operating patterns, but:

- starts no real external obligation;
- sends no real external notification;
- is visibly labelled on every screen, report, printout and export;
- cannot be converted into a live incident.

### Isolate mode

A confirmed isolate-mode change disables federated sign-in. Local authentication, passkeys, TOTP and break-glass remain.

### Break-glass

Every authorised break-glass keyholder has a personal static offline key. Exactly two distinct keyholders are required.

Break-glass:

- uses keys entered in separate authenticated or recovery sessions within a bounded time;
- is time-limited;
- records the two keyholders and reason;
- alerts Major users through available configured channels;
- rate-limits failures;
- allows individual key revocation;
- stores only slow salted verification material;
- does not allow one person with one key to bypass the two-person rule.

Responder network cards are separate from break-glass credentials. They may contain the CRAB address and, for local offline deployment, certificate-authority fingerprint.

### Out-of-band verification

Where an instruction or claimed identity needs independent verification, CRAB uses only verification material registered before that instruction arrived. Caller-supplied contact details cannot become the verification route.

### Trusted validators

CRAB trusts authorised validators and holds them accountable through their signatures. It does not score, rank, count for performance purposes or behaviourally monitor validators.

## Product capabilities

### 1. Incident declaration and common operating picture

A Major can declare a live incident or exercise.

Severity, incident type, impact and other classification values are selected by humans from organisation-defined values. CRAB does not automatically classify the incident.

Every incident screen presents a clear common operating picture including:

- incident identity and mode;
- current situation;
- current operational role holders and deputies;
- impact and affected assets;
- open actions and owners;
- active obligation clocks;
- pending approvals;
- recent confirmed decisions;
- communications cadence;
- time status;
- isolate and break-glass status;
- model-lane availability;
- paper-pack freshness;
- readiness warnings relevant to the incident.

### 2. Contacts, assets and operational roles

CRAB stores organisation-defined:

- contacts and alternative contact routes;
- responders and external advisers;
- operational incident roles and deputies;
- assets and user-defined business impact tiers;
- insurers, regulators and other recipients;
- escalation contacts and delegated authorities.

Changes are governed. No role title, regulator or priority scheme is hard-coded.

### 3. Actions and response checklists

Actions record at minimum:

- owner;
- status;
- due time where applicable;
- source decision, checklist or runbook reference;
- notes and supporting evidence references.

Overdue status is derived and visible.

Organisations may configure simple response checklist templates from their own plans and runbooks. A checklist step may be `not started`, `complete` or `skipped with reason` and may carry an owner, due time, source reference and evidence.

CRAB deliberately does not provide a graphical workflow designer, branching automation language or general orchestration engine. A model may suggest a checklist or actions; humans activate and govern them.

### 4. Facts, timeline and awareness

CRAB maintains one incident timeline derived from the ledger and relevant event times.

It includes facts, reports, decisions, actions, obligations, evidence records, operational-role changes, communications, notification outcomes, recovery activity and closure events.

The timeline distinguishes occurrence, flagging, recording and awareness rather than flattening them into one timestamp.

### 5. User-defined obligations and clocks

An obligation is governed data containing at least:

- trigger definition;
- duration;
- unit: elapsed hours, calendar days or business days;
- timezone;
- user-supplied business calendar where required;
- deadline adjustment rule;
- applicability condition;
- recipient;
- owner or responsible role.

CRAB supports provisional and firm clocks, displays upcoming and overdue obligations, and provides reminders through configured notification channels. A confirmed obligation may later be amended, extended, waived or replaced only through a governed record that preserves the original rule, original deadline, authority/source, reason and resulting deadline/status.

A guided suggested-input workflow may ask for organisation context and propose obligation records with sources and as-at dates. Suggestions never become active without maker-checker.

CRAB does not determine legal applicability or reportability.

### 6. Runbook guidance and retrieval

V0.2 ingests Markdown and Microsoft Word DOCX runbook/retrieval material.

Retrieval:

- filters by incident assignment and compartment before ranking;
- works deterministically without a model;
- returns stable source citations;
- refuses when the authorised corpus does not support the question.

A local or managed model may turn authorised retrieved evidence into a proposed explanation or draft. No model is required for deterministic retrieval.

### 7. Drafting and structured communications

CRAB can help draft, for human review:

- regulator notifications;
- notices to individuals;
- executive or board updates;
- staff communications;
- situation reports;
- holding statements;
- closure and post-incident material.

Situation messaging inside CRAB is structured incident communication, not general-purpose chat.

Organisations configure a situation-report cadence. CRAB reminds the team and may draft the report from its own data. Issuing a governed communication requires the normal control path.

Notification attempts record `delivered`, `failed` or `suppressed`; CRAB never treats an attempted call as confirmed delivery.

### 8. Evidence register and attachments

CRAB provides an evidence register for incident-management purposes.

An evidence record can capture:

- evidence identifier;
- description;
- collector or source;
- collection, occurrence and recording times where known;
- timezone and time-trust status;
- source or storage location;
- cryptographic hash where available;
- current handler or custodian;
- handling/history events;
- related fact, decision or action;
- privilege compartment.

Files and screenshots may be stored as immutable attachments and referenced from governed records. Their hashes are included in integrity checks.

CRAB records evidence and handling metadata. It does not acquire forensic images, analyse malware, certify forensic chain of custody or replace a specialist evidence-management platform.

### 9. Exercises

CRAB runs tabletop and operational exercises in the same application as live response. Exercises may include scheduled injects, response checklists, simulated clocks, communications and paper fallback drills.

Exercise operation is visibly segregated and cannot produce real external obligations or notifications.

### 10. Paper contingency and print

Paper is a first-class contingency mode, not an export afterthought.

CRAB locally generates print-ready Portable Document Format (PDF) artefacts without internet access, a cloud service or a model.

#### Live Incident Brief

The Incident Brief targets one to two pages and is regenerated on material change and on a configured schedule. It presents, at minimum:

- incident and generation identity;
- current situation and impact;
- operational role holders and deputies;
- critical current decisions;
- open priority actions and owners;
- active obligations and deadlines;
- next communications deadline;
- key current contacts;
- time-trust status;
- source ledger position/configuration version.

#### Emergency Response Pack

The Emergency Response Pack is a broader pre-generated contingency pack intended to remain useful if CRAB, PostgreSQL, normal communications or responder devices are unavailable.

It may include, according to approved organisation configuration:

- incident-response plan and selected runbook/checklist material;
- operational role cards and deputy arrangements;
- primary and alternative contact details;
- alternative communications arrangements;
- escalation and delegated-authority information;
- configured obligation reference material;
- approved holding statements and scripts;
- critical asset/contact reference material;
- blank incident log;
- blank decision record;
- blank action register;
- blank communications record;
- blank evidence-handling register.

The pack is regenerated after approved setup changes and on a configured schedule and can be written atomically to the normal local export location and an optional independently stored/removable location.

#### Print quality

Every operational PDF is deliberately designed for paper. It must have professional typography, spacing, visual hierarchy, page breaks, tables, headers, footers and status markings. It must remain clear in colour and monochrome and support both A4 and US Letter page sizes.

The visual standard is the same as for the web product: polished, calm and credible, not a database dump or raw browser printout.

### 11. Paper reconciliation

If responders operate from paper while the digital system is unavailable, CRAB supports controlled reconciliation after restoration.

A reconciled historical entry preserves:

- what happened;
- the original occurred/observed time where known;
- when it was written or recorded on paper where known;
- when it was transcribed into CRAB;
- the transcribing person;
- paper/source reference;
- time-confidence or time-trust information;
- normal maker-checker confirmation for governed state.

CRAB never rewrites a reconciled historical item to make it appear as though it was entered digitally at the original time.

### 12. Recovery, stand-down, post-incident review and closure

CRAB records and coordinates response recovery work without executing changes on affected systems.

It supports:

- containment/recovery actions and owners;
- restoration priorities;
- verification that agreed recovery steps were completed;
- residual risks and exceptions;
- a formal decision to stand down the active emergency response;
- transfer of unresolved long-term work to another system or programme;
- a Post-Incident Review (PIR) based on the ledger;
- administrative closure after the PIR is confirmed.

The PIR derives as much as possible from CRAB's existing record, including:

- executive summary;
- impact;
- key timeline;
- decisions;
- actions;
- communications;
- obligations;
- evidence register;
- what worked;
- what did not work;
- missing information;
- lessons;
- follow-up work and its destination.

Humans review and confirm the PIR.

Closure produces immutable privileged and shareable outputs. The shareable version must contain no privileged material. Confirmed closure makes the incident read-only.

### 13. Readiness

CRAB provides one readiness page and command answering the practical question: **will CRAB work when it is needed?**

The check covers at least:

- PostgreSQL reachability;
- ledger verification state;
- backup age and last backup result;
- latest recorded restore/rebuild test;
- time status;
- local authentication configuration;
- federation and isolate-mode configuration;
- break-glass configuration;
- current Emergency Response Pack generation and freshness;
- normal and optional secondary export destinations;
- local corpus availability;
- configured notification channels and last test status;
- local and managed model-lane availability;
- storage/disk headroom;
- certificate and public/local hostname configuration.

Readiness does not become an enterprise monitoring product. External test notifications are explicit operator actions, not silent health-check side effects.

### 14. Backup, restore and independent verification

CRAB provides documented one-command operations to:

- create a complete backup of durable CRAB state;
- restore onto a replacement host;
- verify the ledger;
- rebuild all projections from the authoritative ledger;
- export a portable independently verifiable ledger.

The rebuild procedure is rehearsed in exercises.

The portable verifier operates without the running CRAB application, database or network and checks at least:

- chain integrity;
- server signatures;
- available human signatures;
- maker/confirmer rules;
- explicit self-approval flags/reasons;
- live/exercise consistency.

Custody of hosts, backups, signing keys, break-glass keys and emergency packs is recorded under governance appropriate to the item.

### 15. Interfaces

The web user interface is primary.

V0.2 also exposes:

- a local command-line interface (CLI) for operation, recovery and verification;
- a local application programming interface (API);
- Model Context Protocol (MCP) tools for authorised clients and agents.

All interfaces use the same application authorisation, proposal, confirmation and ledger boundaries. An alternate interface must not create a second control plane.

### 16. Visual product quality

The web interface must look and behave like a finished production enterprise application.

It must be:

- visually coherent across incident, setup, readiness and review screens;
- calm and legible under stress;
- information-dense without appearing cluttered;
- consistent in typography, spacing, iconography, controls and state treatment;
- explicit about critical warnings without making every screen look alarming;
- usable without externally hosted web assets;
- responsive across normal laptop/desktop browser sizes;
- clear enough that a non-technical executive can identify situation, owners, clocks and pending decisions without understanding CRAB's internal architecture.

Visual polish is part of acceptance, not deferred enhancement work.

### 17. First-report intake and phone-first capture

CRAB must support the period before a full incident declaration, because the first useful information may arrive from a staff member, supplier, customer, helpdesk, external responder or person physically present at an affected location.

A first report can record at least:

- reporter and contact route where known;
- received time and time confidence;
- free-text description;
- affected or suspected service/asset where known;
- immediate safety/business impact where known;
- attached evidence/media;
- source/channel;
- triage status and the human decision to declare, associate, reject or retain the report.

A first report does not automatically become a live incident and does not let CRAB classify severity or reportability.

The web interface must support practical phone-browser capture, including taking or selecting a photograph, audio recording, video or file where the device/browser permits it. Phone capture is an input convenience, not a separate mobile application requirement.

### 18. Evolving scope, impact, unknowns and severity

CRAB must represent the response team's changing understanding rather than only its final conclusion.

The common operating picture can record and preserve history for:

- affected or suspected systems, services and assets;
- affected data categories/populations where known;
- affected people/customers/business areas where relevant;
- physical or cloud locations where relevant;
- affected or implicated suppliers/third parties;
- known facts;
- explicit unknowns/questions requiring investigation;
- hypotheses/assumptions clearly labelled as such;
- impact assessment and confidence;
- human-selected severity and changes to that severity;
- reason/evidence for escalation or de-escalation.

CRAB does not automatically infer that a hypothesis is fact or autonomously change incident severity.

### 19. External participants, temporary access and controlled sharing

A serious incident may include external counsel, forensic responders, insurers, suppliers, cloud/service providers, communications advisers, law enforcement, regulators or other authorised parties.

CRAB must support them without creating a separate uncontrolled collaboration system.

Requirements:

- external people use the same identity, incident-scope and compartment controls as internal people;
- incident access can be sponsored, confirmed, time-bounded, revoked and historically audited;
- compartment read-in/read-out is independent of basic incident membership;
- access expiry/revocation prevents future access without erasing prior actions/provenance;
- ordinary read permission does not automatically imply bulk export, evidence download or unrestricted printing;
- individual reports, evidence and communications may be restricted to a smaller compartment than the incident as a whole;
- search, lists, counts and dashboards must not leak the existence of unauthorised incidents or restricted items where that existence is itself sensitive.

CRAB does not require a separate external-user portal in V0.2; a separate portal is an implementation choice only if justified later.

### 20. Decision authority, continuity, surge and logistics

Application permissions do not by themselves define organisational authority to take consequential response decisions.

CRAB can record organisation-defined operational decision authorities and deputies for matters such as service shutdown, emergency expenditure, external notification, public communication or other organisation-specific decisions. CRAB records the authority; it does not invent or legally validate it.

Long-running responses must survive shift and personnel changes. CRAB therefore supports a governed handover view/record containing at least:

- current situation and impact;
- current operational role holders/deputies;
- open critical actions;
- pending decisions/approvals;
- active obligations and next deadlines;
- current unknowns/questions;
- significant recent changes;
- next objectives and communications commitments.

The incident may also record requests for surge resources, specialist assistance, equipment, funding or logistics, their owner/priority/status and the decision that authorised them. CRAB coordinates these requests; it does not become a procurement, HR or facilities platform.

### 21. Stakeholder enquiries and communication provenance

CRAB must be able to preserve the operational history of important incoming and outgoing communications without becoming general-purpose chat.

For issued communications, CRAB can record:

- audience/recipient;
- approved content/version;
- approver/authority;
- channel;
- issue/attempt time;
- delivery outcome;
- acknowledgement/response where operationally meaningful;
- related incident decision or obligation.

Incoming stakeholder enquiries or demand from staff, customers, media, suppliers or other parties can be recorded, grouped and assigned for response where they materially affect incident operations.

### 22. Rich evidence media and hostile-content safety

Evidence/attachments may include documents, photographs, screenshots, audio, video, email files, packet captures, archives, executables or other organisation-permitted binary content.

CRAB must:

- preserve the original bytes without silent transformation;
- record original filename, declared/detected media type where available, byte size and cryptographic hash;
- enforce deployment-configurable upload-size and storage limits before durable write;
- associate content with evidence/source/collector and incident/compartment metadata;
- re-authorise every view/download against current incident, compartment and action permission;
- treat all uploaded content as untrusted;
- never execute uploaded content;
- avoid unsafe browser rendering of active content;
- allow deliberately safe presentation of supported media only where the implementation can do so without weakening the evidence boundary;
- retain download/export provenance where required by access policy.

CRAB does not perform malware analysis, media transcoding, image enhancement, speech-to-text, facial recognition or forensic authenticity certification in V0.2.

### 23. Related incidents

CRAB can create governed relationships between incidents where responders determine that separate incidents are related.

A relationship must not:

- merge or rewrite either incident's history;
- grant access to the related incident;
- reveal restricted details to a user without access to both sides.

The purpose is to preserve operational context without introducing a parent ticketing hierarchy or general case-management graph.

### 24. Retention, legal hold and governed disposition

“Nothing is erased” applies to the active governed incident record: corrections are new versions and live history is not destructively edited.

CRAB must nevertheless support organisation-defined post-closure records obligations. It therefore supports governed retention metadata and legal/records holds so that the organisation can distinguish:

- records still required for operational, legal, regulatory, insurance or organisational reasons;
- records under explicit hold;
- records eligible for later disposition under approved policy.

Any eventual disposition is outside the live-incident mutation path, must be authorised and auditable, and must not silently rewrite the historical record. The exact destruction mechanism is an EDD design issue and must not be implemented until the retention/hold semantics have been red-teamed.

### 25. Accessibility and pressure usability

Visual quality includes accessibility and operation under stress.

The primary interface must support:

- keyboard operation for core workflows;
- status meaning that does not depend on colour alone;
- readable contrast and typography;
- practical touch targets and phone-browser capture paths;
- clear indication of current incident and compartment scope;
- explicit distinction between operational role and application permission;
- predictable behaviour when access is denied or information is intentionally hidden.

Accessibility and mobile capture are product-quality requirements, not separate frontend products.

## Out of scope for V0.2

CRAB does not:

- detect incidents, scan networks, ingest a Security Information and Event Management (SIEM) feed or perform security monitoring;
- perform containment, isolation, password reset, endpoint action or any other change on affected systems;
- perform root-cause problem management or manage long-term remediation after the incident has been handed off and closed;
- acquire forensic images, analyse malware or provide certified forensic chain of custody; CRAB may store organisation-provided forensic artefacts as opaque evidence files;
- determine whether an event is legally reportable;
- provide legal advice or decide whether legal privilege applies;
- provide general-purpose chat, voice conferencing or video conferencing; stored audio/video evidence is not conferencing;
- negotiate or pay ransoms;
- hard-code jurisdictions, regulators, legal thresholds, deadlines, holiday calendars or operational role titles;
- provide a graphical workflow designer, branching workflow language or general automation/orchestration engine;
- become a general ticketing platform;
- become an enterprise monitoring or observability platform;
- provide automatic technical remediation;
- provide automatic model-provider failover;
- require vector search, embeddings or a vector database in V0.2;
- provide a large integration marketplace or vendor-specific notification SDK estate;
- operate as a multi-tenant hosted Software as a Service (SaaS) platform;
- maintain or author the organisation's incident-response plan as a document-management system;
- ingest runbook/retrieval corpus formats beyond Markdown and DOCX in V0.2; arbitrary evidence-file storage is a separate capability and does not make those files part of the retrieval corpus by default;
- execute, emulate, detonate, transcode or automatically analyse uploaded evidence media;
- become a customer-service/helpdesk platform, supplier-management platform, procurement platform or facilities/logistics system merely because incident coordination records those parties or requests;
- guarantee confidentiality against a malicious Linux root or unrestricted PostgreSQL administrator unless a later higher-assurance encryption design explicitly changes that trust boundary.

## Acceptance criteria

### Core operation and degraded operation

**AC-01 — Offline core.** With internet access removed, an authorised user can declare an incident, use the board, read governed runbook guidance, record reports and governed changes, manage actions, see obligation clocks, use local authentication and generate local print material.

**AC-02 — Corporate identity unavailable.** With the corporate identity provider unavailable, local authentication remains functional and a Major can place CRAB in isolate mode.

**AC-03 — Corporate services unavailable.** No core incident function depends on the corporate network, collaboration platform or shared drive once CRAB has been prepared.

**AC-04 — Model unavailable.** With all model lanes unavailable, deterministic retrieval, ledger, controls, clocks, board, actions, evidence register, messaging, notifications through available deterministic channels, print and recovery functions continue.

### Governance and identity

**AC-05 — One governed change path.** Every governed Item kind uses the same proposal/confirmation control path.

**AC-06 — Maker and confirmer differ.** A proposal cannot be normally confirmed by the same named human who authored it.

**AC-07 — Controlled self-approval.** Self-approval succeeds only when durable policy permits it, the configured single-person window is active and a non-blank reason is provided; it is visibly flagged on the single-approval dashboard.

**AC-08 — No destructive editing.** A governed live-incident record cannot be deleted or overwritten; correction creates a new version.

**AC-09 — Agent attribution.** Every agent-originated operation identifies its named human principal and cannot exceed that human's access.

**AC-10 — Break-glass.** Exactly two distinct valid break-glass keyholders can establish time-limited emergency access; one keyholder cannot; the event is ledgered and an alert attempt is recorded.

**AC-11 — Isolate mode.** In isolate mode, OIDC federation is refused while configured local password, passkey, TOTP and break-glass paths remain available.

**AC-12 — Out-of-band verification.** Inbound contact details cannot replace the pre-registered verification route.

### Privilege, time and exercise separation

**AC-13 — Privilege containment.** A user without a required compartment cannot view, search, retrieve, export or send that compartment's material to a model.

**AC-14 — Privilege change governed.** Changing an item's privilege/compartment status requires maker-checker and is preserved in history.

**AC-15 — Awareness fidelity.** A fact can show separately when it occurred, was flagged and was recorded, plus configured person/role awareness times.

**AC-16 — Time uncertainty.** Every ledger entry records time status; when time is untrusted, time-dependent displays and deadlines are visibly marked `time unverified`.

**AC-17 — Exercise segregation.** An exercise starts no real external obligation, performs no external notification, is visibly labelled on every output, and cannot be converted into a live incident.

### People, actions and checklists

**AC-18 — Operational roles.** The board and printouts show organisation-defined operational role holders and deputies without changing CRAB's Major/Minor/Observer permission model.

**AC-19 — Role handover.** A role handover preserves old and new holder, effective time and complete history.

**AC-20 — Action ownership.** Every action has an owner and status; any due action derives and displays overdue state correctly.

**AC-21 — Checklist execution.** A configured checklist can be activated without code change, its steps can be owned, completed or skipped with a reason, and the source runbook/checklist reference remains visible.

**AC-22 — No workflow engine dependency.** Checklist execution works without a graphical workflow engine, scripting language or automation service.

### Obligations

**AC-23 — User-defined obligations.** A user-defined obligation can be added through maker-checker with no code change and produces the expected deadline from trigger, duration, unit, timezone and calendar.

**AC-24 — Provisional clocks.** A proposed triggering fact creates a provisional clock; confirmation makes it firm; rejection removes its active effect while history remains.

**AC-25 — Calendar behaviour.** Hours, calendar days, business days, configured weekends/holidays and deadline adjustments produce deterministic results or visible refusal for contradictory input.

**AC-26 — Suggested obligations remain proposals.** Suggested-input output cannot become active without normal maker-checker.

### Guidance and AI

**AC-27 — Governed retrieval.** A runbook question returns only authorised passages with stable citations, or a governed refusal when the authorised corpus does not support it.

**AC-28 — No model dependency for retrieval.** Keyword retrieval works with no embedding model, local LLM or network.

**AC-29 — Sensitive managed-lane refusal.** Content not approved for a managed cloud lane is deterministically refused before the provider call.

**AC-30 — Human authority over AI.** A model-generated draft or suggestion never creates effective governed state without a named human maker and required confirmation.

**AC-31 — Evaluation regression.** The Bafflewick evaluation set meets its configured threshold and a regression beyond the allowed threshold fails Continuous Integration (CI).

### Communications

**AC-32 — Structured situation reports.** A situation report can be generated from CRAB state on the configured cadence and becomes an issued governed communication only after the required confirmation.

**AC-33 — Delivery outcome visible.** Every notification attempt records delivered, failed or suppressed; one channel failure does not cause another attempted channel to be silently omitted.

**AC-34 — Exercise notification suppression.** Exercise-mode external notifications are suppressed before external I/O while local exercise records remain.

### Evidence

**AC-35 — Evidence register.** Evidence records capture the configured identity, source/collector, times, location, hash and handling history without claiming forensic acquisition.

**AC-36 — Attachment integrity.** Altering a stored/exported attachment after its recorded hash is established is detected during integrity verification.

### Paper contingency and visual quality

**AC-37 — Incident Brief.** A current Incident Brief can be locally generated as a professional PDF, targets one to two pages, carries generation/source version information, and is regenerated on material change and schedule.

**AC-38 — Emergency Response Pack.** A current Emergency Response Pack can be locally generated without internet, cloud service or model and contains the organisation-approved contingency material and blank paper forms.

**AC-39 — Database-down paper access.** With PostgreSQL stopped, the last successfully generated Incident Brief and Emergency Response Pack remain readable from their configured derived-file location.

**AC-40 — Independent paper copy.** When an optional secondary/removable export location is configured and available, a successful pack generation updates it atomically or records a visible failure without destroying the last good copy.

**AC-41 — Print quality.** Incident briefs, emergency packs, evidence registers, timelines, Post-Incident Review reports and blank forms are purpose-designed for paper, render correctly on A4 and US Letter, and remain clear in colour and monochrome.

**AC-42 — Paper reconciliation.** A paper-era event can be transcribed after restoration while preserving original occurrence/paper time, transcription time, transcriber, source reference and time confidence; normal governance applies and the system never rewrites it as if digitally entered earlier.

**AC-43 — Web visual quality.** The primary incident, approval, readiness and closure screens pass a documented visual review showing a polished, coherent production interface rather than an engineering prototype.

### Recovery, review and closure

**AC-44 — Recovery coordination.** Recovery actions, owners, restoration priorities, verification and residual risks can be recorded without CRAB executing technical remediation.

**AC-45 — Stand-down distinct from closure.** A confirmed stand-down ends the active emergency phase without immediately making the incident read-only.

**AC-46 — Post-Incident Review.** A PIR can be generated from the ledger, reviewed by humans and confirmed before administrative closure.

**AC-47 — Closure integrity.** Administrative closure is refused until required closure conditions are met; after closure the incident is read-only.

**AC-48 — Privileged and shareable closure.** Closure produces immutable privileged and shareable outputs, and the shareable output contains no privileged material.

### Readiness, backup and integrity

**AC-49 — Readiness check.** One readiness page/command reports the required database, ledger, backup, restore-test, time, authentication, pack, notification, corpus, model, storage and certificate states without requiring specialist platform tooling.

**AC-50 — Restore and rebuild.** A documented restore onto a replacement host verifies the ledger and rebuilds projections that match the pre-backup logical state.

**AC-51 — Standalone verification.** The standalone verifier detects a broken chain, invalid server signature, invalid available human signature, impermissible self-confirmation and live/exercise mismatch without the running database or network.

**AC-52 — Tamper evidence.** Altering a past authoritative ledger entry is detected on full verification.

**AC-53 — Source-of-truth discipline.** Rebuilding all projections from the ledger yields the same business state as the live projections; no second live authoritative store is required.

### Interface, status and build

**AC-54 — One control plane.** Equivalent state-changing requests through web, local API, CLI or MCP reach the same authorisation, proposal, confirmation and ledger mechanisms.

**AC-55 — System status visible.** The persistent status area reflects setup, isolate mode, break-glass, time trust, model-lane availability, active single-person window, pending approvals and paper-pack freshness within one page refresh.

**AC-56 — Decision-loop time.** CRAB calculates and displays fact-flagged → decision-confirmed → notification-issued timing where all three events exist.

**AC-57 — One application build.** CRAB builds as one application binary for `linux/amd64` and `linux/arm64`; runtime components are internal to that application rather than separately operated services.

**AC-58 — Minimum-hardware test.** The Raspberry Pi 5 (8 GB) acceptance run is attempted against every EDD minimum-hardware budget and degraded-operation test and the measured result is recorded whether it passes or fails.

### Scoped access and administration

**AC-59 — No universal Major visibility.** A Major assigned to Incident A cannot discover, read, search, export or retrieve Incident B solely because they are a Major.

**AC-60 — Custodian separation.** A Custodian can administer configured identity/access metadata without automatically gaining incident-content or compartment-content access.

**AC-61 — Sponsored incident access.** Incident membership changes require the configured scoped sponsorship/administrative control path; the grant, expiry and revocation history is preserved.

**AC-62 — Compartment read-in.** Incident membership alone does not grant a restricted compartment; read-in/read-out follows the configured Compartment Controller path and is historically visible.

**AC-63 — Break-glass does not widen scope.** Successful break-glass authentication leaves the principal's incident, compartment and action permissions unchanged.

**AC-64 — Object-level enforcement.** Changing an incident/item/blob identifier in a web/API/CLI/MCP request cannot return an object outside the principal's authorised scope.

**AC-65 — Hidden means hidden.** Unauthorised incidents/compartments do not appear through search, lists, counts, retrieval citations or generated outputs where their existence is not authorised for disclosure.

**AC-66 — Export is separately authorised.** A principal who can read an item cannot automatically bulk-export, print or download it unless the relevant action permission allows that output.

### Intake, scope and continuity

**AC-67 — First-report intake.** A first report can be captured with source, received time, description and optional evidence without automatically declaring a live incident; a human can later associate, reject/retain or use it to propose declaration.

**AC-68 — Phone evidence capture.** From a supported phone browser, an authorised user can take/select supported media or a file and submit it to an assigned incident without a separate native application.

**AC-69 — Evolving scope.** A responder can record changing affected/suspected assets, services, people/data populations, locations and suppliers while preserving previous confirmed assessments.

**AC-70 — Unknowns remain unknown.** Questions, unknowns, assumptions and hypotheses are visibly distinguishable from confirmed facts and cannot silently become confirmed facts.

**AC-71 — Severity history.** Human-selected severity can be escalated or de-escalated through governance with reason/evidence and complete prior history.

**AC-72 — Shift handover.** A handover view/record can be produced from authorised CRAB state showing situation, roles, critical actions, pending decisions, clocks, unknowns, next objectives and communication commitments.

**AC-73 — Resource request coordination.** A surge/specialist/equipment/funding/logistics request can be assigned, prioritised, tracked and linked to its authority without CRAB becoming the fulfilment system.

### External parties and communications

**AC-74 — Temporary external access.** An external responder can receive incident/compartment access that expires or is revoked as configured; historical actions and provenance remain after access ends.

**AC-75 — Selective release.** An incident participant can be prevented from seeing a restricted report, communication or attachment inside an incident they are otherwise assigned to.

**AC-76 — Third-party coordination.** Contacts, actions, communications and decisions involving suppliers, insurers, counsel or other authorised external parties can be recorded without requiring a separate integration platform.

**AC-77 — Communication provenance.** An issued material communication can show approved version, audience, authority/approver, channel, issue/attempt time and delivery outcome, plus acknowledgement/response where recorded.

**AC-78 — Incoming enquiries.** Material incoming stakeholder enquiries can be recorded and assigned without opening a general-purpose chat/helpdesk subsystem.

### Evidence media and hostile files

**AC-79 — Rich evidence media.** CRAB can preserve organisation-permitted images, audio, video and arbitrary binary evidence while retaining original bytes, filename, size, media metadata, hash, incident link and compartment.

**AC-80 — Hostile file safety.** Uploading an active or potentially malicious file does not execute it on the CRAB server or render it as trusted active browser content; unsupported/unsafe content remains retrievable only through a safe authorised delivery path.

**AC-81 — Attachment re-authorisation.** Every evidence-media view/download is authorised at request time; possession or guessing of a blob/attachment identifier is insufficient.

**AC-82 — Configurable evidence size.** Evidence upload size/storage limits are deployment-configurable and enforced before durable write; the Raspberry Pi test may use a smaller fixture limit without becoming the product-wide ceiling.

### Obligations, relationships and records lifecycle

**AC-83 — Obligation override history.** A confirmed obligation may be governedly amended, extended, waived or replaced while preserving the original deadline/rule, authority/source, reason and resulting status/deadline.

**AC-84 — Related incidents without access leakage.** Two incidents can be linked without merging histories or granting either side's participants access to the other incident.

**AC-85 — Retention and hold.** A closed incident can carry governed retention and legal/records-hold metadata; held records are not eligible for disposition while the hold is active.

**AC-86 — Governed disposition boundary.** Any eventual post-retention disposition requires an explicit authorised process and audit record and cannot masquerade as an ordinary live-incident edit/delete.

### Accessibility and operational quality

**AC-87 — Accessible status.** Core status/priority/permission meaning remains understandable without colour alone and core workflows are keyboard-operable.

**AC-88 — Scope clarity.** The interface makes the current incident and relevant compartment context visible enough to reduce accidental work in the wrong scope without exposing unauthorised names/details.


## Definition of done for V0.2

V0.2 is complete when:

- every acceptance criterion has executable, reviewable or visual acceptance evidence appropriate to the criterion;
- deterministic and failure-path tests pass in CI;
- Bafflewick end-to-end acceptance covers first-report intake, scoped access, rich evidence capture, live incident operation, external participant access/revocation, model loss, corporate identity loss, paper fallback, restoration, paper reconciliation, shift handover, stand-down, PIR and closure;
- an exercise proves the current Emergency Response Pack is usable without the live application;
- backup, restore, projection rebuild and standalone ledger verification are demonstrated from documentation;
- the Raspberry Pi minimum-hardware acceptance record is captured;
- the web interface and print artefacts pass the documented product-quality review;
- the generated EDD is recovered from the implementation with `codebase-to-edd`;
- the hand-written/generated EDD comparison shows that maker-checker, incident/compartment filtering, hostile-evidence handling, state authority, paper contingency and degradation boundaries survived implementation;
- the EDD is then restructured for LIVE status with design-detail sections retained as an appendix.

---

## Appendix A — Design decision record

The earlier design phase produced D-01 to D-45. This baseline preserves those decisions unless explicitly refined or superseded below.

| # | Decision | Status |
|---|---|---|
| D-01 | Business problem is cyber incident response, not problem management | retained |
| D-02 | Product name: Cyber Response Action Board (CRAB) | retained |
| D-03 | Global scope; no hard-coded local rules; obligations user-defined | retained |
| D-04 | Web UI primary; headless CLI, local API and MCP also supported | retained |
| D-05 | External PostgreSQL; CRAB process itself holds no authoritative durable state | **refined by D-46** |
| D-06 | V0.1 corpus formats: Markdown and DOCX | retained |
| D-07 | Raspberry Pi 5 (8 GB) acceptance is an aspiration/test, not production target | retained/refined by D-35 |
| D-08 | Every governed change is maker-checker; author cannot normally confirm own version | retained |
| D-09 | Provisional clocks begin from proposed trigger facts | retained |
| D-10 | Controlled single-person windows with flagged self-approval | retained |
| D-11 | No destructive delete during live incident; closed incidents read-only | retained |
| D-12 | Setup prepared by one Major and approved by another | retained |
| D-13 | Human in the middle; agents act for named humans | retained |
| D-14 | Break-glass uses personal offline keys and exactly two keyholders | retained |
| D-15 | Application roles: Major, Minor, Observer | retained |
| D-16 | Confirmed governed entries carry maker, confirmer and server provenance/signatures | retained |
| D-17 | Closure material retained immutably | retained |
| D-18 | Local/federated/passkey/TOTP/break-glass authentication and out-of-band verification in scope | retained |
| D-19 | Requirements found by benchmarking are kept; engineering simplifies mechanisms, not mission | retained |
| D-20 | Legal privilege/compartment status on every relevant item; privileged and shareable outputs | retained |
| D-21 | Actions have owner, status and due time | retained |
| D-22 | Exercise mode segregated from live incidents | retained |
| D-23 | Awareness distinguishes occurred, flagged, recorded and recipient-awareness times | retained |
| D-24 | Structured incident messaging, not general chat | retained |
| D-25 | User-defined situation-report cadence | retained |
| D-26 | Printable incident material | **expanded by D-47** |
| D-27 | Attachments hashed for integrity; no claim of forensic chain of custody | **expanded by D-49** |
| D-28 | Holding statements and scripts in approved setup | retained |
| D-29 | User-defined business impact tiers | retained |
| D-30 | Isolate mode disables federation | retained |
| D-31 | Pi test, triple provenance, MCP and evaluation remain | retained |
| D-32 | One Item lifecycle, derived views, two schedulers, one plotting/report path, ledger as truth | retained |
| D-33 | Twelve component lineages; timeline/actions/messaging/exercises remain inside CRAB | retained |
| D-34 | Decision-loop time measured | retained |
| D-35 | Production runs on any suitable independent Linux host; Pi is minimum-hardware test only | retained |
| D-36 | Network independence delivered through deployment patterns, not an embedded networking subsystem | retained |
| D-37 | Local certificate authority only for offline local deployment; public certificates otherwise | retained |
| D-38 | Time status on ledger entries; manual time change governed | retained |
| D-39 | Break-glass keys static/offline/typed; responder network cards separate | retained |
| D-40 | System status visible throughout the UI | retained |
| D-41 | Validators are trusted/accountable; no scoring or behavioural monitoring | retained |
| D-42 | Thin pre-generated printable pack | **superseded by D-47** |
| D-43 | Backup, restore, rebuild and custody records | retained |
| D-44 | Independent portable ledger verification | retained |
| D-45 | Explicit resource budgets measured on the minimum-hardware test | retained |
| **D-46** | PostgreSQL is the single durable live store; append-only ledger authoritative for governed state; operational projections rebuildable; supporting immutable content/credentials remain in the same store; exported ledger is not a second live authority | new baseline |
| **D-47** | Paper contingency is first-class: polished local PDF Incident Brief plus comprehensive Emergency Response Pack, last-good copies and optional independent/removable copy | new baseline |
| **D-48** | Organisation-defined operational incident roles and deputies are separate from Major/Minor/Observer permissions | new baseline |
| **D-49** | CRAB records evidence metadata and handling history but does not acquire/analyse evidence or claim certified forensic custody | new baseline |
| **D-50** | Simple executable response checklists are in scope; no graphical workflow engine or branching automation language | new baseline |
| **D-51** | Recovery coordination, formal stand-down and confirmed Post-Incident Review precede administrative closure; long-term remediation is handed off | new baseline |
| **D-52** | A single readiness page/command tests CRAB's own ability to operate and reports last rehearsal/restore evidence | new baseline |
| **D-53** | Web UI and print output must be professionally designed and visually polished; appearance and pressure-legibility are acceptance requirements | new baseline |
| **D-54** | D-47 to D-53 create no new micro-components; they are CRAB-owned Item kinds, views, renderers and operating flows composed from the existing twelve component lineages | new baseline |

| **D-55** | Major/Minor/Observer authority is scoped by incident assignment and compartment; no application role has universal incident visibility by role alone | V0.2 requirement |
| **D-56** | Custodian administers identity/access metadata without automatic content access; Incident Sponsor and Compartment Controller are scoped designations rather than new universal roles | V0.2 requirement |
| **D-57** | Break-glass recovers authentication only and does not widen incident/compartment/action authorisation | V0.2 requirement |
| **D-58** | First-report intake and phone-browser field capture are in scope before full incident declaration | V0.2 requirement |
| **D-59** | Scope/impact, explicit unknowns/hypotheses and human severity changes are preserved as evolving incident state/history | V0.2 requirement |
| **D-60** | External responders use the same scoped access model; access may expire/revoke and ordinary read does not imply unrestricted export/download/print | V0.2 requirement |
| **D-61** | Long-running incident continuity includes shift handover, deputies, pending decisions, unknowns, deadlines and resource/surge requests | V0.2 requirement |
| **D-62** | Material incoming/outgoing communication provenance is retained without creating a general-purpose chat/helpdesk product | V0.2 requirement |
| **D-63** | CRAB preserves rich media and arbitrary permitted evidence as untrusted immutable content; it does not execute, transcode or analyse that content | V0.2 requirement |
| **D-64** | Evidence upload size is configurable; the Pi fixture budget is not a product-wide media ceiling | V0.2 requirement |
| **D-65** | Obligation extensions, waivers, amendments and replacements preserve original rule/deadline and governing authority/reason | V0.2 requirement |
| **D-66** | Incidents may be related without destructive merge or access inheritance | V0.2 requirement |
| **D-67** | Post-closure retention/legal-hold/disposition is governed separately from live-incident no-delete semantics | V0.2 requirement |
| **D-68** | Accessibility, phone capture and clear incident/compartment scope are part of product quality | V0.2 requirement |
| **D-69** | New requirements do not automatically create new micro-components; secure immutable binary-content storage is the only immediate extraction candidate and must pass the EDD removal/reuse test | V0.2 design direction |

## Appendix B — External requirement basis

This PRD is intentionally not a copy of any single framework or commercial product. The V0.1 baseline and V0.2 reopening were cross-checked against:

- Australian Signals Directorate / Australian Cyber Security Centre guidance on cyber incident response planning, including first-report routes, hard-copy plans/playbooks, backup communications, roles/deputies, evidence records and rich evidence sources, external responders, surge/logistics, exercises, recovery and post-incident learning;
- National Institute of Standards and Technology (NIST) Special Publication 800-61 Revision 3, *Incident Response Recommendations and Considerations for Cybersecurity Risk Management*, including lifecycle, supplier coordination, evidence preservation and records considerations;
- UK National Cyber Security Centre incident-management guidance on escalation, empowered decision-makers, deputies, alternate communications and sustained coordination;
- current incident/case-management capabilities in platforms including ServiceNow Security Incident Response, IBM QRadar SOAR, TheHive, Cortex XSOAR, PagerDuty and crisis-management platforms, particularly attachments/evidence, external participation, controlled sharing, mobile use and related cases;
- practitioner feedback on operational complexity, workflow maintenance, poor search, clunky case management and specialist SOAR engineering burden.

Those sources validate requirements and failure modes. They do not determine CRAB's architecture. CRAB retains the required incident-management outcomes while deliberately refusing unnecessary workflow engines, automation estates and service sprawl.
