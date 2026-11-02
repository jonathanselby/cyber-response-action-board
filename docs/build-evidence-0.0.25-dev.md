# CRAB build evidence — 0.0.25-dev

**Date:** 2 November 2026
**Status:** development checkpoint. Post-UAT remediation, chunk C4 (the long night), and the **P0 checkpoint**.
**Governing documents:** `docs/uat-remediation-triage.md` §3.4; `docs/remediation-build-plan.md` C4; `TEST-ACCEPTANCE-NOTES.md` §5 (T-227); `docs/bafflewick-acceptance.md` phase 10
**Implementation commit:** `dad5a4b`

## Scope

| Item | Result | Tests |
|---|---|---|
| Clock-controlled browser harness (test only: governed clock replaced through the runtime factory) | done | — |
| 72-hour scenario through the browser runtime | done | T-227 |
| Bafflewick phase 10; threshold 10/10 | done | `scripts/bafflewick-acceptance.sh` |

## Scenario timeline (governed clock)

| Time | What happens | What is asserted |
|---|---|---|
| H+0 | Incident declared; nina (Major), dave and cora (Minor) onboarded and given access; alice incident commander | — |
| H+1 | jane (external) onboarded with TOTP; six-hour incident access and FORENSICS read-in; records a fact | — |
| H+7 | jane's access ends | jane refused; her fact remains |
| H+9 | eight-hour session limit | old session refused; everyone signs in again |
| H+12 | night shift: handover; commander alice → nina | Board shows nina; handover objectives, commitments and unknowns present |
| H+13 | alice steps down from Major to Minor (Custodians bob → alice) | alice can no longer propose; her earlier proposal is still decided by nina |
| H+20 | dave leaves: removed by the sponsor, then suspended | dave's fresh session is refused after removal |
| H+24 | lex (external counsel) onboarded; LEGAL controller and read-in | lex reaches LEGAL; cora (communications) does not |
| H+30 | cora drafts a customer notice | visible to the commander |
| H+36 | day shift: handover; commander → bob; lex signs in | lex's fresh session works before the account ends |
| H+41 | lex's account ended at H+40 | live session and sign-in both refused |
| H+72 | end | Board shows bob as commander and the history; ledger verifies; every projection rebuilds byte-identically |

No step uses a shared account, SQL or break-glass. The only direct database reads are the final verification and the rebuild fingerprint.

## Mutation check

Ignoring incident-membership expiry alone is **not** caught. Incident membership and the matching GENERAL compartment row expire independently, so one filter covers for the other (defence in depth). Removing both filters fails the scenario at H+7 (*jane still sees INC-LN after her access expired*).

## Review gate

This was a self red-team review.

| Finding | Classification | Action |
|---|---|---|
| The governed clock exposed 52 remaining direct host-clock reads in composition, presentation and one app path (`cmd/crab` 47). They stamp some records and displays with host time rather than governed time | real, out of C4 scope | register F-11 → C10 (time behaviour) |
| Not exercised over 72 hours: obligation clocks, issued communications and notifications | notifications blocked on notifier v0.2; obligations and issue are covered elsewhere (Bafflewick 3) | recorded |

## P0 checkpoint

| Finding | Result |
|---|---|
| R-01 user administration | pass (C1–C2) |
| R-02 external responders | pass (C1–C3) |
| R-03 access changes and active sessions | pass (C1–C3) |
| R-04 shift and surge operation | pass (C4) |

All P0 findings pass. Per the plan, C5 onward may start, subject to approval of the contract addendum.

## Gates

| Gate | Build environment | GitHub CI (`dad5a4b`) |
|---|---|---|
| gofmt, vet | clean | pass |
| `go test -race -count=1 ./...` | pass | pass |
| Bafflewick | **10/10** | pass |
| visual preflight | pass | pass |
