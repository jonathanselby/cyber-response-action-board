# CRAB build evidence — 0.0.29-dev

**Date:** 2 November 2026
**Status:** development checkpoint. Post-UAT remediation, chunk C8 (browser and accessibility harness). There are **no product code changes**; the defects found are recorded for C9.
**Governing documents:** `docs/uat-remediation-triage.md` §3.12, §3.15, §6; `docs/remediation-build-plan.md` C8; `TEST-ACCEPTANCE-NOTES.md` §5 (T-245, T-246, T-235f)
**Implementation commits:** `77808af`, review fix (harness only) in the following commit

## Scope

| Item | Result |
|---|---|
| `uat/`: test-only harness (U-19), outside the Go module, with Playwright 1.63.0 and axe-core 4.10.3 pinned by `package-lock.json` | done |
| `TestUATServe` (Go, test-only, skipped in the normal suite): seeds the demo data and the stress fixture and serves CRAB on loopback. A control API (row counts, ending a person's sessions, stop) lets the harness observe state without SQL in the browser | done |
| T-245 journeys on Chromium, Firefox and WebKit, with phone emulation (Pixel 7 on Chromium, iPhone 13 on WebKit) | done; 9 journeys |
| T-246: axe on 16 screens (2 signed out, 14 signed in) in light and dark, on all three engines | done |
| `uat/known-defects.json`: a recorded defect passes; a new failure, or a recorded case that now passes, fails the run | done |
| `.github/workflows/uat-browser.yml`: on every push; it requires all three engines and axe, and uploads results and screenshots | done |
| Board stress fixture (R-21): 41 actions, 13 unknowns, 7 clocks, 6 roles, 18 decisions, 4 compartments, 3 related incidents, 8 approvals waiting | done; `TestBoardStressFixture` |

## Results (GitHub CI, all three engines)

123 cases: 82 pass, and 41 fail as recorded defects.

| Journey | Chromium | Firefox | WebKit |
|---|---|---|---|
| Refresh and back never resubmit | pass | pass | pass |
| Two tabs; signing out in one ends the other | pass | pass | pass |
| Visible sign-out control | **F-13** | **F-13** | **F-13** |
| Expired session keeps the typed input or says what was lost | **F-14** | **F-14** | **F-14** |
| Interrupted upload leaves no evidence or blob | pass | — | — |
| No horizontal scrolling at 640 px (200%) | **F-15** | **F-15** | **F-15** |
| No horizontal scrolling at 320 px (400%) | **F-15** | **F-15** | **F-15** |
| Keyboard only: sign in, skip link, visible focus | pass | pass | pass |
| Phone: no overflow, no cut-off header text | **R-20** | — | **R-20** |
| Stress Board capture | about 26,600 px tall (R-21) | same | same |

The interrupted-upload journey tests the server, so it runs on one engine. Firefox has no mobile emulation.

**Accessibility.** Dark theme: no serious or critical violations on any screen. Light theme: `color-contrast` (serious) for `gas-muted` and `gas-help` text on 9 of the 16 screens on every engine (**F-16**). The other 7 pass.

## Review gate

This was a self red-team review of the harness.

| Finding | Classification | Action |
|---|---|---|
| Sign-in, form submits and keyboard Enter waited for a load state that was already reached, so Firefox and WebKit checked the page before navigating ("Enter did not sign in", one flaky sign-in) | real harness defect | fixed: they now wait for the navigation; all three engines pass the keyboard journey |
| The phone journey's full-page stress screenshot exceeded WebKit's 32,767 px limit | real harness defect | fixed: it captures the first screen only |
| The local development container has only Chromium and no axe-core or npm registry, so the full run is CI-only | environment | the harness skips unavailable engines locally; `UAT_REQUIRE_ALL=1` makes CI fail if anything is missing |
| The lockfile was generated in CI; artifact download is blocked here, so it was copied from the job log | provenance | it carries the npm integrity hashes, and `npm ci` verifies it on every run |

## Gates

| Gate | Build environment | GitHub CI |
|---|---|---|
| gofmt, vet | clean | pass |
| `go test -race -count=1 ./...` | pass | pass |
| Bafflewick | **10/10** | pass |
| visual preflight | pass | pass |
| uat-browser (new) | Chromium journeys: 10 cases, 5 recorded defects, 0 unexpected | pass: 123 cases, 41 recorded defects, 0 unexpected |
