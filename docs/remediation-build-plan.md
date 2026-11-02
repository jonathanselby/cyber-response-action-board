# CRAB — Post-UAT remediation build plan

**Date:** 2 November 2026
**Method:** *Jonno's Garage — Build Session Control* (the execution method; not reinvented here)
**What to build:** `docs/uat-remediation-triage.md` (decisions §0, tier placement §3.17), `docs/remediation-register.md`
**Baseline:** `66b188a`, 0.0.21-dev, locally proven green on 2 November (§3)

> **One coherent chunk at a time. Finish and gate it. Publish it in two commits. Then move on.**

---

## 1. Sources of truth

In order of precedence. If something in the build seems to need a different answer, check what these documents require first. If they genuinely need to change, **stop and raise the conflict**. Never substitute quietly.

1. `PRD.md` V0.2 plus `docs/contract-addendum-v0.2.1.md` (A1–A4) **once approved**.
2. `EDD.md` V0.4 (frozen) plus the same addendum.
3. `docs/uat-remediation-triage.md`: decisions (§0), pseudocode (§3), tier placement (§3.17), test rows (§9), stop signs (§10).
4. `docs/remediation-register.md`: every finding with its disposition.
5. Existing build discipline: `BUILD-NOTES.md`, `UI-SHELL-BUILD-NOTES.md`, `UAT2-BUILD-NOTES.md` stop signs. They still apply.
6. `docs/notifier-v0.2-requirement.md`, for chunk C12 only.

**Approval gate.** C1–C4 need no contract change: identity, external access and revocation are already PRD requirements. C5–C7 (reopening, intake, observables, summary) do not start until the addendum is approved.

---

## 2. Current state

This block is refreshed at the end of every chunk. If it cannot be stated in these lines, stop and reset (§9).

```text
Authoritative design   PRD V0.2 + EDD V0.4; addendum V0.2.1 APPROVED (2 Nov 2026)
Branch / implementation main @ C6 merge (0.0.27-dev)
Last green gate        C6 — local and GitHub CI: gofmt, vet, race tests + PostgreSQL, ratchet, abuse suite, Bafflewick 10/10, visual preflight
Known defects          P0 passed; R-09 (C5) and R-06 (C6) pass; F-11 host-clock reads → C10; R-05 blocked on notifier v0.2
Next coherent chunk    C7 — observables and management summary (addenda A2, A3)
UAT started            no (Phase 4 human UAT follows C13)
Release frozen         no
```

---

## 3. Gate and environment

### 3.1 The gate

The same commands CI runs, run locally before every publication. **GitHub CI remains the authority.** The local gate exists so that nothing is published that is already known to fail.

```text
gofmt -l ./cmd ./internal                     # must print nothing
go mod tidy && git diff --exit-code go.mod go.sum
go vet ./...
go test -race -count=1 ./...                  # with CRAB_TEST_POSTGRES_ADMIN_URL
./scripts/build.sh ./bin/crab                 # plus: crab version matches VERSION
./scripts/bafflewick-acceptance.sh            # threshold per chunk (9/9, then 10/10 from C4)
./scripts/visual-acceptance-preflight.sh
+ chunk-specific gates as they are added (abuse suite, Playwright, proof harnesses)
```

From C1 the gate also includes `TestTierImportBoundaries` plus the policy-reference ratchet (T-253).

### 3.2 This build environment (operational, not architectural)

- **Go 1.27.2** was built from the official source tag, because the toolchain download is blocked here. `GOTOOLCHAIN=local`.
- **Module fetching:** `GOPROXY=direct`. `golang.org/x/*`, `gopkg.in/yaml.v3` and `gopkg.in/check.v1` resolve to their GitHub mirrors through a **local `go.work` that is git-excluded and never committed**. `go.mod` and `go.sum` are untouched.
- **PostgreSQL 16** runs locally. CI uses 17, so any version-specific difference is settled by CI.
- **Playwright 1.63 and Node 22** are present for C8.

None of this enters the repository. It is the "transfer problems are not architecture" rule (control §18).

### 3.3 Publication

Pushing from this session is currently **refused** (the repository is attached read-only).

- **Preferred:** install the Claude GitHub App on `cyber-response-action-board`, or re-link GitHub in claude.ai settings. That is one action, after which every chunk follows **branch → CI → main** with no handoff at all.
- **Fallback:** one file per chunk, a git bundle holding the chunk's two commits, plus one command:

  ```text
  git fetch <chunk>.bundle uat3/<chunk>:uat3/<chunk> && git push origin uat3/<chunk>
  ```

  Then open a pull request from that branch. No patches, fragments or line edits.

---

## 4. The chunk loop

Every chunk follows the same steps. Nothing is skipped and nothing is added.

| Step | What happens | Done when |
|---|---|---|
| 1. Entry | Previous chunk merged to `main` and CI green; state block current; chunk dependencies met | Stated in one line |
| 2. Design check | Re-read the triage rows and pseudocode for this chunk. Confirm placement against §3.17: presentation renders, application decides, data stores. Write any pseudocode the triage does not already hold *before* code | No open design question; otherwise **stop and ask** |
| 3. Reproduce | For defects: a failing test that shows the defect. For new capability: failing acceptance tests from the triage rows (T-ids). Negative tests are mandatory for security, state, permissions, evidence and approvals | Tests fail for the right reason |
| 4. Build | The whole chunk on one branch `uat3/<chunk>`. Smallest sufficient design. No work belonging to another chunk | Tests pass |
| 5. Local gate | §3.1 in full | All green |
| 6. Commit 1: implementation | One commit: code, migrations, tests, the new `TEST-ACCEPTANCE-NOTES.md` rows, embedded templates | Formatter output preserved exactly; no hand edits after formatting |
| 7. CI | Publish the branch; wait for every workflow | Green. A red result is fixed **in this chunk** before anything else |
| 8. Review gate | Red-team review of the **stable** branch. Every finding is classified (§5) | Only genuine defects fixed; rerun 5 and 7 |
| 9. Commit 2: evidence | `docs/build-evidence-<version>.md`, `CHANGELOG.md`, `VERSION` **and `internal/buildinfo` default version** (T: `TestDefaultVersionMatchesVERSIONFile`), register rows → `pass` with evidence, README build-status line, state block. **The full local gate runs again before this push** | Evidence describes what was proven, nothing more |
| 10. Merge | Branch → `main`; CI green on `main` | Repository clean; next chunk may start |

**Size rule.** A chunk holds one family of architectural decisions. If, during step 4, it starts to hold two, or becomes hard to reason about, split it at the next natural engineering boundary. Finish the first part through step 10 before continuing.

**Commits.** Normally two per chunk. A real defect found by review is folded into commit 1 before step 9, while the branch is unmerged. After merge, a tiny proven defect may take one small commit. No micro-commits.

---

## 5. Review gate

The reviewer is a red team against a stable implementation, not a new design phase. The reviewer can be the product owner's external LLM review, or an independent review agent with no access to the build conversation if the product owner asks for one.

Each finding is recorded in that chunk's build evidence:

| Question | If no |
|---|---|
| Is it a real defect, observed in code, tests, screenshots or UAT? | Reject: speculative |
| Does the fix keep the locked architecture and §3.17 placement? | Reject, or raise as a design conflict |
| Is it in this chunk's scope? | Add to the register for its proper chunk |
| Can it be solved more simply? | Use the simpler fix |

Late chunks narrow the work. A review may not add capability by accumulation.

---

## 6. Chunks

Versions advance one dev step per chunk. The release version is decided after human UAT (§8).

### C0 — Publish the plan (docs only)

- **Content:** triage, register, addendum (draft), notifier requirement, this plan. One squashed commit on `uat3/remediation`.
- **Gate:** links workflow green (docs only).
- **Exit:** on `main`. The product owner approves or amends the addendum, and the approval is recorded in the addendum's status line in C5's evidence commit.

### C1 — Identity and credential core (0.0.22-dev) — P0, security boundary

- **Covers:** R-01 (core), R-03 (principal rule), F-1, F-2, F-3, U-15 rules. No browser screens.
- **Build:**
  - `security.identity` record, decode and validation (`security/access`);
  - projection `crab.identities` plus **migration 018** with back-fill of `active` identities for existing role-eligible humans;
  - bootstrap writes identities in the same atomic batch;
  - eligibility maker rule (Custodian, distinct usable confirmer);
  - `account_usable` used by authentication **and** `PrincipalAt`, with the clock passed in (F-2);
  - session deletion inside the projection transaction on suspension or disablement;
  - session sweep on the timed-wait cadence (F-3);
  - enrolment codes, forced password set, reset (supporting-state columns, `security.credential_issued`);
  - TOTP enrolment and verification for `external=true` (existing `authn` functions);
  - the policy-reference ratchet test (§3.17).
- **Tests:** T-214–T-220, T-248, T-250 (service and data level), T-253. Bafflewick 9/9 unchanged.
- **Not in this chunk:** People screens, external defaults, abuse suite.

### C2 — User administration in the browser (0.0.23-dev) — P0, one complete presentation capability

- **Covers:** R-01 (journey), U-15 readiness, U-17 default.
- **Build:**
  - Access admin *People* section (list, add, detail, suspend, reactivate, disable, reset, revoke-all as grouped membership-end proposals);
  - first sign-in flow (enrolment code → password → TOTP for external people);
  - grouped create-plus-eligibility approval;
  - readiness warning below three usable Custodians.
  - **App-tier view models only.** Templates render flags.
- **Tests:** T-221 (the Jane journey end to end through the browser runtime), T-249. Web tests assert rendering and refusal mapping only.
- **Exit extra:** a visual-capture run including the People screens.

### C3 — External access, live revocation and abuse suite (0.0.24-dev) — P0/P1, security proof

- **Covers:** R-02, R-03 (proof), R-16.
- **Build:**
  - external default and maximum expiry (U-18), EXTERNAL in words in view models;
  - live-session revocation matrix (≥2 sessions per person; browser, API, MCP);
  - route-table × principal abuse suite with the coverage test.
- **Tests:** T-222–T-226.

### C4 — The long night (0.0.25-dev) — P0 closure

- **Covers:** R-04.
- **Build:** Bafflewick phase 10, a 72-hour fake-clock scenario through the browser runtime. The threshold becomes 10/10, with documentation updated in the evidence commit.
- **Tests:** T-227.
- **Exit extra:** a **P0 checkpoint**, with R-01–R-04 at `pass` in the register. If any P0 is not passing, nothing in C5 onward starts.

### C5 — Closure and reopening (0.0.26-dev) — P1, needs approved addendum A1

- **Covers:** R-09.
- **Build:** named closure blockers (pending proposals, unfired clocks) returned by `domain.ClosurePrerequisites`; `incident.reopening` governed record; supersession of closure and stand-down; Close-out renders what it is given.
- **Tests:** T-228, T-229.

### C6 — Machine first-report intake (0.0.27-dev) — P1, security boundary, addendum A4

- **Covers:** R-06.
- **Build:** `security.intake_key` (governed, hashed, expiry, revoke, rotation overlap ≤24 h); `POST /api/v1/first-reports`; per-key idempotency and rate bound; `docs/first-report-api.md`.
- **Tests:** T-230.

### C7 — Observables and management summary (0.0.28-dev) — P1, addendum A2 + A3

- **Covers:** R-07, R-08.
- **Build:** `incident.observable` on the Evidence screen with derived related incident (both-visible rule); the five-metric panel on the Command Summary; the T-199 amendment.
- **Tests:** T-231, T-232.
- **Split rule:** if either half grows, split into C7a and C7b.

### C8 — Browser and accessibility harness (0.0.29-dev) — P1, validation tooling

- **Covers:** R-17, R-18 (harness), R-21 fixture.
- **Build:**
  - `uat/` with pinned Playwright and axe-core (lockfile), its own workflow, outside the Go module;
  - journeys on Chromium, Firefox and WebKit plus phone emulation;
  - the command-board stress fixture in the end-to-end harness.
- **Product code changes:** none. Defects found are **recorded in the register** for C9, not fixed here.
- **Tests:** T-245 and T-246 run; expected to report defects.

### C9 — Board, approvals and mechanical UX (0.0.30-dev) — defects from C8 and UAT

- **Covers:** R-20, R-21, R-22, F-7.
- **Build:**
  - approvals view model moved to the app tier (F-7) **first**;
  - *If you reject* consequence, EXTERNAL, access approvals in operator language;
  - *Now* block and top-N per section (app decides, template renders);
  - status-line wrap;
  - every C8 defect;
  - logo and signed-out navigation if `go-app-shell` v0.2 is supplied (U-14), otherwise recorded as known shell limits.
- **Tests:** T-235–T-237. C8 is rerun and must reach green: T-245, T-246.

### C10 — Failure and recovery proofs (0.0.31-dev) — P1/P2

- **Covers:** R-12, R-13, R-14, R-15, R-19, F-4.
- **Build:**
  - correctness oracle;
  - `proof` build tag with failpoints (absent from release builds);
  - destructive kill harness;
  - PostgreSQL stop/start, tmpfs and disk-full tablespace;
  - populated backup/restore diff;
  - upgrade from `66b188a` to HEAD, including interrupted and failed migration;
  - backwards-clock guard and fake-clock time cases;
  - orphan-blob readiness check.
- **Fixes:** only defects the harnesses demonstrate.
- **Tests:** T-240–T-244.

### C11 — Load and soak (0.0.32-dev) — P1

- **Covers:** R-10, R-11, F-5.
- **Build:** load driver (`proof` tag), accelerated 72-hour soak, resource sampling. Fix **measured** bottlenecks only, with before and after numbers in evidence.
- **Pi run:** handed to the product owner as one command (`scripts/final-acceptance.sh`, extended) with one evidence file to return.
- **Tests:** T-238, T-239.

### C12 — Notifications (version at the time) — P1, gated on Garage `notifier` v0.2

- **Covers:** R-05.
- **Entry:** notifier v0.2 released with its component acceptance (`docs/notifier-v0.2-requirement.md`).
- **Build:** dependency bump; `crab.notification_routes` plus masked audit record; request record, delivery display, manual resend; the five events with scope-safe counts, debounce, shared-destination rule and `no route`.
- **Tests:** T-233, T-234, T-251, T-252.
- **Timing:** slots in after C7 whenever the notifier is released. It must merge before C13.
- **If the notifier is not available by C13:** R-05 is recorded as *deferred — blocked on notifier v0.2* only with the product owner's explicit agreement. It is never quietly left open.

### C13 — Final automated gate and UAT pack (release candidate)

- **Build:**
  - one gate script implementing brief §26 (every automated gate above in one run);
  - every register row with a disposition;
  - `docs/uat-phase4-script.md` (simple: a fresh install, two browser sessions, the representative journeys, about 90 minutes);
  - the Phase 3 outcome recorded in `docs/visual-acceptance.md` (F-6).
- **Then:** architecture **frozen**. No further feature work.

---

## 7. Human UAT (Phase 4)

- **Run:** `crab setup && crab serve`, then open the address shown. For a phone on the same network, `hostname -I` gives the laptop address. No helper machinery.
- **Representative coverage:** primary desktop browser, a second desktop browser, a real Android phone, tablet and phone widths, keyboard, dark mode, print. Exactly what was tested is recorded in `docs/visual-acceptance.md`, and nothing is claimed beyond it. For example: *Safari/iOS — not tested; no certification claimed*.
- **Findings:**
  - **Fix now:** broken behaviour, security, accessibility, layout, visible inconsistency, confusing interaction, real device defect.
  - **Do not add:** features, abstractions, frameworks, "while we are here", redesign.
  - Each fix goes through the chunk loop at the smallest size.

---

## 8. Close the build

When UAT passes and the final gate is green:

1. Promote the version (number decided then, in Garage style).
2. Refresh release evidence: build evidence, generated EDD and drift review, `final-acceptance-progress.md`, README status. No stale "pending" or "release candidate" wording, no old hashes.
3. Run the final CI.
4. Tag and release with **one-sentence release notes**, matching the existing Garage style.
5. **Stop.**

---

## 9. Drift reset

If the session starts to deteriorate (state cannot be stated in the §2 block, chunks bleed into each other, or review is adding scope):

> Return to the latest locked PRD, EDD, addendum and last green chunk on `main`. State what is complete and proven, and the single next chunk. Discard any method invented after the last green point. Do not redesign. Do not micro-commit. Build the chunk completely, gate it, publish it in two commits, then continue.

If it cannot recover, start a new session with: PRD, EDD, addendum, triage, register, this plan, the latest build evidence and the Build Session Control file.

---

## 10. Dependency map

```text
C0 ─► C1 ─► C2 ─► C3 ─► C4 ═╗ P0 checkpoint
                            ╠► C5 ─► C6 ─► C7 ─► C8 ─► C9 ─► C10 ─► C11 ─► C13 ─► UAT ─► release ─► stop
addendum approved ──────────╝                ▲
notifier v0.2 released ─────────────────► C12 ┘ (any time after C7, before C13)
go-app-shell v0.2 (optional) ───────────────────► C9
```
