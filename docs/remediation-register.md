# CRAB — Post-UAT remediation register

One row per finding, so no UAT finding disappears during implementation. Detail lives in `docs/uat-remediation-triage.md`. Resolved design decisions are in its §0; contract changes are in `docs/contract-addendum-v0.2.1.md`. Update **Result** as each slice lands: `open`, `in progress`, `pass` (with evidence file), `deferred (reason)` or `not built (reason)`.

| ID | Severity | Finding | Fix | Test | Result |
|---|---|---|---|---|---|
| R-01 | P0 | No complete way to add and administer users after setup | `security.identity` record; eligibility maker rule; enrolment codes and reset; `account_usable`; Access admin *People*; readiness warns below three usable Custodians (S22) | T-214–T-221, T-248, T-249 | pass — C1 core and C2 browser journey (`build-evidence-0.0.22-dev.md`, `build-evidence-0.0.23-dev.md`; T-221) |
| R-02 | P0 | External responders represented but not usable | External flag, required expiry, required TOTP (U-16), EXTERNAL badge on the same identity/scope model (S22–S23) | T-222, T-224, T-250 | pass — C1–C3 (`build-evidence-0.0.24-dev.md`; T-221, T-222, T-224) |
| R-03 | P0 | Access changes must take effect in active sessions | Per-request principal reload (exists, proven); account check at principal load; clock-based expiry; session deletion (S22–S23) | T-218, T-219, T-223, T-226 | pass — C1–C3 (T-218, T-219, T-223, T-226) |
| R-04 | P0 | 72-hour shift and surge operation | Bafflewick phase 10 *the long night* (S24) | T-227 | open |
| R-05 | P1 | Notifications too thin | **Blocked on Garage notifier release** (U-13; requirement in `docs/notifier-v0.2-requirement.md`). Then: per-person routes in supporting state with masked ledger audit; request record, delivery display, manual resend; no auto retry; the five V1 events (U-20), scope-safe counts, debounce, shared destinations content-free and event 4 only (S25N) | T-233, T-234, T-251, T-252 | open (blocked) |
| R-06 | P1 | Vendor-neutral first-report intake | Intake keys (scoped, revocable, rotatable, bound to an accountable human); `POST /api/v1/first-reports`; idempotent, rate-bounded, documented; addendum A4 (S25) | T-230 | open |
| R-07 | P1 | Indicators / observables | `incident.observable` record on the Evidence screen; derived related incident only when both are visible; addendum A2 (S25) | T-231 | open |
| R-08 | P1 | Management summary | Panel on the Command Summary over visible incidents; defined median decision loop; addendum A3 (S25) | T-232, T-199 amended | open |
| R-09 | P1 | Closure and reopening | Closure blockers for pending proposals and clocks; governed reopening; addendum A1 (S24) | T-228, T-229 | open |
| R-10 | P1 | Heavy operational use | Load harness and oracle; fix measured bottlenecks only (S27) | T-238 | open |
| R-11 | P1 | Soak | Accelerated 72-hour soak (S27) | T-239, T-220 | open |
| R-12 | P1 | Destructive failure | Failpoint kill harness with oracle (S27) | T-240 | open |
| R-13 | P1 | Database and storage failure | PostgreSQL stop/start, tmpfs, disk-full tablespace (S27) | T-241 | open |
| R-14 | P1 | Backup and restore on populated data | Extended Bafflewick 6 with full diff (S27) | T-242 | open |
| R-15 | P1 | Upgrade and migration | Old→new populated upgrade, interrupted and failed migration (S27) | T-243 | open |
| R-16 | P1 | Black-box access-control abuse | Route-table × principal abuse suite with coverage check (S23) | T-224–T-226 | pass — C3 (T-224, T-225); suite proven able to fail by mutation |
| R-17 | P1 | Browser and device acceptance | Playwright journeys on Chromium, Firefox and WebKit (U-19, test-only), phone emulation, manual device checklist (S28) | T-245 | open |
| R-18 | P1 | Accessibility automation | axe on every screen, light and dark (S28) | T-246 | open |
| R-19 | P2 | Abnormal time behaviour | Backwards-clock guard; fake-clock cases for forward jumps, resume and DST (S27) | T-244 | open |
| R-20 | — | Mechanical UX defects | Status wrap; defects from browser runs; logo and signed-out navigation per U-14 (S26) | T-237 | open |
| R-21 | — | Command-board stress | Stress fixture; Now block; top N per section (S26) | T-235 | open |
| R-22 | — | Approval comprehension | Reject consequence, EXTERNAL, access approvals in operator language (S26) | T-236 | open |
| F-1 | P1 | Browser login is password-only; TOTP and passkeys not wired | TOTP required for external people (U-16); internal policy unchanged (S22) | T-250 | pass — TOTP required for external people (C1) |
| F-2 | P0 | Membership expiry uses `time.Now()` not the governed clock | Pass the kernel clock to `PrincipalAt` (S22) | T-219 | pass (C1) |
| F-3 | P1 | Expired sessions never swept | Bounded sweep on the existing timed-wait cadence (S22) | T-220 | pass — swept on sign-in, bounded (C1) |
| F-4 | P1 | Blob written before its ledger record | Prove; report orphans in readiness; no automatic delete (S27) | T-240 | open |
| F-5 | P1 | `Frame` reads the configuration file on every request | Measure; load once at start if shown (S27) | T-238 | open |
| F-7 | P1 | Policy in the composition tier (`cmd/crab`): approval eligibility hints, kind exclusions and approval wording; 14 role/authorisation references across 6 files | Move into an app-tier approvals query (S26); policy-reference ratchet that only goes down and is zero in touched files (S22 onward); §3.17 tier placement | T-253 | open |
| F-8 | P1 | Browser sign-in has no failure rate limit (`authn.FailureLimiter` exists but is unwired); found in C1 review | Wire the existing limiter into sign-in and enrolment, keyed by person and source, with the generic refusal (C3) | C3 abuse suite | pass — C3 (`TestSignInFailuresAreLimited`) |
| F-9 | P1 | Access admin *Incident members* table lists every incident's members to any Custodian, inconsistent with U-17; found in C2 review | Show members only for incidents the Custodian may see; count the rest (C3) | C3 abuse suite | not built — contradicts EDD §9.6 (Custodian administers access metadata across incidents) and an existing test; People/table inconsistency raised with the product owner |
| F-10 | P0 | Found by the C3 abuse suite: a Major not on an incident could link a first report to it (write into a hidden incident; existence oracle; EDD §9.5) | Triage needs record authority in the incident, or for propose_declaration read authority or being the declaration's maker | T-224; `TestContinuityProductFlowThroughLedgerAndPostgres` negative case | pass — C3 |
| F-6 | — | Phase 3 human UAT outcome not recorded | Record it in `docs/visual-acceptance.md` (S22) | — | open |
