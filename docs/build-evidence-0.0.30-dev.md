# CRAB build evidence — 0.0.30-dev

**Date:** 2 November 2026
**Status:** development checkpoint. Post-UAT remediation, chunk C9 (Board, approvals and mechanical UX, plus the defects C8 found).
**Governing documents:** `docs/uat-remediation-triage.md` §3.14–§3.17; `docs/remediation-build-plan.md` C9; `docs/contract-addendum-v0.2.1.md` A1 (T-228, *unfired clocks*); `TEST-ACCEPTANCE-NOTES.md` §5 (T-235–T-237, T-245, T-246)
**Implementation commits:** `f600f5b`, review fix `5d3c259`

## Scope

| Item | Result | Tests |
|---|---|---|
| F-7: `app.ApprovalsService` decides what a person sees and may decide, and owns all approval wording. `app.CapabilitiesOf` provides the capability hints the browser uses. The Major-only triage rule moved from `cmd/crab` into the app | done; policy-reference ratchet at **zero** for `cmd/crab` and `internal/presentation` | T-253, T-236 |
| R-22: proposer name, EXTERNAL and role; *If you confirm* and a specific *If you reject*; access approvals in operator language | done | T-236 `TestApprovalComprehension` |
| R-21: Now block (`app.BoardNow`, at most 7 items, in the stated order); the most urgent five of actions, unknowns, decisions, clocks and timeline, with the rest behind *All N …* (`app.OrderBoard`); the template only renders | done; the stress Board went from about 27,800 px to 18,200 px tall | T-235 `TestBoardNowAndSections` |
| F-12: an active overdue obligation clock blocks closure until it is waived, and is named as overdue. Per the contract's *unfired clocks*; there is no "met" state | done | T-228 (extended) |
| F-13: *Sign out* in the navigation opens a one-button page; the session still ends only by POST | done | `TestSignOutIsOffered`, T-245 |
| F-14: an expired session records nothing, says so, and gives back the typed text, leaving out routing fields and secrets | done | `TestExpiredSessionGivesBackInput`, `TestUnsavedFieldsLeaveOutSecretsAndRouting`, T-245 |
| F-15: Readiness table in a stacked panel; Approvals filter options are incident references (WebKit sizes a select to its longest option); related-incident choice is a radio list. No layout CSS | done | T-245 reflow at 640 and 320 px |
| F-16: light `--gas-text-muted` changed from #687786 to #56636f (Storm Blue hue). The palette lock now checks AA contrast against every light background | done | palette test, T-246 |
| R-20: status line wraps (`white-space:normal`); the phone journey now detects vertical clipping too | done | `TestStatusLineWraps`, T-245 |
| U-14: logo and signed-out navigation | **not built**: needs go-app-shell v0.2, which was not supplied | recorded as a known shell limit |

## Results

The browser harness (GitHub CI) passes on Chromium, Firefox and WebKit with phone emulation: **123 cases, 0 failing**. `uat/known-defects.json` is empty.

## Review gate

This was a self red-team review.

| Finding | Classification | Action |
|---|---|---|
| WebKit: the Approvals filter `<select>` was 12 px wider than a 320 px screen. WebKit does not shrink a select below its longest option | real defect (found by CI) | fixed in `5d3c259` without layout CSS; the related-incident select changed to radios for the same reason |
| Removing the adapter's Major check on triage would have let Minors triage | real; caught before commit | the rule moved into `app.ContinuityService.TriageFirstReport` |
| A board limit of 0 (an unset view) would have hidden every item | real; caught before commit | `head`/`tail` treat 0 as "no folding"; the templates guard the *All N* block |
| Changing a locked palette colour | a deliberate accessibility fix | lock updated, with a contrast assertion so the value cannot regress |

## Gates

| Gate | Build environment | GitHub CI (`5d3c259`) |
|---|---|---|
| gofmt, vet | clean | pass |
| `go test -race -count=1 ./...` | pass | pass |
| Bafflewick | **10/10** | pass |
| visual preflight | pass | pass |
| uat-browser | Chromium: 10 cases, 0 failing | **123 cases, 0 failing** |
