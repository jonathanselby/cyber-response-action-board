# CRAB — UI Shell Build Notes

**Version:** 0.1  
**Date:** 26 October 2026  
**Companion to:** `EDD.md` V0.4 (frozen), `UX-PRINT-BUILD-NOTES.md`, `BUILD-NOTES.md`  
**Tests:** `TEST-ACCEPTANCE-NOTES.md` §3 (T-166 to T-192)  
**Dependency decision:** `docs/go-app-shell-dependency-decision.md`  
**Trigger:** Phase 1 human UAT failed AC-43 (recorded in `docs/visual-acceptance.md`)

These notes are deliberately outside the PRD and EDD. They are detailed implementation guidance for the engineer (human or LLM) rebuilding CRAB's browser presentation tier on `go-app-shell`. They may change as code teaches us things.

They may **not**:

- add, remove or reinterpret a PRD acceptance criterion;
- change an EDD invariant, the security floor, the two write semantics or the tier boundaries;
- delete or weaken any test in `TEST-ACCEPTANCE-NOTES.md`.

Where this work exposes a product or architecture gap, these notes record it in §19 and route it to the governing documents. They do not solve it here.

> **The shell owns presentation. CRAB owns meaning. The browser does the rest.**

---

## 1. Problem

Phase 1 UAT verdict: *the frontend is not up to enterprise standard.* The backend is release-candidate quality. The presentation tier at `494bfd9` is not.

| Defect | Evidence in code | Requirement it fails |
|---|---|---|
| 13 separately inlined full-document templates | `template.Must(...Parse(\`<!doctype html>…\`))` across 8 files | AC-43, UX notes §2 |
| Navigation injected by buffering responses and string-replacing `<main>` | `drop2_navigation.go` | EDD §9.12 (template output encoding), BUILD-NOTES §1.8 |
| 49-line stylesheet; no tablet/desktop layout, no dark mode | `static/app.css` | AC-43, UX notes §2 |
| No persistent status/context area | header = "CRAB" + version only | AC-55, AC-88, UX notes §3 |
| Machine-format inputs | "Collected at (RFC3339)", free-text compartment box | UX notes §1, §22 |
| Approval surface shows record type + hidden IDs | `operator.go` pending list | UX notes §12 ("never Approve Item #123") |
| Paper path absent from web | "use your browser Print command" | AC-37, PRD principle 5 |
| Six separate decide routes for one mechanism | `/declaration/decide`, `/incident/{id}/decide`, `/access/decide`, `/obligations/decide`, `…/communications/decide`, `…/severity/decide` | EDD §6.2 (one maker-checker), BUILD-NOTES §22 ("another approval path") |
| Six optional runtime interfaces found by type assertion | `requireDrop2Runtime[T]` | staged-build scaffolding; product is now whole |
| User-facing refusals as plain-text `http.Error` | `drop2_*` handlers | UX notes §22 ("predictable denied/hidden behaviour") |
| Wired services with no web route | see §6.3 | AC-21, 27, 42, 44–48, 70, 72 web reachability |
| Stale holding page | `templates/index.html` "first executable checkpoint" | AC-43 |

---

## 2. Doctrine for this work

These rules extend BUILD-NOTES §1.

1. **Remove before adding.** Delete CRAB's hand-built chrome; let the released shell provide it.
2. **Use the shell exactly as released.** `go-app-shell v0.1.0`, unmodified, with no fork, vendored patch or upstream change requested by this work. Every need maps to an existing slot (§7).
3. **One of each mechanism:**
   - one page builder;
   - one `template.HTML` conversion;
   - one runtime interface;
   - one approvals surface;
   - one decide route.
4. **Server-rendered multi-page application.** No client state, no list/detail selection state, no CRAB-authored JavaScript. BUILD-NOTES §15 *permits* small JS; this work does not need it, so it does not use it.
5. **Presentation exposes; it never invents.** A screen that needs behaviour no application service provides waits for the product track (§19).
6. **Authority stays in the app tier.** Navigation and frame never make permission decisions; refusals come from app services and render calmly in-frame.
7. **Semantic language.** UX notes §5 verbs only. `RecordType`, `governance`, `record_id`, "item" never appear in primary UX.
8. **Status by text.** Colour reinforces; text carries meaning (AC-87).
9. **Every negative test survives.** Markup assertions are updated; no test is deleted (EDD §29.1).

---

## 3. Conformance map

| Governing text | How this work conforms |
|---|---|
| EDD §4.2 three tiers | shell imported only by `internal/presentation/web` (T-166); no SQL/security imports added |
| EDD §6.2 one maker-checker | one `POST /decide` dispatching to existing per-area app `Decide` (T-172–T-175) |
| EDD §9.5 hidden means hidden | no incident switcher; status counts from scoped queries only (T-171) |
| EDD §9.12 browser security | CSP, `no-store`, same-origin mutation guard, `nosniff`, `Referrer-Policy: same-origin` unchanged; single trust conversion (T-167) |
| EDD §15 presentation | `net/http` + `html/template` + embedded assets; no Node/SPA |
| EDD §26 dependencies | decision record `docs/go-app-shell-dependency-decision.md` |
| EDD §29.1 security regression rule | §17 maps every changed route to its existing negative tests |
| EDD §31 locked removals | no SPA chain, no self-HTTP, no client store |
| UX §3 global shell | §7 frame: identity, incident, mode, phase, compartment, time trust, isolate/break-glass, readiness degradation, pending approvals |
| UX §4 incident board | §9.3 Board section order |
| UX §6 record vs governed | write-kind help text on every form (§11.1) |
| UX §7 facts/unknowns/hypotheses | `FACT` / `UNKNOWN` / `HYPOTHESIS` text badges |
| UX §8 operational phase | phase in `App.Context` on every incident page |
| UX §12 approval UX | §10 approvals template: summary, effective → proposed, maker, reason, compartment, consequence, controls |
| UX §16 handover | Board carries every handover section; *Record handover* stores the immutable report (§9.3) |
| UX §17 exercise | `EXERCISE` text in header + notice on every incident page (T-168, T-169) |
| UX §21 mobile scope | phone-first intake/evidence; full command responsive (T-187) |
| UX §22 accessibility | shell landmarks, skip link, focus, 44px targets; CRAB adds text status (T-188, T-189) |
| UX §25 stop signs | §18 repeats the relevant ones |
| BUILD-NOTES §22 stop signs | no new approval path (one route replaces six), no generic UI |

---

## 4. Target shape

```text
request
  -> handler                parse syntax only
  -> Runtime method         cmd/crab composition -> app service (authorises) -> view
  -> s.page(...)            shell.Page: header, nav, status, actions, body
  -> templates/<screen>     html/template, gas-* classes
  -> shell.Render           go-app-shell v0.1.0
  -> HTML + shell.css + crab.css   (+ shell PWA registration script)
```

### 4.1 Files after the refactor

```text
internal/presentation/web/
  server.go         New(), mux, securityHeaders, asset mounts, PWA routes, /healthz
  page.go           Context, Action, page(), nav table, show(), post(), render()   (~150–200 lines)
  runtime.go        the single Runtime interface + view types it returns
  incident.go       Incidents, Board, Close-out handlers
  evidence.go       Evidence register/detail/capture/handling/attachment
  comms.go          Communications handlers
  access.go         Incident access, access admin, readiness
  system.go         sign-in, setup, intake, approvals, decide, obligation library
  api.go            JSON API (unchanged contract)
  evidence_views.go attachment headers (unchanged)
  templates/*.html  one file per screen body (§13)
  static/crab.css   ≤ 2 KiB
  static/icon.svg   CRAB mark (manifest/offline only)
```

### 4.2 Old file → new home

| Today | After |
|---|---|
| `operator.go` (template + handlers + guards) | `system.go`, `incident.go`; guards → `page.go` `post()` |
| `drop2_navigation.go` | **deleted**; nav table in `page.go` |
| `drop2_registry.go` | **deleted**; `splitCSV`/`parse*` helpers that survive move to `page.go` |
| `drop2_control.go` | **deleted**; Incidents and Board absorb global/incident control summaries |
| `drop2_access.go` | `access.go` |
| `drop2_evidence.go` | `evidence.go` |
| `drop2_obligations.go` | Clocks section of Board (`incident.go`) + obligation library (`system.go`) |
| `drop2_communications.go` | `comms.go` |
| `continuity_handlers.go` / `continuity_template.go` / `continuity_types.go` | intake → `system.go`; scope/severity/handover/resource → Board (`incident.go`); types → `runtime.go` |
| `incident_views.go` (action/timeline panels) | `templates/board.html` sections |
| `static/app.css` | **deleted** → `shell.css` + `crab.css` |
| `templates/index.html` | **deleted**; no-runtime case renders a minimal frame page |

---

## 5. `page.go` specification

### 5.1 Context

`Context` is resolved once per request by the session lookup the runtime already performs. It costs no additional query beyond what authentication already reads (T-170 budget).

```go
type Context struct {
    HumanID   string
    Role      string          // Major | Minor | Observer (application permission)
    Custodian bool
    Status    StatusFlags
    Incident  *IncidentContext // nil outside an incident route or when unauthorised
}

type StatusFlags struct {
    TimeTrust         string // trusted | drifting | untrusted
    Isolate           bool
    BreakGlass        bool
    SinglePersonWindow bool
    PendingApprovals  int    // scoped to what this human may decide
    PackStale         bool
    ReadinessFail     bool
    ModelUnavailable  bool   // optional assistance only
}

type IncidentContext struct {
    ID, Title, Mode, Phase, Compartment string // Mode: live|exercise; Phase: derived (EDD §8)
}
```

`Incident` is populated only by handlers under `/incident/{id}/…`, and only from an authorised `Incident(...)` response. A refused incident leaves it nil, so the frame never names it.

### 5.2 Page builder

```go
type Action struct {
    Label   string
    URL     string // GET link, or form action when Method == "post"
    Method  string // "" (link) | "post"
    Primary bool
}

func (s *Server) page(w http.ResponseWriter, ctx Context, title, summary, tmpl string, data any, actions ...Action)
```

Behaviour:

1. Execute `templates/<tmpl>` with `data` into a `bytes.Buffer`. On error, render the generic failure body; never partial output.
2. Execute the shared `actions` template with `actions` into a buffer.
3. If `ctx.Incident != nil`, prefix the body with the shared `incident-context` template. This holds the exercise notice and `Viewing <COMPARTMENT>` line, so no screen can omit them (T-168, T-169).
4. Convert the buffers to `template.HTML`. **This is the only `template.HTML(` in the package** (T-167).
5. Build `shell.Page` per §7 and call `s.shell.Render(w, page)`.
6. Set `Content-Type: text/html; charset=utf-8`. Security headers come from `securityHeaders`.

### 5.3 GET helper

```go
func (s *Server) show(w http.ResponseWriter, r *http.Request, title, summary, tmpl string,
    load func(ctx Context) (data any, actions []Action, err error))
```

- No session → redirect `/` (sign-in).
- `load` error → `page(...)` with the `refused` template: a calm `gas-notice--danger` with the error text from the app tier and a link back to Incidents. HTTP status is 403 for authorisation refusals and 404 for not-found/hidden. The message must not be more specific than today's responses (T-186).
- Message code in `?m=` → one `gas-notice--success` from a fixed map (§11.4). Unknown codes are ignored.

### 5.4 POST helper

```go
func (s *Server) post(w http.ResponseWriter, r *http.Request, okURL string,
    do func(ctx Context, form url.Values) (msg string, err error),
    redisplay func(ctx Context, form url.Values, err error))
```

1. Require a session; otherwise 401 frame page.
2. `sameOrigin(r)` exactly as today (`Referrer-Policy: same-origin` fix retained).
3. `parseSmallForm` (or the multipart variant for evidence with existing limits).
4. Call `do`.
5. Success → `303 See Other` to `okURL + "?m=" + msg` (Post/Redirect/Get).
6. Error → `redisplay` re-renders the same screen with a `gas-error-summary` (autofocus) and the user's input preserved (T-186). Status 422 for validation, 403 for refusal.

### 5.5 Navigation table

Static data in `page.go`. It is filtered **only** by whether `ctx.Incident != nil`. There is no permission logic.

```go
var responseNav = []navItem{{"Incidents", "/"}, {"Approvals", "/approvals"}, {"First reports", "/intake"}}
var incidentNav = []navItem{{"Board", ""}, {"Evidence", "/evidence"}, {"Communications", "/communications"},
                            {"Access", "/access"}, {"Close-out", "/close"}}           // prefixed /incident/{id}
var systemNav   = []navItem{{"Readiness", "/readiness"}, {"Obligation library", "/obligations"}, {"Access admin", "/access"}}
```

- Group labels: `Response`; `<INC-ID>` (incident ID only, never title, so the rail stays narrow); `System`.
- `Current` is set by the handler passing its nav key, not by path sniffing.
- Sign-in and setup pages pass no navigation.

---

## 6. Runtime seam

### 6.1 One interface

`runtime.go` declares **one** `Runtime` interface. `cmd/crab/operator_runtime.go` (and the `drop2_*` composition files there) implement all of it. `requireDrop2Runtime` and the six optional interfaces are deleted. A missing method is a compile error, not a runtime 501.

The `drop2_*.go` files in `cmd/crab` may be renamed for clarity in the same slice. Their logic is unchanged.

### 6.2 Methods: retained, renamed, merged

| Group | Methods | Notes |
|---|---|---|
| Session | `Bootstrapped`, `Bootstrap`, `Authenticate`, `Logout`, `Context(token, incidentID)` | `Context` replaces `Home` for frame use and returns §5.1 |
| Incidents | `Incidents(token)`, `DeclareIncident(...)` | |
| Board | `Board(token, incidentID, compartment)` | one composed view: COP + scope/severity history + clocks + actions + roles + handover inputs + comms commitments + recent change |
| Facts & knowledge | `RecordFact`, `RecordUncertainty(kind unknown|hypothesis)` | `RecordUncertainty` **newly exposed** (§6.3) |
| Actions | `ProposeAction`, `ProposeActionStatus(actionID, version, status, skipReason)`, `ActivateChecklist` | status change is a new `ProposeAction` version (domain already validates skip reason) |
| Decisions & roles | `ProposeDecision`, `ProposeRoleAssignment` | **newly exposed** |
| Scope & severity | `RecordScopeAssessment`, `ProposeSeverityChange`, `SeverityHistoryValues(token)` | see §19 G-1 |
| Handover & resources | `RecordHandover`, `RecordResourceRequest` | |
| Obligations | `ObligationLibrary`, `ProposeObligationDefinition`, `ProposeObligationTrigger`, `ProposeObligationChange` | |
| Evidence | unchanged set | |
| Communications | unchanged set minus `DecideCommunication` | |
| Access | unchanged set minus `DecideAccess`; plus `ReadableCompartments(token, incidentID)` | |
| Intake | `Intake`, `CaptureFirstReport`, `TriageFirstReport` | |
| Close-out | `CloseOut(token, incidentID)`, `RecordRecoveryUpdate`, `ProposeStandDown`, `GeneratePIRDraft`, `ProposePIR`, `ProposeRetention`, `ProposeClosure` | **newly exposed** `RecoveryService` |
| Paper | `IncidentBriefPDF`, `EmergencyPackPDF`, `PaperTranscription` | **newly exposed** `PrintService` / `RecordPaperTranscription` |
| Guidance | `Retrieve(token, incidentID, question)` | **newly exposed** `RetrievalService`; deterministic only. Model drafting stays out of this refactor (§19 G-3) |
| Approvals | `Approvals(token)`, `Decide(token, kind, incidentID, recordID, version, outcome, reason)` | replaces six decide methods |
| Readiness | `Readiness(token)` | |

### 6.3 Services wired today with no web route

These exist in `internal/app` and are already authorised, but have no browser path:

| App service | Web exposure added | AC made reachable in browser |
|---|---|---|
| `IncidentService.RecordUncertainty` | Board → *Record unknown* / *Add hypothesis* | AC-70 |
| `IncidentService.ProposeDecision` | Board → *Record decision* | AC-05, AC-56 |
| `IncidentService.ProposeOperationalRoleAssignment` | Board → *Assign role* | AC-18, AC-19 |
| `IncidentService.ActivateChecklist` | Board → *Activate checklist* | AC-21 |
| `IncidentService.RecordPaperTranscription` | Close-out → *Transcribe paper record* | AC-42 |
| `RetrievalService.Retrieve` | Board → *Ask the runbooks* | AC-27 |
| `RecoveryService.*` | Close-out | AC-44–48 |
| `PrintService.GenerateIncidentBrief` / `GenerateEmergencyResponsePack` | Board / Close-out downloads | AC-37, AC-38 |

No new application service, domain type, record kind, migration or ledger semantic is introduced.

### 6.4 `Decide` dispatch

`cmd/crab` maps `kind` to the existing per-area app `Decide`. The mapping is a closed table:

| `kind` (form value) | App call | Human label |
|---|---|---|
| `declaration` | `IncidentService.Decide` (declaration) | Incident declaration |
| `incident` | `IncidentService.Decide` (action / decision / fact / role) | the record's semantic label |
| `severity` | `ContinuityService.DecideSeverity` | Severity change |
| `obligation` | `ObligationService.Decide` | Obligation |
| `communication` | `CommunicationService.Decide` | Communication issue / cadence |
| `access` | `AccessService.Decide` | Access change |
| `recovery` | `RecoveryService.Decide` | Stand-down / PIR / retention / closure |

Rules:
- An unknown `kind` is refused before any service call (T-172).
- The presentation tier passes form values only. Each app `Decide` keeps its existing guarantees: it reloads the stored proposal, binds the exact version, refuses maker-as-confirmer and enforces single-person-window policy (T-173–T-175).
- Self-approval reason is passed through when supplied; the app tier decides whether it is permitted.

---

## 7. Frame: shell slot mapping

| Shell field | Content | Rule |
|---|---|---|
| `Config.AssetBase` | `/_app-shell` | mount `shell.AssetsFS()` with `http.FileServer` under `GET /_app-shell/` |
| `Config.AdditionalStylesheets` | `/static/crab.css?v=<build>` | `<build>` = `buildinfo.Version + "-" + Commit + "-" + BuiltAt`, computed once |
| `Config.ThemeColorLight` / `Dark` | `#ffffff` / `#101613` | |
| `Page.DocumentTitle` | `<Title> · <INC-ID> · CRAB` or `<Title> · CRAB` | |
| `App.Name` | `CRAB` | |
| `App.Context` | `LIVE · INC-0042 · Payroll ransomware · Recovery` | built from `ctx.Incident`; empty when nil |
| `App.Status` | status line (§7.1) | |
| `App.UserLabel` | `j.selby · Major` (+ ` · Custodian`) | application permission only; operational roles appear on the Board |
| `Navigation` | §5.5 | |
| `Title` / `Summary` | screen name / purpose line | incident pages: summary ends with `Viewing GENERAL` |
| `Actions` | rendered `actions` template | ≤ 1 primary, ≤ 2 secondary in the header |
| `Workspace` | `{Mode: shell.WorkspaceSingle, Content: body}` | always single |
| `PWA` | `{ManifestURL: "/manifest.webmanifest", RegistrationURL: "/_app-shell/pwa.js", ServiceWorkerURL: "/service-worker.js"}` | on every page |

### 7.1 Status line algorithm

Build a list in this priority order, adding only abnormal states, and join with ` · `:

1. `EXERCISE` (when `ctx.Incident.Mode == exercise`; it is also in Context)
2. `Integrity failed — read only` (when readiness reports ledger integrity failure)
3. `Break-glass active`
4. `Isolate on`
5. `Time unverified` (untrusted) / `Time drifting`
6. `Single-person window open`
7. `N approvals` (N > 0)
8. `Paper pack stale`
9. `Readiness attention`
10. `Assistance unavailable` (model; last, never presented as core health)

If the list is empty, show `Ready`. Truncate after the first four items, followed by `+N`. Full detail lives on the Board's *Resilience* section and on Readiness.

Example: `EXERCISE · Isolate on · Time unverified · 3 approvals`.

### 7.2 Incident context block

The `incident-context` template renders at the top of every incident page body:

```html
{{if eq .Mode "exercise"}}
<p class="gas-notice gas-notice--warning"><strong>EXERCISE</strong> — no real obligations start and no external notifications are sent.</p>
{{end}}
```

The compartment appears in the `Summary` (§7) rather than here, to avoid a second band.

---

## 8. Routes

### 8.1 Browser routes after refactor

| Method & path | Screen / effect | Replaces |
|---|---|---|
| `GET /` | sign-in · setup · Incidents | same |
| `POST /setup` `POST /login` `POST /logout` | unchanged behaviour | same |
| `POST /incident` | propose declaration | same |
| `GET /approvals` | Approvals | new |
| `POST /decide` | single decide | the six `…/decide` routes |
| `GET /intake` `POST /intake` `POST /intake/{reportID}/triage` | First reports | same |
| `GET /incident/{id}` | Board | Board + `/operations` + `/obligations` (incident) + `/control` |
| `POST /incident/{id}/fact` | Record fact | same |
| `POST /incident/{id}/uncertainty` | Record unknown / Add hypothesis | new exposure |
| `POST /incident/{id}/action` | Add action | same |
| `POST /incident/{id}/action/status` | Complete / Skip action (governed version) | new exposure |
| `POST /incident/{id}/checklist` | Activate checklist | new exposure |
| `POST /incident/{id}/decision` | Record decision | new exposure |
| `POST /incident/{id}/role` | Assign role | new exposure |
| `POST /incident/{id}/scope` | Update scope | `/operations/scope` |
| `POST /incident/{id}/severity` | Propose severity change | `/operations/severity` |
| `POST /incident/{id}/handover` | Record handover | `/operations/handover` |
| `POST /incident/{id}/resource` | Record resource request | `/operations/resource` |
| `POST /incident/{id}/obligation/trigger` `…/obligation/change` | clocks | `/obligations/trigger`, `/obligations/change` |
| `GET /incident/{id}/guidance?q=` | Board guidance results (same page, anchor) | new exposure |
| `GET /incident/{id}/brief.pdf` | Incident Brief download | new exposure |
| `GET /incident/{id}/evidence` `POST …/evidence` | Evidence | same |
| `GET /evidence/{eid}` `GET …/attachment` `POST …/handling` | Evidence detail | same |
| `GET /incident/{id}/communications` + existing POSTs except `decide` | Communications | same |
| `GET /incident/{id}/access` + existing POSTs | Access | same |
| `GET /incident/{id}/close` | Close-out | new exposure |
| `POST /incident/{id}/close/{recovery-update,stand-down,pir,retention,closure,transcription}` | Close-out actions | new exposure |
| `GET /incident/{id}/pack.pdf` | Emergency Response Pack download | new exposure |
| `GET /readiness` `GET /access` `GET /obligations` `POST /obligations/definition` | System | same (minus `/access/decide`, `/obligations/decide`) |
| `GET /manifest.webmanifest` `GET /service-worker.js` `GET /offline` `GET /static/icon.svg` | PWA (static, unauthenticated, no incident data) | new |
| `GET /_app-shell/*` | shell assets | new |
| `GET /static/crab.css` | CRAB tokens | replaces `app.css` |

Removed GET routes (`/control`, `/incident/{id}/control`, `/incident/{id}/operations`, `/incident/{id}/obligations`) return `301` to the Board or Incidents for one release, then are deleted.

### 8.2 Unchanged

- `/api/v1/*` JSON contract is unchanged, including `POST /api/v1/incidents/{id}/decisions`. API decide stays as is; consolidating it is out of scope.
- MCP and CLI are unchanged.
- `/healthz` is unchanged.

---

## 9. Screens

All screens are `WorkspaceSingle`. Lists are `gas-table-wrap` tables or `gas-list` links. Detail is an ordinary page. Each spec gives sections, fields, actions and empty states.

### 9.1 Sign-in / setup

- **Setup:** existing two-person bootstrap form. Explanatory text: *"Two different people create the first accounts together. Available only from this host before first use."* Passwords use `minlength=12` and `autocomplete="new-password"`.
- **Sign-in:** Human ID and password, `autocomplete` set.
- No navigation. `App.Status` is empty.

### 9.2 Incidents `/`

- **Title:** Incidents. **Summary:** "Incidents you are assigned to."
- **Primary action:** *Declare incident* (only when the Home view says `CanDeclare`). It opens an anchored section `#declare` on the same page with the form: ID, Mode (Live / Exercise radio with exercise explanation), Title, Summary, Severity (§11.3), and help text: *"Takes effect when another eligible person confirms."*
- **Table:** Incident (title link) · ID · Mode (text badge) · Phase · Severity · Pending approvals (count, scoped).
- **Pending declarations:** one line, *"N declarations awaiting confirmation — review in Approvals"*, linking to Approvals. The decide forms leave this page.
- **Empty state:** `gas-empty` *"You are not assigned to any incident."*

### 9.3 Board `/incident/{id}`

The Board is the common operating picture **and** the live handover (UX §4, §16). Section order:

| # | Section (h2) | Content | Section actions |
|---|---|---|---|
| 1 | **Situation now** | situation statement; severity with *last changed by/at*; impact/scope summary (latest confirmed + pending); derived phase | *Propose severity change* · *Update scope* |
| 2 | **What we know** | three subsections with text badges: `FACT` (statement, source, occurred/flagged/recorded times, `time unverified` where applicable), `UNKNOWN`, `HYPOTHESIS`; hypotheses never render under Facts (AC-70) | *Record fact* · *Record unknown* · *Add hypothesis* |
| 3 | **Who is in charge** | operational roles and deputies (organisation titles) | *Assign role* |
| 4 | **Critical actions** | table: action, owner, status (text), due, `OVERDUE` badge, source ref; checklist steps grouped by activation | *Add action* · *Activate checklist*; per row *Complete* / *Skip with reason* |
| 5 | **Pending decisions** | proposals in this incident the user may decide, summary only, linking to Approvals | — |
| 6 | **Active clocks** | table sorted by deadline: obligation, trigger, deadline, `PROVISIONAL`/`FIRM`, `OVERDUE`, `time unverified` | *Record trigger* |
| 7 | **Communications commitments** | next situation report due; issued in last 24h with outcome; open enquiries count → Communications | — |
| 8 | **Recent change** | last 10 confirmed decisions / material facts / role changes / comms outcomes (timeline subset) | link: full timeline anchor `#timeline` |
| 9 | **Next objectives** | from latest handover record and current open priority actions | *Record handover* |
| 10 | **Resilience** | paper-pack age, readiness warnings, isolate/break-glass, time trust, model assistance (as optional) | *Download Incident Brief (PDF)* |
| 11 | **Guidance** | *Ask the runbooks* form; results with citations or governed refusal text | — |
| 12 | **Timeline** `#timeline` | full timeline with occurrence/flagged/recorded columns | — |
| 13 | **Resource requests** | requests with owner, priority, status, authorising decision | *Record resource request* |

- **Header actions:** primary *Record fact*; secondary *Download Incident Brief* and *Record handover*.
- Section forms render as `<details>` disclosure blocks under each section (no JS), titled with the verb. They are closed by default and open when re-displayed after an error.
- Screen print (§15.2) prints sections 1–10 and hides forms, giving the paper handover.
- **Board density gate:** at 320 px every section header must be reachable within a reasonable scroll, and sections 1, 4, 5 and 6 must be readable without horizontal scroll (tables wrap via `gas-table-wrap`). If UAT finds the Board too long on phones, the fix is section ordering or collapsing, not new pages. New pages need a UAT finding.

### 9.4 Approvals `/approvals`

See §10.

### 9.5 Evidence `/incident/{id}/evidence`

- **Primary action:** *Add evidence* (anchored form `#add`, phone-first per §11.5).
- **Compartment selector** (GET form): `<select>` of `ReadableCompartments`, defaulting to GENERAL.
- **Table:** filename (link to detail), evidence ID, collected, size (human units), detected type, handler, integrity badge (`VERIFIED` / `INTEGRITY FAILURE`).
- **Detail page** `/evidence/{eid}`:
  - `gas-kv` metadata (§13.9 of EDD fields);
  - safe inline preview where the existing policy allows, otherwise *Download original*;
  - handling history table;
  - *Record handling event* form.
- **Empty state:** *"No evidence in this compartment."*

### 9.6 Communications `/incident/{id}/communications`

- **Primary action:** *Draft situation report*. **Secondary:** *Record enquiry*, *Set report cadence*.
- **Sections:**
  - **Drafts:** each draft with *Submit for approval*.
  - **Awaiting approval:** links to Approvals.
  - **Issued:** audience, channel, approver, issued at, per-channel outcome badges `DELIVERED` / `FAILED` / `SUPPRESSED`, plus *Send notification* where permitted.
  - **Enquiries:** source, received, assignee, status.

### 9.7 Access `/incident/{id}/access`

- **Primary action:** *Request access*. **Secondary:** *Designate sponsor*, *Designate controller*, *Read into compartment*.
- **Tables:** members (person, role, effective, expires, status); compartments (name, controller, members).
- Proposals note *"Takes effect when a Custodian confirms."* and link to Approvals.

### 9.8 Close-out `/incident/{id}/close`

Phase-dependent primary action:

| Derived phase | Primary action |
|---|---|
| ACTIVE RESPONSE / RECOVERY | *Record recovery update* (secondary: *Propose stand-down*) |
| STOOD DOWN / REVIEW | *Start PIR*. Generates the PIR draft from `GeneratePIR`, shown as an editable form whose sections follow PRD §12, then *Submit PIR for confirmation* |
| CLOSURE READY | *Propose closure* |
| CLOSED | none; read-only notice; *Download closure material* |

- **Sections:**
  - **Recovery work:** restoration priorities, verification, residual risks.
  - **Stand-down.**
  - **Post-Incident Review.**
  - **Retention & hold:** *Propose retention/hold*.
  - **Paper:** *Download Emergency Response Pack*, *Transcribe paper record*. The transcription form has a "what happened" field, original time, paper time, paper reference and time confidence. Transcription time is set by server.
- **Closure prerequisites** are shown as a checklist with text `MET` / `NOT MET` from `RecoveryState`.

### 9.9 First reports `/intake`

Phone-first, single column at all widths.

1. **Record first report** form first (§11.5): reporter, contact route, received time, description, suspected service/asset, immediate impact, channel, evidence capture inputs (camera photo, audio, video, file).
2. **Awaiting triage** table: received, reporter, summary, status. Each row has a triage form: *Declare incident from this report* / *Associate with incident* (`<select>` of assigned incidents) / *Retain as non-incident* / *Reject as trigger*. A reason is required.

### 9.10 Readiness `/readiness`

- `gas-kv` list, one row per check in EDD §19 order.
- Each row: `PASS` / `WARN` / `FAIL` text badge + human-readable reason + *last checked*.
- No actions. External notification tests remain explicit CLI operations (EDD §19).

### 9.11 Obligation library `/obligations`

- Table of definitions: name, trigger field, duration + unit, timezone, recipient, status, version.
- *Propose obligation* form: duration unit `<select>` (hours / calendar days / business days), timezone `<select>` of IANA zones (business data, governed, required by domain), calendar fields.
- Pending definitions link to Approvals.

### 9.12 Access admin `/access`

- Custodian view of identities, eligibility and designations. Metadata only, never incident content (AC-60).
- Pending changes link to Approvals.

### 9.13 Refused / not found

The `refused` template shows a `gas-notice--danger` with the app-tier message and a link to Incidents. It is rendered in-frame, but without incident context when the incident is not authorised.

---

## 10. Approvals

### 10.1 Screen

- **Title:** Approvals. **Summary:** "Changes waiting for a second person."
- **Grouping:**
  1. *Waiting for you* (decidable by this human);
  2. *Your proposals awaiting others* (read-only, with status).
- **Incident filter:** `?incident=<id>` (GET), with options limited to assigned incidents.
- **Empty state:** *"Nothing is waiting for your decision."*

### 10.2 One approval template (UX §12)

Every pending item renders through `templates/approval.html` with this view type:

```go
type ApprovalView struct {
    Kind          string   // closed set §6.4
    Label         string   // "Severity change", "Communication issue", "Incident access"…
    IncidentID    string   // empty for system-scope (obligation library, access admin)
    IncidentTitle string
    Compartment   string
    Summary       string   // one sentence: what changes
    Effective     []Field  // current effective state (empty for new records)
    Proposed      []Field  // proposed version
    Maker         string
    ProposedAt    string
    Reason        string   // reason / evidence / source
    Consequence   string   // e.g. "Starts a provisional 72h clock", "Grants GENERAL access until 2026-11-02"
    SelfApproval  bool     // window active and policy permits author self-confirmation
    RecordID      string   // technical reference
    Version       string   // exact version binding
    CanDecide     bool
}
type Field struct{ Name, Value string }
```

Rendering:

```text
[Label badge]  Summary                                   INC-0042 · GENERAL
Current  →  Proposed        (two-column gas-kv; single column when Effective is empty)
Proposed by <Maker> at <time> · Reason: <Reason>
Consequence: <Consequence>
[Confirm] [Reject]   reason field (required for Reject; required for self-approval)
<details> Technical reference: record <RecordID> version <Version> </details>
```

- **Single-person window:** when `SelfApproval` is true and the viewer is the maker, show `SELF-APPROVAL` badge text and a required reason field. The app tier decides whether it is permitted (AC-07).
- **Consequence strings** are composed in `cmd/crab` from existing view data (clock definitions, access expiry, closure effects), with no new domain logic. When nothing meaningful is known, render none.
- The form posts `kind`, `incident_id`, `record_id`, `version`, `outcome`, `reason` to `POST /decide`, then redirects back to `/approvals` (or `?incident=`).

### 10.3 Inline approvals elsewhere

Board §5 and Communications/Access "awaiting approval" sections show `Summary` + `Maker` + a link to `/approvals?incident=<id>#<record>`. They do **not** carry decide forms. There is one decision surface.

---

## 11. Forms and inputs

### 11.1 Write-kind help text

| Write kind | Help text under the form title |
|---|---|
| `Record(...)` | *Preserved as supplied. It does not change the incident's effective state.* |
| `Propose(...)` | *Takes effect when another eligible person confirms.* |

### 11.2 Field conventions

- Wrap each field in `<div class="gas-field">` with `<label>`, input and optional `<p class="gas-help">`.
- On error, the field gets `aria-invalid="true"` + `<p class="gas-field-error" id="…-error">` + `aria-describedby`.
- An error summary `<div class="gas-error-summary" role="alert" tabindex="-1" autofocus>` lists field errors as in-page links.
- Mark required fields with `required` and the word "(required)" in the label, not an asterisk alone.
- One `gas-button--primary` per form.

### 11.3 Input replacements

| Today | Build | Server |
|---|---|---|
| `… (RFC3339)` text inputs | `<input type="datetime-local">` labelled `(UTC)` until decision U-2 resolves | parse `2006-01-02T15:04` in the display zone → UTC; out-of-range/blank-required → field error |
| free-text compartment | `<select>` from `ReadableCompartments` | value must be in the returned list, otherwise refuse |
| free-text severity | `<input list="severity-values">` + `<datalist>` from `SeverityHistoryValues` (values previously confirmed in this organisation) | unchanged validation (non-empty). See §19 G-1 |
| priority free text `P1` | `<input list>` with `P1 P2 P3` suggestions | unchanged |
| action owner free text | `<input list>` from incident members (authorised view) | unchanged |
| comma-separated lists (`splitCSV`) | `<textarea>` one per line | split on newline; trim; de-dupe |

### 11.4 Success message codes

A fixed map in `page.go`. These are the only texts a `?m=` can produce:

```text
fact-recorded        "Fact recorded."
uncertainty-recorded "Recorded."
proposed             "Proposed. It takes effect when another eligible person confirms."
decided-confirmed    "Confirmed."
decided-rejected     "Rejected."
evidence-preserved   "Evidence preserved. SHA-256 recorded."
report-recorded      "First report recorded."
handover-recorded    "Handover recorded."
notified             "Notification attempts recorded — see outcomes."
```

### 11.5 Phone-first capture (AC-68)

Applies to First reports and Add evidence:

- the form is the first content;
- inputs are full width (shell default under 768 px);
- one primary button at the end;
- file inputs:
  - `accept="image/*" capture="environment"` — *Take photo*
  - `accept="audio/*" capture` — *Record audio*
  - `accept="video/*" capture` — *Record video*
  - `accept="*/*"` — *Choose file*

Each input is labelled by its verb. The existing `capture="environment"` test assertion is retained. Upload limits and streaming are unchanged.

---

## 12. Theme: `crab.css`

The budget is ≤ 2 KiB uncompressed (T-185). Only tokens and print are allowed. Layout belongs to the shell.

```css
:root {
  --gas-accent: #274b36;      /* Garage green, from current app.css */
  --gas-on-accent: #ffffff;
  --gas-focus: #1f6a3c;
}
@media (prefers-color-scheme: dark) {
  :root { --gas-accent: #7fbf98; --gas-on-accent: #0d1410; --gas-focus: #9fd8b4; }
}
@media print {
  details > summary { display: none; }
  details:not([open]) > *:not(summary) { display: block; }   /* print section content, not forms */
  form, .gas-page-actions { display: none !important; }
  .gas-header { position: static; }
  .gas-header .gas-app-context, .gas-header .gas-status-text { display: inline !important; }
}
```

- Verify the token names against `shell.css` v0.1.0 (`--gas-accent`, `--gas-on-accent`, `--gas-focus` exist).
- Check contrast for both themes against WCAG 2.2 AA during S15.1.
- No `.crab-*` semantic classes. Facts, unknowns, hypotheses, overdue, provisional and outcomes use `gas-badge` + `gas-badge--{info|warning|danger|success}` **with text**.

| Meaning | Badge text | Variant |
|---|---|---|
| fact | `FACT` | info |
| unknown | `UNKNOWN` | warning |
| hypothesis | `HYPOTHESIS` | warning |
| overdue | `OVERDUE` | danger |
| provisional clock | `PROVISIONAL` | warning |
| firm clock | `FIRM` | info |
| delivered / failed / suppressed | `DELIVERED` / `FAILED` / `SUPPRESSED` | success / danger / info |
| readiness | `PASS` / `WARN` / `FAIL` | success / warning / danger |
| mode | `LIVE` / `EXERCISE` | info / warning |
| self-approval | `SELF-APPROVAL` | warning |
| integrity | `VERIFIED` / `INTEGRITY FAILURE` | success / danger |

---

## 13. Templates

### 13.1 Files

```text
templates/
  layout-parts.html   {{define "actions"}} {{define "incident-context"}} {{define "refused"}}
                      {{define "notice"}} {{define "error-summary"}} {{define "field"}}
  signin.html  setup.html  incidents.html  approvals.html  approval.html
  board.html   evidence.html  evidence-detail.html  communications.html
  access.html  close.html  intake.html  readiness.html  obligations.html  access-admin.html
  offline.html (static; rendered at startup into bytes, served by /offline)
```

- Parse once in `New()` with `template.ParseFS(embedded, "templates/*.html")`.
- The `field` template takes `{Name, Label, Type, Value, Help, Error, Required, Options}` to keep field markup uniform without a Go form builder. It is a template, not a builder: the screen template still owns field order and wording.
- **Shell class cheat sheet** (v0.1.0): `gas-panel`, `gas-stack`, `gas-cluster`, `gas-empty`, `gas-notice[--info|success|warning|danger]`, `gas-badge[--…]`, `gas-form`, `gas-field`, `gas-help`, `gas-field-error`, `gas-error-summary`, `gas-list`, `gas-table-wrap`, `gas-kv`, `gas-muted`, `gas-compact-only`, `gas-wide-only`, `gas-button`, `gas-button--primary`.
- Use sections with `<h2>` per Board section; the shell owns the page `<h1>`.

### 13.2 Escaping rules

- All CRAB data enters templates as data. Corpus passages, model output, filenames, evidence descriptions and enquiry text are never converted to `template.HTML`.
- URLs with IDs use `{{.ID | urlquery}}` inside paths, or are built in Go with `url.PathEscape` and passed as `template.URL` only when constructed from escaped IDs. Prefer the template-side escape.
- No inline `style=` or `<script>`. CSP stays `style-src 'self'` with no `unsafe-inline`.

---

## 14. Progressive Web App

The shell provides the mechanics. CRAB supplies config and an offline page.

```go
pwa := shell.PWAConfig{
    Name: "CRAB — Cyber Response Action Board", ShortName: "CRAB",
    StartURL: "/", Scope: "/", Display: "standalone",
    ThemeColor: "#274b36", BackgroundColor: "#ffffff",
    Icons:    []shell.PWAIcon{{Src: "/static/icon.svg", Sizes: "any", Type: "image/svg+xml"}},
    Precache: []string{"/_app-shell/shell.css", "/_app-shell/pwa.js", "/static/crab.css?v=" + build, "/static/icon.svg"},
    OfflineURL:   "/offline",
    CacheVersion: build,
}
manifest, _ := shell.Manifest(pwa)       // computed once at startup
worker, _   := shell.ServiceWorker(pwa)  // computed once at startup
```

- **Routes:** `/manifest.webmanifest` (`application/manifest+json`), `/service-worker.js` (`text/javascript`, `Service-Worker-Allowed` not needed at root scope), `/offline`, `/static/icon.svg`. All are static, require no session and carry no incident data.
- **Always enabled.** Browsers register workers only in secure contexts; CRAB already requires HTTPS off loopback. There is no configuration flag.
- **Never cached:** authenticated pages. `securityHeaders` keeps `Cache-Control: no-store` on all routes. The shell worker answers only exact precache URLs plus offline fallback for failed navigations (T-179).
- **Offline page text** (static):

  > **CRAB cannot be reached.**  
  > Use the most recent printed Incident Brief and Emergency Response Pack.  
  > Record decisions, actions and communications on the blank paper forms in the pack.  
  > When CRAB returns, transcribe them through **Close-out → Transcribe paper record**; original times are preserved.

- **CSP:** unchanged. `default-src 'self'` covers `script-src`, `worker-src` and `manifest-src` for these same-origin URLs.
- **Not used:** no mutation queue, no background sync, no push, no browser storage.

---

## 15. Paper

### 15.1 PDFs

- `GET /incident/{id}/brief.pdf?size=A4|LETTER&mode=colour|mono`
  - Calls `PrintService.GenerateIncidentBrief` through the runtime with the session principal. Separate `print` permission is enforced in the app tier (AC-66).
  - Response: `Content-Type: application/pdf`, `Content-Disposition: attachment; filename="<INC-ID>-incident-brief.pdf"`, `Cache-Control: no-store`.
  - The existing atomic last-good derived copy is still written to `CRAB_EXPORT_DIR` (and secondary) by the service as today.
- `GET /incident/{id}/pack.pdf` works the same way for the Emergency Response Pack.
- **Size/mode** come from a small GET form beside the download link, defaulting to `CRAB_PAGE_SIZE_DEFAULT` and colour.
- **Failure:** in-frame refusal with the app-tier reason. A last-good copy failure surfaces the existing visible error.

### 15.2 Screen print

`crab.css @media print` plus the shell's print reset. The Board prints sections 1–10 with header context (mode, incident, phase) and status line. This is a convenience handover, not an operational artefact. The operational artefact is the PDF.

---

## 16. Security checklist (unchanged floor, re-verified)

- [ ] `securityHeaders` unchanged: CSP, `no-store`, `nosniff`, `DENY`, `Referrer-Policy: same-origin`.
- [ ] Every POST passes `sameOrigin` (existing test for `Origin: null` regression retained).
- [ ] Session cookie attributes unchanged.
- [ ] No `template.HTML(` outside `page.go` (T-167).
- [ ] No `<script>` other than the shell registration script (T-178).
- [ ] Attachment and PDF responses are `attachment` + `no-store`; inline media policy unchanged (T-157 still passes).
- [ ] `/decide` unknown kind refused before service call (T-172).
- [ ] Frame and status never name or count an unauthorised incident (T-171).
- [ ] Service-worker precache contains no authenticated route (T-179).
- [ ] Import gate: shell only in presentation (T-166).

---

## 17. Existing tests that move

Every existing web test is kept. Assertions about markup are updated to the new structure, and assertions about behaviour are unchanged.

| Existing test area | Moves to | Note |
|---|---|---|
| `operator_test.go` setup/login/declare/decide | `system_test.go`, `incident_test.go` | decide assertions now via `POST /decide kind=declaration|incident` |
| `continuity_test.go` incl. `capture="environment"` | `system_test.go` (intake), `incident_test.go` (scope/severity/handover) | severity decide via `/decide kind=severity` |
| `drop2_*` access/obligation/communication decide tests | `system_test.go` | all negative cases preserved against `/decide` |
| `server_test.go` security headers / origin | unchanged | |
| `visual_contract_test.go` | updated to shell classes + T-185/T-187/T-188 | |
| `api_test.go` | unchanged | |
| `cmd/crab` runtime tests | unchanged behaviour; type names may change | |

---

## 18. Stop signs for this work

Stop and return to these notes or the EDD if any of the following appear:

- a change to go-app-shell, a fork or a vendored copy;
- a second page builder, a second `template.HTML` conversion or HTML string concatenation;
- CRAB JavaScript, a client store, `localStorage`, HTMX/Alpine/React or a build step;
- list/detail selection state or a modal system;
- a decide route other than `/decide` (API excepted);
- a navigation entry built from permission guesses or listing other incidents;
- a screen created without a UAT reason (the target is ten screens plus sign-in/setup);
- a new app service, domain type, record kind, migration or deployment setting without going through §19;
- `crab.css` growing layout rules;
- "item", record-type names or raw IDs in primary UX;
- any deleted test.

---

## 19. Gaps found and routing

These are **not** solved in this refactor. They go through the locked process.

| ID | Gap | Governing text | Interim in this refactor | Route |
|---|---|---|---|---|
| G-1 | No governed organisation severity scale; severity is free text | PRD §1 ("organisation-defined values"), AC-71 | `<datalist>` of previously confirmed severities; validation unchanged | product/domain slice: governed setup record + projection; may need TEST additions. No PRD/EDD change needed: the requirement already exists |
| G-2 | No organisation display timezone; times entered as UTC | PRD awareness/time sections; UX §1 | label inputs `(UTC)` | decision U-2 below |
| G-3 | Model drafting not exposed in web | PRD §7 (optional) | none; deterministic retrieval only | follow-on slice after S15 using existing `ModelService`; no new mechanism |
| G-4 | Timeline/evidence-register/review PDFs not in web | AC-41 | Brief + Pack only | follow-on: add downloads using existing `PrintService` methods |
| G-5 | API decide not consolidated | — | unchanged | optional later; API contract stability |

### Open decisions

| ID | Decision | Recommendation |
|---|---|---|
| U-1 | Board length on phones | ship section order §9.3; resolve from S15.3 UAT evidence |
| U-2 | Display timezone | a deployment presentation setting `CRAB_DISPLAY_TIMEZONE` (IANA, default `UTC`) affects parsing/display only, never obligation calculation (which keeps its governed per-definition timezone). Record as a short decision note before S15.2 if accepted |
| U-3 | Redirect period for removed GET routes | one release, then delete |

---

## 20. Slices

These continue BUILD-NOTES §2 as **Slice 15**. Each sub-slice keeps CI green, Bafflewick 8/8 and the visual preflight passing.

### S15.1 — Frame

1. Add `github.com/jonathanselby/go-app-shell v0.1.0` to `go.mod`; `go mod tidy`.
2. Extend `internal/architecture/imports_test.go` with the shell rule (T-166).
3. Write `page.go` (§5), `runtime.go` (single interface; temporarily embedding the old ones is acceptable *within* this sub-slice only) and `templates/layout-parts.html`.
4. Mount `/_app-shell/` and `/static/crab.css`; delete `app.css` and `index.html`.
5. Move sign-in, setup, Incidents and Board (sections 1, 2, 4, 5, 8 from existing data) onto `page()`.
6. Delete `drop2_navigation.go`; remove its middleware from `Handler()`.
7. Add T-166, T-167, T-168, T-169, T-170, T-171.

**Exit:** existing tests green (markup updated); new tests green; manual check at 320/768/1366.

### S15.2 — Screens and one decision path

1. Collapse runtime interfaces into one; delete `requireDrop2Runtime`.
2. Approvals screen + `approval.html` + `POST /decide`; delete the six decide routes; move all decide tests (T-172–T-176).
3. Evidence, Communications, Access, First reports, Readiness, Obligation library, Access admin onto templates.
4. Board sections 3, 6, 7, 9–13; newly exposed services (§6.3) except Close-out.
5. Input replacements (§11.3) (T-182, T-183, T-184).
6. 301s for removed GET routes.

**Exit:** all decide-path negative tests pass through `/decide`; Bafflewick 8/8.

### S15.3 — Close-out, paper, PWA, acceptance

1. Close-out screen + routes (T-181).
2. Brief/Pack PDF downloads (T-180).
3. PWA routes, offline page, icon (T-178, T-179); no-JS core flow check (T-177).
4. Budgets in `scripts/visual-acceptance-preflight.sh` (T-185).
5. Human review per `docs/visual-acceptance.md`, using the human rows T-187–T-192. Record results.
6. Build evidence `docs/build-evidence-0.0.15-dev.md`; CHANGELOG; VERSION `0.0.15-dev`.

**Exit:** human UAT re-run passes AC-43; T-166–T-192 evidenced.

---

## 21. Budgets

| Budget | Ceiling | Evidence |
|---|---|---|
| Required JS / CRAB-authored JS | 0 / 0 | T-177, T-178 |
| `crab.css` | ≤ 2 KiB | T-185 |
| Shell CSS + PWA JS | ≤ 32 KiB (shell gate) | shell CI |
| Extra DB queries per page for frame/status | 0 beyond session lookup | T-170 |
| `template.HTML(` conversions | 1 | T-167 |
| Browser decide routes | 1 | T-172 |
| Screens | 10 + sign-in/setup | review |
| Web package non-test lines | below today's ~2,430 after S15.3 | build evidence |
| Remote assets | 0 | existing preflight |

---

## 22. Implementation record (0.0.15-dev)

Slice 15 was built in one pass. What the code taught us is recorded here, as these notes require.

### Decisions taken during build

- **U-2 display timezone:** no new deployment setting was added. Every browser time is shown and entered as UTC, labelled on each field. A deployment display zone remains open and needs a decision note before it is added.
- **U-3 redirects:** `/control`, `/incident/{id}/operations`, `/obligations` (incident) and `/incident/{id}/control` return `301` to their new home.
- **Approvals payloads:** one read-only data-tier file (`internal/data/postgres/approval_queries.go`) reads the stored payload of a proposal that an authorised listing has already returned. It also lists pending close-out proposals, which had no reader. It adds no table, migration or state.
- **Paper downloads:** a capture wrapper around the existing artifact store returns the same bytes the atomic last-good file receives, so the browser download and the derived copy are identical.
- **`crab.css`:** 1,046 bytes. It holds tokens, print rules and a few native-element rules the shell does not cover: `details/summary`, `fieldset` and form-cluster alignment. These are **upstream candidates for go-app-shell v0.2**; CRAB does not fork or patch the shell.
- **Frame cost:** the status line is resolved in one runtime call, using the isolate flag, the session break-glass flag, the integrity flag, a paper-pack directory check and the scoped approvals listing. It does no ledger verification, readiness probe or backup hashing per page (T-170 wording corrected accordingly).

### Gaps found (routed, not solved here)

| ID | Gap | Interim |
|---|---|---|
| G-6 | No runtime single-person window policy exists. Self-approval is only modelled in the ledger verifier. | No self-approval UI; `/decide` refuses maker-as-confirmer (T-175 corrected). |
| G-7 | No governed store of checklist templates. `ActivateChecklist` needs a template the caller supplies. | Checklist steps display on the Board; activation is not exposed. |
| G-8 | No browser surface for governed corpus admission (runbook upload). | *Ask the runbooks* searches whatever corpus already exists and refuses honestly when none matches. |
| G-9 | No live time-trust signal; readiness reports the host clock as `warn`. | `TIME UNVERIFIED` shows wherever a record carries non-trusted time; the status line shows time only when a signal exists. |

### Not yet evidenced

- PostgreSQL integration tests and Bafflewick 8/8 run in GitHub Actions only; this build environment had no PostgreSQL server.
- Human review T-187 to T-192 is the Phase 2 UAT.
