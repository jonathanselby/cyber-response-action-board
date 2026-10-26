# CRAB — Test and Acceptance Notes

**Version:** 0.1  
**Date:** 20 October 2026  
**Companion to:** `CRAB-EDD-v0.4-FINAL.md`

This file keeps detailed test material outside the final EDD. The PRD acceptance criteria remain the product contract; tests may be consolidated/table-driven during implementation, but behavioural, failure and attack coverage must not disappear.

**Rule:** mechanisms may merge; negative-security and product outcomes survive.

---

## 1. V0.3 baseline test catalogue

The following T-01 to T-150 catalogue is retained from EDD V0.3 as the behavioural baseline. References such as `9.2` identify the old V0.3 pseudocode section and should be replaced by concrete Go test names during implementation; the test intent and acceptance mapping are what remain authoritative.

### 1.1 Core control and state

| ID | Pseudocode / area | Given | When | Then | Source |
|---|---|---|---|---|---|
| T-01 | 9.2 | each governed Item kind | propose/confirm | same control implementation is used | AC-05 / D-32 |
| T-02 | 9.3 | author of proposal | tries normal confirmation | denied | AC-06 |
| T-03 | 9.3 | policy + active single window + reason | author confirms | succeeds, visibly flagged | AC-07 |
| T-04 | 9.3 | missing any self-approval prerequisite | author confirms | denied | AC-07 |
| T-05 | Item versioning | confirmed live Item | user corrects | new version created; prior unchanged | AC-08 |
| T-06 | 9.4 | normal append | commit | sequence/hash/signature valid | AC-52 |
| T-07 | 9.4 | unexpected ledger tail | append | mutation fails; full verification required | integrity failure |
| T-08 | 9.5 | live projections | full rebuild | logical state/digest matches | AC-53 |
| T-09 | incident | closed incident | propose state change | refused | AC-47 |
| T-10 | reports | Minor authorised for incident | writes comment/progress report | immutable report accepted, no governed state mutation | role model |

### 1.2 Identity, authorisation and privilege

| ID | Area | Given | When | Then | Source |
|---|---|---|---|---|---|
| T-11 | auth | corporate IdP unavailable | local login | succeeds if valid | AC-02 |
| T-12 | isolate | isolate confirmed | OIDC login | refused before token verification | AC-11 |
| T-13 | break-glass | one valid keyholder | recover | refused | AC-10 |
| T-14 | break-glass | two distinct valid keyholders | recover | time-limited session + report + alert attempt | AC-10 |
| T-15 | delegation | agent + correct named human | request | permissions equal named human | AC-09 |
| T-16 | delegation | agent swaps human | request | denied | AC-09 |
| T-17 | privilege | unprivileged principal + privileged Item | read/search/export | no disclosure | AC-13 |
| T-18 | 9.6 | privileged passage lexically strongest | unprivileged retrieval | passage excluded before ranking | AC-13/27 |
| T-19 | model | restricted content | managed lane requested | refusal before network I/O | AC-29 |
| T-20 | privilege | existing Item | privilege change | ordinary maker-checker required and history preserved | AC-14 |
| T-21 | OOB | inbound contact differs from registered callback | verification plan | registered route retained, inbound ignored | AC-12 |

### 1.3 Incident roles, actions and checklists

| ID | Area | Given | When | Then | Source |
|---|---|---|---|---|---|
| T-22 | operational roles | custom organisation role names | assign/deputise | board/print display names; auth roles unchanged | AC-18 |
| T-23 | handover | active role holder | confirmed replacement effective at time | old/new/effective history visible | AC-19 |
| T-24 | actions | action with due time | now passes due | overdue derives true without stored state mutation | AC-20 |
| T-25 | checklist | approved template | activate | proposed Actions created with source refs | AC-21 |
| T-26 | checklist | step | skip without reason | refused | AC-21 |
| T-27 | checklist | no workflow subsystem running | execute template/actions | full checklist capability works | AC-22 |

### 1.4 Obligations and time

| ID | Area | Given | When | Then | Source |
|---|---|---|---|---|---|
| T-28 | 9.8 | proposed trigger fact | calculate | provisional deadline | AC-24 |
| T-29 | 9.8 | trigger fact confirmed | calculate | same deadline becomes firm | AC-24 |
| T-30 | 9.8 | trigger fact rejected | calculate | active clock removed; history remains | AC-24 |
| T-31 | clock | hours/calendar/business-day fixtures incl DST | calculate | reference expected results | AC-23/25 |
| T-32 | clock | contradictory timezone/calendar | calculate | visible refusal, no silent coercion | AC-25 |
| T-33 | time | time status untrusted | display clocks/timeline | `time unverified` visible | AC-16 |
| T-34 | awareness | fact with four time concepts | timeline | concepts displayed separately | AC-15 |
| T-35 | obligation config | new organisation rule | confirm | works without code change | AC-23 |
| T-36 | suggested input | model suggestion | before human control | obligation inactive | AC-26 |

### 1.5 Retrieval, models and evaluation

| ID | Area | Given | When | Then | Source |
|---|---|---|---|---|---|
| T-37 | 9.6 | supported authorised query | retrieve | cited passages returned | AC-27 |
| T-38 | 9.6 | unsupported query | retrieve | governed refusal | AC-27 |
| T-39 | 9.6 | no model/network | retrieve | keyword retrieval works | AC-28 |
| T-40 | 9.7 | model unavailable | draft | failure shown; sources remain | AC-04 |
| T-41 | model | queue at max | new model job | rejected; core page/mutation remains responsive | budget / priority |
| T-42 | evaluation | Bafflewick baseline | run evaluator | threshold met | AC-31 |
| T-43 | evaluation | injected regression | CI | build gate fails | AC-31 |
| T-44 | AI authority | model draft | no human maker/confirmation | no effective governed state | AC-30 |

### 1.6 Communications and exercises

| ID | Area | Given | When | Then | Source |
|---|---|---|---|---|---|
| T-45 | situation report | cadence due | generate | draft uses current CRAB state | AC-32 |
| T-46 | communication | unconfirmed draft | send | external governed issue refused | AC-32 |
| T-47 | notifier | first channel fails, second succeeds | attempt | both outcomes recorded | AC-33 |
| T-48 | exercise | external webhook configured | notify | external I/O suppressed; local record retained | AC-17, AC-34 |
| T-49 | exercise | all output types | render/export | `EXERCISE` visible | AC-17 |
| T-50 | exercise | exercise incident | attempt conversion to live | refused | AC-17 |

### 1.7 Evidence and integrity

| ID | Area | Given | When | Then | Source |
|---|---|---|---|---|---|
| T-51 | evidence | complete evidence metadata | record/handling update | history visible without forensic claim | AC-35 |
| T-52 | attachment | stored blob changed/corrupted | verify | hash mismatch detected | AC-36 |
| T-53 | ledger | historical entry byte altered | full verify | chain/signature failure | AC-52 |
| T-54 | portable verifier | broken hash/signature/self-approval/mode fixtures | `crab verify` | each distinct failure detected offline | AC-51 |
| T-55 | startup | ledger verification pending | user mutates | mutation gated until pass | integrity design |
| T-56 | startup | ledger verification fails | user reads vs mutates | authorised read/export available; mutation refused | failure row |

### 1.8 Paper, UI and outage operation

| ID | Area | Given | When | Then | Source |
|---|---|---|---|---|---|
| T-57 | 9.9 | material incident change | generate brief | source ledger position current; 1–2 page target | AC-37 |
| T-58 | 9.9 | approved setup | generate emergency pack offline | all mandatory configured sections/forms present | AC-38 |
| T-59 | print | A4 and Letter fixtures | render | no clipped content; controlled page breaks | AC-41 |
| T-60 | print | colour and monochrome inspection | render | no status meaning lost; readable hierarchy | AC-41 |
| T-61 | print failure | last-good pack exists | renderer fails | last-good remains byte-identical; status stale | failure row |
| T-62 | DB loss | generated packs exist | PostgreSQL stopped | PDFs remain readable | AC-39 |
| T-63 | secondary export | destination mounted | generate | primary + secondary updated atomically | AC-40 |
| T-64 | secondary export | destination absent | generate | primary succeeds; warning recorded; old secondary untouched | AC-40 |
| T-65 | 9.10 | paper-era event | reconcile | historic time/source + current transcription time both preserved | AC-42 |
| T-66 | 9.10 | paper event | reconcile | digital `recorded_at` cannot be backdated | AC-42 |
| T-67 | web | primary incident/approval/readiness/closure fixtures | visual review | documented production-quality review passes | AC-43 |
| T-68 | web | external network disconnected | load UI | no externally hosted asset required | AC-01 / resilience |

### 1.9 Recovery, closure and readiness

| ID | Area | Given | When | Then | Source |
|---|---|---|---|---|---|
| T-69 | recovery | recovery actions/residual risk | record | coordination visible; no technical action executed | AC-44 |
| T-70 | 9.12 | confirmed stand-down | inspect incident | phase changes; incident still writable for PIR | AC-45 |
| T-71 | PIR | complete ledger | generate/review/confirm | PIR contains required derived sections | AC-46 |
| T-72 | closure | PIR missing | close | refused | AC-47 |
| T-73 | closure | privileged + nonprivileged data | generate shareable | privileged material absent | AC-48 |
| T-74 | 9.11 | healthy configured system | readiness | required checks reported PASS/WARN/FAIL coherently | AC-49 |
| T-75 | readiness | optional model down | run | overall core readiness not falsely failed; optional degradation visible | AC-49 |
| T-76 | backup/restore | known Bafflewick state | backup → new host restore → rebuild | logical projection state matches | AC-50 |
| T-77 | restore | signing private key absent | start | read/verify possible; governed writes disabled | failure row |
| T-78 | host loss | current secondary/printed pack + backup | rebuild | operation can continue on paper then restore/reconcile | AC-01/38/42/50 |

### 1.10 Interface, status, build and performance

| ID | Area | Given | When | Then | Source |
|---|---|---|---|---|---|
| T-79 | adapters | equivalent proposal via web/API/CLI/MCP | execute | same control/ledger semantics | AC-54 |
| T-80 | status | each status state changes | refresh | persistent status area reflects within one refresh | AC-55 |
| T-81 | metric | fact flag + decision confirm + notification issue | calculate | decision-loop durations correct | AC-56 |
| T-82 | build | clean source | build | single application binary for amd64/arm64 | AC-57 |
| T-83 | Pi | V0.1 binary/PostgreSQL | run budget suite | every budget measured and recorded | AC-58 |
| T-84 | overload | model saturates Pi | core operation | model work sheds first; ledger/controls remain within limit | priority TEST |
| T-85 | TLS | public host | passkey assertion | works over public HTTPS | design-review TEST |
| T-86 | TLS | offline local CA host | passkey assertion | works with rehearsed local trust | design-review TEST |
| T-87 | page budget | Bafflewick active incident | load non-model pages | normal/max response targets measured | budget |
| T-88 | dependencies | module graph | release review | direct dependencies within budget and justified | budget |
| T-89 | deployment independence | internet/corporate network/shared services unavailable but rehearsed independent CRAB path available | operate incident | core incident functions remain available without corporate services | AC-03 |

Coverage rule: every PRD acceptance criterion, every material failure row, every degraded-operation row and every `TEST` item in section 8 maps to at least one named test above. Implementation may add tests but may not remove the mapped evidence without replacing it.

### 1.11 V0.2 authority, tiering and exfiltration hardening

| ID | Area | Given | When | Then | Source |
|---|---|---|---|---|---|
| T-90 | incident scope | Major not assigned to Incident B | list/read Incident B | Incident B/content not disclosed | AC-59 / D-57/D-62 |
| T-91 | incident scope | Major assigned to Incident A only | list incidents | only permitted incident metadata returned | AC-59/65 / D-57 |
| T-92 | Minor scope | Minor assigned to Incident A only | direct object ID from Incident B | denied/no useful existence oracle | AC-64/65 / D-62 |
| T-93 | Observer | Observer assigned to incident | attempt comment/propose/export | read permitted; mutation/export denied unless separately granted | AC-66 / D-62/D-63 |
| T-94 | Custodian | Custodian not incident-assigned | open incident/search/retrieval | content denied; access metadata screen remains usable | AC-60 / D-58 |
| T-95 | Custodian metadata | sensitive incident configured opaque | access administration | opaque incident/compartment identifier sufficient to administer grant | AC-60/65 / D-64 |
| T-96 | incident grant | valid Sponsor proposal + Custodian confirmation | person next reads incident | access succeeds only after confirmation | AC-61 / D-59 |
| T-97 | incident grant | Custodian invents grant without Sponsor proposal | confirm/administer | refused | AC-61 / D-59 |
| T-98 | incident grant | Sponsor tries to self-confirm access grant as Custodian | confirm | refused; no single-person shortcut | AC-61 / D-59 |
| T-99 | compartment | incident member lacks compartment | direct read/search/download | no compartment content disclosed | AC-62/64 / D-60/D-62 |
| T-100 | compartment | valid Controller proposal + Custodian confirmation | next read | compartment becomes visible | AC-62 / D-60 |
| T-101 | compartment | Custodian proposes read-in without valid Controller | confirm | refused | AC-62 / D-60 |
| T-102 | break-glass | user has scope A only | break-glass succeeds | session still has scope A only | AC-63 / D-61 |
| T-103 | break-glass | break-glass user requests unassigned incident | read | denied | AC-63/64 / D-61/D-62 |
| T-104 | action separation | user may view incident but lacks export action | bulk export | denied | AC-66 / D-63 |
| T-105 | attachment | user guesses attachment ID outside scope | download | denied/no content | AC-64/66 / D-62/D-63 |
| T-106 | print | user lacks compartment but requests privileged print | generate | privileged content absent or request denied as policy defines | AC-66 / D-63/D-67 |
| T-107 | search | unassigned incident contains strongest lexical match | search/retrieval | candidate excluded before ranking | AC-65 / D-62 |
| T-108 | API fields | authorised endpoint does not require sensitive field X | response | X is absent; no wholesale struct serialisation | D-63 |
| T-109 | adapters | equivalent read through web/API/CLI/MCP | authorise | same decision/result scope | AC-64 / D-62 |
| T-110 | presentation tier | source tree | static dependency check | presentation has no SQL/pgx/data-postgres import | D-55/D-56 |
| T-111 | business tier | source tree | static dependency check | domain/business has no concrete PostgreSQL dependency | D-56 |
| T-112 | data tier | unauthorised cross-incident row requested in RLS spike | query | database also denies if RLS retained | D-65 TEST |
| T-113 | database role | runtime DB credentials | inspect privileges | not superuser/table owner/BYPASSRLS if RLS retained | D-65 TEST |
| T-114 | managed AI | authorised incident user but content policy forbids lane | request | no provider network I/O | D-62/D-63 |
| T-115 | logging | incident body/model prompt/secret in request | routine logging | sensitive body/secret absent from logs | exfiltration boundary |
| T-116 | browser | sensitive incident view | inspect response/client storage | no-store policy where applicable; no incident content in localStorage | exfiltration boundary |
| T-117 | paper | standing Emergency Response Pack | render | only approved contingency content; privileged sections excluded unless explicitly separate | D-67 |
| T-118 | privileged paper | separately authorised privileged pack | render | conspicuous marking + authorised principal/custody metadata | D-67 |
| T-119 | backup | ordinary Custodian only | request bulk backup/content export | not granted by Custodian authority alone | AC-60/66 / D-58/D-63 |
| T-120 | bootstrap | fresh installation | establish initial authorities | at least two Custodians established without universal incident-content grant | D-68 TEST |
| T-121 | incident creation | new incident names initial Sponsor(s) | declaration confirmed | only named/scoped participants gain access | D-68 |
| T-122 | compartment creation | new compartment names initial Controller(s) | creation confirmed | no other incident member receives compartment automatically | D-68 |
| T-123 | revocation | confirmed incident or compartment revocation | subsequent request | access denied immediately by current projection; history remains | AC-61/62/74 / D-59/D-60 |
| T-124 | error oracle | unauthorised and nonexistent object IDs | request | responses do not materially distinguish existence | AC-65 / D-62 |
| T-125 | physical/trusted-boundary documentation | authorised human/root/DBA scenario | review threat model | limitation stated; no false prevention claim | D-66 |

V0.2 established **125 named pre-code tests**. V0.3 retains all of them and adds T-126 through T-150 for the reopened requirements.

### 1.12 V0.3 reopened-requirement tests

| ID | Area | Given | When | Then | Source |
|---|---|---|---|---|---|
| T-126 | intake | first report without live incident | submit with description/source | retained without auto-declaration | AC-67 |
| T-127 | intake | first report with media | Major chooses existing incident | association preserves original received/source times | AC-67 |
| T-128 | mobile | supported phone browser | capture/select image and submit | evidence stored without native app | AC-68 |
| T-129 | scope | confirmed scope V1 | new assessment V2 | V1 remains history; V2 effective only after governance | AC-69 |
| T-130 | uncertainty | hypothesis Item | render/search | visibly not a confirmed fact | AC-70 |
| T-131 | severity | severity High | confirmed de-escalation Medium | reason/evidence/history retained | AC-71 |
| T-132 | handover | active incident | generate handover | required situation/actions/clocks/unknowns/objectives shown within authorisation | AC-72 |
| T-133 | resources | specialist request | assign/update | owner/priority/status/authority tracked | AC-73 |
| T-134 | temp access | external Minor with expiry | after expiry requests incident | denied; historical authored records remain | AC-74 |
| T-135 | selective sharing | assigned user lacks LEGAL | request legal attachment | denied without content/existence leakage beyond policy | AC-75/65 |
| T-136 | third party | insurer/supplier contact | record action/comms | normal CRAB records suffice; no separate workflow service | AC-76 |
| T-137 | comms | approved notification | issue attempt | approved version/audience/approver/channel/time/outcome retained | AC-77 |
| T-138 | enquiry | incoming media/customer enquiry | record/assign | tracked as structured response work, not chat | AC-78 |
| T-139 | evidence | JPEG/audio/video | upload | original bytes + metadata + hash retained | AC-79 |
| T-140 | hostile evidence | HTML/SVG/executable/macro file | upload then view | never executed/served as trusted active content | AC-80 |
| T-141 | blob access | guessed blob ID from other incident | download | denied before bytes returned | AC-81/64 |
| T-142 | media limits | payload exceeds configured max | upload | refused before durable commit; no partial authoritative record | AC-82 |
| T-143 | media limits | Pi fixture uses 25 MiB, production config 500 MiB | run | limits differ without code change | AC-82 |
| T-144 | obligation | confirmed deadline | authorised extension | original + new deadline/reason/authority visible | AC-83 |
| T-145 | related incident | user sees A not B | view A relationship | B content/metadata hidden per policy; no access inheritance | AC-84 |
| T-146 | hold | closed incident under legal hold | retention expires | disposition eligibility remains false | AC-85 |
| T-147 | disposition | closed incident eligible, no hold | attempt ordinary delete | refused; separate future disposition path required | AC-86 |
| T-148 | accessibility | status-only colour removed/monochrome | operate core view | meaning remains understandable | AC-87 |
| T-149 | keyboard | core approval/intake workflow | keyboard only | workflow operable | AC-87 |
| T-150 | scope UI | user assigned multiple incidents | switch/current view | current incident/compartment clear without leaking unavailable scope | AC-88 |

V0.3 therefore carries **150 named pre-code tests**. Further V0.4 simplification may merge mechanisms but must preserve the behavioural coverage.

## 2. Final-design regression additions

The V0.3 catalogue above remains valuable behavioural coverage. V0.4 changes mechanisms in several places, so the following tests are added rather than deleting earlier attack/failure intent.

| ID | Area | Given | When | Then | Source |
|---|---|---|---|---|---|
| T-151 | Record boundary | authorised responder records an observation saying severity should change | `Record(...)` commits | observation appears in timeline/history but effective severity does not change | V0.4 section 6 |
| T-152 | governed communication | drafted situation report exists | user attempts to mark/issue it through immediate-record path | refused; approved/issued communication requires normal `Propose -> Decide` | V0.4 section 13.8 |
| T-153 | GENERAL access | authenticated Major not assigned to Incident A | request GENERAL item from A | denied; role alone grants no incident membership | security floor |
| T-154 | restricted compartment | user has governed Incident A membership/GENERAL but no LEGAL read-in | request LEGAL object | denied; GENERAL does not imply restricted compartment | security floor |
| T-155 | object binding | user changes object/blob ID while preserving an authorised incident parameter | request | server resolves authoritative object scope and denies cross-scope object | AC-64 / security floor |
| T-156 | blob partial failure | large evidence upload disconnects or exceeds limit before completion | ingest aborts | no usable authoritative blob/evidence record is visible; staged data is not treated as evidence | hostile blob boundary |
| T-157 | safe inline media | active/mislabelled/ambiguous fixture claims passive type | request inline view | fails closed to safe download/refusal; no active execution | AC-80 / final media policy |
| T-158 | operational phase | confirmed stand-down then PIR then closure prerequisites | project incident | phase moves through review/closure-ready while persisted lifecycle remains live until confirmed close | AC-45..47 |
| T-159 | semantic UX | action/fact/evidence/communication operations share backend envelope | visual/product review | each retains dedicated terminology/forms/views; no generic Item CRUD surface in primary workflows | product-quality guardrail |
| T-160 | RLS pool isolation | RLS retained after spike and pooled connection serves different scoped users sequentially | transactions reuse connection | transaction-local scope cannot leak; deliberate cross-scope query is denied | security floor RLS gate |
| T-161 | RLS removal alternative | RLS spike is rejected with evidence | run cross-scope repository/authorisation suite | all application negative tests remain; recorded decision explains why RLS increased ambiguity/risk | security floor RLS gate |
| T-162 | Custodian recovery | normal Custodian quorum unavailable | local recovery attempted with fewer than two recovery proofs or one human | refused | security floor |
| T-163 | Custodian recovery scope | two valid recovery proofs restore Custodian quorum | recovered Custodian requests incident content without membership | denied; recovery restores admin authority only | security floor |
| T-164 | no cross-incident blob dedupe | identical bytes uploaded to two incidents | store both | separate opaque blob identities exist; retention/access of one does not create relationship to the other | blob boundary |
| T-165 | access expiry live page | external responder has page open when grant expires | next query/download/mutation occurs | current authorisation is re-evaluated and access denied | AC-74/81 |

## 3. UI shell regression additions

Phase 1 human UAT failed AC-43. Slice 15 rebuilds the browser presentation tier on `go-app-shell` (see `UI-SHELL-BUILD-NOTES.md` and `docs/go-app-shell-dependency-decision.md`). The PRD acceptance criteria and every test above are unchanged; existing web tests move with their routes and keep their behavioural/negative assertions. The following tests are added. T-166 to T-186 are automated; T-187 to T-192 are human/visual acceptance recorded in `docs/visual-acceptance.md`.

| ID | Area | Given | When | Then | Source |
|---|---|---|---|---|---|
| T-166 | tier boundary | shell dependency added | architecture import test runs | `go-app-shell` is imported only by `internal/presentation/web`; presentation still imports no data/pgx/security package | EDD 4.2 / 26 |
| T-167 | template trust | web package source | static check runs | exactly one `template.HTML(` conversion exists, in the page builder; no markup is built by string concatenation | EDD 9.12 |
| T-168 | mode marking | authorised user on any incident page | page renders | `LIVE` or `EXERCISE` appears as text in the persistent header context | AC-17 / AC-87 |
| T-169 | exercise marking | exercise incident | any incident page renders | exercise notice stating no real obligations or external notifications appears in the body, independent of colour | AC-17 / UX 17 |
| T-170 | status area | isolate, break-glass, untrusted time, open single-person window, pending approvals or stale paper pack present | any authenticated page renders | each active state appears as text in the persistent status line within one page refresh; healthy state shows `Ready`; the frame is resolved in one runtime call using scoped projection queries, with no ledger verification, readiness probe or backup hashing per page | AC-55 / AC-16 |
| T-171 | hidden means hidden | user assigned to Incident A only; Incident B has pending approvals | user loads any page, the navigation, status line or Approvals | Incident B's ID, title, compartments and counts never appear | AC-59 / AC-65 / EDD 9.5 |
| T-172 | single decide path | authenticated user | `POST /decide` with an unknown or tampered `kind` | refused before any application service is called; no ledger entry | EDD 6.2 |
| T-173 | single decide path | proposal authored by user X | X confirms through `POST /decide` for every kind | refused outside a permitted single-person window | AC-06 |
| T-174 | single decide path | proposal superseded by a newer version | confirm with the old version through `POST /decide` | refused; exact-version binding preserved | AC-05 / EDD 6.2 |
| T-175 | single decide path | author of any pending proposal | author confirms via `POST /decide` | refused; no runtime single-person window policy exists yet (UI notes gap G-6), so the browser offers no self-approval path; AC-07 self-approval remains covered by ledger verification tests until G-6 is built | AC-06 / AC-07 |
| T-176 | approval surface | pending governed change of each kind | Approvals renders | shows change summary, effective and proposed values, maker, reason/evidence, compartment and consequence where known; record IDs appear only in technical reference | UX 12 / AC-43 |
| T-177 | degradation | JavaScript disabled | bootstrap, sign in, declare, confirm, record fact, add evidence | each core flow completes | AC-01 / AC-87 |
| T-178 | client runtime | any rendered page and the static asset tree | inspected | no `<script>` other than the shell PWA registration script; no CRAB-authored JavaScript file | EDD 15 / 31 |
| T-179 | PWA boundary | generated service worker and responses | inspected and exercised offline | precache contains only shell CSS/JS, CRAB CSS, icon and offline page; no `/incident`, `/evidence`, `/approvals` or `/api` URL is cached; authenticated pages retain `Cache-Control: no-store`; non-GET is never queued | EDD 9.12 / AC-39 |
| T-180 | paper from web | user may read incident but lacks `print` permission | requests Incident Brief or Emergency Pack PDF from the browser | refused before incident content is read; with permission, PDF returns as attachment with `no-store` and last-good derived copy behaviour unchanged | AC-37 / AC-38 / AC-66 |
| T-181 | close-out from web | incident through recovery | stand-down, PIR and closure are proposed and decided through the browser | each uses normal maker-checker; stand-down does not make the incident read-only; closure is refused until prerequisites are met and then makes it read-only | AC-45 / AC-46 / AC-47 |
| T-182 | time input | `datetime-local` values including blank-required, malformed and boundary values | submitted | stored in UTC per the configured display zone or refused with a field error; no silent coercion | AC-15 / AC-16 |
| T-183 | compartment input | user read into GENERAL only | compartment selector renders, then a tampered LEGAL value is submitted | LEGAL is not offered and the tampered request is refused without disclosure | AC-13 / AC-75 / AC-88 |
| T-184 | severity input | severity suggestions rendered | user views a severity form | suggestions come only from confirmed severities in the user's authorised scope; no severity values are hard-coded | PRD principle 4 / AC-65 |
| T-185 | asset budget | built assets | visual preflight runs | `crab.css` ≤ 2 KiB, contains no layout rules, and no remote asset is referenced | EDD 15 / UX 2 |
| T-186 | refusal behaviour | denied, hidden or invalid request through any browser form | submitted | refusal renders inside the application frame with an error summary, input preserved where safe, and no more existence information than the pre-refactor response | AC-64 / AC-65 / UX 22 |
| T-187 | phone capture | 320 px phone browser | record first report with photo, then add evidence to an assigned incident | each completes from a single screen with the form first, full-width inputs and one primary action | AC-68 / UX 21 |
| T-188 | keyboard and zoom | keyboard only at 200% zoom | navigate, approve, record fact, add evidence | all operable with visible focus and no hidden-focus trap in the compact navigation | AC-87 / UX 22 |
| T-189 | colour independence | light, dark and monochrome rendering | review Board, Approvals, Readiness | mode, status, clock, outcome and fact/unknown/hypothesis meaning remain clear from text | AC-87 |
| T-190 | responsive frame | 320, 390, 768, 1024, 1366 and 1920 px | review every screen | phone drawer, tablet rail and desktop rail behave correctly; no page-level horizontal scroll; tables scroll locally | AC-43 |
| T-191 | operating picture comprehension | non-technical executive with a Bafflewick incident | opens the Board | identifies situation, owners, active clocks and pending decisions within 30 seconds | PRD 16 / AC-43 |
| T-192 | approval comprehension | confirmer with a pending severity, communication and access change | opens Approvals | states what each approval changes and its consequence without opening technical reference | UX 12 / AC-43 |

Slice 15 therefore carries **27 additional named tests**. No earlier test is removed or weakened.
