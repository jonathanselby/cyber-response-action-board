# CRAB — Post-UAT remediation triage and proposed plan

**Date:** 2 November 2026
**Baseline:** `66b188a`, 0.0.21-dev (`main` after PR #3)
**Input:** *CRAB — Post-UAT Remediation Build Brief* (22 findings plus the Garage engineering rules, testing rule, register, final gate and handover point)
**Method:** each finding is checked against PRD V0.2, EDD V0.4, `TEST-ACCEPTANCE-NOTES.md`, Bafflewick and the code at `66b188a`. It is then given a verdict using the constraint review (`engineering-under-constraints/principles/constraint-review.md`).
**Register:** `docs/remediation-register.md` holds one row per finding: `ID | Severity | Finding | Fix | Test | Result`.
**Status:** design decisions resolved 2 November 2026 (§0). Contract addendum drafted in `docs/contract-addendum-v0.2.1.md`. No code changes until the addendum and plan are approved.

> **Prove the engineering so the human UAT can be about the experience.**

---

## 0. Resolved UAT design decisions

Decided by the product owner on 2 November 2026. They close the architectural questions before implementation starts. They are not reopened during the build.

> **U-15:** Maintain dual control. Support multiple Custodians. Recommend minimum three; readiness warning below three. No single-person emergency bypass.
> **U-12:** Create a narrowly scoped PRD/EDD addendum for incident reopening, observables, management summary and machine API intake. Do not expand beyond those capabilities.
> **U-16:** External interactive users require MFA/TOTP. Machine identities use scoped, revocable credentials.
> **U-19:** Use Playwright as test-only browser/accessibility automation. It must not become a production dependency.
> **U-13/U-20:** Complete the reusable notifier before implementing CRAB notification changes. Notifications are limited to events requiring attention/action and are never authoritative.

What each decision means for this plan:

- **Dual control does not mean exactly two administrators.** It means no single administrator can perform the governed action alone. Any number of Custodians is supported, and readiness warns when fewer than three are *active and usable*: not suspended, not expired, and holding a working credential. There is no emergency override, and Custodian kinds stay excluded from single-person windows.
- **The addendum** (`docs/contract-addendum-v0.2.1.md`) defines each of the four additions tightly enough that none becomes a subsystem.
- **MFA:** TOTP is required for every `external=true` interactive person. Internal people keep the existing authentication policy; this UAT does not redefine it. Intake keys are not people: they are scoped to intake only, revocable, rotatable and expiring.
- **Playwright:** test-only, outside the binary and the runtime, pinned and reproducible, callable from CI and the UAT tooling. It covers Chromium, Firefox and WebKit, viewports, keyboard and axe.
- **Notifier sequence:** notifier → component tests → integrate CRAB → CRAB tests. No temporary CRAB notification plumbing. V1 events are limited to the five in §3.5. *CRAB contains the truth. Notifications tell someone to look at CRAB.*

Still open with a default recommendation, adopted unless changed: U-14 (shell v0.2), U-17 (Custodian membership view), U-18 (30-day external maximum), U-21 (API path), U-2 (UTC).

---

## 1. Summary

The brief contains three kinds of work.

1. **Product gaps (P0, 4 findings).** People cannot be added after setup, and external or temporary people therefore exist only in tests. The access model underneath is sound: membership expiry, compartment read-in, Sponsor/Controller governance and per-request principal reload are already in the code. What is missing is the **identity lifecycle**: there is no record that a person exists, no record of whether they are active, and no way to issue them a credential. This is the release blocker. It is one new governed record type, one supporting credential change and one check in the one place a session becomes a principal.
2. **Small product additions (P1, 5 findings).** These are:
   - first-report intake from other systems;
   - observables;
   - a management summary;
   - delivery status and manual resend for notifications;
   - closure and reopening.

   Each fits as one record type or one view inside an existing screen. Three of them change the product contract: reopening, observables and the management summary. Two touch locked EDD rules: no automatic retry, and no independent non-human principal. Neither rule needs to move (§3.5, §3.6).
3. **Proof (P1/P2, 13 findings).** Load, soak, destructive failure, database and storage failure, backup and restore, upgrade, black-box abuse, browser and device, accessibility, time, board density and approvals. Most of this is test harness, not product code. The harnesses are Go tests and shell scripts against the existing end-to-end harness. They add no new product dependency. The one test-only addition is Playwright with axe-core for the browser matrix and accessibility checks (decided, U-19).

Reading the code also found six defects or gaps that the brief does not name (§4). The most important are:

- the browser login is password-only, although TOTP and passkey verification exist in `authn`;
- membership expiry is evaluated against `time.Now()` rather than the governed clock;
- expired sessions are never swept;
- uploaded bytes are written before the ledger record that references them.

Proposed delivery is seven slices, P0 first (§7). Two items depend on Garage components that the brief says are being handled separately:

- **all CRAB notification changes** wait for a newer `notifier` (only v0.1.0, file plus HTTP, exists). Decided: notifier first, then CRAB (U-13);
- **the header logo and signed-out navigation** depend on `go-app-shell` v0.2 (only v0.1.0 is tagged).

Notification work is sequenced as its own slice, gated on the notifier release (S25N). The shell items are adopted when supplied (U-14).

---

## 2. Code facts that shaped the triage

| Fact | Where | Consequence |
|---|---|---|
| Access record types are role eligibility, Custodian, Sponsor, Controller, incident membership, compartment membership, output permission and isolate. There is no identity or account record | `security/access/access.go:14-23` | Nothing records that a person exists, their details, internal/external status or active/suspended/disabled |
| `authoriseMakerAtDecision` has no case for `security.role_eligibility` or `security.custodian_designation`, so it falls to `default: deny` | `app/access.go:284-334` | Role eligibility can only be created at bootstrap today. This confirms the gap noted in Stage 4 |
| Bootstrap makes both setup people Custodian and Major | `cmd/crab/operator_runtime.go:258-259` | Every installation starts with exactly two Custodians. Dual control for user administration is satisfiable only while both are reachable (U-15) |
| Local credentials are one row per human (salt, iterations, hash) with no state, expiry or must-change flag | migration `002`; `data/postgres/security_store.go:35` | Credential issuance and reset need a small schema change in supporting state, not in the ledger |
| Browser and MCP resolve the session, then **reload the principal from projections on every request** | `cmd/crab/web_runtime.go:34-38`; `operator_runtime.go:325-333` | Role, incident, compartment and output changes already take effect on the next request in every session. That needs proving with tests, not building |
| Incident and compartment memberships carry `effective_at`/`expires_at`, and `PrincipalAt` filters on them | migration `010`; `data/postgres/access_projection.go:244-300` | Temporary access already exists. Expiry is enforced at read time with no timer |
| `Principal()` passes `time.Now()` to `PrincipalAt` | `access_projection.go:240-242` | Expiry ignores the governed clock (F-2) |
| `Sessions.Resolve` checks only the token digest and expiry. It knows nothing about the human's status | `security/authn/session.go:51-69` | Suspension or disablement needs one check where the session becomes a principal, plus deleting that human's sessions |
| Expired sessions are deleted only when that token is presented again. There is no sweep | `authn/session.go:64-66`; `security_store.go:97` | Sessions grow without bound over a soak (F-3) |
| Session TTL is 8 hours | `operator_runtime.go:33` | "Session expiry with unfinished form" (R-17) is reachable inside one shift |
| The browser login is local password only. `AuthenticateTOTP` and `VerifyPasskey` exist and are unreferenced by the web or cmd tiers | `operator_runtime.go:309-319`; `security/authn/advanced.go` | External users would get a single factor (F-1, U-16) |
| Access Decide checks role eligibility before confirming incident membership, and incident membership before confirming compartment read-in | `app/access.go:178-194` | The Jane journey order (eligible → incident → compartment) is already enforced |
| First-report capture requires an authenticated Major or Minor human | `app/continuity.go:52` | A SIEM cannot submit without a credential model for non-human sources (R-06) |
| The JSON API bearer token is a human browser session token. Request bodies are capped at 64 KiB | `presentation/web/api.go:11,131-144` | There is no machine credential. The size bound already exists |
| Evidence and first-report bytes are written to the blob store, then the ledger record is appended | `app/evidence.go:52,71`; `app/continuity.go:59,69` | A crash between the two leaves an unreferenced blob. It is not ambiguous business state, but it needs to be proven and swept or reported (F-4) |
| Migrations run in one transaction under an advisory lock | `data/postgres/migrate.go:82-87,160` | An interrupted migration should leave the old schema intact. That needs proving rather than building |
| Notification outcomes are ledger records projected to `notification_outcomes` with `delivered/failed/suppressed`. There is no retry | migration `013:80-97`; EDD §17.2, §18, §31 | The audit trail exists. Request status, display and manual resend do not |
| Garage `notifier` v0.1.0 supports `file` and `http` channels only, and explicitly excludes email/SMS SDKs and retries | `jonathanselby/notifier` README | Email, SMS and Teams depend on a newer notifier (U-13) |
| `ErrIncidentClosed`: closed incidents are read-only. Nothing in code, PRD or EDD reopens one | `domain/recovery.go:27`; PRD §12; EDD §13.16 | Reopening is a contract change (R-09) |
| Approvals already show effective → proposed, maker, reason and consequence for each item | `presentation/web/runtime.go:259-281` | R-22 is wording and hierarchy (for example, what rejection means), not a new surface |
| `BoardView` renders every section in full. There is no "needs attention now" block and no limit per section | `runtime.go:212-231` | At 41 actions and 13 unknowns, priority is lost (R-21) |
| `Frame` runs on every request: session, principal (5 queries), approvals across all kinds, `config.Load()` from disk and pack freshness | `cmd/crab/web_runtime.go:33-63` | This is the first suspect under load (R-10). Measure before changing |
| The pool is capped at 8 connections, even if the URL asks for more | `data/postgres/pool.go:13,26-28` | 50 users will queue on 8 connections. That may be correct (slower, not wrong) but must be measured |
| The visual preflight is Go-only static HTML contracts. Visual capture uses headless Chrome and gates nothing | `scripts/visual-acceptance-preflight.sh`; `scripts/visual-capture.sh:10` | No real-browser keyboard, zoom, axe or multi-engine check exists today (R-17, R-18) |
| `go-app-shell` has one tag, v0.1.0 | `git ls-remote` | Header logo and signed-out navigation remain blocked on the shell (R-20, U-14) |
| Bafflewick phase 2 already proves external-responder expiry and revocation at the service level | `docs/bafflewick-acceptance.md` | The P0 gap is that a real person cannot be onboarded, not that scoping fails |

---

## 3. Triage

The verdicts follow the constraint review:

- **KEEP**: accept as asked.
- **SIMPLIFY**: accept with a smaller mechanism.
- **REMOVE**: not justified, or not compatible with the locked contract.
- **TEST**: an assumption not yet demonstrated, to be proven, with a fix only if proving fails.

### 3.1 R-01 P0 — Complete user administration

| Element | Verdict | Reason |
|---|---|---|
| One governed identity record `security.identity` | **KEEP** | Payload: `{human_id, display_name, organisation, contact_route, external, status: active\|suspended\|disabled, account_expires_at?, reason}`. It covers create, details, internal/external status, activate, modify, suspend, reactivate, disable and expiry with **one** record type, each change a new version. Projected to `crab.identities`, so it can be rebuilt |
| Role eligibility after setup | **KEEP** | The record type exists. Add the missing `authoriseMakerAtDecision` case: Custodian maker, different Custodian confirmer. Confirming eligibility requires an `active` identity |
| Create the person and grant eligibility as one approval | **SIMPLIFY** | The Access admin form proposes the identity and the eligibility together. The confirmer sees one grouped approval (the same grouping used for checklist activations), and each record is still decided individually. This saves the 02:17 Custodian a second trip |
| "Establish credentials" and "reset/recover" | **SIMPLIFY** | The Custodian issues a **one-time enrolment code**: generated, shown once, delivered out of band by the Custodian, stored hashed, valid for at most 24 hours. On first sign-in the person must set a password under `authn.ValidateNewPassword`. Reset issues a new code and deletes every session for that person. No email loop and no self-service recovery: CRAB must work without corporate mail |
| Credential issue/reset audit | **KEEP** | `security.credential_issued` is a `Record(...)` naming issuer, subject and expiry, with no secret. The secret lives only in the credentials table (supporting state, EDD §10.2) |
| Credential issue: dual control? | **KEEP** | A single Custodian may issue or reset, because the code works only for an `active` identity, and activation is already dual-controlled (U-15). Every governed identity, eligibility and intake-key change needs two distinct Custodians |
| Custodian edge cases | **TEST** | The ugly cases from U-15: a suspended Custodian, an expired Custodian, a locked-out Custodian (no working credential), and a Custodian who leaves while their proposal awaits confirmation. Expected: none of them can confirm; a pending proposal from a now-unusable maker stays visible and can be rejected but not confirmed; readiness counts only usable Custodians |
| Fewer than three usable Custodians | **KEEP** | Readiness `warn` with the count and the reason. It does not block operation |
| "Revoke access" | **SIMPLIFY** | This is not a new concept. Revocation means ending memberships (existing records) and/or suspending or disabling the identity. The Access admin person view offers *Revoke all incident access* as grouped membership-end proposals |
| "Set temporary access expiry" | **KEEP** | Two levels, both existing or one field: (a) per incident/compartment membership expiry (exists); (b) `account_expires_at` on the identity, after which the person cannot authenticate at all |
| Disabled is terminal | **KEEP** | `disabled` cannot be reactivated, so a `human_id` is never reused. A returning person gets a new identity. Suspension is the reversible state |
| Screen | **SIMPLIFY** | Extend Access admin with a *People* section (list, add, person detail). No new screen; the budget stays at 11 |
| Custodian sees no incident content | **KEEP** | The person view shows memberships by incident ID and compartment name only for incidents the Custodian may see. Otherwise it shows a count without names (EDD §9.6, *hidden means hidden*). U-17 default, adopted unless changed |
| Directory sync, SCIM, groups, self-registration | **REMOVE** | Pulls toward ITSM/IdP. Not requested |

**Pseudocode — identity lifecycle (consequential, per brief rule 5)**

```text
propose_identity(maker, change):
    require maker is Custodian
    require change.status in {active, suspended, disabled}
    if current(change.human_id).status == disabled: refuse "disabled identities are final"
    if change.account_expires_at set: require > now and <= now + MAX_ACCOUNT_LIFETIME (U-18)
    append Propose(security.identity, change)

decide_identity(confirmer, proposal, outcome):
    require confirmer is Custodian and confirmer != proposal.maker
    require account_usable(confirmer, now)                         # suspended/expired Custodians cannot confirm
    if outcome == confirmed: require account_usable(proposal.maker, now) and maker still Custodian
    require proposal.version == current pending version            # existing exact-binding
    append Decide(...)                                             # projection updates crab.identities
    if confirmed and change.status != active:
        delete all crab.sessions where human_id = change.human_id   # same transaction as projection

issue_enrolment_code(custodian, human_id):
    require custodian is Custodian
    require identity(human_id).status == active and not account_expired(now)
    code = random 20 chars (shown once)
    store hash(code), expires_at = now + 24h, must_set_password = true
    delete all sessions for human_id
    append Record(security.credential_issued, {subject, issuer, expires_at})   # no secret

authenticate(human_id, secret):
    if not account_usable(human_id, clock.now): fail (generic message)
    if credential is enrolment code: verify, then require new password before any session
    else verify password
    if identity.external: require enrolled TOTP and a valid code     # U-16; enrol at first sign-in
    create session

account_usable(human_id, now):           # ONE function, used by authenticate AND principal load
    id = identity(human_id)
    return id exists and id.status == active
       and (id.account_expires_at is null or id.account_expires_at > now)
```

Bootstrap identities become `security.identity` records in the same atomic setup batch. A migration back-fills `active` identities for existing role-eligible humans, so upgrade keeps everyone signed in (R-15).

### 3.2 R-02 P0 — External responder onboarding

| Element | Verdict | Reason |
|---|---|---|
| External people use the same identity, scope and compartment model | **KEEP** | PRD §19, EDD §13.11. `external=true` is a field on the identity, not a second user type or portal |
| Explicit scope, expiry, revocation, audit | **KEEP** | Scope and expiry already exist on memberships. The account expiry from R-01 adds a hard stop. Audit is the ledger |
| External default expiry | **SIMPLIFY** | When `external=true`, the add form requires an account expiry and pre-fills the incident membership expiry. Suggested default 7 days, maximum per U-18. Internal people may have no expiry |
| MFA for external people | **KEEP** | U-16. TOTP is enrolled at first sign-in, straight after the enrolment code and password are set, using the existing `authn.AuthenticateTOTP`. A credential reset also resets TOTP enrolment. No new dependency: enrolment shows the base32 secret and the `otpauth://` URI as text for manual entry. A QR code would need an encoder the standard library lacks, so it is left to the human UAT to say whether one is needed |
| External marking in the interface | **KEEP** | Text badge `EXTERNAL` beside the name on Board roles, Access, Approvals maker/confirmer and the Brief. No colour-only meaning |
| No visibility outside authorised incidents and compartments | **TEST** | Already the authorisation rule. Prove it with the abuse suite (R-16) using an external identity created through the browser |
| Separate external portal, guest links | **REMOVE** | PRD §19 says no portal unless justified |

### 3.3 R-03 P0 — Access changes and active sessions

| Element | Verdict | Reason |
|---|---|---|
| Role increase or reduction, incident removal, compartment removal and explicit revocation take effect in live sessions | **TEST** | The principal is reloaded per request (§2), so this should already hold. Prove it with two or more simultaneous sessions per person across browser, JSON API and MCP |
| Suspension, disablement and account expiry take effect in live sessions | **KEEP** | New. `account_usable` is checked in `PrincipalAt` (one place), so every existing and future surface inherits it. Session deletion on suspension or disablement is defence in depth, not the control |
| Membership expiry takes effect without a timer | **TEST** | Read-time filter (§2). Prove with the clock moved past expiry while a session stays open |
| Expiry uses the governed clock | **KEEP** | F-2. `Principal()` takes the kernel clock reading |
| In-flight form after revocation | **KEEP** | A POST after revocation is refused in frame with the ordinary refusal and no data echo. A test covers it |
| Push logout or websocket | **REMOVE** | Not needed. The next request is refused |

**Pseudocode — the one principal rule**

```text
principal_for_request(token):
    session = sessions.resolve(token, clock.now)        # digest + session expiry
    if none: return unauthenticated
    if not account_usable(session.human_id, clock.now): return unauthenticated
    return load_principal(session.human_id, clock.now)  # role, memberships filtered by effective/expires
```

### 3.4 R-04 P0 — Shift and surge operation

| Element | Verdict | Reason |
|---|---|---|
| 72-hour incident with shifts, joiners, leavers, role changes, external specialists, legal, comms, handovers, temporary and expiring access, changing Majors | **TEST** | Every mechanism exists except onboarding (R-01/R-02). Add **Bafflewick phase 10, *the long night***: a scripted 72-hour scenario on the fake clock, through the browser runtime, with assertions at each shift boundary (who can see what, who holds which role, handover contents, expired people refused, history intact, ledger verifies, projections rebuild identically) |
| Changing Majors | **TEST** | Role eligibility change from Major to Minor while that person has pending proposals. Expected: their pending proposals stay pending and decidable by another Major. Their own decide attempts are refused from the next request. The phase asserts this |
| Unsafe workarounds | **TEST** | The phase fails if any step needs a shared account, direct SQL, a bootstrap rerun or break-glass |
| Rota or scheduling features | **REMOVE** | Not CRAB's job (PRD §20) |

### 3.5 R-05 P1 — Notifications

| Element | Verdict | Reason |
|---|---|---|
| Show requested → attempted → delivered/failed (and suppressed) per issued communication | **KEEP** | The data already exists. The Communications screen gets one *Delivery* table per communication with channel, time, outcome and detail in words |
| A notification *request* record | **SIMPLIFY** | The existing notify action already records each attempt. Add one `communication.notification_requested` `Record(...)` before the attempts, so "requested but process died before attempt" is visible after restart (R-12) |
| Retry status | **SIMPLIFY** | Automatic retry is a locked removal (EDD §17.2, §31). The interface states *Failed — not retried automatically*. A failure is visible on the Board status line until resent or acknowledged |
| Manual resend | **KEEP** | *Resend to failed channels* on the communication: same `notify` permission, same approved content digest, same channels limited to the failed ones. It produces new attempt records linked to the earlier attempt. Exercise suppression still applies before I/O |
| Sequence | **KEEP** | U-13: **notifier → component tests → integrate CRAB → CRAB tests.** CRAB notification work starts only when the reusable notifier with the needed channels is released. No temporary CRAB channel adapters. Until then the existing file and HTTPS behaviour stays exactly as it is |
| Email, SMS, Teams/Slack-style webhook | **KEEP** (in notifier) | Channel types belong to the Garage `notifier` (v0.1.0 has `file` and `http`). CRAB passes the composed message, recipients and channel names, then records the outcome. Body shapes, SMTP and SMS gateways are notifier concerns |
| V1 notification events | **KEEP** | U-20, exactly five, each addressed to the person who must act: (1) **assignment needing attention**: I am made owner of an action or holder of an operational role; (2) **decision needed**: a proposal I am eligible to confirm is waiting, debounced to one notice per person per incident per interval; (3) **deadline**: an obligation warning, deadline or breach, sent to the clock owner from the existing timed wait list; (4) **material escalation**: a confirmed severity change or phase change, to incident members, only where the organisation enables it; (5) **access needing action**: temporary access expiring soon, to the Sponsor or Controller who can renew it |
| Not notified | **REMOVE** | Timeline updates, facts, comments, evidence uploads and ordinary state changes. Alert fatigue defeats the purpose |
| Content | **KEEP** | A notice says *what needs you* and links to CRAB. It carries the incident ID and the item title only where the recipient may read them, and never restricted-compartment content. *CRAB contains the truth; notifications tell someone to look at CRAB.* A notice is never an approval route |
| Per-person address storage | **KEEP** (decided 2 Nov) | The address is **security-supporting state, not ledger**: `crab.notification_routes(human_id, channel, address, updated_at)`, one optional route per person, included in backups. The ledger gets `security.notification_route_changed` naming who changed whose route, the channel, the time and a **masked hint** (`…789`, `j…@forensic.example`), never the address. Disabling a person deletes their row and the audit record remains (Garage rule 8). Suspension keeps the row and sends nothing |
| Who sets it | **KEEP** (decided 2 Nov) | The person, after sign-in, or a Custodian when creating the person. `Record(...)`, not dual-controlled: a notice carries no content and approves nothing. Never a credential-recovery or verification route (AC-12). Nothing is sent to an account that is not usable |
| Notice content | **KEEP** (decided 2 Nov) | Kind of item plus incident ID (the recipient can already read the incident) and *Sign in to CRAB*. No titles, summaries, compartment names or action links. Counts include only items the recipient is eligible to decide **and may read**, so a restricted item is never revealed by a count |
| Shared destinations | **KEEP** (decided 2 Nov) | Fixed-target channels (for example a Teams channel webhook) may receive **only event 4**, only where the organisation enables it, and only as a content-free notice with no incident ID. Every other event goes to per-person routes only |
| No route | **KEEP** | Recorded outcome `no route`, nothing sent. The item stays on the person's Command Summary. Readiness `warn`: *n Majors/Custodians have no notification route* |
| Debounce | **KEEP** | *Decision needed* collapses per person per incident within a window (default 15 minutes) into one notice with a count. The window is evaluated from the timed wait list, not a new scheduler |
| Notifier request shape | **TEST** (requirement handed to notifier work) | Per-recipient `deliveries[{channel, to, ref}]`; the channel declares `addressing: fixed\|per_recipient` and an `address_pattern`; outcomes echo `ref`, never `to`; v0.1-style requests remain valid. Specified in `docs/notifier-v0.2-requirement.md`. If the released notifier differs, CRAB adapts to it |
| Recipient lists, templates, inbox, two-way messaging | **REMOVE** | "Do not build a communications platform" |

**Pseudocode — resend**

```text
resend(user, communication_id):
    require authorise(user, notify, communication.scope)
    require communication is confirmed-issued and digest unchanged
    failed = channels whose latest attempt is failed
    if none: refuse "nothing to resend"
    append Record(notification_requested, {communication_id, failed, resend_of: last_attempt_ids})
    for channel in failed: attempt → append Record(outcome)          # existing path, sequential
```

### 3.6 R-06 P1 — Universal first-report intake

| Element | Verdict | Reason |
|---|---|---|
| One small authenticated endpoint | **KEEP** | `POST /api/v1/first-reports` (JSON, existing 64 KiB cap), creating an ordinary first report with `source_channel = <key name>`. The brief's `/first-report` is satisfied by this documented route. Decision **U-21** if the exact path matters |
| Machine credential | **SIMPLIFY** | An **intake key** is a scoped machine credential, not a person. It is bound to one accountable named human (EDD: an agent never acts as an independent principal) and permits **only** first-report capture. A Custodian proposes it and another Custodian confirms. It is shown once, stored hashed, has an expiry and is revocable. **Rotation**: a new secret is issued under the same key name, with an optional overlap of at most 24 hours, after which the old secret stops working. It is recorded as `security.intake_key` (governed). Every report shows the key name and the accountable human. TOTP does not apply (U-16). No general API IAM: intake keys authorise nothing else |
| A report never becomes an incident on its own | **KEEP** | PRD §17. Triage remains human |
| Idempotency | **KEEP** | An optional `external_ref` per key. A repeat returns the original report ID and is never a duplicate. This answers *committed once* for retrying senders |
| Rate bound | **KEEP** | Per-key bounded rate (the existing `FailureLimiter` pattern). Over the bound returns `429`. No queue |
| Attachments over the API | **REMOVE** | Not needed for a first signal. They keep the hostile-byte path in the browser |
| Vendor connectors, polling, webhooks from CRAB to vendors | **REMOVE** | Explicitly excluded |
| Documentation | **KEEP** | `docs/first-report-api.md`: one page with a `curl` example, fields, errors and limits |

### 3.7 R-07 P1 — Indicators / observables

| Element | Verdict | Reason |
|---|---|---|
| `type \| value \| status \| source \| first_seen \| related_incident` | **SIMPLIFY** | `incident.observable` as an immutable `Record(...)` (an observation, not a governed decision). Types: `ip, domain, url, hostname, email, hash`. Status: `suspected, confirmed, benign, blocked`. A status change is a new record version. `first_seen` is a stated time with time confidence, like a first report |
| `related_incident` | **SIMPLIFY** | Derived, not stored. The same normalised value in another incident is shown **only if the viewer can read both** (the same rule as related incidents, T-205). Otherwise nothing is shown. No cross-incident search for people without access |
| Compartment | **KEEP** | Observables carry a compartment like evidence. Default `GENERAL` |
| Value handling | **KEEP** | Light syntactic validation per type. Values are rendered as text and never as links (no click-through to hostile URLs). They are not sent to model lanes unless the managed-model policy already permits that compartment |
| Where | **SIMPLIFY** | A section on the Evidence screen (*Evidence and indicators*). It is not added to the Board, which is already dense (R-21) |
| Enrichment, feeds, STIX/MISP export, reputation lookups, bulk import | **REMOVE** | Threat-intelligence platform |

### 3.8 R-08 P1 — Management summary

| Element | Verdict | Reason |
|---|---|---|
| Active incidents, by severity, overdue critical actions, overdue obligations, median decision-loop time | **KEEP** | One *Management summary* panel on the Command Summary, computed **only over incidents the viewer can see** (no universal visibility; T-198 still holds) |
| Median decision-loop time | **SIMPLIFY** | Defined as proposal time → decision time for confirmed governed records in visible incidents over the last 7 days. One SQL aggregate over existing proposal and decision projections. The definition is printed under the number |
| Exercise incidents | **KEEP** | Excluded from the live summary and labelled if shown |
| T-199 (Command Summary adds no queries) | **TEST** | This panel adds one or two aggregate queries. Amend T-199 to "a fixed, small number of queries independent of incident count" rather than break it silently |
| Charts, trends, exports, filters | **REMOVE** | BI product |

### 3.9 R-09 P1 — Closure and reopening

| Element | Verdict | Reason |
|---|---|---|
| Accidental closure | **TEST** | Closure is already `Propose → Decide` with prerequisites (EDD §13.16). Prove that one person cannot close, and that the confirmation screen states the consequence (*read-only; outputs generated*) |
| Closure with outstanding governed matters | **KEEP** | `domain.ClosurePrerequisites` (`domain/recovery.go:236`) requires stand-down, confirmed PIR, retention, and every recovery update verified or handed off. It returns one generic `ErrClosurePrerequisites`. Extend it to also refuse while any **pending proposal** or **unfired obligation clock** exists on the incident, and to return **each blocker by name** so the Close-out screen can list them. Test what is refused |
| Reopening | **KEEP** (contract change) | `incident.reopening`, governed: different confirmer, Major on the incident, mandatory reason. The phase is derived (`DeriveOperationalPhase`), not stored, so reopening **supersedes the closure and the stand-down**. Both stay in history and the derived phase falls back to RECOVERY or ACTIVE RESPONSE by the existing rule. Closing again needs a new stand-down, an updated PIR and a new closure. Earlier closure outputs remain immutable and are marked *superseded by reopening at …*. Excluded from single-person windows. **PRD §12 and EDD §13.16 amendment** (U-12) |
| State after reopening | **TEST** | Memberships that expired while closed stay expired. Pending proposals from before closure cannot exist (closure refuses them). Clocks do not restart unless re-triggered. Projections rebuild identically |
| Erasing history | **REMOVE** | Never |

**Pseudocode — reopen**

```text
propose_reopen(maker, incident, reason):
    require recovery_state.closed_at set and maker is Major member of incident
    require reason non-blank
    append Propose(incident.reopening, {reason, supersedes: [closure_id, stand_down_id]})

decide_reopen(confirmer, proposal):
    require confirmer != maker, confirmer is Major member, not via single-person window
    append Decide(...)
    # projection: closed_at = nil, stand_down_at = nil (records kept, marked superseded)
    # phase = DeriveOperationalPhase(state)   → RECOVERY or ACTIVE RESPONSE by existing rule
```

### 3.10 R-10 to R-15 P1 — Proof under load, time, failure and change

These are proof findings. The harness design is in §6. The verdict for each is **TEST**: prove it, then fix what the proof breaks. Any optimisation must cite a measured bottleneck (brief rule 2).

| ID | Verdict | Specific hypotheses to test first |
|---|---|---|
| R-10 heavy load | **TEST** | `Frame` cost per request; the 8-connection pool cap; Approvals assembled across all kinds on every frame; PDF generation in the request goroutine; `config.Load()` per request. Acceptance: no wrong answer, no 5xx except declared `503` back-pressure; p50/p95/p99 recorded on the laptop rig and on the Pi |
| R-11 soak | **TEST** | Session growth (F-3); goroutines from notifier HTTP with slow endpoints; timed-wait-list wake-ups firing exactly once; pool connections returned; memory flat after warm-up |
| R-12 destructive | **TEST** | Kill points: after blob write before ledger append (F-4); inside ledger append; between notification request and attempt; mid-PDF write (atomic rename already); mid-backup; mid-closure. Expected for each: *committed once* or *not committed*, the ledger verifies, projections rebuild identically, orphan blobs reported by `crab readiness` |
| R-13 database and storage | **TEST** | PostgreSQL down: every mutation refused visibly, no `200` without commit, recovery on return without restart. Disk full on the export directory: last-good copy kept (exists). Disk full on PostgreSQL: refusal surfaced, not swallowed |
| R-14 backup and restore | **TEST** | Mechanics exist (Bafflewick 6). Extend to a populated Bafflewick-scale instance with identities, memberships, observables, proposals and blobs. Compare every projection table and every blob SHA-256 before and after restore on a replacement database |
| R-15 upgrade and migration | **TEST** | Old binary (`66b188a`) populates → new binary migrates (018+) → `crab verify` → continue the incident. Kill during migration: old schema intact (single transaction). Failed migration: refused start with a clear message, then restore |

### 3.11 R-16 P1 — Black-box access-control abuse

| Element | Verdict | Reason |
|---|---|---|
| Identifier manipulation across incident, user, proposal, attachment, evidence, compartment, download URL, API and MCP | **KEEP** | A route-table-driven abuse suite: for **every** registered route, run as each of an outsider, a member of another incident, a member without the compartment, an expired person, a suspended person and an external person, with identifiers from objects they must not reach. Expected: the same generic refusal as for a made-up ID, no content, and no difference in timing or status that reveals existence |
| No route is forgotten | **KEEP** | A test fails if a route in the mux has no abuse case, so new routes cannot slip through |
| Stale session | **KEEP** | A token kept after revocation, suspension, expiry and logout is refused on all three surfaces |
| UI-only enforcement | **TEST** | Already prohibited (EDD §31). The suite calls routes directly, never through rendered forms |

### 3.12 R-17 / R-18 P1 — Browser, device and accessibility

| Element | Verdict | Reason |
|---|---|---|
| Chrome, Edge, Firefox, Safari | **SIMPLIFY** | An automated real-browser run on Chromium, Firefox and WebKit covers the engines. Edge is Chromium. Safari is approximated by WebKit, and a real Safari run is recorded by hand if a Mac is available |
| iOS Safari, Android Chrome | **SIMPLIFY** | Device emulation in automation (viewport, touch, user agent) plus a short manual device checklist in `docs/visual-acceptance.md`. Claiming real-device automation would be untrue |
| Refresh, back, multiple tabs, session expiry with unfinished form, interrupted mobile upload, small/large screen, 200% and 400% zoom, keyboard only | **KEEP** | Scripted journeys. Expected behaviour to fix if absent: POST-redirect-GET so refresh and back never resubmit; an expired session preserves the typed input after sign-in, server-side and single use, or at minimum says plainly what was lost (decision in slice, see T-235); an interrupted upload leaves no evidence record (F-4) |
| Automated accessibility | **KEEP** | axe-core rules on every screen in light and dark, plus label, landmark, focus-order and contrast checks. Screen-reader compatibility is limited to what automation can assert (names, roles, live regions). The human UAT judges the rest |
| Driver | **KEEP** | **Decided U-19:** Playwright with axe-core. It is test-only, pinned through a lockfile in its own `uat/` directory, run in its own workflow, never imported by the Go module, absent from the binary and runtime, and callable from CI and the UAT tooling |

### 3.13 R-19 P2 — Time behaviour

| Element | Verdict | Reason |
|---|---|---|
| Clock moves backwards | **KEEP** | If the clock reading is earlier than the last ledger timestamp, the kernel downgrades the time status to `untrusted` for that write and marks it. History is never written as if time went backwards silently. The timed wait list must not re-fire past wake-ups |
| Clock moves forwards or VM resumes | **TEST** | On wake, overdue timed waits fire **once**, marked late. Membership and account expiry apply immediately (read-time) |
| Synchronised but materially wrong | **TEST** | CRAB cannot detect this without an external reference, and none is added (brief: no distributed time infrastructure). The backwards check catches the backwards case only. Record the limit honestly in readiness help text |
| Daylight-saving transition | **TEST** | Everything is UTC internally and on screen (U-2 still open). Prove that deadlines spanning a local DST change keep their duration. If U-2 later adds local display, retest |

### 3.14 R-20 — Mechanical UX defects

| Element | Verdict | Reason |
|---|---|---|
| CRAB branding in the header | **TEST** | Needs a logo slot in `go-app-shell` (only v0.1.0 exists). Decision **U-14**: wait for shell v0.2, or contribute the slot to the shell first and adopt it, which keeps the stop sign "no layout CSS in CRAB" |
| Empty signed-out navigation | **TEST** | Same shell dependency (the rail renders with no items). Same decision |
| Mobile header/status truncation | **KEEP** | Reproduce from visual capture at 360–390 px. The status line wraps instead of truncating, and abnormal states keep their full words |
| Responsive failures, unclear validation messages, keyboard defects | **KEEP** | Collect them from the real-browser runs (R-17) into the register, then fix each one with a test |
| Visual language | **KEEP** | Storm Blue unchanged |

### 3.15 R-21 — Command-board stress

| Element | Verdict | Reason |
|---|---|---|
| Fixture: 41 actions, 13 unknowns, 7 clocks, 6 roles, 18 decisions, 4 compartments, 3 related incidents, 8 approvals | **KEEP** | Seeded in the end-to-end harness and captured in visual capture |
| "What do I need to worry about right now?" | **KEEP** | One **Now** block at the top of the Board with at most about 7 items, ordered: integrity or time problems → overdue or next-due clocks → overdue critical actions → approvals waiting for *me* → failed notifications → new unknowns since my last handover. Each is a link |
| Long sections | **SIMPLIFY** | Each section shows its most urgent N, with *All 41 actions* expanding the rest through native `<details>`. It is server-rendered with no JavaScript and no new layout CSS beyond what the shell provides. Check T-185 first; native `details` rules are already a go-app-shell v0.2 candidate (UAT2 S17 item 8) |
| Dashboard framework, widgets, user-configurable layout | **REMOVE** | Explicitly excluded |

### 3.16 R-22 — Approval comprehension

| Element | Verdict | Reason |
|---|---|---|
| What, current, proposed, proposer, reason, consequence | **KEEP** | Already present (§2). Make two gaps explicit: an **If you reject** line beside **If you confirm**, and the proposer's EXTERNAL badge and role |
| Access approvals in operator language | **TEST** | Identity, eligibility and membership approvals must read as *Give Jane Doe (EXTERNAL, Forensic Co) Minor access to INC-004 until 08:17 UTC*, not record types. Test against the R-01 fixture |
| Nothing behind extra clicks | **KEEP** | The full consequence shows on the queue row. Technical references stay folded |

---

### 3.17 Tier placement: no business logic in presentation

Product-owner instruction, 2 November: **no business logic in the presentation layer.** EDD §4.2 already requires it. This section states it for every remediation capability so that it is not left to judgement during the build.

**What counts as presentation** (`internal/presentation/{web,api,mcp,pdf}`): parse syntax, collect input, call one application service, render the returned view model, map errors to refusal text. Nothing else.

**The composition tier** (`cmd/crab/*_runtime.go`) wires services and adapts types. It is treated with the **same rule as presentation**: it may translate, but it may not decide. This matters because today it is where policy has crept in (F-7).

| Capability | Presentation does only | Application/domain/security decides | Data does |
|---|---|---|---|
| User administration (R-01) | People list/form; shows `CanPropose`, `CanDecide` flags returned by the app | Custodian authority, distinct confirmer, `account_usable`, disabled-is-final, expiry limits, grouped approval composition, what a Custodian may see of memberships (U-17) | `crab.identities` projection, credential rows, session deletion in the projection transaction |
| Enrolment, TOTP (R-01, U-16) | Code/password/TOTP forms; displays the once-only secret it is handed | Code generation, expiry, must-set-password, external-requires-TOTP, reset clears enrolment | Hashed credential and TOTP rows |
| Live revocation (R-03) | Nothing new | `principal_for_request` and `account_usable`, the one place a session becomes a principal | Memberships filtered at read time |
| External marking (R-02) | Renders the `External` boolean in the view model as the word EXTERNAL | Default and maximum expiry, required account expiry | — |
| Closure and reopening (R-09) | Close-out form; lists blockers it is given | Closure blockers by name, reopen rules, supersession, window exclusion | Projection updates |
| Intake API (R-06) | Decode JSON, return status codes | Key validity, scope (intake only), rotation overlap, idempotency, rate bound | Hashed key rows, `external_ref` uniqueness |
| Observables (R-07) | Form and table | Type validation, normalisation, status versions, related-incident visibility | Projection; related lookup query returns only authorised matches |
| Management summary (R-08) | Renders five numbers and the definition text | Which incidents count, exercise exclusion, metric definitions | Fixed aggregate queries |
| Notifications (R-05, S25N) | Route form; delivery table; resend button shown when the app says `CanResend` | The five events, recipient resolution, eligibility and read-scope counts, debounce, content composition, shared-destination rule, masked hint, exercise suppression, resend eligibility | `notification_routes`, outcome projection |
| Board *Now* block (R-21) | Renders an ordered list it is given | What is "now", ordering, top N per section | Existing reads |
| Approvals (R-22) | Renders what/current/proposed/proposer/reason/confirm/reject text and flags | `CanDecide`, `CanSelfApprove`, summary and consequence text (moved from `cmd/crab`, see F-7) | Existing reads |

**Enforcement.** The existing `TestTierImportBoundaries` checks imports only. It cannot see policy written with types the presentation already has. Three additions:

1. **Policy-reference ratchet.** A static test counts references to role constants, `authz.Authorise`, Custodian checks and window/kind policy in `internal/presentation/**` and `cmd/crab/*_runtime.go`. The baseline at `66b188a` is **14 references in 6 `cmd/crab` files** and none in decision position in `internal/presentation`, apart from the navigation hint in `page.go:214`. The count may only go down. Every file touched by this remediation must reach zero.
2. **Rules are tested where they live.** Every rule in the table above has its positive and negative tests in `internal/app`, `internal/domain` or `internal/security`. Web tests use the existing fake runtime (`fake_runtime_test.go`) and assert only rendering and refusal mapping.
3. **One decision, many surfaces.** Each rule that browser, API and MCP all reach is exercised through at least two surfaces against the same app service, which proves there is no surface-specific copy.

---

## 4. Defects and gaps found while reading (not named in the brief)

| ID | Finding | Proposal |
|---|---|---|
| F-1 | Browser login is password-only. TOTP and passkey verification exist but are not wired. PRD (*Identity*) says passkeys and TOTP remain available | **Decided U-16:** TOTP is required for external people, using the existing `authn` code. The internal authentication policy is unchanged by this UAT. Passkeys are deferred, since WebAuthn needs JavaScript and the product has zero required JavaScript |
| F-2 | Membership expiry uses `time.Now()`, not the governed clock | Fix with R-03 |
| F-3 | Expired sessions are never swept | One bounded delete of expired rows on the existing timed-wait cadence. No new scheduler |
| F-4 | Blob written before its ledger record, so a crash leaves an unreferenced blob | Prove with R-12. Readiness reports unreferenced blobs. A Custodian-governed sweep is offered only if the proof shows it is needed. Never delete automatically |
| F-5 | `Frame` reads the configuration file on every request | Measure in R-10. Load once at start, since deployment configuration is file-first and changes need a restart anyway |
| F-7 | Policy in the composition tier: `cmd/crab/web_runtime.go:683-689` decides `CanDecide` and `CanSelfApprove` with a hard-coded kind list that duplicates the single-person window exclusions, and builds approval summaries and consequences (`:715`, `:731`). 14 role/authorisation references sit across 6 `cmd/crab` files. The app services still enforce on submit, so this is a drift risk, not a bypass | Move the approval eligibility hints and wording into an app-tier approvals query that uses the same policy functions as `Decide` (S26, with R-22). Add the policy-reference ratchet (§3.17). Refactor only what the remediation touches: no general rewrite |
| F-6 | The Phase 3 human UAT record in `docs/visual-acceptance.md` is still *pending* | Record the Phase 3 outcome that led to this brief, so the register has a source |

---

## 5. Constraint review notes

**Mission.** Let a small team, including people who arrive at 02:17, run a cyber incident safely when normal systems are degraded, and leave a record that holds up later.

**State added.** Identities, intake keys, observables, reopening and the notification request. All are ledger records with projections that can be rebuilt. Credential codes are security-supporting state like today's passwords. No new store, queue or scheduler.

**Interfaces added.**
- the first-report API, inbound, with a key bound to a named human;
- email and SMS adapters, outbound, through the notifier (U-13);
- a browser driver, test only (U-19).

**Degraded operation.**
- Onboarding works with no corporate identity provider or mail. The enrolment code is delivered by the Custodian, for example by phone.
- Notification failure is visible and resendable, not hidden.
- PostgreSQL loss refuses writes instead of guessing.

**Environment test.** The assumption most likely to fail is that *two Custodians are reachable at 02:17*. Decided (U-15): keep dual control, support any number of Custodians, warn below three usable ones, and test the cases where one is suspended, expired, locked out or leaves mid-proposal.

**Removal test.** None of the following is built:
- SCIM or directory sync
- an external portal
- automatic retry
- recipient lists or templates
- vendor connectors
- API attachments
- threat-intelligence enrichment
- charts
- a dashboard framework
- push logout
- an NTP client

**Change test.** A newer notifier or shell is adopted through configuration and a version bump, not CRAB code.

| KEEP | SIMPLIFY | REMOVE | TEST |
|---|---|---|---|
| Identity record; eligibility after setup; enrolment audit; terminal disable; two-level expiry; Custodian sees no content; external via the same model; EXTERNAL badge; account check at principal load; clock-based expiry; resend; delivery display; intake endpoint; idempotency; rate bound; observable compartment and safe rendering; management panel; closure blockers; reopening; abuse suite; route coverage; stale sessions; journeys; axe; backwards-clock guard; status wrap; Now block; reject consequence | Grouped create plus eligibility; enrolment code; revoke as membership end; external default expiry; one People section; notification request record; no auto retry; intake key bound to a human; observable as record; derived related incident; observables on Evidence; median definition; engines not brands; device emulation; per-section top N | SCIM/sync/self-registration; external portal; push logout; rota; recipient lists, templates, inbox; vendor connectors; API attachments; enrichment, feeds, export; charts, trends; dashboard framework; erasing history; NTP client | Live-session effect of role and membership changes; membership expiry without timer; 72-hour phase; changing Majors; channel coverage; operational alerts; accidental closure; state after reopen; T-199 amendment; R-10 to R-15; UI-only enforcement; browser driver; forward clock and resume; wrong-but-synchronised; DST; header logo; signed-out nav; access approval wording |

---

## 6. Proof harness design (R-10 to R-17, R-19, R-21)

One rule: **CRAB may become slower; it must not become wrong.** Every harness ends with the same correctness oracle:

```text
oracle():
    crab verify --ledger <export> --keyring <keyring>          # chain and signatures intact
    snapshot projections → rebuild → snapshot → must be identical
    every evidence/corpus blob re-hashes to its recorded SHA-256
    every HTTP 2xx mutation in the client log has exactly one ledger record (no duplicates, none missing)
    no client saw content outside its principal's scope
```

| Harness | Shape | Where it runs |
|---|---|---|
| Load (`R-10`) | Go program under `cmd/crab` behind the `proof` build tag. N simulated users (50 by default, each with its own principal and scope) drive the Bafflewick-scale fixture through HTTP. It records p50/p95/p99, errors, CPU, RSS, goroutines and `pg_stat_activity` counts. No k6 and no new module | CI (short profile) and the laptop and Pi rigs (full profile), results in build evidence |
| Soak (`R-11`) | The same driver at a steady rate. **Accelerated 72 hours**: real time for about 6 hours with the fake clock advancing 12× for timers and expiry, which is justified because the time-dependent paths use the clock. It samples memory, goroutines, sessions and connections every minute and fails on monotonic growth after warm-up | Scheduled workflow plus manual rig run |
| Destructive (`R-12`) | `crab serve` as a subprocess. Kill points use test-only failpoints compiled in with the `proof` tag (absent from release builds). `SIGKILL` at each point, restart, run the oracle | CI |
| DB and storage (`R-13`) | A local PostgreSQL in the job (`pg_ctl stop/start`), a small `tmpfs` for the export directory, and a size-limited tablespace for disk-full | CI with `sudo` |
| Backup/restore (`R-14`) | Populate → `crab backup` → restore to a fresh database → oracle plus a table-by-table diff | CI (Bafflewick 6 extended) |
| Upgrade (`R-15`) | Build `66b188a` and HEAD. Old binary populates, new binary migrates, oracle, continue scenario. Kill during migration. Corrupt migration fixture | CI |
| Abuse (`R-16`) | Go test over the mux route table × principal matrix | CI, part of `go test` |
| Browser/a11y (`R-17/18`) | Driver per U-19 against the end-to-end harness with the stress fixture | CI workflow (gating) plus visual-capture artefacts |
| Time (`R-19`) | Fake clock in the kernel and timed wait list: backwards, forwards, resume and DST cases | CI, part of `go test` |

---

## 7. Proposed slices

Each slice keeps CI green, Bafflewick at threshold and the visual preflight passing. Each ends with build evidence, a CHANGELOG entry and updated register rows. The testing rule applies throughout: **reproduce → write or extend a test → fix → rerun → regression**, with negative tests for security, state, permissions, evidence and approvals.

### S22 — Identity lifecycle and user administration (0.0.22-dev) — P0

1. `security.identity` record, projection, migration 018 (with back-fill for existing humans).
2. Role-eligibility maker rule; grouped create-plus-eligibility approval.
3. Enrolment codes, forced password set, reset, session deletion; `security.credential_issued`.
4. `account_usable` in authentication and `PrincipalAt`; clock-based expiry (F-2); session sweep (F-3).
5. Access admin *People* section.
6. TOTP enrolment and second factor for external people (U-16).
7. Readiness check: usable Custodians, warning below three (U-15).
8. Policy-reference ratchet (§3.17), held in every later slice.

**Exit:** T-214 to T-221 and T-248 to T-250, T-253 green; the Jane journey passes end to end through the browser runtime.

### S23 — External access, live revocation and abuse suite (0.0.23-dev) — P0/P1

1. External flag behaviour, default and maximum expiry, EXTERNAL badges.
2. Live-session revocation matrix across browser, API and MCP with multiple sessions.
3. Route-table abuse suite with the coverage test (R-16).

**Exit:** T-222 to T-226 green.

### S24 — Long incidents and closure (0.0.24-dev) — P0/P1

1. Bafflewick phase 10 *the long night* (72-hour fake-clock scenario). Threshold becomes 10/10.
2. Closure blockers for pending proposals and clocks; reopening (per the approved addendum).

**Exit:** T-227 to T-229 green; Bafflewick 10/10.

### S25 — Small operational additions (0.0.25-dev) — P1

1. Intake keys and `POST /api/v1/first-reports` with docs.
2. Observables on the Evidence screen.
3. Management summary panel.

**Exit:** T-230 to T-232 green.

### S25N — Notifications (version at the time) — P1, gated on Garage `notifier` release

Starts only when the reusable notifier with the needed channels has passed its own component tests (U-13). It can run in parallel with S26–S27 once unblocked.

1. Bump the notifier dependency. Map CRAB channel configuration to the notifier's.
2. Notification request record, delivery display, manual resend.
3. `crab.notification_routes` (supporting state) with the masked `security.notification_route_changed` audit record; route form on the person's own page and in Access admin *People*.
4. The five V1 events (§3.5): recipient resolution, scope-safe counts, debounce, shared-destination rule, `no route` outcome.
5. Exercise suppression and the scope rule on notice content re-proven.

**Exit:** T-233, T-234, T-251, T-252 green.

### S26 — Board hierarchy, approvals and mechanical UX (0.0.26-dev)

1. Stress fixture; the Now block; top N per section.
2. Approval wording: reject consequence, EXTERNAL, access in operator language. The approvals view model moves to the app tier first (F-7).
3. Mechanical defects found by S28's browser runs feed back here. Header logo and signed-out navigation per U-14.

**Exit:** T-235 to T-237 green.

### S27 — Proof harnesses (0.0.27-dev) — P1/P2

Load, soak, destructive, database/storage, backup/restore, upgrade and time harnesses (§6). Fix demonstrated defects only.

**Exit:** T-238 to T-244 green; measured results recorded in build evidence on the laptop rig. The Pi run uses `scripts/final-acceptance.sh`, extended.

### S28 — Browser, device and accessibility gate; handover (0.0.28-dev)

1. Real-browser journeys and axe checks (U-19); manual device checklist.
2. Final automated gate (brief §26) as one script; register fully dispositioned; Phase 4 human UAT script.
3. **Stop.**

**Exit:** T-245 to T-247 green; every register row has a result.

**Why this order.** P0 first, because it blocks real use. S22 must precede everything that needs real people: the external tests, the 72-hour phase and the abuse matrix. The small additions come before the UX pass so that the Board and Approvals are restyled once with their final content. The harnesses come late so that they measure the final shape, but upgrade (S27) uses migration 018 from S22 as its real test case.

---

## 8. Decisions

| ID | Decision | Status |
|---|---|---|
| U-12 | Contract changes: reopening, observables, management summary, machine API intake | **Decided 2 Nov:** narrowly scoped addendum, `docs/contract-addendum-v0.2.1.md`. Nothing beyond those four |
| U-13 | Notifier channels and sequence | **Decided 2 Nov:** reusable notifier first, then CRAB integration (S25N). No temporary CRAB plumbing |
| U-15 | Dual control with too few Custodians | **Decided 2 Nov:** dual control kept, any number of Custodians, readiness warns below three usable, no single-person emergency bypass |
| U-16 | MFA | **Decided 2 Nov:** TOTP required for external interactive people. Internal policy unchanged. Machine identities use scoped, revocable, rotatable credentials |
| U-19 | Browser driver | **Decided 2 Nov:** Playwright plus axe-core, test-only, outside the binary and runtime, pinned |
| U-20 | Notification events | **Decided 2 Nov:** the five attention/action events in §3.5; never authoritative |
| U-14 | Header logo and signed-out navigation | Default: adopt `go-app-shell` v0.2 when supplied. If it is not supplied by S26, record both as known shell limits again rather than adding CRAB layout CSS |
| U-17 | Custodian's view of a person's incident memberships | Default: show incidents the Custodian can see; for others show *n other incidents* with no names |
| U-18 | Maximum account lifetime for external people | Default: 30 days, renewable by a governed change; no maximum for internal people |
| U-21 | API path | Default: `POST /api/v1/first-reports` |
| U-2 | Display timezone | Default: UTC, labelled |

---

## 9. Proposed test rows

These rows are appended to `TEST-ACCEPTANCE-NOTES.md` as §5 when the plan is approved. No existing test is removed or weakened. T-199 is amended as noted.

| ID | Area | Then |
|---|---|---|
| T-214 | identity | Creating, modifying, suspending, reactivating or disabling a person needs a Custodian maker and a different Custodian confirmer; a non-Custodian is refused; disabled is final; every change is a ledger version |
| T-215 | identity | Role eligibility after setup: Custodian maker, different confirmer, active identity required; incident membership refused for a person without eligibility (existing rule) |
| T-216 | credentials | An enrolment code is shown once, stored hashed, expires, works only for an active, unexpired identity, forces a compliant password before any session, and its issue is recorded without the secret |
| T-217 | credentials | Reset invalidates the old password and every session of that person on all surfaces |
| T-218 | account state | Suspension, disablement and account expiry refuse sign-in **and** refuse the next request of every existing session on browser, API and MCP |
| T-219 | expiry | Membership and account expiry are evaluated against the governed clock; moving the clock past expiry refuses access with no timer |
| T-220 | sessions | Expired sessions are removed by the sweep; the session count is bounded over a soak |
| T-221 | journey | The Jane journey (brief R-01, steps 1–10) passes end to end through the browser runtime, including immediate revocation |
| T-222 | external | An external person requires an account expiry; EXTERNAL appears in words on Board roles, Access, Approvals and the Brief; they see only their incident and compartments |
| T-223 | live revocation | With two or more simultaneous sessions per person, role reduction, incident removal, compartment removal and explicit revocation take effect on the next request in every session; an in-flight form POST is refused in frame |
| T-224 | abuse | For every route, each principal in the abuse matrix using foreign identifiers gets the same refusal as for a nonexistent identifier, with no content |
| T-225 | abuse coverage | A route registered without an abuse case fails the build |
| T-226 | stale session | Tokens kept after logout, revocation, suspension and expiry are refused on browser, API and MCP |
| T-227 | long night | Bafflewick phase 10: a 72-hour scenario with shifts, joiners, leavers, external, legal and comms people, expiring access, changing Majors and handovers needs no shared account, SQL or break-glass; the ledger verifies and projections rebuild identically |
| T-228 | closure | Closure is refused while pending proposals or unfired clocks exist and names each blocker; one person cannot close |
| T-229 | reopening | Reopening needs a reason and a different confirmer; history and closure outputs are kept and marked superseded; expired access stays expired; it is never self-approvable |
| T-230 | intake API | A valid intake key creates a first report attributed to the key and its accountable human; the key can do nothing else; an expired, revoked, rotated-out or wrong key is refused identically; rotation overlap is bounded; over-rate returns `429`; a repeat `external_ref` returns the original report |
| T-231 | observables | Observables are recorded, versioned and compartment-scoped; values render as text only; a related incident appears only when the viewer can read both |
| T-232 | management | The summary counts only incidents the viewer can see; median decision-loop time matches a hand-computed fixture; exercise incidents are excluded |
| T-233 | notification | Each consequential notification shows requested, attempted and outcome; *not retried automatically* is stated; a request recorded before a crash is visible after restart |
| T-234 | resend | Resend requires `notify`, uses the unchanged approved content, targets failed channels only, records new attempts linked to the old ones, and remains suppressed in exercise |
| T-235 | board | With the stress fixture, the Now block lists the top items in the stated order, and every section shows its most urgent items first with access to the full list |
| T-236 | approvals | Every approval shows what, current, proposed, proposer (with EXTERNAL), reason, and confirm and reject consequences; access approvals read in operator language |
| T-237 | mechanical UX | The status line never truncates abnormal states at 360 px; every defect from the browser runs has a test |
| T-238 | load | At 50 users and the brief's data volumes, the correctness oracle passes; there are no 5xx except declared back-pressure; p50/p95/p99, CPU, memory and connections are recorded |
| T-239 | soak | Accelerated 72 hours: no monotonic growth in memory, goroutines, sessions or connections after warm-up; timed waits fire exactly once; the oracle passes |
| T-240 | destructive | At every kill point, after restart the operation is committed once or not committed; the oracle passes; orphan blobs are reported, not hidden |
| T-241 | database/storage | With PostgreSQL down or disk full, no mutation returns success without commit; the failure is visible; service resumes when the dependency returns |
| T-242 | backup/restore | A populated restore reproduces every projection table and blob hash exactly |
| T-243 | upgrade | Old to new with populated state migrates, verifies and continues; an interrupted migration leaves the old schema; a failed migration refuses start clearly |
| T-244 | time | Backwards clock writes are downgraded and marked; wake after suspend fires overdue waits once, marked late; durations survive DST |
| T-245 | browser | Journeys pass on Chromium, Firefox and WebKit plus phone emulation: refresh and back never resubmit, multiple tabs, expired session with unfinished form, interrupted upload, 200% and 400% zoom, keyboard only |
| T-246 | accessibility | axe reports no serious or critical violations on any screen in light or dark |
| T-247 | gate | The final automated gate script passes and every register row has a disposition |
| T-248 | Custodian edge cases | A suspended, expired or locked-out Custodian cannot confirm; a proposal whose maker became unusable can be rejected but not confirmed; no path lets one Custodian complete a governed identity, eligibility or intake-key change alone, including through a single-person window |
| T-249 | Custodian readiness | Readiness warns, with the count, when fewer than three Custodians are usable; operation continues |
| T-250 | external MFA | An external person cannot get a session without TOTP enrolment and a valid code; reset clears enrolment; internal sign-in is unchanged; intake keys never prompt for TOTP |
| T-251 | notification events | Only the five V1 events notify, each to the person who must act; notice content contains nothing the recipient may not read and never restricted-compartment content; no timeline, comment, evidence or ordinary change notifies; a notice cannot approve anything |
| T-252 | notification routes | The address is never in the ledger, export or outcome records, only a masked hint; disabling a person deletes their route; a route is never used for recovery; shared destinations receive only content-free event 4 notices when enabled; a person without a route gets `no route` and the item stays on their Command Summary |
| T-253 | tier placement | The policy-reference ratchet does not increase and is zero in every file touched by this remediation; every §3.17 rule has app/domain/security tests; web tests assert rendering only |

**Amended:** T-199 becomes *Command Summary uses a fixed number of queries, independent of incident count*.

---

## 10. Stop signs

- a second identity store, user table outside the ledger, or IdP sync;
- a principal that does not pass through `account_usable`;
- an independent non-human principal (intake keys are bound to a human and limited to one action);
- automatic notification retry or a delivery queue;
- an attachment on the intake API;
- observable enrichment, feeds or export formats;
- a chart library, dashboard framework or configurable Board;
- layout CSS or `url(` in `crab.css`, or JavaScript in the product;
- a test-only dependency in the product build or binary;
- any count, link, error or timing that reveals an unauthorised incident, person or compartment;
- self-approval of identity, eligibility, intake keys, reopening or closure;
- a deleted or weakened test;
- any single-person bypass of Custodian dual control;
- temporary CRAB notification adapters ahead of the reusable notifier;
- Playwright or Node anywhere in the Go module, binary or runtime;
- a business rule, eligibility decision, count rule or message composition in `internal/presentation` or `cmd/crab` runtime adapters;
- a notification address in the ledger, an export or an outcome record;
- feature work after S28.
