# CRAB build evidence — 0.0.28-dev

**Date:** 2 November 2026
**Status:** development checkpoint. Post-UAT remediation, chunk C7 (indicators and management summary, contract addendum A2 and A3).
**Governing documents:** `docs/contract-addendum-v0.2.1.md` A2 and A3 (approved 2 November 2026); `docs/uat-remediation-triage.md` §3.7, §3.8; `docs/remediation-build-plan.md` C7; `TEST-ACCEPTANCE-NOTES.md` §5 (T-231, T-232, amended T-199)
**Implementation commit:** `2265865`

## Scope

| Item | Result | Tests |
|---|---|---|
| `incident.observable` covering `ip`, `domain`, `url`, `hostname`, `email` and `hash`. Each is validated and normalised per type (IPv6 compressed, IPv4-mapped unwrapped, names lower-cased, hashes lower-case hex of 32, 40, 64 or 128 characters) | done | T-231, `TestNormaliseObservable` |
| Immutable `Record`: a status change is a new version that keeps the incident, compartment, type, value and first-seen time. The projection enforces this. One projection table, `crab.observable_versions` | done | T-231 |
| Evidence screen *Indicators* section. Values are escaped text, never links; there is no lookup, enrichment, feed or export | done | T-231, `TestIndicatorsRenderAsTextNotLinks` |
| Related incident, derived at read time with one query per page. It is named only when the viewer may read that incident and the matching compartment; otherwise nothing is shown | done | T-231 |
| Management summary on the Command Summary: active incidents by severity, overdue critical actions (P1 or critical), overdue obligations, and median decision-loop time over the last 7 days. It covers live incidents and the compartments the viewer may read, with the definitions shown | done | T-232 |
| Four fixed aggregate queries, the same for 1 incident or 40 (T-199 amended) | done | `TestManagementSummaryUsesAFixedNumberOfQueries` |
| `crab.decision_loops`: a rebuildable projection that the kernel applies to every append, so no runtime can omit it. The ledger backfills it once on upgrade, and the result matches what the projection builds | done | `TestDecisionLoopBackfillMatchesProjection` |
| Browser runtime adapters stamp records with the kernel clock instead of the host clock (most of F-11) | done | T-232 (decision loops on the governed clock), full suite |

**Drift note.** `crab.decision_loops` is a new table. It is a rebuildable projection (`authority=rebuildable`) derived from the ledger, like every other projection; it is not a store. No scheduler, queue or independent principal was added. The rules are in `domain` and `app`; the web tier only renders. The policy-reference ratchet is unchanged.

**F-11 note.** The decision-loop time is computed from record timestamps, so those timestamps had to come from the governed clock. Every `cmd/crab` runtime adapter now uses the kernel clock. 17 host-clock reads remain (the CLI, adapters, web, and two data-tier helpers). They stay in C10, together with the static check.

## Mutation checks

| Rule removed | Failure |
|---|---|
| Exercise incidents excluded from the decision loop | `TestManagementSummary`: the median is no longer 30m |
| Only confirmed decisions counted | `TestManagementSummary`: the median is no longer 30m |
| Decision loop limited to scope pairs (hidden incidents) | `TestManagementSummary`: the median is no longer 30m |
| Related incident shown only to an authorised viewer | `TestObservables`: alice's evidence page reveals INC-B |

## Review gate

This was a self red-team review.

| Finding | Classification | Action |
|---|---|---|
| Closure is blocked by obligation clocks that are still running, but not by active clocks whose deadline has passed, so an incident can close with an overdue, unmet obligation (a C5 gap, found while defining *overdue* here) | real, outside C7 scope; it changes the closure rule | register F-12; product owner to confirm, then fix in C9 |
| The browser cannot set an action due date, so *overdue critical actions* counts only actions that got a due date another way (checklists or the API) | known product gap, not a C7 defect | recorded; T-232 sets due dates through a projection fixture |
| The observable status form returns to the page named in a `back` field | restricted to `/incident/…` paths on the same origin | no change |

## Gates

| Gate | Build environment | GitHub CI (`2265865`) |
|---|---|---|
| gofmt, vet | clean | pass |
| `go test -race -count=1 ./...` | pass | pass |
| Bafflewick | **10/10** | pass |
| visual preflight | pass | pass |
