# CRAB — UAT Phase 2 Build Notes

**Version:** 0.1  
**Date:** 26 October 2026  
**Companion to:** `EDD.md` V0.4 (frozen), `UI-SHELL-BUILD-NOTES.md`, `UX-PRINT-BUILD-NOTES.md`, `BUILD-NOTES.md`  
**Triage:** `docs/uat-phase2-triage.md`  
**Tests:** `TEST-ACCEPTANCE-NOTES.md` §4 (T-193 to T-213)  
**Trigger:** Phase 2 human UAT (`docs/visual-acceptance.md`): functional pass; new locked visual direction and six product gaps

These notes carry the build detail for the Phase 2 UAT response. Like the UI shell notes, they may change as code teaches us things, and they may **not**:

- add, remove or reinterpret a PRD acceptance criterion;
- change an EDD invariant, the security floor, the two write semantics or the tier boundaries;
- delete or weaken any test in `TEST-ACCEPTANCE-NOTES.md`.

Every capability here is already required by PRD V0.2. Nothing here is new product scope.

> **Stop making assertions CRAB cannot prove. Keep the rest as it is.**

---

## 1. Doctrine for this work

1. **One of each mechanism** still holds: one page builder, one decide route, one maker-checker, one clock, one organisation configuration path.
2. **Ledger first.** New state is a ledger record with a rebuildable projection. No new store, no new service, no new dependency.
3. **Presentation exposes; it never invents.** Screens show what application services return.
4. **Hidden means hidden.** Counts, links and relationships never reveal an incident the viewer cannot see.
5. **The shell stays unmodified.** Anything the shell cannot do is either dropped or requested upstream (§3.6).
6. **Seeds, not hard-coding.** Defaults offered at setup are confirmed by people and governed afterwards (PRD principle 4).

---

## 2. Stage 1 — Quick corrections (0.0.16-dev)

### 2.1 Password policy (T-193)

- `authn.ValidateNewPassword(password string) error` is the only rule for human-chosen passwords: at least 12 characters, at least one upper-case letter, one lower-case letter and one character that is neither a letter nor a digit.
- Called by the operator runtime before `NewPasswordRecord` for both setup identities, and by the web setup handler for an in-frame field error.
- Break-glass keys are generated material and keep their own rule.
- The UAT owner chose composition rules; current NIST guidance prefers length plus breached-password screening. That choice is recorded here and in the triage; no breached-password list is added.

### 2.2 Time authority (T-194 to T-196)

**Problem.** Twelve constructors stamped every record `trusted` without evidence.

**Mechanism.**

- `internal/adapters/clock`: `Clock.Read() Reading{Now time.Time; Status string}`.
- The host implementation reads the kernel's NTP discipline with `adjtimex` (standard library `syscall`, read-only, unprivileged). `TIME_ERROR` or `STA_UNSYNC` → `untrusted`. Synchronised with estimated error above 250 ms → `drifting`. Otherwise `trusted`. Non-Linux hosts and any error → `untrusted`. Readings are cached for 5 seconds.
- Domain and data constructors stamp `untrusted` by default: claim nothing.
- The kernel's `Append` is the single write path. It stamps `TimeStatus` from the clock on every record that carries no human cryptographic signature. Human-signed setup records are stamped by their caller from the same clock before signing.
- Decided records are stamped at decision time. The two projection checks that compared a decided record's time status with its stored proposal (access, corpus) compare the binding fields only: version, payload, maker, compartment and mode. Time status is evidence about the write, not part of the proposal.
- Readiness `time_status` reports the live reading. The frame status line carries it (*Time unverified* / *Time drifting*).
- Screens and PDFs show `TIME UNVERIFIED` beside any time whose record is not `trusted`. Obligation clocks inherit the trigger record's status.
- No NTP client. No configuration flag.

### 2.3 PDF preview (T-197)

- Every PDF download gains a *Preview* link: same route, same authorisation, `?view=inline`, opening in a new tab.
- Inline is limited to CRAB-generated PDFs. Evidence attachments keep their own policy (T-157).
- Response headers for the PDF stay `no-store`, `nosniff`, `X-Frame-Options: DENY`. The HTML CSP is not relaxed. Browser viewer behaviour is recorded in build evidence.

### 2.4 Organisation Emergency Response Pack

- Readiness gains *Emergency Response Pack* download and preview at `/pack.pdf`, not tied to an incident. It needs `print` permission and uses the organisation configuration once Stage 3 exists.
- The per-incident route stays for compatibility.

---

## 3. Stage 2 — Storm Blue (0.0.17-dev)

### 3.1 Tokens

`crab.css` maps the locked palette onto shell tokens. No layout rules, no `url(`, ≤ 2 KiB (T-185).

| Token | Light | Dark |
|---|---|---|
| `--gas-accent` | Storm Blue `#334A5E` | `#9FB4C7` |
| `--gas-on-accent` | `#FFFFFF` | `#17212B` |
| `--gas-focus` | `#334A5E` | `#B9CAD9` |
| `--gas-info` | `#334A5E` | `#9FB4C7` |
| `--gas-text` | Deep Ink `#17212B` | `#E7EAEC` |
| `--gas-text-muted` | Slate `#687786` | `#A3AEB9` |
| `--gas-border` | Mist `#E7EAEC` | `#2C3A47` |
| `--gas-surface` | Paper `#F7F7F5` | `#121A22` |
| `--gas-surface-raised` | `#FFFFFF` | `#1A242E` |
| `--gas-success` | `#2E7D5A` | `#6FBF97` |
| `--gas-warning` | text: Deep Ink; fill: Amber `#C6922D` | `#E0B35C` |
| `--gas-danger` | `#B74B4B` | `#E58A8A` |

Contrast rules (decision U-5):

- Text sits on `--gas-surface-raised` (white) panels; Paper is page background only.
- Amber is never text colour in light mode; warning badges use Deep Ink text on an Amber-tinted fill with an Amber border.
- Form inputs use a Slate border (≥ 3:1). Dividers use Mist.
- T-200 computes WCAG ratios for every text token on its surface in both themes.

### 3.2 Logo

- Assets: `docs/design/logo/` (source); served copies under `static/`: `icon.svg` (Storm Blue crab), `logo.png` (lockup), `logo-dark.png`, `app-icon-512.png`, `app-icon-192.png`.
- Used on sign-in, setup and offline pages (`<img>` in CRAB templates), PWA manifest, favicon, PDF headers.
- Shell header shows the text wordmark until go-app-shell offers an icon slot.
- PWA precache list adds the logo files (T-179 still holds: no authenticated URL).

### 3.3 Command Summary (T-198, T-199, T-213)

- Replaces the Incidents screen at `/`. Title *Command Summary*, summary *What needs your attention now.*
- Primary action *Declare incident* when permitted; the declare form stays an anchored section on the page.
- Section **Needs attention** (`gas-list`): approvals waiting for this user; first reports awaiting triage (only when the user may triage); Emergency Response Pack stale; single-person window open (Stage 5). Each links to the owning screen. Empty: *Nothing needs your attention.*
- Section **My active incidents**: one row per authorised incident with ID, title, mode badge, severity and phase as text, pending count and *Open*. Empty: *You are not assigned to any incident.*
- Footnote: *Only items you are authorised to see are shown.*
- Data: the existing frame call plus the Home view. No new projection reads.
- Navigation label stays *Incidents* → renamed *Command Summary* to match the page.

### 3.4 Screen pattern pass

| Pattern | Screens | Rules applied |
|---|---|---|
| Entry / Setup | sign-in, setup, offline | constrained width via shell, logo, plain explanation, one primary action |
| Workspace / Overview | Command Summary, Approvals, First reports list, Readiness, Obligation library, Access admin, Organisation | title, one primary action, panels, factual empty states |
| Capture / Record | first-report form, add evidence | single column, section headings, verb-labelled file inputs, one primary action |
| Review / Decision | Board, evidence detail, Close-out, Communications, Access | summary first, state as text, secondary actions subordinate |

Template changes only. No new classes beyond shell classes.

### 3.5 Print

- Renderer: Storm Blue headings and rules in colour mode; black in monochrome. Go Regular body, Go Bold headings (both from `golang.org/x/image`, already a dependency).
- Header: logo mark (embedded PNG), document title, compact metadata bar (incident, mode, generated, source sequence, time trust). Footer unchanged.
- Tables: Mist rules, no fills. Status remains text.
- Output stays deterministic for fixed inputs (T-201).

### 3.6 Upstream request (not a dependency)

go-app-shell v0.2 candidates: header icon slot; native `details`/`fieldset` rules (UI notes §22); optional two-panel overview. CRAB adopts a release only through the normal dependency-upgrade route.

---

## 4. Stage 3 — Organisation configuration (0.0.18-dev)

### 4.1 Record family

| Record type | Payload | Notes |
|---|---|---|
| `system.severity_scale` | ordered `levels[]{label, description}`, 2–7 entries, unique labels | seed: *Severity level 1* (least severe) … *Severity level 5* (most severe) |
| `system.emergency_configuration` | organisation name; contacts; external parties; escalation lines; role cards; procedures | fixed shape, size-bounded |
| `system.checklist_template` | existing `domain.ChecklistTemplate` | Stage 4 |
| `system.single_person_window` | §6 | Stage 5 |

All are `governed`, `SYSTEM` compartment, Propose → Decide through the one `/decide` route with kind `organisation`. Maker and confirmer must be Major or Custodian-designated Major per existing setup governance; the confirmer differs from the maker.

### 4.2 Severity scale (T-202)

- Setup form gains the scale, pre-filled with the five seed levels. The two setup people confirm it with the bootstrap ceremony as the first `system.severity_scale` record.
- The Organisation screen proposes later changes.
- Declaration and severity change require a value from the current confirmed scale. Without a confirmed scale, declaration is refused with *Define the severity scale first*.
- History is untouched; earlier free-text values display as recorded.

### 4.3 Emergency configuration (T-203, T-204)

- Contacts: name, role, primary route, alternative route, notes.
- External parties: same shape plus category (insurer, regulator, counsel, adviser, supplier, other).
- Escalation: ordered lines. Role cards: title, responsibilities, deputy. Procedures: title and text, at most 20, each ≤ 4,000 characters.
- Read permission: any authenticated Major or Custodian. Print still needs `print`.
- Incident Brief *Key contacts* and the whole Emergency Response Pack come from the confirmed configuration. A confirmation after the last pack generation marks the pack stale.
- Emergency contacts are never an out-of-band verification route (AC-12).

---

## 5. Stage 4 — Related incidents and checklists (0.0.19-dev)

### 5.1 Related incidents (T-205, T-206)

- `incident.relationship`, governed, recorded under incident A with payload `{related_incident_id, reason, active}`; compartment `GENERAL`.
- Propose and confirm require authority to read both incidents. The app tier checks both before writing or deciding.
- Projection `crab.incident_relationships(incident_id, related_incident_id, reason, active, maker, confirmer, confirmed_at)`, stored in both directions.
- Board section *Related incidents* lists only links where the viewer can read the other incident. Nothing else is shown.
- Unlink proposes `active=false`. No merge, no access, no timeline sharing.

### 5.2 Checklist templates (T-207)

- Templates live in the Organisation screen as `system.checklist_template` records.
- Board *Activate checklist* lists confirmed templates; activation calls the existing `ActivateChecklist`.
- Approvals groups proposals that share a checklist activation under one heading; each is still decided individually.

---

## 6. Stage 5 — Single-person window (0.0.20-dev)

- `system.single_person_window`, governed: `{window_id, starts_at, ends_at, humans[], kinds[], reason, active}`.
- Opening, extending or closing early is Propose → Decide by two people. A window can never cover its own record type.
- Maximum length 24 hours. `starts_at` cannot be earlier than the proposal time.
- Covered kinds are chosen from incident decisions, actions, roles, severity, scope, communications and obligations. Never covered: access, isolate, custodian, organisation configuration, closure, retention.
- Only Majors may be named.
- Inside an active window, a named Major may confirm their own proposal of a covered kind with a non-blank reason. The decided record carries `self_approval=true` and the reason; the ledger already validates them.
- Every self-approval shows `SELF-APPROVAL` on Approvals, Board, timeline, Brief and Command Summary. Status line shows *Single-person window open*.
- Window checks use the clock. Self-approvals under a non-trusted clock carry both markers.

---

## 7. Stop signs

- a second clock, a second decide route or a second configuration path;
- a new table without a ledger record type behind it;
- a dependency, web font, icon set or JavaScript;
- `crab.css` layout rules or `url(`;
- any count or link that reveals an unauthorised incident;
- self-approval of access, organisation configuration or the window itself;
- a hard-coded severity list outside the setup seed;
- a deleted or weakened test.

---

## 8. Implementation record

### Stage 1 — 0.0.16-dev

- Time status is treated as an upper bound: the kernel stores `WeakerTimeStatus(record, clock)` for every record without a human cryptographic signature. Callers may downgrade, never upgrade.
- The access and corpus projections stopped comparing a decision's time status with its proposal's (decisions are stamped at decision time). Version, payload, maker, compartment and mode binding is unchanged.
- First reports with no stated time confidence take the clock's reading instead of `trusted`.
- Inline PDFs carry `frame-ancestors 'none'` rather than the HTML policy, so built-in PDF viewers are not blocked by `object-src 'none'`.
- The PDF footer states `TIME VERIFIED`, `TIME DRIFTING` or `TIME UNVERIFIED` in words.
- Evidence: `docs/build-evidence-0.0.16-dev.md`.
