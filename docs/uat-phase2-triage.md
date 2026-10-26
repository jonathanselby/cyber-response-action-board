# CRAB — Phase 2 UAT triage and proposed plan

**Date:** 26 October 2026  
**Baseline:** `78bfd4a`, 0.0.15-dev  
**Inputs:** UAT feedback `20261028`; *CRAB Visual Design Handover — Storm Blue Locked* v2; UI design guide; handover guide; Command Summary mockup; logo  
**Method:** each finding is checked against PRD V0.2, EDD V0.4, `UI-SHELL-BUILD-NOTES.md` (§19, §22), `TEST-ACCEPTANCE-NOTES.md` and the code at `78bfd4a`, then reviewed with the constraint review (`engineering-under-constraints/principles/constraint-review.md`).  
**Status:** proposal. No code changes until approved.

> **The shell owns presentation. CRAB owns meaning. The browser does the rest.**

---

## 1. Summary

The feedback is two kinds of work:

1. **Presentation** — Storm Blue, logo, Command Summary home, PDF preview, print polish, password rule. These stay inside the presentation tier, the PDF renderer and one authn function.
2. **Product gaps** — six capabilities the PRD already requires and the build has not yet delivered. Four were already routed as G-1, G-6, G-7 and G-9. None needs a PRD or EDD change.

The design handover is compatible with the shell as released. Every colour maps to an existing `--gas-*` token. Three things in the reference images do not fit the current contract and are removed or deferred: the Inter web font, navigation/panel icons and the logo in the shell header (§3.1).

Four findings in the reference material need a decision before build (§6): dark-mode tokens, two contrast failures in the locked palette, a severity vocabulary that collides with CRAB role names, and the logo source file.

Proposed delivery is five slices, smallest risk first (§5). The single-person window goes last because it changes the core control.

---

## 2. Code facts that shaped the triage

| Fact | Where | Consequence |
|---|---|---|
| The 12-character rule exists only in the web handler; authn has no password policy | `presentation/web/system.go:132` | Policy belongs in `authn`, called by every human-chosen password path |
| `authn.NewPasswordRecord` is also used for generated break-glass keys | `security/authn/breakglass.go:16` | Composition rule must not go inside `NewPasswordRecord` |
| `TimeStatus: "trusted"` is hard-coded in 12 places across domain and data tiers | `domain/semantic.go`, `continuity.go`, `obligation.go`, `data/postgres/*`, `ceremony.go` | Item 1 is real. The fix touches constructor signatures, not just one call |
| Every write already receives `now time.Time` from composition; about 60 `time.Now()` calls, mostly in `cmd/crab` | `cmd/crab/web_runtime.go` and others | One clock at composition is a natural seam |
| Readiness reports host time as a fixed `warn` | `cmd/crab/readiness_runtime.go:41` | No live time signal exists (G-9) |
| Emergency Response Pack is three hard-coded paragraphs plus blank forms, titled `Incident <id>` | `cmd/crab/web_runtime.go:446` | Item 2 is real; the organisation name is also missing |
| Incident Brief is generated with `contacts = nil` | `cmd/crab/web_runtime.go:440` | Brief contact section is always empty today |
| Ledger already validates `SelfApproval` + reason; no runtime window policy | `ledger/ledger.go:146`; G-6 | Item 4 needs policy and UI, not a new record shape |
| `ActivateChecklist` and template validation exist; no template store | `app/incident.go:100`; `domain/action.go`; G-7 | Item 6 is a store plus a screen |
| No incident relationship code anywhere | grep | Item 3 is a new governed record kind |
| Severity is free text with a datalist of previous values | §11.3; G-1 | Item 5 replaces the datalist with a governed list |
| Shell header renders `App.Name` as text only; no logo or icon slot | `go-app-shell` `shell.html` | Logo cannot go in the header without a shell change |
| T-185 forbids `url(` and `width:` in `crab.css`; budget 2 KiB, currently 1,046 bytes | `visual_contract_test.go` | Logo-by-CSS and web fonts are excluded; token-only restyle fits |
| CSP is `default-src 'self'; img-src 'self'; object-src 'none'; frame-ancestors 'none'` | `server.go:146` | PDF preview must open as a top-level tab, not an embedded frame |
| PDF renderer uses Go Regular only and a green accent `(28,92,64)` | `presentation/pdf/renderer.go` | Print restyle: Storm Blue + Go Bold (same `x/image` module, no new dependency) |
| `IncidentLink` carries ID, mode, title, severity, phase, pending count — no affected service or last-updated time | `presentation/web/runtime.go:139` | Command Summary shows what exists; it does not invent fields |

---

## 3. Triage

Categories follow the constraint review: **KEEP** (accept as asked), **SIMPLIFY** (accept with a smaller mechanism), **REMOVE** (not justified, or not compatible with the locked contract), **TEST** (assumption not yet demonstrated).

### 3.1 UX overhaul — Storm Blue

| Element | Verdict | Reason |
|---|---|---|
| Locked palette as primary UI | **KEEP** | Maps one-to-one to shell tokens: Storm Blue → `--gas-accent`/`--gas-focus`/`--gas-info`; Deep Ink → `--gas-text`; Slate → `--gas-text-muted`; Mist → `--gas-border`; Paper → `--gas-surface`; white → `--gas-surface-raised`; green/amber/red → `--gas-success/warning/danger`. Token-only change in `crab.css`. |
| Green demoted to status only | **KEEP** | Removes `#274b36` from `crab.css`, PWA theme colour, icon and PDF renderer. |
| Four screen patterns | **KEEP** | Mapping: Entry = sign-in, setup; Workspace = Command Summary, Approvals, First reports list, Readiness, Obligation library, Access admin; Capture = first-report form, add evidence; Review = Board, evidence detail, Close-out. No new pattern needed. |
| Command Summary as home | **SIMPLIFY** | Replace the Incidents page at `/` rather than add a screen. Keeps the 10-screen budget (§21) and the stop sign on screens without UAT reason. See §3.3. |
| Logo in header | **SIMPLIFY** | Shell has no logo slot and T-185 forbids `url(` in `crab.css`. Show the wordmark as text in the header now; show the logo on sign-in/setup (template `<img>`, `img-src 'self'`), PWA icon, favicon, offline page and every PDF. Request an `AppHeader` icon slot in go-app-shell v0.2 through its release process; adopt when released. |
| Inter font | **REMOVE** | Needs `@font-face` with `url(` (T-185), an embedded font asset and a dependency decision. Shell already uses `system-ui`, which is close in tone. PDF uses embedded Go Regular + Go Bold. |
| Icons in navigation and *Needs attention* panel | **REMOVE** | Shell nav has no icon slot; icons would need `url(` or inline SVG per item. The handover says "pretty without decoration". Text, counts and badges carry the meaning. |
| Coloured dot inside status chips | **REMOVE** | Needs `width:`/`height:` pseudo-elements (T-185). Badge tint plus text is enough and is colour-independent (AC-87). |
| Two-column summary layout | **SIMPLIFY** | Grid rules are layout and belong to the shell (T-185). Single column: *Needs attention* first, then incidents. Two-column is a shell v0.2 candidate if UAT still wants it. |
| Locked palette contrast | **TEST** | Computed WCAG ratios: Slate on Paper 4.28 (fails AA 4.5 for body text), on white 4.59 (passes). Amber as text 2.6–2.8 (fails everywhere). Mist as input border 1.1 against Paper (fails 3:1 non-text). **Decided (U-5):** fix as recommended, keeping visual continuity with the handover. |
| Dark mode | **TEST** | Handover gives light values only; shell and T-189/T-190 require dark. Needs derived Storm Blue dark tokens. **Decided (U-4):** dark mode kept; derived tokens to be shown for approval in S17. |
| `crab.css` budget | **TEST** | Light + dark tokens + badge tints + print should fit 2 KiB. Measure; do not raise the budget without evidence. |
| Print restyle | **KEEP** | PDF renderer: Storm Blue headings, Go Bold hierarchy, compact metadata bar, Mist table rules, logo mark, no colour-only meaning. Mono mode unchanged. Screen print CSS recoloured. |
| Reference file names | **SIMPLIFY** | Handover names three PNGs that differ from the supplied files. Commit the references once under `docs/design/` with the handover's names so later readers find them. |

### 3.2 Password composition

| Element | Verdict | Reason |
|---|---|---|
| Require upper, lower and special character, minimum 12 | **KEEP** | UAT requirement. One `authn.ValidateNewPassword` used by web setup and the CLI/operator setup path. Break-glass generated keys keep their own rule. |
| Web field hint | **KEEP** | Label states the rule; server is the authority; error renders in-frame (T-186). |
| Composition rules versus NIST SP 800-63B | **TEST** | Current NIST guidance discourages composition rules in favour of length and breached-password checks. Record the UAT decision in the build notes; do not add a breached-password list (would need a data file and update process). Decision U-11 on whether a digit is also required. |

### 3.3 Command Summary (home page after sign-in)

| Element | Verdict | Reason |
|---|---|---|
| Page at `/` titled *Command Summary*, summary *What needs your attention now.* | **KEEP** | Workspace pattern. Replaces the Incidents page content. |
| *Declare incident* as the single primary action | **KEEP** | Existing behaviour, shown only when `CanDeclare`. |
| *Needs attention*: approvals waiting, first reports awaiting triage, paper pack stale | **KEEP** | All three already resolved in the one `Frame` runtime call or the Home view. No extra queries (T-170 budget). |
| *My active incidents* list with ID, title, severity + phase as text, pending count, *Open* | **SIMPLIFY** | Use existing `IncidentLink` fields. *Affected service* and *Updated N min ago* are not in the view; adding them needs new projection reads. Omit until a UAT finding requires them. |
| *View all incidents* link | **REMOVE** | The list already shows every incident the user may see; there is no separate list to link to. |
| "Only items you are authorised to see are shown." | **KEEP** | Footnote text. Behaviour already guaranteed by T-171. |
| Permission-scoped counts | **TEST** | First-report count must come from the intake permission, not a global count. New test row. |
| Header label *Joe Tester · Major · Custodian* | **KEEP** | This is the application permission label (§7). It is correct as designed. |

### 3.4 Print preview

| Element | Verdict | Reason |
|---|---|---|
| See the finished report as a PDF before printing | **SIMPLIFY** | Add *Preview* beside each download: same route, same authorisation, `Content-Disposition: inline`, opens in a new tab in the browser's PDF viewer. No HTML-to-PDF, no headless browser (rejected in `pdf-renderer-spike-decision.md`), no embedded frame (`frame-ancestors 'none'` stays). |
| Download stays attachment | **KEEP** | T-180 unchanged. Inline applies only to CRAB-generated PDFs; evidence attachments keep T-157 behaviour. |
| Browser PDF viewer under CRAB's CSP | **TEST** | `object-src 'none'` on the PDF response may block some built-in viewers. Test Chrome, Firefox and Safari. If blocked, the PDF response gets PDF-appropriate headers (no HTML CSP needed on a non-HTML body; `nosniff`, `no-store`, `X-Frame-Options: DENY` kept). Record the result before changing any header. |
| Preview for the organisation-level Emergency Response Pack | **KEEP** | Today the pack is only reachable per incident. Expose it from Readiness, which is where *Paper pack stale* points. |

### 3.5 Item 1 — Time authority

| Element | Verdict | Reason |
|---|---|---|
| One clock returning time plus `trusted / drifting / untrusted` | **KEEP** | Small interface in composition; passed into app services in place of bare `time.Time`. |
| Host synchronisation check | **SIMPLIFY** | Linux `adjtimex` read (standard library `syscall`, no new dependency): unsynchronised or error → `untrusted`; synchronised with estimated error above a threshold → `drifting`; otherwise `trusted`. Non-Linux → `untrusted`. Cached for a few seconds. No NTP client. |
| Remove the 12 hard-coded `"trusted"` literals | **KEEP** | Constructors take the status. Static test forbids the literal outside the clock and tests. |
| Mark timestamps and deadlines TIME UNVERIFIED | **KEEP** | Board, clocks, Approvals, Command Summary, status line, Brief, Pack, timeline export. Obligation deadlines inherit the weaker of trigger-time and computation-time status. |
| Readiness reports the live clock state | **KEEP** | Replaces the fixed `warn`. Closes G-9. |
| Pi without RTC or network | **TEST** | Expect `untrusted` until sync. Verify on the Pi acceptance run. |

### 3.6 Item 2 — Organisation emergency configuration

| Element | Verdict | Reason |
|---|---|---|
| One governed organisation record for emergency operating information | **KEEP** | PRD §2 already requires governed contacts, advisers, insurers, regulators, escalation and role cards. |
| Content limited to: organisation name; contacts with primary and alternative routes; external parties (insurer, regulator, counsel, advisers) using the same contact shape; escalation route as ordered lines; role cards (title, responsibilities, deputy); up to N short procedures as text | **SIMPLIFY** | Fixed shape, no free schema, no attachments, no asset register. Not a CMDB or document store. |
| Populate Incident Brief (key contacts) and Emergency Response Pack | **KEEP** | Replaces the hard-coded pack sections and the `nil` contacts. |
| Changes via Propose → Decide | **KEEP** | Same `/decide` route, new `kind`. |
| Pack becomes stale when configuration changes after the last pack | **KEEP** | Feeds the existing stale signal. |
| Contact data is personal data | **TEST** | Reading the configuration needs a defined permission. Printing still needs `print` (AC-66). Decide who can read it in the browser. |
| Out-of-band verification routes | **REMOVE** from this record | AC-12 requires pre-registered verification material. Emergency contacts must not become a verification route. Keep them separate. |

### 3.7 Item 3 — Related incidents

| Element | Verdict | Reason |
|---|---|---|
| Governed assertion *A is related to B*, with reason and time | **KEEP** | AC-84, EDD §13.14, D-66. |
| Proposer and confirmer must each be authorised for both incidents | **KEEP** | Prevents using a link to learn that a hidden incident exists. |
| A viewer sees the link only if they can see both incidents | **KEEP** | Hidden means hidden (T-171). No "related to an incident you cannot view" placeholder; that would leak existence. |
| No merge, no access grant, no shared timeline, no compartment inheritance | **KEEP** | EDD §13.14. |
| Unlinking is a new governed version | **KEEP** | Nothing is erased. |
| Graph view, automatic correlation | **REMOVE** | Not requested and not justified. |

### 3.8 Item 4 — Single-person emergency approval window

| Element | Verdict | Reason |
|---|---|---|
| Time-bounded window naming start, end, permitted people and covered decision kinds | **KEEP** | PRD single-person windows, AC-07, D-10, G-6. |
| Opening, extending or closing early is itself dual-controlled | **KEEP** | The window is agreed by two people in advance so one can act later. |
| Excluded kinds: access grants (EDD §6), organisation configuration, severity scale, checklist templates, the window itself, closure and retention | **KEEP** | EDD forbids silent application to access grants; the rest prevent a lone user changing policy. Decision U-9 confirms the list. |
| Self-approval needs a non-blank reason; flagged `SELF-APPROVAL` on Approvals, Board, timeline, Brief and Command Summary | **KEEP** | AC-07 single-approval dashboard. Ledger already carries the fields. |
| Maximum window length | **TEST** | Needs a governed ceiling. Decision U-9. |
| Window bounds while time is unverified | **TEST** | A wrong clock could extend a window. Recommendation: allow but flag both markers. Decision U-10. |
| Status line *Single-person window open* | **KEEP** | Already in the §7.1 algorithm. |
| T-175 | **KEEP** | Unchanged outside a window. New rows cover inside-window behaviour. |

### 3.9 Item 5 — Governed severity values

| Element | Verdict | Reason |
|---|---|---|
| Organisation-defined small ordered scale | **KEEP** | PRD classification rule, AC-71, G-1. |
| Changes governed | **KEEP** | Part of the organisation configuration family. |
| Responders select; server refuses values outside the scale | **KEEP** | `<select>` replaces the datalist. |
| Existing free-text severities | **KEEP** as history | Ledger history is not rewritten. Existing incidents show their stored value until a governed severity change selects a scale value. |
| Define during first-run setup | **SIMPLIFY** | Setup already has two people present. Offer the scale on the setup form as the first confirmed configuration; later changes go through governance. Decision U-7. |
| Example values *Major / Moderate / Minor* | **REMOVE** | They collide with CRAB application roles *Major / Minor*. **Decided (U-8):** the default scale is *Severity level 1* (least severe) to *Severity level 5* (most severe), labels only, no monetary thresholds. |
| Default scale is a seed, not hard-coded | **KEEP** | PRD principle 4 forbids hard-coded classification values. Setup offers Severity level 1–5 pre-filled; the two setup people confirm it; later changes are governed. Level descriptions are optional organisation text. |
| Fit of the source scale | **TEST** | The levels follow the UK Cyber Monitoring Centre's 1–5 scale, which scores systemic events across many organisations. Inside one organisation most incidents would sit at level 1 unless the organisation writes its own level descriptions. Review at UAT. |
| T-184 | **KEEP** | Still true. A new row adds that values come only from the governed scale. |

### 3.10 Item 6 — Governed checklist templates

| Element | Verdict | Reason |
|---|---|---|
| Library of approved templates | **KEEP** | G-7. Domain validation already exists. |
| Template: name, source reference, ordered steps with title, priority, optional default role, due offset | **KEEP** | Matches existing `ChecklistTemplate`. |
| *Activate checklist* proposes actions through maker-checker | **KEEP** | Existing `ActivateChecklist`. Exposes the Board action held back in §22. |
| Branching, automation, designers | **REMOVE** | Explicitly excluded. |
| Human load: ten steps create ten approvals | **TEST** | Approvals should group proposals from one activation so a confirmer can review them together, each still decided individually. Do not add batch-confirm without a decision. |

---

## 4. Constraint review notes

**Mission.** Let a small team run a cyber incident when normal systems are degraded, and leave a record that holds up later.

**State added by this plan.** Organisation configuration (severity scale, emergency information, checklist templates), single-person windows, incident relationships. All are ledger records with rebuildable projections. No new store.

**Interfaces added.** Host clock (read-only kernel call). Nothing else crosses a process boundary.

**Degraded operation.** Unverified time keeps CRAB running and marks its claims. Emergency configuration makes the printed pack useful with CRAB down. The single-person window keeps decisions moving when only one Major is available.

**Environment test.** The assumption most likely to fail is the host clock. It is now tested rather than asserted.

**Removal test.** Inter, icons, chip dots, a second home screen, a *View all* link, a graph engine, batch confirmation, NTP client and an HTML-to-PDF path are all removed or not built. Each removal leaves function intact.

**Change test.** A different shell version can supply the header logo and two-column layout later without CRAB changes beyond configuration.

| KEEP | SIMPLIFY | REMOVE | TEST |
|---|---|---|---|
| Storm Blue tokens; green to status only; four patterns; print restyle; password rule; Command Summary content; one clock; TIME UNVERIFIED marking; emergency configuration; related incidents; single-person window; severity scale; checklist templates | Command Summary replaces Incidents; logo outside header until shell v0.2; single-column summary; PDF preview as inline tab; adjtimex-only time check; fixed-shape emergency record; severity scale at setup | Inter; nav and panel icons; chip dots; *View all incidents*; affected-service and last-updated fields; verification routes in emergency contacts; correlation/graph; branching checklists | palette contrast; dark tokens; `crab.css` budget; PDF viewer under CSP; scoped first-report count; Pi clock; contact-data permission; window ceiling; window under unverified time; severity vocabulary; checklist approval load; NIST note |

---

## 5. Proposed slices

Each slice keeps CI green, Bafflewick 8/8 and the visual preflight passing. Each ends with build evidence and a CHANGELOG entry. Version numbers are proposals.

### S16 — Quick corrections (0.0.16-dev)

1. `authn.ValidateNewPassword`; web setup and operator setup call it; label text updated.
2. Clock: interface, Linux implementation, composition wiring; remove hard-coded `"trusted"`; readiness uses the live state; TIME UNVERIFIED rendering on screen and in PDFs.
3. PDF *Preview* links (inline) beside each download; organisation-level pack from Readiness.

**Exit:** T-193 to T-197 green. CSP result recorded.

### S17 — Storm Blue presentation (0.0.17-dev)

1. U-4, U-5, U-6 and U-8 are decided. Confirm U-3 before starting.
2. `crab.css` tokens light and dark; contrast test.
3. Logo: SVG icon and lockup under `static/`; PWA icon, favicon, theme colour; sign-in/setup and offline page.
4. Command Summary replaces the Incidents page at `/`.
5. Pattern pass across the ten screens: spacing, headings, one primary action, empty states, helper text. Template and token work only.
6. PDF renderer restyle: Storm Blue, Go Bold, metadata bar, logo mark; deterministic-bytes test updated.
7. Commit design references under `docs/design/`.
8. Upstream request to go-app-shell for v0.2: header icon slot; native `details`/`fieldset` rules (§22 candidates); optional two-column overview. Not a dependency of this slice.

**Exit:** T-198 to T-201 green; human re-run of T-187 to T-192 plus T-211 to T-213; AC-43 signed off in `docs/visual-acceptance.md`.

### S18 — Organisation configuration (0.0.18-dev)

1. One organisation configuration record family; one *Organisation* screen under System (Workspace pattern), one new `/decide` kind.
2. Severity scale (setup form + governed change); severity `<select>` everywhere.
3. Emergency information; Incident Brief contacts; Emergency Response Pack sections; stale-on-change.

**Exit:** T-202 to T-204 green. Screen budget becomes 11 (UAT reason recorded).

### S19 — Related incidents and checklist templates (0.0.19-dev)

1. Relationship record, projection, Board section, `/decide` kind.
2. Checklist templates in the Organisation screen; *Activate checklist* on the Board; activation grouping on Approvals.

**Exit:** T-205 to T-207 green.

### S20 — Single-person window (0.0.20-dev)

1. Window record (governed), policy check inside each app `Decide`, excluded kinds.
2. Self-approval form field (reason) shown only when the policy permits; `SELF-APPROVAL` badges; status line.

**Exit:** T-208 to T-210 green; Bafflewick extended with one single-person scenario; G-6 closed.

**Why this order.** S16 is small and fixes claims CRAB makes today. S17 is what UAT is waiting to see. S18 provides the governed-configuration path S19 and S20 reuse. S20 depends on the clock (S16) and changes the core control, so it goes last with the most tests around it.

---

## 6. Open decisions

| ID | Decision | Recommendation |
|---|---|---|
| U-3 | Phase 2 scores for T-187 to T-192 | Recorded as *not scored*; confirm, or supply individual results |
| U-4 | Dark mode | **Decided 26 Oct:** keep dark mode. Derive Storm Blue dark tokens (dark slate surfaces, lightened Storm Blue accent) and show them for approval in S17 |
| U-5 | Contrast failures in the locked palette | **Decided 26 Oct:** as recommended, keeping visual continuity. Text sits on white panels, Paper is page background only (Slate 4.59 passes). Amber is a fill/border colour only; warning text uses Deep Ink. Input borders use Slate, dividers use Mist |
| U-6 | Logo source | **Done 26 Oct:** single-crab assets traced from the supplied PNG in `docs/design/logo/` — lockup PNG (light and dark), icon PNG at 512/192/32, app icon on Paper, icon SVG. Wired into the app in S17 |
| U-7 | When the severity scale is defined | Offer it on the setup form (two people present); changes later through governance. Declaration requires a scale to exist |
| U-8 | Severity vocabulary | **Decided 26 Oct:** default scale *Severity level 1* (least severe) to *Severity level 5* (most severe); monetary thresholds ignored. Offered pre-filled at setup and governed thereafter, not hard-coded |
| U-9 | Single-person window limits | Governed maximum length (suggest 24 h); excluded kinds as §3.8; only Majors may be named |
| U-10 | Self-approval while time is unverified | Allow and flag both markers; refusing would stop response exactly when infrastructure is degraded |
| U-11 | Password rule detail | Minimum 12; at least one upper, one lower and one non-alphanumeric. Digit not required unless UAT wants it |
| U-2 | Display timezone (still open from §19) | Unchanged; UAT did not raise it. Times remain UTC and labelled |

---

## 7. Proposed test rows

Appended to `TEST-ACCEPTANCE-NOTES.md` §3 (or a new §4) when the plan is approved. No existing test is removed or weakened.

| ID | Area | Then |
|---|---|---|
| T-193 | password policy | Setup refuses a password missing upper, lower or non-alphanumeric, or under 12, with an in-frame field error, via web and operator setup; break-glass generated keys unaffected |
| T-194 | time authority | No `TimeStatus: "trusted"` literal exists outside the clock package and tests; every write takes its status from the clock |
| T-195 | time authority | With an unsynchronised clock, new records, clocks, Approvals, status line, Brief and Pack show `TIME UNVERIFIED`; operation continues |
| T-196 | time authority | With a synchronised clock within threshold, records are `trusted`; above threshold, `drifting`; readiness reflects the live state |
| T-197 | print preview | *Preview* uses the same authorisation as download, returns `inline` with `no-store`; downloads stay `attachment` (T-180); evidence attachments unchanged (T-157) |
| T-198 | Command Summary | Counts and incidents shown are only those the user is authorised to see, including the first-report count |
| T-199 | Command Summary | Rendering adds no queries beyond the frame call and the Home view |
| T-200 | palette | `crab.css` uses the locked values; green/amber/red appear only in status tokens; text tokens meet WCAG AA on their surfaces in light and dark |
| T-201 | print style | Restyled PDFs remain deterministic, embed the logo locally, keep monochrome meaning and fetch nothing at runtime |
| T-202 | severity scale | Scale changes need a different confirmer; declarations and severity changes refuse values outside the scale; earlier free-text values still display |
| T-203 | emergency configuration | Changes are governed; Brief and Pack contain the confirmed contacts, parties, escalation and role cards; reading needs the defined permission; printing still needs `print` |
| T-204 | emergency configuration | A confirmed configuration change marks the pack stale until regenerated |
| T-205 | related incidents | Proposing or confirming a link needs authority for both incidents; a user with access to only one sees no trace of the link or its reason |
| T-206 | related incidents | A link grants no access, merges no history and inherits no compartment; unlinking is a new version |
| T-207 | checklist templates | Templates are governed; activation proposes each step through maker-checker with template provenance; Approvals groups them by activation |
| T-208 | single-person window | Opening, extending or closing a window needs a different confirmer; excluded kinds can never be self-approved |
| T-209 | single-person window | Inside an active window a named Major may self-approve a covered kind with a non-blank reason; the result is flagged `SELF-APPROVAL` on Approvals, Board, timeline, Brief and Command Summary. Outside the window, after its end, for an unnamed person or an uncovered kind, it is refused |
| T-210 | single-person window | Window checks use the clock; self-approvals made under unverified time carry both markers |
| T-211 | human: Storm Blue | Every screen follows one of the four patterns and the handover checklist, in light and dark |
| T-212 | human: print | Brief and Pack in colour and monochrome, A4 and Letter, are attractive, readable and unclipped |
| T-213 | human: Command Summary | A responder states what needs attention and opens the right item within 30 seconds of sign-in |
