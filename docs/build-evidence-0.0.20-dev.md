# CRAB build evidence — 0.0.20-dev

**Date:** 26 October 2026  
**Status:** development checkpoint. UAT Phase 2 response, Stage 5 (single-person emergency approval window).  
**Governing notes:** `UAT2-BUILD-NOTES.md` §6, `TEST-ACCEPTANCE-NOTES.md` §4

## Scope

| Item | Result | Tests |
|---|---|---|
| Governed `system.single_person_window` record: start, end, named Majors, covered kinds, reason, active | done | T-208 |
| Opening, extending or closing early needs a second person; the maker can never confirm | done | T-208 |
| Refused: more than 24 hours, a start before the proposal time, a non-Major, an uncovered kind | done | T-208 |
| Covered kinds limited to decisions, actions, operational roles, severity, communications, report cadence, obligations and stand-down | done | T-208 |
| Never covered: access, isolate, custodian, organisation configuration, the window itself, closure, retention | done | T-208 |
| A named Major confirms their own covered proposal inside the window, only with a reason | done | T-209 |
| Outside the window, for an uncovered kind or as a non-named Major the self-approval is refused | done | T-209 |
| `SELF-APPROVAL` shown on Approvals, Board, timeline, Incident Brief and Command Summary; status line shows *Single-person window open* | done | T-209 |
| Self-approval under an unverified clock carries both markers | done | T-210 |

## Design notes

- One decide route: self-approval is the same `/decide` form with a reason field, offered only when the window covers the viewer, the kind and the current time.
- The window check reads the kernel clock; the service sets the self-approval context only for incident, severity, communication, obligation and recovery decisions, so access and organisation decisions cannot reach it.
- Self-approved severity changes and communication issues are now written to the timeline (previously only the Board badge showed them).
- No migration: windows are organisation records (Stage 3 tables) keyed by window identifier.

## Bafflewick

Phase 9 added: *degraded authority* — unverified time, single-person window, related incidents and organisation paper. Threshold is now 9/9 (`docs/bafflewick-acceptance.md`).

## Gates run in the build environment

| Gate | Result |
|---|---|
| `gofmt -l ./cmd ./internal` | clean |
| `go vet ./...` | pass |
| `go test -race -count=1 ./...` with PostgreSQL 16 | pass |
| `scripts/visual-acceptance-preflight.sh` | pass |
| `scripts/bafflewick-acceptance.sh` | 9/9 |
| `scripts/build.sh` linux/amd64 | pass; 18,100,487 bytes |
| `scripts/build.sh` linux/arm64 | pass; 16,777,376 bytes |

## Still open

- No path to add accounts or role eligibility after setup (see 0.0.19-dev evidence). A window naming one Major is therefore the realistic way to keep going when the second setup person is unavailable.
- T-211 to T-213 remain human acceptance at Phase 3 UAT.
